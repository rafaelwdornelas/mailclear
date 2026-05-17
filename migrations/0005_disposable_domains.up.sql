CREATE TABLE disposable_domains (
    domain      CITEXT       PRIMARY KEY,
    source      TEXT         NOT NULL DEFAULT 'manual',
    added_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON TABLE disposable_domains IS 'Lista atualizada de provedores descartáveis';

CREATE INDEX disposable_domains_source_idx
    ON disposable_domains (source);

CREATE INDEX disposable_domains_updated_at_brin_idx
    ON disposable_domains USING BRIN (updated_at);
