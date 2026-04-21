package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"worker/internal/persistence"
	conf "worker/pkg/config"
	"worker/pkg/x/amqpx"
	services "worker/services"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Dispatcher struct {
	scheduler     map[string]TaskHandler
	projectLocker ProjectLocker
	taskTracker   TaskTracker
}

func NewDispatcher(scheduler map[string]TaskHandler, projectLocker ProjectLocker) Dispatcher {
	if projectLocker == nil {
		projectLocker = NoopProjectLocker{}
	}
	return Dispatcher{
		scheduler:     scheduler,
		projectLocker: projectLocker,
		taskTracker:   NewTaskTracker(),
	}
}

func (d Dispatcher) HandleMessage(ch *amqp.Channel, msg amqp.Delivery) error {
	return HandleMessage(ch, msg, d.scheduler, d.projectLocker, d.taskTracker)
}

func HandleMessage(ch *amqp.Channel, msg amqp.Delivery, scheduler map[string]TaskHandler, projectLocker ProjectLocker, taskTracker TaskTracker) error {
	if projectLocker == nil {
		projectLocker = NoopProjectLocker{}
	}
	if taskTracker == nil {
		taskTracker = NoopTaskTracker{}
	}

	retries := amqpx.HeaderRetry(msg.Headers)

	var task VideoTaskMessage
	if err := json.Unmarshal(msg.Body, &task); err != nil {
		_ = publishToDLQ(ch, msg.Body, retries+1)
		return msg.Ack(false)
	}

	return projectLocker.WithProject(taskProjectID(task), func() error {
		projectID := taskProjectID(task)
		if err := ensureProjectActive(projectID); err != nil {
			if errors.Is(err, context.Canceled) {
				log.Printf("🛑 跳过已取消任务 task_id=%s type=%s project_id=%s", task.TaskID, task.TaskType, projectID)
				if trackerErr := taskTracker.OnTaskCancelled(task, retries); trackerErr != nil {
					log.Printf("⚠️ task tracker cancel update failed task_id=%s project_id=%s err=%v", task.TaskID, projectID, trackerErr)
				}
				return msg.Ack(false)
			}
			log.Printf("❌ project activity check failed task_id=%s project_id=%s err=%v", task.TaskID, projectID, err)
			if isNonRetryable(err) {
				if dlqErr := publishToDLQ(ch, msg.Body, retries); dlqErr != nil {
					_ = msg.Nack(false, true)
					return dlqErr
				}
				return msg.Ack(false)
			}
			_ = msg.Nack(false, false)
			return err
		}

		materials := buildTaskMaterials(task)
		log.Printf("🎬 收到任务 task_id=%s type=%s retries=%d project_id=%s 所需材料=[%s]",
			task.TaskID, task.TaskType, retries, projectID, strings.Join(materials, ","))
		if err := taskTracker.OnTaskStart(task, retries); err != nil {
			if errors.Is(err, context.Canceled) {
				log.Printf("🛑 任务启动前已取消 task_id=%s type=%s project_id=%s", task.TaskID, task.TaskType, projectID)
				if trackerErr := taskTracker.OnTaskCancelled(task, retries); trackerErr != nil {
					log.Printf("⚠️ task tracker cancel update failed task_id=%s project_id=%s err=%v", task.TaskID, projectID, trackerErr)
				}
				return msg.Ack(false)
			}
			log.Printf("❌ task tracker start failed task_id=%s project_id=%s err=%v", task.TaskID, projectID, err)
			if isNonRetryable(err) {
				if dlqErr := publishToDLQ(ch, msg.Body, retries); dlqErr != nil {
					_ = msg.Nack(false, true)
					return dlqErr
				}
				return msg.Ack(false)
			}
			_ = msg.Nack(false, false)
			return err
		}

		taskCtx, cancel := context.WithCancel(context.Background())
		stopWatcher := startProjectCancellationWatcher(taskCtx, projectID, cancel)
		defer func() {
			stopWatcher()
			cancel()
		}()

		if err := processTask(taskCtx, ch, task, scheduler); err != nil {
			if errors.Is(err, context.Canceled) {
				log.Printf("🛑 任务已取消 task_id=%s type=%s project_id=%s", task.TaskID, task.TaskType, projectID)
				if trackerErr := taskTracker.OnTaskCancelled(task, retries); trackerErr != nil {
					log.Printf("⚠️ task tracker cancel update failed task_id=%s project_id=%s err=%v", task.TaskID, projectID, trackerErr)
				}
				return msg.Ack(false)
			}
			log.Printf("❌ 任务处理失败 task_id=%s project_id=%s err=%v", task.TaskID, projectID, err)
			if isNonRetryable(err) {
				log.Printf("⛔ 不可重试错误，任务终止且不再重试 task_id=%s project_id=%s", task.TaskID, projectID)
				if trackerErr := taskTracker.OnTaskFailed(task, retries, err); trackerErr != nil {
					log.Printf("⚠️ task tracker final failure update failed task_id=%s project_id=%s err=%v", task.TaskID, projectID, trackerErr)
				}
				if dlqErr := publishToDLQ(ch, msg.Body, retries); dlqErr != nil {
					_ = msg.Nack(false, true)
					return dlqErr
				}
				return msg.Ack(false)
			}
			if retries >= conf.Get[int]("worker.task_max_retries") {
				if trackerErr := taskTracker.OnTaskFailed(task, retries, err); trackerErr != nil {
					log.Printf("⚠️ task tracker max-retries failure update failed task_id=%s project_id=%s err=%v", task.TaskID, projectID, trackerErr)
				}
				if dlqErr := publishToDLQ(ch, msg.Body, retries+1); dlqErr != nil {
					_ = msg.Nack(false, true)
					return dlqErr
				}
				return msg.Ack(false)
			}
			if retryErr := publishToRetry(ch, msg.Body, retries+1); retryErr != nil {
				_ = msg.Nack(false, true)
				return retryErr
			}
			if trackerErr := taskTracker.OnTaskRetry(task, retries, err); trackerErr != nil {
				log.Printf("⚠️ task tracker retry update failed task_id=%s project_id=%s err=%v", task.TaskID, projectID, trackerErr)
			}
			log.Printf("🔁 任务进入延迟重试 task_id=%s next_retry=%d delay=%s project_id=%s", task.TaskID, retries+1, TaskRetryDelay(retries+1).String(), projectID)
			return msg.Ack(false)
		}

		log.Printf("✅ %s 任务处理完成 task_id=%s project_id=%s", task.TaskType, task.TaskID, projectID)
		if err := taskTracker.OnTaskSucceeded(task, retries); err != nil {
			log.Printf("⚠️ task tracker success update failed task_id=%s project_id=%s err=%v", task.TaskID, projectID, err)
		}

		return msg.Ack(false)
	})
}

