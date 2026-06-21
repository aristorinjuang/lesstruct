package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/internal/client"
	"github.com/spf13/cobra"
)

// mediaVariant is the cmd-layer projection of a single thumbnail variant — it
// keeps only the URL and pixel dimensions. The server's MediaVariant also
// carries an internal filePath the CLI never prints, which decoding drops.
type mediaVariant struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// mediaSummary is the cmd-layer projection of a media item — it mirrors the
// server's MediaProjection field set, so decoding a server envelope into it
// drops unknown fields silently. Used by the text output of get/list.
type mediaSummary struct {
	ID        int                     `json:"id"`
	Filename  string                  `json:"filename"`
	MimeType  string                  `json:"mimeType"`
	FileSize  int64                   `json:"fileSize"`
	URL       string                  `json:"url"`
	AltText   string                  `json:"altText"`
	Width     int                     `json:"width"`
	Height    int                     `json:"height"`
	IsWebP    bool                    `json:"isWebp"`
	Hash      string                  `json:"hash"`
	Variants  map[string]mediaVariant `json:"variants,omitempty"`
	UpdatedAt string                  `json:"updatedAt,omitempty"`
}

// newMediaCmd builds the `media` command tree. The persistent-flag pointers
// (apiKey/baseURL/output) are captured by each subcommand's RunE so it reads
// the parsed values at execution time.
func newMediaCmd(apiKey, baseURL, output *string) *cobra.Command {
	media := &cobra.Command{
		Use:           "media",
		Short:         "Manage media over /api/v1/media",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var file, altText, metadata string
	upload := &cobra.Command{
		Use:   "upload",
		Short: "Upload a media file",
		Long: "Upload a media file to /api/v1/media as multipart/form-data. " +
			"Reads the file from --file <path>, optionally sending --alt-text " +
			"(built into {\"altText\":\"...\"}) or --metadata <raw JSON> (escape hatch). " +
			"--metadata values may be strings, numbers, booleans, or objects " +
			"(passed through as typed JSON); the server persists only altText today. " +
			"--alt-text and --metadata are mutually exclusive.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMediaUpload(cmd, args, mediaUploadOptions{
				apiKey:   *apiKey,
				baseURL:  *baseURL,
				output:   *output,
				file:     file,
				altText:  altText,
				metadata: metadata,
			})
		},
	}
	upload.Flags().StringVar(
		&file,
		"file",
		"",
		"path to the file to upload (required)",
	)
	upload.Flags().StringVar(
		&altText,
		"alt-text",
		"",
		"alt text for accessibility (built into {\"altText\":\"...\"})",
	)
	upload.Flags().StringVar(
		&metadata,
		"metadata",
		"",
		"raw JSON metadata; values may be typed (string/number/bool/object). Server persists only altText. Mutually exclusive with --alt-text",
	)

	var listLimit int
	var listCursor string
	listCmd := &cobra.Command{
		Use:           "list",
		Short:         "List media (paginated)",
		Long:          "List the caller's own media via GET /api/v1/media?limit=&cursor=. --limit defaults to 0 (server's default).",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMediaList(cmd, args, mediaListOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
				limit:   listLimit,
				cursor:  listCursor,
			})
		},
	}
	listCmd.Flags().IntVar(
		&listLimit,
		"limit",
		0,
		"page size (0 = server default)",
	)
	listCmd.Flags().StringVar(
		&listCursor,
		"cursor",
		"",
		"opaque cursor returned by a previous list call",
	)

	getCmd := &cobra.Command{
		Use:           "get <id>",
		Short:         "Get a single media item by id",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMediaGet(cmd, args, mediaReadOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	media.AddCommand(upload)
	media.AddCommand(listCmd)
	media.AddCommand(getCmd)
	return media
}

// mediaUploadOptions bundles the resolved flag values for runMediaUpload.
type mediaUploadOptions struct {
	apiKey   string
	baseURL  string
	output   string
	file     string
	altText  string
	metadata string
}

// mediaReadOptions bundles the resolved flag values for runMediaGet.
type mediaReadOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// mediaListOptions bundles the resolved flag values for runMediaList.
type mediaListOptions struct {
	apiKey  string
	baseURL string
	output  string
	limit   int
	cursor  string
}

// runMediaUpload implements `lesstruct-cli media upload`.
func runMediaUpload(cmd *cobra.Command, args []string, opts mediaUploadOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	if opts.file == "" {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: --file is required",
		}
	}
	if opts.altText != "" && opts.metadata != "" {
		return &exitError{
			code: client.ExitValidation,
			msg:  "lesstruct-cli: use --alt-text OR --metadata, not both",
		}
	}

	// Open the file before credentials so a missing file surfaces as a
	// validation error, not a network/credential error. Reject directories
	// (and symlinks to directories) early with a clear message rather than
	// failing deep inside the multipart writer with a confusing "copy file
	// bytes" error.
	fileInfo, err := os.Stat(opts.file)
	if err != nil {
		return &exitError{
			code: client.ExitValidation,
			msg:  fmt.Sprintf("lesstruct-cli: read --file %q: %s", opts.file, err),
		}
	}
	if fileInfo.IsDir() {
		return &exitError{
			code: client.ExitValidation,
			msg:  fmt.Sprintf("lesstruct-cli: --file %q is a directory", opts.file),
		}
	}
	f, err := os.Open(opts.file)
	if err != nil {
		return &exitError{
			code: client.ExitValidation,
			msg:  fmt.Sprintf("lesstruct-cli: read --file %q: %s", opts.file, err),
		}
	}
	defer func() { _ = f.Close() }()

	// Build the metadata map (or nil when no metadata is provided). When the
	// user passes --alt-text, send it as `{"altText": "..."}` even if empty
	// (so `--alt-text ""` is distinguishable from "no --alt-text"). For
	// --metadata, values are passed through as typed JSON — numbers, booleans,
	// and nested objects are all accepted. Note the server's `MediaMetadata`
	// persists only `altText` today, so non-altText keys are ignored server-side
	// until a metadata store is added; the client no longer rejects them.
	var meta map[string]any
	switch {
	case opts.altText != "" || cmd.Flags().Changed("alt-text"):
		meta = map[string]any{"altText": opts.altText}
	case opts.metadata != "":
		if uerr := json.Unmarshal([]byte(opts.metadata), &meta); uerr != nil {
			return &exitError{
				code: client.ExitValidation,
				msg:  fmt.Sprintf("lesstruct-cli: --metadata is not valid JSON: %s", uerr),
			}
		}
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	data, meta2, err := cl.UploadMedia(
		cmd.Context(),
		client.UploadMediaRequest{
			File:     f,
			Filename: filepath.Base(opts.file),
			Metadata: meta,
		},
	)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no media",
		}
	}

	return printMediaGet(cmd.OutOrStdout(), opts.output, data, meta2, "Uploaded media")
}

