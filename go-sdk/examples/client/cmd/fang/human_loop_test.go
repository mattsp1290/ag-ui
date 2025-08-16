package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHumanLoopBasic tests basic human-loop command execution
func TestHumanLoopBasic(t *testing.T) {
	// Create mock SSE server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/human_in_the_loop", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		
		// Verify request body
		var payload map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)
		
		assert.Equal(t, "test-session", payload["thread_id"])
		assert.Equal(t, "test-run", payload["run_id"])
		
		// Send SSE response
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		
		// Send events
		fmt.Fprintf(w, "data: %s\n\n", `{"type": "RUN_STARTED", "threadId": "test-session", "runId": "test-run"}`)
		fmt.Fprintf(w, "data: %s\n\n", `{"type": "RUN_FINISHED", "threadId": "test-session", "runId": "test-run"}`)
		w.(http.Flusher).Flush()
	}))
	defer server.Close()
	
	// Test command execution
	// Create and execute command
	rootCmd := newRootCommand()
	rootCmd.SetArgs([]string{
		"human-loop",
		"--message", "Test message",
		"--server", server.URL,
		"--session-id", "test-session",
		"--run-id", "test-run",
		"--json",
		"--timeout", "2s",
	})
	
	// Execute command
	output := &bytes.Buffer{}
	rootCmd.SetOut(output)
	rootCmd.SetErr(output)
	
	err := rootCmd.Execute()
	assert.NoError(t, err)
}

// TestHumanLoopToolCallEvents tests TOOL_CALL event handling
func TestHumanLoopToolCallEvents(t *testing.T) {
	tests := []struct {
		name     string
		events   []string
		expected []string
	}{
		{
			name: "tool_call_start",
			events: []string{
				`{"type": "RUN_STARTED", "threadId": "test", "runId": "run1"}`,
				`{"type": "TOOL_CALL_START", "toolCallId": "tc1", "toolName": "generate_haiku"}`,
				`{"type": "RUN_FINISHED", "threadId": "test", "runId": "run1"}`,
			},
			expected: []string{
				"generate_haiku",
				"Tool execution started",
			},
		},
		{
			name: "tool_call_with_args",
			events: []string{
				`{"type": "RUN_STARTED", "threadId": "test", "runId": "run1"}`,
				`{"type": "TOOL_CALL_START", "toolCallId": "tc1", "toolName": "weather_api"}`,
				`{"type": "TOOL_CALL_ARGS", "toolCallId": "tc1", "args": {"location": "Tokyo"}}`,
				`{"type": "TOOL_CALL_END", "toolCallId": "tc1", "result": {"temperature": "20°C"}}`,
				`{"type": "RUN_FINISHED", "threadId": "test", "runId": "run1"}`,
			},
			expected: []string{
				"weather_api",
				"Receiving arguments",
				"Tool completed",
			},
		},
		{
			name: "multiple_tools",
			events: []string{
				`{"type": "RUN_STARTED", "threadId": "test", "runId": "run1"}`,
				`{"type": "TOOL_CALL_START", "toolCallId": "tc1", "toolName": "tool1"}`,
				`{"type": "TOOL_CALL_END", "toolCallId": "tc1"}`,
				`{"type": "TOOL_CALL_START", "toolCallId": "tc2", "toolName": "tool2"}`,
				`{"type": "TOOL_CALL_END", "toolCallId": "tc2"}`,
				`{"type": "RUN_FINISHED", "threadId": "test", "runId": "run1"}`,
			},
			expected: []string{
				"tool1",
				"tool2",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock SSE server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				
				// Send events
				for _, event := range tt.events {
					fmt.Fprintf(w, "data: %s\n\n", event)
					w.(http.Flusher).Flush()
					time.Sleep(10 * time.Millisecond)
				}
			}))
			defer server.Close()
			
			// Test command execution
			rootCmd := newRootCommand()
			
			// Set command arguments
			rootCmd.SetArgs([]string{
				"human-loop",
				"--message", "Test message",
				"--server", server.URL,
				"--approval", "auto",
				"--timeout", "2s",
			})
			
			// Execute command
			output := &bytes.Buffer{}
			rootCmd.SetOut(output)
			rootCmd.SetErr(output)
			
			err := rootCmd.Execute()
			assert.NoError(t, err)
			
			// Verify output contains expected strings
			outputStr := output.String()
			for _, expected := range tt.expected {
				assert.Contains(t, outputStr, expected)
			}
		})
	}
}

