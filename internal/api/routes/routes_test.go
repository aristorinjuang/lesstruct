package routes_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/api/handlers/agent"
	agentmocks "github.com/aristorinjuang/lesstruct/internal/api/handlers/agent/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	mwmocks "github.com/aristorinjuang/lesstruct/internal/api/middleware/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/routes"
	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	"github.com/aristorinjuang/lesstruct/internal/domain/role"
	emailmocks "github.com/aristorinjuang/lesstruct/internal/email/mocks"
	"github.com/aristorinjuang/lesstruct/internal/modules"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	repomocks "github.com/aristorinjuang/lesstruct/internal/repository/mocks"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/capability"
	contentdomain "github.com/aristorinjuang/lesstruct/kernel/content"
	"github.com/aristorinjuang/lesstruct/kernel/httpx"
	"github.com/aristorinjuang/lesstruct/modules/adminpanel"
	authapi "github.com/aristorinjuang/lesstruct/modules/auth/api"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	authmocks "github.com/aristorinjuang/lesstruct/modules/auth/domain/mocks"
	commentapi "github.com/aristorinjuang/lesstruct/modules/comments/api"
	commentagent "github.com/aristorinjuang/lesstruct/modules/comments/api/agent"
	commentagentmocks "github.com/aristorinjuang/lesstruct/modules/comments/api/agent/mocks"
	commentapimocks "github.com/aristorinjuang/lesstruct/modules/comments/api/mocks"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	"github.com/aristorinjuang/lesstruct/modules/importers"
	mediaapi "github.com/aristorinjuang/lesstruct/modules/media/api"
	mediaagent "github.com/aristorinjuang/lesstruct/modules/media/api/agent"
	mediaagentmocks "github.com/aristorinjuang/lesstruct/modules/media/api/agent/mocks"
	mediaapimocks "github.com/aristorinjuang/lesstruct/modules/media/api/mocks"
	posttypesbasic "github.com/aristorinjuang/lesstruct/modules/posttypes-basic"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
	seoapi "github.com/aristorinjuang/lesstruct/modules/seo/api"
	usersapi "github.com/aristorinjuang/lesstruct/modules/users/api"
	"github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func defaultSecurityHeadersMiddleware(t *testing.T) *middleware.SecurityHeadersMiddleware {
	t.Helper()
	cspName, cspValue := config.CSPConfig{}.Build()
	return middleware.NewSecurityHeadersMiddleware(cspName, cspValue, config.CSPConfig{}.XFrameOptions(), 31536000)
}

func setupTestRouter(t *testing.T) *chi.Mux {
	t.Helper()
	r, _ := setupTestRouterWithBearerDeps(t)
	return r
}

// bearerDeps holds the Bearer-group mock dependencies a test can configure to drive
// the /api/v1/content routes through the real middleware + handler stack.
type bearerDeps struct {
	verifier      *mwmocks.MockAPIKeyVerifier
	user          *mwmocks.MockUserLookup
	content       *agentmocks.MockContentService
	media         *mediaagentmocks.MockMediaService
	browserMedia  *mediaapimocks.MockMediaServiceInterface
	comment       *commentagentmocks.MockCommentService
	browser       *commentapimocks.MockCommentService
	jwtManager    *appauth.JWTManager
	notifications *repomocks.MockNotificationRepo
	userRepo      *repomocks.MockUserRepo
}

// testSSGSurface is a bare SSGSurface for route-gate tests. Generate is never
// invoked: the tests assert the capability gate rejects before the handler runs.
type testSSGSurface struct{}

func (testSSGSurface) Generate(http.ResponseWriter, *http.Request) {}

// verifiedUserLookup resolves every JWT subject to an active account. Route
// tests exercise routing and role gates; session-state enforcement has its own
// middleware tests.
type verifiedUserLookup struct{}

func (verifiedUserLookup) GetUserByID(_ context.Context, userID int) (*user.User, error) {
	return &user.User{
		ID:       userID,
		Username: "user",
		Role:     "Contributor",
		Status:   "verified",
	}, nil
}

// setupTestRouterWithBearerDeps builds the full routes.Setup router and also returns
// the mock dependencies wired into the Bearer /api/v1 group, so Bearer route tests
// can configure the verifier/user/content mocks per-case. The shared setupTestRouter
// delegates here and discards the mocks (existing tests do not hit Bearer routes, so
// the no-expectation mocks are never invoked).
func setupTestRouterWithBearerDeps(t *testing.T) (*chi.Mux, bearerDeps) {
	t.Helper()
	return setupTestRouterWithFlags(t, false, true, nil, nil)
}

// setupTestRouterWithFlags is setupTestRouterWithOptions with the MCP endpoint
// disabled (the production default) and the headless and comments feature
// flags configurable, plus optional SPA handler/handler stubs so tests can
// prove what headless mode mounts (or unmounts). A nil spaHandler keeps the
// behavior of the default helper (no content-site catch-all registered at all).
func setupTestRouterWithFlags(t *testing.T, headlessEnabled bool, commentsEnabled bool, spaHandler routes.AdminPanelSurface, staticHnd http.Handler) (*chi.Mux, bearerDeps) {
	t.Helper()
	return setupTestRouterWithOptions(t, headlessEnabled, false, commentsEnabled, spaHandler, staticHnd)
}

