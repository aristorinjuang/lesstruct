package agent

import (
	"encoding/base64"
	"errors"
	"strconv"
)

// errInvalidCursor is the single failure returned by decodeCursor for any unparseable,
// non-numeric, or non-positive cursor token. The List handler maps it to 400
// VALIDATION_ERROR without disclosing why the token was bad.
var errInvalidCursor = errors.New("invalid cursor")

// encodeCursor produces an opaque, URL-safe keyset token for the given content id using
// unpadded base64 (RawURLEncoding) so the token contains no `=` padding — clients can
// drop it into a query string verbatim without worrying about padding being trimmed or
// mis-encoded by intermediaries. Clients must echo the token verbatim and never construct
// it. Opacity is for contract stability, not secrecy — the token is NOT signed;
// tamper-evidence is a post-MVP concern. The underlying value is the decimal id, so
// id DESC paging is stable across concurrent inserts/deletes (a new row lands on page 1;
// a deleted row never shifts later pages).
func encodeCursor(id int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(id)))
}

// decodeCursor inverts encodeCursor. An empty cursor means "first page" → (0, nil). Any
// token that fails base64 decoding, is not a decimal integer, or decodes to id <= 0 is
// rejected with errInvalidCursor (→ 400 VALIDATION_ERROR).
func decodeCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}

	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, errInvalidCursor
	}

	id, err := strconv.Atoi(string(b))
	if err != nil || id <= 0 {
		return 0, errInvalidCursor
	}

	return id, nil
}
