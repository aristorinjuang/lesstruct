package client

import (
	"errors"
	"net/http"
)

// Documented CLI exit-code scheme (see docs/api-reference.md and the architecture).
// The cmd layer maps outcomes to these and os.Exit()s with them.
const (
	ExitOK          = 0
	ExitGeneric     = 1
	ExitUsage       = 2
	ExitAuth        = 3
	ExitNotFound    = 4
	ExitValidation  = 5
	ExitRateLimited = 6
	ExitServer      = 7
)

// ExitCode maps a Client error to the documented exit-code scheme. A nil error
// is success (0); an *APIError maps by HTTP status (401→auth, 404→not-found,
// 429→rate-limited, other 4xx→validation, 5xx→server, 0→generic); any other
// error is generic (1). Wrapped *APIErrors are unwrapped via errors.As.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		return ExitGeneric
	}
	switch {
	case apiErr.StatusCode == 0:
		return ExitGeneric
	case apiErr.StatusCode == http.StatusUnauthorized:
		return ExitAuth
	case apiErr.StatusCode == http.StatusNotFound:
		return ExitNotFound
	case apiErr.StatusCode == http.StatusTooManyRequests:
		return ExitRateLimited
	case apiErr.StatusCode >= 500:
		return ExitServer
	case apiErr.StatusCode >= 400:
		return ExitValidation
	default:
		// A non-nil error in the 1xx/2xx/3xx range (e.g. an error envelope on a
		// 200, or a non-JSON 2xx) is still a failure — never report success for
		// an error. Map it to generic so callers/scripts do not treat it as OK.
		return ExitGeneric
	}
}
