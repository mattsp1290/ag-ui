package routes

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
)

// createTestConfig creates a config optimized for fast tests
func createTestConfig() *config.Config {
	cfg := config.New()
	cfg.StreamingChunkDelay = 1 * time.Millisecond // Very short delay for tests
	return cfg
}

func TestHumanInTheLoopHandler_Headers(t *testing.T) {
	cfg := createTestConfig()
	handler := HumanInTheLoopHandler(cfg)
	app := fiber.New()
	app.Post("/test", handler)

	// Create test request with valid messages
	input := RunAgentInput{
		Messages: []map[string]interface{}{
			{"role": "user", "content": "Hello, please help me with a task"},
		},
	}
	reqBody, _ := json.Marshal(input)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify SSE headers
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "keep-alive", resp.Header.Get("Connection"))
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Cache-Control", resp.Header.Get("Access-Control-Allow-Headers"))
}

func TestHumanInTheLoopHandler_InvalidRequest(t *testing.T) {
	cfg := createTestConfig()
	handler := HumanInTheLoopHandler(cfg)
	app := fiber.New()
	app.Post("/test", handler)

	testCases := []struct {
		name     string
		reqBody  string
		expected string
	}{
		{
			name:     "empty request body",
			reqBody:  "",
			expected: "Invalid request body",
		},
		{
			name:     "invalid json",
			reqBody:  `{"invalid": json}`,
			expected: "Invalid request body",
		},
		{
			name:     "empty messages array",
			reqBody:  `{"messages": []}`,
			expected: "Messages array cannot be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", strings.NewReader(tc.reqBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			assert.Contains(t, result["error"].(string), tc.expected)
		})
	}
}

func TestHumanInTheLoopHandler_ToolBranch(t *testing.T) {
	cfg := createTestConfig()
	handler := HumanInTheLoopHandler(cfg)
	app := fiber.New()
	app.Post("/test", handler)

	// Create request with tool message as last message
	input := RunAgentInput{
		Messages: []map[string]interface{}{
			{"role": "user", "content": "Please analyze this data"},
			{"role": "tool", "content": "Tool execution result"},
		},
	}
	reqBody, _ := json.Marshal(input)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Parse SSE stream
	events := parseSSEStream(t, resp.Body)

	// Verify event sequence for tool branch (assistant text response)
	require.GreaterOrEqual(t, len(events), 4, "Should have at least 4 events: RUN_STARTED, TEXT_MESSAGE_START, TEXT_MESSAGE_CONTENT, TEXT_MESSAGE_END, RUN_FINISHED")

	// Check event types in order
	expectedTypes := []string{"RUN_STARTED", "TEXT_MESSAGE_START", "TEXT_MESSAGE_CONTENT", "TEXT_MESSAGE_END", "RUN_FINISHED"}
	for i, expectedType := range expectedTypes {
		if i < len(events) {
			assert.Equal(t, expectedType, events[i]["type"], "Event %d should be %s", i, expectedType)
		}
	}

	// Verify assistant message content
	var foundContent bool
	for _, event := range events {
		if event["type"] == "TEXT_MESSAGE_CONTENT" {
			if delta, ok := event["delta"].(string); ok && strings.Contains(delta, "Thank you for using the tool") {
				foundContent = true
				break
			}
		}
	}
	assert.True(t, foundContent, "Should contain assistant response content")
}

func TestHumanInTheLoopHandler_NonToolBranch(t *testing.T) {
	cfg := createTestConfig()
	handler := HumanInTheLoopHandler(cfg)
	app := fiber.New()
	app.Post("/test", handler)

	// Create request with user message as last message
	input := RunAgentInput{
		Messages: []map[string]interface{}{
			{"role": "user", "content": "Please help me create a plan"},
		},
	}
	reqBody, _ := json.Marshal(input)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Parse SSE stream
	events := parseSSEStream(t, resp.Body)

	// Should have more events due to tool call with multiple TOOL_CALL_ARGS
	require.GreaterOrEqual(t, len(events), 5, "Should have at least RUN_STARTED, TOOL_CALL_START, TOOL_CALL_ARGS (multiple), TOOL_CALL_END, RUN_FINISHED")

	// Check for required event types
	eventTypes := make(map[string]int)
	for _, event := range events {
		eventTypes[event["type"].(string)]++
	}

	assert.Equal(t, 1, eventTypes["RUN_STARTED"], "Should have exactly 1 RUN_STARTED event")
	assert.Equal(t, 1, eventTypes["TOOL_CALL_START"], "Should have exactly 1 TOOL_CALL_START event")
	assert.GreaterOrEqual(t, eventTypes["TOOL_CALL_ARGS"], 3, "Should have multiple TOOL_CALL_ARGS events")
	assert.Equal(t, 1, eventTypes["TOOL_CALL_END"], "Should have exactly 1 TOOL_CALL_END event")
	assert.Equal(t, 1, eventTypes["RUN_FINISHED"], "Should have exactly 1 RUN_FINISHED event")

	// Verify tool call details
	var toolCallName string
	var toolCallID string
	for _, event := range events {
		if event["type"] == "TOOL_CALL_START" {
			toolCallName = event["toolCallName"].(string)
			toolCallID = event["toolCallId"].(string)
			break
		}
	}
	assert.Equal(t, "generate_task_steps", toolCallName, "Tool call should be generate_task_steps")
	assert.NotEmpty(t, toolCallID, "Tool call ID should not be empty")

	// Verify JSON concatenation validity
	argsEvents := []string{}
	for _, event := range events {
		if event["type"] == "TOOL_CALL_ARGS" {
			argsEvents = append(argsEvents, event["delta"].(string))
		}
	}

	// Concatenate all TOOL_CALL_ARGS deltas
	concatenatedJSON := strings.Join(argsEvents, "")

	// Verify it's valid JSON
	var result map[string]interface{}
	err = json.Unmarshal([]byte(concatenatedJSON), &result)
	require.NoError(t, err, "Concatenated TOOL_CALL_ARGS should form valid JSON: %s", concatenatedJSON)

	// Verify the structure contains steps array
	steps, ok := result["steps"].([]interface{})
	require.True(t, ok, "Result should contain steps array")
	assert.GreaterOrEqual(t, len(steps), 5, "Steps array should contain multiple steps")

	// Verify each step has expected structure
	for i, stepInterface := range steps {
		step, ok := stepInterface.(map[string]interface{})
		require.True(t, ok, "Each step should be an object")
		assert.Contains(t, step, "step", "Step %d should have 'step' field", i)
		assert.Contains(t, step, "description", "Step %d should have 'description' field", i)
	}
}

