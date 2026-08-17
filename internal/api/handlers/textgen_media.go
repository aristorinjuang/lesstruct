package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
)

// stopWords is a small set of common English words to exclude from keyword matching.
var stopWords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "and": {}, "or": {}, "but": {}, "in": {}, "on": {},
	"at": {}, "to": {}, "for": {}, "of": {}, "with": {}, "by": {}, "from": {}, "as": {},
	"is": {}, "was": {}, "are": {}, "were": {}, "be": {}, "been": {}, "being": {},
	"have": {}, "has": {}, "had": {}, "do": {}, "does": {}, "did": {}, "will": {},
	"would": {}, "could": {}, "should": {}, "may": {}, "might": {}, "shall": {},
	"can": {}, "need": {}, "must": {}, "not": {}, "no": {}, "nor": {},
	"it": {}, "its": {}, "this": {}, "that": {}, "these": {}, "those": {},
	"i": {}, "you": {}, "he": {}, "she": {}, "we": {}, "they": {}, "me": {}, "him": {},
	"her": {}, "us": {}, "them": {}, "my": {}, "your": {}, "his": {}, "our": {}, "their": {},
	"what": {}, "which": {}, "who": {}, "whom": {}, "where": {}, "when": {}, "why": {}, "how": {},
	"all": {}, "each": {}, "every": {}, "both": {}, "few": {}, "more": {}, "most": {}, "other": {},
	"some": {}, "such": {}, "than": {}, "too": {}, "very": {}, "just": {}, "about": {},
	"above": {}, "after": {}, "again": {}, "also": {}, "here": {}, "there": {}, "then": {},
	"once": {}, "only": {}, "own": {}, "same": {}, "so": {}, "if": {}, "into": {},
	"out": {}, "up": {}, "down": {}, "off": {}, "over": {}, "under": {}, "between": {},
	// Domain-specific common words to reduce noise
	"section": {}, "block": {}, "page": {}, "content": {}, "text": {}, "image": {},
	"button": {}, "link": {}, "list": {}, "item": {}, "card": {}, "column": {},
	"row": {}, "grid": {}, "container": {}, "wrapper": {}, "element": {},
}

// tokenize splits text into lowercase word tokens, filtering stop words.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				word := current.String()
				if _, isStop := stopWords[word]; !isStop && len(word) > 1 {
					tokens = append(tokens, word)
				}
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		word := current.String()
		if _, isStop := stopWords[word]; !isStop && len(word) > 1 {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

// scoredMedia pairs a media item with its relevance score.
type scoredMedia struct {
	media *mediadomain.Media
	score int
}

// buildMediaContext fetches the user's recent images and selects the top 20
// most relevant to the given prompt based on keyword overlap with alt text.
// Returns an empty string if no images are available or the user has no media.
func buildMediaContext(
	ctx context.Context,
	mediaService MediaLister,
	userIDStr string,
	prompt string,
) string {
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		return ""
	}

	mediaList, err := mediaService.ListByCursor(ctx, userID, 100, 0)
	if err != nil || len(mediaList) == 0 {
		return ""
	}

	// Filter to images only
	images := make([]*mediadomain.Media, 0, len(mediaList))
	for _, m := range mediaList {
		if isImageMime(m.MimeType) {
			images = append(images, m)
		}
	}
	if len(images) == 0 {
		return ""
	}

	promptTokens := tokenize(prompt)

	// Score each image by keyword overlap with alt text (or filename fallback)
	scored := make([]scoredMedia, 0, len(images))
	for _, img := range images {
		alt := strings.TrimSpace(img.AltText)
		if alt == "" {
			// Fall back to original filename without extension
			alt = img.OriginalFilename
			if dotIdx := strings.LastIndex(alt, "."); dotIdx > 0 {
				alt = alt[:dotIdx]
			}
		}

		imgTokens := tokenize(alt)
		score := countOverlap(promptTokens, imgTokens)
		scored = append(scored, scoredMedia{media: img, score: score})
	}

	// Sort by score descending, then by ID descending (recency) for ties
	// Since ListByCursor returns id DESC, earlier items are more recent
	// We use a stable sort to preserve recency for equal scores
	for i := 1; i < len(scored); i++ {
		for j := i; j > 0; j-- {
			if scored[j].score > scored[j-1].score ||
				(scored[j].score == scored[j-1].score && scored[j].media.ID > scored[j-1].media.ID) {
				scored[j], scored[j-1] = scored[j-1], scored[j]
			} else {
				break
			}
		}
	}

	// If all scores are zero, take the 20 most recent (already in ID DESC order from ListByCursor)
	hasNonZero := false
	for _, s := range scored {
		if s.score > 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		// Re-sort by ID descending (most recent first)
		for i := 1; i < len(scored); i++ {
			for j := i; j > 0; j-- {
				if scored[j].media.ID > scored[j-1].media.ID {
					scored[j], scored[j-1] = scored[j-1], scored[j]
				} else {
					break
				}
			}
		}
	}

	// Take top 20
	limit := min(len(scored), 20)
	selected := scored[:limit]

	// Format the context string
	var sb strings.Builder
	sb.WriteString("Available images (use these URLs only when relevant; never invent image URLs):\n")
	for _, s := range selected {
		alt := strings.TrimSpace(s.media.AltText)
		if alt == "" {
			alt = s.media.OriginalFilename
		}
		fmt.Fprintf(&sb, "- alt: %q | url: %s\n", alt, s.media.URL)
	}

	return sb.String()
}

// countOverlap counts the number of unique tokens that appear in both slices.
func countOverlap(a, b []string) int {
	set := make(map[string]struct{}, len(a))
	for _, t := range a {
		set[t] = struct{}{}
	}
	count := 0
	seen := make(map[string]struct{})
	for _, t := range b {
		if _, exists := set[t]; exists {
			if _, already := seen[t]; !already {
				count++
				seen[t] = struct{}{}
			}
		}
	}
	return count
}

// isImageMime checks if the mime type represents an image.
func isImageMime(mimeType mediadomain.MimeType) bool {
	switch mimeType {
	case mediadomain.MimeTypeJPEG, mediadomain.MimeTypePNG,
		mediadomain.MimeTypeGIF, mediadomain.MimeTypeWebP:
		return true
	}
	return false
}
