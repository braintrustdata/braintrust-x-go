package traceopenai

import (
	"bytes"
	"io"
	"sync"
)

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

// translateMetricKey translates metric keys to be consistent between APIs
func translateMetricKey(key string) string {
	switch key {
	case "input_tokens":
		return "prompt_tokens"
	case "output_tokens":
		return "completion_tokens"
	case "total_tokens":
		return "tokens"
	}
	return key
}

// toInt64 converts various numeric types to int64
func toInt64(v any) (bool, int64) {
	switch v := v.(type) {
	case float64:
		return true, int64(v)
	case int64:
		return true, v
	case int:
		return true, int64(v)
	case float32:
		return true, int64(v)
	case uint64:
		return true, int64(v)
	case uint:
		return true, int64(v)
	case uint32:
		return true, int64(v)
	default:
		return false, 0
	}
}