// TestHumanLoopApprovalWorkflow tests manual and auto approval modes
func TestHumanLoopApprovalWorkflow(t *testing.T) {
	tests := []struct {
		name         string
		approvalMode string
		userInput    string
		expectPrompt bool
	}{
		{
			name:         "auto_approval",
			approvalMode: "auto",
			userInput:    "",
			expectPrompt: false,
		},
		{
			name:         "manual_approval_approve",
			approvalMode: "manual",
			userInput:    "a\n",
			expectPrompt: true,
		},
		{
			name:         "manual_approval_reject",
			approvalMode: "manual",
			userInput:    "r\n",
			expectPrompt: true,
		},
		{
			name:         "manual_approval_skip",
			approvalMode: "manual",
			userInput:    "s\n",
			expectPrompt: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock SSE server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				
				// Send events
				fmt.Fprintf(w, "data: %s\n\n", `{"type": "RUN_STARTED", "threadId": "test", "runId": "run1"}`)
				fmt.Fprintf(w, "data: %s\n\n", `{"type": "TOOL_CALL_START", "toolCallId": "tc1", "toolName": "test_tool"}`)
				
				// Only send TOOL_CALL_END if not rejected
				if !strings.Contains(tt.userInput, "r") {
					fmt.Fprintf(w, "data: %s\n\n", `{"type": "TOOL_CALL_END", "toolCallId": "tc1"}`)
				}
				
				fmt.Fprintf(w, "data: %s\n\n", `{"type": "RUN_FINISHED", "threadId": "test", "runId": "run1"}`)
				w.(http.Flusher).Flush()
			}))
			defer server.Close()
			
			// Test command execution
			rootCmd := newRootCommand()
			
			// Set command arguments
			rootCmd.SetArgs([]string{
				"human-loop",
				"--message", "Test message",
				"--server", server.URL,
				"--approval", tt.approvalMode,
				"--timeout", "2s",
			})
			
			// Mock stdin for manual approval
			if tt.expectPrompt {
				rootCmd.SetIn(strings.NewReader(tt.userInput))
			}
			
			// Execute command
			output := &bytes.Buffer{}
			rootCmd.SetOut(output)
			rootCmd.SetErr(output)
			
			err := rootCmd.Execute()
			
			// Check for approval prompts
			outputStr := output.String()
			if tt.expectPrompt {
				assert.Contains(t, outputStr, "[A]pprove")
				
				if strings.Contains(tt.userInput, "a") {
					assert.Contains(t, outputStr, "approved")
				} else if strings.Contains(tt.userInput, "r") {
					assert.Contains(t, outputStr, "rejected")
				} else if strings.Contains(tt.userInput, "s") {
					assert.Contains(t, outputStr, "skipped")
				}
			}
			
			// Manual rejection should not error
			if strings.Contains(tt.userInput, "r") {
				// Command should exit gracefully
				assert.NoError(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestHumanLoopMessageSnapshot tests MESSAGES_SNAPSHOT handling
func TestHumanLoopMessageSnapshot(t *testing.T) {
	// Create mock SSE server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		
		// Send events with MESSAGES_SNAPSHOT
		fmt.Fprintf(w, "data: %s\n\n", `{"type": "RUN_STARTED", "threadId": "test", "runId": "run1"}`)
		
		messagesSnapshot := map[string]interface{}{
			"type": "MESSAGES_SNAPSHOT",
			"messages": []map[string]interface{}{
				{
					"id":      "msg1",
					"role":    "user",
					"content": "Generate a haiku",
				},
				{
					"id":   "msg2",
					"role": "assistant",
					"toolCalls": []map[string]interface{}{
						{
							"id":   "tc1",
							"type": "function",
							"function": map[string]interface{}{
								"name":      "generate_haiku",
								"arguments": `{"topic": "programming"}`,
							},
						},
					},
				},
				{
					"id":         "msg3",
					"role":       "tool",
					"content":    "Haiku generated successfully",
					"toolCallId": "tc1",
				},
			},
		}
		
		snapshotJSON, _ := json.Marshal(messagesSnapshot)
		fmt.Fprintf(w, "data: %s\n\n", string(snapshotJSON))
		fmt.Fprintf(w, "data: %s\n\n", `{"type": "RUN_FINISHED", "threadId": "test", "runId": "run1"}`)
		w.(http.Flusher).Flush()
	}))
	defer server.Close()
	
	// Test command execution
	rootCmd := newRootCommand()
	
	// Set command arguments
	rootCmd.SetArgs([]string{
		"human-loop",
		"--message", "Generate a haiku",
		"--server", server.URL,
		"--json",
		"--timeout", "2s",
	})
	
	// Execute command
	output := &bytes.Buffer{}
	rootCmd.SetOut(output)
	rootCmd.SetErr(output)
	
	err := rootCmd.Execute()
	assert.NoError(t, err)
	
	// Verify JSON output contains message snapshot
	outputStr := output.String()
	assert.Contains(t, outputStr, "MESSAGES_SNAPSHOT")
	assert.Contains(t, outputStr, "generate_haiku")
	assert.Contains(t, outputStr, "toolCalls")
}

