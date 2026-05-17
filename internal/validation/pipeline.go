package validation

import (
	"context"
	"time"

	"github.com/rafaelwdornelas/mailclear/internal/metrics"
)

// Pipeline orquestra as stages na ordem fornecida.
// Stages mutam *Email diretamente; o pipeline cuida de short-circuit e métricas.
type Pipeline struct {
	stages []Stage
	met    *metrics.ValidationMetrics
}

// NewPipeline cria um pipeline com as stages na ordem dada.
func NewPipeline(met *metrics.ValidationMetrics, stages ...Stage) *Pipeline {
	return &Pipeline{stages: stages, met: met}
}

// Run executa o pipeline sobre um único email. Devolve erro apenas em falha
// inesperada (context cancelado, IO grave). Falhas de validação são representadas
// em e.Status / e.FailedAt.
func (p *Pipeline) Run(ctx context.Context, e *Email) error {
	for _, s := range p.stages {
		if err := ctx.Err(); err != nil {
			return err
		}
		start := time.Now()
		err := s.Run(ctx, e)
		dur := time.Since(start)

		if p.met != nil {
			p.met.StageDuration.WithLabelValues(s.Name()).Observe(dur.Seconds())
		}

		if err != nil {
			if p.met != nil {
				p.met.StageResult.WithLabelValues(s.Name(), "fail").Inc()
			}
			e.Stages[s.Name()] = StageOutcome{Passed: false, Reason: err.Error(), Duration: dur}
			if s.ShortCircuit() {
				e.FailedAt = s.Name()
				if e.Status == "" {
					e.Status = StatusInvalid
				}
				if p.met != nil {
					p.met.ShortCircuit.WithLabelValues(s.Name()).Inc()
					p.met.ResultStatus.WithLabelValues(string(e.Status)).Inc()
				}
				return nil
			}
			continue
		}

		e.Stages[s.Name()] = StageOutcome{Passed: true, Duration: dur}
		if p.met != nil {
			p.met.StageResult.WithLabelValues(s.Name(), "ok").Inc()
		}
	}

	// NÃO definimos StatusUnknown aqui — o ScoreEngine.Apply é o responsável
	// pela classificação final. Setar aqui faria o scorer respeitar "unknown"
	// e nunca atribuir valid/risky/invalid baseado no score.
	if p.met != nil && e.Status != "" {
		p.met.ResultStatus.WithLabelValues(string(e.Status)).Inc()
	}
	return nil
}
