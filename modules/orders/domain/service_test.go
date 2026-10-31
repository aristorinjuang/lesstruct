package orders_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel/legal"
	domain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
	"github.com/aristorinjuang/lesstruct/modules/orders/domain/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type seamMocks struct {
	store     *mocks.MockOrderStore
	cart      *mocks.MockCartSource
	content   *mocks.MockContentSource
	mail      *mocks.MockMailer
	inventory *mocks.MockInventory
}

// trackingToken is the per-order secret the tracking tests present; its
// hash mirrors the domain's at-rest form (hex SHA-256) for expectations.
const trackingToken = "track-token"

var trackingTokenHash = sha256Hex(trackingToken)

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func newSeams(t *testing.T) seamMocks {
	return seamMocks{
		store:     mocks.NewMockOrderStore(t),
		cart:      mocks.NewMockCartSource(t),
		content:   mocks.NewMockContentSource(t),
		mail:      mocks.NewMockMailer(t),
		inventory: mocks.NewMockInventory(t),
	}
}

func newService(seams seamMocks, opts ...domain.ServiceOption) *domain.Service {
	return domain.NewService(seams.store, seams.cart, seams.content, seams.mail, seams.inventory, "IDR", opts...)
}

// checkoutInput is the valid baseline: every field the checkout requires,
// carrying stray whitespace the service must normalize.
func checkoutInput() domain.CheckoutInput {
	return domain.CheckoutInput{
		Token: "token",
		Email: " Shopper@Example.COM ",
		Name:  " Annisa Putri ",
		Phone: " +6281234567890 ",
		ShipTo: domain.Address{
			Street:     " Jl. Kemang Raya 21 ",
			City:       " Jakarta Selatan ",
			Province:   " DKI Jakarta ",
			PostalCode: " 12730 ",
		},
	}
}

func baseLine() domain.CartSnapshotLine {
	return domain.CartSnapshotLine{
		ProductContentID: 7,
		SKU:              "TSHIRT",
		Quantity:         2,
		UnitPrice:        299000,
		InStock:          true,
		Available:        true,
	}
}

func variantLine() domain.CartSnapshotLine {
	return domain.CartSnapshotLine{
		ProductContentID: 8,
		VariantID:        new(int64(3)),
		SKU:              "MUG-BLUE",
		Quantity:         1,
		UnitPrice:        150000,
		InStock:          true,
		Available:        true,
	}
}

func pendingOrder() *domain.Order {
	return &domain.Order{
		ID:            42,
		Status:        domain.StatusPending,
		CustomerEmail: "shopper@example.com",
		CustomerName:  "Annisa Putri",
		ShipTo:        checkoutInput().ShipTo,
	}
}

// errStoreFailure is the ad-hoc store error shared between a table row's
// setup and its wantErr, so errors.Is matches instances.
var errStoreFailure = errors.New("db error")

// itemedOrder is a pending order carrying one stock-tracked line.
func itemedOrder() *domain.Order {
	order := pendingOrder()
	order.Items = []domain.OrderItem{{
		ID:               1,
		OrderID:          42,
		ProductContentID: 7,
		SKU:              "TSHIRT",
		Title:            "Nainggolan T-Shirt",
		UnitPrice:        299000,
		Quantity:         2,
	}}
	return order
}

// paidOrder is the settled shape of itemedOrder.
func paidOrder() *domain.Order {
	order := itemedOrder()
	order.Status = domain.StatusPaid
	return order
}

