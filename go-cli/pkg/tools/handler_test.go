package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewToolCallHandler(t *testing.T) {
	registry := NewToolRegistry()
	config := ToolHandlerConfig{
		ServerURL: "http://localhost:8000",
		Endpoint:  "/tool_based_generative_ui",
	}
	logger := logrus.New()
	
	handler := NewToolCallHandler(registry, config, logger)
	
	assert.NotNil(t, handler)
	assert.Equal(t, registry, handler.registry)
	assert.NotNil(t, handler.client)
	assert.Equal(t, 30*time.Second, handler.config.Timeout)
	assert.Equal(t, 3, handler.config.RetryAttempts)
}

func TestParseAndValidateArgs(t *testing.T) {
	registry := NewToolRegistry()
	
	// Register a test tool
	tool := &Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"param1": {
					Type:        "string",
					Description: "First parameter",
				},
				"param2": {
					Type:        "number",
					Description: "Second parameter",
				},
			},
			Required: []string{"param1"},
		},
	}
	require.NoError(t, registry.Register(tool))
	
	handler := NewToolCallHandler(registry, ToolHandlerConfig{}, nil)
	
	tests := []struct {
		name      string
		toolCall  *ToolCall
		wantError bool
		expected  map[string]interface{}
	}{
		{
			name: "valid arguments",
			toolCall: &ToolCall{
				ID:   "tc-123",
				Type: "function",
				Function: FunctionCall{
					Name:      "test_tool",
					Arguments: `{"param1": "value1", "param2": 42}`,
				},
			},
			wantError: false,
			expected: map[string]interface{}{
				"param1": "value1",
				"param2": float64(42),
			},
		},
		{
			name: "missing required argument",
			toolCall: &ToolCall{
				ID:   "tc-124",
				Type: "function",
				Function: FunctionCall{
					Name:      "test_tool",
					Arguments: `{"param2": 42}`,
				},
			},
			wantError: true,
		},
		{
			name: "invalid JSON",
			toolCall: &ToolCall{
				ID:   "tc-125",
				Type: "function",
				Function: FunctionCall{
					Name:      "test_tool",
					Arguments: `{invalid json}`,
				},
			},
			wantError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := handler.parseAndValidateArgs(tt.toolCall, tool)
			
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, args)
			}
		})
	}
}

func TestCreateToolResultMessage(t *testing.T) {
	handler := NewToolCallHandler(nil, ToolHandlerConfig{}, nil)
	
	toolCallID := "tc-123"
	content := "Tool executed successfully"
	
	msg := handler.createToolResultMessage(toolCallID, content)
	
	assert.Equal(t, "tool", msg["role"])
	assert.Equal(t, content, msg["content"])
	assert.Equal(t, toolCallID, msg["toolCallId"])
	assert.NotEmpty(t, msg["id"])
}

func TestSubmitToolResult(t *testing.T) {
	// Create a test server
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/tool_based_generative_ui", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))
		
		// Parse request body
		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		require.NoError(t, err)
		
		// Send SSE response
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"type\":\"RUN_STARTED\"}\n\n"))
	}))
	defer server.Close()
	
	handler := NewToolCallHandler(nil, ToolHandlerConfig{
		ServerURL: server.URL,
		Endpoint:  "/tool_based_generative_ui",
	}, nil)
	
	ctx := context.Background()
	threadID := "thread-123"
	runID := "run-456"
	messages := []interface{}{
		map[string]interface{}{
			"id":         "msg-1",
			"role":       "tool",
			"content":    "Result",
			"toolCallId": "tc-123",
		},
	}
	
	err := handler.submitToolResult(ctx, threadID, runID, messages)
	assert.NoError(t, err)
	
	// Verify request body
	assert.Equal(t, threadID, receivedBody["thread_id"])
	assert.Equal(t, runID, receivedBody["run_id"])
	assert.Equal(t, messages, receivedBody["messages"])
}

func TestProcessMessagesSnapshot(t *testing.T) {
	registry := NewToolRegistry()
	
	// Register a test tool
	tool := &Tool{
		Name:        "generate_haiku",
		Description: "Generate a haiku",
		Parameters: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"topic": {
					Type: "string",
				},
			},
		},
	}
	require.NoError(t, registry.Register(tool))
	
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	handler := NewToolCallHandler(registry, ToolHandlerConfig{
		ServerURL:   server.URL,
		Endpoint:    "/tool_based_generative_ui",
		Interactive: false,
	}, logrus.New())
	
	ctx := context.Background()
	messages := []interface{}{
		map[string]interface{}{
			"id":   "msg-1",
			"role": "assistant",
			"toolCalls": []interface{}{
				map[string]interface{}{
					"id":   "tc-123",
					"type": "function",
					"function": map[string]interface{}{
						"name":      "generate_haiku",
						"arguments": `{"topic": "spring"}`,
					},
				},
			},
		},
	}
	
	err := handler.ProcessMessagesSnapshot(ctx, messages, "thread-123", "run-456")
	// The handler will try to execute the tool, which will fail since it's not implemented
	// But it should not return an error from ProcessMessagesSnapshot
	assert.NoError(t, err)
}

func TestHandleToolCallRequest(t *testing.T) {
	registry := NewToolRegistry()
	
	// Register a test tool
	tool := &Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"input": {
					Type: "string",
				},
			},
			Required: []string{"input"},
		},
	}
	require.NoError(t, registry.Register(tool))
	
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	handler := NewToolCallHandler(registry, ToolHandlerConfig{
		ServerURL:   server.URL,
		Endpoint:    "/tool_based_generative_ui",
		Interactive: false,
	}, logrus.New())
	
	ctx := context.Background()
	toolCall := &ToolCall{
		ID:   "tc-123",
		Type: "function",
		Function: FunctionCall{
			Name:      "test_tool",
			Arguments: `{"input": "test value"}`,
		},
	}
	
	messages := []interface{}{
		map[string]interface{}{
			"id":   "msg-1",
			"role": "user",
			"content": "Test message",
		},
	}
	
	err := handler.HandleToolCallRequest(ctx, toolCall, "thread-123", "run-456", messages)
	assert.NoError(t, err)
}

func TestRetryLogic(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			// Fail the first attempt
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			// Succeed on the second attempt
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()
	
	handler := NewToolCallHandler(nil, ToolHandlerConfig{
		ServerURL:     server.URL,
		Endpoint:      "/tool_based_generative_ui",
		RetryAttempts: 3,
		RetryDelay:    10 * time.Millisecond,
	}, logrus.New())
	
	ctx := context.Background()
	err := handler.submitToolResult(ctx, "thread-123", "run-456", []interface{}{})
	
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}