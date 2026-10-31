package payments_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	domain "github.com/aristorinjuang/lesstruct/modules/payments/domain"
	"github.com/aristorinjuang/lesstruct/modules/payments/domain/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// errStoreFailure is the ad-hoc store error shared between a table row's
// setup and its wantErr, so errors.Is matches instances.
var errStoreFailure = errors.New("db error")

// payableOrder is the pending order a guest submits against.
func payableOrder() *domain.OrderRef {
	return &domain.OrderRef{
		ID:            42,
		OrderNumber:   "ORD-000042",
		Status:        "pending",
		TotalMinor:    522500,
		CustomerEmail: "shopper@example.com",
		Currency:      "IDR",
	}
}

// submittedPayment is the shape the store echoes back after an insert.
func submittedPayment() *domain.Payment {
	return &domain.Payment{
		ID:        7,
		OrderID:   42,
		Method:    domain.MethodManual,
		Status:    domain.StatusPending,
		Reference: "BR-12345",
		Amount:    522500,
	}
}

func newSeams(t *testing.T) (store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
	t.Helper()
	return mocks.NewMockPaymentsStore(t), mocks.NewMockOrders(t), mocks.NewMockMailer(t)
}

func newService(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer, methods ...domain.Method) *domain.Service {
	if methods == nil {
		methods = []domain.Method{domain.MethodManual}
	}
	return domain.NewPaymentsService(store, orders, mail, nil, methods)
}

func TestService_Methods(t *testing.T) {
	t.Run("returns the adopted methods", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		service := newService(store, orders, mail, domain.MethodManual, domain.MethodMidtrans)

		assert.Equal(t, []domain.Method{domain.MethodManual, domain.MethodMidtrans}, service.Methods())
	})
}

func TestParseMethod(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		want  domain.Method
		valid bool
	}{
		{
			name:  "the manual transfer method resolves",
			raw:   "manual",
			want:  domain.MethodManual,
			valid: true,
		},
		{
			name:  "unknown values resolve to nothing",
			raw:   "cod",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, ok := domain.ParseMethod(tt.raw)
			assert.Equal(t, tt.valid, ok)
			if tt.valid {
				assert.Equal(t, tt.want, method)
			}
		})
	}
}

func TestParseStatus(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		want  domain.Status
		valid bool
	}{
		{
			name:  "pending resolves",
			raw:   "pending",
			want:  domain.StatusPending,
			valid: true,
		},
		{
			name:  "verified resolves",
			raw:   "verified",
			want:  domain.StatusVerified,
			valid: true,
		},
		{
			name:  "rejected resolves",
			raw:   "rejected",
			want:  domain.StatusRejected,
			valid: true,
		},
		{
			name:  "unknown values resolve to nothing",
			raw:   "refunded",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, ok := domain.ParseStatus(tt.raw)
			assert.Equal(t, tt.valid, ok)
			if tt.valid {
				assert.Equal(t, tt.want, status)
			}
		})
	}
}

func TestSentinelMessages(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		message string
	}{
		{
			name:    "payment not found renders",
			err:     domain.ErrPaymentNotFound,
			message: "payment not found",
		},
		{
			name:    "order not found renders",
			err:     domain.ErrOrderNotFound,
			message: "order not found",
		},
		{
			name:    "order not payable renders",
			err:     domain.ErrOrderNotPayable,
			message: "order is not payable",
		},
		{
			name:    "insufficient stock renders",
			err:     domain.ErrInsufficientStock,
			message: "insufficient stock for the order",
		},
		{
			name:    "invalid reference renders",
			err:     domain.ErrInvalidReference,
			message: "reference must be between 1 and 500 characters",
		},
		{
			name:    "invalid method renders",
			err:     domain.ErrInvalidMethod,
			message: "payment method is not offered",
		},
		{
			name:    "payment not pending renders",
			err:     domain.ErrPaymentNotPending,
			message: "payment is not pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.message, tt.err.Error())
		})
	}
}

