package hugo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

// featuredCheckLimit is the number of body <img> tags examined when deciding
// whether prepending the featured image would show the same picture twice.
const featuredCheckLimit = 3

// Reasons reported by featuredDuplicate.
const (
	// dupReasonNone: no duplicate — the featured image should be prepended.
	dupReasonNone = ""
	// dupReasonExact: a leading body image already carries the exact same URL.
	dupReasonExact = "exact"
	// dupReasonVisual: a leading body image is perceptually the same picture
	// under a different URL (different export, size, or query string).
	dupReasonVisual = "visual"
)

// staticRefRe matches href/src attributes on known HTML elements with a
// root-relative value, so non-image references (links, iframe demos,
// stylesheets, scripts) can be pointed at the theme's static/ dir. The
// element-anchored pattern avoids rewriting string literals inside inline
// <script>/<pre> text, and the attribute must be whitespace-preceded so
// data-src/data-href stay untouched. <img> src is handled by RewriteBody and
// only reaches this pass when it could not be mapped.
var staticRefRe = regexp.MustCompile(`(?i)<(a|iframe|link|script|source|img)\b[^>]*[\s"](href|src)="(/[^"]*)"`)

// leadingImageSrcs returns the src values of the body's first limit <img>
// tags, in document order.
func leadingImageSrcs(body string, limit int) []string {
	matches := imageSrcRe.FindAllStringSubmatch(body, limit)
	srcs := make([]string, 0, len(matches))
	for _, match := range matches {
		srcs = append(srcs, match[1])
	}
	return srcs
}

// featuredDuplicate reports whether prepending the mapped featured image would
// render the same picture twice at the top of the body. The body's first
// featuredCheckLimit images are checked: an exact URL match (the cover already
// appears among them) wins silently; otherwise a perceptual-hash comparison
// catches the same picture under a different URL — a re-upload, resized
// export, or hotlink variant that content-hash dedup cannot see. Images whose
// perceptual hash is unknown (unmappable references, undecodable files) fall
// back to the exact-URL check only.
func featuredDuplicate(body string, featuredURL string, mapper *MediaMapper) (string, string) {
	srcs := leadingImageSrcs(body, featuredCheckLimit)
	for _, src := range srcs {
		if src == featuredURL {
			return dupReasonExact, src
		}
	}

	featuredHash, ok := mapper.PHashFor(featuredURL)
	if !ok {
		return dupReasonNone, ""
	}
	for _, src := range srcs {
		hash, ok := mapper.PHashFor(src)
		if ok && mediadomain.PerceptuallySimilar(featuredHash, hash) {
			return dupReasonVisual, src
		}
	}
	return dupReasonNone, ""
}

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

// failure records a reference whose migration failed, including what the body
// was left pointing at (the /static/ fallback when available, else the
// original reference) so retries return the same value as the first call.
type failure struct {
	reason string
	url    string
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
	cache      map[string]string   // reference -> local media URL
	failed     map[string]failure  // reference -> migration failure
	phashes    map[string]uint64   // mapped media URL -> perceptual hash of the image bytes
	reported   map[string]struct{} // references already surfaced as warnings
	skipMedia  bool
}

// staticURL returns the documented /static/<path> URL for a reference that
// resolves to a file under the extracted static/ dir. The operator mirrors
// their Hugo static/ into the theme's static/ so these references serve. The
// path is resolved without query/fragment; the original suffix is preserved
// in the returned URL. Returns "" when the reference does not resolve to an
// existing static file.
func (m *MediaMapper) staticURL(ref string) string {
	path := ref
	suffix := ""
	if parsed, err := url.Parse(ref); err == nil && parsed.Path != "" {
		path = parsed.Path
		if parsed.RawQuery != "" {
			suffix = "?" + parsed.RawQuery
		}
		if parsed.Fragment != "" {
			suffix += "#" + parsed.Fragment
		}
	}

	abs, ok := m.localPath(path)
	if !ok {
		return ""
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return ""
	}
	rel := filepath.ToSlash(filepath.Clean(strings.TrimPrefix(path, "/")))
	return "/static/" + rel + suffix
}

// recordFailure notes a reference whose media migration failed, along with
// the URL the body was left pointing at; the importer surfaces these in the
// job's errors list instead of failing silently.
func (m *MediaMapper) recordFailure(ref string, reason string, url string) {
	m.mu.Lock()
	m.failed[ref] = failure{reason: reason, url: url}
	m.mu.Unlock()
}

// IsFailed reports whether the reference's migration failed.
func (m *MediaMapper) IsFailed(ref string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.failed[ref]
	return ok
}

// PHashFor returns the perceptual hash recorded for a mapped media URL and
// whether one is known. Hashes are recorded when the image is ingested —
// re-uploaded from the archive, downloaded from a remote URL, or served from
// its /static/ copy under skipMedia.
func (m *MediaMapper) PHashFor(mediaURL string) (uint64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hash, ok := m.phashes[mediaURL]
	return hash, ok
}

// recordPHash stores the perceptual hash computed for a mapped media URL.
func (m *MediaMapper) recordPHash(mediaURL string, hash uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.phashes[mediaURL] = hash
}

