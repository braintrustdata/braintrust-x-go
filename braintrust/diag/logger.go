package diag

import (
	"log"
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
	globalLogger Logger = noopLogger{}
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

// ClearLogger the current logger.
func ClearLogger() {
	SetLogger(noopLogger{})
}

// SetDebugLogger will log debug messages and warnings to Go's standard logger.
func SetDebugLogger() {
	SetLogger(&DebugLogger{})
}

// SetWarnLogger will log warnings to Go's standard logger.
func SetWarnLogger() {
	SetLogger(&WarnLogger{})
}

func Debugf(format string, args ...any) {
	logger := get()
	if logger == nil {
		return
	}
	logger.Debugf(format, args...)
}

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

// DebugLogger logs everything to the standard logger.
type DebugLogger struct{}

func (DebugLogger) Debugf(format string, args ...any) {
	log.Printf("DEBUG braintrust: "+format, args...)
}

func (DebugLogger) Warnf(format string, args ...any) {
	log.Printf("WARN braintrust: "+format, args...)
}

var _ Logger = DebugLogger{}

// WarnLogger logs only warnings to the standard logger.
type WarnLogger struct{}

func (WarnLogger) Debugf(string, ...any) {}
func (WarnLogger) Warnf(format string, args ...any) {
	log.Printf("WARN braintrust: "+format, args...)
}

var _ Logger = WarnLogger{}
