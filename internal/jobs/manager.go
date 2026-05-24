// Package jobs orquestra a criação, submissão e atualização de jobs de validação.
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelwdornelas/mailclear/internal/storage"
	"github.com/rafaelwdornelas/mailclear/internal/validation"
	"github.com/rafaelwdornelas/mailclear/internal/workers"
)

// Manager combina o repository de jobs com o pool de workers e o validator.
// Expõe operações de alto nível para o handler HTTP.
type Manager struct {
	jobs      *storage.JobsRepo
	results   *storage.ResultsRepo
	validator validation.Validator
	pool      *workers.Pool
	batchSize int
}

// NewManager monta o Manager.
func NewManager(
	jobsRepo *storage.JobsRepo,
	resultsRepo *storage.ResultsRepo,
	val validation.Validator,
	pool *workers.Pool,
	batchSize int,
) *Manager {
	if batchSize <= 0 {
		batchSize = 1000
	}
	return &Manager{
		jobs:      jobsRepo,
		results:   resultsRepo,
		validator: val,
		pool:      pool,
		batchSize: batchSize,
	}
}

// QueueUsage devolve a fração [0,1] da fila do pool em uso.
// O handler HTTP de import usa pra aplicar backpressure (503).
func (m *Manager) QueueUsage() float64 { return m.pool.QueueUsage() }

// QueueLen devolve quantas tasks estão pendentes agora.
func (m *Manager) QueueLen() int { return m.pool.QueueLen() }

// Create cria um job na DB e marca como pending.
func (m *Manager) Create(ctx context.Context, name, source string, total int64, metadata map[string]any) (*storage.Job, error) {
	var raw []byte
	if metadata != nil {
		var err error
		raw, err = json.Marshal(metadata)
		if err != nil {
			return nil, err
		}
	}
	return m.jobs.Create(ctx, name, source, total, raw)
}

// Submit divide emails em batches e enfileira no pool.
// Marca o job como 'running' e dispara um goroutine que monitora o progresso
// para sinalizar 'completed' quando processed >= total.
func (m *Manager) Submit(ctx context.Context, jobID uuid.UUID, emails []string) error {
	if err := m.jobs.UpdateStatus(ctx, jobID, "running"); err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	total := int64(len(emails))
	for i := 0; i < len(emails); i += m.batchSize {
		end := i + m.batchSize
		if end > len(emails) {
			end = len(emails)
		}
		task := &workers.Task{JobID: jobID, Emails: emails[i:end]}
		if err := m.pool.Submit(ctx, task); err != nil {
			return err
		}
	}
	go m.waitForCompletion(jobID, total)
	return nil
}

// waitForCompletion faz polling no progresso do job e o marca como
// 'completed' quando todos os emails foram processados.
// Usa context.Background() porque o ctx do request HTTP termina ao retornar
// a resposta — o tracker precisa sobreviver ao request.
// Para jobs grandes, polling de 2s tem custo desprezível.
func (m *Manager) waitForCompletion(jobID uuid.UUID, total int64) {
	if total <= 0 {
		// Sem nada a processar — marca completo direto.
		_ = m.jobs.UpdateStatus(context.Background(), jobID, "completed")
		return
	}
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	// Timeout máximo de segurança: 24h. Jobs maiores precisam ser quebrados.
	deadline := time.Now().Add(24 * time.Hour)

	for range t.C {
		if time.Now().After(deadline) {
			_ = m.jobs.UpdateStatus(context.Background(), jobID, "failed")
			return
		}
		j, err := m.jobs.Get(context.Background(), jobID)
		if err != nil {
			return
		}
		// Se foi cancelado/marcado por outro lugar, não interfere.
		if j.Status == "cancelled" || j.Status == "completed" || j.Status == "failed" {
			return
		}
		if j.Processed >= total {
			_ = m.jobs.UpdateStatus(context.Background(), jobID, "completed")
			return
		}
	}
}

// Get devolve o status do job.
func (m *Manager) Get(ctx context.Context, id uuid.UUID) (*storage.Job, error) {
	return m.jobs.Get(ctx, id)
}

// List devolve uma página de jobs.
func (m *Manager) List(ctx context.Context, limit, offset int32) ([]*storage.Job, error) {
	return m.jobs.List(ctx, limit, offset)
}

// Cancel cancela um job.
func (m *Manager) Cancel(ctx context.Context, id uuid.UUID) error {
	return m.jobs.Cancel(ctx, id)
}

// Results devolve uma página de resultados.
func (m *Manager) Results(ctx context.Context, jobID uuid.UUID, status string, afterID int64, limit int32) ([]*storage.EmailResult, error) {
	return m.results.ListByJob(ctx, jobID, status, afterID, limit)
}

// CountByStatus devolve contagem por status no job.
func (m *Manager) CountByStatus(ctx context.Context, jobID uuid.UUID) (map[string]int64, error) {
	return m.results.CountByStatus(ctx, jobID)
}

// MarkCompleted atualiza status do job (chamado externamente quando o
// chamador sabe que todos os batches foram enfileirados E processados).
// Em fluxo simples (channels), o caller decide quando chamar.
func (m *Manager) MarkCompleted(ctx context.Context, id uuid.UUID) error {
	return m.jobs.UpdateStatus(ctx, id, "completed")
}

// Requeue cria um job novo com os mesmos emails do job `srcID` e o submete
// para processamento. O job original não é alterado — fica como referência.
//
// O novo job tem:
//   - nome do job original + " (requeue)"
//   - metadata.requeued_from = <id original>
//   - total = quantidade de emails únicos buscados de `emails.email_original`
//
// Note que duplicatas no `email_normalized` são silenciosamente filtradas
// pelo BulkInsert via ON CONFLICT DO NOTHING (efeito do fix do A1).
func (m *Manager) Requeue(ctx context.Context, srcID uuid.UUID) (*storage.Job, error) {
	src, err := m.jobs.Get(ctx, srcID)
	if err != nil {
		return nil, fmt.Errorf("job origem: %w", err)
	}

	emails, err := m.results.ListOriginalsByJob(ctx, srcID)
	if err != nil {
		return nil, fmt.Errorf("listar emails: %w", err)
	}
	if len(emails) == 0 {
		return nil, fmt.Errorf("job %s não tem emails para re-enfileirar", srcID)
	}

	metaRaw, _ := json.Marshal(map[string]any{
		"requeued_from": srcID.String(),
		"source_name":   src.Name,
	})

	job, err := m.jobs.Create(ctx, src.Name+" (requeue)", "requeue", int64(len(emails)), metaRaw)
	if err != nil {
		return nil, fmt.Errorf("criar job: %w", err)
	}

	if err := m.Submit(ctx, job.ID, emails); err != nil {
		return nil, fmt.Errorf("submit: %w", err)
	}
	return job, nil
}
