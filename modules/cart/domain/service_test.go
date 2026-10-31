package cart_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	domain "github.com/aristorinjuang/lesstruct/modules/cart/domain"
	"github.com/aristorinjuang/lesstruct/modules/cart/domain/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func storedCart() *domain.StoredCart {
	return &domain.StoredCart{
		ID:        11,
		TokenHash: "hash",
		Lines: []domain.StoredLine{
			{ID: 1, CartID: 11, ProductContentID: 7, Quantity: 2},
		},
	}
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("no entropy") }

func availableItem() domain.CatalogItem {
	return domain.CatalogItem{ProductContentID: 7, SKU: "TSHIRT", UnitPrice: 299000, InStock: true}
}

func TestService_CreateCart(t *testing.T) {
	t.Run("mints a token and persists an empty cart", func(t *testing.T) {
		store := mocks.NewMockCartStore(t)
		store.On("CreateCart", mock.Anything, mock.AnythingOfType("string")).Return(int64(11), nil)

		service := domain.NewService(store, mocks.NewMockCatalogSource(t))
		token, cart, err := service.CreateCart(context.Background())

		require.NoError(t, err)
		assert.NotEmpty(t, token)
		assert.Equal(t, int64(11), cart.ID)
		assert.Empty(t, cart.Lines)
	})

	t.Run("store failure passes through", func(t *testing.T) {
		store := mocks.NewMockCartStore(t)
		store.On("CreateCart", mock.Anything, mock.AnythingOfType("string")).Return(int64(0), errors.New("db error"))

		service := domain.NewService(store, mocks.NewMockCatalogSource(t))
		_, _, err := service.CreateCart(context.Background())

		assert.ErrorContains(t, err, "db error")
	})

	t.Run("mint failure passes through", func(t *testing.T) {
		restore := domain.SetRandSourceForTest(failReader{})
		defer restore()

		service := domain.NewService(mocks.NewMockCartStore(t), mocks.NewMockCatalogSource(t))
		_, _, err := service.CreateCart(context.Background())

		assert.ErrorContains(t, err, "failed to mint cart token")
	})
}

func TestService_GetCart(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		setup     func(*mocks.MockCartStore, *mocks.MockCatalogSource)
		wantErr   error
		wantLines int
	}{
		{
			name:  "resolves lines with live prices",
			token: "token",
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(availableItem(), nil)
			},
			wantLines: 1,
		},
		{
			name:  "unresolvable lines stay readable but unavailable",
			token: "token",
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(domain.CatalogItem{}, domain.ErrUnknownProduct)
			},
			wantLines: 1,
		},
		{
			name:  "catalog failure aborts the read",
			token: "token",
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(domain.CatalogItem{}, errors.New("db error"))
			},
			wantErr: errors.New("failed to resolve cart line"),
		},
		{
			name:    "empty token fails without the store",
			token:   "",
			setup:   func(*mocks.MockCartStore, *mocks.MockCatalogSource) {},
			wantErr: domain.ErrCartNotFound,
		},
		{
			name:  "unknown token passes through",
			token: "ghost",
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(nil, domain.ErrCartNotFound)
			},
			wantErr: domain.ErrCartNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mocks.NewMockCartStore(t)
			catalog := mocks.NewMockCatalogSource(t)
			tt.setup(store, catalog)

			service := domain.NewService(store, catalog)
			cart, err := service.GetCart(context.Background(), tt.token)

			if tt.wantErr != nil {
				require.Error(t, err)
				if errors.Is(tt.wantErr, domain.ErrCartNotFound) {
					assert.ErrorIs(t, err, domain.ErrCartNotFound)
				} else {
					assert.ErrorContains(t, err, tt.wantErr.Error())
				}
				return
			}
			require.NoError(t, err)
			assert.Len(t, cart.Lines, tt.wantLines)
		})
	}
}

