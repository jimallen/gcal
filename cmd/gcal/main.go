package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/jima/gcal"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "gcal",
	Short: "Google Calendar CLI",
	Long:  "A command-line tool for fetching and managing Google Calendar events.",
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authorize with Google Calendar",
	Long:  "Run the OAuth2 flow to authorize gcal with your Google account.",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")

		creds, err := gcal.LoadCredentials()
		if err != nil {
			return outputError("not_configured", err.Error())
		}

		if err := gcal.RunAuthFlow(creds, port); err != nil {
			return outputError("auth_error", err.Error())
		}

		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check authorization status",
	Long:  "Check if Google Calendar is configured and authorized.",
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, credErr := gcal.LoadCredentials()
		token, tokenErr := gcal.LoadToken()

		response := map[string]interface{}{
			"success":    credErr == nil && tokenErr == nil && token != nil,
			"configured": credErr == nil && creds != nil,
			"authorized": tokenErr == nil && token != nil,
		}

		if credErr != nil {
			response["message"] = "Credentials not configured: " + credErr.Error()
		} else if tokenErr != nil {
			response["message"] = "Token error: " + tokenErr.Error()
		} else if token == nil {
			response["message"] = "Not authorized - run 'gcal auth' first"
		} else {
			response["message"] = "Google Calendar is configured and authorized"
		}

		return outputJSON(response)
	},
}

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Fetch calendar events",
	Long:  "Fetch calendar events. By default, fetches events within the next 48 hours.",
	RunE: func(cmd *cobra.Command, args []string) error {
		hours, _ := cmd.Flags().GetInt("hours")
		days, _ := cmd.Flags().GetInt("days")
		calendarsStr, _ := cmd.Flags().GetString("calendars")

		// Days flag takes precedence over hours
		if cmd.Flags().Changed("days") {
			hours = days * 24
		}

		var calendarIDs []string
		if calendarsStr != "" {
			calendarIDs = strings.Split(calendarsStr, ",")
			for i := range calendarIDs {
				calendarIDs[i] = strings.TrimSpace(calendarIDs[i])
			}
		}

		ctx := context.Background()
		var response gcal.Response

		if hours == 0 {
			response = gcal.FetchTodayEvents(ctx, calendarIDs)
		} else {
			response = gcal.FetchUpcomingEvents(ctx, calendarIDs, hours)
		}

		return outputJSON(response)
	},
}

var calendarsCmd = &cobra.Command{
	Use:   "calendars",
	Short: "List all calendars",
	Long:  "List all calendars you have access to.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		response := gcal.ListCalendars(ctx)
		return outputJSON(response)
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove saved OAuth token",
	Long:  "Remove the saved OAuth token. You'll need to run 'gcal auth' again after this.",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return outputError("error", "failed to get home directory: "+err.Error())
		}

		tokenPath := home + "/.local/share/gcal/gcal-tokens.json"
		if err := os.Remove(tokenPath); err != nil {
			if os.IsNotExist(err) {
				return outputJSON(map[string]interface{}{
					"success": true,
					"message": "No token found - already logged out",
				})
			}
			return outputError("error", "failed to remove token: "+err.Error())
		}

		return outputJSON(map[string]interface{}{
			"success": true,
			"message": "Token removed successfully",
		})
	},
}

func init() {
	authCmd.Flags().IntP("port", "p", gcal.DefaultCallbackPort, "Callback port for OAuth flow")
	eventsCmd.Flags().IntP("hours", "H", 48, "Fetch events within next N hours (0 for today only)")
	eventsCmd.Flags().IntP("days", "d", 0, "Fetch events within next N days (takes precedence over --hours)")
	eventsCmd.Flags().String("calendars", "", "Comma-separated calendar IDs (default: primary)")

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(eventsCmd)
	rootCmd.AddCommand(calendarsCmd)
	rootCmd.AddCommand(logoutCmd)
}

func outputJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func outputError(code, message string) error {
	return outputJSON(map[string]interface{}{
		"success": false,
		"error":   code,
		"message": message,
	})
}
