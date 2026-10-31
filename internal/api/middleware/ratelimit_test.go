package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	appmiddleware "github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	"github.com/stretchr/testify/assert"
)

func TestRateLimitMiddleware_RequestsUnderLimit(t *testing.T) {
	middleware := appmiddleware.NewRateLimitMiddleware(true, 5, 100, 60)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRateLimitMiddleware_RequestsOverLimit_Returns429(t *testing.T) {
	middleware := appmiddleware.NewRateLimitMiddleware(true, 3, 3, 3)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler(nextHandler)

	for range 3 {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))
}

func TestRateLimitMiddleware_SubsequentRequestsDuringCooldown(t *testing.T) {
	middleware := appmiddleware.NewRateLimitMiddleware(true, 2, 2, 2)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler(nextHandler)

	for range 2 {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}

	for range 3 {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	}
}

func TestRateLimitMiddleware_DifferentLimitsForDifferentHandlers(t *testing.T) {
	middleware := appmiddleware.NewRateLimitMiddleware(true, 2, 5, 3)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		getHandler func() http.Handler
		limit      int
	}{
		{
			name:       "auth handler respects auth limit",
			getHandler: func() http.Handler { return middleware.AuthHandler(nextHandler) },
			limit:      2,
		},
		{
			name:       "public handler respects public limit",
			getHandler: func() http.Handler { return middleware.PublicHandler(nextHandler) },
			limit:      3,
		},
		{
			name:       "api handler respects api limit",
			getHandler: func() http.Handler { return middleware.Handler(nextHandler) },
			limit:      5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := tt.getHandler()

			for range tt.limit {
				req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
				req.RemoteAddr = "10.0.0.1:1234"
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				assert.Equal(t, http.StatusOK, rr.Code)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.RemoteAddr = "10.0.0.1:1234"
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusTooManyRequests, rr.Code)
		})
	}
}

func TestRateLimitMiddleware_PerIPIsolation(t *testing.T) {
	middleware := appmiddleware.NewRateLimitMiddleware(true, 2, 2, 2)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler(nextHandler)

	for range 2 {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "192.168.1.2:5678"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "different IP should have independent counter")
}

func TestRateLimitMiddleware_DisabledPassthrough(t *testing.T) {
	middleware := appmiddleware.NewRateLimitMiddleware(false, 1, 1, 1)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler(nextHandler)

	for range 50 {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "disabled middleware should pass all requests through")
	}
}

func TestRateLimitMiddleware_ResponseFormat(t *testing.T) {
	middleware := appmiddleware.NewRateLimitMiddleware(true, 1, 1, 1)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	req = httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var resp response.Response
	err := json.NewDecoder(rr.Body).Decode(&resp)
	assert.NoError(t, err)

	errInfo, ok := resp.Error.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "RATE_LIMIT_EXCEEDED", errInfo["code"])
	assert.Equal(t, "Too many requests. Please try again later.", errInfo["message"])
}

func TestRateLimitMiddleware_RateLimitHeaders(t *testing.T) {
	middleware := appmiddleware.NewRateLimitMiddleware(true, 5, 5, 5)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Limit"))
	assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Reset"))
}

func TestRateLimitMiddleware_DisabledAuthHandlerPassthrough(t *testing.T) {
	middleware := appmiddleware.NewRateLimitMiddleware(false, 1, 1, 1)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.AuthHandler(nextHandler)

	for range 20 {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}
}

func TestRateLimitMiddleware_DisabledPublicHandlerPassthrough(t *testing.T) {
	middleware := appmiddleware.NewRateLimitMiddleware(false, 1, 1, 1)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.PublicHandler(nextHandler)

	for range 20 {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/content_items", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}
}

func TestRateLimitMiddleware_HealthEndpointNotRateLimited(t *testing.T) {
	middleware := appmiddleware.NewRateLimitMiddleware(true, 1, 1, 1)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "second request should be rate-limited")

	for range 5 {
		healthHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rr := httptest.NewRecorder()
		healthHandler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "health endpoint bypasses rate limiter")
	}
}

// apiKeyBearer builds a Bearer Authorization header value for a presented API key
// "lesstruct_<keyID>_<secret>", mirroring the on-the-wire format the extractor parses.
func apiKeyBearer(keyID string) string {
	return "Bearer " + apikey.KeyPrefix + keyID + "_" + testKeySecret
}

