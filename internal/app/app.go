// Package app is the Lesstruct composition root: the shared platform —
// config, database, identity, content engine, media stack, plugin system,
// boot — that every edition wires its module list into. An edition is a
// fixed module list plus wiring; the concrete editions live in
// internal/editions/<name> (imported only by their thin mains) so a binary
// links only the modules its edition carries.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/api/handlers/agent"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/api/routes"
	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/aristorinjuang/lesstruct/internal/constants"
	"github.com/aristorinjuang/lesstruct/internal/content/export"
	appdatabase "github.com/aristorinjuang/lesstruct/internal/database"
	apikeydomain "github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	dashboarddomain "github.com/aristorinjuang/lesstruct/internal/domain/dashboard"
	"github.com/aristorinjuang/lesstruct/internal/email"
	"github.com/aristorinjuang/lesstruct/internal/modules"
	pluginadapter "github.com/aristorinjuang/lesstruct/internal/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/bootstrap"
	"github.com/aristorinjuang/lesstruct/internal/plugin/hostfunctions"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	mysqlrepository "github.com/aristorinjuang/lesstruct/internal/repository/mysql"
	postgresqlrepository "github.com/aristorinjuang/lesstruct/internal/repository/postgresql"
	sqliterepository "github.com/aristorinjuang/lesstruct/internal/repository/sqlite"
	appstorage "github.com/aristorinjuang/lesstruct/internal/storage"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/legal"
	"github.com/aristorinjuang/lesstruct/kernel/tiptap"
	"github.com/aristorinjuang/lesstruct/modules/adminpanel"
	authmodule "github.com/aristorinjuang/lesstruct/modules/auth"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	"github.com/aristorinjuang/lesstruct/modules/media"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	mediainfra "github.com/aristorinjuang/lesstruct/modules/media/infrastructure"
	mediastore "github.com/aristorinjuang/lesstruct/modules/media/store"
	posttypesbasic "github.com/aristorinjuang/lesstruct/modules/posttypes-basic"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
	aliasdomain "github.com/aristorinjuang/lesstruct/modules/seo/alias"
	seodomain "github.com/aristorinjuang/lesstruct/modules/seo/domain"
	seostore "github.com/aristorinjuang/lesstruct/modules/seo/store"
	usersmodule "github.com/aristorinjuang/lesstruct/modules/users"
	"github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/aristorinjuang/lesstruct/modules/users/domain/profilepicture"
)

// isValidUsername validates that a username contains only safe characters
// to prevent SQL injection if username constants become configurable
func isValidUsername(username string) bool {
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)
	return usernameRegex.MatchString(username)
}

// resolveLegal projects the published page set onto the legal resolver's
// vocabulary and resolves the conventional documents against the preference
// chain. A read failure logs and resolves nothing: the consent gates then
// follow the "pages absent" behavior instead of blocking the boot.
func resolveLegal(ctx context.Context, contentService *contentdomain.Service, languages []string) legal.Resolution {
	pages, err := contentService.GetPublishedPages(ctx)
	if err != nil {
		log.Printf("Warning: Failed to load published pages for legal resolution: %v", err)
		return legal.Resolve(nil, languages)
	}
	candidates := make([]legal.Candidate, 0, len(pages))
	for _, page := range pages {
		candidates = append(candidates, legal.Candidate{
			PageID:             page.ID,
			Slug:               page.Slug,
			Language:           page.Language,
			TranslationGroupID: page.TranslationGroupID,
			UpdatedAt:          page.UpdatedAt,
		})
	}
	return legal.Resolve(candidates, languages)
}

// legalDocumentURL prefers the operator's explicit override over the resolved
// page and returns an empty string when the document exists at neither.
func legalDocumentURL(page legal.Page, overrides map[legal.Page]string, resolution legal.Resolution) string {
	if override := strings.TrimSpace(overrides[page]); override != "" {
		return override
	}
	if document, ok := resolution.Document(page); ok {
		return document.URL()
	}
	return ""
}

// isGeminiModel returns true if the model name indicates a Gemini image model.
func isGeminiModel(model string) bool {
	return strings.HasPrefix(model, "gemini-")
}

// isGPTModel returns true if the model name indicates an OpenAI GPT image model.
func isGPTModel(model string) bool {
	return strings.HasPrefix(model, "gpt-image-")
}

// isPostgres returns true when the database driver is PostgreSQL.
func isPostgres(driver string) bool {
	return driver == "postgres"
}

// isMySQL returns true when the database driver is MySQL.
func isMySQL(driver string) bool {
	return driver == "mysql"
}

func newUserRepo(driver string, db *sql.DB) repository.UserRepo {
	if isPostgres(driver) {
		return postgresqlrepository.NewUserRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewUserRepository(db)
	}
	return repository.NewUserRepository(db)
}

func newFailedLoginRepo(driver string, db *sql.DB) authdomain.FailedLoginAttemptRepo {
	if isPostgres(driver) {
		return postgresqlrepository.NewFailedLoginAttemptRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewFailedLoginAttemptRepository(db)
	}
	return repository.NewFailedLoginAttemptRepository(db)
}

func newVerificationTokenRepo(driver string, db *sql.DB) authdomain.VerificationTokenRepo {
	if isPostgres(driver) {
		return postgresqlrepository.NewVerificationTokenRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewVerificationTokenRepository(db)
	}
	return repository.NewVerificationTokenRepository(db)
}

func newMFAChallengeRepo(driver string, db *sql.DB) authdomain.MFAChallengeRepo {
	if isPostgres(driver) {
		return postgresqlrepository.NewMFAChallengeRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewMFAChallengeRepository(db)
	}
	return repository.NewMFAChallengeRepository(db)
}

func newPasswordResetTokenRepo(driver string, db *sql.DB) authdomain.PasswordResetTokenRepo {
	if isPostgres(driver) {
		return postgresqlrepository.NewPasswordResetTokenRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewPasswordResetTokenRepository(db)
	}
	return repository.NewPasswordResetTokenRepository(db)
}

func newBlockedEmailRepo(driver string, db *sql.DB) repository.BlockedEmailRepo {
	if isPostgres(driver) {
		return postgresqlrepository.NewBlockedEmailRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewBlockedEmailRepository(db)
	}
	return repository.NewBlockedEmailRepository(db)
}

