package gcal

import "time"

// Event represents a calendar event for JSON output
type Event struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Start          string   `json:"start"` // ISO8601
	End            string   `json:"end"`   // ISO8601
	Attendees      []string `json:"attendees"`
	AttendeeCount  int      `json:"attendeeCount"`
	MeetingURL     string   `json:"meetingUrl,omitempty"`
	HasConflict    bool     `json:"hasConflict"`
	ResponseStatus string   `json:"responseStatus"`
}

// Response is the JSON output for gcal events
type Response struct {
	Success  bool    `json:"success"`
	LastSync string  `json:"lastSync,omitempty"` // ISO8601
	Events   []Event `json:"events,omitempty"`
	Error    string  `json:"error,omitempty"`   // machine-readable code
	Message  string  `json:"message,omitempty"` // human-readable
}

// TokenStore holds OAuth tokens for persistence
type TokenStore struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
}

// Credentials holds OAuth client credentials
type Credentials struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// CalendarInfo represents a calendar for listing
type CalendarInfo struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Primary bool   `json:"primary"`
}

// CalendarsResponse is the JSON output for gcal calendars
type CalendarsResponse struct {
	Success   bool           `json:"success"`
	Calendars []CalendarInfo `json:"calendars,omitempty"`
	Error     string         `json:"error,omitempty"`
	Message   string         `json:"message,omitempty"`
}

// Error codes
const (
	ErrNotConfigured = "not_configured"
	ErrTokenExpired  = "token_expired"
	ErrNetworkError  = "network_error"
	ErrAPIError      = "api_error"
)

// NewErrorResponse creates a structured error response
func NewErrorResponse(code, message string) Response {
	return Response{
		Success: false,
		Error:   code,
		Message: message,
	}
}

// NewSuccessResponse creates a successful events response
func NewSuccessResponse(events []Event) Response {
	return Response{
		Success:  true,
		LastSync: time.Now().Format(time.RFC3339),
		Events:   events,
	}
}
