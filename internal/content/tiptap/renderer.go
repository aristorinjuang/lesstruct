package tiptap

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

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

type ImageResolver func(src string) []ImageVariant

type Renderer struct {
	imageResolver ImageResolver
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

	var sb strings.Builder
	sb.WriteString("<figure>")
	fmt.Fprintf(&sb, `<img src="%s"`, escapeAttr(src))

	if r.imageResolver != nil {
		variants := r.imageResolver(src)
		if len(variants) > 0 {
			sb.WriteString(` srcset="`)
			for i, v := range variants {
				if i > 0 {
					sb.WriteString(", ")
				}
				fmt.Fprintf(&sb, "%s %dw", escapeAttr(v.URL), v.Width)
			}
			sb.WriteString(`"`)
			sb.WriteString(` sizes="100vw"`)
		}
	}

	if alt, ok := n.Attrs["alt"].(string); ok && alt != "" {
		fmt.Fprintf(&sb, ` alt="%s"`, escapeAttr(alt))
	}
	if title, ok := n.Attrs["title"].(string); ok && title != "" {
		fmt.Fprintf(&sb, ` title="%s"`, escapeAttr(title))
	}

	sb.WriteString(` loading="lazy">`)
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

func (r Renderer) Render(tiptapJSON string) (string, error) {
	var doc document
	if err := json.Unmarshal([]byte(tiptapJSON), &doc); err != nil {
		return "", fmt.Errorf("failed to parse tiptap json: %w", err)
	}

	if doc.Type != "doc" {
		return "", fmt.Errorf("invalid tiptap document: expected root type 'doc', got %q", doc.Type)
	}

	var sb strings.Builder
	for _, child := range doc.Content {
		sb.WriteString(r.renderNode(child))
	}
	return sb.String(), nil
}

func NewRenderer(resolver ImageResolver) Renderer {
	return Renderer{imageResolver: resolver}
}
