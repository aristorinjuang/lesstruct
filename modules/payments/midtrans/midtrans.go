// Package midtrans is the Midtrans Snap driver for the payments module:
// it mints hosted-checkout sessions over the Snap REST API and
// authenticates webhook notifications with the SHA-512 server-key
// checksum. The driver speaks the API through a plain net/http client a
// caller may replace in tests.
package midtrans

import (
	"bytes"
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	paymentsdomain "github.com/aristorinjuang/lesstruct/modules/payments/domain"
)

const (
	// snapSandbox and snapProduction are the Snap checkout bases; the
	// mint call posts /snap/v1/transactions.
	snapSandbox    = "https://app.sandbox.midtrans.com"
	snapProduction = "https://app.midtrans.com"

	// requestTimeout bounds one upstream call so a slow gateway never
	// pins the request.
	requestTimeout = 15 * time.Second

	// replyCapBytes caps one upstream reply body.
	replyCapBytes = 1 << 20
)

// mintExpiry bounds the session lifetime; the payload carries whole
// hours, so a sub-hour TTL rounds up.
type mintExpiry struct {
	Duration int64  `json:"duration"`
	Unit     string `json:"unit"`
}

// mintRequest is the transaction mint payload. The merchant order id
// carries the display order number, so every webhook and API lookup keys
// off the same derived value the platform already serves.
type mintRequest struct {
	TransactionDetails struct {
		OrderID     string `json:"order_id"`
		GrossAmount int64  `json:"gross_amount"`
	} `json:"transaction_details"`
	CustomerDetails struct {
		Email string `json:"email"`
	} `json:"customer_details"`
	Expiry *mintExpiry `json:"expiry,omitempty"`
	// Callbacks steers the buyer back after Snap payment: finish lands on
	// the order tracking view (order, email, and tracking token ride the
	// query so the page can auto-load), replacing Midtrans' dashboard
	// default. Omitted when the driver has no finish base.
	Callbacks *mintCallbacks `json:"callbacks,omitempty"`
}

// mintCallbacks carries the post-payment redirect.
type mintCallbacks struct {
	Finish string `json:"finish"`
}

// mintResponse is the Snap session reply.
type mintResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
}

// Option customizes the driver.
type Option func(*Driver)

// WithHTTPClient replaces the default upstream client.
func WithHTTPClient(client *http.Client) Option {
	return func(d *Driver) {
		d.client = client
	}
}

// WithSessionTTL sets the Snap session expiry, rounded up to whole
// hours (the finest unit the payload carries).
func WithSessionTTL(ttl time.Duration) Option {
	return func(d *Driver) {
		d.sessionTTL = ttl
	}
}

// WithFinishBase steers the buyer back after Snap payment: the driver
// sends callbacks.finish pointing at the order tracking view under this
// public origin. Empty keeps the current behavior (no callbacks sent, so
// Midtrans falls back to its dashboard default).
func WithFinishBase(base string) Option {
	return func(d *Driver) {
		d.finishBase = base
	}
}

// Driver is the paymentsdomain.Gateway implementation for Midtrans Snap.
type Driver struct {
	serverKey  string
	snapBase   string
	sessionTTL time.Duration
	finishBase string
	client     *http.Client
}

