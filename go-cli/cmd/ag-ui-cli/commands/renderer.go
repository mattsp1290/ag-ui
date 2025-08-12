package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// ChatRenderer defines the interface for rendering chat events
type ChatRenderer interface {
	OnTextMessageStart(data string) error
	OnTextMessageContent(data string) error
	OnTextMessageEnd(data string) error
	OnToolCallRequested(data string) error
	OnToolCallResult(data string) error
	OnRunComplete(data string) error
	OnError(data string) error
}

// PrettyRenderer renders events in a human-readable format
type PrettyRenderer struct {
	writer    io.Writer
	useColors bool
	buffer    strings.Builder
}

// NewPrettyRenderer creates a new pretty renderer
func NewPrettyRenderer(w io.Writer, useColors bool) *PrettyRenderer {
	return &PrettyRenderer{
		writer:    w,
		useColors: useColors,
	}
}

func (r *PrettyRenderer) OnTextMessageStart(data string) error {
	r.buffer.Reset()
	if !quiet {
		fmt.Fprint(r.writer, "\n🤖 Assistant: ")
	}
	return nil
}

func (r *PrettyRenderer) OnTextMessageContent(data string) error {
	var content map[string]interface{}
	if err := json.Unmarshal([]byte(data), &content); err != nil {
		return err
	}

	if delta, ok := content["delta"].(string); ok {
		r.buffer.WriteString(delta)
		fmt.Fprint(r.writer, delta)
	}
	return nil
}

func (r *PrettyRenderer) OnTextMessageEnd(data string) error {
	fmt.Fprintln(r.writer) // New line after message
	return nil
}

func (r *PrettyRenderer) OnToolCallRequested(data string) error {
	if !quiet {
		var toolCall map[string]interface{}
		if err := json.Unmarshal([]byte(data), &toolCall); err != nil {
			return err
		}

		if function, ok := toolCall["function"].(map[string]interface{}); ok {
			name := function["name"]
			fmt.Fprintf(r.writer, "\n🔧 Tool Call: %s\n", name)
			if verbose {
				fmt.Fprintf(r.writer, "   Arguments: %s\n", function["arguments"])
			}
		}
	}
	return nil
}

func (r *PrettyRenderer) OnToolCallResult(data string) error {
	if !quiet {
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			return err
		}

		fmt.Fprintf(r.writer, "✅ Tool Result: %s\n", result["content"])
	}
	return nil
}

func (r *PrettyRenderer) OnRunComplete(data string) error {
	if !quiet && verbose {
		fmt.Fprintln(r.writer, "\n✨ Run complete")
	}
	return nil
}

func (r *PrettyRenderer) OnError(data string) error {
	fmt.Fprintf(r.writer, "\n❌ Error: %s\n", data)
	return nil
}

// JSONRenderer outputs events as line-delimited JSON
type JSONRenderer struct {
	encoder *json.Encoder
}

// NewJSONRenderer creates a new JSON renderer
func NewJSONRenderer(w io.Writer) *JSONRenderer {
	return &JSONRenderer{
		encoder: json.NewEncoder(w),
	}
}

func (r *JSONRenderer) outputEvent(eventType string, data string) error {
	var parsedData interface{}
	if err := json.Unmarshal([]byte(data), &parsedData); err != nil {
		// If parsing fails, use raw string
		parsedData = data
	}

	output := map[string]interface{}{
		"type":      eventType,
		"timestamp": fmt.Sprintf("%d", timeNow().UnixMilli()),
		"data":      parsedData,
	}

	return r.encoder.Encode(output)
}

func (r *JSONRenderer) OnTextMessageStart(data string) error {
	return r.outputEvent("text_message_start", data)
}

func (r *JSONRenderer) OnTextMessageContent(data string) error {
	return r.outputEvent("text_message_content", data)
}

func (r *JSONRenderer) OnTextMessageEnd(data string) error {
	return r.outputEvent("text_message_end", data)
}

func (r *JSONRenderer) OnToolCallRequested(data string) error {
	return r.outputEvent("tool_call_requested", data)
}

func (r *JSONRenderer) OnToolCallResult(data string) error {
	return r.outputEvent("tool_call_result", data)
}

func (r *JSONRenderer) OnRunComplete(data string) error {
	return r.outputEvent("run_complete", data)
}

func (r *JSONRenderer) OnError(data string) error {
	return r.outputEvent("error", data)
}

// Helper function for getting current time
func timeNow() time.Time {
	return time.Now()
}