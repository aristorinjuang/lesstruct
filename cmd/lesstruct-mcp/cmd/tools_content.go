package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// contentListArgs carries the `content_list` filters. Every field is optional;
// an empty struct lists the caller's own content, newest first.
type contentListArgs struct {
	Limit    int      `json:"limit,omitempty" jsonschema:"page size (0 = server default, max 100)"`
	Cursor   string   `json:"cursor,omitempty" jsonschema:"opaque cursor from a previous call's meta.pagination.nextCursor"`
	Tags     []string `json:"tags,omitempty" jsonschema:"filter by tags (AND-of-tags: the post must carry every tag)"`
	Language string   `json:"language,omitempty" jsonschema:"filter by language code (e.g. en)"`
	Status   string   `json:"status,omitempty" jsonschema:"filter by status: draft or published"`
	PostType string   `json:"postType,omitempty" jsonschema:"filter by post type (e.g. post, page)"`
	Author   string   `json:"author,omitempty" jsonschema:"filter by author username (admin only)"`
	Search   string   `json:"search,omitempty" jsonschema:"substring filter on title/meta-description (min 2 chars)"`
}

// contentGetArgs carries the `content_get` id.
type contentGetArgs struct {
	ID int `json:"id" jsonschema:"content item id"`
}

// contentCreateArgs carries the `content_create` payload. Body is Markdown
// (or HTML/Tiptap when format says so); an empty format defaults to markdown.
type contentCreateArgs struct {
	Body          string         `json:"body" jsonschema:"content body, Markdown by default"`
	Title         string         `json:"title,omitempty" jsonschema:"title (default: derived from the first heading or line)"`
	Slug          string         `json:"slug,omitempty" jsonschema:"custom URL slug, create-only (lowercase, unique per language)"`
	Format        string         `json:"format,omitempty" jsonschema:"markdown (default), tiptap, or html"`
	PostType      string         `json:"postType,omitempty" jsonschema:"content type (must match a configured post type)"`
	Tags          []string       `json:"tags,omitempty" jsonschema:"tags (trimmed, deduped)"`
	Language      string         `json:"language,omitempty" jsonschema:"language code (must be configured on the server)"`
	Published     bool           `json:"published,omitempty" jsonschema:"publish immediately"`
	CustomFields  map[string]any `json:"customFields,omitempty" jsonschema:"custom field values keyed by field name"`
	TranslationOf int            `json:"translationOf,omitempty" jsonschema:"id of the content item this translates (joins its translation group)"`
}

// contentUpdateArgs carries the `content_update` payload. Patch semantics:
// absent optional fields keep their stored values, so a body-only edit never
// wipes tags/postType/language and never unpublishes. Pass published:false
// explicitly to unpublish; pass tags:[] to clear all tags.
type contentUpdateArgs struct {
	ID           int            `json:"id" jsonschema:"content item id"`
	Body         string         `json:"body,omitempty" jsonschema:"new body (absent keeps the stored body, title, and format)"`
	Title        string         `json:"title,omitempty" jsonschema:"new title (absent: derived from body when the body is given, else kept)"`
	Format       string         `json:"format,omitempty" jsonschema:"markdown, tiptap, or html (needs a body; absent with a body means markdown)"`
	PostType     string         `json:"postType,omitempty" jsonschema:"new post type (absent keeps the stored one)"`
	Tags         []string       `json:"tags,omitempty" jsonschema:"new tags (absent/null keeps the stored ones; [] clears them)"`
	Language     string         `json:"language,omitempty" jsonschema:"new language (absent keeps the stored one)"`
	Published    *bool          `json:"published,omitempty" jsonschema:"new publish state (absent keeps the stored state)"`
	CustomFields map[string]any `json:"customFields,omitempty" jsonschema:"REPLACES all custom fields when present (absent/null preserves them)"`
}

// contentDeleteArgs carries the `content_delete` id.
type contentDeleteArgs struct {
	ID int `json:"id" jsonschema:"content item id"`
}

// contentPublishArgs carries the `content_publish` / `content_unpublish` id.
type contentPublishArgs struct {
	ID int `json:"id" jsonschema:"content item id"`
}

// contentSystemFieldsArgs carries the `content_system_fields` payload (admin
// only): admin-managed system field values keyed by field name.
type contentSystemFieldsArgs struct {
	ID     int            `json:"id" jsonschema:"content item id"`
	Fields map[string]any `json:"fields,omitempty" jsonschema:"system field values keyed by field name (at least one required)"`
}

