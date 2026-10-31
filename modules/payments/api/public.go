// Package api hosts the payment module's surfaces: the public proof
// submission for manual transfers, hosted-session minting, and gateway
// webhooks (rate-limited, no auth chain) plus the admin payment list,
// detail, and decisions (JWT-protected, admin-gated). Everything writes
// through kernel/httpx — a new surface with no legacy envelope contract.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	httpx "github.com/aristorinjuang/lesstruct/kernel/httpx"
	paymentsdomain "github.com/aristorinjuang/lesstruct/modules/payments/domain"
)

const (
	// maxBodyBytes caps the JSON bodies the surfaces decode. The public
	// router rate-limits requests, not bytes, so the submission body —
	// served to anonymous traffic — must bound its own read; the admin
	// decision bodies ride the same guard for symmetry.
	maxBodyBytes = 1 << 20
)

// submitBody is the POST /orders/{orderNumber}/payments body: the guest
// gate (email plus the order's tracking token) plus the chosen method and
// the verbatim proof text.
type submitBody struct {
	Email     string `json:"email"`
	Token     string `json:"token"`
	Method    string `json:"method"`
	Reference string `json:"reference"`
}

// mintBody is the POST /orders/{orderNumber}/payments/session body: the
// guest gate rides the email and the order's tracking token.
type mintBody struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

// gatewayBody is the raw gateway webhook event: the fields the driver
// authenticates over (order id, status code, gross amount, signature)
// ride strings verbatim from the wire so the checksum and the amount
// conversion both see exactly what the gateway sent.
type gatewayBody struct {
	OrderID           string `json:"order_id"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	TransactionID     string `json:"transaction_id"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
}

// paymentPayload is the read shape of one payment record.
type paymentPayload struct {
	ID         int64     `json:"id"`
	OrderID    int64     `json:"orderId"`
	Method     string    `json:"method"`
	Status     string    `json:"status"`
	Reference  string    `json:"reference"`
	Amount     int64     `json:"amount"`
	VerifiedBy string    `json:"verifiedBy,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// sessionPayload is the minted hosted-checkout session.
type sessionPayload struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirectUrl"`
}

// projectPayment maps the domain record onto the response shape.
func projectPayment(payment *paymentsdomain.Payment) paymentPayload {
	return paymentPayload{
		ID:         payment.ID,
		OrderID:    payment.OrderID,
		Method:     string(payment.Method),
		Status:     string(payment.Status),
		Reference:  payment.Reference,
		Amount:     payment.Amount,
		VerifiedBy: payment.VerifiedBy,
		CreatedAt:  payment.CreatedAt,
		UpdatedAt:  payment.UpdatedAt,
	}
}

// paymentOrderPayload is the order context riding a payment detail read:
// the number an admin reads, the lifecycle state the decision depends
// on, and the payable total the proof amount answers to.
type paymentOrderPayload struct {
	OrderNumber string `json:"orderNumber"`
	Status      string `json:"status"`
	TotalMinor  int64  `json:"totalMinor"`
}

// paymentDetailPayload is the admin read shape of one payment record.
type paymentDetailPayload struct {
	paymentPayload
	Order paymentOrderPayload `json:"order"`
}

// projectPaymentDetail maps the domain detail onto the response shape.
func projectPaymentDetail(detail *paymentsdomain.PaymentDetail) paymentDetailPayload {
	return paymentDetailPayload{
		paymentPayload: projectPayment(&detail.Payment),
		Order: paymentOrderPayload{
			OrderNumber: detail.OrderNumber,
			Status:      detail.OrderStatus,
			TotalMinor:  detail.OrderTotal,
		},
	}
}

// decodeBody caps and decodes one JSON body; a failure replies
// VALIDATION_ERROR and reports false.
func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return false
	}
	return true
}

