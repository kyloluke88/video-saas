package main

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"worker/bootstrap"
	"worker/internal/consumer"
	"worker/internal/pipeline"
	idiompipeline "worker/internal/pipeline/idiom"
	podcastaudiopipeline "worker/internal/pipeline/podcast_audio"
	podcastcomposepipeline "worker/internal/pipeline/podcast_compose"
	conf "worker/pkg/config"

	amqp "github.com/rabbitmq/amqp091-go"
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
		conf.Get[string]("worker.google_tts_model"),
	)
	log.Printf("⚙️ ELEVENLABS_TTS_ENABLED=%t ELEVENLABS_TTS_MODEL=%s",
		conf.Get[bool]("worker.elevenlabs_tts_enabled"),
		conf.Get[string]("worker.elevenlabs_tts_model"),
	)
	log.Printf("⚙️ WORKER_CONCURRENCY=%d RABBITMQ_PREFETCH=%d",
		conf.Get[int]("worker.worker_concurrency"),
		conf.Get[int]("worker.rabbitmq_prefetch"),
	)

	amqpURL := buildAMQPURL()
	dispatcher := pipeline.NewDispatcher(newTaskScheduler(), pipeline.NewProjectLocker())
	for {
		conn, err := connectAndPrepare(amqpURL)
		if err != nil {
			log.Printf("❌ RabbitMQ 初始化失败，3s 后重试: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

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
		time.Sleep(2 * time.Second)
	}

}

func newTaskScheduler() map[string]pipeline.TaskHandler {
	return map[string]pipeline.TaskHandler{
		"plan.v1":                   idiompipeline.HandlePlan,
		"scene.generate.v1":         idiompipeline.HandleSceneGenerate,
		"compose.v1":                idiompipeline.HandleProjectCompose,
		"podcast.audio.generate.v1": podcastaudiopipeline.HandleGenerate,
		"podcast.compose.v1":        podcastcomposepipeline.HandleCompose,
		"upload.v1":                 idiompipeline.HandleUploadTask,
	}
}

func buildAMQPURL() string {
	if rawURL := conf.Get[string]("worker.rabbitmq_url"); rawURL != "" {
		return rawURL
	}

	vhostRaw := conf.Get[string]("worker.rabbitmq_vhost")
	vhost := strings.TrimPrefix(vhostRaw, "/")
	if vhostRaw == "/" || vhostRaw == "" {
		vhost = "%2f"
	} else {
		vhost = url.PathEscape(vhost)
	}
	return fmt.Sprintf(
		"amqp://%s:%s@%s:%s/%s",
		conf.Get[string]("worker.rabbitmq_user"),
		conf.Get[string]("worker.rabbitmq_password"),
		conf.Get[string]("worker.rabbitmq_host"),
		conf.Get[string]("worker.rabbitmq_port"),
		vhost,
	)
}

func setupTopology(ch *amqp.Channel) error {
	exchange := conf.Get[string]("worker.rabbitmq_exchange")
	exchangeType := conf.Get[string]("worker.rabbitmq_exchange_type")
	dlx := conf.Get[string]("worker.rabbitmq_dlx")
	queue := conf.Get[string]("worker.rabbitmq_queue")
	routingKey := conf.Get[string]("worker.rabbitmq_routing_key")
	dlq := conf.Get[string]("worker.rabbitmq_dlq")
	dlqRoutingKey := conf.Get[string]("worker.rabbitmq_dlq_routing_key")
	retryQueueBase := conf.Get[string]("worker.rabbitmq_retry_queue")
	retryRoutingKeyBase := conf.Get[string]("worker.rabbitmq_retry_routing_key")

	if err := ch.ExchangeDeclare(exchange, exchangeType, true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(dlx, "direct", true, false, false, false, nil); err != nil {
		return err
	}

	mainArgs := amqp.Table{
		"x-dead-letter-exchange":    dlx,
		"x-dead-letter-routing-key": dlqRoutingKey,
	}
	if _, err := ch.QueueDeclare(queue, true, false, false, false, mainArgs); err != nil {
		return err
	}
	if err := ch.QueueBind(queue, routingKey, exchange, false, nil); err != nil {
		return err
	}

	if _, err := ch.QueueDeclare(dlq, true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(dlq, dlqRoutingKey, dlx, false, nil); err != nil {
		return err
	}

	for attempt := 1; attempt <= 3; attempt++ {
		delay := pipeline.TaskRetryDelay(attempt)
		retryArgs := amqp.Table{
			"x-message-ttl":             int32(delay / time.Millisecond),
			"x-dead-letter-exchange":    exchange,
			"x-dead-letter-routing-key": routingKey,
		}
		retryQueue := pipeline.TaskRetryQueueName(retryQueueBase, attempt)
		retryRoutingKey := pipeline.TaskRetryRoutingKey(retryRoutingKeyBase, attempt)
		if _, err := ch.QueueDeclare(retryQueue, true, false, false, false, retryArgs); err != nil {
			return err
		}
		if err := ch.QueueBind(retryQueue, retryRoutingKey, exchange, false, nil); err != nil {
			return err
		}
	}
	return nil
}

func connectAndPrepare(amqpURL string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	defer ch.Close()

	if err := setupTopology(ch); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return conn, nil
}
