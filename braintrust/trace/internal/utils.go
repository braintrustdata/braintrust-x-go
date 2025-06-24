// Package internal provides shared utilities for OpenTelemetry middleware tracers.
package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// BufferedReader saves data read from the readCloser and triggers an action
// when fully read or closed.
type BufferedReader struct {
	src    io.ReadCloser
	buf    *bytes.Buffer
	onDone func(io.Reader) // called once when fully read or closed
	once   sync.Once
	closed bool
}

// NewBufferedReader creates a new buffered reader that calls onDone when fully read or closed.
func NewBufferedReader(src io.ReadCloser, onDone func(io.Reader)) *BufferedReader {
	return &BufferedReader{
		src:    src,
		buf:    &bytes.Buffer{},
		onDone: onDone,
	}
}

func (r *BufferedReader) Read(p []byte) (int, error) {
	n, err := r.src.Read(p)
	if n > 0 {
		_, _ = r.buf.Write(p[:n])
	}
	if err == io.EOF {
		r.trigger()
	}
	return n, err
}

// Close closes the underlying reader and triggers the onDone callback.
func (r *BufferedReader) Close() error {
	r.closed = true
	r.trigger()
	return r.src.Close()
}

// trigger ensures onDone is only called once
func (r *BufferedReader) trigger() {
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

// ParseUsageTokens parses the usage tokens from various API responses
// It handles different API formats using a unified approach
func ParseUsageTokens(usage map[string]interface{}) map[string]int64 {
	metrics := make(map[string]int64)

	if usage == nil {
		return metrics
	}

	// Parse token metrics and translate names to be consistent
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

// SetJSONAttr is a helper function to set JSON attributes on spans
func SetJSONAttr(span trace.Span, key string, value any) error {
	jsonStr, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal attribute %s: %w", key, err)
	}
	span.SetAttributes(attribute.String(key, string(jsonStr)))
	return nil
}
