package pipeline

import (
	"worker/internal/dto"

	amqp "github.com/rabbitmq/amqp091-go"
)

type TaskHandler func(ch *amqp.Channel, task dto.VideoTaskMessage) error
