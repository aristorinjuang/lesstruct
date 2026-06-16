package util_test

import (
	"bytes"
	"testing"

	apputil "github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
)

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := apputil.NewLogger(&buf)

	tests := []struct {
		name       string
		logFunc    func(string, ...any)
		logMessage string
		wantPrefix string
	}{
		{
			name:       "Info log",
			logFunc:    logger.Info,
			logMessage: "test info message",
			wantPrefix: "INFO:",
		},
		{
			name:       "Error log",
			logFunc:    logger.Error,
			logMessage: "test error message",
			wantPrefix: "ERROR:",
		},
		{
			name:       "Debug log",
			logFunc:    logger.Debug,
			logMessage: "test debug message",
			wantPrefix: "DEBUG:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc(tt.logMessage)
			output := buf.String()

			assert.Contains(t, output, tt.wantPrefix, "Log output missing prefix")
			assert.Contains(t, output, tt.logMessage, "Log output missing message")
		})
	}
}

func TestLoggerFormatting(t *testing.T) {
	var buf bytes.Buffer
	logger := apputil.NewLogger(&buf)

	logger.Info("Server started on %s:%d", "localhost", 8080)
	output := buf.String()

	assert.Contains(t, output, "localhost", "Formatted log missing hostname")
	assert.Contains(t, output, "8080", "Formatted log missing port")
}