func processTask(ctx context.Context, ch *amqp.Channel, task VideoTaskMessage, scheduler map[string]TaskHandler) error {
	handler, ok := scheduler[task.TaskType]
	if !ok {
		return fmt.Errorf("unsupported task type: %s", task.TaskType)
	}
	return handler(ctx, ch, task)
}

func publishToRetry(ch *amqp.Channel, body []byte, retries int) error {
	delay := TaskRetryDelay(retries)
	return ch.Publish(conf.Get[string]("worker.rabbitmq_exchange"), TaskRetryRoutingKey(conf.Get[string]("worker.rabbitmq_retry_routing_key"), retries), false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    nowUTC(),
		Body:         body,
		Headers: amqp.Table{
			"x-retry-count":    int32(retries),
			"x-retry-delay-ms": int32(delay / time.Millisecond),
		},
	})
}

func publishToDLQ(ch *amqp.Channel, body []byte, retries int) error {
	return ch.Publish(conf.Get[string]("worker.rabbitmq_dlx"), conf.Get[string]("worker.rabbitmq_dlq_routing_key"), false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    nowUTC(),
		Body:         body,
		Headers: amqp.Table{
			"x-retry-count": int32(retries),
		},
	})
}

func isNonRetryable(err error) bool {
	var svcPermanent services.NonRetryableError
	if errors.As(err, &svcPermanent) {
		return true
	}
	var storeFatal persistence.FatalError
	return errors.As(err, &storeFatal)
}

