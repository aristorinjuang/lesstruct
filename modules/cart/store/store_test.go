package store_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel/database"
	cartdomain "github.com/aristorinjuang/lesstruct/modules/cart/domain"
	"github.com/aristorinjuang/lesstruct/modules/cart/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func setupStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		CREATE TABLE cart_carts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_hash TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE cart_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cart_id INTEGER NOT NULL,
			product_content_id INTEGER NOT NULL,
			variant_id INTEGER NOT NULL DEFAULT 0,
			quantity INTEGER NOT NULL DEFAULT 1,
			FOREIGN KEY (cart_id) REFERENCES cart_carts(id) ON DELETE CASCADE
		);
		CREATE UNIQUE INDEX idx_cart_items_line ON cart_items(cart_id, product_content_id, variant_id);
	`)
	require.NoError(t, err)

	return store.New(db, database.SQLiteDialect())
}

func TestStore_CreateFind_RoundTrip(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	id, err := s.CreateCart(ctx, "hash-1")
	require.NoError(t, err)
	assert.Positive(t, id)

	cart, err := s.FindCart(ctx, "hash-1")
	require.NoError(t, err)
	assert.Equal(t, id, cart.ID)
	assert.Equal(t, "hash-1", cart.TokenHash)
	assert.Empty(t, cart.Lines)
}

func TestStore_FindCart_Unknown(t *testing.T) {
	_, err := setupStore(t).FindCart(context.Background(), "ghost")
	assert.ErrorIs(t, err, cartdomain.ErrCartNotFound)
}

func TestStore_AddLine_Merges(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	id, err := s.CreateCart(ctx, "hash-1")
	require.NoError(t, err)

	first, err := s.AddLine(ctx, id, 7, nil, 2)
	require.NoError(t, err)
	assert.Equal(t, 2, first.Quantity)
	assert.Nil(t, first.VariantID)

	second, err := s.AddLine(ctx, id, 7, nil, 3)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "same (product, variant) key must merge")
	assert.Equal(t, 5, second.Quantity)

	variant, err := s.AddLine(ctx, id, 7, new(int64(3)), 1)
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, variant.ID, "a variant key must not merge with the base line")
	require.NotNil(t, variant.VariantID)
	assert.Equal(t, int64(3), *variant.VariantID)

	cart, err := s.FindCart(ctx, "hash-1")
	require.NoError(t, err)
	require.Len(t, cart.Lines, 2)
}

func TestStore_SetLineQuantity(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	id, err := s.CreateCart(ctx, "hash-1")
	require.NoError(t, err)
	line, err := s.AddLine(ctx, id, 7, nil, 2)
	require.NoError(t, err)

	require.NoError(t, s.SetLineQuantity(ctx, id, line.ID, 9))

	cart, err := s.FindCart(ctx, "hash-1")
	require.NoError(t, err)
	require.Len(t, cart.Lines, 1)
	assert.Equal(t, 9, cart.Lines[0].Quantity)
}

func TestStore_SetLineQuantity_ForeignCart(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	mine, err := s.CreateCart(ctx, "mine")
	require.NoError(t, err)
	line, err := s.AddLine(ctx, mine, 7, nil, 1)
	require.NoError(t, err)

	theirs, err := s.CreateCart(ctx, "theirs")
	require.NoError(t, err)

	assert.ErrorIs(t, s.SetLineQuantity(ctx, theirs, line.ID, 5), cartdomain.ErrLineNotFound)
}

func TestStore_RemoveLine(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	id, err := s.CreateCart(ctx, "hash-1")
	require.NoError(t, err)
	line, err := s.AddLine(ctx, id, 7, nil, 1)
	require.NoError(t, err)

	require.NoError(t, s.RemoveLine(ctx, id, line.ID))

	cart, err := s.FindCart(ctx, "hash-1")
	require.NoError(t, err)
	assert.Empty(t, cart.Lines)

	assert.NoError(t, s.RemoveLine(ctx, id, 9999), "unknown lines are a no-op")
}

func TestStore_DeleteCart(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	id, err := s.CreateCart(ctx, "hash-1")
	require.NoError(t, err)
	_, err = s.AddLine(ctx, id, 7, nil, 1)
	require.NoError(t, err)

	require.NoError(t, s.DeleteCart(ctx, id))

	_, err = s.FindCart(ctx, "hash-1")
	assert.ErrorIs(t, err, cartdomain.ErrCartNotFound)

	// The delete is the checkout claim: an already-deleted (or unknown) cart
	// reports ErrCartNotFound instead of silently succeeding, so a parallel
	// checkout that lost the race reads the cart as gone.
	assert.ErrorIs(t, s.DeleteCart(ctx, 9999), cartdomain.ErrCartNotFound, "unknown carts report ErrCartNotFound")
}
