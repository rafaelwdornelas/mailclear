-- Enum de status do job
CREATE TYPE job_status AS ENUM (
    'pending',
    'running',
    'paused',
    'completed',
    'failed',
    'cancelled'
);

-- Tabela de jobs (lotes de validação)
CREATE TABLE jobs (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT         NOT NULL,
    status      job_status   NOT NULL DEFAULT 'pending',
    source      TEXT         NOT NULL DEFAULT 'api',

    total       BIGINT       NOT NULL DEFAULT 0,
    processed   BIGINT       NOT NULL DEFAULT 0,
    valid       BIGINT       NOT NULL DEFAULT 0,
    invalid     BIGINT       NOT NULL DEFAULT 0,
    risky       BIGINT       NOT NULL DEFAULT 0,
    unknown     BIGINT       NOT NULL DEFAULT 0,
    disposable  BIGINT       NOT NULL DEFAULT 0,

    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,

    metadata    JSONB        NOT NULL DEFAULT '{}'::jsonb,

    CONSTRAINT jobs_counts_non_negative
        CHECK (total >= 0 AND processed >= 0 AND valid >= 0
               AND invalid >= 0 AND risky >= 0 AND unknown >= 0 AND disposable >= 0),
    CONSTRAINT jobs_processed_le_total
        CHECK (processed <= total OR total = 0),
    CONSTRAINT jobs_finished_after_started
        CHECK (finished_at IS NULL OR started_at IS NULL OR finished_at >= started_at)
);

COMMENT ON TABLE  jobs IS 'Cada job representa um lote de validação';
COMMENT ON COLUMN jobs.metadata IS 'JSONB livre: tags, opts, callback URL';

CREATE INDEX jobs_status_created_at_idx
    ON jobs (status, created_at DESC);

CREATE INDEX jobs_created_at_brin_idx
    ON jobs USING BRIN (created_at);

CREATE INDEX jobs_metadata_gin_idx
    ON jobs USING GIN (metadata jsonb_path_ops);

CREATE TRIGGER jobs_updated_at
    BEFORE UPDATE ON jobs
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
