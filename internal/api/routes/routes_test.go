package routes_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/api/handlers/agent"
	agentmocks "github.com/aristorinjuang/lesstruct/internal/api/handlers/agent/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	mwmocks "github.com/aristorinjuang/lesstruct/internal/api/middleware/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/routes"
	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	authdomain "github.com/aristorinjuang/lesstruct/internal/domain/auth"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	contentmocks "github.com/aristorinjuang/lesstruct/internal/domain/content/mocks"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
	"github.com/aristorinjuang/lesstruct/internal/domain/user"
	emailmocks "github.com/aristorinjuang/lesstruct/internal/email/mocks"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	repomocks "github.com/aristorinjuang/lesstruct/internal/repository/mocks"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(t *testing.T) *chi.Mux {
	t.Helper()
	r, _ := setupTestRouterWithBearerDeps(t)
	return r
}

// bearerDeps holds the Bearer-group mock dependencies a test can configure to drive
// the /api/v1/content routes through the real middleware + handler stack.
type bearerDeps struct {
	verifier   *mwmocks.MockAPIKeyVerifier
	user       *mwmocks.MockUserLookup
	content    *agentmocks.MockContentService
	media      *agentmocks.MockMediaService
	jwtManager *appauth.JWTManager
}

// setupTestRouterWithBearerDeps builds the full routes.Setup router and also returns
// the mock dependencies wired into the Bearer /api/v1 group, so Bearer route tests
// can configure the verifier/user/content mocks per-case. The shared setupTestRouter
// delegates here and discards the mocks (existing tests do not hit Bearer routes, so
// the no-expectation mocks are never invoked).
func setupTestRouterWithBearerDeps(t *testing.T) (*chi.Mux, bearerDeps) {
	t.Helper()

	// Create test auth handler
	hash, err := appauth.HashPassword("admin")
	require.NoError(t, err, "Failed to hash password")

	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)
	registrationService := authdomain.NewRegistrationService(nil)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)
	mockUserRepo := repomocks.NewMockUserRepo(t)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	loginService := authdomain.NewLoginService(mockUserRepo, repomocks.NewMockFailedLoginAttemptRepo(t), nil)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(mockUserRepo, passwordResetTokenRepo, 1)
	authHandler := handlers.NewAuthHandler(
		authService,
		jwtManager,
		logger,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		mockUserRepo,
		failedLoginRepo,
		notificationRepo,
		emailService,
		blockedEmailRepo,
	)
	firstLoginHandler := handlers.NewFirstLoginHandler(firstLoginService, mockUserRepo, logger)
	notificationHandler := handlers.NewNotificationHandler(logger, notificationRepo)

	userManagementService := user.NewUserManagementService(mockUserRepo, blockedEmailRepo)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	userManagementHandler := handlers.NewUserManagementHandler(userManagementService, nil, mockUserRepo, softDeleteRepo, jwtManager, emailService, logger, nil)

	// Initialize profile management dependencies (Story 1.7)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	profileService := user.NewProfileService(mockUserRepo, emailUpdateTokenRepo, userDataExportRepo, emailService, nil)
	// Initialize account deletion service (Story 1.8)
	userDeletionRepo := repomocks.NewMockUserDeletionRepo(t)
	accountDeletionService := user.NewAccountDeletionService(mockUserRepo, userDeletionRepo, emailService, logger)

	profileHandler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	authMiddleware := middleware.NewAuthMiddleware(jwtManager)
	adminMiddleware := middleware.NewAdminMiddleware(authMiddleware)

	// Initialize content handler with mock service (Story 2.1)
	contentHandler := handlers.NewContentHandler(mocks.NewMockContentServiceInterface(t), logger, "http://localhost:3000")

	// Initialize media handler with mock service (Story 2.3)
	mediaHandler := handlers.NewMediaHandler(mocks.NewMockMediaServiceInterface(t), nil, logger)

	// Initialize post type handler (Story 2.6)
	postTypeHandler := handlers.NewPostTypeHandler(posttype.NewService(), logger)
	// Initialize dashboard handler (Story 2.8)
	dashboardHandler := handlers.NewDashboardHandler(mocks.NewMockDashboardServiceInterface(t), logger)

	corsMiddleware := middleware.NewCORSMiddleware([]string{"http://localhost:8080"}, logger)
	csrfMiddleware := middleware.NewCSRFMiddleware(logger, nil, nil)

	rateLimitMiddleware := middleware.NewRateLimitMiddleware(false, 5, 100, 60)

	// Initialize SEO handler (Story 4.2)
	seoHandler := handlers.NewSEOHandler(mocks.NewMockContentServiceInterface(t), "http://localhost:3000", logger)

	// Initialize comment handler (Story 4.7)
	commentHandler := handlers.NewCommentHandler(contentmocks.NewMockServiceInterface(t))

	// Initialize Bearer-group dependencies (Story 2.1): real middleware + handler
	// wired to mock interfaces so Bearer route tests can configure them per-case.
	verifier := mwmocks.NewMockAPIKeyVerifier(t)
	userLookup := mwmocks.NewMockUserLookup(t)
	apiKeyAuthMiddleware := middleware.NewAPIKeyAuthMiddleware(verifier, userLookup, logger)
	contentSvc := agentmocks.NewMockContentService(t)
	agentContentHandler := agent.NewContentHandler(contentSvc, nil, logger)
	mediaSvc := agentmocks.NewMockMediaService(t)
	agentMediaHandler := agent.NewMediaHandler(mediaSvc, logger)
	commentSvc := agentmocks.NewMockCommentService(t)
	agentCommentHandler := agent.NewCommentHandler(commentSvc, logger)

	deps := bearerDeps{
		verifier:   verifier,
		user:       userLookup,
		content:    contentSvc,
		media:      mediaSvc,
		jwtManager: jwtManager,
	}

	r := routes.Setup(
		authHandler,
		firstLoginHandler,
		notificationHandler,
		userManagementHandler,
		profileHandler,
		contentHandler,
		mediaHandler,
		postTypeHandler,
		dashboardHandler,
		seoHandler,
		commentHandler,
		nil,
		nil,
		nil,
		apiKeyAuthMiddleware,
		agentContentHandler,
		agentMediaHandler,
		agentCommentHandler,
		adminMiddleware,
		corsMiddleware,
		middleware.NewNoCookieMiddleware(logger),
		csrfMiddleware,
		rateLimitMiddleware,
		jwtManager,
		nil,
		nil,
		false,
		false,
		nil,
		[]string{"en"},
	)
	return r, deps
}