func TestService_Checkout(t *testing.T) {
	tests := []struct {
		name    string
		input   domain.CheckoutInput
		setup   func(seamMocks)
		wantErr error
		want    *domain.Order
	}{
		{
			name:  "snapshots lines into a pending order",
			input: checkoutInput(),
			setup: func(seams seamMocks) {
				seams.cart.On("Snapshot", mock.Anything, "token").Return(domain.CartSnapshot{
					Lines: []domain.CartSnapshotLine{baseLine(), variantLine()},
				}, nil)
				seams.content.On("ProductTitle", mock.Anything, 7).Return("Nainggolan T-Shirt", true)
				seams.content.On("ProductTitle", mock.Anything, 8).Return("  ", false)
				seams.cart.On("Consume", mock.Anything, "token").Return(nil)
				seams.store.On("CreateOrder", mock.Anything, mock.AnythingOfType("*orders.Order")).Return(int64(42), nil)
				seams.mail.On("SendOrderConfirmationEmail", mock.Anything, mock.Anything).Return(nil)
			},
			want: &domain.Order{
				ID:            42,
				Status:        domain.StatusPending,
				CustomerEmail: "shopper@example.com",
				CustomerName:  "Annisa Putri",
				CustomerPhone: "+6281234567890",
				ShipTo: domain.Address{
					Street:     "Jl. Kemang Raya 21",
					City:       "Jakarta Selatan",
					Province:   "DKI Jakarta",
					PostalCode: "12730",
				},
				Currency: "IDR",
				Subtotal: 748000,
				Items: []domain.OrderItem{
					{
						ProductContentID: 7,
						SKU:              "TSHIRT",
						Title:            "Nainggolan T-Shirt",
						UnitPrice:        299000,
						Quantity:         2,
					},
					{
						ProductContentID: 8,
						VariantID:        new(int64(3)),
						SKU:              "MUG-BLUE",
						Title:            "Product",
						UnitPrice:        150000,
						Quantity:         1,
					},
				},
			},
		},
		{
			name: "malformed email is refused",
			input: func() domain.CheckoutInput {
				input := checkoutInput()
				input.Email = "not-an-email"
				return input
			}(),
			setup:   func(seamMocks) {},
			wantErr: domain.ErrInvalidEmail,
		},
		{
			name: "missing email is refused",
			input: func() domain.CheckoutInput {
				input := checkoutInput()
				input.Email = "   "
				return input
			}(),
			setup:   func(seamMocks) {},
			wantErr: domain.ErrInvalidEmail,
		},
		{
			name: "missing name is refused",
			input: func() domain.CheckoutInput {
				input := checkoutInput()
				input.Name = "   "
				return input
			}(),
			setup:   func(seamMocks) {},
			wantErr: domain.ErrInvalidName,
		},
		{
			name: "incomplete address is refused",
			input: func() domain.CheckoutInput {
				input := checkoutInput()
				input.ShipTo.PostalCode = "  "
				return input
			}(),
			setup:   func(seamMocks) {},
			wantErr: domain.ErrInvalidAddress,
		},
		{
			name:  "unknown cart is refused",
			input: checkoutInput(),
			setup: func(seams seamMocks) {
				seams.cart.On("Snapshot", mock.Anything, "token").Return(domain.CartSnapshot{}, domain.ErrCartGone)
			},
			wantErr: domain.ErrCartGone,
		},
		{
			name:  "cart seam failure passes through",
			input: checkoutInput(),
			setup: func(seams seamMocks) {
				seams.cart.On("Snapshot", mock.Anything, "token").Return(domain.CartSnapshot{}, errors.New("db error"))
			},
			wantErr: errors.New("db error"),
		},
		{
			name:  "empty cart is refused",
			input: checkoutInput(),
			setup: func(seams seamMocks) {
				seams.cart.On("Snapshot", mock.Anything, "token").Return(domain.CartSnapshot{}, nil)
			},
			wantErr: domain.ErrEmptyCart,
		},
		{
			name:  "unavailable line is refused",
			input: checkoutInput(),
			setup: func(seams seamMocks) {
				line := baseLine()
				line.Available = false
				seams.cart.On("Snapshot", mock.Anything, "token").Return(domain.CartSnapshot{
					Lines: []domain.CartSnapshotLine{line},
				}, nil)
			},
			wantErr: domain.ErrLineUnavailable,
		},
		{
			name:  "out-of-stock line is refused",
			input: checkoutInput(),
			setup: func(seams seamMocks) {
				line := baseLine()
				line.InStock = false
				seams.cart.On("Snapshot", mock.Anything, "token").Return(domain.CartSnapshot{
					Lines: []domain.CartSnapshotLine{line},
				}, nil)
			},
			wantErr: domain.ErrOutOfStock,
		},
		{
			name:  "store failure passes through",
			input: checkoutInput(),
			setup: func(seams seamMocks) {
				seams.cart.On("Snapshot", mock.Anything, "token").Return(domain.CartSnapshot{
					Lines: []domain.CartSnapshotLine{baseLine()},
				}, nil)
				seams.content.On("ProductTitle", mock.Anything, 7).Return("Nainggolan T-Shirt", true)
				seams.cart.On("Consume", mock.Anything, "token").Return(nil)
				seams.store.On("CreateOrder", mock.Anything, mock.AnythingOfType("*orders.Order")).Return(int64(0), errors.New("db error"))
			},
			wantErr: errors.New("failed to create order"),
		},
		{
			name:  "spent cart aborts before any order is born",
			input: checkoutInput(),
			setup: func(seams seamMocks) {
				seams.cart.On("Snapshot", mock.Anything, "token").Return(domain.CartSnapshot{
					Lines: []domain.CartSnapshotLine{baseLine()},
				}, nil)
				seams.cart.On("Consume", mock.Anything, "token").Return(domain.ErrCartGone)
			},
			wantErr: domain.ErrCartGone,
		},
		{
			name:  "cart claim failure aborts the checkout",
			input: checkoutInput(),
			setup: func(seams seamMocks) {
				seams.cart.On("Snapshot", mock.Anything, "token").Return(domain.CartSnapshot{
					Lines: []domain.CartSnapshotLine{baseLine()},
				}, nil)
				seams.cart.On("Consume", mock.Anything, "token").Return(errors.New("delete blew up"))
			},
			wantErr: errors.New("delete blew up"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seams := newSeams(t)
			tt.setup(seams)
			service := newService(seams)

			order, err := service.Checkout(context.Background(), tt.input)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr) || strings.Contains(err.Error(), tt.wantErr.Error()), "unexpected error: %v", err)
				return
			}
			require.NoError(t, err)
			// The tracking secret is minted fresh per checkout, so the
			// fixture comparison excludes it — then both halves are pinned
			// non-empty: the raw token must reach the response/email and
			// the hash must reach the store row.
			expected := *tt.want
			expected.TrackingToken = order.TrackingToken
			expected.TrackingTokenHash = order.TrackingTokenHash
			assert.Equal(t, &expected, order)
			assert.NotEmpty(t, order.TrackingToken, "raw tracking token")
			assert.NotEmpty(t, order.TrackingTokenHash, "tracking token hash")
		})
	}
}

