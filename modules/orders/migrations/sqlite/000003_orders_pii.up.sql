-- orders 000003: blind index for the encrypted customer email. PII
-- columns (customer_email, customer_name, customer_phone, ship_to) now
-- carry AES-256-GCM ciphertext in the v1: envelope; randomized ciphertext
-- cannot be queried, so exact-match guest tracking resolves through this
-- deterministic HMAC column over the normalized email.
ALTER TABLE orders_order ADD COLUMN customer_email_idx TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_orders_order_email_idx ON orders_order(customer_email_idx);
