package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/constants"
	"github.com/aristorinjuang/lesstruct/internal/domain/thumbnail"
)

// ParseDateFilter converts a predefined date filter string to a time threshold
func ParseDateFilter(dateFilter string) (time.Time, bool) {
	now := time.Now()
	switch dateFilter {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), true
	case "this_week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location()), true
	case "this_month":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), true
	default:
		return time.Time{}, false
	}
}

// UploadRequest represents a request to upload media
type UploadRequest struct {
	File       multipart.File
	FileHeader *multipart.FileHeader
	UserID     int
	AltText    string
}

// Service handles media business logic
type Service struct {
	repo             Repository
	storage          Storage
	processor        *Processor
	thumbnailService *thumbnail.Service
}

// validateFile validates the uploaded file
func (s *Service) validateFile(file multipart.File, header *multipart.FileHeader) error {
	if err := ValidateFileSize(header.Size); err != nil {
		return err
	}

	if err := ValidateFileExtension(header.Filename); err != nil {
		return err
	}

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err := ValidateFileContent(buffer[:n]); err != nil {
		return fmt.Errorf("file content does not match a supported image type: %w", err)
	}

	if err := ValidateFileSignature(buffer[:n]); err != nil {
		return fmt.Errorf("file signature verification failed: %w", err)
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	return nil
}

// generateFilename generates a unique filename based on hash
func (s *Service) generateFilename(hash string) string {
	return hash[:16] + ".webp"
}

// generateUniqueHash appends _1, _2, etc. to the hash until a unique one is found
func (s *Service) generateUniqueHash(ctx context.Context, hash string) (string, error) {
	uniqueHash := hash
	for i := 1; ; i++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		_, err := s.repo.FindByHash(ctx, uniqueHash)
		if err != nil {
			if errors.Is(err, ErrMediaNotFound) {
				return uniqueHash, nil
			}
			return "", fmt.Errorf("failed to check hash uniqueness: %w", err)
		}
		uniqueHash = fmt.Sprintf("%s_%d", hash, i)
	}
}

// GenerateDuplicateFilename appends -2 suffix before extension
func GenerateDuplicateFilename(filename string) string {
	idx := strings.LastIndex(filename, ".")
	if idx == -1 {
		return filename + "-2"
	}
	return filename[:idx] + "-2" + filename[idx:]
}

// Upload handles the complete media upload flow
func (s *Service) Upload(ctx context.Context, req UploadRequest) (*Media, error) {
	if err := s.validateFile(req.File, req.FileHeader); err != nil {
		return nil, fmt.Errorf("file validation failed: %w", err)
	}

	if err := ValidateAltText(req.AltText); err != nil {
		return nil, fmt.Errorf("alt text validation failed: %w", err)
	}

	_, _ = req.File.Seek(0, 0)

	hash, err := s.processor.GenerateHash(req.File)
	if err != nil {
		return nil, fmt.Errorf("failed to generate hash: %w", err)
	}

	_, _ = req.File.Seek(0, 0)

	existing, err := s.repo.FindByHash(ctx, hash)
	if err == nil && existing != nil {
		return nil, &DuplicateMediaError{Existing: existing}
	}

	_, _ = req.File.Seek(0, 0)

	webpData, metadata, err := s.processor.ConvertToWebP(req.File)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to WebP: %w", err)
	}

	filename := s.generateFilename(hash)
	filePath, err := s.storage.Save(filename, bytes.NewReader(webpData))
	if err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	originalFilename := SanitizeFilename(req.FileHeader.Filename)

	variants := make(map[string]MediaVariant)
	var savedVariantPaths []string
	if s.thumbnailService != nil {
		for _, tc := range s.thumbnailService.GetAll() {
			_, _ = req.File.Seek(0, 0)

			variantData, variantMeta, err := s.processor.Resize(req.File, tc.MaxWidth)
			if err != nil {
				for _, p := range savedVariantPaths {
					_ = s.storage.Delete(p)
				}
				_ = s.storage.Delete(filePath)
				return nil, fmt.Errorf("failed to generate variant %s: %w", tc.Suffix, err)
			}

			variantFilename := hash[:16] + tc.Suffix + ".webp"
			variantPath, err := s.storage.Save(variantFilename, bytes.NewReader(variantData))
			if err != nil {
				for _, p := range savedVariantPaths {
					_ = s.storage.Delete(p)
				}
				_ = s.storage.Delete(filePath)
				return nil, fmt.Errorf("failed to save variant %s: %w", tc.Suffix, err)
			}

			savedVariantPaths = append(savedVariantPaths, variantPath)

			variants[tc.Suffix] = MediaVariant{
				FilePath: variantPath,
				URL:      s.storage.GetURL(variantPath),
				Width:    variantMeta.Width,
				Height:   variantMeta.Height,
			}
		}
	}

	media := &Media{
		UserID:           req.UserID,
		Filename:         filename,
		OriginalFilename: originalFilename,
		MimeType:         MimeTypeWebP,
		FileSize:         int64(len(webpData)),
		Width:            metadata.Width,
		Height:           metadata.Height,
		AltText:          strings.TrimSpace(req.AltText),
		IsWebP:           true,
		FilePath:         filePath,
		URL:              s.storage.GetURL(filePath),
		Hash:             hash,
		Variants:         variants,
	}

	if err := s.repo.Create(ctx, media); err != nil {
		_ = s.storage.Delete(filePath)
		for _, p := range savedVariantPaths {
			_ = s.storage.Delete(p)
		}
		return nil, fmt.Errorf("failed to create media record: %w", err)
	}

	return media, nil
}