// hashStaticFile records the perceptual hash of an image served from its
// /static/ copy so duplicate detection also works without media migration.
// Failures are silently ignored — hashing is best-effort.
func (m *MediaMapper) hashStaticFile(ref string, staticURL string) {
	path, ok := m.localPath(ref)
	if !ok {
		return
	}
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	hash, err := mediadomain.PerceptualHash(file)
	if err == nil {
		m.recordPHash(staticURL, hash)
	}
}

// Failure describes one reference the importer should surface as a warning.
type Failure struct {
	Ref    string
	Reason string
	URL    string
}

// TakeUnreportedFailures returns the failures not yet reported (sorted by
// reference) and marks them reported, so each failure is warned exactly once
// across the whole import.
func (m *MediaMapper) TakeUnreportedFailures() []Failure {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Failure
	for ref, f := range m.failed {
		if _, ok := m.reported[ref]; ok {
			continue
		}
		out = append(out, Failure{Ref: ref, Reason: f.reason, URL: f.url})
	}
	slices.SortFunc(out, func(a, b Failure) int {
		return strings.Compare(a.Ref, b.Ref)
	})
	for _, f := range out {
		m.reported[f.Ref] = struct{}{}
	}
	return out
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
// are read from the static dir and re-uploaded via GenerateFromBytes; remote
// URLs go through the shared downloader. A reference whose migration fails is
// recorded (IsFailed/TakeUnreportedFailures) and mapped to its /static/ copy
// when the file exists under the extracted static dir, otherwise kept at the
// original reference — the same value is returned on every retry.
func (m *MediaMapper) Map(ctx context.Context, ref string, userID int) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if !isSupportedImagePath(ref) {
		return ref
	}

	m.mu.Lock()
	if local, ok := m.cache[ref]; ok {
		m.mu.Unlock()
		return local
	}
	if f, ok := m.failed[ref]; ok {
		m.mu.Unlock()
		return f.url
	}
	m.mu.Unlock()

	if m.skipMedia {
		// No media upload: keep the reference when it is remote, or point it
		// at the theme's static/ dir when it resolves to an extracted
		// static/ file (the documented mirror convention).
		if staticURL := m.staticURL(ref); staticURL != "" {
			m.hashStaticFile(ref, staticURL)
			return staticURL
		}
		m.recordFailure(ref, "media migration skipped (skipMedia) and no file under the archive's static/ dir", ref)
		return ref
	}

	var local string
	var err error

	if isRemoteURL(ref) {
		if m.downloader == nil {
			return ref
		}
		local, err = m.downloader.DownloadAndUpload(ctx, ref, userID)
		if err == nil && local != "" {
			if hash, ok := m.downloader.PHash(ref); ok {
				m.recordPHash(local, hash)
			}
		}
	} else {
		path, ok := m.localPath(ref)
		if !ok {
			return ref
		}
		local, err = m.uploadLocal(ctx, path, ref, userID)
	}

	if err != nil {
		// The migration failed; fall back to the /static/ copy when the file
		// exists under the extracted static/ dir, and always record the
		// failure so the import job surfaces it.
		reason := err.Error()
		if staticURL := m.staticURL(ref); staticURL != "" {
			m.recordFailure(ref, reason, staticURL)
			return staticURL
		}
		m.recordFailure(ref, reason, ref)
		return ref
	}

	if local == "" {
		m.recordFailure(ref, "static file not found in the archive", ref)
		return ref
	}

	m.mu.Lock()
	m.cache[ref] = local
	m.mu.Unlock()
	return local
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

	// Record the perceptual hash of the source bytes so the importer can
	// detect visually identical covers; best-effort, undecodable files simply
	// have no hash.
	hash, hashErr := mediadomain.PerceptualHash(bytes.NewReader(body))

	media, err := m.service.GenerateFromBytes(ctx, body, userID, altTextFromRef(ref), filepath.Base(path))
	if err != nil {
		var dupErr *mediadomain.DuplicateMediaError
		if errors.As(err, &dupErr) && dupErr.Existing != nil {
			if hashErr == nil {
				m.recordPHash(dupErr.Existing.URL, hash)
			}
			return dupErr.Existing.URL, nil
		}
		return "", fmt.Errorf("failed to re-upload static image %q: %w", ref, err)
	}
	if hashErr == nil {
		m.recordPHash(media.URL, hash)
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
		mapped := m.Map(ctx, match[1], userID)
		if mapped == "" {
			return tag
		}
		return strings.Replace(tag, match[1], mapped, 1)
	})
}

// RewriteStaticRefs rewrites root-relative href/src references that resolve to
// files under the extracted Hugo static/ dir to their /static/<path> URLs —
// the documented convention (operators mirror their Hugo static/ into the
// theme's static/). References that resolve nowhere, remote URLs, and content
// permalinks are left untouched.
func (m *MediaMapper) RewriteStaticRefs(body string) string {
	if m.staticDir == "" {
		return body
	}
	return staticRefRe.ReplaceAllStringFunc(body, func(attr string) string {
		idx := staticRefRe.FindStringSubmatchIndex(attr)
		value := attr[idx[6]:idx[7]]
		staticURL := m.staticURL(value)
		if staticURL == "" {
			return attr
		}
		return attr[:idx[6]] + staticURL + attr[idx[7]:]
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
		failed:     make(map[string]failure),
		phashes:    make(map[string]uint64),
		reported:   make(map[string]struct{}),
		skipMedia:  skipMedia,
	}
}
