package store_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel/database"
	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
	"github.com/aristorinjuang/lesstruct/modules/catalog/store"
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
		CREATE TABLE catalog_products (
			content_id INTEGER PRIMARY KEY,
			price INTEGER NOT NULL DEFAULT 0,
			sale_price INTEGER,
			sku TEXT NOT NULL DEFAULT '',
			track_inventory INTEGER NOT NULL DEFAULT 1,
			stock INTEGER NOT NULL DEFAULT 0,
			weight_grams INTEGER,
			low_stock_threshold INTEGER,
			options TEXT NOT NULL DEFAULT '{}',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE catalog_variants (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			product_content_id INTEGER NOT NULL,
			options TEXT NOT NULL DEFAULT '{}',
			sku TEXT NOT NULL DEFAULT '',
			price INTEGER,
			stock INTEGER NOT NULL DEFAULT 0,
			weight_grams INTEGER,
			image_url TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (product_content_id) REFERENCES catalog_products(content_id) ON DELETE CASCADE
		);
	`)
	require.NoError(t, err)

	dialect := database.SQLiteDialect()

	return store.New(db, dialect)
}

func sampleProduct() *catalogdomain.Product {
	return &catalogdomain.Product{
		ContentID:         7,
		Price:             299000,
		SKU:               "TSHIRT",
		TrackInventory:    true,
		Stock:             12,
		Options:           map[string][]string{"size": {"S", "M"}},
		LowStockThreshold: new(3),
		Variants: []catalogdomain.Variant{
			{Options: map[string]string{"size": "S"}, SKU: "TSHIRT-S", Stock: 4},
			{Options: map[string]string{"size": "M"}, SKU: "TSHIRT-M", Stock: 8, Price: new(int64(319000)), ImageURL: "/uploads/media/m.webp"},
		},
	}
}

func TestStore_UpsertGet_RoundTrip(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	// Get before any write: not found.
	_, err := s.GetProduct(ctx, 7)
	assert.ErrorIs(t, err, catalogdomain.ErrProductNotFound)

	require.NoError(t, s.UpsertProduct(ctx, 7, sampleProduct()))

	product, err := s.GetProduct(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, int64(299000), product.Price)
	assert.Equal(t, "TSHIRT", product.SKU)
	assert.True(t, product.TrackInventory)
	assert.Equal(t, 12, product.Stock)
	require.NotNil(t, product.LowStockThreshold)
	assert.Equal(t, 3, *product.LowStockThreshold)
	assert.Nil(t, product.SalePrice)
	assert.Nil(t, product.WeightGrams)
	assert.Equal(t, map[string][]string{"size": {"S", "M"}}, product.Options)

	require.Len(t, product.Variants, 2)
	assert.NotZero(t, product.Variants[0].ID)
	assert.Equal(t, map[string]string{"size": "S"}, product.Variants[0].Options)
	assert.Equal(t, "TSHIRT-S", product.Variants[0].SKU)
	assert.Nil(t, product.Variants[0].Price)
	require.NotNil(t, product.Variants[1].Price)
	assert.Equal(t, int64(319000), *product.Variants[1].Price)
	assert.Equal(t, "/uploads/media/m.webp", product.Variants[1].ImageURL)
}

func TestStore_Upsert_ReplacesMatrixAndUpdates(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	require.NoError(t, s.UpsertProduct(ctx, 7, sampleProduct()))

	updated := sampleProduct()
	updated.Price = 199000
	updated.SalePrice = new(int64(149000))
	updated.WeightGrams = new(180)
	updated.TrackInventory = false
	updated.Variants = []catalogdomain.Variant{
		{Options: map[string]string{"size": "M"}, SKU: "TSHIRT-M-ONLY", Stock: 1},
	}
	require.NoError(t, s.UpsertProduct(ctx, 7, updated))

	product, err := s.GetProduct(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, int64(199000), product.Price)
	require.NotNil(t, product.SalePrice)
	assert.Equal(t, int64(149000), *product.SalePrice)
	require.NotNil(t, product.WeightGrams)
	assert.Equal(t, 180, *product.WeightGrams)
	assert.False(t, product.TrackInventory)

	// The matrix was replaced wholesale: one variant, new SKU, new ID.
	require.Len(t, product.Variants, 1)
	assert.Equal(t, "TSHIRT-M-ONLY", product.Variants[0].SKU)
}

func TestStore_Upsert_NilablesRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	product := &catalogdomain.Product{
		ContentID:      9,
		Price:          5000,
		TrackInventory: false,
		Options:        map[string][]string{"color": {"Red"}},
	}
	require.NoError(t, s.UpsertProduct(ctx, 9, product))

	got, err := s.GetProduct(ctx, 9)
	require.NoError(t, err)
	assert.Nil(t, got.SalePrice)
	assert.Nil(t, got.WeightGrams)
	assert.Nil(t, got.LowStockThreshold)
	assert.Equal(t, "", got.SKU)
}

func TestStore_AdjustStock(t *testing.T) {
	tests := []struct {
		name      string
		contentID int
		variantID *int64
		delta     int
		seed      func(*store.Store)
		want      bool
		wantStock *int // product-level stock after the move
	}{
		{
			name:      "product bucket decrements atomically",
			contentID: 7,
			delta:     -5,
			seed: func(s *store.Store) {
				require.NoError(t, s.UpsertProduct(context.Background(), 7, sampleProduct()))
			},
			want:      true,
			wantStock: new(7),
		},
		{
			name:      "move below the floor leaves the bucket untouched",
			contentID: 7,
			delta:     -13,
			seed: func(s *store.Store) {
				require.NoError(t, s.UpsertProduct(context.Background(), 7, sampleProduct()))
			},
			want:      false,
			wantStock: new(12),
		},
		{
			name:      "restock moves the bucket up",
			contentID: 7,
			delta:     3,
			seed: func(s *store.Store) {
				require.NoError(t, s.UpsertProduct(context.Background(), 7, sampleProduct()))
			},
			want:      true,
			wantStock: new(15),
		},
		{
			name:      "untracked bucket reports unchanged",
			contentID: 9,
			delta:     -1,
			seed: func(s *store.Store) {
				require.NoError(t, s.UpsertProduct(context.Background(), 9, &catalogdomain.Product{
					ContentID:      9,
					Price:          5000,
					TrackInventory: false,
					Stock:          4,
					Options:        map[string][]string{"color": {"Red"}},
				}))
			},
			want: false,
		},
		{
			name:      "missing product reports unchanged",
			contentID: 99,
			delta:     -1,
			seed:      func(*store.Store) {},
			want:      false,
		},
		{
			name:      "variant bucket decrements by id",
			contentID: 7,
			variantID: new(int64(1)),
			delta:     -3,
			seed: func(s *store.Store) {
				require.NoError(t, s.UpsertProduct(context.Background(), 7, sampleProduct()))
			},
			want: true,
		},
		{
			name:      "variant move below the floor reports unchanged",
			contentID: 7,
			variantID: new(int64(1)),
			delta:     -5,
			seed: func(s *store.Store) {
				require.NoError(t, s.UpsertProduct(context.Background(), 7, sampleProduct()))
			},
			want: false,
		},
		{
			name:      "unknown variant id reports unchanged",
			contentID: 7,
			variantID: new(int64(999)),
			delta:     -1,
			seed: func(s *store.Store) {
				require.NoError(t, s.UpsertProduct(context.Background(), 7, sampleProduct()))
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			s := setupStore(t)
			tt.seed(s)

			got, err := s.AdjustStock(ctx, tt.contentID, tt.variantID, tt.delta)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)

			if tt.wantStock != nil {
				product, err := s.GetProduct(ctx, tt.contentID)
				require.NoError(t, err)
				assert.Equal(t, *tt.wantStock, product.Stock)
			}
			if tt.variantID != nil && tt.want {
				product, err := s.GetProduct(ctx, tt.contentID)
				require.NoError(t, err)
				require.Len(t, product.Variants, 2)
				assert.Equal(t, 1, product.Variants[0].Stock) // 4 - 3
			}
		})
	}
}
