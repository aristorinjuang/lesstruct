package api_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/legal"
	ordersapi "github.com/aristorinjuang/lesstruct/modules/orders/api"
	ordersdomain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
	ordersmocks "github.com/aristorinjuang/lesstruct/modules/orders/domain/mocks"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// testRouter mounts both surfaces exactly as the module and composition
// root do: two route trees, the public realm and the JWT-protected admin
// realm, so path params never collide across realms.
func testRouter(handler *ordersapi.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/public", func(public chi.Router) {
		public.Post("/checkout", handler.Checkout)
		public.Get("/orders/{orderNumber}", handler.TrackOrder)
	})
	r.Route("/admin", func(admin chi.Router) {
		admin.Get("/orders", handler.ListOrders)
		admin.Get("/orders/invoice-settings", handler.GetInvoiceSettings)
		admin.Put("/orders/invoice-settings", handler.UpdateInvoiceSettings)
		admin.Get("/orders/{orderID}", handler.GetOrder)
		admin.Post("/orders/{orderID}/status", handler.TransitionOrder)
		admin.Post("/orders/{orderID}/shipping", handler.SetShippingOrder)
		admin.Get("/orders/{orderID}/invoice", handler.InvoiceOrder)
	})
	return r
}

// principal is the admin identity the auth chain injects in production.
func principal() *kernel.Principal {
	return &kernel.Principal{ID: "1", Username: "admin", Roles: []string{"admin"}}
}

