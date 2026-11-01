package sitessr

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"

	"html/template"

	"github.com/aristorinjuang/lesstruct/kernel/customfield"
	"github.com/aristorinjuang/lesstruct/kernel/i18n"
	"github.com/aristorinjuang/lesstruct/kernel/legal"
	"github.com/aristorinjuang/lesstruct/kernel/sanitize"
	"github.com/aristorinjuang/lesstruct/kernel/seo"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
)

// absoluteImageURL absolutizes a media URL for contexts that require it
// (og:image, twitter:image, JSON-LD). Absolute inputs pass through unchanged,
// so legacy rows and S3-hosted files are untouched; without a base URL the
// value is returned as-is. An empty input stays empty — emitting the bare
// site URL as og:image would both bypass the theme's default-image fallback
// and point crawlers at a non-fetchable resource.
// ResourceUser is the [[public_field]] resource name for user fields, matching
// the config registry's vocabulary.
const ResourceUser = "user"

// primaryLanguage returns the first configured language, defaulting to "en".
func primaryLanguage(languages []string) string {
	if len(languages) == 0 {
		return "en"
	}
	return languages[0]
}

func declaresPostScript(pt posttype.PostType) bool {
	for _, f := range pt.Fields {
		if f.Slug == customfield.PostScriptSlug {
			return true
		}
	}
	for _, f := range pt.SystemFields {
		if f.Slug == customfield.PostScriptSlug {
			return true
		}
	}
	return false
}

// galleryEntry is one coerced gallery item before media resolution.
type galleryEntry struct {
	url string
	alt string
}

// coerceGalleryEntries normalizes a stored gallery value into URL/alt pairs:
// the canonical []any of {url, alt} objects, a single {url, alt} object, a
// legacy newline-separated string, or a plain []string. Blank URLs and
// malformed items are dropped.
func coerceGalleryEntries(raw any) []galleryEntry {
	switch v := raw.(type) {
	case string:
		var out []galleryEntry
		for line := range strings.SplitSeq(v, "\n") {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				out = append(out, galleryEntry{url: trimmed})
			}
		}
		return out
	case []string:
		var out []galleryEntry
		for _, line := range v {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				out = append(out, galleryEntry{url: trimmed})
			}
		}
		return out
	case map[string]any:
		rawURL, ok := v["url"].(string)
		if !ok || strings.TrimSpace(rawURL) == "" {
			return nil
		}
		alt, _ := v["alt"].(string)
		return []galleryEntry{{url: strings.TrimSpace(rawURL), alt: alt}}
	case map[string]string:
		if strings.TrimSpace(v["url"]) == "" {
			return nil
		}
		return []galleryEntry{{url: strings.TrimSpace(v["url"]), alt: v["alt"]}}
	case []any:
		var out []galleryEntry
		for _, item := range v {
			switch entry := item.(type) {
			case string:
				if trimmed := strings.TrimSpace(entry); trimmed != "" {
					out = append(out, galleryEntry{url: trimmed})
				}
			case map[string]any:
				rawURL, ok := entry["url"].(string)
				if !ok || strings.TrimSpace(rawURL) == "" {
					continue
				}
				alt, _ := entry["alt"].(string)
				out = append(out, galleryEntry{url: strings.TrimSpace(rawURL), alt: alt})
			case map[string]string:
				if strings.TrimSpace(entry["url"]) == "" {
					continue
				}
				out = append(out, galleryEntry{url: strings.TrimSpace(entry["url"]), alt: entry["alt"]})
			default:
				continue
			}
		}
		return out
	default:
		return nil
	}
}

// HomepageSection mirrors one [homepage_section] block from config.toml. Core
// maps its config type onto this at construction.
type HomepageSection struct {
	PostType      string
	Limit         int
	Offset        int
	Title         string
	TitleID       string
	Description   string
	DescriptionID string
}

type DataAssembler struct {
	contentService      ContentReader
	postTypeResolver    PostTypeResolver
	userFieldResolver   UserFieldResolver
	userProvider        UserProvider
	renderer            BodyRenderer
	mediaRepo           mediadomain.Repository
	languages           []string
	homepageSections    []HomepageSection
	siteConfig          SiteConfig
	postsPerPage        int
	publicFieldRegistry PublicFieldLookup
	iframeHosts         []string
	baseURL             string
	uiCatalog           *i18n.Catalog
	legalOverrides      map[legal.Page]string
	legalNav            legal.NavPlacement
	checkoutConsent     bool
	registrationConsent bool
	cookieNotice        legal.NoticeCopy
}

