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
		if idx := strings.Index(h, "="); idx > 0 {
			names = append(names, h[:idx])
		} else {
			names = append(names, h)
		}
	}
	return names
}

type noCookieResponseWriter struct {
	http.ResponseWriter
	stripped []string
}

func (w *noCookieResponseWriter) stripCookies() {
	if values := w.ResponseWriter.Header().Values(setCookie); len(values) > 0 {
		w.stripped = make([]string, len(values))
		copy(w.stripped, values)
		w.ResponseWriter.Header().Del(setCookie)
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
	logger *util.Logger
}

func (m *NoCookieMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapper := &noCookieResponseWriter{ResponseWriter: w}
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

func NewNoCookieMiddleware(logger *util.Logger) *NoCookieMiddleware {
	if logger == nil {
		logger = util.NewLogger(io.Discard)
	}
	return &NoCookieMiddleware{logger: logger}
}
