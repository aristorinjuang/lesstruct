package ssg

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/contentpage"
	tpl "github.com/aristorinjuang/lesstruct/internal/api/template"

	aliasdomain "github.com/aristorinjuang/lesstruct/internal/domain/alias"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/seo"
)

const (
	ssgPageSize      = 100
	feedItemLimit    = 20
	feedExcerptRunes = 300
)

// staticAssetPattern captures the path portion of /static/... references in
// rendered HTML so the export can ship only embedded assets pages actually use.
var staticAssetPattern = regexp.MustCompile(`/static/([A-Za-z0-9._\-/]+)`)

// alwaysExportedAssets lists embedded assets copied into every export even when
// no literal reference appears in the rendered HTML: the two stylesheets are
// linked through assetURL's cache-busting form (/static/base.<hash>.css), so
// their plain names never show up in a page.
var alwaysExportedAssets = map[string]bool{
	"base.css":  true,
	"style.css": true,
}

func sanitizePathSegment(s string) string {
	if s == "" || strings.Contains(s, "/") || strings.Contains(s, "..") || s == "." {
		return "_"
	}
	return s
}

// normalizeAliasPath validates an alias as an exportable site path and returns
// it without leading/trailing slashes, or "" when it cannot be exported
// (empty, traversal markers, backslashes, or resolving to the root).
func normalizeAliasPath(alias string) string {
	trimmed := strings.TrimSpace(alias)
	if trimmed == "" || strings.Contains(trimmed, "..") || strings.Contains(trimmed, "\\") {
		return ""
	}

	cleaned := strings.Trim(path.Clean("/"+trimmed), "/")
	if cleaned == "" {
		return ""
	}

	for segment := range strings.SplitSeq(cleaned, "/") {
		if segment == "." || segment == ".." {
			return ""
		}
	}

	return cleaned
}

// aliasExportPath maps a normalized alias to its location in the archive:
// dotted aliases become flat files ("old.html"), everything else gets an
// index.html directory page ("old/path/index.html"). Returns "" for the root.
func aliasExportPath(aliasPath string) string {
	if aliasPath == "" {
		return ""
	}
	if strings.HasSuffix(aliasPath, ".html") {
		return aliasPath
	}
	return aliasPath + "/index.html"
}

// collectReferencedAssets records every /static/<path> reference found in
// rendered HTML.
func collectReferencedAssets(pageHTML string, referenced map[string]bool) {
	for _, match := range staticAssetPattern.FindAllStringSubmatch(pageHTML, -1) {
		referenced[match[1]] = true
	}
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func feedExcerpt(c *contentdomain.Content) string {
	if c.MetaDescription != "" {
		return c.MetaDescription
	}
	if c.OGDescription != "" {
		return c.OGDescription
	}
	var text string
	if c.Format == contentdomain.FormatHTML {
		text = seo.ExtractPlainTextFromHTML(c.Content)
	} else {
		text = seo.ExtractPlainText(c.Content)
	}
	return seo.TruncateText(strings.Join(strings.Fields(text), " "), feedExcerptRunes)
}

func copyFS(fsys fs.FS, destDir string) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, filepath.FromSlash(path))

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		if !d.Type().IsRegular() {
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}

		return nil
	})
}