func (a *DataAssembler) absoluteImageURL(imageURL string) string {
	if imageURL == "" || a.baseURL == "" {
		return imageURL
	}
	return seo.BuildURL(a.baseURL, imageURL)
}

func (a *DataAssembler) resolvePostImage(imageURL string) (thumbURL, mobileURL, srcset, sizes string, variants map[string]string, originalURL string) {
	if a.mediaRepo == nil || imageURL == "" {
		return imageURL, imageURL, "", "", nil, imageURL
	}
	hash := ExtractHashFromURL(imageURL)
	if hash == "" {
		return imageURL, imageURL, "", "", nil, imageURL
	}
	m, err := a.mediaRepo.FindByHashPrefix(context.Background(), hash)
	if err != nil {
		log.Printf("WARNING: resolvePostImage FindByHashPrefix failed for hash %q: %v", hash, err)
		return imageURL, imageURL, "", "", nil, imageURL
	}
	if m == nil {
		return imageURL, imageURL, "", "", nil, imageURL
	}
	originalURL = m.URL
	if len(m.Variants) > 0 {
		variants = make(map[string]string, len(m.Variants))
		for k, v := range m.Variants {
			variants[k] = v.URL
		}
	}
	srcset = buildImageSrcset(m.Variants)
	if srcset != "" {
		sizes = postCardSizes
		// Cards prefer the medium variant when the thumbnail pipeline
		// generated one: at 64/280/616 defaults the medium (280) fits the
		// card frame better than the 64px thumb. The thumb stays the
		// fallback, then large for medium-less configs; without any
		// variant, the stored URL renders as-is.
		if medium, ok := m.Variants["_medium"]; ok && medium.URL != "" {
			thumbURL = medium.URL
		} else if thumb, ok := m.Variants["_thumb"]; ok && thumb.URL != "" {
			thumbURL = thumb.URL
		} else if large, ok := m.Variants["_large"]; ok && large.URL != "" {
			thumbURL = large.URL
		} else {
			thumbURL = imageURL
		}
		// Mobile viewports (<=767px) never load the large rendition: the
		// card slot is ~342 CSS px, but a 2-3x DPR phone resolves the 616w
		// srcset entry. The mobile source is medium where generated, thumb
		// otherwise — never large.
		if medium, ok := m.Variants["_medium"]; ok && medium.URL != "" {
			mobileURL = medium.URL
		} else if thumb, ok := m.Variants["_thumb"]; ok && thumb.URL != "" {
			mobileURL = thumb.URL
		} else {
			mobileURL = thumbURL
		}
	} else {
		thumbURL = imageURL
		mobileURL = imageURL
	}
	return thumbURL, mobileURL, srcset, sizes, variants, originalURL
}

// resolveGalleryImage resolves one gallery entry against the media
// repository. Variant URLs are matched by suffix name (_thumb/_medium/_large)
// so they adapt to any [[thumbnail]] config; each falls back to the original
// URL when the record is missing or lacks that variant, and external URLs
// (unresolvable hashes) render as-is.
func (a *DataAssembler) resolveGalleryImage(rawURL, alt string) GalleryImage {
	original := rawURL
	fallback := GalleryImage{
		URL: original, Alt: alt,
		Thumb: original, Medium: original, Large: original,
		Original: original,
	}
	if a.mediaRepo == nil || rawURL == "" {
		return fallback
	}
	hash := ExtractHashFromURL(rawURL)
	if hash == "" {
		return fallback
	}
	m, err := a.mediaRepo.FindByHashPrefix(context.Background(), hash)
	if err != nil {
		log.Printf("WARNING: resolveGalleryImage FindByHashPrefix failed for hash %q: %v", hash, err)
		return fallback
	}
	if m == nil {
		return fallback
	}
	if m.URL != "" {
		original = m.URL
	}
	at := func(suffix string) string {
		if v, ok := m.Variants[suffix]; ok && v.URL != "" {
			return v.URL
		}
		return original
	}
	return GalleryImage{
		URL:      original,
		Alt:      alt,
		Thumb:    at("_thumb"),
		Medium:   at("_medium"),
		Large:    at("_large"),
		Srcset:   buildImageSrcset(m.Variants),
		Original: original,
	}
}

// resolveGalleryField coerces a stored gallery value and resolves every
// entry. It returns nil when the value carries no usable URLs.
func (a *DataAssembler) resolveGalleryField(raw any) []GalleryImage {
	entries := coerceGalleryEntries(raw)
	if len(entries) == 0 {
		return nil
	}
	images := make([]GalleryImage, 0, len(entries))
	for _, e := range entries {
		images = append(images, a.resolveGalleryImage(e.url, e.alt))
	}
	return images
}

