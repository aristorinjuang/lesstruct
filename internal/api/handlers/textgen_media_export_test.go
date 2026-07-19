package handlers

import "context"

// BuildMediaContextForTest exports buildMediaContext for testing.
func BuildMediaContextForTest(ctx context.Context, mediaService MediaLister, userIDStr, prompt string) string {
	return buildMediaContext(ctx, mediaService, userIDStr, prompt)
}

// TokenizeForTest exports tokenize for testing.
func TokenizeForTest(text string) []string {
	return tokenize(text)
}

// CountOverlapForTest exports countOverlap for testing.
func CountOverlapForTest(a, b []string) int {
	return countOverlap(a, b)
}