func TestService_GetCart_UnavailableLine(t *testing.T) {
	store := mocks.NewMockCartStore(t)
	catalog := mocks.NewMockCatalogSource(t)
	store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
	catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(domain.CatalogItem{}, domain.ErrUnknownProduct)

	service := domain.NewService(store, catalog)
	cart, err := service.GetCart(context.Background(), "token")

	require.NoError(t, err)
	require.Len(t, cart.Lines, 1)
	assert.False(t, cart.Lines[0].Available)
	assert.Equal(t, int64(0), cart.Subtotal())
	assert.Equal(t, 0, cart.ItemCount())
}

func TestService_AddItem(t *testing.T) {
	variantID := int64(3)
	zeroVariant := int64(0)

	tests := []struct {
		name      string
		productID int
		variantID *int64
		quantity  int
		setup     func(*mocks.MockCartStore, *mocks.MockCatalogSource)
		wantErr   error
	}{
		{
			name:      "new line appends and returns the refreshed cart",
			productID: 7,
			quantity:  1,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(availableItem(), nil).Twice()
				store.On("AddLine", mock.Anything, int64(11), 7, (*int64)(nil), 1).Return(domain.StoredLine{ID: 5}, nil)
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(&domain.StoredCart{ID: 11}, nil).Once()
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
			},
		},
		{
			name:      "existing line merges by incrementing",
			productID: 7,
			quantity:  2,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(availableItem(), nil)
				store.On("SetLineQuantity", mock.Anything, int64(11), int64(1), 4).Return(nil)
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
			},
		},
		{
			name:      "merge past the cap is refused",
			productID: 7,
			quantity:  98,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(availableItem(), nil)
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
			},
			wantErr: domain.ErrInvalidQuantity,
		},
		{
			name:      "out-of-stock demand is refused",
			productID: 7,
			quantity:  1,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				item := availableItem()
				item.InStock = false
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(item, nil)
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(&domain.StoredCart{ID: 11}, nil)
			},
			wantErr: domain.ErrOutOfStock,
		},
		{
			name:      "unknown product passes through",
			productID: 9,
			quantity:  1,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				catalog.On("ResolveLine", mock.Anything, 9, (*int64)(nil)).Return(domain.CatalogItem{}, domain.ErrUnknownProduct)
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(&domain.StoredCart{ID: 11}, nil)
			},
			wantErr: domain.ErrUnknownProduct,
		},
		{
			name:      "non-positive content id is unknown by construction",
			productID: 0,
			quantity:  1,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(&domain.StoredCart{ID: 11}, nil)
			},
			wantErr: domain.ErrUnknownProduct,
		},
		{
			name:      "non-positive variant id is unknown by construction",
			productID: 7,
			variantID: &zeroVariant,
			quantity:  1,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(&domain.StoredCart{ID: 11}, nil)
			},
			wantErr: domain.ErrUnknownVariant,
		},
		{
			name:      "quantity zero is invalid",
			productID: 7,
			quantity:  0,
			setup:     func(*mocks.MockCartStore, *mocks.MockCatalogSource) {},
			wantErr:   domain.ErrInvalidQuantity,
		},
		{
			name:      "quantity past the cap is invalid",
			productID: 7,
			quantity:  100,
			setup:     func(*mocks.MockCartStore, *mocks.MockCatalogSource) {},
			wantErr:   domain.ErrInvalidQuantity,
		},
		{
			name:      "unknown cart passes through",
			productID: 7,
			quantity:  1,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(nil, domain.ErrCartNotFound)
			},
			wantErr: domain.ErrCartNotFound,
		},
		{
			name:      "different product appends a new line",
			productID: 7,
			quantity:  1,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				other := &domain.StoredCart{ID: 11, Lines: []domain.StoredLine{
					{ID: 1, CartID: 11, ProductContentID: 8, Quantity: 1},
				}}
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(availableItem(), nil).Twice()
				store.On("AddLine", mock.Anything, int64(11), 7, (*int64)(nil), 1).Return(domain.StoredLine{ID: 2}, nil)
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(other, nil).Once()
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
			},
		},
		{
			name:      "variant value mismatch appends a new line",
			productID: 7,
			variantID: &variantID,
			quantity:  1,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				other := int64(4)
				stored := &domain.StoredCart{ID: 11, Lines: []domain.StoredLine{
					{ID: 1, CartID: 11, ProductContentID: 7, VariantID: &other, Quantity: 1},
				}}
				item := availableItem()
				item.VariantID = &variantID
				catalog.On("ResolveLine", mock.Anything, 7, &variantID).Return(item, nil)
				store.On("AddLine", mock.Anything, int64(11), 7, &variantID, 1).Return(domain.StoredLine{ID: 6}, nil)
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(stored, nil).Once()
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(availableItem(), nil)
			},
		},
		{
			name:      "append write failure passes through",
			productID: 7,
			quantity:  1,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(availableItem(), nil)
				store.On("AddLine", mock.Anything, int64(11), 7, (*int64)(nil), 1).Return(domain.StoredLine{}, errors.New("db error"))
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(&domain.StoredCart{ID: 11}, nil)
			},
			wantErr: errors.New("failed to add cart line"),
		},
		{
			name:      "merge write failure passes through",
			productID: 7,
			quantity:  2,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(availableItem(), nil)
				store.On("SetLineQuantity", mock.Anything, int64(11), int64(1), 4).Return(errors.New("db error"))
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
			},
			wantErr: errors.New("failed to merge cart line"),
		},
		{
			name:      "variant key distinguishes lines",
			productID: 7,
			variantID: &variantID,
			quantity:  1,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				item := availableItem()
				item.VariantID = &variantID
				catalog.On("ResolveLine", mock.Anything, 7, &variantID).Return(item, nil)
				store.On("AddLine", mock.Anything, int64(11), 7, &variantID, 1).Return(domain.StoredLine{ID: 6}, nil)
				withVariant := &domain.StoredCart{ID: 11, Lines: []domain.StoredLine{
					{ID: 1, CartID: 11, ProductContentID: 7, Quantity: 2},
					{ID: 6, CartID: 11, ProductContentID: 7, VariantID: &variantID, Quantity: 1},
				}}
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil).Once()
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(withVariant, nil)
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(availableItem(), nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mocks.NewMockCartStore(t)
			catalog := mocks.NewMockCatalogSource(t)
			tt.setup(store, catalog)

			service := domain.NewService(store, catalog)
			cart, err := service.AddItem(context.Background(), "token", tt.productID, tt.variantID, tt.quantity)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr) || strings.Contains(err.Error(), tt.wantErr.Error()), "unexpected error: %v", err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cart)
		})
	}
}

