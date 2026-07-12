package wordpress

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

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

// Importer orchestrates a WordPress import: parse the WXR, download images,
// convert each item to TipTap JSON, and create it via the content service.
type Importer struct {
	contentService contentCreator
	downloader     *MediaDownloader
	resolver       userResolver
	postTypes      postTypeLister
	logger         *util.Logger
}

// downloadImages collects every image URL across all items, downloads each once,
// and returns a map of WordPress URL to local media URL.
func (imp *Importer) downloadImages(ctx context.Context, items []ParsedItem, userID int) (map[string]string, []string) {
	imageMap := make(map[string]string)
	var errs []string
	seen := make(map[string]struct{})

	for _, item := range items {
		for _, imageURL := range ExtractImageURLs(item.Content) {
			if _, ok := seen[imageURL]; ok {
				continue
			}
			seen[imageURL] = struct{}{}

			local, err := imp.downloader.DownloadAndUpload(ctx, imageURL, userID)
			if err != nil {
				if imp.logger != nil {
					imp.logger.Error("WordPress import: image download failed for %s: %v", imageURL, err)
				}
				errs = append(errs, fmt.Sprintf("image not downloaded: %s", imageURL))
				continue
			}
			if local != "" {
				imageMap[imageURL] = local
			}
		}
	}
	return imageMap, errs
}

func (imp *Importer) importItem(
	ctx context.Context,
	item ParsedItem,
	imageMap map[string]string,
	userID int,
	result *ImportResult,
) {
	contentJSON, err := ConvertBlocks(item.Content, imageMap)
	if err != nil {
		result.Skipped++
		msg := fmt.Sprintf("skipped %q: failed to convert content: %v", item.Title, err)
		result.Errors = append(result.Errors, msg)
		return
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
		Content:      contentJSON,
		Tags:         item.Tags,
		Status:       status,
		PostType:     item.PostType,
		CustomFields: customFields,
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
// aggregated result.
func (imp *Importer) Import(ctx context.Context, wxrData io.Reader, userID int) (*ImportResult, error) {
	allowedTypes := imp.allowedPostTypes()

	doc, err := Parse(wxrData, allowedTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WXR: %w", err)
	}

	imageMap, downloadErrors := imp.downloadImages(ctx, doc.Items, userID)

	result := &ImportResult{Errors: downloadErrors}

	authorByLogin := make(map[string]ParsedAuthor, len(doc.Authors))
	for _, a := range doc.Authors {
		authorByLogin[a.Login] = a
	}

	creatorCache := make(map[string]int)

	for _, item := range doc.Items {
		itemUserID := imp.resolveUserID(ctx, item.Creator, authorByLogin, userID, creatorCache, result)
		imp.importItem(ctx, item, imageMap, itemUserID, result)
	}
	return result, nil
}

// NewImporter creates an importer.
func NewImporter(
	contentService contentCreator,
	downloader *MediaDownloader,
	resolver userResolver,
	postTypes postTypeLister,
	logger *util.Logger,
) *Importer {
	return &Importer{
		contentService: contentService,
		downloader:     downloader,
		resolver:       resolver,
		postTypes:      postTypes,
		logger:         logger,
	}
}
