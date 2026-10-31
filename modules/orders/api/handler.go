// Package api hosts the order module's surfaces: the public checkout and
// guest tracking (rate-limited, no auth chain) and the admin order list,
// detail, and lifecycle transitions (JWT-protected, admin-gated).
// Everything writes through kernel/httpx — a new surface with no legacy
// envelope contract.
package api

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/capability"
	httpx "github.com/aristorinjuang/lesstruct/kernel/httpx"
	"github.com/aristorinjuang/lesstruct/kernel/legal"
	ordersdomain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
)

const (
	// cartTokenHeader carries the cart token on the checkout request,
	// mirroring the header the cart surface already uses.
	cartTokenHeader = "X-Cart-Token"
	// maxBodyBytes caps the JSON bodies the surfaces decode. The public
	// router rate-limits requests, not bytes, so the checkout body —
	// served to anonymous traffic — must bound its own read; the admin
	// transition rides the same guard for symmetry.
	maxBodyBytes = 1 << 20
)

// checkoutBody is the POST /checkout body: the cart to consume plus the
// customer contact and shipping address. The token rides the body when the
// client can set one, else the cart header.
type checkoutBody struct {
	Token           string               `json:"token,omitempty"`
	Email           string               `json:"email"`
	Name            string               `json:"name"`
	Phone           string               `json:"phone,omitempty"`
	ShipTo          ordersdomain.Address `json:"shipTo"`
	ConsentAccepted bool                 `json:"consentAccepted,omitempty"`
}

// transitionBody is the POST /orders/{orderID}/status body.
type transitionBody struct {
	Status string `json:"status"`
}

// shippingBody is the POST /orders/{orderID}/shipping body: the shipping
// line in minor units, replacing whatever the order carries.
type shippingBody struct {
	Amount int64 `json:"amount"`
}

// invoiceSettingsBody is the PUT /orders/invoice-settings body. taxRate
// rides a pointer so a missing key is distinguishable from an explicit zero.
type invoiceSettingsBody struct {
	SellerName    string   `json:"sellerName"`
	SellerAddress string   `json:"sellerAddress"`
	SellerTaxID   string   `json:"sellerTaxId"`
	TaxRate       *float64 `json:"taxRate"`
}

// invoiceSettingsPayload is the read shape of the live invoice block.
type invoiceSettingsPayload struct {
	SellerName    string  `json:"sellerName"`
	SellerAddress string  `json:"sellerAddress"`
	SellerTaxID   string  `json:"sellerTaxId"`
	TaxRate       float64 `json:"taxRate"`
}

// addressPayload is the read shape of the shipping address snapshot.
type addressPayload struct {
	Street     string `json:"street"`
	City       string `json:"city"`
	Province   string `json:"province"`
	PostalCode string `json:"postalCode"`
}

// linePayload is the read shape of one snapshotted order line: the frozen
// title, SKU, and unit price the live catalog cannot rewrite anymore.
type linePayload struct {
	ID               int64  `json:"id"`
	ProductContentID int    `json:"productContentId"`
	VariantID        *int64 `json:"variantId,omitempty"`
	SKU              string `json:"sku,omitempty"`
	Title            string `json:"title"`
	UnitPrice        int64  `json:"unitPrice"`
	Quantity         int    `json:"quantity"`
	LineTotal        int64  `json:"lineTotal"`
}

// manualPayeePayload is the read shape of the payee block on tracking
// payloads.
type manualPayeePayload struct {
	Bank    string `json:"bank"`
	Account string `json:"account"`
	Name    string `json:"name"`
}

// orderPayload is the read shape of one order. Detail reads fill items;
// list reads leave them an empty (never null) array and carry the
// read-time itemCount aggregate.
type orderPayload struct {
	ID            int64               `json:"id"`
	OrderNumber   string              `json:"orderNumber"`
	Status        ordersdomain.Status `json:"status"`
	CustomerEmail string              `json:"customerEmail"`
	CustomerName  string              `json:"customerName"`
	CustomerPhone string              `json:"customerPhone,omitempty"`
	ShipTo        addressPayload      `json:"shipTo"`
	Currency      string              `json:"currency"`
	Subtotal      int64               `json:"subtotal"`
	ShippingMinor int64               `json:"shippingMinor"`
	TotalMinor    int64               `json:"totalMinor"`
	Items         []linePayload       `json:"items"`
	ItemCount     int                 `json:"itemCount"`
	// TrackingToken is the raw per-order secret. It is populated only in
	// the checkout response — the one moment the caller receives it —
	// and never on later reads: lookups must present it, so it is not
	// something the API hands back.
	TrackingToken string              `json:"trackingToken,omitempty"`
	ManualPayee   *manualPayeePayload `json:"manualPayee,omitempty"`
	Consent       *legal.Consent      `json:"consent,omitempty"`
	CreatedAt     time.Time           `json:"createdAt"`
	UpdatedAt     time.Time           `json:"updatedAt"`
}