// setupTestRouterWithOptions is setupTestRouterWithFlags with the MCP endpoint
// toggle exposed, so the /mcp mount can be exercised on the real router.
func setupTestRouterWithOptions(t *testing.T, headlessEnabled bool, mcpEnabled bool, commentsEnabled bool, spaHandler routes.AdminPanelSurface, staticHnd http.Handler) (*chi.Mux, bearerDeps) {
	t.Helper()

	// Create test auth handler
	hash, err := appauth.HashPassword("admin")
	require.NoError(t, err, "Failed to hash password")

	authService := authdomain.NewAuthService(hash, modules.NewUserCredentialPolicy())
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(modules.NewUserCredentialPolicy())
	registrationService := authdomain.NewRegistrationService(nil, modules.NewUserCredentialPolicy(), commentsEnabled)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)
	mockUserRepo := repomocks.NewMockUserRepo(t)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	loginService := authdomain.NewLoginService(mockUserRepo, authmocks.NewMockFailedLoginAttemptRepo(t), modules.NewUserCredentialPolicy(), nil)
	failedLoginRepo := authmocks.NewMockFailedLoginAttemptRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := authmocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(mockUserRepo, passwordResetTokenRepo, modules.NewUserCredentialPolicy(), 1)
	authHandler := newRouteAuthHandler(
		authService,
		jwtManager,
		modules.NewLogger(logger),
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		mockUserRepo,
		failedLoginRepo,
		modules.NewAuthNotificationRecorder(notificationRepo),
		emailService,
		blockedEmailRepo,
	)
	firstLoginHandler := authapi.NewFirstLoginHandler(firstLoginService, mockUserRepo, modules.NewUserCredentialPolicy(), modules.NewLogger(logger))
	notificationHandler := handlers.NewNotificationHandler(logger, notificationRepo)

	userManagementService := user.NewUserManagementService(mockUserRepo, blockedEmailRepo, modules.NewUserCredentialPolicy(), true)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	userManagementHandler := usersapi.NewUserManagementHandler(userManagementService, nil, mockUserRepo, modules.NewSoftDeleteStore(softDeleteRepo), emailService, modules.NewLogger(logger), nil)

	// Initialize profile management dependencies (Story 1.7)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	profileService := user.NewProfileService(mockUserRepo, emailUpdateTokenRepo, userDataExportRepo, emailService, modules.NewUserCredentialPolicy(), nil)
	// Initialize account deletion service (Story 1.8)
	userDeletionRepo := repomocks.NewMockUserDeletionRepo(t)
	accountDeletionService := user.NewAccountDeletionService(mockUserRepo, userDeletionRepo, emailService, modules.NewLogger(logger))

	profileHandler := usersapi.NewProfileHandler(profileService, accountDeletionService, modules.NewLogger(logger), nil, nil)

	// The picture handler mounts through the structural surface: a bare
	// instance suffices since these tests never hit its endpoints.
	profilePictureHandler := usersapi.NewProfilePictureHandler(nil, nil, nil)

	authMiddleware := middleware.NewAuthMiddleware(jwtManager, verifiedUserLookup{})
	adminMiddleware := middleware.NewAdminMiddleware(authMiddleware)

	// Initialize content handler with mock service (Story 2.1)
	contentHandler := handlers.NewContentHandler(mocks.NewMockContentServiceInterface(t), logger, "http://localhost:3000", nil, nil)

	// Initialize media handler with mock service (Story 2.3) — the real media
	// module handler over the mock service, exactly like production mounts
	// the module's handlers.
	browserMediaSvc := mediaapimocks.NewMockMediaServiceInterface(t)
	mediaHandler := mediaapi.NewMediaHandler(browserMediaSvc, nil, logger)

	// Initialize dashboard handler (Story 2.8)
	dashboardHandler := handlers.NewDashboardHandler(mocks.NewMockDashboardServiceInterface(t), nil, logger)

	corsMiddleware := middleware.NewCORSMiddleware([]string{"http://localhost:8080"}, logger)
	csrfMiddleware := middleware.NewCSRFMiddleware(logger, nil, nil)

	rateLimitMiddleware := middleware.NewRateLimitMiddleware(false, 5, 100, 60)
	secHeaderMW := defaultSecurityHeadersMiddleware(t)

	// Initialize SEO handler (Story 4.2) — the real seo module handler over
	// an empty published set, exactly like production mounts the module's
	// handler. Route tests never serve SEO leaves through it.
	seoHandler := seoapi.NewSEOHandler(
		seoapi.PublishedPages(func(ctx context.Context, limit, offset int) ([]seoapi.PageRef, error) {
			return nil, nil
		}),
		"http://localhost:3000",
		headlessEnabled,
		logger,
	)

	// Initialize comment handler (Story 4.7) — the real comments module
	// handlers over the mock services, exactly like production mounts the
	// module's handlers. Content lookups resolve test fixtures inline.
	browserContentMock := commentapimocks.NewMockCommentService(t)
	commentContentResolver := commentapi.ContentResolver(
		func(ctx context.Context, slug, language string) (commentapi.ContentRef, error) {
			if slug == "my-post" {
				return commentapi.ContentRef{ID: 5, AllowComments: true}, nil
			}
			return commentapi.ContentRef{}, commentapi.ErrContentNotFound
		},
	)
	commentHandler := commentapi.NewCommentHandler(browserContentMock, commentContentResolver)

	// Real importers module with no adopted services. Route tests only
	// register handler method values (import endpoints are never invoked),
	// so bare handlers suffice — mirroring how media/seo/comments mount
	// their real module handlers.
	importersModule := importers.New()

	// Initialize Bearer-group dependencies (Story 2.1): real middleware + handler
	// wired to mock interfaces so Bearer route tests can configure them per-case.
	verifier := mwmocks.NewMockAPIKeyVerifier(t)
	userLookup := mwmocks.NewMockUserLookup(t)
	apiKeyAuthMiddleware := middleware.NewAPIKeyAuthMiddleware(verifier, userLookup, logger)
	contentSvc := agentmocks.NewMockContentService(t)
	agentContentHandler := agent.NewContentHandler(contentSvc, nil, logger)
	mediaSvc := mediaagentmocks.NewMockMediaService(t)
	agentMediaHandler := mediaagent.NewMediaHandler(mediaSvc, logger)
	commentSvc := commentagentmocks.NewMockCommentService(t)
	agentCommentLookup := commentagent.ContentLookup(
		func(ctx context.Context, id int) (commentagent.ContentDetail, error) {
			return commentagent.ContentDetail{ID: id, UserID: 1, Published: true, AllowComments: true}, nil
		},
	)
	agentCommentHandler := commentagent.NewCommentHandler(commentSvc, agentCommentLookup, logger)

	deps := bearerDeps{
		verifier:      verifier,
		user:          userLookup,
		content:       contentSvc,
		media:         mediaSvc,
		browserMedia:  browserMediaSvc,
		comment:       commentSvc,
		browser:       browserContentMock,
		jwtManager:    jwtManager,
		notifications: notificationRepo,
		userRepo:      mockUserRepo,
	}

	// First-party modules mount exactly like production: the real
	// posttypes-basic module (default registry, no resolver) serves the
	// post-type surface, so route tests exercise the module mount.
	postTypesModule := posttypesbasic.New(
		posttypesbasic.WithService(posttype.NewService()),
		posttypesbasic.WithCommentsEnabled(commentsEnabled),
	)
	moduleRegistry := kernel.NewRegistry()
	require.NoError(t, moduleRegistry.Register(postTypesModule))
	// The stub agent-commerce module below proves the registry loop wires
	// AgentRouteProvider modules onto the agent realm with key auth
	// pre-chained, using the same capability resolver production passes.
	require.NoError(t, moduleRegistry.Register(stubAgentCommerceModule{
		resolve: middleware.NewCapabilityResolver(role.NewService()),
	}))

	r := routes.Setup(
		authHandler,
		firstLoginHandler,
		notificationHandler,
		userManagementHandler,
		profileHandler,
		contentHandler,
		mediaHandler,
		dashboardHandler,
		seoHandler,
		commentHandler,
		profilePictureHandler,
		importersModule.WordPressHandler(),
		nil,
		importersModule.HugoHandler(),
		handlers.NewExportHandler(nil, logger),
		testSSGSurface{},
		nil,
		apiKeyAuthMiddleware,
		agentContentHandler,
		agentMediaHandler,
		agentCommentHandler,
		adminMiddleware,
		corsMiddleware,
		middleware.NewNoCookieMiddleware(logger, kernel.SessionCookieName, kernel.AuthHintCookieName),
		csrfMiddleware,
		rateLimitMiddleware,
		secHeaderMW,
		authMiddleware,
		spaHandler,
		staticHnd,
		false,
		false,
		false,
		nil,
		false,
		nil,
		[]string{"en"},
		nil,
		headlessEnabled,
		mcpEnabled,
		commentsEnabled,
		"local",
		middleware.NewCapabilityResolver(role.NewService()),
		moduleRegistry,
	)
	return r, deps
}

