// Package app is the Lesstruct composition root: the default (blog)
// edition wires every first-party module here and runs the server. An
// edition is this list plus wiring — see cmd/ for alternate editions that
// reuse Run with different build constraints.
package app

import (
	"cmp"
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
	"slices"
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
	hugo "github.com/aristorinjuang/lesstruct/modules/importers/hugo"
	"github.com/aristorinjuang/lesstruct/modules/importers"
	"github.com/aristorinjuang/lesstruct/internal/content/ssg"
	"github.com/aristorinjuang/lesstruct/kernel/tiptap"
	wordpress "github.com/aristorinjuang/lesstruct/modules/importers/wordpress"
	appdatabase "github.com/aristorinjuang/lesstruct/internal/database"
	apikeydomain "github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	dashboarddomain "github.com/aristorinjuang/lesstruct/internal/domain/dashboard"
	usersmodule "github.com/aristorinjuang/lesstruct/modules/users"
	"github.com/aristorinjuang/lesstruct/modules/users/domain/profilepicture"
	textgendomain "github.com/aristorinjuang/lesstruct/modules/textgen/domain"
	"github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/aristorinjuang/lesstruct/internal/email"
	"github.com/aristorinjuang/lesstruct/kernel/i18n"
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
	"github.com/aristorinjuang/lesstruct/modules/comments"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	authmodule "github.com/aristorinjuang/lesstruct/modules/auth"
	commentstore "github.com/aristorinjuang/lesstruct/modules/comments/store"
	"github.com/aristorinjuang/lesstruct/modules/adminpanel"
	"github.com/aristorinjuang/lesstruct/modules/media"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	mediastore "github.com/aristorinjuang/lesstruct/modules/media/store"
	posttypesbasic "github.com/aristorinjuang/lesstruct/modules/posttypes-basic"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
	"github.com/aristorinjuang/lesstruct/modules/seo"
	textgenmodule "github.com/aristorinjuang/lesstruct/modules/textgen"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	themedefault "github.com/aristorinjuang/lesstruct/modules/theme-default"
	aliasdomain "github.com/aristorinjuang/lesstruct/modules/seo/alias"
	seodomain "github.com/aristorinjuang/lesstruct/modules/seo/domain"
	seostore "github.com/aristorinjuang/lesstruct/modules/seo/store"
)

// isValidUsername validates that a username contains only safe characters
// to prevent SQL injection if username constants become configurable
func isValidUsername(username string) bool {
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)
	return usernameRegex.MatchString(username)
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

func newCommentRepo(driver string, db *sql.DB) commentdomain.CommentRepository {
	drivers, err := appdatabase.Drivers()
	if err != nil {
		log.Fatalf("Failed to resolve database drivers: %v", err)
	}
	drv, err := drivers.Get(driver)
	if err != nil {
		log.Fatalf("Failed to resolve database driver %q: %v", driver, err)
	}
	return commentstore.New(db, drv.Dialect())
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
	ssgHandler *handlers.SSGHandler,
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
	jwtManager *auth.JWTManager,
	spaHandler routes.AdminPanelSurface,
	staticHandler http.Handler,
	imageGenEnabled bool,
	imageReferenceEnabled bool,
	textGenEnabled bool,
	textGenHandler routes.TextGenSurface,
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
		jwtManager,
		spaHandler,
		staticHandler,
		imageGenEnabled,
		imageReferenceEnabled,
		textGenEnabled,
		textGenHandler,
		languages,
		aliasRedirectHandler,
		headlessEnabled,
		commentsEnabled,
		storageDriver,
		authorize,
		moduleRegistry,
	)

	// Create HTTP server with timeouts. ReadHeaderTimeout guards against
	// Slowloris while ReadTimeout and WriteTimeout are zero (off) by default
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

