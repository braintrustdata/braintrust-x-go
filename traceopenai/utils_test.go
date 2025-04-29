package traceopenai

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTeeAsynchronousRead(t *testing.T) {
	assert := assert.New(t)

	input := []byte("hello world")
	var in bytes.Buffer
	in.Write(input)

	out1, out2 := Tee(io.NopCloser(&in))

	var wg sync.WaitGroup

	var data1, data2 []byte
	var err1, err2 error

	wg.Add(1)
	go func() {
		defer wg.Done()
		data1, err1 = io.ReadAll(out1)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		// Add a small delay to ensure reads are not synchronized
		time.Sleep(10 * time.Millisecond)
		data2, err2 = io.ReadAll(out2)
	}()

	wg.Wait()

	assert.NoError(err1)
	assert.NoError(err2)
	assert.Equal(input, data1)
	assert.Equal(input, data2)
}

func TestTeeMultipleWritesAndReads(t *testing.T) {
	assert := assert.New(t)

	pr, pw := io.Pipe()
	out1, out2 := Tee(pr)

	// Write and read in chunks
	chunks := []string{
		"first chunk ",
		"second chunk ",
		"third chunk",
	}

	var expected bytes.Buffer

	for _, chunk := range chunks {
		// Write chunk
		_, err := pw.Write([]byte(chunk))
		assert.NoError(err)
		expected.WriteString(chunk)

		// Read chunk from both outputs
		buf1 := make([]byte, 1024)
		buf2 := make([]byte, 1024)

		n1, err1 := out1.Read(buf1)
		assert.NoError(err1)
		assert.Equal(chunk, string(buf1[:n1]))

		n2, err2 := out2.Read(buf2)
		assert.NoError(err2)
		assert.Equal(chunk, string(buf2[:n2]))
	}

	// Close the writer to signal EOF
	pw.Close()

	// Try to read from both outputs - should get EOF
	buf1 := make([]byte, 1024)
	buf2 := make([]byte, 1024)

	n1, err1 := out1.Read(buf1)
	assert.Equal(0, n1)
	assert.Equal(io.EOF, err1)

	n2, err2 := out2.Read(buf2)
	assert.Equal(0, n2)
	assert.Equal(io.EOF, err2)

	// Verify total content matches expected
	assert.Equal("first chunk second chunk third chunk", expected.String())
}