func TestSetup(t *testing.T) {
	hash, err := appauth.HashPassword("admin")
	require.NoError(t, err, "Failed to hash password")

	authService := authdomain.NewAuthService(hash, modules.NewUserCredentialPolicy())
	jwtManager := appauth.NewJWTManager("test-secret")
	logger := util.NewLogger(os.Stdout)
	firstLoginService := authdomain.NewFirstLoginService(modules.NewUserCredentialPolicy())
	registrationService := authdomain.NewRegistrationService(nil, modules.NewUserCredentialPolicy(), true)
	verificationService := authdomain.NewVerificationService(nil, nil, 24)
	mockUserRepo := repomocks.NewMockUserRepo(t)
	notificationRepo := repomocks.NewMockNotificationRepo(t)
	loginService := authdomain.NewLoginService(mockUserRepo, authmocks.NewMockFailedLoginAttemptRepo(t), modules.NewUserCredentialPolicy(), nil)
	failedLoginRepo := authmocks.NewMockFailedLoginAttemptRepo(t)
	emailService := emailmocks.NewMockEmailService(t)
	blockedEmailRepo := repomocks.NewMockBlockedEmailRepo(t)
	passwordResetTokenRepo := authmocks.NewMockPasswordResetTokenRepo(t)
	passwordResetService := authdomain.NewPasswordResetService(mockUserRepo, passwordResetTokenRepo, modules.NewUserCredentialPolicy(), 1)
	authHandler := newRouteAuthHandler(
		authService,
		jwtManager,
		modules.NewLogger(logger),
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		mockUserRepo,
		failedLoginRepo,
		modules.NewAuthNotificationRecorder(notificationRepo),
		emailService,
		blockedEmailRepo,
	)
	firstLoginHandler := authapi.NewFirstLoginHandler(firstLoginService, mockUserRepo, modules.NewUserCredentialPolicy(), modules.NewLogger(logger))
	notificationHandler := handlers.NewNotificationHandler(logger, notificationRepo)

	userManagementService := user.NewUserManagementService(mockUserRepo, blockedEmailRepo, modules.NewUserCredentialPolicy(), true)
	softDeleteRepo := repomocks.NewMockSoftDeleteRepo(t)
	userManagementHandler := usersapi.NewUserManagementHandler(userManagementService, nil, mockUserRepo, modules.NewSoftDeleteStore(softDeleteRepo), emailService, modules.NewLogger(logger), nil)

	// Initialize profile management dependencies (Story 1.7)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	profileService := user.NewProfileService(mockUserRepo, emailUpdateTokenRepo, userDataExportRepo, emailService, modules.NewUserCredentialPolicy(), nil)
	// Initialize account deletion service (Story 1.8)
	userDeletionRepo := repomocks.NewMockUserDeletionRepo(t)
	accountDeletionService := user.NewAccountDeletionService(mockUserRepo, userDeletionRepo, emailService, modules.NewLogger(logger))

	profileHandler := usersapi.NewProfileHandler(profileService, accountDeletionService, modules.NewLogger(logger), nil, nil)

	// The picture handler mounts through the structural surface: a bare
	// instance suffices since these tests never hit its endpoints.
	profilePictureHandler := usersapi.NewProfilePictureHandler(nil, nil, nil)

	authMiddleware := middleware.NewAuthMiddleware(jwtManager, verifiedUserLookup{})
	adminMiddleware := middleware.NewAdminMiddleware(authMiddleware)
	corsMiddleware := middleware.NewCORSMiddleware([]string{"http://localhost:8080"}, logger)
	csrfMiddleware := middleware.NewCSRFMiddleware(logger, nil, nil)

	// Initialize content handler with mock service (Story 2.1)
	contentHandler := handlers.NewContentHandler(mocks.NewMockContentServiceInterface(t), logger, "http://localhost:3000", nil, nil)

	// Initialize media handler with mock service (Story 2.3) — the real media
	// module handler over the mock service, exactly like production mounts
	// the module's handlers.
	mediaHandler := mediaapi.NewMediaHandler(mediaapimocks.NewMockMediaServiceInterface(t), nil, logger)

	// Initialize dashboard handler (Story 2.8)
	dashboardHandler := handlers.NewDashboardHandler(mocks.NewMockDashboardServiceInterface(t), nil, logger)

	rateLimitMiddleware := middleware.NewRateLimitMiddleware(false, 5, 100, 60)
	secHeaderMW := defaultSecurityHeadersMiddleware(t)

	// Initialize SEO handler (Story 4.2)
	seoHandler := seoapi.NewSEOHandler(
		seoapi.PublishedPages(func(ctx context.Context, limit, offset int) ([]seoapi.PageRef, error) {
			return nil, nil
		}),
		"http://localhost:3000",
		false,
		logger,
	)

	// Initialize comment handler (Story 4.7)
	commentHandler := commentapi.NewCommentHandler(
		commentapimocks.NewMockCommentService(t),
		commentapi.ContentResolver(
			func(ctx context.Context, slug, language string) (commentapi.ContentRef, error) {
				return commentapi.ContentRef{}, commentapi.ErrContentNotFound
			},
		),
	)

	// Real importers module with no adopted services (registration only —
	// import endpoints are never invoked in route tests).
	importersModule := importers.New()

	// Initialize Bearer-group dependencies (Story 2.1)
	apiKeyAuthMiddleware := middleware.NewAPIKeyAuthMiddleware(
		mwmocks.NewMockAPIKeyVerifier(t),
		mwmocks.NewMockUserLookup(t),
		logger,
	)
	agentContentHandler := agent.NewContentHandler(agentmocks.NewMockContentService(t), nil, logger)
	agentMediaHandler := mediaagent.NewMediaHandler(mediaagentmocks.NewMockMediaService(t), logger)
	agentCommentHandler := commentagent.NewCommentHandler(
		commentagentmocks.NewMockCommentService(t),
		commentagent.ContentLookup(
			func(ctx context.Context, id int) (commentagent.ContentDetail, error) {
				return commentagent.ContentDetail{ID: id, UserID: 1, Published: true, AllowComments: true}, nil
			},
		),
		logger,
	)

	r := routes.Setup(
		authHandler,
		firstLoginHandler,
		notificationHandler,
		userManagementHandler,
		profileHandler,
		contentHandler,
		mediaHandler,
		dashboardHandler,
		seoHandler,
		commentHandler,
		profilePictureHandler,
		importersModule.WordPressHandler(),
		nil,
		importersModule.HugoHandler(),
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
		secHeaderMW,
		authMiddleware,
		nil,
		nil,
		false,
		false,
		false,
		nil,
		false,
		nil,
		[]string{"en"},
		nil,
		false,
		false,
		true,
		"local",
		middleware.NewCapabilityResolver(role.NewService()),
		nil,
	)

	assert.NotNil(t, r, "Setup() returned nil router")
}

