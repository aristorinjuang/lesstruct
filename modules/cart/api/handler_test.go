package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cartapi "github.com/aristorinjuang/lesstruct/modules/cart/api"
	cartdomain "github.com/aristorinjuang/lesstruct/modules/cart/domain"
	"github.com/aristorinjuang/lesstruct/modules/cart/domain/mocks"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testRouter(handler *cartapi.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/cart", handler.CreateCart)
	r.Get("/cart", handler.GetCart)
	r.Delete("/cart", handler.DeleteCart)
	r.Post("/cart/items", handler.AddItem)
	r.Patch("/cart/items/{itemId}", handler.UpdateItem)
	r.Delete("/cart/items/{itemId}", handler.RemoveItem)
	return r
}

func storedCart() *cartdomain.StoredCart {
	return &cartdomain.StoredCart{
		ID:        11,
		TokenHash: "hash",
		Lines: []cartdomain.StoredLine{
			{ID: 1, CartID: 11, ProductContentID: 7, Quantity: 2},
		},
	}
}

func availableItem() cartdomain.CatalogItem {
	return cartdomain.CatalogItem{ProductContentID: 7, SKU: "TSHIRT", UnitPrice: 299000, InStock: true}
}

func identifiedItem() cartdomain.CatalogItem {
	item := availableItem()
	item.Title = "Heavyweight Tee"
	item.Slug = "heavyweight-tee"
	item.ImageURL = "/media/tee.jpg"
	return item
}

func serve(t *testing.T, router *chi.Mux, method, path, headerToken, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if headerToken != "" {
		req.Header.Set("X-Cart-Token", headerToken)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestHandler_CreateCart(t *testing.T) {
	store := mocks.NewMockCartStore(t)
	store.On("CreateCart", mock.Anything, mock.AnythingOfType("string")).Return(int64(11), nil)

	handler := cartapi.New(cartdomain.NewService(store, mocks.NewMockCatalogSource(t)), "IDR")
	w := serve(t, testRouter(handler), http.MethodPost, "/cart", "", "")

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Cart-Token"))

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	data := resp["data"].(map[string]any)
	assert.NotEmpty(t, data["token"])
	assert.Equal(t, data["token"], w.Header().Get("X-Cart-Token"))
}

func TestHandler_GetCart(t *testing.T) {
	tests := []struct {
		name         string
		headerToken  string
		setup        func(*mocks.MockCartStore, *mocks.MockCatalogSource)
		expectedCode int
		expectedErr  string
	}{
		{
			name:        "returns the live-priced cart",
			headerToken: "token",
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(availableItem(), nil)
			},
			expectedCode: http.StatusOK,
		},
		{
			name:         "missing token is unauthorized",
			headerToken:  "",
			setup:        func(*mocks.MockCartStore, *mocks.MockCatalogSource) {},
			expectedCode: http.StatusUnauthorized,
			expectedErr:  "INVALID_CART_TOKEN",
		},
		{
			name:        "unknown token is unauthorized",
			headerToken: "ghost",
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(nil, cartdomain.ErrCartNotFound)
			},
			expectedCode: http.StatusUnauthorized,
			expectedErr:  "INVALID_CART_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mocks.NewMockCartStore(t)
			catalog := mocks.NewMockCatalogSource(t)
			tt.setup(store, catalog)

			handler := cartapi.New(cartdomain.NewService(store, catalog), "IDR")
			w := serve(t, testRouter(handler), http.MethodGet, "/cart", tt.headerToken, "")

			assert.Equal(t, tt.expectedCode, w.Code)
			var resp map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
			if tt.expectedErr != "" {
				assert.Equal(t, tt.expectedErr, resp["error"].(map[string]any)["code"])
				return
			}
			data := resp["data"].(map[string]any)
			assert.Equal(t, "IDR", data["currency"])
			assert.Equal(t, float64(598000), data["subtotal"])
			assert.Equal(t, float64(2), data["itemCount"])
		})
	}
}

func TestHandler_GetCart_LineIdentity(t *testing.T) {
	tests := []struct {
		name       string
		item       cartdomain.CatalogItem
		expected   map[string]any
		unexpected []string
	}{
		{
			name: "identity fields are serialized",
			item: identifiedItem(),
			expected: map[string]any{
				"title":    "Heavyweight Tee",
				"slug":     "heavyweight-tee",
				"imageUrl": "/media/tee.jpg",
			},
		},
		{
			name:       "empty identity fields are omitted",
			item:       availableItem(),
			unexpected: []string{"title", "slug", "imageUrl"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mocks.NewMockCartStore(t)
			catalog := mocks.NewMockCatalogSource(t)
			store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
			catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(tt.item, nil)

			handler := cartapi.New(cartdomain.NewService(store, catalog), "IDR")
			w := serve(t, testRouter(handler), http.MethodGet, "/cart", "token", "")

			require.Equal(t, http.StatusOK, w.Code)
			var resp map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
			lines := resp["data"].(map[string]any)["lines"].([]any)
			require.Len(t, lines, 1)
			line := lines[0].(map[string]any)
			for key, expected := range tt.expected {
				assert.Equal(t, expected, line[key])
			}
			for _, key := range tt.unexpected {
				_, exists := line[key]
				assert.False(t, exists, "%s must be omitted", key)
			}
		})
	}
}