func (a *DataAssembler) isPostTypeSlug(slug string) bool {
	if a.postTypeResolver == nil {
		return false
	}
	_, err := a.postTypeResolver.GetBySlug(slug)
	return err == nil
}

// preferredPages keeps one page per translation group for navigation,
// preferring configured languages in config order — a page that only exists
// in a secondary language still gets a nav slot instead of disappearing.
// Pages written in unconfigured languages are always dropped, matching the
// repository-level listing behaviour. Input order is preserved.
func (a *DataAssembler) preferredPages(pages []ContentRef) []ContentRef {
	if len(pages) == 0 {
		return nil
	}

	rank := make(map[string]int, len(a.languages))
	for i, lang := range a.languages {
		rank[lang] = i
	}

	best := make(map[int]ContentRef, len(pages))
	for _, page := range pages {
		pageRank, ok := rank[page.Language]
		if !ok {
			continue
		}
		current, ok := best[pageGroupKey(page)]
		if !ok || pageRank < rank[current.Language] {
			best[pageGroupKey(page)] = page
		}
	}

	chosen := make([]ContentRef, 0, len(best))
	for _, page := range pages {
		if best[pageGroupKey(page)].ID == page.ID && best[pageGroupKey(page)].Language == page.Language {
			chosen = append(chosen, page)
		}
	}
	return chosen
}

func (a *DataAssembler) buildLanguageLinks(ctx context.Context, content ContentRef, currentLang string) []LanguageLink {
	if len(a.languages) <= 1 {
		return nil
	}

	primaryLang := a.languages[0]

	groupID := content.ID
	if content.TranslationGroupID != nil {
		groupID = *content.TranslationGroupID
	}

	translations, err := a.contentService.GetTranslations(ctx, groupID, content.ID)
	if err != nil {
		log.Printf("failed to get translations for group %d: %v", groupID, err)
	}

	transByLang := make(map[string]ContentRef)
	for _, t := range translations {
		transByLang[t.Language] = t
	}

	if content.Language != primaryLang {
		if primary, err := a.contentService.GetPublishedByID(ctx, groupID); err == nil {
			transByLang[primary.Language] = primary
		}
	} else {
		transByLang[content.Language] = content
	}

	links := make([]LanguageLink, 0, len(a.languages)-1)
	for _, lang := range a.languages {
		if lang == currentLang {
			continue
		}
		if trans, ok := transByLang[lang]; ok {
			links = append(links, LanguageLink{
				Code: lang,
				Name: displayLanguage(lang),
				URL:  "/" + trans.Slug,
			})
		}
	}

	return links
}

func (a *DataAssembler) buildHomeSections(ctx context.Context) []HomeSection {
	if len(a.homepageSections) == 0 {
		return nil
	}
	lang := a.preferredLang()
	sections := make([]HomeSection, 0, len(a.homepageSections))
	for _, hs := range a.homepageSections {
		limit := hs.Limit
		if limit <= 0 {
			limit = defaultHomeSectionLimit
		}
		contents, err := a.contentService.GetPublishedByPostType(ctx, hs.PostType, a.languages, 0, 0, limit, hs.Offset)
		if err != nil {
			log.Printf("failed to get homepage section %q: %v", hs.PostType, err)
			continue
		}
		if len(contents) == 0 {
			continue
		}
		title := ""
		if lang == "id" && strings.TrimSpace(hs.TitleID) != "" {
			title = hs.TitleID
		} else if strings.TrimSpace(hs.Title) != "" {
			title = hs.Title
		}
		description := ""
		if lang == "id" && strings.TrimSpace(hs.DescriptionID) != "" {
			description = hs.DescriptionID
		} else if strings.TrimSpace(hs.Description) != "" {
			description = hs.Description
		}
		url := "/" + hs.PostType
		if a.postTypeResolver != nil {
			if resolved, resolveErr := a.postTypeResolver.GetBySlug(hs.PostType); resolveErr == nil {
				if title == "" {
					title = a.localizedPostTypeName(resolved, lang)
				}
				if description == "" {
					description = a.localizedPostTypeDescription(resolved, lang)
				}
			}
		}
		if title == "" {
			title = hs.PostType
		}
		posts := make([]PostItem, 0, len(contents))
		for _, c := range contents {
			posts = append(posts, a.buildPostItem(ctx, c))
		}
		sections = append(sections, HomeSection{
			PostTypeSlug: hs.PostType,
			Title:        title,
			Description:  description,
			URL:          url,
			Posts:        posts,
		})
	}
	return sections
}

