package api

import (
	"encoding/json"
	"testing"
)

// TestDatasetEvent_Tags tests that DatasetEvent can serialize and deserialize tags
func TestDatasetEvent_Tags(t *testing.T) {
	tests := []struct {
		name     string
		event    DatasetEvent
		wantJSON string
	}{
		{
			name: "event with tags",
			event: DatasetEvent{
				Input:    "test input",
				Expected: "test output",
				Tags:     []string{"tag1", "tag2"},
			},
			wantJSON: `{"input":"test input","expected":"test output","tags":["tag1","tag2"]}`,
		},
		{
			name: "event without tags",
			event: DatasetEvent{
				Input:    "test input",
				Expected: "test output",
			},
			wantJSON: `{"input":"test input","expected":"test output"}`,
		},
		{
			name: "event with empty tags",
			event: DatasetEvent{
				Input:    "test input",
				Expected: "test output",
				Tags:     []string{},
			},
			wantJSON: `{"input":"test input","expected":"test output"}`,
		},
		{
			name: "event with single tag",
			event: DatasetEvent{
				Input:    "test input",
				Expected: "test output",
				Tags:     []string{"single-tag"},
			},
			wantJSON: `{"input":"test input","expected":"test output","tags":["single-tag"]}`,
		},
		{
			name: "event with nil tags",
			event: DatasetEvent{
				Input:    "test input",
				Expected: "test output",
				Tags:     nil,
			},
			wantJSON: `{"input":"test input","expected":"test output"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			gotJSON, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("failed to marshal event: %v", err)
			}

			// Compare JSON (normalize for comparison)
			var got, want interface{}
			if err := json.Unmarshal(gotJSON, &got); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantJSON), &want); err != nil {
				t.Fatalf("failed to unmarshal expected: %v", err)
			}

			gotStr, _ := json.Marshal(got)
			wantStr, _ := json.Marshal(want)
			if string(gotStr) != string(wantStr) {
				t.Errorf("JSON mismatch:\ngot:  %s\nwant: %s", gotStr, wantStr)
			}

			// Test unmarshaling
			var unmarshaled DatasetEvent
			if err := json.Unmarshal([]byte(tt.wantJSON), &unmarshaled); err != nil {
				t.Fatalf("failed to unmarshal JSON: %v", err)
			}

			// Verify tags
			if len(tt.event.Tags) != len(unmarshaled.Tags) {
				t.Errorf("tags length mismatch: got %d, want %d", len(unmarshaled.Tags), len(tt.event.Tags))
			}
			for i, tag := range tt.event.Tags {
				if i >= len(unmarshaled.Tags) || unmarshaled.Tags[i] != tag {
					t.Errorf("tag[%d] mismatch: got %v, want %v", i, unmarshaled.Tags, tt.event.Tags)
				}
			}
		})
	}
}
