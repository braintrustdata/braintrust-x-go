package internal

import (
	"fmt"
	"runtime/debug"
	"testing"

	"github.com/braintrust/braintrust-x-go/braintrust/diag"
)

// FailTestsOnWarnings will fail tests if warnings are produced during tests.
func FailTestsOnWarnings(t *testing.T) {
	diag.SetLogger(newFailTestLogger(t))
}

type failTestLogger struct {
	t *testing.T
}

func newFailTestLogger(t *testing.T) *failTestLogger {
	return &failTestLogger{t: t}
}

func (f *failTestLogger) Debugf(format string, args ...any) {}

func (f *failTestLogger) Warnf(format string, args ...any) {
	f.t.Fatalf("failTestLogger caught a warning: %s\n%s", fmt.Sprintf(format, args...), string(debug.Stack()))
}

var _ diag.Logger = &failTestLogger{}