// checkoutToken prefers the X-Cart-Token header, falling back to the body
// token field. Absent from both, it is empty and the checkout fails closed.
func checkoutToken(r *http.Request, bodyToken string) string {
	if token := strings.TrimSpace(r.Header.Get(cartTokenHeader)); token != "" {
		return token
	}
	return strings.TrimSpace(bodyToken)
}

// parseOrderID extracts and validates the {orderID} path value.
func parseOrderID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("orderID"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid order ID", nil)
		return 0, false
	}
	return id, true
}

// projectOrder maps the domain order onto the response shape. List reads
// carry the store's ItemCount aggregate; detail reads derive it from the
// loaded items so the two never drift.
func projectOrder(order *ordersdomain.Order) orderPayload {
	itemCount := order.ItemCount
	if len(order.Items) > 0 {
		itemCount = len(order.Items)
	}
	out := orderPayload{
		ID:            order.ID,
		OrderNumber:   order.DisplayNumber(),
		Status:        order.Status,
		CustomerEmail: order.CustomerEmail,
		CustomerName:  order.CustomerName,
		CustomerPhone: order.CustomerPhone,
		ShipTo: addressPayload{
			Street:     order.ShipTo.Street,
			City:       order.ShipTo.City,
			Province:   order.ShipTo.Province,
			PostalCode: order.ShipTo.PostalCode,
		},
		Currency:      order.Currency,
		Subtotal:      order.Subtotal,
		ShippingMinor: order.ShippingMinor,
		TotalMinor:    order.TotalMinor(),
		Items:         make([]linePayload, 0, len(order.Items)),
		ItemCount:     itemCount,
		TrackingToken: order.TrackingToken,
		Consent:       order.Consent,
		CreatedAt:     order.CreatedAt,
		UpdatedAt:     order.UpdatedAt,
	}
	for i := range order.Items {
		out.Items = append(out.Items, linePayload{
			ID:               order.Items[i].ID,
			ProductContentID: order.Items[i].ProductContentID,
			VariantID:        order.Items[i].VariantID,
			SKU:              order.Items[i].SKU,
			Title:            order.Items[i].Title,
			UnitPrice:        order.Items[i].UnitPrice,
			Quantity:         order.Items[i].Quantity,
			LineTotal:        order.Items[i].LineTotal(),
		})
	}
	return out
}

// newInvoiceSettingsPayload projects the settings onto the wire shape.
func newInvoiceSettingsPayload(settings ordersdomain.InvoiceSettings) invoiceSettingsPayload {
	return invoiceSettingsPayload{
		SellerName:    settings.SellerName,
		SellerAddress: settings.SellerAddress,
		SellerTaxID:   settings.SellerTaxID,
		TaxRate:       settings.TaxRate,
	}
}

// mapServiceError maps domain sentinels to the wire codes: checkout state
// conflicts (consumed carts, sold-out or vanished lines) read as 409,
// input violations as 400, unknown orders as a non-committal 404.
func mapServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ordersdomain.ErrOrderNotFound):
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Order not found", nil)
	case errors.Is(err, ordersdomain.ErrCartGone):
		httpx.Error(w, http.StatusConflict, "CART_UNAVAILABLE", "Cart not found or already checked out", nil)
	case errors.Is(err, ordersdomain.ErrEmptyCart):
		httpx.Error(w, http.StatusConflict, "EMPTY_CART", "Cart is empty", nil)
	case errors.Is(err, ordersdomain.ErrLineUnavailable):
		httpx.Error(w, http.StatusConflict, "LINE_UNAVAILABLE", "A cart line is no longer available", nil)
	case errors.Is(err, ordersdomain.ErrOutOfStock):
		httpx.Error(w, http.StatusConflict, "OUT_OF_STOCK", "Item is out of stock", nil)
	case errors.Is(err, ordersdomain.ErrConsentRequired):
		httpx.Error(w, http.StatusBadRequest, "CONSENT_REQUIRED", "Consent to the terms and privacy policy is required", nil)
	case errors.Is(err, ordersdomain.ErrInvalidTransition):
		httpx.Error(w, http.StatusConflict, "INVALID_TRANSITION", err.Error(), nil)
	case errors.Is(err, ordersdomain.ErrInvalidEmail),
		errors.Is(err, ordersdomain.ErrInvalidName),
		errors.Is(err, ordersdomain.ErrInvalidAddress),
		errors.Is(err, ordersdomain.ErrInvalidShipping),
		errors.Is(err, ordersdomain.ErrInvalidStatus):
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
	default:
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
	}
}

