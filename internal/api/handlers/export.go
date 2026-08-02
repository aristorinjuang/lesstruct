package handlers

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/content/export"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

type ExportHandler struct {
	exporter *export.Exporter
	logger   *util.Logger
}

func (h *ExportHandler) Export(w http.ResponseWriter, r *http.Request) {
	tmpFile, err := os.CreateTemp("", "lesstruct-export-*.tar.gz")
	if err != nil {
		h.logger.Error("Export: failed to create temp file: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to create export file", nil)
		return
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	result, err := h.exporter.Export(r.Context(), tmpFile)
	if err != nil {
		h.logger.Error("Export: failed to export content: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "export_error", "Failed to export content", nil)
		return
	}

	if err := tmpFile.Sync(); err != nil {
		h.logger.Error("Export: failed to sync temp file: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to finalize export", nil)
		return
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		h.logger.Error("Export: failed to seek temp file: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to prepare export", nil)
		return
	}

	stat, err := tmpFile.Stat()
	if err != nil {
		h.logger.Error("Export: failed to stat temp file: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to read export", nil)
		return
	}

	now := time.Now()
	timestamp := now.Format(exportFilenameTimestamp)
	filename := "lesstruct-export-" + timestamp + ".tar.gz"

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))

	http.ServeContent(w, r, filename, now, tmpFile)

	h.logger.Info("Export complete: %d content items, %d media files",
		result.ContentExported, result.MediaBundled)
	if len(result.Errors) > 0 {
		h.logger.Error("Export had %d errors", len(result.Errors))
	}
}

func NewExportHandler(exporter *export.Exporter, logger *util.Logger) *ExportHandler {
	return &ExportHandler{
		exporter: exporter,
		logger:   logger,
	}
}
