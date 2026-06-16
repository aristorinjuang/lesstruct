package wordpress

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
)

// mediaService is the subset of the media domain service used to re-upload
// downloaded WordPress images. Keeping it narrow avoids coupling the importer
// to the full media.Service surface.
type mediaService interface {
	GenerateFromBytes(
		ctx context.Context,
		imageBytes []byte,
		userID int,
		altText string,
		originalFilename string,
	) (*mediadomain.Media, error)
}

// isImageURL reports whether a URL's path looks like a supported image file.
// Audio, video, and document URLs are skipped so we only download images.
func isImageURL(imageURL string) bool {
	parsed, err := url.Parse(imageURL)
	if err != nil {
		return false
	}
	path := strings.ToLower(parsed.Path)
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp"} {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// isImageContent validates downloaded bytes via the media domain's signature
// check, which inspects magic bytes regardless of file extension.
func isImageContent(body []byte) bool {
	return mediadomain.ValidateFileSignature(body) == nil
}

// altTextFromURL derives a non-empty alt text from the URL filename; the media
// service rejects empty alt text.
func altTextFromURL(imageURL string) string {
	name := filenameFromURL(imageURL)
	if dot := strings.LastIndex(name, "."); dot > 0 {
		name = name[:dot]
	}
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.TrimSpace(name)
	if name == "" {
		return "Imported from WordPress"
	}
	return name
}

// filenameFromURL extracts the last path segment of a URL.
func filenameFromURL(imageURL string) string {
	parsed, err := url.Parse(imageURL)
	if err != nil {
		return "wordpress-import"
	}
	path := strings.TrimRight(parsed.Path, "/")
	if path == "" {
		return "wordpress-import"
	}
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// MediaDownloader downloads images from a WordPress site and re-uploads them
// through the media service (which converts to WebP, deduplicates by hash, and
// generates thumbnails). Results are cached per URL so each image is fetched at
// most once per import.
type MediaDownloader struct {
	httpClient   *http.Client
	mediaService mediaService
	cache        map[string]string // WordPress URL -> local media URL
	failed       map[string]struct{}
}

func (d *MediaDownloader) download(ctx context.Context, imageURL string, userID int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("invalid image URL %q: %w", imageURL, err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download image %q: %w", imageURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %q returned status %d", imageURL, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, mediadomain.MaxFileSize))
	if err != nil {
		return "", fmt.Errorf("failed to read image body %q: %w", imageURL, err)
	}
	if !isImageContent(body) {
		return "", fmt.Errorf("downloaded content from %q is not a supported image", imageURL)
	}

	media, err := d.mediaService.GenerateFromBytes(ctx, body, userID, altTextFromURL(imageURL), filenameFromURL(imageURL))
	if err != nil {
		var dupErr *mediadomain.DuplicateMediaError
		if errors.As(err, &dupErr) && dupErr.Existing != nil {
			return dupErr.Existing.URL, nil
		}
		return "", fmt.Errorf("failed to re-upload image %q: %w", imageURL, err)
	}

	return media.URL, nil
}

// DownloadAndUpload fetches the image at imageURL and re-uploads it to local
// storage. Returns the local media URL. Non-image URLs and failed downloads
// return an empty string and nil error so the caller can fall back to the
// original WordPress URL without aborting the import.
func (d *MediaDownloader) DownloadAndUpload(ctx context.Context, imageURL string, userID int) (string, error) {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" || !isImageURL(imageURL) {
		return "", nil
	}

	if local, ok := d.cache[imageURL]; ok {
		return local, nil
	}
	if _, ok := d.failed[imageURL]; ok {
		return "", nil
	}

	local, err := d.download(ctx, imageURL, userID)
	if err != nil {
		d.failed[imageURL] = struct{}{}
		return "", err
	}

	d.cache[imageURL] = local
	return local, nil
}

// NewMediaDownloader creates a downloader. If httpClient is nil a default client
// with a 30-second timeout is used.
func NewMediaDownloader(httpClient *http.Client, mediaService mediaService) *MediaDownloader {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &MediaDownloader{
		httpClient:   httpClient,
		mediaService: mediaService,
		cache:        make(map[string]string),
		failed:       make(map[string]struct{}),
	}
}