// TestModulePostTypesMounted proves the posttypes-basic module serves its
// endpoints through the real middleware chain: the protected list requires a
// JWT and returns the registry types, the public list is anonymous, and the
// roles endpoint fails closed without a role registry.
func TestModulePostTypesMounted(t *testing.T) {
	r, deps := setupTestRouterWithBearerDeps(t)

	adminToken, err := deps.jwtManager.GenerateToken("1", "admin", "Admin", 0)
	require.NoError(t, err)

	tests := []struct {
		name       string
		target     string
		withAuth   bool
		wantStatus int
		wantErr    bool
	}{
		{
			name:       "success - protected list serves registry types",
			target:     "/api/v1/post_types",
			withAuth:   true,
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "error - protected list without token is unauthorized",
			target:     "/api/v1/post_types",
			withAuth:   false,
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name:       "success - public list is anonymous",
			target:     "/api/v1/public/post_types",
			withAuth:   false,
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "error - roles without registry fails closed",
			target:     "/api/v1/roles",
			withAuth:   true,
			wantStatus: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			if tt.withAuth {
				req.Header.Set("Authorization", "Bearer "+adminToken)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)
			if tt.wantErr {
				return
			}
			var envelope map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&envelope))
			data, ok := envelope["data"].([]any)
			require.True(t, ok, "data must be an array")
			require.NotEmpty(t, data, "registry must serve types")
		})
	}
}

// TestAdminExtensionsMounted proves the manifest endpoint aggregates the
// test registry's modules behind the admin chain: an admin JWT serves the
// versioned payload, anonymous callers are rejected.
func TestAdminExtensionsMounted(t *testing.T) {
	r, deps := setupTestRouterWithBearerDeps(t)

	adminToken, err := deps.jwtManager.GenerateToken("1", "admin", "Admin", 0)
	require.NoError(t, err)

	tests := []struct {
		name       string
		withAuth   bool
		wantStatus int
	}{
		{
			name:       "success - admin token serves versioned payload",
			withAuth:   true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "error - anonymous caller is unauthorized",
			withAuth:   false,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/admin/extensions", nil)
			if tt.withAuth {
				req.Header.Set("Authorization", "Bearer "+adminToken)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus != http.StatusOK {
				return
			}

			var envelope map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&envelope))
			assert.Nil(t, envelope["error"])
			data, ok := envelope["data"].(map[string]any)
			require.True(t, ok, "data must be an object")
			assert.Equal(t, float64(1), data["version"])
			modules, ok := data["modules"].([]any)
			require.True(t, ok, "modules must be an array")
			require.Len(t, modules, 1, "test registry holds only posttypes-basic")
			entry, ok := modules[0].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "posttypes-basic", entry["name"])
		})
	}
}

func TestHealthRoute(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Health route status")

	assert.Contains(t, w.Body.String(), `"status":"ok"`, "Health route body should contain status ok")
	assert.Contains(t, w.Body.String(), `"features":{"imageGeneration":false,"imageReference":false,"textGeneration":false,"speechToText":false}`, "Health route body should contain features")
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

func TestMFARoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "verify - empty POST body",
			method:     http.MethodPost,
			path:       "/api/auth/mfa/verify",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "verify - GET is not allowed",
			method:     http.MethodGet,
			path:       "/api/auth/mfa/verify",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "resend - empty POST body",
			method:     http.MethodPost,
			path:       "/api/auth/mfa/resend",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "resend - GET is not allowed",
			method:     http.MethodGet,
			path:       "/api/auth/mfa/resend",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupTestRouter(t)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "MFA route status")
		})
	}
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
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"), "Referrer-Policy should be set")
	assert.Equal(t, "max-age=31536000", w.Header().Get("Strict-Transport-Security"), "HSTS should be set when configured")
	assert.Equal(t, "camera=(), microphone=(), geolocation=(), browsing-topics=()", w.Header().Get("Permissions-Policy"), "Permissions-Policy should be set")
	assert.Empty(t, w.Header().Get("X-XSS-Protection"), "the deprecated X-XSS-Protection header must not be emitted")
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

