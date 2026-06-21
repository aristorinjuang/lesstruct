package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/internal/client"
	"github.com/spf13/cobra"
)

// contentSummary is the cmd-layer projection of a content item — it mirrors the
// server's ContentProjection field set, so decoding a server envelope into it
// drops unknown fields silently. Used by the text output of get/list/delete.
type contentSummary struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// newContentCmd builds the `content` command tree. The persistent-flag pointers
// (apiKey/baseURL/output) are captured by each subcommand's RunE so it reads
// the parsed values at execution time.
func newContentCmd(apiKey, baseURL, output *string) *cobra.Command {
	var (
		file, title, postType, language string
		tags                            []string
		fields                          []string
		translationOf                   int
		published                       bool
	)

	content := &cobra.Command{
		Use:           "content",
		Short:         "Manage content over /api/v1/content",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	create := &cobra.Command{
		Use:     "create [markdown]",
		Aliases: []string{"new"},
		Short:   "Create content from Markdown",
		Long: "Create content from Markdown read via --file <path>, a positional " +
			"argument, or piped stdin. Sends POST /api/v1/content with format: markdown. " +
			"Use repeatable --field key=value to set custom fields (auto-typed: " +
			"true/false→bool, numbers→number, else string) and --translation-of <id> " +
			"to mark this item as a translation of an existing one.",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// --translation-of is optional; only send translationGroupId when the
			// flag is present (0 is never a valid content id, but Changed() is the
			// explicit signal so an absent flag sends nothing on the wire).
			var translationGroupID *int
			if cmd.Flags().Changed("translation-of") {
				id := translationOf
				translationGroupID = &id
			}
			return runContentCreate(cmd, args, contentCreateOptions{
				apiKey:             *apiKey,
				baseURL:            *baseURL,
				output:             *output,
				file:               file,
				title:              title,
				published:          published,
				postType:           postType,
				tags:               tags,
				language:           language,
				fields:             fields,
				translationGroupID: translationGroupID,
			})
		},
	}

	create.Flags().StringVar(
		&file,
		"file",
		"",
		"read Markdown from this file",
	)
	create.Flags().StringVar(
		&title,
		"title",
		"",
		"content title (default: derived from the first heading or line)",
	)
	create.Flags().BoolVar(
		&published,
		"published",
		false,
		"publish immediately (isPublished=true)",
	)
	create.Flags().StringVar(
		&postType,
		"post-type",
		"",
		"content type (e.g. \"post\", \"page\") — must match a configured post type",
	)
	create.Flags().StringSliceVar(
		&tags,
		"tags",
		nil,
		"comma-separated tags (e.g. --tags foo,bar); trims, drops empties, dedupes (preserves first occurrence)",
	)
	create.Flags().StringVar(
		&language,
		"language",
		"",
		"language code (e.g. \"en\") — must be in the server's configured languages",
	)
	create.Flags().StringArrayVar(
		&fields,
		"field",
		nil,
		"custom field as key=value (repeatable). Values are auto-typed: \"true\"/\"false\"→bool, integers/floats→number, else string",
	)
	create.Flags().IntVar(
		&translationOf,
		"translation-of",
		0,
		"id of content this item translates (joins its translation group; the server validates the id exists)",
	)

	var (
		updateFile, updateTitle, updatePostType, updateLanguage string
		updateFields                                            []string
		updateTags                                              []string
		updatePublished                                         bool
	)
	update := &cobra.Command{
		Use:   "update <id> [markdown]",
		Short: "Update an existing content item from Markdown",
		Long: "Update an existing content item by id. Sends PUT /api/v1/content/{id} " +
			"with format: markdown. The Markdown body is read via --file <path>, a " +
			"second positional argument, or piped stdin. The title is taken from " +
			"--title or, when absent, derived from the first heading or line of the " +
			"body (same as `content create`). Patch semantics: --post-type, --tags, " +
			"--language, and --published are PRESERVED from the existing item when " +
			"omitted, so a body-only edit no longer wipes them and does not unpublish; " +
			"set them explicitly to change, and pass --published=false to unpublish. " +
			"Repeatable --field key=value REPLACES all custom fields (auto-typed; omit " +
			"to preserve). SEO metadata (metaDescription, ogTitle, ogDescription), " +
			"allowComments, and translationGroupId are server-managed and preserved. " +
			"slug is accepted but not honored (the server auto-generates it from the title).",
		Args:          cobra.MaximumNArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentUpdate(cmd, args, contentUpdateOptions{
				apiKey:    *apiKey,
				baseURL:   *baseURL,
				output:    *output,
				file:      updateFile,
				title:     updateTitle,
				published: updatePublished,
				postType:  updatePostType,
				tags:      updateTags,
				language:  updateLanguage,
				fields:    updateFields,
				changed: contentUpdateChanged{
					tags:      cmd.Flags().Changed("tags"),
					postType:  cmd.Flags().Changed("post-type"),
					language:  cmd.Flags().Changed("language"),
					published: cmd.Flags().Changed("published"),
				},
			})
		},
	}
	update.Flags().StringVar(
		&updateFile,
		"file",
		"",
		"read Markdown from this file",
	)
	update.Flags().StringVar(
		&updateTitle,
		"title",
		"",
		"content title (default: derived from the first heading or line)",
	)
	update.Flags().BoolVar(
		&updatePublished,
		"published",
		false,
		"publish immediately (isPublished=true)",
	)
	update.Flags().StringVar(
		&updatePostType,
		"post-type",
		"",
		"content type (e.g. \"post\", \"page\") — must match a configured post type",
	)
	update.Flags().StringSliceVar(
		&updateTags,
		"tags",
		nil,
		"comma-separated tags (e.g. --tags foo,bar); trims, drops empties, dedupes (preserves first occurrence)",
	)
	update.Flags().StringVar(
		&updateLanguage,
		"language",
		"",
		"language code (e.g. \"en\") — must be in the server's configured languages",
	)
	update.Flags().StringArrayVar(
		&updateFields,
		"field",
		nil,
		"custom field as key=value (repeatable; REPLACES all custom fields). Auto-typed: \"true\"/\"false\"→bool, numbers→number, else string. Omit to preserve existing fields",
	)

	var (
		limit        int
		cursor       string
		listTags     []string
		listLanguage string
		listStatus   string
		listPostType string
		listAuthor   string
		listSearch   string
	)
	list := &cobra.Command{
		Use:   "list",
		Short: "List content (paginated)",
		Long: "List the caller's own content via GET /api/v1/content. Pagination via " +
			"--limit (default 0 = server default, currently 50, max 100) and " +
			"--cursor (opaque token from a previous list call). " +
			"Filters AND together with the cursor; pass multiple --tags to AND-of-tags " +
			"(the post must carry every tag). " +
			"--search requires at least 2 characters; shorter values are dropped. " +
			"--author is admin-only; the server returns 403 to non-admins.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentList(cmd, args, contentListOptions{
				apiKey:   *apiKey,
				baseURL:  *baseURL,
				output:   *output,
				limit:    limit,
				cursor:   cursor,
				tags:     listTags,
				language: listLanguage,
				status:   listStatus,
				postType: listPostType,
				author:   listAuthor,
				search:   listSearch,
			})
		},
	}
	list.Flags().IntVar(
		&limit,
		"limit",
		0,
		"page size (0 = server default, currently 50; max 100)",
	)
	list.Flags().StringVar(
		&cursor,
		"cursor",
		"",
		"opaque cursor returned by a previous list call",
	)
	list.Flags().StringSliceVar(
		&listTags,
		"tags",
		nil,
		"filter by tag (comma-separated or repeated); AND-of-tags on the server — the post must carry every tag",
	)
	list.Flags().StringVar(
		&listLanguage,
		"language",
		"",
		"filter by language code (e.g. \"en\")",
	)
	list.Flags().StringVar(
		&listStatus,
		"status",
		"",
		"filter by status: \"draft\" or \"published\"",
	)
	list.Flags().StringVar(
		&listPostType,
		"post-type",
		"",
		"filter by post type (e.g. \"post\", \"page\")",
	)
	list.Flags().StringVar(
		&listAuthor,
		"author",
		"",
		"filter by author (admin only — the server returns 403 to non-admins)",
	)
	list.Flags().StringVar(
		&listSearch,
		"search",
		"",
		"filter by title / meta-description substring (case-insensitive; min 2 chars; shorter values are dropped)",
	)

	getCmd := &cobra.Command{
		Use:           "get <id>",
		Short:         "Get a single content item by id",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentGet(cmd, args, contentReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	deleteCmd := &cobra.Command{
		Use:           "delete <id>",
		Short:         "Delete a content item by id",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentDelete(cmd, args, contentDeleteOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	// publish / unpublish are standalone status-toggle verbs that do NOT
	// require constructing a full update payload. They POST to dedicated
	// /api/v1/content/{id}/{publish,unpublish} endpoints and accept no body
	// flags. Idempotent on the server: publishing an already-published post
	// is a no-op 200.
	publishCmd := &cobra.Command{
		Use:   "publish <id>",
		Short: "Publish a content item (sets status=published)",
		Long: "Publish the content item identified by <id> via POST " +
			"/api/v1/content/{id}/publish. No body required. On the " +
			"draft→published edge the server auto-generates SEO metadata and " +
			"fires the AfterPublish plugin hook; publishing an already-" +
			"published post is a 200 no-op.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentPublish(cmd, args, contentPublishOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	unpublishCmd := &cobra.Command{
		Use:   "unpublish <id>",
		Short: "Unpublish a content item (sets status=draft)",
		Long: "Unpublish the content item identified by <id> via POST " +
			"/api/v1/content/{id}/unpublish. No body required. Unpublishing an " +
			"already-draft post is a 200 no-op. Never fires the AfterPublish " +
			"hook (the hook is wired to the draft→published edge only).",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentUnpublish(cmd, args, contentUnpublishOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	// system-fields sets the admin-managed system fields on an item. Admin only:
	// the server returns 403 unless the API key belongs to an Admin. Reuses the
	// same --field key=value (auto-typed) parsing as create/update.
	var systemFieldsFlags []string
	systemFieldsCmd := &cobra.Command{
		Use:   "system-fields <id>",
		Short: "Set admin-managed system fields on a content item (admin only)",
		Long: "Set the admin-managed system fields (e.g. editorial_status, " +
			"internal_notes) on a content item via PUT " +
			"/api/v1/content/{id}/system-fields. Admin only: the server returns " +
			"403 unless the API key belongs to an Admin. Use repeatable " +
			"--field key=value to set fields (auto-typed: true/false→bool, " +
			"numbers→number, else string); the server validates each key against " +
			"the item's post-type system-field schema and each value's type.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContentSystemFields(cmd, args, contentSystemFieldsOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
				fields:  systemFieldsFlags,
			})
		},
	}
	systemFieldsCmd.Flags().StringArrayVar(
		&systemFieldsFlags,
		"field",
		nil,
		"system field as key=value (repeatable). Auto-typed: \"true\"/\"false\"→bool, numbers→number, else string",
	)

	content.AddCommand(create)
	content.AddCommand(list)
	content.AddCommand(getCmd)
	content.AddCommand(update)
	content.AddCommand(deleteCmd)
	content.AddCommand(publishCmd)
	content.AddCommand(unpublishCmd)
	content.AddCommand(systemFieldsCmd)
	return content
}

// contentCreateOptions bundles the resolved flag values for runContentCreate
// (kept off the package scope so each ExecuteArgs call is isolated).
type contentCreateOptions struct {
	apiKey             string
	baseURL            string
	output             string
	file               string
	title              string
	published          bool
	postType           string
	tags               []string
	language           string
	fields             []string
	translationGroupID *int
}

// contentUpdateChanged records which optional update flags were explicitly set,
// so runContentUpdate can apply patch semantics: an omitted flag carries the
// existing value forward instead of being sent as a zero that the server would
// apply as a forced replace (clearing tags/postType/language, or unpublishing).
type contentUpdateChanged struct {
	tags      bool
	postType  bool
	language  bool
	published bool
}

// contentUpdateOptions bundles the resolved flag values for runContentUpdate
// (kept off the package scope so each ExecuteArgs call is isolated). The field
// set mirrors contentCreateOptions because the update endpoint accepts the
// same agent v1 payload shape.
type contentUpdateOptions struct {
	apiKey    string
	baseURL   string
	output    string
	file      string
	title     string
	published bool
	postType  string
	tags      []string
	language  string
	fields    []string
	changed   contentUpdateChanged
}

// runContentCreate implements `lesstruct-cli content create`.
func runContentCreate(cmd *cobra.Command, args []string, opts contentCreateOptions) error {
	output := opts.output
	if output != "text" && output != "json" {
		return &exitError{
			code: client.ExitUsage,
			msg:  fmt.Sprintf("lesstruct-cli: invalid --output %q (want \"text\" or \"json\")", output),
		}
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	body, err := readMarkdown(cmd, args, opts.file)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}
	if strings.TrimSpace(body) == "" {
		return &exitError{
			code: client.ExitValidation,
			msg:  "lesstruct-cli: no markdown body provided",
		}
	}

	title := opts.title
	if title == "" {
		title = deriveTitle(body)
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	customFields, err := parseCustomFields(opts.fields)
	if err != nil {
		return &exitError{code: client.ExitValidation, msg: "lesstruct-cli: " + err.Error()}
	}

	data, meta, err := cl.CreateContent(cmd.Context(), client.CreateContentRequest{
		Title:              title,
		Body:               body,
		Format:             "markdown",
		PostType:           opts.postType,
		Tags:               normalizeTags(opts.tags),
		Language:           opts.language,
		IsPublished:        opts.published,
		CustomFields:       customFields,
		TranslationGroupID: opts.translationGroupID,
	})
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		// A 2xx with no data is a server contract violation — surface it rather
		// than printing a bogus "Created content #0" line.
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no content",
		}
	}

	if err := printContent(cmd.OutOrStdout(), output, data, meta, "Created"); err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}
	return nil
}

