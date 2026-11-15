package blog

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
)

const (
	// themeValidationTempPattern names the scratch dirs template validation
	// writes a theme into for the real site-ssr parse pipeline.
	themeValidationTempPattern = "lesstruct-theme-validate-*"

	// sampleRenderTime is a fixed timestamp so sample renders are
	// deterministic.
	sampleRenderTime = "2026-01-02T03:04:05Z"
)

// renderStep is one smoke render of a page template with sample data.
type renderStep struct {
	page   string
	render func(*sitessr.Templates) (string, error)
}

// hasTemplateFiles reports whether the theme overrides any page template.
// Stylesheet-only themes skip the render gate entirely.
func hasTemplateFiles(files map[string]string) bool {
	for file := range files {
		if strings.HasPrefix(file, "templates/") && strings.HasSuffix(file, ".html") {
			return true
		}
	}
	return false
}

// writeThemeArtifact materializes a theme file map under a fresh temp dir so
// the real site-ssr pipeline can parse it. The caller removes the dir.
func writeThemeArtifact(files map[string]string) (string, error) {
	dir, err := os.MkdirTemp("", themeValidationTempPattern)
	if err != nil {
		return "", fmt.Errorf("create template scratch dir: %w", err)
	}
	for rel, content := range files {
		clean := path.Clean(rel)
		if clean == "." || path.IsAbs(clean) || strings.HasPrefix(clean, "../") {
			_ = os.RemoveAll(dir)
			return "", fmt.Errorf("unsafe theme path %q", rel)
		}
		target := filepath.Join(dir, filepath.FromSlash(clean))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			_ = os.RemoveAll(dir)
			return "", fmt.Errorf("create theme dir for %s: %w", rel, err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			_ = os.RemoveAll(dir)
			return "", fmt.Errorf("write theme file %s: %w", rel, err)
		}
	}
	return dir, nil
}

// readThemeTemplateSources renders the edition's embedded page templates into
// the prompt source block. Files are marked so the model can map them to the
// documented file contract.
func readThemeTemplateSources(files fs.FS) string {
	if files == nil {
		return ""
	}
	entries, err := fs.ReadDir(files, "pages")
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(path.Ext(entry.Name()))
		if ext != ".gohtml" && ext != ".html" {
			continue
		}
		data, err := fs.ReadFile(files, path.Join("pages", entry.Name()))
		if err != nil {
			continue
		}
		b.WriteString("--- template: ")
		b.WriteString(strings.TrimSuffix(entry.Name(), path.Ext(entry.Name())))
		b.WriteString(" ---\n")
		b.Write(data)
		b.WriteString("\n\n")
	}
	return b.String()
}

func samplePost(slug, title, postType string) sitessr.PostItem {
	return sitessr.PostItem{
		Slug:            slug,
		Title:           title,
		MetaDescription: "A sample description used during theme validation.",
		ImageURL:        "/static/sample.webp",
		ImageSrcset:     "/static/sample.webp 1x, /static/sample@2x.webp 2x",
		ImageSizes:      "100vw",
		ImageVariants:   map[string]string{"thumb": "/static/sample-thumb.webp"},
		ImageMobileURL:  "/static/sample-thumb.webp",
		OriginalURL:     "/static/sample-original.webp",
		Author:          "Ada Lovelace",
		Username:        "ada",
		AuthorAvatarURL: "/static/avatar.webp",
		CreatedAt:       sampleTime(),
		PostType:        postType,
		Tags:            []string{"go", "themes"},
		CustomFields:    map[string]any{"images": []string{"/static/sample.webp"}},
	}
}

func samplePosts() []sitessr.PostItem {
	return []sitessr.PostItem{
		samplePost("sample-post", "Sample Post", "post"),
		samplePost("another-post", "Another Post", "post"),
	}
}