// Mint opens a hosted checkout session for the order.
func (d *Driver) Mint(ctx context.Context, order *paymentsdomain.OrderRef) (*paymentsdomain.GatewaySession, error) {
	if order.TotalMinor <= 0 {
		return nil, fmt.Errorf("midtrans mint: the order total %d is not payable", order.TotalMinor)
	}

	var payload mintRequest
	payload.TransactionDetails.OrderID = order.OrderNumber
	payload.TransactionDetails.GrossAmount = order.TotalMinor
	payload.CustomerDetails.Email = order.CustomerEmail
	if d.sessionTTL > 0 {
		hours := (d.sessionTTL + time.Hour - 1) / time.Hour
		payload.Expiry = &mintExpiry{Duration: int64(max(hours, 1)), Unit: "hour"}
	}
	if base := strings.TrimRight(d.finishBase, "/"); base != "" {
		finish := url.Values{}
		finish.Set("order", order.OrderNumber)
		finish.Set("email", order.CustomerEmail)
		if order.TrackingToken != "" {
			finish.Set("t", order.TrackingToken)
		}
		payload.Callbacks = &mintCallbacks{Finish: base + "/orders/track?" + finish.Encode()}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the midtrans mint request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.snapBase+"/snap/v1/transactions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to build the midtrans mint request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(d.serverKey, "")

	response, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("midtrans mint request failed: %w: %w", err, paymentsdomain.ErrGatewayUpstream)
	}
	defer func() { _ = response.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(response.Body, replyCapBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to read the midtrans mint response: %w: %w", err, paymentsdomain.ErrGatewayUpstream)
	}
	if response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("midtrans mint failed (HTTP %d): %s: %w", response.StatusCode, strings.TrimSpace(string(raw)), paymentsdomain.ErrGatewayUpstream)
	}

	var reply mintResponse
	if err := json.Unmarshal(raw, &reply); err != nil {
		return nil, fmt.Errorf("failed to decode the midtrans mint response: %w: %w", err, paymentsdomain.ErrGatewayUpstream)
	}
	if reply.Token == "" {
		return nil, fmt.Errorf("midtrans mint returned no token: %w", paymentsdomain.ErrGatewayUpstream)
	}
	return &paymentsdomain.GatewaySession{
		Token:       reply.Token,
		RedirectURL: reply.RedirectURL,
	}, nil
}

// Authenticate verifies the webhook checksum: SHA-512 over the order id,
// notification status code, gross-amount string, and server key,
// hex-encoded. The comparison is constant-time so a forged event cannot
// probe the expected signature byte by byte.
func (d *Driver) Authenticate(orderNumber string, statusCode string, grossAmount string, signature string) bool {
	mac := sha512.Sum512([]byte(orderNumber + statusCode + grossAmount + d.serverKey))
	return subtle.ConstantTimeCompare([]byte(signature), []byte(hex.EncodeToString(mac[:]))) == 1
}

// MinorUnits renders a notification gross amount ("522500.00") into the
// minor units the platform stores. The IDR ledger holds whole rupiah —
// rupiah is its own minor unit — so the integer part is the amount and
// any other fractional sweep refuses rather than round money.
func (d *Driver) MinorUnits(grossAmount string) (int64, error) {
	clean := strings.TrimSpace(grossAmount)
	intPart, frac, hasFrac := strings.Cut(clean, ".")
	units, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil || units < 0 {
		return 0, fmt.Errorf("unparseable gross amount %q", grossAmount)
	}
	if hasFrac && frac != "" && frac != "00" {
		return 0, fmt.Errorf("gross amount %q carries fractional rupiah the ledger cannot hold", grossAmount)
	}
	return units, nil
}

// WellFormedServerKey reports whether the key has the Midtrans server-key
// shape for the environment: SB-Mid-server-… on sandbox, Mid-server-…
// on production. A Merchant-ID-shaped value (or quotes pasted from a
// dashboard) fails closed here so the operator hears about it at startup
// instead of at the first "Pay now" tap. Surrounding whitespace is
// tolerated; anything else must match verbatim.
func WellFormedServerKey(key string, production bool) bool {
	trimmed := strings.TrimSpace(key)
	if production {
		return strings.HasPrefix(trimmed, "Mid-server-")
	}
	return strings.HasPrefix(trimmed, "SB-Mid-server-")
}

// New builds the driver over a server key. Production=false serves the
// sandbox; WithHTTPClient swaps the default bounded client and
// WithSessionTTL mirrors the orders pending-expiry window into the Snap
// session so a hosted checkout cannot outlive its order.
func New(serverKey string, production bool, options ...Option) *Driver {
	base := snapSandbox
	if production {
		base = snapProduction
	}
	driver := &Driver{
		serverKey: serverKey,
		snapBase:  base,
		client:    &http.Client{Timeout: requestTimeout},
	}
	for _, option := range options {
		option(driver)
	}
	return driver
}