func TestService_Checkout_Consent(t *testing.T) {
	terms := legal.Document{Page: legal.PageTermsOfService, PageID: 3, Slug: "terms-of-service", Language: "en"}
	privacy := legal.Document{Page: legal.PagePrivacyPolicy, PageID: 4, Slug: "privacy-policy", Language: "en"}

	tests := []struct {
		name        string
		policy      domain.ConsentPolicy
		accepted    bool
		setup       func(seamMocks)
		wantErr     error
		wantConsent bool
	}{
		{
			name:     "required consent refuses an unchecked checkout before touching the cart",
			policy:   domain.ConsentPolicy{Required: true, Terms: terms, Privacy: privacy},
			accepted: false,
			setup:    func(seamMocks) {},
			wantErr:  domain.ErrConsentRequired,
		},
		{
			name:     "required consent stores the document evidence",
			policy:   domain.ConsentPolicy{Required: true, Terms: terms, Privacy: privacy},
			accepted: true,
			setup: func(seams seamMocks) {
				seams.cart.On("Snapshot", mock.Anything, "token").Return(domain.CartSnapshot{
					Lines: []domain.CartSnapshotLine{baseLine()},
				}, nil)
				seams.content.On("ProductTitle", mock.Anything, 7).Return("Nainggolan T-Shirt", true)
				seams.cart.On("Consume", mock.Anything, "token").Return(nil)
				seams.store.On("CreateOrder", mock.Anything, mock.AnythingOfType("*orders.Order")).Return(int64(42), nil)
				seams.mail.On("SendOrderConfirmationEmail", mock.Anything, mock.Anything).Return(nil)
			},
			wantConsent: true,
		},
		{
			name:     "a disabled gate ignores the flag and stores no evidence",
			policy:   domain.ConsentPolicy{},
			accepted: false,
			setup: func(seams seamMocks) {
				seams.cart.On("Snapshot", mock.Anything, "token").Return(domain.CartSnapshot{
					Lines: []domain.CartSnapshotLine{baseLine()},
				}, nil)
				seams.content.On("ProductTitle", mock.Anything, 7).Return("Nainggolan T-Shirt", true)
				seams.cart.On("Consume", mock.Anything, "token").Return(nil)
				seams.store.On("CreateOrder", mock.Anything, mock.AnythingOfType("*orders.Order")).Return(int64(42), nil)
				seams.mail.On("SendOrderConfirmationEmail", mock.Anything, mock.Anything).Return(nil)
			},
			wantConsent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seams := newSeams(t)
			tt.setup(seams)
			service := newService(seams, domain.WithConsentPolicy(tt.policy))
			input := checkoutInput()
			input.ConsentAccepted = tt.accepted

			order, err := service.Checkout(context.Background(), input)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			if !tt.wantConsent {
				assert.Nil(t, order.Consent)
				return
			}
			require.NotNil(t, order.Consent)
			assert.Equal(t, terms, order.Consent.Terms)
			assert.Equal(t, privacy, order.Consent.Privacy)
			assert.False(t, order.Consent.AcceptedAt.IsZero())
		})
	}
}

