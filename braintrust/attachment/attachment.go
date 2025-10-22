// Package attachment provides utilities for creating and managing attachments
// in Braintrust traces. Attachments allow you to include images, files, and
// other binary data in your traces.
//
// Most users won't need to use this package directly, as the instrumentation
// middleware (traceopenai, traceanthropic) automatically handles attachment
// conversion. This package is primarily useful for:
//   - Manual logging without instrumentation
//   - Custom scenarios not covered by auto-instrumentation
//   - Writing instrumentation for new providers
package attachment

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
)

// ContentType represents the MIME type of an attachment.
type ContentType string

// Common content types for attachments
const (
	ImageJPEG ContentType = "image/jpeg"
	ImagePNG  ContentType = "image/png"
	ImageGIF  ContentType = "image/gif"
	ImageWEBP ContentType = "image/webp"
	TextPlain ContentType = "text/plain"
)

// Attachment represents a file or binary data attachment in Braintrust format.
// When serialized to JSON, it produces the format expected by Braintrust:
//
//	{"type": "base64_attachment", "content": "data:image/jpeg;base64,..."}
type Attachment struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// FromBytes creates an attachment from raw bytes.
// The bytes are base64-encoded and wrapped in a data URL.
//
// Example:
//
//	data, _ := os.ReadFile("image.jpg")
//	att := attachment.FromBytes(attachment.IMAGE_JPEG, data)
func FromBytes(contentType ContentType, data []byte) *Attachment {
	encoded := base64.StdEncoding.EncodeToString(data)
	return &Attachment{
		Type:    "base64_attachment",
		Content: formatDataURL(string(contentType), encoded),
	}
}

// FromFile creates an attachment by reading a file from the filesystem.
// Returns an error if the file cannot be read.
//
// Example:
//
//	att, err := attachment.FromFile(attachment.IMAGE_JPEG, "/path/to/image.jpg")
//	if err != nil {
//	    log.Fatal(err)
//	}
func FromFile(contentType ContentType, path string) (*Attachment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return FromBytes(contentType, data), nil
}

// FromReader creates an attachment by reading from an io.Reader.
// All data from the reader is consumed and base64-encoded.
// Returns an error if reading fails.
//
// Example:
//
//	file, _ := os.Open("image.jpg")
//	defer file.Close()
//	att, err := attachment.FromReader(attachment.IMAGE_JPEG, file)
func FromReader(contentType ContentType, r io.Reader) (*Attachment, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read from reader: %w", err)
	}
	return FromBytes(contentType, data), nil
}

// FromBase64 creates an attachment from already base64-encoded data.
// The base64 data is wrapped in a data URL without re-encoding.
//
// Example:
//
//	base64Data := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
//	att := attachment.FromBase64(attachment.IMAGE_PNG, base64Data)
func FromBase64(contentType ContentType, base64Data string) *Attachment {
	return &Attachment{
		Type:    "base64_attachment",
		Content: formatDataURL(string(contentType), base64Data),
	}
}

// FromURL creates an attachment by fetching data from an HTTP/HTTPS URL.
// The content type is automatically derived from the Content-Type response header.
// Returns an error if the HTTP request fails or if the response status is not 200 OK.
//
// Example:
//
//	att, err := attachment.FromURL("https://example.com/image.png")
//	if err != nil {
//	    log.Fatal(err)
//	}
func FromURL(url string) (*Attachment, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch URL %s: status %d", url, resp.StatusCode)
	}

	// Get content type from response header
	contentType := ContentType(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return FromReader(contentType, resp.Body)
}

// formatDataURL formats a data URL with the given content type and base64 data.
// Returns a string in the format: "data:image/jpeg;base64,..."
func formatDataURL(contentType, base64Data string) string {
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data)
}