func TestSetup(t *testing.T) {
	hash, err := appauth.HashPassword("admin")
	require.NoError(t, err, "Failed to hash password")

	authService := authdomain.NewAuthService(hash)
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(hash)
	registrationService := authdomain.NewRegistrationService(nil)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)
	mockUserRepo := repomocks.NewMockUserRepo(t)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	loginService := authdomain.NewLoginService(mockUserRepo, repomocks.NewMockFailedLoginAttemptRepo(t), nil)
	failedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(mockUserRepo, passwordResetTokenRepo, 1)
	authHandler := handlers.NewAuthHandler(authService, jwtManager, logger, firstLoginService, registrationService, verificationService, loginService, passwordResetService, mockUserRepo, failedLoginRepo, notificationRepo, emailService, blockedEmailRepo)
	firstLoginHandler := handlers.NewFirstLoginHandler(firstLoginService, mockUserRepo, logger)
	notificationHandler := handlers.NewNotificationHandler(logger, notificationRepo)

	userManagementService := user.NewUserManagementService(mockUserRepo, blockedEmailRepo)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	userManagementHandler := handlers.NewUserManagementHandler(userManagementService, nil, mockUserRepo, softDeleteRepo, jwtManager, emailService, logger, nil)

	// Initialize profile management dependencies (Story 1.7)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	profileService := user.NewProfileService(mockUserRepo, emailUpdateTokenRepo, userDataExportRepo, emailService, nil)
	// Initialize account deletion service (Story 1.8)
	userDeletionRepo := repomocks.NewMockUserDeletionRepo(t)
	accountDeletionService := user.NewAccountDeletionService(mockUserRepo, userDeletionRepo, emailService, logger)

	profileHandler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, logger, nil, nil)

	authMiddleware := middleware.NewAuthMiddleware(jwtManager)
	adminMiddleware := middleware.NewAdminMiddleware(authMiddleware)
	corsMiddleware := middleware.NewCORSMiddleware([]string{"http://localhost:8080"}, logger)
	csrfMiddleware := middleware.NewCSRFMiddleware(logger, nil, nil)

	// Initialize content handler with mock service (Story 2.1)
	contentHandler := handlers.NewContentHandler(mocks.NewMockContentServiceInterface(t), logger, "http://localhost:3000")

	// Initialize media handler with mock service (Story 2.3)
	mediaHandler := handlers.NewMediaHandler(mocks.NewMockMediaServiceInterface(t), nil, logger)

	// Initialize post type handler (Story 2.6)
	postTypeHandler := handlers.NewPostTypeHandler(posttype.NewService(), logger)
	// Initialize dashboard handler (Story 2.8)
	dashboardHandler := handlers.NewDashboardHandler(mocks.NewMockDashboardServiceInterface(t), logger)

	rateLimitMiddleware := middleware.NewRateLimitMiddleware(false, 5, 100, 60)

	// Initialize SEO handler (Story 4.2)
	seoHandler := handlers.NewSEOHandler(mocks.NewMockContentServiceInterface(t), "http://localhost:3000", logger)

	// Initialize comment handler (Story 4.7)
	commentHandler := handlers.NewCommentHandler(contentmocks.NewMockServiceInterface(t))

	// Initialize Bearer-group dependencies (Story 2.1)
	apiKeyAuthMiddleware := middleware.NewAPIKeyAuthMiddleware(
		mwmocks.NewMockAPIKeyVerifier(t),
		mwmocks.NewMockUserLookup(t),
		logger,
	)
	agentContentHandler := agent.NewContentHandler(agentmocks.NewMockContentService(t), nil, logger)
	agentMediaHandler := agent.NewMediaHandler(agentmocks.NewMockMediaService(t), logger)
	agentCommentHandler := agent.NewCommentHandler(agentmocks.NewMockCommentService(t), logger)

	r := routes.Setup(
		authHandler,
		firstLoginHandler,
		notificationHandler,
		userManagementHandler,
		profileHandler,
		contentHandler,
		mediaHandler,
		postTypeHandler,
		dashboardHandler,
		seoHandler,
		commentHandler,
		nil,
		nil,
		nil,
		apiKeyAuthMiddleware,
		agentContentHandler,
		agentMediaHandler,
		agentCommentHandler,
		adminMiddleware,
		corsMiddleware,
		middleware.NewNoCookieMiddleware(logger),
		csrfMiddleware,
		rateLimitMiddleware,
		jwtManager,
		nil,
		nil,
		false,
		false,
		nil,
		[]string{"en"},
	)

	assert.NotNil(t, r, "Setup() returned nil router")
}

