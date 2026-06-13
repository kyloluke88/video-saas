package bootstrap

import (
	btsConfig "worker/config"
	"worker/pkg/config"
	"worker/pkg/logger"
)

type InitOptions struct {
	WithDB    bool
	WithRedis bool
}

func Initialize(env string) error {
	return InitializeWithOptions(env, InitOptions{
		WithDB:    true,
		WithRedis: true,
	})
}

func InitializeWithOptions(env string, opts InitOptions) error {
	btsConfig.Initialize()
	config.InitConfig(env)
	if err := logger.InitLogger(config.Get[string]("log.filename")); err != nil {
		return err
	}
	if opts.WithDB {
		if err := SetupDB(); err != nil {
			return err
		}
	}
	if opts.WithRedis {
		if err := SetupRedis(); err != nil {
			return err
		}
	}
	return nil
}
