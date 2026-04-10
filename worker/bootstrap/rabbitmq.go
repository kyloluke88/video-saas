package bootstrap

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"worker/internal/app/task"
	conf "worker/pkg/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

func SetupRabbitMQ() (*amqp.Connection, error) {
	const (
		maxAttempts = 20
		retryDelay  = 2 * time.Second
	)

	amqpURL := buildAMQPURL()
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		conn, err := connectAndPrepareRabbitMQ(amqpURL)
		if err == nil {
			if attempt > 1 {
				log.Printf("✅ RabbitMQ connected after retry attempts=%d", attempt)
			}
			return conn, nil
		}

		lastErr = err
		log.Printf("⚠️ RabbitMQ connect failed attempt=%d/%d err=%v", attempt, maxAttempts, err)
		time.Sleep(retryDelay)
	}

	return nil, lastErr
}

func buildAMQPURL() string {
	if rawURL := conf.Get[string]("worker.rabbitmq_url"); rawURL != "" {
		return rawURL
	}

	vhostRaw := conf.Get[string]("worker.rabbitmq_vhost")
	vhost := strings.TrimPrefix(vhostRaw, "/")
	if vhostRaw == "/" || vhostRaw == "" {
		vhost = "%2f"
	} else {
		vhost = url.PathEscape(vhost)
	}
	return fmt.Sprintf(
		"amqp://%s:%s@%s:%s/%s",
		conf.Get[string]("worker.rabbitmq_user"),
		conf.Get[string]("worker.rabbitmq_password"),
		conf.Get[string]("worker.rabbitmq_host"),
		conf.Get[string]("worker.rabbitmq_port"),
		vhost,
	)
}

func setupRabbitMQTopology(ch *amqp.Channel) error {
	exchange := conf.Get[string]("worker.rabbitmq_exchange")
	exchangeType := conf.Get[string]("worker.rabbitmq_exchange_type")
	dlx := conf.Get[string]("worker.rabbitmq_dlx")
	queue := conf.Get[string]("worker.rabbitmq_queue")
	routingKey := conf.Get[string]("worker.rabbitmq_routing_key")
	dlq := conf.Get[string]("worker.rabbitmq_dlq")
	dlqRoutingKey := conf.Get[string]("worker.rabbitmq_dlq_routing_key")
	retryQueueBase := conf.Get[string]("worker.rabbitmq_retry_queue")
	retryRoutingKeyBase := conf.Get[string]("worker.rabbitmq_retry_routing_key")

	if err := ch.ExchangeDeclare(exchange, exchangeType, true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(dlx, "direct", true, false, false, false, nil); err != nil {
		return err
	}

	mainArgs := amqp.Table{
		"x-dead-letter-exchange":    dlx,
		"x-dead-letter-routing-key": dlqRoutingKey,
	}
	if _, err := ch.QueueDeclare(queue, true, false, false, false, mainArgs); err != nil {
		return err
	}
	if err := ch.QueueBind(queue, routingKey, exchange, false, nil); err != nil {
		return err
	}

	if _, err := ch.QueueDeclare(dlq, true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(dlq, dlqRoutingKey, dlx, false, nil); err != nil {
		return err
	}

	for attempt := 1; attempt <= 3; attempt++ {
		delay := task.TaskRetryDelay(attempt)
		retryArgs := amqp.Table{
			"x-message-ttl":             int32(delay / time.Millisecond),
			"x-dead-letter-exchange":    exchange,
			"x-dead-letter-routing-key": routingKey,
		}
		retryQueue := task.TaskRetryQueueName(retryQueueBase, attempt)
		retryRoutingKey := task.TaskRetryRoutingKey(retryRoutingKeyBase, attempt)
		if _, err := ch.QueueDeclare(retryQueue, true, false, false, false, retryArgs); err != nil {
			return err
		}
		if err := ch.QueueBind(retryQueue, retryRoutingKey, exchange, false, nil); err != nil {
			return err
		}
	}
	return nil
}

func connectAndPrepareRabbitMQ(amqpURL string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	defer ch.Close()

	if err := setupRabbitMQTopology(ch); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return conn, nil
}