// serve issues one request, optionally carrying a principal.
func serve(t *testing.T, router *chi.Mux, method, path string, who *kernel.Principal, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if who != nil {
		req = req.WithContext(kernel.ContextWithPrincipal(req.Context(), *who))
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decodeData(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	data, _ := resp["data"].(map[string]any)
	return data
}

// decodeListResponse decodes a list envelope once: the data array and the
// pagination metadata.
func decodeListResponse(t *testing.T, w *httptest.ResponseRecorder) ([]any, map[string]any) {
	t.Helper()
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	items, _ := resp["data"].([]any)
	meta, _ := resp["meta"].(map[string]any)
	return items, meta
}

func decodeErrorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	errInfo, _ := resp["error"].(map[string]any)
	code, _ := errInfo["code"].(string)
	return code
}

// adminResolver grants every capability, the users module's shape.
func adminResolver() kernel.CapabilityResolver {
	return func(_ kernel.Principal, capability string) bool {
		return capability == "admin"
	}
}

// denyResolver refuses everything, isolating the gate from the surface.
func denyResolver() kernel.CapabilityResolver {
	return func(kernel.Principal, string) bool { return false }
}

type harness struct {
	store     *ordersmocks.MockOrderStore
	cart      *ordersmocks.MockCartSource
	content   *ordersmocks.MockContentSource
	mail      *ordersmocks.MockMailer
	inventory *ordersmocks.MockInventory
	router    *chi.Mux
}

func newHarness(t *testing.T, resolve kernel.CapabilityResolver) *harness {
	t.Helper()
	return newHarnessWithService(t, resolve)
}

// newHarnessWithService builds the harness over a service carrying extra
// options (consent policy), so a test can exercise a gated checkout without
// re-wiring the whole router.
func newHarnessWithService(t *testing.T, resolve kernel.CapabilityResolver, opts ...ordersdomain.ServiceOption) *harness {
	t.Helper()

	store := ordersmocks.NewMockOrderStore(t)
	cart := ordersmocks.NewMockCartSource(t)
	content := ordersmocks.NewMockContentSource(t)
	mail := ordersmocks.NewMockMailer(t)
	inventory := ordersmocks.NewMockInventory(t)
	service := ordersdomain.NewService(store, cart, content, mail, inventory, "IDR", opts...)
	return &harness{
		store:     store,
		cart:      cart,
		content:   content,
		mail:      mail,
		inventory: inventory,
		router:    testRouter(ordersapi.New(service, resolve, nil)),
	}
}

const (
	// maxBodyBytes mirrors the handler's body cap so the oversized-input
	// case can exceed it by one.
	maxBodyBytes = 1 << 20

	// checkoutBody is the valid checkout request.
	checkoutBody = `{"token":"token","email":"shopper@example.com","name":"Annisa Putri","phone":"+6281234567890","shipTo":{"street":"Jl. Kemang Raya 21","city":"Jakarta Selatan","province":"DKI Jakarta","postalCode":"12730"}}`
)

// cartSnapshot prerenders a one-line snapshot.
func cartSnapshot() ordersdomain.CartSnapshot {
	return ordersdomain.CartSnapshot{Lines: []ordersdomain.CartSnapshotLine{{
		ProductContentID: 7,
		SKU:              "TSHIRT",
		Quantity:         2,
		UnitPrice:        299000,
		InStock:          true,
		Available:        true,
	}}}
}

// storedOrder is the persistence shape the store mocks return.
func storedOrder() *ordersdomain.Order {
	return &ordersdomain.Order{
		ID:            42,
		Status:        ordersdomain.StatusPending,
		CustomerEmail: "shopper@example.com",
		CustomerName:  "Annisa Putri",
		CustomerPhone: "+6281234567890",
		ShipTo: ordersdomain.Address{
			Street:     "Jl. Kemang Raya 21",
			City:       "Jakarta Selatan",
			Province:   "DKI Jakarta",
			PostalCode: "12730",
		},
		Currency: "IDR",
		Subtotal: 598000,
		Items: []ordersdomain.OrderItem{{
			ID:               1,
			OrderID:          42,
			ProductContentID: 7,
			SKU:              "TSHIRT",
			Title:            "Nainggolan T-Shirt",
			UnitPrice:        299000,
			Quantity:         2,
		}},
	}
}

func TestHandler_Checkout(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		setup    func(*harness)
		wantCode int
		wantErr  string
	}{
		{
			name: "consumes the cart into a pending order",
			body: checkoutBody,
			setup: func(h *harness) {
				h.cart.On("Snapshot", mock.Anything, "token").Return(cartSnapshot(), nil)
				h.content.On("ProductTitle", mock.Anything, 7).Return("Nainggolan T-Shirt", true)
				h.cart.On("Consume", mock.Anything, "token").Return(nil)
				h.store.On("CreateOrder", mock.Anything, mock.AnythingOfType("*orders.Order")).Return(int64(42), nil)
				h.mail.On("SendOrderConfirmationEmail", mock.Anything, mock.Anything).Return(nil)
			},
			wantCode: http.StatusOK,
		},
		{
			name:     "malformed body is refused",
			body:     "{",
			setup:    func(*harness) {},
			wantCode: http.StatusBadRequest,
			wantErr:  "VALIDATION_ERROR",
		},
		{
			name:     "oversized body is refused",
			body:     `{"email":"` + strings.Repeat("a", maxBodyBytes) + `"}`,
			setup:    func(*harness) {},
			wantCode: http.StatusBadRequest,
			wantErr:  "VALIDATION_ERROR",
		},
		{
			name: "unknown cart reads as a conflict",
			body: checkoutBody,
			setup: func(h *harness) {
				h.cart.On("Snapshot", mock.Anything, "token").Return(ordersdomain.CartSnapshot{}, ordersdomain.ErrCartGone)
			},
			wantCode: http.StatusConflict,
			wantErr:  "CART_UNAVAILABLE",
		},
		{
			name: "lost cart race reads as a conflict",
			body: checkoutBody,
			setup: func(h *harness) {
				h.cart.On("Snapshot", mock.Anything, "token").Return(cartSnapshot(), nil)
				h.cart.On("Consume", mock.Anything, "token").Return(ordersdomain.ErrCartGone)
			},
			wantCode: http.StatusConflict,
			wantErr:  "CART_UNAVAILABLE",
		},
		{
			name: "empty cart reads as a conflict",
			body: checkoutBody,
			setup: func(h *harness) {
				h.cart.On("Snapshot", mock.Anything, "token").Return(ordersdomain.CartSnapshot{}, nil)
			},
			wantCode: http.StatusConflict,
			wantErr:  "EMPTY_CART",
		},
		{
			name: "vanished lines read as a conflict",
			body: checkoutBody,
			setup: func(h *harness) {
				snapshot := cartSnapshot()
				snapshot.Lines[0].Available = false
				h.cart.On("Snapshot", mock.Anything, "token").Return(snapshot, nil)
			},
			wantCode: http.StatusConflict,
			wantErr:  "LINE_UNAVAILABLE",
		},
		{
			name: "sold-out lines read as a conflict",
			body: checkoutBody,
			setup: func(h *harness) {
				snapshot := cartSnapshot()
				snapshot.Lines[0].InStock = false
				h.cart.On("Snapshot", mock.Anything, "token").Return(snapshot, nil)
			},
			wantCode: http.StatusConflict,
			wantErr:  "OUT_OF_STOCK",
		},
		{
			name:     "invalid customer contact is a validation error",
			body:     `{"token":"token","email":"nope","name":"Annisa Putri","shipTo":{"street":"A","city":"B","province":"C","postalCode":"1"}}`,
			setup:    func(*harness) {},
			wantCode: http.StatusBadRequest,
			wantErr:  "VALIDATION_ERROR",
		},
		{
			name: "store failures read as internal errors",
			body: checkoutBody,
			setup: func(h *harness) {
				h.cart.On("Snapshot", mock.Anything, "token").Return(cartSnapshot(), nil)
				h.content.On("ProductTitle", mock.Anything, 7).Return("Nainggolan T-Shirt", true)
				h.cart.On("Consume", mock.Anything, "token").Return(nil)
				h.store.On("CreateOrder", mock.Anything, mock.AnythingOfType("*orders.Order")).Return(int64(0), errors.New("db error"))
			},
			wantCode: http.StatusInternalServerError,
			wantErr:  "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, adminResolver())
			tt.setup(h)

			w := serve(t, h.router, http.MethodPost, "/public/checkout", nil, tt.body)

			assert.Equal(t, tt.wantCode, w.Code)
			if tt.wantErr != "" {
				assert.Equal(t, tt.wantErr, decodeErrorCode(t, w))
				return
			}
			data := decodeData(t, w)
			assert.Equal(t, "ORD-000042", data["orderNumber"])
			assert.Equal(t, "pending", data["status"])
			assert.Equal(t, float64(598000), data["subtotal"])
			assert.Equal(t, "IDR", data["currency"])
			items, _ := data["items"].([]any)
			line, _ := items[0].(map[string]any)
			assert.Equal(t, float64(598000), line["lineTotal"])
			assert.Equal(t, "Nainggolan T-Shirt", line["title"])
		})
	}
}

