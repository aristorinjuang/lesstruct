package wordpress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

const downloadWorkers = 10

var wpDatetimeFormats = []string{
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02",
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

	default:
		return raw, nil
	}
}

// contentCreator is the subset of the content domain service needed to create
// imported items.
type contentCreator interface {
	Create(ctx context.Context, userID int, req contentdomain.CreateContentRequest) (*contentdomain.Content, error)
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
	workerCount := downloadWorkers
	if len(allURLs) < workerCount {
		workerCount = len(allURLs)
	}

	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
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
		}()
	}

	wg.Wait()

	return imageMap, errs
}

func (imp *Importer) importItem(
	ctx context.Context,
	item ParsedItem,
	imageMap map[string]string,
	featuredImageURL string,
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
		Title:        item.Title,
		Content:      contentBody,
		Tags:         item.Tags,
		Status:       status,
		Format:       format,
		PostType:     item.PostType,
		CustomFields: customFields,
		Language:     imp.language,
	}

	if _, err := imp.contentService.Create(ctx, userID, req); err != nil {
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
			return nil, fmt.Errorf("%s: %w", field.Name, err)
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

// resolveFeaturedImage resolves a WordPress post's _thumbnail_id to a local
// media URL. The attachment post ID is looked up in the pre-built attachments
// map (built by the parser from attachment items), downloaded via the media
// downloader, and remapped to a local URL. On failure the original WordPress
// URL is returned so the image is hotlinked rather than lost.
func (imp *Importer) resolveFeaturedImage(
	ctx context.Context,
	item ParsedItem,
	attachments map[int]string,
	userID int,
	result *ImportResult,
) string {
	thumbnailID, ok := item.Meta["_thumbnail_id"]
	if !ok || strings.TrimSpace(thumbnailID) == "" {
		return ""
	}
	id, err := strconv.Atoi(strings.TrimSpace(thumbnailID))
	if err != nil || id == 0 {
		return ""
	}
	wpURL, ok := attachments[id]
	if !ok || wpURL == "" {
		return ""
	}
	local, err := imp.downloader.DownloadAndUpload(ctx, wpURL, userID)
	if err != nil {
		if imp.logger != nil {
			imp.logger.Error("WordPress import: featured image download failed for %s: %v", wpURL, err)
		}
		result.Errors = append(result.Errors, fmt.Sprintf("featured image not downloaded: %s", wpURL))
		return wpURL
	}
	if local != "" {
		return local
	}
	return wpURL
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
// Images are downloaded per-item — each item's inline images are fetched just
// before its content is created. The MediaDownloader cache deduplicates across
// items so every unique image URL is fetched at most once for the entire import.
// This means content starts appearing in the database immediately (no lengthy
// "download all images first" phase), and progress updates are meaningful from
// the very first item.
func (imp *Importer) Import(ctx context.Context, wxrData io.Reader, userID int, onProgress func(Progress)) (*ImportResult, error) {
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
		itemImageMap, downloadErrs := imp.downloadImages(ctx, []ParsedItem{item}, userID)
		result.Errors = append(result.Errors, downloadErrs...)

		itemUserID := imp.resolveUserID(ctx, item.Creator, authorByLogin, userID, creatorCache, result)
		featuredURL := imp.resolveFeaturedImage(ctx, item, doc.Attachments, userID, result)
		imp.importItem(ctx, item, itemImageMap, featuredURL, itemUserID, result)

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
