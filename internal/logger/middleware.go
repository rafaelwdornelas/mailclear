package logger

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

// HTTPMiddleware loga cada requisição com método, path, status, duração e bytes.
func HTTPMiddleware(base zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Injeta logger no contexto, com request_id se disponível
			reqID := middleware.GetReqID(r.Context())
			logger := base.With().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote", r.RemoteAddr).
				Str("request_id", reqID).
				Logger()
			ctx := logger.WithContext(r.Context())

			next.ServeHTTP(ww, r.WithContext(ctx))

			logger.Info().
				Int("status", ww.Status()).
				Int("bytes", ww.BytesWritten()).
				Dur("dur", time.Since(start)).
				Msg("http")
		})
	}
}
