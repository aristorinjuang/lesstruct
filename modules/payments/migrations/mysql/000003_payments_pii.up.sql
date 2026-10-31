-- payments 000003: blind index for the encrypted reference. The
-- reference and verified_by columns now carry AES-256-GCM ciphertext in
-- the v1: envelope; randomized ciphertext cannot be queried, so the
-- gateway webhook dedup resolves through this deterministic HMAC column
-- over the transaction reference.
ALTER TABLE payments_payment ADD COLUMN reference_idx VARCHAR(128) NOT NULL DEFAULT '';

CREATE INDEX idx_payments_payment_reference_idx ON payments_payment (reference_idx);
