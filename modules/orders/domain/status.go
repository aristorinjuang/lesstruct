package orders

import "slices"

// Status is an order lifecycle state: a snapshot of what the fulfillment
// pipeline has done with the order. Payment confirmation moves pending to
// paid; fulfillment walks paid through packed, shipped, and delivered.
// Cancelled, refunded, and expired are terminal.
type Status string

const (
	// StatusPending is a placed order awaiting payment confirmation.
	StatusPending Status = "pending"
	// StatusPaid is a payment-confirmed order awaiting preparation.
	StatusPaid Status = "paid"
	// StatusPacked is a packaged order awaiting shipment.
	StatusPacked Status = "packed"
	// StatusShipped is an order handed to the carrier.
	StatusShipped Status = "shipped"
	// StatusDelivered is an order received by the customer.
	StatusDelivered Status = "delivered"
	// StatusCancelled is an order withdrawn before shipment.
	StatusCancelled Status = "cancelled"
	// StatusRefunded is an order whose payment was returned to the
	// customer.
	StatusRefunded Status = "refunded"
	// StatusExpired is a pending order never confirmed in time.
	StatusExpired Status = "expired"
)

var (
	// lifecycle enumerates the legal status transitions. The map omits
	// terminal states entirely: cancelled, refunded, and expired close an
	// order's history, so nothing may leave them. The paid move is not
	// surface-triggerable: payment confirmation is the only path in,
	// through ConfirmPaid. Expired flips both ways — the admin surface and
	// the expiry sweep.
	lifecycle = map[Status][]Status{
		StatusPending:   {StatusPaid, StatusCancelled, StatusExpired},
		StatusPaid:      {StatusPacked, StatusCancelled, StatusRefunded},
		StatusPacked:    {StatusShipped, StatusCancelled, StatusRefunded},
		StatusShipped:   {StatusDelivered, StatusRefunded},
		StatusDelivered: {StatusRefunded},
	}
)

// CanTransition reports whether from may move to to under the order
// lifecycle. Every legal move appears in the lifecycle map; refusals are
// domain errors at the call site, not silent no-ops.
func CanTransition(from, to Status) bool {
	return slices.Contains(lifecycle[from], to)
}

// ParseStatus converts a wire status value into the vocabulary, reporting
// whether the value is a known state.
func ParseStatus(raw string) (Status, bool) {
	status := Status(raw)
	switch status {
	case StatusPending, StatusPaid, StatusPacked, StatusShipped,
		StatusDelivered, StatusCancelled, StatusRefunded, StatusExpired:
		return status, true
	default:
		return "", false
	}
}
