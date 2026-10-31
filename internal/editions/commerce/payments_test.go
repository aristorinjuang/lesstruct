package commerce_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/editions/commerce"
	ordersdomain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
	ordersmocks "github.com/aristorinjuang/lesstruct/modules/orders/domain/mocks"
	paymentsdomain "github.com/aristorinjuang/lesstruct/modules/payments/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ordersSeamHarness builds a live orders service over mocked seams and
// adapts it the way wirePayments does, so the tests exercise the real
// Tracking/Get/ConfirmPaid paths behind the payments Orders seam.
func ordersSeamHarness(t *testing.T) (
	paymentsdomain.Orders,
	*ordersmocks.MockOrderStore,
	*ordersmocks.MockInventory,
	*ordersmocks.MockMailer,
) {
	t.Helper()
	store := ordersmocks.NewMockOrderStore(t)
	inventory := ordersmocks.NewMockInventory(t)
	mail := ordersmocks.NewMockMailer(t)
	service := ordersdomain.NewService(
		store,
		ordersmocks.NewMockCartSource(t),
		ordersmocks.NewMockContentSource(t),
		mail,
		inventory,
		"IDR",
	)
	return commerce.NewOrderSeams(service), store, inventory, mail
}

// pendingOrdersOrder is the storage shape of a payable order in the
// orders module.
func pendingOrdersOrder() *ordersdomain.Order {
	return &ordersdomain.Order{
		ID:            42,
		Status:        ordersdomain.StatusPending,
		CustomerEmail: "shopper@example.com",
		Currency:      "IDR",
		Subtotal:      522500,
		Items: []ordersdomain.OrderItem{{
			ProductContentID: 7,
			SKU:              "TSHIRT",
			Title:            "T-Shirt",
			UnitPrice:        299000,
			Quantity:         2,
		}},
	}
}

// ordersTrackingToken is the per-order secret the payments seam tests
// present; ordersTokenHash mirrors the orders domain's at-rest hash form.
const ordersTrackingToken = "track-token"

var ordersTokenHash = ordersSha256Hex(ordersTrackingToken)

func ordersSha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func TestOrderSeams_Details(t *testing.T) {
	t.Run("maps the guest gate through tracking", func(t *testing.T) {
		seam, store, _, _ := ordersSeamHarness(t)
		order := pendingOrdersOrder()
		order.ShippingMinor = 15000
		store.On("FindByNumberAndEmailAndToken", mock.Anything, int64(42), "shopper@example.com", ordersTokenHash).Return(order, nil)

		ref, err := seam.Details(context.Background(), "ORD-000042", "shopper@example.com", ordersTrackingToken)

		require.NoError(t, err)
		assert.Equal(t, &paymentsdomain.OrderRef{
			ID:            42,
			OrderNumber:   "ORD-000042",
			Status:        "pending",
			TotalMinor:    537500,
			CustomerEmail: "shopper@example.com",
			Currency:      "IDR",
			TrackingToken: ordersTrackingToken,
		}, ref)
	})

	t.Run("an order miss maps to the payments not-found", func(t *testing.T) {
		seam, store, _, _ := ordersSeamHarness(t)
		store.On("FindByNumberAndEmailAndToken", mock.Anything, int64(42), "shopper@example.com", ordersTokenHash).
			Return(nil, ordersdomain.ErrOrderNotFound)

		_, err := seam.Details(context.Background(), "ORD-000042", "shopper@example.com", ordersTrackingToken)

		assert.ErrorIs(t, err, paymentsdomain.ErrOrderNotFound)
	})
}

func TestOrderSeams_DetailsByID(t *testing.T) {
	t.Run("maps the administrative read", func(t *testing.T) {
		seam, store, _, _ := ordersSeamHarness(t)
		order := pendingOrdersOrder()
		order.Status = ordersdomain.StatusCancelled
		store.On("FindByID", mock.Anything, int64(42)).Return(order, nil)

		ref, err := seam.DetailsByID(context.Background(), 42)

		require.NoError(t, err)
		assert.Equal(t, "ORD-000042", ref.OrderNumber)
		assert.Equal(t, "cancelled", ref.Status)
	})
}

