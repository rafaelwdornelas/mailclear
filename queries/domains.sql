-- name: GetDomainCache :one
SELECT * FROM domains_cache WHERE domain = $1;

-- name: UpsertDomainCache :exec
INSERT INTO domains_cache (
    domain, mx_records, a_records, aaaa_records,
    has_mx, has_a, is_disposable, is_role_only, is_free_provider, is_catch_all,
    status, ttl_seconds, last_checked_at, expires_at, check_count
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8, $9, $10,
    $11, $12, now(), now() + ($12 || ' seconds')::interval, 1
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
    is_catch_all     = EXCLUDED.is_catch_all,
    status           = EXCLUDED.status,
    ttl_seconds      = EXCLUDED.ttl_seconds,
    last_checked_at  = now(),
    expires_at       = now() + (EXCLUDED.ttl_seconds || ' seconds')::interval,
    check_count      = domains_cache.check_count + 1;

-- name: IncrementDomainHit :exec
UPDATE domains_cache SET hit_count = hit_count + 1 WHERE domain = $1;

-- name: ListExpiredDomains :many
SELECT domain FROM domains_cache
WHERE status = 'ok' AND expires_at < now()
ORDER BY expires_at ASC
LIMIT $1;

-- name: DeleteDomainCache :exec
DELETE FROM domains_cache WHERE domain = $1;
