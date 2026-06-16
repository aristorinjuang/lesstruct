package wordpress

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// mapStatus converts a WordPress post status to a Lesstruct content status.
// Anything that is not explicitly "publish" is imported as a draft so that
// pending, scheduled, or private content is never accidentally published.
func mapStatus(wpStatus string) string {
	if strings.TrimSpace(wpStatus) == "publish" {
		return "published"
	}
	return "draft"
}

// collectTags gathers tag names from item-level category elements. Both
// "post_tag" and "category" domains are treated as tags. Duplicates are removed.
func collectTags(categories []itemCategory) []string {
	seen := make(map[string]struct{}, len(categories))
	tags := make([]string, 0, len(categories))
	for _, c := range categories {
		if c.Domain != "post_tag" && c.Domain != "category" {
			continue
		}
		name := strings.TrimSpace(c.Value)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		tags = append(tags, name)
	}
	return tags
}

// Parse reads a WordPress eXtended RSS (WXR) stream and returns a normalized
// document containing only post and page items. Statuses are mapped to the
// Lesstruct vocabulary ("publish" → "published", everything else → "draft").
// Tags are collected from item-level category elements with domain "post_tag"
// or "category".
func Parse(r io.Reader) (*WXRDocument, error) {
	var root rss
	decoder := xml.NewDecoder(r)
	decoder.Strict = false
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("failed to decode WXR XML: %w", err)
	}

	doc := &WXRDocument{
		SiteTitle: strings.TrimSpace(root.Channel.Title),
		SiteURL:   strings.TrimSpace(root.Channel.BaseBlogURL),
		Items:     make([]ParsedItem, 0, len(root.Channel.Items)),
	}

	for _, it := range root.Channel.Items {
		postType := strings.TrimSpace(it.PostType)
		if postType != "post" && postType != "page" {
			continue
		}

		doc.Items = append(doc.Items, ParsedItem{
			Title:    strings.TrimSpace(it.Title),
			Content:  it.ContentEncoded,
			Slug:     strings.TrimSpace(it.PostName),
			Status:   mapStatus(it.Status),
			PostType: postType,
			Tags:     collectTags(it.Categories),
			PubDate:  strings.TrimSpace(it.PubDate),
		})
	}

	return doc, nil
}