func TestHandler_AddItem(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		setup        func(*mocks.MockCartStore, *mocks.MockCatalogSource)
		expectedCode int
		expectedErr  string
	}{
		{
			name: "merges the line and returns the cart",
			body: `{"productContentId":7,"quantity":1}`,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(availableItem(), nil).Twice()
				store.On("AddLine", mock.Anything, int64(11), 7, (*int64)(nil), 1).Return(cartdomain.StoredLine{ID: 5}, nil)
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(&cartdomain.StoredCart{ID: 11}, nil).Once()
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
			},
			expectedCode: http.StatusOK,
		},
		{
			name:         "malformed body is a validation error",
			body:         `{oops`,
			setup:        func(*mocks.MockCartStore, *mocks.MockCatalogSource) {},
			expectedCode: http.StatusBadRequest,
			expectedErr:  "VALIDATION_ERROR",
		},
		{
			name: "out-of-stock demand conflicts",
			body: `{"productContentId":7,"quantity":1}`,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				item := availableItem()
				item.InStock = false
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(item, nil)
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(&cartdomain.StoredCart{ID: 11}, nil)
			},
			expectedCode: http.StatusConflict,
			expectedErr:  "OUT_OF_STOCK",
		},
		{
			name: "unknown product is not found",
			body: `{"productContentId":9,"quantity":1}`,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				catalog.On("ResolveLine", mock.Anything, 9, (*int64)(nil)).Return(cartdomain.CatalogItem{}, cartdomain.ErrUnknownProduct)
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(&cartdomain.StoredCart{ID: 11}, nil)
			},
			expectedCode: http.StatusNotFound,
			expectedErr:  "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mocks.NewMockCartStore(t)
			catalog := mocks.NewMockCatalogSource(t)
			tt.setup(store, catalog)

			handler := cartapi.New(cartdomain.NewService(store, catalog), "IDR")
			w := serve(t, testRouter(handler), http.MethodPost, "/cart/items", "token", tt.body)

			assert.Equal(t, tt.expectedCode, w.Code)
			var resp map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
			if tt.expectedErr != "" {
				assert.Equal(t, tt.expectedErr, resp["error"].(map[string]any)["code"])
			}
		})
	}
}

func TestHandler_UpdateItem(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		body         string
		setup        func(*mocks.MockCartStore, *mocks.MockCatalogSource)
		expectedCode int
		expectedErr  string
	}{
		{
			name: "replaces the quantity",
			path: "/cart/items/1",
			body: `{"quantity":5}`,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
				catalog.On("ResolveLine", mock.Anything, 7, (*int64)(nil)).Return(availableItem(), nil).Twice()
				store.On("SetLineQuantity", mock.Anything, int64(11), int64(1), 5).Return(nil)
			},
			expectedCode: http.StatusOK,
		},
		{
			name:         "non-numeric item id is a validation error",
			path:         "/cart/items/abc",
			body:         `{"quantity":5}`,
			setup:        func(*mocks.MockCartStore, *mocks.MockCatalogSource) {},
			expectedCode: http.StatusBadRequest,
			expectedErr:  "VALIDATION_ERROR",
		},
		{
			name: "unknown line is not found",
			path: "/cart/items/99",
			body: `{"quantity":5}`,
			setup: func(store *mocks.MockCartStore, catalog *mocks.MockCatalogSource) {
				store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
			},
			expectedCode: http.StatusNotFound,
			expectedErr:  "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mocks.NewMockCartStore(t)
			catalog := mocks.NewMockCatalogSource(t)
			tt.setup(store, catalog)

			handler := cartapi.New(cartdomain.NewService(store, catalog), "IDR")
			w := serve(t, testRouter(handler), http.MethodPatch, tt.path, "token", tt.body)

			assert.Equal(t, tt.expectedCode, w.Code)
			var resp map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
			if tt.expectedErr != "" {
				assert.Equal(t, tt.expectedErr, resp["error"].(map[string]any)["code"])
			}
		})
	}
}

func TestHandler_RemoveItem(t *testing.T) {
	store := mocks.NewMockCartStore(t)
	catalog := mocks.NewMockCatalogSource(t)
	store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil).Once()
	store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(&cartdomain.StoredCart{ID: 11}, nil)
	store.On("RemoveLine", mock.Anything, int64(11), int64(1)).Return(nil)

	handler := cartapi.New(cartdomain.NewService(store, catalog), "IDR")
	w := serve(t, testRouter(handler), http.MethodDelete, "/cart/items/1", "token", "")

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_DeleteCart(t *testing.T) {
	store := mocks.NewMockCartStore(t)
	catalog := mocks.NewMockCatalogSource(t)
	store.On("FindCart", mock.Anything, mock.AnythingOfType("string")).Return(storedCart(), nil)
	store.On("DeleteCart", mock.Anything, int64(11)).Return(nil)

	handler := cartapi.New(cartdomain.NewService(store, catalog), "IDR")
	w := serve(t, testRouter(handler), http.MethodDelete, "/cart", "token", "")

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, true, resp["data"].(map[string]any)["deleted"])
}
