package wordpress

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const wpNamespace = "http://wordpress.org/export/1.2/"

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

// skipElement reads tokens until the matching end tag of the current element is
// found, allowing the parser to recover from a failed DecodeElement and continue
// to the next sibling element.
func skipElement(decoder *xml.Decoder) error {
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

// Parse reads a WordPress eXtended RSS (WXR) stream and returns a normalized
// document containing only items whose post type is in allowedTypes. Items are
// decoded one at a time via streaming tokens so a single malformed item never
// aborts the entire parse — the bad item is skipped and parsing continues.
// Statuses are mapped to the Lesstruct vocabulary ("publish" → "published",
// everything else → "draft"). Tags are collected from item-level category
// elements with domain "post_tag" or "category". Custom field values
// (<wp:postmeta>) are collected into each item's Meta map.
func Parse(r io.Reader, allowedTypes map[string]bool) (*WXRDocument, error) {
	decoder := xml.NewDecoder(r)
	decoder.Strict = false

	doc := &WXRDocument{
		Attachments: make(map[int]string),
	}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decode WXR XML: %w", err)
		}

		se, ok := token.(xml.StartElement)
		if !ok {
			continue
		}

		switch se.Name.Local {
		case "item":
			var it item
			if err := decoder.DecodeElement(&it, &se); err != nil {
				_ = skipElement(decoder)
				continue
			}

			postType := strings.TrimSpace(it.PostType)

			if postType == "attachment" {
				url := strings.TrimSpace(it.AttachmentURL)
				if url != "" && it.PostID != 0 {
					doc.Attachments[it.PostID] = url
				}
				continue
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

		case "author":
			if se.Name.Space != wpNamespace {
				continue
			}
			var a author
			if err := decoder.DecodeElement(&a, &se); err != nil {
				continue
			}
			login := strings.TrimSpace(a.Login)
			if login == "" {
				continue
			}
			doc.Authors = append(doc.Authors, ParsedAuthor{
				Login:       login,
				Email:       strings.TrimSpace(a.Email),
				DisplayName: strings.TrimSpace(a.DisplayName),
			})

		case "title":
			var title string
			if err := decoder.DecodeElement(&title, &se); err == nil {
				if doc.SiteTitle == "" {
					doc.SiteTitle = strings.TrimSpace(title)
				}
			}

		case "base_blog_url":
			var url string
			if err := decoder.DecodeElement(&url, &se); err == nil {
				if doc.SiteURL == "" {
					doc.SiteURL = strings.TrimSpace(url)
				}
			}
		}
	}

	return doc, nil
}
