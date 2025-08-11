package ui

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSSETranscriptChat tests a complete chat conversation SSE transcript
func TestSSETranscriptChat(t *testing.T) {
	// This simulates a complete chat session with tool calls
	transcript := []struct {
		eventType string
		data      json.RawMessage
	}{
		// Run started
		{"RUN_STARTED", json.RawMessage(`{"runId": "run-123", "sessionId": "session-456"}`)},
		
		// Assistant starts message
		{"TEXT_MESSAGE_START", json.RawMessage(`{"messageId": "msg-1", "role": "assistant"}`)},
		{"TEXT_MESSAGE_CONTENT", json.RawMessage(`{"messageId": "msg-1", "content": "I'll help you search for that information. "}`)},
		
		// Tool call for search
		{"TOOL_CALL_START", json.RawMessage(`{"toolCallId": "call-1", "toolName": "web_search"}`)},
		{"TOOL_CALL_ARGS", json.RawMessage(`{"toolCallId": "call-1", "arguments": {"query": "latest AI developments 2024"}}`)},
		{"TOOL_CALL_END", json.RawMessage(`{"toolCallId": "call-1"}`)},
		
		// Tool result
		{"TOOL_CALL_RESULT", json.RawMessage(`{
			"toolCallId": "call-1",
			"result": {
				"results": [
					{"title": "GPT-5 Announced", "url": "https://example.com/1"},
					{"title": "New Vision Models", "url": "https://example.com/2"}
				]
			}
		}`)},
		
		// Assistant continues
		{"TEXT_MESSAGE_CONTENT", json.RawMessage(`{"messageId": "msg-1", "content": "Based on my search, here are the latest AI developments:\n\n"}`)},
		{"TEXT_MESSAGE_CONTENT", json.RawMessage(`{"messageId": "msg-1", "content": "1. GPT-5 has been announced with improved reasoning capabilities\n"}`)},
		{"TEXT_MESSAGE_CONTENT", json.RawMessage(`{"messageId": "msg-1", "content": "2. New vision models show significant improvements in understanding\n"}`)},
		{"TEXT_MESSAGE_END", json.RawMessage(`{"messageId": "msg-1"}`)},
		
		// Run finished
		{"RUN_FINISHED", json.RawMessage(`{"runId": "run-123"}`)},
	}

	// Test with pretty output
	var prettyBuf bytes.Buffer
	prettyRenderer := NewRenderer(RendererConfig{
		OutputMode: OutputModePretty,
		NoColor:    true,
		Writer:     &prettyBuf,
	})

	for _, event := range transcript {
		err := prettyRenderer.HandleEvent(event.eventType, event.data)
		require.NoError(t, err)
	}

	prettyOutput := prettyBuf.String()
	assert.Contains(t, prettyOutput, "assistant")
	assert.Contains(t, prettyOutput, "Tool Call: web_search")
	assert.Contains(t, prettyOutput, "Tool Result:")
	assert.Contains(t, prettyOutput, "GPT-5")
	assert.Contains(t, prettyOutput, "vision models")

	// Verify message accumulation
	msg, exists := prettyRenderer.GetMessage("msg-1")
	assert.True(t, exists)
	assert.True(t, msg.IsComplete)
	expectedContent := "I'll help you search for that information. Based on my search, here are the latest AI developments:\n\n1. GPT-5 has been announced with improved reasoning capabilities\n2. New vision models show significant improvements in understanding\n"
	assert.Equal(t, expectedContent, msg.Content.String())

	// Test with JSON output
	var jsonBuf bytes.Buffer
	jsonRenderer := NewRenderer(RendererConfig{
		OutputMode: OutputModeJSON,
		Writer:     &jsonBuf,
	})

	eventCount := 0
	for _, event := range transcript {
		// Skip non-UI events for JSON test
		if event.eventType == "RUN_STARTED" || event.eventType == "RUN_FINISHED" {
			continue
		}
		err := jsonRenderer.HandleEvent(event.eventType, event.data)
		require.NoError(t, err)
		eventCount++
	}

	// Verify JSON output
	lines := strings.Split(strings.TrimSpace(jsonBuf.String()), "\n")
	assert.Equal(t, eventCount, len(lines))

	for _, line := range lines {
		var jsonEvent map[string]interface{}
		err := json.Unmarshal([]byte(line), &jsonEvent)
		require.NoError(t, err)
		assert.Contains(t, jsonEvent, "event")
		assert.Contains(t, jsonEvent, "data")
	}
}

