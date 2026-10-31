// Package store is the single-source order repository: one implementation
// against *sql.DB plus a kernel Dialect, following the cart/media store
// pattern. Placeholder rendering and RETURNING support come from the
// dialect. Variant IDs map nil (base-product line) to 0 at the boundary so
// the schema behaves identically on every driver, and the shipping address
// round-trips as a JSON snapshot.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
	"github.com/aristorinjuang/lesstruct/kernel/legal"
	"github.com/aristorinjuang/lesstruct/kernel/pii"
	ordersdomain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
)

// noVariantID is the stored variant key for base-product lines.
const (
	noVariantID = 0
)

// variantKey renders the stored variant key: 0 for base-product lines.
func variantKey(variantID *int64) int64 {
	if variantID == nil {
		return noVariantID
	}
	return *variantID
}

// marshalShipTo freezes the shipping address into its stored JSON shape.
func marshalShipTo(address ordersdomain.Address) (string, error) {
	payload, err := json.Marshal(address)
	if err != nil {
		return "", fmt.Errorf("failed to marshal shipping address: %w", err)
	}
	return string(payload), nil
}

// unmarshalShipTo rebuilds the shipping address. The store wrote the JSON
// itself (marshalShipTo), so malformed payloads indicate tampering and read
// back as an empty address instead of failing every order read.
func unmarshalShipTo(payload string) ordersdomain.Address {
	var address ordersdomain.Address
	_ = json.Unmarshal([]byte(payload), &address)
	return address
}

// marshalConsent freezes the consent evidence into its stored shape. A nil
// consent stores NULL — the order predates consent enforcement or the store
// does not ask for it, and there is nothing to keep.
func marshalConsent(consent *legal.Consent) (sql.NullTime, sql.NullString, error) {
	if consent == nil {
		return sql.NullTime{}, sql.NullString{}, nil
	}
	payload, err := json.Marshal(consent)
	if err != nil {
		return sql.NullTime{}, sql.NullString{}, fmt.Errorf("failed to marshal consent evidence: %w", err)
	}
	return sql.NullTime{Time: consent.AcceptedAt, Valid: true}, sql.NullString{String: string(payload), Valid: true}, nil
}

// unmarshalConsent rebuilds the consent evidence. The store wrote the JSON
// itself, so a malformed payload indicates tampering and fails the read
// instead of presenting wrong evidence.
func unmarshalConsent(acceptedAt sql.NullTime, evidence sql.NullString) (*legal.Consent, error) {
	if !evidence.Valid || evidence.String == "" {
		return nil, nil
	}
	var consent legal.Consent
	if err := json.Unmarshal([]byte(evidence.String), &consent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal consent evidence: %w", err)
	}
	if acceptedAt.Valid {
		consent.AcceptedAt = acceptedAt.Time
	}
	return &consent, nil
}

// Store is the order single-source store. PII fields (customer email,
// name, phone, shipping snapshot) are encrypted at this boundary: writes
// seal them through the cipher and reads open them, while guest tracking
// resolves through the blind-index column. A nil cipher stores and reads
// plaintext (legacy mode); the startup sweep encrypts those rows once a
// key is provisioned.
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

// indexField derives the blind index over the normalized lookup value. A
// nil cipher leaves the index column empty.
func (s *Store) indexField(normalized string) string {
	if s.cipher == nil {
		return ""
	}
	return s.cipher.Index(normalized)
}