// trackingToken is the per-order secret every guest gate expectation
// presents. The value is arbitrary — the service only normalizes and
// forwards it.
const trackingToken = "track-token"

func TestService_Submit(t *testing.T) {
	tests := []struct {
		name      string
		number    string
		email     string
		method    domain.Method
		reference string
		setup     func(*mocks.MockPaymentsStore, *mocks.MockOrders, *mocks.MockMailer)
		wantErr   error
		want      *domain.Payment
	}{
		{
			name:      "records the proof verbatim and snapshots the amount",
			number:    "ORD-000042",
			email:     "shopper@example.com",
			method:    domain.MethodManual,
			reference: "BR-12345",
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				orders.On("Details", mock.Anything, "ORD-000042", "shopper@example.com", trackingToken).Return(payableOrder(), nil)
				store.On("Insert", mock.Anything, mock.MatchedBy(func(p *domain.Payment) bool {
					return p.OrderID == 42 && p.Amount == 522500 && p.Status == domain.StatusPending
				})).Return(int64(7), nil)
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
				mail.On("SendPaymentReceivedEmail", mock.Anything, payableOrder(), domain.MethodManual, "BR-12345").Return(nil)
			},
			want: submittedPayment(),
		},
		{
			name:   "normalizes the guest email through the gate",
			number: "ORD-000042",
			email:  " SHOPPER@Example.com ",
			method: domain.MethodManual,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				orders.On("Details", mock.Anything, "ORD-000042", "shopper@example.com", trackingToken).Return(payableOrder(), nil)
				store.On("Insert", mock.Anything, mock.Anything).Return(int64(7), nil)
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
				mail.On("SendPaymentReceivedEmail", mock.Anything, mock.Anything, domain.MethodManual, mock.Anything).Return(nil)
			},
			reference: "BR-12345",
			want:      submittedPayment(),
		},
		{
			name:      "trims the proof text",
			number:    "ORD-000042",
			email:     "shopper@example.com",
			method:    domain.MethodManual,
			reference: "  BR-12345  ",
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				orders.On("Details", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(payableOrder(), nil)
				store.On("Insert", mock.Anything, mock.MatchedBy(func(p *domain.Payment) bool {
					return p.Reference == "BR-12345"
				})).Return(int64(7), nil)
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
				mail.On("SendPaymentReceivedEmail", mock.Anything, mock.Anything, mock.Anything, "BR-12345").Return(nil)
			},
			want: submittedPayment(),
		},
		{
			name:      "a blank reference refuses validation",
			number:    "ORD-000042",
			email:     "shopper@example.com",
			method:    domain.MethodManual,
			reference: "   ",
			setup:     func(*mocks.MockPaymentsStore, *mocks.MockOrders, *mocks.MockMailer) {},
			wantErr:   domain.ErrInvalidReference,
		},
		{
			name:      "an over-long reference refuses validation",
			number:    "ORD-000042",
			email:     "shopper@example.com",
			method:    domain.MethodManual,
			reference: strings.Repeat("x", domain.MaxReferenceRunes+1),
			setup:     func(*mocks.MockPaymentsStore, *mocks.MockOrders, *mocks.MockMailer) {},
			wantErr:   domain.ErrInvalidReference,
		},
		{
			name:      "a 500-rune multibyte reference passes",
			number:    "ORD-000042",
			email:     "shopper@example.com",
			method:    domain.MethodManual,
			reference: strings.Repeat("é", domain.MaxReferenceRunes),
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				orders.On("Details", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(payableOrder(), nil)
				store.On("Insert", mock.Anything, mock.Anything).Return(int64(7), nil)
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
				mail.On("SendPaymentReceivedEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			want: submittedPayment(),
		},
		{
			name:      "an unoffered method refuses validation",
			number:    "ORD-000042",
			email:     "shopper@example.com",
			method:    domain.Method("cod"),
			reference: "BR-12345",
			setup:     func(*mocks.MockPaymentsStore, *mocks.MockOrders, *mocks.MockMailer) {},
			wantErr:   domain.ErrInvalidMethod,
		},
		{
			name:      "an unresolvable order passes through",
			number:    "ORD-000042",
			email:     "shopper@example.com",
			method:    domain.MethodManual,
			reference: "BR-12345",
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				orders.On("Details", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, domain.ErrOrderNotFound)
			},
			wantErr: domain.ErrOrderNotFound,
		},
		{
			name:      "settled orders refuse proofs",
			number:    "ORD-000042",
			email:     "shopper@example.com",
			method:    domain.MethodManual,
			reference: "BR-12345",
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				settled := payableOrder()
				settled.Status = "paid"
				orders.On("Details", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(settled, nil)
			},
			wantErr: domain.ErrOrderNotPayable,
		},
		{
			name:      "insert failure passes through",
			number:    "ORD-000042",
			email:     "shopper@example.com",
			method:    domain.MethodManual,
			reference: "BR-12345",
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				orders.On("Details", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(payableOrder(), nil)
				store.On("Insert", mock.Anything, mock.Anything).Return(int64(0), errStoreFailure)
			},
			wantErr: errStoreFailure,
		},
		{
			name:      "re-read failure after insert passes through",
			number:    "ORD-000042",
			email:     "shopper@example.com",
			method:    domain.MethodManual,
			reference: "BR-12345",
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				orders.On("Details", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(payableOrder(), nil)
				store.On("Insert", mock.Anything, mock.Anything).Return(int64(7), nil)
				store.On("FindByID", mock.Anything, int64(7)).Return(nil, errStoreFailure)
			},
			wantErr: errStoreFailure,
		},
		{
			name:      "email failures never fail the submission",
			number:    "ORD-000042",
			email:     "shopper@example.com",
			method:    domain.MethodManual,
			reference: "BR-12345",
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				orders.On("Details", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(payableOrder(), nil)
				store.On("Insert", mock.Anything, mock.Anything).Return(int64(7), nil)
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
				mail.On("SendPaymentReceivedEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("smtp down"))
			},
			want: submittedPayment(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, orders, mail := newSeams(t)
			tt.setup(store, orders, mail)
			service := newService(store, orders, mail)

			payment, err := service.Submit(context.Background(), tt.number, tt.email, trackingToken, tt.method, tt.reference)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "unexpected error: %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, payment)
		})
	}
}

