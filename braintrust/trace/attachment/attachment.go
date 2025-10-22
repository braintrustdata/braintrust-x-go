// Package attachment provides utilities for creating and managing attachments
// in Braintrust traces.
//
// Attachments allow you to log arbitrary binary data, like images, audio, video,
// PDFs, and large JSON objects, as part of your traces. This enables multimodal
// evaluations, handling substantial data structures, and advanced use cases such
// as summarizing visual content or analyzing document metadata. Attachments are
// securely stored in an object store and associated with your organization.
//
// Most users won't need to use this package directly, as the instrumentation
// middleware (traceopenai, traceanthropic) automatically handles attachment
// conversion. This package is primarily useful for:
//   - Manual logging without instrumentation
//   - Custom scenarios not covered by auto-instrumentation
//   - Writing instrumentation for new providers
//
// For more information, see: https://www.braintrust.dev/docs/guides/attachments
package attachment

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Common MIME types for attachments
const (
	ImageJPEG = "image/jpeg"
	ImagePNG  = "image/png"
	ImageGIF  = "image/gif"
	ImageWEBP = "image/webp"
	TextPlain = "text/plain"
	PDF       = "application/pdf"
)

// Attachment represents a file or binary data attachment in Braintrust format.
// When serialized to JSON, it produces the format expected by Braintrust:
//
//	{"type": "base64_attachment", "content": "data:image/jpeg;base64,..."}
type Attachment struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// FromReader creates an attachment by reading from an io.Reader.
// All data from the reader is consumed and base64-encoded.
// Returns an error if reading fails.
//
// Common content types: "image/jpeg", "image/png", "image/gif", "image/webp",
// "application/pdf", "text/plain"
//
// Example:
//
//	file, _ := os.Open("image.jpg")
//	defer file.Close()
//	att, err := attachment.FromReader("image/jpeg", file)
func FromReader(contentType string, r io.Reader) (*Attachment, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read from reader: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return &Attachment{
		Type:    "base64_attachment",
		Content: formatDataURL(contentType, encoded),
	}, nil
}

// FromBytes creates an attachment from raw bytes.
// The bytes are base64-encoded and wrapped in a data URL.
//
// Example:
//
//	data, _ := os.ReadFile("image.jpg")
//	att := attachment.FromBytes("image/jpeg", data)
func FromBytes(contentType string, data []byte) *Attachment {
	att, _ := FromReader(contentType, bytes.NewReader(data))
	return att
}

// FromFile creates an attachment by reading a file from the filesystem.
// Returns an error if the file cannot be read.
//
// Example:
//
//	att, err := attachment.FromFile("image/jpeg", "/path/to/image.jpg")
//	if err != nil {
//	    log.Fatal(err)
//	}
func FromFile(contentType string, path string) (*Attachment, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()
	return FromReader(contentType, file)
}

// FromBase64 creates an attachment from already base64-encoded data.
// The base64 data is wrapped in a data URL without re-encoding.
//
// Example:
//
//	base64Data := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
//	att := attachment.FromBase64("image/png", base64Data)
func FromBase64(contentType string, base64Data string) *Attachment {
	return &Attachment{
		Type:    "base64_attachment",
		Content: formatDataURL(contentType, base64Data),
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
	contentType := resp.Header.Get("Content-Type")
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
