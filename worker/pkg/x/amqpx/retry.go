package amqpx

import amqp "github.com/rabbitmq/amqp091-go"

func HeaderRetry(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	if v, ok := headers["x-retry-count"]; ok {
		switch t := v.(type) {
		case int8:
			return int(t)
		case int16:
			return int(t)
		case int32:
			return int(t)
		case int64:
			return int(t)
		case int:
			return t
		}
	}
	return 0
}