// manualPayeeComplete reports whether the adopted payee block is renderable:
// the email requires all three fields, so tracking mirrors it instead of
// emitting a panel with empty bank/account/name rows.
func manualPayeeComplete(payee *ManualPayee) bool {
	return payee != nil && payee.Bank != "" && payee.Account != "" && payee.Name != ""
}

// ManualPayee is the destination account for manual bank transfers,
// adopted from the [payments.manual] config block via the composition
// root. Empty means no payee block is configured and tracking payloads
// carry no payee section.
type ManualPayee struct {
	Bank    string
	Account string
	Name    string
}

// InvoiceSettingsStore reads and writes the live invoice settings. The
// module owns the state (mutex-guarded) and persists it through the config
// write-back seam; GET and PUT /orders/invoice-settings serve it. A nil
// store leaves the settings surface unavailable.
type InvoiceSettingsStore interface {
	InvoiceSettings() ordersdomain.InvoiceSettings
	SetInvoiceSettings(settings ordersdomain.InvoiceSettings) error
}

// InvoiceSupport carries the invoice endpoint's collaborators: the seller
// identity and PPN rate plus the PDF renderer. A nil support leaves the
// endpoint unconfigured. Store, when present, supersedes Settings so the
// invoice reflects an admin edit without a restart.
type InvoiceSupport struct {
	Settings ordersdomain.InvoiceSettings
	Renderer ordersdomain.InvoiceRenderer
	Store    InvoiceSettingsStore
}

// Handler serves the order surfaces over the adopted service; New builds it
// once with the admin capability resolver.
type Handler struct {
	service     *ordersdomain.Service
	resolve     kernel.CapabilityResolver
	invoice     *InvoiceSupport
	manualPayee *ManualPayee
}

// SetManualPayee adopts the manual payee block for guest tracking payloads.
// A nil payee leaves tracking payloads without the payee section.
func (h *Handler) SetManualPayee(payee *ManualPayee) {
	h.manualPayee = payee
}

// authorize is the admin gate every admin method runs. A missing resolver
// fails closed: wiring mistakes leak order records, not access.
func (h *Handler) authorize(w http.ResponseWriter, r *http.Request) bool {
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok || h.resolve == nil || !h.resolve(principal, string(capability.CapabilityAdmin)) {
		httpx.Error(w, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "You do not have permission to manage orders", nil)
		return false
	}
	return true
}

// invoiceSettings resolves the effective invoice settings: the live store
// when adopted, else the boot-time values.
func (h *Handler) invoiceSettings() ordersdomain.InvoiceSettings {
	if h.invoice == nil {
		return ordersdomain.InvoiceSettings{}
	}
	if h.invoice.Store != nil {
		return h.invoice.Store.InvoiceSettings()
	}
	return h.invoice.Settings
}

// Checkout handles POST /checkout: claims the presented cart into a
// pending order (the atomic delete arbitrates parallel checkouts) and
// snapshots its prices, SKUs, and titles.
func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body checkoutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}
	order, err := h.service.Checkout(r.Context(), ordersdomain.CheckoutInput{
		Token:           checkoutToken(r, body.Token),
		Email:           body.Email,
		Name:            body.Name,
		Phone:           body.Phone,
		ShipTo:          body.ShipTo,
		ConsentAccepted: body.ConsentAccepted,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	httpx.Success(w, projectOrder(order))
}

// TrackOrder handles GET /orders/{orderNumber}?email=&token=: guest
// tracking behind the three-factor gate. All of number, email, and the
// per-order tracking token must match; everything else reads as not found.
// A pending order with a configured payee carries the manualPayee block
// for the tracking panel; absent payee leaves the current panel unchanged.
func (h *Handler) TrackOrder(w http.ResponseWriter, r *http.Request) {
	order, err := h.service.Tracking(r.Context(), r.PathValue("orderNumber"), r.URL.Query().Get("email"), r.URL.Query().Get("token"))
	if err != nil {
		mapServiceError(w, err)
		return
	}
	payload := projectOrder(order)
	if manualPayeeComplete(h.manualPayee) && order.Status == ordersdomain.StatusPending {
		payload.ManualPayee = &manualPayeePayload{
			Bank:    h.manualPayee.Bank,
			Account: h.manualPayee.Account,
			Name:    h.manualPayee.Name,
		}
	}
	httpx.Success(w, payload)
}

// ListOrders handles GET /orders: the newest orders first, the status
// filter optional, keyed cursor pagination.
func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}

	limit := httpx.ParseListLimit(r)
	beforeID, err := httpx.DecodeCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid cursor", nil)
		return
	}

	var status *ordersdomain.Status
	if raw := r.URL.Query().Get("status"); raw != "" {
		parsed, ok := ordersdomain.ParseStatus(raw)
		if !ok {
			httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid status filter", nil)
			return
		}
		status = &parsed
	}

	orders, err := h.service.List(r.Context(), status, int64(beforeID), limit)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	hasMore := len(orders) > limit
	if hasMore {
		orders = orders[:limit]
	}
	nextCursor := ""
	if hasMore && len(orders) > 0 {
		nextCursor = httpx.EncodeCursor(int(orders[len(orders)-1].ID))
	}

	payloads := make([]orderPayload, 0, len(orders))
	for i := range orders {
		payloads = append(payloads, projectOrder(&orders[i]))
	}
	httpx.SuccessList(w, payloads, httpx.ListMeta{Pagination: httpx.Pagination{NextCursor: nextCursor, HasMore: hasMore}})
}

