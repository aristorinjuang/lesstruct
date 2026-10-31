package pii_test

import (
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel/pii"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testMasterKey(seed byte) []byte {
	key := make([]byte, pii.MasterKeySize)
	for i := range key {
		key[i] = seed + byte(i)
	}
	return key
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		master  []byte
		wantErr bool
	}{
		{
			name:    "success - 32-byte master key",
			master:  testMasterKey(1),
			wantErr: false,
		},
		{
			name:    "error - nil master key",
			master:  nil,
			wantErr: true,
		},
		{
			name:    "error - short master key",
			master:  testMasterKey(1)[:16],
			wantErr: true,
		},
		{
			name:    "error - long master key",
			master:  append(testMasterKey(1), 0x00),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pii.New(tt.master)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, pii.ErrInvalidMasterKey)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
		})
	}
}

func TestCipher_RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "success - email",
			input: "shopper@example.com",
		},
		{
			name:  "success - name",
			input: "Annisa Putri",
		},
		{
			name:  "success - unicode address",
			input: `{"street":"Jl. Kemang Raya 21","city":"Jakarta Selatan"}`,
		},
		{
			name:  "success - long reference",
			input: strings.Repeat("BR-12345 ", 50),
		},
		{
			name:  "success - empty stays empty",
			input: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sealer, err := pii.New(testMasterKey(7))
			require.NoError(t, err)

			sealed, err := sealer.Encrypt(tt.input)
			require.NoError(t, err)
			if tt.input == "" {
				assert.Equal(t, "", sealed)
			} else {
				assert.True(t, pii.IsEncrypted(sealed))
				assert.NotContains(t, sealed, tt.input)
			}

			opened, err := sealer.Decrypt(sealed)
			require.NoError(t, err)
			assert.Equal(t, tt.input, opened)
		})
	}
}

func TestCipher_NonceUniqueness(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "success - same plaintext seals differently twice",
			input: "shopper@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sealer, err := pii.New(testMasterKey(9))
			require.NoError(t, err)

			first, err := sealer.Encrypt(tt.input)
			require.NoError(t, err)
			second, err := sealer.Encrypt(tt.input)
			require.NoError(t, err)

			assert.NotEqual(t, first, second)
		})
	}
}

func TestCipher_TamperRejection(t *testing.T) {
	sealer, err := pii.New(testMasterKey(11))
	require.NoError(t, err)
	sealed, err := sealer.Encrypt("shopper@example.com")
	require.NoError(t, err)
	raw := sealed[len(pii.Prefix):]
	tampered := raw[:len(raw)-4] + "AAAA"

	tests := []struct {
		name    string
		stored  string
		wantErr bool
		want    string
	}{
		{
			name:    "error - flipped envelope bytes",
			stored:  pii.Prefix + tampered,
			wantErr: true,
		},
		{
			name:    "error - truncated envelope",
			stored:  pii.Prefix + raw[:8],
			wantErr: true,
		},
		{
			name:    "error - non-base64 envelope",
			stored:  pii.Prefix + "!!!not-base64!!!",
			wantErr: true,
		},
		{
			name:    "error - foreign key cannot open",
			stored:  sealed,
			wantErr: false,
			want:    "shopper@example.com",
		},
		{
			name:    "success - legacy plaintext passes through",
			stored:  "shopper@example.com",
			wantErr: false,
			want:    "shopper@example.com",
		},
		{
			name:    "success - unknown prefix passes through",
			stored:  "v2:some-future-envelope",
			wantErr: false,
			want:    "v2:some-future-envelope",
		},
		{
			name:    "success - empty stays empty",
			stored:  "",
			wantErr: false,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sealer.Decrypt(tt.stored)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, pii.ErrCorruptCiphertext)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCipher_ForeignKeyCannotOpen(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{
			name: "error - envelope under another master key",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, err := pii.New(testMasterKey(13))
			require.NoError(t, err)
			second, err := pii.New(testMasterKey(77))
			require.NoError(t, err)

			sealed, err := first.Encrypt("shopper@example.com")
			require.NoError(t, err)

			_, err = second.Decrypt(sealed)
			require.Error(t, err)
			assert.Equal(t, tt.want, true)
			assert.ErrorIs(t, err, pii.ErrCorruptCiphertext)
		})
	}
}

func TestCipher_Index(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "success - email",
			input: "shopper@example.com",
		},
		{
			name:  "success - reference",
			input: "BR-12345",
		},
		{
			name:  "success - empty stays empty",
			input: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sealer, err := pii.New(testMasterKey(21))
			require.NoError(t, err)

			first := sealer.Index(tt.input)
			second := sealer.Index(tt.input)

			assert.Equal(t, first, second)
			if tt.input == "" {
				assert.Equal(t, "", first)
			} else {
				assert.NotEmpty(t, first)
				assert.NotContains(t, first, tt.input)
				assert.NotEqual(t, tt.input, first)
			}
		})
	}
}

func TestCipher_IndexSeparation(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
	}{
		{
			name: "success - distinct inputs yield distinct indexes",
			a:    "shopper@example.com",
			b:    "someone-else@example.com",
		},
		{
			name: "success - case variants yield distinct indexes",
			a:    "shopper@example.com",
			b:    "Shopper@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sealer, err := pii.New(testMasterKey(23))
			require.NoError(t, err)
			other, err := pii.New(testMasterKey(99))
			require.NoError(t, err)

			assert.NotEqual(t, sealer.Index(tt.a), sealer.Index(tt.b))
			assert.NotEqual(t, sealer.Index(tt.a), other.Index(tt.a))
		})
	}
}

func TestIsEncrypted(t *testing.T) {
	tests := []struct {
		name   string
		stored string
		want   bool
	}{
		{
			name:   "success - enveloped value",
			stored: "v1:c2VhbGVk",
			want:   true,
		},
		{
			name:   "success - plaintext",
			stored: "shopper@example.com",
			want:   false,
		},
		{
			name:   "success - empty",
			stored: "",
			want:   false,
		},
		{
			name:   "success - other prefix",
			stored: "v2:c2VhbGVk",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pii.IsEncrypted(tt.stored))
		})
	}
}
