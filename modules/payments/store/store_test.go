package store_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel/database"
	"github.com/aristorinjuang/lesstruct/kernel/pii"
	paymentsdomain "github.com/aristorinjuang/lesstruct/modules/payments/domain"
	"github.com/aristorinjuang/lesstruct/modules/payments/store"
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

// submittedManual is a typical pending manual transfer proof.
func submittedManual(orderID int64) *paymentsdomain.Payment {
	return &paymentsdomain.Payment{
		OrderID:   orderID,
		Method:    paymentsdomain.MethodManual,
		Status:    paymentsdomain.StatusPending,
		Reference: "BR-12345",
		Amount:    522500,
	}
}

// decide flips one pending record to a decision the way the service does,
// so tests build on the same predicates.
func decide(t *testing.T, s *store.Store, id int64, status paymentsdomain.Status, by string) {
	t.Helper()
	require.NoError(t, s.SetStatus(context.Background(), id, status, by))
}

func TestStore_InsertFindByID_RoundTrip(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	id, err := s.Insert(ctx, submittedManual(42))
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))

	found, err := s.FindByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, found.ID)
	assert.Equal(t, int64(42), found.OrderID)
	assert.Equal(t, paymentsdomain.MethodManual, found.Method)
	assert.Equal(t, paymentsdomain.StatusPending, found.Status)
	assert.Equal(t, "BR-12345", found.Reference)
	assert.Equal(t, int64(522500), found.Amount)
	assert.Equal(t, "", found.VerifiedBy)
	assert.False(t, found.CreatedAt.IsZero(), "storage timestamps fill the record")
	assert.False(t, found.UpdatedAt.IsZero(), "storage timestamps fill the record")
}

func TestStore_FindByID_Unknown(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	_, err := s.FindByID(ctx, 999)
	assert.ErrorIs(t, err, paymentsdomain.ErrPaymentNotFound)
}

func TestStore_FindByReference(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	id, err := s.Insert(ctx, &paymentsdomain.Payment{
		OrderID:    42,
		Method:     paymentsdomain.MethodMidtrans,
		Status:     paymentsdomain.StatusVerified,
		Reference:  "T-1",
		Amount:     522500,
		VerifiedBy: "midtrans",
	})
	require.NoError(t, err)

	found, err := s.FindByReference(ctx, "T-1")
	require.NoError(t, err)
	assert.Equal(t, id, found.ID)
	assert.Equal(t, "T-1", found.Reference)

	_, err = s.FindByReference(ctx, "T-missing")
	assert.ErrorIs(t, err, paymentsdomain.ErrPaymentNotFound)
}

func TestStore_SetStatus_FlipsAndRecords(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	id, err := s.Insert(ctx, submittedManual(42))
	require.NoError(t, err)

	decide(t, s, id, paymentsdomain.StatusVerified, "admin")

	found, err := s.FindByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, paymentsdomain.StatusVerified, found.Status)
	assert.Equal(t, "admin", found.VerifiedBy)
	assert.False(t, found.CreatedAt.IsZero(), "created_at is immutable")
}

func TestStore_SetStatus_Unknown(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	err := s.SetStatus(ctx, 999, paymentsdomain.StatusVerified, "admin")
	assert.ErrorIs(t, err, paymentsdomain.ErrPaymentNotFound)
}

