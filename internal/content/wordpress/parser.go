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

// collectMeta builds a raw key→value map from an item's postmeta entries. All
// entries are preserved (including ACF internal keys prefixed with "_") so that
// downstream consumers (e.g. featured-image resolution) can access them.
func collectMeta(postmeta []postMeta) map[string]string {
	if len(postmeta) == 0 {
		return nil
	}
	meta := make(map[string]string, len(postmeta))
	for _, pm := range postmeta {
		key := strings.TrimSpace(pm.Key)
		if key == "" {
			continue
		}
		meta[key] = pm.Value
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

// Parse reads a WordPress eXtended RSS (WXR) stream and returns a normalized
// document containing only items whose post type is in allowedTypes. Statuses
// are mapped to the Lesstruct vocabulary ("publish" → "published", everything
// else → "draft"). Tags are collected from item-level category elements with
// domain "post_tag" or "category". Custom field values (<wp:postmeta>) are
// collected into each item's Meta map.
func Parse(r io.Reader, allowedTypes map[string]bool) (*WXRDocument, error) {
	var root rss
	decoder := xml.NewDecoder(r)
	decoder.Strict = false
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("failed to decode WXR XML: %w", err)
	}

	doc := &WXRDocument{
		SiteTitle:  strings.TrimSpace(root.Channel.Title),
		SiteURL:    strings.TrimSpace(root.Channel.BaseBlogURL),
		Authors:    make([]ParsedAuthor, 0, len(root.Channel.Authors)),
		Items:      make([]ParsedItem, 0, len(root.Channel.Items)),
		Attachments: make(map[int]string),
	}

	for _, a := range root.Channel.Authors {
		login := strings.TrimSpace(a.Login)
		if login == "" {
			continue
		}
		doc.Authors = append(doc.Authors, ParsedAuthor{
			Login:       login,
			Email:       strings.TrimSpace(a.Email),
			DisplayName: strings.TrimSpace(a.DisplayName),
		})
	}

	for _, it := range root.Channel.Items {
		postType := strings.TrimSpace(it.PostType)

		// Capture attachment URLs for featured-image resolution before filtering.
		if postType == "attachment" {
			url := strings.TrimSpace(it.AttachmentURL)
			if url != "" && it.PostID != 0 {
				doc.Attachments[it.PostID] = url
			}
		}

		if !allowedTypes[postType] {
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
			Creator:  strings.TrimSpace(it.Creator),
			Meta:     collectMeta(it.PostMeta),
		})
	}

	return doc, nil
}
