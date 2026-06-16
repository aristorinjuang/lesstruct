package profilepicture

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/media"
)

const (
	// ProfilePictureSize is the target size for profile pictures (96x96)
	ProfilePictureSize = 96
	// ProfilePictureMaxFileSize is the maximum allowed file size (10MB)
	ProfilePictureMaxFileSize = 10 * 1024 * 1024
)

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

// Service handles profile picture operations
type Service struct {
	userRepo  UserRepo
	storage   *LocalStorage
	processor *Processor
}

// Upload handles the upload of a profile picture
func (s *Service) Upload(
	ctx context.Context,
	userID int,
	username string,
	file multipart.File,
	header *multipart.FileHeader,
) (string, error) {
	// Validate file size
	if err := media.ValidateFileSize(header.Size); err != nil {
		return "", fmt.Errorf("file size exceeds limit: %w", err)
	}

	// Validate MIME type from header
	if err := media.ValidateMimeType(header.Header.Get("Content-Type")); err != nil {
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
	if err := media.ValidateFileContent(buffer); err != nil {
		return "", fmt.Errorf("invalid file content: %w", err)
	}

	// Reset file reader
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("failed to reset file reader: %w", err)
	}

	// Process image: crop and convert to WebP
	webpData, err := s.processor.CropAndConvertToWebP(file, ProfilePictureSize)
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

// Delete removes a user's profile picture
func (s *Service) Delete(ctx context.Context, userID int) error {
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
	storage *LocalStorage,
	processor *Processor,
) *Service {
	return &Service{
		userRepo:  userRepo,
		storage:   storage,
		processor: processor,
	}
}