// mapError renders domain sentinels on both surfaces.
func mapError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, paymentsdomain.ErrInvalidReference),
		errors.Is(err, paymentsdomain.ErrInvalidMethod):
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
	case errors.Is(err, paymentsdomain.ErrOrderNotFound):
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Order not found", nil)
	case errors.Is(err, paymentsdomain.ErrOrderNotPayable):
		httpx.Error(w, http.StatusConflict, "ORDER_NOT_PAYABLE", "The order cannot accept payment proofs", nil)
	case errors.Is(err, paymentsdomain.ErrInsufficientStock):
		httpx.Error(w, http.StatusConflict, "OUT_OF_STOCK", "Item is out of stock", nil)
	case errors.Is(err, paymentsdomain.ErrPaymentNotPending):
		httpx.Error(w, http.StatusConflict, "PAYMENT_NOT_PENDING", err.Error(), nil)
	case errors.Is(err, paymentsdomain.ErrPaymentNotFound):
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Payment not found", nil)
	case errors.Is(err, paymentsdomain.ErrGatewayUnavailable):
		httpx.Error(w, http.StatusServiceUnavailable, "GATEWAY_UNAVAILABLE", "No payment gateway is configured", nil)
	case errors.Is(err, paymentsdomain.ErrGatewayUpstream):
		httpx.Error(w, http.StatusBadGateway, "GATEWAY_UPSTREAM", "The payment gateway call failed; the store is fine", nil)
	case errors.Is(err, paymentsdomain.ErrAmountMismatch):
		httpx.Error(w, http.StatusBadRequest, "AMOUNT_MISMATCH", err.Error(), nil)
	case errors.Is(err, paymentsdomain.ErrOrderNotRefundable):
		httpx.Error(w, http.StatusConflict, "ORDER_NOT_REFUNDABLE", err.Error(), nil)
	default:
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
	}
}

// ListMethods returns the offered payment methods: the static
// configuration the store adopted, never merged from submissions.
func (h *Handler) ListMethods(w http.ResponseWriter, r *http.Request) {
	methods := h.service.Methods()
	out := make([]string, 0, len(methods))
	for i := range methods {
		out = append(out, string(methods[i]))
	}
	httpx.Success(w, map[string]any{"methods": out})
}

// SubmitPayment records one manual transfer proof for a guest, gated on
// the order number, email, and tracking token matching.
func (h *Handler) SubmitPayment(w http.ResponseWriter, r *http.Request) {
	var body submitBody
	if !decodeBody(w, r, &body) {
		return
	}

	payment, err := h.service.Submit(
		r.Context(),
		r.PathValue("orderNumber"),
		body.Email,
		body.Token,
		paymentsdomain.Method(body.Method),
		body.Reference,
	)
	if err != nil {
		mapError(w, err)
		return
	}
	httpx.Success(w, projectPayment(payment))
}

// MintPayment opens a hosted checkout session for a payable order,
// gated on the order number, email, and tracking token matching like
// every guest surface. A store without a gateway driver answers 503.
func (h *Handler) MintPayment(w http.ResponseWriter, r *http.Request) {
	var body mintBody
	if !decodeBody(w, r, &body) {
		return
	}

	session, err := h.service.Mint(r.Context(), r.PathValue("orderNumber"), body.Email, body.Token)
	if err != nil {
		mapError(w, err)
		return
	}
	httpx.Success(w, sessionPayload{
		Token:       session.Token,
		RedirectURL: session.RedirectURL,
	})
}

// MidtransWebhook authenticates and folds one gateway event. The
// signature rides the raw strings, a checksum failure answers 403 so
// forged events never fold, and a malformed amount answers 400.
func (h *Handler) MidtransWebhook(w http.ResponseWriter, r *http.Request) {
	var body gatewayBody
	if !decodeBody(w, r, &body) {
		return
	}
	if h.gateway == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "GATEWAY_UNAVAILABLE", "No payment gateway is configured", nil)
		return
	}
	if !h.gateway.Authenticate(body.OrderID, body.StatusCode, body.GrossAmount, body.SignatureKey) {
		httpx.Error(w, http.StatusForbidden, "SIGNATURE_INVALID", "Webhook signature is invalid", nil)
		return
	}
	gross, err := h.gateway.MinorUnits(body.GrossAmount)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "AMOUNT_MISMATCH", "Unparseable gross amount", nil)
		return
	}

	event := &paymentsdomain.Notification{
		OrderNumber:   body.OrderID,
		TransactionID: body.TransactionID,
		Status:        body.TransactionStatus,
		FraudStatus:   body.FraudStatus,
		GrossMinor:    gross,
	}
	if err := h.service.HandleNotification(r.Context(), event); err != nil {
		mapError(w, err)
		return
	}
	httpx.Success(w, map[string]any{"ok": true})
}
