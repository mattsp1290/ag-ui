package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.config == nil {
		t.Fatal("Manager config is nil")
	}
	if m.envPrefix != "AGUI_" {
		t.Errorf("Expected envPrefix to be 'AGUI_', got '%s'", m.envPrefix)
	}
}

func TestSetDefaults(t *testing.T) {
	m := NewManager()
	m.setDefaults()
	
	if m.config.ServerURL != "http://localhost:8080" {
		t.Errorf("Expected default ServerURL to be 'http://localhost:8080', got '%s'", m.config.ServerURL)
	}
	if m.config.LogLevel != "info" {
		t.Errorf("Expected default LogLevel to be 'info', got '%s'", m.config.LogLevel)
	}
	if m.config.LogFormat != "text" {
		t.Errorf("Expected default LogFormat to be 'text', got '%s'", m.config.LogFormat)
	}
	if m.config.Output != "text" {
		t.Errorf("Expected default Output to be 'text', got '%s'", m.config.Output)
	}
	if m.config.Extras == nil {
		t.Error("Expected Extras to be initialized")
	}
}

func TestMergeConfig(t *testing.T) {
	m := NewManager()
	m.setDefaults()
	
	source := &Config{
		ServerURL: "https://api.example.com",
		APIKey:    "test-key",
		LogLevel:  "debug",
		Extras: map[string]string{
			"custom1": "value1",
		},
	}
	
	m.mergeConfig(source)
	
	if m.config.ServerURL != "https://api.example.com" {
		t.Errorf("Expected ServerURL to be merged, got '%s'", m.config.ServerURL)
	}
	if m.config.APIKey != "test-key" {
		t.Errorf("Expected APIKey to be merged, got '%s'", m.config.APIKey)
	}
	if m.config.LogLevel != "debug" {
		t.Errorf("Expected LogLevel to be merged, got '%s'", m.config.LogLevel)
	}
	if m.config.LogFormat != "text" {
		t.Errorf("Expected LogFormat to remain default, got '%s'", m.config.LogFormat)
	}
	if m.config.Extras["custom1"] != "value1" {
		t.Errorf("Expected custom1 extra to be merged")
	}
}

func TestApplyEnvironment(t *testing.T) {
	// Save current env vars
	oldServer := os.Getenv("AGUI_SERVER")
	oldAPIKey := os.Getenv("AGUI_API_KEY")
	oldLogLevel := os.Getenv("AGUI_LOG_LEVEL")
	defer func() {
		os.Setenv("AGUI_SERVER", oldServer)
		os.Setenv("AGUI_API_KEY", oldAPIKey)
		os.Setenv("AGUI_LOG_LEVEL", oldLogLevel)
	}()
	
	// Set test env vars
	os.Setenv("AGUI_SERVER", "https://env.example.com")
	os.Setenv("AGUI_API_KEY", "env-key")
	os.Setenv("AGUI_LOG_LEVEL", "warn")
	
	m := NewManager()
	m.setDefaults()
	m.applyEnvironment()
	
	if m.config.ServerURL != "https://env.example.com" {
		t.Errorf("Expected ServerURL from env, got '%s'", m.config.ServerURL)
	}
	if m.config.APIKey != "env-key" {
		t.Errorf("Expected APIKey from env, got '%s'", m.config.APIKey)
	}
	if m.config.LogLevel != "warn" {
		t.Errorf("Expected LogLevel from env, got '%s'", m.config.LogLevel)
	}
}

func TestApplyFlags(t *testing.T) {
	m := NewManager()
	m.setDefaults()
	
	flags := map[string]string{
		"server":     "https://flag.example.com",
		"api-key":    "flag-key",
		"log-level":  "error",
		"log-format": "json",
		"output":     "json",
	}
	
	m.ApplyFlags(flags)
	
	if m.config.ServerURL != "https://flag.example.com" {
		t.Errorf("Expected ServerURL from flags, got '%s'", m.config.ServerURL)
	}
	if m.config.APIKey != "flag-key" {
		t.Errorf("Expected APIKey from flags, got '%s'", m.config.APIKey)
	}
	if m.config.LogLevel != "error" {
		t.Errorf("Expected LogLevel from flags, got '%s'", m.config.LogLevel)
	}
	if m.config.LogFormat != "json" {
		t.Errorf("Expected LogFormat from flags, got '%s'", m.config.LogFormat)
	}
	if m.config.Output != "json" {
		t.Errorf("Expected Output from flags, got '%s'", m.config.Output)
	}
}

