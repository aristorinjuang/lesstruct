-- API keys (Epic 1, Story 1.1) — MySQL
CREATE TABLE IF NOT EXISTS api_keys (
    id            INT AUTO_INCREMENT PRIMARY KEY,
    user_id       INT NOT NULL,
    name          VARCHAR(120) NOT NULL,
    key_id        VARCHAR(24) NOT NULL,
    key_hash      VARCHAR(64) NOT NULL,
    last_used_at  DATETIME NULL,
    last_used_ip  VARCHAR(45) NULL,
    expires_at    DATETIME NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at    DATETIME NULL,
    -- Per-user unique name among active (non-revoked) keys — DB backstop for AC9.
    -- MySQL has no partial/filtered indexes, so a VIRTUAL generated column that
    -- is non-NULL only for active keys emulates it (UNIQUE ignores NULLs → revoked
    -- names stay reusable). Derived, not persisted. MySQL 5.7+.
    active_name   VARCHAR(120) GENERATED ALWAYS AS (CASE WHEN revoked_at IS NULL THEN name ELSE NULL END) VIRTUAL,
    UNIQUE KEY idx_api_keys_key_id (key_id),
    UNIQUE KEY idx_api_keys_user_name (user_id, active_name),
    KEY idx_api_keys_user_id (user_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
