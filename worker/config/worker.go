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
			"rabbitmq_url":                     config.Env("RABBITMQ_URL", ""),
			"rabbitmq_host":                    config.Env("RABBITMQ_HOST", "rabbitmq"),
			"rabbitmq_port":                    config.Env("RABBITMQ_PORT", "5672"),
			"rabbitmq_user":                    config.Env("RABBITMQ_USER", "guest"),
			"rabbitmq_password":                config.Env("RABBITMQ_PASSWORD", "guest"),
			"rabbitmq_vhost":                   config.Env("RABBITMQ_VHOST", "/"),
			"rabbitmq_exchange":                config.Env("RABBITMQ_EXCHANGE", "video.tasks"),
			"rabbitmq_exchange_type":           config.Env("RABBITMQ_EXCHANGE_TYPE", "direct"),
			"rabbitmq_queue":                   config.Env("RABBITMQ_QUEUE", "video.tasks.generate"),
			"rabbitmq_routing_key":             config.Env("RABBITMQ_ROUTING_KEY", "video.generate"),
			"rabbitmq_retry_queue":             config.Env("RABBITMQ_RETRY_QUEUE", "video.tasks.generate.retry"),
			"rabbitmq_retry_routing_key":       config.Env("RABBITMQ_RETRY_ROUTING_KEY", "video.generate.retry"),
			"rabbitmq_dlx":                     config.Env("RABBITMQ_DLX", "video.tasks.dlx"),
			"rabbitmq_dlq":                     config.Env("RABBITMQ_DLQ", "video.tasks.generate.dlq"),
			"rabbitmq_dlq_routing_key":         config.Env("RABBITMQ_DLQ_ROUTING_KEY", "video.generate.dlq"),
			"rabbitmq_retry_delay_ms":          cast.ToInt(config.Env("RABBITMQ_RETRY_DELAY_MS", 10000)),
			"rabbitmq_prefetch":                cast.ToInt(config.Env("RABBITMQ_PREFETCH", 1)),
			"worker_concurrency":               cast.ToInt(config.Env("WORKER_CONCURRENCY", 2)),
			"task_max_retries":                 cast.ToInt(config.Env("TASK_MAX_RETRIES", 3)),
			"db_connection":                    config.Env("DB_CONNECTION", "postgresql"),
			"db_host":                          config.Env("DB_HOST", "postgres"),
			"db_port":                          config.Env("DB_PORT", "5432"),
			"db_database":                      config.Env("DB_DATABASE", config.Env("POSTGRES_DB", "video_db")),
			"db_username":                      config.Env("DB_USERNAME", config.Env("POSTGRES_USER", "video")),
			"db_password":                      config.Env("DB_PASSWORD", config.Env("POSTGRES_PASSWORD", "")),
			"db_sslmode":                       config.Env("DB_SSLMODE", "disable"),
			"db_max_idle_connections":          cast.ToInt(config.Env("DB_MAX_IDLE_CONNECTIONS", 10)),
			"db_max_open_connections":          cast.ToInt(config.Env("DB_MAX_OPEN_CONNECTIONS", 25)),
			"db_max_life_seconds":              cast.ToInt(config.Env("DB_MAX_LIFE_SECONDS", 300)),
			"redis_host":                       config.Env("REDIS_HOST", "redis"),
			"redis_port":                       config.Env("REDIS_PORT", "6379"),
			"redis_username":                   config.Env("REDIS_USERNAME", ""),
			"redis_password":                   config.Env("REDIS_PASSWORD", ""),
			"redis_database":                   cast.ToInt(config.Env("REDIS_MAIN_DB", 1)),
			"seedance_base_url":                config.Env("SEEDANCE_BASE_URL", "https://seedanceapi.org/v1"),
			"seedance_generate_path":           config.Env("SEEDANCE_GENERATE_PATH", "/generate"),
			"seedance_status_path":             config.Env("SEEDANCE_STATUS_PATH", "/status"),
			"seedance_api_key":                 config.Env("SEEDANCE_API_KEY", ""),
			"seedance_poll_interval_sec":       cast.ToInt(config.Env("SEEDANCE_POLL_INTERVAL_SEC", 8)),
			"seedance_max_poll_attempts":       cast.ToInt(config.Env("SEEDANCE_MAX_POLL_ATTEMPTS", 60)),
			"seedance_http_timeout_sec":        cast.ToInt(config.Env("SEEDANCE_HTTP_TIMEOUT_SEC", 30)),
			"seedance_enabled":                 cast.ToBool(config.Env("SEEDANCE_ENABLED", true)),
			"bgm_enabled":                      cast.ToBool(config.Env("BGM_ENABLED", false)),
			"worker_assets_dir":                config.Env("WORKER_ASSETS_DIR", "/app/assets"),
			"ffmpeg_work_dir":                  config.Env("FFMPEG_WORK_DIR", "/app/outputs"),
			"ffmpeg_timeout_sec":               cast.ToInt(config.Env("FFMPEG_TIMEOUT_SEC", 300)),
			"podcast_ffmpeg_timeout_sec":       cast.ToInt(config.Env("PODCAST_FFMPEG_TIMEOUT_SEC", 0)),
			"podcast_mode":                     podcastMode,
			"podcast_x264_preset":              podcastX264Preset,
			"podcast_keep_ass":                 podcastKeepASS,
			"google_tts_enabled":               cast.ToBool(config.Env("GOOGLE_TTS_ENABLED", true)),
			"google_cloud_project_id":          config.Env("GOOGLE_CLOUD_PROJECT_ID", ""),
			"google_user_project":              config.Env("GOOGLE_USER_PROJECT", ""),
			"google_access_token":              config.Env("GOOGLE_ACCESS_TOKEN", ""),
			"google_service_account_json_path": config.Env("GOOGLE_SERVICE_ACCOUNT_JSON_PATH", config.Env("GOOGLE_APPLICATION_CREDENTIALS", "")),
			"google_service_account_json":      config.Env("GOOGLE_SERVICE_ACCOUNT_JSON", ""),
			"google_oauth_token_url":           config.Env("GOOGLE_OAUTH_TOKEN_URL", "https://oauth2.googleapis.com/token"),
			"google_tts_url":                   config.Env("GOOGLE_TTS_URL", "https://texttospeech.googleapis.com/v1/text:synthesize"),
			"google_tts_model":                 config.Env("GOOGLE_TTS_MODEL", "gemini-2.5-pro-tts"),
			"google_tts_audio_encoding":        config.Env("GOOGLE_TTS_AUDIO_ENCODING", "MP3"),
			"google_tts_sample_rate_hz":        cast.ToInt(config.Env("GOOGLE_TTS_SAMPLE_RATE_HZ", 24000)),
			"google_tts_speaking_rate":         cast.ToFloat64(config.Env("GOOGLE_TTS_SPEAKING_RATE", 1.0)),
			"google_tts_ja_speaking_rate":      cast.ToFloat64(config.Env("GOOGLE_TTS_JA_SPEAKING_RATE", 0.85)),
			"google_tts_zh_male_voice_id":      config.Env("GOOGLE_TTS_ZH_MALE_VOICE_ID", ""),
			"google_tts_zh_female_voice_id":    config.Env("GOOGLE_TTS_ZH_FEMALE_VOICE_ID", ""),
			"google_tts_ja_male_voice_id":      config.Env("GOOGLE_TTS_JA_MALE_VOICE_ID", ""),
			"google_tts_ja_female_voice_id":    config.Env("GOOGLE_TTS_JA_FEMALE_VOICE_ID", ""),
			"google_tts_prompt_append":         config.Env("GOOGLE_TTS_PROMPT_APPEND", ""),
			"google_tts_zh_prompt_append":      config.Env("GOOGLE_TTS_ZH_PROMPT_APPEND", ""),
			"google_tts_ja_prompt_append":      config.Env("GOOGLE_TTS_JA_PROMPT_APPEND", ""),
			"google_tts_prompt_max_bytes":      cast.ToInt(config.Env("GOOGLE_TTS_PROMPT_MAX_BYTES", 1200)),
			"elevenlabs_tts_enabled":           cast.ToBool(config.Env("ELEVENLABS_TTS_ENABLED", false)),
			"elevenlabs_api_key":               config.Env("ELEVENLABS_API_KEY", ""),
			"elevenlabs_base_url":              config.Env("ELEVENLABS_BASE_URL", "https://api.elevenlabs.io"),
			"elevenlabs_dialogue_path":         config.Env("ELEVENLABS_DIALOGUE_PATH", "/v1/text-to-dialogue/with-timestamps"),
			"elevenlabs_tts_model":             config.Env("ELEVENLABS_TTS_MODEL", "eleven_v3"),
			"elevenlabs_output_format":         config.Env("ELEVENLABS_OUTPUT_FORMAT", "mp3_44100_128"),
			"elevenlabs_tts_speed":             cast.ToFloat64(config.Env("ELEVENLABS_TTS_SPEED", 1.0)),
			"elevenlabs_tts_prompt_append":     config.Env("ELEVENLABS_TTS_PROMPT_APPEND", ""),
			"elevenlabs_tts_zh_prompt_append":  config.Env("ELEVENLABS_TTS_ZH_PROMPT_APPEND", ""),
			"elevenlabs_tts_ja_prompt_append":  config.Env("ELEVENLABS_TTS_JA_PROMPT_APPEND", ""),
			"elevenlabs_tts_prompt_max_bytes":  cast.ToInt(config.Env("ELEVENLABS_TTS_PROMPT_MAX_BYTES", 1200)),
			"elevenlabs_tts_male_voice_id":     config.Env("ELEVENLABS_TTS_MALE_VOICE_ID", ""),
			"elevenlabs_tts_female_voice_id":   config.Env("ELEVENLABS_TTS_FEMALE_VOICE_ID", ""),
			"mfa_enabled":                      cast.ToBool(config.Env("MFA_ENABLED", false)),
			"mfa_command":                      config.Env("MFA_COMMAND", "mfa"),
			"mfa_temporary_directory":          config.Env("MFA_TEMPORARY_DIRECTORY", ""),
			"mfa_beam":                         cast.ToInt(config.Env("MFA_BEAM", 10)),
			"mfa_retry_beam":                   cast.ToInt(config.Env("MFA_RETRY_BEAM", 40)),
			"mfa_zh_dictionary":                config.Env("MFA_ZH_DICTIONARY", ""),
			"mfa_zh_acoustic_model":            config.Env("MFA_ZH_ACOUSTIC_MODEL", ""),
			"mfa_zh_g2p_model":                 config.Env("MFA_ZH_G2P_MODEL", "mandarin_china_mfa"),
			"mfa_ja_dictionary":                config.Env("MFA_JA_DICTIONARY", ""),
			"mfa_ja_acoustic_model":            config.Env("MFA_JA_ACOUSTIC_MODEL", ""),
			"mfa_ja_g2p_model":                 config.Env("MFA_JA_G2P_MODEL", "japanese_mfa"),
			"podcast_block_gap_ms":             cast.ToInt(config.Env("PODCAST_BLOCK_GAP_MS", 280)),
			"podcast_template_gap_ms":          cast.ToInt(config.Env("PODCAST_TEMPLATE_GAP_MS", 280)),
			"s3_enabled":                       cast.ToBool(config.Env("S3_ENABLED", false)),
			"s3_endpoint":                      config.Env("S3_ENDPOINT", ""),
			"s3_region":                        config.Env("S3_REGION", "us-east-1"),
			"s3_bucket":                        config.Env("S3_BUCKET", ""),
			"s3_access_key":                    config.Env("S3_ACCESS_KEY", ""),
			"s3_secret_key":                    config.Env("S3_SECRET_KEY", ""),
			"s3_public_url":                    config.Env("S3_PUBLIC_URL", ""),
		}
	})
}
