package response_test

import (
	"testing"

	appresponse "github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeCursor(t *testing.T) {
	tests := []struct {
		name     string
		id       int
		expected string
	}{
		{
			name:     "encodes a single-digit id to unpadded base64url",
			id:       5,
			expected: "NQ",
		},
		{
			name:     "encodes a multi-digit id to unpadded base64url",
			id:       100,
			expected: "MTAw",
		},
		{
			name:     "encodes a large id",
			id:       9359,
			expected: "OTM1OQ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, appresponse.EncodeCursor(tt.id))
		})
	}
}

func TestDecodeCursor(t *testing.T) {
	tests := []struct {
		name       string
		cursor     string
		expectedID int
		wantErr    bool
	}{
		{
			name:   "empty cursor means first page",
			cursor: "",
		},
		{
			name:       "decodes a valid cursor",
			cursor:     "MTAw",
			expectedID: 100,
		},
		{
			name:       "round-trips with EncodeCursor",
			cursor:     appresponse.EncodeCursor(9359),
			expectedID: 9359,
		},
		{
			name:    "rejects non-base64 text",
			cursor:  "not-a-cursor!",
			wantErr: true,
		},
		{
			name:    "rejects padded base64",
			cursor:  "MTAw=",
			wantErr: true,
		},
		{
			name:    "rejects non-numeric payload",
			cursor:  "aGVsbG8",
			wantErr: true,
		},
		{
			name:    "rejects zero id",
			cursor:  "MA",
			wantErr: true,
		},
		{
			name:    "rejects negative id",
			cursor:  "LTE",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := appresponse.DecodeCursor(tt.cursor)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, appresponse.ErrInvalidCursor)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedID, id)
		})
	}
}