const testKeySecret = "deadbeefdeadbeefdeadbeefdeadbeef"

// TestRateLimitMiddleware_APIKeyHandler_PerKeyIsolation proves the API-key bucket is
// keyed by the presented keyID (not IP): two distinct keyIDs sent from the SAME
// client IP each get their own independent apiPerMinute budget.
func TestRateLimitMiddleware_APIKeyHandler_PerKeyIsolation(t *testing.T) {
	mw := appmiddleware.NewRateLimitMiddleware(true, 1, 2, 1)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw.APIKeyHandler(nextHandler)

	const (
		keyAlpha = "abcdef123456"
		keyBeta  = "fedcba654321"
	)

	doRequest := func(keyID string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/content", nil)
		// Same client IP for both keys — isolation must come from the keyID, not IP.
		req.RemoteAddr = "203.0.113.7:5555"
		req.Header.Set("Authorization", apiKeyBearer(keyID))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	// Alpha key exhausts its budget of 2, then is throttled.
	assert.Equal(t, http.StatusOK, doRequest(keyAlpha))
	assert.Equal(t, http.StatusOK, doRequest(keyAlpha))
	assert.Equal(t, http.StatusTooManyRequests, doRequest(keyAlpha), "alpha key exhausted")

	// Beta key still has its full, independent budget from the same IP.
	assert.Equal(t, http.StatusOK, doRequest(keyBeta), "beta key must have an independent budget")
	assert.Equal(t, http.StatusOK, doRequest(keyBeta), "beta key second request still under its own limit")
}

// TestRateLimitMiddleware_APIKeyHandler_IPFallback proves that when no Bearer
// header is present, the bucket falls back to the client IP — so two different IPs
// without keys get independent budgets (the same behavior as the browser Handler).
func TestRateLimitMiddleware_APIKeyHandler_IPFallback(t *testing.T) {
	mw := appmiddleware.NewRateLimitMiddleware(true, 1, 1, 1)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw.APIKeyHandler(nextHandler)

	doRequest := func(remoteAddr string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/content", nil)
		req.RemoteAddr = remoteAddr
		// No Authorization header → keyByAPIKeyOrIP must fall back to IP.
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	assert.Equal(t, http.StatusOK, doRequest("198.51.100.1:1111"))
	assert.Equal(t, http.StatusTooManyRequests, doRequest("198.51.100.1:1111"), "same IP exhausted")
	assert.Equal(t, http.StatusOK, doRequest("198.51.100.2:2222"), "different IP gets its own budget")
}

// TestRateLimitMiddleware_AgentIPHandler_Ceiling proves the per-IP ceiling closes
// the key-rotation gap: distinct fake keyIDs from one IP each get their own
// per-key bucket, but share the per-IP ceiling, so rotating keys are throttled
// once the IP budget is spent.
func TestRateLimitMiddleware_AgentIPHandler_Ceiling(t *testing.T) {
	mw := appmiddleware.NewRateLimitMiddleware(true, 1, 2, 1)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// Mirror the production chain: per-IP ceiling outside the per-key limiter.
	handler := mw.AgentIPHandler(mw.APIKeyHandler(nextHandler))

	doRequest := func(keyID, remoteAddr string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/export", nil)
		req.RemoteAddr = remoteAddr
		req.Header.Set("Authorization", apiKeyBearer(keyID))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	// Two distinct keyIDs each have their own per-key budget, but both share
	// the IP ceiling of 2.
	assert.Equal(t, http.StatusOK, doRequest("aaaaaaaaaaaa", "203.0.113.30:5555"))
	assert.Equal(t, http.StatusOK, doRequest("bbbbbbbbbbbb", "203.0.113.30:5555"))
	assert.Equal(t, http.StatusTooManyRequests, doRequest("cccccccccccc", "203.0.113.30:5555"),
		"a third distinct keyID from the same IP must be throttled by the IP ceiling")

	// A different IP is unaffected.
	assert.Equal(t, http.StatusOK, doRequest("dddddddddddd", "203.0.113.31:5555"))
}

// TestRateLimitMiddleware_APIKeyHandler_DisabledPassthrough proves a disabled
// middleware passes every request through regardless of key/IP (parity with the
// other handlers' disabled behavior).
func TestRateLimitMiddleware_APIKeyHandler_DisabledPassthrough(t *testing.T) {
	mw := appmiddleware.NewRateLimitMiddleware(false, 1, 1, 1)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw.APIKeyHandler(nextHandler)

	for range 30 {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/content", nil)
		req.RemoteAddr = "203.0.113.9:9999"
		req.Header.Set("Authorization", apiKeyBearer("abcdef123456"))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "disabled API-key limiter should pass all requests through")
	}
}

// TestRateLimitMiddleware_APIKeyHandler_ResponseFormat proves the per-key limit
// handler emits the v1 RATE_LIMITED envelope code (NOT the browser group's
// RATE_LIMIT_EXCEEDED).
func TestRateLimitMiddleware_APIKeyHandler_ResponseFormat(t *testing.T) {
	mw := appmiddleware.NewRateLimitMiddleware(true, 1, 1, 1)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw.APIKeyHandler(nextHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/content", nil)
	req.RemoteAddr = "203.0.113.20:1234"
	req.Header.Set("Authorization", apiKeyBearer("abcdef123456"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Second request from the same key exhausts the budget → RATE_LIMITED.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/content", nil)
	req.RemoteAddr = "203.0.113.20:1234"
	req.Header.Set("Authorization", apiKeyBearer("abcdef123456"))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var resp response.Response
	err := json.NewDecoder(rr.Body).Decode(&resp)
	assert.NoError(t, err)

	errInfo, ok := resp.Error.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "RATE_LIMITED", errInfo["code"], "API-key path must emit RATE_LIMITED, not RATE_LIMIT_EXCEEDED")
	assert.Equal(t, "Too many requests. Please try again later.", errInfo["message"])
}

// TestRateLimitMiddleware_AgentAuthFailureHandler proves the outcome-based
// Bearer limiter counts 401 responses per client IP: failures under the
// budget pass through, the first failure past the budget is refused with
// 429 RATE_LIMITED, and non-401 responses never spend the budget — so fake
// API keys are throttled no matter how often the keyID rotates, while valid
// keys and successful requests are unaffected.
func TestRateLimitMiddleware_AgentAuthFailureHandler(t *testing.T) {
	tests := []struct {
		name         string
		threshold    int
		enabled      bool
		nextStatus   int
		requests     int
		wantCode     int
		wantEnvelope string
	}{
		{
			name:       "success - failures under threshold pass",
			threshold:  3,
			enabled:    true,
			nextStatus: http.StatusUnauthorized,
			requests:   3,
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:         "error - failure past threshold is refused",
			threshold:    3,
			enabled:      true,
			nextStatus:   http.StatusUnauthorized,
			requests:     4,
			wantCode:     http.StatusTooManyRequests,
			wantEnvelope: "RATE_LIMITED",
		},
		{
			name:       "success - successful responses never count",
			threshold:  2,
			enabled:    true,
			nextStatus: http.StatusOK,
			requests:   30,
			wantCode:   http.StatusOK,
		},
		{
			name:       "success - non-401 statuses never count",
			threshold:  2,
			enabled:    true,
			nextStatus: http.StatusForbidden,
			requests:   10,
			wantCode:   http.StatusForbidden,
		},
		{
			name:       "success - default threshold allows fifteen failures",
			threshold:  0,
			enabled:    true,
			nextStatus: http.StatusUnauthorized,
			requests:   appmiddleware.DefaultAgentFailuresPerMinute,
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:         "error - failure past default threshold is refused",
			threshold:    0,
			enabled:      true,
			nextStatus:   http.StatusUnauthorized,
			requests:     appmiddleware.DefaultAgentFailuresPerMinute + 1,
			wantCode:     http.StatusTooManyRequests,
			wantEnvelope: "RATE_LIMITED",
		},
		{
			name:       "success - disabled middleware passes failures through",
			threshold:  0,
			enabled:    false,
			nextStatus: http.StatusUnauthorized,
			requests:   20,
			wantCode:   http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := appmiddleware.NewRateLimitMiddleware(tt.enabled, 1, 1, 1)
			if tt.threshold > 0 {
				mw.SetAgentFailuresPerMinute(tt.threshold)
			}
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.nextStatus)
			})
			handler := mw.AgentAuthFailureHandler(nextHandler)

			var lastCode int
			var lastBody *httptest.ResponseRecorder
			for range tt.requests {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/export", nil)
				req.RemoteAddr = "203.0.113.44:5555"
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				lastCode = rr.Code
				lastBody = rr
			}

			assert.Equal(t, tt.wantCode, lastCode)
			if tt.wantEnvelope != "" {
				var resp response.Response
				assert.NoError(t, json.NewDecoder(lastBody.Body).Decode(&resp))
				errInfo, ok := resp.Error.(map[string]any)
				assert.True(t, ok)
				assert.Equal(t, tt.wantEnvelope, errInfo["code"])
				assert.Equal(t, "Too many requests. Please try again later.", errInfo["message"])
				assert.Equal(t, "60", lastBody.Header().Get("Retry-After"), "failure refusals must carry a backoff hint")
			}
		})
	}
}

