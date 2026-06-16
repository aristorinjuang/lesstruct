package util

import (
	"fmt"
	"io"
	"log"
	"os"
)

// Logger provides structured logging capabilities
type Logger struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger
}

// Info logs an informational message
func (l *Logger) Info(format string, v ...any) {
	_ = l.infoLogger.Output(2, fmt.Sprintf(format, v...))
}

// Error logs an error message
func (l *Logger) Error(format string, v ...any) {
	_ = l.errorLogger.Output(2, fmt.Sprintf(format, v...))
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...any) {
	_ = l.debugLogger.Output(2, fmt.Sprintf(format, v...))
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(format string, v ...any) {
	_ = l.errorLogger.Output(2, fmt.Sprintf(format, v...))
	os.Exit(1)
}

// NewLogger creates a new logger instance
func NewLogger(w io.Writer) *Logger {
	return &Logger{
		infoLogger:  log.New(w, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		errorLogger: log.New(w, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
		debugLogger: log.New(w, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile),
	}
}
