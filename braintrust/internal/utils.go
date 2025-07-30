// revive:disable:package-comments
// revive:disable:exported
package internal

import (
	"fmt"
	"runtime/debug"
	"testing"

	"github.com/braintrustdata/braintrust-x-go/braintrust/log"
)

// FailTestsOnWarnings will fail tests if warnings are produced during tests. Currently
// not able to be parallelized.
func FailTestsOnWarnings(t *testing.T) {
	t.Helper()
	original := log.Get()
	log.Set(newFailTestLogger(t))
	t.Cleanup(func() {
		log.Set(original)
	})
}

type failTestLogger struct {
	t *testing.T
}

func newFailTestLogger(t *testing.T) *failTestLogger {
	t.Helper()
	return &failTestLogger{t: t}
}

func (f *failTestLogger) Debugf(_ string, _ ...any) {}

func (f *failTestLogger) Infof(_ string, _ ...any) {}

func (f *failTestLogger) Warnf(format string, args ...any) {
	f.t.Helper()
	f.t.Fatalf("failTestLogger caught a warning: %s\n%s", fmt.Sprintf(format, args...), string(debug.Stack()))
}

var _ log.Logger = &failTestLogger{}
