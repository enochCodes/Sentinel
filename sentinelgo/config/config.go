package config

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AppConfig holds the application configuration, typically loaded from sentinel.yaml
type AppConfig struct {
	DefaultHeaders map[string]string `yaml:"defaultheaders"`
	CustomCookies  []http.Cookie     `yaml:"customcookies"`
	MaxRetries     int               `yaml:"maxretries"`
	RiskThreshold  float64           `yaml:"riskthreshold"`
	APIKeys        map[string]string `yaml:"apikeys"`
}

// SessionState holds the persistent state of the session, typically stored in ~/.sentinel/state.json
type SessionState struct {
	LastTargetURL   string `json:"lasttargeturl"`
	LastReason      string `json:"lastreason"`
	LastProxyConfig string `json:"lastproxyconfig"`
}

// LoadAppConfig reads the YAML file specified by filePath and unmarshals it into AppConfig.
// If the file doesn't exist, it returns a default AppConfig.
func LoadAppConfig(filePath string) (*AppConfig, error) {
	config := &AppConfig{
		MaxRetries:    3,
		RiskThreshold: 75.0,
		DefaultHeaders: make(map[string]string),
		APIKeys: make(map[string]string),
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil // Return default config if file doesn't exist
		}
		return nil, err
	}

	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// LoadSessionState reads the JSON file specified by filePath and unmarshals it into SessionState.
// If the file doesn't exist, it returns a default SessionState.
func LoadSessionState(filePath string) (*SessionState, error) {
	state := &SessionState{}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil // Return default state if file doesn't exist
		}
		return nil, err
	}

	err = json.Unmarshal(data, state)
	if err != nil {
		return nil, err
	}

	return state, nil
}

// SaveSessionState marshals SessionState to JSON and writes it to the specified filePath.
// It ensures the directory for filePath is created if it doesn't exist.
func SaveSessionState(filePath string, state *SessionState) error {
	dir := filepath.Dir(filePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Ensure user-specific directory is handled correctly
		if dir == filepath.Join("~", ".sentinel") || dir == filepath.Join(os.Getenv("HOME"), ".sentinel") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			dir = filepath.Join(homeDir, ".sentinel")
		}
		err = os.MkdirAll(dir, 0750) // Use 0750 for permissions
		if err != nil {
			return err
		}
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0600) // Use 0600 for permissions
}

// SaveAppConfig marshals AppConfig to YAML and writes it to the specified filePath.
func SaveAppConfig(filePath string, cfg *AppConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644) // Use 0644 for config files
}
