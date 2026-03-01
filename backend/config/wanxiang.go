package config

import "api/pkg/config"
import "github.com/spf13/cast"

func init() {
	config.Add("wanxiang", func() map[string]interface{} {
		return map[string]interface{}{
			"enabled":            cast.ToBool(config.Env("WANXIANG_ENABLED", false)),
			"base_url":           config.Env("WANXIANG_BASE_URL", "https://dashscope.aliyuncs.com"),
			"api_key":            config.Env("WANXIANG_API_KEY", config.Env("DASHSCOPE_API_KEY", "")),
			"model":              config.Env("WANXIANG_MODEL", "wan2.6-t2i"),
			"create_path":        config.Env("WANXIANG_CREATE_PATH", "/api/v1/services/aigc/multimodal-generation/generation"),
			"task_path_template": config.Env("WANXIANG_TASK_PATH_TEMPLATE", "/api/v1/tasks/%s"),
			"size":               config.Env("WANXIANG_SIZE", "1024*1024"),
			"prompt_extend":      cast.ToBool(config.Env("WANXIANG_PROMPT_EXTEND", true)),
			"num_images":         cast.ToInt(config.Env("WANXIANG_NUM_IMAGES", 1)),
			"http_timeout_sec":   cast.ToInt(config.Env("WANXIANG_HTTP_TIMEOUT_SEC", 120)),
			"poll_interval_sec":  cast.ToInt(config.Env("WANXIANG_POLL_INTERVAL_SEC", 5)),
			"max_poll_attempts":  cast.ToInt(config.Env("WANXIANG_MAX_POLL_ATTEMPTS", 60)),
		}
	})
}
