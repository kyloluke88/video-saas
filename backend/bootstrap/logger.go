package bootstrap

import (
	"api/pkg/config"
	"api/pkg/logger"
)

// SetupLogger 初始化 Logger
func SetupLogger() {

	logger.InitLogger(
		config.Get[string]("log.filename"),
		config.Get[int]("log.max_size"),
		config.Get[int]("log.max_backup"),
		config.Get[int]("log.max_age"),
		config.Get[bool]("log.compress"),
		config.Get[string]("log.type"),
		config.Get[string]("log.level"),
	)
}
