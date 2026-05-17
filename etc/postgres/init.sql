-- ════════════════════════════════════════════════════════════════════════
-- MailClear — Inicialização do PostgreSQL
-- PROJETO HARDCODED: senha fixa "mailclear".
-- (Acesso só via 127.0.0.1; pg_hba garante isso)
-- ════════════════════════════════════════════════════════════════════════
\set ON_ERROR_STOP on

-- ── Cria role apenas se não existir ──────────────────────────────────
SELECT 'CREATE ROLE mailclear LOGIN PASSWORD ''mailclear'''
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'mailclear')
\gexec

-- Garante a senha (caso a role já existisse com outra)
ALTER ROLE mailclear WITH LOGIN PASSWORD 'mailclear';

-- ── Cria database apenas se não existir ──────────────────────────────
SELECT 'CREATE DATABASE mailclear OWNER mailclear ENCODING ''UTF8'' LC_COLLATE ''C.UTF-8'' LC_CTYPE ''C.UTF-8'' TEMPLATE template0'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'mailclear')
\gexec

-- ── Conecta no DB para criar extensions ──────────────────────────────
\c mailclear

CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "citext";
CREATE EXTENSION IF NOT EXISTS "btree_gin";

GRANT ALL PRIVILEGES ON DATABASE mailclear TO mailclear;
GRANT ALL ON SCHEMA public TO mailclear;
