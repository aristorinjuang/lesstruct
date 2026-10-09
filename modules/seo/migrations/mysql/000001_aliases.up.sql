-- seo module schema: the content_aliases legacy-URL table.
-- Runs in the module's own schema_migrations_seo table, after the core
-- migrations (the content_items parent table comes from core 000001).
-- The indexes fold into the CREATE TABLE as KEY clauses (rather than
-- separate CREATE INDEX statements, which MySQL cannot re-apply): with
-- CREATE TABLE IF NOT EXISTS the whole statement no-ops on upgrades from
-- pre-module installs (whose aliases came from core 000004).
CREATE TABLE IF NOT EXISTS content_aliases (
    id INT AUTO_INCREMENT PRIMARY KEY,
    content_id INT NOT NULL,
    alias VARCHAR(255) NOT NULL UNIQUE,
    KEY idx_content_aliases_alias (alias),
    KEY idx_content_aliases_content_id (content_id),
    FOREIGN KEY (content_id) REFERENCES content_items(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
