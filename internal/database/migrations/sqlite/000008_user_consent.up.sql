-- users 000008: GDPR consent evidence captured at self-registration.
-- consent_at records when the registrant affirmed the terms; consent_evidence
-- snapshots the agreed document versions (page ID, last-updated timestamp,
-- slug, language) as JSON. Both stay NULL for accounts created before consent
-- was required and for accounts an administrator creates.
ALTER TABLE users ADD COLUMN consent_at DATETIME;

ALTER TABLE users ADD COLUMN consent_evidence TEXT;
