package bootstrap

import (
	btsConfig "worker/config"
	"worker/pkg/config"
	"worker/pkg/logger"
)

func Initialize(env string) error {
	btsConfig.Initialize()
	config.InitConfig(env)
	return logger.InitLogger(config.Get[string]("log.filename"))
}