// TestAgentExportAndSSG_AdminOnly proves the agent-realm content export and SSG
// endpoints are admin-gated: a valid API key owned by a non-admin role is
// forbidden, and an unauthenticated caller never reaches them. The export
// archive carries unpublished drafts, so a buyer-level key must not read it.
func TestAgentExportAndSSG_AdminOnly(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		role         string
		withKey      bool
		wantCode     int
		wantEnvelope string
	}{
		{
			name:         "export - non-admin key is forbidden",
			path:         "/api/v1/export",
			role:         "Commentator",
			withKey:      true,
			wantCode:     http.StatusForbidden,
			wantEnvelope: "INSUFFICIENT_PERMISSIONS",
		},
		{
			name:         "ssg - non-admin key is forbidden",
			path:         "/api/v1/ssg",
			role:         "Contributor",
			withKey:      true,
			wantCode:     http.StatusForbidden,
			wantEnvelope: "INSUFFICIENT_PERMISSIONS",
		},
		{
			name:         "export - unauthenticated is rejected",
			path:         "/api/v1/export",
			wantCode:     http.StatusUnauthorized,
			wantEnvelope: "UNAUTHORIZED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, deps := setupTestRouterWithBearerDeps(t)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.withKey {
				verifiedKey := &apikey.APIKey{ID: 7, UserID: testBearerUserID, KeyID: testBearerKeyID}
				deps.verifier.EXPECT().Verify(mock.Anything, testBearerToken).Return(verifiedKey, nil)
				deps.verifier.EXPECT().UpdateLastUsed(mock.Anything, 7, mock.Anything).Return(nil)
				deps.user.EXPECT().GetUserByID(mock.Anything, testBearerUserID).
					Return(&user.User{ID: testBearerUserID, Username: "buyer", Role: tt.role, Status: "verified"}, nil)
				req.Header.Set("Authorization", "Bearer "+testBearerToken)
			}

			code, envelope := envelopeCode(t, r, req)

			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantEnvelope, envelope)
		})
	}
}

// TestNotificationsRoute_AdminOnly proves the notification feed is no longer
// anonymous: its messages carry registration usernames and email addresses, so
// the route requires an admin JWT (401 without a token, 403 for a non-admin).
func TestNotificationsRoute_AdminOnly(t *testing.T) {
	tests := []struct {
		name         string
		role         string
		withToken    bool
		wantCode     int
		wantEnvelope string
	}{
		{
			name:         "anonymous is rejected",
			wantCode:     http.StatusUnauthorized,
			wantEnvelope: "MISSING_TOKEN",
		},
		{
			name:         "non-admin is forbidden",
			role:         "Commentator",
			withToken:    true,
			wantCode:     http.StatusForbidden,
			wantEnvelope: "INSUFFICIENT_PERMISSIONS",
		},
		{
			name:      "admin can read the feed",
			role:      "Admin",
			withToken: true,
			wantCode:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, deps := setupTestRouterWithBearerDeps(t)

			if tt.wantCode == http.StatusOK {
				deps.notifications.EXPECT().GetNotifications(mock.Anything, 100).
					Return([]repository.Notification{
						{
							ID:      1,
							Type:    repository.NotificationType("user_registered"),
							Message: "New user registration: alice (alice@example.com)",
						},
					}, nil)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
			if tt.withToken {
				token, err := deps.jwtManager.GenerateToken("1", "user", tt.role, 0)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
			}

			code, envelope := envelopeCode(t, r, req)

			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantEnvelope, envelope)
		})
	}
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
	owningUser := &user.User{ID: testBearerUserID, Username: "alice", Role: "Editor", Status: "verified"}

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
//
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

// TestBearerAPIRoutes_RotatedInvalidKeysAreThrottled proves the outcome-based
// limiter closes the finding-10 gap on the real Bearer chain order: DISTINCT
// fake keyIDs from one IP each return 401 until the IP spends its
// auth-failure budget, then further attempts are refused with 429 — while a
// different IP still gets its own budget. Attempt-based budgets are set high
// so only the outcome-based handler can trip.
func TestBearerAPIRoutes_RotatedInvalidKeysAreThrottled(t *testing.T) {
	verifier := mwmocks.NewMockAPIKeyVerifier(t)
	verifier.EXPECT().Verify(mock.Anything, mock.Anything).Return(nil, apikey.ErrKeyNotFound).Maybe()
	userLookup := mwmocks.NewMockUserLookup(t)
	logger := util.NewLogger(io.Discard)

	authMw := middleware.NewAPIKeyAuthMiddleware(verifier, userLookup, logger)
	rateMw := middleware.NewRateLimitMiddleware(true, 1000, 1000, 1000)
	rateMw.SetAgentFailuresPerMinute(3)

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(rateMw.AgentAuthFailureHandler) // OUTERMOST — must see the final 401
		r.Use(rateMw.AgentIPHandler)
		r.Use(rateMw.APIKeyHandler)
		r.Use(authMw.Handler)
		r.Post("/api/v1/content", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	doRequest := func(keyID, remoteAddr string) (int, string) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/content", nil)
		req.Header.Set("Authorization", "Bearer lesstruct_"+keyID+"_badsecretbadsecretbadsecret00")
		req.RemoteAddr = remoteAddr
		return envelopeCode(t, r, req)
	}

	const attackerIP = "203.0.113.7:5555"
	for i, keyID := range []string{"aaaaaaaaaaaa", "bbbbbbbbbbbb", "cccccccccccc"} {
		code, envelope := doRequest(keyID, attackerIP)
		assert.Equal(t, http.StatusUnauthorized, code, "rotated key %d must still reach auth", i+1)
		assert.Equal(t, "INVALID_API_KEY", envelope)
	}

	code, envelope := doRequest("dddddddddddd", attackerIP)
	assert.Equal(t, http.StatusTooManyRequests, code, "fourth distinct fake key from the same IP must be throttled")
	assert.Equal(t, "RATE_LIMITED", envelope)

	code, _ = doRequest("eeeeeeeeeeee", "203.0.113.8:5555")
	assert.Equal(t, http.StatusUnauthorized, code, "a different IP keeps its own failure budget")
}

// headlessRouter builds a router in headless mode with the real module SPA
// handler (embedded admin FS) and a stub content-site static handler, so
// tests can prove what headless mode mounts and unmounts.
func headlessRouter(t *testing.T) *chi.Mux {
	t.Helper()
	spaHandler := adminpanel.New().SPAHandler()
	staticHnd := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r, _ := setupTestRouterWithFlags(t, true, true, spaHandler, staticHnd)
	return r
}

func TestHeadlessMode_UnmountsContentSite(t *testing.T) {
	r := headlessRouter(t)

	// The catch-all content-site route must NOT be mounted: any non-API path 404s.
	req := httptest.NewRequest(http.MethodGet, "/hello-world", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, "content catch-all must be unmounted in headless mode")

	// /static/* (theme assets) must also be unmounted.
	req = httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, "/static/* must be unmounted in headless mode")

	// Admin SPA and API stay reachable.
	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "/admin must stay mounted in headless mode")

	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "/api/health must stay mounted in headless mode")
}

func TestHeadlessMode_MountsContentSite(t *testing.T) {
	// Non-headless mode must keep serving /static/* — the control case for
	// TestHeadlessMode_UnmountsContentSite.
	spaHandler := adminpanel.New().SPAHandler()
	staticHnd := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r, _ := setupTestRouterWithFlags(t, false, true, spaHandler, staticHnd)

	req := httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "/static/* must stay mounted when headless is off")
}

