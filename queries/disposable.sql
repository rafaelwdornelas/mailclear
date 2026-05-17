-- name: ListDisposableDomains :many
SELECT domain FROM disposable_domains ORDER BY domain;

-- name: IsDisposable :one
SELECT EXISTS(SELECT 1 FROM disposable_domains WHERE domain = $1) AS is_disposable;

-- name: UpsertDisposableDomain :exec
INSERT INTO disposable_domains (domain, source)
VALUES ($1, $2)
ON CONFLICT (domain) DO UPDATE SET
    source     = EXCLUDED.source,
    updated_at = now();

-- name: BulkUpsertDisposable :exec
INSERT INTO disposable_domains (domain, source)
SELECT unnest(@domains::text[]), @source::text
ON CONFLICT (domain) DO UPDATE SET
    source     = EXCLUDED.source,
    updated_at = now();

-- name: CountDisposable :one
SELECT count(*) FROM disposable_domains;

-- name: ListRolePrefixes :many
SELECT prefix, category, risk_score FROM role_prefixes ORDER BY prefix;
