package modules_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/modules"
	"github.com/stretchr/testify/assert"
)

// stubMFASender is a test double for the email-channel code sender.
type stubMFASender struct {
	to       string
	username string
	code     string
	err      error
}

func (s *stubMFASender) SendMFACodeEmail(_ context.Context, to, username, code string) error {
	s.to = to
	s.username = username
	s.code = code
	return s.err
}

// stubLogger records formatted log lines.
type stubLogger struct {
	info []string
}

func (s *stubLogger) Info(format string, args ...any) {
	s.info = append(s.info, fmt.Sprintf(format, args...))
}

func (s *stubLogger) Error(string, ...any) {}

func TestMFACodeDeliverer(t *testing.T) {
	tests := []struct {
		name    string
		sender  *stubMFASender
		devMode bool
		enabled bool
		wantDev bool
		wantErr bool
	}{
		{
			name:    "success - email is used whenever SMTP is configured",
			sender:  &stubMFASender{},
			devMode: true,
			enabled: true,
			wantDev: false,
			wantErr: false,
		},
		{
			name:    "success - dev mode without SMTP logs the code",
			sender:  &stubMFASender{},
			devMode: true,
			enabled: false,
			wantDev: true,
			wantErr: false,
		},
		{
			name:    "error - production without SMTP fails closed through the mailer",
			sender:  &stubMFASender{err: errors.New("email service is disabled")},
			devMode: false,
			enabled: false,
			wantDev: false,
			wantErr: true,
		},
		{
			name:    "error - dev mode with SMTP uses email and surfaces its failure",
			sender:  &stubMFASender{err: errors.New("smtp down")},
			devMode: true,
			enabled: true,
			wantDev: false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &stubLogger{}
			deliverer := modules.NewMFACodeDeliverer(tt.sender, logger, tt.devMode, tt.enabled)

			err := deliverer.DeliverMFACode(context.Background(), "user@example.com", "user", "123456")

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			if tt.wantDev {
				assert.Empty(t, tt.sender.code, "email sender must not run in log mode")
				assert.Len(t, logger.info, 1, "dev log line")
				assert.Contains(t, logger.info[0], "123456", "dev log must carry the code")
				assert.Contains(t, logger.info[0], "u***@example.com", "masked recipient")
				assert.Contains(t, logger.info[0], "user", "username")
				return
			}

			assert.Equal(t, "user@example.com", tt.sender.to, "recipient")
			assert.Equal(t, "user", tt.sender.username, "username")
			assert.Equal(t, "123456", tt.sender.code, "code")
			assert.Empty(t, logger.info, "production must never log the code")
		})
	}
}
