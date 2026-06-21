package main

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
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/contentpage"
	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/api/handlers/agent"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/api/routes"
	"github.com/aristorinjuang/lesstruct/internal/api/static"
	"github.com/aristorinjuang/lesstruct/internal/api/template"
	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/aristorinjuang/lesstruct/internal/constants"
	"github.com/aristorinjuang/lesstruct/internal/content/tiptap"
	"github.com/aristorinjuang/lesstruct/internal/content/wordpress"
	appdatabase "github.com/aristorinjuang/lesstruct/internal/database"
	apikeydomain "github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	authdomain "github.com/aristorinjuang/lesstruct/internal/domain/auth"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	dashboarddomain "github.com/aristorinjuang/lesstruct/internal/domain/dashboard"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
	"github.com/aristorinjuang/lesstruct/internal/domain/profilepicture"
	seodomain "github.com/aristorinjuang/lesstruct/internal/domain/seo"
	"github.com/aristorinjuang/lesstruct/internal/domain/textgen"
	"github.com/aristorinjuang/lesstruct/internal/domain/user"
	"github.com/aristorinjuang/lesstruct/internal/email"
	"github.com/aristorinjuang/lesstruct/internal/i18n"
	pluginadapter "github.com/aristorinjuang/lesstruct/internal/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/bootstrap"
	"github.com/aristorinjuang/lesstruct/internal/plugin/hostfunctions"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	mysqlrepository "github.com/aristorinjuang/lesstruct/internal/repository/mysql"
	postgresqlrepository "github.com/aristorinjuang/lesstruct/internal/repository/postgresql"
	sqliterepository "github.com/aristorinjuang/lesstruct/internal/repository/sqlite"
	"github.com/aristorinjuang/lesstruct/internal/util"
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

func newFailedLoginRepo(driver string, db *sql.DB) repository.FailedLoginAttemptRepo {
	if isPostgres(driver) {
		return postgresqlrepository.NewFailedLoginAttemptRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewFailedLoginAttemptRepository(db)
	}
	return repository.NewFailedLoginAttemptRepository(db)
}

func newVerificationTokenRepo(driver string, db *sql.DB) repository.VerificationTokenRepo {
	if isPostgres(driver) {
		return postgresqlrepository.NewVerificationTokenRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewVerificationTokenRepository(db)
	}
	return repository.NewVerificationTokenRepository(db)
}

