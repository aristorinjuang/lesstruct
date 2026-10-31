// Package kernel session cookie contract: the browser session cookie name
// plus the HTTPS-detection helper shared by middleware, auth handlers, and
// app wiring.
package kernel

import (
	"net/http"
	"strings"
	"time"
)

const (
	// SessionCookieName is the HttpOnly browser session cookie. It is
	// namespaced away from the storefront token keys.
	SessionCookieName = "lesstruct_jwt"
)

// RequestIsHTTPS reports whether the visitor reached the site over TLS.
// X-Forwarded-Proto wins when present so deployments behind a
// TLS-terminating reverse proxy see the client-facing scheme; otherwise the
// connection state decides. Only the first forwarded value is used.
func RequestIsHTTPS(r *http.Request) bool {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		if first, _, _ := strings.Cut(proto, ","); strings.TrimSpace(first) != "" {
			return strings.EqualFold(strings.TrimSpace(first), "https")
		}
	}

	return r.TLS != nil
}

// SetSessionCookie writes the browser session cookie. The cookie is HttpOnly,
// SameSite=Lax, Path=/, and Secure exactly on HTTPS so plain-HTTP localhost
// keeps working. Lifetime matches the token expiry.
func SetSessionCookie(
	w http.ResponseWriter,
	r *http.Request,
	token string,
	expires time.Time,
) {
	maxAge := max(0, int(time.Until(expires).Seconds()))

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   RequestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie clears the browser session cookie with matching
// attributes so the deletion replaces the session cookie.
func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   RequestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}