func TestHealthRoute(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Health route status")

	assert.Contains(t, w.Body.String(), `"status":"ok"`, "Health route body should contain status ok")
	assert.Contains(t, w.Body.String(), `"features":{"imageGeneration":false,"textGeneration":false}`, "Health route body should contain features")
}

func TestLoginRoute_ValidRequest(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should return bad request due to empty body
	assert.Equal(t, http.StatusBadRequest, w.Code, "Login route status")
}

func TestLoginRoute_MethodNotAllowed(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code, "Login route GET status")
}

func TestAuthRoutes_ExemptFromCSRF(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "Auth routes should not be protected by CSRF — should return 400 for empty body, not 403")
}

func TestHealthRoute_ExemptFromCSRF(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Health route should not be protected by CSRF")
}

func TestPublicContentRoutes_ExemptFromCSRF(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/posts", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusForbidden, w.Code, "Public content routes should not be blocked by CSRF")
}

func TestSecurityHeaders(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"), "X-Frame-Options should be DENY")
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"), "X-Content-Type-Options should be nosniff")
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"), "X-XSS-Protection should be 1; mode=block")
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"), "Referrer-Policy should be set")
}

func TestContentSecurityPolicyHeader(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	assert.NotEmpty(t, csp, "Content-Security-Policy header should be set")
	assert.Contains(t, csp, "default-src 'self'", "CSP should contain default-src 'self'")
	assert.Contains(t, csp, "script-src 'self'", "CSP should contain script-src 'self'")
	assert.Contains(t, csp, "frame-ancestors 'none'", "CSP should contain frame-ancestors 'none'")
	assert.Contains(t, csp, "form-action 'self'", "CSP should contain form-action 'self'")
}

