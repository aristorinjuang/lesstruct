// Package store is the single-source payment record repository: one
// implementation against *sql.DB plus a kernel Dialect, following the
// orders store pattern. Placeholder rendering comes from the dialect;
// everything else (columns, scans, messages) is shared.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
	"github.com/aristorinjuang/lesstruct/kernel/pii"
	paymentsdomain "github.com/aristorinjuang/lesstruct/modules/payments/domain"
)

const (
	// paymentColumns is the shared column list of the payment record, in
	// the scan and insert order the row functions rely on.
	paymentColumns = "id, order_id, method, status, reference, amount, verified_by, created_at, updated_at"
)

// scanPayment fills a payment record from one row of the shared column
// list, opening the encrypted reference and verifier identity through
// the cipher.
func (s *Store) scanPayment(scan func(dest ...any) error, payment *paymentsdomain.Payment) error {
	var reference, verifiedBy string
	if err := scan(
		&payment.ID,
		&payment.OrderID,
		&payment.Method,
		&payment.Status,
		&reference,
		&payment.Amount,
		&verifiedBy,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	); err != nil {
		return err
	}
	decrypted, err := s.decryptField(reference)
	if err != nil {
		return fmt.Errorf("failed to decrypt payment reference: %w", err)
	}
	payment.Reference = decrypted
	if decrypted, err = s.decryptField(verifiedBy); err != nil {
		return fmt.Errorf("failed to decrypt payment verifier: %w", err)
	}
	payment.VerifiedBy = decrypted
	return nil
}

// Store is the payment record single-source store. The reference and
// verifier identity are encrypted at this boundary: writes seal them
// through the cipher and reads open them, while webhook dedup resolves
// through the blind-index column over the reference. A nil cipher stores
// and reads plaintext (legacy mode); the startup sweep encrypts those
// rows once a key is provisioned.
type Store struct {
	db      *sql.DB
	dialect kerneldatabase.Dialect
	cipher  *pii.Cipher
}

// placeholders returns a positional placeholder generator: each call renders
// the next bind marker ($1, $2… on PostgreSQL; ? elsewhere), so call order
// matches argument order by construction.
func (s *Store) placeholders() func() string {
	index := 0
	return func() string {
		marker := s.dialect.Placeholder(index)
		index++
		return marker
	}
}

// encryptField seals one PII field for storage. A nil cipher stores
// plaintext (legacy mode); empty stays empty either way.
func (s *Store) encryptField(plaintext string) (string, error) {
	if s.cipher == nil {
		return plaintext, nil
	}
	return s.cipher.Encrypt(plaintext)
}

// decryptField opens one stored PII field. A nil cipher reads legacy
// plaintext as-is; a corrupt envelope fails the read — never silent wrong
// data.
func (s *Store) decryptField(stored string) (string, error) {
	if s.cipher == nil {
		return stored, nil
	}
	return s.cipher.Decrypt(stored)
}

// indexField derives the blind index over the lookup value. A nil cipher
// leaves the index column empty.
func (s *Store) indexField(value string) string {
	if s.cipher == nil {
		return ""
	}
	return s.cipher.Index(value)
}

// Insert persists one submitted payment record: the storage identity and
// timestamps fill the struct before return.
func (s *Store) Insert(ctx context.Context, payment *paymentsdomain.Payment) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	reference, err := s.encryptField(payment.Reference)
	if err != nil {
		return 0, fmt.Errorf("failed to encrypt payment reference: %w", err)
	}
	verifiedBy, err := s.encryptField(payment.VerifiedBy)
	if err != nil {
		return 0, fmt.Errorf("failed to encrypt payment verifier: %w", err)
	}
	referenceIdx := s.indexField(payment.Reference)

	next := s.placeholders()
	query := `
		INSERT INTO payments_payment (order_id, method, status, reference, amount, verified_by, reference_idx)
		VALUES (` + next() + `, ` + next() + `, ` + next() + `, ` + next() + `, ` + next() + `, ` + next() + `, ` + next() + `)
	`
	result, err := s.db.ExecContext(
		ctx,
		query,
		payment.OrderID,
		string(payment.Method),
		string(payment.Status),
		reference,
		payment.Amount,
		verifiedBy,
		referenceIdx,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert payment: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to read payment identity: %w", err)
	}
	return id, nil
}

