package config

import "worker/pkg/config"

func init() {
	config.Add("log", func() map[string]interface{} {
		return map[string]interface{}{
			"filename": config.Env("LOG_NAME", "storage/logs/worker.log"),
		}
	})
}