func TestService_ConfirmPaid(t *testing.T) {
	tests := []struct {
		name    string
		id      int64
		setup   func(seamMocks)
		wantErr error
		want    *domain.Order
	}{
		{
			name: "claims stock, flips the order, and emails the customer",
			id:   42,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(itemedOrder(), nil).Once()
				seams.inventory.On("Reserve", mock.Anything, itemedOrder().Items).Return(nil)
				seams.store.On("MarkPaid", mock.Anything, int64(42)).Return(true, nil)
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(paidOrder(), nil).Once()
				seams.mail.On("SendOrderStatusEmail", mock.Anything, paidOrder(), domain.StatusPending).Return(nil)
			},
			want: paidOrder(),
		},
		{
			name: "already settled orders read steady",
			id:   42,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(paidOrder(), nil).Once()
			},
			want: paidOrder(),
		},
		{
			name: "unknown order passes through",
			id:   999,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(999)).Return(nil, domain.ErrOrderNotFound)
			},
			wantErr: domain.ErrOrderNotFound,
		},
		{
			name: "non-pending orders refuse confirmation",
			id:   42,
			setup: func(seams seamMocks) {
				shipped := itemedOrder()
				shipped.Status = domain.StatusShipped
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(shipped, nil).Once()
			},
			wantErr: domain.ErrInvalidTransition,
		},
		{
			name: "a stock shortfall leaves the order pending",
			id:   42,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(itemedOrder(), nil).Once()
				seams.inventory.On("Reserve", mock.Anything, itemedOrder().Items).Return(domain.ErrInsufficientStock)
			},
			wantErr: domain.ErrInsufficientStock,
		},
		{
			name: "reserve failure passes through",
			id:   42,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(itemedOrder(), nil).Once()
				seams.inventory.On("Reserve", mock.Anything, itemedOrder().Items).Return(errors.New("db error"))
			},
			wantErr: errors.New("failed to reserve stock"),
		},
		{
			name: "a lost flip unwinds the claim and reads the winner",
			id:   42,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(itemedOrder(), nil).Once()
				seams.inventory.On("Reserve", mock.Anything, itemedOrder().Items).Return(nil)
				seams.store.On("MarkPaid", mock.Anything, int64(42)).Return(false, nil)
				seams.inventory.On("Release", mock.Anything, itemedOrder().Items).Return(nil)
				// The loser re-reads the winner's row without re-emailing.
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(paidOrder(), nil).Once()
			},
			want: paidOrder(),
		},
		{
			name: "storage failure after the claim releases it",
			id:   42,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(itemedOrder(), nil).Once()
				seams.inventory.On("Reserve", mock.Anything, itemedOrder().Items).Return(nil)
				seams.store.On("MarkPaid", mock.Anything, int64(42)).Return(false, errStoreFailure)
				seams.inventory.On("Release", mock.Anything, itemedOrder().Items).Return(nil)
			},
			wantErr: errors.New("failed to mark order paid"),
		},
		{
			name: "post-flip read failure passes through",
			id:   42,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(itemedOrder(), nil).Once()
				seams.inventory.On("Reserve", mock.Anything, itemedOrder().Items).Return(nil)
				seams.store.On("MarkPaid", mock.Anything, int64(42)).Return(true, nil)
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(nil, errStoreFailure).Once()
			},
			wantErr: errStoreFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seams := newSeams(t)
			tt.setup(seams)
			service := newService(seams)

			order, err := service.ConfirmPaid(context.Background(), tt.id)

			if tt.wantErr != nil {
				assert.True(t, errors.Is(err, tt.wantErr) || strings.Contains(err.Error(), tt.wantErr.Error()), "unexpected error: %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, order)
		})
	}
}

