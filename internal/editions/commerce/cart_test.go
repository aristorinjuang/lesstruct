package commerce_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/editions/commerce"
	cartdomain "github.com/aristorinjuang/lesstruct/modules/cart/domain"
	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubProductStore is a hand stub for the catalog persistence seam: the
// adapter tests need scripted products, not mock choreography.
type stubProductStore struct {
	products map[int]*catalogdomain.Product
	err      error
}

// stubMailer is a silent catalog Mailer: the catalog-source and inventory
// adapter tests never exercise the notice path.
type stubMailer struct{}

func (stubMailer) SendLowStockAlert(_ context.Context, _ *catalogdomain.Product, _ *int64, _ int) error {
	return nil
}

// newCatalogService builds the catalog service over the stub store with
// the notice path silenced.
func newCatalogService(store *stubProductStore) *catalogdomain.Service {
	return catalogdomain.NewService(store, stubMailer{})
}

func (s *stubProductStore) GetProduct(_ context.Context, contentID int) (*catalogdomain.Product, error) {
	if s.err != nil {
		return nil, s.err
	}
	product, ok := s.products[contentID]
	if !ok {
		return nil, catalogdomain.ErrProductNotFound
	}
	return product, nil
}

func (s *stubProductStore) UpsertProduct(_ context.Context, _ int, _ *catalogdomain.Product) error {
	return nil
}

// AdjustStock mirrors the real store's guarded update: tracked buckets
// only, refusing any move below the floor so compensation tests can act
// on a genuine stockout.
func (s *stubProductStore) AdjustStock(_ context.Context, contentID int, variantID *int64, delta int) (bool, error) {
	product, ok := s.products[contentID]
	if !ok {
		return false, nil
	}
	if variantID == nil {
		if !product.TrackInventory || product.Stock+delta < 0 {
			return false, nil
		}
		product.Stock += delta
		return true, nil
	}
	for i := range product.Variants {
		if product.Variants[i].ID != *variantID {
			continue
		}
		if product.Variants[i].Stock+delta < 0 {
			return false, nil
		}
		product.Variants[i].Stock += delta
		return true, nil
	}
	return false, nil
}

func catalogServiceWith(products map[int]*catalogdomain.Product) *catalogdomain.Service {
	return newCatalogService(&stubProductStore{products: products})
}

