package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChatCommandBasic tests basic chat command functionality
func TestChatCommandBasic(t *testing.T) {
	// Create temporary config directory
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	os.Setenv("AGUI_CONFIG_PATH", configPath)
	defer os.Unsetenv("AGUI_CONFIG_PATH")
	
	// Set log level to error to reduce noise
	os.Setenv("AGUI_LOG_LEVEL", "error")
	defer os.Unsetenv("AGUI_LOG_LEVEL")
	
	t.Run("chat_with_simple_message", func(t *testing.T) {
		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			assert.Equal(t, "/tool_based_generative_ui", r.URL.Path)
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			
			// Parse request body
			var reqBody map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			
			messages, ok := reqBody["messages"].([]interface{})
			require.True(t, ok)
			require.Greater(t, len(messages), 0)
			
			firstMsg := messages[0].(map[string]interface{})
			assert.Equal(t, "user", firstMsg["role"])
			assert.Contains(t, firstMsg["content"], "Hello")
			
			// Send SSE response
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			
			events := []string{
				TestFixtures.SSEEvents["RUN_STARTED"],
				TestFixtures.SSEEvents["TEXT_MESSAGE_START"],
				`data: {"type":"TEXT_MESSAGE_CONTENT","delta":"Hello! I'm here to help.","messageId":"test-msg-001"}

`,
				TestFixtures.SSEEvents["TEXT_MESSAGE_END"],
				TestFixtures.SSEEvents["RUN_FINISHED"],
			}
			
			for _, event := range events {
				fmt.Fprint(w, event)
				w.(http.Flusher).Flush()
				time.Sleep(5 * time.Millisecond)
			}
		}))
		defer server.Close()
		
		// Run chat command
		cmd := newRootCommand()
		cmd.SetArgs([]string{
			"chat",
			"--message", "Hello",
			"--server", server.URL,
			"--interactive=false",
			"--output", "text",
		})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		
		err := cmd.Execute()
		require.NoError(t, err)
		
		output := buf.String()
		assert.Contains(t, output, "Hello! I'm here to help.")
	})
	
	t.Run("chat_with_tool_execution", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			
			// Send events with tool execution
			events := []string{
				TestFixtures.SSEEvents["RUN_STARTED"],
				TestFixtures.SSEEvents["TOOL_CALL_START"],
				TestFixtures.SSEEvents["TOOL_CALL_CHUNK"],
				TestFixtures.SSEEvents["TOOL_CALL_END"],
				TestFixtures.SSEEvents["MESSAGES_SNAPSHOT_WITH_TOOL"],
				TestFixtures.SSEEvents["RUN_FINISHED"],
			}
			
			for _, event := range events {
				fmt.Fprint(w, event)
				w.(http.Flusher).Flush()
				time.Sleep(5 * time.Millisecond)
			}
		}))
		defer server.Close()
		
		cmd := newRootCommand()
		cmd.SetArgs([]string{
			"chat",
			"--message", "Generate a haiku",
			"--server", server.URL,
			"--interactive=false",
		})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		
		err := cmd.Execute()
		require.NoError(t, err)
		
		output := buf.String()
		assert.Contains(t, output, "Spring rain falling down")
		assert.Contains(t, output, "generate_haiku")
	})
	
	t.Run("chat_with_streaming_mode", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// For streaming mode, it should hit /agentic_chat
			assert.Equal(t, "/agentic_chat", r.URL.Path)
			
			w.Header().Set("Content-Type", "text/event-stream")
			
			events := []string{
				TestFixtures.SSEEvents["RUN_STARTED"],
				TestFixtures.SSEEvents["TEXT_MESSAGE_START"],
				`data: {"type":"TEXT_MESSAGE_CONTENT","delta":"Streaming ","messageId":"msg-1"}

`,
				`data: {"type":"TEXT_MESSAGE_CONTENT","delta":"response ","messageId":"msg-1"}

`,
				`data: {"type":"TEXT_MESSAGE_CONTENT","delta":"test.","messageId":"msg-1"}

`,
				TestFixtures.SSEEvents["TEXT_MESSAGE_END"],
				TestFixtures.SSEEvents["RUN_FINISHED"],
			}
			
			for _, event := range events {
				fmt.Fprint(w, event)
				w.(http.Flusher).Flush()
				time.Sleep(10 * time.Millisecond)
			}
		}))
		defer server.Close()
		
		cmd := newRootCommand()
		cmd.SetArgs([]string{
			"chat",
			"--message", "Test streaming",
			"--server", server.URL,
			"--streaming",
			"--interactive=false",
		})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		
		err := cmd.Execute()
		require.NoError(t, err)
		
		output := buf.String()
		assert.Contains(t, output, "Streaming response test.")
	})
	
	t.Run("chat_with_json_output", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			
			events := []string{
				TestFixtures.SSEEvents["RUN_STARTED"],
				TestFixtures.SSEEvents["MESSAGES_SNAPSHOT_NO_TOOL"],
				TestFixtures.SSEEvents["RUN_FINISHED"],
			}
			
			for _, event := range events {
				fmt.Fprint(w, event)
				w.(http.Flusher).Flush()
			}
		}))
		defer server.Close()
		
		cmd := newRootCommand()
		cmd.SetArgs([]string{
			"chat",
			"--message", "Test JSON",
			"--server", server.URL,
			"--output", "json",
			"--interactive=false",
		})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		
		err := cmd.Execute()
		require.NoError(t, err)
		
		// Output should be JSON lines
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			var event map[string]interface{}
			err := json.Unmarshal([]byte(line), &event)
			assert.NoError(t, err, "Each line should be valid JSON")
		}
	})
}

