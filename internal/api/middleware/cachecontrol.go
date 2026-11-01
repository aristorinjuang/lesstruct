package middleware

import "net/http"

// CacheControlMiddleware stamps a Cache-Control header on every response that
// passes through it. Uploads are write-once (content-hashed media,
// timestamped profile pictures), so they are immutable; anything else gets a
// bounded TTL from the caller.
type CacheControlMiddleware struct {
	value string
}

// Handler implements the middleware: set the header, then pass through.
func (m *CacheControlMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", m.value)
		next.ServeHTTP(w, r)
	})
}

// NewCacheControlMiddleware builds a Cache-Control stamping middleware.
func NewCacheControlMiddleware(value string) *CacheControlMiddleware {
	return &CacheControlMiddleware{value: value}
}
