package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrPasswordInvalid = errors.New("password must be at least 12 characters with mixed case, numbers, and special characters")
)

// Argon2id parameters per RFC 9106 / OWASP recommendations
const (
	argon2idMemory  = 64 * 1024 // 64 MiB in KiB
	argon2idTime    = 3         // iterations
	argon2idThreads = 2         // parallelism
	argon2idSaltLen = 16        // bytes
	argon2idKeyLen  = 32        // bytes

	argon2idPrefix = "$argon2id$"
	bcryptPrefix2a = "$2a$"
	bcryptPrefix2b = "$2b$"
	bcryptPrefix2y = "$2y$"
)

// passwordCharSets defines the character sets used for password generation
var passwordCharSets = struct {
	uppercase string
	lowercase string
	digits    string
	special   string
	all       string
}{
	uppercase: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
	lowercase: "abcdefghijklmnopqrstuvwxyz",
	digits:    "0123456789",
	special:   "!@#$%^&*()-_=+",
}

func init() {
	passwordCharSets.all = passwordCharSets.uppercase +
		passwordCharSets.lowercase +
		passwordCharSets.digits +
		passwordCharSets.special
}

// phcHash represents the decoded components of a PHC string format hash
type phcHash struct {
	version int
	memory  uint32
	time    uint32
	threads uint8
	salt    []byte
	hash    []byte
}

// generateRandomSalt generates a cryptographically secure random salt
func generateRandomSalt(length int) ([]byte, error) {
	salt := make([]byte, length)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random salt: %w", err)
	}
	return salt, nil
}

// isBcryptHash checks if a hash string is in bcrypt format
func isBcryptHash(hash string) bool {
	return strings.HasPrefix(hash, bcryptPrefix2a) ||
		strings.HasPrefix(hash, bcryptPrefix2b) ||
		strings.HasPrefix(hash, bcryptPrefix2y)
}

// isArgon2idHash checks if a hash string is in Argon2id PHC format
func isArgon2idHash(hash string) bool {
	return strings.HasPrefix(hash, argon2idPrefix)
}

// decodePHCHash parses a PHC string format hash into its components
func decodePHCHash(encoded string) (*phcHash, error) {
	parts := strings.Split(encoded, "$")
	// Expected: ["", "argon2id", "v=19", "m=65536,t=3,p=2", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return nil, errors.New("invalid argon2id hash format")
	}

	// Parse version
	versionStr := strings.TrimPrefix(parts[2], "v=")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid argon2id version: %w", err)
	}

	// Parse parameters
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return nil, errors.New("invalid argon2id parameters")
	}

	var memory, time uint32
	var threads uint8
	for _, p := range params {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid parameter: %s", p)
		}
		val, err := strconv.Atoi(kv[1])
		if err != nil {
			return nil, fmt.Errorf("invalid parameter value for %s: %w", kv[0], err)
		}
		switch kv[0] {
		case "m":
			memory = uint32(val)
		case "t":
			time = uint32(val)
		case "p":
			threads = uint8(val)
		}
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, fmt.Errorf("invalid argon2id salt encoding: %w", err)
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, fmt.Errorf("invalid argon2id hash encoding: %w", err)
	}

	return &phcHash{
		version: version,
		memory:  memory,
		time:    time,
		threads: threads,
		salt:    salt,
		hash:    hash,
	}, nil
}

// verifyBcryptPassword verifies a password against a bcrypt hash
func verifyBcryptPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// verifyArgon2idPassword verifies a password against an Argon2id PHC format hash
func verifyArgon2idPassword(hashedPassword, password string) error {
	parsed, err := decodePHCHash(hashedPassword)
	if err != nil {
		return fmt.Errorf("failed to parse hash: %w", err)
	}

	if parsed.memory < 8192 {
		return errors.New("invalid argon2id hash: memory parameter too low")
	}
	if parsed.time < 1 {
		return errors.New("invalid argon2id hash: time parameter too low")
	}
	if parsed.threads < 1 {
		return errors.New("invalid argon2id hash: threads parameter too low")
	}
	if len(parsed.salt) < 8 {
		return errors.New("invalid argon2id hash: salt too short")
	}
	if len(parsed.hash) < 16 {
		return errors.New("invalid argon2id hash: key too short")
	}

	computedHash := argon2.IDKey(
		[]byte(password),
		parsed.salt,
		parsed.time,
		parsed.memory,
		parsed.threads,
		uint32(len(parsed.hash)),
	)

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare(parsed.hash, computedHash) != 1 {
		return errors.New("password does not match")
	}

	return nil
}

// GeneratePassword generates a cryptographically secure random password
// using crypto/rand. The length must be >= 12. If 0 is passed, defaults to 16.
func GeneratePassword(length int) (string, error) {
	if length == 0 {
		length = 16
	}
	if length < 12 {
		return "", fmt.Errorf("password length must be at least 12, got %d", length)
	}

	result := make([]byte, length)

	// Guarantee at least one of each required character type
	sets := []string{
		passwordCharSets.uppercase,
		passwordCharSets.lowercase,
		passwordCharSets.digits,
		passwordCharSets.special,
	}
	for i, set := range sets {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random character: %w", err)
		}
		result[i] = set[idx.Int64()]
	}

	// Fill remaining positions from all character sets
	for i := len(sets); i < length; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(passwordCharSets.all))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random character: %w", err)
		}
		result[i] = passwordCharSets.all[idx.Int64()]
	}

	// Shuffle using Fisher-Yates to randomize guaranteed positions
	for i := length - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", fmt.Errorf("failed to shuffle password: %w", err)
		}
		result[i], result[j.Int64()] = result[j.Int64()], result[i]
	}

	return string(result), nil
}

// ValidatePassword checks if a password meets security criteria
func ValidatePassword(password string) error {
	// Check for empty or whitespace-only passwords, and minimum length (#4)
	if len(strings.TrimSpace(password)) < 12 {
		return ErrPasswordInvalid
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasNumber || !hasSpecial {
		return ErrPasswordInvalid
	}

	return nil
}

// HashPassword generates an Argon2id hash from a plain text password using PHC string format
func HashPassword(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", errors.New("password cannot be empty")
	}

	salt, err := generateRandomSalt(argon2idSaltLen)
	if err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		argon2idTime,
		argon2idMemory,
		argon2idThreads,
		argon2idKeyLen,
	)

	// Encode in PHC string format: $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argon2idMemory, argon2idTime, argon2idThreads,
		encodedSalt, encodedHash,
	), nil
}

// VerifyPassword compares a plain text password with a stored hash.
// It supports both Argon2id (PHC format) and bcrypt hashes for backward compatibility.
func VerifyPassword(hashedPassword, password string) error {
	if isArgon2idHash(hashedPassword) {
		return verifyArgon2idPassword(hashedPassword, password)
	}
	if isBcryptHash(hashedPassword) {
		return verifyBcryptPassword(hashedPassword, password)
	}
	return errors.New("unsupported password hash format")
}

// NeedsRehash returns true if the hash uses an outdated algorithm or parameters
// and should be re-hashed with the current recommended settings.
func NeedsRehash(hashedPassword string) bool {
	if isBcryptHash(hashedPassword) {
		return true
	}

	if !isArgon2idHash(hashedPassword) {
		return false
	}

	parsed, err := decodePHCHash(hashedPassword)
	if err != nil {
		return true
	}

	return parsed.memory != argon2idMemory ||
		parsed.time != argon2idTime ||
		parsed.threads != argon2idThreads
}
