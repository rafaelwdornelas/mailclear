// Package logger encapsula o setup do zerolog com integração ao zerolog/console
// e ao formato JSON consumido pelo journalctl.
package logger

import (
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// New cria um logger configurado conforme level e format.
//   level:  trace|debug|info|warn|error|fatal|panic
//   format: json | console
func New(level, format string) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	var w io.Writer = os.Stdout
	if strings.EqualFold(format, "console") {
		w = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	}
	return zerolog.New(w).With().Timestamp().Logger()
}

// FromContext retorna o logger embutido no contexto, ou um default.
func FromContext(ctx context.Context) *zerolog.Logger {
	l := zerolog.Ctx(ctx)
	if l == nil || l.GetLevel() == zerolog.Disabled {
		base := zerolog.New(os.Stdout).With().Timestamp().Logger()
		return &base
	}
	return l
}

// WithJobID retorna um contexto com job_id embutido nos logs.
func WithJobID(ctx context.Context, jobID string) context.Context {
	return zerolog.Ctx(ctx).With().Str("job_id", jobID).Logger().WithContext(ctx)
}

// WithRequestID retorna um contexto com request_id embutido nos logs.
func WithRequestID(ctx context.Context, reqID string) context.Context {
	return zerolog.Ctx(ctx).With().Str("request_id", reqID).Logger().WithContext(ctx)
}