// registerContentTools adds the content authoring tools to ms. All handlers
// close over the shared /api/v1 client.
func registerContentTools(ms *mcp.Server, c *client.Client) {
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "content_list",
		Description: "List the caller's content, newest first (paginated). Follow meta.pagination.nextCursor while meta.pagination.hasMore is true.",
	}, makeContentListHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "content_get",
		Description: "Get a single content item by id, including its body and custom fields.",
	}, makeContentGetHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "content_create",
		Description: "Create content from Markdown. Returns the new item's id, slug, and status.",
	}, makeContentCreateHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "content_update",
		Description: "Update a content item by id with patch semantics: absent fields keep stored values.",
	}, makeContentUpdateHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "content_delete",
		Description: "Delete a content item by id. Destructive and final.",
	}, makeContentDeleteHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "content_publish",
		Description: "Publish a content item (sets status=published). Idempotent: publishing an already-published post is a no-op.",
	}, makeContentPublishHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "content_unpublish",
		Description: "Unpublish a content item (sets status=draft). Idempotent.",
	}, makeContentUnpublishHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "content_system_fields",
		Description: "Set admin-managed system fields on a content item (admin only). At least one field is required.",
	}, makeContentSystemFieldsHandler(c))
}

// makeContentListHandler returns the `content_list` handler.
func makeContentListHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, contentListArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args contentListArgs,
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.ListContent(ctx, args.Limit, args.Cursor, client.ListContentFilters{
			Tags:     normalizeMCPTags(args.Tags),
			Language: strings.TrimSpace(args.Language),
			Status:   strings.TrimSpace(args.Status),
			PostType: strings.TrimSpace(args.PostType),
			Author:   strings.TrimSpace(args.Author),
			Search:   strings.TrimSpace(args.Search),
		})
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeContentGetHandler returns the `content_get` handler.
func makeContentGetHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, contentGetArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args contentGetArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid content id: %d", args.ID)), nil, nil
		}
		data, meta, err := c.GetContent(ctx, args.ID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no content")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeContentCreateHandler returns the `content_create` handler.
func makeContentCreateHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, contentCreateArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args contentCreateArgs,
	) (*mcp.CallToolResult, any, error) {
		format := args.Format
		if format == "" {
			format = "markdown"
		}
		if err := validateMCPFormat(format); err != nil {
			return toolError(err), nil, nil
		}
		if strings.TrimSpace(args.Body) == "" {
			return toolError(fmt.Errorf("VALIDATION_ERROR: no body provided")), nil, nil
		}
		title := args.Title
		if title == "" {
			title = deriveMCPTitle(args.Body)
		}
		var translationGroupID *int
		if args.TranslationOf > 0 {
			translationGroupID = &args.TranslationOf
		}
		data, meta, err := c.CreateContent(ctx, client.CreateContentRequest{
			Title:              title,
			Slug:               args.Slug,
			Body:               args.Body,
			Format:             format,
			PostType:           args.PostType,
			Tags:               normalizeMCPTags(args.Tags),
			Language:           args.Language,
			IsPublished:        args.Published,
			CustomFields:       args.CustomFields,
			TranslationGroupID: translationGroupID,
		})
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no content")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeContentUpdateHandler returns the `content_update` handler. It mirrors the
// CLI's patch semantics: absent fields are carried forward from the stored
// item so a body-only edit never wipes metadata or unpublishes.
func makeContentUpdateHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, contentUpdateArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args contentUpdateArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid content id: %d", args.ID)), nil, nil
		}
		if err := validateMCPFormat(args.Format); err != nil {
			return toolError(err), nil, nil
		}

		bodyGiven := strings.TrimSpace(args.Body) != ""
		if !bodyGiven && args.Format != "" {
			return toolError(fmt.Errorf("VALIDATION_ERROR: format needs a body")), nil, nil
		}

		body := args.Body
		format := args.Format
		if bodyGiven && format == "" {
			format = "markdown"
		}
		title := args.Title
		postType := args.PostType
		tags := args.Tags
		tagsGiven := tags != nil
		language := args.Language
		var isPublished bool
		publishedGiven := args.Published != nil
		if publishedGiven {
			isPublished = *args.Published
		}

		needExisting := !bodyGiven || postType == "" || !tagsGiven || language == "" || !publishedGiven
		if needExisting {
			existing, err := fetchMCPExisting(ctx, c, args.ID)
			if err != nil {
				return toolError(err), nil, nil
			}
			if !bodyGiven {
				body = existing.Body
				format = existing.Format
			}
			if title == "" {
				if bodyGiven {
					title = deriveMCPTitle(body)
				} else {
					title = existing.Title
				}
			}
			if postType == "" {
				postType = existing.PostType
			}
			if !tagsGiven {
				tags = existing.Tags
			}
			if language == "" {
				language = existing.Language
			}
			if !publishedGiven {
				isPublished = existing.Status == "published"
			}
		} else if title == "" {
			title = deriveMCPTitle(body)
		}

		data, meta, err := c.UpdateContent(ctx, args.ID, client.UpdateContentRequest{
			Title:        title,
			Body:         body,
			Format:       format,
			PostType:     postType,
			Tags:         tags,
			Language:     language,
			IsPublished:  isPublished,
			CustomFields: args.CustomFields,
		})
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no content")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeContentDeleteHandler returns the `content_delete` handler.
func makeContentDeleteHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, contentDeleteArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args contentDeleteArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid content id: %d", args.ID)), nil, nil
		}
		if _, _, err := c.DeleteContent(ctx, args.ID); err != nil {
			return toolError(err), nil, nil
		}
		return toolJSON(map[string]any{"deleted": true, "id": args.ID}), nil, nil
	}
}