// listImageURL returns the image that list cards and list-page og:image tags
// should use: the explicit FeaturedImage first, then the first body image
// (format-aware), or empty when neither exists.
func (a *DataAssembler) listImageURL(c ContentRef) string {
	if trimmed := strings.TrimSpace(c.FeaturedImage); trimmed != "" {
		return trimmed
	}
	if c.Format == FormatHTML {
		return seo.ExtractImageURLFromHTML(c.Content)
	}
	return seo.ExtractImageURL(c.Content)
}

func (a *DataAssembler) buildPostItem(ctx context.Context, c ContentRef) PostItem {
	imageURL := a.listImageURL(c)
	thumbURL, mobileURL, imageSrcset, imageSizes, imageVariants, originalURL := a.resolvePostImage(imageURL)

	var authorAvatarURL string
	if a.userProvider != nil && c.Username != "" {
		if user, err := a.userProvider.GetUserByUsername(ctx, c.Username); err == nil && user != nil {
			authorAvatarURL = user.ProfilePicture
		}
	}

	customFields := c.CustomFields
	if customFields == nil {
		customFields = map[string]any{}
	}

	return PostItem{
		Slug:            c.Slug,
		Title:           c.Title,
		MetaDescription: c.MetaDescription,
		ImageURL:        thumbURL,
		ImageSrcset:     imageSrcset,
		ImageSizes:      imageSizes,
		ImageVariants:   imageVariants,
		ImageMobileURL:  mobileURL,
		OriginalURL:     originalURL,
		Author:          c.Author,
		Username:        c.Username,
		AuthorAvatarURL: authorAvatarURL,
		CreatedAt:       c.CreatedAt,
		PostType:        c.PostType,
		Tags:            c.Tags,
		CustomFields:    customFields,
	}
}

func (a *DataAssembler) exposedSystemFieldSchemas(values map[string]any) []customfield.FieldSchema {
	if a.userFieldResolver == nil || a.publicFieldRegistry == nil || len(values) == 0 {
		return nil
	}

	exposed := a.publicFieldRegistry.ExposedFields(ResourceUser, "")
	if len(exposed) == 0 {
		return nil
	}

	allSystem := a.userFieldResolver.GetUserSystemFields()
	if len(allSystem) == 0 {
		return nil
	}

	exposedSet := make(map[string]bool, len(exposed))
	for _, slug := range exposed {
		exposedSet[slug] = true
	}

	filtered := make([]customfield.FieldSchema, 0, len(exposed))
	for _, sf := range allSystem {
		if exposedSet[sf.Slug] {
			filtered = append(filtered, sf)
		}
	}

	return filtered
}

func (a *DataAssembler) PrimaryLanguage() string {
	return primaryLanguage(a.languages)
}

func (a *DataAssembler) SiteConfig() SiteConfig {
	return a.siteConfig
}

func (a *DataAssembler) Languages() []string {
	return a.languages
}

// forRequest returns a shallow copy of the assembler with the negotiated
// language chain. Listings built from the copy prefer the visitor's language
// first; the shared assembler keeps the configured order for the next request.
func (a *DataAssembler) forRequest(languages []string) *DataAssembler {
	next := *a
	next.languages = append([]string{}, languages...)
	return &next
}

// siteLanguageLinks builds the site-wide language switcher for the given
// path: one entry per configured language pointing at the same path with
// ?lang set, so a switch re-renders the listing in that language or redirects
// a post to its translation. Existing query state (page, year, month) is
// preserved; only the lang parameter is replaced.
func (a *DataAssembler) siteLanguageLinks(currentPath string, query url.Values) []LanguageLink {
	if len(a.languages) <= 1 {
		return nil
	}
	if currentPath == "" {
		currentPath = "/"
	}
	links := make([]LanguageLink, 0, len(a.languages))
	for _, lang := range a.languages {
		q := url.Values{}
		for key, values := range query {
			q[key] = append([]string{}, values...)
		}
		q.Set("lang", lang)
		links = append(links, LanguageLink{
			Code: lang,
			Name: displayLanguage(lang),
			URL:  currentPath + "?" + q.Encode(),
		})
	}
	return links
}

func (a *DataAssembler) ContentService() ContentReader {
	return a.contentService
}

func (a *DataAssembler) Renderer() BodyRenderer {
	return a.renderer
}

func (a *DataAssembler) PostsPerPage() int {
	if a.postsPerPage <= 0 {
		return defaultPostsPerPage
	}
	return a.postsPerPage
}

func (a *DataAssembler) WithPublicFieldRegistry(registry PublicFieldLookup) *DataAssembler {
	a.publicFieldRegistry = registry
	return a
}

