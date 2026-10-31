-- Session revocation. Every JWT carries the user's token_version; the auth
-- middleware compares it with this column on each authenticated request. A
-- password change or reset bumps the column, which invalidates every token
-- issued before the change instead of leaving it valid until its 24h expiry.
ALTER TABLE users ADD COLUMN token_version INTEGER NOT NULL DEFAULT 0;
