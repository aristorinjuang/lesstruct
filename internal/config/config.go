package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves an environment variable as an integer or returns a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvDuration retrieves an environment variable as a duration (e.g. "2h", "30m")
// or returns a default value. Invalid values fall back to the default.
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

// getEnvBool retrieves an environment variable as a boolean or returns a default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

// isValidOrigin checks if an origin string has a valid http/https scheme and a non-empty host
func isValidOrigin(origin string) bool {
	if strings.HasPrefix(origin, "https://") {
		return len(origin) > len("https://")
	}
	if strings.HasPrefix(origin, "http://") {
		return len(origin) > len("http://")
	}
	return false
}

// Config holds all application configuration
type Config struct {
	Host           string
	Port           int
	DBPath         string
	DBDriver       string
	DBDSN          string
	DBPoolMaxConns int

	// StorageDriver selects the file storage backend: "local" (default) or "s3".
	// The "s3" driver works against AWS S3 AND MinIO (both speak the S3 API).
	// Read via STORAGE_DRIVER.
	StorageDriver string

	// StorageS3Endpoint is the S3-compatible endpoint URL (e.g. "http://minio:9000").
	// Leave empty to use the default AWS endpoints. Read via STORAGE_S3_ENDPOINT.
	StorageS3Endpoint string

	// StorageS3Region is the AWS region or MinIO region (MinIO uses "us-east-1" by
	// default). Read via STORAGE_S3_REGION.
	StorageS3Region string

	// StorageS3Bucket is the bucket media and profile pictures are stored in.
	// Read via STORAGE_S3_BUCKET.
	StorageS3Bucket string

	// StorageS3AccessKeyID is the access key for the S3-compatible backend.
	// Read via STORAGE_S3_ACCESS_KEY_ID.
	StorageS3AccessKeyID string

	// StorageS3SecretAccessKey is the secret key for the S3-compatible backend.
	// Read via STORAGE_S3_SECRET_ACCESS_KEY.
	StorageS3SecretAccessKey string

	// StorageS3UsePathStyle forces path-style addressing ("bucket.example.com/key"
	// vs "example.com/bucket/key"). Required for most MinIO deployments.
	// Read via STORAGE_S3_USE_PATH_STYLE.
	StorageS3UsePathStyle bool

	// StorageS3PublicBaseURL is the base URL served to clients (the bucket's public
	// URL or a CDN/CloudFront distribution, e.g. "https://cdn.example.com").
	// GetURL returns this prefix + object key. Read via STORAGE_S3_PUBLIC_BASE_URL.
	StorageS3PublicBaseURL string

	JWTSecret          string
	LogLevel           string
	SMTPHost           string
	SMTPPort           int
	SMTPUser           string
	SMTPPassword       string
	SMTPFrom           string
	ConfigDir          string
	ConfigFile         string
	CORSAllowedOrigins string
	SiteURL            string
	DevMode            bool
	AdminDevURL        string
	ThemeDir           string
	PostPerPage        int

	// ServerReadHeaderTimeout is the maximum duration to read request headers
	// (Slowloris protection). Read via SERVER_READ_HEADER_TIMEOUT.
	ServerReadHeaderTimeout time.Duration

	// ServerReadTimeout is the maximum duration to read the entire request
	// including the body. A zero value means no timeout — body size is capped
	// per-handler via MaxBytesReader / maxBodySizeMiddleware. Read via
	// SERVER_READ_TIMEOUT.
	ServerReadTimeout time.Duration

	// ServerWriteTimeout is the maximum duration to write the response after
	// the request headers have been read. A zero value means no timeout. Read
	// via SERVER_WRITE_TIMEOUT.
	ServerWriteTimeout time.Duration

	RateLimitEnabled         bool
	RateLimitAuthPerMinute   int
	RateLimitAPIPerMinute    int
	RateLimitPublicPerMinute int

	AIImageGenerationAPIKey      string
	AIImageGenerationModel       string
	AIImageGenerationSize        string
	AIImageGenerationAspectRatio string

	AITextGenerationAPIKey  string
	AITextGenerationBaseURL string
	AITextGenerationModel   string

	APIKeyPepper string

	// ImportMaxSizeMB is the ceiling (in megabytes) for any importer upload
	// (WordPress WXR now; future importers reuse the same cap). Read via
	// IMPORT_MAX_SIZE_MB. Use ImportMaxSize() for the byte value.
	ImportMaxSizeMB int

	// WordPressImportTimeout is the maximum duration a WordPress import job may
	// run after the HTTP response has been sent (the job runs in a background
	// goroutine). Read via WORDPRESS_IMPORT_TIMEOUT.
	WordPressImportTimeout time.Duration

	// HugoImportTimeout is the maximum duration a Hugo import job may run after
	// the HTTP response has been sent (the job runs in a background goroutine).
	// Read via HUGO_IMPORT_TIMEOUT.
	HugoImportTimeout time.Duration
}

