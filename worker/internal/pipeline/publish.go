package pipeline

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"worker/internal/dto"
	conf "worker/pkg/config"
	"worker/pkg/helpers"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishTask(ch *amqp.Channel, taskType string, payload map[string]interface{}) error {
	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	if suffix := taskIDSuffixFromPayload(payload); suffix != "" {
		taskID = fmt.Sprintf("%s-%s", taskID, suffix)
	}
	body, err := json.Marshal(dto.VideoTaskMessage{
		TaskID:    taskID,
		TaskType:  taskType,
		Payload:   payload,
		CreatedAt: nowUTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	return ch.Publish(conf.Get[string]("worker.rabbitmq_exchange"), conf.Get[string]("worker.rabbitmq_routing_key"), false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    nowUTC(),
		Body:         body,
	})
}

func taskIDSuffixFromPayload(payload map[string]interface{}) string {
	if payload == nil {
		return ""
	}
	raw := strings.TrimSpace(helpers.GetString(payload, "idiom_name_en"))
	if raw == "" {
		return ""
	}
	return toSafeSlug(raw)
}

func toSafeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	return s
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
