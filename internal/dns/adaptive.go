package dns

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// AdaptiveSemaphore é um semáforo com limite ajustável em runtime via SetLimit.
//
// Implementação custom (sync.Cond + contador) em vez de usar
// golang.org/x/sync/semaphore para evitar deadlock: SetLimit nunca bloqueia.
// Quando o limit é reduzido abaixo do inflight atual, as goroutines em
// execução TERMINAM normalmente (não são abortadas); apenas novos Acquire
// passam a respeitar o novo limit.
//
// Race-free: todas as transições passam por s.mu.
// Context-aware: Acquire respeita ctx.Done().
type AdaptiveSemaphore struct {
	mu       sync.Mutex
	cond     *sync.Cond
	limit    int
	inflight int

	// Métricas observáveis (atomic — leitura sem lock).
	acquiredTotal atomic.Int64
	releasedTotal atomic.Int64
}

// NewAdaptiveSemaphore cria com limite inicial. absoluteMax é apenas hint
// (não é enforçado aqui — quem chama SetLimit deve respeitar).
func NewAdaptiveSemaphore(initial, absoluteMax int) *AdaptiveSemaphore {
	if initial < 1 {
		initial = 1
	}
	_ = absoluteMax // mantido na assinatura por compat; cap é do AIMD
	s := &AdaptiveSemaphore{limit: initial}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// Acquire pega 1 slot. Bloqueia se inflight >= limit. Respeita ctx.
func (s *AdaptiveSemaphore) Acquire(ctx context.Context) error {
	// Goroutine watcher: se ctx cancelar enquanto esperamos, broadcast
	// pra liberar o Wait. Cleanup garantido via canal `done`.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.cond.Broadcast()
			s.mu.Unlock()
		case <-done:
		}
	}()

	s.mu.Lock()
	defer s.mu.Unlock()
	for s.inflight >= s.limit {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.cond.Wait()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.inflight++
	s.acquiredTotal.Add(1)
	return nil
}

// Release devolve 1 slot e libera 1 waiter (se houver).
func (s *AdaptiveSemaphore) Release() {
	s.mu.Lock()
	if s.inflight > 0 {
		s.inflight--
	}
	s.cond.Signal()
	s.mu.Unlock()
	s.releasedTotal.Add(1)
}

// SetLimit ajusta o limite efetivo. NÃO bloqueia — se reduzir abaixo
// do inflight atual, novos Acquire esperam até Release ir baixando o
// inflight para o novo limit.
func (s *AdaptiveSemaphore) SetLimit(newLimit int) {
	if newLimit < 1 {
		newLimit = 1
	}
	s.mu.Lock()
	if newLimit > s.limit {
		// Aumentou: pode haver waiters para acordar.
		s.limit = newLimit
		s.cond.Broadcast()
	} else {
		s.limit = newLimit
		// Não fazemos nada com inflight em curso — eles terminam normalmente.
	}
	s.mu.Unlock()
}

// Limit devolve o limite efetivo atual.
func (s *AdaptiveSemaphore) Limit() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.limit
}

// Inflight devolve slots em uso agora.
func (s *AdaptiveSemaphore) Inflight() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int64(s.inflight)
}

// ── Controlador AIMD ──────────────────────────────────────────────────

// MetricsHook é o subset de DNSMetrics que o AIMD escreve.
// Definido como interface pra evitar import circular com internal/metrics.
type MetricsHook interface {
	SetInflightLimit(v float64)
	SetInflightCurrent(v float64)
	SetLastErrorRate(v float64)
}

// AIMDController ajusta o AdaptiveSemaphore baseado em métricas de erro DNS.
// Inspirado em TCP congestion control (AIMD) com duas alavancas de queda:
//
//   - Additive Increase: erro < lower → sobe `step` (gradual)
//   - Multiplicative Decrease: erro > upper → divide por `decreaseFactor`
//   - Fast Decrease: erro > criticalRate → divide por `fastFactor` (mais agressivo)
type AIMDController struct {
	sem          *AdaptiveSemaphore
	getRate      func() (totalDelta, errorDelta uint64)
	log          zerolog.Logger
	tickInterval time.Duration
	metrics      MetricsHook

	// Thresholds (frações 0-1)
	errorRateUpper    float64 // > esse = decrease (ex: 0.05 = 5%)
	errorRateCritical float64 // > esse = fast decrease (ex: 0.20 = 20%)
	errorRateLower    float64 // < esse = increase (ex: 0.01 = 1%)

	// Increments
	additiveStep   int     // quanto subir por tick (default 64)
	multiplicative float64 // fator divisão em decrease (default 2.0)
	fastFactor     float64 // fator divisão em fast decrease (default 4.0)
	minLimit       int     // nunca desce abaixo (default 32)
	maxLimit       int     // nunca passa de (default 4096)

	// minSamples: ignora ticks com pouco dado. Sem isso, 3 queries de teste
	// com 3 erros disparam FAST-DECREASE (rate=100%) mesmo sendo só ruído.
	minSamples uint64

	// Observabilidade
	lastErrorRate atomic.Uint64 // float64 bits
}

