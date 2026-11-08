package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// CatalogGetProduct sends GET /api/v1/catalog/products/{contentID} and
// returns the decoded admin commerce data and meta payloads, or an
// *APIError on failure. Unknown content IDs read 404 NOT_FOUND; non-admin
// keys read 403.
func (c *Client) CatalogGetProduct(
	ctx context.Context,
	contentID int,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/catalog/products/%d", contentID), nil, nil)
}

// CatalogUpsertProduct sends PUT /api/v1/catalog/products/{contentID} with
// the given commerce body (the admin wire shape: price, salePrice, sku,
// trackInventory, stock, weightGrams, lowStockThreshold, options,
// variants) and returns the decoded data and meta payloads, or an
// *APIError on failure. The call replaces the variant matrix; callers
// that only change scalars should read first and resend the stored axes.
func (c *Client) CatalogUpsertProduct(
	ctx context.Context,
	contentID int,
	body map[string]any,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPut, fmt.Sprintf("/api/v1/catalog/products/%d", contentID), nil, body)
}

// OrdersList sends GET /api/v1/orders with optional status and cursor
// filters and returns the decoded data (newest-first order summaries) and
// meta (with pagination), or an *APIError on failure. Empty filters are
// omitted so the default call lists everything.
func (c *Client) OrdersList(
	ctx context.Context,
	status string,
	cursor string,
) (json.RawMessage, json.RawMessage, error) {
	query := url.Values{}
	if status != "" {
		query.Set("status", status)
	}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	return c.do(ctx, http.MethodGet, "/api/v1/orders", query, nil)
}

// OrdersGet sends GET /api/v1/orders/{orderID} and returns the decoded
// order detail (full item snapshot) and meta payloads, or an *APIError on
// failure. Unknown IDs read 404 NOT_FOUND; non-admin keys read 403.
func (c *Client) OrdersGet(
	ctx context.Context,
	id int64,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/orders/%d", id), nil, nil)
}

// OrdersTransition sends POST /api/v1/orders/{orderID}/status with the
// target lifecycle status and returns the decoded order and meta payloads,
// or an *APIError on failure. Illegal moves read 409; "paid" is refused
// outright — payment confirmation is the only path into paid.
func (c *Client) OrdersTransition(
	ctx context.Context,
	id int64,
	status string,
) (json.RawMessage, json.RawMessage, error) {
	body := map[string]any{"status": status}
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/orders/%d/status", id), nil, body)
}

// OrdersSetShipping sends POST /api/v1/orders/{orderID}/shipping with the
// shipping line in integer minor units and returns the decoded order and
// meta payloads, or an *APIError on failure. Only pending orders carry
// shipping; settled orders refuse with 409.
func (c *Client) OrdersSetShipping(
	ctx context.Context,
	id int64,
	amount int64,
) (json.RawMessage, json.RawMessage, error) {
	body := map[string]any{"amount": amount}
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/orders/%d/shipping", id), nil, body)
}

// InvoiceSettingsGet sends GET /api/v1/orders/invoice-settings and returns
// the decoded live block (seller identity plus PPN rate) and meta
// payloads, or an *APIError on failure.
func (c *Client) InvoiceSettingsGet(
	ctx context.Context,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/orders/invoice-settings", nil, nil)
}

// InvoiceSettingsSet sends PUT /api/v1/orders/invoice-settings with the
// given block and returns the decoded saved block and meta payloads, or
// an *APIError on failure. Missing, non-finite, or out-of-range taxRate
// reads 400; the save applies to the next rendered invoice without a
// restart.
func (c *Client) InvoiceSettingsSet(
	ctx context.Context,
	body map[string]any,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPut, "/api/v1/orders/invoice-settings", nil, body)
}

// PaymentsList sends GET /api/v1/payments with optional status and cursor
// filters and returns the decoded data (newest-first payment records) and
// meta (with pagination), or an *APIError on failure. Empty filters are
// omitted so the default call lists everything.
func (c *Client) PaymentsList(
	ctx context.Context,
	status string,
	cursor string,
) (json.RawMessage, json.RawMessage, error) {
	query := url.Values{}
	if status != "" {
		query.Set("status", status)
	}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	return c.do(ctx, http.MethodGet, "/api/v1/payments", query, nil)
}

// PaymentsGet sends GET /api/v1/payments/{paymentID} and returns the
// decoded payment record with its order context and meta payloads, or an
// *APIError on failure.
func (c *Client) PaymentsGet(
	ctx context.Context,
	id int64,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/payments/%d", id), nil, nil)
}

// PaymentsVerify sends POST /api/v1/payments/{paymentID}/verify with an
// empty body and returns the decoded verified record and meta payloads,
// or an *APIError on failure. Decided records read 409, as do proofs
// whose order already left pending.
func (c *Client) PaymentsVerify(
	ctx context.Context,
	id int64,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/payments/%d/verify", id), nil, nil)
}

// PaymentsReject sends POST /api/v1/payments/{paymentID}/reject with an
// empty body and returns the decoded rejected record and meta payloads, or
// an *APIError on failure. The order stays pending; proofs whose order
// already left pending read 409 with no rejection email.
func (c *Client) PaymentsReject(
	ctx context.Context,
	id int64,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/payments/%d/reject", id), nil, nil)
}

// DeleteMedia sends DELETE /api/v1/media/{id} and returns nil data and
// meta on success (204 No Content carries no envelope), or an *APIError on
// failure. Non-owners read 403; ghost ids read 404.
func (c *Client) DeleteMedia(
	ctx context.Context,
	id int,
) (json.RawMessage, json.RawMessage, error) {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/media/%d", id), nil, nil)
}
