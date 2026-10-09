-- seo module schema: the content_aliases legacy-URL table.
-- Runs in the module's own schema_migrations_seo table, after the core
-- migrations (the content_items parent table comes from core 000001).
-- CREATE IF NOT EXISTS keeps upgrades from pre-module installs (whose
-- aliases came from core 000004) idempotent.
CREATE TABLE IF NOT EXISTS content_aliases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content_id INTEGER NOT NULL,
    alias TEXT NOT NULL UNIQUE,
    FOREIGN KEY (content_id) REFERENCES content_items(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_content_aliases_alias ON content_aliases(alias);
CREATE INDEX IF NOT EXISTS idx_content_aliases_content_id ON content_aliases(content_id);
