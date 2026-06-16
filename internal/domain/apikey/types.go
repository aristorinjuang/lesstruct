package apikey

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// KeyPrefix is the mandatory recognizable prefix of every issued key string.
	KeyPrefix = "lesstruct_"
	keyIDLen  = 12 // hex chars (6 random bytes)
	secretLen = 32 // hex chars (16 random bytes, 128 bits)
	maxNameLen = 120
)

var (
	// ErrInvalidKeyName is returned when key name validation fails.
	ErrInvalidKeyName = errors.New("key name is required and must be between 1 and 120 characters")
	// ErrDuplicateKeyName is returned when an API key with the same name already exists for the user.
	ErrDuplicateKeyName = errors.New("an API key with this name already exists")
	// ErrKeyGeneration is returned when secure random key material cannot be generated.
	ErrKeyGeneration = errors.New("failed to generate API key material")
	// ErrUniqueConstraint is returned by repositories when an insert violates a
	// UNIQUE constraint. The service uses it to disambiguate a concurrent
	// duplicate-name insert from a key_id collision.
	ErrUniqueConstraint = errors.New("api key unique constraint violation")
	// ErrKeyNotFound is returned when no API key matches the given id for the
	// user (covers both nonexistent and not-owned — safe-disclosure: identical
	// 404 either way).
	ErrKeyNotFound = errors.New("api key not found")
	// ErrMalformedKey is returned by Verify when the presented key string is not a
	// valid lesstruct_ key (bad prefix, wrong segment structure, or wrong keyID/secret
	// length). The middleware maps it to 401 INVALID_API_KEY.
	ErrMalformedKey = errors.New("malformed api key")
	// ErrKeyRevoked is returned by Verify when the key exists and the secret matches
	// but the key has been revoked (revoked_at is set). Maps to 401 REVOKED_KEY.
	ErrKeyRevoked = errors.New("api key has been revoked")
	// ErrKeyExpired is returned by Verify when the key exists and the secret matches
	// but the key's expires_at is in the past. Maps to 401 EXPIRED_KEY.
	ErrKeyExpired = errors.New("api key has expired")
)

// APIKey is the persisted API key entity. The secret is NEVER stored and is
// therefore intentionally absent from this struct.
type APIKey struct {
	ID         int        `json:"id"`
	UserID     int        `json:"userId"`
	Name       string     `json:"name"`
	KeyID      string     `json:"keyId"`
	KeyHash    string     `json:"-"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	LastUsedIP string     `json:"lastUsedIp,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
}

// ValidateKeyName validates a key label (package-level, like content.ValidateTitle).
func ValidateKeyName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxNameLen {
		return ErrInvalidKeyName
	}
	return nil
}

// DisplayPrefix returns the masked display form "lesstruct_<keyID>••••" for
// safe display in list views. The secret is never part of this string. The
// full keyID (12 hex chars) is shown because it is the public lookup prefix,
// not a secret.
func DisplayPrefix(keyID string) string {
	return KeyPrefix + keyID + "••••"
}
