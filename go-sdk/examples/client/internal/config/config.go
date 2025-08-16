package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the complete AG-UI client configuration
type Config struct {
	ServerURL     string            `yaml:"server" json:"server"`
	APIKey        string            `yaml:"api_key" json:"api_key"`
	AuthHeader    string            `yaml:"auth_header,omitempty" json:"auth_header,omitempty"`
	AuthScheme    string            `yaml:"auth_scheme,omitempty" json:"auth_scheme,omitempty"`
	LogLevel      string            `yaml:"log_level" json:"log_level"`
	LogFormat     string            `yaml:"log_format" json:"log_format"`
	Output        string            `yaml:"output" json:"output"`
	LastSessionID string            `yaml:"last_session_id,omitempty" json:"last_session_id,omitempty"`
	Extras        map[string]string `yaml:"extras,omitempty" json:"extras,omitempty"`
}

// Manager handles configuration resolution and persistence
type Manager struct {
	config     *Config
	configPath string
	envPrefix  string
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{
		config:    &Config{},
		envPrefix: "AGUI_",
	}
}

// Load resolves configuration from all sources with proper precedence
// Order: defaults -> config file -> environment -> flags
func (m *Manager) Load() error {
	// 1. Set defaults
	m.setDefaults()

	// 2. Load from config file
	if err := m.loadFromFile(); err != nil {
		// Config file is optional, so we don't return error
		// but we could log it for debugging
	}

	// 3. Apply environment variables
	m.applyEnvironment()

	// Note: Flags are applied later via ApplyFlags method
	// since they need to be parsed by cobra first

	return nil
}

// setDefaults sets the default configuration values
func (m *Manager) setDefaults() {
	m.config.ServerURL = "http://localhost:8080"
	m.config.AuthHeader = "Authorization"
	m.config.AuthScheme = "Bearer"
	m.config.LogLevel = "info"
	m.config.LogFormat = "text"
	m.config.Output = "text"
	if m.config.Extras == nil {
		m.config.Extras = make(map[string]string)
	}
}

// loadFromFile loads configuration from the config file
func (m *Manager) loadFromFile() error {
	configPath := m.GetConfigPath()
	m.configPath = configPath

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Config file doesn't exist, which is fine
			return nil
		}
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var fileConfig Config
	if err := yaml.Unmarshal(data, &fileConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Merge file config into current config
	m.mergeConfig(&fileConfig)
	return nil
}

// applyEnvironment applies environment variables to the configuration
func (m *Manager) applyEnvironment() {
	// Check each environment variable and apply if set
	if val := os.Getenv(m.envPrefix + "SERVER"); val != "" {
		m.config.ServerURL = val
	}
	if val := os.Getenv(m.envPrefix + "API_KEY"); val != "" {
		m.config.APIKey = val
	}
	if val := os.Getenv(m.envPrefix + "AUTH_HEADER"); val != "" {
		m.config.AuthHeader = val
	}
	if val := os.Getenv(m.envPrefix + "AUTH_SCHEME"); val != "" {
		m.config.AuthScheme = val
	}
	if val := os.Getenv(m.envPrefix + "LOG_LEVEL"); val != "" {
		m.config.LogLevel = val
	}
	if val := os.Getenv(m.envPrefix + "LOG_FORMAT"); val != "" {
		m.config.LogFormat = val
	}
	if val := os.Getenv(m.envPrefix + "OUTPUT"); val != "" {
		m.config.Output = val
	}
	if val := os.Getenv(m.envPrefix + "LAST_SESSION_ID"); val != "" {
		m.config.LastSessionID = val
	}
}

// ApplyFlags applies command-line flags to the configuration
// This should be called after cobra has parsed the flags
func (m *Manager) ApplyFlags(flags map[string]string) {
	if val, ok := flags["server"]; ok && val != "" {
		m.config.ServerURL = val
	}
	if val, ok := flags["api-key"]; ok && val != "" {
		m.config.APIKey = val
	}
	if val, ok := flags["auth-header"]; ok && val != "" {
		m.config.AuthHeader = val
	}
	if val, ok := flags["auth-scheme"]; ok && val != "" {
		m.config.AuthScheme = val
	}
	if val, ok := flags["log-level"]; ok && val != "" {
		m.config.LogLevel = val
	}
	if val, ok := flags["log-format"]; ok && val != "" {
		m.config.LogFormat = val
	}
	if val, ok := flags["output"]; ok && val != "" {
		m.config.Output = val
	}
}

// mergeConfig merges source config into the current config
// Only non-empty values are merged
func (m *Manager) mergeConfig(source *Config) {
	if source.ServerURL != "" {
		m.config.ServerURL = source.ServerURL
	}
	if source.APIKey != "" {
		m.config.APIKey = source.APIKey
	}
	if source.AuthHeader != "" {
		m.config.AuthHeader = source.AuthHeader
	}
	if source.AuthScheme != "" {
		m.config.AuthScheme = source.AuthScheme
	}
	if source.LogLevel != "" {
		m.config.LogLevel = source.LogLevel
	}
	if source.LogFormat != "" {
		m.config.LogFormat = source.LogFormat
	}
	if source.Output != "" {
		m.config.Output = source.Output
	}
	if source.LastSessionID != "" {
		m.config.LastSessionID = source.LastSessionID
	}
	if source.Extras != nil {
		if m.config.Extras == nil {
			m.config.Extras = make(map[string]string)
		}
		for k, v := range source.Extras {
			m.config.Extras[k] = v
		}
	}
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *Config {
	return m.config
}

// GetConfigPath returns the path to the configuration file
func (m *Manager) GetConfigPath() string {
	// Check if a custom config path is set via environment
	if customPath := os.Getenv(m.envPrefix + "CONFIG_PATH"); customPath != "" {
		return customPath
	}

	// Use XDG_CONFIG_HOME if set, otherwise use default
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, _ := os.UserHomeDir()
		if runtime.GOOS == "windows" {
			// On Windows, use %AppData%
			configHome = os.Getenv("APPDATA")
			if configHome == "" {
				configHome = filepath.Join(homeDir, "AppData", "Roaming")
			}
		} else {
			// On Unix-like systems, use ~/.config
			configHome = filepath.Join(homeDir, ".config")
		}
	}

	return filepath.Join(configHome, "ag-ui", "client", "config.yaml")
}

