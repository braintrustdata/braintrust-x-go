package attachment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachment_FromBytes_EncodesCorrectly(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Create attachment from raw bytes
	data := []byte("test data")
	att := FromBytes(ImageJPEG, data)

	require.NotNil(att)
	assert.Equal("base64_attachment", att.Type)

	// Should contain data URL with base64 encoding
	assert.Contains(att.Content, "data:image/jpeg;base64,")
	// "test data" in base64 is "dGVzdCBkYXRh"
	assert.Contains(att.Content, "dGVzdCBkYXRh")
}

func TestAttachment_FromFile_ReadsJPEG(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jpg")
	testData := []byte("fake jpeg data")
	err := os.WriteFile(testFile, testData, 0644)
	require.NoError(err)

	// Read attachment from file
	att, err := FromFile(ImageJPEG, testFile)
	require.NoError(err)
	require.NotNil(att)

	assert.Equal("base64_attachment", att.Type)
	assert.Contains(att.Content, "data:image/jpeg;base64,")
	// "fake jpeg data" base64 encoded
	assert.Contains(att.Content, "ZmFrZSBqcGVnIGRhdGE=")
}

func TestAttachment_FromFile_ErrorOnNonExistentFile(t *testing.T) {
	require := require.New(t)

	// Try to read non-existent file
	att, err := FromFile(ImagePNG, "/nonexistent/file.png")

	require.Error(err)
	assert.Nil(t, att)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestAttachment_FromReader_ReadsStream(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Create a temporary file and open it as a reader
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.png")
	testData := []byte("fake png data")
	err := os.WriteFile(testFile, testData, 0644)
	require.NoError(err)

	file, err := os.Open(testFile)
	require.NoError(err)
	defer func() {
		_ = file.Close()
	}()

	// Read attachment from reader
	att, err := FromReader(ImagePNG, file)
	require.NoError(err)
	require.NotNil(att)

	assert.Equal("base64_attachment", att.Type)
	assert.Contains(att.Content, "data:image/png;base64,")
	// "fake png data" base64 encoded
	assert.Contains(att.Content, "ZmFrZSBwbmcgZGF0YQ==")
}

func TestAttachment_FromBase64_WrapsData(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Use already base64-encoded data
	// This is a tiny 1x1 transparent PNG
	base64Data := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

	att := FromBase64(ImagePNG, base64Data)
	require.NotNil(att)

	assert.Equal("base64_attachment", att.Type)
	assert.Contains(att.Content, "data:image/png;base64,")
	// Should contain the exact base64 data without re-encoding
	assert.Contains(att.Content, base64Data)
	assert.Equal("data:image/png;base64,"+base64Data, att.Content)
}

func TestAttachment_JSONMarshal_CorrectFormat(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Create an attachment
	data := []byte("test image")
	att := FromBytes(ImageJPEG, data)

	// Marshal to JSON
	jsonData, err := json.Marshal(att)
	require.NoError(err)

	// Parse the JSON back
	var result map[string]interface{}
	err = json.Unmarshal(jsonData, &result)
	require.NoError(err)

	// Verify structure matches Braintrust format
	assert.Equal("base64_attachment", result["type"])
	assert.Contains(result["content"], "data:image/jpeg;base64,")

	// Verify it can be used in a message structure
	message := map[string]interface{}{
		"role": "user",
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "What's in this image?",
			},
			att, // Include attachment
		},
	}

	messageJSON, err := json.Marshal(message)
	require.NoError(err)

	// Verify the full message structure
	var parsedMessage map[string]interface{}
	err = json.Unmarshal(messageJSON, &parsedMessage)
	require.NoError(err)

	assert.Equal("user", parsedMessage["role"])
	content, ok := parsedMessage["content"].([]interface{})
	require.True(ok, "content should be an array")
	assert.Len(content, 2)

	// Check text part
	textPart, ok := content[0].(map[string]interface{})
	require.True(ok, "text part should be a map")
	assert.Equal("text", textPart["type"])

	// Check attachment part
	attPart, ok := content[1].(map[string]interface{})
	require.True(ok, "attachment part should be a map")
	assert.Equal("base64_attachment", attPart["type"])
	assert.Contains(attPart["content"], "data:image/jpeg;base64,")
}

func TestAttachment_FromURL_FetchesImage(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Create a mock HTTP server
	testData := []byte("image from url")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(testData)
	}))
	defer server.Close()

	// Fetch attachment from URL
	att, err := FromURL(server.URL)
	require.NoError(err)
	require.NotNil(att)

	assert.Equal("base64_attachment", att.Type)
	assert.Contains(att.Content, "data:image/png;base64,")
	// "image from url" base64 encoded
	assert.Contains(att.Content, "aW1hZ2UgZnJvbSB1cmw=")
}

func TestAttachment_FromURL_ErrorOn404(t *testing.T) {
	require := require.New(t)

	// Create a mock HTTP server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Try to fetch from URL
	att, err := FromURL(server.URL)

	require.Error(err)
	assert.Nil(t, att)
	assert.Contains(t, err.Error(), "status 404")
}

func TestAttachment_FromURL_ErrorOnInvalidURL(t *testing.T) {
	require := require.New(t)

	// Try to fetch from invalid URL
	att, err := FromURL("http://this-domain-does-not-exist-12345.com/image.png")

	require.Error(err)
	assert.Nil(t, att)
	assert.Contains(t, err.Error(), "failed to fetch URL")
}
