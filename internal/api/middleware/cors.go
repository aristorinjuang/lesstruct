package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/util"
)

const (
	accessControlAllowOrigin      = "Access-Control-Allow-Origin"
	accessControlAllowMethods     = "Access-Control-Allow-Methods"
	accessControlAllowHeaders     = "Access-Control-Allow-Headers"
	accessControlAllowCredentials = "Access-Control-Allow-Credentials"
	accessControlMaxAge           = "Access-Control-Max-Age"
	varyHeader                    = "Vary"
)

// isOriginAllowed checks if the given origin is in the allowed origins list
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	return slices.Contains(allowedOrigins, origin)
}

// requestOrigin reconstructs the request's own origin (scheme://host) for
// same-origin detection. X-Forwarded-Proto wins when present so deployments
// behind a TLS-terminating reverse proxy compare against the client-facing
// scheme; otherwise the TLS state decides. Only the first forwarded value is
// used.
func requestOrigin(r *http.Request) string {
	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		if first, _, _ := strings.Cut(proto, ","); strings.TrimSpace(first) != "" {
			scheme = strings.ToLower(strings.TrimSpace(first))
		}
	} else if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// isSameOrigin reports whether the Origin header matches the request's own
// origin. Same-origin browser requests (e.g. admin SPA XHRs) carry an Origin
// header but are not CORS requests — they skip the allowlist check and, in
// particular, the unauthorized-origin log line.
func isSameOrigin(r *http.Request, origin string) bool {
	if origin == "" {
		return false
	}
	return origin == requestOrigin(r)
}

type CORSMiddleware struct {
	allowedOrigins []string
	logger         *util.Logger
}

func (m *CORSMiddleware) setCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set(accessControlAllowOrigin, origin)
	w.Header().Set(accessControlAllowCredentials, "true")
	w.Header().Add(varyHeader, "Origin")
}

func (m *CORSMiddleware) handlePreflight(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")

	if isSameOrigin(r, origin) {
		w.WriteHeader(http.StatusOK)
		return
	}
	if isOriginAllowed(origin, m.allowedOrigins) {
		m.setCORSHeaders(w, origin)
		w.Header().Set(accessControlAllowMethods, "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set(accessControlAllowHeaders, "Content-Type, Authorization, X-Requested-With")
		w.Header().Set(accessControlMaxAge, "86400")
	} else if origin != "" {
		m.logger.Info("CORS preflight rejected: unauthorized origin %s", origin)
	}

	w.WriteHeader(http.StatusOK)
}

func (m *CORSMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if r.Method == http.MethodOptions && origin != "" {
			m.handlePreflight(w, r)
			return
		}

		if isSameOrigin(r, origin) {
			next.ServeHTTP(w, r)
			return
		}
		if isOriginAllowed(origin, m.allowedOrigins) {
			m.setCORSHeaders(w, origin)
		} else if origin != "" {
			m.logger.Info("CORS request rejected: unauthorized origin %s for %s %s", origin, r.Method, r.URL.Path)
		}

		next.ServeHTTP(w, r)
	})
}

func NewCORSMiddleware(allowedOrigins []string, logger *util.Logger) *CORSMiddleware {
	copied := make([]string, len(allowedOrigins))
	copy(copied, allowedOrigins)

	return &CORSMiddleware{
		allowedOrigins: copied,
		logger:         logger,
	}
}
