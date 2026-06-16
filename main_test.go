package main_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
)

func loadConfigForTest() (*config.Config, error) {
	return config.Load()
}

func TestServerConfiguration(t *testing.T) {
	// Test that server respects environment configuration
	tests := []struct {
		name    string
		port    string
		host    string
		wantErr bool
	}{
		{
			name:    "Valid configuration",
			port:    "8082",
			host:    "127.0.0.1",
			wantErr: false,
		},
		{
			name:    "Default port",
			port:    "",
			host:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			if tt.port != "" {
				_ = os.Setenv("PORT", tt.port)
			} else {
				_ = os.Unsetenv("PORT")
			}
			if tt.host != "" {
				_ = os.Setenv("HOST", tt.host)
			} else {
				_ = os.Unsetenv("HOST")
			}
			_ = os.Setenv("JWT_SECRET", "test-secret-key-that-is-at-least-32-characters-long")

			// Load configuration to verify it works
			cfg, err := loadConfigForTest()
			if (err != nil) != tt.wantErr {
				t.Errorf("loadConfigForTest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				expectedPort := 8080
				if tt.port != "" {
					_, _ = fmt.Sscanf(tt.port, "%d", &expectedPort)
				}
				if cfg.Port != expectedPort {
					t.Errorf("Port = %d; want %d", cfg.Port, expectedPort)
				}

				expectedHost := "0.0.0.0"
				if tt.host != "" {
					expectedHost = tt.host
				}
				if cfg.Host != expectedHost {
					t.Errorf("Host = %s; want %s", cfg.Host, expectedHost)
				}
			}
		})
	}
}
