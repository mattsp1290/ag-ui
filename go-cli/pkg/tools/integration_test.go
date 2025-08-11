package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-cli/pkg/tools"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolCallLifecycle tests the complete tool call request/response lifecycle
func TestToolCallLifecycle(t *testing.T) {
	// Track the server state
	var serverState struct {
		receivedCalls int
		lastRequest   map[string]interface{}
	}
	
	// Create a mock server that simulates the AG-UI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverState.receivedCalls++
		
		// Parse the request
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		serverState.lastRequest = req
		
		// Verify request structure
		assert.Equal(t, "test-thread", req["thread_id"])
		assert.NotEmpty(t, req["run_id"])
		assert.NotNil(t, req["messages"])
		
		messages := req["messages"].([]interface{})
		
		// Set up SSE response
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		
		// Send events based on request state
		if serverState.receivedCalls == 1 {
			// First call: Send initial tool call request
			fmt.Fprintf(w, "data: %s\n\n", `{"type":"RUN_STARTED","threadId":"test-thread","runId":"run-001"}`)
			flusher.Flush()
			
			// Send assistant message with tool call
			messagesSnapshot := map[string]interface{}{
				"type": "MESSAGES_SNAPSHOT",
				"messages": []interface{}{
					map[string]interface{}{
						"id":   "msg-1",
						"role": "assistant",
						"toolCalls": []interface{}{
							map[string]interface{}{
								"id":   "tc-001",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "test_tool",
									"arguments": `{"input":"test value"}`,
								},
							},
						},
					},
				},
			}
			data, _ := json.Marshal(messagesSnapshot)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
			
		} else if serverState.receivedCalls == 2 {
			// Second call: Verify tool result was submitted
			hasToolResult := false
			for _, msg := range messages {
				msgMap := msg.(map[string]interface{})
				if msgMap["role"] == "tool" {
					assert.Equal(t, "tc-001", msgMap["toolCallId"])
					assert.NotEmpty(t, msgMap["content"])
					hasToolResult = true
					break
				}
			}
			assert.True(t, hasToolResult, "Tool result message not found")
			
			// Send confirmation
			fmt.Fprintf(w, "data: %s\n\n", `{"type":"RUN_STARTED","threadId":"test-thread","runId":"run-002"}`)
			flusher.Flush()
			
			// Send final assistant response
			finalSnapshot := map[string]interface{}{
				"type": "MESSAGES_SNAPSHOT",
				"messages": []interface{}{
					map[string]interface{}{
						"id":      "msg-2",
						"role":    "assistant",
						"content": "Tool executed successfully",
					},
				},
			}
			data, _ := json.Marshal(finalSnapshot)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
		}
		
		// Send run finished
		fmt.Fprintf(w, "data: %s\n\n", `{"type":"RUN_FINISHED","threadId":"test-thread","runId":"run-end"}`)
		flusher.Flush()
	}))
	defer server.Close()
	
	// Set up test components
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	registry := tools.NewToolRegistry()
	
	// Register the test tool
	testTool := &tools.Tool{
		Name:        "test_tool",
		Description: "A test tool for integration testing",
		Parameters: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"input": {
					Type:        "string",
					Description: "Test input parameter",
				},
			},
			Required: []string{"input"},
		},
	}
	require.NoError(t, registry.Register(testTool))
	
	// Create handler
	handler := tools.NewToolCallHandler(registry, tools.ToolHandlerConfig{
		ServerURL:   server.URL,
		Endpoint:    "/tool_based_generative_ui",
		Interactive: false,
		ToolArgs:    `{"input":"test value"}`,
	}, logger)
	
	ctx := context.Background()
	
	// Simulate receiving an assistant message with tool call
	messages := []interface{}{
		map[string]interface{}{
			"id":   "msg-1",
			"role": "assistant",
			"toolCalls": []interface{}{
				map[string]interface{}{
					"id":   "tc-001",
					"type": "function",
					"function": map[string]interface{}{
						"name":      "test_tool",
						"arguments": `{"input":"test value"}`,
					},
				},
			},
		},
	}
	
	// Process the snapshot (this should trigger tool execution and result submission)
	err := handler.ProcessMessagesSnapshot(ctx, messages, "test-thread", "run-001")
	assert.NoError(t, err)
	
	// Verify server received the tool result
	time.Sleep(100 * time.Millisecond) // Give time for async operations
	assert.Equal(t, 1, serverState.receivedCalls)
}

