// Package api hosts the cart module's public HTTP surface: token minting,
// cart reads, line management, and cart deletion. Everything writes through
// kernel/httpx — this is a new surface with no legacy envelope contract.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/aristorinjuang/lesstruct/kernel/httpx"
	cartdomain "github.com/aristorinjuang/lesstruct/modules/cart/domain"
)

const (
	// cartTokenHeader carries the cart token on requests and responses,
	// mirroring the Authorization header pattern the realms already use.
	cartTokenHeader = "X-Cart-Token"
	// maxBodyBytes caps the JSON bodies the line writes decode. The public
	// router rate-limits requests, not bytes, so these bodies — served to
	// anonymous traffic — must bound their own read.
	maxBodyBytes = 1 << 20
)

// addItemBody is the POST /cart/items body. Token rides the header when the
// client can set one, else the body field.
type addItemBody struct {
	Token            string `json:"token,omitempty"`
	ProductContentID int    `json:"productContentId"`
	VariantID        *int64 `json:"variantId,omitempty"`
	Quantity         int    `json:"quantity"`
}

// updateItemBody is the PATCH /cart/items/{itemId} body.
type updateItemBody struct {
	Token    string `json:"token,omitempty"`
	Quantity int    `json:"quantity"`
}

// cartResponse is the read shape of a cart: resolved lines with live
// prices plus the currency for display.
type cartResponse struct {
	ID        int64      `json:"id"`
	Currency  string     `json:"currency"`
	Subtotal  int64      `json:"subtotal"`
	ItemCount int        `json:"itemCount"`
	Lines     []lineView `json:"lines"`
}

// lineView is the read shape of one cart line.
type lineView struct {
	ID               int64  `json:"id"`
	ProductContentID int    `json:"productContentId"`
	VariantID        *int64 `json:"variantId,omitempty"`
	SKU              string `json:"sku,omitempty"`
	Title            string `json:"title,omitempty"`
	Slug             string `json:"slug,omitempty"`
	ImageURL         string `json:"imageUrl,omitempty"`
	Quantity         int    `json:"quantity"`
	UnitPrice        int64  `json:"unitPrice"`
	LineTotal        int64  `json:"lineTotal"`
	InStock          bool   `json:"inStock"`
	Available        bool   `json:"available"`
}

// cartToken reads the presented cart token from the X-Cart-Token header,
// falling back to a JSON body token field (for headerless clients on
// bodyless requests: GET and DELETE).
func cartToken(r *http.Request) string {
	return resolveToken(r, "")
}

// resolveToken prefers the header token, then the body token carried by
// mutation payloads. Absent on both, it is empty and the service fails it
// closed.
func resolveToken(r *http.Request, bodyToken string) string {
	if token := strings.TrimSpace(r.Header.Get(cartTokenHeader)); token != "" {
		return token
	}
	if bodyToken != "" {
		return strings.TrimSpace(bodyToken)
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return ""
	}
	return strings.TrimSpace(body.Token)
}

// parseItemID extracts and validates the {itemId} path value.
func parseItemID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("itemId"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid cart item ID", nil)
		return 0, false
	}
	return id, true
}

// projectCart maps the domain cart onto the response shape.
func projectCart(cart *cartdomain.Cart, currency string) cartResponse {
	out := cartResponse{
		ID:        cart.ID,
		Currency:  currency,
		Subtotal:  cart.Subtotal(),
		ItemCount: cart.ItemCount(),
		Lines:     make([]lineView, 0, len(cart.Lines)),
	}
	for _, line := range cart.Lines {
		out.Lines = append(out.Lines, lineView{
			ID:               line.ID,
			ProductContentID: line.ProductContentID,
			VariantID:        line.VariantID,
			SKU:              line.SKU,
			Title:            line.Title,
			Slug:             line.Slug,
			ImageURL:         line.ImageURL,
			Quantity:         line.Quantity,
			UnitPrice:        line.UnitPrice,
			LineTotal:        line.LineTotal(),
			InStock:          line.InStock,
			Available:        line.Available,
		})
	}
	return out
}

// mapServiceError maps domain sentinels to the public envelope codes:
// unknown or unusable tokens read as unauthorized (never a persistence
// leak), missing products and variants as not found, stock and quantity
// violations as their own codes.
func mapServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, cartdomain.ErrCartNotFound):
		httpx.Error(w, http.StatusUnauthorized, "INVALID_CART_TOKEN", "Cart token is missing, invalid, or expired", nil)
	case errors.Is(err, cartdomain.ErrLineNotFound):
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Cart line not found", nil)
	case errors.Is(err, cartdomain.ErrUnknownProduct),
		errors.Is(err, cartdomain.ErrUnknownVariant):
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Product not found", nil)
	case errors.Is(err, cartdomain.ErrOutOfStock):
		httpx.Error(w, http.StatusConflict, "OUT_OF_STOCK", "Item is out of stock", nil)
	case errors.Is(err, cartdomain.ErrInvalidQuantity):
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
	default:
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
	}
}

// Handler serves the public cart surface; New builds it once over the
// adopted service and the display currency for totals payloads.
type Handler struct {
	service  *cartdomain.Service
	currency string
}

// CreateCart handles POST /cart: mints a token and an empty cart. The token
// rides the response body and the X-Cart-Token header; clients present it on
// every subsequent cart call.
func (h *Handler) CreateCart(w http.ResponseWriter, r *http.Request) {
	token, cart, err := h.service.CreateCart(r.Context())
	if err != nil {
		mapServiceError(w, err)
		return
	}
	w.Header().Set(cartTokenHeader, token)
	httpx.Success(w, map[string]any{
		"token": token,
		"cart":  projectCart(cart, h.currency),
	})
}

// GetCart handles GET /cart: the live-priced cart for the presented token.
func (h *Handler) GetCart(w http.ResponseWriter, r *http.Request) {
	cart, err := h.service.GetCart(r.Context(), cartToken(r))
	if err != nil {
		mapServiceError(w, err)
		return
	}
	httpx.Success(w, projectCart(cart, h.currency))
}

// AddItem handles POST /cart/items: resolves the sellable line and merges
// it into the cart.
func (h *Handler) AddItem(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body addItemBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}
	cart, err := h.service.AddItem(r.Context(), resolveToken(r, body.Token), body.ProductContentID, body.VariantID, body.Quantity)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	httpx.Success(w, projectCart(cart, h.currency))
}

// UpdateItem handles PATCH /cart/items/{itemId}: replaces a line quantity.
func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body updateItemBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}
	itemID, ok := parseItemID(w, r)
	if !ok {
		return
	}
	cart, err := h.service.UpdateItem(r.Context(), resolveToken(r, body.Token), itemID, body.Quantity)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	httpx.Success(w, projectCart(cart, h.currency))
}

// RemoveItem handles DELETE /cart/items/{itemId}: drops one line.
func (h *Handler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	itemID, ok := parseItemID(w, r)
	if !ok {
		return
	}
	cart, err := h.service.RemoveItem(r.Context(), cartToken(r), itemID)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	httpx.Success(w, projectCart(cart, h.currency))
}

// DeleteCart handles DELETE /cart: empties and removes the cart. The token
// cannot be reused afterwards.
func (h *Handler) DeleteCart(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteCart(r.Context(), cartToken(r)); err != nil {
		mapServiceError(w, err)
		return
	}
	httpx.Success(w, map[string]any{"deleted": true})
}

// New builds the handler over the adopted service and display currency.
func New(service *cartdomain.Service, currency string) *Handler {
	return &Handler{service: service, currency: currency}
}