// FindByID returns the payment record, or
// paymentsdomain.ErrPaymentNotFound.
func (s *Store) FindByID(ctx context.Context, id int64) (*paymentsdomain.Payment, error) {
	return s.findPayment(ctx, "id", id)
}

// findPayment scans one payment row over the given single-value predicate
// query, so FindByID and FindByReference share the read shape. With a
// cipher the reference predicate resolves through the blind-index column
// (randomized ciphertext cannot be queried); without one it matches the
// plaintext column directly.
func (s *Store) findPayment(ctx context.Context, predicate string, value any) (*paymentsdomain.Payment, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if s.cipher != nil && predicate == "reference" {
		if reference, ok := value.(string); ok {
			predicate = "reference_idx"
			value = s.indexField(reference)
		}
	}

	next := s.placeholders()
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT `+paymentColumns+`
		FROM payments_payment
		WHERE `+predicate+` = `+next(),
		value,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find payment: %w", err)
	}
	defer func() { _ = rows.Close() }()

	payment := &paymentsdomain.Payment{}
	if rows.Next() {
		if err := s.scanPayment(rows.Scan, payment); err != nil {
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}
	} else if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to find payment: %w", err)
	} else {
		return nil, paymentsdomain.ErrPaymentNotFound
	}
	return payment, rows.Err()
}

// List returns the newest payment records carrying the given status (nil
// is any), strictly below the keyset boundary (0 means from the newest),
// at most limit rows.
func (s *Store) List(ctx context.Context, status *paymentsdomain.Status, beforeID int64, limit int) ([]paymentsdomain.Payment, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	next := s.placeholders()
	conditions := make([]string, 0, 2)
	args := make([]any, 0, 3)
	if status != nil {
		conditions = append(conditions, "status = "+next())
		args = append(args, string(*status))
	}
	if beforeID > 0 {
		conditions = append(conditions, "id < "+next())
		args = append(args, beforeID)
	}

	var query strings.Builder
	query.WriteString(`SELECT ` + paymentColumns + ` FROM payments_payment`)
	if len(conditions) > 0 {
		query.WriteString(" WHERE " + strings.Join(conditions, " AND "))
	}
	query.WriteString(" ORDER BY id DESC LIMIT " + next())
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list payments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]paymentsdomain.Payment, 0, limit)
	for rows.Next() {
		payment := paymentsdomain.Payment{}
		if err := s.scanPayment(rows.Scan, &payment); err != nil {
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}
		out = append(out, payment)
	}
	return out, rows.Err()
}

// ListByOrder returns every record paying one order, newest first. Order
// histories are short — a handful of proofs and at most one gateway
// row — so the read carries no pagination.
func (s *Store) ListByOrder(ctx context.Context, orderID int64) ([]paymentsdomain.Payment, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	next := s.placeholders()
	rows, err := s.db.QueryContext(ctx, `SELECT `+paymentColumns+` FROM payments_payment WHERE order_id = `+next()+` ORDER BY id DESC`, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to list order payments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]paymentsdomain.Payment, 0, 4)
	for rows.Next() {
		payment := paymentsdomain.Payment{}
		if err := s.scanPayment(rows.Scan, &payment); err != nil {
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}
		out = append(out, payment)
	}
	return out, rows.Err()
}

// SetStatus flips the record to the given status, recording who decided
// it. An unknown record reads as paymentsdomain.ErrPaymentNotFound.
func (s *Store) SetStatus(ctx context.Context, id int64, status paymentsdomain.Status, verifiedBy string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	sealed, err := s.encryptField(verifiedBy)
	if err != nil {
		return fmt.Errorf("failed to encrypt payment verifier: %w", err)
	}

	next := s.placeholders()
	result, err := s.db.ExecContext(ctx, `
		UPDATE payments_payment
		SET status = `+next()+`, verified_by = `+next()+`, updated_at = CURRENT_TIMESTAMP
		WHERE id = `+next(),
		string(status), sealed, id)
	if err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read payment status update: %w", err)
	}
	if affected == 0 {
		return paymentsdomain.ErrPaymentNotFound
	}
	return nil
}

// FindByReference returns the record carrying the gateway transaction
// reference, or paymentsdomain.ErrPaymentNotFound. An empty reference
// never matches: empty plaintext leaves the blind-index column empty, so
// querying it could hit an unrelated empty-index row.
func (s *Store) FindByReference(ctx context.Context, reference string) (*paymentsdomain.Payment, error) {
	if reference == "" {
		return nil, paymentsdomain.ErrPaymentNotFound
	}
	return s.findPayment(ctx, "reference", reference)
}

// EncryptPending encrypts up to limit legacy plaintext rows in place and
// fills their blind-index columns, returning how many rows changed. The
// startup backfill sweep calls it in batches until it reports zero: a
// second boot is a no-op because enveloped rows no longer match the
// sweep predicate. A nil cipher encrypts nothing and reports zero. Fields
// already carrying the envelope are left untouched, so concurrent sweeps
// converge instead of double-encrypting.
func (s *Store) EncryptPending(ctx context.Context, limit int) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s.cipher == nil || limit <= 0 {
		return 0, nil
	}

	pattern := pii.Prefix + "%"
	next := s.placeholders()
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, reference, verified_by
		FROM payments_payment
		WHERE (reference <> `+next()+` AND reference NOT LIKE `+next()+`)
		   OR (verified_by <> `+next()+` AND verified_by NOT LIKE `+next()+`)
		ORDER BY id
		LIMIT `+next()+`
	`, "", pattern, "", pattern, limit)
	if err != nil {
		return 0, fmt.Errorf("failed to read legacy payments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type legacyRow struct {
		id         int64
		reference  string
		verifiedBy string
	}
	pending := make([]legacyRow, 0, limit)
	for rows.Next() {
		var row legacyRow
		if err := rows.Scan(&row.id, &row.reference, &row.verifiedBy); err != nil {
			return 0, fmt.Errorf("failed to scan legacy payment: %w", err)
		}
		pending = append(pending, row)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("failed iterating legacy payments: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("failed closing legacy payments: %w", err)
	}

	encrypted := 0
	for _, row := range pending {
		plainReference, err := s.decryptField(row.reference)
		if err != nil {
			return 0, fmt.Errorf("failed to encrypt legacy payment %d: %w", row.id, err)
		}
		reference, changed, err := s.encryptLegacy(row.reference)
		if err != nil {
			return 0, fmt.Errorf("failed to encrypt legacy payment %d: %w", row.id, err)
		}
		changedReference := changed
		verifiedBy, changed, err := s.encryptLegacy(row.verifiedBy)
		if err != nil {
			return 0, fmt.Errorf("failed to encrypt legacy payment %d: %w", row.id, err)
		}
		if !changedReference && !changed {
			continue
		}
		write := s.placeholders()
		if _, err := s.db.ExecContext(ctx, `
			UPDATE payments_payment
			SET reference = `+write()+`, verified_by = `+write()+`, reference_idx = `+write()+`, updated_at = CURRENT_TIMESTAMP
			WHERE id = `+write()+`
		`, reference, verifiedBy, s.indexField(plainReference), row.id); err != nil {
			return 0, fmt.Errorf("failed to encrypt legacy payment %d: %w", row.id, err)
		}
		encrypted++
	}
	return encrypted, nil
}

// encryptLegacy seals one legacy field when it still needs it: empty and
// already-enveloped values pass through with changed=false.
func (s *Store) encryptLegacy(stored string) (string, bool, error) {
	if stored == "" || pii.IsEncrypted(stored) {
		return stored, false, nil
	}
	sealed, err := s.encryptField(stored)
	if err != nil {
		return "", false, err
	}
	return sealed, true, nil
}

// New builds the store over an open database, its dialect, and the PII
// cipher. A nil cipher stores and reads plaintext (legacy mode); a set
// cipher seals the reference and verifier identity on write and resolves
// webhook dedup through the blind-index column.
func New(db *sql.DB, dialect kerneldatabase.Dialect, piiCipher *pii.Cipher) *Store {
	return &Store{db: db, dialect: dialect, cipher: piiCipher}
}
