// Package store is the single-source cart repository: one implementation
// against *sql.DB plus a kernel Dialect, following the comments/media
// store pattern. Placeholder rendering and RETURNING support come from the
// dialect. Variant IDs map nil (base-product line) to 0 at the boundary so
// the line uniqueness constraint behaves identically on every driver.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
	cartdomain "github.com/aristorinjuang/lesstruct/modules/cart/domain"
)

// noVariantID is the stored variant key for base-product lines.
const (
	noVariantID = 0
)

// variantKey renders the stored variant key: 0 for base-product lines.
func variantKey(variantID *int64) int64 {
	if variantID == nil {
		return noVariantID
	}
	return *variantID
}

// sameVariant reports whether two variant keys match, treating nil and
// non-nil as distinct.
func sameVariant(a, b *int64) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || *a == *b
}

// Store is the cart single-source store.
type Store struct {
	db      *sql.DB
	dialect kerneldatabase.Dialect
}

// placeholders returns a positional placeholder generator: each call renders
// the next bind marker ($1, $2… on PostgreSQL; ? elsewhere), so call order
// matches argument order by construction.
func (s *Store) placeholders() func() string {
	index := 0
	return func() string {
		marker := s.dialect.Placeholder(index)
		index++
		return marker
	}
}

// CreateCart persists an empty cart keyed by token hash and returns its ID.
func (s *Store) CreateCart(ctx context.Context, tokenHash string) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	next := s.placeholders()
	if s.dialect.SupportsReturning() {
		var id int64
		err := s.db.QueryRowContext(ctx, `
			INSERT INTO cart_carts (token_hash) VALUES (`+next()+`)
			RETURNING id
		`, tokenHash).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("failed to create cart: %w", err)
		}
		return id, nil
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO cart_carts (token_hash) VALUES (`+next()+`)
	`, tokenHash)
	if err != nil {
		return 0, fmt.Errorf("failed to create cart: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to read cart id: %w", err)
	}
	return id, nil
}

// FindCart returns the cart with its raw lines in id order, or
// cartdomain.ErrCartNotFound when the token hash matches no cart.
func (s *Store) FindCart(ctx context.Context, tokenHash string) (*cartdomain.StoredCart, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	next := s.placeholders()
	var cart cartdomain.StoredCart
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM cart_carts WHERE token_hash = `+next()+`
	`, tokenHash).Scan(&cart.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, cartdomain.ErrCartNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find cart: %w", err)
	}
	cart.TokenHash = tokenHash

	lines, err := s.db.QueryContext(ctx, `
		SELECT id, product_content_id, variant_id, quantity
		FROM cart_items WHERE cart_id = `+s.dialect.Placeholder(0)+`
		ORDER BY id
	`, cart.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list cart lines: %w", err)
	}
	defer func() { _ = lines.Close() }()

	for lines.Next() {
		var line cartdomain.StoredLine
		var variantID int64
		if err := lines.Scan(&line.ID, &line.ProductContentID, &variantID, &line.Quantity); err != nil {
			return nil, fmt.Errorf("failed to scan cart line: %w", err)
		}
		line.CartID = cart.ID
		if variantID != noVariantID {
			variant := variantID
			line.VariantID = &variant
		}
		cart.Lines = append(cart.Lines, line)
	}
	if err := lines.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating cart lines: %w", err)
	}

	return &cart, nil
}

// AddLine appends a line, merging with an existing line on the same
// (product, variant) key by incrementing its quantity. A duplicate-key hit
// from a concurrent add retries once against the fresh row set.
func (s *Store) AddLine(ctx context.Context, cartID int64, productContentID int, variantID *int64, quantity int) (cartdomain.StoredLine, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	line, err := s.addLineOnce(ctx, cartID, productContentID, variantID, quantity)
	if err != nil && s.dialect.IsDuplicateError(err) {
		line, err = s.addLineOnce(ctx, cartID, productContentID, variantID, quantity)
	}
	if err != nil {
		return cartdomain.StoredLine{}, err
	}
	return line, nil
}

// SetLineQuantity replaces a line's quantity; the line must belong to the
// cart or cartdomain.ErrLineNotFound results.
func (s *Store) SetLineQuantity(ctx context.Context, cartID int64, itemID int64, quantity int) error {
	if ctx == nil {
		ctx = context.Background()
	}

	next := s.placeholders()
	result, err := s.db.ExecContext(ctx, `
		UPDATE cart_items SET quantity = `+next()+`
		WHERE id = `+next()+` AND cart_id = `+next()+`
	`, quantity, itemID, cartID)
	if err != nil {
		return fmt.Errorf("failed to update cart line: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read cart line update: %w", err)
	}
	if affected == 0 {
		return cartdomain.ErrLineNotFound
	}
	return nil
}

