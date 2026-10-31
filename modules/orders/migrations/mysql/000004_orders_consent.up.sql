-- orders 000004: GDPR consent evidence. consent_at records when the buyer
-- affirmed the terms; consent_evidence snapshots the agreed document
-- versions (page ID, last-updated timestamp, slug, language) as JSON. Both
-- stay NULL for orders placed before consent was enforced. TEXT stays
-- nullable: MySQL refuses a default on TEXT columns.
ALTER TABLE orders_order ADD COLUMN consent_at DATETIME NULL;

ALTER TABLE orders_order ADD COLUMN consent_evidence TEXT NULL;
