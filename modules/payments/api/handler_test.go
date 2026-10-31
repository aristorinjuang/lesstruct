package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	paymentsapi "github.com/aristorinjuang/lesstruct/modules/payments/api"
	paymentsdomain "github.com/aristorinjuang/lesstruct/modules/payments/domain"
	paymentsmocks "github.com/aristorinjuang/lesstruct/modules/payments/domain/mocks"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// testRouter mounts both surfaces exactly as the module and composition
// root do: two route trees, the public realm and the JWT-protected admin
// realm, so path params never collide across realms.
func testRouter(handler *paymentsapi.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/public", func(public chi.Router) {
		public.Get("/payments/methods", handler.ListMethods)
		public.Post("/orders/{orderNumber}/payments", handler.SubmitPayment)
		public.Post("/orders/{orderNumber}/payments/session", handler.MintPayment)
		public.Post("/payments/midtrans/webhook", handler.MidtransWebhook)
	})
	r.Route("/admin", func(admin chi.Router) {
		admin.Get("/payments", handler.ListPayments)
		admin.Get("/payments/{paymentID}", handler.GetPayment)
		admin.Post("/payments/{paymentID}/verify", handler.VerifyPayment)
		admin.Post("/payments/{paymentID}/reject", handler.RejectPayment)
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
	req := httptest.NewRequest(method, path, strings.NewReader(body))
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

// adminResolver grants the core admin capability the users module carries.
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
	store     *paymentsmocks.MockPaymentsStore
	orders    *paymentsmocks.MockOrders
	mail      *paymentsmocks.MockMailer
	gateway   *paymentsmocks.MockGateway
	router    *chi.Mux
	adminNoop *chi.Mux
}

func newHarness(t *testing.T, resolve kernel.CapabilityResolver) *harness {
	t.Helper()
	store := paymentsmocks.NewMockPaymentsStore(t)
	orders := paymentsmocks.NewMockOrders(t)
	mail := paymentsmocks.NewMockMailer(t)
	gateway := paymentsmocks.NewMockGateway(t)
	service := paymentsdomain.NewPaymentsService(
		store,
		orders,
		mail,
		gateway,
		[]paymentsdomain.Method{paymentsdomain.MethodManual},
	)
	return &harness{
		store:     store,
		orders:    orders,
		mail:      mail,
		gateway:   gateway,
		router:    testRouter(paymentsapi.New(service, gateway, resolve)),
		adminNoop: testRouter(paymentsapi.New(service, gateway, denyResolver())),
	}
}

const (
	// maxBodyBytes mirrors the handler's body cap so the oversized-input
	// case can exceed it.
	maxBodyBytes = 1 << 20
)

// pendingRecord snapshots a pending manual proof.
func pendingRecord() *paymentsdomain.Payment {
	return &paymentsdomain.Payment{
		ID:        7,
		OrderID:   42,
		Method:    paymentsdomain.MethodManual,
		Status:    paymentsdomain.StatusPending,
		Reference: "BR-12345",
		Amount:    522500,
	}
}

// trackingToken is the per-order secret every guest gate expectation
// presents. The value is arbitrary — the handler only forwards it.
const trackingToken = "track-token"

// payable returns the checkout gate's pending order view.
func payable() *paymentsdomain.OrderRef {
	return &paymentsdomain.OrderRef{
		ID:            42,
		OrderNumber:   "ORD-000042",
		Status:        "pending",
		TotalMinor:    522500,
		CustomerEmail: "shopper@example.com",
		Currency:      "IDR",
	}
}

func TestListMethods(t *testing.T) {
	t.Run("returns the adopted methods", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodGet, "/public/payments/methods", nil, "")

		require.Equal(t, http.StatusOK, w.Code)
		data := decodeData(t, w)
		methods, _ := data["methods"].([]any)
		assert.Equal(t, []any{"manual"}, methods)
	})
}

