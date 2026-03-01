package config

import "worker/pkg/config"
import "github.com/spf13/cast"

func init() {
	config.Add("worker", func() map[string]interface{} {
		return map[string]interface{}{
			"rabbitmq_url":                 config.Env("RABBITMQ_URL", ""),
			"rabbitmq_host":                config.Env("RABBITMQ_HOST", "rabbitmq"),
			"rabbitmq_port":                config.Env("RABBITMQ_PORT", "5672"),
			"rabbitmq_user":                config.Env("RABBITMQ_USER", "guest"),
			"rabbitmq_password":            config.Env("RABBITMQ_PASSWORD", "guest"),
			"rabbitmq_vhost":               config.Env("RABBITMQ_VHOST", "/"),
			"rabbitmq_exchange":            config.Env("RABBITMQ_EXCHANGE", "video.tasks"),
			"rabbitmq_exchange_type":       config.Env("RABBITMQ_EXCHANGE_TYPE", "direct"),
			"rabbitmq_queue":               config.Env("RABBITMQ_QUEUE", "video.tasks.generate"),
			"rabbitmq_routing_key":         config.Env("RABBITMQ_ROUTING_KEY", "video.generate"),
			"rabbitmq_retry_queue":         config.Env("RABBITMQ_RETRY_QUEUE", "video.tasks.generate.retry"),
			"rabbitmq_retry_routing_key":   config.Env("RABBITMQ_RETRY_ROUTING_KEY", "video.generate.retry"),
			"rabbitmq_dlx":                 config.Env("RABBITMQ_DLX", "video.tasks.dlx"),
			"rabbitmq_dlq":                 config.Env("RABBITMQ_DLQ", "video.tasks.generate.dlq"),
			"rabbitmq_dlq_routing_key":     config.Env("RABBITMQ_DLQ_ROUTING_KEY", "video.generate.dlq"),
			"rabbitmq_retry_delay_ms":      cast.ToInt(config.Env("RABBITMQ_RETRY_DELAY_MS", 10000)),
			"rabbitmq_prefetch":            cast.ToInt(config.Env("RABBITMQ_PREFETCH", 20)),
			"task_max_retries":             cast.ToInt(config.Env("TASK_MAX_RETRIES", 3)),
			"seedance_base_url":            config.Env("SEEDANCE_BASE_URL", "https://seedanceapi.org/v1"),
			"seedance_generate_path":       config.Env("SEEDANCE_GENERATE_PATH", "/generate"),
			"seedance_status_path":         config.Env("SEEDANCE_STATUS_PATH", "/status"),
			"seedance_api_key":             config.Env("SEEDANCE_API_KEY", ""),
			"seedance_poll_interval_sec":   cast.ToInt(config.Env("SEEDANCE_POLL_INTERVAL_SEC", 8)),
			"seedance_max_poll_attempts":   cast.ToInt(config.Env("SEEDANCE_MAX_POLL_ATTEMPTS", 60)),
			"seedance_http_timeout_sec":    cast.ToInt(config.Env("SEEDANCE_HTTP_TIMEOUT_SEC", 30)),
			"seedance_dry_run_enable":      cast.ToBool(config.Env("SEEDANCE_DRY_RUN_ENABLE", true)),
			"ffmpeg_postprocess_enabled":   cast.ToBool(config.Env("FFMPEG_POSTPROCESS_ENABLED", true)),
			"bgm_enable":                   cast.ToBool(config.Env("BGM_ENABLE", false)),
			"ffmpeg_work_dir":              config.Env("FFMPEG_WORK_DIR", "/app/artifacts"),
			"ffmpeg_timeout_sec":           cast.ToInt(config.Env("FFMPEG_TIMEOUT_SEC", 300)),
			"tts_api_url":                  config.Env("TTS_API_URL", ""),
			"tts_api_key":                  config.Env("TTS_API_KEY", ""),
			"tts_provider":                 config.Env("TTS_PROVIDER", "http"),
			"tts_tencent_region":           config.Env("TTS_TENCENT_REGION", "ap-guangzhou"),
			"tts_tencent_secret_id":        config.Env("TTS_TENCENT_SECRET_ID", ""),
			"tts_tencent_secret_key":       config.Env("TTS_TENCENT_SECRET_KEY", ""),
			"tts_tencent_voice_type":       cast.ToInt64(config.Env("TTS_TENCENT_VOICE_TYPE", int64(101001))),
			"tts_tencent_primary_language": cast.ToInt64(config.Env("TTS_TENCENT_PRIMARY_LANGUAGE", int64(1))),
			"tts_tencent_model_type":       cast.ToInt64(config.Env("TTS_TENCENT_MODEL_TYPE", int64(1))),
			"tts_tencent_codec":            config.Env("TTS_TENCENT_CODEC", "mp3"),
			"s3_enabled":                   cast.ToBool(config.Env("S3_ENABLED", false)),
			"s3_endpoint":                  config.Env("S3_ENDPOINT", ""),
			"s3_region":                    config.Env("S3_REGION", "us-east-1"),
			"s3_bucket":                    config.Env("S3_BUCKET", ""),
			"s3_access_key":                config.Env("S3_ACCESS_KEY", ""),
			"s3_secret_key":                config.Env("S3_SECRET_KEY", ""),
			"s3_public_url":                config.Env("S3_PUBLIC_URL", ""),
		}
	})
}
