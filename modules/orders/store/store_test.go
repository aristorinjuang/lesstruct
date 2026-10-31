package store_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel/database"
	"github.com/aristorinjuang/lesstruct/kernel/legal"
	"github.com/aristorinjuang/lesstruct/kernel/pii"
	ordersdomain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
	"github.com/aristorinjuang/lesstruct/modules/orders/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func testCipher(t *testing.T) *pii.Cipher {
	t.Helper()
	key := make([]byte, pii.MasterKeySize)
	for i := range key {
		key[i] = byte(i + 1)
	}
	sealer, err := pii.New(key)
	require.NoError(t, err)
	return sealer
}

func setupStore(t *testing.T) *store.Store {
	t.Helper()
	s, _ := setupStoreDB(t)
	return s
}

// setupStoreDB additionally exposes the raw database handle so sweep
// tests can backdate created_at: the store owns the sweep semantics, the
// test owns the clock.
func setupStoreDB(t *testing.T) (*store.Store, *sql.DB) {
	t.Helper()
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
			consent_evidence TEXT,
			tracking_token_hash TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE orders_order_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id INTEGER NOT NULL,
			product_content_id INTEGER NOT NULL,
			variant_id INTEGER NOT NULL DEFAULT 0,
			sku TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			unit_price INTEGER NOT NULL DEFAULT 0,
			quantity INTEGER NOT NULL DEFAULT 1,
			FOREIGN KEY (order_id) REFERENCES orders_order(id) ON DELETE CASCADE
		);
	`)
	require.NoError(t, err)

	return store.New(db, database.SQLiteDialect(), testCipher(t)), db
}

func sampleOrder(email string) *ordersdomain.Order {
	variantID := new(int64(3))
	return &ordersdomain.Order{
		Status:        ordersdomain.StatusPending,
		CustomerEmail: email,
		CustomerName:  "Annisa Putri",
		CustomerPhone: "+6281234567890",
		ShipTo: ordersdomain.Address{
			Street:     "Jl. Kemang Raya 21",
			City:       "Jakarta Selatan",
			Province:   "DKI Jakarta",
			PostalCode: "12730",
		},
		Currency: "IDR",
		Subtotal: 748000,
		Items: []ordersdomain.OrderItem{
			{
				ProductContentID: 7,
				SKU:              "TSHIRT",
				Title:            "Nainggolan T-Shirt",
				UnitPrice:        299000,
				Quantity:         2,
			},
			{
				ProductContentID: 8,
				VariantID:        variantID,
				SKU:              "MUG-BLUE",
				Title:            "Product",
				UnitPrice:        150000,
				Quantity:         1,
			},
		},
	}
}

func TestStore_CreateFind_RoundTrip(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)
	order := sampleOrder("shopper@example.com")

	id, err := s.CreateOrder(ctx, order)
	require.NoError(t, err)
	assert.Positive(t, id)
	assert.False(t, order.CreatedAt.IsZero())
	assert.False(t, order.UpdatedAt.IsZero())

	found, err := s.FindByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, found.ID)
	assert.Equal(t, ordersdomain.StatusPending, found.Status)
	assert.Equal(t, "shopper@example.com", found.CustomerEmail)
	assert.Equal(t, "IDR", found.Currency)
	assert.Equal(t, int64(748000), found.Subtotal)
	assert.Equal(t, "Jl. Kemang Raya 21", found.ShipTo.Street)
	assert.Equal(t, "12730", found.ShipTo.PostalCode)
	assert.False(t, found.CreatedAt.IsZero())

	require.Len(t, found.Items, 2)
	assert.Positive(t, found.Items[0].ID)
	assert.Equal(t, id, found.Items[0].OrderID)
	assert.Equal(t, 7, found.Items[0].ProductContentID)
	assert.Nil(t, found.Items[0].VariantID)
	assert.Equal(t, "TSHIRT", found.Items[0].SKU)
	assert.Equal(t, int64(598000), found.Items[0].LineTotal())
	assert.Positive(t, found.Items[1].ID)
	require.NotNil(t, found.Items[1].VariantID)
	assert.Equal(t, int64(3), *found.Items[1].VariantID)
}

func TestStore_ConsentRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)
	acceptedAt := time.Date(2026, 10, 25, 9, 30, 0, 0, time.UTC)
	order := sampleOrder("shopper@example.com")
	order.Consent = &legal.Consent{
		AcceptedAt: acceptedAt,
		Terms:      legal.Document{Page: legal.PageTermsOfService, PageID: 3, Slug: "terms-of-service", Language: "en"},
		Privacy:    legal.Document{Page: legal.PagePrivacyPolicy, PageID: 4, Slug: "privacy-policy", Language: "en"},
	}

	id, err := s.CreateOrder(ctx, order)
	require.NoError(t, err)

	detail, err := s.FindByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, detail.Consent)
	assert.Equal(t, acceptedAt, detail.Consent.AcceptedAt)
	assert.Equal(t, "terms-of-service", detail.Consent.Terms.Slug)
	assert.Equal(t, 4, detail.Consent.Privacy.PageID)

	listed, err := s.List(ctx, nil, 0, 50)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.NotNil(t, listed[0].Consent)
	assert.Equal(t, "privacy-policy", listed[0].Consent.Privacy.Slug)

	plainID, err := s.CreateOrder(ctx, sampleOrder("plain@example.com"))
	require.NoError(t, err)
	plain, err := s.FindByID(ctx, plainID)
	require.NoError(t, err)
	assert.Nil(t, plain.Consent, "orders without consent evidence read back without it")
}

func TestStore_FindByID_Unknown(t *testing.T) {
	_, err := setupStore(t).FindByID(context.Background(), 999)
	assert.ErrorIs(t, err, ordersdomain.ErrOrderNotFound)
}

func TestStore_FindByNumberAndEmailAndToken(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	order := sampleOrder("shopper@example.com")
	order.TrackingToken = "track-token"
	order.TrackingTokenHash = sha256Hex(order.TrackingToken)
	id, err := s.CreateOrder(ctx, order)
	require.NoError(t, err)

	found, err := s.FindByNumberAndEmailAndToken(ctx, id, "shopper@example.com", sha256Hex("track-token"))
	require.NoError(t, err)
	assert.Equal(t, id, found.ID)

	_, err = s.FindByNumberAndEmailAndToken(ctx, id, "someone-else@example.com", sha256Hex("track-token"))
	assert.ErrorIs(t, err, ordersdomain.ErrOrderNotFound, "a wrong email never opens an order")

	_, err = s.FindByNumberAndEmailAndToken(ctx, id, "shopper@example.com", sha256Hex("wrong-token"))
	assert.ErrorIs(t, err, ordersdomain.ErrOrderNotFound, "a wrong token never opens an order")

	// Legacy orders carry an empty hash; the guest gate can never match one.
	legacyID, err := s.CreateOrder(ctx, sampleOrder("legacy@example.com"))
	require.NoError(t, err)
	_, err = s.FindByNumberAndEmailAndToken(ctx, legacyID, "legacy@example.com", sha256Hex("track-token"))
	assert.ErrorIs(t, err, ordersdomain.ErrOrderNotFound, "legacy orders are untrackable through the guest gate")
}

// sha256Hex mirrors the domain's token hash so store tests can present the
// at-rest form.
func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func TestStore_List(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	firstID, err := s.CreateOrder(ctx, sampleOrder("first@example.com"))
	require.NoError(t, err)
	secondID, err := s.CreateOrder(ctx, sampleOrder("second@example.com"))
	require.NoError(t, err)
	third := sampleOrder("third@example.com")
	third.Status = ordersdomain.StatusPaid
	thirdID, err := s.CreateOrder(ctx, third)
	require.NoError(t, err)

	newest, err := s.List(ctx, nil, 0, 50)
	require.NoError(t, err)
	require.Len(t, newest, 3)
	assert.Equal(t, thirdID, newest[0].ID)
	assert.Equal(t, secondID, newest[1].ID)
	assert.Equal(t, firstID, newest[2].ID)
	assert.Empty(t, newest[0].Items, "list rows carry no items")
	assert.Equal(t, 2, newest[0].ItemCount, "list rows carry the read-time line count")
	assert.Equal(t, 2, newest[1].ItemCount)
	assert.Equal(t, 2, newest[2].ItemCount)

	page, err := s.List(ctx, nil, thirdID, 50)
	require.NoError(t, err)
	require.Len(t, page, 2)
	assert.Equal(t, secondID, page[0].ID)

	limited, err := s.List(ctx, nil, 0, 2)
	require.NoError(t, err)
	assert.Len(t, limited, 2)

	paid := ordersdomain.StatusPaid
	filtered, err := s.List(ctx, &paid, 0, 50)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, thirdID, filtered[0].ID)

	unknown := ordersdomain.Status("refunded")
	none, err := s.List(ctx, &unknown, 0, 50)
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestStore_SetStatus(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	id, err := s.CreateOrder(ctx, sampleOrder("shopper@example.com"))
	require.NoError(t, err)

	require.NoError(t, s.SetStatus(ctx, id, ordersdomain.StatusPaid))

	found, err := s.FindByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, ordersdomain.StatusPaid, found.Status)
	assert.False(t, found.UpdatedAt.Before(found.CreatedAt))

	err = s.SetStatus(ctx, 999, ordersdomain.StatusPaid)
	assert.ErrorIs(t, err, ordersdomain.ErrOrderNotFound)
}

func TestStore_TimestampsMonotonic(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	id, err := s.CreateOrder(ctx, sampleOrder("shopper@example.com"))
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	require.NoError(t, s.SetStatus(ctx, id, ordersdomain.StatusPacked))

	found, err := s.FindByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, ordersdomain.StatusPacked, found.Status)
}

func TestStore_MarkPaid(t *testing.T) {
	ctx := context.Background()

	t.Run("flips a pending order once", func(t *testing.T) {
		s := setupStore(t)
		id, err := s.CreateOrder(ctx, sampleOrder("shopper@example.com"))
		require.NoError(t, err)

		flipped, err := s.MarkPaid(ctx, id)
		require.NoError(t, err)
		assert.True(t, flipped)

		found, err := s.FindByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, ordersdomain.StatusPaid, found.Status)

		flipped, err = s.MarkPaid(ctx, id)
		require.NoError(t, err)
		assert.False(t, flipped, "the second flip loses the pending guard")
	})

	t.Run("an unknown order reports unchanged", func(t *testing.T) {
		s := setupStore(t)

		flipped, err := s.MarkPaid(ctx, 999)
		require.NoError(t, err)
		assert.False(t, flipped)
	})
}

func TestStore_SetShipping(t *testing.T) {
	ctx := context.Background()

	t.Run("writes and replaces the line on a pending order", func(t *testing.T) {
		s := setupStore(t)
		id, err := s.CreateOrder(ctx, sampleOrder("shopper@example.com"))
		require.NoError(t, err)

		wrote, err := s.SetShipping(ctx, id, 15000)
		require.NoError(t, err)
		assert.True(t, wrote)

		found, err := s.FindByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, int64(15000), found.ShippingMinor)

		wrote, err = s.SetShipping(ctx, id, 20000)
		require.NoError(t, err)
		assert.True(t, wrote)

		found, err = s.FindByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, int64(20000), found.ShippingMinor)
	})

	t.Run("an unknown order reports unchanged", func(t *testing.T) {
		s := setupStore(t)

		wrote, err := s.SetShipping(ctx, 999, 15000)
		require.NoError(t, err)
		assert.False(t, wrote)
	})

	t.Run("a settled order loses the pending guard", func(t *testing.T) {
		s := setupStore(t)
		id, err := s.CreateOrder(ctx, sampleOrder("shopper@example.com"))
		require.NoError(t, err)
		require.NoError(t, s.SetStatus(ctx, id, ordersdomain.StatusPaid))

		wrote, err := s.SetShipping(ctx, id, 15000)
		require.NoError(t, err)
		assert.False(t, wrote, "the write loses the pending guard")

		found, err := s.FindByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, int64(0), found.ShippingMinor)
	})
}

func TestStore_ExpirePending(t *testing.T) {
	ctx := context.Background()
	// A sweep boundary in the past: every order created after it — that
	// is, every freshly placed order — stays pending.
	boundary := time.Now().UTC().Add(-time.Hour)

	t.Run("flips only pendings older than the boundary", func(t *testing.T) {
		s, db := setupStoreDB(t)
		stale, err := s.CreateOrder(ctx, sampleOrder("stale@example.com"))
		require.NoError(t, err)
		_, err = db.Exec(`UPDATE orders_order SET created_at = ? WHERE id = ?`,
			boundary.Add(-time.Hour), stale)
		require.NoError(t, err)
		fresh, err := s.CreateOrder(ctx, sampleOrder("fresh@example.com"))
		require.NoError(t, err)

		flipped, err := s.ExpirePending(ctx, boundary, 100)
		require.NoError(t, err)
		assert.Equal(t, []int64{stale}, flipped)

		staleOrder, err := s.FindByID(ctx, stale)
		require.NoError(t, err)
		assert.Equal(t, ordersdomain.StatusExpired, staleOrder.Status)

		freshOrder, err := s.FindByID(ctx, fresh)
		require.NoError(t, err)
		assert.Equal(t, ordersdomain.StatusPending, freshOrder.Status)
	})

	t.Run("honors the limit", func(t *testing.T) {
		s, db := setupStoreDB(t)
		first, err := s.CreateOrder(ctx, sampleOrder("a@example.com"))
		require.NoError(t, err)
		second, err := s.CreateOrder(ctx, sampleOrder("b@example.com"))
		require.NoError(t, err)
		_, err = db.Exec(`UPDATE orders_order SET created_at = ? WHERE id IN (?, ?)`,
			boundary.Add(-time.Hour), first, second)
		require.NoError(t, err)

		flipped, err := s.ExpirePending(ctx, boundary, 1)
		require.NoError(t, err)
		assert.Equal(t, []int64{first}, flipped, "the sweep walks ascending and stops at the limit")
	})

	t.Run("nothing stale is an empty sweep", func(t *testing.T) {
		s := setupStore(t)
		_, err := s.CreateOrder(ctx, sampleOrder("shopper@example.com"))
		require.NoError(t, err)

		flipped, err := s.ExpirePending(ctx, boundary, 100)
		require.NoError(t, err)
		assert.Empty(t, flipped)
	})
}

func TestStore_CiphertextAtRest(t *testing.T) {
	ctx := context.Background()
	s, db := setupStoreDB(t)

	order := sampleOrder("shopper@example.com")
	order.CustomerPhone = ""
	id, err := s.CreateOrder(ctx, order)
	require.NoError(t, err)

	var email, name, phone, ship, emailIdx string
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT customer_email, customer_name, customer_phone, ship_to, customer_email_idx
		FROM orders_order WHERE id = ?
	`, id).Scan(&email, &name, &phone, &ship, &emailIdx))

	for _, stored := range []string{email, name, ship} {
		assert.True(t, pii.IsEncrypted(stored), "PII at rest carries the envelope")
	}
	assert.NotContains(t, email, "shopper@example.com")
	assert.NotContains(t, name, "Annisa Putri")
	assert.NotContains(t, ship, "Kemang")
	assert.Equal(t, "", phone, "empty phone stays empty, never encrypted")
	assert.Equal(t, testCipher(t).Index("shopper@example.com"), emailIdx)

	found, err := s.FindByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "shopper@example.com", found.CustomerEmail)
	assert.Equal(t, "Annisa Putri", found.CustomerName)
	assert.Equal(t, "", found.CustomerPhone)
	assert.Equal(t, "Jl. Kemang Raya 21", found.ShipTo.Street)
}

