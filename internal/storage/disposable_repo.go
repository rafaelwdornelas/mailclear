package storage

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DisposableRepo gerencia a lista de domínios disposable e role prefixes.
type DisposableRepo struct {
	pool *pgxpool.Pool
}

// NewDisposableRepo cria o repository.
func NewDisposableRepo(pool *pgxpool.Pool) *DisposableRepo {
	return &DisposableRepo{pool: pool}
}

// List devolve todos os domínios disposable persistidos.
func (r *DisposableRepo) List(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT domain FROM disposable_domains ORDER BY domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// BulkUpsert insere/atualiza em massa.
func (r *DisposableRepo) BulkUpsert(ctx context.Context, domains []string, source string) error {
	if len(domains) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO disposable_domains (domain, source)
		SELECT unnest($1::text[]), $2
		ON CONFLICT (domain) DO UPDATE SET
		    source     = EXCLUDED.source,
		    updated_at = now()
	`, domains, source)
	return err
}

// Count devolve quantidade total de disposable persistidos.
func (r *DisposableRepo) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM disposable_domains`).Scan(&n)
	return n, err
}

// RolePrefix representa um prefixo de role account com risco.
type RolePrefix struct {
	Prefix    string
	Category  string
	RiskScore int16
}

// ListRolePrefixes devolve todos os prefixos de role accounts.
func (r *DisposableRepo) ListRolePrefixes(ctx context.Context) ([]RolePrefix, error) {
	rows, err := r.pool.Query(ctx, `SELECT prefix, category, risk_score FROM role_prefixes ORDER BY prefix`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RolePrefix
	for rows.Next() {
		var rp RolePrefix
		if err := rows.Scan(&rp.Prefix, &rp.Category, &rp.RiskScore); err != nil {
			return nil, err
		}
		out = append(out, rp)
	}
	return out, rows.Err()
}
