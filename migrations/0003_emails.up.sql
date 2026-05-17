-- Enums
CREATE TYPE email_status AS ENUM (
    'valid',
    'invalid',
    'risky',
    'unknown',
    'disposable'
);

CREATE TYPE email_classification AS ENUM (
    'personal',
    'role',
    'free',
    'corporate',
    'educational',
    'government',
    'unknown'
);

-- Tabela mestre PARTICIONADA por hash de job_id (16 partições)
-- Hash partitioning distribui escrita uniforme mesmo com jobs grandes
-- chegando em rajada. 16 = bom equilíbrio paralelismo/manutenção.
CREATE TABLE emails (
    id                 BIGSERIAL,
    job_id             UUID                  NOT NULL,
    email_original     TEXT                  NOT NULL,
    email_normalized   CITEXT                NOT NULL,
    local_part         TEXT                  NOT NULL,
    domain             CITEXT                NOT NULL,

    status             email_status          NOT NULL,
    classification     email_classification  NOT NULL DEFAULT 'unknown',
    score              SMALLINT              NOT NULL DEFAULT 0
                       CHECK (score BETWEEN 0 AND 100),

    reasons            JSONB                 NOT NULL DEFAULT '[]'::jsonb,
    suggested_email    TEXT,

    is_disposable      BOOLEAN               NOT NULL DEFAULT false,
    is_role            BOOLEAN               NOT NULL DEFAULT false,
    has_mx             BOOLEAN               NOT NULL DEFAULT false,

    created_at         TIMESTAMPTZ           NOT NULL DEFAULT now(),

    PRIMARY KEY (id, job_id),
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
) PARTITION BY HASH (job_id);

-- Cria 16 partições
DO $$
BEGIN
    FOR i IN 0..15 LOOP
        EXECUTE format(
            'CREATE TABLE emails_p%s PARTITION OF emails
             FOR VALUES WITH (MODULUS 16, REMAINDER %s)',
            i, i
        );
    END LOOP;
END$$;

COMMENT ON TABLE  emails IS 'Resultado individual da validação; particionada por hash de job_id';
COMMENT ON COLUMN emails.email_normalized IS 'Lower; gmail dots e +tags removidos quando aplicável';
COMMENT ON COLUMN emails.score IS '0-100; 0=lixo, 100=quase certeza válido';
COMMENT ON COLUMN emails.reasons IS 'Array de {code, message}';

-- Índices (propagados para partições)
CREATE INDEX emails_job_id_idx              ON emails (job_id);
CREATE INDEX emails_job_id_status_idx       ON emails (job_id, status);
CREATE INDEX emails_domain_idx              ON emails (domain);
CREATE INDEX emails_status_idx              ON emails (status);
CREATE INDEX emails_created_at_brin_idx     ON emails USING BRIN (created_at);
CREATE INDEX emails_reasons_gin_idx         ON emails USING GIN (reasons jsonb_path_ops);

CREATE UNIQUE INDEX emails_job_email_unique_idx
    ON emails (job_id, email_normalized);
