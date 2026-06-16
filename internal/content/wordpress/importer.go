package wordpress

import (
	"context"
	"errors"
	"fmt"
	"io"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

// contentCreator is the subset of the content domain service needed to create
// imported items.
type contentCreator interface {
	Create(ctx context.Context, userID int, req contentdomain.CreateContentRequest) (*contentdomain.Content, error)
}

// ImportResult summarizes the outcome of an import run.
type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}

// Importer orchestrates a WordPress import: parse the WXR, download images,
// convert each item to TipTap JSON, and create it via the content service.
type Importer struct {
	contentService contentCreator
	downloader     *MediaDownloader
	logger         *util.Logger
}

// downloadImages collects every image URL across all items, downloads each once,
// and returns a map of WordPress URL to local media URL.
func (imp *Importer) downloadImages(ctx context.Context, items []ParsedItem, userID int) (map[string]string, []string) {
	imageMap := make(map[string]string)
	var errs []string
	seen := make(map[string]struct{})

	for _, item := range items {
		for _, imageURL := range ExtractImageURLs(item.Content) {
			if _, ok := seen[imageURL]; ok {
				continue
			}
			seen[imageURL] = struct{}{}

			local, err := imp.downloader.DownloadAndUpload(ctx, imageURL, userID)
			if err != nil {
				if imp.logger != nil {
					imp.logger.Error("WordPress import: image download failed for %s: %v", imageURL, err)
				}
				errs = append(errs, fmt.Sprintf("image not downloaded: %s", imageURL))
				continue
			}
			if local != "" {
				imageMap[imageURL] = local
			}
		}
	}
	return imageMap, errs
}

func (imp *Importer) importItem(
	ctx context.Context,
	item ParsedItem,
	imageMap map[string]string,
	userID int,
	result *ImportResult,
) {
	contentJSON, err := ConvertBlocks(item.Content, imageMap)
	if err != nil {
		result.Skipped++
		msg := fmt.Sprintf("skipped %q: failed to convert content: %v", item.Title, err)
		result.Errors = append(result.Errors, msg)
		return
	}

	status := contentdomain.StatusDraft
	if item.Status == "published" {
		status = contentdomain.StatusPublished
	}

	req := contentdomain.CreateContentRequest{
		Title:    item.Title,
		Content:  contentJSON,
		Tags:     item.Tags,
		Status:   status,
		PostType: item.PostType,
	}

	if _, err := imp.contentService.Create(ctx, userID, req); err != nil {
		result.Skipped++
		if errors.Is(err, contentdomain.ErrSlugAlreadyExists) {
			result.Errors = append(result.Errors, fmt.Sprintf("skipped %q: a post with this slug already exists", item.Title))
			return
		}
		result.Errors = append(result.Errors, fmt.Sprintf("skipped %q: %v", item.Title, err))
		return
	}

	result.Imported++
}

// Import reads a WXR stream and imports every post and page. Image download
// failures are logged and recorded but never abort the run; the original
// WordPress URL is kept as a fallback. Returns an aggregated result.
func (imp *Importer) Import(ctx context.Context, wxrData io.Reader, userID int) (*ImportResult, error) {
	doc, err := Parse(wxrData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WXR: %w", err)
	}

	imageMap, downloadErrors := imp.downloadImages(ctx, doc.Items, userID)

	result := &ImportResult{Errors: downloadErrors}
	for _, item := range doc.Items {
		imp.importItem(ctx, item, imageMap, userID, result)
	}
	return result, nil
}

// NewImporter creates an importer.
func NewImporter(contentService contentCreator, downloader *MediaDownloader, logger *util.Logger) *Importer {
	return &Importer{
		contentService: contentService,
		downloader:     downloader,
		logger:         logger,
	}
}
