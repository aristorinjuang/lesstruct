package auth_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateToken(t *testing.T) {
	tests := []struct {
		name    string
		reader  io.Reader
		wantLen int
		wantErr bool
	}{
		{
			name:    "success - 32 random bytes in hex",
			reader:  bytes.NewReader(bytes.Repeat([]byte{0xAB}, 32)),
			wantLen: 64,
		},
		{
			name:    "error - entropy source fails",
			reader:  &failAfterReader{err: errors.New("entropy failure")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := authdomain.SetRandReaderForTest(tt.reader)
			defer restore()

			token, err := authdomain.GenerateToken()

			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, token)
				return
			}

			require.NoError(t, err)
			assert.Len(t, token, tt.wantLen)
			assert.Equal(t, strings.Repeat("ab", 32), token)
		})
	}
}
