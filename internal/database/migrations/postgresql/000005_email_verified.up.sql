-- Track whether the registrant's email address has been proven by clicking the
-- verification link. Distinct from users.status: a "pending" user may already
-- have email_verified = true while awaiting admin approval.
ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT FALSE;

-- Backfill: users who are already "verified" have proven their email.
UPDATE users SET email_verified = TRUE WHERE status = 'verified';