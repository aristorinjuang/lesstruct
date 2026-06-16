package markdown

import (
	"log"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/domain/sanitize"
	"github.com/yuin/goldmark/ast"
)

// ttNode is a TipTap document node. Its shape mirrors the structure validated
// by internal/domain/sanitize.ValidateTipTapDocument, so only node types,
// marks, and attributes in that allow-list are ever emitted.
type ttNode struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []ttNode       `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []ttMark       `json:"marks,omitempty"`
}

// ttMark is a TipTap mark (inline formatting such as bold or a link).
type ttMark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// sanitizer is the shared bluemonday-backed sanitizer reused for raw HTML.
// The strict policy (SanitizePlainText) strips every tag and attribute,
// leaving only escaped plain text — so raw HTML markup is never emitted.
var sanitizer = sanitize.NewSanitizer()

// renderBlocks converts the block-level children of a node into TipTap nodes.
func renderBlocks(n ast.Node, source []byte) []ttNode {
	nodes := make([]ttNode, 0)
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		nodes = append(nodes, renderBlock(c, source)...)
	}
	return nodes
}

// renderBlock converts a single block-level goldmark node into one or more
// TipTap nodes. Unknown / deferred node kinds are stripped and logged rather
// than causing the conversion to fail.
func renderBlock(n ast.Node, source []byte) []ttNode {
	switch n.Kind() {
	case ast.KindParagraph, ast.KindTextBlock:
		// A paragraph whose sole child is an image unwraps to a block-level
		// image (mirrors the wordpress converter and Tiptap's block image).
		if isSoleImageParagraph(n) {
			return []ttNode{imageNode(n.FirstChild().(*ast.Image), source)}
		}
		inline := renderInline(n, source, nil)
		if len(inline) == 0 {
			return []ttNode{{Type: "paragraph"}}
		}
		return []ttNode{{Type: "paragraph", Content: inline}}
	case ast.KindHeading:
		return []ttNode{{
			Type:    "heading",
			Attrs:   map[string]any{"level": n.(*ast.Heading).Level},
			Content: renderInline(n, source, nil),
		}}
	case ast.KindThematicBreak:
		return []ttNode{{Type: "horizontalRule"}}
	case ast.KindCodeBlock:
		return []ttNode{codeBlock("", n, source)}
	case ast.KindFencedCodeBlock:
		return []ttNode{codeBlock(string(n.(*ast.FencedCodeBlock).Language(source)), n, source)}
	case ast.KindBlockquote:
		content := renderBlocks(n, source)
		if len(content) == 0 {
			content = []ttNode{{Type: "paragraph"}}
		}
		return []ttNode{{Type: "blockquote", Content: content}}
	case ast.KindList:
		return []ttNode{renderList(n.(*ast.List), source)}
	case ast.KindHTMLBlock:
		text := sanitizeHTML(htmlBlockText(n.(*ast.HTMLBlock), source))
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []ttNode{{Type: "paragraph", Content: []ttNode{textNode(text, nil)}}}
	case ast.KindLinkReferenceDefinition:
		// `[label]: url "title"` definitions are metadata, not rendered content.
		return nil
	default:
		log.Printf("markdown: stripping unsupported %s block", n.Kind())
		return nil
	}
}

// renderList converts a goldmark List into a TipTap bulletList or orderedList.
// List.Start is intentionally ignored: Tiptap's ordered list defaults to 1.
func renderList(l *ast.List, source []byte) ttNode {
	listType := "bulletList"
	if l.IsOrdered() {
		listType = "orderedList"
	}
	items := make([]ttNode, 0)
	for c := l.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Kind() != ast.KindListItem {
			continue
		}
		items = append(items, renderListItem(c, source))
	}
	return ttNode{Type: listType, Content: items}
}

// renderListItem converts a goldmark ListItem into a TipTap listItem. Tiptap
// requires each listItem to contain at least one block, so an empty item gets
// a paragraph (tight lists carry a TextBlock child, which renderBlock maps to
// a paragraph; loose lists carry Paragraph children directly).
func renderListItem(n ast.Node, source []byte) ttNode {
	content := renderBlocks(n, source)
	if len(content) == 0 {
		content = []ttNode{{Type: "paragraph"}}
	}
	return ttNode{Type: "listItem", Content: content}
}

// isSoleImageParagraph reports whether the node has exactly one child and that
// child is an image — the standalone-image case that unwraps to a block image.
func isSoleImageParagraph(n ast.Node) bool {
	only := n.FirstChild()
	return only != nil && only.NextSibling() == nil && only.Kind() == ast.KindImage
}

// imageNode builds a TipTap image node. src comes from the Image destination,
// alt from the image's child text, and title only when present.
func imageNode(img *ast.Image, source []byte) ttNode {
	attrs := map[string]any{"src": string(img.Destination)}
	if alt := collectText(img, source); alt != "" {
		attrs["alt"] = alt
	}
	if len(img.Title) > 0 {
		attrs["title"] = string(img.Title)
	}
	return ttNode{Type: "image", Attrs: attrs}
}

// codeBlock builds a TipTap codeBlock node. language is the fenced info string
// (empty for indented code blocks, in which case no language attr is emitted).
func codeBlock(language string, n ast.Node, source []byte) ttNode {
	node := ttNode{Type: "codeBlock"}
	if language != "" {
		node.Attrs = map[string]any{"language": language}
	}
	node.Content = []ttNode{textNode(codeText(n, source), nil)}
	return node
}

// renderInline converts the inline children of a node into TipTap nodes (text
// nodes carrying marks, plus hardBreak / image nodes). The marks accumulator
// threads formatting (e.g. bold inside italic) through the recursion.
func renderInline(n ast.Node, source []byte, marks []ttMark) []ttNode {
	nodes := make([]ttNode, 0)
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		nodes = append(nodes, renderInlineNode(c, source, marks)...)
	}
	return nodes
}

// renderInlineNode converts a single inline goldmark node into TipTap nodes,
// inheriting the marks accumulated from its ancestors.
func renderInlineNode(n ast.Node, source []byte, marks []ttMark) []ttNode {
	switch n.Kind() {
	case ast.KindText:
		return renderText(n.(*ast.Text), source, marks)
	case ast.KindEmphasis:
		level := n.(*ast.Emphasis).Level
		markType := "italic"
		if level == 2 {
			markType = "bold"
		}
		return renderInline(n, source, appendMarks(marks, ttMark{Type: markType}))
	case ast.KindCodeSpan:
		// Code spans carry a code mark and never inherit other marks; their
		// raw text is concatenated verbatim from the child text nodes.
		code := collectText(n, source)
		if code == "" {
			return nil
		}
		return []ttNode{textNode(code, []ttMark{{Type: "code"}})}
	case ast.KindLink:
		link := n.(*ast.Link)
		return renderInline(n, source, appendMarks(marks, linkMark(string(link.Destination), string(link.Title))))
	case ast.KindAutoLink:
		al := n.(*ast.AutoLink)
		href := string(al.URL(source))
		label := string(al.Label(source))
		if label == "" {
			label = href
		}
		return []ttNode{textNode(label, appendMarks(marks, linkMark(href, "")))}
	case ast.KindImage:
		return []ttNode{imageNode(n.(*ast.Image), source)}
	case ast.KindRawHTML:
		text := sanitizeHTML(string(n.(*ast.RawHTML).Segments.Value(source)))
		if text == "" {
			return nil
		}
		return []ttNode{textNode(text, marks)}
	default:
		log.Printf("markdown: stripping unsupported %s inline node", n.Kind())
		return nil
	}
}

// renderText maps a goldmark Text node to TipTap text node(s), honoring hard
// and soft line breaks: a hard break emits a trailing hardBreak node; a soft
// break appends a space to the text run (CommonMark softbreak → space).
func renderText(t *ast.Text, source []byte, marks []ttMark) []ttNode {
	value := string(t.Value(source))
	switch {
	case t.HardLineBreak():
		nodes := make([]ttNode, 0, 2)
		if value != "" {
			nodes = append(nodes, textNode(value, marks))
		}
		return append(nodes, ttNode{Type: "hardBreak"})
	case t.SoftLineBreak():
		if value == "" {
			return []ttNode{textNode(" ", marks)}
		}
		return []ttNode{textNode(value+" ", marks)}
	default:
		if value == "" {
			return nil
		}
		return []ttNode{textNode(value, marks)}
	}
}

// linkMark builds a TipTap link mark from a destination and optional title.
func linkMark(href, title string) ttMark {
	attrs := map[string]any{"href": href}
	if title != "" {
		attrs["title"] = title
	}
	return ttMark{Type: "link", Attrs: attrs}
}

// appendMarks returns a new slice with m appended to marks, leaving the caller's
// slice untouched so ancestor formatting is not mutated.
func appendMarks(marks []ttMark, m ttMark) []ttMark {
	out := make([]ttMark, 0, len(marks)+1)
	out = append(out, marks...)
	out = append(out, m)
	return out
}

// textNode builds a TipTap text node, attaching marks only when non-empty.
func textNode(text string, marks []ttMark) ttNode {
	node := ttNode{Type: "text", Text: text}
	if len(marks) > 0 {
		node.Marks = marks
	}
	return node
}

// collectText concatenates the value of every descendant Text node, used to
// extract code-span content, image alt text, and link labels.
func collectText(n ast.Node, source []byte) string {
	var b strings.Builder
	var walk func(ast.Node)
	walk = func(nd ast.Node) {
		for c := nd.FirstChild(); c != nil; c = c.NextSibling() {
			if c.Kind() == ast.KindText {
				b.Write(c.(*ast.Text).Value(source))
			}
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// codeText extracts a code block's source text, trimming the single trailing
// newline that goldmark includes per line segment.
func codeText(n ast.Node, source []byte) string {
	return strings.TrimSuffix(string(n.Lines().Value(source)), "\n")
}

// sanitizeHTML reduces raw HTML to safe plain text via the project's bluemonday
// sanitizer (strict policy). Raw HTML markup is never emitted into the document.
func sanitizeHTML(raw string) string {
	return sanitizer.SanitizePlainText(raw)
}

// htmlBlockText reconstructs an HTMLBlock's raw source (its lines plus the
// optional closure line) without using the deprecated Node.Text method.
func htmlBlockText(hb *ast.HTMLBlock, source []byte) string {
	raw := string(hb.Lines().Value(source))
	if hb.HasClosure() {
		raw += string(hb.ClosureLine.Value(source))
	}
	return raw
}

// newDoc wraps rendered block content in a TipTap doc root, ensuring the doc is
// never empty (a minimal paragraph satisfies the content validator).
func newDoc(content []ttNode) ttNode {
	if len(content) == 0 {
		content = []ttNode{{Type: "paragraph"}}
	}
	return ttNode{Type: "doc", Content: content}
}
