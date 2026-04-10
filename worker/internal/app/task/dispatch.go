package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
					log.Printf("⚠️ task tracker cancel update failed task_id=%s err=%v", task.TaskID, trackerErr)
				}
				return msg.Ack(false)
			}
			log.Printf("❌ project activity check failed task_id=%s err=%v", task.TaskID, err)
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

		log.Printf("🎬 收到任务 task_id=%s type=%s retries=%d", task.TaskID, task.TaskType, retries)
		if err := taskTracker.OnTaskStart(task, retries); err != nil {
			if errors.Is(err, context.Canceled) {
				log.Printf("🛑 任务启动前已取消 task_id=%s type=%s project_id=%s", task.TaskID, task.TaskType, projectID)
				if trackerErr := taskTracker.OnTaskCancelled(task, retries); trackerErr != nil {
					log.Printf("⚠️ task tracker cancel update failed task_id=%s err=%v", task.TaskID, trackerErr)
				}
				return msg.Ack(false)
			}
			log.Printf("❌ task tracker start failed task_id=%s err=%v", task.TaskID, err)
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
					log.Printf("⚠️ task tracker cancel update failed task_id=%s err=%v", task.TaskID, trackerErr)
				}
				return msg.Ack(false)
			}
			log.Printf("❌ 任务处理失败 task_id=%s: %v", task.TaskID, err)
			if isNonRetryable(err) {
				log.Printf("⛔ 不可重试错误，任务终止且不再重试 task_id=%s", task.TaskID)
				if trackerErr := taskTracker.OnTaskFailed(task, retries, err); trackerErr != nil {
					log.Printf("⚠️ task tracker final failure update failed task_id=%s err=%v", task.TaskID, trackerErr)
				}
				if dlqErr := publishToDLQ(ch, msg.Body, retries); dlqErr != nil {
					_ = msg.Nack(false, true)
					return dlqErr
				}
				return msg.Ack(false)
			}
			if retries >= conf.Get[int]("worker.task_max_retries") {
				if trackerErr := taskTracker.OnTaskFailed(task, retries, err); trackerErr != nil {
					log.Printf("⚠️ task tracker max-retries failure update failed task_id=%s err=%v", task.TaskID, trackerErr)
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
				log.Printf("⚠️ task tracker retry update failed task_id=%s err=%v", task.TaskID, trackerErr)
			}
			log.Printf("🔁 任务进入延迟重试 task_id=%s next_retry=%d delay=%s", task.TaskID, retries+1, TaskRetryDelay(retries+1).String())
			return msg.Ack(false)
		}

		log.Printf("✅ 当前任务节点处理完成 task_id=%s type=%s project_id=%s", task.TaskID, task.TaskType, taskProjectID(task))
		if err := taskTracker.OnTaskSucceeded(task, retries); err != nil {
			log.Printf("⚠️ task tracker success update failed task_id=%s err=%v", task.TaskID, err)
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
