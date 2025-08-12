package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the CLI configuration
type Config struct {
	ServerURL string `json:"server_url"`
	APIKey    string `json:"api_key"`
}

// State represents the CLI state
type State struct {
	LastSessionID string `json:"last_session_id"`
}

// Load loads the configuration from the config file
func Load() (*Config, error) {
	// Default configuration
	cfg := &Config{
		ServerURL: getEnvOrDefault("AG_UI_SERVER_URL", "http://localhost:8000"),
		APIKey:    os.Getenv("AG_UI_API_KEY"),
	}

	// Try to load from config file
	configPath := getConfigPath()
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err == nil {
			if err := json.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config: %w", err)
			}
		}
	}

	// Environment variables take precedence
	if url := os.Getenv("AG_UI_SERVER_URL"); url != "" {
		cfg.ServerURL = url
	}
	if key := os.Getenv("AG_UI_API_KEY"); key != "" {
		cfg.APIKey = key
	}

	return cfg, nil
}

// LoadState loads the CLI state
func LoadState() (*State, error) {
	statePath := getStatePath()
	if statePath == "" {
		return nil, fmt.Errorf("no state file path")
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{}, nil
		}
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}

	return &state, nil
}

// SaveState saves the CLI state
func SaveState(state *State) error {
	statePath := getStatePath()
	if statePath == "" {
		return fmt.Errorf("no state file path")
	}

	// Ensure directory exists
	dir := filepath.Dir(statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	return nil
}

func getConfigPath() string {
	// Check for explicit config file
	if path := os.Getenv("AG_UI_CONFIG"); path != "" {
		return path
	}

	// Check XDG config directory
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "ag-ui", "config.json")
	}

	// Fall back to home directory
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "ag-ui", "config.json")
	}

	return ""
}

func getStatePath() string {
	// Check XDG data directory
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "ag-ui", "state.json")
	}

	// Fall back to home directory
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "ag-ui", "state.json")
	}

	return ""
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}