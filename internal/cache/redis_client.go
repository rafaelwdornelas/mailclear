package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/rafaelwdornelas/mailclear/internal/config"
)

// NewRedisClient cria o *redis.Client a partir da config e valida com Ping.
func NewRedisClient(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	opt, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	if cfg.PoolSize > 0 {
		opt.PoolSize = cfg.PoolSize
	}
	cli := redis.NewClient(opt)
	if err := cli.Ping(ctx).Err(); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return cli, nil
}
