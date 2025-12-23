// Package gcal provides OAuth2 authentication and token management for Google Calendar API access.
package gcal

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

const (
	credentialsFile     = "gcal-credentials.json"
	tokenFile           = "gcal-tokens.json"
	DefaultCallbackPort = 8085
)

// getConfigDir returns ~/.config/gcal
func getConfigDir() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "gcal"), nil
}

// getDataDir returns ~/.local/share/gcal
func getDataDir() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	dir := filepath.Join(dataHome, "gcal")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// LoadCredentials loads OAuth client credentials from config
func LoadCredentials() (*Credentials, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, fmt.Errorf("get config dir: %w", err)
	}

	path := filepath.Join(configDir, credentialsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("credentials not found at %s - please configure OAuth credentials", path)
		}
		return nil, fmt.Errorf("read credentials: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	if creds.ClientID == "" || creds.ClientSecret == "" {
		return nil, fmt.Errorf("credentials file missing clientId or clientSecret")
	}

	return &creds, nil
}

// getOAuthConfig creates OAuth2 config from credentials
func getOAuthConfig(creds *Credentials, port int) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  fmt.Sprintf("http://localhost:%d/callback", port),
		Scopes:       []string{calendar.CalendarReadonlyScope},
	}
}

// LoadToken loads saved OAuth token from data dir
func LoadToken() (*oauth2.Token, error) {
	dataDir, err := getDataDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dataDir, tokenFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No token yet
		}
		return nil, fmt.Errorf("read token: %w", err)
	}

	var store TokenStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	return &oauth2.Token{
		AccessToken:  store.AccessToken,
		RefreshToken: store.RefreshToken,
		TokenType:    store.TokenType,
		Expiry:       store.Expiry,
	}, nil
}

// SaveToken saves OAuth token to data dir with 0600 permissions
func SaveToken(token *oauth2.Token) error {
	dataDir, err := getDataDir()
	if err != nil {
		return err
	}

	store := TokenStore{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}

	path := filepath.Join(dataDir, tokenFile)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write token: %w", err)
	}

	return nil
}

// RunAuthFlow performs the OAuth browser flow and saves the token
func RunAuthFlow(creds *Credentials, port int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if port <= 0 {
		port = DefaultCallbackPort
	}
	config := getOAuthConfig(creds, port)

	// Create a channel to receive the auth code
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Start local HTTP server for callback
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("start callback server: %w", err)
	}

	// Use a new mux to avoid global handler registration
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code in callback")
			http.Error(w, "No code received", http.StatusBadRequest)
			return
		}
		codeChan <- code
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body><h1>Authorization successful!</h1><p>You can close this tab and return to the terminal.</p></body></html>`)
	})

	server := &http.Server{
		Handler: mux,
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Generate auth URL and open browser
	authURL := config.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("Opening browser for authorization...\n")
	fmt.Printf("If browser doesn't open, visit:\n%s\n\n", authURL)

	// Try to open browser
	openBrowser(authURL)

	// Wait for callback or timeout
	var code string
	select {
	case code = <-codeChan:
		// Success
	case err := <-errChan:
		server.Shutdown(ctx)
		return err
	case <-ctx.Done():
		server.Shutdown(ctx)
		return fmt.Errorf("authorization timeout - no response received")
	}

	// Gracefully shutdown server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		// Log but don't fail - we already have the code
		fmt.Fprintf(os.Stderr, "warning: failed to shutdown server gracefully: %v\n", err)
	}

	// Exchange code for token
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}

	// Save token
	if err := SaveToken(token); err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	fmt.Println("Authorization successful! Token saved.")
	return nil
}

// openBrowser opens URL in default browser
func openBrowser(url string) {
	// Fire and forget
	go func() {
		var cmd string
		var args []string

		// Determine command based on OS
		// Check for Windows first
		if os.Getenv("OS") == "Windows_NT" || os.Getenv("COMSPEC") != "" {
			cmd = "cmd"
			args = []string{"/c", "start", url}
		} else {
			// Try to detect macOS vs Linux
			// macOS typically has "open" command
			if path, err := exec.LookPath("open"); err == nil && path != "" {
				cmd = "open"
				args = []string{url}
			} else {
				// Default to xdg-open for Linux
				cmd = "xdg-open"
				args = []string{url}
			}
		}

		// Try to find the command in PATH
		path, err := exec.LookPath(cmd)
		if err != nil {
			// Silently fail if we can't find the command
			return
		}

		// Use exec.Command for better cross-platform support
		cmdObj := exec.Command(path, args...)
		cmdObj.Start() // Fire and forget - don't wait
	}()
}

// GetClient returns an authenticated HTTP client, refreshing token if needed
func GetClient(ctx context.Context) (*http.Client, error) {
	creds, err := LoadCredentials()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrNotConfigured, err)
	}

	token, err := LoadToken()
	if err != nil {
		return nil, fmt.Errorf("load token: %w", err)
	}
	if token == nil {
		return nil, fmt.Errorf("%s: no token found - run 'gcal auth' first", ErrNotConfigured)
	}

	config := getOAuthConfig(creds, DefaultCallbackPort)
	tokenSource := config.TokenSource(ctx, token)

	// Get potentially refreshed token
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrTokenExpired, err)
	}

	// Save if token was refreshed
	if newToken.AccessToken != token.AccessToken {
		if err := SaveToken(newToken); err != nil {
			// Log but don't fail - we still have a valid token
			fmt.Fprintf(os.Stderr, "warning: failed to save refreshed token: %v\n", err)
		}
	}

	return oauth2.NewClient(ctx, tokenSource), nil
}

// IsConfigured checks if credentials and token are available
func IsConfigured() bool {
	creds, err := LoadCredentials()
	if err != nil || creds == nil {
		return false
	}
	token, err := LoadToken()
	return err == nil && token != nil
}
