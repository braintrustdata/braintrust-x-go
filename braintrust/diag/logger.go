// Package diag provides diagnostic logging functionality for the Braintrust SDK.
package diag

import (
	"log"
	"os"
	"strings"
	"sync"
)

// Logger is an interface you can implement to send diagnostic
// messages to the destination of your choice.
type Logger interface {
	Debugf(format string, args ...any)
	Warnf(format string, args ...any)
}

var (
	mu           sync.RWMutex
	globalLogger Logger = warnLogger{}
)

// SetLogger will use the given logger for logging messages.
func SetLogger(logger Logger) {
	if logger == nil {
		logger = &noopLogger{} // just in case
	}
	mu.Lock()
	defer mu.Unlock()
	globalLogger = logger
}

// GetLogger returns the current logger.
func GetLogger() Logger {
	mu.Lock()
	defer mu.Unlock()
	return globalLogger
}

// ClearLogger clears the current logger.
func ClearLogger() {
	SetLogger(noopLogger{})
}

// SetDebugLogger will log debug messages and warnings to Go's standard logger.
func SetDebugLogger() {
	SetLogger(&debugLogger{})
}

// SetWarnLogger will log warnings to Go's standard logger.
func SetWarnLogger() {
	SetLogger(&warnLogger{})
}

// Debugf logs a debug message using the configured logger.
func Debugf(format string, args ...any) {
	logger := get()
	if logger == nil {
		return
	}
	logger.Debugf(format, args...)
}

// Warnf logs a warning message using the configured logger.
func Warnf(format string, args ...any) {
	logger := get()
	if logger == nil {
		return
	}
	logger.Warnf(format, args...)
}

func get() Logger {
	mu.RLock()
	defer mu.RUnlock()
	return globalLogger
}

// noopLogger logs to nowhere.
type noopLogger struct{}

func (noopLogger) Debugf(string, ...any) {}
func (noopLogger) Warnf(string, ...any)  {}

var _ Logger = noopLogger{}

// debugLogger logs everything to the standard logger.
type debugLogger struct{}

func (debugLogger) Debugf(format string, args ...any) {
	log.Printf("DEBUG braintrust: "+format, args...)
}

func (debugLogger) Warnf(format string, args ...any) {
	log.Printf("WARN braintrust: "+format, args...)
}

var _ Logger = debugLogger{}

// warnLogger logs only warnings to the standard logger.
type warnLogger struct{}

func (warnLogger) Debugf(string, ...any) {}
func (warnLogger) Warnf(format string, args ...any) {
	log.Printf("WARN braintrust: "+format, args...)
}

var _ Logger = warnLogger{}

// init checks for BRAINTRUST_DEBUG environment variable and sets debug logger if true
func init() {
	if strings.ToLower(os.Getenv("BRAINTRUST_DEBUG")) == "true" {
		SetDebugLogger()
	}
}
