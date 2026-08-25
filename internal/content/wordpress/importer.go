package wordpress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

const downloadWorkers = 10

var wpDatetimeFormats = []string{
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02",
	"20060102", // ACF date-picker "Ymd" storage format (e.g. 20250601)
}

// convertMetaValue converts a raw WordPress postmeta string to the Go type
// expected by the content domain's custom-field validator:
//   - number  → float64
//   - datetime → RFC 3339 string (WordPress stores "YYYY-MM-DD HH:MM:SS")
//   - date    → "YYYY-MM-DD" string (time portion stripped if present)
//   - checkbox → bool
//   - all other types pass through as-is (validated downstream)
func convertMetaValue(field customfield.FieldSchema, raw string) (any, error) {
	trimmed := strings.TrimSpace(raw)
	switch field.Type {
	case customfield.FieldTypeNumber:
		n, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return nil, fmt.Errorf("must be a number")
		}
		return n, nil

	case customfield.FieldTypeDatetime:
		for _, layout := range wpDatetimeFormats {
			if t, err := time.Parse(layout, trimmed); err == nil {
				return t.UTC().Format(time.RFC3339), nil
			}
		}
		return nil, fmt.Errorf("must be a valid datetime")

	case customfield.FieldTypeDate:
		for _, layout := range wpDatetimeFormats {
			if t, err := time.Parse(layout, trimmed); err == nil {
				return t.Format("2006-01-02"), nil
			}
		}
		return nil, fmt.Errorf("must be a valid date")

	case customfield.FieldTypeCheckbox:
		b, err := strconv.ParseBool(trimmed)
		if err != nil {
			return nil, fmt.Errorf("must be a boolean")
		}
		return b, nil

	case customfield.FieldTypeUrl:
		// Mirror the content domain's URL rule so an invalid value (e.g. a
		// stray ACF attachment ID) is caught here and dropped for optional
		// fields instead of failing the whole item at Create.
		u, err := url.Parse(trimmed)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, fmt.Errorf("must be a valid http(s) URL")
		}
		return trimmed, nil

	default:
		return strings.TrimSpace(raw), nil
	}
}

