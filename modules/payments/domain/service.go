package payments

import (
	"context"
	"errors"
	"slices"
	"strings"
)

const (
	// ListLimitDefault is the sweep page size the service applies when a
	// caller passes no usable limit.
	ListLimitDefault = 50
	// ListLimitCeiling caps one page so a large limit cannot scan the
	// whole history.
	ListLimitCeiling = 100
)

// errPayment is the sentinel family surfaced by the service.
type errPayment string

func (e errPayment) Error() string { return string(e) }

// Sentinels surfaced by the service and mapped to HTTP responses by the
// handlers. A tracking-style gate on the public surface means lookup
// misses and email mismatches read the same.
var (
	// ErrPaymentNotFound is returned when a payment record does not
	// exist.
	ErrPaymentNotFound = errPayment("payment not found")
	// ErrOrderNotFound is returned when the referenced order cannot be
	// resolved — unknown number, or the email does not match it. The
	// reasons are deliberately indistinguishable.
	ErrOrderNotFound = errPayment("order not found")
	// ErrOrderNotPayable is returned when the order exists but cannot
	// accept a payment proof: it is already settled, expired, or
	// cancelled.
	ErrOrderNotPayable = errPayment("order is not payable")
	// ErrInsufficientStock is returned when payment confirmation cannot
	// cover the sold quantities from the orders inventory seam. The
	// order stays pending for fulfillment to top up and retry.
	ErrInsufficientStock = errPayment("insufficient stock for the order")
	// ErrInvalidReference is returned when the proof text is blank or
	// exceeds the cap.
	ErrInvalidReference = errPayment("reference must be between 1 and 500 characters")
	// ErrInvalidMethod is returned when the selected method is not one
	// the store adopted in its configuration.
	ErrInvalidMethod = errPayment("payment method is not offered")
	// ErrPaymentNotPending is returned when an administrative verify or
	// reject arrives for a record whose decision was already made.
	ErrPaymentNotPending = errPayment("payment is not pending")
	// ErrGatewayUnavailable is returned when a hosted checkout is
	// requested but the store did not adopt a gateway driver.
	ErrGatewayUnavailable = errPayment("no payment gateway is configured")
	// ErrGatewayUpstream is returned when the adopted gateway driver
	// reaches upstream but the call fails: transport errors, non-2xx
	// replies, and undecodable bodies. Operators read it as "the gateway
	// or its key is at fault", never as a store bug.
	ErrGatewayUpstream = errPayment("payment gateway upstream call failed")
	// ErrAmountMismatch is returned when a webhook settlement's gross
	// amount disagrees with the order it pays.
	ErrAmountMismatch = errPayment("webhook amount does not match the order")
	// ErrOrderNotRefundable is returned when a webhook refund arrives
	// for an order whose lifecycle cannot refund.
	ErrOrderNotRefundable = errPayment("order is not refundable")
)

// normalizeList clamps a caller's page size onto the service sweep
// bounds: unusable values take the default, oversized values the
// ceiling.
func normalizeList(limit int) int {
	if limit <= 0 {
		return ListLimitDefault
	}
	return min(limit, ListLimitCeiling)
}

// PaymentsStore is the persistence seam the service is built over. The
// single-source store satisfies it; tests use mocks.
type PaymentsStore interface {
	// Insert persists one submitted payment record and returns the
	// generated ID.
	Insert(ctx context.Context, payment *Payment) (int64, error)
	// FindByID returns the record, or ErrPaymentNotFound.
	FindByID(ctx context.Context, id int64) (*Payment, error)
	// FindByReference returns the record carrying the gateway
	// transaction reference, or ErrPaymentNotFound.
	FindByReference(ctx context.Context, reference string) (*Payment, error)
	// List returns the newest records carrying the given status (nil is
	// any), strictly below the keyset boundary (0 means from the
	// newest), at most limit rows.
	List(ctx context.Context, status *Status, beforeID int64, limit int) ([]Payment, error)
	// ListByOrder returns every record paying one order, newest first.
	ListByOrder(ctx context.Context, orderID int64) ([]Payment, error)
	// SetStatus flips the record to the given status, recording who
	// decided it.
	SetStatus(ctx context.Context, id int64, status Status, verifiedBy string) error
}

