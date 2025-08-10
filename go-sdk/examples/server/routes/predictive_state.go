package routes

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
)

// PredictiveStateHandler creates the SSE endpoint for predictive state updates
func PredictiveStateHandler(cfg *config.Config) fiber.Handler {
	logger := slog.Default()

	return func(c fiber.Ctx) error {
		// Extract parameters
		requestID := c.Locals("requestid")
		if requestID == nil {
			requestID = "unknown"
		}

		// Structured logging context
		logCtx := []any{
			"request_id", requestID,
			"route", c.Route().Path,
		}

		logger.Info("Predictive state SSE connection initiated", logCtx...)

		// Set SSE headers
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Headers", "Cache-Control")

		// Create context for this connection
		connCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start streaming
		return c.SendStreamWriter(func(w *bufio.Writer) {
			if err := runPredictiveStateSequence(connCtx, w, logger, logCtx); err != nil {
				logger.Error("Predictive state sequence failed", append(logCtx, "error", err)...)
			}
		})
	}
}

// runPredictiveStateSequence executes the predictive state update flow
func runPredictiveStateSequence(ctx context.Context, w *bufio.Writer, logger *slog.Logger, logCtx []any) error {
	// Create initial state for this demo
	initialState := &DemoState{
		Counter:     0,
		Items:       []DemoItem{},
		LastUpdated: time.Now(),
		Version:     1,
	}

	// Step 1: Send initial STATE_SNAPSHOT
	snapshot := createStateSnapshot(initialState)
	if err := writePredictiveSSEEvent(w, snapshot, ""); err != nil {
		return fmt.Errorf("failed to write initial snapshot: %w", err)
	}
	logger.Debug("Sent initial snapshot", logCtx...)

	// Generate prediction ID for correlation
	predictionID := fmt.Sprintf("pred_%d", time.Now().UnixNano())

	// Step 2: Generate and send predictive delta
	predictiveDelta := createPredictiveDelta(initialState, predictionID)
	if err := writePredictiveSSEEvent(w, predictiveDelta, ""); err != nil {
		return fmt.Errorf("failed to write predictive delta: %w", err)
	}
	logger.Debug("Sent predictive delta", append(logCtx, "prediction_id", predictionID)...)

	// Step 3: Simulate server-side processing with delay
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second): // Simulate processing delay
		logger.Debug("Completed server processing", logCtx...)
	}

	// Step 4: Determine if prediction was correct (simulate with random chance)
	predictionCorrect := rand.Float64() > 0.3 // 70% chance of correct prediction

	// Step 5: Send reconciliation delta
	var reconciliationDelta *StateDelta
	if predictionCorrect {
		// Send confirming delta
		reconciliationDelta = createConfirmingDelta(predictionID)
		logger.Debug("Prediction was correct, sending confirmation", append(logCtx, "prediction_id", predictionID)...)
	} else {
		// Send corrective delta
		newState := &DemoState{
			Counter:     5, // Different from predicted value
			Items:       []DemoItem{{ID: "item_1", Value: "Corrected item", Type: "correction"}},
			LastUpdated: time.Now(),
			Version:     2,
		}
		reconciliationDelta = createCorrectiveDelta(initialState, newState, predictionID)
		logger.Debug("Prediction was incorrect, sending correction", append(logCtx, "prediction_id", predictionID)...)
	}

	if err := writePredictiveSSEEvent(w, reconciliationDelta, ""); err != nil {
		return fmt.Errorf("failed to write reconciliation delta: %w", err)
	}

	// Step 6: Optionally send final snapshot for clarity
	finalState := applyPredictedChanges(initialState, predictionCorrect)
	finalSnapshot := createStateSnapshot(finalState)
	if err := writePredictiveSSEEvent(w, finalSnapshot, ""); err != nil {
		return fmt.Errorf("failed to write final snapshot: %w", err)
	}
	logger.Debug("Sent final snapshot", logCtx...)

	// Keep connection alive for a bit to demonstrate completion
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(1 * time.Second):
		// Send completion event
		completionEvent := map[string]interface{}{
			"type":          "predictive_sequence_complete",
			"prediction_id": predictionID,
			"correct":       predictionCorrect,
			"timestamp":     time.Now().Format(time.RFC3339),
		}
		if err := writePredictiveSSEEvent(w, completionEvent, "completion"); err != nil {
			return fmt.Errorf("failed to write completion event: %w", err)
		}
	}

	return nil
}

// DemoState represents the state structure for the predictive demo
type DemoState struct {
	Counter     int        `json:"counter"`
	Items       []DemoItem `json:"items"`
	LastUpdated time.Time  `json:"last_updated"`
	Version     int        `json:"version"`
}

