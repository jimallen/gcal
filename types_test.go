package gcal

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestNewErrorResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		code    string
		message string
		want    Response
	}{
		{
			name:    "not configured error",
			code:    ErrNotConfigured,
			message: "credentials not found",
			want: Response{
				Success: false,
				Error:   ErrNotConfigured,
				Message: "credentials not found",
			},
		},
		{
			name:    "API error",
			code:    ErrAPIError,
			message: "failed to fetch events",
			want: Response{
				Success: false,
				Error:   ErrAPIError,
				Message: "failed to fetch events",
			},
		},
	}

	for _, tt := range tests {
		tt := tt // Capture loop variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewErrorResponse(tt.code, tt.message)

			// Compare only relevant fields (ignore LastSync which is time-dependent)
			want := tt.want
			if diff := cmp.Diff(got.Success, want.Success); diff != "" {
				t.Errorf("NewErrorResponse() Success mismatch (-got +want):\n%s", diff)
			}
			if diff := cmp.Diff(got.Error, want.Error); diff != "" {
				t.Errorf("NewErrorResponse() Error mismatch (-got +want):\n%s", diff)
			}
			if diff := cmp.Diff(got.Message, want.Message); diff != "" {
				t.Errorf("NewErrorResponse() Message mismatch (-got +want):\n%s", diff)
			}
			if got.Events != nil {
				t.Errorf("NewErrorResponse() Events should be nil, got %v", got.Events)
			}
		})
	}
}

func TestNewSuccessResponse(t *testing.T) {
	t.Parallel()

	now := time.Now()
	events := []Event{
		{
			ID:    "event1",
			Title: "Test Event",
			Start: now.Format(time.RFC3339),
			End:   now.Add(time.Hour).Format(time.RFC3339),
		},
	}

	got := NewSuccessResponse(events)

	if !got.Success {
		t.Error("NewSuccessResponse() Success = false, want true")
	}
	if diff := cmp.Diff(len(got.Events), len(events)); diff != "" {
		t.Errorf("NewSuccessResponse() Events length mismatch (-got +want):\n%s", diff)
	}
	if got.LastSync == "" {
		t.Error("NewSuccessResponse() LastSync is empty")
	}
	// Verify LastSync is valid RFC3339
	if _, err := time.Parse(time.RFC3339, got.LastSync); err != nil {
		t.Errorf("NewSuccessResponse() LastSync is not valid RFC3339: %v", err)
	}
	if got.Error != "" {
		t.Errorf("NewSuccessResponse() Error should be empty, got %q", got.Error)
	}
	if got.Message != "" {
		t.Errorf("NewSuccessResponse() Message should be empty, got %q", got.Message)
	}
	// Verify events match
	if diff := cmp.Diff(got.Events, events); diff != "" {
		t.Errorf("NewSuccessResponse() Events mismatch (-got +want):\n%s", diff)
	}
}

func TestNewSuccessResponse_EmptyEvents(t *testing.T) {
	t.Parallel()

	got := NewSuccessResponse([]Event{})

	if !got.Success {
		t.Error("NewSuccessResponse() Success = false, want true")
	}
	if len(got.Events) != 0 {
		t.Errorf("NewSuccessResponse() Events length = %v, want 0", len(got.Events))
	}
	if got.LastSync == "" {
		t.Error("NewSuccessResponse() LastSync is empty")
	}
}

// ExampleNewErrorResponse demonstrates creating an error response
func ExampleNewErrorResponse() {
	resp := NewErrorResponse(ErrNotConfigured, "credentials not found")
	_ = resp
	// Output:
}

// ExampleNewSuccessResponse demonstrates creating a success response
func ExampleNewSuccessResponse() {
	events := []Event{
		{
			ID:    "event1",
			Title: "Team Meeting",
			Start: "2024-01-15T14:00:00Z",
			End:   "2024-01-15T15:00:00Z",
		},
	}
	resp := NewSuccessResponse(events)
	_ = resp
	// Output:
}
