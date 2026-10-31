-- cart module schema: token-keyed carts with raw quantity lines. Prices
-- are never stored — totals resolve live from the catalog at read time.
-- Runs in the module's own schema_migrations_cart table, after the core
-- migrations. CREATE IF NOT EXISTS keeps upgrades idempotent.
--
-- variant_id is 0 for base-product lines (never NULL) so the line
-- uniqueness constraint behaves identically on every driver.
CREATE TABLE IF NOT EXISTS cart_carts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token_hash TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cart_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cart_id INTEGER NOT NULL,
    product_content_id INTEGER NOT NULL,
    variant_id INTEGER NOT NULL DEFAULT 0,
    quantity INTEGER NOT NULL DEFAULT 1,
    FOREIGN KEY (cart_id) REFERENCES cart_carts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_cart_items_cart ON cart_items(cart_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cart_items_line ON cart_items(cart_id, product_content_id, variant_id);