// NewAIMDController monta o controlador com defaults sensatos para produção.
//   - tick: 3s (reage rápido a saturação)
//   - upper: 5% erro = decrease
//   - critical: 20% erro = fast decrease
//   - lower: 1% erro = increase
//   - step: +64 por tick (sobe rápido pro ótimo)
//   - decrease: /2
//   - fast: /4
//   - min: 32, max: 4096
//   - minSamples: 50 (ignora ticks ruidosos)
//
// Importante: "erro" aqui é APENAS timeout (não servfail) — servfail é resposta
// do autoritativo upstream, não saturação local. Ver stats_reader.go.
func NewAIMDController(sem *AdaptiveSemaphore, getRate func() (uint64, uint64), log zerolog.Logger) *AIMDController {
	return &AIMDController{
		sem:               sem,
		getRate:           getRate,
		log:               log.With().Str("component", "aimd").Logger(),
		tickInterval:      3 * time.Second,
		errorRateUpper:    0.05,
		errorRateCritical: 0.20,
		errorRateLower:    0.01,
		additiveStep:      64,
		multiplicative:    2.0,
		fastFactor:        4.0,
		minLimit:          32,
		maxLimit:          4096,
		minSamples:        50,
	}
}

// WithMetrics injeta um sink de métricas (chaining opcional).
func (c *AIMDController) WithMetrics(m MetricsHook) *AIMDController {
	c.metrics = m
	return c
}

// Run roda o loop de controle até ctx ser cancelado. Bloqueante.
func (c *AIMDController) Run(ctx context.Context) {
	t := time.NewTicker(c.tickInterval)
	defer t.Stop()

	c.log.Info().
		Int("initial_limit", c.sem.Limit()).
		Dur("tick", c.tickInterval).
		Float64("upper", c.errorRateUpper).
		Float64("critical", c.errorRateCritical).
		Float64("lower", c.errorRateLower).
		Int("step", c.additiveStep).
		Int("min", c.minLimit).
		Int("max", c.maxLimit).
		Uint64("min_samples", c.minSamples).
		Msg("AIMD iniciado")

	for {
		select {
		case <-ctx.Done():
			c.log.Info().Msg("AIMD encerrado")
			return
		case <-t.C:
			c.tick()
		}
	}
}

func (c *AIMDController) tick() {
	totalDelta, errorDelta := c.getRate()

	// Atualiza gauges observáveis sempre (mesmo sem tráfego).
	if c.metrics != nil {
		c.metrics.SetInflightLimit(float64(c.sem.Limit()))
		c.metrics.SetInflightCurrent(float64(c.sem.Inflight()))
	}

	if totalDelta == 0 {
		return
	}

	// Sample size mínimo: evita reagir a ruído estatístico (ex: 3 queries de
	// teste dando 3 erros vira rate=100% que dispara FAST-DECREASE em vão).
	if totalDelta < c.minSamples {
		return
	}

	rate := float64(errorDelta) / float64(totalDelta)
	c.lastErrorRate.Store(float64ToBits(rate))
	if c.metrics != nil {
		c.metrics.SetLastErrorRate(rate)
	}

	old := c.sem.Limit()
	var newLimit int
	var action string

	switch {
	case rate > c.errorRateCritical:
		// Fast decrease — sistema em colapso, corta drástico
		newLimit = int(float64(old) / c.fastFactor)
		action = "FAST-DECREASE"
	case rate > c.errorRateUpper:
		// Decrease normal
		newLimit = int(float64(old) / c.multiplicative)
		action = "DECREASE"
	case rate < c.errorRateLower:
		// Increase aditivo
		newLimit = old + c.additiveStep
		action = "INCREASE"
	default:
		newLimit = old
		action = "STABLE"
	}

	// Clamp
	if newLimit < c.minLimit {
		newLimit = c.minLimit
	}
	if newLimit > c.maxLimit {
		newLimit = c.maxLimit
	}

	if newLimit != old {
		c.sem.SetLimit(newLimit)
		c.log.Info().
			Str("action", action).
			Float64("error_rate", rate).
			Uint64("delta_total", totalDelta).
			Uint64("delta_errors", errorDelta).
			Int("old_limit", old).
			Int("new_limit", newLimit).
			Int64("inflight", c.sem.Inflight()).
			Msg("AIMD")
	}
}

// LastErrorRate devolve a taxa do último tick (0 se nenhum).
func (c *AIMDController) LastErrorRate() float64 {
	bits := c.lastErrorRate.Load()
	if bits == 0 {
		return 0
	}
	return bitsToFloat64(bits)
}

// Helpers atômicos para float64.
func float64ToBits(f float64) uint64 {
	if f == 0 {
		return 1 // sentinela "0 com amostra"
	}
	return uint64Bits(f)
}

func bitsToFloat64(b uint64) float64 {
	if b == 1 {
		return 0
	}
	return float64FromBits(b)
}
