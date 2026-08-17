package middleware

import "net/http"

type SecurityHeadersMiddleware struct {
	cspHeaderName  string
	cspHeaderValue string
	xFrameOptions  string
}

func (m *SecurityHeadersMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.xFrameOptions != "" {
			w.Header().Set("X-Frame-Options", m.xFrameOptions)
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if m.cspHeaderValue != "" {
			w.Header().Set(m.cspHeaderName, m.cspHeaderValue)
		}
		next.ServeHTTP(w, r)
	})
}

func NewSecurityHeadersMiddleware(cspHeaderName, cspHeaderValue, xFrameOptions string) *SecurityHeadersMiddleware {
	return &SecurityHeadersMiddleware{
		cspHeaderName:  cspHeaderName,
		cspHeaderValue: cspHeaderValue,
		xFrameOptions:  xFrameOptions,
	}
}
