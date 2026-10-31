-- catalog module schema: product commerce data (1:1 with a content item of
-- the configured product post type) plus the variant matrix. Runs in the
-- module's own schema_migrations_catalog table, after the core migrations.
-- CREATE IF NOT EXISTS keeps upgrades idempotent. Prices are integer minor
-- units — never floats.
CREATE TABLE IF NOT EXISTS catalog_products (
    content_id INTEGER PRIMARY KEY,
    price BIGINT NOT NULL DEFAULT 0,
    sale_price BIGINT,
    sku TEXT NOT NULL DEFAULT '',
    track_inventory BOOLEAN NOT NULL DEFAULT TRUE,
    stock INTEGER NOT NULL DEFAULT 0,
    weight_grams INTEGER,
    low_stock_threshold INTEGER,
    options TEXT NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS catalog_variants (
    id BIGSERIAL PRIMARY KEY,
    product_content_id INTEGER NOT NULL,
    options TEXT NOT NULL DEFAULT '{}',
    sku TEXT NOT NULL DEFAULT '',
    price BIGINT,
    stock INTEGER NOT NULL DEFAULT 0,
    weight_grams INTEGER,
    image_url TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (product_content_id) REFERENCES catalog_products(content_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_catalog_variants_product ON catalog_variants(product_content_id);
