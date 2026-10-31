package client_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func downloadClient(t *testing.T, srv *httptest.Server) *client.Client {
	t.Helper()
	c, err := client.New(srv.URL, "k")
	require.NoError(t, err)
	return c
}

func TestClient_DownloadRespectsSizeCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(bytes.Repeat([]byte("x"), 4096))
	}))
	defer srv.Close()

	c := downloadClient(t, srv)
	var out bytes.Buffer
	err := c.ExportContent(context.Background(), &out, client.DownloadOptions{MaxBytes: 100})

	require.Error(t, err, "oversized bodies must fail loudly instead of writing a truncated archive")
	assert.Contains(t, err.Error(), "100")
}

func TestClient_DownloadUnderCapSucceeds(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := downloadClient(t, srv)
	var out bytes.Buffer
	require.NoError(t, c.ExportContent(context.Background(), &out, client.DownloadOptions{MaxBytes: 100}))
	assert.Equal(t, body, out.Bytes(), "a body exactly at the cap is kept, not tripped")
}

func TestClient_DownloadTimeoutTrips(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Drip slower than the test timeout: proves a hung server cannot
		// park the CLI forever.
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
			_, _ = w.Write([]byte("too-late"))
		}
	}))
	defer srv.Close()

	c := downloadClient(t, srv)
	var out bytes.Buffer
	start := time.Now()
	err := c.ExportContent(context.Background(), &out, client.DownloadOptions{Timeout: 300 * time.Millisecond})

	require.Error(t, err, "a hung download must fail instead of hanging")
	assert.Less(t, time.Since(start), 5*time.Second, "the timeout must fire long before the drip finishes")
}