// Orders is the seam to the orders module: resolve the payable target and
// confirm its payment. The adapter supplies the live order service; its
// failures map onto ErrOrderNotFound (unknown number, email mismatch, or
// unknown ID), ErrOrderNotPayable (no longer pending), and
// ErrInsufficientStock (the inventory claim was refused).
type Orders interface {
	// Details resolves the payable order the guest is submitting
	// against, gated on the number, email, and per-order tracking token
	// matching — the same gate as guest tracking, so lookups never
	// enumerate. A miss is ErrOrderNotFound regardless of reason.
	Details(ctx context.Context, orderNumber string, email string, token string) (*OrderRef, error)
	// DetailsByID resolves one order's payable view directly. The
	// administrative reject decision rides it for the response payload;
	// it must not widen the guest gate — callers sit behind the admin
	// capability chain.
	DetailsByID(ctx context.Context, orderID int64) (*OrderRef, error)
	// DetailsByNumber resolves the payable view from a presented
	// display number. The gateway webhook settlement is the signed
	// caller — the number arrives from the gateway's signed event,
	// never from an unauthenticated guest.
	DetailsByNumber(ctx context.Context, orderNumber string) (*OrderRef, error)
	// Confirm flips the pending order to paid, decrementing its stock;
	// an order that left pending refuses with ErrOrderNotPayable, and a
	// refused inventory claim with ErrInsufficientStock.
	Confirm(ctx context.Context, orderID int64) (*OrderRef, error)
	// Refund transitions the order to refunded through the order
	// lifecycle — the stock unwind lives orders-side. An order the
	// lifecycle cannot refund refuses with ErrOrderNotRefundable.
	Refund(ctx context.Context, orderID int64) error
}

// Mailer delivers the customer-facing payment emails. Delivery is
// best-effort by contract: the payment record is authoritative, a failed
// send never fails a submission or a decision, and the adapter logs its
// own failures.
type Mailer interface {
	// SendPaymentReceivedEmail confirms the proof submission: what was
	// submitted, and that a human decision follows.
	SendPaymentReceivedEmail(ctx context.Context, order *OrderRef, method Method, reference string) error
	// SendPaymentRejectedEmail explains a rejected proof: the customer
	// may resubmit with corrected references while the order is open.
	SendPaymentRejectedEmail(ctx context.Context, order *OrderRef, method Method, reference string) error
}

// Service is the payments domain service: manual proof submission over the
// payment record store, gated and confirmed through the orders seam, with
// an optional gateway driver for hosted checkout.
type Service struct {
	store   PaymentsStore
	orders  Orders
	mail    Mailer
	gateway Gateway
	methods []Method
}

// payableOrder resolves the order behind a pending proof and refuses when
// it already left pending: a gateway settlement (or a racing admin
// decision) reached it first, and ConfirmPaid's idempotent early-return
// must not bless a second verified record. The check-then-confirm window
// stays arbitrated orders-side.
func (s *Service) payableOrder(ctx context.Context, payment *Payment) (*OrderRef, error) {
	order, err := s.orders.DetailsByID(ctx, payment.OrderID)
	if err != nil {
		return nil, err
	}
	if order.Status != string(StatusPending) {
		return nil, ErrOrderNotPayable
	}
	return order, nil
}

// Methods returns the offered payment methods, in adoption order. The
// list is static store configuration, never merged from submissions.
func (s *Service) Methods() []Method {
	return s.methods
}

