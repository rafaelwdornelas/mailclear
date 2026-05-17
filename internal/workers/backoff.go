package workers

import (
	"math/rand"
	"time"
)

// Backoff calcula um intervalo exponencial com jitter.
//   attempt=0  -> base*0  + jitter
//   attempt=1  -> base*1  + jitter
//   attempt=2  -> base*2  + jitter
//   ...
// Capeado em maxDelay.
func Backoff(attempt int, base, maxDelay time.Duration) time.Duration {
	if attempt <= 0 {
		return 0
	}
	d := base * time.Duration(1<<uint(attempt-1)) // nolint:gosec — base small
	if d > maxDelay {
		d = maxDelay
	}
	// jitter ±25%
	j := float64(d) * 0.25
	delta := time.Duration((rand.Float64()*2 - 1) * j) // nolint:gosec
	return d + delta
}
