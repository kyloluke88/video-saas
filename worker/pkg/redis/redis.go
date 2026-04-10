package redis

import (
	"context"
	"sync"

	redis "github.com/redis/go-redis/v9"
)

type RedisClient struct {
	Client  *redis.Client
	Context context.Context
}

var (
	once  sync.Once
	Redis *RedisClient
)

func ConnectRedis(address string, username string, password string, db int) error {
	var connectErr error
	once.Do(func() {
		client := &RedisClient{
			Context: context.Background(),
		}
		client.Client = redis.NewClient(&redis.Options{
			Addr:     address,
			Username: username,
			Password: password,
			DB:       db,
		})
		if _, err := client.Client.Ping(client.Context).Result(); err != nil {
			connectErr = err
			return
		}
		Redis = client
	})
	return connectErr
}

func Ready() bool {
	return Redis != nil && Redis.Client != nil
}
