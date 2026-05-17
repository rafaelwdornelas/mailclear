package metrics

import "github.com/prometheus/client_golang/prometheus"

// ValidationMetrics agrega métricas do pipeline de validação.
type ValidationMetrics struct {
	StageDuration *prometheus.HistogramVec
	StageResult   *prometheus.CounterVec
	ShortCircuit  *prometheus.CounterVec
	ResultStatus  *prometheus.CounterVec
}

func newValidationMetrics(reg prometheus.Registerer) *ValidationMetrics {
	m := &ValidationMetrics{
		StageDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "validation_stage_duration_seconds",
				Help:    "Latência por stage do pipeline em segundos.",
				Buckets: []float64{0.00001, 0.0001, 0.001, 0.01, 0.1, 1},
			},
			[]string{"stage"},
		),
		StageResult: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "validation_stage_result_total", Help: "Quantidade de execuções por stage e resultado (ok|fail)."},
			[]string{"stage", "result"},
		),
		ShortCircuit: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "validation_short_circuit_total", Help: "Quantidade de short-circuits por stage."},
			[]string{"stage"},
		),
		ResultStatus: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "validation_result_status_total", Help: "Classificação final dos emails validados."},
			[]string{"status"},
		),
	}
	reg.MustRegister(m.StageDuration, m.StageResult, m.ShortCircuit, m.ResultStatus)
	return m
}
