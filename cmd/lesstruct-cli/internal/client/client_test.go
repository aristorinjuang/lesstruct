package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingHandler returns a fixed body+status and records the request for
// assertions. Multiple registrations are wrapped in a mux by each test.
//
// For multipart requests, the server-side parse is attempted and the file +
// metadata parts are recorded in gotFile / gotFilename / gotMetadata so the
// media tests can assert on the multipart shape.
type recordingHandler struct {
	body        string
	status      int
	gotMethod   string
	gotPath     string
	gotQuery    string
	gotAuth     string
	gotCT       string
	gotPayload  map[string]any
	gotFile     []byte
	gotFilename string
	gotMetadata string
}

func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.gotMethod = r.Method
	h.gotPath = r.URL.Path
	h.gotQuery = r.URL.RawQuery
	h.gotAuth = r.Header.Get("Authorization")
	h.gotCT = r.Header.Get("Content-Type")

	// Multipart requests: parse the parts (file + metadata). The server's
	// actual handler uses r.ParseMultipartForm(mediadomain.MaxFileSize) (32MB);
	// 32<<20 matches.
	if strings.HasPrefix(h.gotCT, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err == nil {
			if f, fh, ferr := r.FormFile("file"); ferr == nil {
				h.gotFile, _ = io.ReadAll(f)
				h.gotFilename = fh.Filename
			}
			h.gotMetadata = r.FormValue("metadata")
		}
	} else if r.Body != nil {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &h.gotPayload)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.status)
	_, _ = w.Write([]byte(h.body))
}

func TestClient_CreateContent_Success(t *testing.T) {
	h := &recordingHandler{
		status: http.StatusOK,
		body:   `{"data":{"content":{"id":7,"title":"Hello","slug":"hello","status":"draft"}}}`,
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c, err := client.New(srv.URL, "lesstruct_a1b2c3d4e5f6_<secret>")
	require.NoError(t, err)

	data, meta, err := c.CreateContent(context.Background(), client.CreateContentRequest{
		Title:       "Hello",
		Body:        "# Hello",
		Format:      "markdown",
		IsPublished: false,
	})
	require.NoError(t, err)

	// Request shape
	assert.Equal(t, http.MethodPost, h.gotMethod)
	assert.Equal(t, "/api/v1/content", h.gotPath)
	assert.Equal(t, "Bearer lesstruct_a1b2c3d4e5f6_<secret>", h.gotAuth)
	assert.Equal(t, "application/json", h.gotCT)
	assert.Equal(t, "Hello", h.gotPayload["title"])
	assert.Equal(t, "# Hello", h.gotPayload["body"])
	assert.Equal(t, "markdown", h.gotPayload["format"])
	_, hasPublished := h.gotPayload["isPublished"]
	assert.False(t, hasPublished, "isPublished must be omitted when false")

	// Response decoding
	var content struct {
		Content struct {
			ID int `json:"id"`
		} `json:"content"`
	}
	require.NoError(t, json.Unmarshal(data, &content))
	assert.Equal(t, 7, content.Content.ID)
	assert.Nil(t, meta)
}

func TestClient_CreateContent_MetadataRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		req        client.CreateContentRequest
		wantKeys   []string // payload keys that MUST be present
		missingKey []string // payload keys that MUST be absent
		wantEqual  map[string]any
	}{
		{
			name: "postType/tags/language round-trip when set",
			req: client.CreateContentRequest{
				Title:    "Hello",
				Body:     "# Hello",
				Format:   "markdown",
				PostType: "post",
				Tags:     []string{"a", "b"},
				Language: "en",
			},
			wantKeys:  []string{"title", "body", "format", "postType", "tags", "language"},
			wantEqual: map[string]any{"postType": "post", "language": "en"},
		},
		{
			name: "postType/tags/language absent from wire when unset (omitempty)",
			req: client.CreateContentRequest{
				Title:  "Hello",
				Body:   "# Hello",
				Format: "markdown",
			},
			wantKeys:   []string{"title", "body", "format"},
			missingKey: []string{"postType", "tags", "language"},
		},
		{
			name: "empty tags slice still omitted (omitempty on empty slice)",
			req: client.CreateContentRequest{
				Title: "Hello",
				Body:  "# Hello",
				Tags:  []string{},
			},
			wantKeys:   []string{"title", "body"},
			missingKey: []string{"postType", "tags", "language"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &recordingHandler{
				status: http.StatusOK,
				body:   `{"data":{"content":{"id":7,"title":"Hello","slug":"hello","status":"draft"}}}`,
			}
			srv := httptest.NewServer(h)
			defer srv.Close()

			c, err := client.New(srv.URL, "k")
			require.NoError(t, err)

			_, _, err = c.CreateContent(context.Background(), tt.req)
			require.NoError(t, err)

			for _, k := range tt.wantKeys {
				_, ok := h.gotPayload[k]
				assert.True(t, ok, "payload must contain key %q", k)
			}
			for _, k := range tt.missingKey {
				_, ok := h.gotPayload[k]
				assert.False(t, ok, "payload must NOT contain key %q (omitempty)", k)
			}
			for k, v := range tt.wantEqual {
				assert.Equal(t, v, h.gotPayload[k], "payload[%q]", k)
			}
			// Only compare tags when the input was non-empty. An empty slice
			// is `omitempty` → field is absent from the wire; comparing
			// `[]any(nil)` to `[]any{}` would be a meaningless false fail.
			if len(tt.req.Tags) > 0 {
				gotTags, _ := h.gotPayload["tags"].([]any)
				wantTags := make([]any, len(tt.req.Tags))
				for i, s := range tt.req.Tags {
					wantTags[i] = s
				}
				assert.Equal(t, wantTags, gotTags, "payload[tags]")
			}
		})
	}
}

