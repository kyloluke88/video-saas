package bootstrap

import (
	"api/pkg/config"
	"api/pkg/queue"
	"log"
	"time"
)

func SetupRabbitMQ() {
	if !config.Get[bool]("rabbitmq.enabled") {
		queue.SetEnabled(false)
		log.Printf("⚠️ RabbitMQ disabled by RABBITMQ_ENABLED=false")
		return
	}

	queue.SetEnabled(true)

	cfg := queue.RabbitConfig{
		URL: config.Get[string]("rabbitmq.url"),

		Host:     config.Get[string]("rabbitmq.host"),
		Port:     config.Get[string]("rabbitmq.port"),
		Username: config.Get[string]("rabbitmq.username"),
		Password: config.Get[string]("rabbitmq.password"),
		VHost:    config.Get[string]("rabbitmq.vhost"),

		Exchange:     config.Get[string]("rabbitmq.exchange"),
		ExchangeType: config.Get[string]("rabbitmq.exchange_type"),
		Queue:        config.Get[string]("rabbitmq.queue"),
		RoutingKey:   config.Get[string]("rabbitmq.routing_key"),

		DLX:           config.Get[string]("rabbitmq.dlx"),
		DLQ:           config.Get[string]("rabbitmq.dlq"),
		DLQRoutingKey: config.Get[string]("rabbitmq.dlq_routing_key"),

		RetryQueue:      config.Get[string]("rabbitmq.retry_queue"),
		RetryRoutingKey: config.Get[string]("rabbitmq.retry_routing_key"),
		RetryDelayMs:    config.Get[int]("rabbitmq.retry_delay_ms"),
	}

	const (
		maxAttempts = 20
		retryDelay  = 2 * time.Second
	)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := queue.ConnectRabbitMQ(cfg); err == nil {
			if attempt > 1 {
				log.Printf("✅ RabbitMQ connected after retry attempts=%d", attempt)
			}
			return
		} else {
			lastErr = err
			log.Printf("⚠️ RabbitMQ connect failed attempt=%d/%d err=%v", attempt, maxAttempts, err)
		}
		time.Sleep(retryDelay)
	}

	if lastErr != nil {
		panic(lastErr)
	}
}
