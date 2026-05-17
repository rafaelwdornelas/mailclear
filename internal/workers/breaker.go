package workers

import (
	"time"

	"github.com/sony/gobreaker"
)

// NewBreaker cria um circuit breaker simples com defaults sensatos.
// Trip após 5 falhas consecutivas; reset após 30s.
func NewBreaker(name string) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 1,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.ConsecutiveFailures >= 5
		},
	})
}