// envelopeCode performs a request and returns the response's envelope error code
// (or "" when the response carries no error).
func envelopeCode(t *testing.T, r http.Handler, req *http.Request) (int, string) {
	t.Helper()
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	var resp struct {
		Error any `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		return rr.Code, ""
	}
	errInfo, ok := resp.Error.(map[string]any)
	if !ok {
		return rr.Code, ""
	}
	code, _ := errInfo["code"].(string)
	return rr.Code, code
}

// Bearer-route fixtures mirroring the on-the-wire API key format.
const (
	testBearerKeyID  = "abcdef123456"
	testBearerSecret = "deadbeefdeadbeefdeadbeefdeadbeef"
	testBearerToken  = "lesstruct_" + testBearerKeyID + "_" + testBearerSecret
	testBearerUserID = 42
)

// TestBearerAPIRoutes_NoBearerReturns401 proves a request with no Authorization
// header is rejected by the Bearer middleware (401 UNAUTHORIZED) before any handler.
func TestBearerAPIRoutes_NoBearerReturns401(t *testing.T) {
	r, _ := setupTestRouterWithBearerDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/content", nil)
	code, envelope := envelopeCode(t, r, req)

	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "UNAUTHORIZED", envelope)
}

// TestBearerAPIRoutes_InvalidBearerReturnsInvalidAPIKey proves an unverifiable
// Bearer token maps to 401 INVALID_API_KEY (no secret/ownership disclosure).
func TestBearerAPIRoutes_InvalidBearerReturnsInvalidAPIKey(t *testing.T) {
	r, deps := setupTestRouterWithBearerDeps(t)
	deps.verifier.EXPECT().Verify(mock.Anything, "lesstruct_badkey_badsecret").
		Return(nil, apikey.ErrKeyNotFound)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/content", nil)
	req.Header.Set("Authorization", "Bearer lesstruct_badkey_badsecret")
	code, envelope := envelopeCode(t, r, req)

	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "INVALID_API_KEY", envelope)
}

// TestBearerAPIRoutes_ValidKeyReachesAgentHandler proves a valid Bearer key
// authenticates through the middleware and reaches the agent handler, which returns
// 200 with the content in the envelope. This exercises the full
// middleware→handler→service path.
func TestBearerAPIRoutes_ValidKeyReachesAgentHandler(t *testing.T) {
	r, deps := setupTestRouterWithBearerDeps(t)

	verifiedKey := &apikey.APIKey{ID: 7, UserID: testBearerUserID, KeyID: testBearerKeyID}
	owningUser := &repository.User{ID: testBearerUserID, Username: "alice", Role: "Editor"}

	deps.verifier.EXPECT().Verify(mock.Anything, testBearerToken).Return(verifiedKey, nil)
	deps.verifier.EXPECT().UpdateLastUsed(mock.Anything, 7, mock.Anything).Return(nil)
	deps.user.EXPECT().GetUserByID(mock.Anything, testBearerUserID).Return(owningUser, nil)
	deps.content.EXPECT().GetByID(mock.Anything, 123).
		Return(&contentdomain.Content{
			ID: 123, UserID: testBearerUserID, Status: contentdomain.StatusPublished, Title: "Hello",
		}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/content/123", nil)
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"id":123`, "response should contain the fetched content")
}

// TestBearerAPIRoutes_NotCSRFProtected proves the Bearer /api/v1 group is NOT
// protected by CSRF: a cross-site request (which the browser CSRF middleware would
// 403) is instead handled by the Bearer auth layer (401 UNAUTHORIZED). A 403 here
// would mean CSRF was wrongly applied to the group.
func TestBearerAPIRoutes_NotCSRFProtected(t *testing.T) {
	r, _ := setupTestRouterWithBearerDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/content", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	code, _ := envelopeCode(t, r, req)

	assert.NotEqual(t, http.StatusForbidden, code, "Bearer group must NOT be CSRF-protected")
	assert.Equal(t, http.StatusUnauthorized, code, "auth layer (not CSRF) handles the request")
}

