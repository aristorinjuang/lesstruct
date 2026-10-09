-- comments module schema: the comments discussion table.
-- Runs in the module's own schema_migrations_comments table, after the core
-- migrations (the content_items and users parent tables come from core
-- 000001). CREATE IF NOT EXISTS keeps upgrades from pre-module installs
-- (whose comments came from core 000001) idempotent.
CREATE TABLE IF NOT EXISTS comments (
    id SERIAL PRIMARY KEY,
    content_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    comment TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_comments_content_id ON comments(content_id);
CREATE INDEX IF NOT EXISTS idx_comments_user_id ON comments(user_id);
