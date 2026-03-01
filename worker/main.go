package main

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"worker/bootstrap"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Config struct {
	URL string

	Host     string
	Port     string
	Username string
	Password string
	VHost    string

	Exchange        string
	ExchangeType    string
	Queue           string
	RoutingKey      string
	RetryQueue      string
	RetryRoutingKey string
	DLX             string
	DLQ             string
	DLQRoutingKey   string
	RetryDelayMs    int
	Prefetch        int
	MaxRetries      int

	SeedanceBaseURL         string
	SeedanceGeneratePath    string
	SeedanceStatusPath      string
	SeedanceAPIKey          string
	SeedancePollIntervalSec int
	SeedanceMaxPollAttempts int
	SeedanceHTTPTimeoutSec  int
	SeedanceDryRunEnable    bool

	FFmpegPostprocessEnabled bool
	BGMEnable                bool
	FFmpegWorkDir            string
	FFmpegTimeoutSec         int

	TTSAPIURL   string
	TTSAPIKey   string
	TTSProvider string

	TTSTencentRegion          string
	TTSTencentSecretID        string
	TTSTencentSecretKey       string
	TTSTencentVoiceType       int64
	TTSTencentPrimaryLanguage int64
	TTSTencentModelType       int64
	TTSTencentCodec           string

	S3Enabled   bool
	S3Endpoint  string
	S3Region    string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
	S3PublicURL string
}

type VideoTaskMessage struct {
	TaskID    string                 `json:"task_id"`
	TaskType  string                 `json:"task_type"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt string                 `json:"created_at"`
}

type ProjectPlanPayload struct {
	ProjectID         string
	IdiomName         string
	IdiomNameEn       string
	Dynasty           string
	Platform          string
	Category          string
	NarrationLanguage string
	TargetDurationSec int
	ImageURLs         []string
	Characters        []string
	Props             []string
	SceneElements     []string
	Audience          string
	Tone              string
	AspectRatio       string
	Resolution        string
}

type VisualBible struct {
	StyleAnchor       string `json:"style_anchor,omitempty"`
	CharacterAnchor   string `json:"character_anchor,omitempty"`
	EnvironmentAnchor string `json:"environment_anchor,omitempty"`
	NegativePrompt    string `json:"negative_prompt,omitempty"`
}

type ObjectSpec struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type,omitempty"`
	Label     string                 `json:"label,omitempty"`
	Immutable map[string]interface{} `json:"immutable,omitempty"`
	Mutable   map[string]interface{} `json:"mutable,omitempty"`
}

type ScenePlan struct {
	Index       int                    `json:"index"`
	DurationSec int                    `json:"duration_sec"`
	Goal        string                 `json:"goal,omitempty"`
	ObjectsRef  []string               `json:"objects_ref,omitempty"`
	Composition map[string]interface{} `json:"composition,omitempty"`
	Action      []string               `json:"action,omitempty"`
	Prompt      string                 `json:"prompt"`
	Narration   string                 `json:"narration"`
}

type ProjectPlanResult struct {
	ProjectID         string       `json:"project_id"`
	Platform          string       `json:"platform"`
	Category          string       `json:"category"`
	NarrationLanguage string       `json:"narration_language"`
	TargetDurationSec int          `json:"target_duration_sec"`
	AspectRatio       string       `json:"aspect_ratio"`
	Resolution        string       `json:"resolution"`
	ImageURLs         []string     `json:"image_urls"`
	Characters        []string     `json:"characters,omitempty"`
	Props             []string     `json:"props,omitempty"`
	SceneElements     []string     `json:"scene_elements,omitempty"`
	NarrationFull     string       `json:"narration_full"`
	VisualBible       VisualBible  `json:"visual_bible,omitempty"`
	ObjectRegistry    []ObjectSpec `json:"object_registry,omitempty"`
	Scenes            []ScenePlan  `json:"scenes"`
	CreatedAt         string       `json:"created_at"`
}

type SeedanceGenerateRequest struct {
	Prompt        string   `json:"prompt"`
	AspectRatio   string   `json:"aspect_ratio,omitempty"`
	Resolution    string   `json:"resolution,omitempty"`
	Duration      string   `json:"duration,omitempty"`
	GenerateAudio bool     `json:"generate_audio"`
	FixedLens     bool     `json:"fixed_lens"`
	ImageURLs     []string `json:"image_urls,omitempty"`
}

type SeedanceGenerateResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TaskID string `json:"task_id"`
		Status string `json:"status"`
	} `json:"data"`
}

type SeedanceStatusResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TaskID   string   `json:"task_id"`
		Status   string   `json:"status"`
		Response []string `json:"response"`
		VideoURL string   `json:"video_url"`
		Error    string   `json:"error"`
		Output   []struct {
			URL string `json:"url"`
		} `json:"output"`
	} `json:"data"`
}

