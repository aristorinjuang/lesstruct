package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	catalogapi "github.com/aristorinjuang/lesstruct/modules/catalog/api"
	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
	domainmocks "github.com/aristorinjuang/lesstruct/modules/catalog/domain/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// storedProduct is a stocked product with one variant in its matrix.
func storedProduct() *catalogdomain.Product {
	sale := int64(249000)
	weight := 500
	threshold := 2
	return &catalogdomain.Product{
		ContentID:         7,
		Price:             299000,
		SalePrice:         &sale,
		SKU:               "TSHIRT",
		TrackInventory:    true,
		Stock:             10,
		WeightGrams:       &weight,
		LowStockThreshold: &threshold,
		Options:           map[string][]string{"size": {"S", "M"}},
		Variants: []catalogdomain.Variant{
			{ID: 3, ProductContentID: 7, Options: map[string]string{"size": "S"}, SKU: "TSHIRT-S", Stock: 4},
		},
		UpdatedAt: time.Date(2026, 10, 24, 10, 30, 0, 0, time.UTC),
	}
}

func TestHandler_GetProduct_WireShape(t *testing.T) {
	tests := []struct {
		name      string
		contentID string
		setup     func(*domainmocks.MockProductStore)
		wantCode  int
		wantErr   bool
	}{
		{
			name:      "success - camelCase keys",
			contentID: "7",
			setup: func(store *domainmocks.MockProductStore) {
				store.On("GetProduct", mock.Anything, 7).Return(storedProduct(), nil)
			},
			wantCode: http.StatusOK,
			wantErr:  false,
		},
		{
			name:      "error - missing product maps to 404",
			contentID: "7",
			setup: func(store *domainmocks.MockProductStore) {
				store.On("GetProduct", mock.Anything, 7).Return(nil, catalogdomain.ErrProductNotFound)
			},
			wantCode: http.StatusNotFound,
			wantErr:  true,
		},
		{
			name:      "error - invalid content id maps to 400",
			contentID: "bogus",
			setup:     func(*domainmocks.MockProductStore) {},
			wantCode:  http.StatusBadRequest,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := domainmocks.NewMockProductStore(t)
			tt.setup(store)

			service := catalogdomain.NewService(store, domainmocks.NewMockMailer(t))
			handler := catalogapi.New(service, catalogapi.Config{PostType: "product", Currency: "IDR"}, nil, nil, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/products/"+tt.contentID, nil)
			req.SetPathValue("contentID", tt.contentID)
			w := httptest.NewRecorder()

			handler.GetProduct(w, req)

			require.Equal(t, tt.wantCode, w.Code)
			var envelope map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
			if tt.wantErr {
				require.NotNil(t, envelope["error"])
				return
			}
			data, ok := envelope["data"].(map[string]any)
			require.True(t, ok, "data must be an object, got %v", envelope["data"])
			assert.Equal(t, float64(7), data["contentId"])
			assert.Equal(t, float64(299000), data["price"])
			assert.Equal(t, float64(249000), data["salePrice"])
			assert.Equal(t, "TSHIRT", data["sku"])
			assert.Equal(t, true, data["trackInventory"])
			assert.NotContains(t, data, "ContentID")
			assert.NotContains(t, data, "SalePrice")
			assert.NotContains(t, data, "TrackInventory")
			assert.Contains(t, data, "updatedAt")
			variants, ok := data["variants"].([]any)
			require.True(t, ok, "variants must be an array")
			require.Len(t, variants, 1)
			variant, ok := variants[0].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, float64(3), variant["id"])
			assert.Equal(t, float64(7), variant["productContentId"])
			assert.NotContains(t, variant, "ProductContentID")
		})
	}
}

func TestHandler_UpsertProduct_WireShape(t *testing.T) {
	tests := []struct {
		name      string
		contentID string
		body      string
		admin     bool
		setup     func(*domainmocks.MockProductStore)
		wantCode  int
		wantErr   bool
	}{
		{
			name:      "success - PUT returns camelCase keys",
			contentID: "7",
			body:      `{"price":299000,"trackInventory":true,"stock":10,"options":{"size":["S"]},"variants":[{"options":{"size":"S"},"stock":4}]}`,
			admin:     true,
			setup: func(store *domainmocks.MockProductStore) {
				store.On("UpsertProduct", mock.Anything, 7, mock.Anything).Return(nil)
				store.On("GetProduct", mock.Anything, 7).Return(storedProduct(), nil)
			},
			wantCode: http.StatusOK,
			wantErr:  false,
		},
		{
			name:      "error - non-admin is forbidden",
			contentID: "7",
			body:      `{"price":299000,"trackInventory":true,"stock":10,"options":{"size":["S"]},"variants":[]}`,
			admin:     false,
			setup:     func(store *domainmocks.MockProductStore) {},
			wantCode:  http.StatusForbidden,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := domainmocks.NewMockProductStore(t)
			tt.setup(store)

			service := catalogdomain.NewService(store, domainmocks.NewMockMailer(t))
			resolve := func(kernel.Principal, string) bool { return tt.admin }
			handler := catalogapi.New(service, catalogapi.Config{PostType: "product", Currency: "IDR"}, nil, nil, resolve)

			req := httptest.NewRequest(http.MethodPut, "/api/v1/catalog/products/"+tt.contentID, strings.NewReader(tt.body))
			req.SetPathValue("contentID", tt.contentID)
			ctx := kernel.ContextWithPrincipal(req.Context(), kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}})
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			handler.UpsertProduct(w, req)

			require.Equal(t, tt.wantCode, w.Code)
			var envelope map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
			if tt.wantErr {
				require.NotNil(t, envelope["error"])
				return
			}
			data, ok := envelope["data"].(map[string]any)
			require.True(t, ok, "data must be an object, got %v", envelope["data"])
			assert.Equal(t, float64(7), data["contentId"])
			assert.Equal(t, float64(299000), data["price"])
			assert.Equal(t, true, data["trackInventory"])
			assert.NotContains(t, data, "ContentID")
			assert.NotContains(t, data, "TrackInventory")
			assert.Contains(t, data, "updatedAt")
		})
	}
}