// scanOrder fills an order from one row of the shared column list,
// opening the PII fields through the cipher and rebuilding the
// shipping-address snapshot.
func (s *Store) scanOrder(scan func(dest ...any) error, order *ordersdomain.Order) error {
	var email, name, phone, shipTo string
	var consentAt sql.NullTime
	var consentEvidence sql.NullString
	if err := scan(
		&order.ID,
		&order.Status,
		&email,
		&name,
		&phone,
		&shipTo,
		&order.Currency,
		&order.Subtotal,
		&order.ShippingMinor,
		&order.CreatedAt,
		&order.UpdatedAt,
		&consentAt,
		&consentEvidence,
	); err != nil {
		return err
	}
	decrypted, err := s.decryptField(email)
	if err != nil {
		return fmt.Errorf("failed to decrypt order email: %w", err)
	}
	order.CustomerEmail = decrypted
	if decrypted, err = s.decryptField(name); err != nil {
		return fmt.Errorf("failed to decrypt order name: %w", err)
	}
	order.CustomerName = decrypted
	if decrypted, err = s.decryptField(phone); err != nil {
		return fmt.Errorf("failed to decrypt order phone: %w", err)
	}
	order.CustomerPhone = decrypted
	shipToPlain, err := s.decryptField(shipTo)
	if err != nil {
		return fmt.Errorf("failed to decrypt order shipping address: %w", err)
	}
	order.ShipTo = unmarshalShipTo(shipToPlain)
	consent, err := unmarshalConsent(consentAt, consentEvidence)
	if err != nil {
		return err
	}
	order.Consent = consent
	return nil
}

// CreateOrder persists the order and its item snapshot in one transaction:
// the order row inserts first for the generated ID, then every item row
// carries it. Storage identities and timestamps fill the struct before
// return.
func (s *Store) CreateOrder(ctx context.Context, order *ordersdomain.Order) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	shipTo, err := marshalShipTo(order.ShipTo)
	if err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin order transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.insertOrder(ctx, tx, order, shipTo); err != nil {
		return 0, err
	}
	if err := s.insertOrderItems(ctx, tx, order); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit order transaction: %w", err)
	}
	return order.ID, nil
}

// FindByID returns the order with its items, or
// ordersdomain.ErrOrderNotFound.
func (s *Store) FindByID(ctx context.Context, id int64) (*ordersdomain.Order, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Admin reads skip the guest gate: an empty token hash carries no
	// predicate. Guest lookups always pass a hash (the service refuses
	// empty tokens before hashing, and a hash is never empty).
	return s.findOrder(ctx, id, "", "")
}

// FindByNumberAndEmailAndToken returns the order with its items when the
// order ID, the customer email, and the tracking-token hash all match, or
// ordersdomain.ErrOrderNotFound. Matching is exact: the service
// normalizes the presented email and hashes the presented token the way
// checkout stored them.
func (s *Store) FindByNumberAndEmailAndToken(ctx context.Context, id int64, email string, tokenHash string) (*ordersdomain.Order, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.findOrder(ctx, id, email, tokenHash)
}