func TestSubmitPayment(t *testing.T) {
	const body = `{"email":"shopper@example.com","token":"track-token","method":"manual","reference":"BR-12345"}`

	t.Run("records the proof and echoes the record", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.orders.On("Details", mock.Anything, "ORD-000042", "shopper@example.com", trackingToken).Return(payable(), nil)
		h.store.On("Insert", mock.Anything, mock.Anything).Return(int64(7), nil)
		h.store.On("FindByID", mock.Anything, int64(7)).Return(pendingRecord(), nil)
		h.mail.On("SendPaymentReceivedEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		w := serve(t, h.router, http.MethodPost, "/public/orders/ORD-000042/payments", nil, body)

		require.Equal(t, http.StatusOK, w.Code)
		data := decodeData(t, w)
		assert.Equal(t, float64(7), data["id"])
		assert.Equal(t, float64(42), data["orderId"])
		assert.Equal(t, "manual", data["method"])
		assert.Equal(t, "pending", data["status"])
		assert.Equal(t, "BR-12345", data["reference"])
		assert.Equal(t, float64(522500), data["amount"])
	})

	t.Run("an unreadable body is a validation error", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodPost, "/public/orders/ORD-000042/payments", nil, "{not json}")

		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("an oversized body is refused before decode", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodPost, "/public/orders/ORD-000042/payments", nil,
			fmt.Sprintf(`{"email":"shopper@example.com","method":"manual","reference":"%s"}`, strings.Repeat("x", maxBodyBytes)))

		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("domain validation renders as validation errors", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodPost, "/public/orders/ORD-000042/payments", nil,
			`{"email":"shopper@example.com","method":"manual","reference":"  "}`)

		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("an unresolvable order reads as not found", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.orders.On("Details", mock.Anything, "ORD-000042", "shopper@example.com", trackingToken).Return(nil, paymentsdomain.ErrOrderNotFound)

		w := serve(t, h.router, http.MethodPost, "/public/orders/ORD-000042/payments", nil, body)

		require.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "NOT_FOUND", decodeErrorCode(t, w))
	})

	t.Run("a settled order refuses proofs", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.orders.On("Details", mock.Anything, "ORD-000042", "shopper@example.com", trackingToken).Return(nil, paymentsdomain.ErrOrderNotPayable)

		w := serve(t, h.router, http.MethodPost, "/public/orders/ORD-000042/payments", nil, body)

		require.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "ORDER_NOT_PAYABLE", decodeErrorCode(t, w))
	})

	t.Run("storage failures render internal", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.orders.On("Details", mock.Anything, "ORD-000042", "shopper@example.com", trackingToken).Return(payable(), nil)
		h.store.On("Insert", mock.Anything, mock.Anything).Return(int64(0), errors.New("db down"))

		w := serve(t, h.router, http.MethodPost, "/public/orders/ORD-000042/payments", nil, body)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, "INTERNAL_ERROR", decodeErrorCode(t, w))
	})
}

