-- payments module schema: one submitted payment record per proof, keyed
-- to the order it pays. Money is stored in minor units, snapshotted with
-- the submission so a later price edit can never rewrite the proof.
-- Reference is the proof text itself — transfer numbers, gateway
-- transaction identifiers, verification codes — capped and verbatim.
--
-- order_id is indexed but never FK-bound: modules do not declare foreign
-- keys across module boundaries (the schema here is identical on every
-- driver), so a proof's history survives independently of the orders
-- table's lifecycle.
CREATE TABLE IF NOT EXISTS payments_payment (
    id BIGINT NOT NULL AUTO_INCREMENT,
    order_id BIGINT NOT NULL,
    method VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    reference VARCHAR(500) NOT NULL DEFAULT '',
    amount BIGINT NOT NULL DEFAULT 0,
    verified_by VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_payments_payment_order ON payments_payment (order_id);
CREATE INDEX idx_payments_payment_status ON payments_payment (status);
