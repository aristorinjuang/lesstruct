package sitessr

import (
	"context"
	"fmt"
	"log"
	"strings"

	"html/template"

	"github.com/aristorinjuang/lesstruct/kernel/customfield"
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

// HomepageSection mirrors one [homepage_section] block from config.toml. Core
// maps its config type onto this at construction.
type HomepageSection struct {
	PostType string
	Limit    int
	Offset   int
	Title    string
}

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
}

func (a *DataAssembler) absoluteImageURL(imageURL string) string {
	if imageURL == "" || a.baseURL == "" {
		return imageURL
	}
	return seo.BuildURL(a.baseURL, imageURL)
}

func (a *DataAssembler) resolvePostImage(imageURL string) (thumbURL, srcset, sizes string, variants map[string]string, originalURL string) {
	if a.mediaRepo == nil || imageURL == "" {
		return imageURL, "", "", nil, imageURL
	}
	hash := ExtractHashFromURL(imageURL)
	if hash == "" {
		return imageURL, "", "", nil, imageURL
	}
	m, err := a.mediaRepo.FindByHashPrefix(context.Background(), hash)
	if err != nil {
		log.Printf("WARNING: resolvePostImage FindByHashPrefix failed for hash %q: %v", hash, err)
		return imageURL, "", "", nil, imageURL
	}
	if m == nil {
		return imageURL, "", "", nil, imageURL
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
		if thumb, ok := m.Variants["_thumb"]; ok {
			thumbURL = thumb.URL
		} else {
			thumbURL = imageURL
		}
	} else {
		thumbURL = imageURL
	}
	return thumbURL, srcset, sizes, variants, originalURL
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

	groupKey := func(page ContentRef) int {
		if page.TranslationGroupID != nil {
			return *page.TranslationGroupID
		}
		return page.ID
	}

	best := make(map[int]ContentRef, len(pages))
	for _, page := range pages {
		pageRank, ok := rank[page.Language]
		if !ok {
			continue
		}
		current, ok := best[groupKey(page)]
		if !ok || pageRank < rank[current.Language] {
			best[groupKey(page)] = page
		}
	}

	chosen := make([]ContentRef, 0, len(best))
	for _, page := range pages {
		if best[groupKey(page)].ID == page.ID && best[groupKey(page)].Language == page.Language {
			chosen = append(chosen, page)
		}
	}
	return chosen
}

