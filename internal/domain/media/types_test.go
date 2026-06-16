package media_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/stretchr/testify/assert"
)

func TestValidateMimeType(t *testing.T) {
	tests := []struct {
		name      string
		mimeType  string
		wantErr   bool
		expectErr error
	}{
		{
			name:     "valid jpeg",
			mimeType: "image/jpeg",
			wantErr:  false,
		},
		{
			name:     "valid png",
			mimeType: "image/png",
			wantErr:  false,
		},
		{
			name:     "valid gif",
			mimeType: "image/gif",
			wantErr:  false,
		},
		{
			name:      "invalid pdf",
			mimeType:  "application/pdf",
			wantErr:   true,
			expectErr: media.ErrInvalidMimeType,
		},
		{
			name:     "valid webp",
			mimeType: "image/webp",
			wantErr:  false,
		},
		{
			name:      "empty string",
			mimeType:  "",
			wantErr:   true,
			expectErr: media.ErrInvalidMimeType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := media.ValidateMimeType(tt.mimeType)

			if tt.wantErr {
				assert.Error(t, err, "ValidateMimeType() expected error")
				if tt.expectErr != nil {
					assert.Equal(t, tt.expectErr, err, "ValidateMimeType() error type")
				}
			} else {
				assert.NoError(t, err, "ValidateMimeType() unexpected error")
			}
		})
	}
}

func TestValidateFileExtension(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "valid jpg",
			filename: "image.jpg",
			wantErr:  false,
		},
		{
			name:     "valid jpeg",
			filename: "photo.jpeg",
			wantErr:  false,
		},
		{
			name:     "valid png",
			filename: "picture.PNG",
			wantErr:  false,
		},
		{
			name:     "valid gif",
			filename: "animation.gif",
			wantErr:  false,
		},
		{
			name:     "invalid pdf",
			filename: "document.pdf",
			wantErr:  true,
		},
		{
			name:     "invalid no extension",
			filename: "noextension",
			wantErr:  true,
		},
		{
			name:     "valid webp",
			filename: "image.webp",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := media.ValidateFileExtension(tt.filename)

			if tt.wantErr {
				assert.Error(t, err, "ValidateFileExtension() expected error")
			} else {
				assert.NoError(t, err, "ValidateFileExtension() unexpected error")
			}
		})
	}
}

func TestValidateAltText(t *testing.T) {
	tests := []struct {
		name      string
		altText   string
		wantErr   bool
		expectErr error
	}{
		{
			name:    "valid alt text",
			altText: "A beautiful sunset over the ocean",
			wantErr: false,
		},
		{
			name:    "empty string",
			altText: "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			altText: "   ",
			wantErr: true,
		},
		{
			name:    "too long",
			altText: string(make([]byte, 501)),
			wantErr: true,
		},
		{
			name:    "at max length",
			altText: string(make([]byte, 500)),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := media.ValidateAltText(tt.altText)

			if tt.wantErr {
				assert.Error(t, err, "ValidateAltText() expected error")
				if tt.expectErr != nil {
					assert.Equal(t, tt.expectErr, err, "ValidateAltText() error type")
				}
			} else {
				assert.NoError(t, err, "ValidateAltText() unexpected error")
			}
		})
	}
}

func TestValidateFileSize(t *testing.T) {
	tests := []struct {
		name    string
		size    int64
		wantErr bool
	}{
		{
			name:    "within limit",
			size:    5 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:    "exactly at limit",
			size:    10 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:    "exceeds limit",
			size:    11 * 1024 * 1024,
			wantErr: true,
		},
		{
			name:    "zero bytes",
			size:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := media.ValidateFileSize(tt.size)

			if tt.wantErr {
				assert.Error(t, err, "ValidateFileSize() expected error")
			} else {
				assert.NoError(t, err, "ValidateFileSize() unexpected error")
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "clean filename",
			filename: "image.jpg",
			want:     "image.jpg",
		},
		{
			name:     "with spaces",
			filename: "my photo.jpg",
			want:     "my_photo.jpg",
		},
		{
			name:     "with special chars",
			filename: "photo@#$%.jpg",
			want:     "photo.jpg",
		},
		{
			name:     "with double dots",
			filename: "photo..jpg",
			want:     "photo.jpg",
		},
		{
			name:     "with trailing spaces",
			filename: " photo.jpg ",
			want:     "photo.jpg",
		},
		{
			name:     "all special characters",
			filename: "@#$%",
			want:     "upload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := media.SanitizeFilename(tt.filename)

			assert.Equal(t, tt.want, got, "SanitizeFilename()")
		})
	}
}