func newSoftDeleteRepo(driver string, db *sql.DB) repository.SoftDeleteRepo {
	if isPostgres(driver) {
		return postgresqlrepository.NewSoftDeleteRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewSoftDeleteRepository(db)
	}
	return repository.NewSoftDeleteRepository(db)
}

func newEmailUpdateTokenRepo(driver string, db *sql.DB) repository.EmailUpdateTokenRepo {
	if isPostgres(driver) {
		return postgresqlrepository.NewEmailUpdateTokenRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewEmailUpdateTokenRepository(db)
	}
	return repository.NewEmailUpdateTokenRepository(db)
}

func newUserDataExportRepo(driver string, db *sql.DB) repository.UserDataExportRepo {
	if isPostgres(driver) {
		return postgresqlrepository.NewUserDataExportRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewUserDataExportRepository(db)
	}
	return repository.NewUserDataExportRepository(db)
}

func newUserDeletionRepo(driver string, db *sql.DB) repository.UserDeletionRepo {
	if isPostgres(driver) {
		return postgresqlrepository.NewUserDeletionRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewUserDeletionRepository(db)
	}
	return repository.NewUserDeletionRepository(db)
}

func newContentRepo(driver string, db *sql.DB) contentdomain.Repository {
	if isPostgres(driver) {
		return postgresqlrepository.NewContentRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewContentRepository(db)
	}
	return sqliterepository.NewContentRepository(db)
}

func newMediaRepo(driver string, db *sql.DB) mediadomain.Repository {
	drivers, err := appdatabase.Drivers()
	if err != nil {
		log.Fatalf("Failed to resolve database drivers: %v", err)
	}
	drv, err := drivers.Get(driver)
	if err != nil {
		log.Fatalf("Failed to resolve database driver %q: %v", driver, err)
	}
	return mediastore.New(db, drv.Dialect())
}

// newStorage builds the file storage backend selected by STORAGE_DRIVER.
// keyPrefix is the object key prefix / folder for the resource ("media/" or
// "profile_pictures/"); localDir and localURLPrefix apply to the local driver.
func newStorage(driver string, cfg *config.Config, keyPrefix string, localDir string, localURLPrefix string) (appstorage.Storage, error) {
	if driver == "s3" {
		return appstorage.NewS3Storage(appstorage.S3Options{
			Endpoint:        cfg.StorageS3Endpoint,
			Region:          cfg.StorageS3Region,
			Bucket:          cfg.StorageS3Bucket,
			AccessKeyID:     cfg.StorageS3AccessKeyID,
			SecretAccessKey: cfg.StorageS3SecretAccessKey,
			UsePathStyle:    cfg.StorageS3UsePathStyle,
			PublicBaseURL:   cfg.StorageS3PublicBaseURL,
			KeyPrefix:       keyPrefix,
		})
	}
	return appstorage.NewLocalStorage(localDir, localURLPrefix), nil
}

func newDashboardRepo(driver string, db *sql.DB) dashboarddomain.Repository {
	if isPostgres(driver) {
		return postgresqlrepository.NewDashboardRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewDashboardRepository(db)
	}
	return sqliterepository.NewDashboardRepository(db)
}

func newAPIKeyRepo(driver string, db *sql.DB) apikeydomain.Repository {
	if isPostgres(driver) {
		return postgresqlrepository.NewAPIKeyRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewAPIKeyRepository(db)
	}
	return sqliterepository.NewAPIKeyRepository(db)
}

func newAliasRepo(driver string, db *sql.DB) aliasdomain.Repository {
	drivers, err := appdatabase.Drivers()
	if err != nil {
		log.Fatalf("Failed to resolve database drivers: %v", err)
	}
	drv, err := drivers.Get(driver)
	if err != nil {
		log.Fatalf("Failed to resolve database driver %q: %v", driver, err)
	}
	return seostore.New(db, drv.Dialect())
}

func ensureDirectories(cfg *config.Config, logger *util.Logger) error {
	directories := []string{
		"plugins",
		"data",
	}

	// Ensure all directories exist
	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Log the database path for visibility
	if cfg.DBDriver == "sqlite" && cfg.DBPath != "" {
		logger.Info("Database will be stored at: %s", cfg.DBPath)
	}

	return nil
}

func startServer(
	cfg *config.Config,
	utilLogger *util.Logger,
	authHandler routes.AuthSurface,
	firstLoginHandler routes.FirstLoginSurface,
	notificationHandler *handlers.NotificationHandler,
	userManagementHandler routes.UserManagementSurface,
	profileHandler routes.ProfileSurface,
	contentHandler *handlers.ContentHandler,
	mediaHandler routes.MediaBrowserHandler,
	dashboardHandler *handlers.DashboardHandler,
	seoHandler routes.SEOSurface,
	commentHandler routes.CommentBrowserHandler,
	profilePictureHandler routes.ProfilePictureSurface,
	wordPressHandler routes.WordPressSurface,
	apiKeyHandler *handlers.APIKeyHandler,
	hugoHandler routes.HugoSurface,
	exportHandler *handlers.ExportHandler,
	ssgHandler routes.SSGSurface,
	configExtras func() map[string]any,
	apiKeyAuthMiddleware *middleware.APIKeyAuthMiddleware,
	agentContentHandler *agent.ContentHandler,
	agentMediaHandler routes.MediaAgentHandler,
	agentCommentHandler routes.CommentAgentHandler,
	adminMiddleware *middleware.AdminMiddleware,
	corsMiddleware *middleware.CORSMiddleware,
	noCookieMiddleware *middleware.NoCookieMiddleware,
	csrfMiddleware *middleware.CSRFMiddleware,
	rateLimitMiddleware *middleware.RateLimitMiddleware,
	securityHeadersMiddleware *middleware.SecurityHeadersMiddleware,
	authMiddleware *middleware.AuthMiddleware,
	spaHandler routes.AdminPanelSurface,
	staticHandler http.Handler,
	imageGenEnabled bool,
	imageReferenceEnabled bool,
	textGenEnabled bool,
	textGenHandler routes.TextGenSurface,
	speechEnabled bool,
	speechHandler routes.SpeechSurface,
	languages []string,
	aliasRedirectHandler *handlers.AliasRedirectHandler,
	headlessEnabled bool,
	commentsEnabled bool,
	storageDriver string,
	authorize kernel.CapabilityResolver,
	moduleRegistry *kernel.Registry,
) *http.Server {
	// Setup HTTP router
	router := routes.Setup(
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
		wordPressHandler,
		apiKeyHandler,
		hugoHandler,
		exportHandler,
		ssgHandler,
		configExtras,
		apiKeyAuthMiddleware,
		agentContentHandler,
		agentMediaHandler,
		agentCommentHandler,
		adminMiddleware,
		corsMiddleware,
		noCookieMiddleware,
		csrfMiddleware,
		rateLimitMiddleware,
		securityHeadersMiddleware,
		authMiddleware,
		spaHandler,
		staticHandler,
		imageGenEnabled,
		imageReferenceEnabled,
		textGenEnabled,
		textGenHandler,
		speechEnabled,
		speechHandler,
		languages,
		aliasRedirectHandler,
		headlessEnabled,
		commentsEnabled,
		storageDriver,
		authorize,
		moduleRegistry,
	)

	// Create HTTP server with timeouts. ReadHeaderTimeout guards against	// Slowloris while ReadTimeout and WriteTimeout are zero (off) by default
	// so large uploads (WordPress import, media upload) are not capped by a
	// global deadline — per-handler body limiters and context deadlines apply.
	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: cfg.ServerReadHeaderTimeout,
		ReadTimeout:       cfg.ServerReadTimeout,
		WriteTimeout:      cfg.ServerWriteTimeout,
		IdleTimeout:       60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		utilLogger.Info("Server started on %s:%d", cfg.Host, cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utilLogger.Fatal("Server failed to start: %v", err)
		}
	}()

	return server
}

