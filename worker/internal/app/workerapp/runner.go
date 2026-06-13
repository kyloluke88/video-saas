package workerapp

import (
	"log"
	"time"

	"worker/bootstrap"
	"worker/internal/app/consumer"
	"worker/internal/app/task"
	conf "worker/pkg/config"
)

func RunQueue(queueName string, scheduler map[string]task.TaskHandler) error {
	conn, err := bootstrap.SetupRabbitMQ()
	if err != nil {
		return err
	}

	dispatcher := task.NewDispatcher(scheduler, task.NewProjectLocker())
	for {
		pool := consumer.Pool{
			Connection:  conn,
			Queue:       queueName,
			Prefetch:    conf.Get[int]("worker.rabbitmq_prefetch"),
			Concurrency: conf.Get[int]("worker.worker_concurrency"),
			Handler:     dispatcher.HandleMessage,
		}
		if err := pool.Run(); err != nil {
			_ = conn.Close()
			log.Printf("⚠️ RabbitMQ 消费连接关闭，准备重连: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		_ = conn.Close()
		for {
			conn, err = bootstrap.SetupRabbitMQ()
			if err != nil {
				log.Printf("⚠️ RabbitMQ 重连失败，3s 后重试: %v", err)
				time.Sleep(3 * time.Second)
				continue
			}
			break
		}
		time.Sleep(2 * time.Second)
	}
}