// TestHumanLoopErrorHandling tests error scenarios
func TestHumanLoopErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		serverURL   string
		serverError bool
		expectError bool
		errorMsg    string
	}{
		{
			name:        "missing_message",
			message:     "",
			serverURL:   "http://localhost:8000",
			expectError: true,
			errorMsg:    "message is required",
		},
		{
			name:        "missing_server",
			message:     "Test",
			serverURL:   "http://invalid-server-that-does-not-exist:99999",  // Invalid server to force connection error
			expectError: true,
			errorMsg:    "failed to connect to server",
		},
		{
			name:        "server_error",
			message:     "Test",
			serverURL:   "will-be-replaced",
			serverError: true,
			expectError: true,
			errorMsg:    "500",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.serverError {
				// Create server that returns error
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("Internal Server Error"))
				}))
				defer server.Close()
				tt.serverURL = server.URL
			} else if tt.serverURL == "will-be-replaced" {
				// Create normal server
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)
					fmt.Fprintf(w, "data: %s\n\n", `{"type": "RUN_STARTED"}`)
					fmt.Fprintf(w, "data: %s\n\n", `{"type": "RUN_FINISHED"}`)
					w.(http.Flusher).Flush()
				}))
				defer server.Close()
				tt.serverURL = server.URL
			}
			
			// Test command execution
			rootCmd := newRootCommand()
			
			// Build command arguments
			args := []string{"human-loop"}
			if tt.message != "" {
				args = append(args, "--message", tt.message)
			}
			if tt.serverURL != "" {
				args = append(args, "--server", tt.serverURL)
			}
			args = append(args, "--timeout", "1s")
			
			// Set command arguments
			rootCmd.SetArgs(args)
			
			// Execute command
			output := &bytes.Buffer{}
			rootCmd.SetOut(output)
			rootCmd.SetErr(output)
			
			err := rootCmd.Execute()
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, output.String(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestHumanLoopOutputModes tests JSON and pretty output modes
func TestHumanLoopOutputModes(t *testing.T) {
	tests := []struct {
		name       string
		jsonMode   bool
		checkFor   []string
		checkNotIn []string
	}{
		{
			name:     "json_output",
			jsonMode: true,
			checkFor: []string{
				`"type"`,
				`"TOOL_CALL_START"`,
				`"toolCallId"`,
				`"toolName"`,
			},
			checkNotIn: []string{
				"🔧",
				"Tool execution started",
			},
		},
		{
			name:     "pretty_output",
			jsonMode: false,
			checkFor: []string{
				"🔧",
				"Tool execution started",
				"generate_haiku",
			},
			checkNotIn: []string{
				`"type"`,
				`"TOOL_CALL_START"`,
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock SSE server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				
				// Send events
				fmt.Fprintf(w, "data: %s\n\n", `{"type": "RUN_STARTED", "threadId": "test", "runId": "run1"}`)
				fmt.Fprintf(w, "data: %s\n\n", `{"type": "TOOL_CALL_START", "toolCallId": "tc1", "toolName": "generate_haiku"}`)
				fmt.Fprintf(w, "data: %s\n\n", `{"type": "TOOL_CALL_END", "toolCallId": "tc1", "result": {"haiku": "Code flows like water"}}`)
				fmt.Fprintf(w, "data: %s\n\n", `{"type": "RUN_FINISHED", "threadId": "test", "runId": "run1"}`)
				w.(http.Flusher).Flush()
			}))
			defer server.Close()
			
			// Test command execution
			rootCmd := newRootCommand()
			
			// Build command arguments
			args := []string{
				"human-loop",
				"--message", "Test message",
				"--server", server.URL,
				"--approval", "auto",
				"--timeout", "2s",
			}
			if tt.jsonMode {
				args = append(args, "--json")
			}
			
			// Set command arguments
			rootCmd.SetArgs(args)
			
			// Execute command
			output := &bytes.Buffer{}
			rootCmd.SetOut(output)
			rootCmd.SetErr(output)
			
			err := rootCmd.Execute()
			assert.NoError(t, err)
			
			outputStr := output.String()
			
			// Check for expected strings
			for _, expected := range tt.checkFor {
				assert.Contains(t, outputStr, expected, "Output should contain: %s", expected)
			}
			
			// Check strings that should not be present
			for _, notExpected := range tt.checkNotIn {
				assert.NotContains(t, outputStr, notExpected, "Output should not contain: %s", notExpected)
			}
		})
	}
}

