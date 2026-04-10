package task

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

type TaskHandler func(ctx context.Context, ch *amqp.Channel, task VideoTaskMessage) error