func TestService_Verify(t *testing.T) {
	tests := []struct {
		name       string
		id         int64
		verifiedBy string
		setup      func(*mocks.MockPaymentsStore, *mocks.MockOrders, *mocks.MockMailer)
		wantErr    error
		want       *domain.Payment
	}{
		{
			name:       "confirms the order and marks the record verified",
			id:         7,
			verifiedBy: "admin",
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil).Once()
				orders.On("DetailsByID", mock.Anything, int64(42)).Return(payableOrder(), nil)
				orders.On("Confirm", mock.Anything, int64(42)).Return(payableOrder(), nil)
				store.On("SetStatus", mock.Anything, int64(7), domain.StatusVerified, "admin").Return(nil)
				verified := submittedPayment()
				verified.Status = domain.StatusVerified
				verified.VerifiedBy = "admin"
				store.On("FindByID", mock.Anything, int64(7)).Return(verified, nil).Once()
			},
			want: func() *domain.Payment {
				verified := submittedPayment()
				verified.Status = domain.StatusVerified
				verified.VerifiedBy = "admin"
				return verified
			}(),
		},
		{
			name: "an unknown payment passes through",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(nil, domain.ErrPaymentNotFound)
			},
			wantErr: domain.ErrPaymentNotFound,
		},
		{
			name: "decided records refuse to reopen",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				verified := submittedPayment()
				verified.Status = domain.StatusRejected
				verified.VerifiedBy = "other"
				store.On("FindByID", mock.Anything, int64(7)).Return(verified, nil)
			},
			wantErr: domain.ErrPaymentNotPending,
		},
		{
			name: "a gateway-paid order refuses a second verified record",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
				paid := payableOrder()
				paid.Status = "paid"
				orders.On("DetailsByID", mock.Anything, int64(42)).Return(paid, nil)
			},
			wantErr: domain.ErrOrderNotPayable,
		},
		{
			name: "an unresolvable order passes through",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
				orders.On("DetailsByID", mock.Anything, int64(42)).Return(nil, domain.ErrOrderNotFound)
			},
			wantErr: domain.ErrOrderNotFound,
		},
		{
			name: "an unpayable order refuses confirmation",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
				orders.On("DetailsByID", mock.Anything, int64(42)).Return(payableOrder(), nil)
				orders.On("Confirm", mock.Anything, int64(42)).Return(nil, domain.ErrOrderNotPayable)
			},
			wantErr: domain.ErrOrderNotPayable,
		},
		{
			name: "a stock shortfall refuses while the order waits",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
				orders.On("DetailsByID", mock.Anything, int64(42)).Return(payableOrder(), nil)
				orders.On("Confirm", mock.Anything, int64(42)).Return(nil, domain.ErrInsufficientStock)
			},
			wantErr: domain.ErrInsufficientStock,
		},
		{
			name: "an unknown order passes through",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
				orders.On("DetailsByID", mock.Anything, int64(42)).Return(payableOrder(), nil)
				orders.On("Confirm", mock.Anything, int64(42)).Return(nil, domain.ErrOrderNotFound)
			},
			wantErr: domain.ErrOrderNotFound,
		},
		{
			name: "a status write failure passes through",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
				orders.On("DetailsByID", mock.Anything, int64(42)).Return(payableOrder(), nil)
				orders.On("Confirm", mock.Anything, int64(42)).Return(payableOrder(), nil)
				store.On("SetStatus", mock.Anything, int64(7), domain.StatusVerified, "admin").Return(errStoreFailure)
			},
			verifiedBy: "admin",
			wantErr:    errStoreFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, orders, mail := newSeams(t)
			tt.setup(store, orders, mail)
			service := newService(store, orders, mail)

			payment, err := service.Verify(context.Background(), tt.id, tt.verifiedBy)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "unexpected error: %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, payment)
		})
	}
}

