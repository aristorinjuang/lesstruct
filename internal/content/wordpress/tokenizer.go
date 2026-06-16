package wordpress

import (
	"encoding/json"
	"regexp"
	"strings"
)

// wpBlock is a parsed WordPress block from the block-editor HTML. Container
// blocks carry their children; leaf blocks carry the raw HTML between their
// opening and closing comments.
type wpBlock struct {
	name     string
	attrs    map[string]any
	raw      string // accumulated non-comment text belonging to this block
	children []*wpBlock
}

// commentRe matches a single HTML comment, non-greedy. WordPress block
// delimiters and special comments (e.g. <!--more-->) never contain nested
// comments, so stopping at the first "-->" is correct.
var commentRe = regexp.MustCompile(`(?s)<!--(.*?)-->`)

// tokenizeBlocks parses WordPress block-editor HTML into a tree of blocks. The
// returned root block holds top-level blocks as children and any loose
// non-block text in its raw field.
func tokenizeBlocks(content string) *wpBlock {
	root := &wpBlock{name: "root"}
	stack := []*wpBlock{root}
	prevEnd := 0

	flushRaw := func(text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		top := stack[len(stack)-1]
		top.raw += text
	}

	matches := commentRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range matches {
		flushRaw(content[prevEnd:m[0]])
		inner := strings.TrimSpace(content[m[2]:m[3]])
		prevEnd = m[1]

		switch {
		case strings.HasPrefix(inner, "/wp:"):
			name := strings.TrimPrefix(inner, "/wp:")
			popBlock(&stack, name)
		case strings.HasPrefix(inner, "wp:"):
			body := strings.TrimSpace(strings.TrimPrefix(inner, "wp:"))
			name, attrs, selfClose := parseBlockHeader(body)
			block := &wpBlock{name: name, attrs: attrs}
			top := stack[len(stack)-1]
			top.children = append(top.children, block)
			if !selfClose {
				stack = append(stack, block)
			}
		default:
			// Non-WP comment (e.g. <!--more-->, <!--nextpage-->) — keep as text
			// so its presence inside a block is preserved in raw.
			flushRaw(content[m[0]:m[1]])
		}
	}
	flushRaw(content[prevEnd:])

	return root
}

// parseBlockHeader parses the body of a "wp:NAME {json} [/]" block header into
// its name, optional JSON attributes, and self-closing flag.
func parseBlockHeader(body string) (string, map[string]any, bool) {
	selfClose := false
	if strings.HasSuffix(body, "/") {
		selfClose = true
		body = strings.TrimSpace(strings.TrimSuffix(body, "/"))
	}

	name := body
	attrsStr := ""
	if idx := strings.IndexAny(body, " \t\n"); idx >= 0 {
		name = body[:idx]
		attrsStr = strings.TrimSpace(body[idx:])
	}

	var attrs map[string]any
	if strings.HasPrefix(attrsStr, "{") {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(attrsStr), &parsed); err == nil {
			attrs = parsed
		}
	}
	return name, attrs, selfClose
}

// popBlock pops the block stack until a block matching name is removed. This is
// tolerant of minor nesting mismatches so a malformed export never leaves blocks
// stranded on the stack.
func popBlock(stack *[]*wpBlock, name string) {
	for i := len(*stack) - 1; i > 0; i-- {
		if (*stack)[i].name == name {
			*stack = (*stack)[:i]
			return
		}
	}
}