func TestStore_BlindIndexLookup(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	order := sampleOrder("shopper@example.com")
	order.TrackingTokenHash = sha256Hex("track-token")
	id, err := s.CreateOrder(ctx, order)
	require.NoError(t, err)

	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{
			name:    "success - exact normalized email hits the index",
			email:   "shopper@example.com",
			wantErr: false,
		},
		{
			name:    "success - case-variant resolves once normalized like the service does",
			email:   strings.ToLower(strings.TrimSpace("  Shopper@Example.COM  ")),
			wantErr: false,
		},
		{
			name:    "error - wrong email misses indistinguishably",
			email:   "someone-else@example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := s.FindByNumberAndEmailAndToken(ctx, id, tt.email, sha256Hex("track-token"))
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, ordersdomain.ErrOrderNotFound)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, id, found.ID)
		})
	}
}

func TestStore_EncryptPending(t *testing.T) {
	ctx := context.Background()
	s, db := setupStoreDB(t)

	legacyShip := `{"street":"Jl. Kemang Raya 21","city":"Jakarta Selatan","province":"DKI Jakarta","postalCode":"12730"}`
	_, err := db.ExecContext(ctx, `
		INSERT INTO orders_order (status, customer_email, customer_name, customer_phone, ship_to, currency, subtotal)
		VALUES ('pending', 'legacy@example.com', 'Legacy Shopper', '+6281000000000', ?, 'IDR', 100000)
	`, legacyShip)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO orders_order (status, customer_email, customer_name, customer_phone, ship_to, currency, subtotal)
		VALUES ('pending', 'plain@example.com', 'Plain Shopper', '', '{}', 'IDR', 50000)
	`)
	require.NoError(t, err)

	encrypted, err := s.EncryptPending(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, 2, encrypted)

	var email, emailIdx, phone string
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT customer_email, customer_email_idx, customer_phone FROM orders_order WHERE customer_email_idx = ?
	`, testCipher(t).Index("legacy@example.com")).Scan(&email, &emailIdx, &phone))
	assert.True(t, pii.IsEncrypted(email))
	assert.NotContains(t, email, "legacy@example.com")

	found, err := s.FindByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "Legacy Shopper", found.CustomerName)
	assert.Equal(t, "+6281000000000", found.CustomerPhone)
	assert.Equal(t, "12730", found.ShipTo.PostalCode)

	again, err := s.EncryptPending(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, 0, again, "a second sweep is a no-op")

	limited, err := s.EncryptPending(ctx, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, limited)
}

