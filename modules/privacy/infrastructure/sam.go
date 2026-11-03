// Package infrastructure adapts the privacy Segmenter port onto Meta's
// Model API (SAM 3.1): one Responses-API call per concept, boxes extracted.
package infrastructure

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	privacydomain "github.com/aristorinjuang/lesstruct/modules/privacy/domain"
)

const (
	// defaultBaseURL is the Model API root. Tests override it with an
	// httptest server via WithBaseURL.
	defaultBaseURL = "https://api.meta.ai"
	// defaultModel is the SAM checkpoint used when AI_SAM_MODEL is empty.
	defaultModel = "sam-3.1"
	// defaultTimeout bounds one concept call.
	defaultTimeout = 30 * time.Second
	// responsesPath is the Model API Responses route SAM segments over.
	responsesPath = "/v1/responses"
)

// boxPattern matches one segmentation box token in the response text:
// <|box;x1=353;y1=382;x2=453;y2=427;w=537;h=561|>. Coordinates arrive in
// source pixels; masks are ignored because blurring needs only boxes.
var boxPattern = regexp.MustCompile(`<\|box;x1=(\d+);y1=(\d+);x2=(\d+);y2=(\d+);`)

// responsesRequest is the Model API call for one concept: a single user
// message pairing the concept noun phrase with the image as a data URL.
type responsesRequest struct {
	Model  string             `json:"model"`
	Input  []responsesMessage `json:"input"`
	Stream bool               `json:"stream"`
}

// responsesMessage is one Responses-API message.
type responsesMessage struct {
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content []responsesPart `json:"content"`
}

// responsesPart is one message part: the concept phrase or the image.
type responsesPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// responsesResponse is the Model API reply. Segmentation results ride the
// output_text lane; an empty lane is a valid zero-match outcome.
type responsesResponse struct {
	Output []responsesOutput `json:"output"`
}

// responsesOutput is one output item.
type responsesOutput struct {
	Type    string                   `json:"type"`
	Content []responsesOutputContent `json:"content"`
}

// responsesOutputContent is one output content part.
type responsesOutputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// sniffMimeType reports the image MIME type for a data URL from the magic
// bytes. Callers pass validated uploads (JPEG, PNG, GIF, WebP), so unknown
// input falls back to PNG rather than failing the scan.
func sniffMimeType(data []byte) string {
	if len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 6 && string(data[0:6]) == "GIF89a" || len(data) >= 6 && string(data[0:6]) == "GIF87a" {
		return "image/gif"
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	return "image/png"
}

// atoi parses a decimal box coordinate. The pattern only matches digits,
// so the conversion cannot fail; a failure yields 0 and the degenerate
// box is skipped by the caller.
func atoi(s string) int {
	n := 0
	for i := range len(s) {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

// SAMOption configures a SAMService.
type SAMOption func(*SAMService)

// WithBaseURL overrides the Model API root (tests).
func WithBaseURL(url string) SAMOption {
	return func(s *SAMService) {
		if strings.TrimSpace(url) != "" {
			s.baseURL = strings.TrimRight(strings.TrimSpace(url), "/")
		}
	}
}

// WithHTTPClient overrides the HTTP client (timeouts, tests).
func WithHTTPClient(client *http.Client) SAMOption {
	return func(s *SAMService) {
		if client != nil {
			s.httpClient = client
		}
	}
}

// SAMService implements privacydomain.Segmenter over HTTP.
type SAMService struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// segmentConcept calls the Model API once for concept and maps its box
// tokens to detections. An empty output_text lane is a valid zero-match
// outcome; a reply with no output_text at all is a wire error and fails
// closed so an API change can never pass images through unblurred.
func (s *SAMService) segmentConcept(
	ctx context.Context,
	imageBytes []byte,
	concept string,
) ([]privacydomain.Detection, error) {
	dataURL := "data:" + sniffMimeType(imageBytes) + ";base64," + base64.StdEncoding.EncodeToString(imageBytes)
	payload, err := json.Marshal(responsesRequest{
		Model: s.model,
		Input: []responsesMessage{
			{
				Type: "message",
				Role: "user",
				Content: []responsesPart{
					{Type: "input_text", Text: concept},
					{Type: "input_image", ImageURL: dataURL},
				},
			},
		},
		Stream: false,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal segment request: %w", err)
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		s.baseURL+responsesPath,
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("build segment request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("segment %q: %w", concept, err)
	}
	defer func() { _ = resp.Body.Close }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read segment %q response: %w", concept, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("segment %q: status %d", concept, resp.StatusCode)
	}
	var decoded responsesResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode segment %q response: %w", concept, err)
	}
	var texts []string
	for _, item := range decoded.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "output_text" {
				texts = append(texts, part.Text)
			}
		}
	}
	if texts == nil {
		return nil, fmt.Errorf("segment %q: no output_text in response", concept)
	}
	out := make([]privacydomain.Detection, 0)
	for _, text := range texts {
		for _, m := range boxPattern.FindAllStringSubmatch(text, -1) {
			box := image.Rect(
				atoi(m[1]),
				atoi(m[2]),
				atoi(m[3]),
				atoi(m[4]),
			)
			if box.Empty() {
				continue
			}
			out = append(out, privacydomain.NewDetection(concept, box))
		}
	}
	return out, nil
}

// Segment implements privacydomain.Segmenter. Concept calls fan out
// concurrently and are collected in vocabulary order, so the worst case
// stays near one call timeout instead of growing with every concept. The
// first error in vocabulary order wins and the scan fails closed.
func (s *SAMService) Segment(
	ctx context.Context,
	imageBytes []byte,
	concepts []string,
) ([]privacydomain.Detection, error) {
	if len(imageBytes) == 0 {
		return nil, fmt.Errorf("empty image")
	}
	effective := concepts
	if len(effective) == 0 {
		effective = append([]string(nil), privacydomain.DefaultConcepts...)
	}
	tokens := make([]string, 0, len(effective))
	for _, concept := range effective {
		token := strings.ToLower(strings.TrimSpace(concept))
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
	}
	type outcome struct {
		detections []privacydomain.Detection
		err        error
	}
	outcomes := make([]outcome, len(tokens))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	for i, token := range tokens {
		wg.Go(func() {
			detections, err := s.segmentConcept(ctx, imageBytes, token)
			if err != nil {
				cancel()
				outcomes[i].err = err
				return
			}
			outcomes[i].detections = detections
		})
	}
	wg.Wait()
	all := make([]privacydomain.Detection, 0)
	for _, o := range outcomes {
		if o.err != nil {
			return nil, o.err
		}
		all = append(all, o.detections...)
	}
	return all, nil
}

// NewSAMService builds a SAM 3.1 client. An empty model falls back to
// sam-3.1; an empty base URL falls back to the Model API root; a nil client
// gets a 30s timeout. The API key travels as a Bearer token and is never
// logged.
func NewSAMService(apiKey, model string, opts ...SAMOption) *SAMService {
	if strings.TrimSpace(model) == "" {
		model = defaultModel
	}
	s := &SAMService{
		apiKey:     apiKey,
		model:      strings.TrimSpace(model),
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