func TestCommentsDisabled_UnmountsCommentRoutes(t *testing.T) {
	r, _ := setupTestRouterWithFlags(t, false, false, nil, nil)

	// The /api/v1 subtree middleware (RequireAuth) runs before chi's NotFound,
	// so an unauthenticated request to a disabled leaf returns 401. With a
	// valid JWT the unmounted leaves fall through to chi's 404 while mounted
	// ones reach their handlers.
	jwtManager := appauth.NewJWTManager("test-secret")
	token, err := jwtManager.GenerateToken("1", "admin", "Admin", 0)
	require.NoError(t, err, "Failed to generate test token")

	// Agent realm comment routes unmounted: an authenticated request 404s
	// (the path falls through the /api/v1 subtree to NotFound).
	req := httptest.NewRequest(http.MethodPost, "/api/v1/content/123/comments", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, "agent comment create must be unmounted when comments are disabled")

	// Public comments unmounted.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/public/content_items/my-post/comments", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, "public comment list must be unmounted when comments are disabled")

	// Browser moderation + my-comments unmounted: with a valid JWT they 404
	// instead of reaching a handler.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/my-comments", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, "my-comments must be unmounted when comments are disabled")

	req = httptest.NewRequest(http.MethodGet, "/api/v1/comments/pending", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, "comment moderation must be unmounted when comments are disabled")

	// Non-comment routes must be unaffected.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/content_items", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "content_items must stay mounted when comments are disabled")
}

