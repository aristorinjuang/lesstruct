package profilepicture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	kernelstorage "github.com/aristorinjuang/lesstruct/kernel/storage"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
)

const (
	// ProfilePictureSize is the target size for profile pictures (96x96)
	ProfilePictureSize = 96
	// ProfilePictureMaxFileSize is the maximum allowed file size (10MB)
	ProfilePictureMaxFileSize = 10 * 1024 * 1024
)

// principalID parses the principal's decimal ID for repository calls,
// mirroring identity.IntID (fail-closed with the kernel sentinel).
func principalID(principal kernel.Principal) (int, error) {
	id, err := strconv.Atoi(principal.ID)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", kernel.ErrInvalidPrincipalID, principal.ID)
	}
	return id, nil
}

// SanitizeUsernameForFilename removes path separators and unsafe characters
// from a username to prevent path traversal in filenames.
func SanitizeUsernameForFilename(username string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, username)
	if safe == "" {
		return "user"
	}
	return safe
}

// UserRepo defines the interface for user repository operations needed by the service
type UserRepo interface {
	GetUserByID(ctx context.Context, userID int) (string, error)
	UpdateProfilePicture(ctx context.Context, userID int, profilePicture string) error
	DeleteProfilePicture(ctx context.Context, userID int) error
}

// Redactor blurs plates and nudity while keeping faces visible. The
// composition root adapts the privacy service onto it; a nil Redactor
// keeps the legacy pipeline byte-identical.
type Redactor interface {
	Redact(ctx context.Context, imageBytes []byte) ([]byte, error)
}

// ServiceOption configures a Service.
type ServiceOption func(*Service)

// WithRedactor adopts the privacy redactor (composition root, nil when
// AI_SAM_API_KEY is unset). Without one uploads skip the scan entirely.
func WithRedactor(redactor Redactor) ServiceOption {
	return func(s *Service) {
		if redactor != nil {
			s.redactor = redactor
		}
	}
}

// Service handles profile picture operations
type Service struct {
	userRepo  UserRepo
	storage   kernelstorage.Storage
	processor *Processor
	redactor  Redactor
}

// Upload handles the upload of a profile picture for the acting principal.
// The principal supplies both the owner ID and the username for the filename.
func (s *Service) Upload(
	ctx context.Context,
	principal kernel.Principal,
	file multipart.File,
	header *multipart.FileHeader,
) (string, error) {
	userID, err := principalID(principal)
	if err != nil {
		return "", err
	}
	username := principal.Username
	// Validate file size
	if err := mediadomain.ValidateFileSize(header.Size); err != nil {
		return "", fmt.Errorf("file size exceeds limit: %w", err)
	}

	// Validate MIME type from header
	if err := mediadomain.ValidateMimeType(header.Header.Get("Content-Type")); err != nil {
		return "", fmt.Errorf("invalid MIME type: %w", err)
	}

	// Read first 512 bytes for content type detection
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	buffer = buffer[:n]

	// Validate file content
	if err := mediadomain.ValidateFileContent(buffer); err != nil {
		return "", fmt.Errorf("invalid file content: %w", err)
	}

	// Reset file reader
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("failed to reset file reader: %w", err)
	}

	// Privacy redaction precedes crop/convert so variants inherit blurred
	// pixels. Faces stay visible; plates and nudity blur. Any scan failure
	// rejects fail-closed and stores nothing.
	cropSource := io.Reader(file)
	if s.redactor != nil {
		_, _ = file.Seek(0, io.SeekStart)
		raw, err := io.ReadAll(file)
		if err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		redacted, err := s.redactor.Redact(ctx, raw)
		if err != nil {
			return "", fmt.Errorf("privacy redaction failed: %w", errors.Join(mediadomain.ErrPrivacyScanFailed, err))
		}
		cropSource = bytes.NewReader(redacted)
	}

	// Process image: crop and convert to WebP
	webpData, err := s.processor.CropAndConvertToWebP(cropSource, ProfilePictureSize)
	if err != nil {
		return "", fmt.Errorf("failed to process image: %w", err)
	}

	// Generate filename
	safeName := SanitizeUsernameForFilename(username)
	filename := fmt.Sprintf("%s_%s.webp", safeName, time.Now().Format("20060102150405"))

	// Save new file
	reader := bytes.NewReader(webpData)
	filePath, err := s.storage.Save(filename, reader)
	if err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	// Get old filename before updating DB
	oldFilename, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		// If we can't get old filename, still update with new one
		log.Printf("WARNING: failed to get old profile picture for user %d: %v", userID, err)
	}

	// Update DB
	if err := s.userRepo.UpdateProfilePicture(ctx, userID, filename); err != nil {
		// Try to clean up new file on DB failure
		_ = s.storage.Delete(filePath)
		return "", fmt.Errorf("failed to update profile picture in database: %w", err)
	}

	// Delete old file if exists and differs from new filename
	if oldFilename != "" && oldFilename != filename {
		if err := s.storage.Delete(oldFilename); err != nil {
			log.Printf("WARNING: failed to delete old profile picture %s: %v", oldFilename, err)
		}
	}

	return s.storage.GetURL(filename), nil
}

// Delete removes the acting principal's profile picture
func (s *Service) Delete(ctx context.Context, principal kernel.Principal) error {
	userID, err := principalID(principal)
	if err != nil {
		return err
	}
	// Get current filename from DB
	filename, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user profile picture: %w", err)
	}

	// No-op if user has no picture
	if filename == "" {
		return nil
	}

	// Delete file from disk
	if err := s.storage.Delete(filename); err != nil {
		log.Printf("WARNING: failed to delete profile picture file %s: %v", filename, err)
	}

	// Clear DB column
	if err := s.userRepo.DeleteProfilePicture(ctx, userID); err != nil {
		return fmt.Errorf("failed to clear profile picture in database: %w", err)
	}

	return nil
}

// NewService creates a new profile picture service
func NewService(
	userRepo UserRepo,
	storage kernelstorage.Storage,
	processor *Processor,
	opts ...ServiceOption,
) *Service {
	s := &Service{
		userRepo:  userRepo,
		storage:   storage,
		processor: processor,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
