package markdown

import (
	"encoding/json"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
)

// Convert parses Markdown src into a Tiptap JSON document string.
//
// It uses core CommonMark only (no GFM extensions), so tables, strikethrough,
// and task-lists never enter the AST — they are deferred per the architecture.
// Unsupported node kinds are stripped and logged; the conversion itself never
// fails on content: empty or whitespace-only input yields a minimal valid doc.
// The only error returned is an internal JSON marshal failure, which is
// effectively unreachable for the node structures emitted here.
func Convert(src string) (string, error) {
	source := []byte(src)
	root := goldmark.New().Parser().Parse(text.NewReader(source))
	doc := newDoc(renderBlocks(root, source))

	data, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tiptap document: %w", err)
	}
	return string(data), nil
}