func targzDir(sourceDir string, w io.Writer) error {
	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	err := filepath.Walk(sourceDir, func(filePath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, filePath)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if fi.IsDir() {
			header.Name += "/"
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.IsDir() {
			data, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			if _, err := tw.Write(data); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}
	return gw.Close()
}

type renderTask struct {
	path   string
	amp    bool
	render func(context.Context) (string, error)
}

type Generator struct {
	assembler  *contentpage.DataAssembler
	templates  *tpl.Templates
	contentSvc contentpage.ContentService
	aliasSvc   *aliasdomain.Service
	siteURL    string
	languages  []string
	perPage    int
	mediaDir   string
	theme      *tpl.Theme
}

func (g *Generator) pageURL(taskPath string) string {
	if taskPath == "/" {
		return g.siteURL
	}
	return g.siteURL + strings.TrimSuffix(taskPath, "/") + "/"
}

func (g *Generator) writeAMPCanonicalLinks(publicDir string, tasks []renderTask) error {
	for _, task := range tasks {
		if !task.amp {
			continue
		}
		htmlPath := filepath.Join(publicDir, task.path, "amp", "index.html")
		data, err := os.ReadFile(htmlPath)
		if err != nil {
			continue
		}
		ampHTML := string(data)
		canonical := g.pageURL(task.path)
		link := fmt.Sprintf(`<link rel="canonical" href="%s">`, canonical)
		ampHTML = strings.Replace(ampHTML, "<head>", "<head>\n  "+link, 1)
		_ = os.WriteFile(htmlPath, []byte(ampHTML), 0644)
	}
	return nil
}

func (g *Generator) enumerate(ctx context.Context) ([]renderTask, error) {
	var tasks []renderTask

	perPage := g.perPage

	page := 1
	for {
		contents, err := g.contentSvc.GetPublishedByPostType(ctx, "post", g.languages, 0, 0, perPage+1, (page-1)*perPage)
		if err != nil {
			break
		}
		items := contents
		hasNext := len(contents) > perPage
		if hasNext {
			items = contents[:perPage]
		}

		if len(items) == 0 {
			break
		}

		taskPath := "/"
		if page > 1 {
			taskPath = fmt.Sprintf("/page/%d", page)
		}

		p := page
		tasks = append(tasks, renderTask{
			path: taskPath,
			amp:  false,
			render: func(ctx context.Context) (string, error) {
				data, err := g.assembler.BuildHomeData(ctx, p, 0, 0)
				if err != nil {
					return "", err
				}
				return g.templates.RenderHomeToString(data)
			},
		})

		if !hasNext {
			break
		}
		page++
	}

	allContent, err := g.enumerateAllContent(ctx)
	if err != nil {
		return nil, err
	}

	seenPostTypes := make(map[string]bool)
	seenUsernames := make(map[string]bool)

	for _, c := range allContent {
		slug := sanitizePathSegment(c.Slug)
		if slug == "_" {
			continue
		}

		s := slug
		tasks = append(tasks, renderTask{
			path: "/" + slug,
			amp:  true,
			render: func(ctx context.Context) (string, error) {
				data, err := g.assembler.BuildContentData(ctx, s)
				if err != nil {
					return "", err
				}
				return g.templates.RenderContentToString(data)
			},
		})

		if c.PostType != "" {
			seenPostTypes[c.PostType] = true
		}
		if c.Username != "" {
			seenUsernames[c.Username] = true
		}
	}

	postTypes, err := g.contentSvc.GetPublishedCustomPostTypes(ctx)
	if err == nil {
		for _, pt := range postTypes {
			seenPostTypes[pt] = true
		}
	}

	for pt := range seenPostTypes {
		if pt == "post" || pt == "" {
			continue
		}
		ptSegment := sanitizePathSegment(pt)
		if ptSegment == "_" {
			continue
		}

		ptPage := 1
		for {
			contents, err := g.contentSvc.GetPublishedByPostType(ctx, pt, g.languages, 0, 0, perPage+1, (ptPage-1)*perPage)
			if err != nil {
				break
			}
			items := contents
			hasNext := len(contents) > perPage
			if hasNext {
				items = contents[:perPage]
			}
			if len(items) == 0 {
				break
			}

			taskPath := "/" + ptSegment
			if ptPage > 1 {
				taskPath = fmt.Sprintf("/%s/page/%d", ptSegment, ptPage)
			}

			s := pt
			p := ptPage
			tasks = append(tasks, renderTask{
				path: taskPath,
				amp:  false,
				render: func(ctx context.Context) (string, error) {
					data, err := g.assembler.BuildIndexData(ctx, s, p, 0, 0)
					if err != nil {
						return "", err
					}
					return g.templates.RenderIndexToString(data)
				},
			})

			if !hasNext {
				break
			}
			ptPage++
		}
	}

	for username := range seenUsernames {
		userSegment := sanitizePathSegment(username)
		if userSegment == "_" {
			continue
		}

		authorPage := 1
		for {
			data, err := g.assembler.BuildAuthorData(ctx, username, authorPage)
			if err != nil {
				break
			}
			taskPath := "/authors/" + userSegment
			if authorPage > 1 {
				taskPath = fmt.Sprintf("/authors/%s/page/%d", userSegment, authorPage)
			}

			u := username
			p := authorPage
			tasks = append(tasks, renderTask{
				path: taskPath,
				amp:  false,
				render: func(ctx context.Context) (string, error) {
					data, err := g.assembler.BuildAuthorData(ctx, u, p)
					if err != nil {
						return "", err
					}
					return g.templates.RenderAuthorToString(data)
				},
			})

			if !data.HasNext {
				break
			}
			authorPage++
		}
	}

	tags, err := g.contentSvc.GetPublishedTags(ctx)
	if err == nil {
		for _, tag := range tags {
			tagSegment := sanitizePathSegment(tag)
			if tagSegment == "_" {
				continue
			}

			tagPage := 1
			for {
				data, err := g.assembler.BuildTagData(ctx, tag, tagPage, 0, 0)
				if err != nil {
					break
				}
				taskPath := "/tags/" + tagSegment
				if tagPage > 1 {
					taskPath = fmt.Sprintf("/tags/%s/page/%d", tagSegment, tagPage)
				}

				t := tag
				p := tagPage
				tasks = append(tasks, renderTask{
					path: taskPath,
					amp:  false,
					render: func(ctx context.Context) (string, error) {
						data, err := g.assembler.BuildTagData(ctx, t, p, 0, 0)
						if err != nil {
							return "", err
						}
						return g.templates.RenderTagToString(data)
					},
				})

				if !data.HasNext {
					break
				}
				tagPage++
			}
		}
	}

	return tasks, nil
}

func (g *Generator) enumerateAllContent(ctx context.Context) ([]*contentdomain.Content, error) {
	var all []*contentdomain.Content
	offset := 0
	for {
		contents, err := g.contentSvc.GetPublished(ctx, ssgPageSize, offset)
		if err != nil {
			return all, err
		}
		if len(contents) == 0 {
			break
		}
		all = append(all, contents...)
		offset += len(contents)
		if len(contents) < ssgPageSize {
			break
		}
	}
	return all, nil
}

// copyEmbeddedAssets copies the embedded static layer into the export, keeping
// only assets referenced by rendered pages. Stylesheets are always exported
// (see alwaysExportedAssets) and unminified dev sources never are.
func copyEmbeddedAssets(fsys fs.FS, destDir string, referenced map[string]bool) error {
	return fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}

		name := path.Base(p)
		if strings.HasSuffix(name, ".src.css") {
			return nil
		}
		if !alwaysExportedAssets[p] && !referenced[p] {
			return nil
		}

		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}

		destPath := filepath.Join(destDir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", p, err)
		}

		return nil
	})
}

