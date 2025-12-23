# gcal - Google Calendar CLI

A command-line tool for fetching and managing Google Calendar events. Perfect for automation, status bars, and scripting.

## Features

- 🔐 OAuth2 authentication with Google Calendar
- 📅 Fetch today's events or upcoming events within N hours
- 👥 Filter events (only accepted events with attendees)
- 🔗 Automatic meeting URL extraction (Zoom, Google Meet, Teams, WebEx)
- ⚠️ Conflict detection for overlapping events
- 📊 JSON output for easy parsing and integration
- 📋 List all available calendars

## Installation

### From Source

```bash
git clone https://github.com/jima/gcal.git
cd gcal
go build -o gcal ./cmd/gcal
sudo mv gcal /usr/local/bin/
```

### Using Go Install

```bash
go install github.com/jima/gcal/cmd/gcal@latest
```

## Quick Start

### 1. Set Up OAuth Credentials

First, you need to create OAuth credentials in the [Google Cloud Console](https://console.cloud.google.com/):

1. Create a new project or select an existing one
2. Enable the Google Calendar API
3. Go to "Credentials" → "Create Credentials" → "OAuth client ID"
4. Choose "Desktop app" as the application type
5. Download the credentials JSON file

### 2. Configure Credentials

Create the config directory and save your credentials:

```bash
mkdir -p ~/.config/gcal
# Copy your downloaded credentials file to:
# ~/.config/gcal/gcal-credentials.json
```

The credentials file should have this format:

```json
{
  "clientId": "your-client-id.apps.googleusercontent.com",
  "clientSecret": "your-client-secret"
}
```

### 3. Authorize

Run the authorization command:

```bash
gcal auth
```

This will:
- Open your browser for Google OAuth consent
- Save the token to `~/.local/share/gcal/gcal-tokens.json`

### 4. Check Status

Verify everything is set up correctly:

```bash
gcal status
```

### 5. Fetch Events

Get your upcoming events:

```bash
gcal events
```

## Commands

### `gcal auth`

Authorize with Google Calendar. Run this once to set up authentication.

**Options:**
- `-p, --port <port>` - Callback port for OAuth flow (default: 8085)

**Example:**
```bash
gcal auth
gcal auth --port 8086
```

### `gcal status`

Check if Google Calendar is configured and authorized.

**Output:**
```json
{
  "success": true,
  "configured": true,
  "authorized": true,
  "message": "Google Calendar is configured and authorized"
}
```

### `gcal events`

Fetch calendar events. By default, fetches events within the next 48 hours.

**Options:**
- `-H, --hours <N>` - Fetch events within next N hours (default: 48)
- `--calendars <ids>` - Comma-separated calendar IDs (default: primary)

**Examples:**
```bash
# Get events for next 48 hours (default)
gcal events

# Get events for next 24 hours
gcal events --hours 24

# Get today's events only (use 0 hours)
gcal events --hours 0

# Get events from specific calendars
gcal events --calendars "primary,calendar-id-1,calendar-id-2"
```

**Output:**
```json
{
  "success": true,
  "lastSync": "2024-01-15T10:30:00Z",
  "events": [
    {
      "id": "event-id-123",
      "title": "Team Meeting",
      "start": "2024-01-15T14:00:00Z",
      "end": "2024-01-15T15:00:00Z",
      "attendees": ["Alice", "Bob"],
      "attendeeCount": 2,
      "meetingUrl": "https://meet.google.com/abc-defg-hij",
      "hasConflict": false,
      "responseStatus": "accepted"
    }
  ]
}
```

### `gcal calendars`

List all calendars you have access to.

**Example:**
```bash
gcal calendars
```

**Output:**
```json
{
  "success": true,
  "calendars": [
    {
      "id": "primary",
      "summary": "Your Name",
      "primary": true
    },
    {
      "id": "calendar-id-123",
      "summary": "Work Calendar",
      "primary": false
    }
  ]
}
```

### `gcal logout`

Remove saved OAuth token. You'll need to run `gcal auth` again after this.

**Example:**
```bash
gcal logout
```

## Event Filtering

The tool automatically filters events to show only relevant meetings:

- ✅ Only events you've **accepted** (`responseStatus: "accepted"`)
- ✅ Only events with **attendees** (filters out personal events, focus time, etc.)
- ❌ Skips **cancelled** events
- ❌ Skips **all-day** events (no specific time)

## Meeting URL Detection

The tool automatically extracts meeting URLs from:

- Google Meet (Hangout links)
- Zoom
- Microsoft Teams
- WebEx

URLs are extracted from:
- Event hangout links
- Conference data
- Event description
- Event location

## Conflict Detection

Events that overlap in time are automatically marked with `"hasConflict": true`. This helps identify scheduling conflicts.

## File Locations

- **Credentials:** `~/.config/gcal/gcal-credentials.json`
- **Tokens:** `~/.local/share/gcal/gcal-tokens.json`

The tool respects the XDG Base Directory Specification:
- Config: `$XDG_CONFIG_HOME/gcal/` (default: `~/.config/gcal/`)
- Data: `$XDG_DATA_HOME/gcal/` (default: `~/.local/share/gcal/`)

## Use Cases

### Status Bar Integration

```bash
# Get next meeting
gcal events --hours 24 | jq -r '.events[0].title'
```

### Meeting Reminder Script

```bash
#!/bin/bash
EVENTS=$(gcal events --hours 1)
NEXT_MEETING=$(echo "$EVENTS" | jq -r '.events[0]')
if [ "$NEXT_MEETING" != "null" ]; then
  TITLE=$(echo "$NEXT_MEETING" | jq -r '.title')
  URL=$(echo "$NEXT_MEETING" | jq -r '.meetingUrl')
  echo "Upcoming: $TITLE"
  [ -n "$URL" ] && echo "Join: $URL"
fi
```

### Check for Conflicts

```bash
gcal events --hours 48 | jq '.events[] | select(.hasConflict == true)'
```

### List All Calendars

```bash
gcal calendars | jq -r '.calendars[] | "\(.id): \(.summary)"'
```

## Troubleshooting

### "credentials not found"

Make sure you've created `~/.config/gcal/gcal-credentials.json` with your OAuth credentials.

### "no token found - run 'gcal auth' first"

Run `gcal auth` to authorize the application.

### "authorization timeout"

The OAuth flow timed out. Try running `gcal auth` again. Make sure port 8085 (or your specified port) is available.

### "token_expired"

Your refresh token may have been revoked. Run `gcal logout` and then `gcal auth` to re-authorize.

### Port Already in Use

If port 8085 is in use, specify a different port:

```bash
gcal auth --port 8086
```

## Error Codes

The tool uses structured error codes in JSON responses:

- `not_configured` - Credentials or token not found
- `token_expired` - OAuth token expired and couldn't be refreshed
- `network_error` - Network connectivity issue
- `api_error` - Google Calendar API error

## Requirements

- Go 1.23+ (for building from source)
- OAuth credentials from Google Cloud Console
- Google Calendar API enabled in your Google Cloud project

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

