package task

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	conf "worker/pkg/config"
	"worker/pkg/x/mapx"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishTask(ch *amqp.Channel, taskType string, payload map[string]interface{}) error {
	if err := ensureProjectActive(taskProjectID(VideoTaskMessage{Payload: payload})); err != nil {
		return err
	}

	taskID := fmt.Sprintf("%s-%d", taskIDPrefixFromPayload(payload), time.Now().UnixNano())
	body, err := json.Marshal(VideoTaskMessage{
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

func taskIDPrefixFromPayload(payload map[string]interface{}) string {
	if payload == nil {
		return "task"
	}
	if raw := strings.TrimSpace(mapx.GetString(payload, "project_id")); raw != "" {
		return raw
	}
	if requestPayload, ok := payload["request_payload"].(map[string]interface{}); ok {
		if raw := strings.TrimSpace(mapx.GetString(requestPayload, "project_id")); raw != "" {
			return raw
		}
	}
	raw := strings.TrimSpace(mapx.GetString(payload, "idiom_name_en"))
	if raw == "" {
		return "task"
	}
	if slug := toSafeSlug(raw); slug != "" {
		return slug
	}
	return "task"
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
