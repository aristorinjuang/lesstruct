package response

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
)

// List limit bounds shared by all cursor-paginated list endpoints.
const (
	DefaultListLimit = 50
	MinListLimit     = 1
	MaxListLimit     = 100
)

// ParseListLimit clamps the ?limit query param into [MinListLimit, MaxListLimit] with a
// default of DefaultListLimit. Missing/invalid/negative → default; over-max → max.
func ParseListLimit(r *http.Request) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return DefaultListLimit
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < MinListLimit {
		return DefaultListLimit
	}
	if limit > MaxListLimit {
		return MaxListLimit
	}
	return limit
}

// ErrInvalidCursor is the single failure returned by DecodeCursor for any unparseable,
// non-numeric, or non-positive cursor token. List handlers map it to 400 VALIDATION_ERROR
// without disclosing why the token was bad.
var ErrInvalidCursor = errors.New("invalid cursor")

// EncodeCursor produces an opaque, URL-safe keyset token for the given id using unpadded
// base64 (RawURLEncoding) so the token contains no `=` padding — clients can drop it into
// a query string verbatim without worrying about padding being trimmed or mis-encoded by
// intermediaries. Clients must echo the token verbatim and never construct it. Opacity is
// for contract stability, not secrecy — the token is NOT signed; tamper-evidence is a
// post-MVP concern. The underlying value is the decimal id, so id DESC paging is stable
// across concurrent inserts/deletes (a new row lands on page 1; a deleted row never shifts
// later pages).
func EncodeCursor(id int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(id)))
}

// DecodeCursor inverts EncodeCursor. An empty cursor means "first page" → (0, nil). Any
// token that fails base64 decoding, is not a decimal integer, or decodes to id <= 0 is
// rejected with ErrInvalidCursor.
func DecodeCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}

	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, ErrInvalidCursor
	}

	id, err := strconv.Atoi(string(b))
	if err != nil || id <= 0 {
		return 0, ErrInvalidCursor
	}

	return id, nil
}
