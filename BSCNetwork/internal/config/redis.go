package config

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

func RedisConnect(ctx context.Context, cfg Config) (redis.Client, error) {
	rds := redis.NewClient(
		&redis.Options{
			Addr: fmt.Sprintf("%s:%d",
				cfg.Redis.Host,
				cfg.Redis.Port,
			),
			Password:    cfg.Redis.Password,
			DB:          cfg.Redis.DB,
			DialTimeout: time.Duration(cfg.Redis.Timeout) * time.Second,
			PoolSize:    cfg.Redis.PoolSize,
		})

	_, err := rds.Ping(ctx).Result()

	return *rds, err
}