func TestStore_DecryptCorrupt(t *testing.T) {
	ctx := context.Background()
	s, db := setupStoreDB(t)

	_, err := db.ExecContext(ctx, `
		INSERT INTO orders_order (status, customer_email, customer_name, customer_phone, ship_to, currency, subtotal)
		VALUES ('pending', 'v1:!!!not-base64!!!', 'Corrupt Shopper', '', '{}', 'IDR', 100000)
	`)
	require.NoError(t, err)

	_, err = s.FindByID(ctx, 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, pii.ErrCorruptCiphertext)
}

func TestStore_NilCipherLegacyMode(t *testing.T) {
	ctx := context.Background()
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
			consent_evidence TEXT,
			tracking_token_hash TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE orders_order_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id INTEGER NOT NULL,
			product_content_id INTEGER NOT NULL,
			variant_id INTEGER NOT NULL DEFAULT 0,
			sku TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			unit_price INTEGER NOT NULL DEFAULT 0,
			quantity INTEGER NOT NULL DEFAULT 1,
			FOREIGN KEY (order_id) REFERENCES orders_order(id) ON DELETE CASCADE
		);
	`)
	require.NoError(t, err)

	s := store.New(db, database.SQLiteDialect(), nil)
	id, err := s.CreateOrder(ctx, sampleOrder("shopper@example.com"))
	require.NoError(t, err)

	var stored string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT customer_email FROM orders_order WHERE id = ?`, id).Scan(&stored))
	assert.Equal(t, "shopper@example.com", stored, "nil cipher stores plaintext")

	found, err := s.FindByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "shopper@example.com", found.CustomerEmail)

	encrypted, err := s.EncryptPending(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, 0, encrypted, "nil cipher sweeps nothing")
}