// TestBrowserContentRoutes_StillRequireJWT proves the existing browser-realm route
// POST /api/v1/content_items is unaffected by the new Bearer group: a same-origin
// request with no auth still hits RequireAuth (401 MISSING_TOKEN), not the Bearer
// middleware's UNAUTHORIZED — confirming the two groups are wired independently.
func TestBrowserContentRoutes_StillRequireJWT(t *testing.T) {
	r, _ := setupTestRouterWithBearerDeps(t)

	// Same-origin (no Origin/Sec-Fetch-Site) so CSRF passes; no auth → RequireAuth.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/content_items", nil)
	code, envelope := envelopeCode(t, r, req)

	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "MISSING_TOKEN", envelope, "browser route must still be gated by RequireAuth, not the Bearer middleware")
}

// TestBearerAPIRoutes_NoRouteCollision proves the three POST routes under /api/v1
// resolve to distinct handlers with no Chi collision:
//   - POST /api/v1/content        → Bearer middleware  (UNAUTHORIZED)
//   - POST /api/v1/content_items  → browser RequireAuth (MISSING_TOKEN)
//   - POST /api/v1/content/slug   → browser RequireAuth (MISSING_TOKEN)
// Each returns 401 (route matched) with its own middleware's code (not 404/405).
func TestBearerAPIRoutes_NoRouteCollision(t *testing.T) {
	r, _ := setupTestRouterWithBearerDeps(t)

	tests := []struct {
		name     string
		path     string
		wantCode string
	}{
		{
			name:     "POST /api/v1/content reaches Bearer middleware",
			path:     "/api/v1/content",
			wantCode: "UNAUTHORIZED",
		},
		{
			name:     "POST /api/v1/content_items reaches browser RequireAuth",
			path:     "/api/v1/content_items",
			wantCode: "MISSING_TOKEN",
		},
		{
			name:     "POST /api/v1/content/slug reaches browser RequireAuth",
			path:     "/api/v1/content/slug",
			wantCode: "MISSING_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			code, envelope := envelopeCode(t, r, req)

			assert.NotEqual(t, http.StatusNotFound, code, "%s must match a route", tt.path)
			assert.NotEqual(t, http.StatusMethodNotAllowed, code, "%s must accept POST", tt.path)
			assert.Equal(t, http.StatusUnauthorized, code)
			assert.Equal(t, tt.wantCode, envelope)
		})
	}
}

