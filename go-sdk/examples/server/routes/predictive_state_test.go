package routes

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPredictiveStateHandler(t *testing.T) {
	cfg := &config.Config{
		Host:         "localhost",
		Port:         8080,
		EnableSSE:    true,
		CORSEnabled:  true,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	app := fiber.New()
	handler := PredictiveStateHandler(cfg)
	app.Get("/test", handler)

	// Make request with SSE accept header
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response headers are set correctly for SSE
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "keep-alive", resp.Header.Get("Connection"))
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))

	// Just verify we get a 200 response and some content
	assert.Equal(t, 200, resp.StatusCode)
	
	// Read some initial content to verify streaming starts
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	
	// Check that we got some SSE-formatted content
	content := string(body)
	assert.Contains(t, content, "data:", "Should contain SSE data events")
}

func TestPredictiveStateSequence(t *testing.T) {
	// Test the predictive state sequence logic in isolation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var output strings.Builder
	w := bufio.NewWriter(&output)

	// Create a test logger to avoid nil pointer
	logger := slog.Default()

	// Run the predictive state sequence
	err := runPredictiveStateSequence(ctx, w, logger, []any{"test_context"})
	require.NoError(t, err)

	// Flush any remaining data
	w.Flush()
	result := output.String()

	// Verify the sequence of events
	assert.Contains(t, result, `"type":"STATE_SNAPSHOT"`)
	assert.Contains(t, result, `"type":"STATE_DELTA"`)
	assert.Contains(t, result, `"predictive":true`)
	
	// Should contain either confirmation or correction
	hasConfirmed := strings.Contains(result, `"confirmed":true`)
	hasCorrective := strings.Contains(result, `"corrective":true`)
	assert.True(t, hasConfirmed || hasCorrective, "Should contain either confirmation or correction")

	// Should contain completion event
	assert.Contains(t, result, `"predictive_sequence_complete"`)
}

func TestCreatePredictiveDelta(t *testing.T) {
	initialState := &DemoState{
		Counter:     0,
		Items:       []DemoItem{},
		LastUpdated: time.Now(),
		Version:     1,
	}

	predictionID := "test_prediction_123"
	delta := createPredictiveDelta(initialState, predictionID)

	assert.Equal(t, "STATE_DELTA", delta.Type)
	assert.True(t, delta.Predictive)
	assert.Equal(t, predictionID, delta.PredictionID)
	assert.Len(t, delta.Patches, 3) // counter, version, items

	// Verify patches
	counterPatch := delta.Patches[0]
	assert.Equal(t, "replace", counterPatch["op"])
	assert.Equal(t, "/counter", counterPatch["path"])
	assert.Equal(t, 3, counterPatch["value"]) // Predicted +3

	versionPatch := delta.Patches[1]
	assert.Equal(t, "replace", versionPatch["op"])
	assert.Equal(t, "/version", versionPatch["path"])
	assert.Equal(t, 2, versionPatch["value"]) // Version incremented

	itemPatch := delta.Patches[2]
	assert.Equal(t, "add", itemPatch["op"])
	assert.Equal(t, "/items/-", itemPatch["path"])
	
	// Verify the item being added
	addedItem, ok := itemPatch["value"].(DemoItem)
	require.True(t, ok)
	assert.Equal(t, "predicted_item_1", addedItem.ID)
	assert.Equal(t, "Predicted item", addedItem.Value)
	assert.Equal(t, "prediction", addedItem.Type)
}

func TestCreateConfirmingDelta(t *testing.T) {
	predictionID := "test_prediction_123"
	delta := createConfirmingDelta(predictionID)

	assert.Equal(t, "STATE_DELTA", delta.Type)
	assert.True(t, delta.Confirmed)
	assert.Equal(t, predictionID, delta.PredictionID)
	assert.Empty(t, delta.Patches) // No changes needed for confirmation
}

func TestCreateCorrectiveDelta(t *testing.T) {
	currentState := &DemoState{
		Counter:     0,
		Items:       []DemoItem{},
		LastUpdated: time.Now(),
		Version:     1,
	}

	actualState := &DemoState{
		Counter:     5,
		Items:       []DemoItem{{ID: "corrected_item", Value: "Corrected", Type: "correction"}},
		LastUpdated: time.Now(),
		Version:     2,
	}

	predictionID := "test_prediction_123"
	delta := createCorrectiveDelta(currentState, actualState, predictionID)

	assert.Equal(t, "STATE_DELTA", delta.Type)
	assert.True(t, delta.Corrective)
	assert.Equal(t, predictionID, delta.PredictionID)
	assert.Len(t, delta.Patches, 3) // counter, version, items

	// Verify corrective patches
	counterPatch := delta.Patches[0]
	assert.Equal(t, "replace", counterPatch["op"])
	assert.Equal(t, "/counter", counterPatch["path"])
	assert.Equal(t, 5, counterPatch["value"]) // Corrected value

	versionPatch := delta.Patches[1]
	assert.Equal(t, "replace", versionPatch["op"])
	assert.Equal(t, "/version", versionPatch["path"])
	assert.Equal(t, 2, versionPatch["value"])

	itemsPatch := delta.Patches[2]
	assert.Equal(t, "replace", itemsPatch["op"])
	assert.Equal(t, "/items", itemsPatch["path"])
	
	// Verify the corrected items
	correctedItems, ok := itemsPatch["value"].([]DemoItem)
	require.True(t, ok)
	require.Len(t, correctedItems, 1)
	assert.Equal(t, "corrected_item", correctedItems[0].ID)
	assert.Equal(t, "Corrected", correctedItems[0].Value)
	assert.Equal(t, "correction", correctedItems[0].Type)
}

func TestApplyPredictedChanges(t *testing.T) {
	initialState := &DemoState{
		Counter:     0,
		Items:       []DemoItem{},
		LastUpdated: time.Now(),
		Version:     1,
	}

	// Test correct prediction
	finalStateCorrect := applyPredictedChanges(initialState, true)
	assert.Equal(t, 3, finalStateCorrect.Counter)
	require.Len(t, finalStateCorrect.Items, 1)
	assert.Equal(t, "predicted_item_1", finalStateCorrect.Items[0].ID)
	assert.Equal(t, 2, finalStateCorrect.Version)

	// Test incorrect prediction
	finalStateIncorrect := applyPredictedChanges(initialState, false)
	assert.Equal(t, 5, finalStateIncorrect.Counter)
	require.Len(t, finalStateIncorrect.Items, 1)
	assert.Equal(t, "item_1", finalStateIncorrect.Items[0].ID)
	assert.Equal(t, "Corrected item", finalStateIncorrect.Items[0].Value)
	assert.Equal(t, 2, finalStateIncorrect.Version)
}

func TestWritePredictiveSSEEvent(t *testing.T) {
	var buffer strings.Builder
	w := bufio.NewWriter(&buffer)

	testData := map[string]interface{}{
		"type":    "TEST_EVENT",
		"message": "Test message",
	}

	err := writePredictiveSSEEvent(w, testData, "test")
	require.NoError(t, err)

	w.Flush()
	output := buffer.String()

	// Verify SSE format
	assert.Contains(t, output, "event: test\n")
	assert.Contains(t, output, "data: ")
	assert.Contains(t, output, `"type":"TEST_EVENT"`)
	assert.Contains(t, output, `"message":"Test message"`)
	assert.Contains(t, output, "\n\n") // SSE event terminator
}