func TestClient_ErrorEnvelope(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantCode   string
		wantStatus int
	}{
		{
			name:       "validation error",
			status:     http.StatusBadRequest,
			body:       `{"error":{"code":"VALIDATION_ERROR","message":"title is required"}}`,
			wantCode:   "VALIDATION_ERROR",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unauthorized",
			status:     http.StatusUnauthorized,
			body:       `{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`,
			wantCode:   "INVALID_API_KEY",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "not found",
			status:     http.StatusNotFound,
			body:       `{"error":{"code":"NOT_FOUND","message":"missing"}}`,
			wantCode:   "NOT_FOUND",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "rate limited",
			status:     http.StatusTooManyRequests,
			body:       `{"error":{"code":"RATE_LIMITED","message":"slow down"}}`,
			wantCode:   "RATE_LIMITED",
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "server error",
			status:     http.StatusInternalServerError,
			body:       `{"error":{"code":"INTERNAL_ERROR","message":"boom"}}`,
			wantCode:   "INTERNAL_ERROR",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &recordingHandler{status: tt.status, body: tt.body}
			srv := httptest.NewServer(h)
			defer srv.Close()

			c, err := client.New(srv.URL, "k")
			require.NoError(t, err)

			_, _, err = c.CreateContent(context.Background(), client.CreateContentRequest{
				Title:  "x",
				Body:   "x",
				Format: "markdown",
			})
			require.Error(t, err)

			apiErr, ok := err.(*client.APIError)
			require.True(t, ok, "error must be *client.APIError, got %T", err)
			assert.Equal(t, tt.wantStatus, apiErr.StatusCode)
			assert.Equal(t, tt.wantCode, apiErr.Code)
		})
	}
}

func TestClient_NetworkError(t *testing.T) {
	// An unreachable URL (closed listener) yields a transport error → APIError
	// with StatusCode 0.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	c, err := client.New(srv.URL, "k")
	require.NoError(t, err)

	_, _, err = c.CreateContent(context.Background(), client.CreateContentRequest{
		Title:  "x",
		Body:   "x",
		Format: "markdown",
	})
	require.Error(t, err)
	apiErr, ok := err.(*client.APIError)
	require.True(t, ok)
	assert.Equal(t, 0, apiErr.StatusCode)
}

