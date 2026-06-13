package main

import (
	"log"
	"os"

	"worker/bootstrap"
	"worker/internal/app/task"
	"worker/internal/app/workerapp"
	conf "worker/pkg/config"
)

func main() {
	if err := bootstrap.InitializeWithOptions("", bootstrap.InitOptions{
		WithDB:    true,
		WithRedis: false,
	}); err != nil {
		log.Fatalf("bootstrap init failed: %v", err)
	}
	if !task.SplitQueuesEnabled() {
		log.Fatal("worker-align requires RABBITMQ_SPLIT_QUEUES=true")
	}

	queueName, err := task.QueueForWorkerRole(task.WorkerRoleAlign)
	if err != nil {
		log.Fatalf("resolve worker queue failed: %v", err)
	}
	log.Printf("⚙️ WORKER_KIND=align RABBITMQ_QUEUE=%s", queueName)
	log.Printf("⚙️ WORKER_CONCURRENCY=%d RABBITMQ_PREFETCH=%d",
		conf.Get[int]("worker.worker_concurrency"),
		conf.Get[int]("worker.rabbitmq_prefetch"),
	)
	log.Printf("⚙️ FFMPEG_WORK_DIR=%s", conf.Get[string]("worker.ffmpeg_work_dir"))
	log.Printf("⚙️ MFA_ENABLED=%t MFA_ROOT_DIR=%s",
		conf.Get[bool]("worker.mfa_enabled"),
		os.Getenv("MFA_ROOT_DIR"),
	)

	if err := workerapp.RunQueue(queueName, workerapp.SchedulerForRole(task.WorkerRoleAlign)); err != nil {
		log.Fatalf("rabbitmq worker loop failed: %v", err)
	}
}
