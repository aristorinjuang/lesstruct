-- media module schema: the media_files library table.
-- Runs in the module's own schema_migrations_media table, after the core
-- migrations (the users parent table comes from core 000001). CREATE IF NOT
-- EXISTS keeps upgrades from pre-module installs (whose media_files came
-- from core 000001) idempotent.
CREATE TABLE IF NOT EXISTS media_files (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    filename TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    mime_type VARCHAR(255) NOT NULL,
    file_size INT NOT NULL,
    width INT NULL,
    height INT NULL,
    alt_text TEXT NULL,
    is_webp TINYINT(1) DEFAULT 1,
    file_path TEXT NOT NULL,
    url TEXT NOT NULL,
    hash VARCHAR(255) NOT NULL,
    variants JSON NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY idx_media_files_hash (hash),
    KEY idx_media_files_user_id (user_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