func (g *Generator) writeStaticFiles(ctx context.Context, publicDir string, referencedAssets map[string]bool) error {
	staticDir := filepath.Join(publicDir, "static")
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		return err
	}

	if err := copyEmbeddedAssets(tpl.EmbeddedStaticFS(), staticDir, referencedAssets); err != nil {
		return fmt.Errorf("copy embedded static assets: %w", err)
	}

	if themeFS := tpl.ThemeStaticFS(g.theme); themeFS != nil {
		if err := copyFS(themeFS, staticDir); err != nil {
			return fmt.Errorf("copy theme static assets: %w", err)
		}
	}

	baseCSS, styleCSS := tpl.ReadThemeStyles(g.theme)
	if baseCSS != "" {
		if err := os.WriteFile(filepath.Join(staticDir, "base.css"), []byte(baseCSS), 0644); err != nil {
			return err
		}
	}
	if styleCSS != "" {
		if err := os.WriteFile(filepath.Join(staticDir, "style.css"), []byte(styleCSS), 0644); err != nil {
			return err
		}
	}

	uploadsMediaDir := filepath.Join(publicDir, "uploads", "media")
	if err := os.MkdirAll(uploadsMediaDir, 0755); err != nil {
		return err
	}

	if g.mediaDir != "" {
		entries, err := os.ReadDir(g.mediaDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				srcPath := filepath.Join(g.mediaDir, entry.Name())
				data, err := os.ReadFile(srcPath)
				if err != nil {
					continue
				}
				if err := os.WriteFile(filepath.Join(uploadsMediaDir, entry.Name()), data, 0644); err != nil {
					continue
				}
			}
		}
	}

	// Theme root files (theme's root/ directory, e.g. webpushr-sw.js) land at
	// the archive root so service workers and other fixed-scope files keep the
	// URLs they are registered under.
	if rootFS := tpl.RootFilesFS(g.theme); rootFS != nil {
		if err := copyFS(rootFS, publicDir); err != nil {
			return fmt.Errorf("copy theme root files: %w", err)
		}
	}

	return nil
}