// runContentUpdate implements `lesstruct-cli content update <id> [markdown]`.
// It mirrors runContentCreate's body resolution (--file, second positional,
// stdin) and title derivation (--title or first heading/first line) so the two
// authoring commands feel identical to a user; the only difference is the id
// (positional 1) and the PUT method. The server's update handler preserves the
// unexposed fields (postType, slug, tags, language, etc.) — the CLI does not
// surface flags for them, consistent with the agent v1 surface.
func runContentUpdate(cmd *cobra.Command, args []string, opts contentUpdateOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parseIntID(args[0], "content")
	if err != nil {
		return err
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	// args[1] is the optional Markdown body. The precedence (--file, then
	// positional body, then stdin) is the same as create; the only difference
	// is the positional slot (args[1] here, args[0] for create) because args[0]
	// is the id.
	var bodyArg string
	if len(args) > 1 {
		bodyArg = args[1]
	}
	body, err := readBody(cmd, bodyArg, opts.file)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}
	if strings.TrimSpace(body) == "" {
		return &exitError{
			code: client.ExitValidation,
			msg:  "lesstruct-cli: no markdown body provided",
		}
	}

	title := opts.title
	if title == "" {
		title = deriveTitle(body)
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	customFields, err := parseCustomFields(opts.fields)
	if err != nil {
		return &exitError{code: client.ExitValidation, msg: "lesstruct-cli: " + err.Error()}
	}

	// Patch semantics: tags/post-type/language/published carry the existing value
	// forward when their flag is omitted, so a body-only edit no longer wipes them
	// (and omitting --published no longer silently unpublishes). When any of the
	// four is omitted, GET the current item once and overlay the existing values
	// onto the request. Title and body are always taken from the input — an update
	// is an edit of the body, and the title is derived from it (or --title).
	postType := opts.postType
	tags := normalizeTags(opts.tags)
	language := opts.language
	isPublished := opts.published
	if !opts.changed.postType || !opts.changed.tags || !opts.changed.language || !opts.changed.published {
		getData, _, gerr := cl.GetContent(cmd.Context(), id)
		if gerr != nil {
			return &exitError{code: client.ExitCode(gerr), msg: apiErrorMessage(gerr)}
		}
		if len(getData) == 0 {
			return &exitError{
				code: client.ExitGeneric,
				msg:  "lesstruct-cli: server returned no content while resolving existing fields",
			}
		}
		var existing struct {
			Content struct {
				PostType string   `json:"postType"`
				Tags     []string `json:"tags"`
				Language string   `json:"language"`
				Status   string   `json:"status"`
			} `json:"content"`
		}
		if uerr := json.Unmarshal(getData, &existing); uerr != nil {
			return &exitError{
				code: client.ExitGeneric,
				msg:  fmt.Sprintf("lesstruct-cli: decode existing content: %s", uerr),
			}
		}
		if !opts.changed.postType {
			postType = existing.Content.PostType
		}
		if !opts.changed.tags {
			// Carry the existing tags through verbatim (no re-normalize) so they
			// survive the round-trip; an empty set is preserved as empty.
			tags = existing.Content.Tags
			if tags == nil {
				tags = []string{}
			}
		}
		if !opts.changed.language {
			language = existing.Content.Language
		}
		if !opts.changed.published {
			isPublished = existing.Content.Status == "published"
		}
	}

	data, meta, err := cl.UpdateContent(cmd.Context(), id, client.UpdateContentRequest{
		Title:        title,
		Body:         body,
		Format:       "markdown",
		PostType:     postType,
		Tags:         tags,
		Language:     language,
		IsPublished:  isPublished,
		CustomFields: customFields,
	})
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no content",
		}
	}

	if err := printContent(cmd.OutOrStdout(), opts.output, data, meta, "Updated"); err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}
	return nil
}

