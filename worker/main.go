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
	uploadpipeline "worker/internal/app/workflow/upload"
	"worker/pkg/googlecloud"
	conf "worker/pkg/config"
)

func main() {
	if err := bootstrap.Initialize(""); err != nil {
		log.Fatalf("bootstrap init failed: %v", err)
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

	conn, err := bootstrap.SetupRabbitMQ()
	if err != nil {
		log.Fatalf("rabbitmq bootstrap failed: %v", err)
	}

	dispatcher := task.NewDispatcher(newTaskScheduler(), task.NewProjectLocker())
	for {
		pool := consumer.Pool{
			Connection:  conn,
			Queue:       conf.Get[string]("worker.rabbitmq_queue"),
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

func newTaskScheduler() map[string]task.TaskHandler {
	return map[string]task.TaskHandler{
		"plan.v1":                     idiompipeline.HandlePlan,
		"scene.generate.v1":           idiompipeline.HandleSceneGenerate,
		"compose.v1":                  idiompipeline.HandleProjectCompose,
		"podcast.audio.generate.v1":   podcastaudiopipeline.HandleGenerate,
		"podcast.audio.align.v1":      podcastaudiopipeline.HandleAlign,
		"podcast.compose.render.v1":   podcastcomposepipeline.HandleComposeRender,
		"podcast.compose.finalize.v1": podcastcomposepipeline.HandleComposeFinalize,
		"upload.v1":                   uploadpipeline.HandleUploadTask,
		"podcast.page.persist.v1":     podcastpagepipeline.HandlePersist,
	}
}
