package tiptap

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

// mobileMaxWidth caps what small viewports may load: the widest rendition at
// or below this width (the default _medium pipeline width). A plain srcset
// cannot promise that — a high-DPR phone resolves a narrow CSS slot against
// the largest entry — so the renderer gates it behind a media query.
const mobileMaxWidth = 280

func escapeHTML(s string) string {
	return html.EscapeString(s)
}

func escapeAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// mobileVariant picks the rendition small viewports must load: the widest
// variant at or below mobileMaxWidth, falling back to the smallest variant
// when every rendition exceeds it. Empty when there are no variants.
func mobileVariant(variants []ImageVariant) string {
	best := ""
	bestWidth := 0
	smallest := ""
	smallestWidth := 0
	for _, v := range variants {
		if v.URL == "" || v.Width <= 0 {
			continue
		}
		if smallest == "" || v.Width < smallestWidth {
			smallest, smallestWidth = v.URL, v.Width
		}
		if v.Width <= mobileMaxWidth && v.Width > bestWidth {
			best, bestWidth = v.URL, v.Width
		}
	}
	if best != "" {
		return best
	}
	return smallest
}

// largestVariantSize returns the dimensions of the widest variant, so the
// emitted width/height describe the full rendition and the browser can
// reserve the aspect ratio before the image loads. Zeroes when unknown.
func largestVariantSize(variants []ImageVariant) (width, height int) {
	for _, v := range variants {
		if v.Width > width {
			width, height = v.Width, v.Height
		}
	}
	return width, height
}

// firstImageSrc walks the document depth-first and returns the src of the
// first image node. The first content image is the LCP element on post and
// page views, so the renderer prioritizes exactly that one.
func firstImageSrc(doc document) string {
	var walk func(nodes []node) string
	walk = func(nodes []node) string {
		for i := range nodes {
			if nodes[i].Type == "image" {
				if src, _ := nodes[i].Attrs["src"].(string); src != "" {
					return src
				}
			}
			if src := walk(nodes[i].Content); src != "" {
				return src
			}
		}
		return ""
	}
	return walk(doc.Content)
}

// ImageResolver maps an image src to its sized variants for rendering.
type ImageResolver func(src string) []ImageVariant

// Renderer converts TipTap JSON documents to HTML.
type Renderer struct {
	imageResolver ImageResolver
	// firstImageSrc is the src of the first content image in the document
	// under render, set by Render before dispatch. renderImage treats that
	// image as the LCP candidate (eager load); every later image stays
	// lazy. It lives on the struct rather than in a parameter so the
	// recursive render chain needs no signature changes: Render works on a
	// copy, so concurrent renders never share it.
	firstImageSrc string
}

func (r Renderer) renderNode(n node) string {
	switch n.Type {
	case "doc":
		return r.renderChildren(n.Content)
	case "paragraph":
		return r.renderParagraph(n)
	case "heading":
		return r.renderHeading(n)
	case "bulletList":
		return r.renderList("ul", n)
	case "orderedList":
		return r.renderList("ol", n)
	case "listItem":
		return r.renderListItem(n)
	case "blockquote":
		return r.renderBlockquote(n)
	case "codeBlock":
		return r.renderCodeBlock(n)
	case "hardBreak":
		return "<br>"
	case "horizontalRule":
		return `<hr class="content-wrapper">`
	case "image":
		return r.renderImage(n)
	case "table":
		return r.renderTable(n)
	case "tableRow":
		return r.renderTableRow(n)
	case "tableCell":
		return r.renderTableCell("td", n)
	case "tableHeader":
		return r.renderTableCell("th", n)
	case "text":
		return r.renderText(n)
	case "youtube":
		return r.renderYoutube(n)
	case "emoji":
		return r.renderEmoji(n)
	case "inlineMath":
		return r.renderInlineMath(n)
	case "blockMath":
		return r.renderBlockMath(n)
	default:
		return ""
	}
}