func TestHandler_Checkout_Consent(t *testing.T) {
	terms := legal.Document{Page: legal.PageTermsOfService, PageID: 3, Slug: "terms-of-service", Language: "en"}
	privacy := legal.Document{Page: legal.PagePrivacyPolicy, PageID: 4, Slug: "privacy-policy", Language: "en"}
	policy := ordersdomain.ConsentPolicy{Required: true, Terms: terms, Privacy: privacy}

	tests := []struct {
		name        string
		body        string
		setup       func(*harness)
		wantCode    int
		wantErr     string
		wantConsent bool
	}{
		{
			name:     "an unchecked checkout is refused",
			body:     checkoutBody,
			setup:    func(*harness) {},
			wantCode: http.StatusBadRequest,
			wantErr:  "CONSENT_REQUIRED",
		},
		{
			name: "a checked checkout records the evidence",
			body: `{"token":"token","email":"shopper@example.com","name":"Annisa Putri","phone":"+6281234567890","consentAccepted":true,"shipTo":{"street":"Jl. Kemang Raya 21","city":"Jakarta Selatan","province":"DKI Jakarta","postalCode":"12730"}}`,
			setup: func(h *harness) {
				h.cart.On("Snapshot", mock.Anything, "token").Return(cartSnapshot(), nil)
				h.content.On("ProductTitle", mock.Anything, 7).Return("Nainggolan T-Shirt", true)
				h.cart.On("Consume", mock.Anything, "token").Return(nil)
				h.store.On("CreateOrder", mock.Anything, mock.AnythingOfType("*orders.Order")).Return(int64(42), nil)
				h.mail.On("SendOrderConfirmationEmail", mock.Anything, mock.Anything).Return(nil)
			},
			wantCode:    http.StatusOK,
			wantConsent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarnessWithService(t, adminResolver(), ordersdomain.WithConsentPolicy(policy))
			tt.setup(h)

			w := serve(t, h.router, http.MethodPost, "/public/checkout", nil, tt.body)

			assert.Equal(t, tt.wantCode, w.Code)
			if tt.wantErr != "" {
				assert.Equal(t, tt.wantErr, decodeErrorCode(t, w))
				return
			}
			data := decodeData(t, w)
			consent, ok := data["consent"].(map[string]any)
			require.True(t, ok || tt.wantConsent == false)
			if !tt.wantConsent {
				return
			}
			require.NotNil(t, consent)
			assert.NotEmpty(t, consent["acceptedAt"])
			termsPayload, _ := consent["terms"].(map[string]any)
			assert.Equal(t, "terms-of-service", termsPayload["slug"])
		})
	}
}

// trackingToken is the per-order secret the tracking route presents;
// trackingTokenHash mirrors the domain's at-rest form (hex SHA-256).
const trackingToken = "track-token"

