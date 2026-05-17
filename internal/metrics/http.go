package metrics

import "github.com/prometheus/client_golang/prometheus"

// HTTPMetrics agrega métricas das rotas HTTP.
type HTTPMetrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	InFlight        prometheus.Gauge
}

func newHTTPMetrics(reg prometheus.Registerer) *HTTPMetrics {
	m := &HTTPMetrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "http_requests_total", Help: "Total de requisições HTTP."},
			[]string{"method", "route", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Latência de requisições HTTP em segundos.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "route"},
		),
		InFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{Name: "http_in_flight_requests", Help: "Requisições HTTP em andamento."},
		),
	}
	reg.MustRegister(m.RequestsTotal, m.RequestDuration, m.InFlight)
	return m
}
