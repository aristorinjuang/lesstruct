package auth

import (
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

// randReader is the package entropy source. Tests replace it to cover the
// failure paths of token and code generation; production keeps crypto/rand.
var randReader io.Reader = cryptorand.Reader

// GenerateToken returns a cryptographically random 32-byte token in hex.
func GenerateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := io.ReadFull(randReader, bytes); err != nil {
		return "", fmt.Errorf("failed to generate secure random token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// HashToken hashes a token using SHA-256
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// VerificationToken represents a verification token in the system
type VerificationToken struct {
	ID        int
	UserID    int
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// PasswordResetToken represents a password reset token in the system
type PasswordResetToken struct {
	ID        int
	UserID    int
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}
