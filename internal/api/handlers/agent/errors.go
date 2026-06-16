package agent

import (
	"errors"
	"net/http"

	"github.com/aristorinjuang/lesstruct/internal/api/response"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
)

// handleError maps a content-domain error to the agent API envelope using
// UPPER_SNAKE codes. It reuses the handleContentError pattern (a switch over
// errors.Is) from the admin content handler, but emits the agent API error catalog
// via response.Error instead of the legacy lowercase codes. The auth-path errors
// (UNAUTHORIZED/INVALID_API_KEY/REVOKED_KEY/EXPIRED_KEY/RATE_LIMITED) are emitted
// by the middleware, not here — this mapper only handles the content domain's own
// sentinels.
//
// Mapping (see story Dev Notes §Domain-error → envelope mapping):
//   - ErrContentNotFound         → 404 NOT_FOUND
//   - ErrUnauthorized (ownership)→ 403 FORBIDDEN
//   - the validation sentinels   → 400 VALIDATION_ERROR
//   - anything else              → 500 INTERNAL_ERROR
//
// ErrSlugAlreadyExists could arguably be 409, but the agent API error catalog has
// no CONFLICT code, so it is mapped to VALIDATION_ERROR (400) for catalog
// consistency. The wrapped error's message is echoed where useful, but internal
// stack details are never leaked.
func handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, contentdomain.ErrContentNotFound):
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "Content not found", nil)
	case errors.Is(err, contentdomain.ErrUnauthorized):
		response.Error(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this content", nil)
	case errors.Is(err, contentdomain.ErrInvalidTitle),
		errors.Is(err, contentdomain.ErrInvalidContent),
		errors.Is(err, contentdomain.ErrInvalidStatus),
		errors.Is(err, contentdomain.ErrInvalidSlug),
		errors.Is(err, contentdomain.ErrSlugAlreadyExists),
		errors.Is(err, contentdomain.ErrHTMLInTitle),
		errors.Is(err, contentdomain.ErrHTMLInPlainText),
		errors.Is(err, contentdomain.ErrInvalidTipTapContent),
		errors.Is(err, contentdomain.ErrInvalidFilterField),
		errors.Is(err, contentdomain.ErrInvalidFilterOperator),
		errors.Is(err, contentdomain.ErrInvalidFilterValue),
		errors.Is(err, contentdomain.ErrUnknownSystemFieldKey),
		errors.Is(err, contentdomain.ErrSystemFieldValidation),
		errors.Is(err, contentdomain.ErrCustomFieldValidation),
		errors.Is(err, contentdomain.ErrInvalidLanguage),
		errors.Is(err, contentdomain.ErrTranslationGroupNotFound),
		errors.Is(err, contentdomain.ErrTranslationAlreadyExists):
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
	}
}
