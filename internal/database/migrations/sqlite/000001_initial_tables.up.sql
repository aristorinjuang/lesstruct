-- Lesstruct initial schema — all tables, indexes, and constraints
-- This replaces 24 incremental migrations (000001–000024).
-- Fresh installs get the final schema in one step.

-- ---------------------------------------------------------------------------
-- 1. users
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    email TEXT,
    role TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    name TEXT,
    custom_fields TEXT,
    profile_picture TEXT,
    last_login_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email IS NOT NULL;

-- ---------------------------------------------------------------------------
-- 2. content_items
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS content_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    slug TEXT NOT NULL,
    content TEXT,
    tags TEXT,
    status TEXT DEFAULT 'draft',
    post_type TEXT DEFAULT 'post',
    meta_description TEXT DEFAULT NULL,
    og_title TEXT DEFAULT NULL,
    og_description TEXT DEFAULT NULL,
    allow_comments INTEGER NOT NULL DEFAULT 1,
    custom_fields TEXT DEFAULT NULL,
    updated_by INTEGER DEFAULT NULL,
    language TEXT NOT NULL DEFAULT 'en',
    translation_group_id INTEGER DEFAULT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE(slug, language)
);

CREATE INDEX IF NOT EXISTS idx_content_items_slug ON content_items(slug);
CREATE INDEX IF NOT EXISTS idx_content_items_user_id ON content_items(user_id);
CREATE INDEX IF NOT EXISTS idx_content_items_status ON content_items(status);
CREATE INDEX IF NOT EXISTS idx_content_items_post_type ON content_items(post_type);
CREATE INDEX IF NOT EXISTS idx_content_items_language ON content_items(language);
CREATE INDEX IF NOT EXISTS idx_content_items_translation_group ON content_items(translation_group_id);

-- ---------------------------------------------------------------------------
-- 3. comments
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    comment TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (content_id) REFERENCES content_items(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_comments_content_id ON comments(content_id);
CREATE INDEX IF NOT EXISTS idx_comments_user_id ON comments(user_id);

-- ---------------------------------------------------------------------------
-- 4. media_files
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS media_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
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
    variants TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_media_files_user_id ON media_files(user_id);
CREATE INDEX IF NOT EXISTS idx_media_files_hash ON media_files(hash);

-- ---------------------------------------------------------------------------
-- 5. blocked_emails
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS blocked_emails (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    reason TEXT NOT NULL DEFAULT 'marked_as_spam'
);

CREATE INDEX IF NOT EXISTS idx_blocked_emails_email ON blocked_emails(email);

-- ---------------------------------------------------------------------------
-- 6. failed_login_attempts
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS failed_login_attempts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at DATETIME NOT NULL,
    locked_until DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_failed_login_attempts_user_id ON failed_login_attempts(user_id);
CREATE INDEX IF NOT EXISTS idx_failed_login_attempts_locked_until ON failed_login_attempts(locked_until);

-- ---------------------------------------------------------------------------
-- 7. verification_tokens
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS verification_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_verification_tokens_user_id ON verification_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_verification_tokens_token_hash ON verification_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_verification_tokens_expires_at ON verification_tokens(expires_at);

-- ---------------------------------------------------------------------------
-- 8. password_reset_tokens
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token_hash ON password_reset_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires_at ON password_reset_tokens(expires_at);

-- ---------------------------------------------------------------------------
-- 9. email_update_tokens
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS email_update_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL,
    new_email TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_email_update_tokens_token ON email_update_tokens(token);
CREATE INDEX IF NOT EXISTS idx_email_update_tokens_user ON email_update_tokens(user_id);

-- ---------------------------------------------------------------------------
-- 10. soft_deleted_content
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS soft_deleted_content (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content_type TEXT NOT NULL,
    content_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    deleted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_by INTEGER NOT NULL,
    reason TEXT,
    is_permanent BOOLEAN DEFAULT 0,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (deleted_by) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_soft_deleted_content_lookup ON soft_deleted_content(content_type, content_id);
CREATE INDEX IF NOT EXISTS idx_soft_deleted_content_user ON soft_deleted_content(user_id);