func TestHumanInTheLoopHandler_EventSequence(t *testing.T) {
	cfg := createTestConfig()
	handler := HumanInTheLoopHandler(cfg)
	app := fiber.New()
	app.Post("/test", handler)

	// Test both branches to ensure proper sequencing
	testCases := []struct {
		name         string
		lastRole     messages.MessageRole
		expectedFlow []string
	}{
		{
			name:     "tool_branch",
			lastRole: messages.RoleTool,
			expectedFlow: []string{
				"RUN_STARTED",
				"TEXT_MESSAGE_START",
				"TEXT_MESSAGE_CONTENT",
				"TEXT_MESSAGE_END",
				"RUN_FINISHED",
			},
		},
		{
			name:     "non_tool_branch",
			lastRole: messages.RoleUser,
			expectedFlow: []string{
				"RUN_STARTED",
				"TOOL_CALL_START",
				"TOOL_CALL_ARGS", // Multiple instances expected
				"TOOL_CALL_END",
				"RUN_FINISHED",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := RunAgentInput{
				Messages: []map[string]interface{}{
					{"role": string(tc.lastRole), "content": "Test message content"},
				},
			}
			reqBody, _ := json.Marshal(input)

			req := httptest.NewRequest("POST", "/test", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			events := parseSSEStream(t, resp.Body)
			require.NotEmpty(t, events, "Should receive events")

			// Check that events start and end correctly
			assert.Equal(t, "RUN_STARTED", events[0]["type"], "First event should be RUN_STARTED")
			assert.Equal(t, "RUN_FINISHED", events[len(events)-1]["type"], "Last event should be RUN_FINISHED")

			// Check threadId and runId consistency for run events
			var threadID, runID string
			for _, event := range events {
				if event["type"] == "RUN_STARTED" {
					threadID = event["threadId"].(string)
					runID = event["runId"].(string)
				} else if event["type"] == "RUN_FINISHED" {
					assert.Equal(t, threadID, event["threadId"], "Thread ID should be consistent")
					assert.Equal(t, runID, event["runId"], "Run ID should be consistent")
				}
			}
			assert.NotEmpty(t, threadID, "Thread ID should be set")
			assert.NotEmpty(t, runID, "Run ID should be set")
		})
	}
}

func TestHumanInTheLoopHandler_CancellationHandling(t *testing.T) {
	cfg := createTestConfig()
	handler := HumanInTheLoopHandler(cfg)
	app := fiber.New()
	app.Post("/test", handler)

	// Create request that would trigger the longer tool call flow
	input := RunAgentInput{
		Messages: []map[string]interface{}{
			{"role": "user", "content": "Please help me"},
		},
	}
	reqBody, _ := json.Marshal(input)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// Create a context with short timeout to test cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	// This should handle cancellation gracefully
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 1000 * time.Millisecond})

	// The request should complete (even if truncated) without panicking
	require.NoError(t, err)
	if resp != nil {
		defer resp.Body.Close()
		// Read what we can from the response
		events := parseSSEStream(t, resp.Body)
		// We should at least get the RUN_STARTED event
		if len(events) > 0 {
			assert.Equal(t, "RUN_STARTED", events[0]["type"])
		}
	}
}

// parseSSEStream parses Server-Sent Events from a response body
func parseSSEStream(t *testing.T, body io.Reader) []map[string]interface{} {
	var events []map[string]interface{}
	scanner := bufio.NewScanner(body)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataJSON := strings.TrimPrefix(line, "data: ")
			if dataJSON == "" {
				continue
			}

			var event map[string]interface{}
			err := json.Unmarshal([]byte(dataJSON), &event)
			if err != nil {
				t.Logf("Failed to parse SSE event JSON: %s, error: %v", dataJSON, err)
				continue
			}
			events = append(events, event)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Logf("Scanner error: %v", err)
	}

	return events
}

// Benchmark test for performance validation
func BenchmarkHumanInTheLoopHandler_ToolBranch(b *testing.B) {
	cfg := createTestConfig()
	handler := HumanInTheLoopHandler(cfg)
	app := fiber.New()
	app.Post("/test", handler)

	input := RunAgentInput{
		Messages: []map[string]interface{}{
			{"role": "tool", "content": "Benchmark test result"},
		},
	}
	reqBody, _ := json.Marshal(input)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/test", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}

		// Read and discard the response to complete the request
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkHumanInTheLoopHandler_NonToolBranch(b *testing.B) {
	cfg := createTestConfig()
	handler := HumanInTheLoopHandler(cfg)
	app := fiber.New()
	app.Post("/test", handler)

	input := RunAgentInput{
		Messages: []map[string]interface{}{
			{"role": "user", "content": "Benchmark test request"},
		},
	}
	reqBody, _ := json.Marshal(input)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/test", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}

		// Read and discard the response to complete the request
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
