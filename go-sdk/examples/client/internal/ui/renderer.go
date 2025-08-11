package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	jsonpatch "github.com/evanphx/json-patch/v5"
)

// OutputMode represents the output format
type OutputMode string

const (
	OutputModePretty OutputMode = "pretty"
	OutputModeJSON   OutputMode = "json"
)

// RendererConfig configures the UI renderer
type RendererConfig struct {
	OutputMode    OutputMode
	NoColor       bool
	Quiet         bool
	Writer        io.Writer
	MaxBufferSize int // Maximum size for message buffers
}

// Renderer handles rendering of UI updates and message deltas
type Renderer struct {
	config   RendererConfig
	messages map[string]*MessageState // Track messages by messageId
	state    map[string]interface{}   // Current application state
	mu       sync.RWMutex
	writer   io.Writer
	
	// Color functions for pretty mode
	infoColor    *color.Color
	successColor *color.Color
	errorColor   *color.Color
	dimColor     *color.Color
	boldColor    *color.Color
}

// MessageState tracks the state of an assistant message
type MessageState struct {
	ID         string
	Role       string
	Content    strings.Builder
	StartTime  time.Time
	EndTime    *time.Time
	IsComplete bool
}

// NewRenderer creates a new UI renderer
func NewRenderer(config RendererConfig) *Renderer {
	if config.Writer == nil {
		config.Writer = os.Stdout
	}
	
	if config.MaxBufferSize == 0 {
		config.MaxBufferSize = 1024 * 1024 // 1MB default
	}
	
	r := &Renderer{
		config:   config,
		messages: make(map[string]*MessageState),
		state:    make(map[string]interface{}),
		writer:   config.Writer,
	}
	
	// Initialize colors if not disabled
	if !config.NoColor && config.OutputMode == OutputModePretty {
		r.infoColor = color.New(color.FgCyan)
		r.successColor = color.New(color.FgGreen)
		r.errorColor = color.New(color.FgRed)
		r.dimColor = color.New(color.Faint)
		r.boldColor = color.New(color.Bold)
	} else {
		// Use no-op colors
		r.infoColor = color.New()
		r.successColor = color.New()
		r.errorColor = color.New()
		r.dimColor = color.New()
		r.boldColor = color.New()
	}
	
	return r
}

// HandleEvent processes an SSE event and renders appropriate output
func (r *Renderer) HandleEvent(eventType string, data json.RawMessage) error {
	if r.config.Quiet {
		return nil
	}
	
	switch eventType {
	case "TEXT_MESSAGE_START":
		return r.handleTextMessageStart(data)
	case "TEXT_MESSAGE_CONTENT":
		return r.handleTextMessageContent(data)
	case "TEXT_MESSAGE_END":
		return r.handleTextMessageEnd(data)
	case "TEXT_MESSAGE_CHUNK":
		return r.handleTextMessageChunk(data)
	case "STATE_SNAPSHOT":
		return r.handleStateSnapshot(data)
	case "STATE_DELTA":
		return r.handleStateDelta(data)
	case "TOOL_CALL_START":
		return r.handleToolCallStart(data)
	case "TOOL_CALL_ARGS":
		return r.handleToolCallArgs(data)
	case "TOOL_CALL_END":
		return r.handleToolCallEnd(data)
	case "TOOL_CALL_RESULT":
		return r.handleToolCallResult(data)
	case "THINKING_START":
		return r.handleThinkingStart(data)
	case "THINKING_TEXT_MESSAGE_CONTENT":
		return r.handleThinkingContent(data)
	case "THINKING_END":
		return r.handleThinkingEnd(data)
	default:
		// Unknown events are logged but don't cause errors
		if r.config.OutputMode == OutputModeJSON {
			return r.outputJSON(map[string]interface{}{
				"event": eventType,
				"data":  data,
			})
		}
		return nil
	}
}

