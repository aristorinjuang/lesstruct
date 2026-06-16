package wordpress

import (
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// ttNode is a TipTap document node. It mirrors the structure validated by
// internal/domain/sanitize, so only allowed types/attrs/marks are ever emitted.
type ttNode struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []ttNode       `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []ttMark       `json:"marks,omitempty"`
}

// ttMark is a TipTap mark (inline formatting).
type ttMark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// convertBlock converts a single WordPress block into one or more TipTap nodes.
func convertBlock(b *wpBlock, imageMap map[string]string) []ttNode {
	switch b.name {
	case "paragraph":
		return paragraphFromInline(parseInline(b.raw))
	case "heading":
		return []ttNode{headingNode(b, imageMap)}
	case "quote", "pullquote":
		return quoteNode(b, imageMap)
	case "list":
		return listNode(b)
	case "list-item":
		// Standalone list-items (shouldn't normally occur) become paragraphs.
		return paragraphFromInline(parseInline(b.raw))
	case "image", "cover":
		return imageNode(b, imageMap)
	case "media-text":
		return mediaTextNode(b, imageMap)
	case "gallery":
		return galleryNode(b, imageMap)
	case "code":
		return codeNode(b)
	case "table":
		return tableNode(b)
	case "embed":
		return embedNode(b)
	case "math":
		return mathNode(b)
	case "verse":
		return verseNode(b)
	case "details":
		return detailsNode(b, imageMap)
	case "audio", "file":
		return linkParagraphNode(b)
	case "columns", "column", "group", "social-links":
		return flattenChildren(b, imageMap)
	case "more", "nextpage", "separator", "spacer":
		return []ttNode{{Type: "horizontalRule"}}
	default:
		text := collapseText(b.raw)
		if strings.TrimSpace(text) == "" {
			return flattenChildren(b, imageMap)
		}
		return []ttNode{{Type: "paragraph", Content: []ttNode{{Type: "text", Text: text}}}}
	}
}

// flattenChildren converts a container block's children and returns their nodes
// in order, discarding the wrapper markup.
func flattenChildren(b *wpBlock, imageMap map[string]string) []ttNode {
	var nodes []ttNode
	for _, c := range b.children {
		nodes = append(nodes, convertBlock(c, imageMap)...)
	}
	return nodes
}

func headingNode(b *wpBlock, _ map[string]string) ttNode {
	level := 2
	if v, ok := b.attrs["level"].(float64); ok && v >= 1 && v <= 6 {
		level = int(v)
	} else if l, ok := detectHeadingLevel(b.raw); ok {
		level = l
	}
	content := parseInline(b.raw)
	if len(content) == 0 {
		content = nil
	}
	return ttNode{
		Type:    "heading",
		Attrs:   map[string]any{"level": level},
		Content: content,
	}
}

func quoteNode(b *wpBlock, imageMap map[string]string) []ttNode {
	var content []ttNode
	if len(b.children) > 0 {
		for _, c := range b.children {
			content = append(content, convertBlock(c, imageMap)...)
		}
	} else {
		content = paragraphFromInline(parseInline(b.raw))
	}
	cleanupNodes(&content)
	if len(content) == 0 {
		content = []ttNode{{Type: "paragraph"}}
	}
	return []ttNode{{Type: "blockquote", Content: content}}
}

func listNode(b *wpBlock) []ttNode {
	listType := "bulletList"
	if ordered, ok := b.attrs["ordered"].(bool); ok && ordered {
		listType = "orderedList"
	}

	var items []ttNode
	if hasListItemChildren(b) {
		for _, c := range b.children {
			if c.name != "list-item" {
				continue
			}
			items = append(items, listItemFromBlock(c))
		}
	} else {
		for _, text := range extractListItems(b.raw) {
			items = append(items, makeListItem(text))
		}
	}
	if len(items) == 0 {
		return nil
	}
	return []ttNode{{Type: listType, Content: items}}
}

func hasListItemChildren(b *wpBlock) bool {
	for _, c := range b.children {
		if c.name == "list-item" {
			return true
		}
	}
	return false
}

func listItemFromBlock(b *wpBlock) ttNode {
	content := parseInline(b.raw)
	para := ttNode{Type: "paragraph"}
	if len(content) > 0 {
		para.Content = content
	}
	return ttNode{Type: "listItem", Content: []ttNode{para}}
}

func makeListItem(text string) ttNode {
	text = strings.TrimSpace(text)
	para := ttNode{Type: "paragraph"}
	if text != "" {
		para.Content = []ttNode{{Type: "text", Text: text}}
	}
	return ttNode{Type: "listItem", Content: []ttNode{para}}
}

func imageNode(b *wpBlock, imageMap map[string]string) []ttNode {
	src, ok := findTagAttr(b.raw, "img", "src")
	if !ok {
		if u, ok := b.attrs["url"].(string); ok {
			src = u
		}
	}
	if src == "" {
		return nil
	}
	alt, _ := findTagAttr(b.raw, "img", "alt")
	attrs := map[string]any{"src": remapURL(src, imageMap)}
	if strings.TrimSpace(alt) != "" {
		attrs["alt"] = alt
	}
	return []ttNode{{Type: "image", Attrs: attrs}}
}

func mediaTextNode(b *wpBlock, imageMap map[string]string) []ttNode {
	var nodes []ttNode
	nodes = append(nodes, imageNode(b, imageMap)...)
	nodes = append(nodes, paragraphFromInline(parseInline(b.raw))...)
	cleanupNodes(&nodes)
	return nodes
}

func galleryNode(b *wpBlock, imageMap map[string]string) []ttNode {
	var nodes []ttNode
	for _, c := range b.children {
		if c.name == "image" {
			nodes = append(nodes, imageNode(c, imageMap)...)
		}
	}
	return nodes
}

func codeNode(b *wpBlock) []ttNode {
	text := extractTagText(b.raw, "code")
	if text == "" {
		text = collapseText(b.raw)
	}
	return []ttNode{{Type: "codeBlock", Content: []ttNode{{Type: "text", Text: text}}}}
}

func tableNode(b *wpBlock) []ttNode {
	rows := extractTableRows(b.raw)
	if len(rows) == 0 {
		return nil
	}
	var content []ttNode
	for _, row := range rows {
		content = append(content, ttNode{Type: "tableRow", Content: row})
	}
	return []ttNode{{Type: "table", Content: content}}
}

// extractTableRows scans a raw <table> fragment and returns rows of cell nodes
// (tableCell or tableHeader, each holding a paragraph with the cell text).
func extractTableRows(htmlStr string) [][]ttNode {
	var rows [][]ttNode
	var currentRow []ttNode
	var cellText strings.Builder
	inCell := false
	cellType := "tableCell"

	tokenizer := html.NewTokenizer(strings.NewReader(htmlStr))
	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			return rows
		}
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := tokenizer.TagName()
			switch string(name) {
			case "td":
				inCell = true
				cellType = "tableCell"
				cellText.Reset()
			case "th":
				inCell = true
				cellType = "tableHeader"
				cellText.Reset()
			}
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			switch string(name) {
			case "td", "th":
				if inCell {
					text := strings.TrimSpace(cellText.String())
					para := ttNode{Type: "paragraph"}
					if text != "" {
						para.Content = []ttNode{{Type: "text", Text: text}}
					}
					currentRow = append(currentRow, ttNode{
						Type:    cellType,
						Content: []ttNode{para},
					})
				}
				inCell = false
			case "tr":
				if len(currentRow) > 0 {
					rows = append(rows, currentRow)
					currentRow = nil
				}
			}
		case html.TextToken:
			if inCell {
				cellText.Write(tokenizer.Text())
			}
		}
	}
}

func mathNode(b *wpBlock) []ttNode {
	latex, _ := b.attrs["latex"].(string)
	if latex == "" {
		latex = extractAnnotationLatex(b.raw)
	}
	if latex == "" {
		return nil
	}
	return []ttNode{{Type: "blockMath", Attrs: map[string]any{"latex": latex}}}
}

func verseNode(b *wpBlock) []ttNode {
	text := collapseText(b.raw)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return []ttNode{{
		Type: "paragraph",
		Content: []ttNode{{
			Type:  "text",
			Text:  text,
			Marks: []ttMark{{Type: "italic"}},
		}},
	}}
}

func detailsNode(b *wpBlock, imageMap map[string]string) []ttNode {
	var nodes []ttNode
	if summary := extractTagText(b.raw, "summary"); strings.TrimSpace(summary) != "" {
		nodes = append(nodes, ttNode{
			Type: "paragraph",
			Content: []ttNode{{
				Type:  "text",
				Text:  summary,
				Marks: []ttMark{{Type: "bold"}},
			}},
		})
	}
	for _, c := range b.children {
		nodes = append(nodes, convertBlock(c, imageMap)...)
	}
	if len(nodes) == 0 {
		nodes = paragraphFromInline(parseInline(b.raw))
	}
	return nodes
}

func embedNode(b *wpBlock) []ttNode {
	url, _ := b.attrs["url"].(string)
	provider, _ := b.attrs["providerNameSlug"].(string)
	if url == "" {
		if src, ok := findTagAttr(b.raw, "iframe", "src"); ok {
			url = src
		}
	}
	if embed := youtubeEmbedURL(url, provider); embed != "" {
		return []ttNode{{Type: "youtube", Attrs: map[string]any{"src": embed}}}
	}
	if url == "" {
		return nil
	}
	return []ttNode{linkParagraph("Embed", url)}
}

// linkParagraphNode emits a paragraph with a link to the media referenced by
// audio/file blocks, since TipTap has no dedicated media node for these.
func linkParagraphNode(b *wpBlock) []ttNode {
	if href, ok := findTagAttr(b.raw, "a", "href"); ok && href != "" {
		text, _ := findTagText(b.raw, "a")
		if strings.TrimSpace(text) == "" {
			text = "Link"
		}
		return []ttNode{linkParagraph(text, href)}
	}
	if src, ok := findTagAttr(b.raw, "audio", "src"); ok && src != "" {
		return []ttNode{linkParagraph("Audio", src)}
	}
	return nil
}

func linkParagraph(text, href string) ttNode {
	return ttNode{
		Type: "paragraph",
		Content: []ttNode{{
			Type:  "text",
			Text:  strings.TrimSpace(text),
			Marks: []ttMark{{Type: "link", Attrs: map[string]any{"href": href}}},
		}},
	}
}

// paragraphFromInline wraps inline nodes in a paragraph. Empty inline results
// are dropped so we don't create spurious blank paragraphs.
func paragraphFromInline(inline []ttNode) []ttNode {
	if len(inline) == 0 {
		return nil
	}
	return []ttNode{{Type: "paragraph", Content: inline}}
}

// remapURL returns the local URL for a WordPress source if it has been
// downloaded, otherwise the original URL unchanged.
func remapURL(src string, imageMap map[string]string) string {
	if local, ok := imageMap[src]; ok && local != "" {
		return local
	}
	return src
}

// cleanupNodes removes empty nodes in place to keep the document tidy.
func cleanupNodes(nodes *[]ttNode) {
	cleaned := (*nodes)[:0]
	for _, n := range *nodes {
		if isEmptyNode(n) {
			continue
		}
		cleaned = append(cleaned, n)
	}
	*nodes = cleaned
}

func isEmptyNode(n ttNode) bool {
	if n.Type == "text" {
		return strings.TrimSpace(n.Text) == "" && n.Text != " "
	}
	if len(n.Content) == 0 && n.Text == "" && n.Type != "horizontalRule" &&
		n.Type != "hardBreak" && n.Type != "image" && n.Type != "youtube" &&
		n.Type != "blockMath" && n.Type != "paragraph" {
		return true
	}
	return false
}

// ConvertBlocks converts WordPress block-editor HTML into a TipTap JSON document
// string. imageMap remaps WordPress image URLs to local media URLs; entries not
// present in the map keep their original URL. Unsupported blocks degrade to a
// plain paragraph so no content is silently lost.
func ConvertBlocks(wpContent string, imageMap map[string]string) (string, error) {
	root := tokenizeBlocks(wpContent)

	var content []ttNode
	if strings.TrimSpace(root.raw) != "" {
		content = append(content, paragraphFromInline(parseInline(root.raw))...)
	}
	for _, child := range root.children {
		content = append(content, convertBlock(child, imageMap)...)
	}

	cleanupNodes(&content)
	if len(content) == 0 {
		content = []ttNode{{Type: "paragraph"}}
	}

	doc := ttNode{Type: "doc", Content: content}
	data, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("failed to marshal TipTap document: %w", err)
	}
	return string(data), nil
}

// ExtractImageURLs scans WordPress block HTML and returns every image source
// URL found (img src, plus cover/gallery background URLs in block attrs). The
// importer uses this to know which media to download before conversion.
func ExtractImageURLs(wpContent string) []string {
	root := tokenizeBlocks(wpContent)
	seen := make(map[string]struct{})
	var urls []string
	collect := func(u string) {
		u = strings.TrimSpace(u)
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		urls = append(urls, u)
	}
	var walk func(b *wpBlock)
	walk = func(b *wpBlock) {
		switch b.name {
		case "image", "cover", "media-text":
			if src, ok := findTagAttr(b.raw, "img", "src"); ok {
				collect(src)
			}
		}
		if u, ok := b.attrs["url"].(string); ok {
			collect(u)
		}
		for _, c := range b.children {
			walk(c)
		}
	}
	for _, c := range root.children {
		walk(c)
	}
	return urls
}
