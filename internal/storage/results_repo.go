package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

// emailCols é a ordem canônica de colunas para BulkInsert / staging.
var emailCols = []string{
	"job_id", "email_original", "email_normalized", "local_part", "domain",
	"status", "classification", "score", "reasons", "suggested_email",
	"is_disposable", "is_role", "has_mx",
}

// BulkInsert insere um batch de resultados ignorando conflitos no índice único
// (job_id, email_normalized). Usa staging-table + COPY + INSERT ... ON CONFLICT
// DO NOTHING dentro de uma transação:
//
//  1. Cria TEMP TABLE _staging_emails ON COMMIT DROP (sem constraints).
//  2. COPY do batch direto na staging — rápido.
//  3. INSERT INTO emails SELECT DISTINCT ON ... ON CONFLICT DO NOTHING.
//
// O DISTINCT ON resolve duplicatas DENTRO do batch (mesmo email_normalized
// aparecendo 2x), ON CONFLICT resolve duplicatas com batches anteriores
// (validator.Normalize pode colapsar emails que o importer.Dedup não pegou,
// e.g. john.doe@gmail.com ↔ johndoe@gmail.com).
//
// Devolve quantas linhas foram efetivamente persistidas (0 ≤ inserted ≤ len(items)).
func (r *ResultsRepo) BulkInsert(ctx context.Context, items []*EmailResult) (int64, error) {
	if len(items) == 0 {
		return 0, nil
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

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Staging table sem constraints — qualquer dedup acontece no INSERT abaixo.
	// ON COMMIT DROP garante que somem ao fim da transação (ou no rollback).
	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE _staging_emails (
			job_id           uuid                NOT NULL,
			email_original   text                NOT NULL,
			email_normalized citext              NOT NULL,
			local_part       text                NOT NULL,
			domain           citext              NOT NULL,
			status           email_status        NOT NULL,
			classification   email_classification NOT NULL,
			score            smallint            NOT NULL,
			reasons          jsonb               NOT NULL,
			suggested_email  text,
			is_disposable    bool                NOT NULL,
			is_role          bool                NOT NULL,
			has_mx           bool                NOT NULL
		) ON COMMIT DROP
	`); err != nil {
		return 0, fmt.Errorf("create staging: %w", err)
	}

	if _, err := tx.CopyFrom(ctx,
		pgx.Identifier{"_staging_emails"},
		emailCols,
		pgx.CopyFromRows(rows),
	); err != nil {
		return 0, fmt.Errorf("copy staging: %w", err)
	}

	colList := strings.Join(emailCols, ", ")
	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO emails (%s)
		SELECT DISTINCT ON (job_id, email_normalized) %s
		FROM _staging_emails
		ON CONFLICT (job_id, email_normalized) DO NOTHING
	`, colList, colList))
	if err != nil {
		return 0, fmt.Errorf("insert from staging: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return tag.RowsAffected(), nil
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
	Total      int64 `json:"total"`
	Valid      int64 `json:"valid"`
	Invalid    int64 `json:"invalid"`
	Risky      int64 `json:"risky"`
	Disposable int64 `json:"disposable"`
	Unknown    int64 `json:"unknown"`
}

// Global devolve estatísticas globais agregadas a partir da tabela `jobs`,
// que é pequena (dezenas a centenas de linhas) — varrer `emails` (potencialmente
// bilhões) com COUNT FILTER levava 20+ segundos a 10M linhas.
//
// Os contadores em jobs são atualizados a cada batch via IncrementProgress.
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
	return &s, nil
}

// ListOriginalsByJob devolve a lista de email_original de um job, paginada
// por cursor de id. Usado por requeue para re-submeter o mesmo conjunto.
func (r *ResultsRepo) ListOriginalsByJob(ctx context.Context, jobID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT email_original FROM emails WHERE job_id = $1 ORDER BY id ASC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0, 1024)
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