// Run boots the default edition: loads config, wires services and modules,
// runs migrations, and serves HTTP until shutdown.
func Run() {
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
	// suppresses public-surface mounting, never the module registry.
	commentsEnabled := commentsConfig.IsEnabled() && !modulesConfig.IsDisabled("comments")
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
		authmodule.WithPasswordResetService(passwordResetService),
		authmodule.WithTokenIssuer(jwtManager),
		authmodule.WithUserStore(userRepo),
		authmodule.WithFailedLoginStore(failedLoginRepo),
		authmodule.WithNotificationRecorder(modules.NewAuthNotificationRecorder(notificationRepo)),
		authmodule.WithMailer(emailService),
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
		hash := sitessr.ExtractHashFromURL(imageURL)
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
			imageGenService = mediadomain.NewGoogleGeminiService(
				cfg.AIImageGenerationAPIKey,
				cfg.AIImageGenerationModel,
				cfg.AIImageGenerationSize,
				cfg.AIImageGenerationAspectRatio,
			)
		case isGPTModel(cfg.AIImageGenerationModel):
			imageGenService = mediadomain.NewGPTImageService(
				cfg.AIImageGenerationAPIKey,
				cfg.AIImageGenerationModel,
				cfg.AIImageGenerationSize,
			)
		default:
			imageGenService = mediadomain.NewGoogleImagen4Service(
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
	// registration — so the comments module adopts the live services instead
	// of nil placeholders.
	contentRepo := newContentRepo(cfg.DBDriver, db.DB())
	commentRepo := newCommentRepo(cfg.DBDriver, db.DB())
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
		contentdomain.WithCommentsEnabled(commentsEnabled),
		contentdomain.WithRoleChecker(roleService),
		contentdomain.WithAliasDeleter(aliasService),
	)

	// Comments live in their module since Phase 3.3b: the service is built on
	// the per-driver comment repository with the content-backed AllowComments
	// gate, and the adapter below serves every surface that needs both.
	// Comments live in their module since Phase 3.3b: the service is built on
	// the per-driver comment repository with the content-backed AllowComments
	// gate, and the site-ssr reader (built below) reads comments through it.
	commentService := commentdomain.NewService(
		commentRepo,
		commentdomain.WithAllowCommentsChecker(modules.NewAllowCommentsChecker(contentService)),
	)

	// v1 module system (Phase 3.1): the real posttypes-basic module adopts the
	// config-built registry and replaces its transitional wrapper. Remaining
	// wrappers register alongside it; InitAll resolves the full graph.
	postTypesModule := posttypesbasic.New(
		posttypesbasic.WithService(postTypeService),
		posttypesbasic.WithCapabilityResolver(authorize),
		posttypesbasic.WithCommentsEnabled(commentsEnabled),
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
	commentsModule := comments.New(
		comments.WithService(commentService),
		comments.WithContentResolver(modules.NewBrowserContentResolver(contentService)),
		comments.WithAgentContentLookup(modules.NewAgentContentLookup(contentService)),
		comments.WithRoles(modules.NewCommentRoleCapabilities(roleService)),
		comments.WithAgentRoles(modules.NewAgentCommentRoleCapabilities(roleService)),
		comments.WithLogger(modules.NewLogger(utilLogger)),
	)
	candidates = append(candidates, commentsModule)
	seoModule := seo.New(
		seo.WithPages(modules.NewSEOPublishedPages(contentService)),
		seo.WithBaseURL(baseURL),
		seo.WithHeadless(headlessEnabled),
		seo.WithLogger(modules.NewLogger(utilLogger)),
	)
	candidates = append(candidates, seoModule)
	// v1 module system (Phase 3.5a): the default theme ships its layouts and
	// static assets as the ThemeProvider artifact. The module takes no
	// options — the artifact is static.
	themeModule := themedefault.New()
	candidates = append(candidates, themeModule)
	// The artifact installs immediately (not only in the ThemeProvider loop
	// after InitAll): code below — ReadThemeStyles for the textgen service —
	// needs the embedded filesystem long before the registry transition.
	sitessr.SetEmbeddedFS(themeModule.ThemeFiles())
	// v1 module system (Phase 3.5c): the site-ssr engine registers its
	// lifecycle and content seam. The assembler and handler attach in later
	// slices (3.5d/3.5e).
	siteSSRModule := sitessr.New()
	candidates = append(candidates, siteSSRModule)

	// Initialize theme for content site (templates and static files).
	// Built here — ahead of module registration — so the textgen module
	// adopts the active theme's styles instead of a nil placeholder.
	var theme *sitessr.Theme
	if cfg.ThemeDir != "" {
		theme = &sitessr.Theme{Dir: cfg.ThemeDir}
	}

	// Read the active theme's minified base.css + style.css so AI-generated
	// HTML/CSS reuses the site's design tokens and component classes instead
	// of inventing off-brand styles.
	themeBaseCSS, themeStyleCSS := sitessr.ReadThemeStyles(theme)

	// Initialize AI text generation service
	var textGenService textgendomain.TextGenerationService
	if cfg.IsTextGenerationEnabled() {
		textGenService = textgendomain.NewOpenAITextService(
			cfg.AITextGenerationAPIKey,
			cfg.AITextGenerationBaseURL,
			cfg.AITextGenerationModel,
			themeBaseCSS,
			themeStyleCSS,
		)
	}
	// v1 module system (Phase 3.6b): the real textgen module adopts the
	// OpenAI-compatible service and the media library for context injection.
	// Core mounts the handler explicitly (see Module.Handler): the
	// textGenEnabled feature flag gates mounting.
	textgenModule := textgenmodule.New(
		textgenmodule.WithService(textGenService),
		textgenmodule.WithMediaLister(mediaService),
		textgenmodule.WithLogger(modules.NewLogger(utilLogger)),
	)
	candidates = append(candidates, textgenModule)
	textGenHandler := textgenModule.Handler()

	// Initialize WordPress importer (admin-only content migration)
	wordpressDownloader := wordpress.NewMediaDownloader(
		&http.Client{Timeout: 30 * time.Second},
		mediaService,
	)
	wordpressResolver := wordpress.NewUserResolver(
		modules.NewWordPressUserProvisioner(userRepo),
		modules.NewLogger(utilLogger),
	)
	wordpressImporter := wordpress.NewImporter(
		contentService,
		wordpressDownloader,
		wordpressResolver,
		postTypeService,
		primaryLanguage,
		utilLogger,
	)

	// Initialize Hugo importer (admin-only content migration)
	hugoImporter := hugo.NewImporter(
		contentService,
		aliasService,
		contentService,
		mediaService,
		wordpressDownloader,
		primaryLanguage,
		utilLogger,
	)

	// v1 module system (Phase 3.7d): the real importers module adopts both
	// importers and builds the handlers once, so the browser and agent
	// realms share the same instances and job stores. Core mounts the
	// handlers explicitly (see Module.WordPressHandler/HugoHandler): the
	// surface spans the browser admin group and the agent realm, which a
	// single RegisterRoutes call cannot express.
	importersModule := importers.New(
		importers.WithWordPressImporter(wordpressImporter),
		importers.WithHugoImporter(hugoImporter),
		importers.WithMaxImportBytes(cfg.ImportMaxSize()),
		importers.WithWordPressTimeout(cfg.WordPressImportTimeout),
		importers.WithHugoTimeout(cfg.HugoImportTimeout),
		importers.WithLogger(modules.NewLogger(utilLogger)),
	)
	candidates = append(candidates, importersModule)
	wordPressHandler := importersModule.WordPressHandler()
	hugoHandler := importersModule.HugoHandler()

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

	// Initialize profile picture service (Story 12.8)
	profilePictureService := profilepicture.NewService(
		modules.NewProfilePictureUserRepo(userRepo),
		profilePictureStorage,
		profilepicture.NewProcessor(),
	)

	// Initialize user management service and handler
	userManagementOpts := []user.UserManagementOption{user.WithManagementRoleService(roleService)}
	if registrationConfig.AdminApproval {
		userManagementOpts = append(userManagementOpts, user.WithApprovalEmailRequired())
	}
	userManagementService := user.NewUserManagementService(userRepo, blockedEmailRepo, credentialPolicy, commentsEnabled, userManagementOpts...)
	adminCreateUserService := user.NewAdminCreateUserService(userRepo, blockedEmailRepo, credentialPolicy, commentsEnabled, user.WithRoleService(roleService))
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
	for _, provider := range kernel.Capabilities[kernel.ThemeProvider](moduleRegistry) {
		sitessr.SetEmbeddedFS(provider.ThemeFiles())
		break
	}

	// Module migrations run after the core sequence: each Migrator versions
	// independently in its own schema_migrations_<module> table.
	if err := appdatabase.MigrateModules(db.DB(), cfg.DBDriver, moduleRegistry); err != nil {
		log.Fatalf("Failed to run module migrations: %v", err)
	}

	// Agent (Bearer) content handler — the streamlined API surface for
	// programmatic publishing (Story 2.1). Reuses the same contentService so custom
	// field validation and plugin hooks fire identically to the admin path.
	agentContentHandler := agent.NewContentHandler(contentService, postTypeService, utilLogger)

	// Public content handler — resolves featured images (thumbnail variants) for the
	// public API content-items list via the media-join resolver defined above
	// alongside the media stack.
	contentHandler := handlers.NewContentHandler(contentService, utilLogger, baseURL, profilePictureStorage.GetURL, featuredImageResolver)

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
	contentHandler = contentHandler.WithPublicFieldRegistry(publicFieldRegistry)

	// Agent (Bearer) comment endpoints come from the comments module, which
	// reuses the adopted comment service.
	agentCommentHandler := commentsModule.AgentHandler()

	// Initialize dashboard service and handler (Story 2.8)
	dashboardRepo := newDashboardRepo(cfg.DBDriver, db.DB())
	dashboardService := dashboarddomain.NewService(dashboardRepo)
	dashboardHandler := handlers.NewDashboardHandler(dashboardService, postTypeService, utilLogger)

	notificationHandler := handlers.NewNotificationHandler(utilLogger, notificationRepo)

	authMiddleware := middleware.NewAuthMiddleware(jwtManager)
	adminMiddleware := middleware.NewAdminMiddleware(authMiddleware)

	// SEO discovery surfaces come from the seo module, which projects
	// published content through the adapter above.
	seoHandler := seoModule.Handler()

	// Browser comment endpoints come from the comments module.
	commentHandler := commentsModule.BrowserHandler()

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

	// Initialize no-cookie enforcement middleware (Story 6.1)
	noCookieMiddleware := middleware.NewNoCookieMiddleware(utilLogger)

	// Initialize CSRF middleware (Story 5.2)
	csrfMiddleware := middleware.NewCSRFMiddleware(utilLogger, cfg.ParseCORSOrigins(), jwtManager)

	// Initialize rate limit middleware (Story 5.4)
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(
		cfg.RateLimitEnabled,
		cfg.RateLimitAuthPerMinute,
		cfg.RateLimitAPIPerMinute,
		cfg.RateLimitPublicPerMinute,
	)

	// (#13) Reduce logging exposure
	utilLogger.Info("Admin credentials configured. Please change password after first login.")

	// Load UI translation catalog from translations/ directory
	uiCatalog, err := i18n.NewCatalog(i18n.Embedded(), languages)
	if err != nil {
		log.Fatalf("Failed to load UI translations: %v", err)
	}

	// Initialize content site templates and renderer
	contentTemplates, err := sitessr.NewTemplates(theme, uiCatalog, postTypeSlugs...)
	if err != nil {
		log.Fatalf("Failed to load content templates: %v", err)
	}
	tiptapRenderer := tiptap.NewRenderer(func(src string) []tiptap.ImageVariant {
		hash := sitessr.ExtractHashFromURL(src)
		if hash == "" {
			return nil
		}
		m, err := mediaRepo.FindByHashPrefix(context.Background(), hash)
		if err != nil {
			log.Printf("WARNING: FindByHashPrefix failed for hash %q: %v", hash, err)
			return nil
		}
		if m == nil {
			return nil
		}
		variants := m.Variants
		if len(variants) == 0 {
			return nil
		}
		result := make([]tiptap.ImageVariant, 0, len(variants))
		for _, v := range variants {
			result = append(result, tiptap.ImageVariant{
				URL:   v.URL,
				Width: v.Width,
			})
		}
		slices.SortFunc(result, func(a, b tiptap.ImageVariant) int {
			return cmp.Compare(a.Width, b.Width)
		})
		if m.URL != "" && m.Width > 0 && m.Width > result[len(result)-1].Width {
			result = append(result, tiptap.ImageVariant{
				URL:   m.URL,
				Width: m.Width,
			})
		}
		return result
	})
	userProvider := sitessr.NewUserRepoAdapter(func(ctx context.Context, username string) (*sitessr.UserBasicInfo, error) {
		user, err := userRepo.GetUserByUsername(ctx, username)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, nil
		}
		var avatarURL string
		if user.ProfilePicture != "" {
			avatarURL = profilePictureStorage.GetURL(user.ProfilePicture)
		}
		return &sitessr.UserBasicInfo{
			Name:           user.Name,
			Username:       user.Username,
			CustomFields:   user.CustomFields,
			ProfilePicture: avatarURL,
		}, nil
	})

	homepageSections, err := config.LoadHomepageSections(cfg)
	if err != nil {
		log.Printf("Warning: Failed to load homepage sections config, continuing without sections: %v", err)
		homepageSections = nil
	}
	ssrSections := make([]sitessr.HomepageSection, 0, len(homepageSections))
	for _, hs := range homepageSections {
		ssrSections = append(ssrSections, sitessr.HomepageSection{
			PostType: hs.PostType,
			Limit:    hs.Limit,
			Offset:   hs.Offset,
			Title:    hs.Title,
		})
	}

	siteConfig, err := config.LoadSiteConfig(cfg)
	if err != nil {
		log.Printf("Warning: Failed to load site config, continuing with defaults: %v", err)
		siteConfig = config.SiteConfig{}
	}
	ssrSiteConfig := sitessr.SiteConfig{Name: siteConfig.Name, Logo: siteConfig.Logo}

	cspConfig, err := config.LoadCSPConfig(cfg)
	if err != nil {
		log.Printf("Warning: Failed to load CSP config, continuing with defaults: %v", err)
		cspConfig = config.CSPConfig{}
	}
	cspHeaderName, cspHeaderValue := cspConfig.Build()
	securityHeadersMiddleware := middleware.NewSecurityHeadersMiddleware(cspHeaderName, cspHeaderValue, cspConfig.XFrameOptions())

	ssrReader := modules.NewSiteSSRContentReader(contentService, commentService)
	contentPageHandler := sitessr.NewContentPageHandler(
		ssrReader,
		postTypeService,
		postTypeService,
		userProvider,
		contentTemplates,
		tiptapRenderer,
		mediaRepo,
		languages,
		ssrSections,
		ssrSiteConfig,
		cfg.PostPerPage,
		commentsEnabled,
		registrationEnabled,
	)
	contentPageHandler = contentPageHandler.WithPublicFieldRegistry(publicFieldRegistry)
	contentPageHandler = contentPageHandler.WithBaseURL(baseURL)

	// The sanitizer's iframe allowlist mirrors the CSP frame-src directive
	// (defaults + operator appends) so embeds the CSP allows to be framed are
	// also allowed through the HTML sanitizer on every path.
	iframeHosts := cspConfig.IFrameHosts()
	contentPageHandler = contentPageHandler.WithIFrameHosts(iframeHosts...)
	contentHandler = contentHandler.WithIFrameHosts(iframeHosts...)
	agentContentHandler = agentContentHandler.WithIFrameHosts(iframeHosts...)

	// Static file handler for content site (uses theme from above)
	staticHandler := sitessr.StaticFiles(theme)

	// Alias redirect handler wraps the content site catch-all to serve 301s.
	// Theme root files (theme's root/ directory, e.g. webpushr-sw.js) sit
	// between the alias lookup and the content renderer so they are served at
	// the site root on the dynamic server too.
	aliasRedirectHandler := handlers.NewAliasRedirectHandler(
		aliasService,
		contentService,
		sitessr.RootFilesHandler(theme, contentPageHandler),
	)

	// Initialize content exporter (Phase 2) — exports all content as Hugo-compatible
	// source files with bundled media.
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

	// Initialize static site generator (Phase 3) — generates a fully rendered
	// static site with AMP variants as a tar.gz archive. Aliases are attached
	// so legacy URLs ship as meta-refresh redirect pages.
	ssgGenerator := ssg.NewGenerator(
		contentPageHandler.Assembler(),
		contentTemplates,
		ssrReader,
		filepath.Join("data", "uploads", "media"),
		theme,
		cfg.SiteURL,
	).WithAliases(aliasService)
	ssgHandler := handlers.NewSSGHandler(ssgGenerator, utilLogger)

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
		seoHandler,
		commentHandler,
		profilePictureHandler,
		wordPressHandler,
		apiKeyHandler,
		hugoHandler,
		exportHandler,
		ssgHandler,
		apiKeyAuthMiddleware,
		agentContentHandler,
		mediaModule.AgentHandler(),
		agentCommentHandler,
		adminMiddleware,
		corsMiddleware,
		noCookieMiddleware,
		csrfMiddleware,
		rateLimitMiddleware,
		securityHeadersMiddleware,
		jwtManager,
		spaHandler,
		staticHandler,
		cfg.IsImageGenerationEnabled(),
		imageReferenceEnabled,
		cfg.IsTextGenerationEnabled(),
		textGenHandler,
		languages,
		aliasRedirectHandler,
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
