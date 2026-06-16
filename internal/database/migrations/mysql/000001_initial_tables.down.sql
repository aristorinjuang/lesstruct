-- Reverse of 000001_initial_tables.up.sql — drop all tables.
DROP TABLE IF EXISTS soft_deleted_content;
DROP TABLE IF EXISTS email_update_tokens;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS verification_tokens;
DROP TABLE IF EXISTS failed_login_attempts;
DROP TABLE IF EXISTS blocked_emails;
DROP TABLE IF EXISTS media_files;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS content_items;
DROP TABLE IF EXISTS users;