func TestService_ExpireStale(t *testing.T) {
	sweepBefore := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		setup   func(seamMocks)
		wantErr error
		want    []int64
	}{
		{
			name: "flipped orders email their customers",
			setup: func(seams seamMocks) {
				seams.store.On("ExpirePending", mock.Anything, sweepBefore, 100).Return([]int64{42}, nil)
				expired := pendingOrder()
				expired.Status = domain.StatusExpired
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(expired, nil).Once()
				seams.mail.On("SendOrderStatusEmail", mock.Anything, expired, domain.StatusPending).Return(nil)
			},
			want: []int64{42},
		},
		{
			name: "no stale orders is an empty sweep",
			setup: func(seams seamMocks) {
				seams.store.On("ExpirePending", mock.Anything, sweepBefore, 100).Return([]int64{}, nil)
			},
			want: []int64{},
		},
		{
			name: "a read miss skips the email without failing the sweep",
			setup: func(seams seamMocks) {
				seams.store.On("ExpirePending", mock.Anything, sweepBefore, 100).Return([]int64{42}, nil)
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(nil, domain.ErrOrderNotFound).Once()
			},
			want: []int64{42},
		},
		{
			name: "store failure passes through",
			setup: func(seams seamMocks) {
				seams.store.On("ExpirePending", mock.Anything, sweepBefore, 100).Return(nil, errStoreFailure)
			},
			wantErr: errors.New("failed to expire stale orders"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seams := newSeams(t)
			tt.setup(seams)
			service := newService(seams)

			ids, err := service.ExpireStale(context.Background(), sweepBefore, 100)

			if tt.wantErr != nil {
				assert.True(t, errors.Is(err, tt.wantErr) || strings.Contains(err.Error(), tt.wantErr.Error()), "unexpected error: %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, ids)
		})
	}
}

func TestService_Tracking(t *testing.T) {
	tests := []struct {
		name    string
		number  string
		email   string
		token   string
		setup   func(seamMocks)
		wantErr error
		want    *domain.Order
	}{
		{
			name:   "number, email, and token match",
			number: "ORD-000042",
			email:  " Shopper@Example.COM ",
			token:  trackingToken,
			setup: func(seams seamMocks) {
				seams.store.On("FindByNumberAndEmailAndToken", mock.Anything, int64(42), "shopper@example.com", trackingTokenHash).Return(pendingOrder(), nil)
			},
			want: pendingOrder(),
		},
		{
			name:    "malformed number reads as not found",
			number:  "42",
			email:   "shopper@example.com",
			token:   trackingToken,
			setup:   func(seamMocks) {},
			wantErr: domain.ErrOrderNotFound,
		},
		{
			name:   "store miss reads as not found",
			number: "ORD-000042",
			email:  "shopper@example.com",
			token:  trackingToken,
			setup: func(seams seamMocks) {
				seams.store.On("FindByNumberAndEmailAndToken", mock.Anything, int64(42), "shopper@example.com", trackingTokenHash).Return(nil, domain.ErrOrderNotFound)
			},
			wantErr: domain.ErrOrderNotFound,
		},
		{
			name:    "a missing token is refused before any lookup",
			number:  "ORD-000042",
			email:   "shopper@example.com",
			token:   "  ",
			setup:   func(seamMocks) {},
			wantErr: domain.ErrOrderNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seams := newSeams(t)
			tt.setup(seams)
			service := newService(seams)

			order, err := service.Tracking(context.Background(), tt.number, tt.email, tt.token)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, order)
		})
	}
}

func TestService_Get(t *testing.T) {
	tests := []struct {
		name    string
		id      int64
		setup   func(seamMocks)
		wantErr error
		want    *domain.Order
	}{
		{
			name: "loads the order with its items",
			id:   42,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(pendingOrder(), nil)
			},
			want: pendingOrder(),
		},
		{
			name:    "non-positive ID reads as not found",
			id:      0,
			setup:   func(seamMocks) {},
			wantErr: domain.ErrOrderNotFound,
		},
		{
			name: "unknown ID passes through",
			id:   999,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(999)).Return(nil, domain.ErrOrderNotFound)
			},
			wantErr: domain.ErrOrderNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seams := newSeams(t)
			tt.setup(seams)
			service := newService(seams)

			order, err := service.Get(context.Background(), tt.id)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, order)
		})
	}
}

