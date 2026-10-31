package middleware

import (
	"io"
	"net/http"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/util"
)

const setCookie = "Set-Cookie"

func extractCookieNames(cookieHeaders []string) []string {
	names := make([]string, 0, len(cookieHeaders))
	for _, h := range cookieHeaders {
		names = append(names, parseSetCookieName(h))
	}
	return names
}

func parseSetCookieName(header string) string {
	if idx := strings.Index(header, "="); idx > 0 {
		return strings.TrimSpace(header[:idx])
	}
	return strings.TrimSpace(header)
}

type noCookieResponseWriter struct {
	http.ResponseWriter
	allowed  map[string]struct{}
	stripped []string
}

func (w *noCookieResponseWriter) stripCookies() {
	values := w.ResponseWriter.Header().Values(setCookie)
	if len(values) == 0 {
		return
	}
	var keep []string
	var stripped []string
	for _, h := range values {
		if _, ok := w.allowed[parseSetCookieName(h)]; ok {
			keep = append(keep, h)
			continue
		}
		stripped = append(stripped, h)
	}
	if len(stripped) == 0 {
		return
	}
	w.stripped = append(w.stripped, stripped...)
	w.ResponseWriter.Header().Del(setCookie)
	for _, h := range keep {
		w.ResponseWriter.Header().Add(setCookie, h)
	}
}

func (w *noCookieResponseWriter) WriteHeader(code int) {
	w.stripCookies()
	w.ResponseWriter.WriteHeader(code)
}

func (w *noCookieResponseWriter) Write(b []byte) (int, error) {
	w.stripCookies()
	return w.ResponseWriter.Write(b)
}

type NoCookieMiddleware struct {
	logger  *util.Logger
	allowed map[string]struct{}
}

func (m *NoCookieMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapper := &noCookieResponseWriter{ResponseWriter: w, allowed: m.allowed}
		next.ServeHTTP(wrapper, r)

		if len(wrapper.stripped) > 0 {
			m.logger.Info(
				"Set-Cookie header detected and stripped: path=%s method=%s cookies=%v",
				r.URL.Path,
				r.Method,
				extractCookieNames(wrapper.stripped),
			)
		}
	})
}

func NewNoCookieMiddleware(logger *util.Logger, allowed ...string) *NoCookieMiddleware {
	if logger == nil {
		logger = util.NewLogger(io.Discard)
	}
	allowMap := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			allowMap[trimmed] = struct{}{}
		}
	}
	return &NoCookieMiddleware{logger: logger, allowed: allowMap}
}