// readMarkdown reads the Markdown body by precedence: --file, then a positional
// argument, then stdin. It is the create-command wrapper that pulls the body
// from args[0] (the only positional arg `content create` accepts).
func readMarkdown(cmd *cobra.Command, args []string, file string) (string, error) {
	var bodyArg string
	if len(args) > 0 {
		bodyArg = args[0]
	}
	return readBody(cmd, bodyArg, file)
}

// readBody is the shared core of `content create` and `content update` body
// resolution: --file first, then the explicit body argument (caller picks the
// positional slot), then stdin. Splitting it out keeps each command's arg
// parsing independent (create uses args[0], update uses args[1] because args[0]
// is the id) without duplicating the file/stdin precedence rule.
func readBody(cmd *cobra.Command, bodyArg, file string) (string, error) {
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read --file %q: %w", file, err)
		}
		return string(b), nil
	}
	if bodyArg != "" {
		return bodyArg, nil
	}
	b, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return string(b), nil
}

// deriveTitle extracts a title from the Markdown when --title is absent: the
// text of the first ATX heading (1–6 leading '#'), otherwise the first non-empty
// line, truncated to 200 runes (the server's title cap).
func deriveTitle(md string) string {
	for line := range strings.SplitSeq(md, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if heading := stripATXHeading(trimmed); heading != "" {
			return truncateRunes(heading, 200)
		}
		return truncateRunes(trimmed, 200)
	}
	return ""
}