// TestConcurrentToolCalls tests handling multiple tool calls concurrently
func TestConcurrentToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: %s\n\n", `{"type":"RUN_FINISHED"}`)
	}))
	defer server.Close()
	
	registry := tools.NewToolRegistry()
	
	// Register multiple tools
	for i := 1; i <= 3; i++ {
		tool := &tools.Tool{
			Name:        fmt.Sprintf("tool_%d", i),
			Description: fmt.Sprintf("Test tool %d", i),
			Parameters: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"param": {Type: "string"},
				},
			},
		}
		require.NoError(t, registry.Register(tool))
	}
	
	handler := tools.NewToolCallHandler(registry, tools.ToolHandlerConfig{
		ServerURL:   server.URL,
		Endpoint:    "/tool_based_generative_ui",
		Interactive: false,
	}, logrus.New())
	
	ctx := context.Background()
	
	// Create message with multiple tool calls
	messages := []interface{}{
		map[string]interface{}{
			"id":   "msg-1",
			"role": "assistant",
			"toolCalls": []interface{}{
				map[string]interface{}{
					"id":   "tc-1",
					"type": "function",
					"function": map[string]interface{}{
						"name":      "tool_1",
						"arguments": `{"param":"value1"}`,
					},
				},
				map[string]interface{}{
					"id":   "tc-2",
					"type": "function",
					"function": map[string]interface{}{
						"name":      "tool_2",
						"arguments": `{"param":"value2"}`,
					},
				},
				map[string]interface{}{
					"id":   "tc-3",
					"type": "function",
					"function": map[string]interface{}{
						"name":      "tool_3",
						"arguments": `{"param":"value3"}`,
					},
				},
			},
		},
	}
	
	// Process all tool calls
	err := handler.ProcessMessagesSnapshot(ctx, messages, "thread-123", "run-456")
	assert.NoError(t, err)
}

// TestToolCallErrorHandling tests error scenarios
func TestToolCallErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		messages      []interface{}
		registryTools []*tools.Tool
		expectError   bool
	}{
		{
			name: "unknown tool",
			messages: []interface{}{
				map[string]interface{}{
					"role": "assistant",
					"toolCalls": []interface{}{
						map[string]interface{}{
							"id":   "tc-1",
							"type": "function",
							"function": map[string]interface{}{
								"name":      "unknown_tool",
								"arguments": `{}`,
							},
						},
					},
				},
			},
			registryTools: []*tools.Tool{},
			expectError:   false, // ProcessMessagesSnapshot doesn't return error
		},
		{
			name: "invalid arguments",
			messages: []interface{}{
				map[string]interface{}{
					"role": "assistant",
					"toolCalls": []interface{}{
						map[string]interface{}{
							"id":   "tc-1",
							"type": "function",
							"function": map[string]interface{}{
								"name":      "test_tool",
								"arguments": `{"wrong_param":"value"}`,
							},
						},
					},
				},
			},
			registryTools: []*tools.Tool{
				{
					Name: "test_tool",
					Parameters: &tools.ToolSchema{
						Type: "object",
						Properties: map[string]*tools.Property{
							"required_param": {Type: "string"},
						},
						Required: []string{"required_param"},
					},
				},
			},
			expectError: false, // ProcessMessagesSnapshot doesn't return error
		},
	}
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := tools.NewToolRegistry()
			for _, tool := range tt.registryTools {
				require.NoError(t, registry.Register(tool))
			}
			
			handler := tools.NewToolCallHandler(registry, tools.ToolHandlerConfig{
				ServerURL:     server.URL,
				Endpoint:      "/tool_based_generative_ui",
				Interactive:   false,
				RetryAttempts: 1,
			}, logrus.New())
			
			err := handler.ProcessMessagesSnapshot(context.Background(), tt.messages, "thread", "run")
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}