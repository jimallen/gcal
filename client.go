package gcal

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Meeting URL patterns
var meetingPatterns = []*regexp.Regexp{
	regexp.MustCompile(`https://[a-z0-9.-]*zoom\.us/[^\s<>"]+`),
	regexp.MustCompile(`https://meet\.google\.com/[a-z0-9-]+`),
	regexp.MustCompile(`https://teams\.microsoft\.com/[^\s<>"]+`),
	regexp.MustCompile(`https://[a-z0-9.-]*webex\.com/[^\s<>"]+`),
}

// FetchTodayEvents fetches today's calendar events and returns structured response
func FetchTodayEvents(ctx context.Context, calendarIDs []string) Response {
	client, err := GetClient(ctx)
	if err != nil {
		return NewErrorResponse(ErrNotConfigured, err.Error())
	}

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return NewErrorResponse(ErrAPIError, "failed to create calendar service: "+err.Error())
	}

	// Default to primary calendar
	if len(calendarIDs) == 0 {
		calendarIDs = []string{"primary"}
	}

	// Get today's time range in local timezone
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	var allEvents []Event

	for _, calID := range calendarIDs {
		events, err := srv.Events.List(calID).
			TimeMin(startOfDay.Format(time.RFC3339)).
			TimeMax(endOfDay.Format(time.RFC3339)).
			SingleEvents(true).
			OrderBy("startTime").
			Do()

		if err != nil {
			// Continue with other calendars if one fails
			continue
		}

		for _, item := range events.Items {
			event := convertEvent(item)
			if event != nil {
				allEvents = append(allEvents, *event)
			}
		}
	}

	// Sort by start time
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Start < allEvents[j].Start
	})

	// Detect conflicts
	detectConflicts(allEvents)

	return NewSuccessResponse(allEvents)
}

// FetchUpcomingEvents fetches events within the next N hours
func FetchUpcomingEvents(ctx context.Context, calendarIDs []string, hours int) Response {
	client, err := GetClient(ctx)
	if err != nil {
		return NewErrorResponse(ErrNotConfigured, err.Error())
	}

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return NewErrorResponse(ErrAPIError, "failed to create calendar service: "+err.Error())
	}

	if len(calendarIDs) == 0 {
		calendarIDs = []string{"primary"}
	}

	now := time.Now()
	endTime := now.Add(time.Duration(hours) * time.Hour)

	var allEvents []Event

	for _, calID := range calendarIDs {
		events, err := srv.Events.List(calID).
			TimeMin(now.Format(time.RFC3339)).
			TimeMax(endTime.Format(time.RFC3339)).
			SingleEvents(true).
			OrderBy("startTime").
			Do()

		if err != nil {
			continue
		}

		for _, item := range events.Items {
			event := convertEvent(item)
			if event != nil {
				allEvents = append(allEvents, *event)
			}
		}
	}

	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Start < allEvents[j].Start
	})

	detectConflicts(allEvents)

	return NewSuccessResponse(allEvents)
}

// ListCalendars returns all calendars the user has access to
func ListCalendars(ctx context.Context) CalendarsResponse {
	client, err := GetClient(ctx)
	if err != nil {
		return CalendarsResponse{
			Success: false,
			Error:   ErrNotConfigured,
			Message: err.Error(),
		}
	}

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return CalendarsResponse{
			Success: false,
			Error:   ErrAPIError,
			Message: "failed to create calendar service: " + err.Error(),
		}
	}

	list, err := srv.CalendarList.List().Do()
	if err != nil {
		return CalendarsResponse{
			Success: false,
			Error:   ErrAPIError,
			Message: "failed to list calendars: " + err.Error(),
		}
	}

	var calendars []CalendarInfo
	for _, item := range list.Items {
		calendars = append(calendars, CalendarInfo{
			ID:      item.Id,
			Summary: item.Summary,
			Primary: item.Primary,
		})
	}

	return CalendarsResponse{
		Success:   true,
		Calendars: calendars,
	}
}

// convertEvent converts a Google Calendar event to our Event type
func convertEvent(item *calendar.Event) *Event {
	// Skip cancelled events
	if item.Status == "cancelled" {
		return nil
	}

	// Skip all-day events (no dateTime, only date)
	if item.Start.DateTime == "" {
		return nil
	}

	event := &Event{
		ID:    item.Id,
		Title: item.Summary,
		Start: item.Start.DateTime,
		End:   item.End.DateTime,
	}

	// Extract attendees
	for _, attendee := range item.Attendees {
		if attendee.Self {
			event.ResponseStatus = attendee.ResponseStatus
		} else if attendee.Email != "" {
			event.Attendees = append(event.Attendees, attendee.DisplayName)
			if event.Attendees[len(event.Attendees)-1] == "" {
				event.Attendees[len(event.Attendees)-1] = attendee.Email
			}
		}
	}
	event.AttendeeCount = len(event.Attendees)

	// Skip events without attendees (personal events, focus time, etc.)
	if event.AttendeeCount == 0 {
		return nil
	}

	// Skip events not accepted by user
	if event.ResponseStatus != "accepted" {
		return nil
	}

	// Extract meeting URL
	event.MeetingURL = extractMeetingURL(item)

	return event
}

// extractMeetingURL finds meeting URL from event
func extractMeetingURL(item *calendar.Event) string {
	// Check hangout link first (Google Meet)
	if item.HangoutLink != "" {
		return item.HangoutLink
	}

	// Check conference data
	if item.ConferenceData != nil {
		for _, ep := range item.ConferenceData.EntryPoints {
			if ep.EntryPointType == "video" && ep.Uri != "" {
				return ep.Uri
			}
		}
	}

	// Search in description and location
	searchIn := item.Description + " " + item.Location

	for _, pattern := range meetingPatterns {
		if match := pattern.FindString(searchIn); match != "" {
			return strings.TrimSpace(match)
		}
	}

	return ""
}

// detectConflicts marks events that overlap with each other
func detectConflicts(events []Event) {
	for i := range events {
		for j := i + 1; j < len(events); j++ {
			// Parse times
			startI, errI := time.Parse(time.RFC3339, events[i].Start)
			endI, errIEnd := time.Parse(time.RFC3339, events[i].End)
			startJ, errJ := time.Parse(time.RFC3339, events[j].Start)
			endJ, errJEnd := time.Parse(time.RFC3339, events[j].End)

			if errI != nil || errIEnd != nil || errJ != nil || errJEnd != nil {
				continue
			}

			// Check for overlap: event i ends after event j starts AND event i starts before event j ends
			if endI.After(startJ) && startI.Before(endJ) {
				events[i].HasConflict = true
				events[j].HasConflict = true
			}
		}
	}
}
