package content

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aristorinjuang/lesstruct/internal/domain/sanitize"
)

var (
	// MaxTitleRunes is the maximum allowed length of a content title in runes
	MaxTitleRunes = 200
	// MaxTagRunes is the maximum allowed length of a single tag in runes
	MaxTagRunes = 50
	// MaxMetaDescriptionRunes is the maximum allowed length of a content meta
	// description in runes (mirrors the SEO domain's 160-character limit).
	MaxMetaDescriptionRunes = 160
	// ErrContentNotFound is returned when content cannot be found
	ErrContentNotFound = errors.New("content not found")
	// ErrInvalidTitle is returned when title validation fails
	ErrInvalidTitle = errors.New("title is required and must be between 1 and 200 characters")
	// ErrEmptyContent is returned when content is empty
	ErrEmptyContent = errors.New("content is required")
	// ErrContentTooLong is returned when content exceeds maximum length
	ErrContentTooLong = errors.New("content must be less than 100000 characters")
	// ErrInvalidStatus is returned when status is not valid
	ErrInvalidStatus = errors.New("status must be either 'draft' or 'published'")
	// ErrInvalidSlug is returned when slug validation fails
	ErrInvalidSlug = errors.New("slug must be between 1 and 200 characters and contain only lowercase letters, numbers, and hyphens")
	// ErrSlugAlreadyExists is returned when slug already exists
	ErrSlugAlreadyExists = errors.New("slug already exists")
	// ErrUnauthorized is returned when user doesn't have permission to access content
	ErrUnauthorized = errors.New("unauthorized access to content")
	// ErrCommentNotFound is returned when comment cannot be found
	ErrCommentNotFound = errors.New("comment not found")
	// ErrInvalidCommentText is returned when comment text validation fails
	ErrInvalidCommentText = errors.New("comment text is required and must be between 1 and 2000 characters")
	// ErrInvalidCommentStatus is returned when comment status is not valid
	ErrInvalidCommentStatus = errors.New("comment status must be one of: pending, approved, rejected, spam")
	// ErrHTMLInTitle is returned when title contains HTML
	ErrHTMLInTitle = errors.New("title must not contain HTML tags")
	// ErrInvalidTipTapContent is returned when content is not valid TipTap JSON
	ErrInvalidTipTapContent = errors.New("content must be valid TipTap JSON structure")
	// ErrHTMLInPlainText is returned when a plain text field contains HTML
	ErrHTMLInPlainText = errors.New("field must not contain HTML tags")
	// ErrInvalidFilterField is returned when a custom field filter has an empty field name
	ErrInvalidFilterField = errors.New("filter field must not be empty")
	// ErrInvalidFilterOperator is returned when a custom field filter has an unrecognized operator
	ErrInvalidFilterOperator = errors.New("invalid filter operator")
	// ErrInvalidFilterValue is returned when a custom field filter has an empty value
	ErrInvalidFilterValue = errors.New("filter value must not be empty")
	// ErrInvalidSortOrder is returned when a sort order is not one of the
	// recognised values (asc, desc). The empty string means "use the default"
	// and is always valid.
	ErrInvalidSortOrder = errors.New("sort order must be 'asc' or 'desc'")
	// ErrUnknownSystemFieldKey is returned when a system field key is not in the schema
	ErrUnknownSystemFieldKey = errors.New("unknown system field key")
	// ErrSystemFieldValidation is returned when a system field value fails schema validation
	ErrSystemFieldValidation = errors.New("system field validation failed")
	// ErrCustomFieldValidation is returned when a custom field value fails schema validation
	// (missing required field, wrong type, out-of-range, etc.). It wraps the per-field
	// detail so HTTP layers can map it to a 400 VALIDATION_ERROR via errors.Is.
	ErrCustomFieldValidation = errors.New("custom field validation failed")
	// ErrTranslationGroupNotFound is returned when a translation group ID does not exist
	ErrTranslationGroupNotFound = errors.New("translation group not found")
	// ErrTranslationAlreadyExists is returned when a translation for a language already exists in a group
	ErrTranslationAlreadyExists = errors.New("translation for this language already exists in this group")
	// ErrInvalidLanguage is returned when the language code is not in the configured languages
	ErrInvalidLanguage = errors.New("language is not in the configured languages list")
	// ErrInvalidFormat is returned when the content format is not one of the supported values
	ErrInvalidFormat = errors.New("format must be one of: tiptap, markdown, html")
	// ErrForbiddenPostType is returned when the caller's role cannot manage the
	// requested post type (config-defined role with a restricted manage list).
	ErrForbiddenPostType = errors.New("role cannot manage this post type")
	// ErrForbiddenPublish is returned when the caller's role has no publish
	// capability but attempts the publish operation.
	ErrForbiddenPublish = errors.New("role cannot publish content")
)

