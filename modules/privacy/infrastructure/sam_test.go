package infrastructure_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	privacyinfra "github.com/aristorinjuang/lesstruct/modules/privacy/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seenConcepts records request concepts across concurrent test-server hits.
type seenConcepts struct {
	mu     sync.Mutex
	values []string
}

func (s *seenConcepts) add(concept string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values = append(s.values, concept)
}

func (s *seenConcepts) all() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.values...)
}

// responsesRequest mirrors the client's wire shape so the fixture can
// assert on it without importing unexported types.
type fixtureRequest struct {
	Model string `json:"model"`
	Input []struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			ImageURL string `json:"image_url"`
		} `json:"content"`
	} `json:"input"`
	Stream bool `json:"stream"`
}

// successServer serves one valid box token plus one degenerate box per
// call, recording each concept phrase it receives.
func successServer(t *testing.T, seen *seenConcepts) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/responses", r.URL.Path)
		assert.NotEmpty(t, r.Header.Get("Authorization"))
		var req fixtureRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Len(t, req.Input, 1)
		var concept, imageURL string
		for _, part := range req.Input[0].Content {
			switch part.Type {
			case "input_text":
				concept = part.Text
			case "input_image":
				imageURL = part.ImageURL
			}
		}
		require.NotEmpty(t, concept)
		assert.Contains(t, imageURL, "data:image/")
		assert.Contains(t, imageURL, ";base64,")
		seen.add(concept)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"<0f>0<|box;x1=10;y1=20;x2=40;y2=60;w=64;h=64|>,1<|box;x1=5;y1=5;x2=5;y2=9;w=64;h=64|>"}]}]}`))
	}))
}

func TestSegmentSuccessCollectsBoxes(t *testing.T) {
	seen := &seenConcepts{}
	srv := successServer(t, seen)
	defer srv.Close()

	client := privacyinfra.NewSAMService(
		"key",
		"sam-3.1",
		privacyinfra.WithBaseURL(srv.URL),
	)
	got, err := client.Segment(context.Background(), []byte("img"), []string{"face", "nudity"})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.ElementsMatch(t, []string{"face", "nudity"}, seen.all())
	assert.Equal(t, "face", got[0].Concept)
	assert.Equal(t, 10, got[0].Box.Min.X)
	assert.Equal(t, 20, got[0].Box.Min.Y)
	assert.Equal(t, 40, got[0].Box.Max.X)
	assert.Equal(t, 60, got[0].Box.Max.Y)
}

func TestSegmentDefaults(t *testing.T) {
	seen := &seenConcepts{}
	srv := successServer(t, seen)
	defer srv.Close()

	client := privacyinfra.NewSAMService("key", "", privacyinfra.WithBaseURL(srv.URL))
	got, err := client.Segment(context.Background(), []byte("img"), nil)
	require.NoError(t, err)
	require.NotEmpty(t, got)
	assert.Len(t, seen.all(), 3)
}

func TestSegmentEmptyImage(t *testing.T) {
	client := privacyinfra.NewSAMService("key", "sam-3.1")
	_, err := client.Segment(context.Background(), nil, []string{"face"})
	require.Error(t, err)
}

func TestSegmentSkipsBlanks(t *testing.T) {
	seen := &seenConcepts{}
	srv := successServer(t, seen)
	defer srv.Close()

	client := privacyinfra.NewSAMService("key", "sam-3.1", privacyinfra.WithBaseURL(srv.URL))
	got, err := client.Segment(context.Background(), []byte("img"), []string{"", "  ", "face"})
	require.NoError(t, err)
	// The fixture serves two box tokens: one valid, one degenerate
	// (x1 == x2) — only the valid box survives.
	require.Len(t, got, 1)
	assert.Equal(t, []string{"face"}, seen.all())
}

func TestSegmentEmptyOutputTextIsZeroMatches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":""}]}]}`))
	}))
	defer srv.Close()

	client := privacyinfra.NewSAMService("key", "sam-3.1", privacyinfra.WithBaseURL(srv.URL))
	got, err := client.Segment(context.Background(), []byte("img"), []string{"face"})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSegmentMissingOutputTextFailsClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[]}`))
	}))
	defer srv.Close()

	client := privacyinfra.NewSAMService("key", "sam-3.1", privacyinfra.WithBaseURL(srv.URL))
	_, err := client.Segment(context.Background(), []byte("img"), []string{"face"})
	require.Error(t, err)
}

func TestSegmentServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad"))
	}))
	defer srv.Close()

	client := privacyinfra.NewSAMService("key", "sam-3.1", privacyinfra.WithBaseURL(srv.URL))
	_, err := client.Segment(context.Background(), []byte("img"), []string{"face"})
	require.Error(t, err)
}

func TestSegmentInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	client := privacyinfra.NewSAMService("key", "sam-3.1", privacyinfra.WithBaseURL(srv.URL))
	_, err := client.Segment(context.Background(), []byte("img"), []string{"face"})
	require.Error(t, err)
}

func TestSegmentSniffsImageMimeType(t *testing.T) {
	tests := []struct {
		name string
		head []byte
	}{
		{name: "png", head: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}},
		{name: "jpeg", head: []byte{0xFF, 0xD8, 0xFF, 0xE0}},
		{name: "gif", head: []byte("GIF89a")},
		{name: "webp", head: []byte("RIFFxxxxWEBP")},
		{name: "unknown falls back", head: []byte("img")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seenURL string
			var mu sync.Mutex
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req fixtureRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				mu.Lock()
				for _, part := range req.Input[0].Content {
					if part.Type == "input_image" {
						seenURL = part.ImageURL
					}
				}
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":""}]}]}`))
			}))
			defer srv.Close()

			client := privacyinfra.NewSAMService("key", "sam-3.1", privacyinfra.WithBaseURL(srv.URL))
			_, err := client.Segment(context.Background(), tt.head, []string{"face"})
			require.NoError(t, err)
			mu.Lock()
			defer mu.Unlock()
			assert.Contains(t, seenURL, "data:image/")
			assert.Contains(t, seenURL, ";base64,")
		})
	}
}

func TestWithBaseURLEmptyAndNilClient(t *testing.T) {
	client := privacyinfra.NewSAMService("key", "sam-3.1", privacyinfra.WithBaseURL(""), privacyinfra.WithHTTPClient(nil))
	require.NotNil(t, client)
}

func TestSegmentContextCanceled(t *testing.T) {
	seen := &seenConcepts{}
	srv := successServer(t, seen)
	defer srv.Close()

	client := privacyinfra.NewSAMService("key", "sam-3.1", privacyinfra.WithBaseURL(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Segment(ctx, []byte("img"), []string{"face", "nudity"})
	require.Error(t, err)
}