func TestListPayments(t *testing.T) {
	t.Run("anonymous requests refuse", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodGet, "/admin/payments", nil, "")

		require.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "INSUFFICIENT_PERMISSIONS", decodeErrorCode(t, w))
	})

	t.Run("listed payments carry pagination metadata", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		records := []paymentsdomain.Payment{*pendingRecord()}
		h.store.On("List", mock.Anything, (*paymentsdomain.Status)(nil), int64(0), 50).Return(records, nil)

		w := serve(t, h.router, http.MethodGet, "/admin/payments", principal(), "")

		require.Equal(t, http.StatusOK, w.Code)
		items, meta := decodeListResponse(t, w)
		require.Len(t, items, 1)
		assert.Equal(t, float64(7), items[0].(map[string]any)["id"])
		pagination, _ := meta["pagination"].(map[string]any)
		assert.Equal(t, false, pagination["hasMore"])
		assert.Equal(t, "", pagination["nextCursor"])
	})

	t.Run("a full page advertises continuation", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		records := make([]paymentsdomain.Payment, 0, 2)
		records = append(records, *pendingRecord(), paymentsdomain.Payment{
			ID:        6,
			OrderID:   41,
			Method:    paymentsdomain.MethodManual,
			Status:    paymentsdomain.StatusPending,
			Reference: "BR-00099",
			Amount:    100000,
		})
		h.store.On("List", mock.Anything, (*paymentsdomain.Status)(nil), int64(0), 2).Return(records, nil)

		w := serve(t, h.router, http.MethodGet, "/admin/payments?limit=2", principal(), "")

		require.Equal(t, http.StatusOK, w.Code)
		items, meta := decodeListResponse(t, w)
		require.Len(t, items, 2)
		pagination, _ := meta["pagination"].(map[string]any)
		assert.Equal(t, true, pagination["hasMore"])
		assert.Equal(t, "Ng", pagination["nextCursor"]) // keyset 6 → base64("6")
	})

	t.Run("an unknown status filter is a validation error", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodGet, "/admin/payments?status=paid", principal(), "")

		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("an invalid cursor is a validation error", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodGet, "/admin/payments?cursor=zap", principal(), "")

		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("the status filter reaches the store", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		verified := paymentsdomain.StatusVerified
		h.store.On("List", mock.Anything, &verified, int64(0), 50).Return([]paymentsdomain.Payment{}, nil)

		w := serve(t, h.router, http.MethodGet, "/admin/payments?status=verified", principal(), "")

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("an orderId narrows the read to one order history", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		records := []paymentsdomain.Payment{*pendingRecord()}
		h.store.On("ListByOrder", mock.Anything, int64(42)).Return(records, nil)

		w := serve(t, h.router, http.MethodGet, "/admin/payments?orderId=42", principal(), "")

		require.Equal(t, http.StatusOK, w.Code)
		items, meta := decodeListResponse(t, w)
		require.Len(t, items, 1)
		assert.Equal(t, float64(7), items[0].(map[string]any)["id"])
		pagination, _ := meta["pagination"].(map[string]any)
		assert.Equal(t, false, pagination["hasMore"])
	})

	t.Run("a malformed orderId is a validation error", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodGet, "/admin/payments?orderId=nope", principal(), "")

		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})
}

func TestGetPayment(t *testing.T) {
	t.Run("returns the record with its order context", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(7)).Return(pendingRecord(), nil)
		h.orders.On("DetailsByID", mock.Anything, int64(42)).Return(payable(), nil)

		w := serve(t, h.router, http.MethodGet, "/admin/payments/7", principal(), "")

		require.Equal(t, http.StatusOK, w.Code)
		data := decodeData(t, w)
		assert.Equal(t, float64(7), data["id"])
		assert.Equal(t, "BR-12345", data["reference"])
		order, _ := data["order"].(map[string]any)
		require.NotNil(t, order, "the detail carries the order context")
		assert.Equal(t, "ORD-000042", order["orderNumber"])
		assert.Equal(t, "pending", order["status"])
		assert.Equal(t, float64(522500), order["totalMinor"])
	})

	t.Run("a malformed id is a validation error", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodGet, "/admin/payments/nan", principal(), "")

		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})

	t.Run("an unknown record reads as not found", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(7)).Return(nil, paymentsdomain.ErrPaymentNotFound)

		w := serve(t, h.router, http.MethodGet, "/admin/payments/7", principal(), "")

		require.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "NOT_FOUND", decodeErrorCode(t, w))
	})

	t.Run("an unresolvable order passes through", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(7)).Return(pendingRecord(), nil)
		h.orders.On("DetailsByID", mock.Anything, int64(42)).Return(nil, paymentsdomain.ErrOrderNotFound)

		w := serve(t, h.router, http.MethodGet, "/admin/payments/7", principal(), "")

		require.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "NOT_FOUND", decodeErrorCode(t, w))
	})
}

