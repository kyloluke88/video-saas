package config

import (
	"strings"
	"worker/pkg/config"

	"github.com/spf13/cast"
)

func init() {
	config.Add("worker", func() map[string]interface{} {
		podcastMode := strings.ToLower(strings.TrimSpace(cast.ToString(config.Env("PODCAST_MODE", "debug"))))
		podcastX264Preset := strings.TrimSpace(cast.ToString(config.Env("PODCAST_X264_PRESET", "")))
		if podcastX264Preset == "" {
			if podcastMode == "production" {
				podcastX264Preset = "medium"
			} else {
				podcastX264Preset = "veryfast"
			}
		}
		podcastKeepASS := cast.ToBool(config.Env("PODCAST_KEEP_ASS", podcastMode == "debug"))
		return map[string]interface{}{
			"rabbitmq_url":                             config.Env("RABBITMQ_URL", ""),
			"rabbitmq_host":                            config.Env("RABBITMQ_HOST", "rabbitmq"),
			"rabbitmq_port":                            config.Env("RABBITMQ_PORT", "5672"),
			"rabbitmq_user":                            config.Env("RABBITMQ_USER", "guest"),
			"rabbitmq_password":                        config.Env("RABBITMQ_PASSWORD", "guest"),
			"rabbitmq_vhost":                           config.Env("RABBITMQ_VHOST", "/"),
			"rabbitmq_exchange":                        config.Env("RABBITMQ_EXCHANGE", "video.tasks"),
			"rabbitmq_exchange_type":                   config.Env("RABBITMQ_EXCHANGE_TYPE", "direct"),
			"rabbitmq_queue":                           config.Env("RABBITMQ_QUEUE", "video.tasks.generate"),
			"rabbitmq_routing_key":                     config.Env("RABBITMQ_ROUTING_KEY", "video.generate"),
			"rabbitmq_retry_queue":                     config.Env("RABBITMQ_RETRY_QUEUE", "video.tasks.generate.retry"),
			"rabbitmq_retry_routing_key":               config.Env("RABBITMQ_RETRY_ROUTING_KEY", "video.generate.retry"),
			"rabbitmq_dlx":                             config.Env("RABBITMQ_DLX", "video.tasks.dlx"),
			"rabbitmq_dlq":                             config.Env("RABBITMQ_DLQ", "video.tasks.generate.dlq"),
			"rabbitmq_dlq_routing_key":                 config.Env("RABBITMQ_DLQ_ROUTING_KEY", "video.generate.dlq"),
			"rabbitmq_retry_delay_ms":                  cast.ToInt(config.Env("RABBITMQ_RETRY_DELAY_MS", 10000)),
			"rabbitmq_prefetch":                        cast.ToInt(config.Env("RABBITMQ_PREFETCH", 20)),
			"task_max_retries":                         cast.ToInt(config.Env("TASK_MAX_RETRIES", 3)),
			"seedance_base_url":                        config.Env("SEEDANCE_BASE_URL", "https://seedanceapi.org/v1"),
			"seedance_generate_path":                   config.Env("SEEDANCE_GENERATE_PATH", "/generate"),
			"seedance_status_path":                     config.Env("SEEDANCE_STATUS_PATH", "/status"),
			"seedance_api_key":                         config.Env("SEEDANCE_API_KEY", ""),
			"seedance_poll_interval_sec":               cast.ToInt(config.Env("SEEDANCE_POLL_INTERVAL_SEC", 8)),
			"seedance_max_poll_attempts":               cast.ToInt(config.Env("SEEDANCE_MAX_POLL_ATTEMPTS", 60)),
			"seedance_http_timeout_sec":                cast.ToInt(config.Env("SEEDANCE_HTTP_TIMEOUT_SEC", 30)),
			"seedance_enabled":                         cast.ToBool(config.Env("SEEDANCE_ENABLED", true)),
			"ffmpeg_postprocess_enabled":               cast.ToBool(config.Env("FFMPEG_POSTPROCESS_ENABLED", true)),
			"bgm_enabled":                              cast.ToBool(config.Env("BGM_ENABLED", false)),
			"worker_assets_dir":                        config.Env("WORKER_ASSETS_DIR", "/app/assets"),
			"ffmpeg_work_dir":                          config.Env("FFMPEG_WORK_DIR", "/app/outputs"),
			"ffmpeg_timeout_sec":                       cast.ToInt(config.Env("FFMPEG_TIMEOUT_SEC", 300)),
			"podcast_mode":                             podcastMode,
			"podcast_x264_preset":                      podcastX264Preset,
			"podcast_keep_ass":                         podcastKeepASS,
			"idiom_tts_enabled":                        cast.ToBool(config.Env("IDIOM_TTS_ENABLED", true)),
			"tencent_tts_region":                       config.Env("TENCENT_TTS_REGION", "ap-guangzhou"),
			"tencent_tts_secret_id":                    config.Env("TENCENT_TTS_SECRET_ID", ""),
			"tencent_tts_secret_key":                   config.Env("TENCENT_TTS_SECRET_KEY", ""),
			"tencent_tts_voice_type":                   cast.ToInt64(config.Env("TENCENT_TTS_VOICE_TYPE", int64(101001))),
			"tencent_tts_primary_language":             cast.ToInt64(config.Env("TENCENT_TTS_PRIMARY_LANGUAGE", int64(1))),
			"tencent_tts_model_type":                   cast.ToInt64(config.Env("TENCENT_TTS_MODEL_TYPE", int64(1))),
			"tencent_tts_codec":                        config.Env("TENCENT_TTS_CODEC", "mp3"),
			"elevenlabs_tts_base_url":                  config.Env("ELEVENLABS_TTS_BASE_URL", "https://api.elevenlabs.io"),
			"elevenlabs_tts_api_key":                   config.Env("ELEVENLABS_TTS_API_KEY", ""),
			"elevenlabs_tts_voice_id":                  config.Env("ELEVENLABS_TTS_VOICE_ID", ""),
			"elevenlabs_tts_model_id":                  config.Env("ELEVENLABS_TTS_MODEL_ID", "eleven_multilingual_v2"),
			"elevenlabs_tts_output_format":             config.Env("ELEVENLABS_TTS_OUTPUT_FORMAT", "mp3_44100_128"),
			"elevenlabs_tts_enable_logging":            cast.ToBool(config.Env("ELEVENLABS_TTS_ENABLE_LOGGING", true)),
			"s3_enabled":                               cast.ToBool(config.Env("S3_ENABLED", false)),
			"s3_endpoint":                              config.Env("S3_ENDPOINT", ""),
			"s3_region":                                config.Env("S3_REGION", "us-east-1"),
			"s3_bucket":                                config.Env("S3_BUCKET", ""),
			"s3_access_key":                            config.Env("S3_ACCESS_KEY", ""),
			"s3_secret_key":                            config.Env("S3_SECRET_KEY", ""),
			"s3_public_url":                            config.Env("S3_PUBLIC_URL", ""),
			"tencent_tts_enabled":                      cast.ToBool(config.Env("TENCENT_TTS_ENABLED", true)),
			"elevenlabs_tts_enabled":                   cast.ToBool(config.Env("ELEVENLABS_TTS_ENABLED", true)),
			"tencent_podcast_segment_gap_ms":           cast.ToInt(config.Env("TENCENT_PODCAST_SEGMENT_GAP_MS", 220)),
			"tencent_podcast_same_speaker_gap_ms":      cast.ToInt(config.Env("TENCENT_PODCAST_SAME_SPEAKER_GAP_MS", 80)),
			"tencent_podcast_tts_sample_rate":          cast.ToInt64(config.Env("TENCENT_PODCAST_TTS_SAMPLE_RATE", int64(24000))),
			"tencent_podcast_daily_male_voice_type":    cast.ToInt64(config.Env("TENCENT_PODCAST_DAILY_MALE_VOICE_TYPE", int64(601008))),
			"tencent_podcast_daily_female_voice_type":  cast.ToInt64(config.Env("TENCENT_PODCAST_DAILY_FEMALE_VOICE_TYPE", int64(601010))),
			"tencent_podcast_public_male_voice_type":   cast.ToInt64(config.Env("TENCENT_PODCAST_PUBLIC_MALE_VOICE_TYPE", int64(601008))),
			"tencent_podcast_public_female_voice_type": cast.ToInt64(config.Env("TENCENT_PODCAST_PUBLIC_FEMALE_VOICE_TYPE", int64(601010))),
			"tencent_podcast_daily_male_speed":         cast.ToFloat64(config.Env("TENCENT_PODCAST_DAILY_MALE_SPEED", 0.8)),
			"tencent_podcast_daily_female_speed":       cast.ToFloat64(config.Env("TENCENT_PODCAST_DAILY_FEMALE_SPEED", 0.8)),
			"tencent_podcast_public_male_speed":        cast.ToFloat64(config.Env("TENCENT_PODCAST_PUBLIC_MALE_SPEED", 0.8)),
			"tencent_podcast_public_female_speed":      cast.ToFloat64(config.Env("TENCENT_PODCAST_PUBLIC_FEMALE_SPEED", 0.8)),
			"tencent_podcast_male_emotion":             config.Env("TENCENT_PODCAST_MALE_EMOTION", "happy"),
			"tencent_podcast_female_emotion":           config.Env("TENCENT_PODCAST_FEMALE_EMOTION", "happy"),
			"tencent_podcast_male_emotion_intensity":   cast.ToInt64(config.Env("TENCENT_PODCAST_MALE_EMOTION_INTENSITY", int64(100))),
			"tencent_podcast_female_emotion_intensity": cast.ToInt64(config.Env("TENCENT_PODCAST_FEMALE_EMOTION_INTENSITY", int64(100))),
			"elevenlabs_podcast_segment_gap_ms":        cast.ToInt(config.Env("ELEVENLABS_PODCAST_SEGMENT_GAP_MS", 240)),
			"elevenlabs_podcast_same_speaker_gap_ms":   cast.ToInt(config.Env("ELEVENLABS_PODCAST_SAME_SPEAKER_GAP_MS", 120)),
			"elevenlabs_podcast_dialogue_stability":    cast.ToFloat64(config.Env("ELEVENLABS_PODCAST_DIALOGUE_STABILITY", 0.47)),
			"elevenlabs_podcast_male_voice_id":         config.Env("ELEVENLABS_PODCAST_MALE_VOICE_ID", ""),
			"elevenlabs_podcast_female_voice_id":       config.Env("ELEVENLABS_PODCAST_FEMALE_VOICE_ID", ""),
		}
	})
}
