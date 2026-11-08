package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mediaListArgs carries the `media_list` pagination.
type mediaListArgs struct {
	Limit  int    `json:"limit,omitempty" jsonschema:"page size (0 = server default)"`
	Cursor string `json:"cursor,omitempty" jsonschema:"opaque cursor from a previous call's meta.pagination.nextCursor"`
}

// mediaGetArgs carries the `media_get` / `media_delete` id.
type mediaGetArgs struct {
	ID int `json:"id" jsonschema:"media item id"`
}

// mediaUploadArgs carries the `media_upload` payload. Binary content rides as
// base64 — keep uploads modest; the server enforces its own size limit.
type mediaUploadArgs struct {
	Filename      string `json:"filename" jsonschema:"file name with extension (e.g. photo.jpg)"`
	ContentBase64 string `json:"contentBase64" jsonschema:"file bytes as base64"`
	AltText       string `json:"altText,omitempty" jsonschema:"alt text for the image"`
}

// registerMediaTools adds the media library tools to ms. All handlers close
// over the shared /api/v1 client.
func registerMediaTools(ms *mcp.Server, c *client.Client) {
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "media_list",
		Description: "List media files, newest first (paginated). Follow meta.pagination.nextCursor while meta.pagination.hasMore is true.",
	}, makeMediaListHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "media_get",
		Description: "Get a single media item by id (metadata and URL, not the bytes).",
	}, makeMediaGetHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "media_upload",
		Description: "Upload a file (image or other) as base64. Images are transcoded to WebP server-side.",
	}, makeMediaUploadHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "media_delete",
		Description: "Delete a media item by id. Destructive and final.",
	}, makeMediaDeleteHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "media_reencode",
		Description: "Re-encode the whole media library to WebP (admin only). Takes minutes server-side; prefer it during quiet hours.",
	}, makeMediaReencodeHandler(c))
}

// makeMediaListHandler returns the `media_list` handler.
func makeMediaListHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, mediaListArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args mediaListArgs,
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.ListMedia(ctx, args.Limit, args.Cursor)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeMediaGetHandler returns the `media_get` handler.
func makeMediaGetHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, mediaGetArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args mediaGetArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid media id: %d", args.ID)), nil, nil
		}
		data, meta, err := c.GetMedia(ctx, args.ID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no media")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeMediaUploadHandler returns the `media_upload` handler.
func makeMediaUploadHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, mediaUploadArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args mediaUploadArgs,
	) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.Filename) == "" {
			return toolError(fmt.Errorf("VALIDATION_ERROR: filename is required")), nil, nil
		}
		raw, err := base64.StdEncoding.DecodeString(args.ContentBase64)
		if err != nil {
			return toolError(fmt.Errorf("VALIDATION_ERROR: contentBase64 is not valid base64: %w", err)), nil, nil
		}
		var metadata map[string]any
		if strings.TrimSpace(args.AltText) != "" {
			metadata = map[string]any{"altText": args.AltText}
		}
		data, meta, err := c.UploadMedia(ctx, client.UploadMediaRequest{
			File:     bytes.NewReader(raw),
			Filename: args.Filename,
			Metadata: metadata,
		})
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no media")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeMediaDeleteHandler returns the `media_delete` handler.
func makeMediaDeleteHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, mediaGetArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args mediaGetArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid media id: %d", args.ID)), nil, nil
		}
		if _, _, err := c.DeleteMedia(ctx, args.ID); err != nil {
			return toolError(err), nil, nil
		}
		return toolJSON(map[string]any{"deleted": true, "id": args.ID}), nil, nil
	}
}

// makeMediaReencodeHandler returns the `media_reencode` handler.
func makeMediaReencodeHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ struct{},
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.ReencodeMedia(ctx)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}
