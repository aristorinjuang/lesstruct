package seo

import (
	"encoding/json"
	"fmt"
	"strings"
)

// extractTextRecursive recursively extracts text from TipTap nodes
func extractTextRecursive(nodes []TipTapNode, builder *strings.Builder) {
	for _, node := range nodes {
		if node.Text != "" {
			builder.WriteString(node.Text)
		}

		if node.Type == "paragraph" {
			if len(node.Content) > 0 {
				extractTextRecursive(node.Content, builder)
			}
			builder.WriteString(" ")
		} else if node.Type == "listItem" {
			if len(node.Content) > 0 {
				extractTextRecursive(node.Content, builder)
			}
		} else if node.Type == "heading" {
			if len(node.Content) > 0 {
				extractTextRecursive(node.Content, builder)
			}
			builder.WriteString(" ")
		} else if node.Type == "bulletList" || node.Type == "orderedList" {
			if len(node.Content) > 0 {
				for i, item := range node.Content {
					extractTextRecursive([]TipTapNode{item}, builder)
					if i < len(node.Content)-1 {
						builder.WriteString(" ")
					}
				}
			}
		} else if node.Type == "codeBlock" {
			if len(node.Content) > 0 {
				extractTextRecursive(node.Content, builder)
			}
			builder.WriteString(" ")
		} else if len(node.Content) > 0 {
			extractTextRecursive(node.Content, builder)
		}
	}
}

// findImageRecursive recursively searches for image nodes
func findImageRecursive(nodes []TipTapNode) string {
	for _, node := range nodes {
		if node.Type == "image" {
			if attrs, ok := node.Attrs["src"].(string); ok {
				return attrs
			}
		}

		if len(node.Content) > 0 {
			if url := findImageRecursive(node.Content); url != "" {
				return url
			}
		}
	}

	return ""
}

// TipTapDocument represents a TipTap JSON document
type TipTapDocument struct {
	Type    string       `json:"type"`
	Content []TipTapNode `json:"content"`
}

// TipTapNode represents a node in TipTap JSON
type TipTapNode struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []TipTapNode   `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
}

// ExtractPlainText extracts plain text from TipTap JSON content
func ExtractPlainText(tiptapJSON string) string {
	if tiptapJSON == "" {
		return ""
	}

	var doc TipTapDocument
	if err := json.Unmarshal([]byte(tiptapJSON), &doc); err != nil {
		return ""
	}

	var builder strings.Builder
	extractTextRecursive(doc.Content, &builder)
	return strings.TrimSpace(builder.String())
}

// ExtractImageURL finds the first image URL in TipTap JSON content
func ExtractImageURL(tiptapJSON string) string {
	if tiptapJSON == "" {
		return ""
	}

	var doc TipTapDocument
	if err := json.Unmarshal([]byte(tiptapJSON), &doc); err != nil {
		return ""
	}

	return findImageRecursive(doc.Content)
}

// TruncateText truncates text to a maximum length, adding ellipsis if needed
func TruncateText(text string, maxLength int) string {
	runes := []rune(text)
	if len(runes) <= maxLength {
		return text
	}

	if maxLength <= 3 {
		return string(runes[:maxLength])
	}

	return string(runes[:maxLength-3]) + "..."
}

// BuildURL constructs a full URL from path
func BuildURL(baseURL, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}

	if path == "" {
		return strings.TrimSuffix(baseURL, "/")
	}

	return fmt.Sprintf("%s/%s", strings.TrimSuffix(baseURL, "/"), strings.TrimPrefix(path, "/"))
}