func TestService_List(t *testing.T) {
	statusFilter := domain.StatusPaid
	invalidFilter := domain.Status("superpaid")

	tests := []struct {
		name    string
		status  *domain.Status
		before  int64
		limit   int
		setup   func(seamMocks)
		wantErr error
		want    []domain.Order
	}{
		{
			name:   "unfiltered lists the newest orders at the default page size",
			status: nil,
			limit:  0,
			setup: func(seams seamMocks) {
				seams.store.On("List", mock.Anything, (*domain.Status)(nil), int64(0), domain.ListLimitDefault).Return([]domain.Order{*pendingOrder()}, nil)
			},
			want: []domain.Order{*pendingOrder()},
		},
		{
			name:   "status filter narrows the page",
			status: &statusFilter,
			limit:  25,
			setup: func(seams seamMocks) {
				seams.store.On("List", mock.Anything, &statusFilter, int64(0), 25).Return([]domain.Order{}, nil)
			},
			want: []domain.Order{},
		},
		{
			name:   "oversized pages clamp to the ceiling",
			before: 7,
			limit:  1000,
			setup: func(seams seamMocks) {
				seams.store.On("List", mock.Anything, (*domain.Status)(nil), int64(7), domain.ListLimitCeiling).Return([]domain.Order{}, nil)
			},
			want: []domain.Order{},
		},
		{
			name:    "unknown status filter is refused",
			status:  &invalidFilter,
			limit:   0,
			setup:   func(seamMocks) {},
			wantErr: domain.ErrInvalidStatus,
		},
		{
			name:  "store failure passes through",
			limit: 0,
			setup: func(seams seamMocks) {
				seams.store.On("List", mock.Anything, (*domain.Status)(nil), int64(0), domain.ListLimitDefault).Return(nil, errors.New("db error"))
			},
			wantErr: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seams := newSeams(t)
			tt.setup(seams)
			service := newService(seams)

			orders, err := service.List(context.Background(), tt.status, tt.before, tt.limit)

			if tt.wantErr != nil {
				assert.True(t, errors.Is(err, tt.wantErr) || strings.Contains(err.Error(), tt.wantErr.Error()), "unexpected error: %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, orders)
		})
	}
}

func TestService_Transition(t *testing.T) {
	tests := []struct {
		name    string
		id      int64
		to      domain.Status
		setup   func(seamMocks)
		wantErr error
		want    *domain.Order
	}{
		{
			name:    "the paid target is refused outright",
			id:      42,
			to:      domain.StatusPaid,
			setup:   func(seamMocks) {},
			wantErr: domain.ErrInvalidTransition,
		},
		{
			name: "pending moves to cancelled and emails the customer",
			id:   42,
			to:   domain.StatusCancelled,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(pendingOrder(), nil).Once()
				cancelled := pendingOrder()
				cancelled.Status = domain.StatusCancelled
				seams.store.On("SetStatus", mock.Anything, int64(42), domain.StatusCancelled).Return(nil)
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(cancelled, nil).Once()
				seams.mail.On("SendOrderStatusEmail", mock.Anything, mock.Anything, domain.StatusPending).Return(nil)
			},
			want: func() *domain.Order {
				cancelled := pendingOrder()
				cancelled.Status = domain.StatusCancelled
				return cancelled
			}(),
		},
		{
			name: "paid cancels return the reserved stock",
			id:   42,
			to:   domain.StatusCancelled,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(paidOrder(), nil).Once()
				cancelled := paidOrder()
				cancelled.Status = domain.StatusCancelled
				seams.store.On("SetStatus", mock.Anything, int64(42), domain.StatusCancelled).Return(nil)
				seams.inventory.On("Release", mock.Anything, paidOrder().Items).Return(nil)
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(cancelled, nil).Once()
				seams.mail.On("SendOrderStatusEmail", mock.Anything, mock.Anything, domain.StatusPaid).Return(nil)
			},
			want: func() *domain.Order {
				cancelled := paidOrder()
				cancelled.Status = domain.StatusCancelled
				return cancelled
			}(),
		},
		{
			name: "shipped refunds restock nothing",
			id:   42,
			to:   domain.StatusRefunded,
			setup: func(seams seamMocks) {
				shipped := itemedOrder()
				shipped.Status = domain.StatusShipped
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(shipped, nil).Once()
				refunded := itemedOrder()
				refunded.Status = domain.StatusRefunded
				seams.store.On("SetStatus", mock.Anything, int64(42), domain.StatusRefunded).Return(nil)
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(refunded, nil).Once()
				seams.mail.On("SendOrderStatusEmail", mock.Anything, mock.Anything, domain.StatusShipped).Return(nil)
			},
			want: func() *domain.Order {
				shipped := itemedOrder()
				shipped.Status = domain.StatusRefunded
				return shipped
			}(),
		},
		{
			name:    "unknown status vocabulary is refused",
			id:      42,
			to:      domain.Status("superpaid"),
			setup:   func(seamMocks) {},
			wantErr: domain.ErrInvalidStatus,
		},
		{
			name: "unknown order passes through",
			id:   999,
			to:   domain.StatusCancelled,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(999)).Return(nil, domain.ErrOrderNotFound)
			},
			wantErr: domain.ErrOrderNotFound,
		},
		{
			name: "illegal moves are refused",
			id:   42,
			to:   domain.StatusShipped,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(pendingOrder(), nil)
			},
			wantErr: domain.ErrInvalidTransition,
		},
		{
			name: "write failure passes through",
			id:   42,
			to:   domain.StatusCancelled,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(pendingOrder(), nil)
				seams.store.On("SetStatus", mock.Anything, int64(42), domain.StatusCancelled).Return(errStoreFailure)
			},
			wantErr: errors.New("failed to update order status"),
		},
		{
			name: "re-read failure passes through",
			id:   42,
			to:   domain.StatusCancelled,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(pendingOrder(), nil).Once()
				seams.store.On("SetStatus", mock.Anything, int64(42), domain.StatusCancelled).Return(nil)
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(nil, errStoreFailure).Once()
			},
			wantErr: errStoreFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seams := newSeams(t)
			tt.setup(seams)
			service := newService(seams)

			order, err := service.Transition(context.Background(), tt.id, tt.to)

			if tt.wantErr != nil {
				assert.True(t, errors.Is(err, tt.wantErr) || strings.Contains(err.Error(), tt.wantErr.Error()), "unexpected error: %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, order)
		})
	}
}

