CREATE TABLE IF NOT EXISTS content_aliases (
    id SERIAL PRIMARY KEY,
    content_id INTEGER NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    alias TEXT NOT NULL UNIQUE
);

CREATE INDEX IF NOT EXISTS idx_content_aliases_alias ON content_aliases(alias);
CREATE INDEX IF NOT EXISTS idx_content_aliases_content_id ON content_aliases(content_id);