// List returns the newest orders carrying the given status (nil is any),
// strictly below the keyset boundary (0 means from the newest), at most
// limit rows. Items are not loaded; the per-order line count rides the
// read-time COUNT aggregate into ItemCount.
func (s *Store) List(ctx context.Context, status *ordersdomain.Status, beforeID int64, limit int) ([]ordersdomain.Order, error) {
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
		conditions = append(conditions, "orders_order.id < "+next())
		args = append(args, beforeID)
	}

	var query strings.Builder
	query.WriteString(`
		SELECT orders_order.id, orders_order.status, orders_order.customer_email, orders_order.customer_name, orders_order.customer_phone, orders_order.ship_to, orders_order.currency, orders_order.subtotal, orders_order.shipping_minor, orders_order.created_at, orders_order.updated_at, orders_order.consent_at, orders_order.consent_evidence,
			(SELECT COUNT(*) FROM orders_order_items WHERE orders_order_items.order_id = orders_order.id)
		FROM orders_order
	`)
	if len(conditions) > 0 {
		query.WriteString("WHERE ")
		query.WriteString(strings.Join(conditions, " AND "))
	}
	query.WriteString("ORDER BY orders_order.id DESC LIMIT ")
	query.WriteString(next())
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}
	defer func() { _ = rows.Close() }()

	orders := make([]ordersdomain.Order, 0, limit)
	for rows.Next() {
		var listed ordersdomain.Order
		var itemCount int64
		var email, name, phone, shipTo string
		var consentAt sql.NullTime
		var consentEvidence sql.NullString
		if err := rows.Scan(
			&listed.ID,
			&listed.Status,
			&email,
			&name,
			&phone,
			&shipTo,
			&listed.Currency,
			&listed.Subtotal,
			&listed.ShippingMinor,
			&listed.CreatedAt,
			&listed.UpdatedAt,
			&consentAt,
			&consentEvidence,
			&itemCount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		if decrypted, err := s.decryptField(email); err != nil {
			return nil, fmt.Errorf("failed to decrypt order email: %w", err)
		} else {
			listed.CustomerEmail = decrypted
		}
		if decrypted, err := s.decryptField(name); err != nil {
			return nil, fmt.Errorf("failed to decrypt order name: %w", err)
		} else {
			listed.CustomerName = decrypted
		}
		if decrypted, err := s.decryptField(phone); err != nil {
			return nil, fmt.Errorf("failed to decrypt order phone: %w", err)
		} else {
			listed.CustomerPhone = decrypted
		}
		shipToPlain, err := s.decryptField(shipTo)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt order shipping address: %w", err)
		}
		listed.ShipTo = unmarshalShipTo(shipToPlain)
		consent, err := unmarshalConsent(consentAt, consentEvidence)
		if err != nil {
			return nil, err
		}
		listed.Consent = consent
		listed.ItemCount = int(itemCount)
		orders = append(orders, listed)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating orders: %w", err)
	}
	return orders, nil
}

// SetStatus replaces the lifecycle status of the order, touching
// updated_at. An unknown order reads as ordersdomain.ErrOrderNotFound.
func (s *Store) SetStatus(ctx context.Context, id int64, status ordersdomain.Status) error {
	if ctx == nil {
		ctx = context.Background()
	}

	next := s.placeholders()
	result, err := s.db.ExecContext(ctx, `
		UPDATE orders_order
		SET status = `+next()+`, updated_at = CURRENT_TIMESTAMP
		WHERE id = `+next()+`
	`, string(status), id)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read order status update: %w", err)
	}
	if affected == 0 {
		return ordersdomain.ErrOrderNotFound
	}
	return nil
}

// MarkPaid flips a pending order to paid and reports whether this call
// did the flip. The pending guard lives in the statement, so racing
// confirms, retried webhooks, and the expiry sweep serialize on it — the
// loser reads zero rows.
func (s *Store) MarkPaid(ctx context.Context, id int64) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	next := s.placeholders()
	result, err := s.db.ExecContext(ctx, `
		UPDATE orders_order
		SET status = `+next()+`, updated_at = CURRENT_TIMESTAMP
		WHERE id = `+next()+` AND status = `+next()+`
	`, string(ordersdomain.StatusPaid), id, string(ordersdomain.StatusPending))
	if err != nil {
		return false, fmt.Errorf("failed to mark order paid: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to read paid flip: %w", err)
	}
	return affected > 0, nil
}

