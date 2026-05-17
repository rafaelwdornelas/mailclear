package storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DomainCache representa o registro persistido de info por domínio.
type DomainCache struct {
	Domain          string    `json:"domain"`
	MXRecords       []string  `json:"mx_records"`
	ARecords        []string  `json:"a_records"`
	AAAARecords     []string  `json:"aaaa_records"`
	HasMX           bool      `json:"has_mx"`
	HasA            bool      `json:"has_a"`
	IsDisposable    bool      `json:"is_disposable"`
	IsRoleOnly      bool      `json:"is_role_only"`
	IsFreeProvider  bool      `json:"is_free_provider"`
	IsCatchAll      *bool     `json:"is_catch_all,omitempty"`
	Status          string    `json:"status"`
	TTLSeconds      int32     `json:"ttl_seconds"`
	LastCheckedAt   time.Time `json:"last_checked_at"`
	ExpiresAt       time.Time `json:"expires_at"`
}

// DomainsRepo encapsula operações sobre a tabela domains_cache.
type DomainsRepo struct {
	pool *pgxpool.Pool
}

// NewDomainsRepo cria o repository.
func NewDomainsRepo(pool *pgxpool.Pool) *DomainsRepo {
	return &DomainsRepo{pool: pool}
}

// Get devolve o cache do domínio se existir.
func (r *DomainsRepo) Get(ctx context.Context, domain string) (*DomainCache, error) {
	var d DomainCache
	err := r.pool.QueryRow(ctx, `
		SELECT domain, mx_records, a_records, aaaa_records,
		       has_mx, has_a, is_disposable, is_role_only, is_free_provider, is_catch_all,
		       status::text, ttl_seconds, last_checked_at, expires_at
		FROM domains_cache WHERE domain = $1
	`, domain).Scan(
		&d.Domain, &d.MXRecords, &d.ARecords, &d.AAAARecords,
		&d.HasMX, &d.HasA, &d.IsDisposable, &d.IsRoleOnly, &d.IsFreeProvider, &d.IsCatchAll,
		&d.Status, &d.TTLSeconds, &d.LastCheckedAt, &d.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// Upsert insere ou atualiza um cache de domínio.
func (r *DomainsRepo) Upsert(ctx context.Context, d *DomainCache) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO domains_cache (
		    domain, mx_records, a_records, aaaa_records,
		    has_mx, has_a, is_disposable, is_role_only, is_free_provider, is_catch_all,
		    status, ttl_seconds, last_checked_at, expires_at, check_count
		) VALUES (
		    $1, $2, $3, $4,
		    $5, $6, $7, $8, $9, $10,
		    $11::domain_status, $12, now(), now() + ($12 || ' seconds')::interval, 1
		)
		ON CONFLICT (domain) DO UPDATE SET
		    mx_records       = EXCLUDED.mx_records,
		    a_records        = EXCLUDED.a_records,
		    aaaa_records     = EXCLUDED.aaaa_records,
		    has_mx           = EXCLUDED.has_mx,
		    has_a            = EXCLUDED.has_a,
		    is_disposable    = EXCLUDED.is_disposable,
		    is_role_only     = EXCLUDED.is_role_only,
		    is_free_provider = EXCLUDED.is_free_provider,
		    is_catch_all     = COALESCE(EXCLUDED.is_catch_all, domains_cache.is_catch_all),
		    status           = EXCLUDED.status,
		    ttl_seconds      = EXCLUDED.ttl_seconds,
		    last_checked_at  = now(),
		    expires_at       = now() + (EXCLUDED.ttl_seconds || ' seconds')::interval,
		    check_count      = domains_cache.check_count + 1
	`,
		d.Domain, d.MXRecords, d.ARecords, d.AAAARecords,
		d.HasMX, d.HasA, d.IsDisposable, d.IsRoleOnly, d.IsFreeProvider, d.IsCatchAll,
		d.Status, d.TTLSeconds,
	)
	return err
}

// IncrementHit incrementa hit_count atomicamente.
func (r *DomainsRepo) IncrementHit(ctx context.Context, domain string) error {
	_, err := r.pool.Exec(ctx, `UPDATE domains_cache SET hit_count = hit_count + 1 WHERE domain = $1`, domain)
	return err
}