// writeAliasRedirects emits a self-contained meta-refresh page for every
// content alias whose target is published, mirroring the dynamic server's 301s
// on hosts that cannot run it. Aliases that would shadow an emitted page, a
// reserved root file, or another alias are skipped.
func (g *Generator) writeAliasRedirects(ctx context.Context, publicDir string, takenLocations map[string]struct{}) error {
	if g.aliasSvc == nil {
		return nil
	}

	aliases, err := g.aliasSvc.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("list aliases: %w", err)
	}

	slices.SortFunc(aliases, func(a, b *aliasdomain.Alias) int {
		return strings.Compare(a.Alias, b.Alias)
	})

	for _, a := range aliases {
		aliasPath := normalizeAliasPath(a.Alias)
		location := aliasExportPath(aliasPath)
		if location == "" {
			continue
		}

		if _, conflict := takenLocations[location]; conflict {
			continue
		}

		content, err := g.contentSvc.GetPublishedByID(ctx, a.ContentID)
		if err != nil || content.Slug == "" {
			continue
		}

		slug := sanitizePathSegment(content.Slug)
		if slug == "_" || slug == aliasPath {
			continue
		}

		pageHTML := aliasRedirectPage(seo.BuildURL(g.siteURL, "/"+slug+"/"))

		if err := os.MkdirAll(filepath.Dir(filepath.Join(publicDir, location)), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(publicDir, filepath.FromSlash(location)), []byte(pageHTML), 0644); err != nil {
			return fmt.Errorf("write alias redirect %s: %w", a.Alias, err)
		}

		takenLocations[location] = struct{}{}
	}

	return nil
}

func aliasRedirectPage(target string) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	b.WriteString("\t<meta charset=\"UTF-8\">\n")
	b.WriteString("\t<meta name=\"robots\" content=\"noindex, follow\">\n")
	fmt.Fprintf(&b, "\t<title>Redirecting&hellip;</title>\n")
	fmt.Fprintf(&b, "\t<link rel=\"canonical\" href=\"%s\">\n", html.EscapeString(target))
	fmt.Fprintf(&b, "\t<meta http-equiv=\"refresh\" content=\"0; url=%s\">\n", html.EscapeString(target))
	b.WriteString("</head>\n<body>\n")
	fmt.Fprintf(&b, "\t<p>Page moved. Continue to <a href=\"%s\">%s</a>.</p>\n", html.EscapeString(target), html.EscapeString(target))
	b.WriteString("</body>\n</html>\n")

	return b.String()
}

func (g *Generator) writeSitemap(ctx context.Context, publicDir string, tasks []renderTask) error {
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")

	for _, task := range tasks {
		urlPath := g.pageURL(task.path)
		fmt.Fprintf(&sb, "  <url><loc>%s</loc></url>\n", urlPath)
	}

	sb.WriteString("</urlset>\n")

	return os.WriteFile(filepath.Join(publicDir, "sitemap.xml"), []byte(sb.String()), 0644)
}

func (g *Generator) writeRobotsTxt(publicDir string) error {
	content := fmt.Sprintf("User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", strings.TrimSuffix(g.siteURL, "/"))
	return os.WriteFile(filepath.Join(publicDir, "robots.txt"), []byte(content), 0644)
}

func (g *Generator) writeNotFoundPage(ctx context.Context, publicDir string, referencedAssets map[string]bool) error {
	pageHTML, err := g.templates.RenderNotFoundToString(g.assembler.BuildNotFoundData(ctx, ""))
	if err != nil {
		return fmt.Errorf("render 404 page: %w", err)
	}
	collectReferencedAssets(pageHTML, referencedAssets)

	return os.WriteFile(filepath.Join(publicDir, "404.html"), []byte(pageHTML), 0644)
}

func (g *Generator) writeFeed(ctx context.Context, publicDir string) error {
	posts, err := g.contentSvc.GetPublishedByPostType(ctx, "post", g.languages, 0, 0, feedItemLimit, 0)
	if err != nil {
		return fmt.Errorf("list posts for feed: %w", err)
	}

	siteConfig := g.assembler.SiteConfig()
	primaryLang := g.assembler.PrimaryLanguage()

	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<rss version=\"2.0\">\n")
	sb.WriteString("  <channel>\n")
	fmt.Fprintf(&sb, "    <title>%s</title>\n", xmlEscape(siteConfig.Name))
	fmt.Fprintf(&sb, "    <link>%s</link>\n", xmlEscape(g.siteURL))
	fmt.Fprintf(&sb, "    <description>%s</description>\n", xmlEscape(siteConfig.Name))
	fmt.Fprintf(&sb, "    <language>%s</language>\n", xmlEscape(primaryLang))
	fmt.Fprintf(&sb, "    <lastBuildDate>%s</lastBuildDate>\n", time.Now().UTC().Format(time.RFC1123Z))

	for _, post := range posts {
		slug := sanitizePathSegment(post.Slug)
		if slug == "_" {
			continue
		}
		link := seo.BuildURL(g.siteURL, "/"+slug+"/")

		sb.WriteString("    <item>\n")
		fmt.Fprintf(&sb, "      <title>%s</title>\n", xmlEscape(post.Title))
		fmt.Fprintf(&sb, "      <link>%s</link>\n", xmlEscape(link))
		fmt.Fprintf(&sb, "      <guid>%s</guid>\n", xmlEscape(link))
		fmt.Fprintf(&sb, "      <pubDate>%s</pubDate>\n", post.CreatedAt.UTC().Format(time.RFC1123Z))
		fmt.Fprintf(&sb, "      <description>%s</description>\n", xmlEscape(feedExcerpt(post)))
		sb.WriteString("    </item>\n")
	}

	sb.WriteString("  </channel>\n")
	sb.WriteString("</rss>\n")

	return os.WriteFile(filepath.Join(publicDir, "index.xml"), []byte(sb.String()), 0644)
}

