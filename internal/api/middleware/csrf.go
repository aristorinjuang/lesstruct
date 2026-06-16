package middleware

import (
	"net/http"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

type CSRFMiddleware struct {
	logger     *util.Logger
	cop        *http.CrossOriginProtection
	jwtManager *auth.JWTManager
}

func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if len(authHeader) > 7 && strings.HasPrefix(authHeader, "Bearer ") {
		return authHeader[7:]
	}
	return ""
}

func (m *CSRFMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet &&
			r.Method != http.MethodHead &&
			r.Method != http.MethodOptions {
			if err := m.cop.Check(r); err != nil {
				origin := r.Header.Get("Origin")
				if origin == "" {
					origin = "missing"
				}
				userID := "unauthenticated"
				if m.jwtManager != nil {
					if token := extractBearerToken(r); token != "" {
						if claims, err := m.jwtManager.ValidateToken(token); err == nil {
							userID = claims.UserID
						}
					}
				}
				m.logger.Info(
					"CSRF validation failed: method=%s path=%s origin=%s user_id=%s",
					r.Method,
					r.URL.Path,
					origin,
					userID,
				)
				response.Error(w, http.StatusForbidden, "CSRF_VALIDATION_FAILED", "Cross-origin request rejected", nil)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func NewCSRFMiddleware(
	logger *util.Logger,
	trustedOrigins []string,
	jwtManager *auth.JWTManager,
) *CSRFMiddleware {
	cop := http.NewCrossOriginProtection()
	for _, origin := range trustedOrigins {
		_ = cop.AddTrustedOrigin(origin)
	}
	return &CSRFMiddleware{
		logger:     logger,
		cop:        cop,
		jwtManager: jwtManager,
	}
}