// Load loads configuration from environment variables
// It tries to load .env file first, then reads from actual environment
func Load() (*Config, error) {
	// Try to load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{
		Host:                     getEnv("HOST", "0.0.0.0"),
		Port:                     getEnvInt("PORT", 8080),
		DBPath:                   getEnv("DB_PATH", "data/lesstruct.db"),
		DBDriver:                 getEnv("DB_DRIVER", "sqlite"),
		DBDSN:                    getEnv("DB_DSN", ""),
		DBPoolMaxConns:           getEnvInt("DB_POOL_MAX_CONNS", 20),
		StorageDriver:            getEnv("STORAGE_DRIVER", "local"),
		StorageS3Endpoint:        getEnv("STORAGE_S3_ENDPOINT", ""),
		StorageS3Region:          getEnv("STORAGE_S3_REGION", "us-east-1"),
		StorageS3Bucket:          getEnv("STORAGE_S3_BUCKET", ""),
		StorageS3AccessKeyID:     getEnv("STORAGE_S3_ACCESS_KEY_ID", ""),
		StorageS3SecretAccessKey: getEnv("STORAGE_S3_SECRET_ACCESS_KEY", ""),
		StorageS3UsePathStyle:    getEnvBool("STORAGE_S3_USE_PATH_STYLE", false),
		StorageS3PublicBaseURL:   getEnv("STORAGE_S3_PUBLIC_BASE_URL", ""),
		JWTSecret:                getEnv("JWT_SECRET", ""),
		LogLevel:                 getEnv("LOG_LEVEL", "info"),
		SMTPHost:                 getEnv("SMTP_HOST", ""),
		SMTPPort:                 getEnvInt("SMTP_PORT", 587),
		SMTPUser:                 getEnv("SMTP_USER", ""),
		SMTPPassword:             getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:                 getEnv("SMTP_FROM", ""),
		ConfigDir:                getEnv("CONFIG_DIR", "."),
		ConfigFile:               getEnv("CONFIG_FILE", "config.toml"),
		CORSAllowedOrigins:       getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173"),
		SiteURL:                  getEnv("SITE_URL", "http://localhost:8080"),
		DevMode:                  getEnvBool("DEV_MODE", false),
		AdminDevURL:              getEnv("ADMIN_DEV_URL", "http://localhost:5173"),
		ThemeDir:                 getEnv("THEME_DIR", ""),
		PostPerPage:              getEnvInt("POSTS_PER_PAGE", 50),

		// ServerReadHeaderTimeout defaults to 15s for Slowloris protection.
		ServerReadHeaderTimeout: getEnvDuration("SERVER_READ_HEADER_TIMEOUT", 15*time.Second),
		// ReadTimeout defaults to zero (off) so large import uploads are not
		// capped by a global timeout — per-handler MaxBytesReader limits body
		// size, and ReadHeaderTimeout provides Slowloris protection.
		ServerReadTimeout: getEnvDuration("SERVER_READ_TIMEOUT", 0),
		// WriteTimeout defaults to zero (off). Per-handler context deadlines
		// (e.g. WORDPRESS_IMPORT_TIMEOUT) bound long-running operations.
		ServerWriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 0),

		RateLimitEnabled:         getEnvBool("RATE_LIMIT_ENABLED", true),
		RateLimitAuthPerMinute:   getEnvInt("RATE_LIMIT_AUTH_PER_MINUTE", 5),
		RateLimitAPIPerMinute:    getEnvInt("RATE_LIMIT_API_PER_MINUTE", 100),
		RateLimitPublicPerMinute: getEnvInt("RATE_LIMIT_PUBLIC_PER_MINUTE", 60),

		AIImageGenerationAPIKey:      getEnv("AI_IMAGE_GENERATION_API_KEY", ""),
		AIImageGenerationModel:       getEnv("AI_IMAGE_GENERATION_MODEL", "imagen-4.0-fast-"),
		AIImageGenerationSize:        getEnv("AI_IMAGE_GENERATION_SIZE", ""),
		AIImageGenerationAspectRatio: getEnv("AI_IMAGE_GENERATION_ASPECT_RATIO", ""),

		AITextGenerationAPIKey:  getEnv("AI_TEXT_GENERATION_API_KEY", ""),
		AITextGenerationBaseURL: getEnv("AI_TEXT_GENERATION_BASE_URL", ""),
		AITextGenerationModel:   getEnv("AI_TEXT_GENERATION_MODEL", "gpt-5-mini"),

		APIKeyPepper: getEnv("API_KEY_PEPPER", ""),

		ImportMaxSizeMB:        getEnvInt("IMPORT_MAX_SIZE_MB", 100),
		WordPressImportTimeout: getEnvDuration("WORDPRESS_IMPORT_TIMEOUT", 2*time.Hour),
		HugoImportTimeout:      getEnvDuration("HUGO_IMPORT_TIMEOUT", 10*time.Minute),
	}

	// Validate JWT secret
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters long for security")
	}

	// Validate database driver
	switch cfg.DBDriver {
	case "sqlite":
		// sqlite is the default — no additional validation needed
	case "postgres":
		if cfg.DBDSN == "" {
			return nil, fmt.Errorf("DB_DSN is required when DB_DRIVER=postgres")
		}
		if cfg.DBPoolMaxConns < 1 {
			return nil, fmt.Errorf("DB_POOL_MAX_CONNS must be at least 1, got %d", cfg.DBPoolMaxConns)
		}
	case "mysql":
		if cfg.DBDSN == "" {
			return nil, fmt.Errorf("DB_DSN is required when DB_DRIVER=mysql")
		}
		if !strings.Contains(cfg.DBDSN, "parseTime=true") {
			return nil, fmt.Errorf("DB_DSN must contain parseTime=true for MySQL driver")
		}
		if !strings.Contains(cfg.DBDSN, "multiStatements=true") {
			return nil, fmt.Errorf("DB_DSN must contain multiStatements=true for MySQL driver")
		}
		if cfg.DBPoolMaxConns < 1 {
			return nil, fmt.Errorf("DB_POOL_MAX_CONNS must be at least 1, got %d", cfg.DBPoolMaxConns)
		}
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q: must be 'sqlite', 'postgres', or 'mysql'", cfg.DBDriver)
	}

	// Validate storage driver
	switch cfg.StorageDriver {
	case "local":
		// local is the default — no additional validation needed
	case "s3":
		if cfg.StorageS3Region == "" {
			return nil, fmt.Errorf("STORAGE_S3_REGION is required when STORAGE_DRIVER=s3")
		}
		if cfg.StorageS3Bucket == "" {
			return nil, fmt.Errorf("STORAGE_S3_BUCKET is required when STORAGE_DRIVER=s3")
		}
		if cfg.StorageS3AccessKeyID == "" {
			return nil, fmt.Errorf("STORAGE_S3_ACCESS_KEY_ID is required when STORAGE_DRIVER=s3")
		}
		if cfg.StorageS3SecretAccessKey == "" {
			return nil, fmt.Errorf("STORAGE_S3_SECRET_ACCESS_KEY is required when STORAGE_DRIVER=s3")
		}
		if cfg.StorageS3PublicBaseURL == "" {
			return nil, fmt.Errorf("STORAGE_S3_PUBLIC_BASE_URL is required when STORAGE_DRIVER=s3")
		}
	default:
		return nil, fmt.Errorf("unsupported STORAGE_DRIVER %q: must be 'local' or 's3'", cfg.StorageDriver)
	}

	// Validate port range
	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("PORT must be between 1 and 65535, got %d", cfg.Port)
	}

	// Validate public listing page size (0 falls back to the handler default at
	// render time, but a negative or huge value is rejected up front).
	if cfg.PostPerPage < 0 {
		return nil, fmt.Errorf("POSTS_PER_PAGE must be between 0 (use default) and 100, got %d", cfg.PostPerPage)
	}
	if cfg.PostPerPage > 100 {
		return nil, fmt.Errorf("POSTS_PER_PAGE must be between 0 (use default) and 100, got %d", cfg.PostPerPage)
	}

	// Validate import max size
	if cfg.ImportMaxSizeMB < 1 {
		return nil, fmt.Errorf("IMPORT_MAX_SIZE_MB must be at least 1, got %d", cfg.ImportMaxSizeMB)
	}

	if cfg.WordPressImportTimeout <= 0 {
		return nil, fmt.Errorf("WORDPRESS_IMPORT_TIMEOUT must be positive, got %v", cfg.WordPressImportTimeout)
	}

	if cfg.HugoImportTimeout <= 0 {
		return nil, fmt.Errorf("HUGO_IMPORT_TIMEOUT must be positive, got %v", cfg.HugoImportTimeout)
	}

	return cfg, nil
}

// ImportMaxSize returns the importer upload ceiling in bytes.
func (c *Config) ImportMaxSize() int64 {
	return int64(c.ImportMaxSizeMB) << 20
}

// IsImageGenerationEnabled returns true if the Google Imagen API key is configured
func (c *Config) IsImageGenerationEnabled() bool {
	return c.AIImageGenerationAPIKey != ""
}

// IsTextGenerationEnabled returns true if the AI text generation API key is configured
func (c *Config) IsTextGenerationEnabled() bool {
	return c.AITextGenerationAPIKey != ""
}

// ParseCORSOrigins parses the comma-separated CORS origins string into a slice
// It handles whitespace trimming and validates that each origin has a valid http/https scheme
func (c *Config) ParseCORSOrigins() []string {
	if c.CORSAllowedOrigins == "" {
		return []string{}
	}

	origins := strings.Split(c.CORSAllowedOrigins, ",")
	result := make([]string, 0, len(origins))

	for _, origin := range origins {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" && isValidOrigin(trimmed) {
			result = append(result, trimmed)
		}
	}

	return result
}
