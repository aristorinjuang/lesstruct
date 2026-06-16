package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	"github.com/go-chi/httprate"
)

// keyByAPIKeyOrIP is the per-request rate-limit key for the Bearer /api/v1 group.
// When an "Authorization: Bearer lesstruct_<keyID>_<secret>" header is present, the
// bucket is keyed by the presented keyID (the 12-hex segment between the first two
// underscores); when no Bearer/lesstruct_ header is present it falls back to the
// client IP, mirroring httprate.KeyByIP. Keying by the presented keyID — not the
// validated user — is deliberate: it gives each API key its own budget (attribution +
// fairness, AC #1) and, because the limiter runs BEFORE auth in the Bearer group
// (see routes.go), it also throttles repeated attempts that present the SAME keyID,
// even when that key never verifies. It never returns an error — there is always a
// usable key (keyID or IP).
//
// Scope/limitation: this is per-keyID only (AC #1 keys by API key, not IP). An
// attacker rotating DISTINCT fake keyIDs gets a fresh budget per keyID; there is no
// per-IP ceiling on the Bearer group, so rotation-resistance is intentionally out of
// scope for this limiter.
func keyByAPIKeyOrIP(r *http.Request) (string, error) {
	token := extractBearer(r)
	if token != "" && strings.HasPrefix(token, apikey.KeyPrefix) {
		rest := token[len(apikey.KeyPrefix):]
		parts := strings.SplitN(rest, "_", 2)
		if len(parts) > 0 && parts[0] != "" {
			return parts[0], nil
		}
	}
	return httprate.KeyByIP(r)
}

type RateLimitMiddleware struct {
	enabled         bool
	authPerMinute   int
	apiPerMinute    int
	publicPerMinute int
}

func (m *RateLimitMiddleware) rateLimitHandler(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Too many requests. Please try again later.", nil)
}

// apiKeyRateLimitHandler is the dedicated limit handler for the Bearer /api/v1
// group. It emits the v1 catalog code RATE_LIMITED (NOT the browser group's
// RATE_LIMIT_EXCEEDED). See AC #4 + Story Dev Notes §Per-key rate-limit extractor.
func (m *RateLimitMiddleware) apiKeyRateLimitHandler(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests. Please try again later.", nil)
}

func (m *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
	if !m.enabled {
		return next
	}

	return httprate.Limit(
		m.apiPerMinute,
		time.Minute,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(m.rateLimitHandler),
	)(next)
}

func (m *RateLimitMiddleware) AuthHandler(next http.Handler) http.Handler {
	if !m.enabled {
		return next
	}

	return httprate.Limit(
		m.authPerMinute,
		time.Minute,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(m.rateLimitHandler),
	)(next)
}

func (m *RateLimitMiddleware) PublicHandler(next http.Handler) http.Handler {
	if !m.enabled {
		return next
	}

	return httprate.Limit(
		m.publicPerMinute,
		time.Minute,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(m.rateLimitHandler),
	)(next)
}

// APIKeyHandler rate-limits the Bearer /api/v1 group per API key (via
// keyByAPIKeyOrIP), reusing the existing apiPerMinute limit and 1-minute window. It
// emits the v1 RATE_LIMITED envelope code when the budget is exhausted. When the
// middleware is disabled it passes through (matching Handler/AuthHandler/PublicHandler).
func (m *RateLimitMiddleware) APIKeyHandler(next http.Handler) http.Handler {
	if !m.enabled {
		return next
	}

	return httprate.Limit(
		m.apiPerMinute,
		time.Minute,
		httprate.WithKeyFuncs(keyByAPIKeyOrIP),
		httprate.WithLimitHandler(m.apiKeyRateLimitHandler),
	)(next)
}

func NewRateLimitMiddleware(
	enabled bool,
	authPerMinute, apiPerMinute, publicPerMinute int,
) *RateLimitMiddleware {
	if enabled {
		if authPerMinute < 1 {
			authPerMinute = 1
		}
		if apiPerMinute < 1 {
			apiPerMinute = 1
		}
		if publicPerMinute < 1 {
			publicPerMinute = 1
		}
	}

	return &RateLimitMiddleware{
		enabled:         enabled,
		authPerMinute:   authPerMinute,
		apiPerMinute:    apiPerMinute,
		publicPerMinute: publicPerMinute,
	}
}
