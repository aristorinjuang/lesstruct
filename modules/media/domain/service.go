package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	kernelstorage "github.com/aristorinjuang/lesstruct/kernel/storage"
	thumbnail "github.com/aristorinjuang/lesstruct/modules/media/domain/thumbnail"
)

// adminRoleName is the reserved administrator role. It matches the users
// module's reserved name; the two reconcile when that module lands.
const adminRoleName = "Admin"

// now is the clock seam: tests pin it to cover weekday and month boundaries.
var now = time.Now

// principalUserID parses the principal's decimal ID for repository calls.
// adminScopeUserID resolves the list scope for a media library request:
// admins manage the whole site's media (owner 0 = no filter), every other
// role is scoped to their own library so one account cannot inventory
// another's uploads, hashes, and original filenames.
func adminScopeUserID(principal kernel.Principal) (int, error) {
	if principal.HasRole(adminRoleName) {
		return 0, nil
	}
	return principalUserID(principal)
}

func principalUserID(principal kernel.Principal) (int, error) {
	id, err := strconv.Atoi(principal.ID)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", kernel.ErrInvalidPrincipalID, principal.ID)
	}
	return id, nil
}

// principalRole returns the principal's first role, or "" when anonymous.
func principalRole(principal kernel.Principal) string {
	if len(principal.Roles) == 0 {
		return ""
	}
	return principal.Roles[0]
}

// ParseDateFilter converts a predefined date filter string to a time threshold
func ParseDateFilter(dateFilter string) (time.Time, bool) {
	at := now()
	switch dateFilter {
	case "today":
		return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, at.Location()), true
	case "this_week":
		weekday := int(at.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return time.Date(at.Year(), at.Month(), at.Day()-weekday+1, 0, 0, 0, 0, at.Location()), true
	case "this_month":
		return time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, at.Location()), true
	default:
		return time.Time{}, false
	}
}