// noCookieAllowlist names the only cookies allowed past the NoCookie guard:
// the SSR engine's language preference, the GDPR consent choice, the
// browser session cookie, and its JS-readable auth hint. It exists so
// TestNoCookieAllowlistMatchesEngine can pin the wiring against the engine's
// canonical constants (importing the engine here would link the SSR stack
// into lean binaries like lesstruct-core).
func noCookieAllowlist() []string {
	return []string{"lesstruct_lang", "lesstruct_consent", kernel.SessionCookieName, kernel.AuthHintCookieName}
}

// Run boots an edition: loads config, wires the shared platform services,
// hands the edition its Wire and Mount hooks, runs migrations, and serves
// HTTP until shutdown.
func Run(edition Edition) {
	// Initialize logger
	utilLogger := util.NewLogger(os.Stdout)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Load headless and comments feature flags. Absent blocks keep the
	// current behavior: headless off, comments on.
	headlessConfig, err := config.LoadHeadless(cfg)
	if err != nil {
		log.Fatalf("Failed to load headless config: %v", err)
	}
	headlessEnabled := headlessConfig.Enabled

	commentsConfig, err := config.LoadComments(cfg)
	if err != nil {
		log.Fatalf("Failed to load comments config: %v", err)
	}

	modulesConfig, err := config.LoadModules(cfg)
	if err != nil {
		log.Fatalf("Failed to load modules config: %v", err)
	}

	registrationConfig, err := config.LoadRegistration(cfg)
	if err != nil {
		log.Fatalf("Failed to load registration config: %v", err)
	}
	// Back-compat: disabling the comments module is equivalent to the
	// legacy [comments] toggle. Headless mode is independent (D4): it only
	// suppresses public-surface mounting, never the module registry. An
	// edition without the comment system folds the same way: no module,
	// no comment routes, no comment surface in the content service.
	commentsEnabled := commentsConfig.IsEnabled() && edition.Comments && !modulesConfig.IsDisabled("comments")
	registrationEnabled := registrationConfig.IsEnabled(commentsEnabled)

	// Ensure required directories exist
	if err := ensureDirectories(cfg, utilLogger); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	utilLogger.Info("Starting Lesstruct...")

	// Initialize database
	dbDSN := cfg.DBPath
	if cfg.DBDriver == "postgres" || cfg.DBDriver == "mysql" {
		dbDSN = cfg.DBDSN
	}
	db, err := appdatabase.Open(cfg.DBDriver, dbDSN, cfg.DBPoolMaxConns)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			utilLogger.Error("Failed to close database: %v", err)
		}
	}()

	// Run migrations
	if err := db.RunMigrations(cfg.DBDriver); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	utilLogger.Info("Database initialized successfully")

	// v1 module system: the kernel host and registry are created here; modules
	// register below once their construction inputs exist (role registry,
	// capability resolver), and InitAll runs after the last registration,
	// just before the plugin system starts.
	moduleHost := kernel.NewHost(modules.NewLogger(utilLogger))
	moduleRegistry := moduleHost.Registry()

	// Initialize plugin system
	pluginLogFn := func(msg string) { utilLogger.Info("%s", msg) }

	// Create host function registry with real HTTP client and database access
	// for plugins that declare capabilities in their manifest.
	hostFuncRegistry := hostfunctions.NewRegistry(
		&http.Client{Timeout: 10 * time.Second},
		db.DB(),
		pluginLogFn,
	)

	pluginSys, err := bootstrap.NewPluginSystem(
		context.Background(),
		"plugins",
		cfg.DevMode,
		hostFuncRegistry,
		pluginLogFn,
	)
	if err != nil {
		log.Fatalf("Failed to initialize plugin system: %v", err)
	}

	// Initialize JWT manager (needed early for routes)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)

	// Initialize repository
	userRepo := newUserRepo(cfg.DBDriver, db.DB())
	failedLoginRepo := newFailedLoginRepo(cfg.DBDriver, db.DB())
	tokenRepo := newVerificationTokenRepo(cfg.DBDriver, db.DB())

	// (#3) Validate DefaultUsername is safe before using in SQL
	if !isValidUsername(constants.DefaultUsername) {
		log.Fatalf("Invalid default username: %s", constants.DefaultUsername)
	}

	// Create default admin user if not exists
	defaultPasswordHash, err := auth.HashPassword(constants.DefaultPassword)
	if err != nil {
		log.Fatalf("Failed to hash default password: %v", err)
	}

	// (#6) Check if admin user exists and create if needed
	if _, err := userRepo.GetAdminUser(context.Background()); err != nil {
		// Admin user doesn't exist, create it
		utilLogger.Info("Admin user not found, creating default admin user")
		if isPostgres(cfg.DBDriver) {
			_, err = db.DB().Exec(`
				INSERT INTO users (username, password_hash, role, status)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (username) DO NOTHING
			`, constants.DefaultUsername, defaultPasswordHash, constants.DefaultRole, "verified")
		} else if isMySQL(cfg.DBDriver) {
			_, err = db.DB().Exec(`
				INSERT IGNORE INTO users (username, password_hash, role, status)
				VALUES (?, ?, ?, ?)
			`, constants.DefaultUsername, defaultPasswordHash, constants.DefaultRole, "verified")
		} else {
			_, err = db.DB().Exec(`
				INSERT OR IGNORE INTO users (username, password_hash, role, status)
				VALUES (?, ?, ?, ?)
			`, constants.DefaultUsername, defaultPasswordHash, constants.DefaultRole, "verified")
		}
		if err != nil {
			log.Fatalf("Failed to create default admin user: %v", err)
		}
		// (#13) Reduce logging exposure - don't announce username
		utilLogger.Info("Default admin user created. Please change password after first login.")
	}

	// Credential policy for the users and auth domains: platform
	// password/email/username rules behind the module seams.
	credentialPolicy := modules.NewUserCredentialPolicy()

	authService := authdomain.NewAuthService(defaultPasswordHash, credentialPolicy)
	firstLoginService := authdomain.NewFirstLoginService(credentialPolicy)
	registrationService := authdomain.NewRegistrationService(
		userRepo,
		credentialPolicy,
		commentsEnabled,
		authdomain.WithEnabled(registrationEnabled),
		authdomain.WithDefaultRole(registrationConfig.DefaultRole),
	)
	verificationOpts := make([]authdomain.VerificationOption, 0, 1)
	if registrationConfig.AdminApproval {
		verificationOpts = append(verificationOpts, authdomain.WithAdminApprovalRequired())
	}
	verificationService := authdomain.NewVerificationService(userRepo, tokenRepo, 24, verificationOpts...)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, credentialPolicy, modules.NewLogger(utilLogger))

	mfaChallengeRepo := newMFAChallengeRepo(cfg.DBDriver, db.DB())
	mfaService := authdomain.NewMFAService(
		mfaChallengeRepo,
		authdomain.MFACodeTTL,
		authdomain.MFAResendCooldown,
		authdomain.MFAMaxAttempts,
	)

	passwordResetTokenRepo := newPasswordResetTokenRepo(cfg.DBDriver, db.DB())
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, credentialPolicy, 1)

	notificationRepo := repository.NewInMemoryNotificationRepository()

	// Initialize email service
	smtpConfig := email.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUser,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
	}
	// SITE_URL is the canonical public origin: it drives SEO meta tags,
	// sitemaps, robots.txt and author profile links so they stay correct
	// behind reverse proxies and HTTPS (HOST/PORT only bind the listener).
	baseURL := strings.TrimRight(cfg.SiteURL, "/")
	emailService := email.NewService(smtpConfig, logger, cfg.SiteURL)
	mfaDeliverer := modules.NewMFACodeDeliverer(
		emailService,
		modules.NewLogger(utilLogger),
		cfg.DevMode,
		emailService.IsEnabled(),
	)

	blockedEmailRepo := newBlockedEmailRepo(cfg.DBDriver, db.DB())

	// Module candidates accumulate here in construction order; Phase 4.1
	// prunes the [modules] disabled list (plus cascade) and registers only
	// the kept modules just before InitAll. Handler extraction stays inline
	// — instances exist regardless of registration.
	var candidates []kernel.Module

	// v1 module system (Phase 4.0d): the real auth module adopts the auth
	// services and the session/notification/mail collaborators, building
	// both handlers once. Core mounts the handlers explicitly (see
	// Module.AuthHandler/FirstLoginHandler): the surface spans the public
	// auth group, which a single RegisterRoutes call cannot express.
	authModuleOpts := []authmodule.Option{
		authmodule.WithAuthService(authService),
		authmodule.WithFirstLoginService(firstLoginService),
		authmodule.WithRegistrationService(registrationService),
		authmodule.WithVerificationService(verificationService),
		authmodule.WithLoginService(loginService),
		authmodule.WithMFAService(mfaService),
		authmodule.WithPasswordResetService(passwordResetService),
		authmodule.WithTokenIssuer(modules.NewJWTTokenIssuer(jwtManager)),
		authmodule.WithUserStore(userRepo),
		authmodule.WithFailedLoginStore(failedLoginRepo),
		authmodule.WithNotificationRecorder(modules.NewAuthNotificationRecorder(notificationRepo)),
		authmodule.WithMailer(emailService),
		authmodule.WithMFADeliverer(mfaDeliverer),
		authmodule.WithBlockedEmailChecker(blockedEmailRepo),
		authmodule.WithCredentialPolicy(credentialPolicy),
		authmodule.WithLogger(modules.NewLogger(utilLogger)),
	}
	if registrationConfig.AdminApproval {
		authModuleOpts = append(authModuleOpts, authmodule.WithAdminApprovalRequired())
	}
	authModule := authmodule.New(authModuleOpts...)
	candidates = append(candidates, authModule)
	authHandler := authModule.AuthHandler()
	firstLoginHandler := authModule.FirstLoginHandler()

	softDeleteRepo := newSoftDeleteRepo(cfg.DBDriver, db.DB())

	// Initialize profile management dependencies (Story 1.7)
	emailUpdateTokenRepo := newEmailUpdateTokenRepo(cfg.DBDriver, db.DB())
	userDataExportRepo := newUserDataExportRepo(cfg.DBDriver, db.DB())
	// Initialize post type service and handler (Story 2.6)
	// Must be initialized before profile service for system field stripping
	postTypeService, err := config.LoadPostTypes(cfg)
	if err != nil {
		log.Printf("Warning: Failed to load post types config, using defaults: %v", err)
		postTypeService = posttype.NewService()
	}

	// Initialize the config-driven role registry (Story: roles). Custom
	// [[role]] entries may only reference registered post types, so the slug
	// list is collected from the post type service after loading. Production
	// always runs with this registry; the user/content services fall back to
	// the legacy three-role allowlist only when it is absent.
	postTypeSlugs := make([]string, 0, len(postTypeService.GetAll()))
	for _, pt := range postTypeService.GetAll() {
		postTypeSlugs = append(postTypeSlugs, pt.Slug)
	}
	roleService, err := config.LoadRoles(cfg, postTypeSlugs)
	if err != nil {
		log.Fatalf("Failed to load roles config: %v", err)
	}

	// The [registration] default role must resolve against the role registry.
	// Fail closed at startup so a typo never silently creates users with an
	// unregistered role (who would have zero capabilities). "Admin" is also
	// rejected — public registration must never default to a superuser.
	effectiveDefaultRole := registrationConfig.DefaultRole
	if effectiveDefaultRole == "" {
		effectiveDefaultRole = constants.RoleCommentator
	}
	defaultRole, err := roleService.Get(effectiveDefaultRole)
	if err != nil {
		log.Fatalf("Invalid [registration] default_role %q: %v", effectiveDefaultRole, err)
	}
	if defaultRole.IsAdmin {
		log.Fatalf("Invalid [registration] default_role %q: public registration cannot default to an admin role", effectiveDefaultRole)
	}

	// Capability resolver for declarative route protection (Phase 2 identity
	// seam). Decisions delegate to the role registry; unknown capabilities
	// and unknown roles fail closed.
	authorize := middleware.NewCapabilityResolver(roleService)

	// Media stack (Story 2.3). Built here — ahead of module registration — so
	// the media module adopts the live service instead of a nil placeholder.
	// Thumbnail config needs cfg only; the repo, storage, and service need
	// the open database.
	thumbnailService, err := config.LoadThumbnails(cfg)
	if err != nil {
		log.Fatalf("Failed to load thumbnail config: %v", err)
	}
	uploadsDir := filepath.Join("data", "uploads", "media")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		log.Fatalf("Failed to create uploads directory: %v", err)
	}
	mediaRepo := newMediaRepo(cfg.DBDriver, db.DB())
	mediaStorage, err := newStorage(cfg.StorageDriver, cfg, "media/", uploadsDir, "/uploads/media/")
	if err != nil {
		log.Fatalf("Failed to initialize media storage: %v", err)
	}
	mediaService := mediadomain.NewService(mediaRepo, mediaStorage, thumbnailService)

	// Featured-image resolution for the public content-items list via the
	// same media-join the SSR layer uses. Defined alongside the repo; the
	// content handler adopts it below.
	featuredImageResolver := func(imageURL string) string {
		hash := mediadomain.ExtractHashFromURL(imageURL)
		if hash == "" {
			return imageURL
		}
		m, err := mediaRepo.FindByHashPrefix(context.Background(), hash)
		if err != nil || m == nil {
			return imageURL
		}
		if thumb, ok := m.Variants["_thumb"]; ok {
			return thumb.URL
		}
		return imageURL
	}

	var imageGenService mediadomain.ImageGenerationService
	if cfg.IsImageGenerationEnabled() {
		switch {
		case isGeminiModel(cfg.AIImageGenerationModel):
			imageGenService = mediainfra.NewGoogleGeminiService(
				cfg.AIImageGenerationAPIKey,
				cfg.AIImageGenerationModel,
				cfg.AIImageGenerationSize,
				cfg.AIImageGenerationAspectRatio,
			)
		case isGPTModel(cfg.AIImageGenerationModel):
			imageGenService = mediainfra.NewGPTImageService(
				cfg.AIImageGenerationAPIKey,
				cfg.AIImageGenerationModel,
				cfg.AIImageGenerationSize,
			)
		default:
			imageGenService = mediainfra.NewGoogleImagen4Service(
				cfg.AIImageGenerationAPIKey,
				cfg.AIImageGenerationModel,
				cfg.AIImageGenerationSize,
				cfg.AIImageGenerationAspectRatio,
			)
		}
	}

	// Reference images are advertised to the admin panel only when the
	// configured AI model accepts them (Gemini and GPT Image models do;
	// the default Imagen models are text-to-image only).
	imageReferenceEnabled := imageGenService != nil && imageGenService.SupportsImageReferences()

	// Initialize languages configuration (i18n) — loaded early so content
	// service and WordPress importer can use the site's primary language.
	languages, err := config.LoadLanguages(cfg)
	if err != nil {
		log.Printf("Warning: Failed to load languages config, using defaults: %v", err)
		languages = []string{"en"}
	}
	primaryLanguage := config.PrimaryLanguage(languages)

	// Initialize content service (Story 2.1). Built here — ahead of module
	// registration — so edition modules adopt the live services instead of
	// nil placeholders. Comment repositories belong to the edition that
	// carries the comment system; the content engine itself is comment-free.
	contentRepo := newContentRepo(cfg.DBDriver, db.DB())
	seoService := seodomain.NewService(baseURL, "Lesstruct")
	postTypeAdapter := contentdomain.NewPostTypeAdapter(postTypeService)
	// Initialize alias/redirect service before the content service so delete
	// cascades its alias rows (Hugo import also depends on it).
	aliasRepo := newAliasRepo(cfg.DBDriver, db.DB())
	aliasService := aliasdomain.NewService(aliasRepo)
	contentService := contentdomain.NewServiceWithHooks(
		contentRepo,
		seoService,
		postTypeAdapter,
		pluginadapter.NewHookExecutorAdapter(pluginSys.Registry()),
		contentdomain.WithDefaultLanguage(primaryLanguage),
		contentdomain.WithLanguages(languages...),
		contentdomain.WithCommentsEnabled(commentsEnabled),
		contentdomain.WithRoleChecker(roleService),
		contentdomain.WithAliasDeleter(aliasService),
	)

	// Legal resolution (round 6): the conventional legal documents resolve
	// once at boot so checkout/registration enforcement, the verification
	// email footer, and rendering all refer to the same revision. A page
	// added while the server runs shows up in links immediately (the SSR
	// engine re-resolves per render) but the consent gates keep the boot
	// snapshot until restart — the documented trading off for not querying
	// legal pages on every checkout.
	legalConfig, err := config.LoadLegal(cfg)
	if err != nil {
		log.Fatalf("Failed to load legal config: %v", err)
	}
	legalResolution := resolveLegal(context.Background(), contentService, languages)
	legalDocumentsPresent := legalResolution.Has(legal.PageTermsOfService) && legalResolution.Has(legal.PagePrivacyPolicy)
	checkoutConsent := legalConfig.CheckoutConsentEnabled(legalDocumentsPresent)
	registrationConsent := legalConfig.RegistrationConsentEnabled(legalDocumentsPresent)
	legalOverrides := legalConfig.Overrides()

	termsDocument, _ := legalResolution.Document(legal.PageTermsOfService)
	privacyDocument, _ := legalResolution.Document(legal.PagePrivacyPolicy)
	registrationService.SetConsentPolicy(authdomain.ConsentPolicy{
		Required: registrationConsent,
		Terms:    termsDocument,
		Privacy:  privacyDocument,
	})
	if privacyURL := legalDocumentURL(legal.PagePrivacyPolicy, legalOverrides, legalResolution); privacyURL != "" {
		if strings.HasPrefix(privacyURL, "/") {
			privacyURL = baseURL + privacyURL
		}
		emailService.SetPrivacyPolicyURL(privacyURL)
	}

	// v1 module system (Phase 3.1): the real posttypes-basic module adopts the
	// config-built registry and replaces its transitional wrapper. Remaining
	// wrappers register alongside it; InitAll resolves the full graph.
	postTypesModule := posttypesbasic.New(
		posttypesbasic.WithService(postTypeService),
		posttypesbasic.WithCapabilityResolver(authorize),
		posttypesbasic.WithCommentsEnabled(commentsEnabled),
		posttypesbasic.WithRegistrationEnabled(registrationEnabled),
		posttypesbasic.WithRoles(modules.NewRoleCapabilities(roleService)),
	)
	for _, module := range modules.DefaultEdition() {
		if err := moduleRegistry.Register(module); err != nil {
			log.Fatalf("Failed to register module %q: %v", module.Name(), err)
		}
	}
	candidates = append(candidates, postTypesModule)
	// v1 module system (Phase 3.2): the real media module adopts the live
	// service, image-generation provider, role gates, and logger. Both realm
	// handlers serve the adopted instances, so hashing, dedup, WebP
	// conversion, and thumbnail variants run identically in both realms.
	// Core mounts the handlers explicitly (see Module.BrowserHandler):
	// the media surface spans three body-limit groups plus the dual-auth
	// dispatch, which a single RouteProvider call cannot express.
	mediaModule := media.New(
		media.WithService(mediaService),
		media.WithImageGeneration(imageGenService),
		media.WithRoles(modules.NewMediaRoleCapabilities(roleService)),
		media.WithAgentRoles(modules.NewAgentMediaRoleCapabilities(roleService)),
		media.WithLogger(modules.NewLogger(utilLogger)),
	)
	candidates = append(candidates, mediaModule)

	// Edition hand-off: the platform services are up; the edition constructs
	// its own modules on top of them and fills its optional handler set.
	// Platform modules that register after the edition's (the admin shell
	// and the users module, below) keep the historical candidate order.
	drivers, err := appdatabase.Drivers()
	if err != nil {
		log.Fatalf("Failed to resolve database drivers: %v", err)
	}
	dialect, err := drivers.Get(cfg.DBDriver)
	if err != nil {
		log.Fatalf("Failed to resolve database driver %q: %v", cfg.DBDriver, err)
	}
	core := &Core{
		Cfg:                 cfg,
		Logger:              utilLogger,
		DB:                  db.DB(),
		Dialect:             dialect.Dialect(),
		Registry:            moduleRegistry,
		RoleService:         roleService,
		PostTypeService:     postTypeService,
		PostTypeSlugs:       postTypeSlugs,
		Authorize:           authorize,
		ContentService:      contentService,
		MediaService:        mediaService,
		MediaRepo:           mediaRepo,
		AliasService:        aliasService,
		SEOService:          seoService,
		UserRepo:            userRepo,
		EmailService:        emailService,
		CommentsEnabled:     commentsEnabled,
		RegistrationEnabled: registrationEnabled,
		HeadlessEnabled:     headlessEnabled,
		Languages:           languages,
		PrimaryLanguage:     primaryLanguage,
		BaseURL:             baseURL,
		LegalResolution:     legalResolution,
		LegalOverrides:      legalOverrides,
		LegalNav:            legalConfig.Nav(),
		CheckoutConsent:     checkoutConsent,
		RegistrationConsent: registrationConsent,
		handlers:            &HandlerSet{},
	}
	if edition.Wire != nil {
		if err := edition.Wire(core); err != nil {
			log.Fatalf("Failed to wire %s edition: %v", edition.Name, err)
		}
	}

	// v1 module system (Phase 3.8b): the real adminpanel module serves the
	// admin SPA (embedded production build, dev-server proxy when enabled).
	adminpanelModule := adminpanel.New(
		adminpanel.WithDevMode(cfg.DevMode),
		adminpanel.WithAdminDevURL(cfg.AdminDevURL),
	)
	candidates = append(candidates, adminpanelModule)
	spaHandler := adminpanelModule.SPAHandler()

	// v1 module system: the real users module adopts the management/profile
	// services and the user/soft-delete/mail/field collaborators, building
	// all three handlers once. It registers alongside every other module so
	// InitAll covers it (init order, migrations, shutdown); core mounts the
	// handlers explicitly (see Module accessors) because the surface spans
	// the admin and profile groups, which a single RegisterRoutes call
	// cannot express.
	profileService := user.NewProfileService(userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService, credentialPolicy, postTypeService)

	// Initialize user deletion repository and service (Story 1.8)
	userDeletionRepo := newUserDeletionRepo(cfg.DBDriver, db.DB())
	accountDeletionService := user.NewAccountDeletionService(userRepo, userDeletionRepo, emailService, modules.NewLogger(utilLogger))

	// Initialize profile picture storage (needed by profile handler for URL building)
	profilePicturesDir := filepath.Join("data", "uploads", "profile_pictures")
	if err := os.MkdirAll(profilePicturesDir, 0755); err != nil {
		log.Fatalf("Failed to create profile pictures directory: %v", err)
	}
	profilePictureStorage, err := newStorage(cfg.StorageDriver, cfg, "profile_pictures/", profilePicturesDir, "/uploads/profile_pictures/")
	if err != nil {
		log.Fatalf("Failed to initialize profile picture storage: %v", err)
	}
	core.ProfilePicture = profilePictureStorage

	// Initialize profile picture service (Story 12.8)
	profilePictureService := profilepicture.NewService(
		modules.NewProfilePictureUserRepo(userRepo),
		profilePictureStorage,
		profilepicture.NewProcessor(),
	)

	// Initialize user management service and handler
	userManagementOpts := []user.UserManagementOption{user.WithManagementRoleService(roleService), user.WithManagementRegistrationEnabled(registrationEnabled)}
	if registrationConfig.AdminApproval {
		userManagementOpts = append(userManagementOpts, user.WithApprovalEmailRequired())
	}
	userManagementService := user.NewUserManagementService(userRepo, blockedEmailRepo, credentialPolicy, commentsEnabled, userManagementOpts...)
	adminCreateUserService := user.NewAdminCreateUserService(userRepo, blockedEmailRepo, credentialPolicy, commentsEnabled, user.WithRoleService(roleService), user.WithRegistrationEnabled(registrationEnabled))
	usersModule := usersmodule.New(
		usersmodule.WithUserManagementService(userManagementService),
		usersmodule.WithAdminCreateUserService(adminCreateUserService),
		usersmodule.WithProfileService(profileService),
		usersmodule.WithAccountDeletionService(accountDeletionService),
		usersmodule.WithProfilePictureService(profilePictureService),
		usersmodule.WithUserStore(userRepo),
		usersmodule.WithSoftDeleteStore(modules.NewSoftDeleteStore(softDeleteRepo)),
		usersmodule.WithMailer(emailService),
		usersmodule.WithUserFieldsProvider(postTypeService),
		usersmodule.WithProfilePictureURLBuilder(profilePictureStorage.GetURL),
		usersmodule.WithLogger(modules.NewLogger(utilLogger)),
	)
	candidates = append(candidates, usersModule)
	userManagementHandler := usersModule.UserManagementHandler()
	profileHandler := usersModule.ProfileHandler()
	profilePictureHandler := usersModule.ProfilePictureHandler()

	// The edition's Wire appended its modules to core.Candidates; fold them
	// into the candidate list so registration, InitAll, migrations, and the
	// admin manifest cover them exactly like the platform's own modules.
	candidates = append(candidates, core.Candidates...)

	// v1 module system (Phase 4.1): prune the [modules] disabled list plus
	// every transitive Requires dependent (cascade), warn on each drop and
	// on unknown names (configs move between editions; typos must not
	// break startup), and register only the kept modules. InitAll still
	// fails fast when a kept module's hard dependency is absent.
	kept, err := modules.ApplyDisabled(moduleRegistry, candidates, modulesConfig.Disabled, log.Printf)
	if err != nil {
		log.Fatalf("Failed to register modules: %v", err)
	}
	// A cascaded comments drop (e.g. via disabled users) must unmount the
	// comment routes exactly like the legacy toggle: every route gate reads
	// this flag. Services baked earlier keep the direct-disable value; under
	// cascade their comment paths are unreachable with routes unmounted.
	commentsEnabled = modules.EffectiveCommentsEnabled(commentsEnabled, modulesConfig.Disabled, kept)

	if err := moduleRegistry.InitAll(moduleHost); err != nil {
		log.Fatalf("Failed to initialize modules: %v", err)
	}
	utilLogger.Info("Modules initialized: %s", strings.Join(moduleRegistry.InitOrder(), ", "))

	// Install the default theme artifact into the template engine. The first
	// registered ThemeProvider wins (registry order is by name); THEME_DIR
	// overrides resolve over it. Without a provider only on-disk files
	// resolve — headless editions never render, so they are unaffected.
	// The install hook is set by the edition that carries the SSR engine;
	// platform code never imports a theme or the engine directly.
	if providers := kernel.Capabilities[kernel.ThemeProvider](moduleRegistry); len(providers) > 0 && core.ThemeInstaller != nil {
		core.ThemeInstaller(providers[0].ThemeFiles())
	}

	// Module migrations run after the core sequence: each Migrator versions
	// independently in its own schema_migrations_<module> table.
	if err := appdatabase.MigrateModules(db.DB(), cfg.DBDriver, moduleRegistry); err != nil {
		log.Fatalf("Failed to run module migrations: %v", err)
	}

	// Load the public-field allowlist that gates /api/v1/public/* cf_*/sort_by
	// queries. A missing or invalid block yields an empty registry, which fails
	// closed — every cf_*/sort_by=cf:* request then returns 400
	// field_not_queryable. Operators must add [[public_field]] entries to
	// config.toml to opt in.
	publicFieldRegistry, err := config.LoadPublicFields(cfg)
	if err != nil {
		log.Printf("Warning: Failed to load public_field config, continuing with closed registry: %v", err)
		publicFieldRegistry = &config.PublicFieldRegistry{}
	}

	// CSP-derived security headers (platform — every edition serves them) and
	// the sanitizer iframe allowlist derived from the same frame-src directive.
	cspConfig, err := config.LoadCSPConfig(cfg)
	if err != nil {
		log.Printf("Warning: Failed to load CSP config, continuing with defaults: %v", err)
		cspConfig = config.CSPConfig{}
	}
	cspHeaderName, cspHeaderValue := cspConfig.Build()
	securityHeadersMiddleware := middleware.NewSecurityHeadersMiddleware(cspHeaderName, cspHeaderValue, cspConfig.XFrameOptions(), cfg.HSTSMaxAge)
	iframeHosts := cspConfig.IFrameHosts()

	// The disabled cascade may have dropped the comment system transitively;
	// refresh the effective flags before the edition mounts its handlers.
	commentsEnabled = modules.EffectiveCommentsEnabled(commentsEnabled, modulesConfig.Disabled, kept)
	core.CommentsEnabled = commentsEnabled
	core.RegistrationEnabled = registrationEnabled
	core.IFrameHosts = iframeHosts
	core.PublicFieldRegistry = publicFieldRegistry
	if edition.Mount != nil {
		if err := edition.Mount(core); err != nil {
			log.Fatalf("Failed to mount %s edition: %v", edition.Name, err)
		}
	}
	hs := core.Handlers()

	// Agent (Bearer) content handler — the streamlined API surface for
	// programmatic publishing (Story 2.1). Reuses the same contentService so custom
	// field validation and plugin hooks fire identically to the admin path.
	agentContentHandler := agent.NewContentHandler(contentService, postTypeService, utilLogger)

	// Public content handler — resolves featured images (thumbnail variants) for the
	// public API content-items list via the media-join resolver defined above
	// alongside the media stack.
	contentHandler := handlers.NewContentHandler(contentService, utilLogger, baseURL, profilePictureStorage.GetURL, featuredImageResolver)
	contentHandler = contentHandler.WithPublicFieldRegistry(publicFieldRegistry)

	// Initialize dashboard service and handler (Story 2.8)
	dashboardRepo := newDashboardRepo(cfg.DBDriver, db.DB())
	dashboardService := dashboarddomain.NewService(dashboardRepo)
	dashboardHandler := handlers.NewDashboardHandler(dashboardService, postTypeService, utilLogger)

	notificationHandler := handlers.NewNotificationHandler(utilLogger, notificationRepo)

	authMiddleware := middleware.NewAuthMiddleware(jwtManager, userRepo)
	adminMiddleware := middleware.NewAdminMiddleware(authMiddleware)

	// Initialize API key service and handler (Story 1.1)
	apiKeyRepo := newAPIKeyRepo(cfg.DBDriver, db.DB())
	apiKeyService := apikeydomain.NewService(apiKeyRepo, cfg.APIKeyPepper)
	apiKeyHandler := handlers.NewAPIKeyHandler(
		apiKeyService,
		utilLogger,
	)

	// Initialize the Bearer API-key auth middleware (Story 1.4 built it; Story 2.1
	// mounts it). It injects the owning user via the shared identity keys so the v1
	// handlers read identity auth-agnostically. userRepo satisfies UserLookup.
	apiKeyAuthMiddleware := middleware.NewAPIKeyAuthMiddleware(
		apiKeyService,
		userRepo,
		utilLogger,
	)

	// Initialize CORS middleware with allowed origins from config
	corsMiddleware := middleware.NewCORSMiddleware(cfg.ParseCORSOrigins(), utilLogger)

	// Initialize no-cookie enforcement middleware (Story 6.1). The guard stays
	// fail-closed: only the language preference and the GDPR consent choice
	// pass through; every other Set-Cookie is stripped and logged. Platform
	// code never imports the engine, so the names live in noCookieAllowlist
	// (pinned against the engine constants by TestNoCookieAllowlistMatchesEngine).
	noCookieMiddleware := middleware.NewNoCookieMiddleware(utilLogger, noCookieAllowlist()...)

	// Initialize CSRF middleware (Story 5.2)
	csrfMiddleware := middleware.NewCSRFMiddleware(utilLogger, cfg.ParseCORSOrigins(), jwtManager)

	// Initialize rate limit middleware (Story 5.4)
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(
		cfg.RateLimitEnabled,
		cfg.RateLimitAuthPerMinute,
		cfg.RateLimitAPIPerMinute,
		cfg.RateLimitPublicPerMinute,
	)
	// Outcome-based Bearer limiter: refuse an IP once its Bearer 401s
	// exceed the configured budget (finding-10 remediation).
	rateLimitMiddleware.SetAgentFailuresPerMinute(cfg.AgentFailuresPerMinute)

	// (#13) Reduce logging exposure
	utilLogger.Info("Admin credentials configured. Please change password after first login.")

	// The sanitizer's iframe allowlist mirrors the CSP frame-src directive
	// (defaults + operator appends) so embeds the CSP allows to be framed are
	// also allowed through the HTML sanitizer on every path. The SSR edition
	// applies the same hosts to its content-page handler in Mount.
	contentHandler = contentHandler.WithIFrameHosts(iframeHosts...)
	agentContentHandler = agentContentHandler.WithIFrameHosts(iframeHosts...)

	// Initialize content exporter (Phase 2) — exports all content as Hugo-compatible
	// source files with bundled media. Engine-level: every edition can hand its
	// content back as portable source files.
	exporter := export.NewExporter(
		contentService,
		aliasService,
		func(s string) (string, error) {
			return tiptap.NewRenderer(nil).Render(s)
		},
		filepath.Join("data", "uploads", "media"),
		utilLogger,
	)
	exportHandler := handlers.NewExportHandler(exporter, utilLogger)

	// Start HTTP server
	server := startServer(
		cfg,
		utilLogger,
		authHandler,
		firstLoginHandler,
		notificationHandler,
		userManagementHandler,
		profileHandler,
		contentHandler,
		mediaModule.BrowserHandler(),
		dashboardHandler,
		hs.SEO,
		hs.CommentBrowser,
		profilePictureHandler,
		hs.WordPress,
		apiKeyHandler,
		hs.Hugo,
		exportHandler,
		hs.SSG,
		hs.ConfigExtras,
		apiKeyAuthMiddleware,
		agentContentHandler,
		mediaModule.AgentHandler(),
		hs.CommentAgent,
		adminMiddleware,
		corsMiddleware,
		noCookieMiddleware,
		csrfMiddleware,
		rateLimitMiddleware,
		securityHeadersMiddleware,
		authMiddleware,
		spaHandler,
		hs.Static,
		cfg.IsImageGenerationEnabled(),
		imageReferenceEnabled,
		cfg.IsTextGenerationEnabled() && hs.TextGen != nil,
		hs.TextGen,
		cfg.IsSpeechToTextEnabled() && cfg.IsTextGenerationEnabled() && hs.Speech != nil,
		hs.Speech,
		languages,
		hs.AliasRedirect,
		headlessEnabled,
		commentsEnabled,
		cfg.StorageDriver,
		authorize,
		moduleRegistry,
	)

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	utilLogger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		utilLogger.Error("Server forced to shutdown: %v", err)
	}

	_ = pluginSys.Close(context.Background())

	if err := moduleRegistry.ShutdownAll(); err != nil {
		utilLogger.Error("Module shutdown reported errors: %v", err)
	}

	utilLogger.Info("Server exited")
}
