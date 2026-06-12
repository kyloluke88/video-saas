package main

import (
	"log"
	"time"

	"worker/bootstrap"
	"worker/internal/app/consumer"
	"worker/internal/app/task"
	idiompipeline "worker/internal/app/workflow/idiom"
	podcastaudiopipeline "worker/internal/app/workflow/podcast/audio"
	podcastcomposepipeline "worker/internal/app/workflow/podcast/compose"
	podcastpagepipeline "worker/internal/app/workflow/podcast/page"
	practicalaudiopipeline "worker/internal/app/workflow/practical/audio"
	practicalcomposepipeline "worker/internal/app/workflow/practical/compose"
	practicalimagepipeline "worker/internal/app/workflow/practical/image"
	practicalpagepipeline "worker/internal/app/workflow/practical/page"
	uploadpipeline "worker/internal/app/workflow/upload"
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

	conn, err := bootstrap.SetupRabbitMQ()
	if err != nil {
		log.Fatalf("rabbitmq bootstrap failed: %v", err)
	}

	dispatcher := task.NewDispatcher(newTaskScheduler(workerRole), task.NewProjectLocker())
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

func newTaskScheduler(role string) map[string]task.TaskHandler {
	scheduler := make(map[string]task.TaskHandler)
	normalizedRole := task.NormalizeWorkerRole(role)

	if normalizedRole == task.WorkerRoleAll || normalizedRole == task.WorkerRoleMain {
		scheduler["plan.v1"] = idiompipeline.HandlePlan
		scheduler["scene.generate.v1"] = idiompipeline.HandleSceneGenerate
		scheduler["compose.v1"] = idiompipeline.HandleProjectCompose
		scheduler["practical.audio.generate.v1"] = practicalaudiopipeline.HandleGenerate
		scheduler["practical.image.generate.v1"] = practicalimagepipeline.HandleGenerate
		scheduler["practical.compose.render.v1"] = practicalcomposepipeline.HandleComposeRender
		scheduler["practical.page.persist.v1"] = practicalpagepipeline.HandlePersist
		scheduler["podcast.audio.generate.v1"] = podcastaudiopipeline.HandleGenerate
		scheduler["podcast.compose.render.v1"] = podcastcomposepipeline.HandleComposeRender
		scheduler["podcast.compose.finalize.v1"] = podcastcomposepipeline.HandleComposeFinalize
		scheduler["upload.v1"] = uploadpipeline.HandleUploadTask
		scheduler["podcast.page.persist.v1"] = podcastpagepipeline.HandlePersist
	}

	if normalizedRole == task.WorkerRoleAll || normalizedRole == task.WorkerRoleAlign {
		scheduler["practical.audio.align.v1"] = practicalaudiopipeline.HandleAlign
		scheduler["podcast.audio.align.v1"] = podcastaudiopipeline.HandleAlign
	}

	return scheduler
}