func TestCatalogSource_ResolveLine(t *testing.T) {
	sale := int64(249000)
	override := int64(279000)
	products := map[int]*catalogdomain.Product{
		7: {
			ContentID:      7,
			Price:          299000,
			SalePrice:      &sale,
			SKU:            "TSHIRT",
			TrackInventory: true,
			Stock:          7,
			Variants: []catalogdomain.Variant{
				{ID: 3, ProductContentID: 7, Options: map[string]string{"size": "M"}, SKU: "TSHIRT-M", Price: &override, Stock: 4},
				{ID: 4, ProductContentID: 7, Options: map[string]string{"size": "L"}, SKU: "TSHIRT-L", Stock: 1},
			},
		},
	}

	tests := []struct {
		name      string
		service   *catalogdomain.Service
		identity  commerce.ProductIdentityFunc
		contentID int
		variantID *int64
		expected  cartdomain.CatalogItem
		wantErr   error
		wantExact string
	}{
		{
			name:      "base product resolves the sale-effective price",
			service:   catalogServiceWith(products),
			contentID: 7,
			expected:  cartdomain.CatalogItem{ProductContentID: 7, SKU: "TSHIRT", UnitPrice: 249000, InStock: true},
		},
		{
			name:    "identity labels the resolved line",
			service: catalogServiceWith(products),
			identity: func(_ context.Context, _ int) (commerce.ProductIdentity, bool) {
				return commerce.ProductIdentity{Title: "Tshirt", Slug: "tshirt", ImageURL: "/media/tshirt.jpg"}, true
			},
			contentID: 7,
			expected: cartdomain.CatalogItem{
				ProductContentID: 7,
				SKU:              "TSHIRT",
				Title:            "Tshirt",
				Slug:             "tshirt",
				ImageURL:         "/media/tshirt.jpg",
				UnitPrice:        249000,
				InStock:          true,
			},
		},
		{
			name:    "identity miss leaves the line unlabelled",
			service: catalogServiceWith(products),
			identity: func(_ context.Context, _ int) (commerce.ProductIdentity, bool) {
				return commerce.ProductIdentity{}, false
			},
			contentID: 7,
			expected:  cartdomain.CatalogItem{ProductContentID: 7, SKU: "TSHIRT", UnitPrice: 249000, InStock: true},
		},
		{
			name:      "variant resolves its price override",
			service:   catalogServiceWith(products),
			contentID: 7,
			variantID: new(int64(3)),
			expected:  cartdomain.CatalogItem{ProductContentID: 7, VariantID: new(int64(3)), SKU: "TSHIRT-M", UnitPrice: 279000, InStock: true},
		},
		{
			name:      "variant without override inherits the product effective price",
			service:   catalogServiceWith(products),
			contentID: 7,
			variantID: new(int64(4)),
			expected:  cartdomain.CatalogItem{ProductContentID: 7, VariantID: new(int64(4)), SKU: "TSHIRT-L", UnitPrice: 249000, InStock: true},
		},
		{
			name:    "base line of a sold-out variant matrix is out of stock",
			service: catalogServiceWith(map[int]*catalogdomain.Product{
				8: {
					ContentID:      8,
					Price:          199000,
					SKU:            "CAP",
					TrackInventory: true,
					Stock:          9,
					Options:        map[string][]string{"size": {"S", "M"}},
					Variants: []catalogdomain.Variant{
						{ID: 5, ProductContentID: 8, Options: map[string]string{"size": "S"}, Stock: 0},
						{ID: 6, ProductContentID: 8, Options: map[string]string{"size": "M"}, Stock: 0},
					},
				},
			}),
			contentID: 8,
			expected:  cartdomain.CatalogItem{ProductContentID: 8, SKU: "CAP", UnitPrice: 199000, InStock: false},
		},
		{
			name:    "base line of a stocked variant matrix is in stock despite zero primary",
			service: catalogServiceWith(map[int]*catalogdomain.Product{
				9: {
					ContentID:      9,
					Price:          199000,
					SKU:            "CAP",
					TrackInventory: true,
					Stock:          0,
					Options:        map[string][]string{"size": {"S", "M"}},
					Variants: []catalogdomain.Variant{
						{ID: 7, ProductContentID: 9, Options: map[string]string{"size": "S"}, Stock: 2},
						{ID: 8, ProductContentID: 9, Options: map[string]string{"size": "M"}, Stock: 0},
					},
				},
			}),
			contentID: 9,
			expected:  cartdomain.CatalogItem{ProductContentID: 9, SKU: "CAP", UnitPrice: 199000, InStock: true},
		},
		{
			name:      "unknown product wraps the cart sentinel",
			service:   catalogServiceWith(products),
			contentID: 9,
			wantErr:   cartdomain.ErrUnknownProduct,
		},
		{
			name:      "unknown variant wraps the cart sentinel",
			service:   catalogServiceWith(products),
			contentID: 7,
			variantID: new(int64(99)),
			wantErr:   cartdomain.ErrUnknownVariant,
		},
		{
			name:      "store failure passes through unwrapped",
			service:   newCatalogService(&stubProductStore{err: errors.New("db error")}),
			contentID: 7,
			wantExact: "db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := commerce.NewCatalogSource(tt.service, tt.identity)

			item, err := source.ResolveLine(context.Background(), tt.contentID, tt.variantID)

			if tt.wantExact != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantExact, err.Error())
				return
			}
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, item)
		})
	}
}
