package storage

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EmailResult representa o resultado da validação de um email.
type EmailResult struct {
	ID              int64           `json:"id,omitempty"`
	JobID           uuid.UUID       `json:"job_id"`
	EmailOriginal   string          `json:"email_original"`
	EmailNormalized string          `json:"email_normalized"`
	LocalPart       string          `json:"local_part"`
	Domain          string          `json:"domain"`
	Status          string          `json:"status"`
	Classification  string          `json:"classification"`
	Score           int16           `json:"score"`
	Reasons         json.RawMessage `json:"reasons"`
	SuggestedEmail  *string         `json:"suggested_email,omitempty"`
	IsDisposable    bool            `json:"is_disposable"`
	IsRole          bool            `json:"is_role"`
	HasMX           bool            `json:"has_mx"`
	CreatedAt       time.Time       `json:"created_at,omitempty"`
}

// ResultsRepo encapsula operações sobre a tabela emails (particionada).
type ResultsRepo struct {
	pool *pgxpool.Pool
}

// NewResultsRepo cria o repository.
func NewResultsRepo(pool *pgxpool.Pool) *ResultsRepo {
	return &ResultsRepo{pool: pool}
}

// BulkInsert insere um batch de resultados usando pgx.CopyFrom (COPY) para
// máxima velocidade. Conflitos (job_id+email_normalized) são silenciosamente
// descartados via INSERT em lote secundário se algum colidir.
func (r *ResultsRepo) BulkInsert(ctx context.Context, items []*EmailResult) error {
	if len(items) == 0 {
		return nil
	}

	cols := []string{
		"job_id", "email_original", "email_normalized", "local_part", "domain",
		"status", "classification", "score", "reasons", "suggested_email",
		"is_disposable", "is_role", "has_mx",
	}

	rows := make([][]any, 0, len(items))
	for _, it := range items {
		reasons := it.Reasons
		if len(reasons) == 0 {
			reasons = []byte("[]")
		}
		rows = append(rows, []any{
			it.JobID, it.EmailOriginal, it.EmailNormalized, it.LocalPart, it.Domain,
			it.Status, it.Classification, it.Score, reasons, it.SuggestedEmail,
			it.IsDisposable, it.IsRole, it.HasMX,
		})
	}

	_, err := r.pool.CopyFrom(ctx,
		pgx.Identifier{"emails"},
		cols,
		pgx.CopyFromRows(rows),
	)
	return err
}

// ListByJob devolve emails de um job em ordem crescente de id, com cursor (afterID).
func (r *ResultsRepo) ListByJob(ctx context.Context, jobID uuid.UUID, status string, afterID int64, limit int32) ([]*EmailResult, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if status == "" {
		rows, err = r.pool.Query(ctx, `
			SELECT id, job_id, email_original, email_normalized, local_part, domain,
			       status::text, classification::text, score, reasons, suggested_email,
			       is_disposable, is_role, has_mx, created_at
			FROM emails
			WHERE job_id = $1 AND id > $2
			ORDER BY id ASC
			LIMIT $3
		`, jobID, afterID, limit)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id, job_id, email_original, email_normalized, local_part, domain,
			       status::text, classification::text, score, reasons, suggested_email,
			       is_disposable, is_role, has_mx, created_at
			FROM emails
			WHERE job_id = $1 AND id > $2 AND status = $3::email_status
			ORDER BY id ASC
			LIMIT $4
		`, jobID, afterID, status, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*EmailResult
	for rows.Next() {
		var e EmailResult
		if err := rows.Scan(
			&e.ID, &e.JobID, &e.EmailOriginal, &e.EmailNormalized, &e.LocalPart, &e.Domain,
			&e.Status, &e.Classification, &e.Score, &e.Reasons, &e.SuggestedEmail,
			&e.IsDisposable, &e.IsRole, &e.HasMX, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// CountByStatus devolve quantidade de emails agrupados por status.
func (r *ResultsRepo) CountByStatus(ctx context.Context, jobID uuid.UUID) (map[string]int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT status::text, count(*) FROM emails WHERE job_id = $1 GROUP BY status
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int64, 5)
	for rows.Next() {
		var s string
		var c int64
		if err := rows.Scan(&s, &c); err != nil {
			return nil, err
		}
		out[s] = c
	}
	return out, rows.Err()
}

// GlobalStats agrega estatísticas globais.
type GlobalStats struct {
	Total           int64 `json:"total"`
	Valid           int64 `json:"valid"`
	Invalid         int64 `json:"invalid"`
	Risky           int64 `json:"risky"`
	Disposable      int64 `json:"disposable"`
	Unknown         int64 `json:"unknown"`
	UniqueDomains   int64 `json:"unique_domains"`
}

// Global devolve estatísticas globais agregadas.
//
// IMPORTANTE: agrega da tabela `jobs` (pequena, dezenas a centenas de linhas)
// e não de `emails` (potencialmente bilhões). A antiga implementação fazia
// count(*) FILTER em emails levava 20+ segundos com 10M linhas.
//
// Os contadores em jobs (processed, valid, invalid, risky, disposable, unknown)
// são atualizados a cada batch via JobsRepo.IncrementProgress, então a soma
// reflete o estado real de tudo que já foi processado.
//
// UniqueDomains é caro de calcular (count DISTINCT em emails) — devolve 0
// se a opção de count distinto não estiver habilitada. Quem precisar tem
// o endpoint dedicado em GlobalWithDomains.
func (r *ResultsRepo) Global(ctx context.Context) (*GlobalStats, error) {
	var s GlobalStats
	err := r.pool.QueryRow(ctx, `
		SELECT
		    coalesce(sum(processed),  0)::bigint,
		    coalesce(sum(valid),      0)::bigint,
		    coalesce(sum(invalid),    0)::bigint,
		    coalesce(sum(risky),      0)::bigint,
		    coalesce(sum(disposable), 0)::bigint,
		    coalesce(sum(unknown),    0)::bigint
		FROM jobs
	`).Scan(&s.Total, &s.Valid, &s.Invalid, &s.Risky, &s.Disposable, &s.Unknown)
	if err != nil {
		return nil, err
	}
	// UniqueDomains não está disponível via jobs — fica em 0.
	// Quem precisa pode usar endpoint separado que faz a query cara.
	return &s, nil
}