func taskProjectID(task VideoTaskMessage) string {
	if task.Payload == nil {
		return ""
	}
	value, ok := task.Payload["project_id"]
	if ok && value != nil {
		if projectID, ok := value.(string); ok {
			return projectID
		}
		return fmt.Sprint(value)
	}
	requestPayload, ok := task.Payload["request_payload"].(map[string]interface{})
	if !ok || requestPayload == nil {
		return ""
	}
	requestProjectID, ok := requestPayload["project_id"]
	if !ok || requestProjectID == nil {
		return ""
	}
	if projectID, ok := requestProjectID.(string); ok {
		return projectID
	}
	return fmt.Sprint(requestProjectID)
}

func buildTaskMaterials(task VideoTaskMessage) []string {
	materials := make([]string, 0, 16)
	seen := make(map[string]struct{}, 16)
	appendMaterial := func(name string) {
		text := strings.TrimSpace(name)
		if text == "" {
			return
		}
		if _, ok := seen[text]; ok {
			return
		}
		seen[text] = struct{}{}
		materials = append(materials, text)
	}

	switch strings.TrimSpace(task.TaskType) {
	case "upload.v1":
		appendMaterial("chat_script.pdf")
	case "podcast.audio.generate.v1":
		if scriptName := payloadString(task.Payload, "script_filename"); scriptName != "" {
			appendMaterial(filepath.Base(scriptName))
		}
	case "podcast.page.persist.v1":
		appendMaterial("request_payload.json")
		appendMaterial("script_aligned.json")
	case "podcast.audio.align.v1":
		appendMaterial("script_input.json")
		appendMaterial("blocks")
		appendMaterial("block_states")
	case "podcast.compose.render.v1":
		appendMaterial(firstBackgroundAsset(task.Payload))
		appendMaterial(podcastAnimationAsset(task.Payload))
		appendMaterial(podcastLogoAsset(task.Payload))
		appendMaterial("dialogue.mp3")
	case "podcast.compose.finalize.v1":
		appendMaterial("podcast_base.mp4")
		appendMaterial("script_aligned.json")
		appendMaterial("dialogue.mp3")
	default:
		if scriptName := payloadString(task.Payload, "script_filename"); scriptName != "" {
			appendMaterial(scriptName)
		}
		if backgrounds := payloadStringSlice(task.Payload, "bg_img_filenames"); len(backgrounds) > 0 {
			for _, bg := range backgrounds {
				appendMaterial(bg)
			}
		}
	}

	return materials
}

func payloadString(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func payloadStringSlice(payload map[string]interface{}, key string) []string {
	if payload == nil {
		return nil
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return nil
	}

	clean := func(raw string) string {
		text := strings.TrimSpace(raw)
		if text == "" || text == "<nil>" {
			return ""
		}
		return text
	}

	out := make([]string, 0)
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			if text := clean(item); text != "" {
				out = append(out, text)
			}
		}
	case []interface{}:
		for _, item := range typed {
			if text := clean(fmt.Sprint(item)); text != "" {
				out = append(out, text)
			}
		}
	default:
		if text := clean(fmt.Sprint(typed)); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func firstBackgroundAsset(payload map[string]interface{}) string {
	backgrounds := payloadStringSlice(payload, "bg_img_filenames")
	if len(backgrounds) == 0 {
		return ""
	}
	return filepath.Base(backgrounds[0])
}

func podcastAnimationAsset(payload map[string]interface{}) string {
	lang := strings.ToLower(strings.TrimSpace(payloadString(payload, "lang")))
	switch lang {
	case "ja":
		return "headphone.gif"
	case "zh":
		return "design2_480x480.gif"
	default:
		return ""
	}
}

func podcastLogoAsset(payload map[string]interface{}) string {
	lang := strings.ToLower(strings.TrimSpace(payloadString(payload, "lang")))
	switch lang {
	case "ja":
		return "ja_logo.png"
	default:
		return ""
	}
}
