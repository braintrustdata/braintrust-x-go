package logger

import (
	"log"
	"sync"
)

// Logger is an interface you can implement to add diagnostic
// logging to Braintrust tracing.
type Logger interface {
	Debugf(format string, args ...any)
	Warnf(format string, args ...any)
}

var (
	mu           sync.RWMutex
	globalLogger Logger = noopLogger{}
)

// Set will use the given logger for logging messages.
func Set(logger Logger) {
	if logger == nil {
		logger = &noopLogger{} // just in case
	}
	mu.Lock()
	defer mu.Unlock()
	globalLogger = logger
}

func Clear() {
	Set(noopLogger{})
}

// SetStdLogger uses go's built in `log` package for logging.
func SetStdLogger() {
	Set(&stdLogger{})
}

// Logger returns the current logger.
func Get() Logger {
	mu.RLock()
	defer mu.RUnlock()
	return globalLogger
}

// noopLogger logs to nowhere.
type noopLogger struct{}

func (noopLogger) Debugf(string, ...any) {}
func (noopLogger) Warnf(string, ...any)  {}

// stdLogger logs to the standard logger.
type stdLogger struct{}

func (stdLogger) Debugf(format string, args ...any) {
	log.Printf("DEBUG braintrust: "+format, args...)
}

func (stdLogger) Warnf(format string, args ...any) {
	log.Printf("WARN braintrust: "+format, args...)
}
