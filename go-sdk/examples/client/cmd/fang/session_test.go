package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionCommands(t *testing.T) {
	// Create a temporary directory for config/session files
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	
	// Set environment variable to use temp config path
	os.Setenv("AGUI_CONFIG_PATH", configPath)
	defer os.Unsetenv("AGUI_CONFIG_PATH")
	
	// Set log level to error to reduce noise
	os.Setenv("AGUI_LOG_LEVEL", "error")
	defer os.Unsetenv("AGUI_LOG_LEVEL")
	
	t.Run("session_open_text_output", func(t *testing.T) {
		cmd := newRootCommand()
		cmd.SetArgs([]string{"session", "open", "--label", "test-session"})
		
		// Capture output
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		
		err := cmd.Execute()
		require.NoError(t, err)
		
		output := strings.TrimSpace(buf.String())
		// Should be a UUID
		assert.Regexp(t, "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", output)
	})
	
	t.Run("session_open_json_output", func(t *testing.T) {
		cmd := newRootCommand()
		cmd.SetArgs([]string{"session", "open", "--output", "json", "--label", "prod", "--metadata", `{"env":"production"}`})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		
		err := cmd.Execute()
		require.NoError(t, err)
		
		// Parse JSON output
		var result map[string]interface{}
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		
		assert.Equal(t, "opened", result["status"])
		assert.Equal(t, "prod", result["label"])
		assert.NotEmpty(t, result["threadId"])
		
		metadata, ok := result["metadata"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "production", metadata["env"])
	})
	
	t.Run("session_list_with_active_session", func(t *testing.T) {
		// First open a session
		cmd := newRootCommand()
		cmd.SetArgs([]string{"session", "open", "--label", "list-test", "--output", "text"})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		err := cmd.Execute()
		require.NoError(t, err)
		
		threadID := strings.TrimSpace(buf.String())
		
		// Now list
		cmd = newRootCommand()
		cmd.SetArgs([]string{"session", "list", "--output", "text"})
		
		buf.Reset()
		cmd.SetOut(&buf)
		err = cmd.Execute()
		require.NoError(t, err)
		
		output := buf.String()
		assert.Contains(t, output, "Active session:")
		assert.Contains(t, output, threadID)
		assert.Contains(t, output, "list-test")
	})
	
	t.Run("session_list_json_output", func(t *testing.T) {
		// First open a session
		cmd := newRootCommand()
		cmd.SetArgs([]string{"session", "open", "--label", "json-test"})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		err := cmd.Execute()
		require.NoError(t, err)
		
		threadID := strings.TrimSpace(buf.String())
		
		// List with JSON output
		cmd = newRootCommand()
		cmd.SetArgs([]string{"session", "list", "--output", "json"})
		
		buf.Reset()
		cmd.SetOut(&buf)
		err = cmd.Execute()
		require.NoError(t, err)
		
		var result map[string]interface{}
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		
		assert.Equal(t, threadID, result["threadId"])
		assert.Equal(t, "json-test", result["label"])
		assert.Equal(t, "active", result["status"])
		assert.NotEmpty(t, result["lastOpenedAt"])
	})
	
	t.Run("session_close", func(t *testing.T) {
		// Open a session
		cmd := newRootCommand()
		cmd.SetArgs([]string{"session", "open", "--label", "close-test", "--output", "text"})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		err := cmd.Execute()
		require.NoError(t, err)
		
		threadID := strings.TrimSpace(buf.String())
		
		// Close the session
		cmd = newRootCommand()
		cmd.SetArgs([]string{"session", "close", "--output", "text"})
		
		buf.Reset()
		cmd.SetOut(&buf)
		err = cmd.Execute()
		require.NoError(t, err)
		
		output := buf.String()
		assert.Contains(t, output, threadID)
		
		// Verify no active session
		cmd = newRootCommand()
		cmd.SetArgs([]string{"session", "list", "--output", "text"})
		
		buf.Reset()
		cmd.SetOut(&buf)
		err = cmd.Execute()
		require.NoError(t, err)
		
		assert.Contains(t, buf.String(), "No active session")
	})
	
	t.Run("session_close_idempotent", func(t *testing.T) {
		// Close when no session exists
		cmd := newRootCommand()
		cmd.SetArgs([]string{"session", "close", "--output", "text"})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		
		err := cmd.Execute()
		require.NoError(t, err)
		
		assert.Contains(t, buf.String(), "No active session to close")
		
		// Close again
		cmd = newRootCommand()
		cmd.SetArgs([]string{"session", "close", "--output", "json"})
		
		buf.Reset()
		cmd.SetOut(&buf)
		err = cmd.Execute()
		require.NoError(t, err)
		
		var result map[string]interface{}
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, "closed", result["status"])
	})
	
	t.Run("session_list_no_active_session", func(t *testing.T) {
		// Ensure no active session
		cmd := newRootCommand()
		cmd.SetArgs([]string{"session", "close", "--output", "text"})
		cmd.Execute()
		
		// List sessions
		cmd = newRootCommand()
		cmd.SetArgs([]string{"session", "list", "--output", "text"})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		err := cmd.Execute()
		require.NoError(t, err)
		
		assert.Contains(t, buf.String(), "No active session")
		
		// List with JSON should return empty object
		cmd = newRootCommand()
		cmd.SetArgs([]string{"session", "list", "--output", "json"})
		
		buf.Reset()
		cmd.SetOut(&buf)
		err = cmd.Execute()
		require.NoError(t, err)
		
		assert.Equal(t, "{}", strings.TrimSpace(buf.String()))
	})
}