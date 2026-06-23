package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/internal/client"
	"github.com/spf13/cobra"
)

// commentSummary is the cmd-layer projection of a comment — it mirrors the
// server's CommentProjection field set, so decoding a server envelope into it
// drops unknown fields silently. Used by the text output of create/list/moderate.
type commentSummary struct {
	ID        int    `json:"id"`
	Comment   string `json:"comment"`
	Author    string `json:"author,omitempty"`
	Username  string `json:"username,omitempty"`
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// commentCreateOptions bundles the resolved flag values for runCommentCreate
// (kept off the package scope so each ExecuteArgs call is isolated).
type commentCreateOptions struct {
	apiKey  string
	baseURL string
	output  string
	file    string
}

// commentListOptions bundles the resolved flag values for runCommentList.
type commentListOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// commentDeleteOptions bundles the resolved flag values for runCommentDelete.
type commentDeleteOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// commentModerateOptions bundles the resolved flag values for runCommentModerate.
// verb is the text-output word ("Approved"/"Rejected"/"Marked spam"); status is
// the wire value sent to the server ("approved"/"rejected"/"spam").
type commentModerateOptions struct {
	apiKey  string
	baseURL string
	output  string
	verb    string
	status  string
}

// printComment writes a single comment to w in the requested mode. The `verb`
// parameter prefixes the text line ("Created" for create, "Approved"/
// "Rejected"/"Marked spam" for the moderation verbs). Mirrors the verb pattern
// printContent uses.
func printComment(w io.Writer, mode string, data, meta json.RawMessage, contentID int, verb string) error {
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
		Comment commentSummary `json:"comment"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	c := resp.Comment
	_, err := fmt.Fprintf(w, "%s comment #%d on content #%d (status=%s)\n", verb, c.ID, contentID, c.Status)
	return err
}

// printCommentList writes a comment list response to w in the requested mode.
// The data shape is a bare array [{...}, {...}] (no pagination meta today). Text
// mode prints a discoverability header then one line per comment; json mode
// prints the server envelope verbatim.
func printCommentList(w io.Writer, mode string, data, meta json.RawMessage, contentID int) error {
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

	var items []commentSummary
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if _, err := fmt.Fprintf(w, "Found %d comment(s) on content #%d\n", len(items), contentID); err != nil {
		return err
	}
	for _, c := range items {
		by := c.Username
		if by == "" {
			by = c.Author
		}
		if by != "" {
			_, err := fmt.Fprintf(
				w,
				"  - #%d %q (status=%s, by=%s)\n",
				c.ID,
				c.Comment,
				c.Status,
				by,
			)
			if err != nil {
				return err
			}
		} else {
			_, err := fmt.Fprintf(
				w,
				"  - #%d %q (status=%s)\n",
				c.ID,
				c.Comment,
				c.Status,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// newCommentModerateCmd builds one of the admin moderation verbs
// (approve/reject/spam). Each sends PUT /api/v1/content/{id}/comments/{commentId}/status
// with its status; the server returns 403 to a non-admin key.
func newCommentModerateCmd(
	use string,
	verb string,
	status string,
	short string,
	apiKey, baseURL, output *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <content-id> <comment-id>",
		Short: short,
		Long: short + " via PUT /api/v1/content/{id}/comments/{commentId}/status. " +
			"Admin only: the server returns 403 unless the API key belongs to an Admin.",
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommentModerate(cmd, args, commentModerateOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
				verb:    verb,
				status:  status,
			})
		},
	}
}

// newCommentCmd builds the `comment` command tree. The persistent-flag pointers
// (apiKey/baseURL/output) are captured by each subcommand's RunE so it reads the
// parsed values at execution time (mirrors newContentCmd).
func newCommentCmd(apiKey, baseURL, output *string) *cobra.Command {
	var file string

	comment := &cobra.Command{
		Use:           "comment",
		Short:         "Manage comments over /api/v1/content/{id}/comments",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	create := &cobra.Command{
		Use:     "create <content-id> [text]",
		Aliases: []string{"new"},
		Short:   "Create a comment on a content item",
		Long: "Create a comment on the content item identified by <content-id> via " +
			"POST /api/v1/content/{id}/comments. The comment text is read from a " +
			"positional argument, --file <path>, or piped stdin (in that order). New " +
			"comments start in the \"pending\" moderation status. The content must be " +
			"published (or owned by the caller) and have comments enabled, else the " +
			"server returns 403/404.",
		Args:          cobra.MaximumNArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommentCreate(cmd, args, commentCreateOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
				file:    file,
			})
		},
	}
	create.Flags().StringVar(
		&file,
		"file",
		"",
		"read the comment text from this file",
	)

	list := &cobra.Command{
		Use:   "list <content-id>",
		Short: "List comments on a content item",
		Long: "List every comment (any moderation status) on the content item " +
			"identified by <content-id> via GET /api/v1/content/{id}/comments. " +
			"Scoped to content the caller may see (published, or owned, or admin).",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommentList(cmd, args, commentListOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	deleteCmd := &cobra.Command{
		Use:   "delete <content-id> <comment-id>",
		Short: "Delete a comment (your own, or any if admin)",
		Long: "Delete the comment identified by <comment-id> via DELETE " +
			"/api/v1/content/{id}/comments/{commentId}. An admin API key may delete " +
			"any comment; a non-admin key only its own. The server returns 404 (with " +
			"no disclosure) when the comment is missing or not yours.",
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommentDelete(cmd, args, commentDeleteOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	approve := newCommentModerateCmd(
		"approve",
		"Approved",
		"approved",
		"Approve a comment (admin only)",
		apiKey,
		baseURL,
		output,
	)
	reject := newCommentModerateCmd(
		"reject",
		"Rejected",
		"rejected",
		"Reject a comment (admin only)",
		apiKey,
		baseURL,
		output,
	)
	spam := newCommentModerateCmd(
		"spam",
		"Marked spam",
		"spam",
		"Mark a comment as spam (admin only)",
		apiKey,
		baseURL,
		output,
	)

	comment.AddCommand(create)
	comment.AddCommand(list)
	comment.AddCommand(deleteCmd)
	comment.AddCommand(approve)
	comment.AddCommand(reject)
	comment.AddCommand(spam)
	return comment
}

// runCommentCreate implements `lesstruct-cli comment create <content-id> [text]`.
// It mirrors runContentCreate's body resolution (--file, then the positional text
// argument at args[1], then stdin) so the authoring commands feel identical; the
// only difference is the leading content-id positional (args[0]).
func runCommentCreate(cmd *cobra.Command, args []string, opts commentCreateOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	contentID, err := parseIntID(args[0], "content")
	if err != nil {
		return err
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	// args[1] is the optional comment text. The precedence (--file, then
	// positional text, then stdin) is shared with `content create`/`content
	// update` via readBody.
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
			msg:  "lesstruct-cli: no comment text provided",
		}
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	data, meta, err := cl.CreateComment(cmd.Context(), contentID, client.CreateCommentRequest{
		Comment: body,
	})
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no comment",
		}
	}

	if err := printComment(cmd.OutOrStdout(), opts.output, data, meta, contentID, "Created"); err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}
	return nil
}

