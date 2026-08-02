package hugo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/aristorinjuang/lesstruct/internal/content/wordpress"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
)

// MediaService is the subset of the media domain service used to re-upload
// Hugo images (both local static/ files and remote URLs). It mirrors the
// wordpress package's mediaService interface so the same concrete service can
// be shared.
type MediaService interface {
	GenerateFromBytes(
		ctx context.Context,
		imageBytes []byte,
		userID int,
		altText string,
		originalFilename string,
	) (*mediadomain.Media, error)
}

// imageSrcRe matches an <img> tag and captures its src attribute value.
var imageSrcRe = regexp.MustCompile(`<img\b[^>]*\bsrc="([^"]*)"[^>]*>`)

// isRemoteURL reports whether the reference is an absolute http(s) URL.
func isRemoteURL(ref string) bool {
	parsed, err := url.Parse(ref)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

// isSupportedImagePath reports whether the path looks like a supported image
// file (matching the media domain's accepted extensions).
func isSupportedImagePath(ref string) bool {
	lower := strings.ToLower(ref)
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// MediaMapper migrates images referenced by Hugo content into Lesstruct media.
// Local references (e.g. "/images/foo.jpg") are resolved against the extracted
// static/ directory; remote http(s) URLs are downloaded through the shared
// WordPress media downloader. Results are cached per reference so each image is
// processed at most once per import.
type MediaMapper struct {
	mu         sync.Mutex
	staticDir  string
	service    MediaService
	downloader *wordpress.MediaDownloader
	cache      map[string]string // reference -> local media URL
	failed     map[string]struct{}
	skipMedia  bool
}

// localPath resolves a content reference against the static dir. References are
// root-relative ("/images/foo.jpg") or bare ("images/foo.jpg"); the leading
// slash is stripped before joining. Path traversal is rejected.
func (m *MediaMapper) localPath(ref string) (string, bool) {
	rel := strings.TrimPrefix(ref, "/")
	if rel == "" {
		return "", false
	}
	abs := filepath.Join(m.staticDir, filepath.Clean(rel))
	staticAbs, err := filepath.Abs(m.staticDir)
	if err != nil {
		return "", false
	}
	absClean := filepath.Clean(abs)
	if absClean != staticAbs && !strings.HasPrefix(absClean, staticAbs+string(os.PathSeparator)) {
		return "", false
	}
	return absClean, true
}

// Map resolves a single image reference to its Lesstruct media URL. Local files
// are read from the static dir and re-uploaded via GenerateFromBytes (WebP
// transcode + hash dedup); remote URLs go through the shared downloader. When
// skipMedia is set, or the reference is not an image, the original reference is
// returned unchanged.
func (m *MediaMapper) Map(ctx context.Context, ref string, userID int) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", nil
	}
	if !isSupportedImagePath(ref) {
		return ref, nil
	}

	m.mu.Lock()
	if local, ok := m.cache[ref]; ok {
		m.mu.Unlock()
		return local, nil
	}
	if _, ok := m.failed[ref]; ok {
		m.mu.Unlock()
		return ref, nil
	}
	m.mu.Unlock()

	if m.skipMedia {
		return ref, nil
	}

	var local string
	var err error

	if isRemoteURL(ref) {
		if m.downloader == nil {
			return ref, nil
		}
		local, err = m.downloader.DownloadAndUpload(ctx, ref, userID)
	} else {
		path, ok := m.localPath(ref)
		if !ok {
			return ref, nil
		}
		local, err = m.uploadLocal(ctx, path, ref, userID)
	}

	if err != nil {
		m.mu.Lock()
		m.failed[ref] = struct{}{}
		m.mu.Unlock()
		return ref, err
	}

	if local == "" {
		return ref, nil
	}

	m.mu.Lock()
	m.cache[ref] = local
	m.mu.Unlock()
	return local, nil
}

// uploadLocal reads a local static file and re-uploads it via the media
// service, returning the new media URL. Returns "" with a nil error when the
// file is missing (the caller keeps the original reference).
func (m *MediaMapper) uploadLocal(ctx context.Context, path string, ref string, userID int) (string, error) {
	if m.service == nil {
		return "", nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to open static image %q: %w", ref, err)
	}
	defer func() { _ = file.Close() }()

	body, err := io.ReadAll(io.LimitReader(file, mediadomain.MaxFileSize))
	if err != nil {
		return "", fmt.Errorf("failed to read static image %q: %w", ref, err)
	}

	media, err := m.service.GenerateFromBytes(ctx, body, userID, altTextFromRef(ref), filepath.Base(path))
	if err != nil {
		var dupErr *mediadomain.DuplicateMediaError
		if errors.As(err, &dupErr) && dupErr.Existing != nil {
			return dupErr.Existing.URL, nil
		}
		return "", fmt.Errorf("failed to re-upload static image %q: %w", ref, err)
	}
	return media.URL, nil
}

// RewriteBody replaces the src attribute of every <img> tag in the body with
// the mapped media URL. References that cannot be mapped stay unchanged.
func (m *MediaMapper) RewriteBody(ctx context.Context, body string, userID int) string {
	return imageSrcRe.ReplaceAllStringFunc(body, func(tag string) string {
		match := imageSrcRe.FindStringSubmatch(tag)
		if len(match) != 2 {
			return tag
		}
		mapped, err := m.Map(ctx, match[1], userID)
		if err != nil || mapped == "" {
			return tag
		}
		return strings.Replace(tag, match[1], mapped, 1)
	})
}

// altTextFromRef derives a non-empty alt text from the reference's filename;
// the media service rejects empty alt text.
func altTextFromRef(ref string) string {
	name := ref
	if parsed, err := url.Parse(ref); err == nil && parsed.Path != "" {
		name = parsed.Path
	}
	name = strings.TrimRight(name, "/")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if dot := strings.LastIndex(name, "."); dot > 0 {
		name = name[:dot]
	}
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.TrimSpace(name)
	if name == "" {
		return "Imported from Hugo"
	}
	return name
}

// NewMediaMapper creates a MediaMapper. staticDir is the extracted static/
// directory (may be empty when the archive has none). service re-uploads image
// bytes; downloader handles remote URLs (a nil downloader leaves remote
// references hotlinked). skipMedia disables all media migration.
func NewMediaMapper(
	staticDir string,
	service MediaService,
	downloader *wordpress.MediaDownloader,
	skipMedia bool,
) *MediaMapper {
	return &MediaMapper{
		staticDir:  staticDir,
		service:    service,
		downloader: downloader,
		cache:      make(map[string]string),
		failed:     make(map[string]struct{}),
		skipMedia:  skipMedia,
	}
}
