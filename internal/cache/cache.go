// Package cache define a interface genérica Cache[K,V] e implementações L1/L2.
package cache

import (
	"context"
	"errors"
	"time"
)

// ErrMiss é devolvido quando a chave não existe.
var ErrMiss = errors.New("cache miss")

// Cache é a interface mínima para um cache key-value tipado.
type Cache[K comparable, V any] interface {
	Get(ctx context.Context, key K) (V, error)
	Set(ctx context.Context, key K, val V, ttl time.Duration) error
	Delete(ctx context.Context, key K) error
}