// GetConfigPaths returns all paths where config files are searched
func (m *Manager) GetConfigPaths() []string {
	paths := []string{
		m.GetConfigPath(),
	}

	// Add fallback paths
	homeDir, _ := os.UserHomeDir()
	paths = append(paths,
		filepath.Join(homeDir, ".ag-ui", "client", "config.yaml"),
		filepath.Join(homeDir, ".ag-ui", "config.yaml"),
		"ag-ui.yaml",
		".ag-ui.yaml",
	)

	return paths
}

// SaveToFile saves the current configuration to the config file
func (m *Manager) SaveToFile() error {
	configPath := m.GetConfigPath()

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config to YAML
	data, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write atomically using a temp file
	tempFile := configPath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Rename temp file to actual config file (atomic on most systems)
	if err := os.Rename(tempFile, configPath); err != nil {
		// Cleanup temp file if rename failed
		os.Remove(tempFile)
		return fmt.Errorf("failed to save config file: %w", err)
	}

	return nil
}

// Get returns a specific configuration value by key
func (m *Manager) Get(key string) (string, error) {
	switch strings.ToLower(key) {
	case "server", "serverurl":
		return m.config.ServerURL, nil
	case "apikey", "api_key", "api-key":
		return m.config.APIKey, nil
	case "authheader", "auth_header", "auth-header":
		return m.config.AuthHeader, nil
	case "authscheme", "auth_scheme", "auth-scheme":
		return m.config.AuthScheme, nil
	case "loglevel", "log_level", "log-level":
		return m.config.LogLevel, nil
	case "logformat", "log_format", "log-format":
		return m.config.LogFormat, nil
	case "output":
		return m.config.Output, nil
	case "lastsessionid", "last_session_id", "last-session-id":
		return m.config.LastSessionID, nil
	default:
		// Check extras
		if val, ok := m.config.Extras[key]; ok {
			return val, nil
		}
		return "", fmt.Errorf("unknown configuration key: %s", key)
	}
}

// Set sets a specific configuration value by key
func (m *Manager) Set(key, value string) error {
	switch strings.ToLower(key) {
	case "server", "serverurl":
		m.config.ServerURL = value
	case "apikey", "api_key", "api-key":
		m.config.APIKey = value
	case "authheader", "auth_header", "auth-header":
		m.config.AuthHeader = value
	case "authscheme", "auth_scheme", "auth-scheme":
		m.config.AuthScheme = value
	case "loglevel", "log_level", "log-level":
		m.config.LogLevel = value
	case "logformat", "log_format", "log-format":
		m.config.LogFormat = value
	case "output":
		m.config.Output = value
	case "lastsessionid", "last_session_id", "last-session-id":
		m.config.LastSessionID = value
	default:
		// Store in extras
		if m.config.Extras == nil {
			m.config.Extras = make(map[string]string)
		}
		m.config.Extras[key] = value
	}
	return nil
}

// Unset removes a specific configuration value by key
func (m *Manager) Unset(key string) error {
	switch strings.ToLower(key) {
	case "server", "serverurl":
		m.config.ServerURL = ""
	case "apikey", "api_key", "api-key":
		m.config.APIKey = ""
	case "authheader", "auth_header", "auth-header":
		m.config.AuthHeader = ""
	case "authscheme", "auth_scheme", "auth-scheme":
		m.config.AuthScheme = ""
	case "loglevel", "log_level", "log-level":
		m.config.LogLevel = ""
	case "logformat", "log_format", "log-format":
		m.config.LogFormat = ""
	case "output":
		m.config.Output = ""
	case "lastsessionid", "last_session_id", "last-session-id":
		m.config.LastSessionID = ""
	default:
		// Remove from extras
		if m.config.Extras != nil {
			delete(m.config.Extras, key)
		}
	}
	return nil
}

// GetRedactedConfig returns a copy of the config with sensitive data redacted
func (m *Manager) GetRedactedConfig() *Config {
	redacted := *m.config
	if redacted.APIKey != "" {
		redacted.APIKey = "***"
	}
	return &redacted
}

// ToJSON returns the configuration as JSON string
func (m *Manager) ToJSON(redacted bool) (string, error) {
	var cfg *Config
	if redacted {
		cfg = m.GetRedactedConfig()
	} else {
		cfg = m.config
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal config to JSON: %w", err)
	}
	return string(data), nil
}

// GetEnvironmentVariables returns a list of supported environment variables
func (m *Manager) GetEnvironmentVariables() []string {
	return []string{
		m.envPrefix + "SERVER",
		m.envPrefix + "API_KEY",
		m.envPrefix + "AUTH_HEADER",
		m.envPrefix + "AUTH_SCHEME",
		m.envPrefix + "LOG_LEVEL",
		m.envPrefix + "LOG_FORMAT",
		m.envPrefix + "OUTPUT",
		m.envPrefix + "LAST_SESSION_ID",
		m.envPrefix + "CONFIG_PATH",
	}
}