var trackingTokenHash = sha256Hex(trackingToken)

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func TestHandler_TrackOrder(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		setup    func(*harness)
		wantCode int
		wantErr  string
	}{
		{
			name: "number, email, and token match",
			path: "/public/orders/ORD-000042?email=shopper%40example.com&token=track-token",
			setup: func(h *harness) {
				h.store.On("FindByNumberAndEmailAndToken", mock.Anything, int64(42), "shopper@example.com", trackingTokenHash).Return(storedOrder(), nil)
			},
			wantCode: http.StatusOK,
		},
		{
			name:     "a missing token reads as not found",
			path:     "/public/orders/ORD-000042?email=shopper%40example.com",
			setup:    func(*harness) {},
			wantCode: http.StatusNotFound,
			wantErr:  "NOT_FOUND",
		},
		{
			name:     "malformed number reads as not found",
			path:     "/public/orders/42?email=shopper@example.com&token=track-token",
			setup:    func(*harness) {},
			wantCode: http.StatusNotFound,
			wantErr:  "NOT_FOUND",
		},
		{
			name: "store miss reads as not found",
			path: "/public/orders/ORD-000042?email=wrong@example.com&token=track-token",
			setup: func(h *harness) {
				h.store.On("FindByNumberAndEmailAndToken", mock.Anything, int64(42), "wrong@example.com", trackingTokenHash).Return(nil, ordersdomain.ErrOrderNotFound)
			},
			wantCode: http.StatusNotFound,
			wantErr:  "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, adminResolver())
			tt.setup(h)

			w := serve(t, h.router, http.MethodGet, tt.path, nil, "")

			assert.Equal(t, tt.wantCode, w.Code)
			if tt.wantErr != "" {
				assert.Equal(t, tt.wantErr, decodeErrorCode(t, w))
				return
			}
			data := decodeData(t, w)
			assert.Equal(t, "ORD-000042", data["orderNumber"])
			assert.Equal(t, "Annisa Putri", data["customerName"])
		})
	}
}

func TestHandler_ListOrders(t *testing.T) {
	t.Run("lists the newest orders with pagination metadata", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		first := storedOrder()
		second := storedOrder()
		second.ID = 41
		h.store.On("List", mock.Anything, (*ordersdomain.Status)(nil), int64(0), 1).Return([]ordersdomain.Order{*first, *second}, nil)

		w := serve(t, h.router, http.MethodGet, "/admin/orders?limit=1", principal(), "")

		require.Equal(t, http.StatusOK, w.Code)
		items, meta := decodeListResponse(t, w)
		require.Len(t, items, 1, "the page clamps to the limit")
		firstRow, _ := items[0].(map[string]any)
		assert.Equal(t, "ORD-000042", firstRow["orderNumber"])
		pagination, _ := meta["pagination"].(map[string]any)
		assert.Equal(t, true, pagination["hasMore"])
		assert.Equal(t, "NDI", pagination["nextCursor"])
	})

	t.Run("an unknown status filter is a validation error", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodGet, "/admin/orders?status=superpaid", principal(), "")

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("a malformed cursor is a validation error", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodGet, "/admin/orders?cursor=abc", principal(), "")

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("an anonymous caller is refused", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodGet, "/admin/orders", nil, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "INSUFFICIENT_PERMISSIONS", decodeErrorCode(t, w))
	})

	t.Run("a refused capability is rejected", func(t *testing.T) {
		h := newHarness(t, denyResolver())

		w := serve(t, h.router, http.MethodGet, "/admin/orders", principal(), "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "INSUFFICIENT_PERMISSIONS", decodeErrorCode(t, w))
	})

	t.Run("a missing resolver fails closed", func(t *testing.T) {
		h := newHarness(t, nil)

		w := serve(t, h.router, http.MethodGet, "/admin/orders", principal(), "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "INSUFFICIENT_PERMISSIONS", decodeErrorCode(t, w))
	})

	t.Run("store failures read as internal errors", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("List", mock.Anything, (*ordersdomain.Status)(nil), int64(0), 50).Return(nil, errors.New("db error"))

		w := serve(t, h.router, http.MethodGet, "/admin/orders", principal(), "")

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, "INTERNAL_ERROR", decodeErrorCode(t, w))
	})
}

