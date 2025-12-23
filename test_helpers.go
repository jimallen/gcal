// Package gcal provides test helpers for the gcal package tests.
// These functions are only used in test files.
package gcal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// setupTestEnv configures XDG environment variables for testing
func setupTestEnv(t *testing.T) (configDir, dataDir string, cleanup func()) {
	t.Helper() // Marks this as a test helper

	tmpDir := t.TempDir()

	// Save original environment
	originalXDGConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalXDGDataHome := os.Getenv("XDG_DATA_HOME")

	// Set test environment
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	os.Setenv("XDG_DATA_HOME", tmpDir)

	// Get actual directories
	configDir, err := getConfigDir()
	if err != nil {
		t.Fatalf("Failed to get config dir: %v", err)
	}
	dataDir, err = getDataDir()
	if err != nil {
		t.Fatalf("Failed to get data dir: %v", err)
	}

	// Create directories
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Cleanup function
	cleanup = func() {
		if originalXDGConfigHome == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", originalXDGConfigHome)
		}
		if originalXDGDataHome == "" {
			os.Unsetenv("XDG_DATA_HOME")
		} else {
			os.Setenv("XDG_DATA_HOME", originalXDGDataHome)
		}
	}

	return configDir, dataDir, cleanup
}

// createTestCredentials creates a test credentials file
func createTestCredentials(t *testing.T, configDir string, creds Credentials) string {
	t.Helper()

	path := filepath.Join(configDir, credentialsFile)
	data, err := json.Marshal(creds)
	if err != nil {
		t.Fatalf("Failed to marshal credentials: %v", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to write credentials: %v", err)
	}

	return path
}

// createTestToken creates a test token file
func createTestToken(t *testing.T, dataDir string, store TokenStore) string {
	t.Helper()

	path := filepath.Join(dataDir, tokenFile)
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatalf("Failed to marshal token: %v", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("Failed to write token: %v", err)
	}

	return path
}
