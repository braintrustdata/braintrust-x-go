package traceopenai

import (
	"log"
	"sync"
)

// Logger is the interface for enable diagnostic logging.
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
}

var (
	mu           sync.RWMutex
	globalLogger Logger = noopLogger{}
)

// SetLogger will use the given logger for logging mesages.
func SetLogger(l Logger) {
	mu.Lock()
	defer mu.Unlock()
	globalLogger = l
}

// SetStdLogger uses go's built in `log` package for logging.
func SetStdLogger() {
	SetLogger(&stdLogger{})
}

func logger() Logger {
	mu.RLock()
	defer mu.RUnlock()
	return globalLogger
}

// noopLogger logs to nowhere.
type noopLogger struct{}

func (noopLogger) Debugf(string, ...any) {}
func (noopLogger) Infof(string, ...any)  {}
func (noopLogger) Warnf(string, ...any)  {}

var _ Logger = &noopLogger{}

// stdLogger logs to the standard logger.
type stdLogger struct{}

func (stdLogger) Debugf(format string, args ...any) {
	log.Printf("DEBUG braintrust: "+format, args...)
}

func (stdLogger) Infof(format string, args ...any) {
	log.Printf("INFO braintrust: "+format, args...)
}

func (stdLogger) Warnf(format string, args ...any) {
	log.Printf("WARN braintrust: "+format, args...)
}

var _ Logger = &stdLogger{}
