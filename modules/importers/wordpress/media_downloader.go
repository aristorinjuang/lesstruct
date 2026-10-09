package wordpress

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
)

const (
	maxRetries        = 3
	initialBackoff    = 500 * time.Millisecond
	backoffMultiplier = 2
)

// validateImageURL guards against SSRF: the WordPress importer fetches URLs
// from operator-uploaded WXR files, so the URL must be http(s) and its host
// must not resolve to loopback, private, or link-local addresses (cloud
// metadata endpoints, internal services).
func validateImageURL(imageURL string, allowPrivate bool) error {
	u, err := url.Parse(imageURL)
	if err != nil {
		return fmt.Errorf("invalid image URL %q: %w", imageURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("image URL %q must use http or https", imageURL)
	}
	if allowPrivate {
		return nil
	}

	ips, err := net.LookupIP(u.Hostname())
	if err != nil {
		return fmt.Errorf("cannot resolve image host %q: %w", u.Hostname(), err)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("image URL %q resolves to a forbidden address", imageURL)
		}
	}
	return nil
}

// mediaService is the subset of the media domain service used to re-upload
// downloaded WordPress images. Keeping it narrow avoids coupling the importer
// to the full media.Service surface.
type mediaService interface {
	GenerateFromBytes(
		ctx context.Context,
		imageBytes []byte,
		principal kernel.Principal,
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

// isRetryableError reports whether the error from an HTTP download is worth
// retrying. Transient network errors (timeouts, connection resets, 5xx) are
// retryable. Fatal context signals (Canceled, DeadlineExceeded) are not — the
// job is shutting down so there is no point retrying.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	return true
}

// isRetryableStatusCode returns true for server errors (5xx) and rate
// limiting (429) that may succeed on retry.
func isRetryableStatusCode(code int) bool {
	return code >= 500 || code == http.StatusTooManyRequests
}

// MediaDownloader downloads images from a WordPress site and re-uploads them
// through the media service (which converts to WebP, deduplicates by hash, and
// generates thumbnails). Results are cached per URL so each image is fetched at
// most once per import. A perceptual hash is computed from the downloaded
// bytes so callers can detect visually identical images that content-hash
// dedup cannot see.
type MediaDownloader struct {
	mu           sync.Mutex
	httpClient   *http.Client
	mediaService mediaService
	cache        map[string]string // WordPress URL -> local media URL
	failed       map[string]struct{}
	phashes      map[string]uint64 // WordPress URL -> perceptual hash of the image bytes
	allowPrivate bool              // test hook: skip SSRF host validation
}

// PHash returns the perceptual hash recorded for the downloaded image URL and
// whether one is known — hashes are only available for URLs that were
// successfully downloaded through this downloader.
func (d *MediaDownloader) PHash(imageURL string) (uint64, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	hash, ok := d.phashes[imageURL]
	return hash, ok
}

// download fetches the image and re-uploads it through the media service,
// with retries and backoff.
func (d *MediaDownloader) download(ctx context.Context, imageURL string, userID int) (string, error) {
	if err := validateImageURL(imageURL, d.allowPrivate); err != nil {
		return "", err
	}

	var lastErr error
	backoff := initialBackoff

	for attempt := range maxRetries + 1 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("context canceled before retry %d: %w", attempt, ctx.Err())
			case <-time.After(backoff):
			}
			backoff *= backoffMultiplier
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
		if err != nil {
			return "", fmt.Errorf("invalid image URL %q: %w", imageURL, err)
		}

		resp, err := d.httpClient.Do(req)
		if err != nil {
			if !isRetryableError(err) || attempt == maxRetries {
				return "", fmt.Errorf("failed to download image %q: %w", imageURL, err)
			}
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			if isRetryableStatusCode(resp.StatusCode) && attempt < maxRetries {
				lastErr = fmt.Errorf("download %q returned status %d", imageURL, resp.StatusCode)
				continue
			}
			return "", fmt.Errorf("download %q returned status %d", imageURL, resp.StatusCode)
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, mediadomain.MaxFileSize))
		_ = resp.Body.Close()
		if err != nil {
			if attempt == maxRetries {
				return "", fmt.Errorf("failed to read image body %q: %w", imageURL, err)
			}
			lastErr = err
			continue
		}

		if !isImageContent(body) {
			return "", fmt.Errorf("downloaded content from %q is not a supported image", imageURL)
		}

		if hash, hashErr := mediadomain.PerceptualHash(bytes.NewReader(body)); hashErr == nil {
			d.mu.Lock()
			d.phashes[imageURL] = hash
			d.mu.Unlock()
		}

		media, err := d.mediaService.GenerateFromBytes(ctx, body, kernel.Principal{ID: strconv.Itoa(userID), Roles: []string{}}, altTextFromURL(imageURL), filenameFromURL(imageURL))
		if err != nil {
			var dupErr *mediadomain.DuplicateMediaError
			if errors.As(err, &dupErr) && dupErr.Existing != nil {
				return dupErr.Existing.URL, nil
			}
			return "", fmt.Errorf("failed to re-upload image %q: %w", imageURL, err)
		}

		return media.URL, nil
	}

	return "", lastErr
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

	d.mu.Lock()
	if local, ok := d.cache[imageURL]; ok {
		d.mu.Unlock()
		return local, nil
	}
	if _, ok := d.failed[imageURL]; ok {
		d.mu.Unlock()
		return "", nil
	}
	d.mu.Unlock()

	local, err := d.download(ctx, imageURL, userID)
	if err != nil {
		d.mu.Lock()
		d.failed[imageURL] = struct{}{}
		d.mu.Unlock()
		return "", err
	}

	d.mu.Lock()
	d.cache[imageURL] = local
	d.mu.Unlock()
	return local, nil
}

// NewMediaDownloader creates a downloader. If httpClient is nil a default client
// with a 30-second timeout is used. By default image hosts are validated
// against SSRF (http/https only, no loopback/private/link-local addresses);
// tests pass MediaDownloaderAllowPrivateHosts to target httptest servers on
// loopback.
func NewMediaDownloader(
	httpClient *http.Client,
	mediaService mediaService,
	opts ...MediaDownloaderOption,
) *MediaDownloader {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	d := &MediaDownloader{
		httpClient:   httpClient,
		mediaService: mediaService,
		cache:        make(map[string]string),
		failed:       make(map[string]struct{}),
		phashes:      make(map[string]uint64),
		allowPrivate: false,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// MediaDownloaderOption configures a MediaDownloader.
type MediaDownloaderOption func(*MediaDownloader)

// MediaDownloaderAllowPrivateHosts disables the SSRF host validation so tests
// can target loopback httptest servers. Never use in production wiring.
func MediaDownloaderAllowPrivateHosts() MediaDownloaderOption {
	return func(d *MediaDownloader) {
		d.allowPrivate = true
	}
}