// Submit records one manual payment proof against a payable order: the
// guest is gated on number and email, the method must be offered, and the
// reference text is stored verbatim. The expected amount snapshots with
// the record, so a later price edit can never rewrite the proof. Returns
// the persisted record; the submission acknowledgment is best-effort.
func (s *Service) Submit(ctx context.Context, orderNumber string, email string, token string, method Method, reference string) (*Payment, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" || len([]rune(reference)) > MaxReferenceRunes {
		return nil, ErrInvalidReference
	}
	if !slices.Contains(s.methods, method) {
		return nil, ErrInvalidMethod
	}
	order, err := s.orders.Details(ctx, orderNumber, strings.ToLower(strings.TrimSpace(email)), strings.TrimSpace(token))
	if err != nil {
		return nil, err
	}
	if order.Status != string(StatusPending) {
		return nil, ErrOrderNotPayable
	}

	id, err := s.store.Insert(ctx, &Payment{
		OrderID:   order.ID,
		Method:    method,
		Status:    StatusPending,
		Reference: reference,
		Amount:    order.TotalMinor,
	})
	if err != nil {
		return nil, err
	}
	// Re-read for authoritative storage timestamps.
	created, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.mail.SendPaymentReceivedEmail(ctx, order, method, reference)
	return created, nil
}

// Verify decides a pending proof for the offered method: confirming flips
// the order to paid through the orders seam — the inventory claim, the
// idempotent settle, and the flip arbitration all live there — then marks
// the record verified with the deciding administrator. Claims over
// non-pending records refuse without touching the order, and so do claims
// whose order already left pending: a gateway settlement that landed first
// must never gain a second verified record beside it.
func (s *Service) Verify(ctx context.Context, id int64, verifiedBy string) (*Payment, error) {
	payment, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if payment.Status != StatusPending {
		return nil, ErrPaymentNotPending
	}
	if _, err := s.payableOrder(ctx, payment); err != nil {
		return nil, err
	}
	if _, err := s.orders.Confirm(ctx, payment.OrderID); err != nil {
		return nil, err
	}
	if err := s.store.SetStatus(ctx, payment.ID, StatusVerified, verifiedBy); err != nil {
		return nil, err
	}
	// Re-read for authoritative storage timestamps.
	return s.store.FindByID(ctx, id)
}

// Reject decides a pending proof the other way: the order stays pending —
// the customer may correct and resubmit — and the rejection notice is
// best-effort. A proof whose order already left pending refuses instead:
// rejecting a paid order would email a lie and invite a duplicate
// transfer.
func (s *Service) Reject(ctx context.Context, id int64) (*Payment, error) {
	payment, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if payment.Status != StatusPending {
		return nil, ErrPaymentNotPending
	}
	order, err := s.payableOrder(ctx, payment)
	if err != nil {
		return nil, err
	}
	if err := s.store.SetStatus(ctx, payment.ID, StatusRejected, ""); err != nil {
		return nil, err
	}
	// Re-read for authoritative storage timestamps.
	decided, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.mail.SendPaymentRejectedEmail(ctx, order, payment.Method, payment.Reference)
	return decided, nil
}

// List returns the newest payment records carrying the given status (nil
// is any), strictly below the keyset boundary (0 means from the newest),
// at most limit rows.
func (s *Service) List(ctx context.Context, status *Status, beforeID int64, limit int) ([]Payment, error) {
	return s.store.List(ctx, status, beforeID, normalizeList(limit))
}

// Detail returns one payment record with its order context for the admin
// decision surface.
func (s *Service) Detail(ctx context.Context, id int64) (*PaymentDetail, error) {
	payment, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	order, err := s.orders.DetailsByID(ctx, payment.OrderID)
	if err != nil {
		return nil, err
	}
	return &PaymentDetail{
		Payment:     *payment,
		OrderNumber: order.OrderNumber,
		OrderStatus: order.Status,
		OrderTotal:  order.TotalMinor,
	}, nil
}

// ListByOrder returns every record paying one order, newest first, for
// the order's payment history.
func (s *Service) ListByOrder(ctx context.Context, orderID int64) ([]Payment, error) {
	return s.store.ListByOrder(ctx, orderID)
}

