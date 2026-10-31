-- The lockout notifier tracks when the last lockout email was sent so reminder
-- sends respect the cooldown. PostgreSQL and MySQL carried this column from
-- their initial schema; SQLite did not, so every read of a failed-login record
-- failed with "no such column", the attempt was never recorded, lockout never
-- engaged, and bad passwords for existing users answered 500 instead of 401.
ALTER TABLE failed_login_attempts ADD COLUMN last_email_sent_at DATETIME;
