package orders

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"net/mail"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel/legal"
)

const (
	// ListLimitDefault applies when a caller passes no explicit list page
	// size.
	ListLimitDefault = 50
	// ListLimitCeiling bounds the admin list page size.
	ListLimitCeiling = 100
	// FallbackProductTitle fills the item snapshot when the content seam
	// reports no published title for the product.
	FallbackProductTitle = "Product"
)

// newTrackingToken mints the per-order tracking secret: TrackingTokenBytes
// of crypto-random entropy, base64url without padding for URL-safe
// transport in links and query strings.
func newTrackingToken() (string, error) {
	entropy := make([]byte, TrackingTokenBytes)
	if _, err := rand.Read(entropy); err != nil {
		return "", fmt.Errorf("failed to mint tracking token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(entropy), nil
}

// hashTrackingToken renders the at-rest form of a tracking token: the hex
// SHA-256 of the raw value. Only the hash is stored, so a database read
// can never leak the usable secret.
func hashTrackingToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// customerEmail is the canonical form of a checkout email: trimmed,
// lowercased, and syntax-validated. Tracking emails reuse the canonical form
// without syntax judgment, so a lookup miss never separates a malformed
// address from a malformed order number.
type customerEmail string

// newCustomerEmail canonicalizes and validates a checkout email. Malformed
// addresses fail checkout rather than double as tracking handles.
func newCustomerEmail(raw string) (customerEmail, error) {
	addr, err := mail.ParseAddress(strings.ToLower(strings.TrimSpace(raw)))
	if err != nil || addr.Address == "" {
		return "", ErrInvalidEmail
	}
	return customerEmail(addr.Address), nil
}

// String renders the canonical address.
func (e customerEmail) String() string { return string(e) }

// normalizeCustomerEmail canonicalizes a presented tracking email the way
// checkout stored it: trimmed and lowercased, with no syntax judgment to
// avoid leaking whether a lookup miss came from the address or the number.
func normalizeCustomerEmail(raw string) customerEmail {
	return customerEmail(strings.ToLower(strings.TrimSpace(raw)))
}

// customerName is a checkout customer name: trimmed and required non-empty.
type customerName string

// newCustomerName trims the customer name and requires it non-empty.
func newCustomerName(raw string) (customerName, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", ErrInvalidName
	}
	return customerName(name), nil
}

// String renders the name.
func (n customerName) String() string { return string(n) }

// newShippingAddress trims the shipping fields and requires them complete:
// street, city, province, and postal code all matter for packing labels.
func newShippingAddress(address Address) (Address, error) {
	address.Street = strings.TrimSpace(address.Street)
	address.City = strings.TrimSpace(address.City)
	address.Province = strings.TrimSpace(address.Province)
	address.PostalCode = strings.TrimSpace(address.PostalCode)
	if address.Street == "" || address.City == "" || address.Province == "" || address.PostalCode == "" {
		return Address{}, ErrInvalidAddress
	}
	return address, nil
}

// normalizeList bounds the list page size: zero falls to the default,
// anything else clamps to the ceiling.
func normalizeList(limit int) int {
	if limit <= 0 {
		return ListLimitDefault
	}
	return min(limit, ListLimitCeiling)
}

// ConsentPolicy is the checkout consent contract resolved once at boot:
// whether an affirmative act is required and which document revisions that
// act refers to. A zero policy leaves checkout unchanged.
type ConsentPolicy struct {
	Required bool
	Terms    legal.Document
	Privacy  legal.Document
}

// checkoutConsent stamps the acceptance evidence for a consent-gated
// checkout. It returns nil when the policy does not require consent, so the
// order stores nothing and the wire shape stays unchanged.
func (p ConsentPolicy) checkoutConsent(now time.Time) *legal.Consent {
	if !p.Required {
		return nil
	}
	consent := legal.NewConsent(p.Terms, p.Privacy, now)
	return &consent
}

// ServiceOption configures a Service. Options carry the boot-time contracts
// the composition root resolves once (consent requirements), keeping the
// service free of per-request legal lookups.
type ServiceOption func(*Service)

// WithConsentPolicy adopts the checkout consent contract: when required,
// checkout rejects orders without an affirmative act and records the
// resolved document revisions on every stored order.
func WithConsentPolicy(policy ConsentPolicy) ServiceOption {
	return func(s *Service) {
		s.consent = policy
	}
}

// CheckoutInput is the checkout request: the cart token to consume plus
// the customer contact and shipping address. The order currency comes from
// the platform configuration, not the client.
type CheckoutInput struct {
	Token           string
	Email           string
	Name            string
	Phone           string
	ShipTo          Address
	ConsentAccepted bool
}

// Service is the order domain service: checkout snapshotting over the cart
// and content seams, guest tracking, and the admin lifecycle operations
// over the persistence, mail, and inventory seams.
type Service struct {
	store     OrderStore
	cart      CartSource
	content   ContentSource
	mail      Mailer
	inventory Inventory
	currency  string
	consent   ConsentPolicy
}

// Checkout validates the request, claims the presented cart, and stores it
// as a pending order, then emails the confirmation (best-effort). Every
// line must resolve and be in stock: stock is validated here and only
// decremented later, at payment. The order numbers snapshot live cart
// prices at the moment of purchase, so later catalog edits never rewrite
// history.
func (s *Service) Checkout(ctx context.Context, input CheckoutInput) (*Order, error) {
	email, err := newCustomerEmail(input.Email)
	if err != nil {
		return nil, err
	}
	name, err := newCustomerName(input.Name)
	if err != nil {
		return nil, err
	}
	address, err := newShippingAddress(input.ShipTo)
	if err != nil {
		return nil, err
	}
	// Consent is enforced before the cart is read or claimed: a refused
	// checkout must leave the cart intact for the buyer to retry.
	if s.consent.Required && !input.ConsentAccepted {
		return nil, ErrConsentRequired
	}
	snapshot, err := s.cart.Snapshot(ctx, input.Token)
	if err != nil {
		return nil, err
	}
	if len(snapshot.Lines) == 0 {
		return nil, ErrEmptyCart
	}
	for i := range snapshot.Lines {
		switch {
		case !snapshot.Lines[i].Available:
			return nil, ErrLineUnavailable
		case !snapshot.Lines[i].InStock:
			return nil, ErrOutOfStock
		}
	}
	// Consume is the claim: a delete is atomic, so parallel checkouts on
	// one token race it and only the winner proceeds to order storage —
	// double submits and lost races fail closed with ErrCartGone here.
	// Past this point the cart is gone: a failed order storage abandons
	// the claim, and the customer rebuilds it by re-adding lines. That is
	// the accepted cost — the alternative (consuming after storage) lets
	// a race mint two orders, and duplicate money records are worse than
	// a re-buildable cart.
	if err := s.cart.Consume(ctx, input.Token); err != nil {
		return nil, err
	}
	// The tracking secret mints once, at claim time: the raw token rides
	// the checkout response and the confirmation email, the hash is the
	// only thing stored. Losing it means losing guest access to the order
	// — by design; there is no email-only fallback to guess.
	token, err := newTrackingToken()
	if err != nil {
		return nil, fmt.Errorf("failed to mint tracking token: %w", err)
	}
	order := &Order{
		Status:            StatusPending,
		CustomerEmail:     email.String(),
		CustomerName:      name.String(),
		CustomerPhone:     strings.TrimSpace(input.Phone),
		ShipTo:            address,
		Currency:          s.currency,
		Consent:           s.consent.checkoutConsent(time.Now()),
		TrackingToken:     token,
		TrackingTokenHash: hashTrackingToken(token),
	}
	for i := range snapshot.Lines {
		line := snapshot.Lines[i]
		item := OrderItem{
			ProductContentID: line.ProductContentID,
			VariantID:        line.VariantID,
			SKU:              line.SKU,
			Title:            s.lineTitle(ctx, line.ProductContentID),
			UnitPrice:        line.UnitPrice,
			Quantity:         line.Quantity,
		}
		order.Subtotal += item.LineTotal()
		order.Items = append(order.Items, item)
	}
	id, err := s.store.CreateOrder(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}
	order.ID = id
	// Email delivery is best-effort: the order record is authoritative, so
	// a failed send never fails the checkout. The mailer adapter logs its
	// own failures.
	_ = s.mail.SendOrderConfirmationEmail(ctx, order)
	return order, nil
}

// Tracking resolves one order for its customer. The order number, the
// email, and the per-order tracking token must all match; every other
// combination — unknown number, wrong address, missing or wrong token,
// malformed input — reads as not found so lookups can never enumerate
// orders. The token is what makes the gate strong: numbers are sequential
// and emails are guessable, the 128-bit secret is neither.
func (s *Service) Tracking(ctx context.Context, number string, email string, token string) (*Order, error) {
	id, ok := ParseOrderNumber(number)
	if !ok || strings.TrimSpace(token) == "" {
		return nil, ErrOrderNotFound
	}
	return s.store.FindByNumberAndEmailAndToken(ctx, id, normalizeCustomerEmail(email).String(), hashTrackingToken(strings.TrimSpace(token)))
}

// Get returns one order by ID for the admin surface.
func (s *Service) Get(ctx context.Context, id int64) (*Order, error) {
	if id <= 0 {
		return nil, ErrOrderNotFound
	}
	return s.store.FindByID(ctx, id)
}

// List returns the newest orders for the admin surface: newest first, the
// status filter optional, keyed exclusively on the cursor the next page
// starts from. Items are not loaded — lists render summaries.
func (s *Service) List(
	ctx context.Context,
	status *Status,
	beforeID int64,
	limit int,
) ([]Order, error) {
	if status != nil {
		if _, ok := ParseStatus(string(*status)); !ok {
			return nil, ErrInvalidStatus
		}
	}
	return s.store.List(ctx, status, beforeID, normalizeList(limit))
}

// BuildInvoice derives the render-ready invoice for one order: the
// frozen lines, the shipping line, and the display-only tax breakdown at
// the configured PPN rate. The document is derived on demand — nothing is
// persisted — so it always reflects the current record. Unknown orders
// read as not found.
func (s *Service) BuildInvoice(ctx context.Context, id int64, settings InvoiceSettings) (*InvoiceDocument, error) {
	order, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	rate := settings.TaxRate
	if rate < 0 {
		rate = 0
	}
	currency := order.Currency
	total := NewMoney(order.TotalMinor(), currency)
	tax := NewMoney(int64(math.Round(float64(total.Amount)*rate/100)), currency)

	lines := make([]InvoiceLine, 0, len(order.Items))
	for i := range order.Items {
		item := order.Items[i]
		lines = append(lines, InvoiceLine{
			Title:     item.Title,
			SKU:       item.SKU,
			Quantity:  item.Quantity,
			UnitPrice: NewMoney(item.UnitPrice, currency),
			LineTotal: NewMoney(item.LineTotal(), currency),
		})
	}
	return &InvoiceDocument{
		Number:        invoiceNumber(order.ID),
		OrderNumber:   order.DisplayNumber(),
		IssuedAt:      time.Now().UTC(),
		CustomerName:  order.CustomerName,
		CustomerEmail: order.CustomerEmail,
		CustomerPhone: order.CustomerPhone,
		ShipTo:        order.ShipTo,
		SellerName:    settings.SellerName,
		SellerAddress: settings.SellerAddress,
		SellerTaxID:   settings.SellerTaxID,
		Lines:         lines,
		Subtotal:      NewMoney(order.Subtotal, currency),
		Shipping:      NewMoney(order.ShippingMinor, currency),
		TaxRate:       rate,
		Tax:           tax,
		Total:         total,
	}, nil
}

// Transition moves an order along the lifecycle after validating the
// target status and the transition legality, emails the customer
// (best-effort), and returns the refreshed order. The paid target is
// refused here by contract: payment confirmation is the only path into
// paid, through ConfirmPaid — an order surface can never mint a payment.
func (s *Service) Transition(ctx context.Context, id int64, to Status) (*Order, error) {
	if _, ok := ParseStatus(string(to)); !ok {
		return nil, ErrInvalidStatus
	}
	if to == StatusPaid {
		return nil, ErrInvalidTransition
	}
	order, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !CanTransition(order.Status, to) {
		return nil, ErrInvalidTransition
	}
	from := order.Status
	if err := s.store.SetStatus(ctx, order.ID, to); err != nil {
		return nil, fmt.Errorf("failed to update order status: %w", err)
	}
	// Stock returns on the fulfillment unwind: cancelled and refunded
	// orders that still hold their goods restock line for line, mirroring
	// what Reserve took. Shipment and delivery end the unwind — goods the
	// customer received are not reclaimed from the catalog. Pending
	// orders never restock: payment claims only start at confirmation.
	if (from == StatusPaid || from == StatusPacked) && (to == StatusCancelled || to == StatusRefunded) {
		_ = s.inventory.Release(ctx, order.Items)
	}
	// Re-read for authoritative storage timestamps.
	refreshed, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.mail.SendOrderStatusEmail(ctx, refreshed, from)
	return refreshed, nil
}

// SetShipping replaces the shipping line of a pending order and returns
// the refreshed order. Shipping is admin-set bookkeeping for the manual
// flow — weigh the parcel, then charge it — and only a pending order can
// carry it: once paid the amount is settled, and correction rides reject
// and resubmit. No email rides the write: the customer learns the full
// total when the proof records it.
func (s *Service) SetShipping(ctx context.Context, id int64, amount int64) (*Order, error) {
	if amount < 0 {
		return nil, ErrInvalidShipping
	}
	order, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if order.Status != StatusPending {
		return nil, ErrInvalidTransition
	}
	wrote, err := s.store.SetShipping(ctx, id, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to update order shipping: %w", err)
	}
	if !wrote {
		return nil, ErrInvalidTransition
	}
	// Re-read for authoritative storage timestamps.
	return s.Get(ctx, id)
}

// ConfirmPaid flips a pending order to paid on a confirmed payment and
// decrements its stock first. Reserve and the conditional flip together
// arbitrate racing confirms, retried webhooks, and the expiry sweep:
// only one caller wins the flip, and whoever loses releases the stock it
// claimed so the winner's decrement stands alone. A stock shortfall
// leaves the order pending for fulfillment to top up before the payment
// path retries.
func (s *Service) ConfirmPaid(ctx context.Context, id int64) (*Order, error) {
	order, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if order.Status != StatusPaid && order.Status != StatusPending {
		return nil, ErrInvalidTransition
	}
	if order.Status == StatusPaid {
		// Already settled — webhook retries and double submits read the
		// settled order instead of re-decrementing stock.
		return order, nil
	}
	if err := s.inventory.Reserve(ctx, order.Items); err != nil {
		return nil, fmt.Errorf("failed to reserve stock: %w", err)
	}
	flipped, err := s.store.MarkPaid(ctx, id)
	if err != nil {
		// Storage failed after the claim: release the stock back so the
		// leftover is not a phantom decrement.
		_ = s.inventory.Release(ctx, order.Items)
		return nil, fmt.Errorf("failed to mark order paid: %w", err)
	}
	if !flipped {
		// The flip lost the race — a parallel verify, a retried webhook,
		// or the expiry sweep claimed the order first. Unwind the claim;
		// the authoritative row says what actually happened.
		_ = s.inventory.Release(ctx, order.Items)
		return s.Get(ctx, id)
	}
	paid, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.mail.SendOrderStatusEmail(ctx, paid, StatusPending)
	return paid, nil
}

// ExpireStale flips up to limit pending orders created before the
// boundary to expired and emails each flipped order's customer
// (best-effort), returning the flipped IDs. Items load with each read;
// their quantities are irrelevant here because expiry never touched
// stock.
func (s *Service) ExpireStale(ctx context.Context, before time.Time, limit int) ([]int64, error) {
	ids, err := s.store.ExpirePending(ctx, before, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to expire stale orders: %w", err)
	}
	for _, id := range ids {
		order, err := s.Get(ctx, id)
		if err != nil {
			continue // the sweep's read miss cannot unwind a legal flip
		}
		_ = s.mail.SendOrderStatusEmail(ctx, order, StatusPending)
	}
	return ids, nil
}

// lineTitle snapshots the product title for a checkout line: the
// published title, or the generic fallback when the product no longer
// resolves.
func (s *Service) lineTitle(ctx context.Context, contentID int) string {
	title, ok := s.content.ProductTitle(ctx, contentID)
	if ok {
		if trimmed := strings.TrimSpace(title); trimmed != "" {
			return trimmed
		}
	}
	return FallbackProductTitle
}

// NewService builds the service over the persistence, cart, content,
// mail, and inventory seams, stamping the adopted platform currency into
// every checkout snapshot.
func NewService(
	store OrderStore,
	cart CartSource,
	content ContentSource,
	mail Mailer,
	inventory Inventory,
	currency string,
	opts ...ServiceOption,
) *Service {
	service := &Service{
		store:     store,
		cart:      cart,
		content:   content,
		mail:      mail,
		inventory: inventory,
		currency:  currency,
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}
