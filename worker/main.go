package main

import (
	"log"

	"worker/bootstrap"
	"worker/internal/app/task"
	"worker/internal/app/workerapp"
	conf "worker/pkg/config"
	"worker/pkg/googlecloud"
)

func main() {
	if err := bootstrap.Initialize(""); err != nil {
		log.Fatalf("bootstrap init failed: %v", err)
	}
	workerRole := task.NormalizeWorkerRole(conf.Get[string]("worker.worker_role"))
	if task.SplitQueuesEnabled() && workerRole == task.WorkerRoleAll {
		log.Fatal("worker_role must be main or align when RABBITMQ_SPLIT_QUEUES=true")
	}
	queueName, err := task.QueueForWorkerRole(workerRole)
	if err != nil {
		log.Fatalf("resolve worker queue failed: %v", err)
	}
	log.Printf("⚙️ FFMPEG_WORK_DIR=%s", conf.Get[string]("worker.ffmpeg_work_dir"))
	log.Printf("⚙️ BGM_ENABLED=%t", conf.Get[bool]("worker.bgm_enabled"))
	log.Printf("⚙️ S3_ENABLED=%t", conf.Get[bool]("worker.s3_enabled"))
	log.Printf("⚙️ SEEDANCE_ENABLED=%t", conf.Get[bool]("worker.seedance_enabled"))
	log.Printf("⚙️ GOOGLE_TTS_ENABLED=%t GOOGLE_TTS_MODEL=%s",
		conf.Get[bool]("worker.google_tts_enabled"),
		googlecloud.DefaultTTSModel,
	)
	log.Printf("⚙️ ELEVENLABS_TTS_ENABLED=%t ELEVENLABS_TTS_MODEL=%s",
		conf.Get[bool]("worker.elevenlabs_tts_enabled"),
		conf.Get[string]("worker.elevenlabs_tts_model"),
	)
	log.Printf("⚙️ WORKER_CONCURRENCY=%d RABBITMQ_PREFETCH=%d",
		conf.Get[int]("worker.worker_concurrency"),
		conf.Get[int]("worker.rabbitmq_prefetch"),
	)
	log.Printf("⚙️ WORKER_ROLE=%s RABBITMQ_SPLIT_QUEUES=%t RABBITMQ_QUEUE=%s",
		workerRole,
		task.SplitQueuesEnabled(),
		queueName,
	)

	if err := workerapp.RunQueue(queueName, workerapp.SchedulerForRole(workerRole)); err != nil {
		log.Fatalf("rabbitmq worker loop failed: %v", err)
	}
}