// Mint opens a hosted checkout session for a payable order, gated on the
// number, email, and tracking token matching like every guest surface. The
// session is one-time: the gateway self-verifies and reports back by
// webhook.
func (s *Service) Mint(ctx context.Context, orderNumber string, email string, token string) (*GatewaySession, error) {
	if s.gateway == nil {
		return nil, ErrGatewayUnavailable
	}
	order, err := s.orders.Details(ctx, orderNumber, strings.ToLower(strings.TrimSpace(email)), strings.TrimSpace(token))
	if err != nil {
		return nil, err
	}
	if order.Status != string(StatusPending) {
		return nil, ErrOrderNotPayable
	}
	return s.gateway.Mint(ctx, order)
}

// HandleNotification folds one authenticated webhook event onto the
// platform: settlements confirm the order (the inventory claim, the
// idempotent settle, and the flip arbitration live orders-side) and
// record a self-verifying payment row deduplicated on the transaction
// reference; refunds transition the order refunded through the lifecycle.
// Unknown statuses acknowledge without action so the gateway stops
// retrying, a refunded-already order acknowledges the same way, and a
// card capture still under fraud review waits for the follow-up event.
func (s *Service) HandleNotification(ctx context.Context, event *Notification) error {
	switch event.Status {
	case "settlement", "capture":
		// A card capture can still be under fraud review: only an
		// accepted verdict settles. Non-card channels omit the field
		// (and Midtrans sends settlement for them anyway), so absence
		// settles too. A challenge acks here and the follow-up event
		// (settlement or deny) drives the rest.
		if event.FraudStatus != "" && event.FraudStatus != "accept" {
			return nil
		}
		return s.settleNotification(ctx, event)
	case "refund", "partial_refund":
		return s.refundNotification(ctx, event)
	default:
		return nil
	}
}

// settleNotification confirms a settled gateway payment.
func (s *Service) settleNotification(ctx context.Context, event *Notification) error {
	_, err := s.store.FindByReference(ctx, event.TransactionID)
	switch {
	case err == nil:
		// The event's own transaction reference has already been
		// recorded and confirmed: webhook retries converge here.
		return nil
	case !errors.Is(err, ErrPaymentNotFound):
		// A genuine storage failure must not masquerade as an absent
		// record — recording a second verified row for one transaction
		// is worse than answering the retry later. The fold stops here
		// and the gateway retries.
		return err
	}

	order, err := s.orders.DetailsByNumber(ctx, event.OrderNumber)
	if err != nil {
		return err
	}
	if order.TotalMinor != event.GrossMinor {
		return ErrAmountMismatch
	}
	if _, err := s.orders.Confirm(ctx, order.ID); err != nil {
		return err
	}
	if _, err := s.store.Insert(ctx, &Payment{
		OrderID:    order.ID,
		Method:     MethodMidtrans,
		Status:     StatusVerified,
		Reference:  event.TransactionID,
		Amount:     event.GrossMinor,
		VerifiedBy: "midtrans",
	}); err != nil {
		return err
	}
	return nil
}

// refundNotification transitions a refunded gateway payment.
func (s *Service) refundNotification(ctx context.Context, event *Notification) error {
	order, err := s.orders.DetailsByNumber(ctx, event.OrderNumber)
	if err != nil {
		return err
	}
	if err := s.orders.Refund(ctx, order.ID); err != nil {
		// An order already refunded acknowledges the retry rather than
		// surfacing the lifecycle refusal: the state the gateway
		// reported stands.
		if errors.Is(err, ErrOrderNotRefundable) {
			return nil
		}
		return err
	}
	return nil
}

// GatewayAdopted reports whether the service drives a hosted checkout.
func (s *Service) GatewayAdopted() bool {
	return s.gateway != nil
}

// NewPaymentsService builds the service over the persistence, orders, and
// mail seams, adopting the offered methods in configuration order and an
// optional hosted-checkout gateway (nil degrades to manual proofs).
func NewPaymentsService(
	store PaymentsStore,
	orders Orders,
	mail Mailer,
	gateway Gateway,
	methods []Method,
) *Service {
	return &Service{
		store:   store,
		orders:  orders,
		mail:    mail,
		gateway: gateway,
		methods: append([]Method(nil), methods...),
	}
}
