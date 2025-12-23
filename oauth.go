package gcal

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
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

	server := &http.Server{}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
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

	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
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
		server.Close()
		return err
	case <-time.After(5 * time.Minute):
		server.Close()
		return fmt.Errorf("authorization timeout - no response received")
	}

	server.Close()

	// Exchange code for token
	ctx := context.Background()
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
	// Use xdg-open on Linux
	cmd := "xdg-open"
	args := []string{url}

	// Fire and forget
	go func() {
		proc := &os.ProcAttr{
			Files: []*os.File{nil, nil, nil},
		}
		p, err := os.StartProcess("/usr/bin/xdg-open", append([]string{cmd}, args...), proc)
		if err == nil {
			p.Release()
		}
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