func newPasswordResetTokenRepo(driver string, db *sql.DB) repository.PasswordResetTokenRepo {
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

func newCommentRepo(driver string, db *sql.DB) contentdomain.CommentRepository {
	if isPostgres(driver) {
		return postgresqlrepository.NewCommentRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewCommentRepository(db)
	}
	return sqliterepository.NewCommentRepository(db)
}

func newMediaRepo(driver string, db *sql.DB) mediadomain.Repository {
	if isPostgres(driver) {
		return postgresqlrepository.NewMediaRepository(db)
	}
	if isMySQL(driver) {
		return mysqlrepository.NewMediaRepository(db)
	}
	return sqliterepository.NewMediaRepository(db)
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
	authHandler *handlers.AuthHandler,
	firstLoginHandler *handlers.FirstLoginHandler,
	notificationHandler *handlers.NotificationHandler,
	userManagementHandler *handlers.UserManagementHandler,
	profileHandler *handlers.ProfileHandler,
	contentHandler *handlers.ContentHandler,
	mediaHandler *handlers.MediaHandler,
	postTypeHandler *handlers.PostTypeHandler,
	dashboardHandler *handlers.DashboardHandler,
	seoHandler *handlers.SEOHandler,
	commentHandler *handlers.CommentHandler,
	profilePictureHandler *handlers.ProfilePictureHandler,
	wordPressHandler *handlers.WordPressHandler,
	apiKeyHandler *handlers.APIKeyHandler,
	apiKeyAuthMiddleware *middleware.APIKeyAuthMiddleware,
	agentContentHandler *agent.ContentHandler,
	agentMediaHandler *agent.MediaHandler,
	adminMiddleware *middleware.AdminMiddleware,
	corsMiddleware *middleware.CORSMiddleware,
	noCookieMiddleware *middleware.NoCookieMiddleware,
	csrfMiddleware *middleware.CSRFMiddleware,
	rateLimitMiddleware *middleware.RateLimitMiddleware,
	jwtManager *auth.JWTManager,
	staticServer *static.StaticServer,
	staticHandler http.Handler,
	imageGenEnabled bool,
	textGenEnabled bool,
	textGenHandler *handlers.TextGenHandler,
	languages []string,
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
		postTypeHandler,
		dashboardHandler,
		seoHandler,
		commentHandler,
		profilePictureHandler,
		wordPressHandler,
		apiKeyHandler,
		apiKeyAuthMiddleware,
		agentContentHandler,
		agentMediaHandler,
		adminMiddleware,
		corsMiddleware,
		noCookieMiddleware,
		csrfMiddleware,
		rateLimitMiddleware,
		jwtManager,
		staticServer,
		staticHandler,
		imageGenEnabled,
		textGenEnabled,
		textGenHandler,
		languages,
	)

	// Create HTTP server with timeouts
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 3 * time.Minute, // at least two minutes for waiting the image generation
		IdleTimeout:  60 * time.Second,
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

func main() {
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

	authService := authdomain.NewAuthService(defaultPasswordHash)
	firstLoginService := authdomain.NewFirstLoginService(defaultPasswordHash)
	registrationService := authdomain.NewRegistrationService(userRepo)
	verificationService := authdomain.NewVerificationService(userRepo, tokenRepo, 24)
	loginService := authdomain.NewLoginService(userRepo, failedLoginRepo, utilLogger)

	passwordResetTokenRepo := newPasswordResetTokenRepo(cfg.DBDriver, db.DB())
	passwordResetService := authdomain.NewPasswordResetService(userRepo, passwordResetTokenRepo, 1)

	notificationRepo := repository.NewInMemoryNotificationRepository()

	// Initialize email service
	smtpConfig := email.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUser,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
	}
	baseURL := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
	emailService := email.NewService(smtpConfig, logger, cfg.SiteURL)

	blockedEmailRepo := newBlockedEmailRepo(cfg.DBDriver, db.DB())
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

	// Initialize thumbnail service
	thumbnailService, err := config.LoadThumbnails(cfg)
	if err != nil {
		log.Fatalf("Failed to load thumbnail config: %v", err)
	}

	// Initialize profile service (Story 12.6: needs postTypeService for system field stripping)
	profileService := user.NewProfileService(userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService, postTypeService)

	// Initialize user deletion repository and service (Story 1.8)
	userDeletionRepo := newUserDeletionRepo(cfg.DBDriver, db.DB())
	accountDeletionService := user.NewAccountDeletionService(userRepo, userDeletionRepo, emailService, utilLogger)

	// Initialize profile picture storage (needed by profile handler for URL building)
	profilePicturesDir := filepath.Join("data", "uploads", "profile_pictures")
	if err := os.MkdirAll(profilePicturesDir, 0755); err != nil {
		log.Fatalf("Failed to create profile pictures directory: %v", err)
	}
	profilePictureStorage := profilepicture.NewLocalStorage(profilePicturesDir, cfg.Host, cfg.Port)

	// Initialize profile handler
	profileHandler := handlers.NewProfileHandler(profileService, accountDeletionService, jwtManager, utilLogger, postTypeService, profilePictureStorage.GetURL)

	postTypeHandler := handlers.NewPostTypeHandler(postTypeService, utilLogger)

	// Initialize content service and handler (Story 2.1)
	contentRepo := newContentRepo(cfg.DBDriver, db.DB())
	commentRepo := newCommentRepo(cfg.DBDriver, db.DB())
	seoService := seodomain.NewService(baseURL, "Lesstruct")
	postTypeAdapter := contentdomain.NewPostTypeAdapter(postTypeService)
	contentService := contentdomain.NewServiceWithHooks(
		contentRepo,
		commentRepo,
		seoService,
		postTypeAdapter,
		pluginadapter.NewHookExecutorAdapter(pluginSys.Registry()),
	)
	contentHandler := handlers.NewContentHandler(contentService, utilLogger, baseURL)

	// Initialize agent (Bearer) content handler — the streamlined API surface for
	// programmatic publishing (Story 2.1). Reuses the same contentService so custom
	// field validation and plugin hooks fire identically to the admin path.
	agentContentHandler := agent.NewContentHandler(contentService, postTypeService, utilLogger)

	// Initialize media service and handler (Story 2.3)
	uploadsDir := filepath.Join("data", "uploads", "media")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		log.Fatalf("Failed to create uploads directory: %v", err)
	}
	mediaRepo := newMediaRepo(cfg.DBDriver, db.DB())
	mediaStorage := mediadomain.NewLocalStorage(uploadsDir, cfg.Host, cfg.Port)
	mediaService := mediadomain.NewService(mediaRepo, mediaStorage, thumbnailService)
	// Initialize agent (Bearer) media handler — the streamlined API surface for
	// programmatic media upload/retrieval (Story 2.3). Reuses the same mediaService so
	// hashing, dedup, WebP conversion, and thumbnail variants run identically to admin.
	agentMediaHandler := agent.NewMediaHandler(mediaService, utilLogger)
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

	mediaHandler := handlers.NewMediaHandler(mediaService, imageGenService, utilLogger)

	// Initialize AI text generation service
	var textGenService textgen.TextGenerationService
	if cfg.IsTextGenerationEnabled() {
		textGenService = textgen.NewOpenAITextService(
			cfg.AITextGenerationAPIKey,
			cfg.AITextGenerationBaseURL,
			cfg.AITextGenerationModel,
		)
	}
	textGenHandler := handlers.NewTextGenHandler(textGenService, utilLogger)

	// Initialize profile picture service and handler (Story 12.8)
	profilePictureService := profilepicture.NewService(
		profilepicture.NewRepoAdapter(userRepo),
		profilePictureStorage,
		profilepicture.NewProcessor(),
	)
	profilePictureHandler := handlers.NewProfilePictureHandler(profilePictureService, userRepo, utilLogger)

	// Initialize dashboard service and handler (Story 2.8)
	dashboardRepo := newDashboardRepo(cfg.DBDriver, db.DB())
	dashboardService := dashboarddomain.NewService(dashboardRepo)
	dashboardHandler := handlers.NewDashboardHandler(dashboardService, utilLogger)

	authHandler := handlers.NewAuthHandler(
		authService,
		jwtManager,
		utilLogger,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		passwordResetService,
		userRepo,
		failedLoginRepo,
		notificationRepo,
		emailService,
		blockedEmailRepo,
	)
	firstLoginHandler := handlers.NewFirstLoginHandler(firstLoginService, userRepo, utilLogger)
	notificationHandler := handlers.NewNotificationHandler(utilLogger, notificationRepo)

	// Initialize user management service and handler
	userManagementService := user.NewUserManagementService(userRepo, blockedEmailRepo)
	adminCreateUserService := user.NewAdminCreateUserService(userRepo, blockedEmailRepo)
	userManagementHandler := handlers.NewUserManagementHandler(userManagementService, adminCreateUserService, userRepo, softDeleteRepo, jwtManager, emailService, utilLogger, profilePictureStorage.GetURL)
	authMiddleware := middleware.NewAuthMiddleware(jwtManager)
	adminMiddleware := middleware.NewAdminMiddleware(authMiddleware)

	// Initialize SEO handler (Story 4.2)
	seoHandler := handlers.NewSEOHandler(contentService, baseURL, utilLogger)

	// Initialize comment handler (Story 4.7)
	commentHandler := handlers.NewCommentHandler(contentService)

	// Initialize WordPress import handler (admin-only content migration)
	wordpressDownloader := wordpress.NewMediaDownloader(
		&http.Client{Timeout: 30 * time.Second},
		mediaService,
	)
	wordpressImporter := wordpress.NewImporter(contentService, wordpressDownloader, utilLogger)
	wordPressHandler := handlers.NewWordPressHandler(wordpressImporter, utilLogger)

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

	// Initialize languages configuration (i18n)
	languages, err := config.LoadLanguages(cfg)
	if err != nil {
		log.Printf("Warning: Failed to load languages config, using defaults: %v", err)
		languages = []string{"en"}
	}

	// Load UI translation catalog from translations/ directory
	uiCatalog, err := i18n.NewCatalog(i18n.Embedded(), languages)
	if err != nil {
		log.Fatalf("Failed to load UI translations: %v", err)
	}

	// Initialize theme for content site (templates and static files)
	var theme *template.Theme
	if cfg.ThemeDir != "" {
		theme = &template.Theme{Dir: cfg.ThemeDir}
	}

	// Initialize content site templates and renderer
	contentTemplates, err := template.NewTemplates(theme, uiCatalog)
	if err != nil {
		log.Fatalf("Failed to load content templates: %v", err)
	}
	tiptapRenderer := tiptap.NewRenderer(func(src string) []tiptap.ImageVariant {
		hash := contentpage.ExtractHashFromURL(src)
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
		sort.Slice(result, func(i, j int) bool {
			return result[i].Width < result[j].Width
		})
		return result
	})
	userProvider := contentpage.NewUserRepoAdapter(func(ctx context.Context, username string) (*contentpage.UserBasicInfo, error) {
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
		return &contentpage.UserBasicInfo{
			Name:           user.Name,
			Username:       user.Username,
			CustomFields:   user.CustomFields,
			ProfilePicture: avatarURL,
		}, nil
	})

	contentPageHandler := contentpage.NewContentPageHandler(
		contentService,
		postTypeService,
		postTypeService,
		userProvider,
		contentTemplates,
		tiptapRenderer,
		mediaRepo,
		languages,
	)

	// Initialize static file server for admin panel and content site
	staticServer := static.NewStaticServer(
		cfg.DevMode,
		cfg.AdminDevURL,
		contentPageHandler,
	)

	// Static file handler for content site (uses theme from above)
	staticHandler := template.StaticFiles(theme)

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
		mediaHandler,
		postTypeHandler,
		dashboardHandler,
		seoHandler,
		commentHandler,
		profilePictureHandler,
		wordPressHandler,
		apiKeyHandler,
		apiKeyAuthMiddleware,
		agentContentHandler,
		agentMediaHandler,
		adminMiddleware,
		corsMiddleware,
		noCookieMiddleware,
		csrfMiddleware,
		rateLimitMiddleware,
		jwtManager,
		staticServer,
		staticHandler,
		cfg.IsImageGenerationEnabled(),
		cfg.IsTextGenerationEnabled(),
		textGenHandler,
		languages,
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

	utilLogger.Info("Server exited")
}
