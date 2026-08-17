package config_test

import (
	"os"
	"testing"

	appconfig "github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_DatabaseDriver(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-that-is-at-least-32-characters-long")

	tests := []struct {
		name     string
		setupEnv func(*testing.T)
		wantErr  bool
		errMsg   string
		checkCfg func(*testing.T, *appconfig.Config)
	}{
		{
			name: "default sqlite driver",
			setupEnv: func(t *testing.T) {
				_ = os.Unsetenv("DB_DRIVER")
				_ = os.Unsetenv("DB_DSN")
				_ = os.Unsetenv("DB_POOL_MAX_CONNS")
			},
			wantErr: false,
			checkCfg: func(t *testing.T, cfg *appconfig.Config) {
				assert.Equal(t, "sqlite", cfg.DBDriver)
				assert.Equal(t, "", cfg.DBDSN)
				assert.Equal(t, 20, cfg.DBPoolMaxConns)
			},
		},
		{
			name: "postgres driver with valid DSN",
			setupEnv: func(t *testing.T) {
				t.Setenv("DB_DRIVER", "postgres")
				t.Setenv("DB_DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable")
				t.Setenv("DB_POOL_MAX_CONNS", "20")
			},
			wantErr: false,
			checkCfg: func(t *testing.T, cfg *appconfig.Config) {
				assert.Equal(t, "postgres", cfg.DBDriver)
				assert.Equal(t, "postgres://user:pass@localhost:5432/db?sslmode=disable", cfg.DBDSN)
				assert.Equal(t, 20, cfg.DBPoolMaxConns)
			},
		},
		{
			name: "postgres driver without DB_DSN",
			setupEnv: func(t *testing.T) {
				t.Setenv("DB_DRIVER", "postgres")
				_ = os.Unsetenv("DB_DSN")
				_ = os.Unsetenv("DB_POOL_MAX_CONNS")
			},
			wantErr: true,
			errMsg:   "DB_DSN is required when DB_DRIVER=postgres",
		},
		{
			name: "postgres driver with invalid pool size",
			setupEnv: func(t *testing.T) {
				t.Setenv("DB_DRIVER", "postgres")
				t.Setenv("DB_DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable")
				t.Setenv("DB_POOL_MAX_CONNS", "0")
			},
			wantErr: true,
			errMsg:   "DB_POOL_MAX_CONNS must be at least 1",
		},
		{
			name: "mysql driver with valid DSN",
			setupEnv: func(t *testing.T) {
				t.Setenv("DB_DRIVER", "mysql")
				t.Setenv("DB_DSN", "user:password@tcp(localhost:3306)/lesstruct?parseTime=true&multiStatements=true")
				t.Setenv("DB_POOL_MAX_CONNS", "20")
			},
			wantErr: false,
			checkCfg: func(t *testing.T, cfg *appconfig.Config) {
				assert.Equal(t, "mysql", cfg.DBDriver)
				assert.Equal(t, "user:password@tcp(localhost:3306)/lesstruct?parseTime=true&multiStatements=true", cfg.DBDSN)
				assert.Equal(t, 20, cfg.DBPoolMaxConns)
			},
		},
		{
			name: "mysql driver without DB_DSN",
			setupEnv: func(t *testing.T) {
				t.Setenv("DB_DRIVER", "mysql")
				_ = os.Unsetenv("DB_DSN")
				_ = os.Unsetenv("DB_POOL_MAX_CONNS")
			},
			wantErr: true,
			errMsg:   "DB_DSN is required when DB_DRIVER=mysql",
		},
		{
			name: "mysql driver without parseTime=true",
			setupEnv: func(t *testing.T) {
				t.Setenv("DB_DRIVER", "mysql")
				t.Setenv("DB_DSN", "user:password@tcp(localhost:3306)/lesstruct?multiStatements=true")
				t.Setenv("DB_POOL_MAX_CONNS", "20")
			},
			wantErr: true,
			errMsg:   "DB_DSN must contain parseTime=true for MySQL driver",
		},
		{
			name: "mysql driver without multiStatements=true",
			setupEnv: func(t *testing.T) {
				t.Setenv("DB_DRIVER", "mysql")
				t.Setenv("DB_DSN", "user:password@tcp(localhost:3306)/lesstruct?parseTime=true")
				t.Setenv("DB_POOL_MAX_CONNS", "20")
			},
			wantErr: true,
			errMsg:   "DB_DSN must contain multiStatements=true for MySQL driver",
		},
		{
			name: "mysql driver with invalid pool size",
			setupEnv: func(t *testing.T) {
				t.Setenv("DB_DRIVER", "mysql")
				t.Setenv("DB_DSN", "user:password@tcp(localhost:3306)/lesstruct?parseTime=true&multiStatements=true")
				t.Setenv("DB_POOL_MAX_CONNS", "0")
			},
			wantErr: true,
			errMsg:   "DB_POOL_MAX_CONNS must be at least 1",
		},
		{
			name: "unsupported database driver",
			setupEnv: func(t *testing.T) {
				t.Setenv("DB_DRIVER", "mongodb")
				_ = os.Unsetenv("DB_DSN")
				_ = os.Unsetenv("DB_POOL_MAX_CONNS")
			},
			wantErr: true,
			errMsg:   `unsupported DB_DRIVER "mongodb": must be 'sqlite', 'postgres', or 'mysql'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv(t)
			cfg, err := appconfig.Load()

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
			if tt.checkCfg != nil {
				tt.checkCfg(t, cfg)
			}
		})
	}
}

func TestLoad_DevMode(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-that-is-at-least-32-characters-long")

	tests := []struct {
		name     string
		envVal   string
		expected bool
	}{
		{name: "true lowercase", envVal: "true", expected: true},
		{name: "TRUE uppercase", envVal: "TRUE", expected: true},
		{name: "1", envVal: "1", expected: true},
		{name: "false lowercase", envVal: "false", expected: false},
		{name: "FALSE uppercase", envVal: "FALSE", expected: false},
		{name: "0", envVal: "0", expected: false},
		{name: "empty string", envVal: "", expected: false},
		{name: "random string", envVal: "yes", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DEV_MODE", tt.envVal)

			cfg, err := appconfig.Load()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.DevMode)
		})
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    func(*testing.T)
		wantErr     bool
		checkConfig func(*testing.T, *appconfig.Config)
	}{
		{
			name: "Default values",
			setupEnv: func(t *testing.T) {
				_ = os.Unsetenv("HOST")
				_ = os.Unsetenv("PORT")
				_ = os.Unsetenv("DB_PATH")
				_ = os.Unsetenv("LOG_LEVEL")
				_ = os.Unsetenv("DEV_MODE")
				_ = os.Unsetenv("THEME_DIR")
				// Set JWT_SECRET to empty to prevent .env loading (godotenv does not override existing vars).
				// Empty JWT_SECRET triggers "JWT_SECRET is required" validation error.
				t.Setenv("JWT_SECRET", "")
			},
			wantErr: true, // Missing JWT_SECRET
		},
		{
			name: "All environment variables set",
			setupEnv: func(t *testing.T) {
				t.Setenv("HOST", "127.0.0.1")
				t.Setenv("PORT", "9000")
				t.Setenv("DB_PATH", "custom/path/db.sqlite")
				t.Setenv("JWT_SECRET", "test-secret-key-that-is-at-least-32-characters-long")
				t.Setenv("LOG_LEVEL", "debug")
				_ = os.Unsetenv("DEV_MODE")
				t.Setenv("THEME_DIR", "themes/mytheme")
			},
			wantErr: false,
			checkConfig: func(t *testing.T, cfg *appconfig.Config) {
				assert.Equal(t, "127.0.0.1", cfg.Host, "Host")
				assert.Equal(t, 9000, cfg.Port, "Port")
				assert.Equal(t, "custom/path/db.sqlite", cfg.DBPath, "DBPath")
				assert.Equal(t, "test-secret-key-that-is-at-least-32-characters-long", cfg.JWTSecret, "JWTSecret")
				assert.Equal(t, "debug", cfg.LogLevel, "LogLevel")
				assert.Equal(t, "http://localhost:8080", cfg.SiteURL, "SiteURL default")
				assert.False(t, cfg.DevMode, "DevMode default")
				assert.Equal(t, "themes/mytheme", cfg.ThemeDir, "ThemeDir")
			},
		},
		{
			name: "THEME_DIR defaults to empty",
			setupEnv: func(t *testing.T) {
				t.Setenv("JWT_SECRET", "test-secret-key-that-is-at-least-32-characters-long")
				_ = os.Unsetenv("THEME_DIR")
			},
			wantErr: false,
			checkConfig: func(t *testing.T, cfg *appconfig.Config) {
				assert.Equal(t, "", cfg.ThemeDir, "ThemeDir default")
			},
		},
		{
			name: "Missing JWT_SECRET",
			setupEnv: func(t *testing.T) {
				_ = os.Unsetenv("JWT_SECRET")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv(t)
			cfg, err := appconfig.Load()

			if tt.wantErr {
				assert.Error(t, err, "Load() expected error")
				return
			}

			require.NoError(t, err, "Load() unexpected error")

			if tt.checkConfig != nil {
				tt.checkConfig(t, cfg)
			}
		})
	}
}