// persistMedia saves the WebP payload, generates every configured thumbnail
// variant, and creates the media record. A failure at any step deletes the
// files already written, so storage never keeps orphaned objects. The variant
// source is a factory because uploaded files must be rewound between reads
// while in-memory payloads start a fresh reader each time.
func (s *Service) persistMedia(
	ctx context.Context,
	media *Media,
	webpData []byte,
	variantSource func() io.Reader,
) error {
	filePath, err := s.storage.Save(media.Filename, bytes.NewReader(webpData))
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	variants := make(map[string]MediaVariant)
	savedVariantPaths := make([]string, 0, 2)
	if s.thumbnailService != nil {
		for _, tc := range s.thumbnailService.GetAll() {
			variantData, variantMeta, err := s.processor.Resize(variantSource(), tc.MaxWidth)
			if err != nil {
				s.discardSaved(filePath, savedVariantPaths)
				return fmt.Errorf("failed to generate variant %s: %w", tc.Suffix, err)
			}

			variantFilename := media.Hash[:16] + tc.Suffix + ".webp"
			variantPath, err := s.storage.Save(variantFilename, bytes.NewReader(variantData))
			if err != nil {
				s.discardSaved(filePath, savedVariantPaths)
				return fmt.Errorf("failed to save variant %s: %w", tc.Suffix, err)
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

	media.FilePath = filePath
	media.URL = s.storage.GetURL(filePath)
	media.Variants = variants

	if err := s.repo.Create(ctx, media); err != nil {
		s.discardSaved(filePath, savedVariantPaths)
		return fmt.Errorf("failed to create media record: %w", err)
	}
	return nil
}

// discardSaved best-effort deletes the files written before a failed save or
// insert: the caller reports the original failure, not a cleanup miss.
func (s *Service) discardSaved(filePath string, variantPaths []string) {
	_ = s.storage.Delete(filePath)
	for _, path := range variantPaths {
		_ = s.storage.Delete(path)
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
	storage          kernelstorage.Storage
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

	// ValidateFileContent already checks the magic-number signature of every
	// supported format (including WebP, which DetectContentType cannot
	// identify), so a second signature pass could never fail here.
	if err := ValidateFileContent(buffer[:n]); err != nil {
		return fmt.Errorf("file content does not match a supported image type: %w", err)
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

	media := &Media{
		UserID:           req.UserID,
		Filename:         s.generateFilename(hash),
		OriginalFilename: SanitizeFilename(req.FileHeader.Filename),
		MimeType:         MimeTypeWebP,
		FileSize:         int64(len(webpData)),
		Width:            metadata.Width,
		Height:           metadata.Height,
		AltText:          strings.TrimSpace(req.AltText),
		IsWebP:           true,
		Hash:             hash,
	}

	if err := s.persistMedia(ctx, media, webpData, func() io.Reader {
		_, _ = req.File.Seek(0, 0)
		return req.File
	}); err != nil {
		return nil, err
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

	media := &Media{
		UserID:           req.UserID,
		Filename:         s.generateFilename(uniqueHash),
		OriginalFilename: GenerateDuplicateFilename(SanitizeFilename(req.FileHeader.Filename)),
		MimeType:         MimeTypeWebP,
		FileSize:         int64(len(webpData)),
		Width:            metadata.Width,
		Height:           metadata.Height,
		AltText:          strings.TrimSpace(req.AltText),
		IsWebP:           true,
		Hash:             uniqueHash,
	}

	if err := s.persistMedia(ctx, media, webpData, func() io.Reader {
		_, _ = req.File.Seek(0, 0)
		return req.File
	}); err != nil {
		return nil, err
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

// ListByCursor retrieves the caller's media in newest-first (id DESC) order using keyset
// pagination (beforeID <= 0 means first page). Thin pass-through; all pagination/encoding
// intelligence lives in the agent handler.
func (s *Service) ListByCursor(ctx context.Context, userID int, limit int, beforeID int) ([]*Media, error) {
	mediaList, err := s.repo.ListByCursor(ctx, userID, limit, beforeID)
	if err != nil {
		return nil, fmt.Errorf("failed to list media: %w", err)
	}
	return mediaList, nil
}

// SearchMediaByCursor retrieves media with optional search and date filters in
// newest-first (id DESC) order using keyset pagination (beforeID <= 0 means
// first page). Admins search every user's media; any other role is scoped to
// their own library. All pagination/encoding intelligence lives in the browser
// handler.
func (s *Service) SearchMediaByCursor(
	ctx context.Context,
	principal kernel.Principal,
	search string,
	dateFilter string,
	limit int,
	beforeID int,
) ([]*Media, error) {
	ownerID, err := adminScopeUserID(principal)
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(search)
	hasSearch := trimmed != ""

	since, hasDateFilter := ParseDateFilter(dateFilter)

	var result []*Media

	switch {
	case hasSearch && hasDateFilter:
		result, err = s.repo.FindAllByFilenameAndDateRangeByCursor(ctx, ownerID, trimmed, since, limit, beforeID)
	case hasSearch:
		result, err = s.repo.FindAllByFilenameByCursor(ctx, ownerID, trimmed, limit, beforeID)
	case hasDateFilter:
		result, err = s.repo.FindAllByDateRangeByCursor(ctx, ownerID, since, limit, beforeID)
	default:
		result, err = s.repo.FindAllByCursor(ctx, ownerID, limit, beforeID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to search media: %w", err)
	}
	return result, nil
}

// Count returns the total number of media matching the given search and date
// filters under the principal's scope — admins count the whole library, any
// other role counts their own. Used to render the total badge next to the
// browser admin media library's title.
func (s *Service) Count(ctx context.Context, principal kernel.Principal, search string, dateFilter string) (int, error) {
	ownerID, err := adminScopeUserID(principal)
	if err != nil {
		return 0, err
	}

	trimmed := strings.TrimSpace(search)

	since, hasDateFilter := ParseDateFilter(dateFilter)
	if !hasDateFilter {
		since = time.Time{}
	}

	total, err := s.repo.Count(ctx, ownerID, trimmed, since)
	if err != nil {
		return 0, fmt.Errorf("failed to count media: %w", err)
	}
	return total, nil
}

// GenerateFromBytes creates a media record from raw image bytes by converting to WebP,
// generating a hash, saving to storage, creating thumbnail variants, and storing in the database.
// This is used for AI-generated or programmatically-created images that don't come from file uploads.
func (s *Service) GenerateFromBytes(
	ctx context.Context,
	imageBytes []byte,
	principal kernel.Principal,
	altText string,
	originalFilename string,
) (*Media, error) {
	userID, err := principalUserID(principal)
	if err != nil {
		return nil, err
	}
	if err := ValidateAltText(altText); err != nil {
		return nil, fmt.Errorf("alt text validation failed: %w", err)
	}

	webpData, metadata, err := s.processor.ConvertToWebP(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to convert to WebP: %w", err)
	}

	// The payload is already in memory, so hashing it cannot fail.
	hash, _ := s.processor.GenerateHash(bytes.NewReader(webpData))

	existing, err := s.repo.FindByHash(ctx, hash)
	if err == nil && existing != nil {
		return nil, &DuplicateMediaError{Existing: existing}
	}

	media := &Media{
		UserID:           userID,
		Filename:         s.generateFilename(hash),
		OriginalFilename: SanitizeFilename(originalFilename),
		MimeType:         MimeTypeWebP,
		FileSize:         int64(len(webpData)),
		Width:            metadata.Width,
		Height:           metadata.Height,
		AltText:          strings.TrimSpace(altText),
		IsWebP:           true,
		Hash:             hash,
	}

	if err := s.persistMedia(ctx, media, webpData, func() io.Reader {
		return bytes.NewReader(webpData)
	}); err != nil {
		return nil, err
	}

	return media, nil
}

// Delete deletes media by ID. Admin users can delete any media; others can only delete their own.
func (s *Service) Delete(ctx context.Context, id int, principal kernel.Principal) error {
	userID, err := principalUserID(principal)
	if err != nil {
		return err
	}
	role := principalRole(principal)
	media, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find media: %w", err)
	}

	if role != adminRoleName {
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

// NewService creates a new media service
func NewService(
	repo Repository,
	storage kernelstorage.Storage,
	thumbnailService *thumbnail.Service,
) *Service {
	return &Service{
		repo:             repo,
		storage:          storage,
		processor:        NewProcessor(),
		thumbnailService: thumbnailService,
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