// WithIFrameHosts attaches the sanitizer's iframe host allowlist (derived from
// the CSP frame-src directive) so HTML-format content can render allowed
// embeds on the read path. When not called, iframes stay stripped.
func (a *DataAssembler) WithIFrameHosts(hosts ...string) *DataAssembler {
	a.iframeHosts = append(a.iframeHosts, hosts...)
	return a
}

// WithBaseURL sets the site's canonical base URL (SITE_URL), used only to
// absolutize media URLs in SEO contexts (og:image/twitter:image). Body images,
// cards and srcsets intentionally keep storage-native URLs — relative for the
// local driver, absolute for S3/MinIO.
func (a *DataAssembler) WithBaseURL(baseURL string) *DataAssembler {
	a.baseURL = strings.TrimRight(baseURL, "/")
	return a
}

// WithUICatalog attaches the i18n UI catalog used for engine-generated listing
// strings (browse/posts-by/posts-tagged). When nil, English hardcodes apply;
// when set, the preferred language wins with English fallback.
func (a *DataAssembler) WithUICatalog(catalog *i18n.Catalog) *DataAssembler {
	a.uiCatalog = catalog
	return a
}

func (a *DataAssembler) preferredLang() string {
	return primaryLanguage(a.languages)
}

func (a *DataAssembler) uiFormat(lang, key, fallback string) string {
	if a.uiCatalog == nil {
		return fallback
	}
	if formatted := a.uiCatalog.T(lang, key); formatted != "" && formatted != key {
		return formatted
	}
	return fallback
}

func (a *DataAssembler) localizedPostTypeName(pt posttype.PostType, lang string) string {
	if lang == "id" && strings.TrimSpace(pt.NameID) != "" {
		return pt.NameID
	}
	return pt.Name
}

func (a *DataAssembler) localizedPostTypeDescription(pt posttype.PostType, lang string) string {
	if lang == "id" && strings.TrimSpace(pt.DescriptionID) != "" {
		return pt.DescriptionID
	}
	return pt.Description
}

func (a *DataAssembler) BuildContentData(ctx context.Context, slug string) (ContentData, error) {
	content, err := a.contentService.GetPublishedBySlugAny(ctx, slug)
	if err != nil {
		return ContentData{}, err
	}

	lang := content.Language
	if lang == "" {
		lang = "en"
	}

	var bodyHTML string
	switch content.Format {
	case FormatHTML:
		bodyHTML = sanitize.SanitizeHTMLDocument(content.Content, a.iframeHosts...)
	default:
		bodyHTML, err = a.renderer.Render(content.Content)
		if err != nil {
			bodyHTML = ""
		}
	}

	ogTitle := content.OGTitle
	if ogTitle == "" {
		ogTitle = content.Title
	}

	ogDesc := content.OGDescription
	if ogDesc == "" {
		ogDesc = content.MetaDescription
	}

	// Explicit featured image wins; the body-derived image is the fallback.
	var featuredImage string
	if content.FeaturedImage != "" {
		featuredImage = content.FeaturedImage
	} else {
		switch content.Format {
		case FormatHTML:
			featuredImage = seo.ExtractImageURLFromHTML(content.Content)
		default:
			featuredImage = seo.ExtractImageURL(content.Content)
		}
	}

	currentPath := "/" + slug
	pageChrome := a.buildChrome(ctx, currentPath)

	var formattedFields []FormattedField
	var postScripts template.HTML
	var galleries map[string][]GalleryImage
	if a.postTypeResolver != nil && content.PostType != "" {
		if pt, ptErr := a.postTypeResolver.GetBySlug(content.PostType); ptErr == nil {
			if content.CustomFields != nil {
				formattedFields = formatCustomFields(pt.Fields, content.CustomFields, lang)
				formattedFields = append(formattedFields,
					formatCustomFields(pt.SystemFields, content.CustomFields, lang)...)
				schemas := make([]customfield.FieldSchema, 0, len(pt.Fields)+len(pt.SystemFields))
				schemas = append(schemas, pt.Fields...)
				schemas = append(schemas, pt.SystemFields...)
				for _, f := range schemas {
					if f.Type != customfield.FieldTypeGallery {
						continue
					}
					raw, ok := content.CustomFields[f.Slug]
					if !ok {
						continue
					}
					images := a.resolveGalleryField(raw)
					if len(images) == 0 {
						continue
					}
					if galleries == nil {
						galleries = make(map[string][]GalleryImage)
					}
					galleries[f.Slug] = images
				}
				if declaresPostScript(pt) {
					if raw, ok := content.CustomFields[customfield.PostScriptSlug]; ok {
						if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
							postScripts = template.HTML(s)
						}
					}
				}
			}
		}
	}

	var commentItems []CommentItem
	if content.AllowComments {
		comments, err := a.contentService.GetCommentsForContent(ctx, content.ID)
		if err != nil {
			log.Printf("failed to get comments for content %d: %v", content.ID, err)
		}
		for _, c := range comments {
			commentItems = append(commentItems, CommentItem{
				Author:    c.Author,
				Text:      c.Comment,
				CreatedAt: c.CreatedAt,
			})
		}
	}

	relatedItems := make([]PostItem, 0)
	if related, err := a.contentService.GetRelated(ctx, content.ID, 4); err != nil {
		log.Printf("failed to get related content for content %d: %v", content.ID, err)
	} else {
		for _, c := range related {
			relatedItems = append(relatedItems, a.buildPostItem(ctx, c))
		}
	}

	var authorAvatarURL string
	if a.userProvider != nil && content.Username != "" {
		if user, userErr := a.userProvider.GetUserByUsername(ctx, content.Username); userErr == nil && user != nil {
			authorAvatarURL = user.ProfilePicture
		}
	}

	languageLinks := a.buildLanguageLinks(ctx, content, lang)

	return ContentData{
		LayoutData: LayoutData{
			Title:           content.Title,
			Description:     content.MetaDescription,
			PageTitle:       fmt.Sprintf("%s - %s", content.Title, a.siteConfig.Name),
			OGTitle:         ogTitle,
			OGDesc:          ogDesc,
			OGImage:         a.absoluteImageURL(featuredImage),
			NavigationItems: pageChrome.Header,
			FooterItems:     pageChrome.Footer,
			Legal:           pageChrome.Legal,
			CurrentPath:     currentPath,
			Lang:            lang,
			LanguageLinks:   languageLinks,
			SiteConfig:      a.siteConfig,
		},
		Slug:                  content.Slug,
		Body:                  template.HTML(bodyHTML),
		Tags:                  content.Tags,
		Author:                content.Author,
		Username:              content.Username,
		AuthorAvatarURL:       authorAvatarURL,
		CreatedAt:             content.CreatedAt,
		AllowComments:         content.AllowComments,
		CustomFields:          content.CustomFields,
		CustomFieldsFormatted: formattedFields,
		CustomFieldGalleries:  galleries,
		Related:               relatedItems,
		Comments:              commentItems,
		PostType:              content.PostType,
		PostScripts:           postScripts,
	}, nil
}

