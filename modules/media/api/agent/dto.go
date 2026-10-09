package agent

import (
	"time"

	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
)

// MediaMetadata is the optional JSON `metadata` part of a media upload. altText is the
// only field today; it is REQUIRED by the media service (ValidateAltText) for
// accessibility, so agents should always send it.
type MediaMetadata struct {
	AltText string `json:"altText,omitempty"`
}

// MediaProjection is the public, whitelisted view of a media item returned by the agent
// media endpoints. It deliberately excludes internal/admin-only fields the raw
// mediadomain.Media carries (the numeric owner id, filePath, uploadedBy) so the
// programmatic API surface does not over-disclose — mirroring ContentProjection.
type MediaProjection struct {
	ID               int                                 `json:"id"`
	Filename         string                              `json:"filename"`
	OriginalFilename string                              `json:"originalFilename"`
	MimeType         mediadomain.MimeType                `json:"mimeType"`
	FileSize         int64                               `json:"fileSize"`
	Width            int                                 `json:"width"`
	Height           int                                 `json:"height"`
	AltText          string                              `json:"altText"`
	IsWebP           bool                                `json:"isWebp"`
	Hash             string                              `json:"hash"`
	URL              string                              `json:"url"`
	Variants         map[string]mediadomain.MediaVariant `json:"variants,omitempty"`
	CreatedAt        time.Time                           `json:"createdAt"`
	UpdatedAt        time.Time                           `json:"updatedAt"`
}

// MediaResponse wraps a projected media item for the agent media responses.
// Envelope body: {"data":{"media":{...}}}.
type MediaResponse struct {
	Media MediaProjection `json:"media"`
}

// NewMediaResponse builds a MediaResponse projecting the given media entity to its public
// fields. A nil entity yields an empty projection (defensive — the service contract
// returns a non-nil media on success).
func NewMediaResponse(m *mediadomain.Media) MediaResponse {
	if m == nil {
		return MediaResponse{}
	}
	return MediaResponse{
		Media: MediaProjection{
			ID:               m.ID,
			Filename:         m.Filename,
			OriginalFilename: m.OriginalFilename,
			MimeType:         m.MimeType,
			FileSize:         m.FileSize,
			Width:            m.Width,
			Height:           m.Height,
			AltText:          m.AltText,
			IsWebP:           m.IsWebP,
			Hash:             m.Hash,
			URL:              m.URL,
			Variants:         m.Variants,
			CreatedAt:        m.CreatedAt,
			UpdatedAt:        m.UpdatedAt,
		},
	}
}
