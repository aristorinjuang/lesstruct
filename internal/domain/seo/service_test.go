package seo_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/seo"
)

func TestNewService(t *testing.T) {
	baseURL := "https://example.com"
	siteName := "Test Site"

	service := seo.NewService(baseURL, siteName)

	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestService_Generate(t *testing.T) {
	service := seo.NewService("https://example.com", "Test Site")

	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"This is a test article with some content for SEO purposes."}]}]}`

	input := seo.GenerateInput{
		Title:         "Test Article Title",
		Content:       tiptapJSON,
		URL:           "/posts/test-article",
		DatePublished: "2026-04-10T00:00:00Z",
		DateModified:  "2026-04-10T12:00:00Z",
		AuthorName:    "John Doe",
		Tags:          []string{"go", "cms"},
	}

	metadata, err := service.Generate(input)
	if err != nil {
		t.Fatalf("Service.Generate() error = %v", err)
	}

	if metadata.MetaDescription == "" {
		t.Error("Service.Generate() MetaDescription is empty")
	}
	if metadata.OGTitle != "Test Article Title" {
		t.Errorf("Service.Generate() OGTitle = %q, want %q", metadata.OGTitle, "Test Article Title")
	}
	if metadata.OGDescription == "" {
		t.Error("Service.Generate() OGDescription is empty")
	}
	if metadata.OGType != "article" {
		t.Errorf("Service.Generate() OGType = %q, want %q", metadata.OGType, "article")
	}
	if metadata.OGSiteName != "Test Site" {
		t.Errorf("Service.Generate() OGSiteName = %q, want %q", metadata.OGSiteName, "Test Site")
	}
	if metadata.TwitterCard != "summary_large_image" {
		t.Errorf("Service.Generate() TwitterCard = %q, want %q", metadata.TwitterCard, "summary_large_image")
	}
	if metadata.TwitterTitle != "Test Article Title" {
		t.Errorf("Service.Generate() TwitterTitle = %q, want %q", metadata.TwitterTitle, "Test Article Title")
	}
	if metadata.JSONLD == nil {
		t.Error("Service.Generate() JSONLD is nil")
	}
}

func TestService_Generate_WithOverrides(t *testing.T) {
	service := seo.NewService("https://example.com", "Test Site")

	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"This is a test article with some content for SEO purposes."}]}]}`

	input := seo.GenerateInput{
		Title:           "Test Article Title",
		Content:         tiptapJSON,
		MetaDescription: "Custom meta description",
		OGTitle:         "Custom OG Title",
		OGDescription:   "Custom OG description",
		URL:             "/posts/test-article",
		DatePublished:   "2026-04-10T00:00:00Z",
		DateModified:    "2026-04-10T12:00:00Z",
		AuthorName:      "John Doe",
		Tags:            []string{"go", "cms"},
	}

	metadata, err := service.Generate(input)
	if err != nil {
		t.Fatalf("Service.Generate() error = %v", err)
	}

	if metadata.MetaDescription != "Custom meta description" {
		t.Errorf("Service.Generate() MetaDescription = %q, want %q", metadata.MetaDescription, "Custom meta description")
	}
	if metadata.OGTitle != "Custom OG Title" {
		t.Errorf("Service.Generate() OGTitle = %q, want %q", metadata.OGTitle, "Custom OG Title")
	}
	if metadata.OGDescription != "Custom OG description" {
		t.Errorf("Service.Generate() OGDescription = %q, want %q", metadata.OGDescription, "Custom OG description")
	}
}

func TestService_Generate_WithImage(t *testing.T) {
	service := seo.NewService("https://example.com", "Test Site")

	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content text"}]},{"type":"image","attrs":{"src":"/uploads/media/test.webp"}}]}`

	input := seo.GenerateInput{
		Title:         "Test Article Title",
		Content:       tiptapJSON,
		URL:           "/posts/test-article",
		DatePublished: "2026-04-10T00:00:00Z",
		DateModified:  "2026-04-10T12:00:00Z",
		AuthorName:    "John Doe",
		Tags:          []string{"go", "cms"},
	}

	metadata, err := service.Generate(input)
	if err != nil {
		t.Fatalf("Service.Generate() error = %v", err)
	}

	if metadata.OGImage != "https://example.com/uploads/media/test.webp" {
		t.Errorf("Service.Generate() OGImage = %q, want %q", metadata.OGImage, "https://example.com/uploads/media/test.webp")
	}
	if metadata.TwitterImage != "https://example.com/uploads/media/test.webp" {
		t.Errorf("Service.Generate() TwitterImage = %q, want %q", metadata.TwitterImage, "https://example.com/uploads/media/test.webp")
	}
}

