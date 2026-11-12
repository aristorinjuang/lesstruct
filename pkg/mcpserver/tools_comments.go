package mcpserver

import (
	"context"
	"fmt"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// commentCreateArgs carries the `comment_create` payload.
type commentCreateArgs struct {
	ContentID int    `json:"contentId" jsonschema:"content item id to comment on"`
	Comment   string `json:"comment" jsonschema:"comment text"`
}

// commentListArgs carries the `comment_list` content id.
type commentListArgs struct {
	ContentID int `json:"contentId" jsonschema:"content item id"`
}

// commentDeleteArgs carries the `comment_delete` ids. An admin key may delete
// any comment; a non-admin key only its own (foreign ids read 404).
type commentDeleteArgs struct {
	ContentID int `json:"contentId" jsonschema:"content item id"`
	CommentID int `json:"commentId" jsonschema:"comment id"`
}

// commentModerateArgs carries the `comment_moderate` payload (admin only).
type commentModerateArgs struct {
	ContentID int    `json:"contentId" jsonschema:"content item id"`
	CommentID int    `json:"commentId" jsonschema:"comment id"`
	Status    string `json:"status" jsonschema:"target status: approved, rejected, or spam"`
}

// registerCommentTools adds the discussion tools to ms. All handlers close
// over the shared /api/v1 client.
func registerCommentTools(ms *mcp.Server, c *client.Client) {
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "comment_create",
		Description: "Post a comment on a content item.",
	}, makeCommentCreateHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "comment_list",
		Description: "List the comments of a content item.",
	}, makeCommentListHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "comment_delete",
		Description: "Delete a comment. Destructive and final.",
	}, makeCommentDeleteHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "comment_moderate",
		Description: "Set a comment's moderation status (admin only): approved, rejected, or spam.",
	}, makeCommentModerateHandler(c))
}

// makeCommentCreateHandler returns the `comment_create` handler.
func makeCommentCreateHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, commentCreateArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args commentCreateArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ContentID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid content id: %d", args.ContentID)), nil, nil
		}
		if args.Comment == "" {
			return toolError(fmt.Errorf("VALIDATION_ERROR: comment is required")), nil, nil
		}
		data, meta, err := c.CreateComment(ctx, args.ContentID, client.CreateCommentRequest{Comment: args.Comment})
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no comment")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeCommentListHandler returns the `comment_list` handler.
func makeCommentListHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, commentListArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args commentListArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ContentID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid content id: %d", args.ContentID)), nil, nil
		}
		data, meta, err := c.ListComments(ctx, args.ContentID)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeCommentDeleteHandler returns the `comment_delete` handler.
func makeCommentDeleteHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, commentDeleteArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args commentDeleteArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ContentID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid content id: %d", args.ContentID)), nil, nil
		}
		if args.CommentID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid comment id: %d", args.CommentID)), nil, nil
		}
		if _, _, err := c.DeleteComment(ctx, args.ContentID, args.CommentID); err != nil {
			return toolError(err), nil, nil
		}
		return toolJSON(map[string]any{"deleted": true, "commentId": args.CommentID}), nil, nil
	}
}

// makeCommentModerateHandler returns the `comment_moderate` handler.
func makeCommentModerateHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, commentModerateArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args commentModerateArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ContentID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid content id: %d", args.ContentID)), nil, nil
		}
		if args.CommentID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid comment id: %d", args.CommentID)), nil, nil
		}
		switch args.Status {
		case "approved", "rejected", "spam":
		default:
			return toolError(fmt.Errorf("VALIDATION_ERROR: invalid status %q (want approved, rejected, or spam)", args.Status)), nil, nil
		}
		data, meta, err := c.UpdateCommentStatus(ctx, args.ContentID, args.CommentID, args.Status)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no comment")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}
