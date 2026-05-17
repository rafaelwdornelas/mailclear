package cache

import (
	"context"
	"errors"
	"time"
)

// Multi compõe L1 (in-memory) e L2 (Redis) em cascata.
//   Get: tenta L1, depois L2 (e re-popula L1).
//   Set: grava em ambos.
//   Delete: remove de ambos.
type Multi[V any] struct {
	L1 *LRU[string, V]
	L2 *Redis[V]
}

// NewMulti cria a cascata com L1 e L2 já configurados.
func NewMulti[V any](l1 *LRU[string, V], l2 *Redis[V]) *Multi[V] {
	return &Multi[V]{L1: l1, L2: l2}
}

// Get tenta L1, depois L2 (re-populando L1 em hit de L2).
func (m *Multi[V]) Get(ctx context.Context, key string) (V, error) {
	if m.L1 != nil {
		if v, err := m.L1.Get(ctx, key); err == nil {
			return v, nil
		}
	}
	if m.L2 != nil {
		if v, err := m.L2.Get(ctx, key); err == nil {
			if m.L1 != nil {
				_ = m.L1.Set(ctx, key, v, 0)
			}
			return v, nil
		} else if !errors.Is(err, ErrMiss) {
			// L2 com erro real (timeout etc): degrada para miss
			var zero V
			return zero, ErrMiss
		}
	}
	var zero V
	return zero, ErrMiss
}

// Set grava em ambas as camadas. ttl é usado em L2; L1 usa ttl também (clamped).
func (m *Multi[V]) Set(ctx context.Context, key string, val V, ttl time.Duration) error {
	if m.L1 != nil {
		_ = m.L1.Set(ctx, key, val, ttl)
	}
	if m.L2 != nil {
		return m.L2.Set(ctx, key, val, ttl)
	}
	return nil
}

// Delete remove de ambas as camadas.
func (m *Multi[V]) Delete(ctx context.Context, key string) error {
	if m.L1 != nil {
		_ = m.L1.Delete(ctx, key)
	}
	if m.L2 != nil {
		return m.L2.Delete(ctx, key)
	}
	return nil
}

// PurgeL1 limpa apenas o cache em memória.
// Útil quando se quer forçar releitura sem afetar L2 (Redis compartilhado).
func (m *Multi[V]) PurgeL1() {
	if m.L1 != nil {
		m.L1.Purge()
	}
}
