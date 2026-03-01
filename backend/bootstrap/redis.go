package bootstrap

import (
	"api/pkg/config"
	"api/pkg/redis"
	"fmt"
)

// SetupRedis 初始化 Redis
func SetupRedis() {

	// 建立 Redis 连接
	redis.ConnectRedis(
		fmt.Sprintf("%v:%v", config.Get[string]("redis.host"), config.Get[string]("redis.port")),
		config.Get[string]("redis.username"),
		config.Get[string]("redis.password"),
		config.Get[int]("redis.database"),
	)
}
