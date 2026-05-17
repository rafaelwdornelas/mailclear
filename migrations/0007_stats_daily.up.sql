CREATE TABLE stats_daily (
    date              DATE         PRIMARY KEY,
    total_processed   BIGINT       NOT NULL DEFAULT 0,
    valid_count       BIGINT       NOT NULL DEFAULT 0,
    invalid_count     BIGINT       NOT NULL DEFAULT 0,
    risky_count       BIGINT       NOT NULL DEFAULT 0,
    disposable_count  BIGINT       NOT NULL DEFAULT 0,
    unknown_count     BIGINT       NOT NULL DEFAULT 0,
    role_count        BIGINT       NOT NULL DEFAULT 0,
    typo_count        BIGINT       NOT NULL DEFAULT 0,
    unique_domains    BIGINT       NOT NULL DEFAULT 0,
    dns_queries       BIGINT       NOT NULL DEFAULT 0,
    cache_hits        BIGINT       NOT NULL DEFAULT 0,
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON TABLE stats_daily IS 'Agregados diários de validação';

CREATE TRIGGER stats_daily_updated_at
    BEFORE UPDATE ON stats_daily
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
