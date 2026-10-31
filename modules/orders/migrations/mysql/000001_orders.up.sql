-- orders module schema: first-class orders snapshotted at checkout.
-- Money is stored in minor units at purchase time (never live pricing);
-- product titles and SKUs freeze with the order. The display order
-- number derives from the ID, so nothing extra is stored.
--
-- variant_id is 0 for base-product lines (never NULL) so the schema
-- behaves identically on every driver. Ship_to is a JSON snapshot:
-- structured for packing labels, never a rates integration.
CREATE TABLE IF NOT EXISTS orders_order (
    id BIGINT NOT NULL AUTO_INCREMENT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    customer_email VARCHAR(254) NOT NULL,
    customer_name VARCHAR(255) NOT NULL,
    customer_phone VARCHAR(32) NOT NULL DEFAULT '',
    ship_to JSON NOT NULL,
    currency VARCHAR(8) NOT NULL DEFAULT '',
    subtotal BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS orders_order_items (
    id BIGINT NOT NULL AUTO_INCREMENT,
    order_id BIGINT NOT NULL,
    product_content_id BIGINT NOT NULL,
    variant_id BIGINT NOT NULL DEFAULT 0,
    sku VARCHAR(128) NOT NULL DEFAULT '',
    title VARCHAR(255) NOT NULL DEFAULT '',
    unit_price BIGINT NOT NULL DEFAULT 0,
    quantity INT NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    CONSTRAINT fk_orders_order_items_order FOREIGN KEY (order_id) REFERENCES orders_order (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_orders_order_items_order ON orders_order_items (order_id);
CREATE INDEX idx_orders_order_status ON orders_order (status);
