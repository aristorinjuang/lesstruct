-- media module schema: the media_files library table.
-- Runs in the module's own schema_migrations_media table, after the core
-- migrations (the users parent table comes from core 000001). CREATE IF NOT
-- EXISTS keeps upgrades from pre-module installs (whose media_files came
-- from core 000001) idempotent.
CREATE TABLE IF NOT EXISTS media_files (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    width INTEGER,
    height INTEGER,
    alt_text TEXT,
    is_webp BOOLEAN DEFAULT TRUE,
    file_path TEXT NOT NULL,
    url TEXT NOT NULL,
    hash TEXT UNIQUE NOT NULL,
    variants JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_media_files_user_id ON media_files(user_id);
CREATE INDEX IF NOT EXISTS idx_media_files_hash ON media_files(hash);