// ForceUpload handles media upload bypassing hash uniqueness check
func (s *Service) ForceUpload(ctx context.Context, req UploadRequest) (*Media, error) {
	if err := s.validateFile(req.File, req.FileHeader); err != nil {
		return nil, fmt.Errorf("file validation failed: %w", err)
	}

	if err := ValidateAltText(req.AltText); err != nil {
		return nil, fmt.Errorf("alt text validation failed: %w", err)
	}

	_, _ = req.File.Seek(0, 0)

	hash, err := s.processor.GenerateHash(req.File)
	if err != nil {
		return nil, fmt.Errorf("failed to generate hash: %w", err)
	}

	_, _ = req.File.Seek(0, 0)

	uniqueHash, err := s.generateUniqueHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to generate unique hash: %w", err)
	}

	_, _ = req.File.Seek(0, 0)

	webpData, metadata, err := s.processor.ConvertToWebP(req.File)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to WebP: %w", err)
	}

	filename := s.generateFilename(uniqueHash)
	filePath, err := s.storage.Save(filename, bytes.NewReader(webpData))
	if err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	originalFilename := SanitizeFilename(req.FileHeader.Filename)
	forceOriginalFilename := GenerateDuplicateFilename(originalFilename)

	variants := make(map[string]MediaVariant)
	var savedVariantPaths []string
	if s.thumbnailService != nil {
		for _, tc := range s.thumbnailService.GetAll() {
			_, _ = req.File.Seek(0, 0)

			variantData, variantMeta, err := s.processor.Resize(req.File, tc.MaxWidth)
			if err != nil {
				for _, p := range savedVariantPaths {
					_ = s.storage.Delete(p)
				}
				_ = s.storage.Delete(filePath)
				return nil, fmt.Errorf("failed to generate variant %s: %w", tc.Suffix, err)
			}

			variantFilename := uniqueHash[:16] + tc.Suffix + ".webp"
			variantPath, err := s.storage.Save(variantFilename, bytes.NewReader(variantData))
			if err != nil {
				for _, p := range savedVariantPaths {
					_ = s.storage.Delete(p)
				}
				_ = s.storage.Delete(filePath)
				return nil, fmt.Errorf("failed to save variant %s: %w", tc.Suffix, err)
			}

			savedVariantPaths = append(savedVariantPaths, variantPath)

			variants[tc.Suffix] = MediaVariant{
				FilePath: variantPath,
				URL:      s.storage.GetURL(variantPath),
				Width:    variantMeta.Width,
				Height:   variantMeta.Height,
			}
		}
	}

	media := &Media{
		UserID:           req.UserID,
		Filename:         filename,
		OriginalFilename: forceOriginalFilename,
		MimeType:         MimeTypeWebP,
		FileSize:         int64(len(webpData)),
		Width:            metadata.Width,
		Height:           metadata.Height,
		AltText:          strings.TrimSpace(req.AltText),
		IsWebP:           true,
		FilePath:         filePath,
		URL:              s.storage.GetURL(filePath),
		Hash:             uniqueHash,
		Variants:         variants,
	}

	if err := s.repo.Create(ctx, media); err != nil {
		_ = s.storage.Delete(filePath)
		for _, p := range savedVariantPaths {
			_ = s.storage.Delete(p)
		}
		return nil, fmt.Errorf("failed to create media record: %w", err)
	}

	return media, nil
}

// GetByID retrieves media by ID
func (s *Service) GetByID(ctx context.Context, id int) (*Media, error) {
	media, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get media: %w", err)
	}
	return media, nil
}

// GetAll retrieves all media
func (s *Service) GetAll(ctx context.Context, limit int, offset int) ([]*Media, error) {
	mediaList, err := s.repo.FindAll(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get all media: %w", err)
	}
	return mediaList, nil
}

// ListByCursor retrieves the caller's media in newest-first (id DESC) order using keyset
// pagination (beforeID <= 0 means first page). Thin pass-through mirroring GetAll; all
// pagination/encoding intelligence lives in the agent handler.
func (s *Service) ListByCursor(ctx context.Context, userID int, limit int, beforeID int) ([]*Media, error) {
	mediaList, err := s.repo.ListByCursor(ctx, userID, limit, beforeID)
	if err != nil {
		return nil, fmt.Errorf("failed to list media: %w", err)
	}
	return mediaList, nil
}