// handleTextMessageStart handles the start of a text message
func (r *Renderer) handleTextMessageStart(data json.RawMessage) error {
	var payload struct {
		MessageID string `json:"messageId"`
		Role      string `json:"role"`
	}
	
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal TEXT_MESSAGE_START: %w", err)
	}
	
	r.mu.Lock()
	r.messages[payload.MessageID] = &MessageState{
		ID:        payload.MessageID,
		Role:      payload.Role,
		StartTime: time.Now(),
	}
	r.mu.Unlock()
	
	if r.config.OutputMode == OutputModeJSON {
		return r.outputJSON(map[string]interface{}{
			"event": "TEXT_MESSAGE_START",
			"data":  payload,
		})
	}
	
	// Pretty mode: show message start
	r.dimColor.Fprintf(r.writer, "\n[%s] ", payload.Role)
	return nil
}

// handleTextMessageContent handles message content chunks
func (r *Renderer) handleTextMessageContent(data json.RawMessage) error {
	var payload struct {
		MessageID string `json:"messageId"`
		Content   string `json:"content"`
	}
	
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal TEXT_MESSAGE_CONTENT: %w", err)
	}
	
	r.mu.Lock()
	msg, exists := r.messages[payload.MessageID]
	if !exists {
		// Create message if it doesn't exist (for resilience)
		msg = &MessageState{
			ID:        payload.MessageID,
			StartTime: time.Now(),
		}
		r.messages[payload.MessageID] = msg
	}
	
	// Check buffer size limit
	if msg.Content.Len()+len(payload.Content) > r.config.MaxBufferSize {
		r.mu.Unlock()
		return fmt.Errorf("message buffer size exceeded for message %s", payload.MessageID)
	}
	
	msg.Content.WriteString(payload.Content)
	r.mu.Unlock()
	
	if r.config.OutputMode == OutputModeJSON {
		return r.outputJSON(map[string]interface{}{
			"event": "TEXT_MESSAGE_CONTENT",
			"data":  payload,
		})
	}
	
	// Pretty mode: stream content directly
	fmt.Fprint(r.writer, payload.Content)
	return nil
}

// handleTextMessageEnd handles the end of a text message
func (r *Renderer) handleTextMessageEnd(data json.RawMessage) error {
	var payload struct {
		MessageID string `json:"messageId"`
	}
	
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal TEXT_MESSAGE_END: %w", err)
	}
	
	r.mu.Lock()
	if msg, exists := r.messages[payload.MessageID]; exists {
		now := time.Now()
		msg.EndTime = &now
		msg.IsComplete = true
	}
	r.mu.Unlock()
	
	if r.config.OutputMode == OutputModeJSON {
		return r.outputJSON(map[string]interface{}{
			"event": "TEXT_MESSAGE_END",
			"data":  payload,
		})
	}
	
	// Pretty mode: add newline after message
	fmt.Fprintln(r.writer)
	return nil
}

// handleTextMessageChunk handles optional message chunks
func (r *Renderer) handleTextMessageChunk(data json.RawMessage) error {
	// TEXT_MESSAGE_CHUNK is optional and similar to CONTENT
	return r.handleTextMessageContent(data)
}

// handleStateSnapshot handles state snapshot events
func (r *Renderer) handleStateSnapshot(data json.RawMessage) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal STATE_SNAPSHOT: %w", err)
	}
	
	r.mu.Lock()
	r.state = payload
	r.mu.Unlock()
	
	if r.config.OutputMode == OutputModeJSON {
		return r.outputJSON(map[string]interface{}{
			"event": "STATE_SNAPSHOT",
			"data":  payload,
		})
	}
	
	// Pretty mode: show state summary
	r.infoColor.Fprintln(r.writer, "\n📊 State Updated:")
	r.renderStateSummary(payload)
	return nil
}

