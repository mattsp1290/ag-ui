package routes

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSharedStateUpdateHandler(t *testing.T) {
	// Create test app
	app := fiber.New()
	cfg := &config.Config{}

	// Reset shared store to clean state
	sharedStore = state.NewStore()

	// Add route
	app.Post("/update", SharedStateUpdateHandler(cfg))

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectedOp     string
	}{
		{
			name:           "increment counter",
			requestBody:    `{"op": "increment_counter"}`,
			expectedStatus: 200,
			expectedOp:     "increment_counter",
		},
		{
			name:           "add item",
			requestBody:    `{"op": "add_item", "value": {"value": "test item", "type": "test"}}`,
			expectedStatus: 200,
			expectedOp:     "add_item",
		},
		{
			name:           "reset counter",
			requestBody:    `{"op": "reset_counter"}`,
			expectedStatus: 200,
			expectedOp:     "reset_counter",
		},
		{
			name:           "invalid JSON",
			requestBody:    `{invalid json}`,
			expectedStatus: 400,
		},
		{
			name:           "missing op field",
			requestBody:    `{"action": "increment"}`,
			expectedStatus: 400,
		},
		{
			name:           "unknown operation",
			requestBody:    `{"op": "unknown_op"}`,
			expectedStatus: 200, // Should succeed but be ignored
			expectedOp:     "unknown_op",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/update", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			if tt.expectedStatus == 200 {
				var response map[string]interface{}
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)

				err = json.Unmarshal(body, &response)
				require.NoError(t, err)

				assert.True(t, response["success"].(bool))
				assert.Equal(t, tt.expectedOp, response["operation"])

				// Verify state field exists
				stateInfo, ok := response["state"].(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, stateInfo, "version")
				assert.Contains(t, stateInfo, "counter")
				assert.Contains(t, stateInfo, "items_count")
				assert.Contains(t, stateInfo, "watchers")
			}
		})
	}
}

func TestSharedStateSSEHandler(t *testing.T) {
	// Create test app
	app := fiber.New()
	cfg := &config.Config{}

	// Reset shared store
	sharedStore = state.NewStore()

	// Add route
	app.Get("/stream", SharedStateHandler(cfg))

	t.Run("basic SSE connection", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/stream", nil)
		req.Header.Set("Accept", "text/event-stream")

		resp, err := app.Test(req)
		require.NoError(t, err)

		// Check headers
		assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
		assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
		assert.Equal(t, "keep-alive", resp.Header.Get("Connection"))

		// Read response body
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		bodyStr := string(body)

		// Should contain initial STATE_SNAPSHOT
		assert.Contains(t, bodyStr, "data:")
		assert.Contains(t, bodyStr, "STATE_SNAPSHOT")
	})

	t.Run("SSE with demo flag", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/stream?demo=true", nil)
		req.Header.Set("Accept", "text/event-stream")

		resp, err := app.Test(req)
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		bodyStr := string(body)

		// Should contain STATE_SNAPSHOT at the beginning
		assert.Contains(t, bodyStr, "STATE_SNAPSHOT")

		// May contain keepalive events depending on timing
		lines := strings.Split(bodyStr, "\n")
		dataLines := 0
		for _, line := range lines {
			if strings.HasPrefix(line, "data:") {
				dataLines++
			}
		}

		// Should have at least the initial snapshot
		assert.GreaterOrEqual(t, dataLines, 1)
	})

	t.Run("SSE with correlation ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/stream?cid=test-123", nil)

		resp, err := app.Test(req)
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		bodyStr := string(body)
		assert.Contains(t, bodyStr, "STATE_SNAPSHOT")
	})
}

