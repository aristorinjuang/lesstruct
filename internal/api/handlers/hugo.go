package handlers

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/content/hugo"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

const (
	hugoImportMaxMemory = 64 << 20
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

// hugoJob is the async import job state surfaced through the status endpoint.
type hugoJob struct {
	State      string    `json:"state"`
	Imported   int       `json:"imported"`
	Skipped    int       `json:"skipped"`
	Total      int       `json:"total"`
	Errors     []string  `json:"errors,omitempty"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
}

// hugoJobStore holds in-memory Hugo import jobs keyed by job ID.
type hugoJobStore struct {
	mu     sync.RWMutex
	jobs   map[string]*hugoJob
	latest string
}

func (s *hugoJobStore) create(id string) *hugoJob {
	job := &hugoJob{
		State:     jobStateRunning,
		StartedAt: time.Now(),
	}
	s.mu.Lock()
	s.jobs[id] = job
	s.latest = id
	s.mu.Unlock()
	return job
}

// snapshot returns a copy of the job so callers can read it after the store
// lock is released without racing the import goroutine's progress updates.
func (s *hugoJobStore) snapshot(job *hugoJob) *hugoJob {
	if job == nil {
		return nil
	}
	copied := *job
	if job.Errors != nil {
		copied.Errors = make([]string, len(job.Errors))
		copy(copied.Errors, job.Errors)
	}
	return &copied
}

func (s *hugoJobStore) get(id string) (*hugoJob, bool) {
	s.mu.RLock()
	job, ok := s.jobs[id]
	var out *hugoJob
	if ok {
		out = s.snapshot(job)
	}
	s.mu.RUnlock()
	return out, ok
}

func (s *hugoJobStore) getLatest() (*hugoJob, string, bool) {
	s.mu.RLock()
	id := s.latest
	job, ok := s.jobs[id]
	var out *hugoJob
	if ok {
		out = s.snapshot(job)
	}
	s.mu.RUnlock()
	return out, id, ok
}

func (s *hugoJobStore) updateProgress(id string, p hugo.Progress) {
	s.mu.Lock()
	job, ok := s.jobs[id]
	if ok {
		job.Imported = p.Imported
		job.Skipped = p.Skipped
		job.Total = p.Total
	}
	s.mu.Unlock()
}

func (s *hugoJobStore) finish(id string, state string, result *hugo.ImportResult) {
	s.mu.Lock()
	job, ok := s.jobs[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	job.State = state
	job.FinishedAt = time.Now()
	if result != nil {
		job.Imported = result.Imported
		job.Skipped = result.Skipped
		if len(result.Errors) > maxStoredErrors {
			job.Errors = make([]string, maxStoredErrors)
			copy(job.Errors, result.Errors[:maxStoredErrors])
		} else if len(result.Errors) > 0 {
			job.Errors = make([]string, len(result.Errors))
			copy(job.Errors, result.Errors)
		}
	}
	s.mu.Unlock()
}

func newHugoJobStore() *hugoJobStore {
	return &hugoJobStore{
		jobs: make(map[string]*hugoJob),
	}
}

type HugoHandler struct {
	importer       *hugo.Importer
	logger         *util.Logger
	maxImportBytes int64
	importTimeout  time.Duration
	jobStore       *hugoJobStore
}

// Import accepts a tar.gz archive of a Hugo project and starts an asynchronous
// import job. Returns 202 Accepted with a job ID immediately. The caller can
// poll GET /api/admin/hugo/import/status/{jobId} for progress.
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

	// Spool the upload to a temp file. The multipart form's temp files are
	// cleaned up after the handler returns, so the goroutine needs its own copy.
	tmpFile, err := os.CreateTemp("", "hugo-import-*.tar.gz")
	if err != nil {
		h.logger.Error("Hugo import: failed to create temp file: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to process uploaded file", nil)
		return
	}

	_, err = io.Copy(tmpFile, file)
	if err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		h.logger.Error("Hugo import: failed to copy upload to temp file: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to process uploaded file", nil)
		return
	}
	_ = tmpFile.Close()

	skipMedia := r.FormValue("skipMedia") == "true"

	jobID := newJobID()
	h.jobStore.create(jobID)

	go func() {
		defer func() { _ = os.Remove(tmpFile.Name()) }()
		defer func() {
			if rc := recover(); rc != nil {
				h.logger.Error("Hugo import job %s panicked: %v", jobID, rc)
				h.jobStore.finish(jobID, jobStateFailed, nil)
			}
		}()

		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), h.importTimeout)
		defer cancel()

		result := h.runImport(ctx, tmpFile.Name(), userID, skipMedia, jobID)
		if result == nil {
			return
		}

		h.logger.Info("Hugo import job %s complete: %d imported, %d skipped", jobID, result.Imported, result.Skipped)
		h.jobStore.finish(jobID, jobStateDone, result)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"jobId": jobID,
			"state": jobStateRunning,
		},
		"error": nil,
	})
}

// runImport extracts the archive, parses the site, and runs the importer,
// reporting progress into the job store. It marks the job failed on any error.
func (h *HugoHandler) runImport(
	ctx context.Context,
	archivePath string,
	userID int,
	skipMedia bool,
	jobID string,
) *hugo.ImportResult {
	extractDir, err := os.MkdirTemp("", "hugo-import-*")
	if err != nil {
		h.logger.Error("Hugo import job %s: failed to create temp dir: %v", jobID, err)
		h.jobStore.finish(jobID, jobStateFailed, nil)
		return nil
	}
	defer func() { _ = os.RemoveAll(extractDir) }()

	archiveFile, err := os.Open(archivePath)
	if err != nil {
		h.logger.Error("Hugo import job %s: failed to open archive: %v", jobID, err)
		h.jobStore.finish(jobID, jobStateFailed, nil)
		return nil
	}
	defer func() { _ = archiveFile.Close() }()

	if err := extractTarGz(archiveFile, extractDir); err != nil {
		h.logger.Error("Hugo import job %s: failed to extract archive: %v", jobID, err)
		h.jobStore.finish(jobID, jobStateFailed, nil)
		return nil
	}

	contentDir := filepath.Join(extractDir, "content")
	if info, err := os.Stat(contentDir); err != nil || !info.IsDir() {
		h.jobStore.finish(jobID, jobStateFailed, nil)
		return nil
	}

	site, err := hugo.WalkContentTree(contentDir)
	if err != nil {
		h.logger.Error("Hugo import job %s: failed to parse content: %v", jobID, err)
		h.jobStore.finish(jobID, jobStateFailed, nil)
		return nil
	}

	cfg, err := hugo.LoadSiteConfig(extractDir)
	if err != nil {
		h.logger.Error("Hugo import job %s: failed to parse site config: %v", jobID, err)
		h.jobStore.finish(jobID, jobStateFailed, nil)
		return nil
	}
	site.Config = cfg

	staticDir := filepath.Join(extractDir, "static")
	if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
		site.StaticDir = staticDir
	}

	onProgress := func(p hugo.Progress) {
		h.jobStore.updateProgress(jobID, p)
	}

	opts := hugo.ImportOptions{SkipMedia: skipMedia}
	result := h.importer.Import(ctx, site, userID, opts, onProgress)
	return result
}

// ImportStatus returns the current state of a Hugo import job.
// GET /api/admin/hugo/import/status/{jobId} — specific job
// GET /api/admin/hugo/import/status         — most recent job
func (h *HugoHandler) ImportStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	var job *hugoJob
	var id string
	var ok bool

	if jobID == "" {
		job, id, ok = h.jobStore.getLatest()
	} else {
		job, ok = h.jobStore.get(jobID)
		id = jobID
	}

	if !ok {
		sendErrorResponse(w, http.StatusNotFound, "not_found", "Import job not found", nil)
		return
	}

	sendSuccessResponse(w, http.StatusOK, map[string]any{
		"jobId": id,
		"job":   job,
	})
}

func NewHugoHandler(
	importer *hugo.Importer,
	logger *util.Logger,
	maxImportBytes int64,
	importTimeout time.Duration,
) *HugoHandler {
	return &HugoHandler{
		importer:       importer,
		logger:         logger,
		maxImportBytes: maxImportBytes,
		importTimeout:  importTimeout,
		jobStore:       newHugoJobStore(),
	}
}
