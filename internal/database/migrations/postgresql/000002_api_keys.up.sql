-- API keys (Epic 1, Story 1.1) — PostgreSQL
CREATE TABLE IF NOT EXISTS api_keys (
    id            SERIAL PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    key_id        TEXT NOT NULL,
    key_hash      TEXT NOT NULL,
    last_used_at  TIMESTAMPTZ,
    last_used_ip  TEXT,
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at    TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_key_id  ON api_keys(key_id);
CREATE INDEX        IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
-- Per-user unique name among active (non-revoked) keys — DB backstop for AC9
-- (the app-level COUNT-then-INSERT is racy under concurrency). Revoked names
-- remain reusable because the partial index only covers active rows.
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_user_name ON api_keys(user_id, name) WHERE revoked_at IS NULL;
