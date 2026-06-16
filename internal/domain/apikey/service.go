package apikey

import (
	"context"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// maxCreateAttempts bounds how many times Create regenerates key material before
// giving up on a key_id collision (48-bit keyID space — collisions are
// astronomically rare; this is purely defensive).
const maxCreateAttempts = 3

// Reader is the cryptographic source for key material. It defaults to
// crypto/rand.Reader and is exported so tests can inject a failing or
// deterministic reader (see TestService_Create_RandError).
var Reader io.Reader = cryptorand.Reader

// parseKeyString splits a full key "lesstruct_<keyID>_<secret>" into its keyID
// (12 hex chars) and secret (32 hex chars). It validates the prefix, the segment
// structure, and the fixed lengths — but NOT the hex content (the DB lookup and
// constant-time hash compare reject everything else). It never touches the DB.
func parseKeyString(fullKey string) (keyID string, secret string, err error) {
	if !strings.HasPrefix(fullKey, KeyPrefix) {
		return "", "", errors.New("missing lesstruct_ prefix")
	}
	rest := fullKey[len(KeyPrefix):]
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("api key must contain keyID and secret segments")
	}
	keyID, secret = parts[0], parts[1]
	if len(keyID) != keyIDLen {
		return "", "", fmt.Errorf("invalid keyID length: got %d want %d", len(keyID), keyIDLen)
	}
	if len(secret) != secretLen {
		return "", "", fmt.Errorf("invalid secret length: got %d want %d", len(secret), secretLen)
	}
	return keyID, secret, nil
}

// Service is the sole creator and hasher of API keys. Nothing else touches
// key_hash.
type Service struct {
	repo   Repository
	pepper string // optional; "" disables HMAC
}

// generateKeyMaterial returns (keyID=12 hex chars, secret=32 hex chars).
// keyID is 6 random bytes; secret is 16 random bytes (128 bits of entropy).
func (s *Service) generateKeyMaterial() (keyID string, secret string, err error) {
	combined := make([]byte, (keyIDLen+secretLen)/2) // 22 bytes total
	n, err := Reader.Read(combined)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrKeyGeneration, err)
	}
	if n != len(combined) {
		return "", "", fmt.Errorf("%w: short read from entropy source", ErrKeyGeneration)
	}
	return hex.EncodeToString(combined[:keyIDLen/2]),
		hex.EncodeToString(combined[keyIDLen/2:]),
		nil
}

// hashSecret returns hex(sha256(secret)) when no pepper is configured, or
// hex(hmac-sha256(pepper, secret)) when a pepper is set (defense-in-depth).
// Both produce a 64-char hex string.
func (s *Service) hashSecret(secret string) string {
	if s.pepper == "" {
		sum := sha256.Sum256([]byte(secret))
		return hex.EncodeToString(sum[:])
	}
	mac := hmac.New(sha256.New, []byte(s.pepper))
	_, _ = mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil))
}

// Create generates a new API key for userID, persists the hash, and returns
// the full key string (shown once) plus the persisted entity.
// Format: "lesstruct_<keyID>_<secret>" where keyID=12 hex chars, secret=32 hex chars.
func (s *Service) Create(ctx context.Context, userID int, name string) (string, *APIKey, error) {
	name = strings.TrimSpace(name)
	if err := ValidateKeyName(name); err != nil {
		return "", nil, fmt.Errorf("key name validation failed: %w", err)
	}

	count, err := s.repo.CountByUserAndName(ctx, userID, name)
	if err != nil {
		return "", nil, fmt.Errorf("failed to check duplicate key name: %w", err)
	}
	if count > 0 {
		return "", nil, ErrDuplicateKeyName
	}

	// Insert with a bounded retry on key_id collision (48-bit keyID space). The
	// DB-level UNIQUE(user_id, name) index is the race backstop: if a concurrent
	// request inserted the same name first, the insert fails with
	// ErrUniqueConstraint and the re-check below returns ErrDuplicateKeyName.
	for attempt := 0; attempt < maxCreateAttempts; attempt++ {
		keyID, secret, gerr := s.generateKeyMaterial()
		if gerr != nil {
			return "", nil, gerr
		}

		key := &APIKey{
			UserID:    userID,
			Name:      name,
			KeyID:     keyID,
			KeyHash:   s.hashSecret(secret),
			CreatedAt: time.Now().UTC(),
		}

		if cerr := s.repo.Create(ctx, key); cerr != nil {
			if errors.Is(cerr, ErrUniqueConstraint) {
				// Disambiguate: concurrent duplicate-name insert vs key_id collision.
				rc, rerr := s.repo.CountByUserAndName(ctx, userID, name)
				if rerr == nil && rc > 0 {
					return "", nil, ErrDuplicateKeyName
				}
				continue // assume key_id collision — regenerate and retry
			}
			return "", nil, fmt.Errorf("failed to create api key: %w", cerr)
		}

		return KeyPrefix + keyID + "_" + secret, key, nil
	}
	return "", nil, fmt.Errorf("%w: exhausted %d attempts", ErrKeyGeneration, maxCreateAttempts)
}

