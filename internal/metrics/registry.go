// Package metrics expõe todos os collectors Prometheus da aplicação.
// Todos os collectors são registrados em um *prometheus.Registry próprio,
// não no DefaultRegisterer — facilita testes e isola da stdlib.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Metrics agrupa todos os collectors usados pela aplicação.
type Metrics struct {
	Reg *prometheus.Registry

	HTTP       *HTTPMetrics
	DNS        *DNSMetrics
	Validation *ValidationMetrics
	Workers    *WorkersMetrics
	Jobs       *JobsMetrics
}

// New cria um Registry e registra todos os collectors.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	m := &Metrics{
		Reg:        reg,
		HTTP:       newHTTPMetrics(reg),
		DNS:        newDNSMetrics(reg),
		Validation: newValidationMetrics(reg),
		Workers:    newWorkersMetrics(reg),
		Jobs:       newJobsMetrics(reg),
	}
	return m
}
