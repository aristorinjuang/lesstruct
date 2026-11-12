package mcp

import (
	"net/http"
	"runtime/debug"
	"strings"

	apiv1 "github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/aristorinjuang/lesstruct/pkg/mcpserver"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// inProcessBaseURL is the synthetic origin every in-process /api/v1 request is
// built against. The transport ignores the host; an unresolvable name keeps a
// mis-wired client from silently reaching a real network destination.
const inProcessBaseURL = "http://lesstruct.invalid"

// buildVersion reports the main module version for the MCP handshake. Release
// binaries built from source report (devel), so the fallback is the same "dev"
// the stdio binary shows for a bare go build.
func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}

// bearerToken returns the token in an "Authorization: Bearer <token>" header,
// or "" when the header is absent or uses another scheme. The API-key
// middleware has already verified the token; this is the value that binds each
// request (and its session) to one /api/v1 identity.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

// Handler serves the Lesstruct MCP tool surface over Streamable HTTP. It is
// mounted at /mcp behind the Bearer API-key middleware; each request's key
// becomes the identity of the in-process /api/v1 calls it makes.
type Handler struct {
	streamable *sdk.StreamableHTTPHandler
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.streamable.ServeHTTP(w, r)
}

// New builds a Streamable HTTP handler whose tools call backend in-process.
// backend is the composed server router, so every tool call traverses the same
// middleware and handlers as a real /api/v1 request. A fresh MCP server is
// built per request: stateless mode has no session to reuse, and the API key
// (and therefore the identity) must not leak between callers.
//
// Stateless JSON responses keep the endpoint proxy-friendly and satisfy hosts
// that cap request bodies; DNS-rebinding protection is disabled because the
// server is expected to sit behind a reverse proxy (loopback listener, public
// Host header) and the Bearer key — not the browser's same-origin policy — is
// the security boundary.
func New(backend http.Handler) *Handler {
	transport := &inProcessTransport{router: backend}
	return &Handler{
		streamable: sdk.NewStreamableHTTPHandler(
			func(r *http.Request) *sdk.Server {
				key := bearerToken(r)
				if key == "" {
					return nil
				}
				client, err := apiv1.NewWithClient(
					inProcessBaseURL,
					key,
					&http.Client{Transport: transport},
				)
				if err != nil {
					return nil
				}
				return mcpserver.NewWithClient(client, buildVersion()).MCP()
			},
			&sdk.StreamableHTTPOptions{
				Stateless:                  true,
				JSONResponse:               true,
				DisableLocalhostProtection: true,
			},
		),
	}
}
