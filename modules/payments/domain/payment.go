package payments

import (
	"time"

	"github.com/aristorinjuang/lesstruct/kernel/capability"
)

const (
	// MaxReferenceRunes bounds the proof text: enough for a gateway
	// transaction identifier or a transfer reference, short enough that
	// one submission cannot flood the record.
	MaxReferenceRunes = 500

	// CapabilityManage gates payments admin actions; it is declared
	// module-local (a plain kernel capability string) so the kernel
	// vocabulary stays frozen and the module contributes it through
	// RoleProvider.
	CapabilityManage = capability.Capability("payments.manage")

	// MethodManual is the manual transfer flow: the customer submits a
	// proof reference and an administrator verifies it.
	MethodManual Method = "manual"

	// MethodMidtrans identifies the Midtrans Snap hosted-checkout flow.
	MethodMidtrans Method = "midtrans"

	// Verification statuses.
	StatusPending  Status = "pending"
	StatusVerified Status = "verified"
	StatusRejected Status = "rejected"
)

// Method is the payment-method vocabulary. Manual is the bank-transfer
// proof flow; gateway drivers arrive as additional constants behind the
// same type.
type Method string

// Status is the verification lifecycle of one submitted payment record:
// pending until an administrator verifies or rejects the proof.
type Status string

// Payment is one submitted payment record: the proof text, the method
// that produced it, the verification status, and the amount snapshot taken
// with the submission. Resubmission after a rejection inserts a new row;
// history stays append-only.
type Payment struct {
	ID         int64
	OrderID    int64
	Method     Method
	Status     Status
	Reference  string
	Amount     int64
	VerifiedBy string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// OrderRef is the writable-edge view of the order the payments module may
// touch: gate and act, never mutate. The snapshot amount is the order
// total at the read, taken as the proof's expected value; the number is
// the derived display value (ORD-000042); the currency rides for display
// formatting. The tracking token rides for the gateway finish redirect,
// which lands the buyer back on their own tracking view.
type OrderRef struct {
	ID            int64
	OrderNumber   string
	Status        string
	TotalMinor    int64
	CustomerEmail string
	Currency      string
	TrackingToken string
}

// PaymentDetail is one payment record with its order context: the number
// an admin reads, the order status the decision depends on, and the
// payable total the proof amount answers to. A stale proof — recorded
// before shipping was set — shows its amount against the current total
// so verification never flies blind.
type PaymentDetail struct {
	Payment     Payment
	OrderNumber string
	OrderStatus string
	OrderTotal  int64
}

// ParseMethod resolves a wire value onto the method vocabulary.
func ParseMethod(raw string) (Method, bool) {
	switch Method(raw) {
	case MethodManual, MethodMidtrans:
		return Method(raw), true
	default:
		return "", false
	}
}

// ParseStatus resolves a wire value onto the lifecycle vocabulary.
func ParseStatus(raw string) (Status, bool) {
	switch Status(raw) {
	case StatusPending, StatusVerified, StatusRejected:
		return Status(raw), true
	default:
		return "", false
	}
}
