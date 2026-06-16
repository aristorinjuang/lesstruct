package middleware

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

// bearerScheme is the required Authorization scheme prefix for API-key requests.
const bearerScheme = "Bearer "

// extractBearer returns the token portion of an "Authorization: Bearer <token>"
// header, or "" when the header is absent or does not use the Bearer scheme.
func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" || !strings.HasPrefix(h, bearerScheme) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, bearerScheme))
}

// clientIP returns the request's remote host with the port stripped, for the
// best-effort last_used_ip column. It falls back to the raw RemoteAddr when the
// host:port split fails (e.g. a bare host or a malformed value).
func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

// APIKeyVerifier is the narrow interface the middleware depends on for key
// verification. *apikey.Service satisfies it (Verify + UpdateLastUsed). Defined
// here — not in the domain — so the middleware owns its own seam and the domain
// stays HTTP-agnostic (mirrors the handlers.APIKeyService pattern).
type APIKeyVerifier interface {
	Verify(ctx context.Context, fullKey string) (*apikey.APIKey, error)
	UpdateLastUsed(ctx context.Context, id int, ip string) error
}

// UserLookup is the narrow interface for resolving the owning user. The
// middleware needs only GetUserByID. repository.UserRepo satisfies it.
type UserLookup interface {
	GetUserByID(ctx context.Context, userID int) (*repository.User, error)
}

// APIKeyAuthMiddleware authenticates Bearer API-key tokens and injects the
// owning user into request context using the SAME keys the JWT middleware uses
// (UserIDKey/UsernameKey/RoleKey via newUserContext), so downstream ctxUser
// lookups and RBAC are auth-agnostic (architecture Important gap #1). It is
// mounted onto the Bearer-only /api/v1 group by Story 2.1.
type APIKeyAuthMiddleware struct {
	verifier   APIKeyVerifier
	userLookup UserLookup
	logger     *util.Logger
}

// writeAuthError maps a domain verification error to its 401 envelope code. Only
// the keyID (when known) is logged — never the secret or full key string. The
// mapping is exhaustive over the apikey.Verify error set; any unexpected error
// falls back to INVALID_API_KEY.
func (m *APIKeyAuthMiddleware) writeAuthError(w http.ResponseWriter, err error, key *apikey.APIKey) {
	keyID := ""
	if key != nil {
		keyID = key.KeyID
	}
	switch {
	case errors.Is(err, apikey.ErrKeyRevoked):
		m.logger.Info("API key rejected: reason=revoked keyID=%s", keyID)
		response.Error(w, http.StatusUnauthorized, "REVOKED_KEY", "API key has been revoked", nil)
	case errors.Is(err, apikey.ErrKeyExpired):
		m.logger.Info("API key rejected: reason=expired keyID=%s", keyID)
		response.Error(w, http.StatusUnauthorized, "EXPIRED_KEY", "API key has expired", nil)
	case errors.Is(err, apikey.ErrMalformedKey), errors.Is(err, apikey.ErrKeyNotFound):
		response.Error(w, http.StatusUnauthorized, "INVALID_API_KEY", "Invalid API key", nil)
	default:
		m.logger.Error("API key verification failed: keyID=%s err=%v", keyID, err)
		response.Error(w, http.StatusUnauthorized, "INVALID_API_KEY", "Invalid API key", nil)
	}
}

// Handler authenticates the request via Bearer API key and, on success, injects
// the owning user into context and delegates to next. On any failure it writes a
// 401 envelope with the correct code and does NOT call next.
func (m *APIKeyAuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearer(r)
		if token == "" {
			response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid Authorization header", nil)
			return
		}

		key, err := m.verifier.Verify(r.Context(), token)
		if err != nil {
			m.writeAuthError(w, err, key) // key is non-nil only for revoked/expired
			return
		}

		// Defensive: Verify's contract returns a non-nil key on success, but guard
		// against a faulty/mock implementation to avoid a nil-deref on key.UserID
		// (mirrors the nil-key guard in handlers.CreateAPIKey).
		if key == nil {
			m.logger.Error("API key verifier returned nil key without error")
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to authenticate API key", nil)
			return
		}

		user, err := m.userLookup.GetUserByID(r.Context(), key.UserID)
		if err != nil || user == nil {
			// A verified key whose owner is missing is a data-integrity problem.
			// Log the internal detail but return 401 INVALID_API_KEY so we do not
			// confirm the key was valid to the caller.
			m.logger.Error(
				"API key owner lookup failed: keyID=%s userID=%d err=%v",
				key.KeyID,
				key.UserID,
				err,
			)
			response.Error(w, http.StatusUnauthorized, "INVALID_API_KEY", "Invalid API key", nil)
			return
		}

		// Best-effort usage tracking. A failure here MUST NOT block an otherwise
		// valid authenticated request — log and continue.
		if err := m.verifier.UpdateLastUsed(r.Context(), key.ID, clientIP(r.RemoteAddr)); err != nil {
			m.logger.Error("Failed to update API key last_used: keyID=%s err=%v", key.KeyID, err)
		}

		m.logger.Info("API key authenticated: keyID=%s userID=%d", key.KeyID, user.ID)

		ctx := newUserContext(r.Context(), strconv.Itoa(user.ID), user.Username, user.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// NewAPIKeyAuthMiddleware constructs the middleware. A nil logger degrades to a
// discard sink (mirrors NewNoCookieMiddleware) so the middleware is safe to
// construct in any context.
func NewAPIKeyAuthMiddleware(
	verifier APIKeyVerifier,
	userLookup UserLookup,
	logger *util.Logger,
) *APIKeyAuthMiddleware {
	if logger == nil {
		logger = util.NewLogger(io.Discard)
	}
	return &APIKeyAuthMiddleware{
		verifier:   verifier,
		userLookup: userLookup,
		logger:     logger,
	}
}
