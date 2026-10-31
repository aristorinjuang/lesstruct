package orders_test

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	kernel "github.com/aristorinjuang/lesstruct/kernel"
	ordersmodule "github.com/aristorinjuang/lesstruct/modules/orders"
	ordersdomain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
	ordersmocks "github.com/aristorinjuang/lesstruct/modules/orders/domain/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// testModule builds a module over mocked seams; the sweeper is the only
// runtime surface under test.
func testModule(t *testing.T, store *ordersmocks.MockOrderStore, interval time.Duration) *ordersmodule.Module {
	t.Helper()
	service := ordersdomain.NewService(
		store,
		ordersmocks.NewMockCartSource(t),
		ordersmocks.NewMockContentSource(t),
		ordersmocks.NewMockMailer(t),
		ordersmocks.NewMockInventory(t),
		"IDR",
	)
	return ordersmodule.New(
		ordersmodule.WithService(service),
		ordersmodule.WithSweeper(interval, time.Hour),
	)
}

func TestModule_InvoiceSettings(t *testing.T) {
	tests := []struct {
		name      string
		initial   ordersdomain.InvoiceSettings
		update    ordersdomain.InvoiceSettings
		withSaver bool
		saverErr  error
		wantErr   bool
		wantSaved bool
		wantAfter ordersdomain.InvoiceSettings
	}{
		{
			name:      "persists through the saver and adopts the values",
			initial:   ordersdomain.InvoiceSettings{TaxRate: 12},
			update:    ordersdomain.InvoiceSettings{SellerName: "Demo", TaxRate: 0},
			withSaver: true,
			wantSaved: true,
			wantAfter: ordersdomain.InvoiceSettings{SellerName: "Demo", TaxRate: 0},
		},
		{
			name:      "without a saver the change lives in memory",
			initial:   ordersdomain.InvoiceSettings{TaxRate: 12},
			update:    ordersdomain.InvoiceSettings{TaxRate: 11},
			withSaver: false,
			wantAfter: ordersdomain.InvoiceSettings{TaxRate: 11},
		},
		{
			name:      "a failed write leaves the running settings untouched",
			initial:   ordersdomain.InvoiceSettings{TaxRate: 12},
			update:    ordersdomain.InvoiceSettings{TaxRate: 11},
			withSaver: true,
			saverErr:  errors.New("read-only config"),
			wantErr:   true,
			wantAfter: ordersdomain.InvoiceSettings{TaxRate: 12},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var saved *ordersdomain.InvoiceSettings
			options := []ordersmodule.Option{ordersmodule.WithInvoiceSettings(tt.initial)}
			if tt.withSaver {
				options = append(options, ordersmodule.WithInvoiceSettingsSaver(func(settings ordersdomain.InvoiceSettings) error {
					if tt.saverErr != nil {
						return tt.saverErr
					}
					saved = &settings
					return nil
				}))
			}
			module := ordersmodule.New(options...)

			err := module.SetInvoiceSettings(tt.update)

			if tt.wantErr {
				require.ErrorIs(t, err, tt.saverErr)
				assert.Nil(t, saved)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantAfter, module.InvoiceSettings())
			if tt.wantSaved {
				require.NotNil(t, saved)
				assert.Equal(t, tt.update, *saved)
			}
		})
	}
}

func TestModule_Sweeper(t *testing.T) {
	t.Run("expires stale pendings until shutdown", func(t *testing.T) {
		var sweeps atomic.Int64
		store := ordersmocks.NewMockOrderStore(t)
		store.On("ExpirePending", mock.Anything, mock.Anything, 100).
			Return([]int64{}, nil).
			Run(func(mock.Arguments) { sweeps.Add(1) })
		module := testModule(t, store, 5*time.Millisecond)

		require.NoError(t, module.Init(nil))
		defer func() { require.NoError(t, module.Shutdown()) }()

		require.Eventually(t, func() bool {
			return sweeps.Load() >= 1
		}, 2*time.Second, 5*time.Millisecond, "the sweeper never ran")
	})

	t.Run("shutdown waits for the loop", func(t *testing.T) {
		store := ordersmocks.NewMockOrderStore(t)
		// The first tick may or may not land before the cancel; the
		// test asserts the shutdown completes, not the sweep count.
		store.On("ExpirePending", mock.Anything, mock.Anything, 100).Return([]int64{}, nil).Maybe()
		module := testModule(t, store, 5*time.Millisecond)
		require.NoError(t, module.Init(nil))

		done := make(chan struct{})
		go func() {
			_ = module.Shutdown()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Shutdown never returned")
		}
	})

	t.Run("a sweep with expirations survives a missing logger", func(t *testing.T) {
		var expired atomic.Int64
		store := ordersmocks.NewMockOrderStore(t)
		store.On("ExpirePending", mock.Anything, mock.Anything, 100).Return([]int64{42}, nil)
		order := ordersdomain.Order{ID: 42, Status: ordersdomain.StatusExpired}
		store.On("FindByID", mock.Anything, int64(42)).Return(&order, nil).Run(func(mock.Arguments) {
			expired.Add(1)
		})
		mail := ordersmocks.NewMockMailer(t)
		mail.On("SendOrderStatusEmail", mock.Anything, mock.Anything, ordersdomain.StatusPending).Return(nil)
		service := ordersdomain.NewService(
			store,
			ordersmocks.NewMockCartSource(t),
			ordersmocks.NewMockContentSource(t),
			mail,
			ordersmocks.NewMockInventory(t),
			"IDR",
		)
		// No WithLogger: the sweep's success log must degrade to
		// discard, never panic the loop.
		module := ordersmodule.New(
			ordersmodule.WithService(service),
			ordersmodule.WithSweeper(5*time.Millisecond, time.Hour),
		)

		require.NoError(t, module.Init(nil))
		defer func() { require.NoError(t, module.Shutdown()) }()

		require.Eventually(t, func() bool {
			return expired.Load() >= 1
		}, 2*time.Second, 5*time.Millisecond, "the flip was never read back")
	})

	t.Run("a non-positive interval leaves the sweeper off", func(t *testing.T) {
		store := ordersmocks.NewMockOrderStore(t)
		module := testModule(t, store, 0)

		require.NoError(t, module.Init(nil))
		require.NoError(t, module.Shutdown())
	})
}

func TestModule_Init(t *testing.T) {
	t.Run("a module without its service fails startup", func(t *testing.T) {
		module := ordersmodule.New()

		err := module.Init(nil)

		require.Error(t, err)
	})
}

// compile-time witness that the module satisfies the kernel contract.
var _ kernel.Module = (*ordersmodule.Module)(nil)