func TestGetSet(t *testing.T) {
	m := NewManager()
	m.setDefaults()
	
	tests := []struct {
		key   string
		value string
		alias []string // alternative keys that should work
	}{
		{"server", "https://test.com", []string{"serverurl"}},
		{"apikey", "test-api-key", []string{"api_key", "api-key"}},
		{"loglevel", "debug", []string{"log_level", "log-level"}},
		{"logformat", "json", []string{"log_format", "log-format"}},
		{"output", "json", nil},
		{"lastsessionid", "session123", []string{"last_session_id", "last-session-id"}},
		{"custom_key", "custom_value", nil},
	}
	
	for _, tt := range tests {
		// Test Set
		if err := m.Set(tt.key, tt.value); err != nil {
			t.Errorf("Failed to set %s: %v", tt.key, err)
		}
		
		// Test Get with primary key
		got, err := m.Get(tt.key)
		if err != nil {
			t.Errorf("Failed to get %s: %v", tt.key, err)
		}
		if got != tt.value {
			t.Errorf("Get(%s) = %s, want %s", tt.key, got, tt.value)
		}
		
		// Test Get with aliases
		for _, alias := range tt.alias {
			got, err := m.Get(alias)
			if err != nil {
				t.Errorf("Failed to get %s (alias of %s): %v", alias, tt.key, err)
			}
			if got != tt.value {
				t.Errorf("Get(%s) = %s, want %s", alias, got, tt.value)
			}
		}
	}
	
	// Test getting non-existent key
	_, err := m.Get("nonexistent")
	if err == nil {
		t.Error("Expected error when getting non-existent key")
	}
}

func TestUnset(t *testing.T) {
	m := NewManager()
	m.setDefaults()
	
	// Set some values
	m.Set("server", "https://test.com")
	m.Set("apikey", "test-key")
	m.Set("custom", "value")
	
	// Unset them
	m.Unset("server")
	m.Unset("apikey")
	m.Unset("custom")
	
	// Verify they're empty
	if m.config.ServerURL != "" {
		t.Errorf("Expected ServerURL to be empty after unset, got '%s'", m.config.ServerURL)
	}
	if m.config.APIKey != "" {
		t.Errorf("Expected APIKey to be empty after unset, got '%s'", m.config.APIKey)
	}
	if val, ok := m.config.Extras["custom"]; ok {
		t.Errorf("Expected custom to be removed from extras, but found '%s'", val)
	}
}

func TestGetRedactedConfig(t *testing.T) {
	m := NewManager()
	m.config.ServerURL = "https://test.com"
	m.config.APIKey = "secret-key-12345"
	m.config.LogLevel = "info"
	
	redacted := m.GetRedactedConfig()
	
	if redacted.ServerURL != "https://test.com" {
		t.Errorf("ServerURL should not be redacted")
	}
	if redacted.APIKey != "***" {
		t.Errorf("APIKey should be redacted, got '%s'", redacted.APIKey)
	}
	if redacted.LogLevel != "info" {
		t.Errorf("LogLevel should not be redacted")
	}
}

func TestToJSON(t *testing.T) {
	m := NewManager()
	m.config.ServerURL = "https://test.com"
	m.config.APIKey = "secret-key"
	m.config.LogLevel = "info"
	
	// Test redacted JSON
	jsonStr, err := m.ToJSON(true)
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}
	
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	
	if result["server"] != "https://test.com" {
		t.Errorf("Expected server in JSON")
	}
	if result["api_key"] != "***" {
		t.Errorf("Expected redacted api_key in JSON, got '%v'", result["api_key"])
	}
	
	// Test non-redacted JSON
	jsonStr, err = m.ToJSON(false)
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}
	
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	
	if result["api_key"] != "secret-key" {
		t.Errorf("Expected actual api_key in non-redacted JSON, got '%v'", result["api_key"])
	}
}