func TestService_UpdateItem(t *testing.T) {
	tests := []struct {
		name     string
		itemID   int64
		quantity int
		setup    func(*mocks.MockCartStore, *mocks.MockCatalogSource)
		wantErr  error
	}{
		{
			name:     "replaces the quantity and returns the refreshed cart",
			itemID:   1,
			quantity: 5,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(availableItem(), nil).Twice()
				store.On("SetLineQuantity", mock.Anything, int64(11), int64(1), 5).Return(nil)
			},
		},
		{
			name:     "unknown cart passes through",
			itemID:   1,
			quantity: 5,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(nil, domain.ErrCartNotFound)
			},
			wantErr: domain.ErrCartNotFound,
		},
		{
			name:     "unknown line fails",
			itemID:   99,
			quantity: 5,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
			},
			wantErr: domain.ErrLineNotFound,
		},
		{
			name:     "zero quantity is invalid",
			itemID:   1,
			quantity: 0,
			setup:    func(*mocks.MockCartStore, *mocks.MockCatalogSource) {},
			wantErr:  domain.ErrInvalidQuantity,
		},
		{
			name:     "out-of-stock demand is refused",
			itemID:   1,
			quantity: 5,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
				item := availableItem()
				item.InStock = false
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(item, nil).Once()
			},
			wantErr: domain.ErrOutOfStock,
		},
		{
			name:     "out-of-stock reductions are allowed",
			itemID:   1,
			quantity: 1,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil).Twice()
				item := availableItem()
				item.InStock = false
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(item, nil).Twice()
				store.On("SetLineQuantity", mock.Anything, int64(11), int64(1), 1).Return(nil)
			},
		},
		{
			name:     "update write failure passes through",
			itemID:   1,
			quantity: 5,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(availableItem(), nil)
				store.On("SetLineQuantity", mock.Anything, int64(11), int64(1), 5).Return(errors.New("db error"))
			},
			wantErr: errors.New("failed to update cart line"),
		},
		{
			name:     "vanished product fails",
			itemID:   1,
			quantity: 5,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(domain.CatalogItem{}, domain.ErrUnknownProduct).Once()
			},
			wantErr: domain.ErrUnknownProduct,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mocks.NewMockCartStore(t)
			catalog := mocks.NewMockCatalogSource(t)
			tt.setup(store, catalog)

			service := domain.NewService(store, catalog)
			cart, err := service.UpdateItem(context.Background(), "token", tt.itemID, tt.quantity)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr) || strings.Contains(err.Error(), tt.wantErr.Error()), "unexpected error: %v", err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cart)
		})
	}
}

