package bootstrap

import (
	"fmt"

	conf "worker/pkg/config"
	workerredis "worker/pkg/redis"
)

func SetupRedis() error {
	return workerredis.ConnectRedis(
		fmt.Sprintf("%s:%s", conf.Get[string]("worker.redis_host"), conf.Get[string]("worker.redis_port")),
		conf.Get[string]("worker.redis_username"),
		conf.Get[string]("worker.redis_password"),
		conf.Get[int]("worker.redis_database"),
	)
}
