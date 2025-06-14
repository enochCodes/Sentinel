package config

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AppConfig holds the application configuration, typically loaded from `sentinel.yaml`.
// It defines settings for various aspects of the application's behavior.
type AppConfig struct {
	// DefaultHeaders is a map of HTTP headers that are applied to all outgoing report requests.
	// Example: {"User-Agent": "SentinelGo Client/1.0"}
	DefaultHeaders map[string]string `yaml:"defaultheaders"`

	// CustomCookies is a slice of http.Cookie objects that are added to all outgoing report requests.
	// These are loaded from the configuration and can be used to maintain session state or pass specific tokens.
	CustomCookies []http.Cookie `yaml:"customcookies"`

	// MaxRetries specifies the default maximum number of times a single report attempt will be retried if it fails.
	MaxRetries int `yaml:"maxretries"`

	// RiskThreshold is a percentage (0-100) used by the AI content analyzer.
	// Content scoring above this threshold may trigger special handling or logging.
	RiskThreshold float64 `yaml:"riskthreshold"`

	// APIKeys is a map to store API keys for various external services that SentinelGo might integrate with.
	// Example: {"virustotal": "your_vt_api_key_here"}
	APIKeys map[string]string `yaml:"apikeys"`
}

// SessionState holds persistent data related to user sessions or application state
// that needs to be saved and restored between application runs.
// It is typically stored in a JSON file like `~/.sentinel/state.json`.
type SessionState struct {
	// LastTargetURL stores the most recent target URL used in a reporting session.
	LastTargetURL string `json:"lasttargeturl"`

	// LastReason stores the last reason provided for a report (currently unused as "Reason" was removed from workflow).
	// This field is kept for potential future use or backward compatibility if needed.
	LastReason string `json:"lastreason"`

	// LastProxyConfig stores information about the last proxy configuration used,
	// such as a path to a proxy file or an API endpoint.
	LastProxyConfig string `json:"lastproxyconfig"`
}

// LoadAppConfig reads a YAML configuration file specified by `filePath`,
// unmarshals it into an AppConfig struct, and returns it.
// If the file does not exist, it returns a default AppConfig with predefined values
// (e.g., MaxRetries: 3, RiskThreshold: 75.0) and no error.
// Errors during file reading (other than not found) or YAML unmarshaling are returned.
func LoadAppConfig(filePath string) (*AppConfig, error) {
	// Default configuration values.
	config := &AppConfig{
		MaxRetries:     3,
		RiskThreshold:  75.0,
		DefaultHeaders: make(map[string]string),
		APIKeys:        make(map[string]string),
		CustomCookies:  []http.Cookie{}, // Ensure empty slice, not nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File not found, return the default config and no error.
			return config, nil
		}
		// Other file read error.
		return nil, err
	}

	// Unmarshal the YAML data into the config struct.
	err = yaml.Unmarshal(data, config)
	if err != nil {
		// YAML parsing error.
		return nil, err
	}

	// Ensure maps and slices are not nil if YAML parsing results in them being nil
	// (e.g. if an empty config file only has `maxretries: 5`)
	if config.DefaultHeaders == nil {
		config.DefaultHeaders = make(map[string]string)
	}
	if config.APIKeys == nil {
		config.APIKeys = make(map[string]string)
	}
	if config.CustomCookies == nil {
		config.CustomCookies = []http.Cookie{}
	}


	return config, nil
}

// LoadSessionState reads a JSON file specified by `filePath`,
// unmarshals it into a SessionState struct, and returns it.
// If the file does not exist, it returns a default (empty) SessionState struct and no error.
// Errors during file reading (other than not found) or JSON unmarshaling are returned.
func LoadSessionState(filePath string) (*SessionState, error) {
	state := &SessionState{} // Default empty state.

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File not found, return default state and no error.
			return state, nil
		}
		// Other file read error.
		return nil, err
	}

	// Unmarshal the JSON data into the state struct.
	err = json.Unmarshal(data, state)
	if err != nil {
		// JSON parsing error.
		return nil, err
	}

	return state, nil
}

// SaveSessionState marshals the provided SessionState struct to JSON and
// writes it to the file specified by `filePath`.
// It ensures that the directory for `filePath` is created if it doesn't already exist.
// Special handling is included to resolve "~/" to the user's home directory for paths like `~/.sentinel/state.json`.
// File permissions are set to 0600 for the state file and 0750 for created directories.
func SaveSessionState(filePath string, state *SessionState) error {
	dir := filepath.Dir(filePath)

	// Handle user home directory expansion for common sentinel paths.
	if strings.HasPrefix(dir, "~/") || strings.HasPrefix(dir, filepath.Join("~", ".sentinel")) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		if strings.HasPrefix(dir, "~/") {
			dir = filepath.Join(homeDir, dir[2:])
		} else { // Handles `~/.sentinel` case
			dir = filepath.Join(homeDir, ".sentinel")
		}
	}

	// Create directory if it doesn't exist.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0750) // Use 0750 for directory permissions.
		if err != nil {
			return err
		}
	}

	// Marshal SessionState to JSON with indentation.
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	// Write JSON data to the file.
	return os.WriteFile(filePath, data, 0600) // Use 0600 for user-private file permissions.
}

// SaveAppConfig marshals the provided AppConfig struct to YAML and writes it to the
// file specified by `filePath`. It overwrites the file if it already exists.
// File permissions are set to 0644.
func SaveAppConfig(filePath string, cfg *AppConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644) // Use 0644 for config files (readable by owner/group).
}
