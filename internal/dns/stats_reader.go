package dns

import (
	"sync/atomic"

	dto "github.com/prometheus/client_model/go"

	"github.com/rafaelwdornelas/mailclear/internal/metrics"
)

// MakeRateReader devolve uma função que, a cada chamada, retorna o delta
// (total, erros) desde a última chamada — formato exigido pelo AIMDController.
//
// "Erros" = APENAS timeout. NÃO contamos servfail, nxdomain, nem error
// genérico aqui, porque o AIMD precisa saber se SATURAMOS A CAPACIDADE LOCAL
// (Unbound + rede do servidor), não se domínios upstream estão mal configurados.
//
//   - timeout       → nossa query saiu mas resposta não voltou → saturação real
//   - servfail      → autoritativo do domínio respondeu erro → problema dele
//   - nxdomain      → resposta correta (domínio não existe)
//   - error genérico → bugs/parse fail, geralmente raros
//
// Numa lista real de milhões de emails, 5-10% naturalmente retornam servfail
// por autoritativos quebrados (small biz, listas antigas .com.br). Contar isso
// como "saturação" faz o AIMD estrangular o sistema sem necessidade — vivia
// oscilando entre 16 e 48 inflight em testes reais.
func MakeRateReader(m *metrics.DNSMetrics) func() (uint64, uint64) {
	var lastTotal atomic.Uint64
	var lastErrors atomic.Uint64

	return func() (uint64, uint64) {
		total := readCounter(m.QueriesTotal.WithLabelValues("MX", "ok")) +
			readCounter(m.QueriesTotal.WithLabelValues("MX", "nxdomain")) +
			readCounter(m.QueriesTotal.WithLabelValues("MX", "timeout")) +
			readCounter(m.QueriesTotal.WithLabelValues("MX", "servfail")) +
			readCounter(m.QueriesTotal.WithLabelValues("MX", "error"))

		errs := readCounter(m.QueriesTotal.WithLabelValues("MX", "timeout"))

		prevTotal := lastTotal.Swap(uint64(total))
		prevErrs := lastErrors.Swap(uint64(errs))

		var deltaTotal, deltaErrs uint64
		if uint64(total) >= prevTotal {
			deltaTotal = uint64(total) - prevTotal
		}
		if uint64(errs) >= prevErrs {
			deltaErrs = uint64(errs) - prevErrs
		}
		return deltaTotal, deltaErrs
	}
}

// readCounter extrai o valor numérico de um Prometheus Counter.
// Como o pacote prometheus/client_golang não expõe Value() direto,
// usamos Write(metric) que serializa o counter pra um protobuf interno.
func readCounter(c interface{ Write(*dto.Metric) error }) float64 {
	var m dto.Metric
	if err := c.Write(&m); err != nil {
		return 0
	}
	if m.Counter == nil {
		return 0
	}
	return m.Counter.GetValue()
}