// TestHumanLoopStreamingArgs tests handling of streaming tool arguments
func TestHumanLoopStreamingArgs(t *testing.T) {
	// Create mock SSE server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		
		// Send events with streaming arguments
		fmt.Fprintf(w, "data: %s\n\n", `{"type": "RUN_STARTED", "threadId": "test", "runId": "run1"}`)
		fmt.Fprintf(w, "data: %s\n\n", `{"type": "TOOL_CALL_START", "toolCallId": "tc1", "toolName": "complex_tool"}`)
		
		// Stream arguments in chunks
		fmt.Fprintf(w, "data: %s\n\n", `{"type": "TOOL_CALL_ARGS", "toolCallId": "tc1", "chunk": "{\"param1\":"}`)
		w.(http.Flusher).Flush()
		time.Sleep(10 * time.Millisecond)
		
		fmt.Fprintf(w, "data: %s\n\n", `{"type": "TOOL_CALL_ARGS", "toolCallId": "tc1", "chunk": "\"value1\","}`)
		w.(http.Flusher).Flush()
		time.Sleep(10 * time.Millisecond)
		
		fmt.Fprintf(w, "data: %s\n\n", `{"type": "TOOL_CALL_ARGS", "toolCallId": "tc1", "chunk": "\"param2\":\"value2\"}"}`)
		w.(http.Flusher).Flush()
		
		fmt.Fprintf(w, "data: %s\n\n", `{"type": "TOOL_CALL_END", "toolCallId": "tc1"}`)
		fmt.Fprintf(w, "data: %s\n\n", `{"type": "RUN_FINISHED", "threadId": "test", "runId": "run1"}`)
		w.(http.Flusher).Flush()
	}))
	defer server.Close()
	
	// Test command execution
	rootCmd := newRootCommand()
	
	// Set command arguments
	rootCmd.SetArgs([]string{
		"human-loop",
		"--message", "Test streaming args",
		"--server", server.URL,
		"--approval", "auto",
		"--timeout", "2s",
	})
	
	// Execute command
	output := &bytes.Buffer{}
	rootCmd.SetOut(output)
	rootCmd.SetErr(output)
	
	err := rootCmd.Execute()
	assert.NoError(t, err)
	
	// Verify output shows argument reception
	outputStr := output.String()
	assert.Contains(t, outputStr, "Receiving arguments")
	assert.Contains(t, outputStr, "complex_tool")
}

// TestHumanLoopTimeout tests timeout handling
func TestHumanLoopTimeout(t *testing.T) {
	// Create mock SSE server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		
		// Send initial event
		fmt.Fprintf(w, "data: %s\n\n", `{"type": "RUN_STARTED", "threadId": "test", "runId": "run1"}`)
		w.(http.Flusher).Flush()
		
		// Delay longer than timeout
		time.Sleep(2 * time.Second)
		
		// This event won't be received due to timeout
		fmt.Fprintf(w, "data: %s\n\n", `{"type": "RUN_FINISHED", "threadId": "test", "runId": "run1"}`)
		w.(http.Flusher).Flush()
	}))
	defer server.Close()
	
	// Test command execution
	rootCmd := newRootCommand()
	
	// Set command arguments
	rootCmd.SetArgs([]string{
		"human-loop",
		"--message", "Test timeout",
		"--server", server.URL,
		"--timeout", "500ms",
	})
	
	// Execute command
	output := &bytes.Buffer{}
	rootCmd.SetOut(output)
	rootCmd.SetErr(output)
	
	start := time.Now()
	err := rootCmd.Execute()
	duration := time.Since(start)
	
	// Should timeout within reasonable time
	assert.Less(t, duration, 1*time.Second)
	
	// Timeout should not cause error (graceful handling)
	// The command handles timeout internally
	if err != nil {
		assert.Contains(t, err.Error(), "timeout")
	}
}

// Note: The actual newHumanLoopCommand is in main.go.
// For testing, we'll use the root command to access it.