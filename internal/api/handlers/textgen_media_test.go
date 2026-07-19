package handlers_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMediaLister is a simple mock implementing handlers.MediaLister for testing.
type mockMediaLister struct {
	media []*mediadomain.Media
	err   error
}

func (m *mockMediaLister) ListByCursor(_ context.Context, _ int, _ int, _ int) ([]*mediadomain.Media, error) {
	return m.media, m.err
}

func TestBuildMediaContext_NoMedia(t *testing.T) {
	mock := &mockMediaLister{media: nil, err: nil}
	result := handlers.BuildMediaContextForTest(context.Background(), mock, "1", "a hero section")
	require.Empty(t, result)
}

func TestBuildMediaContext_NoImages(t *testing.T) {
	mock := &mockMediaLister{media: []*mediadomain.Media{
		{ID: 1, MimeType: "video/mp4"},
	}}
	result := handlers.BuildMediaContextForTest(context.Background(), mock, "1", "a hero section")
	require.Empty(t, result)
}

func TestBuildMediaContext_KeywordMatch(t *testing.T) {
	mock := &mockMediaLister{media: []*mediadomain.Media{
		{ID: 3, MimeType: mediadomain.MimeTypeWebP, AltText: "sunset over beach", URL: "http://localhost:8080/uploads/media/aaa111.webp", OriginalFilename: "sunset.webp"},
		{ID: 2, MimeType: mediadomain.MimeTypeJPEG, AltText: "team office meeting", URL: "http://localhost:8080/uploads/media/bbb222.webp", OriginalFilename: "office.jpg"},
		{ID: 1, MimeType: mediadomain.MimeTypePNG, AltText: "hero banner design", URL: "http://localhost:8080/uploads/media/ccc333.webp", OriginalFilename: "hero.png"},
	}}

	result := handlers.BuildMediaContextForTest(context.Background(), mock, "1", "a hero section with a banner")
	assert.Contains(t, result, "Available images")
	assert.Contains(t, result, "hero banner design")
	// "hero banner design" has 2 overlap → should be first
	lines := splitLines(result)
	assert.True(t, len(lines) >= 2)
	assert.Contains(t, lines[1], "hero banner design")
}

func TestBuildMediaContext_AllZeroScores_usesRecentOrder(t *testing.T) {
	mock := &mockMediaLister{media: []*mediadomain.Media{
		{ID: 3, MimeType: mediadomain.MimeTypeWebP, AltText: "", URL: "http://host/3.webp", OriginalFilename: "newest.webp"},
		{ID: 2, MimeType: mediadomain.MimeTypeJPEG, AltText: "", URL: "http://host/2.webp", OriginalFilename: "middle.jpg"},
		{ID: 1, MimeType: mediadomain.MimeTypePNG, AltText: "", URL: "http://host/1.webp", OriginalFilename: "oldest.png"},
	}}

	result := handlers.BuildMediaContextForTest(context.Background(), mock, "42", "xyz random")
	assert.Contains(t, result, "Available images")
	// Most recent first (highest ID): ID 3, then ID 2, then ID 1
	lines := splitLines(result)
	require.True(t, len(lines) >= 4)
	assert.Contains(t, lines[1], "newest.webp")
	assert.Contains(t, lines[2], "middle.jpg")
	assert.Contains(t, lines[3], "oldest.png")
}

func TestBuildMediaContext_InvalidUserID(t *testing.T) {
	mock := &mockMediaLister{}
	result := handlers.BuildMediaContextForTest(context.Background(), mock, "not-a-number", "hero")
	require.Empty(t, result)
}

func TestBuildMediaContext_AltTextFallback(t *testing.T) {
	mock := &mockMediaLister{media: []*mediadomain.Media{
		{ID: 1, MimeType: mediadomain.MimeTypeWebP, AltText: "", URL: "http://host/1.webp", OriginalFilename: "my_photo.jpg"},
	}}

	result := handlers.BuildMediaContextForTest(context.Background(), mock, "1", "my photo")
	assert.Contains(t, result, "my_photo.jpg")
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "basic tokenization",
			input:    "Hello World",
			expected: []string{"hello", "world"},
		},
		{
			name:     "filters stop words",
			input:    "the quick brown fox",
			expected: []string{"quick", "brown", "fox"},
		},
		{
			name:     "single char words filtered",
			input:    "a b c",
			expected: nil,
		},
		{
			name:     "keeps meaningful domain words",
			input:    "hero section gradient background",
			expected: []string{"hero", "gradient", "background"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handlers.TokenizeForTest(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountOverlap(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected int
	}{
		{
			name:     "no overlap",
			a:        []string{"hello", "world"},
			b:        []string{"foo", "bar"},
			expected: 0,
		},
		{
			name:     "partial overlap",
			a:        []string{"hello", "world", "foo"},
			b:        []string{"world", "bar", "foo"},
			expected: 2,
		},
		{
			name:     "deduplication",
			a:        []string{"hello"},
			b:        []string{"hello", "hello"},
			expected: 1,
		},
		{
			name:     "empty slices",
			a:        []string{},
			b:        []string{"hello"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handlers.CountOverlapForTest(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range splitByNewline(s) {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitByNewline(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}