func TestService_Reject(t *testing.T) {
	tests := []struct {
		name    string
		id      int64
		setup   func(*mocks.MockPaymentsStore, *mocks.MockOrders, *mocks.MockMailer)
		wantErr error
		want    *domain.Payment
	}{
		{
			name: "keeps the order open and marks the record rejected",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil).Once()
				orders.On("DetailsByID", mock.Anything, int64(42)).Return(payableOrder(), nil)
				store.On("SetStatus", mock.Anything, int64(7), domain.StatusRejected, "").Return(nil)
				rejected := submittedPayment()
				rejected.Status = domain.StatusRejected
				store.On("FindByID", mock.Anything, int64(7)).Return(rejected, nil).Once()
				mail.On("SendPaymentRejectedEmail", mock.Anything, payableOrder(), domain.MethodManual, "BR-12345").Return(nil)
			},
			want: func() *domain.Payment {
				rejected := submittedPayment()
				rejected.Status = domain.StatusRejected
				return rejected
			}(),
		},
		{
			name: "an unknown payment passes through",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(nil, domain.ErrPaymentNotFound)
			},
			wantErr: domain.ErrPaymentNotFound,
		},
		{
			name: "decided records refuse to reopen",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				verified := submittedPayment()
				verified.Status = domain.StatusVerified
				verified.VerifiedBy = "admin"
				store.On("FindByID", mock.Anything, int64(7)).Return(verified, nil)
			},
			wantErr: domain.ErrPaymentNotPending,
		},
		{
			name: "a gateway-paid order refuses the rejection email",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
				paid := payableOrder()
				paid.Status = "paid"
				orders.On("DetailsByID", mock.Anything, int64(42)).Return(paid, nil)
			},
			wantErr: domain.ErrOrderNotPayable,
		},
		{
			name: "an unresolved order passes through",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
				orders.On("DetailsByID", mock.Anything, int64(42)).Return(nil, domain.ErrOrderNotFound)
			},
			wantErr: domain.ErrOrderNotFound,
		},
		{
			name: "a status write failure passes through",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
				orders.On("DetailsByID", mock.Anything, int64(42)).Return(payableOrder(), nil)
				store.On("SetStatus", mock.Anything, int64(7), domain.StatusRejected, "").Return(errStoreFailure)
			},
			wantErr: errStoreFailure,
		},
		{
			name: "re-read failure after the decision passes through",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil).Once()
				orders.On("DetailsByID", mock.Anything, int64(42)).Return(payableOrder(), nil)
				store.On("SetStatus", mock.Anything, int64(7), domain.StatusRejected, "").Return(nil)
				store.On("FindByID", mock.Anything, int64(7)).Return(nil, errStoreFailure).Once()
			},
			wantErr: errStoreFailure,
		},
		{
			name: "email failures never fail the rejection",
			id:   7,
			setup: func(store *mocks.MockPaymentsStore, orders *mocks.MockOrders, mail *mocks.MockMailer) {
				store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil).Once()
				orders.On("DetailsByID", mock.Anything, int64(42)).Return(payableOrder(), nil)
				store.On("SetStatus", mock.Anything, int64(7), domain.StatusRejected, "").Return(nil)
				rejected := submittedPayment()
				rejected.Status = domain.StatusRejected
				store.On("FindByID", mock.Anything, int64(7)).Return(rejected, nil).Once()
				mail.On("SendPaymentRejectedEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("smtp down"))
			},
			want: func() *domain.Payment {
				rejected := submittedPayment()
				rejected.Status = domain.StatusRejected
				return rejected
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, orders, mail := newSeams(t)
			tt.setup(store, orders, mail)
			service := newService(store, orders, mail)

			payment, err := service.Reject(context.Background(), tt.id)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "unexpected error: %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, payment)
		})
	}
}