func TestHandler_GetOrder(t *testing.T) {
	t.Run("loads one order with its items", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(42)).Return(storedOrder(), nil)

		w := serve(t, h.router, http.MethodGet, "/admin/orders/42", principal(), "")

		require.Equal(t, http.StatusOK, w.Code)
		data := decodeData(t, w)
		assert.Equal(t, "ORD-000042", data["orderNumber"])
		assert.Equal(t, float64(42), data["id"])
		items, _ := data["items"].([]any)
		assert.Len(t, items, 1)
	})

	t.Run("an invalid order ID is a validation error", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodGet, "/admin/orders/abc", principal(), "")

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("an unknown order reads as not found", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(999)).Return(nil, ordersdomain.ErrOrderNotFound)

		w := serve(t, h.router, http.MethodGet, "/admin/orders/999", principal(), "")

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "NOT_FOUND", decodeErrorCode(t, w))
	})

	t.Run("an anonymous caller is refused", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodGet, "/admin/orders/42", nil, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "INSUFFICIENT_PERMISSIONS", decodeErrorCode(t, w))
	})
}

func TestHandler_TransitionOrder(t *testing.T) {
	t.Run("moves pending to cancelled and emails the customer", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(42)).Return(storedOrder(), nil).Once()
		cancelled := storedOrder()
		cancelled.Status = ordersdomain.StatusCancelled
		h.store.On("SetStatus", mock.Anything, int64(42), ordersdomain.StatusCancelled).Return(nil)
		h.store.On("FindByID", mock.Anything, int64(42)).Return(cancelled, nil).Once()
		h.mail.On("SendOrderStatusEmail", mock.Anything, mock.Anything, ordersdomain.StatusPending).Return(nil)

		w := serve(t, h.router, http.MethodPost, "/admin/orders/42/status", principal(), `{"status":"cancelled"}`)

		require.Equal(t, http.StatusOK, w.Code)
		data := decodeData(t, w)
		assert.Equal(t, "cancelled", data["status"])
	})

	t.Run("the paid target reads as a conflict", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodPost, "/admin/orders/42/status", principal(), `{"status":"paid"}`)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "INVALID_TRANSITION", decodeErrorCode(t, w))
	})

	t.Run("illegal moves read as a conflict", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(42)).Return(storedOrder(), nil)

		w := serve(t, h.router, http.MethodPost, "/admin/orders/42/status", principal(), `{"status":"shipped"}`)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "INVALID_TRANSITION", decodeErrorCode(t, w))
	})

	t.Run("an unknown status vocabulary is a validation error", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodPost, "/admin/orders/42/status", principal(), `{"status":"superpaid"}`)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("a malformed body is refused", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodPost, "/admin/orders/42/status", principal(), "{")

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("an anonymous caller is refused", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodPost, "/admin/orders/42/status", nil, `{"status":"paid"}`)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "INSUFFICIENT_PERMISSIONS", decodeErrorCode(t, w))
	})
}

func TestHandler_SetShippingOrder(t *testing.T) {
	t.Run("writes the line and echoes the total", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(42)).Return(storedOrder(), nil).Once()
		h.store.On("SetShipping", mock.Anything, int64(42), int64(15000)).Return(true, nil)
		charged := storedOrder()
		charged.ShippingMinor = 15000
		h.store.On("FindByID", mock.Anything, int64(42)).Return(charged, nil).Once()

		w := serve(t, h.router, http.MethodPost, "/admin/orders/42/shipping", principal(), `{"amount":15000}`)

		require.Equal(t, http.StatusOK, w.Code)
		data := decodeData(t, w)
		assert.Equal(t, float64(15000), data["shippingMinor"])
		assert.Equal(t, float64(613000), data["totalMinor"])
		assert.Equal(t, float64(598000), data["subtotal"])
	})

	t.Run("a negative line reads as a validation error", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodPost, "/admin/orders/42/shipping", principal(), `{"amount":-1}`)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("a settled order reads as a conflict", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		settled := storedOrder()
		settled.Status = ordersdomain.StatusPaid
		h.store.On("FindByID", mock.Anything, int64(42)).Return(settled, nil)

		w := serve(t, h.router, http.MethodPost, "/admin/orders/42/shipping", principal(), `{"amount":15000}`)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "INVALID_TRANSITION", decodeErrorCode(t, w))
	})

	t.Run("an unknown order reads as not found", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(42)).Return(nil, ordersdomain.ErrOrderNotFound)

		w := serve(t, h.router, http.MethodPost, "/admin/orders/42/shipping", principal(), `{"amount":15000}`)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "NOT_FOUND", decodeErrorCode(t, w))
	})

	t.Run("a malformed body is refused", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodPost, "/admin/orders/42/shipping", principal(), "{")

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("a malformed id is refused", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodPost, "/admin/orders/nope/shipping", principal(), `{"amount":15000}`)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("an anonymous caller is refused", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodPost, "/admin/orders/42/shipping", nil, `{"amount":15000}`)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "INSUFFICIENT_PERMISSIONS", decodeErrorCode(t, w))
	})
}