func TestService_SetShipping(t *testing.T) {
	tests := []struct {
		name    string
		id      int64
		amount  int64
		setup   func(seamMocks)
		wantErr error
		want    *domain.Order
	}{
		{
			name:   "a pending order carries the new line",
			id:     42,
			amount: 15000,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(pendingOrder(), nil).Once()
				seams.store.On("SetShipping", mock.Anything, int64(42), int64(15000)).Return(true, nil)
				charged := pendingOrder()
				charged.ShippingMinor = 15000
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(charged, nil).Once()
			},
			want: func() *domain.Order {
				charged := pendingOrder()
				charged.ShippingMinor = 15000
				return charged
			}(),
		},
		{
			name:   "a zero line clears shipping",
			id:     42,
			amount: 0,
			setup: func(seams seamMocks) {
				charged := pendingOrder()
				charged.ShippingMinor = 15000
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(charged, nil).Once()
				seams.store.On("SetShipping", mock.Anything, int64(42), int64(0)).Return(true, nil)
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(pendingOrder(), nil).Once()
			},
			want: pendingOrder(),
		},
		{
			name:    "a negative line refuses validation",
			id:      42,
			amount:  -1,
			setup:   func(seamMocks) {},
			wantErr: domain.ErrInvalidShipping,
		},
		{
			name:   "an unknown order passes through",
			id:     999,
			amount: 15000,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(999)).Return(nil, domain.ErrOrderNotFound)
			},
			wantErr: domain.ErrOrderNotFound,
		},
		{
			name:   "a settled order refuses the write",
			id:     42,
			amount: 15000,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(paidOrder(), nil)
			},
			wantErr: domain.ErrInvalidTransition,
		},
		{
			name:   "a storage failure passes through",
			id:     42,
			amount: 15000,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(pendingOrder(), nil)
				seams.store.On("SetShipping", mock.Anything, int64(42), int64(15000)).Return(false, errStoreFailure)
			},
			wantErr: errStoreFailure,
		},
		{
			name:   "a lost race reads as a refused transition",
			id:     42,
			amount: 15000,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(pendingOrder(), nil)
				seams.store.On("SetShipping", mock.Anything, int64(42), int64(15000)).Return(false, nil)
			},
			wantErr: domain.ErrInvalidTransition,
		},
		{
			name:   "a re-read failure passes through",
			id:     42,
			amount: 15000,
			setup: func(seams seamMocks) {
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(pendingOrder(), nil).Once()
				seams.store.On("SetShipping", mock.Anything, int64(42), int64(15000)).Return(true, nil)
				seams.store.On("FindByID", mock.Anything, int64(42)).Return(nil, errStoreFailure).Once()
			},
			wantErr: errStoreFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seams := newSeams(t)
			tt.setup(seams)
			service := newService(seams)

			order, err := service.SetShipping(context.Background(), tt.id, tt.amount)

			if tt.wantErr != nil {
				assert.True(t, errors.Is(err, tt.wantErr) || strings.Contains(err.Error(), tt.wantErr.Error()), "unexpected error: %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, order)
		})
	}
}

