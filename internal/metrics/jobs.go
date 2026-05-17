package metrics

import "github.com/prometheus/client_golang/prometheus"

// JobsMetrics agrega métricas de jobs.
type JobsMetrics struct {
	Total         prometheus.Counter
	InFlight      prometheus.Gauge
	Duration      *prometheus.HistogramVec
	EmailsTotal   *prometheus.CounterVec
	PhaseDuration *prometheus.HistogramVec // tempo por fase do batch
	PrewarmSkips  prometheus.Counter       // domínios pulados (já em cache)
}

func newJobsMetrics(reg prometheus.Registerer) *JobsMetrics {
	m := &JobsMetrics{
		Total:    prometheus.NewCounter(prometheus.CounterOpts{Name: "jobs_total", Help: "Total de jobs criados."}),
		InFlight: prometheus.NewGauge(prometheus.GaugeOpts{Name: "jobs_in_flight", Help: "Jobs em execução."}),
		Duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "jobs_duration_seconds",
				Help:    "Tempo total de execução do job.",
				Buckets: []float64{1, 5, 10, 30, 60, 300, 900, 1800, 3600},
			},
			[]string{"status"},
		),
		EmailsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "jobs_emails_processed_total", Help: "Emails processados, por status final."},
			[]string{"status"},
		),
		PhaseDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "job_batch_phase_duration_seconds",
				Help:    "Tempo gasto em cada fase do processamento de 1 batch (pre_warm, validate, insert).",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"phase"},
		),
		PrewarmSkips: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "job_prewarm_cache_skips_total",
			Help: "Domínios cujo pre-warm foi pulado pois MX já estava em cache (L1/L2).",
		}),
	}
	reg.MustRegister(m.Total, m.InFlight, m.Duration, m.EmailsTotal, m.PhaseDuration, m.PrewarmSkips)
	return m
}
