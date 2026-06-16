package apikey

import (
	"context"
	"time"
)

// Repository persists API keys. Implementations live in
// internal/repository/{sqlite,postgresql,mysql}.
type Repository interface {
	// Existing (Story 1.1) — do NOT remove:
	Create(ctx context.Context, key *APIKey) error
	CountByUserAndName(ctx context.Context, userID int, name string) (int, error)

	// New (Story 1.2):
	// FindByIDAndUserID returns the key for (id, userID) or ErrKeyNotFound if
	// no such row exists (covers nonexistent AND not-owned — identity-scoped).
	FindByIDAndUserID(ctx context.Context, id, userID int) (*APIKey, error)
	// ListByUserID returns all keys for userID, newest-first.
	ListByUserID(ctx context.Context, userID int) ([]*APIKey, error)
	// RevokeByIDAndUserID sets revoked_at = now for (id, userID). No-op if the
	// row does not exist (the Service guards via FindByIDAndUserID first).
	RevokeByIDAndUserID(ctx context.Context, id, userID int, now time.Time) error

	// New (Story 1.4):
	// FindByKeyID returns the key matching the public lookup prefix keyID,
	// INCLUDING revoked and expired rows, or ErrKeyNotFound if none. It
	// deliberately does NOT filter revoked_at — the Service must observe
	// revoked/expired rows after the hash compare to return the distinct
	// REVOKED_KEY / EXPIRED_KEY codes rather than INVALID_API_KEY.
	FindByKeyID(ctx context.Context, keyID string) (*APIKey, error)
	// UpdateLastUsed sets last_used_at = now and last_used_ip = ip for the key
	// with the given id. Best-effort by contract: callers log errors and continue.
	// No rows-affected check (the key was just verified to exist).
	UpdateLastUsed(ctx context.Context, id int, now time.Time, ip string) error
}
