package ssg

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/api/contentpage"
	tpl "github.com/aristorinjuang/lesstruct/internal/api/template"
	"github.com/aristorinjuang/lesstruct/internal/config"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
)

const ssgPageSize = 100

func sanitizePathSegment(s string) string {
	if s == "" || strings.Contains(s, "/") || strings.Contains(s, "..") || s == "." {
		return "_"
	}
	return s
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
	assembler   *contentpage.DataAssembler
	templates   *tpl.Templates
	contentSvc  contentpage.ContentService
	siteURL     string
	primaryLang string
	perPage     int
	mediaDir    string
	theme       *tpl.Theme
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
		canonical := strings.TrimSuffix(g.siteURL+task.path, "/")
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
		contents, err := g.contentSvc.GetPublishedByPostType(ctx, "post", g.primaryLang, 0, 0, perPage+1, (page-1)*perPage)
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
			contents, err := g.contentSvc.GetPublishedByPostType(ctx, pt, g.primaryLang, 0, 0, perPage+1, (ptPage-1)*perPage)
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

func (g *Generator) writeStaticFiles(ctx context.Context, publicDir string) error {
	staticDir := filepath.Join(publicDir, "static")
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		return err
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

	return nil
}

func (g *Generator) writeSitemap(ctx context.Context, publicDir string, tasks []renderTask) error {
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")

	for _, task := range tasks {
		urlPath := strings.TrimSuffix(g.siteURL+task.path, "/")
		fmt.Fprintf(&sb, "  <url><loc>%s</loc></url>\n", urlPath)
	}

	sb.WriteString("</urlset>\n")

	return os.WriteFile(filepath.Join(publicDir, "sitemap.xml"), []byte(sb.String()), 0644)
}

func (g *Generator) writeRobotsTxt(publicDir string) error {
	content := fmt.Sprintf("User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", strings.TrimSuffix(g.siteURL, "/"))
	return os.WriteFile(filepath.Join(publicDir, "robots.txt"), []byte(content), 0644)
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

	for _, task := range tasks {
		html, err := task.render(ctx)
		if err != nil {
			continue
		}

		dirPath := filepath.Join(publicDir, task.path)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(dirPath, "index.html"), []byte(html), 0644); err != nil {
			continue
		}

		if task.amp {
			ampHTML, err := TransformToAMP(html, g.theme)
			if err != nil {
				continue
			}
			ampDir := filepath.Join(publicDir, task.path, "amp")
			if err := os.MkdirAll(ampDir, 0755); err != nil {
				continue
			}
			if err := os.WriteFile(filepath.Join(ampDir, "index.html"), []byte(ampHTML), 0644); err != nil {
				continue
			}
		}
	}

	if err := g.writeStaticFiles(ctx, publicDir); err != nil {
		return fmt.Errorf("write static files: %w", err)
	}

	if err := g.writeSitemap(ctx, publicDir, tasks); err != nil {
		return fmt.Errorf("write sitemap: %w", err)
	}

	if err := g.writeRobotsTxt(publicDir); err != nil {
		return fmt.Errorf("write robots.txt: %w", err)
	}

	if err := g.writeAMPCanonicalLinks(publicDir, tasks); err != nil {
		return fmt.Errorf("write AMP canonical links: %w", err)
	}

	if err := targzDir(publicDir, w); err != nil {
		return fmt.Errorf("create tar.gz: %w", err)
	}

	return nil
}

func NewGenerator(
	assembler *contentpage.DataAssembler,
	templates *tpl.Templates,
	contentSvc contentpage.ContentService,
	mediaDir string,
	theme *tpl.Theme,
	siteURL string,
) *Generator {
	primaryLang := config.PrimaryLanguage(assembler.Languages())
	return &Generator{
		assembler:   assembler,
		templates:   templates,
		contentSvc:  contentSvc,
		siteURL:     strings.TrimSuffix(siteURL, "/"),
		primaryLang: primaryLang,
		perPage:     assembler.PostsPerPage(),
		mediaDir:    mediaDir,
		theme:       theme,
	}
}