func TestSharedStateIntegration(t *testing.T) {
	// Create test app with both endpoints
	app := fiber.New()
	cfg := &config.Config{}

	// Reset shared store
	sharedStore = state.NewStore()

	app.Get("/stream", SharedStateHandler(cfg))
	app.Post("/update", SharedStateUpdateHandler(cfg))

	t.Run("state changes propagate to SSE stream", func(t *testing.T) {
		// This test is complex because we need to simulate concurrent SSE streaming
		// and state updates. For simplicity, we'll test the components separately
		// and verify the store integration.

		// First, verify initial state
		snapshot := sharedStore.Snapshot()
		assert.Equal(t, int64(1), snapshot.Version)
		assert.Equal(t, 0, snapshot.Counter)
		assert.Equal(t, 0, len(snapshot.Items))

		// Create a watcher to monitor changes
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		watcher, err := sharedStore.Watch(ctx)
		require.NoError(t, err)

		// Perform state update via HTTP endpoint
		updateReq := httptest.NewRequest("POST", "/update",
			strings.NewReader(`{"op": "increment_counter"}`))
		updateReq.Header.Set("Content-Type", "application/json")

		updateResp, err := app.Test(updateReq)
		require.NoError(t, err)
		assert.Equal(t, 200, updateResp.StatusCode)

		// Wait for state change notification
		select {
		case delta := <-watcher.Channel():
			assert.Equal(t, "STATE_DELTA", delta.Type)
			assert.Equal(t, int64(2), delta.Version)
			assert.NotEmpty(t, delta.Patch)

			// Verify patch contains counter increment
			found := false
			for _, op := range delta.Patch {
				if path, ok := op["path"].(string); ok && path == "/counter" {
					if opType, ok := op["op"].(string); ok && opType == "replace" {
						if value, ok := op["value"].(float64); ok && value == 1 {
							found = true
							break
						}
					}
				}
			}
			assert.True(t, found, "Should find counter increment in patch")

		case <-ctx.Done():
			t.Fatal("Timeout waiting for state change")
		}

		// Verify final state
		finalSnapshot := sharedStore.Snapshot()
		assert.Equal(t, int64(2), finalSnapshot.Version)
		assert.Equal(t, 1, finalSnapshot.Counter)
	})
}

func TestSSEEventFormatting(t *testing.T) {
	tests := []struct {
		name      string
		data      interface{}
		eventType string
		expected  []string // Strings that should be in the output
	}{
		{
			name: "simple object",
			data: map[string]interface{}{
				"type":  "test",
				"value": 123,
			},
			eventType: "",
			expected: []string{
				"data: {",
				"\"type\":\"test\"",
				"\"value\":123",
			},
		},
		{
			name:      "with event type",
			data:      map[string]string{"message": "hello"},
			eventType: "custom",
			expected: []string{
				"event: custom",
				"data: {",
				"\"message\":\"hello\"",
			},
		},
		{
			name: "state snapshot",
			data: state.NewStateSnapshot(&state.State{
				Version: 5,
				Counter: 10,
				Items:   []state.Item{{ID: "test", Value: "value"}},
			}),
			eventType: "",
			expected: []string{
				"data: {",
				"\"type\":\"STATE_SNAPSHOT\"",
				"\"version\":5",
				"\"counter\":10",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			bufWriter := bufio.NewWriter(buf)
			err := writeSSEEvent(bufWriter, tt.data, tt.eventType)
			bufWriter.Flush()
			require.NoError(t, err)

			output := buf.String()

			for _, expected := range tt.expected {
				assert.Contains(t, output, expected)
			}

			// Verify it ends with double newline
			assert.True(t, strings.HasSuffix(output, "\n\n"))

			// If event type is specified, verify it's included
			if tt.eventType != "" {
				assert.Contains(t, output, fmt.Sprintf("event: %s\n", tt.eventType))
			}
		})
	}
}

func TestStateOperations(t *testing.T) {
	// Test various state operations work correctly
	store := state.NewStore()

	// Test increment
	err := store.Update(func(s *state.State) {
		s.Counter++
	})
	require.NoError(t, err)

	snapshot := store.Snapshot()
	assert.Equal(t, 1, snapshot.Counter)
	assert.Equal(t, int64(2), snapshot.Version)

	// Test add item
	err = store.Update(func(s *state.State) {
		s.Items = append(s.Items, state.Item{
			ID:    "test-item",
			Value: "test-value",
			Type:  "test",
		})
	})
	require.NoError(t, err)

	snapshot = store.Snapshot()
	assert.Equal(t, 1, len(snapshot.Items))
	assert.Equal(t, "test-item", snapshot.Items[0].ID)
	assert.Equal(t, int64(3), snapshot.Version)

	// Test clear items
	err = store.Update(func(s *state.State) {
		s.Items = make([]state.Item, 0)
	})
	require.NoError(t, err)

	snapshot = store.Snapshot()
	assert.Equal(t, 0, len(snapshot.Items))
	assert.Equal(t, int64(4), snapshot.Version)
}

func TestErrorHandling(t *testing.T) {
	app := fiber.New()
	cfg := &config.Config{}

	app.Post("/update", SharedStateUpdateHandler(cfg))

	t.Run("malformed JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/update",
			strings.NewReader(`{"op": "increment_counter"`)) // Missing closing brace
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode)
	})

	t.Run("empty body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/update", strings.NewReader(""))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode)
	})
}
