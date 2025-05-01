package internal

import (
	"fmt"
	"runtime/debug"
	"testing"

	"github.com/braintrust/braintrust-x-go/braintrust/diag"
)

// FailTestLogger is a diag.Logger that will fail tests if a warning is logged.
type FailTestLogger struct {
	t *testing.T
}

func NewFailTestLogger(t *testing.T) *FailTestLogger {
	return &FailTestLogger{t: t}
}

func (f *FailTestLogger) Debugf(format string, args ...any) {}

func (f *FailTestLogger) Warnf(format string, args ...any) {
	f.t.Fatalf("failTestLogger caught a warning: %s\n%s", fmt.Sprintf(format, args...), string(debug.Stack()))
}

var _ diag.Logger = &FailTestLogger{}