// stubRenderer is a fake invoice renderer: tests script bytes or failure
// and capture the document, never touching maroto.
type stubRenderer struct {
	doc *ordersdomain.InvoiceDocument
	out []byte
	err error
}

func (r *stubRenderer) Render(_ context.Context, doc *ordersdomain.InvoiceDocument) ([]byte, error) {
	r.doc = doc
	return r.out, r.err
}

func newInvoiceHarness(t *testing.T, renderer *stubRenderer) *harness {
	t.Helper()

	var support *ordersapi.InvoiceSupport
	if renderer != nil {
		support = &ordersapi.InvoiceSupport{Renderer: renderer}
	}
	return newHarnessWithSupport(t, support)
}

// newHarnessWithSupport builds the router over the given invoice support, so
// tests can script the live settings store alongside the renderer.
func newHarnessWithSupport(t *testing.T, support *ordersapi.InvoiceSupport) *harness {
	t.Helper()

	store := ordersmocks.NewMockOrderStore(t)
	cart := ordersmocks.NewMockCartSource(t)
	content := ordersmocks.NewMockContentSource(t)
	mail := ordersmocks.NewMockMailer(t)
	inventory := ordersmocks.NewMockInventory(t)
	service := ordersdomain.NewService(store, cart, content, mail, inventory, "IDR")
	return &harness{
		store:     store,
		cart:      cart,
		content:   content,
		mail:      mail,
		inventory: inventory,
		router:    testRouter(ordersapi.New(service, adminResolver(), support)),
	}
}

func newPayeeHarness(t *testing.T, payee *ordersapi.ManualPayee) *harness {
	t.Helper()

	store := ordersmocks.NewMockOrderStore(t)
	cart := ordersmocks.NewMockCartSource(t)
	content := ordersmocks.NewMockContentSource(t)
	mail := ordersmocks.NewMockMailer(t)
	inventory := ordersmocks.NewMockInventory(t)
	service := ordersdomain.NewService(store, cart, content, mail, inventory, "IDR")
	handler := ordersapi.New(service, adminResolver(), nil)
	handler.SetManualPayee(payee)
	return &harness{
		store:     store,
		cart:      cart,
		content:   content,
		mail:      mail,
		inventory: inventory,
		router:    testRouter(handler),
	}
}

func TestHandler_TrackOrder_ManualPayee(t *testing.T) {
	tests := []struct {
		name      string
		status    ordersdomain.Status
		payee     *ordersapi.ManualPayee
		wantPayee bool
	}{
		{
			name:   "pending order with configured payee carries the block",
			status: ordersdomain.StatusPending,
			payee: &ordersapi.ManualPayee{
				Bank:    "BCA",
				Account: "1234567890",
				Name:    "PT Lesstruct Demo",
			},
			wantPayee: true,
		},
		{
			name:   "paid order omits the payee block",
			status: ordersdomain.StatusPaid,
			payee: &ordersapi.ManualPayee{
				Bank:    "BCA",
				Account: "1234567890",
				Name:    "PT Lesstruct Demo",
			},
			wantPayee: false,
		},
		{
			name:      "pending order without configured payee carries no block",
			status:    ordersdomain.StatusPending,
			payee:     nil,
			wantPayee: false,
		},
		{
			name:   "pending order with a partial payee carries no block",
			status: ordersdomain.StatusPending,
			payee: &ordersapi.ManualPayee{
				Bank: "BCA",
			},
			wantPayee: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newPayeeHarness(t, tt.payee)
			order := storedOrder()
			order.Status = tt.status
			h.store.On("FindByNumberAndEmailAndToken", mock.Anything, int64(42), "shopper@example.com", trackingTokenHash).Return(order, nil)

			w := serve(t, h.router, http.MethodGet, "/public/orders/ORD-000042?email=shopper%40example.com&token=track-token", nil, "")

			require.Equal(t, http.StatusOK, w.Code)
			data := decodeData(t, w)
			payee, _ := data["manualPayee"].(map[string]any)
			if !tt.wantPayee {
				assert.Nil(t, data["manualPayee"], "payee block must be absent")
				return
			}
			require.NotNil(t, payee, "payee block must be present")
			assert.Equal(t, "BCA", payee["bank"])
			assert.Equal(t, "1234567890", payee["account"])
			assert.Equal(t, "PT Lesstruct Demo", payee["name"])
		})
	}
}

