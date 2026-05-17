package cache

import (
	"math/rand"
	"time"
)

// JitteredTTL adiciona um jitter de ±10% ao TTL base, ajustado para [min, max].
// Evita "stampedes" sincronizados quando vários domínios expiram juntos.
func JitteredTTL(base, min, max time.Duration) time.Duration {
	if base <= 0 {
		base = min
	}
	jitter := float64(base) * 0.1
	delta := time.Duration((rand.Float64()*2 - 1) * jitter) // nolint:gosec — uso não-criptográfico
	d := base + delta
	if d < min {
		d = min
	}
	if max > 0 && d > max {
		d = max
	}
	return d
}
