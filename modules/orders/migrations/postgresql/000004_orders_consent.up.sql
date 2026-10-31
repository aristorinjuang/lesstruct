-- orders 000004: GDPR consent evidence. consent_at records when the buyer
-- affirmed the terms; consent_evidence snapshots the agreed document
-- versions (page ID, last-updated timestamp, slug, language) as JSON. Both
-- stay NULL for orders placed before consent was enforced.
ALTER TABLE orders_order ADD COLUMN consent_at TIMESTAMP;

ALTER TABLE orders_order ADD COLUMN consent_evidence TEXT;
