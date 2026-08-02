package main_test

import (
	"fmt"
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
			// Set the variables explicitly (empty string = "use default") rather
			// than unsetting them: godotenv.Load() inside config.Load() only fills
			// vars that are absent from the environment, so an empty-but-set var
			// keeps a local .env file (e.g. PORT=8081) from overriding the default.
			t.Setenv("PORT", tt.port)
			t.Setenv("HOST", tt.host)
			t.Setenv("JWT_SECRET", "test-secret-key-that-is-at-least-32-characters-long")

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