func (r Renderer) renderParagraph(n node) string {
	content := r.renderChildren(n.Content)
	if content == "" {
		return `<p class="content-wrapper"></p>`
	}
	return fmt.Sprintf(`<p class="content-wrapper">%s</p>`, content)
}

func (r Renderer) renderHeading(n node) string {
	level := 1
	if l, ok := n.Attrs["level"]; ok {
		if levelFloat, ok := l.(float64); ok {
			level = int(levelFloat)
		}
	}
	if level < 1 {
		level = 1
	}
	if level > 6 {
		level = 6
	}

	content := r.renderChildren(n.Content)
	return fmt.Sprintf(`<h%d class="content-wrapper">%s</h%d>`, level, content, level)
}

func (r Renderer) renderList(tag string, n node) string {
	content := r.renderChildren(n.Content)
	if tag == "ol" {
		var extra string
		if startVal, ok := n.Attrs["start"]; ok {
			start := 1
			switch v := startVal.(type) {
			case float64:
				start = int(v)
			case int:
				start = v
			}
			if start != 1 {
				extra += fmt.Sprintf(` start="%d"`, start)
			}
		}
		if typeVal, ok := n.Attrs["type"]; ok {
			if typeStr, ok := typeVal.(string); ok && typeStr != "" {
				extra += fmt.Sprintf(` type="%s"`, escapeAttr(typeStr))
			}
		}
		return fmt.Sprintf(`<ol class="content-wrapper"%s>%s</ol>`, extra, content)
	}
	return fmt.Sprintf(`<%s class="content-wrapper">%s</%s>`, tag, content, tag)
}

func (r Renderer) renderListItem(n node) string {
	content := r.renderChildren(n.Content)
	return fmt.Sprintf("<li>%s</li>", content)
}

func (r Renderer) renderBlockquote(n node) string {
	content := r.renderChildren(n.Content)
	return fmt.Sprintf(`<blockquote class="content-wrapper">%s</blockquote>`, content)
}

func (r Renderer) renderCodeBlock(n node) string {
	var sb strings.Builder
	sb.WriteString(`<pre class="content-wrapper"><code`)

	if language, ok := n.Attrs["language"].(string); ok && language != "" {
		fmt.Fprintf(&sb, ` class="language-%s"`, escapeAttr(language))
	}

	sb.WriteString(">")
	sb.WriteString(r.renderCodeChildren(n.Content))
	sb.WriteString("</code></pre>")
	return sb.String()
}

func (r Renderer) renderImage(n node) string {
	src, _ := n.Attrs["src"].(string)
	if src == "" {
		return ""
	}

	var variants []ImageVariant
	if r.imageResolver != nil {
		variants = r.imageResolver(src)
	}

	var sb strings.Builder
	sb.WriteString("<figure>")
	wrapped := false
	if mobile := mobileVariant(variants); mobile != "" {
		fmt.Fprintf(&sb, `<picture><source media="(max-width: 767px)" srcset="%s">`, escapeAttr(mobile))
		wrapped = true
	}
	fmt.Fprintf(&sb, `<img src="%s"`, escapeAttr(src))

	if len(variants) > 0 {
		sb.WriteString(` srcset="`)
		for i, v := range variants {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%s %dw", escapeAttr(v.URL), v.Width)
		}
		sb.WriteString(`"`)
		sb.WriteString(` sizes="(max-width: 768px) 100vw, 760px"`)
		// Reserve the aspect ratio from the widest variant. Variants are
		// downscaled proportionally, so the widest variant's ratio matches
		// the original's even when src points at a larger file.
		if w, h := largestVariantSize(variants); w > 0 && h > 0 {
			fmt.Fprintf(&sb, ` width="%d" height="%d"`, w, h)
		}
	}

	if alt, ok := n.Attrs["alt"].(string); ok && alt != "" {
		fmt.Fprintf(&sb, ` alt="%s"`, escapeAttr(alt))
	}
	if title, ok := n.Attrs["title"].(string); ok && title != "" {
		fmt.Fprintf(&sb, ` title="%s"`, escapeAttr(title))
	}

	if src == r.firstImageSrc {
		sb.WriteString(` loading="eager" fetchpriority="high">`)
	} else {
		sb.WriteString(` loading="lazy" decoding="async">`)
	}
	if wrapped {
		sb.WriteString("</picture>")
	}
	sb.WriteString("</figure>")
	return sb.String()
}

