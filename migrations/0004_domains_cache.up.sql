CREATE TYPE domain_status AS ENUM (
    'ok',
    'no_mx',
    'nxdomain',
    'timeout',
    'servfail'
);

CREATE TABLE domains_cache (
    domain            CITEXT        PRIMARY KEY,

    mx_records        TEXT[]        NOT NULL DEFAULT '{}',
    a_records         TEXT[]        NOT NULL DEFAULT '{}',
    aaaa_records      TEXT[]        NOT NULL DEFAULT '{}',

    has_mx            BOOLEAN       NOT NULL DEFAULT false,
    has_a             BOOLEAN       NOT NULL DEFAULT false,

    is_disposable     BOOLEAN       NOT NULL DEFAULT false,
    is_role_only      BOOLEAN       NOT NULL DEFAULT false,
    is_free_provider  BOOLEAN       NOT NULL DEFAULT false,
    is_catch_all      BOOLEAN,

    status            domain_status NOT NULL DEFAULT 'ok',

    ttl_seconds       INTEGER       NOT NULL DEFAULT 3600,

    last_checked_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    expires_at        TIMESTAMPTZ   NOT NULL DEFAULT (now() + INTERVAL '1 hour'),

    hit_count         BIGINT        NOT NULL DEFAULT 0,
    check_count       INTEGER       NOT NULL DEFAULT 0,

    created_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT now()
);

COMMENT ON TABLE  domains_cache IS 'Cache de longa duração de info por domínio (complementa Redis)';
COMMENT ON COLUMN domains_cache.expires_at IS 'Quando o registro deve ser revalidado';
COMMENT ON COLUMN domains_cache.is_catch_all IS 'NULL=desconhecido; true=aceita qualquer; false=rejeita';

CREATE INDEX domains_cache_expires_at_idx
    ON domains_cache (expires_at)
    WHERE status = 'ok';

CREATE INDEX domains_cache_status_idx
    ON domains_cache (status);

CREATE INDEX domains_cache_mx_gin_idx
    ON domains_cache USING GIN (mx_records);

CREATE TRIGGER domains_cache_updated_at
    BEFORE UPDATE ON domains_cache
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
