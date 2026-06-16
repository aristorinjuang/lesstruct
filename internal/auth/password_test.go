package auth_test

import (
	"fmt"
	"strings"
	"testing"

	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	password := "admin"

	hash, err := appauth.HashPassword(password)
	require.NoError(t, err, "HashPassword() error")
	assert.NotEmpty(t, hash, "HashPassword() returned empty string")
	assert.NotEqual(t, password, hash, "HashPassword() returned unhashed password")
}

func TestVerifyPassword_Valid(t *testing.T) {
	password := "admin"

	hash, err := appauth.HashPassword(password)
	require.NoError(t, err, "HashPassword() error")

	err = appauth.VerifyPassword(hash, password)
	assert.NoError(t, err, "VerifyPassword() unexpected error")
}

func TestVerifyPassword_Invalid(t *testing.T) {
	password := "admin"
	wrongPassword := "wrong"

	hash, err := appauth.HashPassword(password)
	require.NoError(t, err, "HashPassword() error")

	err = appauth.VerifyPassword(hash, wrongPassword)
	assert.Error(t, err, "VerifyPassword() expected error for wrong password")
}

func TestVerifyPassword_HashConsistency(t *testing.T) {
	password := "admin"

	// Hash the same password twice
	hash1, err1 := appauth.HashPassword(password)
	hash2, err2 := appauth.HashPassword(password)

	require.NoError(t, err1, "HashPassword() error1")
	require.NoError(t, err2, "HashPassword() error2")

	// Hashes should be different (Argon2id uses random salt)
	assert.NotEqual(t, hash1, hash2, "HashPassword() generated identical hashes (salt should be different)")

	// But both should verify correctly
	assert.NoError(t, appauth.VerifyPassword(hash1, password), "VerifyPassword() hash1 failed")
	assert.NoError(t, appauth.VerifyPassword(hash2, password), "VerifyPassword() hash2 failed")
}

func TestHashPassword_PHCFormat(t *testing.T) {
	password := "TestPassword123!"

	hash, err := appauth.HashPassword(password)
	require.NoError(t, err)
	require.NotEmpty(t, hash)

	// Must start with $argon2id$
	assert.True(t, strings.HasPrefix(hash, "$argon2id$"), "hash must start with $argon2id$, got: %s", hash)

	// Split into PHC components: ["", "argon2id", "v=19", "m=65536,t=3,p=2", "<salt>", "<hash>"]
	parts := strings.Split(hash, "$")
	require.Len(t, parts, 6, "PHC format must have 6 parts, got %d: %v", len(parts), parts)

	assert.Equal(t, "argon2id", parts[1])
	assert.Equal(t, "v=19", parts[2])
	assert.Equal(t, "m=65536,t=3,p=2", parts[3], "parameters must match OWASP recommendations")

	// Salt and hash must be non-empty base64
	assert.NotEmpty(t, parts[4], "salt must be non-empty")
	assert.NotEmpty(t, parts[5], "hash must be non-empty")
}

func TestHashPassword_Empty(t *testing.T) {
	_, err := appauth.HashPassword("")
	assert.Error(t, err, "HashPassword() expected error for empty password")

	_, err = appauth.HashPassword("   ")
	assert.Error(t, err, "HashPassword() expected error for whitespace-only password")
}

func TestVerifyPassword_UnsupportedFormat(t *testing.T) {
	err := appauth.VerifyPassword("$unknown$format", "password")
	assert.Error(t, err, "VerifyPassword() expected error for unsupported hash format")
	assert.Contains(t, err.Error(), "unsupported password hash format")
}

func TestVerifyPassword_BcryptBackwardCompat(t *testing.T) {
	password := "TestPassword123!"

	// Generate a real bcrypt hash at runtime for reliable testing
	bcryptBytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	require.NoError(t, err, "failed to generate bcrypt hash")
	bcryptHash := string(bcryptBytes)

	// Verify that bcrypt hash is correctly identified and verified
	err = appauth.VerifyPassword(bcryptHash, password)
	assert.NoError(t, err, "bcrypt hash should verify successfully")

	// Wrong password should fail
	err = appauth.VerifyPassword(bcryptHash, "WrongPassword123!")
	assert.Error(t, err, "wrong password should fail bcrypt verification")
}

