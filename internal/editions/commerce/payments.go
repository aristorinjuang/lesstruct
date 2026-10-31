package commerce

import (
	"context"
	"errors"

	"github.com/aristorinjuang/lesstruct/internal/email"
	ordersdomain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
	paymentsdomain "github.com/aristorinjuang/lesstruct/modules/payments/domain"
)

// orderRef maps one order onto the seam's writable-edge view. The number
// is the derived display value orders never stores; the tracking token
// rides for the gateway finish redirect.
func orderRef(order *ordersdomain.Order) *paymentsdomain.OrderRef {
	return &paymentsdomain.OrderRef{
		ID:            order.ID,
		OrderNumber:   order.DisplayNumber(),
		Status:        string(order.Status),
		TotalMinor:    order.TotalMinor(),
		CustomerEmail: order.CustomerEmail,
		Currency:      order.Currency,
		TrackingToken: order.TrackingToken,
	}
}

// mapOrdersSeam folds the order sentinel vocabulary into the seam's.
func mapOrdersSeam(err error) error {
	switch {
	case errors.Is(err, ordersdomain.ErrOrderNotFound):
		return paymentsdomain.ErrOrderNotFound
	case errors.Is(err, ordersdomain.ErrInvalidTransition):
		return paymentsdomain.ErrOrderNotPayable
	case errors.Is(err, ordersdomain.ErrInsufficientStock):
		return paymentsdomain.ErrInsufficientStock
	}
	return err
}

// mapRefundSeam folds the order sentinel vocabulary onto the refund
// contract: a lifecycle refusal reads as not-refundable, everything else
// keeps its ground.
func mapRefundSeam(err error) error {
	switch {
	case errors.Is(err, ordersdomain.ErrOrderNotFound):
		return paymentsdomain.ErrOrderNotFound
	case errors.Is(err, ordersdomain.ErrInvalidTransition):
		return paymentsdomain.ErrOrderNotRefundable
	case errors.Is(err, ordersdomain.ErrInsufficientStock):
		return paymentsdomain.ErrInsufficientStock
	}
	return err
}

// orderSeams adapts the orders service onto the payments module's Orders
// seam: payment proofs resolve the payable target through the same
// number-plus-email gate guest tracking uses, and verification confirms
// through the payment-gated ConfirmPaid path — the inventory claim, the
// idempotent settle, and the flip arbitration all live orders-side.
type orderSeams struct {
	service *ordersdomain.Service
}

// Details resolves one payable order for the guest gate.
func (a orderSeams) Details(ctx context.Context, orderNumber string, email string, token string) (*paymentsdomain.OrderRef, error) {
	order, err := a.service.Tracking(ctx, orderNumber, email, token)
	if err != nil {
		return nil, mapOrdersSeam(err)
	}
	ref := orderRef(order)
	// The presented token just passed the tracking gate, so it is the
	// buyer's own secret: echo it for the gateway finish redirect, which
	// lands back on this order's tracking view.
	ref.TrackingToken = token
	return ref, nil
}

// DetailsByID resolves one payable order directly for the administrative
// decision payloads.
func (a orderSeams) DetailsByID(ctx context.Context, orderID int64) (*paymentsdomain.OrderRef, error) {
	order, err := a.service.Get(ctx, orderID)
	if err != nil {
		return nil, mapOrdersSeam(err)
	}
	return orderRef(order), nil
}

// DetailsByNumber resolves the payable view from a presented display
// number. Only the signature-verified webhook calls it with an
// unauthenticated number, so parsing failures read as unknown orders.
func (a orderSeams) DetailsByNumber(ctx context.Context, orderNumber string) (*paymentsdomain.OrderRef, error) {
	id, ok := ordersdomain.ParseOrderNumber(orderNumber)
	if !ok {
		return nil, paymentsdomain.ErrOrderNotFound
	}
	order, err := a.service.Get(ctx, id)
	if err != nil {
		return nil, mapOrdersSeam(err)
	}
	return orderRef(order), nil
}

// Confirm flips the pending order to paid on a verified payment.
func (a orderSeams) Confirm(ctx context.Context, orderID int64) (*paymentsdomain.OrderRef, error) {
	order, err := a.service.ConfirmPaid(ctx, orderID)
	if err != nil {
		return nil, mapOrdersSeam(err)
	}
	return orderRef(order), nil
}

// Refund transitions the order refunded through the lifecycle — the
// stock unwind lives orders-side; goods the customer received stay
// reclaimed-none by the orders service semantics.
func (a orderSeams) Refund(ctx context.Context, orderID int64) error {
	if _, err := a.service.Transition(ctx, orderID, ordersdomain.StatusRefunded); err != nil {
		return mapRefundSeam(err)
	}
	return nil
}

// NewOrderSeams builds the Orders seam adapter over the adopted orders
// service.
func NewOrderSeams(service *ordersdomain.Service) paymentsdomain.Orders {
	return orderSeams{service: service}
}

// paymentMailer adapts the platform email service onto the payments
// module's Mailer seam: money and references ride as display strings, so
// the email service needs no commerce knowledge. Failures log adapter-side
// and never fail a submission or decision.
type paymentMailer struct {
	email *email.Service
}

// SendPaymentReceivedEmail confirms the proof submission.
func (m paymentMailer) SendPaymentReceivedEmail(
	ctx context.Context,
	order *paymentsdomain.OrderRef,
	method paymentsdomain.Method,
	reference string,
) error {
	return m.email.SendPaymentReceivedEmail(
		ctx,
		order.CustomerEmail,
		order.OrderNumber,
		string(method),
		reference,
		ordersdomain.NewMoney(order.TotalMinor, order.Currency).Format(),
	)
}

// SendPaymentRejectedEmail explains a rejected proof.
func (m paymentMailer) SendPaymentRejectedEmail(
	ctx context.Context,
	order *paymentsdomain.OrderRef,
	method paymentsdomain.Method,
	reference string,
) error {
	return m.email.SendPaymentRejectedEmail(
		ctx,
		order.CustomerEmail,
		order.OrderNumber,
		string(method),
		reference,
	)
}

// NewPaymentMailer builds the Mailer seam adapter over the platform email
// service.
func NewPaymentMailer(service *email.Service) paymentsdomain.Mailer {
	return paymentMailer{email: service}
}
