-- name: InsertEmail :exec
INSERT INTO emails (
    job_id, email_original, email_normalized, local_part, domain,
    status, classification, score, reasons, suggested_email,
    is_disposable, is_role, has_mx
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13
)
ON CONFLICT (job_id, email_normalized) DO NOTHING;

-- name: ListEmailsByJob :many
SELECT * FROM emails
WHERE job_id = $1
  AND (sqlc.narg('status')::email_status IS NULL OR status = sqlc.narg('status')::email_status)
  AND id > $2
ORDER BY id ASC
LIMIT $3;

-- name: CountEmailsByJob :one
SELECT count(*) FROM emails WHERE job_id = $1;

-- name: CountEmailsByStatus :many
SELECT status, count(*) AS count
FROM emails
WHERE job_id = $1
GROUP BY status;

-- name: StreamEmailsForExport :many
SELECT id, email_original, email_normalized, domain, status, classification,
       score, suggested_email, is_disposable, is_role, has_mx, created_at
FROM emails
WHERE job_id = $1 AND id > $2
ORDER BY id ASC
LIMIT $3;

-- name: GlobalStats :one
SELECT
    count(*)                                            AS total,
    count(*) FILTER (WHERE status = 'valid')            AS valid_count,
    count(*) FILTER (WHERE status = 'invalid')          AS invalid_count,
    count(*) FILTER (WHERE status = 'risky')            AS risky_count,
    count(*) FILTER (WHERE status = 'disposable')       AS disposable_count,
    count(*) FILTER (WHERE status = 'unknown')          AS unknown_count,
    count(DISTINCT domain)                              AS unique_domains
FROM emails;
