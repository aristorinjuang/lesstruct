-- comments module schema: the comments discussion table.
-- Runs in the module's own schema_migrations_comments table, after the core
-- migrations (the content_items and users parent tables come from core
-- 000001). CREATE IF NOT EXISTS keeps upgrades from pre-module installs
-- (whose comments came from core 000001) idempotent.
CREATE TABLE IF NOT EXISTS comments (
    id INT AUTO_INCREMENT PRIMARY KEY,
    content_id INT NOT NULL,
    user_id INT NOT NULL,
    comment TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    KEY idx_comments_content_id (content_id),
    KEY idx_comments_user_id (user_id),
    FOREIGN KEY (content_id) REFERENCES content_items(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
