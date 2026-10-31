-- payments module schema: one submitted payment record per proof, keyed
-- to the order it pays. Money is stored in minor units, snapshotted with
-- the submission so a later price edit can never rewrite the proof.
-- Reference is the proof text itself — transfer numbers, gateway
-- transaction identifiers, verification codes — capped and verbatim.
--
-- order_id is indexed but never FK-bound: modules do not declare foreign
-- keys across module boundaries, so a proof's history survives
-- independently of the orders table's lifecycle.
CREATE TABLE IF NOT EXISTS payments_payment (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id INTEGER NOT NULL,
    method TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    reference TEXT NOT NULL DEFAULT '',
    amount INTEGER NOT NULL DEFAULT 0,
    verified_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_payments_payment_order ON payments_payment(order_id);
CREATE INDEX IF NOT EXISTS idx_payments_payment_status ON payments_payment(status);
