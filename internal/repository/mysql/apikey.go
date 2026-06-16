package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/apikey"
)

// APIKeyRepository persists API keys using MySQL syntax.
// Implements apikey.Repository.
type APIKeyRepository struct {
	db *sql.DB
}

// scanAPIKey maps a single row's columns into a domain *apikey.APIKey, using
// sql.NullTime/sql.NullString for nullable fields. It accepts the Scan method
// value directly (*sql.Row.Scan or *sql.Rows.Scan) so no mockable interface is
// introduced. Shared by FindByIDAndUserID (QueryRow) and ListByUserID (Rows).
func scanAPIKey(scan func(dest ...any) error) (*apikey.APIKey, error) {
	var (
		key        apikey.APIKey
		lastUsedAt sql.NullTime
		lastUsedIP sql.NullString
		expiresAt  sql.NullTime
		revokedAt  sql.NullTime
	)
	if err := scan(
		&key.ID,
		&key.UserID,
		&key.Name,
		&key.KeyID,
		&key.KeyHash,
		&lastUsedAt,
		&lastUsedIP,
		&expiresAt,
		&key.CreatedAt,
		&revokedAt,
	); err != nil {
		return nil, err
	}
	if lastUsedAt.Valid {
		key.LastUsedAt = &lastUsedAt.Time
	}
	key.LastUsedIP = lastUsedIP.String
	if expiresAt.Valid {
		key.ExpiresAt = &expiresAt.Time
	}
	if revokedAt.Valid {
		key.RevokedAt = &revokedAt.Time
	}
	return &key, nil
}

// Create inserts a new API key record and populates key.ID with the assigned auto-increment ID.
func (r *APIKeyRepository) Create(ctx context.Context, key *apikey.APIKey) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO api_keys (user_id, name, key_id, key_hash, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, key.UserID, key.Name, key.KeyID, key.KeyHash, key.CreatedAt.UTC())
	if err != nil {
		if isMySQLDuplicateError(err) {
			return apikey.ErrUniqueConstraint
		}
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	key.ID = int(id)
	return nil
}

// CountByUserAndName returns the number of non-revoked keys for a user with the
// given name. Used by the domain service to prevent duplicate key names.
func (r *APIKeyRepository) CountByUserAndName(ctx context.Context, userID int, name string) (int, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM api_keys
		WHERE user_id = ? AND name = ? AND revoked_at IS NULL
	`, userID, name).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// FindByIDAndUserID returns the single key matching (id, userID), or
// apikey.ErrKeyNotFound when no such row exists (covers nonexistent AND
// not-owned — identity scoping is enforced by the WHERE user_id predicate).
func (r *APIKeyRepository) FindByIDAndUserID(ctx context.Context, id, userID int) (*apikey.APIKey, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database connection lost: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, key_id, key_hash, last_used_at, last_used_ip, expires_at, created_at, revoked_at
		FROM api_keys
		WHERE id = ? AND user_id = ?
	`, id, userID)
	key, err := scanAPIKey(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apikey.ErrKeyNotFound
		}
		return nil, fmt.Errorf("failed to find api key by id and user: %w", err)
	}
	return key, nil
}

// ListByUserID returns all API keys owned by userID, newest-first. Returns an
// empty (non-nil) slice when the user has no keys.
func (r *APIKeyRepository) ListByUserID(ctx context.Context, userID int) ([]*apikey.APIKey, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database connection lost: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, name, key_id, key_hash, last_used_at, last_used_ip, expires_at, created_at, revoked_at
		FROM api_keys
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list api keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	keys := []*apikey.APIKey{}
	for rows.Next() {
		key, scanErr := scanAPIKey(rows.Scan)
		if scanErr != nil {
			return nil, fmt.Errorf("failed to scan api key: %w", scanErr)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating api key rows: %w", err)
	}
	return keys, nil
}

// RevokeByIDAndUserID sets revoked_at = now for the matching (id, userID)
// row. No rows-affected check is performed — the Service guards via
// FindByIDAndUserID first.
func (r *APIKeyRepository) RevokeByIDAndUserID(ctx context.Context, id, userID int, now time.Time) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, `
		UPDATE api_keys SET revoked_at = ? WHERE id = ? AND user_id = ?
	`, now.UTC(), id, userID); err != nil {
		return fmt.Errorf("failed to revoke api key: %w", err)
	}
	return nil
}

// FindByKeyID returns the key matching the public lookup prefix keyID, INCLUDING
// revoked and expired rows, or apikey.ErrKeyNotFound if none. The deliberate
// absence of a revoked_at IS NULL filter lets the Service distinguish a
// verified-but-revoked key from an unknown keyID.
func (r *APIKeyRepository) FindByKeyID(ctx context.Context, keyID string) (*apikey.APIKey, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	if err := r.db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database connection lost: %w", err)
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, key_id, key_hash, last_used_at, last_used_ip, expires_at, created_at, revoked_at
		FROM api_keys
		WHERE key_id = ?
	`, keyID)
	key, err := scanAPIKey(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apikey.ErrKeyNotFound
		}
		return nil, fmt.Errorf("failed to find api key by key id: %w", err)
	}
	return key, nil
}

// UpdateLastUsed sets last_used_at = now and last_used_ip = ip for the key id.
// Best-effort: no rows-affected check (the key was just verified to exist).
func (r *APIKeyRepository) UpdateLastUsed(ctx context.Context, id int, now time.Time, ip string) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
		UPDATE api_keys SET last_used_at = ?, last_used_ip = ? WHERE id = ?
	`, now.UTC(), ip, id); err != nil {
		return fmt.Errorf("failed to update api key last used: %w", err)
	}
	return nil
}

// NewAPIKeyRepository creates a new MySQL API key repository.
func NewAPIKeyRepository(db *sql.DB) *APIKeyRepository {
	return &APIKeyRepository{
		db: db,
	}
}
