-- catalog module schema: product commerce data (1:1 with a content item of
-- the configured product post type) plus the variant matrix. Runs in the
-- module's own schema_migrations_catalog table, after the core migrations.
-- CREATE IF NOT EXISTS keeps upgrades idempotent. Prices are integer minor
-- units — never floats.
CREATE TABLE IF NOT EXISTS catalog_products (
    content_id BIGINT NOT NULL,
    price BIGINT NOT NULL DEFAULT 0,
    sale_price BIGINT NULL,
    sku VARCHAR(100) NOT NULL DEFAULT '',
    track_inventory TINYINT(1) NOT NULL DEFAULT 1,
    stock INT NOT NULL DEFAULT 0,
    weight_grams INT NULL,
    low_stock_threshold INT NULL,
    options TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (content_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS catalog_variants (
    id BIGINT NOT NULL AUTO_INCREMENT,
    product_content_id BIGINT NOT NULL,
    options TEXT NOT NULL,
    sku VARCHAR(100) NOT NULL DEFAULT '',
    price BIGINT NULL,
    stock INT NOT NULL DEFAULT 0,
    weight_grams INT NULL,
    image_url VARCHAR(500) NOT NULL DEFAULT '',
    PRIMARY KEY (id),
    CONSTRAINT fk_catalog_variants_product FOREIGN KEY (product_content_id) REFERENCES catalog_products (content_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_catalog_variants_product ON catalog_variants (product_content_id);
