package consumer

import (
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Handler func(ch *amqp.Channel, msg amqp.Delivery) error

type Pool struct {
	Connection  *amqp.Connection
	Queue       string
	Prefetch    int
	Concurrency int
	Handler     Handler
}

func (p Pool) Run() error {
	if p.Connection == nil {
		return fmt.Errorf("rabbitmq connection is required")
	}
	if p.Handler == nil {
		return fmt.Errorf("consumer handler is required")
	}
	if p.Queue == "" {
		return fmt.Errorf("rabbitmq queue is required")
	}

	concurrency := maxInt(p.Concurrency, 1)
	prefetch := maxInt(p.Prefetch, 1)
	log.Printf("🟡 Worker 启动，监听队列: %s consumers=%d prefetch=%d", p.Queue, concurrency, prefetch)

	for slot := 1; slot <= concurrency; slot++ {
		go p.runSlot(slot, prefetch)
	}

	closeCh := p.Connection.NotifyClose(make(chan *amqp.Error, 1))
	amqpErr, ok := <-closeCh
	if ok && amqpErr != nil {
		return fmt.Errorf("rabbitmq connection closed: %w", amqpErr)
	}
	return fmt.Errorf("rabbitmq connection closed")
}

func (p Pool) runSlot(slot int, prefetch int) {

	// 这里的目的不是重连整个 RabbitMQ，而是做“消费槽自愈”：
	// 如果某个 slot 的 channel 异常关闭了
	// 但整个 connection 还活着
	// 那就只重建这个 slot
	for {
		if p.Connection == nil || p.Connection.IsClosed() {
			return
		}

		err := p.consume(slot, prefetch)
		if err == nil {
			return
		}
		if p.Connection.IsClosed() {
			return
		}

		log.Printf("⚠️ consumer slot restart slot=%d queue=%s err=%v", slot, p.Queue, err)
		time.Sleep(time.Second)
	}
}

func (p Pool) consume(slot int, prefetch int) error {
	ch, err := p.Connection.Channel()
	if err != nil {
		return fmt.Errorf("consumer slot=%d open channel: %w", slot, err)
	}
	defer ch.Close()

	if err := ch.Qos(prefetch, 0, false); err != nil {
		return fmt.Errorf("consumer slot=%d qos: %w", slot, err)
	}

	chClose := ch.NotifyClose(make(chan *amqp.Error, 1))
	msgs, err := ch.Consume(p.Queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consumer slot=%d register: %w", slot, err)
	}

	log.Printf("🧵 consumer ready slot=%d queue=%s prefetch=%d", slot, p.Queue, prefetch)
	for msg := range msgs {
		if err := p.Handler(ch, msg); err != nil {
			log.Printf("❌ consumer handler failed slot=%d err=%v", slot, err)
			if ch.IsClosed() || p.Connection.IsClosed() {
				return fmt.Errorf("consumer slot=%d handler failed on closed channel: %w", slot, err)
			}
		}
	}

	select {
	case amqpErr, ok := <-chClose:
		if ok && amqpErr != nil && !p.Connection.IsClosed() {
			return fmt.Errorf("consumer slot=%d channel closed: %w", slot, amqpErr)
		}
	default:
	}

	if p.Connection.IsClosed() {
		return nil
	}
	return fmt.Errorf("consumer slot=%d delivery stream closed", slot)
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
