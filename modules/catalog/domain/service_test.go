package catalog_test

import (
	"context"
	"errors"
	"testing"

	domain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
	"github.com/aristorinjuang/lesstruct/modules/catalog/domain/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// errStoreFailure is the ad-hoc store error shared between a table row's
// setup and its wantErr, so errors.Is matches instances.
var errStoreFailure = errors.New("db error")

// trackedProduct is a stocked product with one variant in its matrix.
func trackedProduct() *domain.Product {
	return &domain.Product{
		ContentID:      7,
		Price:          299000,
		TrackInventory: true,
		Stock:          10,
		Variants: []domain.Variant{
			{ID: 3, ProductContentID: 7, Options: map[string]string{"size": "S"}, SKU: "TSHIRT-S", Stock: 4},
		},
	}
}

func TestService_GetProduct(t *testing.T) {
	tests := []struct {
		name      string
		contentID int
		setup     func(*mocks.MockProductStore)
		wantErr   error
	}{
		{
			name:      "returns the stored product",
			contentID: 7,
			setup: func(store *mocks.MockProductStore) {
				store.On("GetProduct", mock.Anything, 7).Return(&domain.Product{ContentID: 7, Price: 100}, nil)
			},
		},
		{
			name:      "not found passes through",
			contentID: 7,
			setup: func(store *mocks.MockProductStore) {
				store.On("GetProduct", mock.Anything, 7).Return(nil, domain.ErrProductNotFound)
			},
			wantErr: domain.ErrProductNotFound,
		},
		{
			name:      "zero content id fails without the store",
			contentID: 0,
			setup: func(store *mocks.MockProductStore) {
			},
			wantErr: domain.ErrProductNotFound,
		},
		{
			name:      "store failure passes through",
			contentID: 7,
			setup: func(store *mocks.MockProductStore) {
				store.On("GetProduct", mock.Anything, 7).Return(nil, errors.New("db error"))
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mocks.NewMockProductStore(t)
			tt.setup(store)

			service := domain.NewService(store, mocks.NewMockMailer(t))
			product, err := service.GetProduct(context.Background(), tt.contentID)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			if tt.name == "store failure passes through" {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.contentID, product.ContentID)
		})
	}
}

func TestService_UpsertProduct(t *testing.T) {
	validInput := domain.UpsertProductInput{
		Price: 299000, Stock: 5, TrackInventory: true,
		Options:  map[string][]string{"size": {"S"}},
		Variants: []domain.VariantInput{{Options: map[string]string{"size": "S"}, SKU: "TSHIRT-S"}},
	}

	t.Run("validates then persists then re-reads", func(t *testing.T) {
		store := mocks.NewMockProductStore(t)
		stored := &domain.Product{ContentID: 7, Price: validInput.Price}
		store.On("UpsertProduct", mock.Anything, 7, mock.Anything).Return(nil)
		store.On("GetProduct", mock.Anything, 7).Return(stored, nil)

		service := domain.NewService(store, mocks.NewMockMailer(t))
		product, err := service.UpsertProduct(context.Background(), 7, validInput)

		assert.NoError(t, err)
		assert.Equal(t, stored, product)
	})

	t.Run("invalid input fails without touching the store", func(t *testing.T) {
		store := mocks.NewMockProductStore(t)

		service := domain.NewService(store, mocks.NewMockMailer(t))
		_, err := service.UpsertProduct(context.Background(), 7, domain.UpsertProductInput{Price: -1})

		assert.ErrorIs(t, err, domain.ErrInvalidPrice)
	})

	t.Run("zero content id fails without touching the store", func(t *testing.T) {
		store := mocks.NewMockProductStore(t)

		service := domain.NewService(store, mocks.NewMockMailer(t))
		_, err := service.UpsertProduct(context.Background(), 0, validInput)

		assert.ErrorIs(t, err, domain.ErrProductNotFound)
	})

	t.Run("store failure passes through", func(t *testing.T) {
		store := mocks.NewMockProductStore(t)
		store.On("UpsertProduct", mock.Anything, 7, mock.Anything).Return(errors.New("db error"))

		service := domain.NewService(store, mocks.NewMockMailer(t))
		_, err := service.UpsertProduct(context.Background(), 7, validInput)

		assert.ErrorContains(t, err, "db error")
	})

	t.Run("re-read failure after successful upsert passes through", func(t *testing.T) {
		store := mocks.NewMockProductStore(t)
		store.On("UpsertProduct", mock.Anything, 7, mock.Anything).Return(nil)
		store.On("GetProduct", mock.Anything, 7).Return(nil, errors.New("re-read error"))

		service := domain.NewService(store, mocks.NewMockMailer(t))
		_, err := service.UpsertProduct(context.Background(), 7, validInput)

		assert.ErrorContains(t, err, "re-read error")
	})
}

func TestService_AdjustStock(t *testing.T) {
	tests := []struct {
		name      string
		contentID int
		variantID *int64
		delta     int
		setup     func(*mocks.MockProductStore)
		wantErr   error
	}{
		{
			name:      "zero delta is a no-op",
			contentID: 7,
			delta:     0,
			setup:     func(*mocks.MockProductStore) {},
		},
		{
			name:      "missing product passes through",
			contentID: 7,
			delta:     -1,
			setup: func(store *mocks.MockProductStore) {
				store.On("GetProduct", mock.Anything, 7).Return(nil, domain.ErrProductNotFound)
			},
			wantErr: domain.ErrProductNotFound,
		},
		{
			name:      "untracked product never reaches the store",
			contentID: 7,
			delta:     -1,
			setup: func(store *mocks.MockProductStore) {
				store.On("GetProduct", mock.Anything, 7).Return(&domain.Product{ContentID: 7, TrackInventory: false}, nil)
			},
		},
		{
			name:      "product bucket decrements",
			contentID: 7,
			delta:     -2,
			setup: func(store *mocks.MockProductStore) {
				store.On("GetProduct", mock.Anything, 7).Return(trackedProduct(), nil)
				store.On("AdjustStock", mock.Anything, 7, (*int64)(nil), -2).Return(true, nil)
			},
		},
		{
			name:      "move below the floor is insufficient",
			contentID: 7,
			delta:     -2,
			setup: func(store *mocks.MockProductStore) {
				store.On("GetProduct", mock.Anything, 7).Return(trackedProduct(), nil)
				store.On("AdjustStock", mock.Anything, 7, (*int64)(nil), -2).Return(false, nil)
			},
			wantErr: domain.ErrInsufficientStock,
		},
		{
			name:      "variant bucket decrements by id",
			contentID: 7,
			variantID: new(int64(3)),
			delta:     -1,
			setup: func(store *mocks.MockProductStore) {
				store.On("GetProduct", mock.Anything, 7).Return(trackedProduct(), nil)
				store.On("AdjustStock", mock.Anything, 7, new(int64(3)), -1).Return(true, nil)
			},
		},
		{
			name:      "unknown variant reads not found",
			contentID: 7,
			variantID: new(int64(99)),
			delta:     -1,
			setup: func(store *mocks.MockProductStore) {
				store.On("GetProduct", mock.Anything, 7).Return(trackedProduct(), nil)
			},
			wantErr: domain.ErrProductNotFound,
		},
		{
			name:      "store failure passes through",
			contentID: 7,
			delta:     5,
			setup: func(store *mocks.MockProductStore) {
				store.On("GetProduct", mock.Anything, 7).Return(trackedProduct(), nil)
				store.On("AdjustStock", mock.Anything, 7, (*int64)(nil), 5).Return(false, errStoreFailure)
			},
			wantErr: errStoreFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mocks.NewMockProductStore(t)
			tt.setup(store)

			service := domain.NewService(store, mocks.NewMockMailer(t))
			err := service.AdjustStock(context.Background(), tt.contentID, tt.variantID, tt.delta)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestService_AdjustStock_LowStockAlert(t *testing.T) {
	thresholded := func(stock int, threshold int) *domain.Product {
		product := trackedProduct()
		product.Stock = stock
		product.LowStockThreshold = new(threshold)
		return product
	}

	tests := []struct {
		name      string
		variantID *int64
		delta     int
		setup     func(*mocks.MockProductStore, *mocks.MockMailer)
		wantErr   error
	}{
		{
			name:  "a base bucket crossing fires once",
			delta: -6,
			setup: func(store *mocks.MockProductStore, mail *mocks.MockMailer) {
				store.On("GetProduct", mock.Anything, 7).Return(thresholded(10, 5), nil)
				store.On("AdjustStock", mock.Anything, 7, (*int64)(nil), -6).Return(true, nil)
				mail.On("SendLowStockAlert", mock.Anything, mock.Anything, (*int64)(nil), 4).Return(nil)
			},
		},
		{
			name:      "a variant bucket crossing fires with its id",
			variantID: new(int64(3)),
			delta:     -2,
			setup: func(store *mocks.MockProductStore, mail *mocks.MockMailer) {
				product := thresholded(10, 3)
				store.On("GetProduct", mock.Anything, 7).Return(product, nil)
				store.On("AdjustStock", mock.Anything, 7, new(int64(3)), -2).Return(true, nil)
				mail.On("SendLowStockAlert", mock.Anything, mock.Anything, new(int64(3)), 2).Return(nil)
			},
		},
		{
			name:  "a restock never fires",
			delta: 5,
			setup: func(store *mocks.MockProductStore, mail *mocks.MockMailer) {
				store.On("GetProduct", mock.Anything, 7).Return(thresholded(3, 5), nil)
				store.On("AdjustStock", mock.Anything, 7, (*int64)(nil), 5).Return(true, nil)
			},
		},
		{
			name:  "a bucket already below stays quiet",
			delta: -1,
			setup: func(store *mocks.MockProductStore, mail *mocks.MockMailer) {
				store.On("GetProduct", mock.Anything, 7).Return(thresholded(4, 5), nil)
				store.On("AdjustStock", mock.Anything, 7, (*int64)(nil), -1).Return(true, nil)
			},
		},
		{
			name:  "a product without a threshold never fires",
			delta: -5,
			setup: func(store *mocks.MockProductStore, mail *mocks.MockMailer) {
				store.On("GetProduct", mock.Anything, 7).Return(trackedProduct(), nil)
				store.On("AdjustStock", mock.Anything, 7, (*int64)(nil), -5).Return(true, nil)
			},
		},
		{
			name:  "a mail failure never fails the adjustment",
			delta: -6,
			setup: func(store *mocks.MockProductStore, mail *mocks.MockMailer) {
				store.On("GetProduct", mock.Anything, 7).Return(thresholded(10, 5), nil)
				store.On("AdjustStock", mock.Anything, 7, (*int64)(nil), -6).Return(true, nil)
				mail.On("SendLowStockAlert", mock.Anything, mock.Anything, (*int64)(nil), 4).Return(errStoreFailure)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mocks.NewMockProductStore(t)
			mail := mocks.NewMockMailer(t)
			tt.setup(store, mail)

			service := domain.NewService(store, mail)
			err := service.AdjustStock(context.Background(), 7, tt.variantID, tt.delta)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}
