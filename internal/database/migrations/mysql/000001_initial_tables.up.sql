-- Lesstruct initial schema — MySQL version
-- All tables, indexes, and constraints translated from PostgreSQL.
-- Fresh installs get the final schema in one step.

CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(255) NOT NULL,
    password_hash TEXT NOT NULL,
    email VARCHAR(255) NULL,
    role VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    name VARCHAR(255) NULL,
    custom_fields JSON NULL,
    profile_picture TEXT NULL,
    last_login_at DATETIME NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY idx_users_username (username),
    KEY idx_users_status (status),
    UNIQUE KEY idx_users_email (email)
);

CREATE TABLE IF NOT EXISTS content_items (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    title TEXT NOT NULL,
    slug VARCHAR(255) NOT NULL,
    content LONGTEXT NULL,
    tags JSON NULL,
    status VARCHAR(50) DEFAULT 'draft',
    post_type VARCHAR(50) DEFAULT 'post',
    meta_description TEXT NULL,
    og_title TEXT NULL,
    og_description TEXT NULL,
    allow_comments TINYINT(1) NOT NULL DEFAULT 1,
    custom_fields JSON NULL,
    updated_by INT NULL,
    language VARCHAR(50) NOT NULL DEFAULT 'en',
    translation_group_id INT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY idx_content_items_slug_language (slug, language),
    KEY idx_content_items_slug (slug),
    KEY idx_content_items_user_id (user_id),
    KEY idx_content_items_status (status),
    KEY idx_content_items_post_type (post_type),
    KEY idx_content_items_language (language),
    KEY idx_content_items_translation_group (translation_group_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);

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

CREATE TABLE IF NOT EXISTS blocked_emails (
    id INT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    reason TEXT NOT NULL,
    UNIQUE KEY idx_blocked_emails_email (email)
);

CREATE TABLE IF NOT EXISTS failed_login_attempts (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    last_attempt_at DATETIME NOT NULL,
    locked_until DATETIME NULL,
    last_email_sent_at DATETIME NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY idx_failed_login_attempts_user_id (user_id),
    KEY idx_failed_login_attempts_locked_until (locked_until),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS verification_tokens (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    token_hash VARCHAR(255) NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY idx_verification_tokens_token_hash (token_hash),
    KEY idx_verification_tokens_user_id (user_id),
    KEY idx_verification_tokens_expires_at (expires_at),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    token_hash VARCHAR(255) NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY idx_password_reset_tokens_token_hash (token_hash),
    KEY idx_password_reset_tokens_user_id (user_id),
    KEY idx_password_reset_tokens_expires_at (expires_at),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS email_update_tokens (
    id INT AUTO_INCREMENT PRIMARY KEY,
    token VARCHAR(255) NOT NULL,
    user_id INT NOT NULL,
    new_email TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY idx_email_update_tokens_token (token),
    KEY idx_email_update_tokens_user (user_id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS soft_deleted_content (
    id INT AUTO_INCREMENT PRIMARY KEY,
    content_type VARCHAR(255) NOT NULL,
    content_id INT NOT NULL,
    user_id INT NOT NULL,
    deleted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_by INT NOT NULL,
    reason TEXT NULL,
    is_permanent TINYINT(1) DEFAULT 0,
    KEY idx_soft_deleted_content_lookup (content_type, content_id),
    KEY idx_soft_deleted_content_user (user_id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (deleted_by) REFERENCES users(id)
);
