package config_test

import (
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig_ImportMaxSizeMB_DefaultValue tests that IMPORT_MAX_SIZE_MB defaults to 100.
func TestConfig_ImportMaxSizeMB_DefaultValue(t *testing.T) {
	// Arrange - unset any existing IMPORT_MAX_SIZE_MB
	_ = os.Unsetenv("IMPORT_MAX_SIZE_MB")
	_ = os.Unsetenv("JWT_SECRET")
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-min-32-chars")

	// Act
	cfg, err := config.Load()
	require.NoError(t, err, "Failed to load config")

	// Assert
	assert.Equal(t, 100, cfg.ImportMaxSizeMB, "Expected default IMPORT_MAX_SIZE_MB to be 100")
}

// TestConfig_ImportMaxSizeMB_CustomValue tests that IMPORT_MAX_SIZE_MB can be customized.
func TestConfig_ImportMaxSizeMB_CustomValue(t *testing.T) {
	// Arrange
	_ = os.Unsetenv("JWT_SECRET")
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-min-32-chars")
	t.Setenv("IMPORT_MAX_SIZE_MB", "250")

	// Act
	cfg, err := config.Load()
	require.NoError(t, err, "Failed to load config")

	// Assert
	assert.Equal(t, 250, cfg.ImportMaxSizeMB, "Expected custom IMPORT_MAX_SIZE_MB")
}

// TestConfig_ImportMaxSizeMB_InvalidValue tests that a non-positive value is rejected.
func TestConfig_ImportMaxSizeMB_InvalidValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "zero is rejected",
			value: "0",
		},
		{
			name:  "negative is rejected",
			value: "-5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			_ = os.Unsetenv("JWT_SECRET")
			t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-min-32-chars")
			t.Setenv("IMPORT_MAX_SIZE_MB", tt.value)

			// Act
			_, err := config.Load()

			// Assert
			require.Error(t, err)
			assert.Contains(t, err.Error(), "IMPORT_MAX_SIZE_MB")
		})
	}
}

// TestConfig_ImportMaxSize_BytesConversion tests that ImportMaxSize converts MB to bytes.
func TestConfig_ImportMaxSize_BytesConversion(t *testing.T) {
	tests := []struct {
		name     string
		mb       int
		expected int64
	}{
		{
			name:     "100MB default",
			mb:       100,
			expected: 100 << 20,
		},
		{
			name:     "250MB custom",
			mb:       250,
			expected: 250 << 20,
		},
		{
			name:     "1MB minimum",
			mb:       1,
			expected: 1 << 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				ImportMaxSizeMB: tt.mb,
			}

			assert.Equal(t, tt.expected, cfg.ImportMaxSize())
		})
	}
}
