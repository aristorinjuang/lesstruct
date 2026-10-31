package tiptap

type node struct {
	Type    string         `json:"type"`
	Content []node         `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []mark         `json:"marks,omitempty"`
	Attrs   map[string]any `json:"attrs,omitempty"`
}

type mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

type document struct {
	Type    string `json:"type"`
	Content []node `json:"content"`
}

// ImageVariant is one sized rendition of an image for rendering.
type ImageVariant struct {
	URL   string
	Width int
	// Height is the rendition height in pixels. Zero when unknown; the
	// renderer omits width/height attributes instead of guessing.
	Height int
}
