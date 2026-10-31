package payments

import "context"

// Gateway is the hosted-checkout seam: a driver mints one-time Snap
// sessions, authenticates Midtrans webhook notifications against the
// server key, and renders notification gross amounts into minor units.
// The composition root supplies the driver; with none adopted the mint
// surface answers ErrGatewayUnavailable and webhooks answer the same,
// so a store without a configured gateway degrades to manual proofs.
type Gateway interface {
	// Mint opens a hosted checkout session for the order: the item
	// details ride the driver's checkout request and the customer
	// completes payment on the gateway's domain.
	Mint(ctx context.Context, order *OrderRef) (*GatewaySession, error)
	// Authenticate reports whether a raw webhook notification carries
	// the key-signed checksum over the order number, notification
	// status code, and raw gross-amount string.
	Authenticate(orderNumber string, statusCode string, grossAmount string, signature string) bool
	// MinorUnits converts a notification gross-amount string ("522500.00")
	// into the minor units the platform stores.
	MinorUnits(grossAmount string) (int64, error)
}

// GatewaySession is one minted hosted checkout session: the token the
// client applies (Snap popup or redirect) and the redirect URL the
// browser lands on.
type GatewaySession struct {
	Token       string
	RedirectURL string
}

// Notification is the verified shape of one gateway webhook event, after
// authentication and amount conversion. Status values are the Midtrans
// transaction vocabulary (settlement, capture, refund,
// partial_refund, expire, cancel, deny); the service folds them onto the
// order lifecycle and self-verifying payment records. FraudStatus rides
// the card-capture review verdict (accept, challenge, deny); non-card
// channels omit it.
type Notification struct {
	OrderNumber   string
	TransactionID string
	Status        string
	FraudStatus   string
	GrossMinor    int64
}