func (a *DataAssembler) buildNavigationItems(ctx context.Context, currentPath string) []NavigationItem {
	items := []NavigationItem{
		{Title: "Home", URL: "/", IsActive: currentPath == "/"},
	}

	pages, err := a.contentService.GetPublishedPages(ctx)
	if err == nil {
		for _, page := range a.preferredPages(pages) {
			items = append(items, NavigationItem{
				Title:    page.Title,
				URL:      "/" + page.Slug,
				IsActive: currentPath == "/"+page.Slug,
			})
		}
	} else {
		log.Printf("failed to get published pages for navigation: %v", err)
	}

	postTypes, err := a.contentService.GetPublishedCustomPostTypes(ctx)
	if err == nil && a.postTypeResolver != nil {
		for _, pt := range postTypes {
			resolved, resolveErr := a.postTypeResolver.GetBySlug(pt)
			name := pt
			if resolveErr == nil && resolved.Name != "" {
				name = resolved.Name
			}
			items = append(items, NavigationItem{
				Title:    name,
				URL:      "/" + pt,
				IsActive: currentPath == "/"+pt,
			})
		}
	} else if err != nil {
		log.Printf("failed to get published custom post types for navigation: %v", err)
	}

	return items
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
		title := hs.Title
		description := ""
		url := "/" + hs.PostType
		if a.postTypeResolver != nil {
			if resolved, resolveErr := a.postTypeResolver.GetBySlug(hs.PostType); resolveErr == nil {
				if title == "" {
					title = resolved.Name
				}
				description = resolved.Description
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

func (a *DataAssembler) buildPostItem(ctx context.Context, c ContentRef) PostItem {
	imageURL := seo.ExtractImageURL(c.Content)
	thumbURL, imageSrcset, imageSizes, imageVariants, originalURL := a.resolvePostImage(imageURL)

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

	var featuredImage string
	switch content.Format {
	case FormatHTML:
		featuredImage = seo.ExtractImageURLFromHTML(content.Content)
	default:
		featuredImage = seo.ExtractImageURL(content.Content)
	}

	currentPath := "/" + slug
	navItems := a.buildNavigationItems(ctx, currentPath)

	var formattedFields []FormattedField
	var postScripts template.HTML
	if a.postTypeResolver != nil && content.PostType != "" {
		if pt, ptErr := a.postTypeResolver.GetBySlug(content.PostType); ptErr == nil {
			if content.CustomFields != nil {
				formattedFields = formatCustomFields(pt.Fields, content.CustomFields, lang)
				formattedFields = append(formattedFields,
					formatCustomFields(pt.SystemFields, content.CustomFields, lang)...)
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
			NavigationItems: navItems,
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
		if imageURL := seo.ExtractImageURL(c.Content); imageURL != "" && ogImage == "" {
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
	navItems := a.buildNavigationItems(ctx, currentPath)

	return IndexData{
		LayoutData: LayoutData{
			Title:           a.siteConfig.Name,
			PageTitle:       a.siteConfig.Name,
			OGImage:         a.absoluteImageURL(ogImage),
			NavigationItems: navItems,
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
		pageTitle = resolved.Name
	}

	posts := make([]PostItem, 0, len(contents))
	var ogImage string
	for _, c := range contents {
		if imageURL := seo.ExtractImageURL(c.Content); imageURL != "" && ogImage == "" {
			ogImage = imageURL
		}
		posts = append(posts, a.buildPostItem(ctx, c))
	}

	currentPath := "/" + postTypeSlug
	navItems := a.buildNavigationItems(ctx, currentPath)

	return IndexData{
		LayoutData: LayoutData{
			Title:           pageTitle,
			PageTitle:       fmt.Sprintf("%s - %s", pageTitle, a.siteConfig.Name),
			Description:     fmt.Sprintf("Browse %s.", pageTitle),
			OGDesc:          fmt.Sprintf("Browse %s.", pageTitle),
			OGImage:         a.absoluteImageURL(ogImage),
			NavigationItems: navItems,
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
		if imageURL := seo.ExtractImageURL(c.Content); imageURL != "" && ogImage == "" {
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
	navItems := a.buildNavigationItems(ctx, currentPath)

	return AuthorData{
		LayoutData: LayoutData{
			Title:           authorName,
			PageTitle:       fmt.Sprintf("%s - %s", authorName, a.siteConfig.Name),
			Description:     fmt.Sprintf("Posts by %s.", authorName),
			OGDesc:          fmt.Sprintf("Posts by %s.", authorName),
			OGImage:         a.absoluteImageURL(ogImage),
			NavigationItems: navItems,
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
		if imageURL := seo.ExtractImageURL(c.Content); imageURL != "" && ogImage == "" {
			ogImage = imageURL
		}
		posts = append(posts, a.buildPostItem(ctx, c))
	}

	currentPath := "/tags/" + tag
	navItems := a.buildNavigationItems(ctx, currentPath)

	return TagData{
		LayoutData: LayoutData{
			Title:           tag,
			PageTitle:       fmt.Sprintf("%s - %s", tag, a.siteConfig.Name),
			Description:     fmt.Sprintf("Posts tagged %q.", tag),
			OGDesc:          fmt.Sprintf("Posts tagged %q.", tag),
			OGImage:         a.absoluteImageURL(ogImage),
			NavigationItems: navItems,
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
	navItems := a.buildNavigationItems(ctx, currentPath)

	return NotFoundData{
		LayoutData: LayoutData{
			Title:           "Not Found",
			PageTitle:       fmt.Sprintf("Not Found - %s", a.siteConfig.Name),
			NavigationItems: navItems,
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
			Name: siteConfig.Name,
			Logo: siteConfig.Logo,
		},
		postsPerPage: postsPerPage,
	}
}
