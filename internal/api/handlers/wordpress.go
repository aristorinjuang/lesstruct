package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/content/wordpress"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

const (
	// wordpressImportMaxMemory is the in-RAM threshold for ParseMultipartForm; the
	// total upload is bounded separately by maxImportBytes (applied via
	// MaxBytesReader). Keeping the RAM threshold modest means a large WXR file
	// spills to temp files instead of sitting wholly in memory.
	wordpressImportMaxMemory = 32 << 20

	// job state constants
	jobStateRunning = "running"
	jobStateDone    = "done"
	jobStateFailed  = "failed"

	// maxStoredErrors caps the number of error strings kept per job to avoid
	// unbounded memory growth on very large imports.
	maxStoredErrors = 1000
)

// importJob tracks the progress of a single async WordPress import.
type importJob struct {
	State         string    `json:"state"`
	Imported      int       `json:"imported"`
	Skipped       int       `json:"skipped"`
	UsersImported int       `json:"usersImported"`
	Total         int       `json:"total"`
	Errors        []string  `json:"errors,omitempty"`
	StartedAt     time.Time `json:"startedAt"`
	FinishedAt    time.Time `json:"finishedAt,omitempty"`
}

// importJobStore holds in-memory import jobs keyed by job ID.
type importJobStore struct {
	mu     sync.RWMutex
	jobs   map[string]*importJob
	latest string
}

func (s *importJobStore) create(id string) *importJob {
	job := &importJob{
		State:     jobStateRunning,
		StartedAt: time.Now(),
	}
	s.mu.Lock()
	s.jobs[id] = job
	s.latest = id
	s.mu.Unlock()
	return job
}

func (s *importJobStore) get(id string) (*importJob, bool) {
	s.mu.RLock()
	job, ok := s.jobs[id]
	s.mu.RUnlock()
	return job, ok
}

func (s *importJobStore) getLatest() (*importJob, string, bool) {
	s.mu.RLock()
	id := s.latest
	job, ok := s.jobs[id]
	s.mu.RUnlock()
	return job, id, ok
}

func (s *importJobStore) updateProgress(id string, p wordpress.Progress) {
	s.mu.Lock()
	job, ok := s.jobs[id]
	if ok {
		job.Imported = p.Imported
		job.Skipped = p.Skipped
		job.UsersImported = p.UsersImported
		job.Total = p.Total
	}
	s.mu.Unlock()
}

func (s *importJobStore) finish(id string, state string, result *wordpress.ImportResult) {
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
		job.UsersImported = result.UsersImported
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

func newImportJobStore() *importJobStore {
	return &importJobStore{
		jobs: make(map[string]*importJob),
	}
}

// WordPressHandler exposes the WordPress import endpoint to administrators.
type WordPressHandler struct {
	importer       *wordpress.Importer
	logger         *util.Logger
	maxImportBytes int64
	importTimeout  time.Duration
	jobStore       *importJobStore
}

// Import accepts a WordPress WXR XML file upload and starts an asynchronous
// import job. Returns 202 Accepted with a job ID immediately. The caller can
// poll GET /api/admin/wordpress/import/status/{jobId} for progress.
func (h *WordPressHandler) Import(w http.ResponseWriter, r *http.Request) {
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
	if err := r.ParseMultipartForm(wordpressImportMaxMemory); err != nil {
		h.logger.Error("WordPress import: failed to parse multipart form: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to parse form data", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.logger.Error("WordPress import: missing file field: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "XML file is required", nil)
		return
	}
	defer func() { _ = file.Close() }()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".xml") {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_file", "File must be a WordPress export (.xml)", nil)
		return
	}

	// Spool the upload to a temp file. The multipart form's temp files are
	// cleaned up after the handler returns, so the goroutine needs its own copy.
	tmpFile, err := os.CreateTemp("", "wp-import-*.xml")
	if err != nil {
		h.logger.Error("WordPress import: failed to create temp file: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to process uploaded file", nil)
		return
	}

	_, err = io.Copy(tmpFile, file)
	if err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		h.logger.Error("WordPress import: failed to copy upload to temp file: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to process uploaded file", nil)
		return
	}
	_ = tmpFile.Close()

	// Generate a unique job ID and register it.
	jobID := newJobID()
	h.jobStore.create(jobID)

	// Re-open the temp file for the goroutine.
	f, err := os.Open(tmpFile.Name())
	if err != nil {
		h.jobStore.finish(jobID, jobStateFailed, nil)
		_ = os.Remove(tmpFile.Name())
		h.logger.Error("WordPress import: failed to re-open temp file: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to process uploaded file", nil)
		return
	}

	skipMedia := r.FormValue("skipMedia") == "true"

	go func() {
		defer func() { _ = os.Remove(tmpFile.Name()) }()
		defer func() { _ = f.Close() }()
		defer func() {
			if rc := recover(); rc != nil {
				h.logger.Error("WordPress import job %s panicked: %v", jobID, rc)
				h.jobStore.finish(jobID, jobStateFailed, nil)
			}
		}()

		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), h.importTimeout)
		defer cancel()

		// Report progress after each item so the status endpoint reflects live counts.
		onProgress := func(p wordpress.Progress) {
			h.jobStore.updateProgress(jobID, p)
		}

		opts := wordpress.ImportOptions{SkipMedia: skipMedia}
		result, err := h.importer.Import(ctx, f, userID, opts, onProgress)
		if err != nil {
			h.logger.Error("WordPress import job %s failed: %v", jobID, err)
			h.jobStore.finish(jobID, jobStateFailed, result)
			return
		}

		h.logger.Info("WordPress import job %s complete: %d imported, %d skipped", jobID, result.Imported, result.Skipped)
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

// ImportStatus returns the current state of an import job.
// GET /api/admin/wordpress/import/status/{jobId} — specific job
// GET /api/admin/wordpress/import/status         — most recent job
func (h *WordPressHandler) ImportStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	var job *importJob
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

// NewWordPressHandler creates a WordPressHandler. maxImportBytes is the total
// upload ceiling (from IMPORT_MAX_SIZE_MB) applied via MaxBytesReader.
func NewWordPressHandler(
	importer *wordpress.Importer,
	logger *util.Logger,
	maxImportBytes int64,
	importTimeout time.Duration,
) *WordPressHandler {
	return &WordPressHandler{
		importer:       importer,
		logger:         logger,
		maxImportBytes: maxImportBytes,
		importTimeout:  importTimeout,
		jobStore:       newImportJobStore(),
	}
}

// newJobID generates a random hex string suitable for use as a job identifier.
func newJobID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
