package hugo

import (
	"context"
	"fmt"
	"strings"

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

type ContentCreator interface {
	Create(ctx context.Context, userID int, req contentdomain.CreateContentRequest) (*contentdomain.Content, error)
}

type AliasCreator interface {
	Create(ctx context.Context, contentID int, aliasStr string) error
}

type Importer struct {
	contentService ContentCreator
	aliasService   AliasCreator
	language       string
	logger         *util.Logger
}

func (imp *Importer) Import(ctx context.Context, site *HugoSite, userID int) *ImportResult {
	result := &ImportResult{}

	for _, g := range groupTranslations(site.Items) {
		switch v := g.(type) {
		case *HugoItem:
			imp.importItem(ctx, v, userID, nil, result)
		case translationGroup:
			enID, ok := imp.importItem(ctx, v.English, userID, nil, result)
			if ok && v.Indonesian != nil {
				imp.importItem(ctx, v.Indonesian, userID, &enID, result)
			}
		}
	}

	return result
}

func (imp *Importer) importItem(
	ctx context.Context,
	item *HugoItem,
	userID int,
	translationGroupID *int,
	result *ImportResult,
) (int, bool) {
	body := TransformShortcodes(item.OriginalBody)

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

	req := contentdomain.CreateContentRequest{
		Title:              item.Title,
		Content:            body,
		Tags:               normalizeTags(item.Tags),
		Status:             status,
		Format:             contentdomain.FormatHTML,
		PostType:           "post",
		MetaDescription:    item.Description,
		CustomFields:       customFields,
		Language:           item.Language,
		TranslationGroupID: translationGroupID,
	}

	if slug != "" {
		req.Slug = slug
	}

	created, err := imp.contentService.Create(ctx, userID, req)
	if err != nil {
		result.Skipped++
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
	language string,
	logger *util.Logger,
) *Importer {
	return &Importer{
		contentService: contentService,
		aliasService:   aliasService,
		language:       language,
		logger:         logger,
	}
}
