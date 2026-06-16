package sanitize

import (
	"errors"
	"fmt"
	"net/url"
)

const (
	maxNodeDepth = 50
	maxNodeCount = 10000

	errNodeDepthExceeded = "document exceeds maximum depth of %d"
	errNodeCountExceeded = "document exceeds maximum node count of %d"
	errUnknownNodeType   = "unknown node type: %q"
	errUnknownMarkType   = "unknown mark type: %q"
	errUnexpectedAttr    = "unexpected attribute %q on node type %q"
	errInvalidHrefScheme = "invalid href scheme: %q"
	errTextNodeNonString = "text node has non-string text field"
	errContentNotArray   = "content field is not an array"
	errNodeMissingType   = "node missing type field"
	errAttrsNotMap       = "attrs field is not an object"
	errMarksNotArray     = "marks field is not an array"
)

var (
	allowedNodeTypes = map[string]bool{
		"doc":            true,
		"paragraph":      true,
		"text":           true,
		"heading":        true,
		"bulletList":     true,
		"orderedList":    true,
		"listItem":       true,
		"blockquote":     true,
		"codeBlock":      true,
		"hardBreak":      true,
		"horizontalRule": true,
		"image":          true,
		"table":          true,
		"tableRow":       true,
		"tableCell":      true,
		"tableHeader":    true,
		"inlineMath":     true,
		"blockMath":      true,
		"youtube":        true,
		"emoji":          true,
	}

	allowedMarkTypes = map[string]bool{
		"bold":      true,
		"italic":    true,
		"underline": true,
		"strike":    true,
		"code":      true,
		"link":      true,
	}

	allowedAttrsPerNode = map[string]map[string]bool{
		"heading": {
			"level": true,
		},
		"codeBlock": {
			"language": true,
			"class":    true,
		},
		"image": {
			"src":    true,
			"alt":    true,
			"title":  true,
			"class":  true,
			"height": true,
			"width":  true,
		},
		"doc": {
			"content": true,
		},
		"youtube": {
			"src":    true,
			"width":  true,
			"height": true,
		},
		"emoji": {
			"name": true,
		},
		"inlineMath": {
			"latex": true,
		},
		"blockMath": {
			"latex": true,
		},
		"tableCell": {
			"colspan":  true,
			"rowspan":  true,
			"colwidth": true,
			"align":    true,
		},
		"tableHeader": {
			"colspan":  true,
			"rowspan":  true,
			"colwidth": true,
			"align":    true,
		},
	}

	allowedAttrsPerMark = map[string]map[string]bool{
		"link": {
			"href":       true,
			"target":     true,
			"rel":        true,
			"class":      true,
			"title":      true,
			"data-type":  true,
			"data-id":    true,
			"data-label": true,
		},
		"code": {
			"class": true,
		},
	}
)

func ValidateTipTapDocument(doc map[string]any) error {
	nodeCount := 0
	return validateNode(doc, 0, &nodeCount)
}

func validateNode(node map[string]any, depth int, count *int) error {
	*count++
	if *count > maxNodeCount {
		return fmt.Errorf(errNodeCountExceeded, maxNodeCount)
	}

	if depth > maxNodeDepth {
		return fmt.Errorf(errNodeDepthExceeded, maxNodeDepth)
	}

	nodeType, ok := node["type"]
	if !ok {
		return errors.New(errNodeMissingType)
	}

	typeStr, ok := nodeType.(string)
	if !ok {
		return fmt.Errorf(errUnknownNodeType, nodeType)
	}

	if typeStr == "doc" {
		content, ok := node["content"]
		if !ok {
			return nil
		}
		contentArr, ok := content.([]any)
		if !ok {
			return errors.New(errContentNotArray)
		}
		for _, child := range contentArr {
			childMap, ok := child.(map[string]any)
			if !ok {
				return errors.New(errContentNotArray)
			}
			if err := validateNode(childMap, depth+1, count); err != nil {
				return err
			}
		}
		return nil
	}

	if !allowedNodeTypes[typeStr] {
		return fmt.Errorf(errUnknownNodeType, typeStr)
	}

	if attrs, ok := node["attrs"]; ok {
		attrsMap, ok := attrs.(map[string]any)
		if !ok {
			return errors.New(errAttrsNotMap)
		}
		allowedAttrs := allowedAttrsPerNode[typeStr]
		for key, val := range attrsMap {
			if allowedAttrs == nil || !allowedAttrs[key] {
				return fmt.Errorf(errUnexpectedAttr, key, typeStr)
			}
			if key == "src" {
				src, ok := val.(string)
				if ok && !isSafeSrc(src) {
					return fmt.Errorf(errInvalidHrefScheme, src)
				}
			}
		}
	}

	if marks, ok := node["marks"]; ok {
		marksArr, ok := marks.([]any)
		if !ok {
			return errors.New(errMarksNotArray)
		}
		for _, mark := range marksArr {
			markMap, ok := mark.(map[string]any)
			if !ok {
				return errors.New(errMarksNotArray)
			}
			if err := validateMark(markMap); err != nil {
				return err
			}
		}
	}

	if typeStr == "text" {
		textVal, ok := node["text"]
		if ok {
			if _, ok := textVal.(string); !ok {
				return errors.New(errTextNodeNonString)
			}
		}
		return nil
	}

	if content, ok := node["content"]; ok {
		contentArr, ok := content.([]any)
		if !ok {
			return errors.New(errContentNotArray)
		}
		for _, child := range contentArr {
			childMap, ok := child.(map[string]any)
			if !ok {
				return errors.New(errContentNotArray)
			}
			if err := validateNode(childMap, depth+1, count); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateMark(mark map[string]any) error {
	markType, ok := mark["type"]
	if !ok {
		return errors.New(errNodeMissingType)
	}

	typeStr, ok := markType.(string)
	if !ok {
		return fmt.Errorf(errUnknownMarkType, markType)
	}

	if !allowedMarkTypes[typeStr] {
		return fmt.Errorf(errUnknownMarkType, typeStr)
	}

	if attrs, ok := mark["attrs"]; ok {
		attrsMap, ok := attrs.(map[string]any)
		if !ok {
			return errors.New(errAttrsNotMap)
		}
		allowedAttrs := allowedAttrsPerMark[typeStr]
		for key, val := range attrsMap {
			if allowedAttrs == nil || !allowedAttrs[key] {
				return fmt.Errorf(errUnexpectedAttr, key, typeStr)
			}
			if key == "href" {
				href, ok := val.(string)
				if ok && !isSafeHref(href) {
					return fmt.Errorf(errInvalidHrefScheme, href)
				}
			}
		}
	}

	return nil
}

func isSafeHref(href string) bool {
	u, err := url.Parse(href)
	if err != nil {
		return false
	}
	return u.Scheme == "" || u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "mailto"
}

func isSafeSrc(src string) bool {
	u, err := url.Parse(src)
	if err != nil {
		return false
	}
	return u.Scheme == "" || u.Scheme == "http" || u.Scheme == "https"
}
