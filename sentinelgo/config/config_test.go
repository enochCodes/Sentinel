package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadAppConfig tests the LoadAppConfig function.
func TestLoadAppConfig(t *testing.T) {
	// Test loading the existing sentinel.yaml (assuming it's in ../config relative to this test file)
	// This relative path might be tricky depending on execution directory of `go test`.
	// It's often better to create a temporary config file for tests or embed it.
	// For now, let's assume sentinel.yaml exists in a known relative path for dev environment.
	// A safer way would be to create a temp config file.

	// Create a temporary sentinel.yaml for testing
	tempDir := t.TempDir()
	tempYAMLPath := filepath.Join(tempDir, "sentinel.yaml")
	yamlContent := `
defaultheaders:
  User-Agent: "TestAgent/1.0"
  X-Custom-Header: "TestValue"
customcookies:
  - name: "test_session"
    value: "testcookie123"
maxretries: 5
riskthreshold: 60.0
apikeys:
  testservice: "testapikey"
`
	err := os.WriteFile(tempYAMLPath, []byte(yamlContent), 0600)
	require.NoError(t, err, "Failed to write temp sentinel.yaml for testing")

	cfg, err := LoadAppConfig(tempYAMLPath)
	require.NoError(t, err, "LoadAppConfig failed for existing file")
	require.NotNil(t, cfg, "Config should not be nil for existing file")

	assert.Equal(t, 5, cfg.MaxRetries, "MaxRetries should match value in test YAML")
	assert.Equal(t, 60.0, cfg.RiskThreshold, "RiskThreshold should match value in test YAML")
	assert.Equal(t, "TestAgent/1.0", cfg.DefaultHeaders["User-Agent"], "User-Agent should match")
	assert.Equal(t, "testapikey", cfg.APIKeys["testservice"], "API key should match")
	require.Len(t, cfg.CustomCookies, 1, "Should be one custom cookie")
	assert.Equal(t, "test_session", cfg.CustomCookies[0].Name, "Cookie name should match")

	// Test loading a non-existent YAML file
	nonExistentPath := filepath.Join(tempDir, "non_existent.yaml")
	defaultCfg, err := LoadAppConfig(nonExistentPath)
	require.NoError(t, err, "LoadAppConfig should return no error for non-existent file")
	require.NotNil(t, defaultCfg, "Default config should not be nil")

	// Check default values (as defined in LoadAppConfig)
	assert.Equal(t, 3, defaultCfg.MaxRetries, "Default MaxRetries should be 3")
	assert.Equal(t, 75.0, defaultCfg.RiskThreshold, "Default RiskThreshold should be 75.0")
	assert.NotNil(t, defaultCfg.DefaultHeaders, "DefaultHeaders map should be initialized")
	assert.NotNil(t, defaultCfg.APIKeys, "APIKeys map should be initialized")
	assert.Empty(t, defaultCfg.CustomCookies, "CustomCookies should be empty by default")
}

// TestSessionState tests saving and loading of SessionState.
func TestSessionState(t *testing.T) {
	tempDir := t.TempDir()
	// On some systems, UserHomeDir might not be writable by tests or might be unexpected.
	// Forcing state file into tempDir for this test.
	// We need to ensure that the SaveSessionState function correctly handles the specific path `~/.sentinel/state.json`
	// by either mocking UserHomeDir or by testing that specific case separately if the test env allows.
	// For this unit test, we'll test the general save/load mechanism with a temp file.

	stateFilePath := filepath.Join(tempDir, "test_state.json")

	originalState := &SessionState{
		LastTargetURL:   "http://example.com/test_target",
		LastReason:      "test_reason",
		LastProxyConfig: "proxies.csv",
	}

	// Test SaveSessionState
	err := SaveSessionState(stateFilePath, originalState)
	require.NoError(t, err, "SaveSessionState failed")

	// Test LoadSessionState
	loadedState, err := LoadSessionState(stateFilePath)
	require.NoError(t, err, "LoadSessionState failed")
	require.NotNil(t, loadedState, "Loaded state should not be nil")

	assert.Equal(t, originalState.LastTargetURL, loadedState.LastTargetURL)
	assert.Equal(t, originalState.LastReason, loadedState.LastReason)
	assert.Equal(t, originalState.LastProxyConfig, loadedState.LastProxyConfig)

	// Test LoadSessionState for non-existent file
	nonExistentStatePath := filepath.Join(tempDir, "non_existent_state.json")
	defaultState, err := LoadSessionState(nonExistentStatePath)
	require.NoError(t, err, "LoadSessionState for non-existent file should not return an error")
	require.NotNil(t, defaultState, "Default state should not be nil")
	assert.Empty(t, defaultState.LastTargetURL, "Default LastTargetURL should be empty")

	// Test SaveSessionState with directory creation (simulating ~/.sentinel)
	// This part is tricky because we want to test the os.UserHomeDir part.
	// We can't easily mock os.UserHomeDir without build tags or interfaces.
	// For now, we trust that if filepath.Dir works and os.MkdirAll works, it's okay.
	// The test for `~/.sentinel` path creation itself is more of an integration/manual test.
	// Let's test if it can create a subdir in the tempDir.
	nestedDir := filepath.Join(tempDir, "newdir", "sub")
	nestedStateFilePath := filepath.Join(nestedDir, "nested_state.json")

	err = SaveSessionState(nestedStateFilePath, originalState)
	require.NoError(t, err, "SaveSessionState should create nested directories")
	_, err = os.Stat(nestedStateFilePath)
	require.NoError(t, err, "Nested state file should exist")

}

// TestLoadAppConfig_ActualFile tests against the actual sentinel.yaml if present
// This test is more of an integration test and might be fragile.
func TestLoadAppConfig_ActualFile(t *testing.T) {
	// Path relative to the project root: "sentinelgo/config/sentinel.yaml"
	// To make this work when `go test ./...` is run from `sentinelgo/` dir:
	actualConfigPath := "sentinel.yaml" // Assumes test is run from sentinelgo/config or sentinel.yaml is copied

	// Check if the actual config file exists, otherwise skip.
	// This requires sentinel.yaml to be in the same dir as config_test.go or CWD to be config/
	// For `go test ./...` from sentinelgo root, path should be "config/sentinel.yaml"
	if _, err := os.Stat("../config/sentinel.yaml"); os.IsNotExist(err) {
		// If running from sentinelgo/config, path is "sentinel.yaml"
		if _, errRel := os.Stat("sentinel.yaml"); os.IsNotExist(errRel) {
			t.Skip("Skipping test against actual sentinel.yaml: file not found at expected locations.")
			return
		}
		actualConfigPath = "sentinel.yaml"
	} else {
		actualConfigPath = "../config/sentinel.yaml"
	}

	cfg, err := LoadAppConfig(actualConfigPath)
	require.NoError(t, err, "Loading actual sentinel.yaml failed")
	require.NotNil(t, cfg, "Actual config should not be nil")

	// Assert some expected default values if the file is the default one.
	// These might change, making the test brittle.
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 80.5, cfg.RiskThreshold) // From the default file created in earlier step
	assert.Equal(t, "SentinelGo Client v1.0", cfg.DefaultHeaders["User-Agent"])
}
