package commerce_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/editions/commerce"
	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
	ordersdomain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stockLine builds one order line at a tracked product bucket quantity.
func stockLine(contentID int, variantID *int64, quantity int) ordersdomain.OrderItem {
	return ordersdomain.OrderItem{
		ID:               1,
		OrderID:          42,
		ProductContentID: contentID,
		VariantID:        variantID,
		SKU:              "TSHIRT",
		Title:            "Nainggolan T-Shirt",
		UnitPrice:        299000,
		Quantity:         quantity,
	}
}

func TestCatalogInventory_Reserve_Release(t *testing.T) {
	variantOf := func(v int64) *int64 { return &v }

	stocks := func(store *stubProductStore, overridden func(*stubProductStore)) *stubProductStore {
		store.products = map[int]*catalogdomain.Product{
			7: {
				ContentID:      7,
				Price:          299000,
				TrackInventory: true,
				Stock:          5,
			},
			8: {
				ContentID:      8,
				Price:          199000,
				TrackInventory: true,
				Stock:          4,
				Variants: []catalogdomain.Variant{
					{ID: 3, ProductContentID: 8, SKU: "TSHIRT-S", Stock: 2},
				},
			},
			9: {
				ContentID:      9,
				Price:          99000,
				TrackInventory: false,
				Stock:          1,
			},
		}
		if overridden != nil {
			overridden(store)
		}
		return store
	}

	tests := []struct {
		name    string
		invoked func(adapter ordersdomain.Inventory, items []ordersdomain.OrderItem) error
		items   []ordersdomain.OrderItem
		seed    func(*stubProductStore)
		check   func(t *testing.T, store *stubProductStore)
		wantErr error
	}{
		{
			name: "reserves every tracked line",
			invoked: func(adapter ordersdomain.Inventory, items []ordersdomain.OrderItem) error {
				return adapter.Reserve(context.Background(), items)
			},
			items: []ordersdomain.OrderItem{
				stockLine(7, nil, 2),
				stockLine(8, variantOf(3), 1),
			},
			seed: func(*stubProductStore) {},
			check: func(t *testing.T, store *stubProductStore) {
				assert.Equal(t, 3, store.products[7].Stock)
				assert.Equal(t, 1, store.products[8].Variants[0].Stock)
			},
		},
		{
			name: "untracked lines never reserve and never fail",
			invoked: func(adapter ordersdomain.Inventory, items []ordersdomain.OrderItem) error {
				return adapter.Reserve(context.Background(), items)
			},
			items: []ordersdomain.OrderItem{
				stockLine(9, nil, 1),
			},
			seed: func(*stubProductStore) {},
			check: func(t *testing.T, store *stubProductStore) {
				assert.Equal(t, 1, store.products[9].Stock, "untracked buckets cannot move")
			},
		},
		{
			name: "a refused line compensates the earlier claims",
			invoked: func(adapter ordersdomain.Inventory, items []ordersdomain.OrderItem) error {
				return adapter.Reserve(context.Background(), items)
			},
			items: []ordersdomain.OrderItem{
				stockLine(7, nil, 2),
				stockLine(8, variantOf(3), 3),
			},
			seed:    func(*stubProductStore) {},
			wantErr: ordersdomain.ErrInsufficientStock,
			check: func(t *testing.T, store *stubProductStore) {
				assert.Equal(t, 5, store.products[7].Stock, "the earlier claim unwound")
				assert.Equal(t, 2, store.products[8].Variants[0].Stock, "the refused line stayed untouched")
			},
		},
		{
			name: "release restocks symmetrically",
			invoked: func(adapter ordersdomain.Inventory, items []ordersdomain.OrderItem) error {
				return adapter.Release(context.Background(), items)
			},
			items: []ordersdomain.OrderItem{
				stockLine(7, nil, 2),
				stockLine(8, variantOf(3), 1),
			},
			seed: func(*stubProductStore) {},
			check: func(t *testing.T, store *stubProductStore) {
				assert.Equal(t, 7, store.products[7].Stock)
				assert.Equal(t, 3, store.products[8].Variants[0].Stock)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := stocks(&stubProductStore{}, tt.seed)
			adapter := commerce.NewCatalogInventory(newCatalogService(store), nil)

			err := tt.invoked(adapter, tt.items)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			tt.check(t, store)
		})
	}
}

func TestCatalogInventory_ReleaseFailureLogsAndSweeps(t *testing.T) {
	// A restock onto a vanished catalog row logs adapter-side and still
	// reports success: the transition that triggered the release stands.
	store := &stubProductStore{products: map[int]*catalogdomain.Product{}}
	adapter := commerce.NewCatalogInventory(newCatalogService(store), nil)

	err := adapter.Release(context.Background(), []ordersdomain.OrderItem{stockLine(7, nil, 2)})

	require.NoError(t, err)
}