func TestService_Generate_WithFeaturedImage(t *testing.T) {
	service := seo.NewService("https://example.com", "Test Site")

	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content text"}]}]}`

	input := seo.GenerateInput{
		Title:         "Test Article Title",
		Content:       tiptapJSON,
		FeaturedImage: "/uploads/media/featured.webp",
		URL:           "/posts/test-article",
		DatePublished: "2026-04-10T00:00:00Z",
		DateModified:  "2026-04-10T12:00:00Z",
		AuthorName:    "John Doe",
		Tags:          []string{"go", "cms"},
	}

	metadata, err := service.Generate(input)
	if err != nil {
		t.Fatalf("Service.Generate() error = %v", err)
	}

	if metadata.OGImage != "https://example.com/uploads/media/featured.webp" {
		t.Errorf("Service.Generate() OGImage = %q, want %q", metadata.OGImage, "https://example.com/uploads/media/featured.webp")
	}
}

func TestService_Generate_ValidationErrors(t *testing.T) {
	service := seo.NewService("https://example.com", "Test Site")

	tests := []struct {
		name    string
		input   seo.GenerateInput
		wantErr error
	}{
		{
			name: "invalid meta description override - too long",
			input: seo.GenerateInput{
				Title:           "Test Title",
				Content:         `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
				MetaDescription: "This is a very long meta description that exceeds one hundred and sixty characters limit and should fail validation because it is simply too long for search engines to accept properly",
			},
			wantErr: seo.ErrInvalidMetaDescription,
		},
		{
			name: "invalid og title override - too long",
			input: seo.GenerateInput{
				Title:    "This is a very long title that exceeds the sixty character limit for OG titles",
				Content:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
				OGTitle:  "This is a very long title that exceeds the sixty character limit for OG titles",
			},
			wantErr: seo.ErrInvalidOGTitle,
		},
			{
				name: "invalid og description override - too long",
				input: seo.GenerateInput{
					Title:         "Test Title",
					Content:       `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`,
					OGDescription: "This is a very long og description that exceeds one hundred and sixty characters limit and should fail validation because it is simply too long for search engines to accept properly",
				},
				wantErr: seo.ErrInvalidOGDescription,
			},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Generate(tt.input)
			if err == nil {
				t.Error("Service.Generate() expected error but got nil")
			}
		})
	}
}

func TestService_Generate_WithTags(t *testing.T) {
	service := seo.NewService("https://example.com", "Test Site")

	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content text"}]}]}`

	input := seo.GenerateInput{
		Title:         "Test Article Title",
		Content:       tiptapJSON,
		URL:           "/posts/test-article",
		DatePublished: "2026-04-10T00:00:00Z",
		DateModified:  "2026-04-10T12:00:00Z",
		AuthorName:    "John Doe",
		Tags:          []string{"go", "cms", "web"},
	}

	metadata, err := service.Generate(input)
	if err != nil {
		t.Fatalf("Service.Generate() error = %v", err)
	}

	keywords, ok := metadata.JSONLD["keywords"].(string)
	if !ok {
		t.Error("Service.Generate() JSONLD keywords is not a string")
	}
	if keywords != "go, cms, web" {
		t.Errorf("Service.Generate() JSONLD keywords = %q, want %q", keywords, "go, cms, web")
	}
}

func TestService_Generate_EmptyContent(t *testing.T) {
	service := seo.NewService("https://example.com", "Test Site")

	input := seo.GenerateInput{
		Title:           "Test Title",
		Content:         "",
		MetaDescription: "Custom description",
		OGDescription:   "Custom OG description",
		URL:             "/posts/test-article",
		DatePublished:   "2026-04-10T00:00:00Z",
		DateModified:    "2026-04-10T12:00:00Z",
		AuthorName:      "John Doe",
	}

	metadata, err := service.Generate(input)
	if err != nil {
		t.Fatalf("Service.Generate() error = %v", err)
	}

	if metadata.MetaDescription != "Custom description" {
		t.Errorf("Service.Generate() MetaDescription = %q, want %q", metadata.MetaDescription, "Custom description")
	}
	if metadata.OGDescription != "Custom OG description" {
		t.Errorf("Service.Generate() OGDescription = %q, want %q", metadata.OGDescription, "Custom OG description")
	}
}

func TestService_Generate_LongContent(t *testing.T) {
	service := seo.NewService("https://example.com", "Test Site")

	longText := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. More content here to make it longer than 160 characters so we can test truncation."}]}]}`

	input := seo.GenerateInput{
		Title:         "Test Title",
		Content:       longText,
		URL:           "/posts/test-article",
		DatePublished: "2026-04-10T00:00:00Z",
		DateModified:  "2026-04-10T12:00:00Z",
		AuthorName:    "John Doe",
	}

	metadata, err := service.Generate(input)
	if err != nil {
		t.Fatalf("Service.Generate() error = %v", err)
	}

	if len(metadata.MetaDescription) > 160 {
		t.Errorf("Service.Generate() MetaDescription length = %d, want <= 160", len(metadata.MetaDescription))
	}
}

func TestService_Generate_JSONLDStructure(t *testing.T) {
	service := seo.NewService("https://example.com", "Test Site")

	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content text"}]}]}`

	input := seo.GenerateInput{
		Title:         "Test Article Title",
		Content:       tiptapJSON,
		URL:           "/posts/test-article",
		DatePublished: "2026-04-10T00:00:00Z",
		DateModified:  "2026-04-10T12:00:00Z",
		AuthorName:    "John Doe",
		Tags:          []string{"go", "cms"},
	}

	metadata, err := service.Generate(input)
	if err != nil {
		t.Fatalf("Service.Generate() error = %v", err)
	}

	jsonLD := metadata.JSONLD

	if jsonLD["@context"] != "https://schema.org" {
		t.Errorf("Service.Generate() JSONLD @context = %v, want %v", jsonLD["@context"], "https://schema.org")
	}
	if jsonLD["@type"] != "Article" {
		t.Errorf("Service.Generate() JSONLD @type = %v, want %v", jsonLD["@type"], "Article")
	}
	if jsonLD["headline"] != "Test Article Title" {
		t.Errorf("Service.Generate() JSONLD headline = %v, want %v", jsonLD["headline"], "Test Article Title")
	}
	if jsonLD["datePublished"] != "2026-04-10T00:00:00Z" {
		t.Errorf("Service.Generate() JSONLD datePublished = %v, want %v", jsonLD["datePublished"], "2026-04-10T00:00:00Z")
	}

	author, ok := jsonLD["author"].(map[string]any)
	if !ok {
		t.Fatal("Service.Generate() JSONLD author is not a map")
	}
	if author["@type"] != "Person" {
		t.Errorf("Service.Generate() JSONLD author @type = %v, want %v", author["@type"], "Person")
	}
	if author["name"] != "John Doe" {
		t.Errorf("Service.Generate() JSONLD author name = %v, want %v", author["name"], "John Doe")
	}
}

func TestService_Generate_EmptyAuthorName(t *testing.T) {
	service := seo.NewService("https://example.com", "Test Site")

	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content text"}]}]}`

	input := seo.GenerateInput{
		Title:         "Test Article",
		Content:       tiptapJSON,
		URL:           "/posts/test-article",
		DatePublished: "2026-04-10T00:00:00Z",
		DateModified:  "2026-04-10T12:00:00Z",
		AuthorName:    "",
		Tags:          []string{"test"},
	}

	metadata, err := service.Generate(input)
	if err != nil {
		t.Fatalf("Service.Generate() error = %v", err)
	}

	author, ok := metadata.JSONLD["author"].(map[string]any)
	if !ok {
		t.Fatal("Service.Generate() JSONLD author is not a map")
	}
	if author["name"] != "Author" {
		t.Errorf("Service.Generate() JSONLD author name = %v, want %v", author["name"], "Author")
	}
}

func TestService_Generate_FeaturedImageOverridesContentImage(t *testing.T) {
	service := seo.NewService("https://example.com", "Test Site")

	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]},{"type":"image","attrs":{"src":"/uploads/media/content-image.webp"}}]}`

	input := seo.GenerateInput{
		Title:         "Test Article",
		Content:       tiptapJSON,
		FeaturedImage: "/uploads/media/featured-image.webp",
		URL:           "/posts/test-article",
		DatePublished: "2026-04-10T00:00:00Z",
		DateModified:  "2026-04-10T12:00:00Z",
		AuthorName:    "Test Author",
	}

	metadata, err := service.Generate(input)
	if err != nil {
		t.Fatalf("Service.Generate() error = %v", err)
	}

	if metadata.OGImage != "https://example.com/uploads/media/featured-image.webp" {
		t.Errorf("Service.Generate() OGImage = %q, want %q", metadata.OGImage, "https://example.com/uploads/media/featured-image.webp")
	}
	if metadata.TwitterImage != "https://example.com/uploads/media/featured-image.webp" {
		t.Errorf("Service.Generate() TwitterImage = %q, want %q", metadata.TwitterImage, "https://example.com/uploads/media/featured-image.webp")
	}

	imageInJSONLD, ok := metadata.JSONLD["image"].(string)
	if !ok {
		t.Fatal("Service.Generate() JSONLD image is not a string")
	}
	if imageInJSONLD != "https://example.com/uploads/media/featured-image.webp" {
		t.Errorf("Service.Generate() JSONLD image = %q, want %q", imageInJSONLD, "https://example.com/uploads/media/featured-image.webp")
	}
}

func TestService_Generate_NoImage(t *testing.T) {
	service := seo.NewService("https://example.com", "Test Site")

	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content without any images"}]}]}`

	input := seo.GenerateInput{
		Title:         "Test Article",
		Content:       tiptapJSON,
		URL:           "/posts/test-article",
		DatePublished: "2026-04-10T00:00:00Z",
		DateModified:  "2026-04-10T12:00:00Z",
		AuthorName:    "Test Author",
	}

	metadata, err := service.Generate(input)
	if err != nil {
		t.Fatalf("Service.Generate() error = %v", err)
	}

	if metadata.OGImage != "" {
		t.Errorf("Service.Generate() OGImage = %q, want empty string", metadata.OGImage)
	}
	if metadata.TwitterImage != "" {
		t.Errorf("Service.Generate() TwitterImage = %q, want empty string", metadata.TwitterImage)
	}
	if _, hasImage := metadata.JSONLD["image"]; hasImage {
		t.Error("Service.Generate() JSONLD should not have image field when no image is present")
	}
}

