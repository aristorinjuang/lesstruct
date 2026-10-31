package commerce

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/app"
	"github.com/aristorinjuang/lesstruct/internal/config"
	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
	"github.com/aristorinjuang/lesstruct/kernel/pii"
	ordersstore "github.com/aristorinjuang/lesstruct/modules/orders/store"
	paymentsstore "github.com/aristorinjuang/lesstruct/modules/payments/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func testMasterKey(t *testing.T) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString(make([]byte, pii.MasterKeySize))
}

func TestNewPIICipher(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "success - valid 32-byte key",
			key:     testMasterKey(t),
			wantErr: false,
		},
		{
			name:    "error - missing key fails closed",
			key:     "",
			wantErr: true,
		},
		{
			name:    "error - non-base64 key fails closed",
			key:     "!!!not-base64!!!",
			wantErr: true,
		},
		{
			name:    "error - short key fails closed",
			key:     base64.StdEncoding.EncodeToString(make([]byte, 16)),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sealer, err := newPIICipher(&config.Config{PIIMasterKey: tt.key})
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, sealer)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, sealer)
		})
	}
}

// backfillStores builds cipher-aware order and payment stores over a
// migrated-schema sqlite database: the shape Mount sweeps after module
// migrations ran.
func backfillStores(t *testing.T) (*ordersstore.Store, *paymentsstore.Store, *sql.DB) {
	t.Helper()
	sealer, err := newPIICipher(&config.Config{PIIMasterKey: testMasterKey(t)})
	require.NoError(t, err)

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		CREATE TABLE orders_order (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			status TEXT NOT NULL DEFAULT 'pending',
			customer_email TEXT NOT NULL,
			customer_name TEXT NOT NULL,
			customer_phone TEXT NOT NULL DEFAULT '',
			ship_to TEXT NOT NULL DEFAULT '{}',
			customer_email_idx TEXT NOT NULL DEFAULT '',
			currency TEXT NOT NULL DEFAULT '',
			subtotal INTEGER NOT NULL DEFAULT 0,
			shipping_minor INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			consent_at DATETIME,
			consent_evidence TEXT
		);
		CREATE TABLE orders_order_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id INTEGER NOT NULL,
			product_content_id INTEGER NOT NULL,
			variant_id INTEGER NOT NULL DEFAULT 0,
			sku TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			unit_price INTEGER NOT NULL DEFAULT 0,
			quantity INTEGER NOT NULL DEFAULT 1
		);
		CREATE TABLE payments_payment (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id INTEGER NOT NULL,
			method TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			reference TEXT NOT NULL DEFAULT '',
			amount INTEGER NOT NULL DEFAULT 0,
			verified_by TEXT NOT NULL DEFAULT '',
			reference_idx TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	require.NoError(t, err)

	return ordersstore.New(db, kerneldatabase.SQLiteDialect(), sealer),
		paymentsstore.New(db, kerneldatabase.SQLiteDialect(), sealer),
		db
}

