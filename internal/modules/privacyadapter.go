package modules

import (
	"context"
	"fmt"

	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	privacydomain "github.com/aristorinjuang/lesstruct/modules/privacy/domain"
	profilepicture "github.com/aristorinjuang/lesstruct/modules/users/domain/profilepicture"
)

// mediaPrivacyRedactor adapts the privacy service onto the media domain's
// Redactor seam: every concept blurs. It lives in core — the only layer
// that may import both sides.
type mediaPrivacyRedactor struct {
	service *privacydomain.Service
}

// Redact blurs faces, plates, and nudity for the media library.
func (r *mediaPrivacyRedactor) Redact(ctx context.Context, imageBytes []byte) ([]byte, error) {
	if r.service == nil {
		return imageBytes, nil
	}
	redacted, err := r.service.RedactMedia(ctx, imageBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", mediadomain.ErrPrivacyScanFailed, err)
	}
	return redacted, nil
}

// profilePrivacyRedactor adapts the privacy service onto the profile
// picture seam: faces stay visible while plates and nudity blur.
type profilePrivacyRedactor struct {
	service *privacydomain.Service
}

// Redact blurs plates and nudity but keeps faces visible.
func (r *profilePrivacyRedactor) Redact(ctx context.Context, imageBytes []byte) ([]byte, error) {
	if r.service == nil {
		return imageBytes, nil
	}
	redacted, err := r.service.RedactProfilePicture(ctx, imageBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", mediadomain.ErrPrivacyScanFailed, err)
	}
	return redacted, nil
}

// NewMediaPrivacyRedactor creates a media Redactor over the privacy
// service. A nil service yields an inert redactor that returns its input
// unchanged, preserving the legacy pipeline when SAM is disabled.
func NewMediaPrivacyRedactor(service *privacydomain.Service) mediadomain.Redactor {
	if service == nil {
		return nil
	}
	return &mediaPrivacyRedactor{service: service}
}

// NewProfilePicturePrivacyRedactor creates a profile-picture Redactor over
// the privacy service. A nil service yields nil, and the service preserves
// the legacy pipeline when SAM is disabled.
func NewProfilePicturePrivacyRedactor(service *privacydomain.Service) profilepicture.Redactor {
	if service == nil {
		return nil
	}
	return &profilePrivacyRedactor{service: service}
}

var (
	_ mediadomain.Redactor    = (*mediaPrivacyRedactor)(nil)
	_ profilepicture.Redactor = (*profilePrivacyRedactor)(nil)
)