func TestService_Generate_EmptyURL(t *testing.T) {
	service := seo.NewService("https://example.com", "Test Site")

	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`

	input := seo.GenerateInput{
		Title:         "Test Article",
		Content:       tiptapJSON,
		URL:           "",
		DatePublished: "2026-04-10T00:00:00Z",
		DateModified:  "2026-04-10T12:00:00Z",
		AuthorName:    "Test Author",
	}

	metadata, err := service.Generate(input)
	if err != nil {
		t.Fatalf("Service.Generate() error = %v", err)
	}

	if metadata.OGURL != "" {
		t.Errorf("Service.Generate() OGURL = %q, want empty string", metadata.OGURL)
	}
}

func TestService_Generate_EmptyTags(t *testing.T) {
	service := seo.NewService("https://example.com", "Test Site")

	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Content"}]}]}`

	input := seo.GenerateInput{
		Title:         "Test Article",
		Content:       tiptapJSON,
		URL:           "/posts/test-article",
		DatePublished: "2026-04-10T00:00:00Z",
		DateModified:  "2026-04-10T12:00:00Z",
		AuthorName:    "Test Author",
		Tags:          []string{},
	}

	metadata, err := service.Generate(input)
	if err != nil {
		t.Fatalf("Service.Generate() error = %v", err)
	}

	if _, hasKeywords := metadata.JSONLD["keywords"]; hasKeywords {
		t.Error("Service.Generate() JSONLD should not have keywords field when tags are empty")
	}
}