type nonRetryableError struct {
	err error
}

func (e nonRetryableError) Error() string { return e.err.Error() }

func main() {
	if err := bootstrap.Initialize(""); err != nil {
		log.Fatalf("bootstrap init failed: %v", err)
	}
	cfg := loadConfig()
	log.Printf("⚙️ FFMPEG_POSTPROCESS_ENABLED=%t FFMPEG_WORK_DIR=%s", cfg.FFmpegPostprocessEnabled, cfg.FFmpegWorkDir)
	log.Printf("⚙️ BGM_ENABLE=%t", cfg.BGMEnable)
	log.Printf("⚙️ S3_ENABLED=%t", cfg.S3Enabled)
	log.Printf("⚙️ SEEDANCE_DRY_RUN_ENABLE=%t", cfg.SeedanceDryRunEnable)
	log.Printf("⚙️ TTS_PROVIDER=%s TTS_ENABLED=%t", cfg.TTSProvider, isTTSEnabled(cfg))

	amqpURL := buildAMQPURL(cfg)
	scheduler := newTaskScheduler()
	for {
		conn, ch, err := connectAndPrepare(amqpURL, cfg)
		if err != nil {
			log.Printf("❌ RabbitMQ 初始化失败，3s 后重试: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

		msgs, err := ch.Consume(cfg.Queue, "", false, false, false, false, nil)
		if err != nil {
			_ = ch.Close()
			_ = conn.Close()
			log.Printf("❌ RabbitMQ 消费注册失败，3s 后重试: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

		log.Printf("🟡 Worker 启动，监听队列: %s", cfg.Queue)
		for msg := range msgs {
			if err := handleMessage(ch, cfg, msg, scheduler); err != nil {
				log.Printf("❌ 处理消息失败: %v", err)
			}
		}

		_ = ch.Close()
		_ = conn.Close()
		log.Println("⚠️ RabbitMQ 消费通道关闭，准备重连...")
		time.Sleep(2 * time.Second)
	}

}

type taskHandler func(ch *amqp.Channel, cfg Config, task VideoTaskMessage) error

func newTaskScheduler() map[string]taskHandler {
	return map[string]taskHandler{
		"plan.v1":           handlePlan,
		"scene.generate.v1": handleSceneGenerate,
		"compose.v1":        handleProjectCompose,
		"upload.v1":         handleUploadTask,
	}
}

func buildAMQPURL(cfg Config) string {
	if cfg.URL != "" {
		return cfg.URL
	}

	vhost := strings.TrimPrefix(cfg.VHost, "/")
	if cfg.VHost == "/" || cfg.VHost == "" {
		vhost = "%2f"
	} else {
		vhost = url.PathEscape(vhost)
	}
	return fmt.Sprintf("amqp://%s:%s@%s:%s/%s", cfg.Username, cfg.Password, cfg.Host, cfg.Port, vhost)
}

func setupTopology(ch *amqp.Channel, cfg Config) error {
	if err := ch.ExchangeDeclare(cfg.Exchange, cfg.ExchangeType, true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(cfg.DLX, "direct", true, false, false, false, nil); err != nil {
		return err
	}

	mainArgs := amqp.Table{
		"x-dead-letter-exchange":    cfg.DLX,
		"x-dead-letter-routing-key": cfg.DLQRoutingKey,
	}
	if _, err := ch.QueueDeclare(cfg.Queue, true, false, false, false, mainArgs); err != nil {
		return err
	}
	if err := ch.QueueBind(cfg.Queue, cfg.RoutingKey, cfg.Exchange, false, nil); err != nil {
		return err
	}

	if _, err := ch.QueueDeclare(cfg.DLQ, true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(cfg.DLQ, cfg.DLQRoutingKey, cfg.DLX, false, nil); err != nil {
		return err
	}

	retryArgs := amqp.Table{
		"x-message-ttl":             int32(cfg.RetryDelayMs),
		"x-dead-letter-exchange":    cfg.Exchange,
		"x-dead-letter-routing-key": cfg.RoutingKey,
	}
	if _, err := ch.QueueDeclare(cfg.RetryQueue, true, false, false, false, retryArgs); err != nil {
		return err
	}
	return ch.QueueBind(cfg.RetryQueue, cfg.RetryRoutingKey, cfg.Exchange, false, nil)
}

func connectAndPrepare(amqpURL string, cfg Config) (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	if err := setupTopology(ch, cfg); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, err
	}

	if err := ch.Qos(cfg.Prefetch, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, err
	}

	return conn, ch, nil
}
