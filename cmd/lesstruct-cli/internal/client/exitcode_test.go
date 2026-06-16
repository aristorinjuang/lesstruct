package client_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/internal/client"
	"github.com/stretchr/testify/assert"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		want   int
		wrap   bool
		status int
		code   string
	}{
		{name: "nil error", err: nil, want: client.ExitOK},
		{name: "non-api error", err: fmt.Errorf("boom"), want: client.ExitGeneric},
		{name: "network error status 0", status: 0, want: client.ExitGeneric},
		{name: "401 auth", status: http.StatusUnauthorized, want: client.ExitAuth},
		{name: "404 not found", status: http.StatusNotFound, want: client.ExitNotFound},
		{name: "400 validation", status: http.StatusBadRequest, want: client.ExitValidation},
		{name: "422 validation", status: 422, want: client.ExitValidation},
		{name: "429 rate limited", status: http.StatusTooManyRequests, want: client.ExitRateLimited},
		{name: "500 server", status: http.StatusInternalServerError, want: client.ExitServer},
		{name: "503 server", status: http.StatusServiceUnavailable, want: client.ExitServer},
		{name: "wrapped 401 resolves via errors.AsType", status: http.StatusUnauthorized, want: client.ExitAuth, wrap: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			switch {
			case tt.err != nil:
				err = tt.err
			case tt.status != 0 || tt.name == "network error status 0":
				apiErr := &client.APIError{StatusCode: tt.status, Code: tt.code}
				if tt.wrap {
					err = fmt.Errorf("create failed: %w", apiErr)
				} else {
					err = apiErr
				}
			}
			assert.Equal(t, tt.want, client.ExitCode(err))
		})
	}
}

func TestExitCodeConstants(t *testing.T) {
	// The documented scheme — agents depend on these exact values.
	assert.Equal(t, 0, client.ExitOK)
	assert.Equal(t, 1, client.ExitGeneric)
	assert.Equal(t, 2, client.ExitUsage)
	assert.Equal(t, 3, client.ExitAuth)
	assert.Equal(t, 4, client.ExitNotFound)
	assert.Equal(t, 5, client.ExitValidation)
	assert.Equal(t, 6, client.ExitRateLimited)
	assert.Equal(t, 7, client.ExitServer)
}