// GetOrder handles GET /orders/{orderID}: one order with its items.
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	id, ok := parseOrderID(w, r)
	if !ok {
		return
	}
	order, err := h.service.Get(r.Context(), id)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	httpx.Success(w, projectOrder(order))
}

// TransitionOrder handles POST /orders/{orderID}/status: one lifecycle move,
// validated against the transition map, emailing the customer.
func (h *Handler) TransitionOrder(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	id, ok := parseOrderID(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body transitionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}
	order, err := h.service.Transition(r.Context(), id, ordersdomain.Status(body.Status))
	if err != nil {
		mapServiceError(w, err)
		return
	}
	httpx.Success(w, projectOrder(order))
}

// SetShippingOrder replaces the shipping line of a pending order.
// Settled, expired, or cancelled orders refuse with INVALID_TRANSITION;
// negative amounts are a validation error.
func (h *Handler) SetShippingOrder(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	id, ok := parseOrderID(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body shippingBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}
	order, err := h.service.SetShipping(r.Context(), id, body.Amount)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	httpx.Success(w, projectOrder(order))
}

// InvoiceOrder streams the order's invoice PDF: derived on demand from
// the stored record, never persisted. Unknown orders read as not found;
// a module without a renderer answers unavailable instead of serving an
// empty surface.
func (h *Handler) InvoiceOrder(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	id, ok := parseOrderID(w, r)
	if !ok {
		return
	}
	if h.invoice == nil || h.invoice.Renderer == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "INVOICE_UNAVAILABLE", "Invoice rendering is not configured", nil)
		return
	}
	doc, err := h.service.BuildInvoice(r.Context(), id, h.invoiceSettings())
	if err != nil {
		mapServiceError(w, err)
		return
	}
	rendered, err := h.invoice.Renderer.Render(r.Context(), doc)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+doc.Number+`.pdf"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(rendered)
}

// GetInvoiceSettings handles GET /orders/invoice-settings: the live seller
// identity and PPN rate the next rendered invoice will use.
func (h *Handler) GetInvoiceSettings(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	if h.invoice == nil || h.invoice.Store == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "INVOICE_SETTINGS_UNAVAILABLE", "Invoice settings are not configured", nil)
		return
	}
	httpx.Success(w, newInvoiceSettingsPayload(h.invoice.Store.InvoiceSettings()))
}

// UpdateInvoiceSettings handles PUT /orders/invoice-settings: validates the
// block, persists it through the module's config write-back, and adopts it
// for the next rendered invoice without a restart. An explicit zero rate is
// preserved — non-PKP sellers charge no PPN.
func (h *Handler) UpdateInvoiceSettings(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	if h.invoice == nil || h.invoice.Store == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "INVOICE_SETTINGS_UNAVAILABLE", "Invoice settings are not configured", nil)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body invoiceSettingsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}
	if body.TaxRate == nil {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "taxRate is required", nil)
		return
	}
	rate := *body.TaxRate
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || rate > 100 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Tax rate must be a percentage between 0 and 100", nil)
		return
	}
	settings := ordersdomain.InvoiceSettings{
		SellerName:    strings.TrimSpace(body.SellerName),
		SellerAddress: strings.TrimSpace(body.SellerAddress),
		SellerTaxID:   strings.TrimSpace(body.SellerTaxID),
		TaxRate:       rate,
	}
	if err := h.invoice.Store.SetInvoiceSettings(settings); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to save invoice settings", nil)
		return
	}
	httpx.Success(w, newInvoiceSettingsPayload(settings))
}

// New builds the handler over the adopted service and the capability
// resolver gating the admin surface.
func New(service *ordersdomain.Service, resolve kernel.CapabilityResolver, invoice *InvoiceSupport) *Handler {
	return &Handler{service: service, resolve: resolve, invoice: invoice}
}
