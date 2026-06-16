package content_test

import (
	"errors"
	"strings"
	"testing"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
)

func TestStatus_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		status  contentdomain.Status
		want    bool
	}{
		{
			name:   "valid draft status",
			status: contentdomain.StatusDraft,
			want:   true,
		},
		{
			name:   "valid published status",
			status: contentdomain.StatusPublished,
			want:   true,
		},
		{
			name:   "invalid status",
			status: contentdomain.Status("invalid"),
			want:   false,
		},
		{
			name:   "empty status",
			status: contentdomain.Status(""),
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("Status.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status contentdomain.Status
		want   string
	}{
		{
			name:   "draft status",
			status: contentdomain.StatusDraft,
			want:   "draft",
		},
		{
			name:   "published status",
			status: contentdomain.StatusPublished,
			want:   "published",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("Status.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateTitle(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantErr error
	}{
		{
			name:    "valid title",
			title:   "My Test Title",
			wantErr: nil,
		},
		{
			name:    "valid title with whitespace",
			title:   "  My Test Title  ",
			wantErr: nil,
		},
		{
			name:    "valid single character title",
			title:   "A",
			wantErr: nil,
		},
		{
			name:    "valid 200 character title",
			title:   strings.Repeat("A", 200),
			wantErr: nil,
		},
		{
			name:    "invalid empty title",
			title:   "",
			wantErr: contentdomain.ErrInvalidTitle,
		},
		{
			name:    "invalid whitespace only title",
			title:   "   ",
			wantErr: contentdomain.ErrInvalidTitle,
		},
		{
			name:    "invalid 201 character title",
			title:   strings.Repeat("A", 201),
			wantErr: contentdomain.ErrInvalidTitle,
		},
		{
			name:    "invalid title with script tag",
			title:   `<script>alert(1)</script>`,
			wantErr: contentdomain.ErrHTMLInTitle,
		},
		{
			name:    "invalid title with bold tag",
			title:   "<b>bold title</b>",
			wantErr: contentdomain.ErrHTMLInTitle,
		},
		{
			name:    "invalid title with img tag",
			title:   `<img src="x" onerror="alert(1)">`,
			wantErr: contentdomain.ErrHTMLInTitle,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := contentdomain.ValidateTitle(tt.title); err != tt.wantErr {
				t.Errorf("ValidateTitle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr error
	}{
		{
			name:    "invalid empty content",
			content: "",
			wantErr: contentdomain.ErrInvalidContent,
		},
		{
			name:    "valid TipTap JSON document",
			content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`,
			wantErr: nil,
		},
		{
			name:    "valid 100000 character TipTap JSON",
			content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"` + strings.Repeat("A", 99915) + `"}]}]}`,
			wantErr: nil,
		},
		{
			name:    "invalid plain text content",
			content: "This is not JSON",
			wantErr: contentdomain.ErrInvalidTipTapContent,
		},
		{
			name:    "invalid 100001 character content",
			content: strings.Repeat("A", 100001),
			wantErr: contentdomain.ErrInvalidContent,
		},
		{
			name:    "invalid TipTap JSON with script node",
			content: `{"type":"doc","content":[{"type":"script","content":[{"type":"text","text":"alert(1)"}]}]}`,
			wantErr: contentdomain.ErrInvalidTipTapContent,
		},
		{
			name:    "invalid TipTap JSON with unknown node type",
			content: `{"type":"doc","content":[{"type":"iframe","attrs":{"src":"https://evil.com"}}]}`,
			wantErr: contentdomain.ErrInvalidTipTapContent,
		},
		{
			name:    "valid empty TipTap document",
			content: `{"type":"doc"}`,
			wantErr: nil,
		},
		{
			name:    "invalid malformed JSON",
			content: `{not json}`,
			wantErr: contentdomain.ErrInvalidTipTapContent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := contentdomain.ValidateContent(tt.content); !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateContent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		wantErr error
	}{
		{
			name:    "valid slug",
			slug:    "my-test-slug",
			wantErr: nil,
		},
		{
			name:    "valid slug with numbers",
			slug:    "my-test-slug-123",
			wantErr: nil,
		},
		{
			name:    "valid slug with leading/trailing whitespace",
			slug:    "  my-test-slug  ",
			wantErr: nil,
		},
		{
			name:    "valid single character slug",
			slug:    "a",
			wantErr: nil,
		},
		{
			name:    "valid 200 character slug",
			slug:    strings.Repeat("a-", 100),
			wantErr: nil,
		},
		{
			name:    "invalid empty slug",
			slug:    "",
			wantErr: contentdomain.ErrInvalidSlug,
		},
		{
			name:    "invalid whitespace only slug",
			slug:    "   ",
			wantErr: contentdomain.ErrInvalidSlug,
		},
		{
			name:    "invalid 201 character slug",
			slug:    strings.Repeat("a", 201),
			wantErr: contentdomain.ErrInvalidSlug,
		},
		{
			name:    "invalid slug with uppercase",
			slug:    "My-Test-Slug",
			wantErr: contentdomain.ErrInvalidSlug,
		},
		{
			name:    "invalid slug with spaces",
			slug:    "my test slug",
			wantErr: contentdomain.ErrInvalidSlug,
		},
		{
			name:    "invalid slug with special characters",
			slug:    "my-test-slug!",
			wantErr: contentdomain.ErrInvalidSlug,
		},
		{
			name:    "invalid slug with underscore",
			slug:    "my_test_slug",
			wantErr: contentdomain.ErrInvalidSlug,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := contentdomain.ValidateSlug(tt.slug); err != tt.wantErr {
				t.Errorf("ValidateSlug() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTags(t *testing.T) {
	tests := []struct {
		name    string
		tags    []string
		wantErr error
	}{
		{
			name:    "valid empty tags",
			tags:    []string{},
			wantErr: nil,
		},
		{
			name:    "valid single tag",
			tags:    []string{"test"},
			wantErr: nil,
		},
		{
			name:    "valid multiple tags",
			tags:    []string{"test1", "test2", "test3"},
			wantErr: nil,
		},
		{
			name:    "valid tags with whitespace",
			tags:    []string{"  test1  ", "  test2  "},
			wantErr: nil,
		},
		{
			name:    "valid 50 character tag",
			tags:    []string{strings.Repeat("A", 50)},
			wantErr: nil,
		},
		{
			name:    "invalid empty tag",
			tags:    []string{""},
			wantErr: errors.New("each tag must be between 1 and 50 characters"),
		},
		{
			name:    "invalid whitespace only tag",
			tags:    []string{"   "},
			wantErr: errors.New("each tag must be between 1 and 50 characters"),
		},
		{
			name:    "invalid 51 character tag",
			tags:    []string{strings.Repeat("A", 51)},
			wantErr: errors.New("each tag must be between 1 and 50 characters"),
		},
		{
			name:    "invalid mix of valid and invalid tags",
			tags:    []string{"valid", ""},
			wantErr: errors.New("each tag must be between 1 and 50 characters"),
		},
		{
			name:    "invalid tag with HTML",
			tags:    []string{`<script>alert(1)</script>`},
			wantErr: contentdomain.ErrHTMLInPlainText,
		},
		{
			name:    "invalid tag with bold tag",
			tags:    []string{"<b>tag</b>"},
			wantErr: contentdomain.ErrHTMLInPlainText,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := contentdomain.ValidateTags(tt.tags); err != nil {
				if tt.wantErr == nil {
					t.Errorf("ValidateTags() unexpected error = %v", err)
				} else if err.Error() != tt.wantErr.Error() {
					t.Errorf("ValidateTags() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else if tt.wantErr != nil {
				t.Errorf("ValidateTags() expected error = %v, got nil", tt.wantErr)
			}
		})
	}
}

func TestValidateCommentText(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		wantErr error
	}{
		{
			name:    "valid comment",
			comment: "This is a valid comment",
			wantErr: nil,
		},
		{
			name:    "valid comment with whitespace",
			comment: "  This is a valid comment  ",
			wantErr: nil,
		},
		{
			name:    "invalid empty comment",
			comment: "",
			wantErr: contentdomain.ErrInvalidCommentText,
		},
		{
			name:    "invalid whitespace only comment",
			comment: "   ",
			wantErr: contentdomain.ErrInvalidCommentText,
		},
		{
			name:    "valid 2000 character comment",
			comment: strings.Repeat("A", 2000),
			wantErr: nil,
		},
		{
			name:    "invalid 2001 character comment",
			comment: strings.Repeat("A", 2001),
			wantErr: contentdomain.ErrInvalidCommentText,
		},
		{
			name:    "invalid comment with script tag",
			comment: `<script>alert(1)</script>`,
			wantErr: contentdomain.ErrHTMLInPlainText,
		},
		{
			name:    "invalid comment with img tag",
			comment: `<img src="x" onerror="alert(1)">`,
			wantErr: contentdomain.ErrHTMLInPlainText,
		},
		{
			name:    "invalid comment with bold tag",
			comment: "<b>bold</b>",
			wantErr: contentdomain.ErrHTMLInPlainText,
		},
		{
			name:    "invalid comment with anchor tag",
			comment: `<a href="https://example.com">link</a>`,
			wantErr: contentdomain.ErrHTMLInPlainText,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := contentdomain.ValidateCommentText(tt.comment); err != tt.wantErr {
				t.Errorf("ValidateCommentText() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateMetaDescription(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  error
	}{
		{name: "empty value allowed", value: "", want: nil},
		{name: "valid plain text", value: "A valid meta description", want: nil},
		{name: "rejects script tag", value: `<script>alert(1)</script>`, want: contentdomain.ErrHTMLInPlainText},
		{name: "rejects bold tag", value: "<b>bold</b>", want: contentdomain.ErrHTMLInPlainText},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contentdomain.ValidateMetaDescription(tt.value); got != tt.want {
				t.Errorf("ValidateMetaDescription() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateOGTitle(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  error
	}{
		{name: "empty value allowed", value: "", want: nil},
		{name: "valid plain text", value: "A valid OG title", want: nil},
		{name: "rejects script tag", value: `<script>alert(1)</script>`, want: contentdomain.ErrHTMLInPlainText},
		{name: "rejects img tag", value: `<img src="x">`, want: contentdomain.ErrHTMLInPlainText},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contentdomain.ValidateOGTitle(tt.value); got != tt.want {
				t.Errorf("ValidateOGTitle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateOGDescription(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  error
	}{
		{name: "empty value allowed", value: "", want: nil},
		{name: "valid plain text", value: "A valid OG description", want: nil},
		{name: "rejects anchor tag", value: `<a href="https://example.com">link</a>`, want: contentdomain.ErrHTMLInPlainText},
		{name: "rejects iframe tag", value: `<iframe src="evil.com"></iframe>`, want: contentdomain.ErrHTMLInPlainText},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contentdomain.ValidateOGDescription(tt.value); got != tt.want {
				t.Errorf("ValidateOGDescription() = %v, want %v", got, tt.want)
			}
		})
	}
}