func TestService_BuildInvoice(t *testing.T) {
	settings := domain.InvoiceSettings{
		SellerName:    "Lesstruct Demo",
		SellerAddress: "Jl. Example No. 1",
		SellerTaxID:   "01.234.567.8-901.234",
		TaxRate:       12,
	}

	t.Run("derives the document with the display-only tax", func(t *testing.T) {
		seams := newSeams(t)
		charged := itemedOrder()
		charged.Currency = "IDR"
		charged.Subtotal = 598000
		charged.ShippingMinor = 15000
		seams.store.On("FindByID", mock.Anything, int64(42)).Return(charged, nil)
		service := newService(seams)

		doc, err := service.BuildInvoice(context.Background(), 42, settings)

		require.NoError(t, err)
		assert.Equal(t, "INV-000042", doc.Number)
		assert.Equal(t, "ORD-000042", doc.OrderNumber)
		assert.Equal(t, "Lesstruct Demo", doc.SellerName)
		assert.Equal(t, "01.234.567.8-901.234", doc.SellerTaxID)
		assert.Equal(t, domain.NewMoney(598000, "IDR"), doc.Subtotal)
		assert.Equal(t, domain.NewMoney(15000, "IDR"), doc.Shipping)
		assert.Equal(t, float64(12), doc.TaxRate)
		assert.Equal(t, domain.NewMoney(73560, "IDR"), doc.Tax, "twelve percent of 613000")
		assert.Equal(t, domain.NewMoney(613000, "IDR"), doc.Total, "the total never moves for tax")
		require.Len(t, doc.Lines, 1)
		assert.Equal(t, "Nainggolan T-Shirt", doc.Lines[0].Title)
		assert.Equal(t, "TSHIRT", doc.Lines[0].SKU)
		assert.Equal(t, domain.NewMoney(299000, "IDR"), doc.Lines[0].UnitPrice)
		assert.Equal(t, domain.NewMoney(598000, "IDR"), doc.Lines[0].LineTotal)
		assert.False(t, doc.IssuedAt.IsZero())
	})

	t.Run("rounds fractional tax to the rupiah", func(t *testing.T) {
		seams := newSeams(t)
		odd := itemedOrder()
		odd.Currency = "IDR"
		odd.Subtotal = 999
		seams.store.On("FindByID", mock.Anything, int64(42)).Return(odd, nil)
		service := newService(seams)

		doc, err := service.BuildInvoice(context.Background(), 42, settings)

		require.NoError(t, err)
		assert.Equal(t, domain.NewMoney(120, "IDR"), doc.Tax, "119.88 rounds to 120")
		assert.Equal(t, domain.NewMoney(999, "IDR"), doc.Total)
	})

	t.Run("a negative rate clamps to zero", func(t *testing.T) {
		seams := newSeams(t)
		order := itemedOrder()
		order.Currency = "IDR"
		seams.store.On("FindByID", mock.Anything, int64(42)).Return(order, nil)
		service := newService(seams)
		broke := settings
		broke.TaxRate = -3

		doc, err := service.BuildInvoice(context.Background(), 42, broke)

		require.NoError(t, err)
		assert.Equal(t, float64(0), doc.TaxRate)
		assert.Equal(t, domain.NewMoney(0, "IDR"), doc.Tax)
	})

	t.Run("an unknown order passes through", func(t *testing.T) {
		seams := newSeams(t)
		seams.store.On("FindByID", mock.Anything, int64(999)).Return(nil, domain.ErrOrderNotFound)
		service := newService(seams)

		_, err := service.BuildInvoice(context.Background(), 999, settings)

		assert.ErrorIs(t, err, domain.ErrOrderNotFound)
	})
}