func (a *DataAssembler) BuildHomeData(ctx context.Context, page, year, month int) (IndexData, error) {
	primaryLang := primaryLanguage(a.languages)
	perPage := a.PostsPerPage()
	offset := (page - 1) * perPage

	contents, err := a.contentService.GetPublishedByPostType(ctx, "post", a.languages, year, month, perPage+1, offset)
	if err != nil {
		return IndexData{}, err
	}
	contents, hasNext := trimToPage(contents, perPage)

	var ogImage string
	posts := make([]PostItem, 0, len(contents))
	for _, c := range contents {
		if imageURL := a.listImageURL(c); imageURL != "" && ogImage == "" {
			ogImage = imageURL
		}
		posts = append(posts, a.buildPostItem(ctx, c))
	}

	tags, tagsErr := a.contentService.GetPublishedTags(ctx)
	if tagsErr != nil {
		log.Printf("failed to get published tags for index: %v", tagsErr)
		tags = nil
	}

	sections := a.buildHomeSections(ctx)

	currentPath := "/"
	pageChrome := a.buildChrome(ctx, currentPath)

	return IndexData{
		LayoutData: LayoutData{
			Title:           a.siteConfig.Name,
			PageTitle:       a.siteConfig.Name,
			OGImage:         a.absoluteImageURL(ogImage),
			NavigationItems: pageChrome.Header,
			FooterItems:     pageChrome.Footer,
			Legal:           pageChrome.Legal,
			CurrentPath:     currentPath,
			Lang:            primaryLang,
			SiteConfig:      a.siteConfig,
		},
		Posts:          posts,
		Tags:           tags,
		Sections:       sections,
		PaginationData: buildPagination(page, hasNext, currentPath, archiveQuery(year, month)),
	}, nil
}