// TestRateLimitMiddleware_AgentAuthFailureHandler_Sequences proves the
// failure budget is per-IP and outcome-shaped: a tripped IP is refused
// while another IP still passes, and successful responses interleaved
// between failures neither spend the budget nor reset it.
func TestRateLimitMiddleware_AgentAuthFailureHandler_Sequences(t *testing.T) {
	const (
		ipA = "203.0.113.44:5555"
		ipB = "203.0.113.45:5555"
	)

	tests := []struct {
		name      string
		threshold int
		statuses  []int
		addrs     []string
		wantCodes []int
	}{
		{
			name:      "tripped IP is refused while another IP passes",
			threshold: 2,
			statuses:  []int{http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized},
			addrs:     []string{ipA, ipA, ipA, ipB},
			wantCodes: []int{http.StatusUnauthorized, http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusUnauthorized},
		},
		{
			name:      "interleaved successes neither spend nor reset the budget",
			threshold: 2,
			statuses:  []int{http.StatusUnauthorized, http.StatusOK, http.StatusUnauthorized, http.StatusUnauthorized},
			addrs:     []string{ipA, ipA, ipA, ipA},
			wantCodes: []int{http.StatusUnauthorized, http.StatusOK, http.StatusUnauthorized, http.StatusTooManyRequests},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := appmiddleware.NewRateLimitMiddleware(true, 1, 1, 1)
			mw.SetAgentFailuresPerMinute(tt.threshold)
			var statusIdx int
			var statuses []int
			handler := mw.AgentAuthFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(statuses[statusIdx])
				statusIdx++
			}))
			statuses = tt.statuses

			var gotCodes []int
			for i := range tt.statuses {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/export", nil)
				req.RemoteAddr = tt.addrs[i]
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				gotCodes = append(gotCodes, rr.Code)
			}

			assert.Equal(t, tt.wantCodes, gotCodes)
		})
	}
}