func TestVerifyPayment(t *testing.T) {
	verified := func() paymentsdomain.Payment {
		r := *pendingRecord()
		r.Status = paymentsdomain.StatusVerified
		r.VerifiedBy = "admin"
		return r
	}()

	t.Run("confirms the order and echoes the decision", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(7)).Return(pendingRecord(), nil).Once()
		h.orders.On("DetailsByID", mock.Anything, int64(42)).Return(payable(), nil)
		h.orders.On("Confirm", mock.Anything, int64(42)).Return(payable(), nil)
		h.store.On("SetStatus", mock.Anything, int64(7), paymentsdomain.StatusVerified, "admin").Return(nil)
		h.store.On("FindByID", mock.Anything, int64(7)).Return(&verified, nil).Once()

		w := serve(t, h.router, http.MethodPost, "/admin/payments/7/verify", principal(), "")

		require.Equal(t, http.StatusOK, w.Code)
		data := decodeData(t, w)
		assert.Equal(t, "verified", data["status"])
		assert.Equal(t, "admin", data["verifiedBy"])
	})

	t.Run("a decided record conflicts", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(7)).Return(&verified, nil)

		w := serve(t, h.router, http.MethodPost, "/admin/payments/7/verify", principal(), "")

		require.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "PAYMENT_NOT_PENDING", decodeErrorCode(t, w))
	})

	t.Run("a stock shortfall conflicts", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(7)).Return(pendingRecord(), nil)
		h.orders.On("DetailsByID", mock.Anything, int64(42)).Return(payable(), nil)
		h.orders.On("Confirm", mock.Anything, int64(42)).Return(nil, paymentsdomain.ErrInsufficientStock)

		w := serve(t, h.router, http.MethodPost, "/admin/payments/7/verify", principal(), "")

		require.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "OUT_OF_STOCK", decodeErrorCode(t, w))
	})

	t.Run("an unpayable order conflicts", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(7)).Return(pendingRecord(), nil)
		h.orders.On("DetailsByID", mock.Anything, int64(42)).Return(payable(), nil)
		h.orders.On("Confirm", mock.Anything, int64(42)).Return(nil, paymentsdomain.ErrOrderNotPayable)

		w := serve(t, h.router, http.MethodPost, "/admin/payments/7/verify", principal(), "")

		require.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "ORDER_NOT_PAYABLE", decodeErrorCode(t, w))
	})

	t.Run("a gateway-paid order refuses a second verified record", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(7)).Return(pendingRecord(), nil)
		paid := *payable()
		paid.Status = "paid"
		h.orders.On("DetailsByID", mock.Anything, int64(42)).Return(&paid, nil)

		w := serve(t, h.router, http.MethodPost, "/admin/payments/7/verify", principal(), "")

		require.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "ORDER_NOT_PAYABLE", decodeErrorCode(t, w))
	})
}

func TestRejectPayment(t *testing.T) {
	rejected := func() paymentsdomain.Payment {
		r := *pendingRecord()
		r.Status = paymentsdomain.StatusRejected
		return r
	}()

	t.Run("rejection echoes the decision", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(7)).Return(pendingRecord(), nil).Once()
		h.orders.On("DetailsByID", mock.Anything, int64(42)).Return(payable(), nil)
		h.store.On("SetStatus", mock.Anything, int64(7), paymentsdomain.StatusRejected, "").Return(nil)
		h.store.On("FindByID", mock.Anything, int64(7)).Return(&rejected, nil).Once()
		h.mail.On("SendPaymentRejectedEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		w := serve(t, h.router, http.MethodPost, "/admin/payments/7/reject", principal(), "")

		require.Equal(t, http.StatusOK, w.Code)
		data := decodeData(t, w)
		assert.Equal(t, "rejected", data["status"])
	})

	t.Run("unauthorized requests refuse", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodPost, "/admin/payments/7/reject", nil, "")

		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("a denied capability refuses", func(t *testing.T) {
		h := newHarness(t, denyResolver())

		w := serve(t, h.router, http.MethodPost, "/admin/payments/7/reject", principal(), "")

		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("a decided record conflicts", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(7)).Return(&rejected, nil)

		w := serve(t, h.router, http.MethodPost, "/admin/payments/7/reject", principal(), "")

		require.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "PAYMENT_NOT_PENDING", decodeErrorCode(t, w))
	})

	t.Run("a gateway-paid order refuses the rejection email", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.store.On("FindByID", mock.Anything, int64(7)).Return(pendingRecord(), nil)
		paid := *payable()
		paid.Status = "paid"
		h.orders.On("DetailsByID", mock.Anything, int64(42)).Return(&paid, nil)

		w := serve(t, h.router, http.MethodPost, "/admin/payments/7/reject", principal(), "")

		require.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "ORDER_NOT_PAYABLE", decodeErrorCode(t, w))
	})
}