var defaultSanitizer *sanitize.Sanitizer

func init() {
	defaultSanitizer = sanitize.NewSanitizer()
}

// Status represents the publication status of content
type Status string

const (
	// StatusDraft represents content that is not yet published
	StatusDraft Status = "draft"
	// StatusPublished represents content that is published
	StatusPublished Status = "published"
)

const (
	// RoleAdmin represents the admin role used for authorization checks
	RoleAdmin = "Admin"
)

// IsValid checks if the status is valid
func (s Status) IsValid() bool {
	return s == StatusDraft || s == StatusPublished
}

// String returns the string representation of the status
func (s Status) String() string {
	return string(s)
}

// Format represents the content format used for storage and rendering
type Format string

const (
	// FormatTiptap stores content as TipTap JSON (default)
	FormatTiptap Format = "tiptap"
	// FormatMarkdown stores content as Markdown (converted to TipTap on write)
	FormatMarkdown Format = "markdown"
	// FormatHTML stores content as raw HTML with inline styles
	FormatHTML Format = "html"
)

// IsValid checks if the format is supported
func (f Format) IsValid() bool {
	return f == FormatTiptap || f == FormatMarkdown || f == FormatHTML
}

// String returns the string representation of the format
func (f Format) String() string {
	return string(f)
}

// CommentStatus represents the moderation status of a comment
type CommentStatus string

const (
	// CommentStatusPending represents a comment awaiting moderation
	CommentStatusPending CommentStatus = "pending"
	// CommentStatusApproved represents an approved comment
	CommentStatusApproved CommentStatus = "approved"
	// CommentStatusRejected represents a rejected comment
	CommentStatusRejected CommentStatus = "rejected"
	// CommentStatusSpam represents a comment marked as spam
	CommentStatusSpam CommentStatus = "spam"
)

// IsValid checks if the comment status is valid
func (s CommentStatus) IsValid() bool {
	return s == CommentStatusPending || s == CommentStatusApproved || s == CommentStatusRejected || s == CommentStatusSpam
}

// String returns the string representation of the comment status
func (s CommentStatus) String() string {
	return string(s)
}

// IsApproved returns true if the comment status is approved
func (s CommentStatus) IsApproved() bool {
	return s == CommentStatusApproved
}

// IsPending returns true if the comment status is pending
func (s CommentStatus) IsPending() bool {
	return s == CommentStatusPending
}

// IsRejected returns true if the comment status is rejected
func (s CommentStatus) IsRejected() bool {
	return s == CommentStatusRejected
}

// IsSpam returns true if the comment status is spam
func (s CommentStatus) IsSpam() bool {
	return s == CommentStatusSpam
}