func TestService_Mint(t *testing.T) {
	gatewaySession := &domain.GatewaySession{
		Token:       "snap-token",
		RedirectURL: "https://app.sandbox.midtrans.com/snap/v2/vtwidget",
	}

	t.Run("a store without a gateway refuses", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		service := newService(store, orders, mail)

		_, err := service.Mint(context.Background(), "ORD-000042", "shopper@example.com", trackingToken)

		assert.ErrorIs(t, err, domain.ErrGatewayUnavailable)
	})

	t.Run("mints a session for a payable order", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		gateway := mocks.NewMockGateway(t)
		gateway.On("Mint", mock.Anything, payableOrder()).Return(gatewaySession, nil)
		orders.On("Details", mock.Anything, "ORD-000042", "shopper@example.com", trackingToken).Return(payableOrder(), nil)
		service := domain.NewPaymentsService(store, orders, mail, gateway, []domain.Method{domain.MethodManual, domain.MethodMidtrans})

		session, err := service.Mint(context.Background(), "ORD-000042", "shopper@example.com", trackingToken)

		require.NoError(t, err)
		assert.Equal(t, gatewaySession, session)
	})

	t.Run("a settled order refuses to mint", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		gateway := mocks.NewMockGateway(t)
		orders.On("Details", mock.Anything, "ORD-000042", "shopper@example.com", trackingToken).Return(nil, domain.ErrOrderNotPayable)
		service := domain.NewPaymentsService(store, orders, mail, gateway, []domain.Method{domain.MethodManual, domain.MethodMidtrans})

		_, err := service.Mint(context.Background(), "ORD-000042", "shopper@example.com", trackingToken)

		assert.ErrorIs(t, err, domain.ErrOrderNotPayable)
	})

	t.Run("a paid order refuses to mint", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		gateway := mocks.NewMockGateway(t)
		settled := payableOrder()
		settled.Status = "paid"
		orders.On("Details", mock.Anything, "ORD-000042", "shopper@example.com", trackingToken).Return(settled, nil)
		service := domain.NewPaymentsService(store, orders, mail, gateway, []domain.Method{domain.MethodManual, domain.MethodMidtrans})

		_, err := service.Mint(context.Background(), "ORD-000042", "shopper@example.com", trackingToken)

		assert.ErrorIs(t, err, domain.ErrOrderNotPayable)
	})

	t.Run("an unresolvable order passes through", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		gateway := mocks.NewMockGateway(t)
		orders.On("Details", mock.Anything, "ORD-000042", "shopper@example.com", trackingToken).Return(nil, domain.ErrOrderNotFound)
		service := domain.NewPaymentsService(store, orders, mail, gateway, []domain.Method{domain.MethodManual, domain.MethodMidtrans})

		_, err := service.Mint(context.Background(), "ORD-000042", "shopper@example.com", trackingToken)

		assert.ErrorIs(t, err, domain.ErrOrderNotFound)
	})
}

