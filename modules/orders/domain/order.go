// Package orders is the official commerce order module: first-class orders
// snapshotted at checkout, a domain-enforced status lifecycle, and guest
// order tracking. The module never imports the cart, content, or catalog
// packages — checkout consumes the cart through the CartSource seam and
// snapshots product titles through the ContentSource seam, with the
// composition root providing the adapters.
package orders

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel/capability"
	"github.com/aristorinjuang/lesstruct/kernel/legal"
)

const (
	// CapabilityManage is the permission that gates order admin operations.
	// The module contributes it to the registry (RoleProvider) and gates
	// its admin surface to Admin for now, so a role-registry grants
	// mechanism can widen access later without changing the wire behavior.
	CapabilityManage = capability.Capability("orders.manage")
	// OrderNumberPrefix precedes every rendered order number.
	OrderNumberPrefix = "ORD-"
	// OrderNumberWidth is the zero-padded digit width of an order number.
	OrderNumberWidth = 6
	// TrackingTokenBytes is the entropy of the per-order tracking secret.
	// 16 bytes (128 bits) base64url-encoded keeps the token unguessable
	// even though order numbers themselves are sequential.
	TrackingTokenBytes = 16
)

// errOrder keeps sentinel construction tidy.
type errOrder string

func (e errOrder) Error() string { return string(e) }

var (
	// ErrOrderNotFound is returned when an order does not exist, an order
	// number cannot parse, or the tracking email does not match. The
	// reasons are deliberately indistinguishable so lookup misses never
	// enumerate orders.
	ErrOrderNotFound = errOrder("order not found")
	// ErrInvalidTransition is returned when the lifecycle forbids a status
	// change.
	ErrInvalidTransition = errOrder("order transition is not allowed")
	// ErrInvalidStatus is returned when a status value sits outside the
	// vocabulary.
	ErrInvalidStatus = errOrder("status is invalid")
	// ErrInvalidEmail is returned when the checkout email is missing or
	// malformed.
	ErrInvalidEmail = errOrder("email is required and must be valid")
	// ErrInvalidName is returned when the checkout name is missing.
	ErrInvalidName = errOrder("name is required")
	// ErrInvalidAddress is returned when the shipping address misses a
	// required field.
	ErrInvalidAddress = errOrder("shipping address is incomplete")
	// ErrInvalidShipping is returned when the shipping line is negative.
	ErrInvalidShipping = errOrder("shipping amount must be non-negative")
	// ErrEmptyCart is returned when the presented cart holds no lines to
	// check out.
	ErrEmptyCart = errOrder("cart is empty")
	// ErrCartGone is returned when the presented cart no longer exists:
	// unknown tokens and already-consumed carts read the same, which is
	// exactly what makes consumed carts a natural double-submit guard.
	ErrCartGone = errOrder("cart is gone")
	// ErrOutOfStock is returned when a cart line is out of stock at
	// checkout. Stock is only validated here; decrement happens at
	// payment.
	ErrOutOfStock = errOrder("item is out of stock")
	// ErrLineUnavailable is returned when a cart line no longer resolves
	// to a sellable product or variant.
	ErrLineUnavailable = errOrder("cart line is no longer available")
	// ErrInsufficientStock is returned when payment confirmation cannot
	// cover the sold quantities from the inventory seam. The order stays
	// pending so the stock can be topped up and the payment path retried.
	ErrInsufficientStock = errOrder("insufficient stock for the order")
	// ErrConsentRequired is returned when checkout demands an affirmative
	// consent act and the request does not carry one.
	ErrConsentRequired = errOrder("consent is required to place an order")
)

// Address is the shipping address stored as a JSON snapshot: structured
// enough for packing labels, never a rates or validation integration.
type Address struct {
	Street     string `json:"street"`
	City       string `json:"city"`
	Province   string `json:"province"`
	PostalCode string `json:"postalCode"`
}

// OrderItem is one snapshotted purchase line. SKU, title, and price freeze
// at checkout so order history survives price edits, content edits, and
// deletion. Lines are immutable after creation — the total derives from
// unit price and quantity, so no extended total is stored.
type OrderItem struct {
	ID               int64
	OrderID          int64
	ProductContentID int
	VariantID        *int64
	SKU              string
	Title            string
	UnitPrice        int64
	Quantity         int
}

// LineTotal extends the line into its snapshot total in minor units.
func (i *OrderItem) LineTotal() int64 {
	return i.UnitPrice * int64(i.Quantity)
}

