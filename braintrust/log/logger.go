// Package log provides logging functionality for the Braintrust SDK.
package log

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
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
}

var (
	noopLogger  = &logger{level: levelOff}
	warnLogger  = &logger{level: levelWarn}
	debugLogger = &logger{level: levelDebug}

	mu           sync.RWMutex
	globalLogger Logger = warnLogger
)

// Set the given logger as our global logger. You can use this to
// plug your own logging system into the braintrust SDK.
func Set(logger Logger) {
	if logger == nil {
		logger = noopLogger // just in case
	}
	mu.Lock()
	defer mu.Unlock()
	globalLogger = logger
}

// Get returns the current logger.
func Get() Logger {
	mu.RLock()
	defer mu.RUnlock()
	return globalLogger
}

// Clear clears the current logger.
func Clear() {
	Set(noopLogger)
}

// Debugf logs a debug message using the configured logger.
func Debugf(format string, args ...any) {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		globalLogger.Debugf(format, args...)
	}
}

// Infof logs an info message using the configured logger.
func Infof(format string, args ...any) {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		globalLogger.Infof(format, args...)
	}
}

// Warnf logs a warning message using the configured logger.
func Warnf(format string, args ...any) {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		globalLogger.Warnf(format, args...)
	}
}

type level int

const (
	levelDebug level = iota
	levelInfo
	levelWarn
	levelOff
)

// logger is prints to the go std logger
type logger struct {
	level level
}

func (l *logger) Debugf(format string, args ...any) {
	if l.level > levelDebug {
		return
	}
	log.Printf("DEBUG braintrust: "+format, args...)
}

func (l *logger) Infof(format string, args ...any) {
	if l.level > levelInfo {
		return
	}
	log.Printf("INFO braintrust: "+format, args...)
}

func (l *logger) Warnf(format string, args ...any) {
	if l.level > levelWarn {
		return
	}
	log.Printf("WARN braintrust: "+format, args...)
}

func init() {
	Set(warnLogger)
	if strings.ToLower(os.Getenv("BRAINTRUST_DEBUG")) == "true" {
		Set(debugLogger)
	}
}
