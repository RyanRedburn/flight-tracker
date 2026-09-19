CREATE TABLE api_keys (
    id         TEXT PRIMARY KEY,
    prefix     TEXT NOT NULL UNIQUE,
    key_hash   BYTEA NOT NULL UNIQUE,
    role       TEXT NOT NULL CHECK (role IN ('consumer', 'subscriber', 'admin')),
    name       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_created_at ON api_keys (created_at DESC);

-- One row per identity×surface. Tokens are milli-tokens (1000 = 1 request).
-- Shared across API replicas so advertised caps are not multiplied per process.
CREATE TABLE rate_limit_buckets (
    bucket_key     TEXT PRIMARY KEY,
    tokens         BIGINT NOT NULL,
    last_refill_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_rate_limit_buckets_last_refill
    ON rate_limit_buckets (last_refill_at);
