package mcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// inProcessRemoteAddr is the synthetic remote address the in-process transport
// stamps on inner requests. Middleware that keys by client IP sees one stable
// bucket (the API-key bucket is the effective limiter), and diagnostics never
// pretend the call came from a real socket.
const inProcessRemoteAddr = "in-process"

// inProcessTransport dispatches /api/v1 requests straight into the server's own
// router instead of a TCP hop. The MCP endpoint reuses the exact agent stack —
// API-key auth, per-key rate limits, edition gating, handler validation — while
// the API key never leaves the process and the server needs no DNS or proxy
// route back to itself.
type inProcessTransport struct {
	router http.Handler
}

// RoundTrip implements http.RoundTripper. It only serves /api/v1 paths: the
// typed client is hard-wired to that surface, and the lesstruct_api escape
// hatch documents the same restriction, so anything else is a programming
// error rather than a route to the rest of the server.
func (t *inProcessTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !strings.HasPrefix(req.URL.Path, "/api/v1/") {
		return nil, fmt.Errorf("mcp: in-process transport only serves /api/v1 paths, got %q", req.URL.Path)
	}

	// The tool call runs inside the outer POST /mcp request, whose context
	// carries chi's route context (method POST, pattern /mcp). chi reuses any
	// route context it finds, so cloning that context would make it dispatch
	// the inner request by the OUTER method — a GET /api/v1/content would land
	// on the POST handler. Replace the route context with a fresh one while
	// keeping cancellation, deadlines, and request-scoped values.
	innerCtx := context.WithValue(req.Context(), chi.RouteCtxKey, chi.NewRouteContext())
	inner := req.Clone(innerCtx)
	// A server request always has a non-nil Body (http.NoBody when empty), but
	// a client request built with a nil body does not. Normalize it so a
	// downstream handler that reads the body cannot hit a nil MaxBytesReader.
	if inner.Body == nil {
		inner.Body = http.NoBody
	}
	inner.RequestURI = req.URL.RequestURI()
	inner.URL.Scheme = ""
	inner.URL.Host = ""
	inner.RemoteAddr = inProcessRemoteAddr

	rec := &responseRecorder{header: make(http.Header)}
	t.router.ServeHTTP(rec, inner)
	return rec.response(req), nil
}

// responseRecorder buffers one in-process response. The /api/v1 surface answers
// with buffered JSON (the archive endpoints stream into an io.Writer that
// buffers too), so no flusher or hijacker behavior is needed.
type responseRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

// Header implements http.ResponseWriter.
func (r *responseRecorder) Header() http.Header {
	return r.header
}

// WriteHeader implements http.ResponseWriter. The first status wins, matching
// net/http's behavior.
func (r *responseRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}

// Write implements http.ResponseWriter. Writing implies 200 when no status has
// been set yet.
func (r *responseRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(p)
}

// response converts the recorded state into an *http.Response. The body is
// fully buffered, so ContentLength is exact and the transport returns it in one
// piece.
func (r *responseRecorder) response(req *http.Request) *http.Response {
	status := r.status
	if status == 0 {
		status = http.StatusOK
	}
	header := r.header.Clone()
	header.Set("Content-Length", strconv.Itoa(r.body.Len()))
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(r.body.Bytes())),
		ContentLength: int64(r.body.Len()),
		Request:       req,
	}
}
