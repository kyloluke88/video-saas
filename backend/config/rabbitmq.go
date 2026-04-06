package config

import "api/pkg/config"
import "github.com/spf13/cast"

func init() {
	config.Add("rabbitmq", func() map[string]interface{} {
		return map[string]interface{}{
			"enabled": cast.ToBool(config.Env("RABBITMQ_ENABLED", true)),
			"url": config.Env("RABBITMQ_URL", ""),

			"host":     config.Env("RABBITMQ_HOST", "127.0.0.1"),
			"port":     config.Env("RABBITMQ_PORT", "5672"),
			"username": config.Env("RABBITMQ_USER", "guest"),
			"password": config.Env("RABBITMQ_PASSWORD", "guest"),
			"vhost":    config.Env("RABBITMQ_VHOST", "/"),

			"exchange":      config.Env("RABBITMQ_EXCHANGE", "video.tasks"),
			"exchange_type": config.Env("RABBITMQ_EXCHANGE_TYPE", "direct"),
			"queue":         config.Env("RABBITMQ_QUEUE", "video.tasks.generate"),
			"routing_key":   config.Env("RABBITMQ_ROUTING_KEY", "video.generate"),

			"dlx":             config.Env("RABBITMQ_DLX", "video.tasks.dlx"),
			"dlq":             config.Env("RABBITMQ_DLQ", "video.tasks.generate.dlq"),
			"dlq_routing_key": config.Env("RABBITMQ_DLQ_ROUTING_KEY", "video.generate.dlq"),

			"retry_queue":       config.Env("RABBITMQ_RETRY_QUEUE", "video.tasks.generate.retry"),
			"retry_routing_key": config.Env("RABBITMQ_RETRY_ROUTING_KEY", "video.generate.retry"),
			"retry_delay_ms":    cast.ToInt(config.Env("RABBITMQ_RETRY_DELAY_MS", 10000)),
		}
	})
}