func TestMimeType_IsSupported(t *testing.T) {
	tests := []struct {
		name string
		m    media.MimeType
		want bool
	}{
		{
			name: "jpeg is supported",
			m:    media.MimeTypeJPEG,
			want: true,
		},
		{
			name: "png is supported",
			m:    media.MimeTypePNG,
			want: true,
		},
		{
			name: "gif is supported",
			m:    media.MimeTypeGIF,
			want: true,
		},
		{
			name: "webp is supported for upload",
			m:    media.MimeTypeWebP,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.m.IsSupported()

			assert.Equal(t, tt.want, got, "MimeType.IsSupported()")
		})
	}
}

func TestMimeType_String(t *testing.T) {
	tests := []struct {
		name string
		m    media.MimeType
		want string
	}{
		{
			name: "jpeg string",
			m:    media.MimeTypeJPEG,
			want: "image/jpeg",
		},
		{
			name: "png string",
			m:    media.MimeTypePNG,
			want: "image/png",
		},
		{
			name: "gif string",
			m:    media.MimeTypeGIF,
			want: "image/gif",
		},
		{
			name: "webp string",
			m:    media.MimeTypeWebP,
			want: "image/webp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.m.String()

			assert.Equal(t, tt.want, got, "MimeType.String()")
		})
	}
}

func TestMimeType_IsWebP(t *testing.T) {
	tests := []struct {
		name string
		m    media.MimeType
		want bool
	}{
		{
			name: "webp is webp",
			m:    media.MimeTypeWebP,
			want: true,
		},
		{
			name: "jpeg is not webp",
			m:    media.MimeTypeJPEG,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.m.IsWebP()

			assert.Equal(t, tt.want, got, "MimeType.IsWebP()")
		})
	}
}

func TestValidateFileContent(t *testing.T) {
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	gifHeader := []byte("GIF89a" + string(make([]byte, 500)))
	webpHeader := append([]byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}, make([]byte, 500)...)

	tests := []struct {
		name    string
		buffer  []byte
		wantErr bool
	}{
		{name: "valid jpeg content", buffer: append(jpegHeader, make([]byte, 502)...), wantErr: false},
		{name: "valid png content", buffer: append(pngHeader, make([]byte, 502)...), wantErr: false},
		{name: "valid gif content", buffer: []byte(gifHeader), wantErr: false},
		{name: "valid webp content", buffer: webpHeader, wantErr: false},
		{name: "invalid pdf content", buffer: append([]byte("%PDF-1.4"), make([]byte, 505)...), wantErr: true},
		{name: "invalid executable content", buffer: append([]byte{0x4D, 0x5A}, make([]byte, 510)...), wantErr: true},
		{name: "invalid html content", buffer: append([]byte("<html>"), make([]byte, 506)...), wantErr: true},
		{name: "empty content", buffer: []byte{}, wantErr: true},
			{name: "truncated jpeg content", buffer: []byte{0xFF, 0xD8}, wantErr: true},
			{name: "truncated single byte", buffer: []byte{0xFF}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := media.ValidateFileContent(tt.buffer)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, media.ErrInvalidFileContent, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFileSignature(t *testing.T) {
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	gif87aHeader := []byte("GIF87a")
	gif89aHeader := []byte("GIF89a")
	webpHeader := []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}

	tests := []struct {
		name    string
		buffer  []byte
		wantErr bool
	}{
		{name: "jpeg signature", buffer: jpegHeader, wantErr: false},
		{name: "png signature", buffer: pngHeader, wantErr: false},
		{name: "gif87a signature", buffer: gif87aHeader, wantErr: false},
		{name: "gif89a signature", buffer: gif89aHeader, wantErr: false},
		{name: "webp signature", buffer: webpHeader, wantErr: false},
		{name: "invalid signature", buffer: []byte{0x00, 0x00, 0x00, 0x00}, wantErr: true},
		{name: "short buffer", buffer: []byte{0xFF, 0xD8}, wantErr: true},
		{name: "empty buffer", buffer: []byte{}, wantErr: true},
		{name: "mismatched jpeg extension with png content", buffer: pngHeader, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := media.ValidateFileSignature(tt.buffer)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, media.ErrInvalidFileContent, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
