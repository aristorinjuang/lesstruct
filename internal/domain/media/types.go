package media

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

var (
	// ErrMediaNotFound is returned when media cannot be found
	ErrMediaNotFound = errors.New("media not found")
	// ErrInvalidFile is returned when file validation fails
	ErrInvalidFile = errors.New("invalid file type")
	// ErrFileTooLarge is returned when file size exceeds limit
	ErrFileTooLarge = errors.New("file size exceeds limit")
	// ErrDuplicateMedia is returned when media with same hash already exists
	ErrDuplicateMedia = errors.New("media already exists")
	// ErrInvalidAltText is returned when alt text validation fails
	ErrInvalidAltText = errors.New("alt text is required and must be less than 500 characters")
	// ErrInvalidMimeType is returned when mime type is not supported
	ErrInvalidMimeType = errors.New("invalid mime type")
	// ErrUnauthorized is returned when user is not authorized for the operation
	ErrUnauthorized = errors.New("unauthorized to delete this media")
	// ErrInvalidFileContent is returned when file content does not match a supported image type
	ErrInvalidFileContent = errors.New("file content does not match a supported image type")
)

// DuplicateMediaError is returned when a media file with the same hash already exists
type DuplicateMediaError struct {
	Existing *Media
}

func (e *DuplicateMediaError) Error() string {
	if e.Existing == nil {
		return "media already exists"
	}
	return fmt.Sprintf("media already exists: %s", e.Existing.OriginalFilename)
}

const (
	// MaxFileSize is the maximum allowed file size (10MB)
	MaxFileSize = 10 * 1024 * 1024
	// MaxAltTextLength is the maximum allowed alt text length
	MaxAltTextLength = 500
)

// SupportedMimeTypes contains all supported image mime types
var SupportedMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

// SupportedExtensions contains all supported file extensions
var SupportedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
}

// MimeType represents the mime type of an image
type MimeType string

const (
	// MimeTypeJPEG represents JPEG image format
	MimeTypeJPEG MimeType = "image/jpeg"
	// MimeTypePNG represents PNG image format
	MimeTypePNG MimeType = "image/png"
	// MimeTypeGIF represents GIF image format
	MimeTypeGIF MimeType = "image/gif"
	// MimeTypeWebP represents WebP image format
	MimeTypeWebP MimeType = "image/webp"
)

// String returns the string representation of the mime type
func (m MimeType) String() string {
	return string(m)
}

// IsSupported checks if the mime type is supported for upload
func (m MimeType) IsSupported() bool {
	return SupportedMimeTypes[string(m)]
}

// IsWebP checks if the mime type is WebP
func (m MimeType) IsWebP() bool {
	return m == MimeTypeWebP
}

// ImageMetadata contains image dimension information
type ImageMetadata struct {
	Width  int
	Height int
}

// MediaVariant holds metadata for a single thumbnail variant.
type MediaVariant struct {
	FilePath string `json:"filePath"`
	URL      string `json:"url"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// Media represents a media file in the system
type Media struct {
	ID               int           `json:"id"`
	UserID           int           `json:"userId"`
	Filename         string        `json:"filename"`
	OriginalFilename string        `json:"originalFilename"`
	MimeType         MimeType      `json:"mimeType"`
	FileSize         int64         `json:"fileSize"`
	Width            int           `json:"width"`
	Height           int           `json:"height"`
	AltText          string        `json:"altText"`
	IsWebP           bool          `json:"isWebp"`
	FilePath         string        `json:"filePath"`
	URL              string        `json:"url"`
	Hash             string                  `json:"hash"`
	Variants         map[string]MediaVariant  `json:"variants"`
	UploadedBy       string        `json:"uploadedBy"`
	CreatedAt        string        `json:"createdAt"`
	UpdatedAt        string        `json:"updatedAt"`
}

// ValidateMimeType validates if a mime type is supported
func ValidateMimeType(mimeType string) error {
	if !SupportedMimeTypes[mimeType] {
		return ErrInvalidMimeType
	}
	return nil
}

// ValidateFileExtension validates if a file extension is supported
func ValidateFileExtension(filename string) error {
	ext := strings.ToLower(getFileExtension(filename))
	if !SupportedExtensions[ext] {
		return ErrInvalidFile
	}
	return nil
}

// ValidateAltText validates the alt text field
func ValidateAltText(altText string) error {
	altText = strings.TrimSpace(altText)
	if altText == "" || utf8.RuneCountInString(altText) > MaxAltTextLength {
		return ErrInvalidAltText
	}
	return nil
}

// MinFileSize is the minimum allowed file size (1 byte)
const MinFileSize int64 = 1

// ValidateFileSize validates if a file size is within limits
func ValidateFileSize(size int64) error {
	if size < MinFileSize {
		return ErrInvalidFile
	}
	if size > MaxFileSize {
		return ErrFileTooLarge
	}
	return nil
}

// getFileExtension extracts the file extension from a filename
func getFileExtension(filename string) string {
	idx := strings.LastIndex(filename, ".")
	if idx == -1 {
		return ""
	}
	return filename[idx:]
}

// SanitizeFilename sanitizes a filename by removing special characters
func SanitizeFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	filename = strings.ReplaceAll(filename, " ", "_")

	var result strings.Builder
	prevWasDot := false
	for _, r := range filename {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-' {
			result.WriteRune(r)
			prevWasDot = false
		} else if r == '.' {
			if !prevWasDot {
				result.WriteRune(r)
				prevWasDot = true
			}
		}
	}

	sanitized := result.String()
	if sanitized == "" {
		return "upload"
	}
	return sanitized
}

// ValidateFileContent validates file content using http.DetectContentType on the first 512 bytes
func ValidateFileContent(buffer []byte) error {
	if len(buffer) == 0 {
		return ErrInvalidFileContent
	}

	detected := http.DetectContentType(buffer)
	if SupportedMimeTypes[detected] {
		return nil
	}

	// http.DetectContentType does not detect WebP; check magic bytes as fallback
	if len(buffer) >= 12 &&
		buffer[0] == 0x52 && buffer[1] == 0x49 && buffer[2] == 0x46 && buffer[3] == 0x46 &&
		buffer[8] == 0x57 && buffer[9] == 0x45 && buffer[10] == 0x42 && buffer[11] == 0x50 {
		return nil
	}

	return ErrInvalidFileContent
}

// ValidateFileSignature validates file content by checking magic number signatures
func ValidateFileSignature(buffer []byte) error {
	if len(buffer) < 4 {
		return ErrInvalidFileContent
	}

	switch {
	case buffer[0] == 0xFF && buffer[1] == 0xD8 && buffer[2] == 0xFF:
		return nil
	case len(buffer) >= 8 && buffer[0] == 0x89 && buffer[1] == 0x50 && buffer[2] == 0x4E && buffer[3] == 0x47 && buffer[4] == 0x0D && buffer[5] == 0x0A && buffer[6] == 0x1A && buffer[7] == 0x0A:
		return nil
	case len(buffer) >= 6 && buffer[0] == 0x47 && buffer[1] == 0x49 && buffer[2] == 0x46 && buffer[3] == 0x38 && (buffer[4] == 0x37 || buffer[4] == 0x39) && buffer[5] == 0x61:
		return nil
	case len(buffer) >= 12 && buffer[0] == 0x52 && buffer[1] == 0x49 && buffer[2] == 0x46 && buffer[3] == 0x46 && buffer[8] == 0x57 && buffer[9] == 0x45 && buffer[10] == 0x42 && buffer[11] == 0x50:
		return nil
	default:
		return ErrInvalidFileContent
	}
}
