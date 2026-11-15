package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
)

const (
	// PreviewCookieName carries the signed theme preview selection. It must
	// stay on the NoCookie allowlist (internal/app) or the guard strips it
	// and preview silently never engages.
	PreviewCookieName = "lesstruct_theme_preview"

	// previewCookieTTL bounds how long a preview selection stays valid.
	previewCookieTTL = 15 * time.Minute
)

// previewMAC computes the cookie authenticator over name and expiry.
func previewMAC(name string, expiry int64, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(name))
	mac.Write([]byte("|"))
	mac.Write([]byte(strconv.FormatInt(expiry, 10)))
	return mac.Sum(nil)
}

// parsePreviewCookie splits a cookie value into its name and expiry parts,
// verifying the authenticator. It reports false on any malformed, expired,
// or forged input.
func parsePreviewCookie(value string, key []byte) (string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return "", false
	}
	nameBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	name := string(nameBytes)
	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", false
	}
	want, err := hex.DecodeString(parts[2])
	if err != nil {
		return "", false
	}
	if subtle.ConstantTimeCompare(want, previewMAC(name, expiry, key)) != 1 {
		return "", false
	}
	if time.Now().Unix() > expiry {
		return "", false
	}
	return name, true
}

// DerivePreviewKey derives the cookie-signing key from the server secret.
// Domain separation keeps a leaked preview cookie useless against the
// session and API-key schemes.
func DerivePreviewKey(jwtSecret string) []byte {
	sum := sha256.Sum256([]byte("lesstruct:theme-preview:" + jwtSecret))
	return sum[:]
}

// SignPreviewCookie mints an authenticated preview cookie value for name,
// valid for the preview TTL. It also returns the expiry for the response.
func SignPreviewCookie(name string, key []byte) (value string, expiresAt time.Time) {
	expiresAt = time.Now().Add(previewCookieTTL)
	expiry := expiresAt.Unix()
	encoded := base64.RawURLEncoding.EncodeToString([]byte(name))
	return encoded + "." + strconv.FormatInt(expiry, 10) + "." + hex.EncodeToString(previewMAC(name, expiry, key)), expiresAt
}

// VerifyPreviewCookie authenticates a preview cookie value and returns the
// theme name. Forged, malformed, or expired values report false.
func VerifyPreviewCookie(value string, key []byte) (string, bool) {
	name, ok := parsePreviewCookie(value, key)
	if !ok {
		return "", false
	}
	if err := themegendomain.ValidateThemeName(name); err != nil {
		return "", false
	}
	return name, true
}

// SetPreviewCookie writes the signed preview selection. Secure mirrors the
// site origin (HTTPS only).
func SetPreviewCookie(w http.ResponseWriter, value string, expiresAt time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     PreviewCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(previewCookieTTL / time.Second),
		Expires:  expiresAt,
		Secure:   secure,
	})
}

// ClearPreviewCookie unsets the preview selection.
func ClearPreviewCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     PreviewCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
		Secure:   secure,
	})
}