// Order is the read shape of one order: the checkout snapshot plus the
// lifecycle state. Items are loaded on detail reads; list reads leave them
// nil and carry the read-time ItemCount aggregate instead.
type Order struct {
	ID            int64
	Status        Status
	Items         []OrderItem
	ItemCount     int
	CustomerEmail string
	CustomerName  string
	CustomerPhone string
	ShipTo        Address
	Currency      string
	Subtotal      int64
	ShippingMinor int64
	// TrackingTokenHash is the SHA-256 of the per-order tracking secret.
	// It is written at checkout and compared on guest lookups; it is
	// never selected back into API payloads. An empty hash marks an order
	// from before the token existed — such orders are untrackable through
	// the guest gate (the email-only gate was the vulnerability).
	TrackingTokenHash string
	// TrackingToken carries the freshly minted raw secret across checkout
	// only: it reaches the checkout response and the confirmation email
	// and is never persisted or returned again.
	TrackingToken string
	// Consent is the GDPR evidence captured at checkout: when the buyer
	// affirmed the terms and which document revisions the affirmation
	// referred to. Nil for orders placed before consent was enforced and
	// for stores that do not ask for it.
	Consent   *legal.Consent
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DisplayNumber renders the human-readable order number: the zero-padded
// order ID. Numbers are derived, never stored — there is nothing to keep
// in sync.
func (o *Order) DisplayNumber() string {
	return fmt.Sprintf("%s%0*d", OrderNumberPrefix, OrderNumberWidth, o.ID)
}

// TotalMinor renders the payable total: the frozen subtotal plus the
// shipping line. Shipping is admin-set while the order is pending and
// never stored anywhere else — one field, one truth.
func (o *Order) TotalMinor() int64 {
	return o.Subtotal + o.ShippingMinor
}

// ParseOrderNumber converts a presented order number back into its order
// ID, accepting the exact RenderedNumber shape (case-sensitive prefix,
// decimal digits).
func ParseOrderNumber(number string) (int64, bool) {
	digits, ok := strings.CutPrefix(number, OrderNumberPrefix)
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// CartSnapshotLine is the narrowed cart vocabulary checkout consumes: the
// line key, its live catalog posture, and the live unit price.
type CartSnapshotLine struct {
	ProductContentID int
	VariantID        *int64
	SKU              string
	Quantity         int
	UnitPrice        int64
	InStock          bool
	Available        bool
}

// CartSnapshot is the priced cart read the CartSource seam resolves at
// checkout time.
type CartSnapshot struct {
	Lines []CartSnapshotLine
}

// CartSource owns the cart's checkout lifecycle: the read that prices the
// presented cart and the atomic delete that claims it. The claim runs
// before the order row is stored, so only the winner of a parallel race on
// one token ever creates an order — and a failed order storage abandons
// the claim the customer rebuilds by re-adding lines. Unknown or
// malformed tokens map to ErrCartGone on both methods (a gone cart aborts
// the checkout with it); every other failure is a persistence problem
// that aborts the checkout verbatim.
type CartSource interface {
	Snapshot(ctx context.Context, token string) (CartSnapshot, error)
	Consume(ctx context.Context, token string) error
}

// ContentSource resolves published product titles for the item snapshot.
// A miss — deleted or unpublished product — is reported as found=false and
// falls back to the module's generic title.
type ContentSource interface {
	ProductTitle(ctx context.Context, contentID int) (string, bool)
}

// Mailer delivers the customer-facing order emails. Delivery is
// best-effort by contract: the order record is authoritative, so a failed
// send never fails the checkout or a transition, and the adapter logs its
// own failures.
type Mailer interface {
	SendOrderConfirmationEmail(ctx context.Context, order *Order) error
	SendOrderStatusEmail(ctx context.Context, order *Order, from Status) error
}

// Inventory is the stock ledger seam between orders and the catalog:
// payment confirmation decrements the sold quantities, and symmetric
// unwinds — a lost flip race, a cancellation or refund before shipment —
// restock them. The line quantities are exactly what a strict Reserve
// consumed, so Release always mirrors its counterpart line for line. The
// adapter maps the catalog's own refused-move sentinel onto
// ErrInsufficientStock; a failed restock is logged adapter-side and does
// not rewind the transition that triggered it.
type Inventory interface {
	// Reserve decrements every line's tracked bucket. A bucket that
	// cannot cover its quantity refuses the whole call with
	// ErrInsufficientStock — the atomic guard keeps racing moves
	// serialized at the row.
	Reserve(ctx context.Context, items []OrderItem) error
	// Release restocks every line with its full quantity, mirroring a
	// strict Reserve or reverting one.
	Release(ctx context.Context, items []OrderItem) error
}

// OrderStore is the persistence seam the service is built over. The
// single-source store satisfies it; tests use mocks.
type OrderStore interface {
	// CreateOrder persists the order and its item snapshot in one
	// transaction and returns the generated order ID.
	CreateOrder(ctx context.Context, order *Order) (int64, error)
	// FindByID returns the order with its items, or ErrOrderNotFound.
	FindByID(ctx context.Context, id int64) (*Order, error)
	// FindByNumberAndEmailAndToken returns the order with its items when
	// the order ID, the customer email, and the tracking-token hash all
	// match, or ErrOrderNotFound. The three factors are the guest gate:
	// sequential numbers and knowable emails alone must never open an
	// order.
	FindByNumberAndEmailAndToken(ctx context.Context, id int64, email string, tokenHash string) (*Order, error)
	// List returns the newest orders carrying the given status (nil is
	// any), strictly after none — beforeID is the exclusive, upper keyset
	// boundary (0 means from the newest), at most limit rows. Items are
	// not loaded.
	List(ctx context.Context, status *Status, beforeID int64, limit int) ([]Order, error)
	// SetStatus replaces the lifecycle status of the order.
	SetStatus(ctx context.Context, id int64, status Status) error
	// SetShipping replaces the shipping line of a pending order and
	// reports whether this call did the write. The pending guard lives
	// in the statement, so a status flip racing the write loses it.
	SetShipping(ctx context.Context, id int64, amount int64) (bool, error)
	// MarkPaid flips a pending order to paid and reports whether this
	// call did the flip. The pending guard lives in the statement, so the
	// loser of a racing confirm, webhook retry, or expiry sweep reads
	// false.
	MarkPaid(ctx context.Context, id int64) (bool, error)
	// ExpirePending flips up to limit pending orders created before the
	// boundary to expired and returns the flipped IDs; every flip keeps
	// the pending guard.
	ExpirePending(ctx context.Context, before time.Time, limit int) ([]int64, error)
}