func TestStore_List(t *testing.T) {
	tests := []struct {
		name     string
		status   *paymentsdomain.Status
		beforeID int64
		limit    int
		seed     func(t *testing.T, s *store.Store) (first, second, third int64)
		expected func(first, second, third int64) []int64
	}{
		{
			name:     "any status returns the newest first",
			status:   nil,
			beforeID: 0,
			limit:    20,
			seed: func(t *testing.T, s *store.Store) (first, second, third int64) {
				var maskedErr error
				first, maskedErr = s.Insert(context.Background(), submittedManual(1))
				require.NoError(t, maskedErr)
				second, maskedErr = s.Insert(context.Background(), submittedManual(2))
				require.NoError(t, maskedErr)
				third, maskedErr = s.Insert(context.Background(), submittedManual(3))
				require.NoError(t, maskedErr)
				decide(t, s, third, paymentsdomain.StatusVerified, "admin")
				return first, second, third
			},
			expected: func(first, second, third int64) []int64 {
				return []int64{third, second, first}
			},
		},
		{
			name:     "the status filter narrows the sweep",
			status:   func() *paymentsdomain.Status { s := paymentsdomain.StatusVerified; return &s }(),
			beforeID: 0,
			limit:    20,
			seed: func(t *testing.T, s *store.Store) (first, second, third int64) {
				var maskedErr error
				first, maskedErr = s.Insert(context.Background(), submittedManual(1))
				require.NoError(t, maskedErr)
				second, maskedErr = s.Insert(context.Background(), submittedManual(2))
				require.NoError(t, maskedErr)
				third, maskedErr = s.Insert(context.Background(), submittedManual(3))
				require.NoError(t, maskedErr)
				decide(t, s, second, paymentsdomain.StatusRejected, "")
				decide(t, s, third, paymentsdomain.StatusVerified, "admin")
				return first, second, third
			},
			expected: func(first, second, third int64) []int64 {
				return []int64{third}
			},
		},
		{
			name:     "the keyset boundary excludes newer rows",
			status:   nil,
			beforeID: 3,
			limit:    20,
			seed: func(t *testing.T, s *store.Store) (first, second, third int64) {
				var maskedErr error
				first, maskedErr = s.Insert(context.Background(), submittedManual(1))
				require.NoError(t, maskedErr)
				second, maskedErr = s.Insert(context.Background(), submittedManual(2))
				require.NoError(t, maskedErr)
				third, maskedErr = s.Insert(context.Background(), submittedManual(3))
				require.NoError(t, maskedErr)
				return first, second, third
			},
			expected: func(first, second, third int64) []int64 {
				return []int64{second, first}
			},
		},
		{
			name:     "a zero limit sweeps nothing",
			status:   nil,
			beforeID: 0,
			limit:    0,
			seed: func(t *testing.T, s *store.Store) (first, second, third int64) {
				var maskedErr error
				first, maskedErr = s.Insert(context.Background(), submittedManual(1))
				require.NoError(t, maskedErr)
				second, maskedErr = s.Insert(context.Background(), submittedManual(2))
				require.NoError(t, maskedErr)
				third, maskedErr = s.Insert(context.Background(), submittedManual(3))
				require.NoError(t, maskedErr)
				return first, second, third
			},
			expected: func(first, second, third int64) []int64 {
				return []int64{}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupStore(t)
			first, second, third := tt.seed(t, s)

			found, err := s.List(context.Background(), tt.status, tt.beforeID, tt.limit)

			require.NoError(t, err)
			ids := make([]int64, 0, len(found))
			for _, record := range found {
				ids = append(ids, record.ID)
			}
			assert.Equal(t, tt.expected(first, second, third), ids)
		})
	}

	t.Run("an empty store lists as an empty, never nil, slice", func(t *testing.T) {
		s := setupStore(t)

		found, err := s.List(context.Background(), nil, 0, 20)

		require.NoError(t, err)
		assert.NotNil(t, found)
		assert.Empty(t, found)
	})
}

func TestStore_ListByOrder(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	first, err := s.Insert(ctx, submittedManual(42))
	require.NoError(t, err)
	second, err := s.Insert(ctx, submittedManual(7))
	require.NoError(t, err)
	third, err := s.Insert(ctx, submittedManual(42))
	require.NoError(t, err)
	decide(t, s, first, paymentsdomain.StatusRejected, "")

	found, err := s.ListByOrder(ctx, 42)
	require.NoError(t, err)
	require.Len(t, found, 2)
	assert.Equal(t, third, found[0].ID, "newest first")
	assert.Equal(t, first, found[1].ID)
	assert.Equal(t, paymentsdomain.StatusRejected, found[1].Status)

	other, err := s.ListByOrder(ctx, 7)
	require.NoError(t, err)
	require.Len(t, other, 1)
	assert.Equal(t, second, other[0].ID)

	empty, err := s.ListByOrder(ctx, 999)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func setupStoreDB(t *testing.T) (*store.Store, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
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
		)
	`)
	require.NoError(t, err)

	return store.New(db, database.SQLiteDialect(), testCipher(t)), db
}

func TestStore_CiphertextAtRest(t *testing.T) {
	ctx := context.Background()
	s, db := setupStoreDB(t)

	id, err := s.Insert(ctx, &paymentsdomain.Payment{
		OrderID:    42,
		Method:     paymentsdomain.MethodMidtrans,
		Status:     paymentsdomain.StatusVerified,
		Reference:  "T-1",
		Amount:     522500,
		VerifiedBy: "midtrans",
	})
	require.NoError(t, err)

	var reference, verifiedBy, referenceIdx string
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT reference, verified_by, reference_idx FROM payments_payment WHERE id = ?
	`, id).Scan(&reference, &verifiedBy, &referenceIdx))

	assert.True(t, pii.IsEncrypted(reference))
	assert.True(t, pii.IsEncrypted(verifiedBy))
	assert.NotContains(t, reference, "T-1")
	assert.NotContains(t, verifiedBy, "midtrans")
	assert.Equal(t, testCipher(t).Index("T-1"), referenceIdx)

	found, err := s.FindByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "T-1", found.Reference)
	assert.Equal(t, "midtrans", found.VerifiedBy)
}

