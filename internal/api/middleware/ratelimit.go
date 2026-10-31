package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	"github.com/go-chi/httprate"
)

const (
	// DefaultAgentFailuresPerMinute is the fallback budget for the
	// outcome-based Bearer limiter: how many 401 responses one client IP
	// may accumulate in a minute before further Bearer requests are
	// refused. Low enough to stop fake-key rotation (the finding-10 PoC
	// sent ~20), high enough to never touch legitimate use.
	DefaultAgentFailuresPerMinute = 15

	// agentFailureMapCap bounds the failure-tracker's per-IP state. Past
	// the cap the tracker sweeps expired windows and then fails open
	// rather than growing without bound.
	agentFailureMapCap = 10000
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
// Scope: the per-keyID budget is paired with a per-IP ceiling (AgentIPHandler)
// applied outside this extractor, so rotating DISTINCT fake keyIDs does not buy
// a fresh budget per keyID — every request from one IP also spends the shared
// per-IP bucket. This extractor stays per-keyID only (AC #1 keys by API key,
// not IP); the ceiling is a separate, outer limiter.
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

// agentFailureBucket counts one IP's Bearer 401 responses inside the
// current fixed window.
type agentFailureBucket struct {
	windowStart time.Time
	count       int
}

// statusCodeRecorder observes the status a downstream chain writes so an
// outer limiter can react to the outcome (auth failures) rather than the
// attempt.
type statusCodeRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusCodeRecorder) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush forwards to the underlying writer when it supports flushing, so
// wrapping the chain never strips streaming responses from handlers that
// assert http.Flusher.
func (w *statusCodeRecorder) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

type RateLimitMiddleware struct {
	enabled                bool
	authPerMinute          int
	apiPerMinute           int
	publicPerMinute        int
	agentFailuresPerMinute int
	mu                     sync.Mutex
	agentFailures          map[string]*agentFailureBucket
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

// agentFailureLimit resolves the effective per-IP auth-failure budget,
// falling back to the default when unset or misconfigured.
func (m *RateLimitMiddleware) agentFailureLimit() int {
	if m.agentFailuresPerMinute < 1 {
		return DefaultAgentFailuresPerMinute
	}
	return m.agentFailuresPerMinute
}

// reserveAgentFailureSlot spends one failure-budget slot for the IP ahead
// of serving the request, so concurrent floods cannot burst past the
// budget: only the first N concurrent requests hold a slot while the rest
// are refused. A non-401 outcome hands its slot back via
// releaseAgentFailureSlot, so successes never spend the budget. Expired
// windows reset on reservation.
func (m *RateLimitMiddleware) reserveAgentFailureSlot(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	bucket, ok := m.agentFailures[key]
	if !ok || now.Sub(bucket.windowStart) >= time.Minute {
		if !ok && len(m.agentFailures) >= agentFailureMapCap {
			m.sweepAgentFailures(now)
			if len(m.agentFailures) >= agentFailureMapCap {
				return true
			}
		}
		m.agentFailures[key] = &agentFailureBucket{windowStart: now, count: 1}
		return true
	}
	if bucket.count >= m.agentFailureLimit() {
		return false
	}
	bucket.count++
	return true
}

// releaseAgentFailureSlot hands back a reserved slot when the outcome was
// not a 401. An already-expired window is dropped instead of decremented.
func (m *RateLimitMiddleware) releaseAgentFailureSlot(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bucket, ok := m.agentFailures[key]
	if !ok {
		return
	}
	if time.Since(bucket.windowStart) >= time.Minute {
		delete(m.agentFailures, key)
		return
	}
	if bucket.count > 0 {
		bucket.count--
	}
}

// sweepAgentFailures drops expired windows. Callers must hold m.mu.
func (m *RateLimitMiddleware) sweepAgentFailures(now time.Time) {
	for key, bucket := range m.agentFailures {
		if now.Sub(bucket.windowStart) >= time.Minute {
			delete(m.agentFailures, key)
		}
	}
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

// AgentIPHandler applies a per-IP ceiling to the Bearer /api/v1 group, on top of
// the per-key budget enforced by APIKeyHandler. An attacker presenting many
// DISTINCT fake keyIDs shares this one IP bucket instead of getting a fresh
// budget for every keyID. The ceiling reuses apiPerMinute and the 1-minute
// window (the same budget a single key gets) and emits the v1 RATE_LIMITED code.
func (m *RateLimitMiddleware) AgentIPHandler(next http.Handler) http.Handler {
	if !m.enabled {
		return next
	}

	return httprate.Limit(
		m.apiPerMinute,
		time.Minute,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(m.apiKeyRateLimitHandler),
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

// AgentAuthFailureHandler is the outcome-based limiter for the Bearer /api/v1
// group: it counts 401 responses per client IP and refuses with the v1
// RATE_LIMITED code once the IP spends its auth-failure budget inside one
// minute. Unlike the attempt-based AgentIPHandler/APIKeyHandler (which run
// BEFORE auth and cap request volume), this handler wraps the whole chain
// INCLUDING auth, observes the outcome, and throttles attackers rotating
// DISTINCT fake keyIDs — every forgery returns 401 and spends the same
// shared IP budget, while successful keys never increment it. Mount it as
// the outermost Bearer middleware so it sees the final status.
func (m *RateLimitMiddleware) AgentAuthFailureHandler(next http.Handler) http.Handler {
	if !m.enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, err := httprate.KeyByIP(r)
		if err != nil {
			key = r.RemoteAddr
		}
		if !m.reserveAgentFailureSlot(key) {
			w.Header().Set("Retry-After", "60")
			m.apiKeyRateLimitHandler(w, r)
			return
		}
		recorder := &statusCodeRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		if recorder.status != http.StatusUnauthorized {
			m.releaseAgentFailureSlot(key)
		}
	})
}

// SetAgentFailuresPerMinute overrides the per-IP Bearer auth-failure budget
// (see DefaultAgentFailuresPerMinute). Values below 1 fall back to the
// default. Call it once at startup, before serving.
func (m *RateLimitMiddleware) SetAgentFailuresPerMinute(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n < 1 {
		n = DefaultAgentFailuresPerMinute
	}
	m.agentFailuresPerMinute = n
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
		enabled:                enabled,
		authPerMinute:          authPerMinute,
		apiPerMinute:           apiPerMinute,
		publicPerMinute:        publicPerMinute,
		agentFailuresPerMinute: DefaultAgentFailuresPerMinute,
		agentFailures:          make(map[string]*agentFailureBucket),
	}
}
