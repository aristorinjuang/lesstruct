package middleware

import (
	"fmt"
	"net/http"
)

const (
	// permissionsPolicy disables browser features the platform never uses, so a
	// compromised or third-party embed cannot ask for them on a Lesstruct page.
	permissionsPolicy = "camera=(), microphone=(), geolocation=(), browsing-topics=()"
)

type SecurityHeadersMiddleware struct {
	cspHeaderName  string
	cspHeaderValue string
	xFrameOptions  string
	hstsMaxAge     int
}

func (m *SecurityHeadersMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.xFrameOptions != "" {
			w.Header().Set("X-Frame-Options", m.xFrameOptions)
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", permissionsPolicy)
		if m.hstsMaxAge > 0 {
			w.Header().Set("Strict-Transport-Security", fmt.Sprintf("max-age=%d", m.hstsMaxAge))
		}
		if m.cspHeaderValue != "" {
			w.Header().Set(m.cspHeaderName, m.cspHeaderValue)
		}
		next.ServeHTTP(w, r)
	})
}

// NewSecurityHeadersMiddleware builds the header middleware. hstsMaxAge is the
// Strict-Transport-Security lifetime in seconds; zero omits the header, which
// keeps plain-HTTP development origins usable. The core config derives the
// default from SITE_URL (https opts in at one year) and allows an override via
// HSTS_MAX_AGE_SECONDS.
func NewSecurityHeadersMiddleware(cspHeaderName, cspHeaderValue, xFrameOptions string, hstsMaxAge int) *SecurityHeadersMiddleware {
	return &SecurityHeadersMiddleware{
		cspHeaderName:  cspHeaderName,
		cspHeaderValue: cspHeaderValue,
		xFrameOptions:  xFrameOptions,
		hstsMaxAge:     hstsMaxAge,
	}
}