// TestBearerAPIRoutes_ListUpdateDeleteAuthAndNoCollision proves the three new agent verbs
// (GET /api/v1/content, PUT/DELETE /api/v1/content/{id}) require a valid Bearer key
// (no key → 401 UNAUTHORIZED via the Bearer middleware), and that every agent
// /api/v1/content* leaf resolves distinctly from the browser-realm
// /api/v1/content_items* and /api/v1/content/slug leaves (which reach the JWT
// RequireAuth middleware → MISSING_TOKEN). Each returns 401 with its own middleware's
// code (never 404/405), confirming no Chi route collision across all content* leaves.
func TestBearerAPIRoutes_ListUpdateDeleteAuthAndNoCollision(t *testing.T) {
	r, _ := setupTestRouterWithBearerDeps(t)

	tests := []struct {
		name     string
		method   string
		path     string
		wantCode string
	}{
		// Agent v1 leaves under /api/v1/content* reach the Bearer middleware → UNAUTHORIZED
		// when no key is presented (never MISSING_TOKEN, which is the browser JWT path).
		{
			name:     "GET /api/v1/content (list) reaches Bearer middleware",
			method:   http.MethodGet,
			path:     "/api/v1/content",
			wantCode: "UNAUTHORIZED",
		},
		{
			name:     "POST /api/v1/content (create) reaches Bearer middleware",
			method:   http.MethodPost,
			path:     "/api/v1/content",
			wantCode: "UNAUTHORIZED",
		},
		{
			name:     "GET /api/v1/content/{id} reaches Bearer middleware",
			method:   http.MethodGet,
			path:     "/api/v1/content/1",
			wantCode: "UNAUTHORIZED",
		},
		{
			name:     "PUT /api/v1/content/{id} (update) reaches Bearer middleware",
			method:   http.MethodPut,
			path:     "/api/v1/content/1",
			wantCode: "UNAUTHORIZED",
		},
		{
			name:     "DELETE /api/v1/content/{id} (delete) reaches Bearer middleware",
			method:   http.MethodDelete,
			path:     "/api/v1/content/1",
			wantCode: "UNAUTHORIZED",
		},
		// Browser-realm leaves under /api/v1/content_items* and /api/v1/content/slug still
		// reach the JWT RequireAuth middleware → MISSING_TOKEN (the two groups are wired
		// independently and are byte-for-byte unchanged).
		{
			name:     "GET /api/v1/content_items reaches browser RequireAuth",
			method:   http.MethodGet,
			path:     "/api/v1/content_items",
			wantCode: "MISSING_TOKEN",
		},
		{
			name:     "PUT /api/v1/content_items/{id} reaches browser RequireAuth",
			method:   http.MethodPut,
			path:     "/api/v1/content_items/1",
			wantCode: "MISSING_TOKEN",
		},
		{
			name:     "DELETE /api/v1/content_items/{id} reaches browser RequireAuth",
			method:   http.MethodDelete,
			path:     "/api/v1/content_items/1",
			wantCode: "MISSING_TOKEN",
		},
		{
			name:     "POST /api/v1/content/slug reaches browser RequireAuth",
			method:   http.MethodPost,
			path:     "/api/v1/content/slug",
			wantCode: "MISSING_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			code, envelope := envelopeCode(t, r, req)

			assert.NotEqual(t, http.StatusNotFound, code, "%s %s must match a route", tt.method, tt.path)
			assert.NotEqual(t, http.StatusMethodNotAllowed, code, "%s %s must accept %s", tt.method, tt.path, tt.method)
			assert.Equal(t, http.StatusUnauthorized, code, "%s %s must be authenticated", tt.method, tt.path)
			assert.Equal(t, tt.wantCode, envelope, "%s %s middleware code", tt.method, tt.path)
		})
	}
}

// TestBearerAPIRoutes_MediaDualAuthDispatch proves the shared /api/v1/media GET routes
// dispatch by auth realm: a Bearer API key reaches the agent handler stack (INVALID_API_KEY
// for an unverifiable key), while a non-Bearer request reaches the browser admin stack
// (MISSING_TOKEN). This resolves the path collision between the agent v1 surface and the
// browser admin media endpoints (Chi routes by path, not auth realm). It also confirms the
// agent POST upload is Bearer-only (UNAUTHORIZED) and the browser media upload/delete
// routes remain reachable (MISSING_TOKEN, not 404).
func TestBearerAPIRoutes_MediaDualAuthDispatch(t *testing.T) {
	r, deps := setupTestRouterWithBearerDeps(t)
	// Any agent-realm Bearer verifies as not-found → INVALID_API_KEY, proving the request
	// reached the agent apiKeyAuth middleware (not the browser RequireAuth).
	deps.verifier.EXPECT().Verify(mock.Anything, mock.Anything).Return(nil, apikey.ErrKeyNotFound).Maybe()

	tests := []struct {
		name     string
		method   string
		path     string
		bearer   bool // set Authorization: Bearer lesstruct_… (agent realm)
		wantCode string
	}{
		{
			name:     "GET /api/v1/media with Bearer reaches agent realm",
			method:   http.MethodGet,
			path:     "/api/v1/media",
			bearer:   true,
			wantCode: "INVALID_API_KEY",
		},
		{
			name:     "GET /api/v1/media without Bearer reaches browser admin realm",
			method:   http.MethodGet,
			path:     "/api/v1/media",
			bearer:   false,
			wantCode: "MISSING_TOKEN",
		},
		{
			name:     "GET /api/v1/media/{id} with Bearer reaches agent realm",
			method:   http.MethodGet,
			path:     "/api/v1/media/5",
			bearer:   true,
			wantCode: "INVALID_API_KEY",
		},
		{
			name:     "GET /api/v1/media/{id} without Bearer reaches browser admin realm",
			method:   http.MethodGet,
			path:     "/api/v1/media/5",
			bearer:   false,
			wantCode: "MISSING_TOKEN",
		},
		{
			name:     "POST /api/v1/media (upload) without Bearer is Bearer-only (UNAUTHORIZED)",
			method:   http.MethodPost,
			path:     "/api/v1/media",
			bearer:   false,
			wantCode: "UNAUTHORIZED",
		},
		{
			name:     "POST /api/v1/media/upload (browser) without Bearer reaches RequireAuth",
			method:   http.MethodPost,
			path:     "/api/v1/media/upload",
			bearer:   false,
			wantCode: "MISSING_TOKEN",
		},
		{
			name:     "DELETE /api/v1/media/{id} (browser) without Bearer reaches RequireAuth",
			method:   http.MethodDelete,
			path:     "/api/v1/media/5",
			bearer:   false,
			wantCode: "MISSING_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.bearer {
				req.Header.Set("Authorization", "Bearer lesstruct_aaaaaaaaaaaa_bb")
			}
			code, envelope := envelopeCode(t, r, req)

			assert.NotEqual(t, http.StatusNotFound, code, "%s %s must match a route", tt.method, tt.path)
			assert.NotEqual(t, http.StatusMethodNotAllowed, code, "%s %s must accept %s", tt.method, tt.path, tt.method)
			assert.Equal(t, http.StatusUnauthorized, code, "%s %s must be authenticated", tt.method, tt.path)
			assert.Equal(t, tt.wantCode, envelope, "%s %s realm/code", tt.method, tt.path)
		})
	}
}