// TestChatCommandWithThinking tests THINKING event handling
func TestChatCommandWithThinking(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	os.Setenv("AGUI_CONFIG_PATH", configPath)
	defer os.Unsetenv("AGUI_CONFIG_PATH")
	os.Setenv("AGUI_LOG_LEVEL", "error")
	defer os.Unsetenv("AGUI_LOG_LEVEL")
	
	t.Run("thinking_events_display", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			
			events := []string{
				TestFixtures.SSEEvents["RUN_STARTED"],
				TestFixtures.SSEEvents["THINKING_START"],
				`data: {"type":"THINKING_DELTA","delta":"Analyzing the request","messageId":"thinking-001"}

`,
				`data: {"type":"THINKING_DELTA","delta":"...","messageId":"thinking-001"}

`,
				TestFixtures.SSEEvents["THINKING_END"],
				TestFixtures.SSEEvents["TEXT_MESSAGE_START"],
				`data: {"type":"TEXT_MESSAGE_CONTENT","delta":"Here's my response.","messageId":"msg-1"}

`,
				TestFixtures.SSEEvents["TEXT_MESSAGE_END"],
				TestFixtures.SSEEvents["RUN_FINISHED"],
			}
			
			for _, event := range events {
				fmt.Fprint(w, event)
				w.(http.Flusher).Flush()
				time.Sleep(10 * time.Millisecond)
			}
		}))
		defer server.Close()
		
		cmd := newRootCommand()
		cmd.SetArgs([]string{
			"chat",
			"--message", "Complex question",
			"--server", server.URL,
			"--interactive=false",
		})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		
		err := cmd.Execute()
		require.NoError(t, err)
		
		output := buf.String()
		// Should show thinking indicator
		assert.Contains(t, output, "🤔")
		// Should show actual response
		assert.Contains(t, output, "Here's my response.")
	})
}

// TestChatCommandWithState tests STATE event handling
func TestChatCommandWithState(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	os.Setenv("AGUI_CONFIG_PATH", configPath)
	defer os.Unsetenv("AGUI_CONFIG_PATH")
	os.Setenv("AGUI_LOG_LEVEL", "error")
	defer os.Unsetenv("AGUI_LOG_LEVEL")
	
	t.Run("state_snapshot_and_delta", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Should hit state endpoint
			assert.Equal(t, "/agentic_generative_ui", r.URL.Path)
			
			w.Header().Set("Content-Type", "text/event-stream")
			
			events := []string{
				TestFixtures.SSEEvents["RUN_STARTED"],
				TestFixtures.SSEEvents["STATE_SNAPSHOT"],
				`data: {"type":"STATE_DELTA","delta":{"counter":43},"path":"/"}

`,
				`data: {"type":"STATE_DELTA","delta":{"counter":44},"path":"/"}

`,
				TestFixtures.SSEEvents["RUN_FINISHED"],
			}
			
			for _, event := range events {
				fmt.Fprint(w, event)
				w.(http.Flusher).Flush()
				time.Sleep(10 * time.Millisecond)
			}
		}))
		defer server.Close()
		
		cmd := newRootCommand()
		cmd.SetArgs([]string{
			"state",
			"--message", "Update counter",
			"--server", server.URL,
			"--json",
		})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		
		err := cmd.Execute()
		require.NoError(t, err)
		
		// Parse JSON output
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		var snapshotFound, deltaFound bool
		
		for _, line := range lines {
			if line == "" {
				continue
			}
			var event map[string]interface{}
			err := json.Unmarshal([]byte(line), &event)
			require.NoError(t, err)
			if event["type"] == "STATE_SNAPSHOT" {
				snapshotFound = true
				snapshot := event["snapshot"].(map[string]interface{})
				assert.Equal(t, float64(42), snapshot["counter"])
			}
			if event["type"] == "STATE_DELTA" {
				deltaFound = true
				delta := event["delta"].(map[string]interface{})
				assert.NotNil(t, delta["counter"])
			}
		}
		
		assert.True(t, snapshotFound, "Should have STATE_SNAPSHOT event")
		assert.True(t, deltaFound, "Should have STATE_DELTA event")
	})
}