func TestService_HandleNotification(t *testing.T) {
	settled := func() *domain.Notification {
		return &domain.Notification{
			OrderNumber:   "ORD-000042",
			TransactionID: "T-1",
			Status:        "settlement",
			GrossMinor:    522500,
		}
	}

	t.Run("a settlement confirms the order and records the transaction", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("FindByReference", mock.Anything, "T-1").Return(nil, domain.ErrPaymentNotFound)
		orders.On("DetailsByNumber", mock.Anything, "ORD-000042").Return(payableOrder(), nil)
		orders.On("Confirm", mock.Anything, int64(42)).Return(payableOrder(), nil)
		store.On("Insert", mock.Anything, mock.MatchedBy(func(p *domain.Payment) bool {
			return p.OrderID == 42 && p.Method == domain.MethodMidtrans &&
				p.Status == domain.StatusVerified && p.Amount == 522500 && p.VerifiedBy == "midtrans"
		})).Return(int64(8), nil)
		service := newService(store, orders, mail)

		require.NoError(t, service.HandleNotification(context.Background(), settled()))
	})

	t.Run("a settled reference converges retries", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("FindByReference", mock.Anything, "T-1").Return(submittedPayment(), nil)
		service := newService(store, orders, mail)

		require.NoError(t, service.HandleNotification(context.Background(), settled()))
	})

	t.Run("a settlement that disagrees with the order refuses", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		event := settled()
		event.GrossMinor = 12345
		store.On("FindByReference", mock.Anything, "T-1").Return(nil, domain.ErrPaymentNotFound)
		orders.On("DetailsByNumber", mock.Anything, "ORD-000042").Return(payableOrder(), nil)
		service := newService(store, orders, mail)

		err := service.HandleNotification(context.Background(), event)

		assert.ErrorIs(t, err, domain.ErrAmountMismatch)
	})

	t.Run("a stock shortfall refuses without recording", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("FindByReference", mock.Anything, "T-1").Return(nil, domain.ErrPaymentNotFound)
		orders.On("DetailsByNumber", mock.Anything, "ORD-000042").Return(payableOrder(), nil)
		orders.On("Confirm", mock.Anything, int64(42)).Return(nil, domain.ErrInsufficientStock)
		service := newService(store, orders, mail)

		err := service.HandleNotification(context.Background(), settled())

		assert.ErrorIs(t, err, domain.ErrInsufficientStock)
	})

	t.Run("a settled order keeps the flip idempotent", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("FindByReference", mock.Anything, "T-1").Return(nil, domain.ErrPaymentNotFound)
		orders.On("DetailsByNumber", mock.Anything, "ORD-000042").Return(payableOrder(), nil)
		orders.On("Confirm", mock.Anything, int64(42)).Return(nil, domain.ErrOrderNotPayable)
		service := newService(store, orders, mail)

		err := service.HandleNotification(context.Background(), settled())

		assert.ErrorIs(t, err, domain.ErrOrderNotPayable)
	})

	t.Run("an unknown order passes through", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("FindByReference", mock.Anything, "T-1").Return(nil, domain.ErrPaymentNotFound)
		orders.On("DetailsByNumber", mock.Anything, "ORD-000042").Return(nil, domain.ErrOrderNotFound)
		service := newService(store, orders, mail)

		err := service.HandleNotification(context.Background(), settled())

		assert.ErrorIs(t, err, domain.ErrOrderNotFound)
	})

	t.Run("a record write failure passes through", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("FindByReference", mock.Anything, "T-1").Return(nil, domain.ErrPaymentNotFound)
		orders.On("DetailsByNumber", mock.Anything, "ORD-000042").Return(payableOrder(), nil)
		orders.On("Confirm", mock.Anything, int64(42)).Return(payableOrder(), nil)
		store.On("Insert", mock.Anything, mock.Anything).Return(int64(0), errStoreFailure)
		service := newService(store, orders, mail)

		err := service.HandleNotification(context.Background(), settled())

		assert.ErrorIs(t, err, errStoreFailure)
	})

	t.Run("a dedupe read failure refuses rather than recording twice", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("FindByReference", mock.Anything, "T-1").Return(nil, errStoreFailure)
		service := newService(store, orders, mail)

		err := service.HandleNotification(context.Background(), settled())

		assert.ErrorIs(t, err, errStoreFailure)
	})

	t.Run("a challenged capture acks and waits for the verdict", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		service := newService(store, orders, mail)

		event := settled()
		event.Status = "capture"
		event.FraudStatus = "challenge"
		require.NoError(t, service.HandleNotification(context.Background(), event))
	})

	t.Run("an accepted capture folds the settlement", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("FindByReference", mock.Anything, "T-1").Return(nil, domain.ErrPaymentNotFound)
		orders.On("DetailsByNumber", mock.Anything, "ORD-000042").Return(payableOrder(), nil)
		orders.On("Confirm", mock.Anything, int64(42)).Return(payableOrder(), nil)
		store.On("Insert", mock.Anything, mock.Anything).Return(int64(8), nil)
		service := newService(store, orders, mail)

		event := settled()
		event.Status = "capture"
		event.FraudStatus = "accept"
		require.NoError(t, service.HandleNotification(context.Background(), event))
	})

	t.Run("a refund folds the order lifecycle", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		orders.On("DetailsByNumber", mock.Anything, "ORD-000042").Return(payableOrder(), nil)
		orders.On("Refund", mock.Anything, int64(42)).Return(nil)
		service := newService(store, orders, mail)

		event := settled()
		event.Status = "refund"
		require.NoError(t, service.HandleNotification(context.Background(), event))
	})

	t.Run("an already-refunded order acknowledges the retry", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		orders.On("DetailsByNumber", mock.Anything, "ORD-000042").Return(payableOrder(), nil)
		orders.On("Refund", mock.Anything, int64(42)).Return(domain.ErrOrderNotRefundable)
		service := newService(store, orders, mail)

		event := settled()
		event.Status = "refund"
		require.NoError(t, service.HandleNotification(context.Background(), event))
	})

	t.Run("a refund of an unknown order passes through", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		orders.On("DetailsByNumber", mock.Anything, "ORD-000042").Return(nil, domain.ErrOrderNotFound)
		service := newService(store, orders, mail)

		event := settled()
		event.Status = "refund"
		err := service.HandleNotification(context.Background(), event)

		assert.ErrorIs(t, err, domain.ErrOrderNotFound)
	})

	t.Run("a refund storage failure passes through", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		orders.On("DetailsByNumber", mock.Anything, "ORD-000042").Return(payableOrder(), nil)
		orders.On("Refund", mock.Anything, int64(42)).Return(errStoreFailure)
		service := newService(store, orders, mail)

		event := settled()
		event.Status = "refund"
		err := service.HandleNotification(context.Background(), event)

		assert.ErrorIs(t, err, errStoreFailure)
	})

	t.Run("an unknown status acknowledges without action", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		service := newService(store, orders, mail)

		event := settled()
		event.Status = "expire"
		require.NoError(t, service.HandleNotification(context.Background(), event))
	})
}