// ParseWPDate parses a WordPress pubDate (RFC 1123 with zone) or the common
// WXR datetime formats into a time.Time. Returns false when the value does not
// match any known layout so callers can fall back to the server timestamp.
func ParseWPDate(raw string) (time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false
	}
	for _, layout := range append(wpDatetimeFormats, time.RFC1123, time.RFC1123Z) {
		if t, err := time.Parse(layout, trimmed); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// truncateRunes truncates s to at most max runes, preserving valid UTF-8.
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// cleanTags drops tags that would fail domain validation: tags that are empty
// after trimming or longer than the rune limit. The result is never nil so the
// request carries a stable empty slice when nothing survives.
func cleanTags(tags []string) []string {
	cleaned := make([]string, 0, len(tags))
	for _, tag := range tags {
		t := strings.TrimSpace(tag)
		if t != "" && utf8.RuneCountInString(t) <= contentdomain.MaxTagRunes {
			cleaned = append(cleaned, t)
		}
	}
	return cleaned
}

// contentCreator is the subset of the content domain service needed to create
// imported items.
type contentCreator interface {
	Create(ctx context.Context, userID int, role string, req contentdomain.CreateContentRequest) (*contentdomain.Content, error)
}

// userResolver resolves a WordPress author login to a Lesstruct userID.
type userResolver interface {
	ResolveOrCreate(ctx context.Context, login, email, displayName string) (userID int, created bool, err error)
}

// postTypeLister is the subset of the post-type domain service used to determine
// which post types are importable and to retrieve their custom field schemas.
type postTypeLister interface {
	GetAll() []posttype.PostType
	GetFieldsByPostType(slug string) ([]customfield.FieldSchema, error)
}

// ImportResult summarizes the outcome of an import run.
type ImportResult struct {
	Imported      int      `json:"imported"`
	Skipped       int      `json:"skipped"`
	UsersImported int      `json:"usersImported"`
	Errors        []string `json:"errors,omitempty"`
}

// Progress reflects the current state of an in-flight import.
type Progress struct {
	Imported      int `json:"imported"`
	Skipped       int `json:"skipped"`
	UsersImported int `json:"usersImported"`
	Total         int `json:"total"`
}

// ImportOptions controls per-import behaviour.
type ImportOptions struct {
	// SkipMedia, when true, skips downloading inline images and featured
	// images. Content is imported with original WordPress image URLs
	// (hotlinked) and no featured image is set.
	SkipMedia bool
}

// Importer orchestrates a WordPress import: parse the WXR, download images,
// convert each item to TipTap JSON, and create it via the content service.
type Importer struct {
	contentService contentCreator
	downloader     *MediaDownloader
	resolver       userResolver
	postTypes      postTypeLister
	language       string
	logger         *util.Logger
}

// downloadImages collects every image URL across all items, downloads each once,
// and returns a map of WordPress URL to local media URL.
func (imp *Importer) downloadImages(ctx context.Context, items []ParsedItem, userID int) (map[string]string, []string) {
	// Collect unique image URLs first.
	var allURLs []string
	seen := make(map[string]struct{})
	for _, item := range items {
		for _, imageURL := range ExtractImageURLs(item.Content) {
			if _, ok := seen[imageURL]; ok {
				continue
			}
			seen[imageURL] = struct{}{}
			allURLs = append(allURLs, imageURL)
		}
	}

	if len(allURLs) == 0 {
		return make(map[string]string), nil
	}

	imageMap := make(map[string]string)
	var errs []string
	var mu sync.Mutex

	urlCh := make(chan string, len(allURLs))
	for _, u := range allURLs {
		urlCh <- u
	}
	close(urlCh)

	var wg sync.WaitGroup
	workerCount := min(len(allURLs), downloadWorkers)

	for range workerCount {
		wg.Go(func() {
			for imageURL := range urlCh {
				local, err := imp.downloader.DownloadAndUpload(ctx, imageURL, userID)
				mu.Lock()
				if err != nil {
					if imp.logger != nil {
						imp.logger.Error("WordPress import: image download failed for %s: %v", imageURL, err)
					}
					errs = append(errs, fmt.Sprintf("image not downloaded: %s", imageURL))
				} else if local != "" {
					imageMap[imageURL] = local
				}
				mu.Unlock()
			}
		})
	}

	wg.Wait()

	return imageMap, errs
}

// featuredCheckLimit is the number of body images examined when deciding
// whether prepending the featured image would show the same picture twice.
const featuredCheckLimit = 3

// Reasons reported by featuredDuplicateOfBody.
const (
	dupNone   = ""
	dupExact  = "exact"
	dupVisual = "visual"
)

func (imp *Importer) importItem(
	ctx context.Context,
	item ParsedItem,
	imageMap map[string]string,
	featuredImageURL string,
	featuredSourceURL string,
	userID int,
	result *ImportResult,
) {
	isElementor := imp.isElementorPage(item)
	var contentBody string
	var format contentdomain.Format

	if isElementor {
		contentBody = imp.extractElementorHTML(item)
		if contentBody == "" {
			contentBody = item.Content
		}
		format = contentdomain.FormatHTML
	} else {
		// Prepend the featured image unless the body would then show the
		// same picture twice — either an existing body image carries the
		// exact same source URL, or one of them is perceptually the same
		// picture under a different URL (a resized export or hotlink variant
		// that content-hash dedup cannot see). Perceptual skips are surfaced
		// as warnings so site owners can audit what was left out.
		reason, duplicateOf := imp.featuredDuplicateOfBody(item.Content, featuredSourceURL)
		switch reason {
		case dupExact:
			featuredImageURL = ""
		case dupVisual:
			featuredImageURL = ""
			result.Errors = append(result.Errors, fmt.Sprintf(
				"warning: featured image %q looks identical to an existing body image %q — prepend skipped",
				featuredSourceURL,
				duplicateOf,
			))
		}

		var err error
		contentBody, err = ConvertBlocks(item.Content, imageMap, featuredImageURL)
		if err != nil {
			result.Skipped++
			msg := fmt.Sprintf("skipped %q: failed to convert content: %v", item.Title, err)
			result.Errors = append(result.Errors, msg)
			return
		}
		format = contentdomain.FormatTiptap
	}

	status := contentdomain.StatusDraft
	if item.Status == "published" {
		status = contentdomain.StatusPublished
	}

	customFields, err := imp.buildCustomFields(item)
	if err != nil {
		result.Skipped++
		result.Errors = append(result.Errors, fmt.Sprintf("skipped %q: %v", item.Title, err))
		return
	}

	req := contentdomain.CreateContentRequest{
		Title:        truncateRunes(item.Title, contentdomain.MaxTitleRunes),
		Content:      contentBody,
		Tags:         cleanTags(item.Tags),
		Status:       status,
		Format:       format,
		PostType:     item.PostType,
		CustomFields: customFields,
		Language:     imp.language,
	}

	if publishedAt, ok := ParseWPDate(item.PubDate); ok {
		req.PublishedAt = &publishedAt
	}

	if _, err := imp.contentService.Create(ctx, userID, contentdomain.RoleAdmin, req); err != nil {
		result.Skipped++
		if errors.Is(err, contentdomain.ErrSlugAlreadyExists) {
			result.Errors = append(result.Errors, fmt.Sprintf("skipped %q: a post with this slug already exists", item.Title))
			return
		}
		result.Errors = append(result.Errors, fmt.Sprintf("skipped %q: %v", item.Title, err))
		return
	}

	result.Imported++
}

// isElementorPage reports whether the parsed item was built with Elementor.
// It checks the _elementor_edit_mode postmeta for "builder" or the presence
// of a non-empty _elementor_element_cache postmeta.
func (imp *Importer) isElementorPage(item ParsedItem) bool {
	if item.Meta == nil {
		return false
	}
	if mode, ok := item.Meta["_elementor_edit_mode"]; ok && strings.TrimSpace(mode) == "builder" {
		return true
	}
	if cache, ok := item.Meta["_elementor_element_cache"]; ok && strings.TrimSpace(cache) != "" {
		return true
	}
	return false
}

// extractElementorHTML attempts to extract the rendered HTML from Elementor's
// postmeta. It looks for _elementor_element_cache.value.content first, then
// falls back to <content:encoded>.
func (imp *Importer) extractElementorHTML(item ParsedItem) string {
	if item.Meta == nil {
		return ""
	}
	cache, ok := item.Meta["_elementor_element_cache"]
	if !ok || strings.TrimSpace(cache) == "" {
		return ""
	}
	var parsed struct {
		Value struct {
			Content string `json:"content"`
		} `json:"value"`
	}
	if err := json.Unmarshal([]byte(cache), &parsed); err == nil && parsed.Value.Content != "" {
		return parsed.Value.Content
	}
	return ""
}

// buildCustomFields maps a parsed item's WordPress postmeta to the custom-field
// values expected by the content domain. Only keys that match a declared field
// schema are included; ACF internal keys (prefixed with "_") and stray WordPress
// meta are ignored. Values are converted to the declared field type. A missing
// required field (or an unconvertible value) returns an error so the caller can
// skip the item with a clear message.
func (imp *Importer) buildCustomFields(item ParsedItem) (map[string]any, error) {
	if imp.postTypes == nil || len(item.Meta) == 0 {
		return nil, nil
	}

	fields, err := imp.postTypes.GetFieldsByPostType(item.PostType)
	if err != nil {
		return nil, fmt.Errorf("failed to get field schema for post type %q: %w", item.PostType, err)
	}
	if len(fields) == 0 {
		return nil, nil
	}

	customFields := make(map[string]any, len(fields))
	for _, field := range fields {
		raw, exists := item.Meta[field.Slug]
		if !exists || strings.TrimSpace(raw) == "" {
			if field.Required && field.Type != customfield.FieldTypeCheckbox {
				return nil, fmt.Errorf("%s is required", field.Name)
			}
			continue
		}
		converted, err := convertMetaValue(field, raw)
		if err != nil {
			if field.Required {
				return nil, fmt.Errorf("%s: %w", field.Name, err)
			}
			// Optional field with an unconvertible value (e.g. a stray ACF
			// attachment ID in a URL field) — drop the value rather than
			// skipping the whole item.
			continue
		}
		customFields[field.Slug] = converted
	}

	if len(customFields) == 0 {
		return nil, nil
	}
	return customFields, nil
}

// resolveUserID maps a WordPress creator login to a Lesstruct userID using the
// cache. On the first encounter it calls the resolver; subsequent items with
// the same creator reuse the cached value. Failures fall back to the importing
// admin's userID.
func (imp *Importer) resolveUserID(
	ctx context.Context,
	creatorLogin string,
	authorByLogin map[string]ParsedAuthor,
	fallbackUserID int,
	cache map[string]int,
	result *ImportResult,
) int {
	if id, ok := cache[creatorLogin]; ok {
		return id
	}

	var id int
	if creatorLogin == "" {
		id = fallbackUserID
	} else {
		email := ""
		displayName := creatorLogin
		if author, found := authorByLogin[creatorLogin]; found {
			email = author.Email
			displayName = author.DisplayName
		}

		resolvedID, created, err := imp.resolver.ResolveOrCreate(ctx, creatorLogin, email, displayName)
		if err != nil {
			if imp.logger != nil {
				imp.logger.Error(
					"WordPress import: failed to resolve author %q, falling back to admin: %v",
					creatorLogin,
					err,
				)
			}
			result.Errors = append(
				result.Errors,
				fmt.Sprintf("could not import author %q, posts assigned to admin: %v", creatorLogin, err),
			)
			id = fallbackUserID
		} else {
			if created {
				result.UsersImported++
			}
			id = resolvedID
		}
	}

	cache[creatorLogin] = id
	return id
}

// featuredDuplicateOfBody reports whether the post's body already shows the
// featured picture among its first images — compared in source-URL space
// against the perceptual hashes recorded by the media downloader during
// download. An exact source-URL match wins silently; otherwise a
// perceptual-hash comparison catches the same picture under a different URL.
// Images whose hash is unknown (not downloaded, undecodable) fall back to the
// exact check only.
func (imp *Importer) featuredDuplicateOfBody(content string, featuredSourceURL string) (string, string) {
	if featuredSourceURL == "" || imp.downloader == nil {
		return dupNone, ""
	}

	srcs := ExtractImageURLs(content)
	if len(srcs) > featuredCheckLimit {
		srcs = srcs[:featuredCheckLimit]
	}
	for _, src := range srcs {
		if src == featuredSourceURL {
			return dupExact, src
		}
	}

	featuredHash, ok := imp.downloader.PHash(featuredSourceURL)
	if !ok {
		return dupNone, ""
	}
	for _, src := range srcs {
		hash, ok := imp.downloader.PHash(src)
		if ok && mediadomain.PerceptuallySimilar(featuredHash, hash) {
			return dupVisual, src
		}
	}
	return dupNone, ""
}

// resolveFeaturedImage resolves a WordPress post's _thumbnail_id to a local
// media URL. The attachment post ID is looked up in the pre-built attachments
// map (built by the parser from attachment items), downloaded via the media
// downloader, and remapped to a local URL. On failure the original WordPress
// URL is returned so the image is hotlinked rather than lost. The original
// WordPress URL is returned alongside for duplicate detection.
func (imp *Importer) resolveFeaturedImage(
	ctx context.Context,
	item ParsedItem,
	attachments map[int]string,
	userID int,
	result *ImportResult,
) (string, string) {
	thumbnailID, ok := item.Meta["_thumbnail_id"]
	if !ok || strings.TrimSpace(thumbnailID) == "" {
		return "", ""
	}
	id, err := strconv.Atoi(strings.TrimSpace(thumbnailID))
	if err != nil || id == 0 {
		return "", ""
	}
	wpURL, ok := attachments[id]
	if !ok || wpURL == "" {
		return "", ""
	}
	local, err := imp.downloader.DownloadAndUpload(ctx, wpURL, userID)
	if err != nil {
		if imp.logger != nil {
			imp.logger.Error("WordPress import: featured image download failed for %s: %v", wpURL, err)
		}
		result.Errors = append(result.Errors, fmt.Sprintf("featured image not downloaded: %s", wpURL))
		return wpURL, wpURL
	}
	if local != "" {
		return local, wpURL
	}
	return wpURL, wpURL
}

func (imp *Importer) allowedPostTypes() map[string]bool {
	allowed := map[string]bool{"post": true, "page": true}
	if imp.postTypes == nil {
		return allowed
	}
	for _, pt := range imp.postTypes.GetAll() {
		allowed[pt.Slug] = true
	}
	return allowed
}

// Import reads a WXR stream and imports every item whose post type is registered
// in the post-type service. Image download failures are logged and recorded but
// never abort the run; the original WordPress URL is kept as a fallback. Authors
// are auto-created as Contributor users and their posts are assigned to them;
// authors that cannot be resolved fall back to the importing admin. Returns an
// aggregated result. The onProgress callback, if non-nil, is called after each
// item with current cumulative totals.
//
// opts controls per-import behaviour:
//   - SkipMedia (default false): when true, skips downloading inline images and
//     featured images. Content is imported with original WordPress image URLs
//     (hotlinked) and no featured image is set.
//
// Images are downloaded per-item when SkipMedia is false — each item's inline
// images are fetched just before its content is created. The MediaDownloader
// cache deduplicates across items so every unique image URL is fetched at most
// once for the entire import. This means content starts appearing in the
// database immediately (no lengthy "download all images first" phase), and
// progress updates are meaningful from the very first item. When SkipMedia is
// true, no download attempts are made and content is imported as-is.
func (imp *Importer) Import(ctx context.Context, wxrData io.Reader, userID int, opts ImportOptions, onProgress func(Progress)) (*ImportResult, error) {
	allowedTypes := imp.allowedPostTypes()

	doc, err := Parse(wxrData, allowedTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WXR: %w", err)
	}

	authorByLogin := make(map[string]ParsedAuthor, len(doc.Authors))
	for _, a := range doc.Authors {
		authorByLogin[a.Login] = a
	}

	result := &ImportResult{}
	creatorCache := make(map[string]int)

	for _, item := range doc.Items {
		var itemImageMap map[string]string
		var featuredURL, featuredSource string
		if !opts.SkipMedia {
			var downloadErrs []string
			itemImageMap, downloadErrs = imp.downloadImages(ctx, []ParsedItem{item}, userID)
			result.Errors = append(result.Errors, downloadErrs...)
			featuredURL, featuredSource = imp.resolveFeaturedImage(ctx, item, doc.Attachments, userID, result)
		}

		itemUserID := imp.resolveUserID(ctx, item.Creator, authorByLogin, userID, creatorCache, result)
		imp.importItem(ctx, item, itemImageMap, featuredURL, featuredSource, itemUserID, result)

		if onProgress != nil {
			onProgress(Progress{
				Imported:      result.Imported,
				Skipped:       result.Skipped,
				UsersImported: result.UsersImported,
				Total:         len(doc.Items),
			})
		}
	}
	return result, nil
}

// NewImporter creates an importer.
func NewImporter(
	contentService contentCreator,
	downloader *MediaDownloader,
	resolver userResolver,
	postTypes postTypeLister,
	language string,
	logger *util.Logger,
) *Importer {
	return &Importer{
		contentService: contentService,
		downloader:     downloader,
		resolver:       resolver,
		postTypes:      postTypes,
		language:       language,
		logger:         logger,
	}
}
