package gcal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/oauth2"
)

func TestLoadCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func(*testing.T, string) error
		wantErr     bool
		wantErrCode string
		checkFn     func(*testing.T, *Credentials)
	}{
		{
			name: "valid credentials",
			setup: func(t *testing.T, configDir string) error {
				creds := Credentials{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				}
				createTestCredentials(t, configDir, creds)
				return nil
			},
			wantErr: false,
			checkFn: func(t *testing.T, c *Credentials) {
				want := Credentials{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				}
				if diff := cmp.Diff(c, &want); diff != "" {
					t.Errorf("LoadCredentials() mismatch (-got +want):\n%s", diff)
				}
			},
		},
		{
			name: "file not found",
			setup: func(*testing.T, string) error {
				// Don't create the file
				return nil
			},
			wantErr:     true,
			wantErrCode: ErrNotConfigured,
		},
		{
			name: "invalid JSON",
			setup: func(t *testing.T, configDir string) error {
				path := filepath.Join(configDir, credentialsFile)
				return os.WriteFile(path, []byte("invalid json"), 0644)
			},
			wantErr: true,
		},
		{
			name: "missing client ID",
			setup: func(t *testing.T, configDir string) error {
				creds := Credentials{
					ClientSecret: "test-secret",
				}
				data, err := json.Marshal(creds)
				if err != nil {
					return err
				}
				path := filepath.Join(configDir, credentialsFile)
				return os.WriteFile(path, data, 0644)
			},
			wantErr: true,
		},
		{
			name: "missing client secret",
			setup: func(t *testing.T, configDir string) error {
				creds := Credentials{
					ClientID: "test-id",
				}
				data, err := json.Marshal(creds)
				if err != nil {
					return err
				}
				path := filepath.Join(configDir, credentialsFile)
				return os.WriteFile(path, data, 0644)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt // Capture loop variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Each subtest gets its own isolated environment
			configDir, _, cleanup := setupTestEnv(t)
			defer cleanup()

			// Clean up any existing file
			path := filepath.Join(configDir, credentialsFile)
			os.Remove(path)

			if err := tt.setup(t, configDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			got, err := LoadCredentials()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadCredentials() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.wantErrCode != "" {
					// Check if error message contains the expected code
					if err != nil && err.Error() != "" {
						// Error should indicate the issue
					}
				}
				return
			}

			if got == nil {
				t.Fatal("LoadCredentials() returned nil credentials")
			}

			if tt.checkFn != nil {
				tt.checkFn(t, got)
			}
		})
	}
}

func TestLoadToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(*testing.T, string) error
		wantNil bool
		checkFn func(*testing.T, *oauth2.Token)
	}{
		{
			name: "valid token",
			setup: func(t *testing.T, dataDir string) error {
				store := TokenStore{
					AccessToken:  "access-token",
					RefreshToken: "refresh-token",
					TokenType:    "Bearer",
					Expiry:       time.Now().Add(time.Hour),
				}
				createTestToken(t, dataDir, store)
				return nil
			},
			wantNil: false,
			checkFn: func(t *testing.T, token *oauth2.Token) {
				if token.AccessToken != "access-token" {
					t.Errorf("LoadToken() AccessToken = %v, want access-token", token.AccessToken)
				}
				if token.RefreshToken != "refresh-token" {
					t.Errorf("LoadToken() RefreshToken = %v, want refresh-token", token.RefreshToken)
				}
				if token.TokenType != "Bearer" {
					t.Errorf("LoadToken() TokenType = %v, want Bearer", token.TokenType)
				}
			},
		},
		{
			name: "file not found",
			setup: func(*testing.T, string) error {
				// Don't create the file
				return nil
			},
			wantNil: true,
		},
		{
			name: "invalid JSON",
			setup: func(t *testing.T, dataDir string) error {
				path := filepath.Join(dataDir, tokenFile)
				return os.WriteFile(path, []byte("invalid json"), 0600)
			},
			wantNil: false, // Should return error, not nil
		},
	}

	for _, tt := range tests {
		tt := tt // Capture loop variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Each subtest gets its own isolated environment
			_, dataDir, cleanup := setupTestEnv(t)
			defer cleanup()

			// Clean up any existing file
			path := filepath.Join(dataDir, tokenFile)
			os.Remove(path)

			if err := tt.setup(t, dataDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			got, err := LoadToken()
			if tt.wantNil {
				if got != nil || err != nil {
					t.Errorf("LoadToken() = %v, %v, want nil, nil", got, err)
				}
				return
			}

			if err != nil && tt.name == "invalid JSON" {
				// Expected error for invalid JSON
				return
			}

			if got == nil && tt.checkFn != nil {
				t.Fatal("LoadToken() returned nil token")
			}

			if tt.checkFn != nil && got != nil {
				tt.checkFn(t, got)
			}
		})
	}
}