// runMediaGet implements `lesstruct-cli media get <id>`.
func runMediaGet(cmd *cobra.Command, args []string, opts mediaReadOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	id, err := parseIntID(args[0], "media")
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

	data, meta, err := cl.GetMedia(cmd.Context(), id)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if len(data) == 0 {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no media",
		}
	}

	return printMediaGet(cmd.OutOrStdout(), opts.output, data, meta, "Media")
}

// runMediaList implements `lesstruct-cli media list`.
func runMediaList(cmd *cobra.Command, args []string, opts mediaListOptions) error {
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

	data, meta, err := cl.ListMedia(cmd.Context(), opts.limit, opts.cursor)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printMediaList(cmd.OutOrStdout(), opts.output, data, meta)
}

// printMediaGet writes a single media item to w in the requested mode. The
// `verb` parameter is "Uploaded" (for the upload command's success line) or
// "Media" (for the get command's success line) — the field set is the same
// but the leading word reflects the action.
func printMediaGet(w io.Writer, mode string, data, meta json.RawMessage, verb string) error {
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
		Media mediaSummary `json:"media"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	m := resp.Media
	// Defensive: a 2xx with no/empty media is a server contract violation —
	// surface it rather than printing a bogus "Media #0" line.
	if m.ID == 0 {
		return fmt.Errorf("decode response: missing media projection")
	}
	_, err := fmt.Fprintf(
		w,
		"%s #%d %q (mime=%s, size=%d, url=%s, altText=%q)\n",
		verb,
		m.ID,
		m.Filename,
		m.MimeType,
		m.FileSize,
		m.URL,
		m.AltText,
	)
	if err != nil {
		return err
	}

	// Thumbnail variants (thumbnail/medium/large) on their own lines, sorted by
	// size key for deterministic output. Omitted entirely when the item has none
	// (non-images, or a server that did not generate variants).
	keys := make([]string, 0, len(m.Variants))
	for k := range m.Variants {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m.Variants[k]
		if _, verr := fmt.Fprintf(w, "  variant %s: url=%s %dx%d\n", k, v.URL, v.Width, v.Height); verr != nil {
			return verr
		}
	}
	return nil
}

// printMediaList writes a media list response to w in the requested mode.
func printMediaList(w io.Writer, mode string, data, meta json.RawMessage) error {
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

	var items []mediaSummary
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
	for _, m := range items {
		if _, err := fmt.Fprintf(
			w,
			"  - #%d %q (mime=%s, size=%d, url=%s)\n",
			m.ID,
			m.Filename,
			m.MimeType,
			m.FileSize,
			m.URL,
		); err != nil {
			return err
		}
	}
	return nil
}