func TestStore_BlindIndexLookup(t *testing.T) {
	ctx := context.Background()
	s := setupStore(t)

	id, err := s.Insert(ctx, &paymentsdomain.Payment{
		OrderID:    42,
		Method:     paymentsdomain.MethodMidtrans,
		Status:     paymentsdomain.StatusVerified,
		Reference:  "T-1",
		Amount:     522500,
		VerifiedBy: "midtrans",
	})
	require.NoError(t, err)

	tests := []struct {
		name      string
		reference string
		wantErr   bool
	}{
		{
			name:      "success - stored transaction ID hits the index",
			reference: "T-1",
			wantErr:   false,
		},
		{
			name:      "error - unknown transaction ID misses",
			reference: "T-missing",
			wantErr:   true,
		},
		{
			name:      "error - empty reference never matches",
			reference: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := s.FindByReference(ctx, tt.reference)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, paymentsdomain.ErrPaymentNotFound)
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

	_, err := db.ExecContext(ctx, `
		INSERT INTO payments_payment (order_id, method, status, reference, amount, verified_by)
		VALUES (42, 'manual', 'pending', 'BR-12345', 522500, '')
	`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO payments_payment (order_id, method, status, reference, amount, verified_by)
		VALUES (43, 'manual', 'verified', 'BR-67890', 100000, 'admin')
	`)
	require.NoError(t, err)

	encrypted, err := s.EncryptPending(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, 2, encrypted)

	var reference, referenceIdx string
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT reference, reference_idx FROM payments_payment WHERE reference_idx = ?
	`, testCipher(t).Index("BR-12345")).Scan(&reference, &referenceIdx))
	assert.True(t, pii.IsEncrypted(reference))
	assert.NotContains(t, reference, "BR-12345")

	found, err := s.FindByReference(ctx, "BR-67890")
	require.NoError(t, err)
	assert.Equal(t, "BR-67890", found.Reference)
	assert.Equal(t, "admin", found.VerifiedBy)

	again, err := s.EncryptPending(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, 0, again, "a second sweep is a no-op")
}

func TestStore_DecryptCorrupt(t *testing.T) {
	ctx := context.Background()
	s, db := setupStoreDB(t)

	_, err := db.ExecContext(ctx, `
		INSERT INTO payments_payment (order_id, method, status, reference, amount, verified_by)
		VALUES (42, 'manual', 'pending', 'v1:!!!not-base64!!!', 522500, '')
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
		)
	`)
	require.NoError(t, err)

	s := store.New(db, database.SQLiteDialect(), nil)
	id, err := s.Insert(ctx, submittedManual(42))
	require.NoError(t, err)

	var stored string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT reference FROM payments_payment WHERE id = ?`, id).Scan(&stored))
	assert.Equal(t, "BR-12345", stored, "nil cipher stores plaintext")

	found, err := s.FindByReference(ctx, "BR-12345")
	require.NoError(t, err)
	assert.Equal(t, "BR-12345", found.Reference)

	encrypted, err := s.EncryptPending(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, 0, encrypted, "nil cipher sweeps nothing")
}
