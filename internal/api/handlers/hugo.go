package handlers

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/content/hugo"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

const (
	maxHugoImportDuration = 10 * time.Minute
	hugoImportMaxMemory   = 64 << 20
)

func extractTarGz(r io.Reader, dest string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		path := filepath.Join(dest, header.Name)
		if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid tar entry path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", path, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			f, err := os.Create(path)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", path, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return fmt.Errorf("failed to write file %s: %w", path, err)
			}
			_ = f.Close()
		}
	}

	return nil
}

type HugoHandler struct {
	importer       *hugo.Importer
	logger         *util.Logger
	maxImportBytes int64
}

func (h *HugoHandler) Import(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxImportBytes)
	if err := r.ParseMultipartForm(hugoImportMaxMemory); err != nil {
		h.logger.Error("Hugo import: failed to parse multipart form: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to parse form data", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.logger.Error("Hugo import: missing file field: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Archive file is required", nil)
		return
	}
	defer func() { _ = file.Close() }()

	name := strings.ToLower(header.Filename)
	if !strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".tgz") {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_file", "File must be a tar.gz archive", nil)
		return
	}

	extractDir, err := os.MkdirTemp("", "hugo-import-*")
	if err != nil {
		h.logger.Error("Hugo import: failed to create temp dir: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to process archive", nil)
		return
	}
	defer func() { _ = os.RemoveAll(extractDir) }()

	if err := extractTarGz(file, extractDir); err != nil {
		h.logger.Error("Hugo import: failed to extract archive: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_archive", "Failed to extract archive", nil)
		return
	}

	contentDir := filepath.Join(extractDir, "content")
	if info, err := os.Stat(contentDir); err != nil || !info.IsDir() {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_archive", "Archive must contain a 'content' directory", nil)
		return
	}

	site, err := hugo.WalkContentTree(contentDir)
	if err != nil {
		h.logger.Error("Hugo import: failed to parse content: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_content", fmt.Sprintf("Failed to parse Hugo content: %v", err), nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), maxHugoImportDuration)
	defer cancel()

	result := h.importer.Import(ctx, site, userID)

	h.logger.Info("Hugo import complete: %d imported, %d skipped", result.Imported, result.Skipped)
	sendSuccessResponse(w, http.StatusOK, result)
}

func NewHugoHandler(
	importer *hugo.Importer,
	logger *util.Logger,
	maxImportBytes int64,
) *HugoHandler {
	return &HugoHandler{
		importer:       importer,
		logger:         logger,
		maxImportBytes: maxImportBytes,
	}
}