func (g *Generator) Generate(ctx context.Context, w io.Writer) error {
	tmpDir, err := os.MkdirTemp("", "lesstruct-ssg-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	publicDir := filepath.Join(tmpDir, "public")
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		return fmt.Errorf("create public dir: %w", err)
	}

	tasks, err := g.enumerate(ctx)
	if err != nil {
		return fmt.Errorf("enumerate pages: %w", err)
	}

	referencedAssets := make(map[string]bool)

	// Locations an alias redirect must never shadow: every emitted page plus
	// the reserved root files written below.
	takenLocations := map[string]struct{}{
		"sitemap.xml": {},
		"robots.txt":  {},
		"404.html":    {},
		"index.xml":   {},
	}

	for _, task := range tasks {
		pageHTML, err := task.render(ctx)
		if err != nil {
			continue
		}
		collectReferencedAssets(pageHTML, referencedAssets)

		dirPath := filepath.Join(publicDir, task.path)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(dirPath, "index.html"), []byte(pageHTML), 0644); err != nil {
			continue
		}
		takenLocations[filepath.ToSlash(filepath.Join(task.path, "index.html"))] = struct{}{}

		if task.amp {
			ampHTML, err := TransformToAMP(pageHTML, g.theme)
			if err != nil {
				continue
			}
			collectReferencedAssets(ampHTML, referencedAssets)

			ampDir := filepath.Join(publicDir, task.path, "amp")
			if err := os.MkdirAll(ampDir, 0755); err != nil {
				continue
			}
			if err := os.WriteFile(filepath.Join(ampDir, "index.html"), []byte(ampHTML), 0644); err != nil {
				continue
			}
			takenLocations[filepath.ToSlash(filepath.Join(task.path, "amp", "index.html"))] = struct{}{}
		}
	}

	if err := g.writeSitemap(ctx, publicDir, tasks); err != nil {
		return fmt.Errorf("write sitemap: %w", err)
	}

	if err := g.writeRobotsTxt(publicDir); err != nil {
		return fmt.Errorf("write robots.txt: %w", err)
	}

	if err := g.writeFeed(ctx, publicDir); err != nil {
		return fmt.Errorf("write feed: %w", err)
	}

	if err := g.writeNotFoundPage(ctx, publicDir, referencedAssets); err != nil {
		return fmt.Errorf("write 404 page: %w", err)
	}

	// Static assets are written after every HTML surface has been rendered so
	// the referenced-asset set is complete.
	if err := g.writeStaticFiles(ctx, publicDir, referencedAssets); err != nil {
		return fmt.Errorf("write static files: %w", err)
	}

	if err := g.writeAliasRedirects(ctx, publicDir, takenLocations); err != nil {
		return fmt.Errorf("write alias redirects: %w", err)
	}

	if err := g.writeAMPCanonicalLinks(publicDir, tasks); err != nil {
		return fmt.Errorf("write AMP canonical links: %w", err)
	}

	if err := targzDir(publicDir, w); err != nil {
		return fmt.Errorf("create tar.gz: %w", err)
	}

	return nil
}

// WithAliases attaches the alias service so legacy URL aliases are exported as
// meta-refresh redirect pages (mirroring the dynamic server's 301s).
func (g *Generator) WithAliases(aliasSvc *aliasdomain.Service) *Generator {
	g.aliasSvc = aliasSvc
	return g
}

func NewGenerator(
	assembler *contentpage.DataAssembler,
	templates *tpl.Templates,
	contentSvc contentpage.ContentService,
	mediaDir string,
	theme *tpl.Theme,
	siteURL string,
) *Generator {
	return &Generator{
		assembler:  assembler,
		templates:  templates,
		contentSvc: contentSvc,
		siteURL:    strings.TrimSuffix(siteURL, "/"),
		languages:  assembler.Languages(),
		perPage:    assembler.PostsPerPage(),
		mediaDir:   mediaDir,
		theme:      theme,
	}
}