// handleStateDelta handles state delta events (JSON Patch)
func (r *Renderer) handleStateDelta(data json.RawMessage) error {
	var patches []interface{}
	if err := json.Unmarshal(data, &patches); err != nil {
		return fmt.Errorf("failed to unmarshal STATE_DELTA: %w", err)
	}
	
	r.mu.Lock()
	// Apply JSON patches to current state
	currentStateJSON, err := json.Marshal(r.state)
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to marshal current state: %w", err)
	}
	
	patchJSON, err := json.Marshal(patches)
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to marshal patches: %w", err)
	}
	
	patch, err := jsonpatch.DecodePatch(patchJSON)
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to decode patch: %w", err)
	}
	
	modifiedJSON, err := patch.Apply(currentStateJSON)
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to apply patch: %w", err)
	}
	
	if err := json.Unmarshal(modifiedJSON, &r.state); err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to unmarshal modified state: %w", err)
	}
	r.mu.Unlock()
	
	if r.config.OutputMode == OutputModeJSON {
		return r.outputJSON(map[string]interface{}{
			"event": "STATE_DELTA",
			"data":  patches,
		})
	}
	
	// Pretty mode: show delta operations
	r.infoColor.Fprintln(r.writer, "\n📝 State Changes:")
	for _, p := range patches {
		if patchMap, ok := p.(map[string]interface{}); ok {
			op := patchMap["op"]
			path := patchMap["path"]
			value := patchMap["value"]
			
			switch op {
			case "add":
				r.successColor.Fprintf(r.writer, "  + %s → %v\n", path, value)
			case "remove":
				r.errorColor.Fprintf(r.writer, "  - %s\n", path)
			case "replace":
				fmt.Fprintf(r.writer, "  ~ %s → %v\n", path, value)
			default:
				fmt.Fprintf(r.writer, "  %s %s %v\n", op, path, value)
			}
		}
	}
	return nil
}

// handleToolCallStart handles tool call start events
func (r *Renderer) handleToolCallStart(data json.RawMessage) error {
	var payload struct {
		ToolCallID string `json:"toolCallId"`
		ToolName   string `json:"toolName"`
	}
	
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal TOOL_CALL_START: %w", err)
	}
	
	if r.config.OutputMode == OutputModeJSON {
		return r.outputJSON(map[string]interface{}{
			"event": "TOOL_CALL_START",
			"data":  payload,
		})
	}
	
	// Pretty mode
	r.boldColor.Fprintf(r.writer, "\n🔧 Tool Call: %s\n", payload.ToolName)
	r.dimColor.Fprintf(r.writer, "   ID: %s\n", payload.ToolCallID)
	return nil
}

// handleToolCallArgs handles tool call arguments
func (r *Renderer) handleToolCallArgs(data json.RawMessage) error {
	var payload struct {
		ToolCallID string                 `json:"toolCallId"`
		Arguments  map[string]interface{} `json:"arguments"`
	}
	
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal TOOL_CALL_ARGS: %w", err)
	}
	
	if r.config.OutputMode == OutputModeJSON {
		return r.outputJSON(map[string]interface{}{
			"event": "TOOL_CALL_ARGS",
			"data":  payload,
		})
	}
	
	// Pretty mode: show arguments
	if len(payload.Arguments) > 0 {
		r.dimColor.Fprintln(r.writer, "   Arguments:")
		for key, value := range payload.Arguments {
			fmt.Fprintf(r.writer, "     %s: %v\n", key, value)
		}
	}
	return nil
}

// handleToolCallEnd handles tool call end events
func (r *Renderer) handleToolCallEnd(data json.RawMessage) error {
	var payload struct {
		ToolCallID string `json:"toolCallId"`
	}
	
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal TOOL_CALL_END: %w", err)
	}
	
	if r.config.OutputMode == OutputModeJSON {
		return r.outputJSON(map[string]interface{}{
			"event": "TOOL_CALL_END",
			"data":  payload,
		})
	}
	
	// Pretty mode
	r.dimColor.Fprintf(r.writer, "   ✓ Tool call completed\n")
	return nil
}