// makeContentPublishHandler returns the `content_publish` handler.
func makeContentPublishHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, contentPublishArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args contentPublishArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid content id: %d", args.ID)), nil, nil
		}
		data, meta, err := c.PublishContent(ctx, args.ID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no content")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeContentUnpublishHandler returns the `content_unpublish` handler.
func makeContentUnpublishHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, contentPublishArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args contentPublishArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid content id: %d", args.ID)), nil, nil
		}
		data, meta, err := c.UnpublishContent(ctx, args.ID)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no content")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeContentSystemFieldsHandler returns the `content_system_fields` handler.
func makeContentSystemFieldsHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, contentSystemFieldsArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args contentSystemFieldsArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return toolError(fmt.Errorf("INVALID_ID: invalid content id: %d", args.ID)), nil, nil
		}
		if len(args.Fields) == 0 {
			return toolError(fmt.Errorf("VALIDATION_ERROR: no system fields provided")), nil, nil
		}
		data, meta, err := c.SetSystemFields(ctx, args.ID, args.Fields)
		if err != nil {
			return toolError(err), nil, nil
		}
		if len(data) == 0 {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned no content")), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// mcpExistingContent is the stored-item projection the update handler carries
// forward for absent fields.
type mcpExistingContent struct {
	Title    string
	Body     string
	Format   string
	PostType string
	Tags     []string
	Language string
	Status   string
}

// fetchMCPExisting reads the stored item for patch-semantics backfill. It
// decodes only the carried-forward subset; unknown fields drop silently.
func fetchMCPExisting(ctx context.Context, c *client.Client, id int) (mcpExistingContent, error) {
	data, _, err := c.GetContent(ctx, id)
	if err != nil {
		return mcpExistingContent{}, err
	}
	if len(data) == 0 {
		return mcpExistingContent{}, fmt.Errorf("INTERNAL_ERROR: server returned no content")
	}
	var resp struct {
		Content struct {
			Title    string   `json:"title"`
			Body     string   `json:"body"`
			Format   string   `json:"format"`
			PostType string   `json:"postType"`
			Tags     []string `json:"tags"`
			Language string   `json:"language"`
			Status   string   `json:"status"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return mcpExistingContent{}, fmt.Errorf("decode existing content: %w", err)
	}
	return mcpExistingContent{
		Title:    resp.Content.Title,
		Body:     resp.Content.Body,
		Format:   resp.Content.Format,
		PostType: resp.Content.PostType,
		Tags:     resp.Content.Tags,
		Language: resp.Content.Language,
		Status:   resp.Content.Status,
	}, nil
}

// validateMCPFormat rejects unknown content formats (empty means "not given").
func validateMCPFormat(format string) error {
	if format == "" {
		return nil
	}
	switch format {
	case "tiptap", "markdown", "html":
		return nil
	default:
		return fmt.Errorf("VALIDATION_ERROR: invalid format %q (want %q, %q, or %q)", format, "tiptap", "markdown", "html")
	}
}

// normalizeMCPTags cleans a tag list before it goes on the wire: trims
// whitespace, drops empties, dedupes preserving first occurrence. Nil in
// stays nil out so absent stays absent.
func normalizeMCPTags(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// deriveMCPTitle extracts a title from Markdown when none is given: the text
// of the first ATX heading, else the first non-empty line, truncated to 200
// runes (the server's title cap). It mirrors the CLI's deriveTitle so both
// binaries author identically.
func deriveMCPTitle(md string) string {
	for line := range strings.SplitSeq(md, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if heading := stripMCPATXHeading(trimmed); heading != "" {
			return truncateMCPRunes(heading, 200)
		}
		return truncateMCPRunes(trimmed, 200)
	}
	return ""
}

// stripMCPATXHeading returns the text of an ATX heading (1–6 leading '#'),
// or "" when the line is not a heading.
func stripMCPATXHeading(line string) string {
	count := 0
	for count < len(line) && line[count] == '#' {
		count++
	}
	if count == 0 || count > 6 {
		return ""
	}
	rest := strings.TrimSpace(line[count:])
	rest = strings.TrimRight(rest, "#")
	return strings.TrimSpace(rest)
}

// truncateMCPRunes returns s truncated to at most max runes.
func truncateMCPRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}
