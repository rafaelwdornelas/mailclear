// Package shutdown fornece um contexto que cancela em SIGINT/SIGTERM.
package shutdown

import (
	"context"
	"os/signal"
	"syscall"
)

// NotifyContext devolve um contexto cancelado quando o processo recebe
// SIGINT ou SIGTERM. O caller deve chamar stop() para liberar recursos.
func NotifyContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
}