func TestMintPayment(t *testing.T) {
	const body = `{"email":"shopper@example.com","token":"track-token"}`

	t.Run("mints a hosted session for a payable order", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.orders.On("Details", mock.Anything, "ORD-000042", "shopper@example.com", trackingToken).Return(payable(), nil)
		h.gateway.On("Mint", mock.Anything, payable()).Return(&paymentsdomain.GatewaySession{
			Token:       "snap-token",
			RedirectURL: "https://app.sandbox.midtrans.com/snap/v2/vtwidget?sometoken",
		}, nil)

		w := serve(t, h.router, http.MethodPost, "/public/orders/ORD-000042/payments/session", nil, body)

		require.Equal(t, http.StatusOK, w.Code)
		data := decodeData(t, w)
		assert.Equal(t, "snap-token", data["token"])
		assert.Contains(t, data["redirectUrl"], "sandbox")
	})

	t.Run("an unreadable body is a validation error", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodPost, "/public/orders/ORD-000042/payments/session", nil, "{oops")

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("a settled order refuses to mint", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.orders.On("Details", mock.Anything, "ORD-000042", "shopper@example.com", trackingToken).Return(nil, paymentsdomain.ErrOrderNotPayable)

		w := serve(t, h.router, http.MethodPost, "/public/orders/ORD-000042/payments/session", nil, body)

		require.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "ORDER_NOT_PAYABLE", decodeErrorCode(t, w))
	})

	t.Run("an unresolvable order reads as not found", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.orders.On("Details", mock.Anything, "ORD-000042", "shopper@example.com", trackingToken).Return(nil, paymentsdomain.ErrOrderNotFound)

		w := serve(t, h.router, http.MethodPost, "/public/orders/ORD-000042/payments/session", nil, body)

		require.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "NOT_FOUND", decodeErrorCode(t, w))
	})
}