func (a *DataAssembler) BuildIndexData(ctx context.Context, postTypeSlug string, page, year, month int) (IndexData, error) {
	primaryLang := primaryLanguage(a.languages)
	perPage := a.PostsPerPage()
	offset := (page - 1) * perPage

	contents, err := a.contentService.GetPublishedByPostType(ctx, postTypeSlug, a.languages, year, month, perPage+1, offset)
	if err != nil {
		return IndexData{}, err
	}
	contents, hasNext := trimToPage(contents, perPage)

	var resolved posttype.PostType
	resolveErr := error(nil)
	if a.postTypeResolver != nil {
		resolved, resolveErr = a.postTypeResolver.GetBySlug(postTypeSlug)
	}
	pageTitle := postTypeSlug
	if resolveErr == nil && resolved.Name != "" {
		pageTitle = a.localizedPostTypeName(resolved, primaryLang)
	}

	posts := make([]PostItem, 0, len(contents))
	var ogImage string
	for _, c := range contents {
		if imageURL := a.listImageURL(c); imageURL != "" && ogImage == "" {
			ogImage = imageURL
		}
		posts = append(posts, a.buildPostItem(ctx, c))
	}

	currentPath := "/" + postTypeSlug
	pageChrome := a.buildChrome(ctx, currentPath)

	browseFormat := a.uiFormat(primaryLang, "ui.browse_posts", "Browse %s.")
	browseText := browseFormat
	if strings.Contains(browseFormat, "%s") || strings.Contains(browseFormat, "%q") || strings.Contains(browseFormat, "%d") {
		browseText = fmt.Sprintf(browseFormat, pageTitle)
	}

	return IndexData{
		LayoutData: LayoutData{
			Title:           pageTitle,
			PageTitle:       fmt.Sprintf("%s - %s", pageTitle, a.siteConfig.Name),
			Description:     browseText,
			OGDesc:          browseText,
			OGImage:         a.absoluteImageURL(ogImage),
			NavigationItems: pageChrome.Header,
			FooterItems:     pageChrome.Footer,
			Legal:           pageChrome.Legal,
			CurrentPath:     currentPath,
			Lang:            primaryLang,
			SiteConfig:      a.siteConfig,
		},
		Posts:          posts,
		PaginationData: buildPagination(page, hasNext, currentPath, archiveQuery(year, month)),
	}, nil
}

func (a *DataAssembler) BuildAuthorData(ctx context.Context, username string, page int) (AuthorData, error) {
	exists, err := a.contentService.AuthorExists(ctx, username)
	if err != nil || !exists {
		return AuthorData{}, fmt.Errorf("author not found: %s", username)
	}

	primaryLang := primaryLanguage(a.languages)
	perPage := a.PostsPerPage()
	offset := (page - 1) * perPage

	contents, err := a.contentService.GetPublishedByAuthorUsername(ctx, username, a.languages, perPage+1, offset)
	if err != nil {
		return AuthorData{}, err
	}
	contents, hasNext := trimToPage(contents, perPage)

	authorName := ""
	var ogImage string
	posts := make([]PostItem, 0, len(contents))
	for _, c := range contents {
		if authorName == "" {
			authorName = c.Author
		}
		if imageURL := a.listImageURL(c); imageURL != "" && ogImage == "" {
			ogImage = imageURL
		}
		posts = append(posts, a.buildPostItem(ctx, c))
	}

	if authorName == "" {
		authorName = username
	}

	var formattedFields []FormattedField
	var authorAvatarURL string
	var authorUser *UserBasicInfo
	if a.userProvider != nil {
		if user, userErr := a.userProvider.GetUserByUsername(ctx, username); userErr == nil && user != nil {
			authorUser = user
		}
	}
	if authorUser != nil {
		authorAvatarURL = authorUser.ProfilePicture
		if a.userFieldResolver != nil {
			userFields := a.userFieldResolver.GetUserFields()
			if len(userFields) > 0 && len(authorUser.CustomFields) > 0 {
				formattedFields = formatCustomFields(userFields, authorUser.CustomFields, primaryLang)
			}

			exposed := a.exposedSystemFieldSchemas(authorUser.CustomFields)
			if len(exposed) > 0 {
				formattedFields = append(formattedFields, formatCustomFields(exposed, authorUser.CustomFields, primaryLang)...)
			}
		}
	}

	currentPath := "/authors/" + username
	pageChrome := a.buildChrome(ctx, currentPath)

	byFormat := a.uiFormat(primaryLang, "ui.posts_by", "Posts by %s.")
	byText := byFormat
	if strings.Contains(byFormat, "%s") || strings.Contains(byFormat, "%q") || strings.Contains(byFormat, "%d") {
		byText = fmt.Sprintf(byFormat, authorName)
	}

	return AuthorData{
		LayoutData: LayoutData{
			Title:           authorName,
			PageTitle:       fmt.Sprintf("%s - %s", authorName, a.siteConfig.Name),
			Description:     byText,
			OGDesc:          byText,
			OGImage:         a.absoluteImageURL(ogImage),
			NavigationItems: pageChrome.Header,
			FooterItems:     pageChrome.Footer,
			Legal:           pageChrome.Legal,
			CurrentPath:     currentPath,
			Lang:            primaryLang,
			SiteConfig:      a.siteConfig,
		},
		AuthorName:            authorName,
		Username:              username,
		AuthorAvatarURL:       authorAvatarURL,
		Posts:                 posts,
		CustomFieldsFormatted: formattedFields,
		PaginationData:        buildPagination(page, hasNext, currentPath, ""),
	}, nil
}

