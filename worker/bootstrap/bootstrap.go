package bootstrap

import (
	btsConfig "worker/config"
	"worker/pkg/config"
	"worker/pkg/logger"
)

func Initialize(env string) error {
	btsConfig.Initialize()
	config.InitConfig(env)
	if err := logger.InitLogger(config.Get[string]("log.filename")); err != nil {
		return err
	}
	if err := SetupDB(); err != nil {
		return err
	}
	if err := SetupRedis(); err != nil {
		return err
	}
	return nil
}
