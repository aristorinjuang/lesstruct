package blog

import (
	"net/http"
	"os"
	"strconv"
	"time"

	ssg "github.com/aristorinjuang/lesstruct/internal/content/ssg"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/aristorinjuang/lesstruct/kernel/httpx"
)

// ssgHandler serves the rendered-static-site export. It lives in the blog
// edition — the only one that carries the SSR engine the generator renders
// with — behind the routes.SSGSurface structural seam.
type ssgHandler struct {
	generator *ssg.Generator
	logger    *util.Logger
}

func (h *ssgHandler) Generate(w http.ResponseWriter, r *http.Request) {
	tmpFile, err := os.CreateTemp("", "lesstruct-ssg-*.tar.gz")
	if err != nil {
		h.logger.Error("SSG: failed to create temp file: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "Failed to create output file", nil)
		return
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	if err := h.generator.Generate(r.Context(), tmpFile); err != nil {
		h.logger.Error("SSG: failed to generate site: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "ssg_error", "Failed to generate static site", nil)
		return
	}

	if err := tmpFile.Sync(); err != nil {
		h.logger.Error("SSG: failed to sync temp file: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "Failed to finalize output", nil)
		return
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		h.logger.Error("SSG: failed to seek temp file: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "Failed to prepare output", nil)
		return
	}

	stat, err := tmpFile.Stat()
	if err != nil {
		h.logger.Error("SSG: failed to stat temp file: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "Failed to read output", nil)
		return
	}

	now := time.Now()
	timestamp := now.Format("20060102-150405")
	filename := "lesstruct-site-" + timestamp + ".tar.gz"

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))

	http.ServeContent(w, r, filename, now, tmpFile)

	h.logger.Info("SSG complete: %s (%d bytes)", filename, stat.Size())
}
