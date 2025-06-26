package log

import (
	"bytes"
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultLogger(_ *testing.T) {
	Debugf("no panic")
	Warnf("no panic")
}

func TestSetNilLogger(_ *testing.T) {
	defer Clear()
	Set(nil)
	Debugf("no panic")
	Warnf("no panic")
}

func TestLoggingFunctions(t *testing.T) {
	logger := &testLogger{}
	Set(logger)
	defer Clear()

	Debugf("debug message %s", "test1")
	Warnf("warn message %s", "test2")

	assert := assert.New(t)
	assert.Equal(logger.debugMsgs, []string{"debug message test1"})
	assert.Equal(logger.warnMsgs, []string{"warn message test2"})
}

func TestDebugLogger(t *testing.T) {
	assert := assert.New(t)
	var buf bytes.Buffer

	w := log.Writer()
	log.SetOutput(&buf)
	Set(debugLogger)
	defer func() {
		Clear()
		log.SetOutput(w)
	}()

	Debugf("123")
	Warnf("4567")

	output := buf.String()
	assert.Contains(output, "DEBUG")
	assert.Contains(output, "123")
	assert.Contains(output, "WARN")
	assert.Contains(output, "456")
}

func TestWarnLogger(t *testing.T) {
	assert := assert.New(t)

	var buf bytes.Buffer

	w := log.Writer()
	defer func() {
		log.SetOutput(w)
	}()

	log.SetOutput(&buf)

	Set(warnLogger)
	defer Clear()

	Debugf("123")
	Warnf("456")

	output := buf.String()
	assert.NotContains(output, "DEBUG")
	assert.NotContains(output, "123")
	assert.Contains(output, "WARN")
	assert.Contains(output, "456")
}

type testLogger struct {
	debugMsgs []string
	warnMsgs  []string
}

func (t *testLogger) Debugf(format string, args ...any) {
	t.debugMsgs = append(t.debugMsgs, fmt.Sprintf(format, args...))
}

func (t *testLogger) Infof(format string, args ...any) {
	// No-op for testing
}

func (t *testLogger) Warnf(format string, args ...any) {
	t.warnMsgs = append(t.warnMsgs, fmt.Sprintf(format, args...))
}