// List returns all API keys owned by userID, newest-first. The returned
// entities never contain the secret (it is not persisted); KeyHash is present
// but carries json:"-" so it is excluded from serialization. Handlers must
// still map to a dedicated list DTO to also drop LastUsedIP and apply
// DisplayPrefix.
func (s *Service) List(ctx context.Context, userID int) ([]*APIKey, error) {
	keys, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list api keys: %w", err)
	}
	return keys, nil
}

// Revoke soft-deletes (sets revoked_at) the key (id, userID). It is
// idempotent: revoking an already-revoked key returns the existing record
// without re-writing. Returns ErrKeyNotFound if no key matches (id, userID) —
// this covers both nonexistent and not-owned (cross-user → identical 404,
// safe disclosure).
func (s *Service) Revoke(ctx context.Context, id, userID int) (*APIKey, error) {
	key, err := s.repo.FindByIDAndUserID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("failed to find api key for revoke: %w", err)
	}
	if key.RevokedAt != nil {
		// Already revoked — idempotent success, no DB write (preserves original timestamp).
		return key, nil
	}
	now := time.Now().UTC()
	if err := s.repo.RevokeByIDAndUserID(ctx, id, userID, now); err != nil {
		return nil, fmt.Errorf("failed to revoke api key: %w", err)
	}
	key.RevokedAt = &now
	return key, nil
}

// Verify validates a full key string and returns the owning key on success. It
// parses the structure, looks up the active-or-revoked-or-expired row by keyID,
// constant-time compares hashSecret(secret) to the stored hash, then checks
// revoked_at and expires_at. It performs NO side effects — the middleware calls
// UpdateLastUsed separately after a successful verify.
//
// Error contract (the middleware maps these to 401 codes):
//   - ErrMalformedKey  — not a valid lesstruct_ key (prefix/structure/lengths)
//   - ErrKeyNotFound   — no key with that keyID, OR the secret does not match
//     (safe disclosure: identical INVALID_API_KEY either way)
//   - ErrKeyRevoked    — secret matches but revoked_at is set (key returned non-nil)
//   - ErrKeyExpired    — secret matches but expires_at is in the past (key returned non-nil)
//
// On ErrKeyRevoked / ErrKeyExpired the matching *APIKey is returned alongside
// the error so the middleware can log the keyID for the audit trail. On every
// other error path the key is nil.
func (s *Service) Verify(ctx context.Context, fullKey string) (*APIKey, error) {
	keyID, secret, err := parseKeyString(fullKey)
	if err != nil {
		return nil, ErrMalformedKey
	}

	key, err := s.repo.FindByKeyID(ctx, keyID)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return nil, ErrKeyNotFound
		}
		// Distinct from the repository's own "failed to find api key by key id"
		// wrap so the chained message is not duplicated; the repo message stays
		// in the %w chain for diagnostics.
		return nil, fmt.Errorf("failed to verify api key: %w", err)
	}

	// Constant-time compare of the presented hash to the stored hash. Both are
	// always 64-char lowercase hex, so byte length is constant (no length leak).
	if subtle.ConstantTimeCompare([]byte(s.hashSecret(secret)), []byte(key.KeyHash)) != 1 {
		// Wrong secret — treat as not-found so the response is byte-identical to
		// an unknown keyID (safe disclosure: both → INVALID_API_KEY).
		return nil, ErrKeyNotFound
	}

	if key.RevokedAt != nil {
		return key, ErrKeyRevoked
	}
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now().UTC()) {
		return key, ErrKeyExpired
	}
	return key, nil
}

// UpdateLastUsed records the time and source IP of a successful authentication
// for the key id. Best-effort by contract: the middleware logs errors and
// continues serving the authenticated request. A repo error is wrapped and
// returned for the middleware to log; it is never surfaced to the caller as 401.
func (s *Service) UpdateLastUsed(ctx context.Context, id int, ip string) error {
	now := time.Now().UTC()
	if err := s.repo.UpdateLastUsed(ctx, id, now, ip); err != nil {
		return fmt.Errorf("failed to update last used: %w", err)
	}
	return nil
}

// NewService creates a new API key Service. Pass an empty pepper to disable
// the HMAC defense-in-depth path (plain SHA-256 hashing is used instead).
func NewService(repo Repository, pepper string) *Service {
	return &Service{
		repo:   repo,
		pepper: pepper,
	}
}