// ExpirePending flips up to limit pending orders created before the
// boundary to expired and returns the flipped IDs. The candidate list
// reads inside the transaction and every flip carries the pending guard,
// so a payment that lands mid-sweep keeps its win.
func (s *Store) ExpirePending(ctx context.Context, before time.Time, limit int) ([]int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin expiry sweep: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	selectNext := s.placeholders()
	rows, err := tx.QueryContext(ctx, `
		SELECT id
		FROM orders_order
		WHERE status = `+selectNext()+` AND created_at < `+selectNext()+`
		ORDER BY id
		LIMIT `+selectNext()+`
	`, string(ordersdomain.StatusPending), before, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to read stale pending orders: %w", err)
	}

	candidates := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan stale pending order: %w", err)
		}
		candidates = append(candidates, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating stale pending orders: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("failed closing stale pending orders: %w", err)
	}

	// One statement shape for every flip: its own three placeholders,
	// rendered once, reused across the loop.
	flipNext := s.placeholders()
	flipSQL := `
		UPDATE orders_order
		SET status = ` + flipNext() + `, updated_at = CURRENT_TIMESTAMP
		WHERE id = ` + flipNext() + ` AND status = ` + flipNext() + ``
	flipped := make([]int64, 0, len(candidates))
	for _, id := range candidates {
		result, err := tx.ExecContext(ctx, flipSQL,
			string(ordersdomain.StatusExpired), id, string(ordersdomain.StatusPending))
		if err != nil {
			return nil, fmt.Errorf("failed to expire order %d: %w", id, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("failed to read expiry flip for order %d: %w", id, err)
		}
		if affected > 0 {
			flipped = append(flipped, id)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit expiry sweep: %w", err)
	}
	return flipped, nil
}

// SetShipping replaces the shipping line of a pending order and reports
// whether this call did the write. The pending guard lives in the
// statement, mirroring MarkPaid: a status flip racing the write loses
// it, and the service reads the loss as a refused transition.
func (s *Store) SetShipping(ctx context.Context, id int64, amount int64) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	next := s.placeholders()
	result, err := s.db.ExecContext(ctx, `
		UPDATE orders_order
		SET shipping_minor = `+next()+`, updated_at = CURRENT_TIMESTAMP
		WHERE id = `+next()+` AND status = `+next()+`
	`, amount, id, string(ordersdomain.StatusPending))
	if err != nil {
		return false, fmt.Errorf("failed to update order shipping: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to read shipping write: %w", err)
	}
	return affected > 0, nil
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
		SELECT id, customer_email, customer_name, customer_phone, ship_to
		FROM orders_order
		WHERE (customer_email <> `+next()+` AND customer_email NOT LIKE `+next()+`)
		   OR (customer_name <> `+next()+` AND customer_name NOT LIKE `+next()+`)
		   OR (customer_phone <> `+next()+` AND customer_phone NOT LIKE `+next()+`)
		   OR (ship_to <> `+next()+` AND ship_to NOT LIKE `+next()+`)
		ORDER BY id
		LIMIT `+next()+`
	`, "", pattern, "", pattern, "", pattern, "", pattern, limit)
	if err != nil {
		return 0, fmt.Errorf("failed to read legacy orders: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type legacyRow struct {
		id    int64
		email string
		name  string
		phone string
		ship  string
	}
	pending := make([]legacyRow, 0, limit)
	for rows.Next() {
		var row legacyRow
		if err := rows.Scan(&row.id, &row.email, &row.name, &row.phone, &row.ship); err != nil {
			return 0, fmt.Errorf("failed to scan legacy order: %w", err)
		}
		pending = append(pending, row)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("failed iterating legacy orders: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("failed closing legacy orders: %w", err)
	}

	encrypted := 0
	for _, row := range pending {
		plainEmail, err := s.decryptField(row.email)
		if err != nil {
			return 0, fmt.Errorf("failed to encrypt legacy order %d: %w", row.id, err)
		}
		email, changed, err := s.encryptLegacy(row.email)
		if err != nil {
			return 0, fmt.Errorf("failed to encrypt legacy order %d: %w", row.id, err)
		}
		changedEmail := changed
		name, changed, err := s.encryptLegacy(row.name)
		if err != nil {
			return 0, fmt.Errorf("failed to encrypt legacy order %d: %w", row.id, err)
		}
		changedName := changed
		phone, changed, err := s.encryptLegacy(row.phone)
		if err != nil {
			return 0, fmt.Errorf("failed to encrypt legacy order %d: %w", row.id, err)
		}
		changedPhone := changed
		ship, changed, err := s.encryptLegacy(row.ship)
		if err != nil {
			return 0, fmt.Errorf("failed to encrypt legacy order %d: %w", row.id, err)
		}
		if !changedEmail && !changedName && !changedPhone && !changed {
			continue
		}
		write := s.placeholders()
		if _, err := s.db.ExecContext(ctx, `
			UPDATE orders_order
			SET customer_email = `+write()+`, customer_name = `+write()+`, customer_phone = `+write()+`,
				ship_to = `+write()+`, customer_email_idx = `+write()+`, updated_at = CURRENT_TIMESTAMP
			WHERE id = `+write()+`
		`, email, name, phone, ship, s.indexField(plainEmail), row.id); err != nil {
			return 0, fmt.Errorf("failed to encrypt legacy order %d: %w", row.id, err)
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

// insertOrder writes the order row inside the transaction, filling the
// generated ID and storage timestamps. The RETURNING branch serves
// PostgreSQL and SQLite; MySQL falls back to LastInsertId plus a fresh
// read.
func (s *Store) insertOrder(
	ctx context.Context,
	tx *sql.Tx,
	order *ordersdomain.Order,
	shipTo string,
) error {
	email, err := s.encryptField(order.CustomerEmail)
	if err != nil {
		return fmt.Errorf("failed to encrypt order email: %w", err)
	}
	name, err := s.encryptField(order.CustomerName)
	if err != nil {
		return fmt.Errorf("failed to encrypt order name: %w", err)
	}
	phone, err := s.encryptField(order.CustomerPhone)
	if err != nil {
		return fmt.Errorf("failed to encrypt order phone: %w", err)
	}
	ship, err := s.encryptField(shipTo)
	if err != nil {
		return fmt.Errorf("failed to encrypt order shipping address: %w", err)
	}
	emailIdx := s.indexField(order.CustomerEmail)
	consentAt, consentEvidence, err := marshalConsent(order.Consent)
	if err != nil {
		return err
	}

	next := s.placeholders()
	if s.dialect.SupportsReturning() {
		err := tx.QueryRowContext(ctx, `
			INSERT INTO orders_order
				(status, customer_email, customer_name, customer_phone, ship_to, customer_email_idx, currency, subtotal, shipping_minor, consent_at, consent_evidence, tracking_token_hash)
			VALUES (`+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`)
			RETURNING id, created_at, updated_at
		`,
			string(order.Status),
			email,
			name,
			phone,
			ship,
			emailIdx,
			order.Currency,
			order.Subtotal,
			order.ShippingMinor,
			consentAt,
			consentEvidence,
			order.TrackingTokenHash,
		).Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)
		if err != nil {
			return fmt.Errorf("failed to insert order: %w", err)
		}
		return nil
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO orders_order
			(status, customer_email, customer_name, customer_phone, ship_to, customer_email_idx, currency, subtotal, shipping_minor, consent_at, consent_evidence, tracking_token_hash)
		VALUES (`+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`)
	`,
		string(order.Status),
		email,
		name,
		phone,
		ship,
		emailIdx,
		order.Currency,
		order.Subtotal,
		order.ShippingMinor,
		consentAt,
		consentEvidence,
		order.TrackingTokenHash,
	)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to read order id: %w", err)
	}
	order.ID = id

	times := s.placeholders()
	if err := tx.QueryRowContext(ctx, `
		SELECT created_at, updated_at FROM orders_order WHERE id = `+times()+`
	`, id).Scan(&order.CreatedAt, &order.UpdatedAt); err != nil {
		return fmt.Errorf("failed to read order timestamps: %w", err)
	}
	return nil
}