// RemoveLine deletes one line of the cart; unknown lines are a no-op.
func (s *Store) RemoveLine(ctx context.Context, cartID int64, itemID int64) error {
	if ctx == nil {
		ctx = context.Background()
	}

	next := s.placeholders()
	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM cart_items WHERE id = `+next()+` AND cart_id = `+next()+`
	`, itemID, cartID); err != nil {
		return fmt.Errorf("failed to remove cart line: %w", err)
	}
	return nil
}

// DeleteCart removes the cart; unknown carts report cartdomain.ErrCartNotFound.
// The delete is the checkout claim: reporting the affected-row count is what
// makes the claim atomic, so of any parallel checkouts on one token exactly
// one proceeds and the losers read the cart as gone instead of silently
// double-deleting a row that is already gone.
func (s *Store) DeleteCart(ctx context.Context, cartID int64) error {
	if ctx == nil {
		ctx = context.Background()
	}

	next := s.placeholders()
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM cart_carts WHERE id = `+next()+`
	`, cartID)
	if err != nil {
		return fmt.Errorf("failed to delete cart: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read cart delete: %w", err)
	}
	if affected == 0 {
		return cartdomain.ErrCartNotFound
	}
	return nil
}

// addLineOnce runs one merge-or-insert attempt inside a transaction.
func (s *Store) addLineOnce(ctx context.Context, cartID int64, productContentID int, variantID *int64, quantity int) (cartdomain.StoredLine, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return cartdomain.StoredLine{}, fmt.Errorf("failed to begin cart transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, product_content_id, variant_id, quantity
		FROM cart_items WHERE cart_id = `+s.dialect.Placeholder(0)+`
	`, cartID)
	if err != nil {
		return cartdomain.StoredLine{}, fmt.Errorf("failed to list cart lines: %w", err)
	}
	var existing *cartdomain.StoredLine
	for rows.Next() {
		var line cartdomain.StoredLine
		var variant int64
		if err := rows.Scan(&line.ID, &line.ProductContentID, &variant, &line.Quantity); err != nil {
			_ = rows.Close()
			return cartdomain.StoredLine{}, fmt.Errorf("failed to scan cart line: %w", err)
		}
		line.CartID = cartID
		if variant != noVariantID {
			v := variant
			line.VariantID = &v
		}
		if line.ProductContentID == productContentID && sameVariant(line.VariantID, variantID) {
			found := line
			existing = &found
		}
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return cartdomain.StoredLine{}, fmt.Errorf("failed iterating cart lines: %w", err)
	}

	next := s.placeholders()
	storedVariant := variantKey(variantID)
	if existing != nil {
		if _, err := tx.ExecContext(ctx, `
			UPDATE cart_items SET quantity = quantity + `+next()+`
			WHERE id = `+next()+`
		`, quantity, existing.ID); err != nil {
			return cartdomain.StoredLine{}, fmt.Errorf("failed to merge cart line: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return cartdomain.StoredLine{}, fmt.Errorf("failed to commit cart transaction: %w", err)
		}
		merged := *existing
		merged.Quantity += quantity
		return merged, nil
	}

	var id int64
	if s.dialect.SupportsReturning() {
		err := tx.QueryRowContext(ctx, `
			INSERT INTO cart_items (cart_id, product_content_id, variant_id, quantity)
			VALUES (`+next()+`, `+next()+`, `+next()+`, `+next()+`)
			RETURNING id
		`, cartID, productContentID, storedVariant, quantity).Scan(&id)
		if err != nil {
			return cartdomain.StoredLine{}, fmt.Errorf("failed to insert cart line: %w", err)
		}
	} else {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO cart_items (cart_id, product_content_id, variant_id, quantity)
			VALUES (`+next()+`, `+next()+`, `+next()+`, `+next()+`)
		`, cartID, productContentID, storedVariant, quantity)
		if err != nil {
			return cartdomain.StoredLine{}, fmt.Errorf("failed to insert cart line: %w", err)
		}
		id, err = result.LastInsertId()
		if err != nil {
			return cartdomain.StoredLine{}, fmt.Errorf("failed to read cart line id: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return cartdomain.StoredLine{}, fmt.Errorf("failed to commit cart transaction: %w", err)
	}
	return cartdomain.StoredLine{ID: id, CartID: cartID, ProductContentID: productContentID, VariantID: variantID, Quantity: quantity}, nil
}

// New builds the store over an open database and its dialect.
func New(db *sql.DB, dialect kerneldatabase.Dialect) *Store {
	return &Store{db: db, dialect: dialect}
}
