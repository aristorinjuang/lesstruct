package agent

import (
	"time"

	contentdomain "github.com/aristorinjuang/lesstruct/kernel/content"
)

// ContentRequest is the agent authoring payload for POST /api/v1/content. It
// maps onto the existing contentdomain.CreateContentRequest (see the mapping
// table in the story Dev Notes). It is intentionally separate from the admin
// handler's CreateContentRequest so the agent contract can evolve independently.
//
// Field notes:
//   - Body   is the canonical content. For Format "tiptap" (default) it is the
//     Tiptap JSON string, stored unchanged. The service validates/sanitizes it.
//   - Format defaults to "tiptap" when omitted. "markdown" is reserved for Story
//     2.4 (Markdown→Tiptap conversion) and is rejected with VALIDATION_ERROR here.
//   - Slug  is admin-only and honored only at create time: an admin may supply a
//     custom slug (validated and uniqueness-checked per language by the service),
//     while a non-admin's slug is dropped and the service auto-generates one from
//     the title. The slug is immutable after creation — it can never be changed
//     via Update (changing it would break the public URL and harm SEO).
//   - IsPublished true maps to StatusPublished; false/omitted maps to StatusDraft.
//   - Tags are normalized via contentdomain.ValidateTags; an invalid tag returns
//     VALIDATION_ERROR from the server.
//   - Language must be in the server's configured languages (config.toml
//     [languages] list); an unknown code returns VALIDATION_ERROR
//     (ErrInvalidLanguage). The CLI cannot pre-validate this and forwards the
//     value as-given so the server is the single source of truth.
//   - TranslationGroupID (the CLI's --translation-of <id>) links the new item to
//     an existing one's translation group. The server validates the id exists
//     (ErrTranslationGroupNotFound → VALIDATION_ERROR) and stores it; a non-nil
//     value is what makes the public language switcher appear on the page.
type ContentRequest struct {
	Title              string         `json:"title"`
	Body               string         `json:"body"`
	Format             string         `json:"format,omitempty"`
	PostType           string         `json:"postType,omitempty"`
	Slug               string         `json:"slug,omitempty"`
	CustomFields       map[string]any `json:"customFields,omitempty"`
	IsPublished        bool           `json:"isPublished,omitempty"`
	Tags               []string       `json:"tags,omitempty"`
	Language           string         `json:"language,omitempty"`
	TranslationGroupID *int           `json:"translationGroupId,omitempty"`
}

// ContentProjection is the public, whitelisted view of a content item returned by the
// agent create/get endpoints. It deliberately excludes internal/admin-only fields the
// raw contentdomain.Content carries (the numeric owner id, updatedBy/updatedByUsername,
// translationGroupId, SEO metadata, allowComments) so the programmatic API surface
// does not over-disclose to integrators or — via IDOR — to other tenants.
type ContentProjection struct {
	ID           int            `json:"id"`
	Title        string         `json:"title"`
	Slug         string         `json:"slug"`
	Body         string         `json:"body"`
	Status       string         `json:"status"`
	PostType     string         `json:"postType,omitempty"`
	Language     string         `json:"language,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	CustomFields map[string]any `json:"customFields,omitempty"`
	Author       string         `json:"author,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

// ContentResponse wraps a projected content item for the agent create/get responses.
// Envelope body: {"data":{"content":{...}}}.
type ContentResponse struct {
	Content ContentProjection `json:"content"`
}

// NewContentResponse builds a ContentResponse projecting the given content entity to
// its public fields. A nil entity yields an empty projection (defensive — the service
// contract returns a non-nil content on success).
func NewContentResponse(c *contentdomain.Content) ContentResponse {
	if c == nil {
		return ContentResponse{}
	}
	return ContentResponse{
		Content: ContentProjection{
			ID:           c.ID,
			Title:        c.Title,
			Slug:         c.Slug,
			Body:         c.Content,
			Status:       string(c.Status),
			PostType:     c.PostType,
			Language:     c.Language,
			Tags:         c.Tags,
			CustomFields: c.CustomFields,
			Author:       c.Author,
			CreatedAt:    c.CreatedAt,
			UpdatedAt:    c.UpdatedAt,
		},
	}
}
