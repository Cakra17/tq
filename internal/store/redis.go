package store

import (
	"github.com/cakra17/tq/internal/config"
	"github.com/redis/go-redis/v9"
)

func NewRedisClient(cfg config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: cfg.Host,
		Password: cfg.Password,
		DB: 0,
	})
}