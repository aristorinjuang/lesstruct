package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// splitPreviewForTest splits a signed value into its wire parts.
func splitPreviewForTest(t *testing.T, value string) []string {
	t.Helper()
	parts := strings.Split(value, ".")
	require.Len(t, parts, 3)
	return parts
}

// signPreviewAt mints a preview cookie value expiring at the given time,
// mirroring the documented wire format.
func signPreviewAt(t *testing.T, name string, key []byte, expiresAt time.Time) string {
	t.Helper()
	expiry := expiresAt.Unix()
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(name))
	mac.Write([]byte("|"))
	mac.Write([]byte(strconv.FormatInt(expiry, 10)))
	encoded := base64.RawURLEncoding.EncodeToString([]byte(name))
	return encoded + "." + strconv.FormatInt(expiry, 10) + "." + hex.EncodeToString(mac.Sum(nil))
}

func TestPreviewCookieRoundTrip(t *testing.T) {
	key := []byte("preview-test-key-32-bytes-long!!")

	tests := []struct {
		name     string
		theme    string
		wantName string
		wantOK   bool
	}{
		{
			name:     "success - signed cookie verifies",
			theme:    "midnight-serif",
			wantName: "midnight-serif",
			wantOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, expiresAt := SignPreviewCookie(tt.theme, key)
			require.False(t, expiresAt.IsZero())
			name, ok := VerifyPreviewCookie(value, key)
			require.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantName, name)
		})
	}
}

func TestVerifyPreviewCookie(t *testing.T) {
	key := []byte("preview-test-key-32-bytes-long!!")
	otherKey := []byte("another-test-key-32-bytes-long!!")

	tests := []struct {
		name     string
		value    func(t *testing.T) string
		wantName string
		wantOK   bool
	}{
		{
			name: "success - fresh signature verifies",
			value: func(t *testing.T) string {
				value, _ := SignPreviewCookie("midnight-serif", key)
				return value
			},
			wantName: "midnight-serif",
			wantOK:   true,
		},
		{
			name: "error - wrong key rejects",
			value: func(t *testing.T) string {
				value, _ := SignPreviewCookie("midnight-serif", otherKey)
				return value
			},
			wantOK: false,
		},
		{
			name: "error - tampered name rejects",
			value: func(t *testing.T) string {
				value, _ := SignPreviewCookie("midnight-serif", key)
				parts := splitPreviewForTest(t, value)
				other := base64.RawURLEncoding.EncodeToString([]byte("paper-light"))
				return other + "." + parts[1] + "." + parts[2]
			},
			wantOK: false,
		},
		{
			name: "error - expired rejects",
			value: func(t *testing.T) string {
				return signPreviewAt(t, "midnight-serif", key, time.Now().Add(-time.Minute))
			},
			wantOK: false,
		},
		{
			name: "error - malformed rejects",
			value: func(t *testing.T) string {
				return "not-a-cookie"
			},
			wantOK: false,
		},
		{
			name: "error - invalid theme name rejects",
			value: func(t *testing.T) string {
				return signPreviewAt(t, "Default", key, time.Now().Add(time.Minute))
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, ok := VerifyPreviewCookie(tt.value(t), key)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantName, name)
		})
	}
}