// TestSSETranscriptWithState tests SSE transcript with state management
func TestSSETranscriptWithState(t *testing.T) {
	transcript := []struct {
		eventType string
		data      json.RawMessage
	}{
		// Initial state snapshot
		{"STATE_SNAPSHOT", json.RawMessage(`{
			"conversation": {
				"id": "conv-123",
				"messageCount": 0,
				"participants": ["user", "assistant"]
			},
			"settings": {
				"theme": "light",
				"language": "en"
			}
		}`)},
		
		// User message (through MESSAGES_SNAPSHOT)
		{"MESSAGES_SNAPSHOT", json.RawMessage(`{
			"messages": [
				{"role": "user", "content": "Hello, can you help me?"}
			]
		}`)},
		
		// Assistant response
		{"TEXT_MESSAGE_START", json.RawMessage(`{"messageId": "msg-1", "role": "assistant"}`)},
		{"TEXT_MESSAGE_CONTENT", json.RawMessage(`{"messageId": "msg-1", "content": "Hello! I'd be happy to help you. "}`)},
		{"TEXT_MESSAGE_CONTENT", json.RawMessage(`{"messageId": "msg-1", "content": "What can I assist you with today?"}`)},
		{"TEXT_MESSAGE_END", json.RawMessage(`{"messageId": "msg-1"}`)},
		
		// State delta update
		{"STATE_DELTA", json.RawMessage(`[
			{"op": "replace", "path": "/conversation/messageCount", "value": 2},
			{"op": "add", "path": "/conversation/lastActivity", "value": "2024-01-10T10:30:00Z"}
		]`)},
		
		// Another state delta
		{"STATE_DELTA", json.RawMessage(`[
			{"op": "replace", "path": "/settings/theme", "value": "dark"}
		]`)},
	}

	var buf bytes.Buffer
	renderer := NewRenderer(RendererConfig{
		OutputMode: OutputModePretty,
		NoColor:    true,
		Writer:     &buf,
	})

	for _, event := range transcript {
		err := renderer.HandleEvent(event.eventType, event.data)
		require.NoError(t, err)
	}

	// Verify state updates
	state := renderer.GetState()
	
	// Check conversation state
	conversation, ok := state["conversation"].(map[string]interface{})
	if ok {
		assert.Equal(t, float64(2), conversation["messageCount"])
		assert.NotNil(t, conversation["lastActivity"])
	}
	
	// Check settings state
	settings, ok := state["settings"].(map[string]interface{})
	if ok {
		assert.Equal(t, "dark", settings["theme"])
		assert.Equal(t, "en", settings["language"])
	}

	output := buf.String()
	assert.Contains(t, output, "State Updated")
	assert.Contains(t, output, "State Changes")
}

// TestSSETranscriptWithThinking tests SSE transcript with thinking/reasoning events
func TestSSETranscriptWithThinking(t *testing.T) {
	transcript := []struct {
		eventType string
		data      json.RawMessage
	}{
		// Thinking phase
		{"THINKING_START", json.RawMessage(`{}`)},
		{"THINKING_TEXT_MESSAGE_CONTENT", json.RawMessage(`{"content": "Analyzing the user's request..."}`)},
		{"THINKING_TEXT_MESSAGE_CONTENT", json.RawMessage(`{"content": " This seems to be about mathematics."}`)},
		{"THINKING_END", json.RawMessage(`{}`)},
		
		// Regular response
		{"TEXT_MESSAGE_START", json.RawMessage(`{"messageId": "msg-1", "role": "assistant"}`)},
		{"TEXT_MESSAGE_CONTENT", json.RawMessage(`{"messageId": "msg-1", "content": "The answer to 2+2 is 4."}`)},
		{"TEXT_MESSAGE_END", json.RawMessage(`{"messageId": "msg-1"}`)},
	}

	var buf bytes.Buffer
	renderer := NewRenderer(RendererConfig{
		OutputMode: OutputModePretty,
		NoColor:    true,
		Writer:     &buf,
	})

	for _, event := range transcript {
		err := renderer.HandleEvent(event.eventType, event.data)
		require.NoError(t, err)
	}

	output := buf.String()
	assert.Contains(t, output, "Thinking...")
	assert.Contains(t, output, "Analyzing the user's request")
	assert.Contains(t, output, "This seems to be about mathematics")
	assert.Contains(t, output, "The answer to 2+2 is 4")
}