// GenerateFromBytes creates a media record from raw image bytes by converting to WebP,
// generating a hash, saving to storage, creating thumbnail variants, and storing in the database.
// This is used for AI-generated or programmatically-created images that don't come from file uploads.
func (s *Service) GenerateFromBytes(
	ctx context.Context,
	imageBytes []byte,
	userID int,
	altText string,
	originalFilename string,
) (*Media, error) {
	if err := ValidateAltText(altText); err != nil {
		return nil, fmt.Errorf("alt text validation failed: %w", err)
	}

	webpData, metadata, err := s.processor.ConvertToWebP(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to convert to WebP: %w", err)
	}

	hash, err := s.processor.GenerateHash(bytes.NewReader(webpData))
	if err != nil {
		return nil, fmt.Errorf("failed to generate hash: %w", err)
	}

	existing, err := s.repo.FindByHash(ctx, hash)
	if err == nil && existing != nil {
		return nil, &DuplicateMediaError{Existing: existing}
	}

	filename := s.generateFilename(hash)
	filePath, err := s.storage.Save(filename, bytes.NewReader(webpData))
	if err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	variants := make(map[string]MediaVariant)
	var savedVariantPaths []string
	if s.thumbnailService != nil {
		for _, tc := range s.thumbnailService.GetAll() {
			variantData, variantMeta, err := s.processor.Resize(bytes.NewReader(webpData), tc.MaxWidth)
			if err != nil {
				for _, p := range savedVariantPaths {
					_ = s.storage.Delete(p)
				}
				_ = s.storage.Delete(filePath)
				return nil, fmt.Errorf("failed to generate variant %s: %w", tc.Suffix, err)
			}

			variantFilename := hash[:16] + tc.Suffix + ".webp"
			variantPath, err := s.storage.Save(variantFilename, bytes.NewReader(variantData))
			if err != nil {
				for _, p := range savedVariantPaths {
					_ = s.storage.Delete(p)
				}
				_ = s.storage.Delete(filePath)
				return nil, fmt.Errorf("failed to save variant %s: %w", tc.Suffix, err)
			}

			savedVariantPaths = append(savedVariantPaths, variantPath)

			variants[tc.Suffix] = MediaVariant{
				FilePath: variantPath,
				URL:      s.storage.GetURL(variantPath),
				Width:    variantMeta.Width,
				Height:   variantMeta.Height,
			}
		}
	}

	sanitized := SanitizeFilename(originalFilename)

	media := &Media{
		UserID:           userID,
		Filename:         filename,
		OriginalFilename: sanitized,
		MimeType:         MimeTypeWebP,
		FileSize:         int64(len(webpData)),
		Width:            metadata.Width,
		Height:           metadata.Height,
		AltText:          strings.TrimSpace(altText),
		IsWebP:           true,
		FilePath:         filePath,
		URL:              s.storage.GetURL(filePath),
		Hash:             hash,
		Variants:         variants,
	}

	if err := s.repo.Create(ctx, media); err != nil {
		_ = s.storage.Delete(filePath)
		for _, p := range savedVariantPaths {
			_ = s.storage.Delete(p)
		}
		return nil, fmt.Errorf("failed to create media record: %w", err)
	}

	return media, nil
}

// Delete deletes media by ID. Admin users can delete any media; others can only delete their own.
func (s *Service) Delete(ctx context.Context, id int, userID int, userRole string) error {
	media, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find media: %w", err)
	}

	if userRole != constants.RoleAdmin {
		if media.UserID != userID {
			return ErrUnauthorized
		}

		if err := s.storage.Delete(media.FilePath); err != nil {
			return fmt.Errorf("failed to delete file: %w", err)
		}

		if err := s.repo.DeleteByOwner(ctx, id, userID); err != nil {
			return fmt.Errorf("failed to delete media: %w", err)
		}

		return nil
	}

	if err := s.storage.Delete(media.FilePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return fmt.Errorf("failed to delete media: %w", err)
	}

	return nil
}

// SearchMedia retrieves media with optional search and date filter
func (s *Service) SearchMedia(
	ctx context.Context,
	search string,
	dateFilter string,
	limit int,
	offset int,
) ([]*Media, error) {
	trimmed := strings.TrimSpace(search)
	hasSearch := trimmed != ""

	since, hasDateFilter := ParseDateFilter(dateFilter)

	var result []*Media
	var err error

	switch {
	case hasSearch && hasDateFilter:
		result, err = s.repo.FindAllByFilenameAndDateRange(ctx, trimmed, since, limit, offset)
	case hasSearch:
		result, err = s.repo.FindAllByFilename(ctx, trimmed, limit, offset)
	case hasDateFilter:
		result, err = s.repo.FindAllByDateRange(ctx, since, limit, offset)
	default:
		result, err = s.repo.FindAll(ctx, limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to search media: %w", err)
	}
	return result, nil
}

// NewService creates a new media service
func NewService(
	repo Repository,
	storage Storage,
	thumbnailService *thumbnail.Service,
) *Service {
	return &Service{
		repo:             repo,
		storage:          storage,
		processor:        NewProcessor(),
		thumbnailService: thumbnailService,
	}
}
