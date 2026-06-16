package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

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
	Host               string
	Port               int
	DBPath             string
	DBDriver           string
	DBDSN              string
	DBPoolMaxConns     int
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
}

// Load loads configuration from environment variables
// It tries to load .env file first, then reads from actual environment
func Load() (*Config, error) {
	// Try to load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{
		Host:               getEnv("HOST", "0.0.0.0"),
		Port:               getEnvInt("PORT", 8080),
		DBPath:             getEnv("DB_PATH", "data/lesstruct.db"),
		DBDriver:           getEnv("DB_DRIVER", "sqlite"),
		DBDSN:              getEnv("DB_DSN", ""),
		DBPoolMaxConns:     getEnvInt("DB_POOL_MAX_CONNS", 20),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		SMTPHost:           getEnv("SMTP_HOST", ""),
		SMTPPort:           getEnvInt("SMTP_PORT", 587),
		SMTPUser:           getEnv("SMTP_USER", ""),
		SMTPPassword:       getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:           getEnv("SMTP_FROM", ""),
		ConfigDir:          getEnv("CONFIG_DIR", "."),
		ConfigFile:         getEnv("CONFIG_FILE", "config.toml"),
		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173"),
		SiteURL:            getEnv("SITE_URL", "http://localhost:8080"),
		DevMode:            getEnvBool("DEV_MODE", false),
		AdminDevURL:        getEnv("ADMIN_DEV_URL", "http://localhost:5173"),
		ThemeDir:           getEnv("THEME_DIR", ""),

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

	// Validate port range
	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("PORT must be between 1 and 65535, got %d", cfg.Port)
	}

	return cfg, nil
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