func (r Renderer) renderTable(n node) string {
	content := r.renderChildren(n.Content)
	return fmt.Sprintf(`<div class="table-wrapper"><table>%s</table></div>`, content)
}

func (r Renderer) renderTableRow(n node) string {
	content := r.renderChildren(n.Content)
	return fmt.Sprintf("<tr>%s</tr>", content)
}

func (r Renderer) renderTableCell(tag string, n node) string {
	content := r.renderChildren(n.Content)
	return fmt.Sprintf("<%s>%s</%s>", tag, content, tag)
}

func (r Renderer) renderText(n node) string {
	if len(n.Marks) > 0 {
		return renderMarks(n.Text, n.Marks)
	}
	return escapeHTML(n.Text)
}

func (r Renderer) renderChildren(children []node) string {
	var sb strings.Builder
	for _, child := range children {
		sb.WriteString(r.renderNode(child))
	}
	return sb.String()
}

func (r Renderer) renderCodeChildren(children []node) string {
	var sb strings.Builder
	for _, child := range children {
		if child.Type == "text" {
			sb.WriteString(escapeHTML(child.Text))
		}
	}
	return sb.String()
}

func (r Renderer) renderEmoji(n node) string {
	name, _ := n.Attrs["name"].(string)
	if name == "" {
		return ""
	}
	if emoji, ok := emojiShortcodeToChar[name]; ok {
		return fmt.Sprintf(`<span class="emoji" data-name="%s">%s</span>`, escapeAttr(name), emoji)
	}
	return fmt.Sprintf(`<span class="emoji" data-name="%s">:%s:</span>`, escapeAttr(name), escapeHTML(name))
}

func (r Renderer) renderInlineMath(n node) string {
	latex, _ := n.Attrs["latex"].(string)
	if latex == "" {
		return ""
	}
	return fmt.Sprintf(`<span class="math-inline">%s</span>`, escapeHTML(latex))
}

func (r Renderer) renderBlockMath(n node) string {
	latex, _ := n.Attrs["latex"].(string)
	if latex == "" {
		return ""
	}
	return fmt.Sprintf(`<div class="math-block">%s</div>`, escapeHTML(latex))
}

func (r Renderer) renderYoutube(n node) string {
	src, _ := n.Attrs["src"].(string)
	if src == "" {
		return ""
	}
	return fmt.Sprintf(
		`<div class="embed-wrapper"><iframe src="%s" frameborder="0" allowfullscreen allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"></iframe></div>`,
		escapeAttr(src),
	)
}

// Render converts a TipTap JSON document to HTML, resolving images through
// the configured ImageResolver.
func (r Renderer) Render(tiptapJSON string) (string, error) {
	var doc document
	if err := json.Unmarshal([]byte(tiptapJSON), &doc); err != nil {
		return "", fmt.Errorf("failed to parse tiptap json: %w", err)
	}

	if doc.Type != "doc" {
		return "", fmt.Errorf("invalid tiptap document: expected root type 'doc', got %q", doc.Type)
	}

	r.firstImageSrc = firstImageSrc(doc)

	var sb strings.Builder
	for _, child := range doc.Content {
		sb.WriteString(r.renderNode(child))
	}
	return sb.String(), nil
}

// NewRenderer builds a Renderer resolving images through resolver.
func NewRenderer(resolver ImageResolver) Renderer {
	return Renderer{imageResolver: resolver}
}
