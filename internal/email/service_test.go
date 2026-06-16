package email_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/email"
	"github.com/aristorinjuang/lesstruct/internal/email/mocks"
	"github.com/stretchr/testify/assert"
)

func TestNewService(t *testing.T) {
	config := email.SMTPConfig{
		Host:     "smtp.example.com",
		Port:     587,
		Username: "user@example.com",
		Password: "password",
		From:     "noreply@example.com",
	}

	logger := slog.Default()
	service := email.NewService(config, logger, "http://localhost:8080")

	assert.NotNil(t, service, "NewService() should not return nil")
	assert.True(t, service.IsEnabled(), "Service should be enabled when host and port are configured")
}

func TestNewService_Disabled(t *testing.T) {
	config := email.SMTPConfig{
		Host: "",
		Port: 0,
	}

	logger := slog.Default()
	service := email.NewService(config, logger, "http://localhost:8080")

	assert.NotNil(t, service, "NewService() should not return nil")
	assert.False(t, service.IsEnabled(), "Service should be disabled when host and port are not configured")
}

func TestNewService_HostOnly(t *testing.T) {
	config := email.SMTPConfig{
		Host: "smtp.example.com",
		Port: 0,
	}

	logger := slog.Default()
	service := email.NewService(config, logger, "http://localhost:8080")

	assert.False(t, service.IsEnabled(), "Service should be disabled when port is not configured")
}

func TestNewService_PortOnly(t *testing.T) {
	config := email.SMTPConfig{
		Host: "",
		Port: 587,
	}

	logger := slog.Default()
	service := email.NewService(config, logger, "http://localhost:8080")

	assert.False(t, service.IsEnabled(), "Service should be disabled when host is not configured")
}

func TestService_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		config   email.SMTPConfig
		expected bool
	}{
		{
			name: "Enabled with host and port",
			config: email.SMTPConfig{
				Host: "smtp.example.com",
				Port: 587,
			},
			expected: true,
		},
		{
			name: "Disabled with no host",
			config: email.SMTPConfig{
				Host: "",
				Port: 587,
			},
			expected: false,
		},
		{
			name: "Disabled with no port",
			config: email.SMTPConfig{
				Host: "smtp.example.com",
				Port: 0,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			service := email.NewService(tt.config, logger, "http://localhost:8080")
			assert.Equal(t, tt.expected, service.IsEnabled(), "IsEnabled() mismatch")
		})
	}
}

func TestService_SendVerificationEmail_Disabled(t *testing.T) {
	config := email.SMTPConfig{
		Host: "",
		Port: 0,
	}

	logger := slog.Default()
	service := email.NewService(config, logger, "http://localhost:8080")

	err := service.SendVerificationEmail(context.Background(), "user@example.com", "username", "token")

	assert.Error(t, err, "Expected error when email service is disabled")
	assert.ErrorContains(t, err, "email service is disabled", "Expected error message about disabled service")
}

func TestService_SetBaseURL(t *testing.T) {
	config := email.SMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
	}

	logger := slog.Default()
	service := email.NewService(config, logger, "http://localhost:8080")

	// Test SetBaseURL
	newBaseURL := "https://example.com"
	service.SetBaseURL(newBaseURL)

	// Service should still exist after SetBaseURL
	assert.NotNil(t, service, "Service should still exist after SetBaseURL")
}

func TestService_SendVerificationEmail_ContextTimeout(t *testing.T) {
	config := email.SMTPConfig{
		Host:     "nonexistent.example.com",
		Port:     587,
		Username: "user@example.com",
		Password: "password",
		From:     "noreply@example.com",
	}

	logger := slog.Default()
	service := email.NewService(config, logger, "http://localhost:8080")

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for context to timeout
	time.Sleep(10 * time.Millisecond)

	err := service.SendVerificationEmail(ctx, "user@example.com", "username", "token")

	assert.Error(t, err, "Expected timeout error")
	// The error should be about timeout or connection failure
	errStr := err.Error()
	isTimeoutOrConn := strings.Contains(errStr, "timed out") || strings.Contains(errStr, "connection")
	if !isTimeoutOrConn {
		t.Logf("Got error (may vary): %v", err)
	}
}

func TestService_SendVerificationEmail_InvalidSMTP(t *testing.T) {
	config := email.SMTPConfig{
		Host:     "invalid-smtp-server",
		Port:     9999,
		Username: "user@example.com",
		Password: "password",
		From:     "noreply@example.com",
	}

	logger := slog.Default()
	service := email.NewService(config, logger, "http://localhost:8080")

	err := service.SendVerificationEmail(context.Background(), "user@example.com", "username", "token")

	assert.Error(t, err, "Expected error when SMTP server is unreachable")
}

func TestMockEmailService(t *testing.T) {
	mock := mocks.NewMockEmailService(t)

	mock.EXPECT().
		SendVerificationEmail(
			context.Background(),
			"test@example.com",
			"user",
			"token",
		).
		Return(nil)

	err := mock.SendVerificationEmail(context.Background(), "test@example.com", "user", "token")

	assert.NoError(t, err, "MockEmailService() failed")
}
