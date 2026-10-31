package markdown

import (
	"encoding/json"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// Convert parses Markdown src into a Tiptap JSON document string.
//
// It uses CommonMark plus the GFM table and strikethrough extensions, so
// fenced tables and `~~strike~~` enter the AST and map onto the tiptap
// nodes the renderer and the admin editor already understand. Task lists
// stay deferred: no other pipeline leg (validator, renderer, editor)
// speaks `taskList`/`taskItem`, so converting them here alone would emit
// documents nothing can render. Unsupported node kinds are stripped and
// logged; the conversion itself never fails on content: empty or
// whitespace-only input yields a minimal valid doc. The only error returned
// is an internal JSON marshal failure, which is effectively unreachable
// for the node structures emitted here.
func Convert(src string) (string, error) {
	source := []byte(src)
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table, extension.Strikethrough),
	)
	root := md.Parser().Parse(text.NewReader(source))
	doc := newDoc(renderBlocks(root, source))

	data, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tiptap document: %w", err)
	}
	return string(data), nil
}
