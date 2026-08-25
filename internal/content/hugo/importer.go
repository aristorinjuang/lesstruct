package hugo

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/content/wordpress"
	aliasdomain "github.com/aristorinjuang/lesstruct/internal/domain/alias"
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
	Create(ctx context.Context, userID int, role string, req contentdomain.CreateContentRequest) (*contentdomain.Content, error)
	GetByID(ctx context.Context, id int) (*contentdomain.Content, error)
}

// AliasCreator manages the legacy-URL aliases attached to imported items.
// Create is the happy path; when it fails with ErrAliasAlreadyExists the
// importer may FindByAlias + Repoint a dangling alias (one whose content_id
// no longer exists) onto the newly imported item.
type AliasCreator interface {
	Create(ctx context.Context, contentID int, aliasStr string) error
	FindByAlias(ctx context.Context, aliasStr string) (*aliasdomain.Alias, error)
	Repoint(ctx context.Context, aliasStr string, fromContentID, toContentID int) error
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

	// Every URL and alias the import is about to create/keep; root-relative
	// references that match one of these are content permalinks, not dead
	// static references, so the unresolved-ref scan stays silent for them.
	knownTargets := make(map[string]struct{})
	for _, item := range site.Items {
		for _, target := range append([]string{item.URL}, item.Aliases...) {
			target = strings.TrimPrefix(strings.TrimSpace(target), "/")
			if target != "" {
				knownTargets[target] = struct{}{}
			}
		}
	}
	// References already surfaced as warnings, so none is emitted twice.
	warned := make(map[string]struct{})

	for _, g := range GroupTranslations(site.Items) {
		switch v := g.(type) {
		case *HugoItem:
			imp.importItem(ctx, v, mediaMapper, knownTargets, warned, userID, nil, result)
		case TranslationGroup:
			enID, _ := imp.importItem(ctx, v.English, mediaMapper, knownTargets, warned, userID, nil, result)
			// Import the Indonesian variant whenever the English item is
			// available — either freshly created or already imported on a
			// re-run (enID is then the existing content ID).
			if enID != 0 && v.Indonesian != nil {
				imp.importItem(ctx, v.Indonesian, mediaMapper, knownTargets, warned, userID, &enID, result)
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

	// Flush failures recorded after the last item's flush (e.g. the final
	// item's featured image) so nothing is lost.
	imp.appendFailures(result, mediaMapper, warned)

	return result
}

// appendFailures appends the mapper's unreported migration failures to the
// result's errors list, each exactly once, and marks them so the unresolved-
// ref scan does not re-warn the same reference.
func (imp *Importer) appendFailures(result *ImportResult, mediaMapper *MediaMapper, warned map[string]struct{}) {
	for _, f := range mediaMapper.TakeUnreportedFailures() {
		warned[f.Ref] = struct{}{}
		result.Errors = append(result.Errors,
			fmt.Sprintf("warning: %q could not be migrated to media (%s); kept the original reference", f.Ref, f.Reason),
		)
	}
}

// scanUnresolvedRefs returns the root-relative href/src references in the body
// that resolve to neither media, nor the theme static convention, nor a known
// content target (URL or alias of an imported item), and carry a file
// extension — i.e. likely dead references the operator should know about.
func scanUnresolvedRefs(body string, knownTargets map[string]struct{}) []string {
	var out []string
	seen := make(map[string]struct{})
	staticRefRe.ReplaceAllStringFunc(body, func(attr string) string {
		idx := staticRefRe.FindStringSubmatchIndex(attr)
		ref := attr[idx[6]:idx[7]]
		if _, ok := seen[ref]; ok {
			return attr
		}
		seen[ref] = struct{}{}

		trimmed := strings.TrimPrefix(ref, "/")
		if strings.HasPrefix(ref, "/static/") || strings.HasPrefix(ref, "/uploads/") {
			return attr
		}
		if _, ok := knownTargets[trimmed]; ok {
			return attr
		}
		if hasFileExtension(ref) {
			out = append(out, ref)
		}
		return attr
	})
	return out
}

// hasFileExtension reports whether the reference's path looks like a file
// (last segment contains a dot), as opposed to a bare permalink path.
func hasFileExtension(ref string) bool {
	path := ref
	if parsed, err := url.Parse(ref); err == nil && parsed.Path != "" {
		path = parsed.Path
	}
	base := path
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	return strings.Contains(base, ".")
}

func (imp *Importer) importItem(
	ctx context.Context,
	item *HugoItem,
	mediaMapper *MediaMapper,
	knownTargets map[string]struct{},
	warned map[string]struct{},
	userID int,
	translationGroupID *int,
	result *ImportResult,
) (int, bool) {
	body := TransformShortcodes(item.OriginalBody)

	if mediaMapper != nil {
		body = mediaMapper.RewriteBody(ctx, body, userID)
		body = mediaMapper.RewriteStaticRefs(body)
		imp.appendFailures(result, mediaMapper, warned)
		// References still pointing at nothing — no media URL, no /static/
		// copy, no known content target — get a warning instead of silence.
		for _, ref := range scanUnresolvedRefs(body, knownTargets) {
			if _, ok := warned[ref]; ok {
				continue
			}
			warned[ref] = struct{}{}
			result.Errors = append(result.Errors,
				fmt.Sprintf("warning: %q left unresolved — no file in the archive's static/ dir and no content target", ref),
			)
		}
	}

	// Featured image: prepend the first frontmatter image (remapped) as a
	// leading <figure>, mirroring how the WordPress importer injects featured
	// images as the first content node (TipTap renders them as figures) —
	// unless the body would then show the
	// same picture twice: either a leading body image already carries the
	// exact mapped URL, or one of them is perceptually the same picture under
	// a different URL (a re-upload or hotlink variant that content-hash dedup
	// cannot see). Perceptual skips are surfaced as warnings so site owners
	// can audit what was left out. Prepending is also skipped when the image
	// failed to migrate (a broken cover would be worse than none).
	if mediaMapper != nil && len(item.Images) > 0 {
		featured := mediaMapper.Map(ctx, item.Images[0], userID)
		if featured != "" && !mediaMapper.IsFailed(item.Images[0]) {
			reason, duplicateOf := featuredDuplicate(body, featured, mediaMapper)
			switch reason {
			case dupReasonExact:
			case dupReasonVisual:
				result.Errors = append(result.Errors, fmt.Sprintf(
					"warning: featured image %q looks identical to the body's leading image %q — prepend skipped",
					item.Images[0],
					duplicateOf,
				))
			default:
				body = fmt.Sprintf(`<figure><img src="%s" alt="%s"></figure>%s`, featured, item.Title, body)
			}
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

	created, err := imp.contentService.Create(ctx, userID, contentdomain.RoleAdmin, req)
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
			if !errors.Is(err, aliasdomain.ErrAliasAlreadyExists) {
				if imp.logger != nil {
					imp.logger.Error("failed to create alias %q for content %d: %v", cleanAlias, created.ID, err)
				}
				continue
			}
			// The alias already exists. When it still points at a live item it
			// stays (an alias is never stolen from another post); when its
			// target no longer exists (dangling row from a deleted item) it is
			// re-pointed onto the freshly imported content.
			existing, findErr := imp.aliasService.FindByAlias(ctx, cleanAlias)
			if findErr != nil {
				if imp.logger != nil {
					imp.logger.Error("failed to look up existing alias %q: %v", cleanAlias, findErr)
				}
				continue
			}
			if _, getErr := imp.contentService.GetByID(ctx, existing.ContentID); getErr != nil {
				if errors.Is(getErr, contentdomain.ErrContentNotFound) {
					if repointErr := imp.aliasService.Repoint(ctx, cleanAlias, existing.ContentID, created.ID); repointErr != nil {
						if errors.Is(repointErr, aliasdomain.ErrAliasNotFound) {
							if imp.logger != nil {
								imp.logger.Info("alias %q gone or re-pointed concurrently; leaving it", cleanAlias)
							}
							continue
						}
						if imp.logger != nil {
							imp.logger.Error("failed to re-point dangling alias %q to content %d: %v", cleanAlias, created.ID, repointErr)
						}
						continue
					}
					if imp.logger != nil {
						imp.logger.Info("re-pointed dangling alias %q to content %d", cleanAlias, created.ID)
					}
					continue
				}
				if imp.logger != nil {
					imp.logger.Error("failed to resolve alias %q target %d: %v", cleanAlias, existing.ContentID, getErr)
				}
				continue
			}
			if imp.logger != nil {
				imp.logger.Info("alias %q already points at live content %d; keeping it", cleanAlias, existing.ContentID)
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
