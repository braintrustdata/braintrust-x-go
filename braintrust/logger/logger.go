package logger

import (
	"bytes"
	"io"
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

// bufferedReader saves data read from the readCloser and triggers an action
// when fully read or closed.
type bufferedReader struct {
	src    io.ReadCloser
	buf    *bytes.Buffer
	onDone func(io.Reader) // called once when fully read or closed
	once   sync.Once
	closed bool
}

func newBufferedReader(src io.ReadCloser, onDone func(io.Reader)) *bufferedReader {
	return &bufferedReader{
		src:    src,
		buf:    &bytes.Buffer{},
		onDone: onDone,
	}
}

func (r *bufferedReader) Read(p []byte) (int, error) {
	n, err := r.src.Read(p)
	if n > 0 {
		_, _ = r.buf.Write(p[:n])
	}
	if err == io.EOF {
		r.trigger()
	}
	return n, err
}

func (r *bufferedReader) Close() error {
	r.closed = true
	r.trigger()
	return r.src.Close()
}

// trigger ensures onDone is only called once
func (r *bufferedReader) trigger() {
	r.once.Do(func() {
		if r.onDone != nil {
			r.onDone(r.buf)
		}
	})
}
