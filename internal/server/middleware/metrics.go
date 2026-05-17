package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chiMW "github.com/go-chi/chi/v5/middleware"

	"github.com/rafaelwdornelas/mailclear/internal/metrics"
)

// Metrics instrumenta cada request HTTP com counter + histogram.
func Metrics(m *metrics.HTTPMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.InFlight.Inc()
			defer m.InFlight.Dec()

			ww := chiMW.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)

			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = r.URL.Path
			}
			m.RequestsTotal.WithLabelValues(r.Method, route, statusBucket(ww.Status())).Inc()
			m.RequestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
		})
	}
}

func statusBucket(s int) string {
	switch {
	case s < 200:
		return "1xx"
	case s < 300:
		return "2xx"
	case s < 400:
		return "3xx"
	case s < 500:
		return "4xx"
	default:
		return "5xx"
	}
}