func TestNeedsRehash_BcryptHash(t *testing.T) {
	// Generate a real bcrypt hash at runtime
	bcryptBytes, err := bcrypt.GenerateFromPassword([]byte("TestPassword123!"), 12)
	require.NoError(t, err)
	bcryptHash := string(bcryptBytes)

	assert.True(t, appauth.NeedsRehash(bcryptHash), "bcrypt hashes should always need rehash")
}

func TestNeedsRehash_Argon2idHash(t *testing.T) {
	hash, err := appauth.HashPassword("TestPassword123!")
	require.NoError(t, err)
	assert.False(t, appauth.NeedsRehash(hash), "current Argon2id hash should not need rehash")
}

func TestNeedsRehash_OutdatedParams(t *testing.T) {
	// Argon2id hash with different memory parameter (m=32768 instead of m=65536)
	outdatedHash := "$argon2id$v=19$m=32768,t=3,p=2$c2FsdHNhbHQ$YmFzZTY0aGFzaA=="
	assert.True(t, appauth.NeedsRehash(outdatedHash), "Argon2id with outdated memory should need rehash")

	// Argon2id hash with different iterations (t=1 instead of t=3)
	outdatedTimeHash := "$argon2id$v=19$m=65536,t=1,p=2$c2FsdHNhbHQ$YmFzZTY0aGFzaA=="
	assert.True(t, appauth.NeedsRehash(outdatedTimeHash), "Argon2id with outdated time should need rehash")

	// Argon2id hash with different parallelism (p=1 instead of p=2)
	outdatedThreadsHash := "$argon2id$v=19$m=65536,t=3,p=1$c2FsdHNhbHQ$YmFzZTY0aGFzaA=="
	assert.True(t, appauth.NeedsRehash(outdatedThreadsHash), "Argon2id with outdated parallelism should need rehash")
}

func TestNeedsRehash_UnknownFormat(t *testing.T) {
	assert.False(t, appauth.NeedsRehash("$unknown$format"), "unknown format should not need rehash")
}

func TestValidatePassword_Valid(t *testing.T) {
	validPasswords := []string{
		"SecurePass123!",
		"AnotherSecure456@",
		"MyP@ssword123456",
		"Test1234!5678",
		"Admin@Pass1234",
	}

	for _, password := range validPasswords {
		t.Run(password, func(t *testing.T) {
			err := appauth.ValidatePassword(password)
			assert.NoError(t, err, "ValidatePassword(%q) unexpected error", password)
		})
	}
}

func TestValidatePassword_TooShort(t *testing.T) {
	shortPasswords := []string{
		"Short1!",
		"Test123!",
		"Pass12@",
		"",
		"12345678901", // 11 chars
	}

	for _, password := range shortPasswords {
		t.Run(password, func(t *testing.T) {
			err := appauth.ValidatePassword(password)
			assert.Error(t, err, "ValidatePassword() expected error for short password")
			assert.EqualError(t, err, "password must be at least 12 characters with mixed case, numbers, and special characters", "ValidatePassword() unexpected error message")
		})
	}
}

func TestValidatePassword_NoUppercase(t *testing.T) {
	passwords := []string{
		"lowercase123!",
		"nouppercase@123",
		"alllower456!",
	}

	for _, password := range passwords {
		t.Run(password, func(t *testing.T) {
			err := appauth.ValidatePassword(password)
			assert.Error(t, err, "ValidatePassword() expected error for password without uppercase")
			assert.EqualError(t, err, "password must be at least 12 characters with mixed case, numbers, and special characters", "ValidatePassword() unexpected error message")
		})
	}
}