func TestHandler_UpsertProduct_InvalidImageURL(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "error - javascript imageUrl maps to 400 naming the field",
			body:    `{"price":299000,"trackInventory":true,"stock":10,"options":{"size":["S"]},"variants":[{"options":{"size":"S"},"stock":4,"imageUrl":"javascript:alert(1)"}]}`,
			wantErr: true,
		},
		{
			name:    "error - data imageUrl maps to 400 naming the field",
			body:    `{"price":299000,"trackInventory":true,"stock":10,"options":{"size":["S"]},"variants":[{"options":{"size":"S"},"stock":4,"imageUrl":"data:image/png;base64,AAA"}]}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := domainmocks.NewMockProductStore(t)

			service := catalogdomain.NewService(store, domainmocks.NewMockMailer(t))
			resolve := func(kernel.Principal, string) bool { return true }
			handler := catalogapi.New(service, catalogapi.Config{PostType: "product", Currency: "IDR"}, nil, nil, resolve)

			req := httptest.NewRequest(http.MethodPut, "/api/v1/catalog/products/7", strings.NewReader(tt.body))
			req.SetPathValue("contentID", "7")
			ctx := kernel.ContextWithPrincipal(req.Context(), kernel.Principal{ID: "1", Username: "admin", Roles: []string{"Admin"}})
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			handler.UpsertProduct(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)
			var envelope map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
			require.NotNil(t, envelope["error"])
			errObj, ok := envelope["error"].(map[string]any)
			require.True(t, ok, "error must be an object")
			assert.Contains(t, errObj["message"], "imageUrl")
		})
	}
}

func TestHandler_GetPublicProduct_StockPosture(t *testing.T) {
	threshold := 5
	healthy := 3
	tests := []struct {
		name         string
		product      *catalogdomain.Product
		wantInStock  bool
		wantLowStock bool
	}{
		{
			name: "sold-out variant matrix reads out of stock despite primary stock",
			product: &catalogdomain.Product{
				ContentID:         7,
				Price:             299000,
				TrackInventory:    true,
				Stock:             9,
				LowStockThreshold: &threshold,
				Options:           map[string][]string{"size": {"S", "M"}},
				Variants:          []catalogdomain.Variant{{ID: 3, ProductContentID: 7, Options: map[string]string{"size": "S"}, Stock: 0}},
			},
			wantInStock:  false,
			wantLowStock: false,
		},
		{
			name: "healthy variant sum reads in stock and not low despite zero primary",
			product: &catalogdomain.Product{
				ContentID:         7,
				Price:             299000,
				TrackInventory:    true,
				Stock:             0,
				LowStockThreshold: &healthy,
				Options:           map[string][]string{"size": {"S", "M"}},
				Variants:          []catalogdomain.Variant{{ID: 3, ProductContentID: 7, Options: map[string]string{"size": "S"}, Stock: 5}},
			},
			wantInStock:  true,
			wantLowStock: false,
		},
		{
			name: "low variant sum reads in stock and low",
			product: &catalogdomain.Product{
				ContentID:         7,
				Price:             299000,
				TrackInventory:    true,
				Stock:             0,
				LowStockThreshold: &threshold,
				Options:           map[string][]string{"size": {"S", "M"}},
				Variants:          []catalogdomain.Variant{{ID: 3, ProductContentID: 7, Options: map[string]string{"size": "S"}, Stock: 2}},
			},
			wantInStock:  true,
			wantLowStock: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := domainmocks.NewMockProductStore(t)
			store.On("GetProduct", mock.Anything, 7).Return(tt.product, nil)

			service := catalogdomain.NewService(store, domainmocks.NewMockMailer(t))
			resolve := func(context.Context, string) (int, bool) { return 7, true }
			handler := catalogapi.New(service, catalogapi.Config{PostType: "product", Currency: "IDR"}, nil, resolve, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/public/catalog/products/tee", nil)
			req.SetPathValue("slug", "tee")
			w := httptest.NewRecorder()

			handler.GetPublicProduct(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			var envelope map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
			data, ok := envelope["data"].(map[string]any)
			require.True(t, ok, "data must be an object, got %v", envelope["data"])
			assert.Equal(t, tt.wantInStock, data["inStock"])
			// lowStock carries omitempty: absent means false.
			lowStock, _ := data["lowStock"].(bool)
			assert.Equal(t, tt.wantLowStock, lowStock)
		})
	}
}
