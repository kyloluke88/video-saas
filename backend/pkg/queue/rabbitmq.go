package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"api/pkg/logger"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type RabbitConfig struct {
	URL string

	Host     string
	Port     string
	Username string
	Password string
	VHost    string

	Exchange     string
	ExchangeType string
	Queue        string
	RoutingKey   string

	DLX           string
	DLQ           string
	DLQRoutingKey string

	RetryQueue      string
	RetryRoutingKey string
	RetryDelayMs    int
}

type RabbitClient struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	cfg  RabbitConfig
}

type VideoTaskMessage struct {
	TaskID    string                 `json:"task_id"`
	TaskType  string                 `json:"task_type"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt string                 `json:"created_at"`
}

var Rabbit *RabbitClient
var rabbitMu sync.Mutex

func ConnectRabbitMQ(cfg RabbitConfig) error {
	rabbitMu.Lock()
	defer rabbitMu.Unlock()

	if Rabbit != nil && Rabbit.isOpen() {
		return nil
	}

	client, err := newRabbitClient(cfg)
	if err != nil {
		return err
	}
	Rabbit = client
	return nil
}

func Ping() error {
	if Rabbit == nil || !Rabbit.isOpen() {
		return fmt.Errorf("rabbitmq is not connected")
	}
	return nil
}

func PublishVideoTask(taskType string, payload map[string]interface{}) (string, error) {
	if err := ensureConnected(); err != nil {
		return "", err
	}

	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	if suffix := taskIDSuffixFromPayload(payload); suffix != "" {
		taskID = fmt.Sprintf("%s-%s", taskID, suffix)
	}
	body, err := json.Marshal(VideoTaskMessage{
		TaskID:    taskID,
		TaskType:  taskType,
		Payload:   payload,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = Rabbit.ch.PublishWithContext(
		ctx,
		Rabbit.cfg.Exchange,
		Rabbit.cfg.RoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now().UTC(),
			Body:         body,
		},
	)
	if err != nil {
		// 热重载或网络抖动后，channel 可能失效，尝试一次重连重发
		if reconnErr := reconnectWithCurrentConfig(); reconnErr != nil {
			return "", err
		}
		err = Rabbit.ch.PublishWithContext(
			ctx,
			Rabbit.cfg.Exchange,
			Rabbit.cfg.RoutingKey,
			false,
			false,
			amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent,
				Timestamp:    time.Now().UTC(),
				Body:         body,
			},
		)
		if err != nil {
			return "", err
		}
	}

	return taskID, nil
}

func taskIDSuffixFromPayload(payload map[string]interface{}) string {
	if payload == nil {
		return ""
	}
	raw, _ := payload["idiom_name_en"].(string)
	raw = strings.TrimSpace(raw)
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
		s = s[:40]
		s = strings.Trim(s, "-")
	}
	return s
}

func newRabbitClient(cfg RabbitConfig) (*RabbitClient, error) {
	amqpURL := cfg.URL
	if amqpURL == "" {
		vhost := strings.TrimPrefix(cfg.VHost, "/")
		if cfg.VHost == "/" || cfg.VHost == "" {
			vhost = "%2f"
		} else {
			vhost = url.PathEscape(vhost)
		}
		amqpURL = fmt.Sprintf("amqp://%s:%s@%s:%s/%s", cfg.Username, cfg.Password, cfg.Host, cfg.Port, vhost)
	}

	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	client := &RabbitClient{conn: conn, ch: ch, cfg: cfg}
	if err := client.setupTopology(); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}

	logger.Info("RabbitMQ", zap.String("exchange", cfg.Exchange), zap.String("queue", cfg.Queue))
	return client, nil
}

func (r *RabbitClient) isOpen() bool {
	if r == nil || r.conn == nil || r.conn.IsClosed() || r.ch == nil || r.ch.IsClosed() {
		return false
	}
	return true
}

func ensureConnected() error {
	rabbitMu.Lock()
	defer rabbitMu.Unlock()

	if Rabbit == nil {
		return fmt.Errorf("rabbitmq is not initialized")
	}
	if Rabbit.isOpen() {
		return nil
	}
	return reconnectLocked(Rabbit.cfg)
}

func reconnectWithCurrentConfig() error {
	rabbitMu.Lock()
	defer rabbitMu.Unlock()

	if Rabbit == nil {
		return fmt.Errorf("rabbitmq is not initialized")
	}
	return reconnectLocked(Rabbit.cfg)
}

func reconnectLocked(cfg RabbitConfig) error {
	client, err := newRabbitClient(cfg)
	if err != nil {
		return err
	}
	if Rabbit != nil {
		if Rabbit.ch != nil {
			_ = Rabbit.ch.Close()
		}
		if Rabbit.conn != nil {
			_ = Rabbit.conn.Close()
		}
	}
	Rabbit = client
	return nil
}

func (r *RabbitClient) setupTopology() error {
	if err := r.ch.ExchangeDeclare(
		r.cfg.Exchange,
		r.cfg.ExchangeType,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	if err := r.ch.ExchangeDeclare(
		r.cfg.DLX,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	mainArgs := amqp.Table{
		"x-dead-letter-exchange":    r.cfg.DLX,
		"x-dead-letter-routing-key": r.cfg.DLQRoutingKey,
	}
	if _, err := r.ch.QueueDeclare(
		r.cfg.Queue,
		true,
		false,
		false,
		false,
		mainArgs,
	); err != nil {
		return err
	}
	if err := r.ch.QueueBind(r.cfg.Queue, r.cfg.RoutingKey, r.cfg.Exchange, false, nil); err != nil {
		return err
	}

	if _, err := r.ch.QueueDeclare(
		r.cfg.DLQ,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}
	if err := r.ch.QueueBind(r.cfg.DLQ, r.cfg.DLQRoutingKey, r.cfg.DLX, false, nil); err != nil {
		return err
	}

	retryArgs := amqp.Table{
		"x-message-ttl":             int32(r.cfg.RetryDelayMs),
		"x-dead-letter-exchange":    r.cfg.Exchange,
		"x-dead-letter-routing-key": r.cfg.RoutingKey,
	}
	if _, err := r.ch.QueueDeclare(
		r.cfg.RetryQueue,
		true,
		false,
		false,
		false,
		retryArgs,
	); err != nil {
		return err
	}
	return r.ch.QueueBind(r.cfg.RetryQueue, r.cfg.RetryRoutingKey, r.cfg.Exchange, false, nil)
}