// DemoItem represents an item in the demo state
type DemoItem struct {
	ID    string `json:"id"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// StateSnapshot represents a complete state snapshot event
type StateSnapshot struct {
	Type      string     `json:"type"`
	State     *DemoState `json:"state"`
	Timestamp time.Time  `json:"timestamp"`
}

// StateDelta represents a state delta event with JSON Patch
type StateDelta struct {
	Type         string                   `json:"type"`
	Patches      []map[string]interface{} `json:"patches"`
	Predictive   bool                     `json:"predictive,omitempty"`
	PredictionID string                   `json:"prediction_id,omitempty"`
	Confirmed    bool                     `json:"confirmed,omitempty"`
	Corrective   bool                     `json:"corrective,omitempty"`
	Timestamp    time.Time                `json:"timestamp"`
}

// createStateSnapshot creates a STATE_SNAPSHOT event
func createStateSnapshot(state *DemoState) *StateSnapshot {
	return &StateSnapshot{
		Type:      "STATE_SNAPSHOT",
		State:     state,
		Timestamp: time.Now(),
	}
}

// createPredictiveDelta creates a predictive STATE_DELTA event
func createPredictiveDelta(currentState *DemoState, predictionID string) *StateDelta {
	// Predict incrementing counter and adding an item
	patches := []map[string]interface{}{
		{
			"op":    "replace",
			"path":  "/counter",
			"value": currentState.Counter + 3, // Predict +3
		},
		{
			"op":    "replace",
			"path":  "/version",
			"value": currentState.Version + 1,
		},
		{
			"op":   "add",
			"path": "/items/-",
			"value": DemoItem{
				ID:    "predicted_item_1",
				Value: "Predicted item",
				Type:  "prediction",
			},
		},
	}

	return &StateDelta{
		Type:         "STATE_DELTA",
		Patches:      patches,
		Predictive:   true,
		PredictionID: predictionID,
		Timestamp:    time.Now(),
	}
}

// createConfirmingDelta creates a confirming STATE_DELTA event
func createConfirmingDelta(predictionID string) *StateDelta {
	return &StateDelta{
		Type:         "STATE_DELTA",
		Patches:      []map[string]interface{}{}, // No changes needed, prediction was correct
		PredictionID: predictionID,
		Confirmed:    true,
		Timestamp:    time.Now(),
	}
}

// createCorrectiveDelta creates a corrective STATE_DELTA event
func createCorrectiveDelta(currentState, actualState *DemoState, predictionID string) *StateDelta {
	// Create patches to correct the state
	patches := []map[string]interface{}{
		{
			"op":    "replace",
			"path":  "/counter",
			"value": actualState.Counter, // Correct value
		},
		{
			"op":    "replace",
			"path":  "/version",
			"value": actualState.Version,
		},
		{
			"op":    "replace",
			"path":  "/items",
			"value": actualState.Items, // Replace entire items array
		},
	}

	return &StateDelta{
		Type:         "STATE_DELTA",
		Patches:      patches,
		PredictionID: predictionID,
		Corrective:   true,
		Timestamp:    time.Now(),
	}
}

// applyPredictedChanges applies the predicted changes to create final state
func applyPredictedChanges(initialState *DemoState, predictionCorrect bool) *DemoState {
	finalState := &DemoState{
		Counter:     initialState.Counter,
		Items:       make([]DemoItem, len(initialState.Items)),
		LastUpdated: time.Now(),
		Version:     initialState.Version + 1,
	}
	copy(finalState.Items, initialState.Items)

	if predictionCorrect {
		// Apply predicted changes
		finalState.Counter = initialState.Counter + 3
		finalState.Items = append(finalState.Items, DemoItem{
			ID:    "predicted_item_1",
			Value: "Predicted item",
			Type:  "prediction",
		})
	} else {
		// Apply corrected changes
		finalState.Counter = 5
		finalState.Items = []DemoItem{{
			ID:    "item_1",
			Value: "Corrected item",
			Type:  "correction",
		}}
	}

	return finalState
}

// writePredictiveSSEEvent writes an event in SSE format
func writePredictiveSSEEvent(w *bufio.Writer, data interface{}, eventType string) error {
	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	// Write event type if specified
	if eventType != "" {
		if _, err := w.WriteString(fmt.Sprintf("event: %s\n", eventType)); err != nil {
			return fmt.Errorf("failed to write event type: %w", err)
		}
	}

	// Escape newlines in JSON data for SSE format
	escapedData := strings.ReplaceAll(string(jsonData), "\n", "\\n")
	escapedData = strings.ReplaceAll(escapedData, "\r", "\\r")

	// Write data line
	if _, err := w.WriteString(fmt.Sprintf("data: %s\n\n", escapedData)); err != nil {
		return fmt.Errorf("failed to write event data: %w", err)
	}

	// Flush immediately
	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush event data: %w", err)
	}

	return nil
}
