package dns

import (
	"context"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

// DomainLimiter implementa um rate-limit token-bucket por domínio.
// Cada domínio recebe seu próprio rate.Limiter sob demanda.
// Útil para não estourar autoritativos como gmail.com / yahoo.com.
type DomainLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
	rps      rate.Limit // queries/segundo por domínio
	burst    int
}

// NewDomainLimiter cria um limiter com rps queries/segundo e burst configurável.
// rps <= 0 desabilita o rate-limit.
func NewDomainLimiter(rps, burst int) *DomainLimiter {
	if burst <= 0 {
		burst = rps
	}
	return &DomainLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
}

// Wait bloqueia até que um token esteja disponível para o domínio ou
// o contexto expire. Devolve ErrRateLimited se o contexto for cancelado.
func (d *DomainLimiter) Wait(ctx context.Context, domain string) error {
	if d == nil || d.rps <= 0 {
		return nil
	}
	domain = strings.ToLower(domain)

	d.mu.RLock()
	lim, ok := d.limiters[domain]
	d.mu.RUnlock()

	if !ok {
		d.mu.Lock()
		// Re-check após upgrade do lock
		if lim, ok = d.limiters[domain]; !ok {
			lim = rate.NewLimiter(d.rps, d.burst)
			d.limiters[domain] = lim
		}
		d.mu.Unlock()
	}

	if err := lim.Wait(ctx); err != nil {
		return ErrRateLimited
	}
	return nil
}