// TestBearerAPIRoutes_InvalidKeysAreThrottled proves the per-key rate limiter runs
// BEFORE Bearer auth (it is the outermost middleware in the Bearer group, per the
// review fix): repeated requests presenting the SAME invalid keyID are throttled with
// 429 RATE_LIMITED even though the key never verifies. This is the brute-force
// protection keyByAPIKeyOrIP exists to provide; it only works because the limiter is
// registered before the auth middleware (which short-circuits on failure).
//
// It builds a dedicated router — the shared setupTestRouter disables rate limiting so
// other tests can flood freely — mirroring the routes.go Bearer-group wiring with the
// limiter enabled at a low per-key limit.
func TestBearerAPIRoutes_InvalidKeysAreThrottled(t *testing.T) {
	verifier := mwmocks.NewMockAPIKeyVerifier(t)
	verifier.EXPECT().Verify(mock.Anything, mock.Anything).Return(nil, apikey.ErrKeyNotFound).Maybe()
	userLookup := mwmocks.NewMockUserLookup(t)
	logger := util.NewLogger(io.Discard)

	authMw := middleware.NewAPIKeyAuthMiddleware(verifier, userLookup, logger)
	rateMw := middleware.NewRateLimitMiddleware(true, 5, 2, 60) // enabled; apiPerMinute = 2

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(rateMw.APIKeyHandler) // OUTERMOST — must run before auth (review fix)
		r.Use(authMw.Handler)
		r.Post("/api/v1/content", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	// Same presented keyID each request → same per-key bucket. apiPerMinute=2 means the
	// first two reach auth (→ 401 INVALID_API_KEY) and the third is throttled (429).
	const sameToken = "Bearer lesstruct_aaaaaaaaaaaa_badsecret"
	doRequest := func() (int, string) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/content", nil)
		req.Header.Set("Authorization", sameToken)
		req.RemoteAddr = "203.0.113.7:5555"
		return envelopeCode(t, r, req)
	}

	c1, e1 := doRequest()
	c2, e2 := doRequest()
	c3, e3 := doRequest()

	assert.Equal(t, http.StatusUnauthorized, c1)
	assert.Equal(t, "INVALID_API_KEY", e1)
	assert.Equal(t, http.StatusUnauthorized, c2)
	assert.Equal(t, "INVALID_API_KEY", e2)
	assert.Equal(t, http.StatusTooManyRequests, c3, "third request with the same invalid keyID must be throttled")
	assert.Equal(t, "RATE_LIMITED", e3)
}