func TestHandler_ListOrders_ItemCount(t *testing.T) {
	h := newHarness(t, adminResolver())
	first := storedOrder()
	first.Items = nil
	first.ItemCount = 3
	second := storedOrder()
	second.ID = 41
	second.Items = nil
	second.ItemCount = 1
	h.store.On("List", mock.Anything, (*ordersdomain.Status)(nil), int64(0), 50).Return([]ordersdomain.Order{*first, *second}, nil)

	w := serve(t, h.router, http.MethodGet, "/admin/orders", principal(), "")

	require.Equal(t, http.StatusOK, w.Code)
	items, _ := decodeListResponse(t, w)
	require.Len(t, items, 2)
	firstRow, _ := items[0].(map[string]any)
	secondRow, _ := items[1].(map[string]any)
	assert.Equal(t, float64(3), firstRow["itemCount"], "list carries the read-time line count")
	assert.Equal(t, float64(1), secondRow["itemCount"])
}

// stubInvoiceSettingsStore is an in-memory live settings store: tests script
// the current values and a write failure.
type stubInvoiceSettingsStore struct {
	settings ordersdomain.InvoiceSettings
	err      error
}

func (s *stubInvoiceSettingsStore) InvoiceSettings() ordersdomain.InvoiceSettings {
	return s.settings
}

func (s *stubInvoiceSettingsStore) SetInvoiceSettings(settings ordersdomain.InvoiceSettings) error {
	if s.err != nil {
		return s.err
	}
	s.settings = settings
	return nil
}

func TestHandler_InvoiceSettings(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		body     string
		store    *stubInvoiceSettingsStore
		who      *kernel.Principal
		wantCode int
		wantErr  string
		wantRate float64
		wantName string
	}{
		{
			name:     "GET returns the live settings",
			method:   http.MethodGet,
			store:    &stubInvoiceSettingsStore{settings: ordersdomain.InvoiceSettings{SellerName: "Demo", TaxRate: 11}},
			who:      principal(),
			wantCode: http.StatusOK,
			wantRate: 11,
			wantName: "Demo",
		},
		{
			name:     "PUT persists the values and trims the identity",
			method:   http.MethodPut,
			body:     `{"sellerName":"  Demo  ","sellerAddress":"Jl. X","sellerTaxId":"01.234","taxRate":11}`,
			store:    &stubInvoiceSettingsStore{settings: ordersdomain.InvoiceSettings{TaxRate: 12}},
			who:      principal(),
			wantCode: http.StatusOK,
			wantRate: 11,
			wantName: "Demo",
		},
		{
			name:     "an explicit zero rate survives",
			method:   http.MethodPut,
			body:     `{"sellerName":"","sellerAddress":"","sellerTaxId":"","taxRate":0}`,
			store:    &stubInvoiceSettingsStore{settings: ordersdomain.InvoiceSettings{TaxRate: 12}},
			who:      principal(),
			wantCode: http.StatusOK,
			wantRate: 0,
		},
		{
			name:     "a missing rate is refused",
			method:   http.MethodPut,
			body:     `{"sellerName":"Demo"}`,
			store:    &stubInvoiceSettingsStore{},
			who:      principal(),
			wantCode: http.StatusBadRequest,
			wantErr:  "VALIDATION_ERROR",
		},
		{
			name:     "an out-of-range rate is refused",
			method:   http.MethodPut,
			body:     `{"taxRate":101}`,
			store:    &stubInvoiceSettingsStore{},
			who:      principal(),
			wantCode: http.StatusBadRequest,
			wantErr:  "VALIDATION_ERROR",
		},
		{
			name:     "a malformed body is refused",
			method:   http.MethodPut,
			body:     `{"taxRate":`,
			store:    &stubInvoiceSettingsStore{},
			who:      principal(),
			wantCode: http.StatusBadRequest,
			wantErr:  "INVALID_REQUEST",
		},
		{
			name:     "a store failure reads as internal",
			method:   http.MethodPut,
			body:     `{"taxRate":11}`,
			store:    &stubInvoiceSettingsStore{err: errors.New("disk full")},
			who:      principal(),
			wantCode: http.StatusInternalServerError,
			wantErr:  "INTERNAL_ERROR",
		},
		{
			name:     "an unconfigured store answers unavailable",
			method:   http.MethodGet,
			who:      principal(),
			wantCode: http.StatusServiceUnavailable,
			wantErr:  "INVOICE_SETTINGS_UNAVAILABLE",
		},
		{
			name:     "an anonymous caller is refused",
			method:   http.MethodPut,
			body:     `{"taxRate":11}`,
			store:    &stubInvoiceSettingsStore{},
			wantCode: http.StatusForbidden,
			wantErr:  "INSUFFICIENT_PERMISSIONS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var support *ordersapi.InvoiceSupport
			if tt.store != nil {
				support = &ordersapi.InvoiceSupport{Store: tt.store}
			}
			h := newHarnessWithSupport(t, support)

			w := serve(t, h.router, tt.method, "/admin/orders/invoice-settings", tt.who, tt.body)

			assert.Equal(t, tt.wantCode, w.Code)
			if tt.wantErr != "" {
				assert.Equal(t, tt.wantErr, decodeErrorCode(t, w))
				return
			}
			data := decodeData(t, w)
			assert.Equal(t, tt.wantRate, data["taxRate"])
			assert.Equal(t, tt.wantName, data["sellerName"])
		})
	}
}