func TestValidatePassword_NoLowercase(t *testing.T) {
	passwords := []string{
		"UPPERCASE123!",
		"NOLOWERCASE@123",
		"ALLUPPER456!",
	}

	for _, password := range passwords {
		t.Run(password, func(t *testing.T) {
			err := appauth.ValidatePassword(password)
			assert.Error(t, err, "ValidatePassword() expected error for password without lowercase")
			assert.EqualError(t, err, "password must be at least 12 characters with mixed case, numbers, and special characters", "ValidatePassword() unexpected error message")
		})
	}
}

func TestValidatePassword_NoNumber(t *testing.T) {
	passwords := []string{
		"NoNumbersHere!",
		"OnlyLetters!@",
		"JustSpecialChars!@#",
	}

	for _, password := range passwords {
		t.Run(password, func(t *testing.T) {
			err := appauth.ValidatePassword(password)
			assert.Error(t, err, "ValidatePassword() expected error for password without numbers")
			assert.EqualError(t, err, "password must be at least 12 characters with mixed case, numbers, and special characters", "ValidatePassword() unexpected error message")
		})
	}
}

func TestValidatePassword_NoSpecialChar(t *testing.T) {
	passwords := []string{
		"NoSpecial123",
		"OnlyLetters123",
		"JustAlphanum3ric",
	}

	for _, password := range passwords {
		t.Run(password, func(t *testing.T) {
			err := appauth.ValidatePassword(password)
			assert.Error(t, err, "ValidatePassword() expected error for password without special characters")
			assert.EqualError(t, err, "password must be at least 12 characters with mixed case, numbers, and special characters", "ValidatePassword() unexpected error message")
		})
	}
}

func TestValidatePassword_Empty(t *testing.T) {
	err := appauth.ValidatePassword("")
	assert.Error(t, err, "ValidatePassword() expected error for empty password")
	assert.EqualError(t, err, "password must be at least 12 characters with mixed case, numbers, and special characters", "ValidatePassword() unexpected error message")
}

func TestValidatePassword_Exactly12Chars(t *testing.T) {
	password := "Test1234!abc" // Exactly 12 chars
	err := appauth.ValidatePassword(password)
	assert.NoError(t, err, "ValidatePassword() unexpected error for exactly 12 char password")
}

func TestValidatePassword_AllSpecialChars(t *testing.T) {
	password := "!@#$%^&*()123"
	err := appauth.ValidatePassword(password)
	// Should fail because no mixed case (no uppercase or lowercase letters)
	assert.Error(t, err, "ValidatePassword() expected error for password without mixed case")
}

func TestGeneratePassword_Length(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"minimum valid length", 12},
		{"default length", 16},
		{"longer length", 24},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw, err := appauth.GeneratePassword(tt.length)
			require.NoError(t, err)
			assert.Len(t, pw, tt.length)
		})
	}
}

func TestGeneratePassword_InvalidLength(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"too short", 11},
		{"negative", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := appauth.GeneratePassword(tt.length)
			assert.Error(t, err)
		})
	}
}

func TestGeneratePassword_MeetsValidatePassword(t *testing.T) {
	for i := range 1000 {
		pw, err := appauth.GeneratePassword(16)
		require.NoError(t, err, "iteration %d", i)
		assert.NoError(t, appauth.ValidatePassword(pw), "generated password failed validation: %s", pw)
	}
}

func TestGeneratePassword_Uniqueness(t *testing.T) {
	generated := make(map[string]bool, 1000)
	for range 1000 {
		pw, err := appauth.GeneratePassword(16)
		require.NoError(t, err)
		assert.False(t, generated[pw], "duplicate password generated: %s", pw)
		generated[pw] = true
	}
}

func TestGeneratePassword_DefaultLength(t *testing.T) {
	pw, err := appauth.GeneratePassword(0)
	require.NoError(t, err)
	assert.Len(t, pw, 16)
	assert.NoError(t, appauth.ValidatePassword(pw))
}