func TestClient_BaseURLJoin(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"content":{"id":1}}}`))
	}))
	defer srv.Close()

	// A trailing slash must be tolerated.
	c, err := client.New(srv.URL+"/", "k")
	require.NoError(t, err)
	_, _, err = c.CreateContent(context.Background(), client.CreateContentRequest{
		Title:  "x",
		Body:   "x",
		Format: "markdown",
	})
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/content", gotPath)
}

func TestClient_InvalidBaseURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "missing scheme", url: "localhost:8080"},
		{name: "garbage", url: "://nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.New(tt.url, "k")
			require.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), "base url"))
		})
	}
}

func TestClient_GetContent(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantCode   string
		wantStatus int
	}{
		{
			name:       "success",
			status:     http.StatusOK,
			body:       `{"data":{"content":{"id":7,"title":"Hello","slug":"hello","status":"published"}}}`,
		},
		{
			name:       "not found",
			status:     http.StatusNotFound,
			body:       `{"error":{"code":"NOT_FOUND","message":"missing"}}`,
			wantCode:   "NOT_FOUND",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "unauthorized",
			status:     http.StatusUnauthorized,
			body:       `{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`,
			wantCode:   "INVALID_API_KEY",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "validation error on bad id",
			status:     http.StatusBadRequest,
			body:       `{"error":{"code":"VALIDATION_ERROR","message":"Invalid content ID"}}`,
			wantCode:   "VALIDATION_ERROR",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &recordingHandler{status: tt.status, body: tt.body}
			srv := httptest.NewServer(h)
			defer srv.Close()

			c, err := client.New(srv.URL, "k")
			require.NoError(t, err)

			data, _, err := c.GetContent(context.Background(), 7)
			if tt.wantCode != "" {
				require.Error(t, err)
				apiErr, ok := err.(*client.APIError)
				require.True(t, ok, "error must be *client.APIError, got %T", err)
				assert.Equal(t, tt.wantStatus, apiErr.StatusCode)
				assert.Equal(t, tt.wantCode, apiErr.Code)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, http.MethodGet, h.gotMethod)
			assert.Equal(t, "/api/v1/content/7", h.gotPath)
			assert.Equal(t, "Bearer k", h.gotAuth)
			var content struct {
				Content struct {
					ID int `json:"id"`
				} `json:"content"`
			}
			require.NoError(t, json.Unmarshal(data, &content))
			assert.Equal(t, 7, content.Content.ID)
		})
	}
}

func TestClient_GetContent_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	c, err := client.New(srv.URL, "k")
	require.NoError(t, err)

	_, _, err = c.GetContent(context.Background(), 7)
	require.Error(t, err)
	apiErr, ok := err.(*client.APIError)
	require.True(t, ok)
	assert.Equal(t, 0, apiErr.StatusCode)
}

func TestClient_ListContent(t *testing.T) {
	t.Run("with items and pagination", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body: `{"data":[
				{"id":7,"title":"A","slug":"a","status":"draft"},
				{"id":6,"title":"B","slug":"b","status":"published"}
			],"meta":{"pagination":{"nextCursor":"Ng","hasMore":true}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		data, meta, err := c.ListContent(context.Background(), 2, "Ng", client.ListContentFilters{})
		require.NoError(t, err)
		assert.Equal(t, http.MethodGet, h.gotMethod)
		assert.Equal(t, "/api/v1/content", h.gotPath)
		// url.Values.Encode sorts keys alphabetically: cursor < limit.
		assert.Equal(t, "cursor=Ng&limit=2", h.gotQuery)
		assert.Equal(t, "Bearer k", h.gotAuth)

		var items []struct {
			ID int `json:"id"`
		}
		require.NoError(t, json.Unmarshal(data, &items))
		assert.Equal(t, []int{7, 6}, []int{items[0].ID, items[1].ID})

		var m struct {
			Pagination struct {
				NextCursor string `json:"nextCursor"`
				HasMore    bool   `json:"hasMore"`
			} `json:"pagination"`
		}
		require.NoError(t, json.Unmarshal(meta, &m))
		assert.Equal(t, "Ng", m.Pagination.NextCursor)
		assert.True(t, m.Pagination.HasMore)
	})

	t.Run("empty list no cursor", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"data":[],"meta":{"pagination":{"hasMore":false}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		data, meta, err := c.ListContent(context.Background(), 0, "", client.ListContentFilters{})
		require.NoError(t, err)
		assert.Equal(t, "", h.gotQuery, "no limit / no cursor → no query string")

		var items []any
		require.NoError(t, json.Unmarshal(data, &items))
		assert.Empty(t, items)

		var m struct {
			Pagination struct {
				HasMore bool `json:"hasMore"`
			} `json:"pagination"`
		}
		require.NoError(t, json.Unmarshal(meta, &m))
		assert.False(t, m.Pagination.HasMore)
	})

	t.Run("invalid cursor", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusBadRequest,
			body:   `{"error":{"code":"VALIDATION_ERROR","message":"Invalid cursor"}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.ListContent(context.Background(), 10, "garbage", client.ListContentFilters{})
		require.Error(t, err)
		apiErr, ok := err.(*client.APIError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
		assert.Equal(t, "VALIDATION_ERROR", apiErr.Code)
	})

	t.Run("single tag filter emits tag query param", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"data":[],"meta":{"pagination":{"hasMore":false}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.ListContent(context.Background(), 0, "", client.ListContentFilters{
			Tags: []string{"go"},
		})
		require.NoError(t, err)
		assert.Equal(t, "tag=go", h.gotQuery)
	})

	t.Run("multiple tag filters emit repeated tag query keys", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"data":[],"meta":{"pagination":{"hasMore":false}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.ListContent(context.Background(), 0, "", client.ListContentFilters{
			Tags: []string{"go", "tutorial"},
		})
		require.NoError(t, err)
		// url.Values encodes a slice as repeated keys, in order.
		assert.Equal(t, "tag=go&tag=tutorial", h.gotQuery)
	})

	t.Run("language filter emits language query param", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"data":[],"meta":{"pagination":{"hasMore":false}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.ListContent(context.Background(), 0, "", client.ListContentFilters{
			Language: "en",
		})
		require.NoError(t, err)
		assert.Equal(t, "language=en", h.gotQuery)
	})

	t.Run("status filter emits status query param", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"data":[],"meta":{"pagination":{"hasMore":false}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.ListContent(context.Background(), 0, "", client.ListContentFilters{
			Status: "draft",
		})
		require.NoError(t, err)
		assert.Equal(t, "status=draft", h.gotQuery)
	})

	t.Run("post_type filter emits post_type query param", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"data":[],"meta":{"pagination":{"hasMore":false}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.ListContent(context.Background(), 0, "", client.ListContentFilters{
			PostType: "post",
		})
		require.NoError(t, err)
		assert.Equal(t, "post_type=post", h.gotQuery)
	})

	t.Run("author filter emits author query param", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"data":[],"meta":{"pagination":{"hasMore":false}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.ListContent(context.Background(), 0, "", client.ListContentFilters{
			Author: "alice",
		})
		require.NoError(t, err)
		assert.Equal(t, "author=alice", h.gotQuery)
	})

	t.Run("search filter emits search query param", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"data":[],"meta":{"pagination":{"hasMore":false}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.ListContent(context.Background(), 0, "", client.ListContentFilters{
			Search: "golang",
		})
		require.NoError(t, err)
		assert.Equal(t, "search=golang", h.gotQuery)
	})

	t.Run("all filters combined produce alphabetically sorted query string", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"data":[],"meta":{"pagination":{"hasMore":false}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.ListContent(context.Background(), 10, "Ng", client.ListContentFilters{
			Tags:     []string{"go", "tutorial"},
			Language: "en",
			Status:   "draft",
			PostType: "post",
			Author:   "alice",
			Search:   "golang",
		})
		require.NoError(t, err)
		// url.Values.Encode sorts keys alphabetically: author < cursor < language < limit < post_type < search < status < tag.
		// Repeated tag keys append in slice order after all other single keys.
		assert.Equal(t,
			"author=alice&cursor=Ng&language=en&limit=10&post_type=post&search=golang&status=draft&tag=go&tag=tutorial",
			h.gotQuery,
		)
	})

	t.Run("empty filter values are omitted from the wire", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"data":[],"meta":{"pagination":{"hasMore":false}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.ListContent(context.Background(), 0, "", client.ListContentFilters{
			Tags:     nil,
			Language: "",
			Status:   "",
			PostType: "",
			Author:   "",
			Search:   "",
		})
		require.NoError(t, err)
		assert.Equal(t, "", h.gotQuery, "all-empty filters → no query string")
	})
}

func TestClient_UpdateContent(t *testing.T) {
	t.Run("success sends PUT with payload and decodes response", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"data":{"content":{"id":7,"title":"Hello v2","slug":"hello-v2","status":"published"}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		data, meta, err := c.UpdateContent(
			context.Background(),
			7,
			client.UpdateContentRequest{
				Title:       "Hello v2",
				Body:        "# Hello v2",
				Format:      "markdown",
				IsPublished: true,
			},
		)
		require.NoError(t, err)
		assert.Equal(t, http.MethodPut, h.gotMethod)
		assert.Equal(t, "/api/v1/content/7", h.gotPath)
		assert.Equal(t, "Bearer k", h.gotAuth)
		assert.Equal(t, "application/json", h.gotCT)
		assert.Equal(t, "Hello v2", h.gotPayload["title"])
		assert.Equal(t, "# Hello v2", h.gotPayload["body"])
		assert.Equal(t, "markdown", h.gotPayload["format"])
		assert.Equal(t, true, h.gotPayload["isPublished"])

		var resp struct {
			Content struct {
				ID     int    `json:"id"`
				Title  string `json:"title"`
				Slug   string `json:"slug"`
				Status string `json:"status"`
			} `json:"content"`
		}
		require.NoError(t, json.Unmarshal(data, &resp))
		assert.Equal(t, 7, resp.Content.ID)
		assert.Equal(t, "Hello v2", resp.Content.Title)
		assert.Equal(t, "hello-v2", resp.Content.Slug)
		assert.Equal(t, "published", resp.Content.Status)
		assert.Nil(t, meta)
	})

	t.Run("isPublished omitted when false", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"data":{"content":{"id":7,"title":"Hi","slug":"hi","status":"draft"}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.UpdateContent(
			context.Background(),
			7,
			client.UpdateContentRequest{
				Title:  "Hi",
				Body:   "x",
				Format: "markdown",
			},
		)
		require.NoError(t, err)
		_, hasPublished := h.gotPayload["isPublished"]
		assert.False(t, hasPublished, "isPublished must be omitted when false")
	})

	tests := []struct {
		name       string
		status     int
		body       string
		wantCode   string
		wantStatus int
	}{
		{
			name:       "not found",
			status:     http.StatusNotFound,
			body:       `{"error":{"code":"NOT_FOUND","message":"missing"}}`,
			wantCode:   "NOT_FOUND",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "unauthorized",
			status:     http.StatusUnauthorized,
			body:       `{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`,
			wantCode:   "INVALID_API_KEY",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "validation error on bad id",
			status:     http.StatusBadRequest,
			body:       `{"error":{"code":"VALIDATION_ERROR","message":"Invalid content ID"}}`,
			wantCode:   "VALIDATION_ERROR",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "validation error on bad body",
			status:     http.StatusBadRequest,
			body:       `{"error":{"code":"VALIDATION_ERROR","message":"body is required"}}`,
			wantCode:   "VALIDATION_ERROR",
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &recordingHandler{status: tt.status, body: tt.body}
			srv := httptest.NewServer(h)
			defer srv.Close()

			c, err := client.New(srv.URL, "k")
			require.NoError(t, err)

			_, _, err = c.UpdateContent(
				context.Background(),
				7,
				client.UpdateContentRequest{Title: "x", Body: "x", Format: "markdown"},
			)
			require.Error(t, err)
			apiErr, ok := err.(*client.APIError)
			require.True(t, ok, "error must be *client.APIError, got %T", err)
			assert.Equal(t, tt.wantStatus, apiErr.StatusCode)
			assert.Equal(t, tt.wantCode, apiErr.Code)
		})
	}
}

func TestClient_UpdateContent_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	c, err := client.New(srv.URL, "k")
	require.NoError(t, err)

	_, _, err = c.UpdateContent(
		context.Background(),
		7,
		client.UpdateContentRequest{Title: "x", Body: "x", Format: "markdown"},
	)
	require.Error(t, err)
	apiErr, ok := err.(*client.APIError)
	require.True(t, ok)
	assert.Equal(t, 0, apiErr.StatusCode)
}

func TestClient_DeleteContent(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantCode   string
		wantStatus int
	}{
		{
			name:   "204 no content success",
			status: http.StatusNoContent,
			body:   "",
		},
		{
			name:       "not found",
			status:     http.StatusNotFound,
			body:       `{"error":{"code":"NOT_FOUND","message":"missing"}}`,
			wantCode:   "NOT_FOUND",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "unauthorized",
			status:     http.StatusUnauthorized,
			body:       `{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`,
			wantCode:   "INVALID_API_KEY",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "validation error on bad id",
			status:     http.StatusBadRequest,
			body:       `{"error":{"code":"VALIDATION_ERROR","message":"Invalid content ID"}}`,
			wantCode:   "VALIDATION_ERROR",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &recordingHandler{status: tt.status, body: tt.body}
			srv := httptest.NewServer(h)
			defer srv.Close()

			c, err := client.New(srv.URL, "k")
			require.NoError(t, err)

			data, meta, err := c.DeleteContent(context.Background(), 7)
			if tt.wantCode != "" {
				require.Error(t, err)
				apiErr, ok := err.(*client.APIError)
				require.True(t, ok)
				assert.Equal(t, tt.wantStatus, apiErr.StatusCode)
				assert.Equal(t, tt.wantCode, apiErr.Code)
				return
			}
			require.NoError(t, err)
			assert.Nil(t, data)
			assert.Nil(t, meta)
			assert.Equal(t, http.MethodDelete, h.gotMethod)
			assert.Equal(t, "/api/v1/content/7", h.gotPath)
		})
	}
}

func TestClient_PublishContent(t *testing.T) {
	const successBody = `{"data":{"content":{"id":7,"title":"Hello","slug":"hello","status":"published"}},"error":null}`

	tests := []struct {
		name       string
		status     int
		body       string
		wantCode   string
		wantStatus int
	}{
		{
			name:   "success - 200 with published projection",
			status: http.StatusOK,
			body:   successBody,
		},
		{
			name:   "idempotent - 200 with no body is also success-without-data",
			status: http.StatusOK,
			body:   "",
		},
		{
			name:       "not found",
			status:     http.StatusNotFound,
			body:       `{"error":{"code":"NOT_FOUND","message":"Content not found"}}`,
			wantCode:   "NOT_FOUND",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "unauthorized",
			status:     http.StatusUnauthorized,
			body:       `{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`,
			wantCode:   "INVALID_API_KEY",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "validation error on bad id",
			status:     http.StatusBadRequest,
			body:       `{"error":{"code":"VALIDATION_ERROR","message":"Invalid content ID"}}`,
			wantCode:   "VALIDATION_ERROR",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &recordingHandler{status: tt.status, body: tt.body}
			srv := httptest.NewServer(h)
			defer srv.Close()

			c, err := client.New(srv.URL, "k")
			require.NoError(t, err)

			data, meta, err := c.PublishContent(context.Background(), 7)
			if tt.wantCode != "" {
				require.Error(t, err)
				apiErr, ok := err.(*client.APIError)
				require.True(t, ok)
				assert.Equal(t, tt.wantStatus, apiErr.StatusCode)
				assert.Equal(t, tt.wantCode, apiErr.Code)
				return
			}
			require.NoError(t, err)
			if tt.body == "" {
				assert.Nil(t, data)
				assert.Nil(t, meta)
			} else {
				assert.Contains(t, string(data), `"status":"published"`)
			}
			assert.Equal(t, http.MethodPost, h.gotMethod)
			assert.Equal(t, "/api/v1/content/7/publish", h.gotPath)
		})
	}
}

func TestClient_UnpublishContent(t *testing.T) {
	const successBody = `{"data":{"content":{"id":7,"title":"Hello","slug":"hello","status":"draft"}},"error":null}`

	tests := []struct {
		name       string
		status     int
		body       string
		wantCode   string
		wantStatus int
	}{
		{
			name:   "success - 200 with draft projection",
			status: http.StatusOK,
			body:   successBody,
		},
		{
			name:   "idempotent - 200 with no body",
			status: http.StatusOK,
			body:   "",
		},
		{
			name:       "not found",
			status:     http.StatusNotFound,
			body:       `{"error":{"code":"NOT_FOUND","message":"Content not found"}}`,
			wantCode:   "NOT_FOUND",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "unauthorized",
			status:     http.StatusUnauthorized,
			body:       `{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`,
			wantCode:   "INVALID_API_KEY",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "validation error on bad id",
			status:     http.StatusBadRequest,
			body:       `{"error":{"code":"VALIDATION_ERROR","message":"Invalid content ID"}}`,
			wantCode:   "VALIDATION_ERROR",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &recordingHandler{status: tt.status, body: tt.body}
			srv := httptest.NewServer(h)
			defer srv.Close()

			c, err := client.New(srv.URL, "k")
			require.NoError(t, err)

			data, meta, err := c.UnpublishContent(context.Background(), 7)
			if tt.wantCode != "" {
				require.Error(t, err)
				apiErr, ok := err.(*client.APIError)
				require.True(t, ok)
				assert.Equal(t, tt.wantStatus, apiErr.StatusCode)
				assert.Equal(t, tt.wantCode, apiErr.Code)
				return
			}
			require.NoError(t, err)
			if tt.body == "" {
				assert.Nil(t, data)
				assert.Nil(t, meta)
			} else {
				assert.Contains(t, string(data), `"status":"draft"`)
			}
			assert.Equal(t, http.MethodPost, h.gotMethod)
			assert.Equal(t, "/api/v1/content/7/unpublish", h.gotPath)
		})
	}
}

func TestClient_Do_EmptySuccess(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "204 no content", status: http.StatusNoContent, body: ""},
		{name: "200 with empty body", status: http.StatusOK, body: ""},
		{name: "304 not modified", status: http.StatusNotModified, body: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &recordingHandler{status: tt.status, body: tt.body}
			srv := httptest.NewServer(h)
			defer srv.Close()

			c, err := client.New(srv.URL, "k")
			require.NoError(t, err)

			data, meta, err := c.GetContent(context.Background(), 1)
			require.NoError(t, err, "2xx with empty body must be success")
			assert.Nil(t, data)
			assert.Nil(t, meta)
		})
	}

	t.Run("200 with error envelope still errors", func(t *testing.T) {
		// Sanity: the 2xx+empty shortcut must NOT short-circuit an actual error
		// envelope. (A 2xx would not normally have an error envelope, but the
		// belt-and-suspenders check matters because some misbehaving servers
		// return both.)
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"error":{"code":"INTERNAL_ERROR","message":"oops"}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.GetContent(context.Background(), 1)
		require.Error(t, err)
		apiErr, ok := err.(*client.APIError)
		require.True(t, ok)
		assert.Equal(t, "INTERNAL_ERROR", apiErr.Code)
	})

	// Note: 204 + 304 + body cannot be exercised via httptest — Go's net/http
	// server strips the body automatically for these statuses (per RFC 7230/7232).
	// The 204/304 + envelope defense in `do()` is therefore verified by code
	// review and the comment in the source, not by an httptest case. The
	// 200 + envelope case above proves the envelope-decoding path works when
	// the body is non-empty.
}

func TestClient_UploadMedia(t *testing.T) {
	const successBody = `{"data":{"media":{"id":5,"filename":"photo.jpg","originalFilename":"photo.jpg","mimeType":"image/jpeg","fileSize":3,"altText":"a view","isWebp":false,"hash":"abc","url":"http://example/uploads/photo.jpg"}}}`

	t.Run("success with file and metadata", func(t *testing.T) {
		h := &recordingHandler{status: http.StatusOK, body: successBody}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		data, _, err := c.UploadMedia(
			context.Background(),
			client.UploadMediaRequest{
				File:     strings.NewReader("jpg"),
				Filename: "photo.jpg",
				Metadata: map[string]string{"altText": "a view"},
			},
		)
		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, h.gotMethod)
		assert.Equal(t, "/api/v1/media", h.gotPath)
		assert.Equal(t, "Bearer k", h.gotAuth)
		assert.True(t, strings.HasPrefix(h.gotCT, "multipart/form-data; boundary="), "Content-Type must be multipart with boundary, got %q", h.gotCT)
		assert.Equal(t, "photo.jpg", h.gotFilename)
		assert.Equal(t, []byte("jpg"), h.gotFile)
		assert.Contains(t, h.gotMetadata, `"altText":"a view"`)

		var resp struct {
			Media struct {
				ID int `json:"id"`
			} `json:"media"`
		}
		require.NoError(t, json.Unmarshal(data, &resp))
		assert.Equal(t, 5, resp.Media.ID)
	})

	t.Run("success without metadata", func(t *testing.T) {
		h := &recordingHandler{status: http.StatusOK, body: successBody}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.UploadMedia(
			context.Background(),
			client.UploadMediaRequest{
				File:     strings.NewReader("x"),
				Filename: "x.bin",
			},
		)
		require.NoError(t, err)
		assert.Equal(t, []byte("x"), h.gotFile)
		assert.Equal(t, "", h.gotMetadata, "no metadata → no metadata part")
	})

	t.Run("missing file", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusBadRequest,
			body:   `{"error":{"code":"VALIDATION_ERROR","message":"file part is required"}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.UploadMedia(
			context.Background(),
			client.UploadMediaRequest{File: strings.NewReader("x"), Filename: "x"},
		)
		require.Error(t, err)
		apiErr, ok := err.(*client.APIError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
		assert.Equal(t, "VALIDATION_ERROR", apiErr.Code)
	})

	t.Run("unauthorized", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusUnauthorized,
			body:   `{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.UploadMedia(
			context.Background(),
			client.UploadMediaRequest{File: strings.NewReader("x"), Filename: "x"},
		)
		require.Error(t, err)
		apiErr, ok := err.(*client.APIError)
		require.True(t, ok)
		assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
	})

	t.Run("duplicate → 409 conflict", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusConflict,
			body:   `{"error":{"code":"CONFLICT","message":"file with the same content hash already exists"}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.UploadMedia(
			context.Background(),
			client.UploadMediaRequest{File: strings.NewReader("x"), Filename: "x"},
		)
		require.Error(t, err)
		apiErr, ok := err.(*client.APIError)
		require.True(t, ok)
		assert.Equal(t, http.StatusConflict, apiErr.StatusCode)
		assert.Equal(t, "CONFLICT", apiErr.Code)
	})
}

func TestClient_GetMedia(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantCode   string
		wantStatus int
	}{
		{
			name:   "success",
			status: http.StatusOK,
			body:   `{"data":{"media":{"id":5,"filename":"photo.jpg","mimeType":"image/jpeg","url":"http://example/uploads/photo.jpg"}}}`,
		},
		{
			name:       "not found",
			status:     http.StatusNotFound,
			body:       `{"error":{"code":"NOT_FOUND","message":"missing"}}`,
			wantCode:   "NOT_FOUND",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "unauthorized",
			status:     http.StatusUnauthorized,
			body:       `{"error":{"code":"INVALID_API_KEY","message":"bad key"}}`,
			wantCode:   "INVALID_API_KEY",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid id",
			status:     http.StatusBadRequest,
			body:       `{"error":{"code":"VALIDATION_ERROR","message":"Invalid media ID"}}`,
			wantCode:   "VALIDATION_ERROR",
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &recordingHandler{status: tt.status, body: tt.body}
			srv := httptest.NewServer(h)
			defer srv.Close()

			c, err := client.New(srv.URL, "k")
			require.NoError(t, err)

			data, _, err := c.GetMedia(context.Background(), 5)
			if tt.wantCode != "" {
				require.Error(t, err)
				apiErr, ok := err.(*client.APIError)
				require.True(t, ok)
				assert.Equal(t, tt.wantStatus, apiErr.StatusCode)
				assert.Equal(t, tt.wantCode, apiErr.Code)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, http.MethodGet, h.gotMethod)
			assert.Equal(t, "/api/v1/media/5", h.gotPath)
			assert.NotNil(t, data)
		})
	}
}

func TestClient_ListMedia(t *testing.T) {
	t.Run("with items and pagination", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body: `{"data":[
				{"id":5,"filename":"a.jpg","mimeType":"image/jpeg"},
				{"id":4,"filename":"b.png","mimeType":"image/png"}
			],"meta":{"pagination":{"nextCursor":"NA","hasMore":true}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		data, meta, err := c.ListMedia(context.Background(), 2, "NA")
		require.NoError(t, err)
		assert.Equal(t, http.MethodGet, h.gotMethod)
		assert.Equal(t, "/api/v1/media", h.gotPath)
		// url.Values.Encode sorts alphabetically: cursor < limit.
		assert.Equal(t, "cursor=NA&limit=2", h.gotQuery)

		var items []struct {
			ID int `json:"id"`
		}
		require.NoError(t, json.Unmarshal(data, &items))
		assert.Equal(t, []int{5, 4}, []int{items[0].ID, items[1].ID})
		assert.NotNil(t, meta)
	})

	t.Run("empty list no cursor", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"data":[],"meta":{"pagination":{"hasMore":false}}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		data, _, err := c.ListMedia(context.Background(), 0, "")
		require.NoError(t, err)
		assert.Equal(t, "", h.gotQuery, "no limit / no cursor → no query string")

		var items []any
		require.NoError(t, json.Unmarshal(data, &items))
		assert.Empty(t, items)
	})

	t.Run("invalid cursor", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusBadRequest,
			body:   `{"error":{"code":"VALIDATION_ERROR","message":"Invalid cursor"}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.ListMedia(context.Background(), 10, "garbage")
		require.Error(t, err)
		apiErr, ok := err.(*client.APIError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
		assert.Equal(t, "VALIDATION_ERROR", apiErr.Code)
	})
}

func TestClient_UploadMedia_Validation(t *testing.T) {
	c, err := client.New("http://example", "k")
	require.NoError(t, err)

	t.Run("nil File is rejected", func(t *testing.T) {
		_, _, err = c.UploadMedia(
			context.Background(),
			client.UploadMediaRequest{File: nil, Filename: "x"},
		)
		require.Error(t, err)
		apiErr, ok := err.(*client.APIError)
		require.True(t, ok)
		assert.Equal(t, "VALIDATION_ERROR", apiErr.Code)
		assert.Contains(t, apiErr.Message, "File is required")
	})

	t.Run("empty Filename is rejected", func(t *testing.T) {
		_, _, err = c.UploadMedia(
			context.Background(),
			client.UploadMediaRequest{File: strings.NewReader("x"), Filename: ""},
		)
		require.Error(t, err)
		apiErr, ok := err.(*client.APIError)
		require.True(t, ok)
		assert.Equal(t, "VALIDATION_ERROR", apiErr.Code)
		assert.Contains(t, apiErr.Message, "Filename is required")
	})
}

func TestClient_DoRaw_EmptySuccess(t *testing.T) {
	// The 204/304 + empty-2xx + 200+envelope defenses from doRequest apply to
	// the doRaw path too (used by UploadMedia). Verify on the new path: a 2xx
	// with empty body through doRaw (multipart body) is success-without-data;
	// a 200 with an error envelope is still an error.
	t.Run("200 with empty body through doRaw", func(t *testing.T) {
		h := &recordingHandler{status: http.StatusOK, body: ""}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.UploadMedia(
			context.Background(),
			client.UploadMediaRequest{File: strings.NewReader("x"), Filename: "x"},
		)
		require.NoError(t, err, "2xx with empty body through doRaw must be success")
	})

	t.Run("200 with error envelope through doRaw still errors", func(t *testing.T) {
		h := &recordingHandler{
			status: http.StatusOK,
			body:   `{"error":{"code":"INTERNAL_ERROR","message":"server lied"}}`,
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		c, err := client.New(srv.URL, "k")
		require.NoError(t, err)

		_, _, err = c.UploadMedia(
			context.Background(),
			client.UploadMediaRequest{File: strings.NewReader("x"), Filename: "x"},
		)
		require.Error(t, err)
		apiErr, ok := err.(*client.APIError)
		require.True(t, ok)
		assert.Equal(t, "INTERNAL_ERROR", apiErr.Code)
	})
}
