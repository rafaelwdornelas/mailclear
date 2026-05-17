package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis é um cache L2 sobre Redis, com serialização JSON e prefixo de chave.
type Redis[V any] struct {
	cli    *redis.Client
	prefix string
}

// NewRedis cria o cache. prefix é prepended a toda chave (ex: "mailclear:").
func NewRedis[V any](cli *redis.Client, prefix string) *Redis[V] {
	return &Redis[V]{cli: cli, prefix: prefix}
}

// Get recupera e desserializa o valor.
func (r *Redis[V]) Get(ctx context.Context, key string) (V, error) {
	var zero V
	b, err := r.cli.Get(ctx, r.prefix+key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return zero, ErrMiss
		}
		return zero, err
	}
	var v V
	if err := json.Unmarshal(b, &v); err != nil {
		return zero, err
	}
	return v, nil
}

// Set serializa e armazena com TTL.
func (r *Redis[V]) Set(ctx context.Context, key string, val V, ttl time.Duration) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return r.cli.Set(ctx, r.prefix+key, b, ttl).Err()
}

// Delete remove a chave.
func (r *Redis[V]) Delete(ctx context.Context, key string) error {
	return r.cli.Del(ctx, r.prefix+key).Err()
}