func TestOrderSeams_DetailsByNumber(t *testing.T) {
	t.Run("parses the display number onto the order read", func(t *testing.T) {
		seam, store, _, _ := ordersSeamHarness(t)
		store.On("FindByID", mock.Anything, int64(42)).Return(pendingOrdersOrder(), nil)

		ref, err := seam.DetailsByNumber(context.Background(), "ORD-000042")

		require.NoError(t, err)
		assert.Equal(t, int64(42), ref.ID)
		assert.Equal(t, "pending", ref.Status)
	})

	t.Run("an unparsable number reads as unknown", func(t *testing.T) {
		seam, _, _, _ := ordersSeamHarness(t)

		_, err := seam.DetailsByNumber(context.Background(), "NOT-A-NUMBER")

		assert.ErrorIs(t, err, paymentsdomain.ErrOrderNotFound)
	})

	t.Run("an order miss maps to the payments not-found", func(t *testing.T) {
		seam, store, _, _ := ordersSeamHarness(t)
		store.On("FindByID", mock.Anything, int64(42)).Return(nil, ordersdomain.ErrOrderNotFound)

		_, err := seam.DetailsByNumber(context.Background(), "ORD-000042")

		assert.ErrorIs(t, err, paymentsdomain.ErrOrderNotFound)
	})
}

func TestOrderSeams_Confirm(t *testing.T) {
	ctx := context.Background()
	t.Run("confirming maps the paid flip", func(t *testing.T) {
		seam, store, inventory, mail := ordersSeamHarness(t)
		claimed := pendingOrdersOrder()
		settled := pendingOrdersOrder()
		settled.Status = ordersdomain.StatusPaid
		store.On("FindByID", mock.Anything, int64(42)).Return(claimed, nil).Once()
		inventory.On("Reserve", mock.Anything, claimed.Items).Return(nil)
		store.On("MarkPaid", mock.Anything, int64(42)).Return(true, nil)
		mail.On("SendOrderStatusEmail", mock.Anything, mock.Anything, ordersdomain.StatusPending).Return(nil)
		store.On("FindByID", mock.Anything, int64(42)).Return(settled, nil).Once()

		ref, err := seam.Confirm(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, "paid", ref.Status)
	})

	t.Run("a non-pending order reads as not payable", func(t *testing.T) {
		seam, store, _, _ := ordersSeamHarness(t)
		expired := pendingOrdersOrder()
		expired.Status = ordersdomain.StatusExpired
		store.On("FindByID", mock.Anything, int64(42)).Return(expired, nil)

		_, err := seam.Confirm(ctx, 42)

		assert.ErrorIs(t, err, paymentsdomain.ErrOrderNotPayable)
	})

	t.Run("a stock shortfall maps to the payments sentinel", func(t *testing.T) {
		seam, store, inventory, _ := ordersSeamHarness(t)
		claimed := pendingOrdersOrder()
		store.On("FindByID", mock.Anything, int64(42)).Return(claimed, nil)
		inventory.On("Reserve", mock.Anything, claimed.Items).Return(ordersdomain.ErrInsufficientStock)

		_, err := seam.Confirm(ctx, 42)

		assert.ErrorIs(t, err, paymentsdomain.ErrInsufficientStock)
	})

	t.Run("storage failures ride through untouched", func(t *testing.T) {
		seam, store, _, _ := ordersSeamHarness(t)
		storageFailure := errors.New("db down")
		store.On("FindByID", mock.Anything, int64(42)).Return(nil, storageFailure)

		_, err := seam.Confirm(ctx, 42)

		assert.ErrorIs(t, err, storageFailure)
	})
}

func TestOrderSeams_Refund(t *testing.T) {
	ctx := context.Background()

	t.Run("a paid order refunds and restocks orders-side", func(t *testing.T) {
		seam, store, inventory, mail := ordersSeamHarness(t)
		paid := pendingOrdersOrder()
		paid.Status = ordersdomain.StatusPaid
		refunded := pendingOrdersOrder()
		refunded.Status = ordersdomain.StatusRefunded
		store.On("FindByID", mock.Anything, int64(42)).Return(paid, nil).Once()
		store.On("SetStatus", mock.Anything, int64(42), ordersdomain.StatusRefunded).Return(nil)
		inventory.On("Release", mock.Anything, paid.Items).Return(nil)
		store.On("FindByID", mock.Anything, int64(42)).Return(refunded, nil).Once()
		mail.On("SendOrderStatusEmail", mock.Anything, mock.Anything, ordersdomain.StatusPaid).Return(nil)

		err := seam.Refund(ctx, 42)

		require.NoError(t, err)
	})

	t.Run("a refund of a pending order reads as not refundable", func(t *testing.T) {
		seam, store, _, _ := ordersSeamHarness(t)
		store.On("FindByID", mock.Anything, int64(42)).Return(pendingOrdersOrder(), nil)

		err := seam.Refund(ctx, 42)

		assert.ErrorIs(t, err, paymentsdomain.ErrOrderNotRefundable)
	})

	t.Run("an unknown order passes through", func(t *testing.T) {
		seam, store, _, _ := ordersSeamHarness(t)
		store.On("FindByID", mock.Anything, int64(42)).Return(nil, ordersdomain.ErrOrderNotFound)

		err := seam.Refund(ctx, 42)

		assert.ErrorIs(t, err, paymentsdomain.ErrOrderNotFound)
	})
}