// runCommentList implements `lesstruct-cli comment list <content-id>`.
func runCommentList(cmd *cobra.Command, args []string, opts commentListOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	contentID, err := parseIntID(args[0], "content")
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

	data, meta, err := cl.ListComments(cmd.Context(), contentID)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	if err := printCommentList(cmd.OutOrStdout(), opts.output, data, meta, contentID); err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}
	return nil
}

// runCommentDelete implements `lesstruct-cli comment delete <content-id> <comment-id>`.
func runCommentDelete(cmd *cobra.Command, args []string, opts commentDeleteOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	contentID, err := parseIntID(args[0], "content")
	if err != nil {
		return err
	}
	commentID, err := parseIntID(args[1], "comment")
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

	if _, _, err := cl.DeleteComment(cmd.Context(), contentID, commentID); err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	if opts.output == "text" {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Deleted comment #%d\n", commentID); err != nil {
			return &exitError{code: client.ExitGeneric, msg: err.Error()}
		}
	}
	// --output json: 204 → no body. The server envelope is empty; printing
	// nothing to stdout is the honest answer. Exit 0 is the success signal.
	return nil
}

// runCommentModerate implements the admin moderation verbs
// (approve/reject/spam). It sends the verb's status via PUT and prints the
// resulting projection. The server returns 403 to a non-admin key.
func runCommentModerate(cmd *cobra.Command, args []string, opts commentModerateOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	contentID, err := parseIntID(args[0], "content")
	if err != nil {
		return err
	}
	commentID, err := parseIntID(args[1], "comment")
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

	data, meta, err := cl.UpdateCommentStatus(cmd.Context(), contentID, commentID, opts.status)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no comment",
		}
	}

	if err := printComment(cmd.OutOrStdout(), opts.output, data, meta, contentID, opts.verb); err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}
	return nil
}