// TestSSETranscriptMultipleTools tests multiple tool calls in sequence
func TestSSETranscriptMultipleTools(t *testing.T) {
	transcript := []struct {
		eventType string
		data      json.RawMessage
	}{
		{"TEXT_MESSAGE_START", json.RawMessage(`{"messageId": "msg-1", "role": "assistant"}`)},
		{"TEXT_MESSAGE_CONTENT", json.RawMessage(`{"messageId": "msg-1", "content": "Let me gather some information for you.\n\n"}`)},
		
		// First tool call
		{"TOOL_CALL_START", json.RawMessage(`{"toolCallId": "call-1", "toolName": "get_weather"}`)},
		{"TOOL_CALL_ARGS", json.RawMessage(`{"toolCallId": "call-1", "arguments": {"location": "New York"}}`)},
		{"TOOL_CALL_END", json.RawMessage(`{"toolCallId": "call-1"}`)},
		{"TOOL_CALL_RESULT", json.RawMessage(`{
			"toolCallId": "call-1",
			"result": {"temperature": 72, "conditions": "sunny"}
		}`)},
		
		// Second tool call
		{"TOOL_CALL_START", json.RawMessage(`{"toolCallId": "call-2", "toolName": "get_news"}`)},
		{"TOOL_CALL_ARGS", json.RawMessage(`{"toolCallId": "call-2", "arguments": {"topic": "technology"}}`)},
		{"TOOL_CALL_END", json.RawMessage(`{"toolCallId": "call-2"}`)},
		{"TOOL_CALL_RESULT", json.RawMessage(`{
			"toolCallId": "call-2",
			"result": {"headlines": ["AI breakthrough", "New smartphone released"]}
		}`)},
		
		// Continue message
		{"TEXT_MESSAGE_CONTENT", json.RawMessage(`{"messageId": "msg-1", "content": "Here's what I found:\n\n"}`)},
		{"TEXT_MESSAGE_CONTENT", json.RawMessage(`{"messageId": "msg-1", "content": "Weather: 72°F and sunny in New York\n"}`)},
		{"TEXT_MESSAGE_CONTENT", json.RawMessage(`{"messageId": "msg-1", "content": "Tech News: AI breakthrough, New smartphone released"}`)},
		{"TEXT_MESSAGE_END", json.RawMessage(`{"messageId": "msg-1"}`)},
	}

	var buf bytes.Buffer
	renderer := NewRenderer(RendererConfig{
		OutputMode: OutputModePretty,
		NoColor:    true,
		Writer:     &buf,
	})

	for _, event := range transcript {
		err := renderer.HandleEvent(event.eventType, event.data)
		require.NoError(t, err)
	}

	output := buf.String()
	
	// Verify both tool calls appear
	assert.Contains(t, output, "Tool Call: get_weather")
	assert.Contains(t, output, "Tool Call: get_news")
	assert.Contains(t, output, "location: New York")
	assert.Contains(t, output, "topic: technology")
	
	// Verify tool results
	assert.Contains(t, output, "temperature: 72")
	assert.Contains(t, output, "AI breakthrough")
	
	// Verify complete message
	msg, exists := renderer.GetMessage("msg-1")
	assert.True(t, exists)
	assert.Contains(t, msg.Content.String(), "Weather: 72°F and sunny")
	assert.Contains(t, msg.Content.String(), "Tech News:")
}

// TestSSETranscriptErrorHandling tests error scenarios
func TestSSETranscriptErrorHandling(t *testing.T) {
	transcript := []struct {
		eventType string
		data      json.RawMessage
	}{
		// Tool call with error
		{"TOOL_CALL_START", json.RawMessage(`{"toolCallId": "call-1", "toolName": "failing_tool"}`)},
		{"TOOL_CALL_ARGS", json.RawMessage(`{"toolCallId": "call-1", "arguments": {"invalid": "params"}}`)},
		{"TOOL_CALL_END", json.RawMessage(`{"toolCallId": "call-1"}`)},
		{"TOOL_CALL_RESULT", json.RawMessage(`{
			"toolCallId": "call-1",
			"error": "Invalid parameters provided"
		}`)},
		
		// Run error
		{"RUN_ERROR", json.RawMessage(`{
			"error": "Rate limit exceeded",
			"code": "RATE_LIMIT"
		}`)},
	}

	var buf bytes.Buffer
	renderer := NewRenderer(RendererConfig{
		OutputMode: OutputModePretty,
		NoColor:    true,
		Writer:     &buf,
	})

	for _, event := range transcript {
		// RUN_ERROR is not handled, but shouldn't cause panic
		_ = renderer.HandleEvent(event.eventType, event.data)
	}

	output := buf.String()
	assert.Contains(t, output, "Tool Error:")
	assert.Contains(t, output, "Invalid parameters provided")
}

// BenchmarkRenderer benchmarks the renderer performance
func BenchmarkRenderer(b *testing.B) {
	benchmarks := []struct {
		name string
		mode OutputMode
	}{
		{"PrettyMode", OutputModePretty},
		{"JSONMode", OutputModeJSON},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			var buf bytes.Buffer
			renderer := NewRenderer(RendererConfig{
				OutputMode: bm.mode,
				NoColor:    true,
				Writer:     &buf,
			})

			// Create test events
			events := []struct {
				eventType string
				data      json.RawMessage
			}{
				{"TEXT_MESSAGE_START", json.RawMessage(`{"messageId": "msg-1", "role": "assistant"}`)},
				{"TEXT_MESSAGE_CONTENT", json.RawMessage(`{"messageId": "msg-1", "content": "Test content"}`)},
				{"TEXT_MESSAGE_END", json.RawMessage(`{"messageId": "msg-1"}`)},
				{"STATE_SNAPSHOT", json.RawMessage(`{"key": "value"}`)},
				{"TOOL_CALL_START", json.RawMessage(`{"toolCallId": "call-1", "toolName": "test"}`)},
				{"TOOL_CALL_END", json.RawMessage(`{"toolCallId": "call-1"}`)},
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, event := range events {
					_ = renderer.HandleEvent(event.eventType, event.data)
				}
				renderer.Clear()
				buf.Reset()
			}
		})
	}
}