func (a *DataAssembler) BuildTagData(ctx context.Context, tag string, page, year, month int) (TagData, error) {
	if tag == "" {
		return TagData{}, fmt.Errorf("tag is empty")
	}

	primaryLang := primaryLanguage(a.languages)
	perPage := a.PostsPerPage()
	offset := (page - 1) * perPage

	contents, err := a.contentService.GetPublishedByTag(ctx, tag, a.languages, year, month, perPage+1, offset)
	if err != nil {
		return TagData{}, err
	}
	contents, hasNext := trimToPage(contents, perPage)

	posts := make([]PostItem, 0, len(contents))
	var ogImage string
	for _, c := range contents {
		if imageURL := a.listImageURL(c); imageURL != "" && ogImage == "" {
			ogImage = imageURL
		}
		posts = append(posts, a.buildPostItem(ctx, c))
	}

	currentPath := "/tags/" + tag
	pageChrome := a.buildChrome(ctx, currentPath)

	taggedFormat := a.uiFormat(primaryLang, "ui.posts_tagged", "Posts tagged %q.")
	taggedText := taggedFormat
	if strings.Contains(taggedFormat, "%s") || strings.Contains(taggedFormat, "%q") || strings.Contains(taggedFormat, "%d") {
		taggedText = fmt.Sprintf(taggedFormat, tag)
	}

	return TagData{
		LayoutData: LayoutData{
			Title:           tag,
			PageTitle:       fmt.Sprintf("%s - %s", tag, a.siteConfig.Name),
			Description:     taggedText,
			OGDesc:          taggedText,
			OGImage:         a.absoluteImageURL(ogImage),
			NavigationItems: pageChrome.Header,
			FooterItems:     pageChrome.Footer,
			Legal:           pageChrome.Legal,
			CurrentPath:     currentPath,
			Lang:            primaryLang,
			SiteConfig:      a.siteConfig,
		},
		TagName:        tag,
		Posts:          posts,
		PaginationData: buildPagination(page, hasNext, currentPath, archiveQuery(year, month)),
	}, nil
}

func (a *DataAssembler) BuildNotFoundData(ctx context.Context, currentPath string) NotFoundData {
	pageChrome := a.buildChrome(ctx, currentPath)

	return NotFoundData{
		LayoutData: LayoutData{
			Title:           "Not Found",
			PageTitle:       fmt.Sprintf("Not Found - %s", a.siteConfig.Name),
			NavigationItems: pageChrome.Header,
			FooterItems:     pageChrome.Footer,
			Legal:           pageChrome.Legal,
			Lang:            primaryLanguage(a.languages),
			SiteConfig:      a.siteConfig,
		},
	}
}

func NewDataAssembler(
	contentService ContentReader,
	postTypeResolver PostTypeResolver,
	userFieldResolver UserFieldResolver,
	userProvider UserProvider,
	renderer BodyRenderer,
	mediaRepo mediadomain.Repository,
	languages []string,
	homepageSections []HomepageSection,
	siteConfig SiteConfig,
	postsPerPage int,
) *DataAssembler {
	if siteConfig.Name == "" {
		siteConfig.Name = defaultSiteName
	}
	return &DataAssembler{
		contentService:    contentService,
		postTypeResolver:  postTypeResolver,
		userFieldResolver: userFieldResolver,
		userProvider:      userProvider,
		renderer:          renderer,
		mediaRepo:         mediaRepo,
		languages:         languages,
		homepageSections:  homepageSections,
		siteConfig: SiteConfig{
			Name:              siteConfig.Name,
			Logo:              siteConfig.Logo,
			Favicon:           siteConfig.Favicon,
			ProductImageRatio: siteConfig.ProductImageRatio,
			HeroImage:         siteConfig.HeroImage,
			HeroTitle:         siteConfig.HeroTitle,
			HeroSubtitle:      siteConfig.HeroSubtitle,
		},
		postsPerPage: postsPerPage,
	}
}
