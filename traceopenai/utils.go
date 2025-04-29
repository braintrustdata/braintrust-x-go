package traceopenai

import (
	"io"
	"log"
	"sync"
)

// Tee sends the same data to two readers.
func Tee(r io.ReadCloser) (io.ReadCloser, io.ReadCloser) {
	// Create two pipes for the output readers
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()

	// Create a MultiWriter that writes to both pipes
	w := io.MultiWriter(w1, w2)

	// Start a goroutine to copy data from input to both pipes
	go func() {
		_, err := io.Copy(w, r)
		r.Close()
		w1.CloseWithError(err)
		w2.CloseWithError(err)
	}()

	return r1, r2
}

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

// SetLogger will use the given logger for logging messages.
func SetLogger(logger Logger) {
	mu.Lock()
	defer mu.Unlock()
	globalLogger = logger
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
func (noopLogger) Warnf(string, ...any)  {}

// stdLogger logs to the standard logger.
type stdLogger struct{}

func (stdLogger) Debugf(format string, args ...any) {
	log.Printf("DEBUG braintrust: "+format, args...)
}

func (stdLogger) Warnf(format string, args ...any) {
	log.Printf("WARN braintrust: "+format, args...)
}
