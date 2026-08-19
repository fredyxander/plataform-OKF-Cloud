CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS documents (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename    TEXT        NOT NULL,
    storage_key TEXT        NOT NULL UNIQUE,
    format      TEXT        NOT NULL DEFAULT 'plaintext',
    size_bytes  BIGINT      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS jobs (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id      UUID        NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    owner_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status           TEXT        NOT NULL DEFAULT 'queued',
    idempotency_key  TEXT        NOT NULL UNIQUE,
    error_message    TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_status CHECK (status IN ('queued', 'processing', 'completed', 'failed'))
);

CREATE TABLE IF NOT EXISTS bundles (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id        UUID        NOT NULL UNIQUE REFERENCES jobs(id) ON DELETE CASCADE,
    owner_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    storage_key   TEXT        NOT NULL,
    is_valid      BOOLEAN     NOT NULL DEFAULT false,
    concept_count INTEGER     NOT NULL DEFAULT 0,
    published_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_documents_owner_id ON documents(owner_id);
CREATE INDEX IF NOT EXISTS idx_jobs_owner_id      ON jobs(owner_id);
CREATE INDEX IF NOT EXISTS idx_jobs_document_id   ON jobs(document_id);
CREATE INDEX IF NOT EXISTS idx_jobs_status        ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_bundles_owner_id   ON bundles(owner_id);

CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_jobs_updated_at
    BEFORE UPDATE ON jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();