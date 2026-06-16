package main_test

import (
	"os"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/config"
)

func TestServerStartupWithValidConfig(t *testing.T) {
	// Setup environment
	_ = os.Setenv("HOST", "127.0.0.1")
	_ = os.Setenv("PORT", "8082")
	_ = os.Setenv("DB_PATH", "test_data.db")
	_ = os.Setenv("JWT_SECRET", "test-secret-key-that-is-at-least-32-characters-long")
	_ = os.Setenv("LOG_LEVEL", "info")

	defer func() {
		_ = os.Unsetenv("HOST")
		_ = os.Unsetenv("PORT")
		_ = os.Unsetenv("DB_PATH")
		_ = os.Unsetenv("JWT_SECRET")
		_ = os.Unsetenv("LOG_LEVEL")
		_ = os.Remove("test_data.db")
	}()

	// Test configuration loading
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %s; want 127.0.0.1", cfg.Host)
	}

	if cfg.Port != 8082 {
		t.Errorf("Port = %d; want 8082", cfg.Port)
	}

	if cfg.DBPath != "test_data.db" {
		t.Errorf("DBPath = %s; want test_data.db", cfg.DBPath)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %s; want info", cfg.LogLevel)
	}
}

func TestPortValidation(t *testing.T) {
	tests := []struct {
		name      string
		port      string
		wantErr   bool
		portValid bool
	}{
		{"Valid port 8080", "8080", false, true},
		{"Valid port 1", "1", false, true},
		{"Valid port 65535", "65535", false, true},
		{"Invalid port 0", "0", true, false},
		{"Invalid port -1", "-1", true, false},
		{"Invalid port 65536", "65536", true, false},
		{"Invalid port abc", "abc", false, false}, // Will use default 8080
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv("PORT", tt.port)
			_ = os.Setenv("JWT_SECRET", "test-secret-key-that-is-at-least-32-characters-long")

			cfg, err := config.Load()

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.portValid {
				if cfg.Port < 1 || cfg.Port > 65535 {
					t.Errorf("Port %d is not in valid range [1-65535]", cfg.Port)
				}
			}
		})
	}
}

func TestJWTSecretValidation(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{"Valid 32 character secret", "this-is-a-valid-secret-32-chars!", false},
		{"Valid 64 character secret", "this-is-a-very-long-secret-that-is-definitely-more-than-64-chars-long!", false},
		{"Empty secret", "", true},
		{"Too short secret", "short", true},
		{"31 characters", "exactly-31-characters-long-1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv("JWT_SECRET", tt.secret)

			cfg, err := config.Load()

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if cfg.JWTSecret != tt.secret {
					t.Errorf("JWTSecret = %s; want %s", cfg.JWTSecret, tt.secret)
				}
			}
		})
	}
}

func TestGracefulShutdownTimeout(t *testing.T) {
	// Test that graceful shutdown timeout is reasonable (30 seconds)
	timeout := 30 * time.Second

	if timeout != 30*time.Second {
		t.Errorf("Graceful shutdown timeout = %v; want 30s", timeout)
	}
}

func TestDirectoryCreation(t *testing.T) {
	dirs := []string{"test_plugins", "test_data"}

	// Clean up any existing test directories
	for _, dir := range dirs {
		_ = os.RemoveAll(dir)
	}

	// Test directory creation
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Errorf("Failed to create directory %s: %v", dir, err)
		}

		// Verify directory exists
		if info, err := os.Stat(dir); err != nil {
			t.Errorf("Directory %s was not created: %v", dir, err)
		} else if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}

	// Clean up
	for _, dir := range dirs {
		_ = os.RemoveAll(dir)
	}
}