// TestRateLimitMiddleware_AgentAuthFailureHandler_ConcurrentBurst proves the
// failure budget holds under concurrency: with many goroutines racing one
// IP, exactly the budgeted requests reach the downstream handler (slots are
// reserved atomically ahead of serving) and the rest are refused.
func TestRateLimitMiddleware_AgentAuthFailureHandler_ConcurrentBurst(t *testing.T) {
	tests := []struct {
		name       string
		threshold  int
		racers     int
		wantPassed int
	}{
		{
			name:       "fifty racers share a budget of five",
			threshold:  5,
			racers:     50,
			wantPassed: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := appmiddleware.NewRateLimitMiddleware(true, 1, 1, 1)
			mw.SetAgentFailuresPerMinute(tt.threshold)
			var passed atomic.Int32
			handler := mw.AgentAuthFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				passed.Add(1)
				w.WriteHeader(http.StatusUnauthorized)
			}))

			codes := make([]int, tt.racers)
			var wg sync.WaitGroup
			for i := range tt.racers {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					req := httptest.NewRequest(http.MethodGet, "/api/v1/export", nil)
					req.RemoteAddr = "203.0.113.44:5555"
					rr := httptest.NewRecorder()
					handler.ServeHTTP(rr, req)
					codes[i] = rr.Code
				}(i)
			}
			wg.Wait()

			assert.Equal(t, int32(tt.wantPassed), passed.Load(), "exactly the budgeted requests may reach the handler")
			refused := 0
			for _, code := range codes {
				if code == http.StatusTooManyRequests {
					refused++
				}
			}
			assert.Equal(t, tt.racers-tt.wantPassed, refused, "every racer past the budget must be refused")
		})
	}
}
