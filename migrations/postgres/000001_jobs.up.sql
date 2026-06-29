CREATE TABLE jobs (
    id         UUID PRIMARY KEY,
    type       TEXT NOT NULL,
    payload    JSONB NOT NULL DEFAULT '{}',
    status     TEXT NOT NULL,
    result     JSONB,
    error      TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_jobs_status ON jobs(status);

CREATE INDEX idx_jobs_created_at ON jobs(created_at DESC);
