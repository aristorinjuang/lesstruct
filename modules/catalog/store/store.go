// Package store is the single-source catalog repository: one implementation
// against *sql.DB plus a kernel Dialect, following the comments/media store
// pattern. Upsert rendering and placeholder differences come from the
// dialect; everything else (columns, scans, messages) is shared.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
)

// productRow mirrors the catalog_products columns; NULLable numeric and
// text columns scan as sql.Null types and map to pointer fields.
type productRow struct {
	Price             int64
	SalePrice         sql.NullInt64
	SKU               string
	TrackInventory    bool
	Stock             int
	WeightGrams       sql.NullInt64
	LowStockThreshold sql.NullInt64
	Options           string
	UpdatedAt         time.Time
}

// variantRow mirrors the catalog_variants columns.
type variantRow struct {
	ID          int64
	Options     string
	SKU         string
	Price       sql.NullInt64
	Stock       int
	WeightGrams sql.NullInt64
	ImageURL    string
}

// Store is the catalog single-source store.
type Store struct {
	db      *sql.DB
	dialect kerneldatabase.Dialect
}

// GetProduct returns the product with its full variant matrix, or
// catalogdomain.ErrProductNotFound when the content item has no row.
func (s *Store) GetProduct(ctx context.Context, contentID int) (*catalogdomain.Product, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var row productRow
	var variants []variantRow

	err := s.db.QueryRowContext(ctx, `
		SELECT price, sale_price, sku, track_inventory, stock, weight_grams, low_stock_threshold, options, updated_at
		FROM catalog_products
		WHERE content_id = `+s.dialect.Placeholder(0)+`
	`, contentID).Scan(
		&row.Price,
		&row.SalePrice,
		&row.SKU,
		&row.TrackInventory,
		&row.Stock,
		&row.WeightGrams,
		&row.LowStockThreshold,
		&row.Options,
		&row.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, catalogdomain.ErrProductNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get catalog product: %w", err)
	}

	variantRows, err := s.db.QueryContext(ctx, `
		SELECT id, options, sku, price, stock, weight_grams, image_url
		FROM catalog_variants
		WHERE product_content_id = `+s.dialect.Placeholder(0)+`
		ORDER BY id
	`, contentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get catalog variants: %w", err)
	}
	defer func() { _ = variantRows.Close() }()

	for variantRows.Next() {
		var v variantRow
		if err := variantRows.Scan(&v.ID, &v.Options, &v.SKU, &v.Price, &v.Stock, &v.WeightGrams, &v.ImageURL); err != nil {
			return nil, fmt.Errorf("failed to scan catalog variant: %w", err)
		}
		variants = append(variants, v)
	}
	if err := variantRows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating catalog variants: %w", err)
	}

	return mapProductToDomain(row, variants, contentID)
}

// UpsertProduct writes the product row (insert or update via the dialect's
// upsert) and replaces the variant matrix wholesale, atomically.
func (s *Store) UpsertProduct(ctx context.Context, contentID int, product *catalogdomain.Product) error {
	if ctx == nil {
		ctx = context.Background()
	}

	optionsJSON, err := json.Marshal(product.Options)
	if err != nil {
		return fmt.Errorf("failed to marshal catalog options: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin catalog transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	upsert, err := s.dialect.Upsert(
		"catalog_products",
		[]string{"content_id"},
		[]string{"price", "sale_price", "sku", "track_inventory", "stock", "weight_grams", "low_stock_threshold", "options", "updated_at"},
	)
	if err != nil {
		return fmt.Errorf("failed to render catalog upsert: %w", err)
	}

	if _, err := tx.ExecContext(ctx, upsert,
		contentID,
		product.Price,
		nullInt64(product.SalePrice),
		product.SKU,
		product.TrackInventory,
		product.Stock,
		nullInt64FromInt(product.WeightGrams),
		nullInt64FromInt(product.LowStockThreshold),
		string(optionsJSON),
		now,
	); err != nil {
		return fmt.Errorf("failed to upsert catalog product: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM catalog_variants WHERE product_content_id = `+s.dialect.Placeholder(0), contentID); err != nil {
		return fmt.Errorf("failed to clear catalog variants: %w", err)
	}

	for _, variant := range product.Variants {
		variantOptions, err := json.Marshal(variant.Options)
		if err != nil {
			return fmt.Errorf("failed to marshal catalog variant options: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO catalog_variants (product_content_id, options, sku, price, stock, weight_grams, image_url)
			VALUES (`+s.dialect.Placeholder(0)+`, `+s.dialect.Placeholder(1)+`, `+s.dialect.Placeholder(2)+`, `+s.dialect.Placeholder(3)+`, `+s.dialect.Placeholder(4)+`, `+s.dialect.Placeholder(5)+`, `+s.dialect.Placeholder(6)+`)
		`, contentID, string(variantOptions), variant.SKU, nullInt64(variant.Price), variant.Stock, nullInt64FromInt(variant.WeightGrams), variant.ImageURL); err != nil {
			return fmt.Errorf("failed to insert catalog variant: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit catalog transaction: %w", err)
	}
	return nil
}

// AdjustStock applies the signed delta to one tracked stock bucket with
// the condition carried in the statement itself: the row-level guard
// `stock + delta >= 0` makes the read-modify-write atomic, so two
// concurrent adjustments serialize at the row and the loser reads zero
// rows. Reports whether the row changed; a zero report means the bucket
// vanished or the move fell below zero (the service classifies).
func (s *Store) AdjustStock(ctx context.Context, contentID int, variantID *int64, delta int) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var query string
	var args []any
	if variantID != nil {
		query = `UPDATE catalog_variants
			SET stock = stock + ` + s.dialect.Placeholder(0) + `
			WHERE id = ` + s.dialect.Placeholder(1) + `
			  AND product_content_id = ` + s.dialect.Placeholder(2) + `
			  AND stock + ` + s.dialect.Placeholder(3) + ` >= 0`
		args = []any{delta, *variantID, contentID, delta}
	} else {
		query = `UPDATE catalog_products
			SET stock = stock + ` + s.dialect.Placeholder(0) + `, updated_at = ` + s.dialect.Placeholder(1) + `
			WHERE content_id = ` + s.dialect.Placeholder(2) + `
			  AND track_inventory
			  AND stock + ` + s.dialect.Placeholder(3) + ` >= 0`
		args = []any{delta, time.Now().UTC(), contentID, delta}
	}

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("failed to adjust catalog stock: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to read catalog stock adjustment: %w", err)
	}
	return changed > 0, nil
}

// New builds the store over an open database and its dialect.
func New(db *sql.DB, dialect kerneldatabase.Dialect) *Store {
	return &Store{db: db, dialect: dialect}
}
