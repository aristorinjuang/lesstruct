-- orders 000005: per-order tracking secret. Every order carries a random
-- token (stored as its SHA-256 hash); guest tracking and guest payment
-- proofs must present it alongside the order number and email. Legacy
-- orders have an empty hash and become untrackable through the guest gate
-- (accepted trade-off: the old email-only gate was the vulnerability).
ALTER TABLE orders_order ADD COLUMN tracking_token_hash TEXT NOT NULL DEFAULT '';
