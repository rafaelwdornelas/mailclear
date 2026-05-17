package workers

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/rafaelwdornelas/mailclear/internal/metrics"
)

// Pool é um worker pool que consome tasks de um channel buffered (backpressure).
// Suporta shutdown gracioso via context cancelation.
type Pool struct {
	handler  TaskHandler
	queue    chan *Task
	workers  int
	logger   zerolog.Logger
	metrics  *metrics.WorkersMetrics
	wg       sync.WaitGroup
	stopOnce sync.Once
}

// NewPool cria um pool com 'workers' goroutines e fila bufferizada em 'queueBuf'.
func NewPool(workers, queueBuf int, handler TaskHandler, log zerolog.Logger, met *metrics.WorkersMetrics) *Pool {
	if workers <= 0 {
		workers = 8
	}
	if queueBuf <= 0 {
		queueBuf = 1024
	}
	return &Pool{
		handler: handler,
		queue:   make(chan *Task, queueBuf),
		workers: workers,
		logger:  log.With().Str("component", "worker_pool").Logger(),
		metrics: met,
	}
}

// Start lança as goroutines workers. Bloqueia até ctx ser cancelado.
func (p *Pool) Start(ctx context.Context) {
	p.logger.Info().Int("workers", p.workers).Int("queue_buf", cap(p.queue)).Msg("iniciando worker pool")
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.run(ctx, i)
	}
}

// Submit envia uma task à fila. Bloqueia se a fila estiver cheia (backpressure).
// Retorna ctx.Err() se cancelado.
func (p *Pool) Submit(ctx context.Context, t *Task) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.queue <- t:
		if p.metrics != nil {
			p.metrics.QueueDepth.Set(float64(len(p.queue)))
		}
		return nil
	}
}

// QueueLen devolve quantas tasks estão pendentes na fila agora.
func (p *Pool) QueueLen() int { return len(p.queue) }

// QueueCap devolve o tamanho máximo da fila (definido em NewPool).
func (p *Pool) QueueCap() int { return cap(p.queue) }

// QueueUsage devolve a fração [0,1] de uso da fila. Usado por
// backpressure no handler HTTP pra decidir se aceita mais uploads.
func (p *Pool) QueueUsage() float64 {
	c := cap(p.queue)
	if c == 0 {
		return 0
	}
	return float64(len(p.queue)) / float64(c)
}

// Stop fecha a fila e aguarda todos os workers terminarem (com timeout).
func (p *Pool) Stop(timeout time.Duration) {
	p.stopOnce.Do(func() {
		close(p.queue)
	})

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info().Msg("worker pool encerrado")
	case <-time.After(timeout):
		p.logger.Warn().Dur("timeout", timeout).Msg("timeout aguardando workers; abandonando")
	}
}

func (p *Pool) run(ctx context.Context, id int) {
	defer p.wg.Done()
	log := p.logger.With().Int("worker_id", id).Logger()

	if p.metrics != nil {
		p.metrics.Active.Inc()
		defer p.metrics.Active.Dec()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case t, ok := <-p.queue:
			if !ok {
				return
			}
			if p.metrics != nil {
				p.metrics.QueueDepth.Set(float64(len(p.queue)))
			}
			start := time.Now()
			err := p.handler(ctx, t)
			if err != nil {
				if p.metrics != nil {
					p.metrics.Processed.WithLabelValues("validate_batch", "error").Inc()
				}
				log.Error().Err(err).Str("job_id", t.JobID.String()).
					Int("batch", len(t.Emails)).
					Dur("dur", time.Since(start)).
					Msg("task falhou")
			} else {
				if p.metrics != nil {
					p.metrics.Processed.WithLabelValues("validate_batch", "ok").Inc()
				}
				log.Debug().Str("job_id", t.JobID.String()).
					Int("batch", len(t.Emails)).
					Dur("dur", time.Since(start)).
					Msg("task ok")
			}
		}
	}
}
