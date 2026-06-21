package agent

import (
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
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
//   - Slug  is accepted for API stability but NOT honored in Story 2.1: the
//     content service auto-generates the slug from the title. See Completion Notes.
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
	CreatedAt    string         `json:"createdAt,omitempty"`
	UpdatedAt    string         `json:"updatedAt,omitempty"`
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

// MediaMetadata is the optional JSON `metadata` part of a media upload. altText is the
// only field today; it is REQUIRED by the media service (ValidateAltText) for
// accessibility, so agents should always send it.
type MediaMetadata struct {
	AltText string `json:"altText,omitempty"`
}

// MediaProjection is the public, whitelisted view of a media item returned by the agent
// media endpoints. It deliberately excludes internal/admin-only fields the raw
// mediadomain.Media carries (the numeric owner id, filePath, uploadedBy) so the
// programmatic API surface does not over-disclose — mirroring ContentProjection.
type MediaProjection struct {
	ID               int                            `json:"id"`
	Filename         string                         `json:"filename"`
	OriginalFilename string                         `json:"originalFilename"`
	MimeType         mediadomain.MimeType           `json:"mimeType"`
	FileSize         int64                          `json:"fileSize"`
	Width            int                            `json:"width"`
	Height           int                            `json:"height"`
	AltText          string                         `json:"altText"`
	IsWebP           bool                           `json:"isWebp"`
	Hash             string                         `json:"hash"`
	URL              string                         `json:"url"`
	Variants         map[string]mediadomain.MediaVariant `json:"variants,omitempty"`
	CreatedAt        string                         `json:"createdAt,omitempty"`
	UpdatedAt        string                         `json:"updatedAt,omitempty"`
}

// MediaResponse wraps a projected media item for the agent media responses.
// Envelope body: {"data":{"media":{...}}}.
type MediaResponse struct {
	Media MediaProjection `json:"media"`
}

// NewMediaResponse builds a MediaResponse projecting the given media entity to its public
// fields. A nil entity yields an empty projection (defensive — the service contract
// returns a non-nil media on success).
func NewMediaResponse(m *mediadomain.Media) MediaResponse {
	if m == nil {
		return MediaResponse{}
	}
	return MediaResponse{
		Media: MediaProjection{
			ID:               m.ID,
			Filename:         m.Filename,
			OriginalFilename: m.OriginalFilename,
			MimeType:         m.MimeType,
			FileSize:         m.FileSize,
			Width:            m.Width,
			Height:           m.Height,
			AltText:          m.AltText,
			IsWebP:           m.IsWebP,
			Hash:             m.Hash,
			URL:              m.URL,
			Variants:         m.Variants,
			CreatedAt:        m.CreatedAt,
			UpdatedAt:        m.UpdatedAt,
		},
	}
}
