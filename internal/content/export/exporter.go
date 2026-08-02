package export

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	aliasdomain "github.com/aristorinjuang/lesstruct/internal/domain/alias"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

const exportPageSize = 100

type mediaFile struct {
	Path string
	Data []byte
}

type ContentService interface {
	GetAll(ctx context.Context, limit int, offset int) ([]*contentdomain.Content, error)
	GetTranslations(ctx context.Context, translationGroupID int, excludeID int) ([]*contentdomain.Content, error)
}

type AliasService interface {
	FindByContentID(ctx context.Context, contentID int) ([]*aliasdomain.Alias, error)
}

type ExportResult struct {
	ContentExported int
	MediaBundled    int
	Errors          []string
}

type Exporter struct {
	contentSvc ContentService
	aliasSvc   AliasService
	bodyToHTML func(string) (string, error)
	mediaDir   string
	logger     *util.Logger
}

func (e *Exporter) writeContent(
	ctx context.Context,
	tw *tar.Writer,
	c *contentdomain.Content,
	result *ExportResult,
	writtenMedia map[string]bool,
) error {
	body := c.Content
	if c.Format == contentdomain.FormatTiptap {
		var err error
		body, err = e.bodyToHTML(c.Content)
		if err != nil {
			return fmt.Errorf("failed to render content to HTML: %w", err)
		}
	}

	mediaFiles, err := e.readMediaFiles(body, writtenMedia, result)
	if err != nil {
		return fmt.Errorf("failed to bundle media: %w", err)
	}

	aliases := e.getAliases(ctx, c)
	frontmatter := BuildFrontmatter(c, aliases)

	postTypeDir := c.PostType
	if postTypeDir == "" {
		postTypeDir = "post"
	}

	fileName := fmt.Sprintf("%s.%s.html", c.Slug, c.Language)
	if c.Language == "" {
		fileName = fmt.Sprintf("%s.html", c.Slug)
	}
	filePath := filepath.Join("content", postTypeDir, fileName)

	var buf bytes.Buffer
	buf.WriteString(frontmatter)
	buf.WriteString(body)
	buf.WriteString("\n")

	header := &tar.Header{
		Name:     filepath.ToSlash(filePath),
		Size:     int64(buf.Len()),
		Mode:     0644,
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header for %s: %w", filePath, err)
	}
	if _, err := tw.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write content file %s: %w", filePath, err)
	}

	for _, mf := range mediaFiles {
		mediaPath := filepath.Join("static", mf.Path)
		header := &tar.Header{
			Name:     filepath.ToSlash(mediaPath),
			Size:     int64(len(mf.Data)),
			Mode:     0644,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write media tar header for %s: %w", mediaPath, err)
		}
		if _, err := tw.Write(mf.Data); err != nil {
			return fmt.Errorf("failed to write media file %s: %w", mediaPath, err)
		}
	}

	return nil
}

func (e *Exporter) getAliases(ctx context.Context, c *contentdomain.Content) []string {
	aliases, err := e.aliasSvc.FindByContentID(ctx, c.ID)
	if err != nil || len(aliases) == 0 {
		return nil
	}
	result := make([]string, len(aliases))
	for i, a := range aliases {
		result[i] = a.Alias
	}
	return result
}

func (e *Exporter) readMediaFiles(
	body string,
	written map[string]bool,
	result *ExportResult,
) ([]mediaFile, error) {
	urls := extractMediaURLs(body)
	if len(urls) == 0 {
		return nil, nil
	}

	var files []mediaFile
	for _, u := range urls {
		fileName := filepath.Base(u)
		if written[fileName] {
			continue
		}

		srcPath := filepath.Join(e.mediaDir, fileName)
		data, err := os.ReadFile(srcPath)
		if err != nil {
			if os.IsNotExist(err) {
				e.logger.Debug("Media file not found (skipping): %s", srcPath)
				continue
			}
			return nil, fmt.Errorf("failed to read media file %s: %w", srcPath, err)
		}

		files = append(files, mediaFile{
			Path: filepath.Join("uploads", "media", fileName),
			Data: data,
		})
		written[fileName] = true
		result.MediaBundled++
	}

	return files, nil
}

func (e *Exporter) Export(ctx context.Context, w io.Writer) (*ExportResult, error) {
	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	result := &ExportResult{}
	writtenMedia := make(map[string]bool)
	offset := 0

	for {
		contents, err := e.contentSvc.GetAll(ctx, exportPageSize, offset)
		if err != nil {
			return result, fmt.Errorf("failed to list content at offset %d: %w", offset, err)
		}
		if len(contents) == 0 {
			break
		}

		for _, c := range contents {
			if err := e.writeContent(ctx, tw, c, result, writtenMedia); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("content %d: %v", c.ID, err))
				e.logger.Error("Export failed for content %d: %v", c.ID, err)
				continue
			}
			result.ContentExported++
		}

		offset += exportPageSize
	}

	if err := tw.Close(); err != nil {
		return result, fmt.Errorf("failed to close tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return result, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return result, nil
}

func NewExporter(
	contentSvc ContentService,
	aliasSvc AliasService,
	bodyToHTML func(string) (string, error),
	mediaDir string,
	logger *util.Logger,
) *Exporter {
	return &Exporter{
		contentSvc: contentSvc,
		aliasSvc:   aliasSvc,
		bodyToHTML: bodyToHTML,
		mediaDir:   mediaDir,
		logger:     logger,
	}
}
