package config

import (
	"api/pkg/config"

	"github.com/spf13/cast"
)

func init() {
	config.Add("deepseek", func() map[string]interface{} {
		return map[string]interface{}{
			"base_url":         config.Env("DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1"),
			"api_key":          config.Env("DEEPSEEK_API_KEY", ""),
			"model":            config.Env("DEEPSEEK_MODEL", "deepseek-chat"),
			"http_timeout_sec": cast.ToInt(config.Env("DEEPSEEK_HTTP_TIMEOUT_SEC", 500)),
		}
	})
}
