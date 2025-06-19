package traceopenai

import (
	"bytes"
	"io"
	"strings"
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

// translateMetricPrefix translates metric prefixes to be consistent between APIs
func translateMetricPrefix(prefix string) string {
	switch prefix {
	case "input":
		return "prompt"
	case "output":
		return "completion"
	default:
		return prefix
	}
}

// parseUsageTokens parses the usage tokens from various OpenAI API responses
// It handles both the responses API and chat completions API using a unified approach
func parseUsageTokens(usage map[string]interface{}) map[string]int64 {
	metrics := make(map[string]int64)

	if usage == nil {
		return metrics
	}

	// we translate metrics names to be consistent with the chat completion api.
	for k, v := range usage {
		if strings.HasSuffix(k, "_tokens_details") {
			prefix := translateMetricPrefix(strings.TrimSuffix(k, "_tokens_details"))
			if details, ok := v.(map[string]interface{}); ok {
				for kd, vd := range details {
					if ok, i := toInt64(vd); ok {
						metrics[prefix+"_"+kd] = i
					}
				}
			}
		} else {
			if ok, i := toInt64(v); ok {
				k = translateMetricKey(k)
				metrics[k] = i
			}
		}
	}

	return metrics
}
