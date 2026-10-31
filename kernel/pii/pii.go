// Package pii is the shared application-level PII primitive: AES-256-GCM
// field encryption with deterministic HMAC blind indexes for exact-match
// lookups. Randomized GCM ciphertext cannot be queried, so each lookup
// carries one HMAC column over the normalized lookup value.
//
// The envelope doubles as the backfill detector and the future rotation
// marker: encrypted values carry the v1: prefix, legacy plaintext rows do
// not. Decrypt passes prefix-less values through untouched, so rows written
// before encryption read back byte-identical. Empty strings stay empty and
// are never encrypted — they carry no information to protect.
//
// A value that literally begins with the prefix in a legacy row is
// indistinguishable from ciphertext; operators must re-key such rows
// through the encrypted write path before enabling the key.
package pii

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	// Prefix marks an encrypted envelope. It doubles as the backfill
	// detector: rows without it are legacy plaintext.
	Prefix = "v1:"
	// NonceSize is the GCM nonce width in bytes.
	NonceSize = 12
	// MasterKeySize is the required master-key width in bytes (AES-256).
	MasterKeySize = 32
)

var (
	// ErrInvalidMasterKey is returned when the master key is not exactly
	// MasterKeySize bytes.
	ErrInvalidMasterKey = errors.New("pii: master key must be 32 bytes")
	// ErrCorruptCiphertext is returned when an enveloped value fails to
	// decode or open. Callers surface it as a store error — never silent
	// wrong data.
	ErrCorruptCiphertext = errors.New("pii: ciphertext is corrupt")
)

// hkdfExtract is the RFC 5869 extract step over SHA-256: a pseudorandom
// key concentrated from the input keying material. The salt is fixed at
// zero because the master key is already high-entropy; domain separation
// rides the expand info strings.
func hkdfExtract(input []byte) []byte {
	mac := hmac.New(sha256.New, make([]byte, sha256.Size))
	mac.Write(input)
	return mac.Sum(nil)
}

// hkdfExpand is the RFC 5869 expand step over SHA-256: it stretches the
// pseudorandom key into length bytes bound to the info string, so the
// encryption and index subkeys never share output.
func hkdfExpand(prk []byte, info string, length int) []byte {
	var okm []byte
	var previous []byte
	for i := 1; len(okm) < length; i++ {
		mac := hmac.New(sha256.New, prk)
		mac.Write(previous)
		mac.Write([]byte(info))
		mac.Write([]byte{byte(i)})
		previous = mac.Sum(nil)
		okm = append(okm, previous...)
	}
	return okm[:length]
}

// IsEncrypted reports whether the stored value carries the encrypted
// envelope. Prefix-less values are legacy plaintext.
func IsEncrypted(stored string) bool {
	return strings.HasPrefix(stored, Prefix)
}

// Cipher encrypts single PII fields and derives their blind indexes. One
// instance serves every commerce store; it holds only subkeys, never the
// master key, and is safe for concurrent use.
type Cipher struct {
	sealer   cipher.AEAD
	indexKey []byte
}

// encryptValue seals one non-empty field into the v1: envelope. Empty
// input stays empty.
func (c *Cipher) encryptValue(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("pii: failed to sample nonce: %w", err)
	}
	sealed := c.sealer.Seal(nonce, nonce, []byte(plaintext), nil)
	return Prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// decryptValue opens one stored field. Prefix-less values are legacy
// plaintext and pass through untouched; empty stays empty. Enveloped
// values that fail to decode or open report ErrCorruptCiphertext.
func (c *Cipher) decryptValue(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	payload, ok := strings.CutPrefix(stored, Prefix)
	if !ok {
		return stored, nil
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("pii: failed to decode envelope: %w", ErrCorruptCiphertext)
	}
	if len(raw) < NonceSize {
		return "", fmt.Errorf("pii: envelope too short: %w", ErrCorruptCiphertext)
	}
	opened, err := c.sealer.Open(nil, raw[:NonceSize], raw[NonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("pii: failed to open envelope: %w", ErrCorruptCiphertext)
	}
	return string(opened), nil
}

// indexValue derives the deterministic blind index over the normalized
// lookup value. Empty input stays empty so unset fields leave the index
// column empty too.
func (c *Cipher) indexValue(normalized string) string {
	if normalized == "" {
		return ""
	}
	mac := hmac.New(sha256.New, c.indexKey)
	mac.Write([]byte(normalized))
	return hex.EncodeToString(mac.Sum(nil))
}

// Encrypt seals one PII field into the v1: envelope. Empty input stays
// empty and is never encrypted.
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	return c.encryptValue(plaintext)
}

// Decrypt opens one stored PII field. Values without the v1: prefix are
// legacy plaintext and pass through untouched; empty stays empty. A
// corrupt envelope reports ErrCorruptCiphertext — never silent wrong data.
func (c *Cipher) Decrypt(stored string) (string, error) {
	return c.decryptValue(stored)
}

// Index derives the deterministic blind index over the normalized lookup
// value: the same input always yields the same index, and distinct inputs
// (barring a hash collision) yield distinct indexes. Callers normalize
// before indexing — the store indexes the already-normalized email the
// service passes, so the write-time and lookup-time forms must match.
// Empty input stays empty.
func (c *Cipher) Index(normalized string) string {
	return c.indexValue(normalized)
}

// New builds the cipher over the 32-byte master key, deriving separate
// encryption and blind-index subkeys via HKDF-SHA256. Any other length
// fails with ErrInvalidMasterKey.
func New(master []byte) (*Cipher, error) {
	if len(master) != MasterKeySize {
		return nil, fmt.Errorf("%w: got %d bytes", ErrInvalidMasterKey, len(master))
	}
	prk := hkdfExtract(master)
	sealer, err := aes.NewCipher(hkdfExpand(prk, "lesstruct pii field encryption v1", MasterKeySize))
	if err != nil {
		return nil, fmt.Errorf("pii: failed to build field cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(sealer)
	if err != nil {
		return nil, fmt.Errorf("pii: failed to build field cipher: %w", err)
	}
	return &Cipher{
		sealer:   gcm,
		indexKey: hkdfExpand(prk, "lesstruct pii blind index v1", MasterKeySize),
	}, nil
}