func TestService_RemoveItem(t *testing.T) {
	t.Run("removes and returns the refreshed cart", func(t *testing.T) {
		store := mocks.NewMockCartStore(t)
		catalog := mocks.NewMockCatalogSource(t)
		store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil).Once()
		store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(&domain.StoredCart{ID: 11}, nil)
		store.On("RemoveLine", mock.Anything, int64(11), int64(1)).Return(nil)

		service := domain.NewService(store, catalog)
		cart, err := service.RemoveItem(context.Background(), "token", 1)

		require.NoError(t, err)
		require.NotNil(t, cart)
	})

	t.Run("remove write failure passes through", func(t *testing.T) {
		store := mocks.NewMockCartStore(t)
		catalog := mocks.NewMockCatalogSource(t)
		store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
		store.On("RemoveLine", mock.Anything, int64(11), int64(1)).Return(errors.New("db error"))

		service := domain.NewService(store, catalog)
		_, err := service.RemoveItem(context.Background(), "token", 1)

		assert.ErrorContains(t, err, "failed to remove cart line")
	})

	t.Run("unknown cart passes through", func(t *testing.T) {
		store := mocks.NewMockCartStore(t)
		catalog := mocks.NewMockCatalogSource(t)
		store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(nil, domain.ErrCartNotFound)

		service := domain.NewService(store, catalog)
		_, err := service.RemoveItem(context.Background(), "token", 1)

		assert.ErrorIs(t, err, domain.ErrCartNotFound)
	})
}

func TestService_DeleteCart(t *testing.T) {
	t.Run("deletes a known cart", func(t *testing.T) {
		store := mocks.NewMockCartStore(t)
		catalog := mocks.NewMockCatalogSource(t)
		store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
		store.On("DeleteCart", mock.Anything, int64(11)).Return(nil)

		service := domain.NewService(store, catalog)
		assert.NoError(t, service.DeleteCart(context.Background(), "token"))
	})

	t.Run("unknown cart fails", func(t *testing.T) {
		store := mocks.NewMockCartStore(t)
		catalog := mocks.NewMockCatalogSource(t)
		store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(nil, domain.ErrCartNotFound)

		service := domain.NewService(store, catalog)
		assert.ErrorIs(t, service.DeleteCart(context.Background(), "token"), domain.ErrCartNotFound)
	})

	t.Run("delete write failure passes through", func(t *testing.T) {
		store := mocks.NewMockCartStore(t)
		catalog := mocks.NewMockCatalogSource(t)
		store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
		store.On("DeleteCart", mock.Anything, int64(11)).Return(errors.New("db error"))

		service := domain.NewService(store, catalog)
		assert.ErrorContains(t, service.DeleteCart(context.Background(), "token"), "failed to delete cart")
	})
}