// TestChatCommandErrorHandling tests error scenarios
func TestChatCommandErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	os.Setenv("AGUI_CONFIG_PATH", configPath)
	defer os.Unsetenv("AGUI_CONFIG_PATH")
	os.Setenv("AGUI_LOG_LEVEL", "error")
	defer os.Unsetenv("AGUI_LOG_LEVEL")
	
	t.Run("server_connection_error", func(t *testing.T) {
		// Use a non-existent server
		cmd := newRootCommand()
		cmd.SetArgs([]string{
			"chat",
			"--message", "Test",
			"--server", "http://localhost:99999", // Invalid port
			"--interactive=false",
		})
		
		var buf bytes.Buffer
		cmd.SetErr(&buf)
		
		err := cmd.Execute()
		assert.Error(t, err)
	})
	
	t.Run("malformed_sse_event", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			
			// Send malformed event
			fmt.Fprint(w, "data: {invalid json}\n\n")
			w.(http.Flusher).Flush()
			
			// Then send valid finish event
			fmt.Fprint(w, TestFixtures.SSEEvents["RUN_FINISHED"])
			w.(http.Flusher).Flush()
		}))
		defer server.Close()
		
		cmd := newRootCommand()
		cmd.SetArgs([]string{
			"chat",
			"--message", "Test",
			"--server", server.URL,
			"--interactive=false",
		})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		
		// Should handle gracefully and continue
		err := cmd.Execute()
		// The command might still succeed if it handles errors gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "json")
		}
	})
	
	t.Run("missing_required_fields", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			
			// Send event missing required fields
			fmt.Fprint(w, `data: {"type":"UNKNOWN_EVENT"}

`)
			w.(http.Flusher).Flush()
			
			fmt.Fprint(w, TestFixtures.SSEEvents["RUN_FINISHED"])
			w.(http.Flusher).Flush()
		}))
		defer server.Close()
		
		cmd := newRootCommand()
		cmd.SetArgs([]string{
			"chat",
			"--message", "Test",
			"--server", server.URL,
			"--interactive=false",
		})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		
		// Should handle unknown events gracefully
		err := cmd.Execute()
		// Command should complete even with unknown events
		assert.NoError(t, err)
	})
}

// TestChatCommandInteractive tests interactive mode features
func TestChatCommandInteractive(t *testing.T) {
	t.Skip("Interactive tests require manual input simulation")
	
	// Note: Testing interactive prompts requires more complex setup
	// with stdin mocking. These tests would need:
	// 1. Mock stdin with predefined inputs
	// 2. Capture prompt outputs
	// 3. Verify state changes based on user choices
	
	// Example structure:
	/*
	t.Run("apply_regenerate_cancel_prompt", func(t *testing.T) {
		// Setup mock stdin
		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		defer func() { os.Stdin = oldStdin }()
		
		// Write user input
		go func() {
			fmt.Fprintln(w, "a") // Apply
			w.Close()
		}()
		
		// Run command with interactive mode
		// ... test implementation
	})
	*/
}

