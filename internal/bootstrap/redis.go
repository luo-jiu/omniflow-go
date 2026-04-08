package bootstrap

import (
	"omniflow-go/internal/config"

	"github.com/redis/go-redis/v9"
)

func NewRedis(cfg *config.Config) (*redis.Client, func(), error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	cleanup := func() {
		_ = client.Close()
	}

	return client, cleanup, nil
}