func TestBackfillPII(t *testing.T) {
	ctx := context.Background()
	orders, payments, db := backfillStores(t)

	_, err := db.ExecContext(ctx, `
		INSERT INTO orders_order (status, customer_email, customer_name, customer_phone, ship_to, currency, subtotal)
		VALUES ('pending', 'legacy@example.com', 'Legacy Shopper', '+6281000000000',
			'{"street":"Jl. Kemang Raya 21","city":"Jakarta Selatan","province":"DKI Jakarta","postalCode":"12730"}',
			'IDR', 100000)
	`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO payments_payment (order_id, method, status, reference, amount, verified_by)
		VALUES (1, 'manual', 'pending', 'BR-12345', 100000, '')
	`)
	require.NoError(t, err)

	s := &state{ordersStore: orders, paymentsStore: payments}
	require.NoError(t, s.backfillPII(ctx))

	var email, emailIdx string
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT customer_email, customer_email_idx FROM orders_order WHERE id = 1
	`).Scan(&email, &emailIdx))
	assert.True(t, pii.IsEncrypted(email))
	assert.NotContains(t, email, "legacy@example.com")
	assert.NotEmpty(t, emailIdx)

	found, err := orders.FindByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "Legacy Shopper", found.CustomerName)
	assert.Equal(t, "12730", found.ShipTo.PostalCode)

	var reference, referenceIdx string
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT reference, reference_idx FROM payments_payment WHERE id = 1
	`).Scan(&reference, &referenceIdx))
	assert.True(t, pii.IsEncrypted(reference))
	assert.NotContains(t, reference, "BR-12345")
	assert.NotEmpty(t, referenceIdx)

	proof, err := payments.FindByReference(ctx, "BR-12345")
	require.NoError(t, err)
	assert.Equal(t, "BR-12345", proof.Reference)

	require.NoError(t, s.backfillPII(ctx), "a second sweep is a no-op")

	remaining, err := orders.EncryptPending(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining)
	remaining, err = payments.EncryptPending(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining)
}

func TestBackfillPII_CorruptRowFailsClosed(t *testing.T) {
	ctx := context.Background()
	orders, payments, db := backfillStores(t)

	_, err := db.ExecContext(ctx, `
		INSERT INTO orders_order (status, customer_email, customer_name, customer_phone, ship_to, currency, subtotal)
		VALUES ('pending', 'v1:!!!not-base64!!!', 'Corrupt Shopper', '', '{}', 'IDR', 100000)
	`)
	require.NoError(t, err)

	s := &state{ordersStore: orders, paymentsStore: payments}
	err = s.backfillPII(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to encrypt legacy order PII")
}

// missingKeyCore builds a core whose config carries the given PII key and
// no config file, so LoadOrders/LoadInvoice fall to defaults and wiring
// reaches the cipher gate without touching any service.
func missingKeyCore(key string) *app.Core {
	return &app.Core{Cfg: &config.Config{
		ConfigDir:    "/nonexistent-lesstruct-test",
		ConfigFile:   "config.toml",
		PIIMasterKey: key,
	}}
}

func TestWireOrders_FailClosed(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{
			name: "error - missing key fails closed",
			key:  "",
		},
		{
			name: "error - non-base64 key fails closed",
			key:  "!!!not-base64!!!",
		},
		{
			name: "error - short key fails closed",
			key:  base64.StdEncoding.EncodeToString(make([]byte, 16)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &state{configExtras: map[string]any{}}
			err := s.wireOrders(missingKeyCore(tt.key))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "PII_MASTER_KEY")
			assert.Nil(t, s.ordersStore)
		})
	}
}

func TestWirePayments_FailClosed(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{
			name: "error - missing key fails closed",
			key:  "",
		},
		{
			name: "error - non-base64 key fails closed",
			key:  "!!!not-base64!!!",
		},
		{
			name: "error - short key fails closed",
			key:  base64.StdEncoding.EncodeToString(make([]byte, 16)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &state{configExtras: map[string]any{}}
			err := s.wirePayments(missingKeyCore(tt.key))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "PII_MASTER_KEY")
			assert.Nil(t, s.paymentsStore)
		})
	}
}

func TestMountWithBackfill_SweepsLegacy(t *testing.T) {
	ctx := context.Background()
	orders, payments, db := backfillStores(t)

	_, err := db.ExecContext(ctx, `
		INSERT INTO orders_order (status, customer_email, customer_name, customer_phone, ship_to, currency, subtotal)
		VALUES ('pending', 'legacy@example.com', 'Legacy Shopper', '+6281000000000',
			'{"street":"Jl. Kemang Raya 21","city":"Jakarta Selatan","province":"DKI Jakarta","postalCode":"12730"}',
			'IDR', 100000)
	`)
	require.NoError(t, err)

	s := &state{ordersStore: orders, paymentsStore: payments}
	called := false
	require.NoError(t, s.mountWithBackfill(&app.Core{}, func(*app.Core) error {
		called = true
		return nil
	}))
	assert.True(t, called, "the commerce mount runs the blog mount chain first")

	found, err := orders.FindByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "Legacy Shopper", found.CustomerName)
}

func TestMountWithBackfill_BlogMountError(t *testing.T) {
	orders, payments, _ := backfillStores(t)
	s := &state{ordersStore: orders, paymentsStore: payments}

	stubErr := errors.New("blog mount failed")
	err := s.mountWithBackfill(&app.Core{}, func(*app.Core) error {
		return stubErr
	})
	require.ErrorIs(t, err, stubErr)
}
