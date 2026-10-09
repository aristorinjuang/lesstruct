package kernel

// Logger is the printf-style logging contract modules use.
//
// Core adapts *util.Logger to it; tests and fixtures may supply any sink.
// Messages use fmt-style verbs, matching the util.Logger convention.
type Logger interface {
	Info(format string, args ...any)
	Error(format string, args ...any)
}
