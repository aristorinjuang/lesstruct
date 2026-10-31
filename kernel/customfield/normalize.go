package customfield

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"
)

// Default caps for the array types when the schema sets no max_items. They
// bound the stored JSON; an explicit max_items overrides them (and must be
// positive — see ValidateMaxItems).
const (
	DefaultMaxItemsList    = 100
	DefaultMaxItemsGallery = 50
)

var (
	// ErrItemNotAllowed is returned when a multiselect value carries an item
	// outside the configured options.
	ErrItemNotAllowed = errors.New("item must be one of the configured options")
	// ErrItemEmpty is returned when a list value carries an empty item.
	ErrItemEmpty = errors.New("items must not be empty")
	// ErrItemNotString is returned when a list or multiselect item is not a
	// JSON string.
	ErrItemNotString = errors.New("items must be strings")
	// ErrItemLimitExceeded is returned when an array value carries more items
	// than the field allows.
	ErrItemLimitExceeded = errors.New("too many items")
	// ErrGalleryItemInvalid is returned when a gallery item is not an object
	// with a url (and optional alt) string pair.
	ErrGalleryItemInvalid = errors.New("gallery items must be objects with a url and an optional alt string")
	// ErrGalleryURLInvalid is returned when a gallery item URL is neither an
	// http(s) URL nor a root-relative media path.
	ErrGalleryURLInvalid = errors.New("gallery item url must be an http(s) URL or a root-relative path")
)

// itemChecker validates one canonical string item.
type itemChecker func(item string) error

func normalizeStringArray(f FieldSchema, value any, legacySeparator string, check itemChecker) (any, error) {
	var items []string
	switch v := value.(type) {
	case string:
		items = splitNonEmpty(v, legacySeparator)
	case []any:
		items = make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, ErrItemNotString
			}
			items = append(items, s)
		}
	default:
		return nil, ErrItemNotString
	}

	limit := arrayLimit(f, DefaultMaxItemsList)
	if len(items) > limit {
		return nil, fmt.Errorf("%w: at most %d allowed", ErrItemLimitExceeded, limit)
	}

	canonical := make([]any, 0, len(items))
	for _, item := range items {
		if err := check(item); err != nil {
			return nil, err
		}
		canonical = append(canonical, item)
	}
	return canonical, nil
}

func checkMultiselectItems(f FieldSchema) itemChecker {
	return func(item string) error {
		if !slices.Contains(f.Options, item) {
			return ErrItemNotAllowed
		}
		return nil
	}
}

// checkListItems closes over the field's per-item max_length.
func checkListItems(f FieldSchema) itemChecker {
	return func(item string) error {
		if strings.TrimSpace(item) == "" {
			return ErrItemEmpty
		}
		if f.MaxLength != nil && utf8.RuneCountInString(item) > *f.MaxLength {
			return fmt.Errorf("items must be at most %d characters", *f.MaxLength)
		}
		return nil
	}
}

func normalizeGallery(f FieldSchema, value any) (any, error) {
	var items []galleryItem
	switch v := value.(type) {
	case string:
		for _, line := range splitNonEmpty(v, "\n") {
			items = append(items, galleryItem{url: line})
		}
	case []any:
		items = make([]galleryItem, 0, len(v))
		for _, raw := range v {
			item, err := parseGalleryItem(raw)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
	default:
		return nil, ErrGalleryItemInvalid
	}

	limit := arrayLimit(f, DefaultMaxItemsGallery)
	if len(items) > limit {
		return nil, fmt.Errorf("%w: at most %d allowed", ErrItemLimitExceeded, limit)
	}

	canonical := make([]any, 0, len(items))
	for _, item := range items {
		if err := ValidateGalleryURL(item.url); err != nil {
			return nil, err
		}
		canonical = append(canonical, map[string]any{
			"url": item.url,
			"alt": item.alt,
		})
	}
	return canonical, nil
}

// galleryItem is one canonical gallery entry: a media-library URL and its
// optional alt text.
type galleryItem struct {
	url string
	alt string
}

func parseGalleryItem(raw any) (galleryItem, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return galleryItem{}, ErrGalleryItemInvalid
	}
	rawURL, ok := m["url"].(string)
	if !ok || strings.TrimSpace(rawURL) == "" {
		return galleryItem{}, ErrGalleryItemInvalid
	}
	alt := ""
	if v, exists := m["alt"]; exists && v != nil {
		s, ok := v.(string)
		if !ok {
			return galleryItem{}, ErrGalleryItemInvalid
		}
		alt = s
	}
	for key := range m {
		if key != "url" && key != "alt" {
			return galleryItem{}, ErrGalleryItemInvalid
		}
	}
	return galleryItem{url: rawURL, alt: alt}, nil
}

// splitNonEmpty splits a legacy string and drops blank segments (trailing
// separators, blank lines).
func splitNonEmpty(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, part)
		}
	}
	return out
}

// arrayLimit resolves the effective item cap: the schema's max_items when
// set, otherwise the type's default.
func arrayLimit(f FieldSchema, fallback int) int {
	if f.MaxItems != nil {
		return *f.MaxItems
	}
	return fallback
}

// ValidateGalleryURL accepts http(s) URLs and root-relative media paths —
// the same shapes the media pipeline and sanitizer allowlists use. Empty is
// rejected: callers that allow absent URLs check for empty first.
func ValidateGalleryURL(rawURL string) error {
	if strings.HasPrefix(rawURL, "/") && !strings.HasPrefix(rawURL, "//") {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ErrGalleryURLInvalid
	}
	return nil
}

// NormalizeValue canonicalizes and validates a custom-field value for the
// array types, accepting the legacy single-string forms so switching a
// field's type in config stays a data-compatible migration:
//
//   - multiselect: comma-separated string → []any{string…}; array values are
//     checked against the configured options.
//   - list: newline-separated string → []any{string…} (blank lines dropped);
//     per-item max_length and max_items enforced.
//   - gallery: one-URL-per-line string → []any{{url, alt}}; items are
//     {url, alt} objects with an http(s) or root-relative url.
//
// Canonical (already-array) input is validated and returned unchanged in
// shape; scalar field types pass through untouched (their validation lives
// in the content service). The returned value is always JSON-marshalable in
// the canonical storage shape: []any of strings, or []any of
// map[string]any{"url","alt"} for gallery.
func NormalizeValue(f FieldSchema, value any) (any, error) {
	switch f.Type {
	case FieldTypeMultiselect:
		return normalizeStringArray(f, value, ",", checkMultiselectItems(f))
	case FieldTypeList:
		return normalizeStringArray(f, value, "\n", checkListItems(f))
	case FieldTypeGallery:
		return normalizeGallery(f, value)
	default:
		return value, nil
	}
}
