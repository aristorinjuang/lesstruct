-- cart module schema: token-keyed carts with raw quantity lines. Prices
-- are never stored — totals resolve live from the catalog at read time.
-- Runs in the module's own schema_migrations_cart table, after the core
-- migrations. CREATE IF NOT EXISTS keeps upgrades idempotent.
--
-- variant_id is 0 for base-product lines (never NULL) so the line
-- uniqueness constraint behaves identically on every driver.
CREATE TABLE IF NOT EXISTS cart_carts (
    id BIGINT NOT NULL AUTO_INCREMENT,
    token_hash VARCHAR(128) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY idx_cart_carts_token (token_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS cart_items (
    id BIGINT NOT NULL AUTO_INCREMENT,
    cart_id BIGINT NOT NULL,
    product_content_id BIGINT NOT NULL,
    variant_id BIGINT NOT NULL DEFAULT 0,
    quantity INT NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    CONSTRAINT fk_cart_items_cart FOREIGN KEY (cart_id) REFERENCES cart_carts (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_cart_items_cart ON cart_items (cart_id);
CREATE UNIQUE INDEX idx_cart_items_line ON cart_items (cart_id, product_content_id, variant_id);