// stripATXHeading returns the text of an ATX heading (1–6 leading '#' then text)
// with the leading hashes, surrounding spaces, and any closing-hash sequence
// removed. It returns "" when the line is not a heading (so the caller falls
// back to using the line verbatim). "# x", "#x", "## x  ##" all yield "x".
func stripATXHeading(line string) string {
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

// truncateRunes returns s truncated to at most max runes.
func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

// normalizeTags cleans the raw tag slice from --tags before forwarding it to
// the server. The contract is:
//   - Whitespace around each entry is trimmed.
//   - Empty entries ("" or whitespace-only) are dropped.
//   - Duplicates are removed, preserving the first occurrence's order.
//
// The server then re-validates via contentdomain.ValidateTags (lowercase, length
// bound, allowed-character check) so the CLI does not duplicate that policy
// here — the goal is just to never send obvious garbage (trailing commas from
// `--tags a,` etc.). nil and an already-empty input return nil so the
// CreateContentRequest's `omitempty` keeps the field off the wire when the
// user did not pass --tags.
func normalizeTags(in []string) []string {
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

// parseCustomFields parses repeated --field key=value flags into a custom
// fields map suitable for the server's customFields payload. Values are coerced
// to bool / number / string (see coerceFieldValue) so they satisfy the server's
// type-strict field validators without the caller writing JSON: "--field
// minutes=30" validates against a number field, "--field has_video=true" against
// a checkbox field, and "--field difficulty=beginner" against a text/select
// field. Returns nil when no fields are provided so the request omits the map
// entirely (on create that means no custom fields; on update it preserves the
// existing ones). An entry without '=', or with an empty key, is a hard error so
// typos are caught before the request is sent.
func parseCustomFields(pairs []string) (map[string]any, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	fields := make(map[string]any, len(pairs))
	for _, pair := range pairs {
		key, value, found := strings.Cut(pair, "=")
		if !found {
			return nil, fmt.Errorf("--field %q must be key=value", pair)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("--field %q has an empty key", pair)
		}
		fields[key] = coerceFieldValue(value)
	}
	return fields, nil
}

// coerceFieldValue converts a --field value string to the most specific JSON
// type that matches it: booleans ("true"/"false"), then integers, then floats,
// else a plain string. The CLI sends customFields as JSON, and the server decodes
// JSON numbers into any as float64 — so an integer here round-trips as a number
// and satisfies the number field validator. Note a value that looks numeric but
// is meant as text (e.g. a "2024" tagline) will be typed as a number; force a
// string by using the raw API in that niche case.
func coerceFieldValue(value string) any {
	switch value {
	case "true":
		return true
	case "false":
		return false
	}
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	return value
}

// apiErrorMessage extracts a human-readable message from a Client error.
func apiErrorMessage(err error) string {
	if apiErr, ok := errors.AsType[*client.APIError](err); ok {
		if apiErr.Message != "" {
			return apiErr.Message
		}
	}
	return err.Error()
}

// printContent writes a single content item to w in the requested mode. The
// `verb` parameter is "Created" (for the create command's success line) or
// "Updated" (for the update command's success line) — the field set is the
// same but the leading word reflects the action. Mirrors the verb pattern
// printMediaGet uses.
func printContent(w io.Writer, mode string, data, meta json.RawMessage, verb string) error {
	if mode == "json" {
		env := struct {
			Data json.RawMessage `json:"data,omitempty"`
			Meta json.RawMessage `json:"meta,omitempty"`
		}{Data: data, Meta: meta}
		encoded, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("encode output: %w", err)
		}
		_, err = fmt.Fprintln(w, string(encoded))
		return err
	}

	var resp struct {
		Content struct {
			ID     int    `json:"id"`
			Title  string `json:"title"`
			Slug   string `json:"slug"`
			Status string `json:"status"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	c := resp.Content
	_, err := fmt.Fprintf(w, "%s content #%d %q -> /%s (%s)\n", verb, c.ID, c.Title, c.Slug, c.Status)
	return err
}

// contentReadOptions bundles the resolved flag values for runContentGet.
type contentReadOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// contentListOptions bundles the resolved flag values for runContentList.
// tags is the raw multi-value from --tags (comma-separated or repeated);
// runContentList normalizes it via normalizeTags (trim, drop empties, dedupe,
// preserve first-occurrence order) before forwarding to the client.
type contentListOptions struct {
	apiKey   string
	baseURL  string
	output   string
	limit    int
	cursor   string
	tags     []string
	language string
	status   string
	postType string
	author   string
	search   string
}

// contentDeleteOptions bundles the resolved flag values for runContentDelete.
type contentDeleteOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// parseIntID validates a positional integer id arg and returns the integer,
// or returns an *exitError mapped to ExitValidation. The kind is used in the
// error message (e.g. "content", "media"). This is the cmd-layer mirror of
// the server's `id <= 0` check; we never send a bad id over the wire.
func parseIntID(arg, kind string) (int, error) {
	id, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil || id <= 0 {
		return 0, &exitError{
			code: client.ExitValidation,
			msg:  fmt.Sprintf("lesstruct-cli: invalid %s id: %q", kind, arg),
		}
	}
	return id, nil
}

// validateOutput checks --output is one of the supported modes.
func validateOutput(output string) error {
	if output != "text" && output != "json" {
		return &exitError{
			code: client.ExitUsage,
			msg:  fmt.Sprintf("lesstruct-cli: invalid --output %q (want \"text\" or \"json\")", output),
		}
	}
	return nil
}

// runContentGet implements `lesstruct-cli content get <id>`.
func runContentGet(cmd *cobra.Command, args []string, opts contentReadOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parseIntID(args[0], "content")
	if err != nil {
		return err
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	data, meta, err := cl.GetContent(cmd.Context(), id)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no content",
		}
	}

	if err := printContentGet(cmd.OutOrStdout(), opts.output, data, meta); err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}
	return nil
}

// runContentList implements `lesstruct-cli content list`.
func runContentList(cmd *cobra.Command, args []string, opts contentListOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	// Normalize --tags so the wire stays clean (trim, drop empties, dedupe,
	// preserve first-occurrence order). nil stays nil → the client emits no
	// ?tag= keys at all (the unfiltered case).
	tags := normalizeTags(opts.tags)

	data, meta, err := cl.ListContent(cmd.Context(), opts.limit, opts.cursor, client.ListContentFilters{
		Tags:     tags,
		Language: strings.TrimSpace(opts.language),
		Status:   strings.TrimSpace(opts.status),
		PostType: strings.TrimSpace(opts.postType),
		Author:   strings.TrimSpace(opts.author),
		Search:   strings.TrimSpace(opts.search),
	})
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	if err := printContentList(cmd.OutOrStdout(), opts.output, data, meta); err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}
	return nil
}

// contentPublishOptions bundles the resolved flag values for runContentPublish
// (kept off the package scope so each ExecuteArgs call is isolated). The
// publish verb takes no body flags — the entire surface is the positional id.
type contentPublishOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// contentUnpublishOptions bundles the resolved flag values for runContentUnpublish
// (mirrors contentPublishOptions; the two verbs share the same shape).
type contentUnpublishOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// runContentPublish implements `lesstruct-cli content publish <id>`. It calls
// the server's dedicated POST /api/v1/content/{id}/publish endpoint (no body)
// and prints the resulting projection. The text line uses the "Published"
// verb to mirror the existing "Created" / "Updated" pattern; the JSON output
// is the server envelope verbatim.
//
// The server endpoint is idempotent: publishing an already-published post is
// a 200 no-op (no hook fires, no SEO regen runs). The CLI does not need to
// inspect the response body to detect this — the success-without-data and
// success-with-projection paths both produce a 200 and the same text/JSON
// output contract.
func runContentPublish(cmd *cobra.Command, args []string, opts contentPublishOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parseIntID(args[0], "content")
	if err != nil {
		return err
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	data, meta, err := cl.PublishContent(cmd.Context(), id)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		// Defensive: a 2xx with no data is a server contract violation. The
		// publish endpoint always returns the ContentResponse on 200, so
		// surface the unexpected empty body rather than printing a bogus
		// "Published content #0" line.
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no content",
		}
	}

	// printContent handles both the JSON-passthrough envelope and the text
	// "Published content #N ..." line. The verb is "Published" so the user
	// can distinguish a freshly-published post from a "Updated" line.
	if err := printContent(cmd.OutOrStdout(), opts.output, data, meta, "Published"); err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}
	return nil
}

// runContentUnpublish implements `lesstruct-cli content unpublish <id>`. It
// calls POST /api/v1/content/{id}/unpublish (no body) and prints the
// resulting draft projection. Same idempotency contract as publish: 200 on
// a no-op. The text verb is "Unpublished".
func runContentUnpublish(cmd *cobra.Command, args []string, opts contentUnpublishOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parseIntID(args[0], "content")
	if err != nil {
		return err
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	data, meta, err := cl.UnpublishContent(cmd.Context(), id)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no content",
		}
	}

	if err := printContent(cmd.OutOrStdout(), opts.output, data, meta, "Unpublished"); err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}
	return nil
}

// contentSystemFieldsOptions bundles the resolved flag values for
// runContentSystemFields (kept off the package scope so each ExecuteArgs call is
// isolated). fields is the raw repeatable --field key=value list, auto-typed by
// parseCustomFields before being sent.
type contentSystemFieldsOptions struct {
	apiKey  string
	baseURL string
	output  string
	fields  []string
}

// runContentSystemFields implements `lesstruct-cli content system-fields <id>`. It
// sets the admin-managed system fields via PUT /api/v1/content/{id}/system-fields.
// Admin only: the server returns 403 to a non-admin API key. Each --field
// key=value is auto-typed (parseCustomFields) and the server validates each key
// against the item's post-type system-field schema and each value's type. At
// least one --field is required (a no-op set is rejected client-side). Mirrors the
// structure of runContentPublish (validate output, parse id, resolve credentials,
// call, print).
func runContentSystemFields(cmd *cobra.Command, args []string, opts contentSystemFieldsOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parseIntID(args[0], "content")
	if err != nil {
		return err
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	systemFields, err := parseCustomFields(opts.fields)
	if err != nil {
		return &exitError{code: client.ExitValidation, msg: "lesstruct-cli: " + err.Error()}
	}
	if len(systemFields) == 0 {
		return &exitError{
			code: client.ExitValidation,
			msg:  "lesstruct-cli: no system fields provided (use --field key=value)",
		}
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	data, meta, err := cl.SetSystemFields(cmd.Context(), id, systemFields)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no content",
		}
	}

	if err := printContent(cmd.OutOrStdout(), opts.output, data, meta, "Updated system fields for"); err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}
	return nil
}

// runContentDelete implements `lesstruct-cli content delete <id>`.
func runContentDelete(cmd *cobra.Command, args []string, opts contentDeleteOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parseIntID(args[0], "content")
	if err != nil {
		return err
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	_, _, err = cl.DeleteContent(cmd.Context(), id)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	if opts.output == "text" {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Deleted content #%d\n", id)
		if err != nil {
			return &exitError{code: client.ExitGeneric, msg: err.Error()}
		}
	}
	// --output json: 204 → no body. The server envelope is empty; printing
	// nothing to stdout is the honest answer. Exit 0 is the success signal.
	return nil
}

// printContentGet writes a single content item to w in the requested mode. The
// data shape is the same as `printContent` ({content: {...}}) but the text
// line is different (it describes an existing item, not a created one).
func printContentGet(w io.Writer, mode string, data, meta json.RawMessage) error {
	if mode == "json" {
		env := struct {
			Data json.RawMessage `json:"data,omitempty"`
			Meta json.RawMessage `json:"meta,omitempty"`
		}{Data: data, Meta: meta}
		encoded, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("encode output: %w", err)
		}
		_, err = fmt.Fprintln(w, string(encoded))
		return err
	}

	var resp struct {
		Content contentSummary `json:"content"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	c := resp.Content
	if c.UpdatedAt != "" {
		_, err := fmt.Fprintf(
			w,
			"Content #%d %q (status=%s, slug=%s, updatedAt=%s)\n",
			c.ID,
			c.Title,
			c.Status,
			c.Slug,
			c.UpdatedAt,
		)
		return err
	}
	_, err := fmt.Fprintf(
		w,
		"Content #%d %q (status=%s, slug=%s)\n",
		c.ID,
		c.Title,
		c.Status,
		c.Slug,
	)
	return err
}

// printContentList writes a list response to w in the requested mode. The data
// shape is a bare array [{...}, {...}] and meta is {pagination:{nextCursor,
// hasMore}}. Text mode prints a discoverability header then one line per item;
// json mode prints the server envelope verbatim.
func printContentList(w io.Writer, mode string, data, meta json.RawMessage) error {
	if mode == "json" {
		env := struct {
			Data json.RawMessage `json:"data,omitempty"`
			Meta json.RawMessage `json:"meta,omitempty"`
		}{Data: data, Meta: meta}
		encoded, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("encode output: %w", err)
		}
		_, err = fmt.Fprintln(w, string(encoded))
		return err
	}

	var items []contentSummary
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	var pagination struct {
		NextCursor string `json:"nextCursor"`
		HasMore    bool   `json:"hasMore"`
	}
	if len(meta) > 0 {
		var m struct {
			Pagination struct {
				NextCursor string `json:"nextCursor"`
				HasMore    bool   `json:"hasMore"`
			} `json:"pagination"`
		}
		_ = json.Unmarshal(meta, &m)
		pagination = m.Pagination
	}

	if _, err := fmt.Fprintf(
		w,
		"Found %d item(s) (hasMore=%t, nextCursor=%q)\n",
		len(items),
		pagination.HasMore,
		pagination.NextCursor,
	); err != nil {
		return err
	}
	for _, c := range items {
		if _, err := fmt.Fprintf(
			w,
			"  - #%d %q (status=%s, slug=%s)\n",
			c.ID,
			c.Title,
			c.Status,
			c.Slug,
		); err != nil {
			return err
		}
	}
	return nil
}
