package storage

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Job representa um lote de validação.
type Job struct {
	ID         uuid.UUID       `json:"id"`
	Name       string          `json:"name"`
	Status     string          `json:"status"`
	Source     string          `json:"source"`
	Total      int64           `json:"total"`
	Processed  int64           `json:"processed"`
	Valid      int64           `json:"valid"`
	Invalid    int64           `json:"invalid"`
	Risky      int64           `json:"risky"`
	Unknown    int64           `json:"unknown"`
	Disposable int64           `json:"disposable"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	StartedAt  *time.Time      `json:"started_at,omitempty"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
	Metadata   json.RawMessage `json:"metadata"`
}

// ProgressDelta representa o incremento atômico de progresso de um job.
type ProgressDelta struct {
	Processed  int64
	Valid      int64
	Invalid    int64
	Risky      int64
	Unknown    int64
	Disposable int64
}

// JobsRepo encapsula operações sobre a tabela jobs.
type JobsRepo struct {
	pool *pgxpool.Pool
}

// NewJobsRepo cria o repository.
func NewJobsRepo(pool *pgxpool.Pool) *JobsRepo {
	return &JobsRepo{pool: pool}
}

// Create insere um novo job e devolve o ID gerado.
func (r *JobsRepo) Create(ctx context.Context, name, source string, total int64, metadata []byte) (*Job, error) {
	if metadata == nil {
		metadata = []byte("{}")
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO jobs (name, source, total, metadata)
		VALUES ($1, $2, $3, $4::jsonb)
		RETURNING id, name, status, source, total, processed, valid, invalid, risky, unknown, disposable,
		          created_at, updated_at, started_at, finished_at, metadata
	`, name, source, total, metadata)

	return scanJob(row)
}

// Get busca um job pelo ID.
func (r *JobsRepo) Get(ctx context.Context, id uuid.UUID) (*Job, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, status, source, total, processed, valid, invalid, risky, unknown, disposable,
		       created_at, updated_at, started_at, finished_at, metadata
		FROM jobs WHERE id = $1
	`, id)
	return scanJob(row)
}

// List devolve jobs paginados (ordem decrescente por created_at).
func (r *JobsRepo) List(ctx context.Context, limit, offset int32) ([]*Job, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, status, source, total, processed, valid, invalid, risky, unknown, disposable,
		       created_at, updated_at, started_at, finished_at, metadata
		FROM jobs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// UpdateStatus muda o status do job e ajusta started_at/finished_at conforme transição.
// Cast explícito para job_status evita SQLSTATE 42P08 (pgx não infere o tipo do
// parâmetro quando ele aparece em contextos diferentes na mesma query).
func (r *JobsRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE jobs
		SET status      = $2::job_status,
		    started_at  = COALESCE(started_at, CASE WHEN $2::job_status = 'running'::job_status THEN now() ELSE NULL END),
		    finished_at = CASE WHEN $2::job_status IN ('completed'::job_status, 'failed'::job_status, 'cancelled'::job_status) THEN now() ELSE finished_at END
		WHERE id = $1
	`, id, status)
	return err
}

// IncrementProgress aplica um delta atômico de contadores.
func (r *JobsRepo) IncrementProgress(ctx context.Context, id uuid.UUID, d ProgressDelta) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE jobs SET
		    processed  = processed  + $2,
		    valid      = valid      + $3,
		    invalid    = invalid    + $4,
		    risky      = risky      + $5,
		    unknown    = unknown    + $6,
		    disposable = disposable + $7
		WHERE id = $1
	`, id, d.Processed, d.Valid, d.Invalid, d.Risky, d.Unknown, d.Disposable)
	return err
}

// Cancel cancela um job ainda em execução/pendente.
func (r *JobsRepo) Cancel(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'cancelled', finished_at = now()
		WHERE id = $1 AND status IN ('pending', 'running', 'paused')
	`, id)
	return err
}

// MarkStaleAsFailed marca jobs em running/pending sem atualização há mais
// que `staleAfter` como failed. Usado no boot da aplicação para limpar
// jobs órfãos que ficaram travados em runs anteriores que crasharam ou
// foram reiniciadas no meio do processamento.
//
// Devolve o número de jobs marcados.
func (r *JobsRepo) MarkStaleAsFailed(ctx context.Context, staleAfter time.Duration) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'failed',
		    finished_at = now()
		WHERE status IN ('pending', 'running')
		  AND updated_at < now() - $1::interval
	`, staleAfter.String())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// rowScanner permite reaproveitar a função de scan para QueryRow e Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(r rowScanner) (*Job, error) {
	var j Job
	err := r.Scan(
		&j.ID, &j.Name, &j.Status, &j.Source,
		&j.Total, &j.Processed, &j.Valid, &j.Invalid, &j.Risky, &j.Unknown, &j.Disposable,
		&j.CreatedAt, &j.UpdatedAt, &j.StartedAt, &j.FinishedAt, &j.Metadata,
	)
	if err != nil {
		return nil, err
	}
	return &j, nil
}