func TestSubmitPayment_MailFailuresStaySilent(t *testing.T) {
	h := newHarness(t, adminResolver())
	h.orders.On("Details", mock.Anything, "ORD-000042", "shopper@example.com", trackingToken).Return(payable(), nil)
	h.store.On("Insert", mock.Anything, mock.Anything).Return(int64(7), nil)
	h.store.On("FindByID", mock.Anything, int64(7)).Return(pendingRecord(), nil)
	h.mail.On("SendPaymentReceivedEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(context.DeadlineExceeded)

	w := serve(t, h.router, http.MethodPost, "/public/orders/ORD-000042/payments", nil,
		`{"email":"shopper@example.com","token":"track-token","method":"manual","reference":"BR-12345"}`)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestMidtransWebhook(t *testing.T) {
	const signedBody = `{"order_id":"ORD-000042","status_code":"200","gross_amount":"522500.00","signature_key":"sig","transaction_id":"T-1","transaction_status":"settlement"}`

	t.Run("a valid settlement folds and acknowledges", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.gateway.On("Authenticate", "ORD-000042", "200", "522500.00", "sig").Return(true)
		h.gateway.On("MinorUnits", "522500.00").Return(int64(522500), nil)
		h.store.On("FindByReference", mock.Anything, "T-1").Return(nil, paymentsdomain.ErrPaymentNotFound)
		h.orders.On("DetailsByNumber", mock.Anything, "ORD-000042").Return(payable(), nil)
		h.orders.On("Confirm", mock.Anything, int64(42)).Return(payable(), nil)
		h.store.On("Insert", mock.Anything, mock.MatchedBy(func(p *paymentsdomain.Payment) bool {
			return p.OrderID == 42 && p.Method == paymentsdomain.MethodMidtrans &&
				p.Status == paymentsdomain.StatusVerified && p.Amount == 522500 && p.VerifiedBy == "midtrans"
		})).Return(int64(8), nil)

		w := serve(t, h.router, http.MethodPost, "/public/payments/midtrans/webhook", nil, signedBody)

		require.Equal(t, http.StatusOK, w.Code)
		data := decodeData(t, w)
		assert.Equal(t, true, data["ok"])
	})

	t.Run("a forged signature refuses", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.gateway.On("Authenticate", "ORD-000042", "200", "522500.00", "sig").Return(false)

		w := serve(t, h.router, http.MethodPost, "/public/payments/midtrans/webhook", nil, signedBody)

		require.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "SIGNATURE_INVALID", decodeErrorCode(t, w))
	})

	t.Run("an unparseable amount refuses", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.gateway.On("Authenticate", "ORD-000042", "200", "522500.00", "sig").Return(true)
		h.gateway.On("MinorUnits", "522500.00").Return(int64(0), errors.New("unparseable"))

		w := serve(t, h.router, http.MethodPost, "/public/payments/midtrans/webhook", nil, signedBody)

		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "AMOUNT_MISMATCH", decodeErrorCode(t, w))
	})

	t.Run("an amount that disagrees with the order refuses", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.gateway.On("Authenticate", "ORD-000042", "200", "522500.00", "sig").Return(true)
		h.gateway.On("MinorUnits", "522500.00").Return(int64(12345), nil)
		h.store.On("FindByReference", mock.Anything, "T-1").Return(nil, paymentsdomain.ErrPaymentNotFound)
		h.orders.On("DetailsByNumber", mock.Anything, "ORD-000042").Return(payable(), nil)

		w := serve(t, h.router, http.MethodPost, "/public/payments/midtrans/webhook", nil, signedBody)

		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "AMOUNT_MISMATCH", decodeErrorCode(t, w))
	})

	t.Run("a refund folds the order lifecycle", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.gateway.On("Authenticate", "ORD-000042", "200", "522500.00", "sig").Return(true)
		h.gateway.On("MinorUnits", "522500.00").Return(int64(522500), nil)
		h.orders.On("DetailsByNumber", mock.Anything, "ORD-000042").Return(payable(), nil)
		h.orders.On("Refund", mock.Anything, int64(42)).Return(nil)

		refundBody := `{"order_id":"ORD-000042","status_code":"200","gross_amount":"522500.00","signature_key":"sig","transaction_id":"T-2","transaction_status":"refund"}`

		w := serve(t, h.router, http.MethodPost, "/public/payments/midtrans/webhook", nil, refundBody)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("an unknown status acknowledges without action", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.gateway.On("Authenticate", "ORD-000042", "201", "522500.00", "sig").Return(true)
		h.gateway.On("MinorUnits", "522500.00").Return(int64(522500), nil)

		expireBody := `{"order_id":"ORD-000042","status_code":"201","gross_amount":"522500.00","signature_key":"sig","transaction_id":"T-3","transaction_status":"expire"}`

		w := serve(t, h.router, http.MethodPost, "/public/payments/midtrans/webhook", nil, expireBody)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("a challenged capture acks without folding", func(t *testing.T) {
		h := newHarness(t, adminResolver())
		h.gateway.On("Authenticate", "ORD-000042", "200", "522500.00", "sig").Return(true)
		h.gateway.On("MinorUnits", "522500.00").Return(int64(522500), nil)

		challengeBody := `{"order_id":"ORD-000042","status_code":"200","gross_amount":"522500.00","signature_key":"sig","transaction_id":"T-4","transaction_status":"capture","fraud_status":"challenge"}`

		w := serve(t, h.router, http.MethodPost, "/public/payments/midtrans/webhook", nil, challengeBody)

		require.Equal(t, http.StatusOK, w.Code)
		data := decodeData(t, w)
		assert.Equal(t, true, data["ok"])
	})

	t.Run("an unreadable body is a validation error", func(t *testing.T) {
		h := newHarness(t, adminResolver())

		w := serve(t, h.router, http.MethodPost, "/public/payments/midtrans/webhook", nil, "{nope")

		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "VALIDATION_ERROR", decodeErrorCode(t, w))
	})
}
