package traceopenai

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/braintrust/braintrust-x-go/braintrust/trace/internal"
)

func TestBufferedReaderBasic(t *testing.T) {
	assert := assert.New(t)

	input := "hello world"
	var in bytes.Buffer
	in.WriteString(input)

	var captured *bytes.Buffer
	onDone := func(r io.Reader) {
		data, _ := io.ReadAll(r)
		captured = bytes.NewBuffer(data)
	}

	reader := internal.NewBufferedReader(io.NopCloser(&in), onDone)

	// Read the data
	output, err := io.ReadAll(reader)
	assert.NoError(err)
	assert.Equal(input, string(output))

	// Verify onDone was called with the correct buffer
	assert.NotNil(captured)
	assert.Equal(input, captured.String())
}

func TestBufferedReaderChunkedReads(t *testing.T) {
	assert := assert.New(t)

	pr, pw := io.Pipe()

	var captured *bytes.Buffer
	onDone := func(r io.Reader) {
		data, _ := io.ReadAll(r)
		captured = bytes.NewBuffer(data)
	}

	reader := internal.NewBufferedReader(pr, onDone)

	// Write and read in chunks
	chunks := []string{
		"first chunk ",
		"second chunk ",
		"third chunk",
	}

	for _, chunk := range chunks {
		// Write chunk
		go func(data string) {
			_, err := pw.Write([]byte(data))
			assert.NoError(err)
		}(chunk)

		// Read chunk
		buf := make([]byte, 1024)
		n, err := reader.Read(buf)
		assert.NoError(err)
		assert.Equal(chunk, string(buf[:n]))
	}

	// Close the writer to signal EOF
	_ = pw.Close()

	// Try to read again - should get EOF
	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	assert.Equal(0, n)
	assert.Equal(io.EOF, err)

	// Verify onDone was called with the correct full buffer
	assert.NotNil(captured)
	assert.Equal("first chunk second chunk third chunk", captured.String())
}

func TestBufferedReaderClose(t *testing.T) {
	assert := assert.New(t)

	pr, pw := io.Pipe()

	var callbackCalled bool
	onDone := func(r io.Reader) {
		callbackCalled = true
		data, _ := io.ReadAll(r)
		assert.Equal("partial data", string(data))
	}

	reader := internal.NewBufferedReader(pr, onDone)

	// Write partial data
	go func() {
		_, err := pw.Write([]byte("partial data"))
		assert.NoError(err)
		// Don't close the pipe yet
	}()

	// Read the partial data
	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	assert.NoError(err)
	assert.Equal("partial data", string(buf[:n]))

	// Now close the reader before reaching EOF
	err = reader.Close()
	assert.NoError(err)

	// Verify the callback was triggered on close
	assert.True(callbackCalled)
}

func TestBufferedReaderCallbackOnlyOnce(t *testing.T) {
	assert := assert.New(t)

	input := "test data"
	var in bytes.Buffer
	in.WriteString(input)

	var callCount int
	onDone := func(_ io.Reader) {
		callCount++
	}

	reader := internal.NewBufferedReader(io.NopCloser(&in), onDone)

	// Read to EOF to trigger callback
	_, err := io.ReadAll(reader)
	assert.NoError(err)
	assert.Equal(1, callCount)

	// Close after EOF - should not trigger callback again
	err = reader.Close()
	assert.NoError(err)
	assert.Equal(1, callCount)
}

func TestToInt64(t *testing.T) {
	assert := assert.New(t)

	// Test various numeric types
	tests := []struct {
		name     string
		input    interface{}
		expected int64
		success  bool
	}{
		{"float64", float64(123.45), int64(123), true},
		{"int64", int64(42), int64(42), true},
		{"int", int(100), int64(100), true},
		{"float32", float32(67.89), int64(67), true},
		{"uint64", uint64(999), int64(999), true},
		{"uint", uint(456), int64(456), true},
		{"uint32", uint32(789), int64(789), true},
		{"string", "not a number", int64(0), false},
		{"nil", nil, int64(0), false},
		{"bool", true, int64(0), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			success, result := internal.ToInt64(test.input)
			assert.Equal(test.success, success, "Expected success to be %v for input %v", test.success, test.input)
			if test.success {
				assert.Equal(test.expected, result, "Expected result to be %v for input %v", test.expected, test.input)
			}
		})
	}
}
