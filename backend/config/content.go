package config

import "api/pkg/config"

func init() {
	config.Add("content", func() map[string]interface{} {
		return map[string]interface{}{
			"projects_dir": config.Env("CONTENT_PROJECTS_DIR", "../worker/outputs/projects"),
		}
	})
}
