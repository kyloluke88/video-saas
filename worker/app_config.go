package main

import conf "worker/pkg/config"

func loadConfig() Config {
	return Config{
		URL: conf.Get[string]("worker.rabbitmq_url"),

		Host:     conf.Get[string]("worker.rabbitmq_host"),
		Port:     conf.Get[string]("worker.rabbitmq_port"),
		Username: conf.Get[string]("worker.rabbitmq_user"),
		Password: conf.Get[string]("worker.rabbitmq_password"),
		VHost:    conf.Get[string]("worker.rabbitmq_vhost"),

		Exchange:        conf.Get[string]("worker.rabbitmq_exchange"),
		ExchangeType:    conf.Get[string]("worker.rabbitmq_exchange_type"),
		Queue:           conf.Get[string]("worker.rabbitmq_queue"),
		RoutingKey:      conf.Get[string]("worker.rabbitmq_routing_key"),
		RetryQueue:      conf.Get[string]("worker.rabbitmq_retry_queue"),
		RetryRoutingKey: conf.Get[string]("worker.rabbitmq_retry_routing_key"),
		DLX:             conf.Get[string]("worker.rabbitmq_dlx"),
		DLQ:             conf.Get[string]("worker.rabbitmq_dlq"),
		DLQRoutingKey:   conf.Get[string]("worker.rabbitmq_dlq_routing_key"),
		RetryDelayMs:    conf.Get[int]("worker.rabbitmq_retry_delay_ms"),
		Prefetch:        conf.Get[int]("worker.rabbitmq_prefetch"),
		MaxRetries:      conf.Get[int]("worker.task_max_retries"),

		SeedanceBaseURL:         conf.Get[string]("worker.seedance_base_url"),
		SeedanceGeneratePath:    conf.Get[string]("worker.seedance_generate_path"),
		SeedanceStatusPath:      conf.Get[string]("worker.seedance_status_path"),
		SeedanceAPIKey:          conf.Get[string]("worker.seedance_api_key"),
		SeedancePollIntervalSec: conf.Get[int]("worker.seedance_poll_interval_sec"),
		SeedanceMaxPollAttempts: conf.Get[int]("worker.seedance_max_poll_attempts"),
		SeedanceHTTPTimeoutSec:  conf.Get[int]("worker.seedance_http_timeout_sec"),
		SeedanceDryRunEnable:    conf.Get[bool]("worker.seedance_dry_run_enable"),

		FFmpegPostprocessEnabled: conf.Get[bool]("worker.ffmpeg_postprocess_enabled"),
		BGMEnable:                conf.Get[bool]("worker.bgm_enable"),
		FFmpegWorkDir:            conf.Get[string]("worker.ffmpeg_work_dir"),
		FFmpegTimeoutSec:         conf.Get[int]("worker.ffmpeg_timeout_sec"),

		TTSAPIURL:   conf.Get[string]("worker.tts_api_url"),
		TTSAPIKey:   conf.Get[string]("worker.tts_api_key"),
		TTSProvider: conf.Get[string]("worker.tts_provider"),

		TTSTencentRegion:          conf.Get[string]("worker.tts_tencent_region"),
		TTSTencentSecretID:        conf.Get[string]("worker.tts_tencent_secret_id"),
		TTSTencentSecretKey:       conf.Get[string]("worker.tts_tencent_secret_key"),
		TTSTencentVoiceType:       conf.Get[int64]("worker.tts_tencent_voice_type"),
		TTSTencentPrimaryLanguage: conf.Get[int64]("worker.tts_tencent_primary_language"),
		TTSTencentModelType:       conf.Get[int64]("worker.tts_tencent_model_type"),
		TTSTencentCodec:           conf.Get[string]("worker.tts_tencent_codec"),

		S3Enabled:   conf.Get[bool]("worker.s3_enabled"),
		S3Endpoint:  conf.Get[string]("worker.s3_endpoint"),
		S3Region:    conf.Get[string]("worker.s3_region"),
		S3Bucket:    conf.Get[string]("worker.s3_bucket"),
		S3AccessKey: conf.Get[string]("worker.s3_access_key"),
		S3SecretKey: conf.Get[string]("worker.s3_secret_key"),
		S3PublicURL: conf.Get[string]("worker.s3_public_url"),
	}
}