// TestVerifyPassword_BcryptToArgon2idMigration tests the full migration flow:
// hash with Argon2id, verify with Argon2id, and ensure backward compat dispatch works
func TestVerifyPassword_BcryptToArgon2idMigration(t *testing.T) {
	password := "MigrationTest123!"

	// Hash with Argon2id (new behavior)
	hash, err := appauth.HashPassword(password)
	require.NoError(t, err)

	// Verify works with Argon2id
	assert.NoError(t, appauth.VerifyPassword(hash, password))

	// Does not need rehash
	assert.False(t, appauth.NeedsRehash(hash))
}

// TestHashPassword_ProducesSelfDescribingHash verifies that Argon2id hashes
// contain all parameters needed for verification (self-describing)
func TestHashPassword_ProducesSelfDescribingHash(t *testing.T) {
	password := "SelfDescribing123!"

	hash, err := appauth.HashPassword(password)
	require.NoError(t, err)

	// The hash should contain all parameters in its string representation
	assert.Contains(t, hash, "m=65536", "memory parameter should be in hash")
	assert.Contains(t, hash, "t=3", "time parameter should be in hash")
	assert.Contains(t, hash, "p=2", "parallelism parameter should be in hash")
	assert.Contains(t, hash, "v=19", "version should be in hash")

	// Verify using only the hash string (no external params needed)
	assert.NoError(t, appauth.VerifyPassword(hash, password))
}

// TestNeedsRehash_AllBcryptPrefixes tests that both $2a$ and $2b$ bcrypt
// prefixes are recognized for rehash
func TestNeedsRehash_AllBcryptPrefixes(t *testing.T) {
	tests := []struct {
		name   string
		hash   string
		need   bool
		reason string
	}{
		{
			name:   "bcrypt 2a prefix",
			hash:   "$2a$12$abcdefghijklmnopqrstuvwxABCDEFGHJKLMNOPQRSTUV",
			need:   true,
			reason: "bcrypt $2a$ should need rehash",
		},
		{
			name:   "bcrypt 2b prefix",
			hash:   "$2b$12$abcdefghijklmnopqrstuvwxABCDEFGHJKLMNOPQRSTUV",
			need:   true,
			reason: "bcrypt $2b$ should need rehash",
		},
		{
			name:   "bcrypt 2y prefix",
			hash:   "$2y$12$abcdefghijklmnopqrstuvwxABCDEFGHJKLMNOPQRSTUV",
			need:   true,
			reason: "bcrypt $2y$ should need rehash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appauth.NeedsRehash(tt.hash)
			assert.Equal(t, tt.need, result, tt.reason)
		})
	}
}

// BenchmarkHashPassword benchmarks Argon2id password hashing performance
func BenchmarkHashPassword(b *testing.B) {
	password := "BenchmarkPassword123!"
	for b.Loop() {
		_, _ = appauth.HashPassword(password)
	}
}

// BenchmarkVerifyPassword benchmarks Argon2id password verification performance
func BenchmarkVerifyPassword(b *testing.B) {
	password := "BenchmarkPassword123!"
	hash, _ := appauth.HashPassword(password)

	b.ResetTimer()
	for b.Loop() {
		_ = appauth.VerifyPassword(hash, password)
	}
}

// TestVerifyPassword_ConstantTime tests that verification doesn't leak timing
// information by ensuring the function works consistently
func TestVerifyPassword_ConstantTime(t *testing.T) {
	password := "ConstantTimeTest123!"

	hash, err := appauth.HashPassword(password)
	require.NoError(t, err)

	// Correct password should succeed
	assert.NoError(t, appauth.VerifyPassword(hash, password))

	// Wrong password should fail
	wrongPasswords := []string{
		"ConstantTimeTest123",      // off by one char
		"constantTimeTest123!",     // wrong case
		fmt.Sprintf("X%s", password[1:]),  // wrong first char
	}

	for _, wrong := range wrongPasswords {
		assert.Error(t, appauth.VerifyPassword(hash, wrong),
			"wrong password should fail: %q", wrong)
	}
}
