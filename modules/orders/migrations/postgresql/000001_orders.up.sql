-- orders module schema: first-class orders snapshotted at checkout.
-- Money is stored in minor units at purchase time (never live pricing);
-- product titles and SKUs freeze with the order. The display order
-- number derives from the ID, so nothing extra is stored.
--
-- variant_id is 0 for base-product lines (never NULL) so the schema
-- behaves identically on every driver. Ship_to is a JSON snapshot:
-- structured for packing labels, never a rates integration.
CREATE TABLE IF NOT EXISTS orders_order (
    id BIGSERIAL PRIMARY KEY,
    status TEXT NOT NULL DEFAULT 'pending',
    customer_email TEXT NOT NULL,
    customer_name TEXT NOT NULL,
    customer_phone TEXT NOT NULL DEFAULT '',
    ship_to JSONB NOT NULL DEFAULT '{}',
    currency TEXT NOT NULL DEFAULT '',
    subtotal BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders_order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL,
    product_content_id INTEGER NOT NULL,
    variant_id BIGINT NOT NULL DEFAULT 0,
    sku TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    unit_price BIGINT NOT NULL DEFAULT 0,
    quantity INTEGER NOT NULL DEFAULT 1,
    FOREIGN KEY (order_id) REFERENCES orders_order(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_orders_order_items_order ON orders_order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_orders_order_status ON orders_order(status);
