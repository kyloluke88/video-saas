package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"worker/internal/dto"
	conf "worker/pkg/config"
	"worker/pkg/helpers"
	services "worker/services"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Dispatcher struct {
	scheduler     map[string]TaskHandler
	projectLocker ProjectLocker
}

func NewDispatcher(scheduler map[string]TaskHandler, projectLocker ProjectLocker) Dispatcher {
	if projectLocker == nil {
		projectLocker = NoopProjectLocker{}
	}
	return Dispatcher{
		scheduler:     scheduler,
		projectLocker: projectLocker,
	}
}

func (d Dispatcher) HandleMessage(ch *amqp.Channel, msg amqp.Delivery) error {
	return HandleMessage(ch, msg, d.scheduler, d.projectLocker)
}

func HandleMessage(ch *amqp.Channel, msg amqp.Delivery, scheduler map[string]TaskHandler, projectLocker ProjectLocker) error {
	if projectLocker == nil {
		projectLocker = NoopProjectLocker{}
	}

	retries := helpers.HeaderRetry(msg.Headers)

	var task dto.VideoTaskMessage
	if err := json.Unmarshal(msg.Body, &task); err != nil {
		_ = publishToDLQ(ch, msg.Body, retries+1)
		return msg.Ack(false)
	}

	return projectLocker.WithProject(taskProjectID(task), func() error {
		log.Printf("🎬 收到任务 task_id=%s type=%s retries=%d", task.TaskID, task.TaskType, retries)
		if err := processTask(ch, task, scheduler); err != nil {
			log.Printf("❌ 任务处理失败 task_id=%s: %v", task.TaskID, err)
			if isNonRetryable(err) {
				log.Printf("⛔ 不可重试错误，任务终止且不再重试 task_id=%s", task.TaskID)
				if dlqErr := publishToDLQ(ch, msg.Body, retries); dlqErr != nil {
					_ = msg.Nack(false, true)
					return dlqErr
				}
				return msg.Ack(false)
			}
			if retries >= conf.Get[int]("worker.task_max_retries") {
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
			log.Printf("🔁 任务进入延迟重试 task_id=%s next_retry=%d delay=%s", task.TaskID, retries+1, TaskRetryDelay(retries+1).String())
			return msg.Ack(false)
		}

		log.Printf("✅ 当前任务节点处理完成 task_id=%s type=%s project_id=%s", task.TaskID, task.TaskType, taskProjectID(task))

		return msg.Ack(false)
	})
}

func processTask(ch *amqp.Channel, task dto.VideoTaskMessage, scheduler map[string]TaskHandler) error {
	handler, ok := scheduler[task.TaskType]
	if !ok {
		return fmt.Errorf("unsupported task type: %s", task.TaskType)
	}
	return handler(ch, task)
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
	return errors.As(err, &svcPermanent)
}

func taskProjectID(task dto.VideoTaskMessage) string {
	if task.Payload == nil {
		return ""
	}
	value, ok := task.Payload["project_id"]
	if !ok || value == nil {
		return ""
	}
	if projectID, ok := value.(string); ok {
		return projectID
	}
	return fmt.Sprint(value)
}