// insertOrderItems writes every snapshot line inside the transaction,
// filling item identities. Each insert renders its own placeholders: one
// statement at a time, positional from zero.
func (s *Store) insertOrderItems(ctx context.Context, tx *sql.Tx, order *ordersdomain.Order) error {
	for i := range order.Items {
		item := &order.Items[i]
		storedVariant := variantKey(item.VariantID)
		item.OrderID = order.ID

		next := s.placeholders()
		if s.dialect.SupportsReturning() {
			err := tx.QueryRowContext(ctx, `
				INSERT INTO orders_order_items
					(order_id, product_content_id, variant_id, sku, title, unit_price, quantity)
				VALUES (`+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`)
				RETURNING id
			`,
				item.OrderID,
				item.ProductContentID,
				storedVariant,
				item.SKU,
				item.Title,
				item.UnitPrice,
				item.Quantity,
			).Scan(&item.ID)
			if err != nil {
				return fmt.Errorf("failed to insert order item: %w", err)
			}
			continue
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO orders_order_items
				(order_id, product_content_id, variant_id, sku, title, unit_price, quantity)
			VALUES (`+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`, `+next()+`)
		`,
			item.OrderID,
			item.ProductContentID,
			storedVariant,
			item.SKU,
			item.Title,
			item.UnitPrice,
			item.Quantity,
		)
		if err != nil {
			return fmt.Errorf("failed to insert order item: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to read order item id: %w", err)
		}
		item.ID = id
	}
	return nil
}

// findOrder loads one order row plus its item rows; a non-empty email
// tightens the match. With a cipher the email resolves through the
// blind-index column (randomized ciphertext cannot be queried); without
// one it matches the plaintext column directly. Unknown orders read as
// ordersdomain.ErrOrderNotFound.
func (s *Store) findOrder(ctx context.Context, id int64, email string, tokenHash string) (*ordersdomain.Order, error) {
	next := s.placeholders()
	query := `
		SELECT id, status, customer_email, customer_name, customer_phone, ship_to, currency, subtotal, shipping_minor, created_at, updated_at, consent_at, consent_evidence
		FROM orders_order
		WHERE id = ` + next()
	args := []any{id}
	if email != "" {
		if s.cipher != nil {
			query += " AND customer_email_idx = " + next()
			args = append(args, s.indexField(email))
		} else {
			query += " AND customer_email = " + next()
			args = append(args, email)
		}
	}
	// The token hash closes the gate — but only when one is presented:
	// admin reads pass the empty hash and skip the predicate, while the
	// guest path always arrives with a hash because the service refuses
	// empty tokens before hashing. A stored empty hash (a legacy order)
	// therefore can never satisfy a guest lookup.
	if tokenHash != "" {
		query += " AND tracking_token_hash = " + next()
		args = append(args, tokenHash)
	}

	var order ordersdomain.Order
	err := s.scanOrder(s.db.QueryRowContext(ctx, query, args...).Scan, &order)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ordersdomain.ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find order: %w", err)
	}

	if err := s.appendItems(ctx, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

// appendItems loads the order's item snapshot in ID order.
func (s *Store) appendItems(ctx context.Context, order *ordersdomain.Order) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, order_id, product_content_id, variant_id, sku, title, unit_price, quantity
		FROM orders_order_items WHERE order_id = `+s.dialect.Placeholder(0)+`
		ORDER BY id
	`, order.ID)
	if err != nil {
		return fmt.Errorf("failed to list order items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	order.Items = []ordersdomain.OrderItem{}
	for rows.Next() {
		var item ordersdomain.OrderItem
		var variantID int64
		if err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.ProductContentID,
			&variantID,
			&item.SKU,
			&item.Title,
			&item.UnitPrice,
			&item.Quantity,
		); err != nil {
			return fmt.Errorf("failed to scan order item: %w", err)
		}
		if variantID != noVariantID {
			variant := variantID
			item.VariantID = &variant
		}
		order.Items = append(order.Items, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed iterating order items: %w", err)
	}
	order.ItemCount = len(order.Items)
	return nil
}

// New builds the store over an open database, its dialect, and the PII
// cipher. A nil cipher stores and reads plaintext (legacy mode); a set
// cipher seals the customer PII fields on write and resolves guest
// tracking through the blind-index column.
func New(db *sql.DB, dialect kerneldatabase.Dialect, piiCipher *pii.Cipher) *Store {
	return &Store{db: db, dialect: dialect, cipher: piiCipher}
}