// TestChatCommandWithSession tests session management
func TestChatCommandWithSession(t *testing.T) {
	t.Run("chat_with_session_id", func(t *testing.T) {
		// Create separate temp directory for this test
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.yaml")
		os.Setenv("AGUI_CONFIG_PATH", configPath)
		defer os.Unsetenv("AGUI_CONFIG_PATH")
		os.Setenv("AGUI_LOG_LEVEL", "error")
		defer os.Unsetenv("AGUI_LOG_LEVEL")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Parse request to check session ID
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)
			
			// Session ID should be in thread_id or threadId
			threadID, ok := reqBody["thread_id"].(string)
			if !ok {
				// Try camelCase version
				threadID, ok = reqBody["threadId"].(string)
			}
			require.True(t, ok, "thread_id or threadId not found in request")
			assert.Equal(t, "test-session-123", threadID)
			
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, TestFixtures.SSEEvents["RUN_STARTED"])
			fmt.Fprint(w, TestFixtures.SSEEvents["MESSAGES_SNAPSHOT_NO_TOOL"])
			fmt.Fprint(w, TestFixtures.SSEEvents["RUN_FINISHED"])
			w.(http.Flusher).Flush()
		}))
		defer server.Close()
		
		cmd := newRootCommand()
		cmd.SetArgs([]string{
			"chat",
			"--message", "Test with session",
			"--session-id", "test-session-123",
			"--server", server.URL,
			"--interactive=false",
		})
		
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		
		err := cmd.Execute()
		require.NoError(t, err)
	})
	
	t.Run("chat_creates_new_session", func(t *testing.T) {
		// Create separate temp directory for this test
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.yaml")
		os.Setenv("AGUI_CONFIG_PATH", configPath)
		defer os.Unsetenv("AGUI_CONFIG_PATH")
		os.Setenv("AGUI_LOG_LEVEL", "error")
		defer os.Unsetenv("AGUI_LOG_LEVEL")
		requestCount := 0
		var capturedThreadID string
		
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)
			
			// Try both snake_case and camelCase
			threadID, ok := reqBody["thread_id"].(string)
			if !ok {
				threadID, ok = reqBody["threadId"].(string)
			}
			require.True(t, ok, "thread_id or threadId not found")
			if requestCount == 1 {
				capturedThreadID = threadID
				// First request should have a generated UUID
				assert.Regexp(t, "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", threadID)
			} else {
				// Subsequent requests should use the same thread ID
				assert.Equal(t, capturedThreadID, threadID)
			}
			
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, TestFixtures.SSEEvents["RUN_STARTED"])
			fmt.Fprint(w, TestFixtures.SSEEvents["MESSAGES_SNAPSHOT_NO_TOOL"])
			fmt.Fprint(w, TestFixtures.SSEEvents["RUN_FINISHED"])
			w.(http.Flusher).Flush()
		}))
		defer server.Close()
		
		// First message
		cmd := newRootCommand()
		cmd.SetArgs([]string{
			"chat",
			"--message", "First message",
			"--server", server.URL,
			"--interactive=false",
		})
		
		err := cmd.Execute()
		require.NoError(t, err)
		
		// Second message (should reuse session)
		cmd = newRootCommand()
		cmd.SetArgs([]string{
			"chat",
			"--message", "Second message",
			"--server", server.URL,
			"--interactive=false",
			"--resume",
		})
		
		err = cmd.Execute()
		require.NoError(t, err)
		
		assert.Equal(t, 2, requestCount)
	})
}

// TestChatCommandEndpoints tests different endpoint configurations
func TestChatCommandEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	os.Setenv("AGUI_CONFIG_PATH", configPath)
	defer os.Unsetenv("AGUI_CONFIG_PATH")
	os.Setenv("AGUI_LOG_LEVEL", "error")
	defer os.Unsetenv("AGUI_LOG_LEVEL")
	
	endpoints := []struct {
		name                string
		command             string
		args                []string
		expected            string
		supportsInteractive bool
	}{
		{
			name:                "tool_based_generative_ui",
			command:             "chat",
			args:                []string{"--message", "Test"},
			expected:            "/tool_based_generative_ui",
			supportsInteractive: true,
		},
		{
			name:                "agentic_chat_streaming",
			command:             "chat",
			args:                []string{"--message", "Test", "--streaming"},
			expected:            "/agentic_chat",
			supportsInteractive: true,
		},
		{
			name:                "human_in_the_loop",
			command:             "human-loop",
			args:                []string{"--message", "Test"},
			expected:            "/human_in_the_loop",
			supportsInteractive: false,
		},
		{
			name:                "agentic_generative_ui",
			command:             "state",
			args:                []string{"--message", "Test"},
			expected:            "/agentic_generative_ui",
			supportsInteractive: false,
		},
		{
			name:                "predictive_state_updates",
			command:             "predictive",
			args:                []string{"--message", "Test"},
			expected:            "/predictive_state_updates",
			supportsInteractive: true,
		},
		{
			name:                "shared_state",
			command:             "shared",
			args:                []string{"--message", "Test"},
			expected:            "/shared_state",
			supportsInteractive: false,
		},
	}
	
	for _, tc := range endpoints {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify the correct endpoint is called
				assert.Equal(t, tc.expected, r.URL.Path)
				
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, TestFixtures.SSEEvents["RUN_STARTED"])
				fmt.Fprint(w, TestFixtures.SSEEvents["RUN_FINISHED"])
				w.(http.Flusher).Flush()
			}))
			defer server.Close()
			
			args := append([]string{tc.command}, tc.args...)
			args = append(args, "--server", server.URL)
			// Only add --interactive=false if the command supports it
			if tc.supportsInteractive {
				args = append(args, "--interactive=false")
			}
			
			cmd := newRootCommand()
			cmd.SetArgs(args)
			
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			
			err := cmd.Execute()
			// Some commands might have different success criteria
			if err != nil && !strings.Contains(err.Error(), "required") {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}