func TestService_GatewayAdopted(t *testing.T) {
	t.Run("reports the gateway absence", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		service := newService(store, orders, mail)

		assert.False(t, service.GatewayAdopted())
	})

	t.Run("reports the gateway presence", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		service := domain.NewPaymentsService(store, orders, mail, mocks.NewMockGateway(t), []domain.Method{domain.MethodManual})

		assert.True(t, service.GatewayAdopted())
	})
}

func TestService_List(t *testing.T) {
	t.Run("passes the filters through", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		records := []domain.Payment{*submittedPayment()}
		store.On("List", mock.Anything, (*domain.Status)(nil), int64(0), 20).Return(records, nil)
		service := newService(store, orders, mail)

		found, err := service.List(context.Background(), nil, 0, 20)

		require.NoError(t, err)
		assert.Equal(t, records, found)
	})

	t.Run("an unusable limit takes the default", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("List", mock.Anything, (*domain.Status)(nil), int64(0), domain.ListLimitDefault).Return([]domain.Payment{}, nil)
		service := newService(store, orders, mail)

		_, err := service.List(context.Background(), nil, 0, 0)

		require.NoError(t, err)
	})

	t.Run("an oversized limit takes the ceiling", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("List", mock.Anything, (*domain.Status)(nil), int64(0), domain.ListLimitCeiling).Return([]domain.Payment{}, nil)
		service := newService(store, orders, mail)

		_, err := service.List(context.Background(), nil, 0, 500)

		require.NoError(t, err)
	})

	t.Run("store failures pass through", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("List", mock.Anything, (*domain.Status)(nil), int64(0), 20).Return(nil, errStoreFailure)
		service := newService(store, orders, mail)

		_, err := service.List(context.Background(), nil, 0, 20)

		assert.ErrorIs(t, err, errStoreFailure)
	})
}