func sampleTime() time.Time {
	parsed, err := time.Parse(time.RFC3339, sampleRenderTime)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func sampleLayoutData() sitessr.LayoutData {
	languages := []sitessr.LanguageLink{
		{Code: "en", Name: "English", URL: "/sample-post"},
		{Code: "id", Name: "Indonesia", URL: "/id/sample-post"},
	}
	return sitessr.LayoutData{
		Title:       "Sample Post",
		Description: "A sample description.",
		OGTitle:     "Sample Post",
		OGDesc:      "A sample description.",
		OGImage:     "/static/sample.webp",
		PageTitle:   "Sample Post",
		NavigationItems: []sitessr.NavigationItem{
			{Title: "Home", URL: "/", IsActive: true},
			{Title: "About", URL: "/about"},
		},
		FooterItems: []sitessr.NavigationItem{
			{Title: "Privacy", URL: "/privacy"},
			{Title: "Terms", URL: "/terms"},
		},
		Legal: sitessr.LegalData{
			CookiePolicyURL:     "/cookie-policy",
			PrivacyPolicyURL:    "/privacy",
			TermsOfServiceURL:   "/terms",
			ReturnsPolicyURL:    "/returns",
			CheckoutConsent:     true,
			RegistrationConsent: true,
			CookieNotice: sitessr.CookieNotice{
				Before:   "We use cookies.",
				LinkText: "Learn more",
				After:    "to improve your experience.",
				URL:      "/privacy",
			},
		},
		CurrentPath:   "/sample-post",
		Lang:          "en",
		LanguageLinks: languages,
		Languages:     languages,
		SiteConfig: sitessr.SiteConfig{
			Name:              "Lesstruct",
			Logo:              "/static/logo.svg",
			Favicon:           "/static/favicon.svg",
			ProductImageRatio: template.CSS("4 / 5"),
			HeroImage:         "/static/hero.webp",
			HeroTitle:         "Sample Hero",
			HeroSubtitle:      "A sample subtitle.",
		},
	}
}

func sampleIndexData() sitessr.IndexData {
	return sitessr.IndexData{
		LayoutData: sampleLayoutData(),
		Posts:      samplePosts(),
		Tags:       []string{"go", "themes"},
		Sections: []sitessr.HomeSection{
			{
				PostTypeSlug: "post",
				Title:        "Latest",
				Description:  "The latest posts.",
				URL:          "/posts",
				Posts:        samplePosts(),
			},
		},
		PaginationData: sitessr.PaginationData{
			CurrentPage: 2,
			HasPrev:     true,
			HasNext:     true,
			PrevURL:     "/page/1",
			NextURL:     "/page/3",
		},
	}
}

func sampleContentData() sitessr.ContentData {
	layout := sampleLayoutData()
	return sitessr.ContentData{
		LayoutData:      layout,
		Slug:            "sample-post",
		Body:            template.HTML("<p>Sample body content.</p>"),
		Tags:            []string{"go", "themes"},
		Author:          "Ada Lovelace",
		Username:        "ada",
		AuthorAvatarURL: "/static/avatar.webp",
		CreatedAt:       sampleTime(),
		AllowComments:   true,
		CustomFields: map[string]any{
			"images": []string{"/static/sample.webp"},
			"rating": "5",
		},
		CustomFieldsFormatted: []sitessr.FormattedField{
			{Label: "Rating", Value: "5"},
		},
		CustomFieldGalleries: map[string][]sitessr.GalleryImage{
			"images": {
				{
					URL:      "/static/sample.webp",
					Alt:      "Sample image",
					Thumb:    "/static/sample-thumb.webp",
					Medium:   "/static/sample.webp",
					Large:    "/static/sample.webp",
					Srcset:   "/static/sample.webp 1x",
					Original: "/static/sample.webp",
				},
			},
		},
		Related: samplePosts(),
		Comments: []sitessr.CommentItem{
			{Author: "Grace Hopper", Text: "A sample comment.", CreatedAt: sampleTime()},
		},
		PostType:    "post",
		PostScripts: template.HTML("<script src=\"/static/example.js\" defer></script>"),
	}
}

func sampleAuthorData() sitessr.AuthorData {
	return sitessr.AuthorData{
		LayoutData:            sampleLayoutData(),
		AuthorName:            "Ada Lovelace",
		Username:              "ada",
		AuthorAvatarURL:       "/static/avatar.webp",
		Posts:                 samplePosts(),
		CustomFieldsFormatted: []sitessr.FormattedField{{Label: "Bio", Value: "Sample bio."}},
		PaginationData:        sitessr.PaginationData{CurrentPage: 1, HasNext: true, NextURL: "/authors/ada/page/2"},
	}
}

func sampleTagData() sitessr.TagData {
	return sitessr.TagData{
		LayoutData:     sampleLayoutData(),
		TagName:        "Go",
		Posts:          samplePosts(),
		PaginationData: sitessr.PaginationData{CurrentPage: 1},
	}
}

// sampleRenderSteps pairs each fixed page with a smoke render using populated
// sample data, so a template that parses but cannot execute at runtime
// (missing field, wrong argument) fails validation.
func sampleRenderSteps() []renderStep {
	indexData := sampleIndexData()
	contentData := sampleContentData()
	authData := sitessr.AuthPageData{LayoutData: sampleLayoutData(), ShowRegister: true}
	notFoundData := sitessr.NotFoundData{LayoutData: sampleLayoutData()}
	verifyData := sitessr.VerifyEmailData{LayoutData: sampleLayoutData()}
	resetData := sitessr.ResetPasswordData{LayoutData: sampleLayoutData()}
	return []renderStep{
		{
			page:   "index.html",
			render: func(t *sitessr.Templates) (string, error) { return t.RenderIndexToString(indexData) },
		},
		{
			page:   "homepage.html",
			render: func(t *sitessr.Templates) (string, error) { return t.RenderHomeToString(indexData) },
		},
		{
			page:   "post.html",
			render: func(t *sitessr.Templates) (string, error) { return t.RenderContentToString(contentData) },
		},
		{
			page:   "author.html",
			render: func(t *sitessr.Templates) (string, error) { return t.RenderAuthorToString(sampleAuthorData()) },
		},
		{
			page:   "tag.html",
			render: func(t *sitessr.Templates) (string, error) { return t.RenderTagToString(sampleTagData()) },
		},
		{
			page:   "not_found.html",
			render: func(t *sitessr.Templates) (string, error) { return t.RenderNotFoundToString(notFoundData) },
		},
		{
			page:   "login.html",
			render: func(t *sitessr.Templates) (string, error) { return t.RenderLoginToString(authData) },
		},
		{
			page:   "register.html",
			render: func(t *sitessr.Templates) (string, error) { return t.RenderRegisterToString(authData) },
		},
		{
			page:   "forgot_password.html",
			render: func(t *sitessr.Templates) (string, error) { return t.RenderForgotPasswordToString(authData) },
		},
		{
			page:   "verify_email.html",
			render: func(t *sitessr.Templates) (string, error) { return t.RenderVerifyEmailToString(verifyData) },
		},
		{
			page:   "reset_password.html",
			render: func(t *sitessr.Templates) (string, error) { return t.RenderResetPasswordToString(resetData) },
		},
	}
}

func samplePostTypeRenderSteps(postTypeSlugs []string) []renderStep {
	steps := make([]renderStep, 0, len(postTypeSlugs))
	for _, slug := range postTypeSlugs {
		if slug == "" || slug == "post" {
			continue
		}
		data := sampleContentData()
		data.PostType = slug
		data.Slug = "sample-" + slug
		steps = append(steps, renderStep{
			page:   slug + ".html",
			render: func(t *sitessr.Templates) (string, error) { return t.RenderContentToString(data) },
		})
	}
	return steps
}

// renderFindings runs every smoke render and turns execution failures into
// error findings naming the page.
func renderFindings(templates *sitessr.Templates, postTypeSlugs []string) []themegendomain.Finding {
	steps := append(sampleRenderSteps(), samplePostTypeRenderSteps(postTypeSlugs)...)
	findings := make([]themegendomain.Finding, 0)
	for _, step := range steps {
		if _, err := step.render(templates); err != nil {
			findings = append(findings, themegendomain.NewFinding(
				"template-render",
				themegendomain.FindingSeverityError,
				step.page+": "+err.Error(),
			))
		}
	}
	return findings
}

// renderingValidator composes the domain validator with template parse and
// smoke-render validation against the real site-ssr stack. Stylesheet-only
// themes pass through unchanged; the domain's error findings short-circuit
// the template gate so the repair loop fixes the file contract first.
type renderingValidator struct {
	base          themegendomain.ThemeValidator
	postTypeSlugs []string
}

func (v renderingValidator) Validate(ctx context.Context, theme themegendomain.Theme) themegendomain.ValidationReport {
	report := v.base.Validate(ctx, theme)
	findings := append([]themegendomain.Finding(nil), report.Findings...)
	if !hasTemplateFiles(theme.Files) || !report.Valid {
		return themegendomain.NewFindingsReport(findings)
	}
	dir, err := writeThemeArtifact(theme.Files)
	if err != nil {
		findings = append(findings, themegendomain.NewFinding(
			"template-io",
			themegendomain.FindingSeverityError,
			err.Error(),
		))
		return themegendomain.NewFindingsReport(findings)
	}
	defer func() {
		_ = os.RemoveAll(dir)
	}()
	templates, err := sitessr.NewTemplates(&sitessr.Theme{Dir: dir}, nil, v.postTypeSlugs...)
	if err != nil {
		findings = append(findings, themegendomain.NewFinding(
			"template-parse",
			themegendomain.FindingSeverityError,
			err.Error(),
		))
		return themegendomain.NewFindingsReport(findings)
	}
	findings = append(findings, renderFindings(templates, v.postTypeSlugs)...)
	return themegendomain.NewFindingsReport(findings)
}

// newThemeValidator builds the composed validator: the domain file/CSS checks
// plus template parse and smoke render for the configured post-type slugs.
func newThemeValidator(postTypeSlugs []string) themegendomain.ThemeValidator {
	return renderingValidator{
		base:          themegendomain.NewValidator(),
		postTypeSlugs: append([]string(nil), postTypeSlugs...),
	}
}