// Content represents a content item in the system
type Content struct {
	ID                 int            `json:"id"`
	UserID             int            `json:"userId"`
	Title              string         `json:"title"`
	Slug               string         `json:"slug"`
	Content            string         `json:"content"`
	Tags               []string       `json:"tags"`
	Status             Status         `json:"status"`
	Format             Format         `json:"format"`
	PostType           string         `json:"postType"`
	MetaDescription    string         `json:"metaDescription,omitempty"`
	OGTitle            string         `json:"ogTitle,omitempty"`
	OGDescription      string         `json:"ogDescription,omitempty"`
	Author             string         `json:"author,omitempty"`
	Username           string         `json:"username,omitempty"`
	AllowComments      bool           `json:"allowComments"`
	CustomFields       map[string]any `json:"customFields,omitempty"`
	UpdatedBy          int            `json:"updatedBy,omitempty"`
	UpdatedByUsername  string         `json:"updatedByUsername,omitempty"`
	Language           string         `json:"language"`
	TranslationGroupID *int           `json:"translationGroupId,omitempty"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
}

// PublishedAuthor is a read-model aggregating a user's published-content
// footprint. It is shaped by the repository (which owns the content↔user join)
// and re-shaped into a public DTO by the handler. CustomFields carries the raw
// user custom_fields map from the database; the handler projects only
// allowlisted slugs into the public response via the [[public_field]] expose
// operation. Email, role, and status are never populated — those stay behind
// authentication.
type PublishedAuthor struct {
	Username       string
	DisplayName    string
	ProfilePicture string
	ContentCount   int
	PostTypes      []string
	CustomFields   map[string]any
}

// PublishedAuthorFilters holds filter and sort parameters for the public
// authors listing. A zero-value filter behaves identically to the legacy
// (limit, offset) call — the default content_count DESC ranking is preserved.
//
// SortBy holds the custom-field slug with no prefix (the "cf:" prefix on the
// HTTP surface is stripped by the handler before constructing the filter).
// When SortBy is empty, content_count DESC remains the ranking. Custom-field
// sorts are numeric-only — non-numeric values sort as zero (per the numeric
// safety wrappers in each repository driver).
type PublishedAuthorFilters struct {
	Limit              int
	Offset             int
	SortBy             string
	SortOrder          string
	CustomFieldFilters []CustomFieldFilter
}

// ArchiveMonth is a read-model aggregating published-content counts by year
// and month. It is shaped by the repository's GROUP BY query and re-shaped
// into a public DTO (with a listing URL) by the handler.
type ArchiveMonth struct {
	Year  int
	Month int
	Count int
}

// FilterOperator represents the type of filter operation
type FilterOperator string

const (
	// FilterOpEqual filters for exact match
	FilterOpEqual FilterOperator = "equal"
	// FilterOpMin filters for minimum value (inclusive)
	FilterOpMin FilterOperator = "min"
	// FilterOpMax filters for maximum value (inclusive)
	FilterOpMax FilterOperator = "max"
)

// SortOrder represents the direction of an ORDER BY clause. The empty string
// is treated as "default" — the repository picks the direction (typically
// DESC for rankings).
type SortOrder string

const (
	// SortOrderAsc sorts from smallest to largest.
	SortOrderAsc SortOrder = "asc"
	// SortOrderDesc sorts from largest to smallest.
	SortOrderDesc SortOrder = "desc"
)

// ValidateSortOrder reports whether s is a valid SortOrder value (including
// the empty string, which means "no explicit direction — use the default").
func ValidateSortOrder(s string) error {
	if s == "" || s == string(SortOrderAsc) || s == string(SortOrderDesc) {
		return nil
	}
	return ErrInvalidSortOrder
}

// CustomFieldFilter represents a filter on a custom field
type CustomFieldFilter struct {
	Field    string
	Operator FilterOperator
	Value    string
}

// ContentFilters holds filter parameters for listing content. All string fields
// use the empty value as "no filter"; a non-empty value enables the corresponding
// WHERE clause. Tags is AND-of-tags: a content item must carry every tag in the
// slice to match.
//
// Status validation (draft/published) is the caller's responsibility — the
// handler rejects unknown values before constructing the filter, so a raw
// ContentFilters{Status: "garbage"} would still pass through this struct.
//
// Author matches the joined users.name (with users.username as a fallback),
// case-insensitive equality. The agent v1 surface only honors it for admins;
// the agent handler returns 403 to non-admins before the filter is built.
//
// SortBy and SortOrder together enable an ORDER BY clause on a custom field.
// SortBy holds the field slug with no prefix (the "cf:" prefix on the public
// HTTP surface is stripped by the handler before constructing the filter).
// When SortBy is empty, the repository's default created_at DESC ordering
// applies. SortOrder is "asc", "desc", or "" (use the default).
type ContentFilters struct {
	Limit              int
	Offset             int
	PostType           string
	Search             string
	Language           string
	Status             string
	Tags               []string
	Author             string
	CustomFieldFilters []CustomFieldFilter
	SortBy             string
	SortOrder          string
}

// MaxCustomFieldFilters limits the number of custom field filters per query
const MaxCustomFieldFilters = 10

// ValidateCustomFieldFilter validates a single custom field filter
func ValidateCustomFieldFilter(f CustomFieldFilter) error {
	if f.Field == "" {
		return ErrInvalidFilterField
	}
	if f.Operator != FilterOpEqual && f.Operator != FilterOpMin && f.Operator != FilterOpMax {
		return ErrInvalidFilterOperator
	}
	if f.Value == "" {
		return ErrInvalidFilterValue
	}
	return nil
}

// ValidateTitle validates the title field
func ValidateTitle(title string) error {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > MaxTitleRunes {
		return ErrInvalidTitle
	}
	if defaultSanitizer.ContainsHTML(title) {
		return ErrHTMLInTitle
	}
	return nil
}

// ValidateFormat validates the content format field
func ValidateFormat(format Format) error {
	if format == "" {
		return nil // empty defaults to tiptap at the service layer
	}
	if !format.IsValid() {
		return ErrInvalidFormat
	}
	return nil
}

// ValidateContent validates the content field. For tiptap and markdown formats
// the content must be valid TipTap JSON; for html format it must be non-empty
// and within the length limit (the HTML sanitizer runs at a later stage).
func ValidateContent(content string, format Format) error {
	if content == "" {
		return ErrEmptyContent
	}
	if utf8.RuneCountInString(content) > 100000 {
		return ErrContentTooLong
	}
	if format == FormatHTML {
		return nil
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		return ErrInvalidTipTapContent
	}
	if err := sanitize.ValidateTipTapDocument(doc); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidTipTapContent, err.Error())
	}
	return nil
}

// ValidateSlug validates the slug field
func ValidateSlug(slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" || utf8.RuneCountInString(slug) > 200 {
		return ErrInvalidSlug
	}
	for _, r := range slug {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return ErrInvalidSlug
		}
	}
	return nil
}

// ValidateTags validates and normalizes the tags field.
// Returns the sanitized tags or an error.
func ValidateTags(tags []string) ([]string, error) {
	trimmed := make([]string, 0, len(tags))
	for _, tag := range tags {
		t := strings.TrimSpace(tag)
		if t == "" || utf8.RuneCountInString(t) > MaxTagRunes {
			return nil, errors.New("each tag must be between 1 and 50 characters")
		}
		if defaultSanitizer.ContainsHTML(t) {
			return nil, ErrHTMLInPlainText
		}
		trimmed = append(trimmed, t)
	}
	return trimmed, nil
}

// ValidateMetaDescription validates the meta description field
func ValidateMetaDescription(value string) error {
	if value == "" {
		return nil
	}
	if defaultSanitizer.ContainsHTML(value) {
		return ErrHTMLInPlainText
	}
	return nil
}

// ValidateOGTitle validates the OG title field
func ValidateOGTitle(value string) error {
	if value == "" {
		return nil
	}
	if defaultSanitizer.ContainsHTML(value) {
		return ErrHTMLInPlainText
	}
	return nil
}

// ValidateOGDescription validates the OG description field
func ValidateOGDescription(value string) error {
	if value == "" {
		return nil
	}
	if defaultSanitizer.ContainsHTML(value) {
		return ErrHTMLInPlainText
	}
	return nil
}

// Comment represents a comment on a content item
type Comment struct {
	ID           int           `json:"id"`
	ContentID    int           `json:"contentId"`
	UserID       int           `json:"userId"`
	Comment      string        `json:"comment"`
	Status       CommentStatus `json:"status"`
	Author       string        `json:"author,omitempty"`
	Username     string        `json:"username,omitempty"`
	Role         string        `json:"role,omitempty"`
	ContentTitle string        `json:"contentTitle,omitempty"`
	ContentSlug  string        `json:"contentSlug,omitempty"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
}

// ValidateCommentText validates the comment text field
func ValidateCommentText(comment string) error {
	comment = strings.TrimSpace(comment)
	if comment == "" || utf8.RuneCountInString(comment) > 2000 {
		return ErrInvalidCommentText
	}
	if defaultSanitizer.ContainsHTML(comment) {
		return ErrHTMLInPlainText
	}
	return nil
}

// ValidateCommentStatus validates the comment status
func ValidateCommentStatus(status CommentStatus) error {
	if !status.IsValid() {
		return ErrInvalidCommentStatus
	}
	return nil
}
