// Package tests provides test utilities for the Braintrust SDK.
package tests

import (
	"fmt"
	"testing"

	"github.com/braintrustdata/braintrust-x-go/logger"
)

// NoopLogger is a logger that does nothing (discards all logs).
type NoopLogger struct{}

// NewNoopLogger creates a new noop logger for tests that expect errors.
func NewNoopLogger() logger.Logger {
	return &NoopLogger{}
}

// Debug discards debug messages.
func (l *NoopLogger) Debug(msg string, args ...any) {}

// Info discards info messages.
func (l *NoopLogger) Info(msg string, args ...any) {}

// Warn discards warning messages.
func (l *NoopLogger) Warn(msg string, args ...any) {}

// Error discards error messages.
func (l *NoopLogger) Error(msg string, args ...any) {}

// FailTestLogger is a logger that fails the test on Error or Warn calls.
type FailTestLogger struct {
	t *testing.T
}

// NewFailTestLogger creates a new test logger that fails on errors or warnings.
func NewFailTestLogger(t *testing.T) logger.Logger {
	t.Helper()
	return &FailTestLogger{t: t}
}

// Debug logs debug messages to the test output.
func (l *FailTestLogger) Debug(msg string, args ...any) {
	l.t.Helper()
	if len(args) > 0 {
		l.t.Logf("[DEBUG] %s %s", msg, format(args))
	} else {
		l.t.Logf("[DEBUG] %s", msg)
	}
}

// Info logs info messages to the test output.
func (l *FailTestLogger) Info(msg string, args ...any) {
	l.t.Helper()
	if len(args) > 0 {
		l.t.Logf("[INFO] %s %s", msg, format(args))
	} else {
		l.t.Logf("[INFO] %s", msg)
	}
}

// Warn fails the test with a warning message.
func (l *FailTestLogger) Warn(msg string, args ...any) {
	l.t.Helper()
	if len(args) > 0 {
		l.t.Fatalf("[WARN] %s %s", msg, format(args))
	} else {
		l.t.Fatalf("[WARN] %s", msg)
	}
}

func (l *FailTestLogger) Error(msg string, args ...any) {
	l.t.Helper()
	if len(args) > 0 {
		l.t.Fatalf("[ERROR] %s %s", msg, format(args))
	} else {
		l.t.Fatalf("[ERROR] %s", msg)
	}
}

// format formats key-value pairs as a readable string
func format(args []any) string {
	if len(args) == 0 {
		return ""
	}
	result := ""
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