func TestHandler_InvoiceSettings_LiveRender(t *testing.T) {
	renderer := &stubRenderer{out: []byte("%PDF-fake")}
	store := &stubInvoiceSettingsStore{settings: ordersdomain.InvoiceSettings{TaxRate: 12}}
	h := newHarnessWithSupport(t, &ordersapi.InvoiceSupport{Renderer: renderer, Store: store})
	h.store.On("FindByID", mock.Anything, int64(42)).Return(storedOrder(), nil)

	w := serve(t, h.router, http.MethodPut, "/admin/orders/invoice-settings", principal(), `{"taxRate":5}`)
	require.Equal(t, http.StatusOK, w.Code)

	w = serve(t, h.router, http.MethodGet, "/admin/orders/42/invoice", principal(), "")
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, renderer.doc)
	assert.Equal(t, float64(5), renderer.doc.TaxRate, "the next invoice reflects the admin edit without a restart")
}

func TestHandler_InvoiceOrder(t *testing.T) {
	t.Run("streams the pdf with download headers", func(t *testing.T) {
		renderer := &stubRenderer{out: []byte("%PDF-fake")}
		h := newInvoiceHarness(t, renderer)
		charged := storedOrder()
		charged.ShippingMinor = 15000
		h.store.On("FindByID", mock.Anything, int64(42)).Return(charged, nil)

		w := serve(t, h.router, http.MethodGet, "/admin/orders/42/invoice", principal(), "")

		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
		assert.Equal(t, `inline; filename="INV-000042.pdf"`, w.Header().Get("Content-Disposition"))
		assert.Equal(t, []byte("%PDF-fake"), w.Body.Bytes())
		require.NotNil(t, renderer.doc)
		assert.Equal(t, ordersdomain.NewMoney(613000, "IDR"), renderer.doc.Total)
		assert.Equal(t, float64(0), renderer.doc.TaxRate)
	})

	t.Run("an unconfigured renderer answers unavailable", func(t *testing.T) {
		h := newInvoiceHarness(t, nil)

		w := serve(t, h.router, http.MethodGet, "/admin/orders/42/invoice", principal(), "")

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		assert.Equal(t, "INVOICE_UNAVAILABLE", decodeErrorCode(t, w))
	})

	t.Run("an unknown order reads as not found", func(t *testing.T) {
		h := newInvoiceHarness(t, &stubRenderer{out: []byte("%PDF-fake")})
		h.store.On("FindByID", mock.Anything, int64(42)).Return(nil, ordersdomain.ErrOrderNotFound)

		w := serve(t, h.router, http.MethodGet, "/admin/orders/42/invoice", principal(), "")

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "NOT_FOUND", decodeErrorCode(t, w))
	})

	t.Run("a renderer failure reads as internal", func(t *testing.T) {
		h := newInvoiceHarness(t, &stubRenderer{err: errors.New("maroto down")})
		h.store.On("FindByID", mock.Anything, int64(42)).Return(storedOrder(), nil)

		w := serve(t, h.router, http.MethodGet, "/admin/orders/42/invoice", principal(), "")

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, "INTERNAL_ERROR", decodeErrorCode(t, w))
	})

	t.Run("a malformed id is refused", func(t *testing.T) {
		h := newInvoiceHarness(t, &stubRenderer{out: []byte("%PDF-fake")})

		w := serve(t, h.router, http.MethodGet, "/admin/orders/nope/invoice", principal(), "")

		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("an anonymous caller is refused", func(t *testing.T) {
		h := newInvoiceHarness(t, &stubRenderer{out: []byte("%PDF-fake")})

		w := serve(t, h.router, http.MethodGet, "/admin/orders/42/invoice", nil, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "INSUFFICIENT_PERMISSIONS", decodeErrorCode(t, w))
	})
}