// handleToolCallResult handles tool call result events
func (r *Renderer) handleToolCallResult(data json.RawMessage) error {
	var payload struct {
		ToolCallID string      `json:"toolCallId"`
		Result     interface{} `json:"result"`
		Error      *string     `json:"error,omitempty"`
	}
	
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal TOOL_CALL_RESULT: %w", err)
	}
	
	if r.config.OutputMode == OutputModeJSON {
		return r.outputJSON(map[string]interface{}{
			"event": "TOOL_CALL_RESULT",
			"data":  payload,
		})
	}
	
	// Pretty mode: show result card
	if payload.Error != nil {
		r.errorColor.Fprintf(r.writer, "\n❌ Tool Error: %s\n", *payload.Error)
	} else {
		r.successColor.Fprintln(r.writer, "\n✅ Tool Result:")
		// Format result based on type
		switch v := payload.Result.(type) {
		case string:
			fmt.Fprintf(r.writer, "   %s\n", v)
		case map[string]interface{}:
			for key, value := range v {
				fmt.Fprintf(r.writer, "   %s: %v\n", key, value)
			}
		default:
			resultJSON, _ := json.MarshalIndent(payload.Result, "   ", "  ")
			fmt.Fprintf(r.writer, "%s\n", resultJSON)
		}
	}
	return nil
}

// handleThinkingStart handles thinking phase start
func (r *Renderer) handleThinkingStart(data json.RawMessage) error {
	if r.config.OutputMode == OutputModeJSON {
		return r.outputJSON(map[string]interface{}{
			"event": "THINKING_START",
			"data":  data,
		})
	}
	
	// Pretty mode
	r.dimColor.Fprintln(r.writer, "\n💭 Thinking...")
	return nil
}

// handleThinkingContent handles thinking phase content
func (r *Renderer) handleThinkingContent(data json.RawMessage) error {
	var payload struct {
		Content string `json:"content"`
	}
	
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal THINKING_TEXT_MESSAGE_CONTENT: %w", err)
	}
	
	if r.config.OutputMode == OutputModeJSON {
		return r.outputJSON(map[string]interface{}{
			"event": "THINKING_TEXT_MESSAGE_CONTENT",
			"data":  payload,
		})
	}
	
	// Pretty mode: show thinking content dimmed
	r.dimColor.Fprint(r.writer, payload.Content)
	return nil
}

// handleThinkingEnd handles thinking phase end
func (r *Renderer) handleThinkingEnd(data json.RawMessage) error {
	if r.config.OutputMode == OutputModeJSON {
		return r.outputJSON(map[string]interface{}{
			"event": "THINKING_END",
			"data":  data,
		})
	}
	
	// Pretty mode
	fmt.Fprintln(r.writer)
	return nil
}

// renderStateSummary renders a summary of the state in pretty mode
func (r *Renderer) renderStateSummary(state map[string]interface{}) {
	if len(state) == 0 {
		r.dimColor.Fprintln(r.writer, "  (empty state)")
		return
	}
	
	// Show top-level keys and simple values
	for key, value := range state {
		switch v := value.(type) {
		case string:
			fmt.Fprintf(r.writer, "  %s: %q\n", key, v)
		case float64, int, bool:
			fmt.Fprintf(r.writer, "  %s: %v\n", key, v)
		case []interface{}:
			fmt.Fprintf(r.writer, "  %s: [%d items]\n", key, len(v))
		case map[string]interface{}:
			fmt.Fprintf(r.writer, "  %s: {%d fields}\n", key, len(v))
		case nil:
			fmt.Fprintf(r.writer, "  %s: null\n", key)
		default:
			fmt.Fprintf(r.writer, "  %s: %T\n", key, v)
		}
	}
}

// outputJSON outputs data in JSON format (one line per object)
func (r *Renderer) outputJSON(data interface{}) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Fprintln(r.writer, string(encoded))
	return nil
}

// GetMessage returns a message by ID
func (r *Renderer) GetMessage(messageID string) (*MessageState, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	msg, exists := r.messages[messageID]
	return msg, exists
}

// GetState returns the current state
func (r *Renderer) GetState() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Return a copy to prevent external modifications
	stateCopy := make(map[string]interface{})
	for k, v := range r.state {
		stateCopy[k] = v
	}
	return stateCopy
}

// Clear clears all tracked messages and state
func (r *Renderer) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.messages = make(map[string]*MessageState)
	r.state = make(map[string]interface{})
}