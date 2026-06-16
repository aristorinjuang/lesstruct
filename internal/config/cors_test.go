package config_test

import (
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig_CORSAllowedOrigins_DefaultValue tests that CORS_ALLOWED_ORIGINS has default value
func TestConfig_CORSAllowedOrigins_DefaultValue(t *testing.T) {
	// Arrange - unset any existing CORS_ALLOWED_ORIGINS
	_ = os.Unsetenv("CORS_ALLOWED_ORIGINS")
	_ = os.Unsetenv("JWT_SECRET")

	// Set required JWT_SECRET
	_ = os.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-min-32-chars")

	// Act
	cfg, err := config.Load()
	require.NoError(t, err, "Failed to load config")

	// Assert
	assert.Equal(t, "http://localhost:5173", cfg.CORSAllowedOrigins, "Expected default CORS_ALLOWED_ORIGINS to be http://localhost:5173")
}

// TestConfig_CORSAllowedOrigins_CustomValue tests that CORS_ALLOWED_ORIGINS can be customized
func TestConfig_CORSAllowedOrigins_CustomValue(t *testing.T) {
	// Arrange
	_ = os.Unsetenv("JWT_SECRET")
	_ = os.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-min-32-chars")
	_ = os.Setenv("CORS_ALLOWED_ORIGINS", "https://example.com,https://test.com")

	// Act
	cfg, err := config.Load()
	require.NoError(t, err, "Failed to load config")

	// Assert
	assert.Equal(t, "https://example.com,https://test.com", cfg.CORSAllowedOrigins, "Expected custom CORS_ALLOWED_ORIGINS")
}

// TestConfig_CORSAllowedOrigins_EmptyValue tests that empty CORS_ALLOWED_ORIGINS uses default
func TestConfig_CORSAllowedOrigins_EmptyValue(t *testing.T) {
	// Arrange
	_ = os.Unsetenv("JWT_SECRET")
	_ = os.Unsetenv("CORS_ALLOWED_ORIGINS")
	_ = os.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-min-32-chars")

	// Act
	cfg, err := config.Load()
	require.NoError(t, err, "Failed to load config")

	// Assert - when not set, default value should be used
	assert.Equal(t, "http://localhost:5173", cfg.CORSAllowedOrigins, "Expected default CORS_ALLOWED_ORIGINS")
}

// TestConfig_ParseCORSOrigins_SingleOrigin tests parsing a single CORS origin
func TestConfig_ParseCORSOrigins_SingleOrigin(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		CORSAllowedOrigins: "http://localhost:8080",
	}

	// Act
	origins := cfg.ParseCORSOrigins()

	// Assert
	assert.Equal(t, []string{"http://localhost:8080"}, origins, "Expected single origin")
}

// TestConfig_ParseCORSOrigins_MultipleOrigins tests parsing multiple CORS origins
func TestConfig_ParseCORSOrigins_MultipleOrigins(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		CORSAllowedOrigins: "http://localhost:8080,https://example.com,https://test.com",
	}

	// Act
	origins := cfg.ParseCORSOrigins()

	// Assert
	assert.Equal(t, []string{"http://localhost:8080", "https://example.com", "https://test.com"}, origins, "Expected multiple origins")
}

// TestConfig_ParseCORSOrigins_EmptyString tests parsing empty CORS origins
func TestConfig_ParseCORSOrigins_EmptyString(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		CORSAllowedOrigins: "",
	}

	// Act
	origins := cfg.ParseCORSOrigins()

	// Assert
	assert.Empty(t, origins, "Expected empty slice for empty string")
}

// TestConfig_ParseCORSOrigins_Whitespace tests parsing CORS origins with whitespace
func TestConfig_ParseCORSOrigins_Whitespace(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		CORSAllowedOrigins: "http://localhost:8080 , https://example.com , https://test.com",
	}

	// Act
	origins := cfg.ParseCORSOrigins()

	// Assert - whitespace should be trimmed
	assert.Equal(t, []string{"http://localhost:8080", "https://example.com", "https://test.com"}, origins, "Expected trimmed origins")
}

// TestConfig_ParseCORSOrigins_WithPorts tests parsing CORS origins with ports
func TestConfig_ParseCORSOrigins_WithPorts(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		CORSAllowedOrigins: "http://localhost:8080,https://example.com:443,http://test.com:3000",
	}

	// Act
	origins := cfg.ParseCORSOrigins()

	// Assert
	assert.Equal(t, []string{"http://localhost:8080", "https://example.com:443", "http://test.com:3000"}, origins, "Expected origins with ports")
}

// TestConfig_ParseCORSOrigins_InvalidOrigins tests that invalid origins are filtered out
func TestConfig_ParseCORSOrigins_InvalidOrigins(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		CORSAllowedOrigins: "http://localhost:8080,*,file:///etc/passwd,javascript:alert(1),https://example.com",
	}

	// Act
	origins := cfg.ParseCORSOrigins()

	// Assert - only valid http/https origins should be returned
	assert.Equal(t, []string{"http://localhost:8080", "https://example.com"}, origins, "Expected only valid http/https origins")
}

// TestConfig_ParseCORSOrigins_SchemeOnly tests that origins with scheme but no host are filtered out
func TestConfig_ParseCORSOrigins_SchemeOnly(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		CORSAllowedOrigins: "http://,https://,http://localhost:8080",
	}

	// Act
	origins := cfg.ParseCORSOrigins()

	// Assert - scheme-only entries should be filtered out
	assert.Equal(t, []string{"http://localhost:8080"}, origins, "Expected scheme-only origins filtered out")
}
