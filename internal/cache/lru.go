package cache

import (
	"context"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// LRU é um cache em memória com expiração por entrada e tamanho fixo.
// Suporta tipos genéricos via parâmetros K,V.
type LRU[K comparable, V any] struct {
	c   *lru.Cache[K, lruEntry[V]]
	mu  sync.RWMutex
	now func() time.Time
}

type lruEntry[V any] struct {
	val       V
	expiresAt time.Time // zero = nunca expira
}

// NewLRU cria um cache LRU com capacidade fixa.
func NewLRU[K comparable, V any](size int) (*LRU[K, V], error) {
	c, err := lru.New[K, lruEntry[V]](size)
	if err != nil {
		return nil, err
	}
	return &LRU[K, V]{c: c, now: time.Now}, nil
}

// Get devolve o valor associado à key. ErrMiss se ausente ou expirado.
func (l *LRU[K, V]) Get(_ context.Context, key K) (V, error) {
	l.mu.RLock()
	e, ok := l.c.Get(key)
	l.mu.RUnlock()

	var zero V
	if !ok {
		return zero, ErrMiss
	}
	if !e.expiresAt.IsZero() && l.now().After(e.expiresAt) {
		l.mu.Lock()
		l.c.Remove(key)
		l.mu.Unlock()
		return zero, ErrMiss
	}
	return e.val, nil
}

// Set adiciona ou atualiza a entrada.
func (l *LRU[K, V]) Set(_ context.Context, key K, val V, ttl time.Duration) error {
	var exp time.Time
	if ttl > 0 {
		exp = l.now().Add(ttl)
	}
	l.mu.Lock()
	l.c.Add(key, lruEntry[V]{val: val, expiresAt: exp})
	l.mu.Unlock()
	return nil
}

// Delete remove a chave.
func (l *LRU[K, V]) Delete(_ context.Context, key K) error {
	l.mu.Lock()
	l.c.Remove(key)
	l.mu.Unlock()
	return nil
}

// Len devolve quantidade atual de entradas.
func (l *LRU[K, V]) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.c.Len()
}

// Purge remove todas as entradas do cache.
func (l *LRU[K, V]) Purge() {
	l.mu.Lock()
	l.c.Purge()
	l.mu.Unlock()
}
