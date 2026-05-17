package metrics

import "github.com/prometheus/client_golang/prometheus"

// WorkersMetrics agrega métricas do worker pool.
type WorkersMetrics struct {
	Active     prometheus.Gauge
	QueueDepth prometheus.Gauge
	Processed  *prometheus.CounterVec
	Retries    prometheus.Counter
}

func newWorkersMetrics(reg prometheus.Registerer) *WorkersMetrics {
	m := &WorkersMetrics{
		Active:     prometheus.NewGauge(prometheus.GaugeOpts{Name: "workers_active", Help: "Workers ativos."}),
		QueueDepth: prometheus.NewGauge(prometheus.GaugeOpts{Name: "workers_queue_depth", Help: "Itens aguardando processamento na fila."}),
		Processed: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "workers_processed_total", Help: "Tarefas processadas, por tipo e resultado."},
			[]string{"task", "result"},
		),
		Retries: prometheus.NewCounter(prometheus.CounterOpts{Name: "workers_retries_total", Help: "Total de retries disparados."}),
	}
	reg.MustRegister(m.Active, m.QueueDepth, m.Processed, m.Retries)
	return m
}