func TestSaveAndLoadFromFile(t *testing.T) {
	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "ag-ui-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Override config path
	os.Setenv("AGUI_CONFIG_PATH", filepath.Join(tempDir, "config.yaml"))
	defer os.Unsetenv("AGUI_CONFIG_PATH")
	
	// Create and save config
	m1 := NewManager()
	m1.config.ServerURL = "https://saved.example.com"
	m1.config.APIKey = "saved-key"
	m1.config.LogLevel = "debug"
	m1.config.LastSessionID = "session-123"
	m1.config.Extras = map[string]string{
		"custom1": "value1",
		"custom2": "value2",
	}
	
	if err := m1.SaveToFile(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}
	
	// Verify file was created
	configPath := m1.GetConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("Config file was not created at %s", configPath)
	}
	
	// Load config in new manager
	m2 := NewManager()
	if err := m2.Load(); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	// Verify loaded values
	if m2.config.ServerURL != "https://saved.example.com" {
		t.Errorf("Expected loaded ServerURL to be 'https://saved.example.com', got '%s'", m2.config.ServerURL)
	}
	if m2.config.APIKey != "saved-key" {
		t.Errorf("Expected loaded APIKey to be 'saved-key', got '%s'", m2.config.APIKey)
	}
	if m2.config.LogLevel != "debug" {
		t.Errorf("Expected loaded LogLevel to be 'debug', got '%s'", m2.config.LogLevel)
	}
	if m2.config.LastSessionID != "session-123" {
		t.Errorf("Expected loaded LastSessionID to be 'session-123', got '%s'", m2.config.LastSessionID)
	}
	if m2.config.Extras["custom1"] != "value1" {
		t.Errorf("Expected loaded custom1 to be 'value1', got '%s'", m2.config.Extras["custom1"])
	}
	if m2.config.Extras["custom2"] != "value2" {
		t.Errorf("Expected loaded custom2 to be 'value2', got '%s'", m2.config.Extras["custom2"])
	}
}

func TestConfigPrecedence(t *testing.T) {
	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "ag-ui-precedence-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Save env vars
	oldServer := os.Getenv("AGUI_SERVER")
	oldAPIKey := os.Getenv("AGUI_API_KEY")
	oldConfigPath := os.Getenv("AGUI_CONFIG_PATH")
	defer func() {
		os.Setenv("AGUI_SERVER", oldServer)
		os.Setenv("AGUI_API_KEY", oldAPIKey)
		os.Setenv("AGUI_CONFIG_PATH", oldConfigPath)
	}()
	
	// Create config file
	configPath := filepath.Join(tempDir, "config.yaml")
	os.Setenv("AGUI_CONFIG_PATH", configPath)
	
	fileConfig := Config{
		ServerURL: "https://file.example.com",
		APIKey:    "file-key",
		LogLevel:  "warn",
		Output:    "json",
	}
	
	data, _ := yaml.Marshal(fileConfig)
	os.WriteFile(configPath, data, 0644)
	
	// Set environment variables
	os.Setenv("AGUI_SERVER", "https://env.example.com")
	os.Setenv("AGUI_API_KEY", "env-key")
	// LogLevel not set in env, should come from file
	
	// Create manager and load
	m := NewManager()
	m.Load()
	
	// At this point: defaults < file < env
	if m.config.ServerURL != "https://env.example.com" {
		t.Errorf("Expected ServerURL from env (precedence), got '%s'", m.config.ServerURL)
	}
	if m.config.APIKey != "env-key" {
		t.Errorf("Expected APIKey from env (precedence), got '%s'", m.config.APIKey)
	}
	if m.config.LogLevel != "warn" {
		t.Errorf("Expected LogLevel from file (no env override), got '%s'", m.config.LogLevel)
	}
	if m.config.Output != "json" {
		t.Errorf("Expected Output from file (no env override), got '%s'", m.config.Output)
	}
	
	// Apply flags (highest precedence)
	flags := map[string]string{
		"server": "https://flag.example.com",
		// APIKey not in flags, should remain from env
		"log-level": "error",
		// Output not in flags, should remain from file
	}
	m.ApplyFlags(flags)
	
	// Final precedence: defaults < file < env < flags
	if m.config.ServerURL != "https://flag.example.com" {
		t.Errorf("Expected ServerURL from flags (highest precedence), got '%s'", m.config.ServerURL)
	}
	if m.config.APIKey != "env-key" {
		t.Errorf("Expected APIKey to remain from env (no flag override), got '%s'", m.config.APIKey)
	}
	if m.config.LogLevel != "error" {
		t.Errorf("Expected LogLevel from flags (highest precedence), got '%s'", m.config.LogLevel)
	}
	if m.config.Output != "json" {
		t.Errorf("Expected Output to remain from file (no env or flag override), got '%s'", m.config.Output)
	}
}