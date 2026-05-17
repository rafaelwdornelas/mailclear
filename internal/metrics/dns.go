package metrics

import "github.com/prometheus/client_golang/prometheus"

// DNSMetrics agrega métricas do DNS resolver.
type DNSMetrics struct {
	QueriesTotal   *prometheus.CounterVec
	QueryDuration  *prometheus.HistogramVec
	CacheHitsTotal *prometheus.CounterVec
	Singleflight   *prometheus.CounterVec
	RateLimited    *prometheus.CounterVec

	// Controle adaptativo (AIMD)
	InflightLimit    prometheus.Gauge // limite atual de queries simultâneas
	InflightCurrent  prometheus.Gauge // queries DNS em vôo agora
	LastErrorRate    prometheus.Gauge // % de erro (timeout+servfail) no último tick
}

func newDNSMetrics(reg prometheus.Registerer) *DNSMetrics {
	m := &DNSMetrics{
		QueriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "dns_query_total", Help: "Total de queries DNS realizadas."},
			[]string{"type", "result"},
		),
		QueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "dns_query_duration_seconds",
				Help:    "Latência de queries DNS em segundos.",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
			},
			[]string{"type"},
		),
		CacheHitsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "dns_cache_hits_total", Help: "Cache hits por camada (l1=LRU, l2=Redis)."},
			[]string{"layer", "type"},
		),
		Singleflight: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "dns_singleflight_total", Help: "Queries deduplicadas por singleflight."},
			[]string{"type"},
		),
		RateLimited: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "dns_rate_limited_total", Help: "Queries bloqueadas pelo rate limiter de domínio."},
			[]string{"domain"},
		),
		InflightLimit: prometheus.NewGauge(
			prometheus.GaugeOpts{Name: "dns_inflight_limit", Help: "Limite atual de queries DNS simultâneas (ajustado por AIMD)."},
		),
		InflightCurrent: prometheus.NewGauge(
			prometheus.GaugeOpts{Name: "dns_inflight_current", Help: "Queries DNS em vôo neste instante."},
		),
		LastErrorRate: prometheus.NewGauge(
			prometheus.GaugeOpts{Name: "dns_last_error_rate", Help: "Taxa de erro (timeout+servfail) do último tick AIMD (0-1)."},
		),
	}
	reg.MustRegister(m.QueriesTotal, m.QueryDuration, m.CacheHitsTotal, m.Singleflight, m.RateLimited,
		m.InflightLimit, m.InflightCurrent, m.LastErrorRate)
	return m
}

// SetInflightLimit implementa dns.MetricsHook.
func (m *DNSMetrics) SetInflightLimit(v float64) { m.InflightLimit.Set(v) }

// SetInflightCurrent implementa dns.MetricsHook.
func (m *DNSMetrics) SetInflightCurrent(v float64) { m.InflightCurrent.Set(v) }

// SetLastErrorRate implementa dns.MetricsHook.
func (m *DNSMetrics) SetLastErrorRate(v float64) { m.LastErrorRate.Set(v) }
