package api

import (
	"net/http"
	"strconv"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/capability"
	httpx "github.com/aristorinjuang/lesstruct/kernel/httpx"
	paymentsdomain "github.com/aristorinjuang/lesstruct/modules/payments/domain"
)

// parsePaymentID extracts and validates the {paymentID} path value.
func parsePaymentID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("paymentID"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid payment ID", nil)
		return 0, false
	}
	return id, true
}

// Handler is the payment module's HTTP surface. The gateway (nil when
// the store disabled hosted checkout) backs the session mint and the
// webhook authentication.
type Handler struct {
	service *paymentsdomain.Service
	gateway paymentsdomain.Gateway
	resolve kernel.CapabilityResolver
}

// authorize checks the module capability and returns the acting
// administrator's username for the record audit trail. Without a
// resolver every admin request is refused (fail closed).
func (h *Handler) authorize(w http.ResponseWriter, r *http.Request) (string, bool) {
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok || h.resolve == nil || !h.resolve(principal, string(capability.CapabilityAdmin)) {
		httpx.Error(w, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "You do not have permission to manage payments", nil)
		return "", false
	}
	return principal.Username, true
}

// ListPayments returns the newest payment records carrying the optional
// status filter, at the keyset cursor, at most limit rows.
func (h *Handler) ListPayments(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r); !ok {
		return
	}

	limit := httpx.ParseListLimit(r)
	var status *paymentsdomain.Status
	if raw := r.URL.Query().Get("status"); raw != "" {
		parsed, ok := paymentsdomain.ParseStatus(raw)
		if !ok {
			httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unknown payment status", nil)
			return
		}
		status = &parsed
	}
	beforeID := int64(0)
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		decoded, err := httpx.DecodeCursor(raw)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid cursor", nil)
			return
		}
		beforeID = int64(decoded)
	}

	// An orderId narrows the read to one order's payment history: short
	// and unpaginated, the status and cursor filters do not apply.
	if raw := r.URL.Query().Get("orderId"); raw != "" {
		orderID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || orderID <= 0 {
			httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid order ID", nil)
			return
		}
		payments, err := h.service.ListByOrder(r.Context(), orderID)
		if err != nil {
			mapError(w, err)
			return
		}
		out := make([]paymentPayload, 0, len(payments))
		for i := range payments {
			out = append(out, projectPayment(&payments[i]))
		}
		httpx.SuccessList(w, out, map[string]any{
			"pagination": map[string]any{
				"nextCursor": "",
				"hasMore":    false,
			},
		})
		return
	}

	payments, err := h.service.List(r.Context(), status, beforeID, limit)
	if err != nil {
		mapError(w, err)
		return
	}

	out := make([]paymentPayload, 0, len(payments))
	nextCursor := ""
	hasMore := false
	if len(payments) == limit && len(payments) > 0 {
		hasMore = true
		nextCursor = httpx.EncodeCursor(int(payments[len(payments)-1].ID))
	}
	for i := range payments {
		out = append(out, projectPayment(&payments[i]))
	}
	httpx.SuccessList(w, out, map[string]any{
		"pagination": map[string]any{
			"nextCursor": nextCursor,
			"hasMore":    hasMore,
		},
	})
}

// GetPayment returns one payment record with its order context.
func (h *Handler) GetPayment(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r); !ok {
		return
	}
	id, ok := parsePaymentID(w, r)
	if !ok {
		return
	}

	detail, err := h.service.Detail(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	httpx.Success(w, projectPaymentDetail(detail))
}

// VerifyPayment decides a pending proof for the positive: the order flips
// to paid and the record carries the deciding administrator.
func (h *Handler) VerifyPayment(w http.ResponseWriter, r *http.Request) {
	verifiedBy, ok := h.authorize(w, r)
	if !ok {
		return
	}
	id, ok := parsePaymentID(w, r)
	if !ok {
		return
	}

	payment, err := h.service.Verify(r.Context(), id, verifiedBy)
	if err != nil {
		mapError(w, err)
		return
	}
	httpx.Success(w, projectPayment(payment))
}

// RejectPayment decides a pending proof for the negative: the order stays
// pending and the record carries the rejection.
func (h *Handler) RejectPayment(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r); !ok {
		return
	}
	id, ok := parsePaymentID(w, r)
	if !ok {
		return
	}

	payment, err := h.service.Reject(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	httpx.Success(w, projectPayment(payment))
}

// New builds the payment HTTP handlers over the adopted service and the
// optional gateway driver.
func New(service *paymentsdomain.Service, gateway paymentsdomain.Gateway, resolve kernel.CapabilityResolver) *Handler {
	return &Handler{service: service, gateway: gateway, resolve: resolve}
}
