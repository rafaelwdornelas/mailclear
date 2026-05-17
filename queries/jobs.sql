-- name: CreateJob :one
INSERT INTO jobs (name, source, total, metadata)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetJob :one
SELECT * FROM jobs WHERE id = $1;

-- name: ListJobs :many
SELECT * FROM jobs
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountJobs :one
SELECT count(*) FROM jobs;

-- name: UpdateJobStatus :exec
UPDATE jobs
SET status = $2,
    started_at = COALESCE(started_at, CASE WHEN $2::job_status = 'running' THEN now() ELSE NULL END),
    finished_at = CASE WHEN $2::job_status IN ('completed', 'failed', 'cancelled') THEN now() ELSE finished_at END
WHERE id = $1;

-- name: IncrementJobProgress :exec
UPDATE jobs
SET processed  = processed  + $2,
    valid      = valid      + $3,
    invalid    = invalid    + $4,
    risky      = risky      + $5,
    unknown    = unknown    + $6,
    disposable = disposable + $7
WHERE id = $1;

-- name: CancelJob :exec
UPDATE jobs
SET status = 'cancelled', finished_at = now()
WHERE id = $1 AND status IN ('pending', 'running', 'paused');

-- name: DeleteJob :exec
DELETE FROM jobs WHERE id = $1;
