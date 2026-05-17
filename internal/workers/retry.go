package workers

import (
	"context"
	"time"
)

// Retry executa fn com backoff exponencial até maxAttempts ou contexto cancelado.
// Devolve o último erro se todas as tentativas falharem.
func Retry(ctx context.Context, maxAttempts int, base, maxDelay time.Duration, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(Backoff(attempt, base, maxDelay)):
			}
		}
		if err := fn(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}
