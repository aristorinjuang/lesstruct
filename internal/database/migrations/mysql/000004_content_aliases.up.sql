CREATE TABLE IF NOT EXISTS content_aliases (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    content_id BIGINT NOT NULL,
    alias VARCHAR(255) NOT NULL UNIQUE,
    FOREIGN KEY (content_id) REFERENCES content_items(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX idx_content_aliases_alias ON content_aliases(alias);
CREATE INDEX idx_content_aliases_content_id ON content_aliases(content_id);
