package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// defaultTransferTimeout caps one import/export/SSG call end to end,
	// mirroring the CLI's 30m archive default.
	defaultTransferTimeout = 30 * time.Minute

	// defaultTransferMaxBytes caps one downloaded tarball at 50 MiB. Archives
	// decode to base64 in the tool result (a third larger on the wire), so the
	// default keeps a single call from exhausting the agent's context; raise
	// maxBytes explicitly for genuinely large sites.
	defaultTransferMaxBytes = 50 << 20
)

// importArgs carries the `import_wordpress` / `import_hugo` payload. Archive
// content rides as base64; polling the matching status tool reports progress.
type importArgs struct {
	Filename      string `json:"filename" jsonschema:"archive file name (e.g. export.xml, site.tar.gz)"`
	ContentBase64 string `json:"contentBase64" jsonschema:"file bytes as base64"`
	SkipMedia     bool   `json:"skipMedia,omitempty" jsonschema:"skip migrating images (they stay linked to original paths/URLs)"`
}

// importStatusArgs carries the `import_status` / `hugo_import_status` job id.
// An empty id fetches the most recent job.
type importStatusArgs struct {
	JobID string `json:"jobId,omitempty" jsonschema:"job id from the import call (empty = most recent job)"`
}

// archiveArgs carries the `export_content` / `ssg_generate` bounds.
type archiveArgs struct {
	MaxBytes int64 `json:"maxBytes,omitempty" jsonschema:"download cap in bytes (0 = 50 MiB default)"`
}

// registerTransferTools adds the migration and static-export tools to ms. All
// handlers close over the shared /api/v1 client.
func registerTransferTools(ms *mcp.Server, c *client.Client) {
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "import_wordpress",
		Description: "Start a WordPress WXR import from base64 file bytes. Returns a job id; poll import_status for progress.",
	}, makeImportWordPressHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "import_status",
		Description: "Get a WordPress import job's status (empty job id = most recent job).",
	}, makeImportStatusHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "import_hugo",
		Description: "Start a Hugo project import from a base64 tar.gz archive. Returns a job id; poll hugo_import_status for progress.",
	}, makeImportHugoHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "hugo_import_status",
		Description: "Get a Hugo import job's status (empty job id = most recent job).",
	}, makeHugoImportStatusHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "export_content",
		Description: "Export all content as a Hugo-compatible tar.gz (base64). Large sites: raise maxBytes explicitly.",
	}, makeExportContentHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "ssg_generate",
		Description: "Render the full site to static HTML and return the tar.gz (base64). Large sites: raise maxBytes explicitly.",
	}, makeSSGGenerateHandler(c))
}

// makeImportWordPressHandler returns the `import_wordpress` handler.
func makeImportWordPressHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, importArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args importArgs,
	) (*mcp.CallToolResult, any, error) {
		raw, err := decodeMCPImport(args)
		if err != nil {
			return toolError(err), nil, nil
		}
		ctx, cancel := context.WithTimeout(ctx, defaultTransferTimeout)
		defer cancel()
		data, meta, err := c.ImportWordPress(ctx, client.ImportWordPressRequest{
			File:      bytes.NewReader(raw),
			Filename:  args.Filename,
			SkipMedia: args.SkipMedia,
		})
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no job")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeImportStatusHandler returns the `import_status` handler.
func makeImportStatusHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, importStatusArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args importStatusArgs,
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.GetImportStatus(ctx, args.JobID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no job")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeImportHugoHandler returns the `import_hugo` handler.
func makeImportHugoHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, importArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args importArgs,
	) (*mcp.CallToolResult, any, error) {
		raw, err := decodeMCPImport(args)
		if err != nil {
			return toolError(err), nil, nil
		}
		ctx, cancel := context.WithTimeout(ctx, defaultTransferTimeout)
		defer cancel()
		data, meta, err := c.ImportHugo(ctx, client.ImportHugoRequest{
			File:      bytes.NewReader(raw),
			Filename:  args.Filename,
			SkipMedia: args.SkipMedia,
		})
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no job")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeHugoImportStatusHandler returns the `hugo_import_status` handler.
func makeHugoImportStatusHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, importStatusArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args importStatusArgs,
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.GetHugoImportStatus(ctx, args.JobID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no job")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeExportContentHandler returns the `export_content` handler.
func makeExportContentHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, archiveArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args archiveArgs,
	) (*mcp.CallToolResult, any, error) {
		return downloadMCPArchive(ctx, "export.tar.gz", args.MaxBytes, c.ExportContent)
	}
}

// makeSSGGenerateHandler returns the `ssg_generate` handler.
func makeSSGGenerateHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, archiveArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args archiveArgs,
	) (*mcp.CallToolResult, any, error) {
		return downloadMCPArchive(ctx, "site.tar.gz", args.MaxBytes, c.GenerateSite)
	}
}

// decodeMCPImport validates the shared import payload and returns the file
// bytes.
func decodeMCPImport(args importArgs) ([]byte, error) {
	if strings.TrimSpace(args.Filename) == "" {
		return nil, fmt.Errorf("VALIDATION_ERROR: filename is required")
	}
	raw, err := base64.StdEncoding.DecodeString(args.ContentBase64)
	if err != nil {
		return nil, fmt.Errorf("VALIDATION_ERROR: contentBase64 is not valid base64: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("VALIDATION_ERROR: contentBase64 decoded to no bytes")
	}
	return raw, nil
}

// downloadMCPArchive streams one server-side tarball through download into a
// base64 tool result. A maxBytes of 0 selects defaultTransferMaxBytes; the
// transfer timeout bounds the whole pull.
func downloadMCPArchive(
	ctx context.Context,
	filename string,
	maxBytes int64,
	download func(context.Context, io.Writer, client.DownloadOptions) error,
) (*mcp.CallToolResult, any, error) {
	if maxBytes <= 0 {
		maxBytes = defaultTransferMaxBytes
	}
	ctx, cancel := context.WithTimeout(ctx, defaultTransferTimeout)
	defer cancel()
	var buf bytes.Buffer
	if err := download(ctx, &buf, client.DownloadOptions{Timeout: defaultTransferTimeout, MaxBytes: maxBytes}); err != nil {
		return toolError(err), nil, nil
	}
	return toolJSON(map[string]any{
		"filename":      filename,
		"bytes":         buf.Len(),
		"contentBase64": base64.StdEncoding.EncodeToString(buf.Bytes()),
	}), nil, nil
}
