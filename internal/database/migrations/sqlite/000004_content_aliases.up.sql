CREATE TABLE IF NOT EXISTS content_aliases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content_id INTEGER NOT NULL,
    alias TEXT NOT NULL UNIQUE,
    FOREIGN KEY (content_id) REFERENCES content_items(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_content_aliases_alias ON content_aliases(alias);
CREATE INDEX IF NOT EXISTS idx_content_aliases_content_id ON content_aliases(content_id);