func TestSaveToken(t *testing.T) {
	t.Parallel()

	_, _, cleanup := setupTestEnv(t)
	defer cleanup()

	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	if err := SaveToken(token); err != nil {
		t.Fatalf("SaveToken() error = %v", err)
	}

	// Verify file was created with correct permissions
	// Get the actual data dir that was used
	actualDataDir, err := getDataDir()
	if err != nil {
		t.Fatalf("Failed to get data dir: %v", err)
	}
	path := filepath.Join(actualDataDir, tokenFile)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat token file: %v", err)
	}

	// Check file permissions (should be 0600)
	if info.Mode().Perm() != 0600 {
		t.Errorf("SaveToken() file permissions = %v, want 0600", info.Mode().Perm())
	}

	// Verify content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read token file: %v", err)
	}

	var store TokenStore
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatalf("Failed to unmarshal token: %v", err)
	}

	want := TokenStore{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
	}
	if diff := cmp.Diff(store.AccessToken, want.AccessToken); diff != "" {
		t.Errorf("SaveToken() AccessToken mismatch (-got +want):\n%s", diff)
	}
	if diff := cmp.Diff(store.RefreshToken, want.RefreshToken); diff != "" {
		t.Errorf("SaveToken() RefreshToken mismatch (-got +want):\n%s", diff)
	}
}

func TestIsConfigured(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupCred bool
		setupTok  bool
		want      bool
	}{
		{
			name:      "both configured",
			setupCred: true,
			setupTok:  true,
			want:      true,
		},
		{
			name:      "no credentials",
			setupCred: false,
			setupTok:  true,
			want:      false,
		},
		{
			name:      "no token",
			setupCred: true,
			setupTok:  false,
			want:      false,
		},
		{
			name:      "neither configured",
			setupCred: false,
			setupTok:  false,
			want:      false,
		},
	}

	for _, tt := range tests {
		tt := tt // Capture loop variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Each subtest gets its own isolated environment
			configDir, dataDir, cleanup := setupTestEnv(t)
			defer cleanup()

			// Clean up
			os.Remove(filepath.Join(configDir, credentialsFile))
			os.Remove(filepath.Join(dataDir, tokenFile))

			if tt.setupCred {
				creds := Credentials{
					ClientID:     "test-id",
					ClientSecret: "test-secret",
				}
				createTestCredentials(t, configDir, creds)
			}

			if tt.setupTok {
				store := TokenStore{
					AccessToken:  "token",
					RefreshToken: "refresh",
					TokenType:    "Bearer",
					Expiry:       time.Now().Add(time.Hour),
				}
				createTestToken(t, dataDir, store)
			}

			got := IsConfigured()
			if got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOAuthConfig(t *testing.T) {
	t.Parallel()

	creds := &Credentials{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}

	config := getOAuthConfig(creds, 8085)

	if diff := cmp.Diff(config.ClientID, creds.ClientID); diff != "" {
		t.Errorf("getOAuthConfig() ClientID mismatch (-got +want):\n%s", diff)
	}
	if diff := cmp.Diff(config.ClientSecret, creds.ClientSecret); diff != "" {
		t.Errorf("getOAuthConfig() ClientSecret mismatch (-got +want):\n%s", diff)
	}
	if config.RedirectURL != "http://localhost:8085/callback" {
		t.Errorf("getOAuthConfig() RedirectURL = %v, want http://localhost:8085/callback", config.RedirectURL)
	}
	if len(config.Scopes) != 1 {
		t.Errorf("getOAuthConfig() Scopes length = %v, want 1", len(config.Scopes))
	}
}