func TestService_Detail(t *testing.T) {
	t.Run("returns the record with its order context", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
		orders.On("DetailsByID", mock.Anything, int64(42)).Return(payableOrder(), nil)
		service := newService(store, orders, mail)

		detail, err := service.Detail(context.Background(), 7)

		require.NoError(t, err)
		assert.Equal(t, &domain.PaymentDetail{
			Payment:     *submittedPayment(),
			OrderNumber: "ORD-000042",
			OrderStatus: "pending",
			OrderTotal:  522500,
		}, detail)
	})

	t.Run("unknown records pass through", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("FindByID", mock.Anything, int64(7)).Return(nil, domain.ErrPaymentNotFound)
		service := newService(store, orders, mail)

		_, err := service.Detail(context.Background(), 7)

		assert.ErrorIs(t, err, domain.ErrPaymentNotFound)
	})

	t.Run("an unresolvable order passes through", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("FindByID", mock.Anything, int64(7)).Return(submittedPayment(), nil)
		orders.On("DetailsByID", mock.Anything, int64(42)).Return(nil, domain.ErrOrderNotFound)
		service := newService(store, orders, mail)

		_, err := service.Detail(context.Background(), 7)

		assert.ErrorIs(t, err, domain.ErrOrderNotFound)
	})
}

func TestService_ListByOrder(t *testing.T) {
	t.Run("returns the order history newest first", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		records := []domain.Payment{*submittedPayment()}
		store.On("ListByOrder", mock.Anything, int64(42)).Return(records, nil)
		service := newService(store, orders, mail)

		found, err := service.ListByOrder(context.Background(), 42)

		require.NoError(t, err)
		assert.Equal(t, records, found)
	})

	t.Run("store failures pass through", func(t *testing.T) {
		store, orders, mail := newSeams(t)
		store.On("ListByOrder", mock.Anything, int64(42)).Return(nil, errStoreFailure)
		service := newService(store, orders, mail)

		_, err := service.ListByOrder(context.Background(), 42)

		assert.ErrorIs(t, err, errStoreFailure)
	})
}