func TestCommentsEnabled_MountsCommentRoutes(t *testing.T) {
	// Control case: with comments enabled the same leaves stay mounted — an
	// authenticated request reaches the comment handlers instead of 404ing.
	r, deps := setupTestRouterWithFlags(t, false, true, nil, nil)

	// Authenticate the Bearer group with a valid key so the agent comment
	// route is reachable.
	verifiedKey := &apikey.APIKey{ID: 7, UserID: 1, KeyID: "aaaaaaaaaaaa"}
	deps.verifier.EXPECT().Verify(mock.Anything, "lesstruct_aaaaaaaaaaaa_bb").Return(verifiedKey, nil)
	deps.verifier.EXPECT().UpdateLastUsed(mock.Anything, 7, mock.Anything).Return(nil)
	deps.user.EXPECT().GetUserByID(mock.Anything, 1).Return(&user.User{ID: 1, Username: "alice", Role: "Admin", Status: "verified"}, nil)
	// Content resolution now runs through the module lookup stub wired in
	// setupTestRouterWithFlags — only the comment operation is mocked here.
	deps.comment.EXPECT().SubmitComment(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&commentdomain.Comment{ID: 1, ContentID: 123, Comment: "hello"}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/content/123/comments", strings.NewReader(`{"comment":"hello"}`))
	req.Header.Set("Authorization", "Bearer lesstruct_aaaaaaaaaaaa_bb")
	code, _ := envelopeCode(t, r, req)
	assert.Equal(t, http.StatusOK, code, "agent comment create must be mounted when comments are enabled")

	// Public comment list route stays mounted and reachable without auth —
	// the content lookup resolves via the router's resolver stub, the comment
	// list via the router's content mock.
	deps.browser.EXPECT().GetCommentsForContent(mock.Anything, 5).
		Return([]*commentdomain.Comment{}, nil)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/public/content_items/my-post/comments", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "public comment list must be mounted when comments are enabled")
}

func TestConfigEndpoint_ExposesFeatureFlags(t *testing.T) {
	r, _ := setupTestRouterWithFlags(t, true, false, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"commentsEnabled":false`, "config must report commentsEnabled=false")
	assert.Contains(t, body, `"headless":true`, "config must report headless=true")

	// And the enabled side.
	r2, _ := setupTestRouterWithFlags(t, false, true, nil, nil)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, req2)
	body2 := w2.Body.String()
	assert.Contains(t, body2, `"commentsEnabled":true`, "config must report commentsEnabled=true")
	assert.Contains(t, body2, `"headless":false`, "config must report headless=false")
}

func TestModuleDisabledComments_MatchesLegacyToggle(t *testing.T) {
	// Route-mount parity: disabling the comments module must unmount exactly
	// the same leaves as the legacy [comments] toggle. Drive the flag through
	// the production mapping (legacy on + module disabled) and probe the same
	// leaves TestCommentsDisabled_UnmountsCommentRoutes covers.
	kept := map[string]bool{"users": true, "auth": true, "media": true}
	commentsEnabled := modules.EffectiveCommentsEnabled(true, []string{"comments"}, kept)
	require.False(t, commentsEnabled, "mapping must fold module disable to false")

	r, _ := setupTestRouterWithFlags(t, false, commentsEnabled, nil, nil)

	jwtManager := appauth.NewJWTManager("test-secret")
	token, err := jwtManager.GenerateToken("1", "admin", "Admin", 0)
	require.NoError(t, err, "Failed to generate test token")

	probes := []struct {
		name   string
		method string
		path   string
		auth   bool
	}{
		{"agent comment create", http.MethodPost, "/api/v1/content/123/comments", true},
		{"public comment list", http.MethodGet, "/api/v1/public/content_items/my-post/comments", false},
		{"my-comments", http.MethodGet, "/api/v1/my-comments", true},
		{"comment moderation", http.MethodGet, "/api/v1/comments/pending", true},
	}

	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			req := httptest.NewRequest(probe.method, probe.path, nil)
			if probe.auth {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusNotFound, w.Code, "%s must be unmounted", probe.name)
		})
	}

	// Non-comment routes must be unaffected.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/content_items", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "content_items must stay mounted")
}

func TestFavicon_RedirectsToStatic(t *testing.T) {
	spaHandler := adminpanel.New().SPAHandler()
	staticHnd := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r, _ := setupTestRouterWithFlags(t, false, true, spaHandler, staticHnd)

	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusFound, w.Code, "favicon must redirect instead of 404ing")
	assert.Equal(t, "/static/favicon.ico", w.Header().Get("Location"))
	assert.Equal(t, "public, max-age=86400", w.Header().Get("Cache-Control"), "favicon redirect carries the bounded static TTL")
}

func TestVerifyEmail_ReachableOnPublicLimiter(t *testing.T) {
	r, _ := setupTestRouterWithFlags(t, false, true, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusNotFound, w.Code, "verify-email must stay mounted on the public auth surface")
	assert.NotEqual(t, http.StatusMethodNotAllowed, w.Code, "verify-email must accept GET")
}

// TestVerifyEmail_SurvivesAuthBucketExhaustion pins the reason verify-email
// rides the public limiter: after the strict auth bucket is exhausted by
// login/register bursts, an auth endpoint 429s while the verification link
// still answers (here: 400 for the empty token, proving it reached the
// handler instead of the limiter). It mirrors the routes.go wiring — global
// group limiter, strict auth subgroup limiter, public limiter on the
// verify-email route — with small buckets.
func TestVerifyEmail_SurvivesAuthBucketExhaustion(t *testing.T) {
	rateMw := middleware.NewRateLimitMiddleware(true, 2, 1000, 1000)

	r := chi.NewRouter()
	r.Route("/api", func(r chi.Router) {
		r.Use(rateMw.Handler)
		r.Route("/auth", func(r chi.Router) {
			r.Use(rateMw.AuthHandler)
			r.Post("/register", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			})
		})
		r.With(rateMw.PublicHandler).Get("/auth/verify-email", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		})
	})

	doPost := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", nil)
		req.RemoteAddr = "203.0.113.9:5555"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	doVerify := func() int {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/verify-email", nil)
		req.RemoteAddr = "203.0.113.9:5555"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	assert.Equal(t, http.StatusBadRequest, doPost(), "first auth request passes the bucket")
	assert.Equal(t, http.StatusBadRequest, doPost(), "second auth request passes the bucket")
	assert.Equal(t, http.StatusTooManyRequests, doPost(), "third auth request exhausts the strict auth bucket")
	assert.Equal(t, http.StatusBadRequest, doVerify(), "verify-email rides the public bucket and still reaches its handler")
}

// stubAgentCommerceModule mounts one representative admin route per
// commerce surface on the agent realm, proving the registry loop wires
// AgentRouteProvider modules with key auth pre-chained. Its handler
// mirrors the real modules' in-handler Admin check, so the role matrix
// below exercises the same gate agents meet on the real surfaces.
type stubAgentCommerceModule struct {
	resolve kernel.CapabilityResolver
}

func (m stubAgentCommerceModule) Name() string             { return "agent-commerce-stub" }
func (m stubAgentCommerceModule) Version() string          { return "v0.0.0" }
func (m stubAgentCommerceModule) Requires() []string       { return nil }
func (m stubAgentCommerceModule) Optional() []string       { return nil }
func (m stubAgentCommerceModule) Init(kernel.Kernel) error { return nil }
func (m stubAgentCommerceModule) Shutdown() error          { return nil }

func (m stubAgentCommerceModule) RegisterDualRoutes(r kernel.DualAuthRegistrar) {
	r.Get("/api/v1/orders", m.serve)
	r.Get("/api/v1/orders/{orderID}", m.serve)
	r.Post("/api/v1/orders/{orderID}/status", m.serve)
	r.Get("/api/v1/catalog/products/{contentID}", m.serve)
	r.Put("/api/v1/catalog/products/{contentID}", m.serve)
	r.Get("/api/v1/payments", m.serve)
	r.Post("/api/v1/payments/{paymentID}/verify", m.serve)
}

func (m stubAgentCommerceModule) serve(w http.ResponseWriter, r *http.Request) {
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok || m.resolve == nil || !m.resolve(principal, string(capability.CapabilityAdmin)) {
		httpx.Error(w, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "You do not have permission", nil)
		return
	}
	httpx.Success(w, map[string]any{"ok": true})
}

func TestDualRealmCommerceRoutes(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		role         string
		withKey      bool
		withJWT      bool
		evilOrigin   bool
		setupMedia   bool
		setupBrowser bool
		wantCode     int
		wantEnvelope string
	}{
		{
			// No credential dispatches to the browser chain, exactly like
			// the co-owned media paths: anonymous callers see the browser
			// auth error.
			name:         "anonymous is rejected",
			method:       http.MethodGet,
			path:         "/api/v1/orders",
			wantCode:     http.StatusUnauthorized,
			wantEnvelope: "MISSING_TOKEN",
		},
		{
			name:         "agent - non-admin key is forbidden",
			method:       http.MethodGet,
			path:         "/api/v1/orders",
			role:         "Commentator",
			withKey:      true,
			wantCode:     http.StatusForbidden,
			wantEnvelope: "INSUFFICIENT_PERMISSIONS",
		},
		{
			name:     "agent - admin key reaches the handler",
			method:   http.MethodGet,
			path:     "/api/v1/orders",
			role:     "Admin",
			withKey:  true,
			wantCode: http.StatusOK,
		},
		{
			name:     "agent - order transition reaches the handler",
			method:   http.MethodPost,
			path:     "/api/v1/orders/42/status",
			role:     "Admin",
			withKey:  true,
			wantCode: http.StatusOK,
		},
		{
			name:     "agent - catalog read reaches the handler",
			method:   http.MethodGet,
			path:     "/api/v1/catalog/products/9",
			role:     "Admin",
			withKey:  true,
			wantCode: http.StatusOK,
		},
		{
			name:     "agent - catalog write reaches the handler",
			method:   http.MethodPut,
			path:     "/api/v1/catalog/products/9",
			body:     "{}",
			role:     "Admin",
			withKey:  true,
			wantCode: http.StatusOK,
		},
		{
			name:     "agent - payments verify reaches the handler",
			method:   http.MethodPost,
			path:     "/api/v1/payments/7/verify",
			role:     "Admin",
			withKey:  true,
			wantCode: http.StatusOK,
		},
		{
			name:       "agent - cross-site origin is ignored without CSRF",
			method:     http.MethodPost,
			path:       "/api/v1/orders/42/status",
			role:       "Admin",
			withKey:    true,
			evilOrigin: true,
			wantCode:   http.StatusOK,
		},
		{
			name:     "browser - admin JWT reaches the handler",
			method:   http.MethodGet,
			path:     "/api/v1/orders",
			role:     "Admin",
			withJWT:  true,
			wantCode: http.StatusOK,
		},
		{
			name:         "browser - cross-site POST is rejected by CSRF",
			method:       http.MethodPost,
			path:         "/api/v1/orders/42/status",
			role:         "Admin",
			withJWT:      true,
			evilOrigin:   true,
			wantCode:     http.StatusForbidden,
			wantEnvelope: "CSRF_VALIDATION_FAILED",
		},
		{
			name:         "browser - anonymous JWT is rejected",
			method:       http.MethodGet,
			path:         "/api/v1/orders",
			wantCode:     http.StatusUnauthorized,
			wantEnvelope: "MISSING_TOKEN",
		},
		{
			name:         "media delete - anonymous is rejected",
			method:       http.MethodDelete,
			path:         "/api/v1/media/9",
			wantCode:     http.StatusUnauthorized,
			wantEnvelope: "MISSING_TOKEN",
		},
		{
			name:       "media delete - admin key reaches the handler",
			method:     http.MethodDelete,
			path:       "/api/v1/media/9",
			role:       "Admin",
			withKey:    true,
			setupMedia: true,
			wantCode:   http.StatusNoContent,
		},
		{
			name:         "media delete - browser admin JWT reaches the handler",
			method:       http.MethodDelete,
			path:         "/api/v1/media/9",
			role:         "Admin",
			withJWT:      true,
			setupBrowser: true,
			wantCode:     http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, deps := setupTestRouterWithBearerDeps(t)

			if tt.withKey {
				verifiedKey := &apikey.APIKey{ID: 7, UserID: testBearerUserID, KeyID: testBearerKeyID}
				deps.verifier.EXPECT().Verify(mock.Anything, testBearerToken).Return(verifiedKey, nil)
				deps.verifier.EXPECT().UpdateLastUsed(mock.Anything, 7, mock.Anything).Return(nil)
				deps.user.EXPECT().GetUserByID(mock.Anything, testBearerUserID).
					Return(&user.User{ID: testBearerUserID, Username: "buyer", Role: tt.role, Status: "verified"}, nil)
			}
			var jwt string
			if tt.withJWT {
				var err error
				jwt, err = deps.jwtManager.GenerateToken("1", "admin", tt.role, 0)
				require.NoError(t, err)
			}
			if tt.setupMedia {
				deps.media.EXPECT().Delete(mock.Anything, 9, mock.Anything).Return(nil)
			}
			if tt.setupBrowser {
				deps.browserMedia.EXPECT().Delete(mock.Anything, 9, mock.Anything).Return(nil)
			}

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.path, body)
			if tt.withKey {
				req.Header.Set("Authorization", "Bearer "+testBearerToken)
			}
			if tt.withJWT {
				req.Header.Set("Authorization", "Bearer "+jwt)
			}
			if tt.evilOrigin {
				req.Header.Set("Origin", "https://evil.com")
				req.Header.Set("Sec-Fetch-Site", "cross-site")
			}

			code, envelope := envelopeCode(t, r, req)

			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantEnvelope, envelope)
		})
	}
}

func TestAgentUsersRoutes_AdminGated(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		role         string
		withKey      bool
		setupRepo    bool
		wantCode     int
		wantEnvelope string
	}{
		{
			name:         "list - anonymous is rejected",
			method:       http.MethodGet,
			path:         "/api/v1/users",
			wantCode:     http.StatusUnauthorized,
			wantEnvelope: "UNAUTHORIZED",
		},
		{
			name:         "list - non-admin key is forbidden",
			method:       http.MethodGet,
			path:         "/api/v1/users",
			role:         "Commentator",
			withKey:      true,
			wantCode:     http.StatusForbidden,
			wantEnvelope: "INSUFFICIENT_PERMISSIONS",
		},
		{
			name:     "list - admin key reaches the handler",
			method:   http.MethodGet,
			path:     "/api/v1/users",
			role:     "Admin",
			withKey:  true,
			wantCode: http.StatusOK,
		},
		{
			name:     "pending queue - admin key reaches the handler",
			method:   http.MethodGet,
			path:     "/api/v1/users/pending",
			role:     "Admin",
			withKey:  true,
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, deps := setupTestRouterWithBearerDeps(t)

			if tt.withKey {
				verifiedKey := &apikey.APIKey{ID: 7, UserID: testBearerUserID, KeyID: testBearerKeyID}
				deps.verifier.EXPECT().Verify(mock.Anything, testBearerToken).Return(verifiedKey, nil)
				deps.verifier.EXPECT().UpdateLastUsed(mock.Anything, 7, mock.Anything).Return(nil)
				deps.user.EXPECT().GetUserByID(mock.Anything, testBearerUserID).
					Return(&user.User{ID: testBearerUserID, Username: "buyer", Role: tt.role, Status: "verified"}, nil)
			}
			if tt.method == http.MethodGet {
				deps.userRepo.EXPECT().GetAllUsers(mock.Anything, "", 100, 0).Return([]*user.User{}, nil).Maybe()
				deps.userRepo.EXPECT().CountUsers(mock.Anything, "").Return(0, nil).Maybe()
				deps.userRepo.EXPECT().GetPendingUsers(mock.Anything, 100, 0).Return([]*user.User{}, nil).Maybe()
			}

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.path, body)
			if tt.withKey {
				req.Header.Set("Authorization", "Bearer "+testBearerToken)
			}

			code, envelope := envelopeCode(t, r, req)

			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantEnvelope, envelope)
		})
	}
}

// TestMCPSurface proves the opt-in /mcp mount end to end: unmounted by default
// (404), Bearer-gated when enabled (401 without a key), and protocol-capable
// with a valid key (initialize returns the lesstruct-mcp server info as JSON).
func TestMCPSurface(t *testing.T) {
	const initializeBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"route-test","version":"1"}}}`

	tests := []struct {
		name     string
		enabled  bool
		withKey  bool
		wantCode int
		wantBody string
	}{
		{
			name:     "disabled - route is not mounted",
			enabled:  false,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "enabled - missing key is rejected",
			enabled:  true,
			wantCode: http.StatusUnauthorized,
			wantBody: "UNAUTHORIZED",
		},
		{
			name:     "enabled - initialize succeeds",
			enabled:  true,
			withKey:  true,
			wantCode: http.StatusOK,
			wantBody: "lesstruct-mcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, deps := setupTestRouterWithOptions(t, false, tt.enabled, true, nil, nil)
			if tt.withKey {
				verifiedKey := &apikey.APIKey{ID: 7, UserID: testBearerUserID, KeyID: testBearerKeyID}
				deps.verifier.EXPECT().Verify(mock.Anything, testBearerToken).Return(verifiedKey, nil)
				deps.verifier.EXPECT().UpdateLastUsed(mock.Anything, 7, mock.Anything).Return(nil)
				deps.user.EXPECT().GetUserByID(mock.Anything, testBearerUserID).
					Return(&user.User{ID: testBearerUserID, Username: "agent", Role: "Admin", Status: "verified"}, nil)
			}

			req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(initializeBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			if tt.withKey {
				req.Header.Set("Authorization", "Bearer "+testBearerToken)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.wantCode, w.Code)
			if tt.wantBody != "" {
				assert.Contains(t, w.Body.String(), tt.wantBody)
			}
		})
	}
}
