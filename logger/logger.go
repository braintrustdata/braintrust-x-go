// Package logger provides logging interfaces and implementations for the Braintrust SDK.
package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// Logger is the interface for SDK logging.
// Compatible with slog, zap, logrus, and other structured loggers.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// defaultLogger is a simple logger that writes to stderr
type defaultLogger struct {
	debug bool
}

// NewDefaultLogger creates a default logger.
// Debug logging is enabled if BRAINTRUST_DEBUG=true
func NewDefaultLogger() Logger {
	debug := strings.ToLower(os.Getenv("BRAINTRUST_DEBUG")) == "true"
	return &defaultLogger{debug: debug}
}

func (l *defaultLogger) Debug(msg string, args ...any) {
	if l.debug {
		l.log("DEBUG", msg, args...)
	}
}

func (l *defaultLogger) Info(msg string, args ...any) {
	l.log("INFO", msg, args...)
}

func (l *defaultLogger) Warn(msg string, args ...any) {
	l.log("WARN", msg, args...)
}

func (l *defaultLogger) Error(msg string, args ...any) {
	l.log("ERROR", msg, args...)
}

func (l *defaultLogger) log(level, msg string, args ...any) {
	formatted := fmt.Sprintf("[braintrust] %s: %s", level, msg)
	if len(args) > 0 {
		formatted += " " + formatArgs(args)
	}
	log.Println(formatted)
}

func formatArgs(args []any) string {
	if len(args) == 0 {
		return ""
	}

	var result string
	for i := 0; i < len(args); i += 2 {
		if i > 0 {
			result += " "
		}
		if i+1 < len(args) {
			result += fmt.Sprintf("%v=%v", args[i], args[i+1])
		} else {
			result += fmt.Sprintf("%v", args[i])
		}
	}
	return result
}
