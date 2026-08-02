package hugo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/content/wordpress"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

func slugFromTitle(title string) string {
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	slug = result.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if len(slug) > 200 {
		slug = slug[:200]
	}
	if slug == "" {
		slug = "untitled"
	}
	return slug
}

func sanitizeSlug(slug string) string {
	slug = strings.ToLower(slug)
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '/' {
			result.WriteRune(r)
		} else if r == ' ' {
			result.WriteRune('-')
		}
	}
	slug = result.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if len(slug) > 200 {
		slug = slug[:200]
	}
	if slug == "" {
		slug = "untitled"
	}
	return slug
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, tag := range tags {
		t := strings.TrimSpace(tag)
		t = strings.ToLower(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		result = append(result, t)
	}
	return result
}

// truncateRunes truncates s to at most max runes, preserving valid UTF-8.
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

type ContentCreator interface {
	Create(ctx context.Context, userID int, req contentdomain.CreateContentRequest) (*contentdomain.Content, error)
}

type AliasCreator interface {
	Create(ctx context.Context, contentID int, aliasStr string) error
}

// SlugResolver reports whether a slug already exists in a language and, when
// it does, resolves the existing content ID. It lets the importer skip
// already-imported items on re-runs (idempotent imports) while still linking
// translated variants to the previously imported English item.
type SlugResolver interface {
	SlugExists(ctx context.Context, slug string, language string) (bool, error)
	GetBySlugAndLanguage(ctx context.Context, slug string, language string) (*contentdomain.Content, error)
}

// ImportOptions controls the Hugo import pipeline.
type ImportOptions struct {
	// SkipMedia disables media migration: images stay linked to their original
	// paths/URLs and no media is created.
	SkipMedia bool
}

// Progress reports import progress to the job store after each item.
type Progress struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
	Total    int `json:"total"`
}

type Importer struct {
	contentService ContentCreator
	aliasService   AliasCreator
	slugChecker    SlugResolver
	mediaService   MediaService
	downloader     *wordpress.MediaDownloader
	language       string
	logger         *util.Logger
}

func (imp *Importer) Import(
	ctx context.Context,
	site *HugoSite,
	userID int,
	opts ImportOptions,
	onProgress func(Progress),
) *ImportResult {
	result := &ImportResult{}

	mediaMapper := NewMediaMapper(
		site.StaticDir,
		imp.mediaService,
		imp.downloader,
		opts.SkipMedia,
	)

	for _, g := range GroupTranslations(site.Items) {
		switch v := g.(type) {
		case *HugoItem:
			imp.importItem(ctx, v, mediaMapper, userID, nil, result)
		case TranslationGroup:
			enID, _ := imp.importItem(ctx, v.English, mediaMapper, userID, nil, result)
			// Import the Indonesian variant whenever the English item is
			// available — either freshly created or already imported on a
			// re-run (enID is then the existing content ID).
			if enID != 0 && v.Indonesian != nil {
				imp.importItem(ctx, v.Indonesian, mediaMapper, userID, &enID, result)
			}
		}

		if onProgress != nil {
			onProgress(Progress{
				Imported: result.Imported,
				Skipped:  result.Skipped,
				Total:    len(site.Items),
			})
		}
	}

	return result
}

func (imp *Importer) importItem(
	ctx context.Context,
	item *HugoItem,
	mediaMapper *MediaMapper,
	userID int,
	translationGroupID *int,
	result *ImportResult,
) (int, bool) {
	body := TransformShortcodes(item.OriginalBody)

	if mediaMapper != nil {
		body = mediaMapper.RewriteBody(ctx, body, userID)
	}

	// Featured image: prepend the first frontmatter image (remapped) as a
	// leading <img>, mirroring how the WordPress importer injects featured
	// images as the first content node.
	if mediaMapper != nil && len(item.Images) > 0 {
		if featured, err := mediaMapper.Map(ctx, item.Images[0], userID); err == nil && featured != "" {
			body = fmt.Sprintf(`<img src="%s" alt="%s">%s`, featured, item.Title, body)
		}
	}

	customFields := make(map[string]any)
	if item.HasMath {
		customFields["hasMath"] = true
	}
	if item.HasChart {
		customFields["hasChart"] = true
	}
	if item.HasDiagrams {
		customFields["hasDiagrams"] = true
	}
	if item.HideMobileImages {
		customFields["hideMobileImages"] = true
	}

	status := contentdomain.StatusPublished
	if item.IsDraft {
		status = contentdomain.StatusDraft
	}

	slug := item.URL
	if slug == "" {
		slug = slugFromTitle(item.Title)
	} else {
		slug = strings.TrimPrefix(slug, "/")
		slug = strings.TrimSuffix(slug, ".html")
		slug = sanitizeSlug(slug)
	}

	// Idempotent re-runs: skip items whose slug already exists in the target
	// language. Without this, re-running an import after a partial failure
	// either errors on every previously-imported item or duplicates content.
	// When the slug exists we resolve its content ID and return it so a
	// translated variant still links to the existing English item.
	if imp.slugChecker != nil {
		exists, err := imp.slugChecker.SlugExists(ctx, slug, item.Language)
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("skipped %q: failed to check slug: %v", item.Title, err))
			return 0, false
		}
		if exists {
			if existing, resolveErr := imp.slugChecker.GetBySlugAndLanguage(ctx, slug, item.Language); resolveErr == nil && existing != nil {
				result.Skipped++
				result.Errors = append(result.Errors, fmt.Sprintf("skipped %q: already imported (slug %q exists)", item.Title, slug))
				return existing.ID, false
			}
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("skipped %q: already imported (slug %q exists)", item.Title, slug))
			return 0, false
		}
	}

	req := contentdomain.CreateContentRequest{
		Title:              item.Title,
		Content:            body,
		Tags:               normalizeTags(item.Tags),
		Status:             status,
		Format:             contentdomain.FormatHTML,
		PostType:           "post",
		MetaDescription:    truncateRunes(item.Description, contentdomain.MaxMetaDescriptionRunes),
		CustomFields:       customFields,
		Language:           item.Language,
		TranslationGroupID: translationGroupID,
	}

	if !item.Date.IsZero() {
		req.PublishedAt = &item.Date
	}

	if slug != "" {
		req.Slug = slug
	}

	created, err := imp.contentService.Create(ctx, userID, req)
	if err != nil {
		result.Skipped++
		if errors.Is(err, contentdomain.ErrSlugAlreadyExists) {
			result.Errors = append(result.Errors, fmt.Sprintf("skipped %q: already imported", item.Title))
			return 0, false
		}
		result.Errors = append(result.Errors, fmt.Sprintf("skipped %q: %v", item.Title, err))
		return 0, false
	}

	for _, aliasStr := range item.Aliases {
		cleanAlias := strings.TrimPrefix(aliasStr, "/")
		if cleanAlias == "" {
			continue
		}
		if err := imp.aliasService.Create(ctx, created.ID, cleanAlias); err != nil {
			if imp.logger != nil {
				imp.logger.Error("failed to create alias %q for content %d: %v", cleanAlias, created.ID, err)
			}
		}
	}

	result.Imported++
	return created.ID, true
}

func NewImporter(
	contentService ContentCreator,
	aliasService AliasCreator,
	slugChecker SlugResolver,
	mediaService MediaService,
	downloader *wordpress.MediaDownloader,
	language string,
	logger *util.Logger,
) *Importer {
	return &Importer{
		contentService: contentService,
		aliasService:   aliasService,
		slugChecker:    slugChecker,
		mediaService:   mediaService,
		downloader:     downloader,
		language:       language,
		logger:         logger,
	}
}
