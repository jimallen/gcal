package gcal

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/calendar/v3"
)

func TestConvertEvent(t *testing.T) {
	t.Parallel()
	now := time.Now()
	startTime := now.Add(2 * time.Hour).Format(time.RFC3339)
	endTime := now.Add(3 * time.Hour).Format(time.RFC3339)

	tests := []struct {
		name    string
		item    *calendar.Event
		wantNil bool
		checkFn func(*testing.T, *Event)
	}{
		{
			name: "valid event with attendees",
			item: &calendar.Event{
				Id:      "event1",
				Summary: "Team Meeting",
				Start: &calendar.EventDateTime{
					DateTime: startTime,
				},
				End: &calendar.EventDateTime{
					DateTime: endTime,
				},
				Attendees: []*calendar.EventAttendee{
					{
						Self:           true,
						ResponseStatus: "accepted",
					},
					{
						Email:       "alice@example.com",
						DisplayName: "Alice",
					},
					{
						Email:       "bob@example.com",
						DisplayName: "Bob",
					},
				},
			},
			wantNil: false,
			checkFn: func(t *testing.T, e *Event) {
				if e.ID != "event1" {
					t.Errorf("convertEvent() ID = %v, want event1", e.ID)
				}
				if e.Title != "Team Meeting" {
					t.Errorf("convertEvent() Title = %v, want Team Meeting", e.Title)
				}
				if e.ResponseStatus != "accepted" {
					t.Errorf("convertEvent() ResponseStatus = %v, want accepted", e.ResponseStatus)
				}
				if e.AttendeeCount != 2 {
					t.Errorf("convertEvent() AttendeeCount = %v, want 2", e.AttendeeCount)
				}
				if len(e.Attendees) != 2 {
					t.Errorf("convertEvent() Attendees length = %v, want 2", len(e.Attendees))
				}
			},
		},
		{
			name: "cancelled event",
			item: &calendar.Event{
				Id:      "event2",
				Status:  "cancelled",
				Summary: "Cancelled Meeting",
				Start: &calendar.EventDateTime{
					DateTime: startTime,
				},
				End: &calendar.EventDateTime{
					DateTime: endTime,
				},
			},
			wantNil: true,
		},
		{
			name: "all-day event",
			item: &calendar.Event{
				Id:      "event3",
				Summary: "All Day Event",
				Start: &calendar.EventDateTime{
					Date: "2024-01-15",
				},
				End: &calendar.EventDateTime{
					Date: "2024-01-16",
				},
			},
			wantNil: true,
		},
		{
			name: "event without attendees",
			item: &calendar.Event{
				Id:      "event4",
				Summary: "Personal Event",
				Start: &calendar.EventDateTime{
					DateTime: startTime,
				},
				End: &calendar.EventDateTime{
					DateTime: endTime,
				},
				Attendees: nil,
			},
			wantNil: true,
		},
		{
			name: "event with only self attendee",
			item: &calendar.Event{
				Id:      "event5",
				Summary: "Focus Time",
				Start: &calendar.EventDateTime{
					DateTime: startTime,
				},
				End: &calendar.EventDateTime{
					DateTime: endTime,
				},
				Attendees: []*calendar.EventAttendee{
					{
						Self:           true,
						ResponseStatus: "accepted",
					},
				},
			},
			wantNil: true,
		},
		{
			name: "event not accepted",
			item: &calendar.Event{
				Id:      "event6",
				Summary: "Declined Meeting",
				Start: &calendar.EventDateTime{
					DateTime: startTime,
				},
				End: &calendar.EventDateTime{
					DateTime: endTime,
				},
				Attendees: []*calendar.EventAttendee{
					{
						Self:           true,
						ResponseStatus: "declined",
					},
					{
						Email:       "alice@example.com",
						DisplayName: "Alice",
					},
				},
			},
			wantNil: true,
		},
		{
			name: "event with nil attendees slice",
			item: &calendar.Event{
				Id:      "event7",
				Summary: "Event",
				Start: &calendar.EventDateTime{
					DateTime: startTime,
				},
				End: &calendar.EventDateTime{
					DateTime: endTime,
				},
				Attendees: nil,
			},
			wantNil: true,
		},
		{
			name: "attendee without display name uses email",
			item: &calendar.Event{
				Id:      "event8",
				Summary: "Meeting",
				Start: &calendar.EventDateTime{
					DateTime: startTime,
				},
				End: &calendar.EventDateTime{
					DateTime: endTime,
				},
				Attendees: []*calendar.EventAttendee{
					{
						Self:           true,
						ResponseStatus: "accepted",
					},
					{
						Email:       "no-name@example.com",
						DisplayName: "",
					},
				},
			},
			wantNil: false,
			checkFn: func(t *testing.T, e *Event) {
				if len(e.Attendees) != 1 {
					t.Errorf("convertEvent() Attendees length = %v, want 1", len(e.Attendees))
				}
				if e.Attendees[0] != "no-name@example.com" {
					t.Errorf("convertEvent() Attendees[0] = %v, want no-name@example.com", e.Attendees[0])
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt // Capture loop variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := convertEvent(tt.item)
			if (got == nil) != tt.wantNil {
				t.Errorf("convertEvent() returned nil = %v, want nil = %v", got == nil, tt.wantNil)
				return
			}
			if tt.checkFn != nil && got != nil {
				tt.checkFn(t, got)
			}
		})
	}
}

func TestExtractMeetingURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		item *calendar.Event
		want string
	}{
		{
			name: "Google Meet hangout link",
			item: &calendar.Event{
				HangoutLink: "https://meet.google.com/abc-defg-hij",
			},
			want: "https://meet.google.com/abc-defg-hij",
		},
		{
			name: "conference data video entry point",
			item: &calendar.Event{
				ConferenceData: &calendar.ConferenceData{
					EntryPoints: []*calendar.EntryPoint{
						{
							EntryPointType: "video",
							Uri:            "https://zoom.us/j/123456789",
						},
					},
				},
			},
			want: "https://zoom.us/j/123456789",
		},
		{
			name: "Zoom URL in description",
			item: &calendar.Event{
				Description: "Join Zoom meeting: https://zoom.us/j/123456789",
			},
			want: "https://zoom.us/j/123456789",
		},
		{
			name: "Google Meet URL in description",
			item: &calendar.Event{
				Description: "Meeting link: https://meet.google.com/abc-defg-hij",
			},
			want: "https://meet.google.com/abc-defg-hij",
		},
		{
			name: "Teams URL in location",
			item: &calendar.Event{
				Location: "https://teams.microsoft.com/l/meetup-join/19%3ameeting_abc",
			},
			want: "https://teams.microsoft.com/l/meetup-join/19%3ameeting_abc",
		},
		{
			name: "WebEx URL in description",
			item: &calendar.Event{
				Description: "WebEx: https://example.webex.com/meet/123",
			},
			want: "https://example.webex.com/meet/123",
		},
		{
			name: "no meeting URL",
			item: &calendar.Event{
				Description: "Regular meeting with no link",
			},
			want: "",
		},
		{
			name: "hangout link takes precedence",
			item: &calendar.Event{
				HangoutLink: "https://meet.google.com/abc-defg-hij",
				Description: "Also has Zoom: https://zoom.us/j/123456789",
			},
			want: "https://meet.google.com/abc-defg-hij",
		},
	}

	for _, tt := range tests {
		tt := tt // Capture loop variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := extractMeetingURL(tt.item)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("extractMeetingURL() mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestDetectConflicts(t *testing.T) {
	t.Parallel()
	baseTime := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		events []Event
		want   []bool // expected HasConflict for each event
	}{
		{
			name: "no conflicts",
			events: []Event{
				{
					ID:    "event1",
					Start: baseTime.Format(time.RFC3339),
					End:   baseTime.Add(time.Hour).Format(time.RFC3339),
				},
				{
					ID:    "event2",
					Start: baseTime.Add(2 * time.Hour).Format(time.RFC3339),
					End:   baseTime.Add(3 * time.Hour).Format(time.RFC3339),
				},
			},
			want: []bool{false, false},
		},
		{
			name: "overlapping events",
			events: []Event{
				{
					ID:    "event1",
					Start: baseTime.Format(time.RFC3339),
					End:   baseTime.Add(time.Hour).Format(time.RFC3339),
				},
				{
					ID:    "event2",
					Start: baseTime.Add(30 * time.Minute).Format(time.RFC3339),
					End:   baseTime.Add(90 * time.Minute).Format(time.RFC3339),
				},
			},
			want: []bool{true, true},
		},
		{
			name: "adjacent events (no conflict)",
			events: []Event{
				{
					ID:    "event1",
					Start: baseTime.Format(time.RFC3339),
					End:   baseTime.Add(time.Hour).Format(time.RFC3339),
				},
				{
					ID:    "event2",
					Start: baseTime.Add(time.Hour).Format(time.RFC3339),
					End:   baseTime.Add(2 * time.Hour).Format(time.RFC3339),
				},
			},
			want: []bool{false, false},
		},
		{
			name: "event completely inside another",
			events: []Event{
				{
					ID:    "event1",
					Start: baseTime.Format(time.RFC3339),
					End:   baseTime.Add(2 * time.Hour).Format(time.RFC3339),
				},
				{
					ID:    "event2",
					Start: baseTime.Add(30 * time.Minute).Format(time.RFC3339),
					End:   baseTime.Add(time.Hour).Format(time.RFC3339),
				},
			},
			want: []bool{true, true},
		},
		{
			name: "three way conflict",
			events: []Event{
				{
					ID:    "event1",
					Start: baseTime.Format(time.RFC3339),
					End:   baseTime.Add(time.Hour).Format(time.RFC3339),
				},
				{
					ID:    "event2",
					Start: baseTime.Add(30 * time.Minute).Format(time.RFC3339),
					End:   baseTime.Add(90 * time.Minute).Format(time.RFC3339),
				},
				{
					ID:    "event3",
					Start: baseTime.Add(45 * time.Minute).Format(time.RFC3339),
					End:   baseTime.Add(2 * time.Hour).Format(time.RFC3339),
				},
			},
			want: []bool{true, true, true},
		},
		{
			name:   "empty events list",
			events: []Event{},
			want:   []bool{},
		},
		{
			name: "single event",
			events: []Event{
				{
					ID:    "event1",
					Start: baseTime.Format(time.RFC3339),
					End:   baseTime.Add(time.Hour).Format(time.RFC3339),
				},
			},
			want: []bool{false},
		},
		{
			name: "invalid time format (should not panic)",
			events: []Event{
				{
					ID:    "event1",
					Start: "invalid-time",
					End:   baseTime.Add(time.Hour).Format(time.RFC3339),
				},
				{
					ID:    "event2",
					Start: baseTime.Format(time.RFC3339),
					End:   "invalid-time",
				},
			},
			want: []bool{false, false}, // Should gracefully handle parse errors
		},
	}

	for _, tt := range tests {
		tt := tt // Capture loop variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Make a copy to avoid modifying the original
			events := make([]Event, len(tt.events))
			copy(events, tt.events)

			detectConflicts(events)

			if len(events) != len(tt.want) {
				t.Fatalf("detectConflicts() events length = %v, want %v", len(events), len(tt.want))
			}

			gotConflicts := make([]bool, len(events))
			for i := range events {
				gotConflicts[i] = events[i].HasConflict
			}
			if diff := cmp.Diff(gotConflicts, tt.want); diff != "" {
				t.Errorf("detectConflicts() HasConflict mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestDetectConflicts_InvalidTimeFormat(t *testing.T) {
	t.Parallel()

	// Test that invalid time formats don't cause panics
	events := []Event{
		{
			ID:    "event1",
			Start: "not-a-time",
			End:   "also-not-a-time",
		},
		{
			ID:    "event2",
			Start: "also-not-a-time",
			End:   "not-a-time",
		},
	}

	// Should not panic
	detectConflicts(events)

	// Events should not be marked as conflicting due to parse errors
	if events[0].HasConflict || events[1].HasConflict {
		t.Error("detectConflicts() should not mark events as conflicting when times are invalid")
	}
}
