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
