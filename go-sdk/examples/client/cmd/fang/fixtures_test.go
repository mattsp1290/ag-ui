package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

// TestFixtures contains common test data and mock responses
var TestFixtures = struct {
	ThreadID      string
	RunID         string
	SessionID     string
	MessageID     string
	ToolCallID    string
	SampleHaiku   map[string]interface{}
	SampleTools   []Tool
	SampleState   map[string]interface{}
	SSEEvents     map[string]string
	APIResponses  map[string]interface{}
}{
	ThreadID:   "test-thread-123",
	RunID:      "test-run-456",
	SessionID:  "test-session-789",
	MessageID:  "test-msg-001",
	ToolCallID: "test-call-001",
	
	SampleHaiku: map[string]interface{}{
		"english": []interface{}{
			"Spring rain falling down",
			"Gentle drops on new green leaves",
			"Life begins again",
		},
		"japanese": []interface{}{
			"春の雨降る",
			"新緑の葉に優しく",
			"命再び",
		},
		"topic": "nature",
	},
	
	SampleTools: []Tool{
		{
			Name:         "generate_haiku",
			Description:  "Generate a haiku poem",
			Tags:         []string{"creative", "text"},
			Capabilities: []string{"async"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"topic": map[string]interface{}{
						"type":        "string",
						"description": "The topic for the haiku",
					},
				},
				"required": []string{"topic"},
			},
		},
		{
			Name:         "http_get",
			Description:  "Make HTTP GET requests",
			Tags:         []string{"network", "http"},
			Capabilities: []string{"async", "retry"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The URL to fetch",
						"format":      "uri",
					},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:         "shell_execute",
			Description:  "Execute shell commands",
			Tags:         []string{"system", "cli"},
			Capabilities: []string{"local"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The command to execute",
					},
				},
				"required": []string{"command"},
			},
		},
	},
	
	SampleState: map[string]interface{}{
		"counter":   float64(42),
		"status":    "active",
		"timestamp": "2025-01-14T12:00:00Z",
		"items": []interface{}{
			"item1",
			"item2",
			"item3",
		},
		"config": map[string]interface{}{
			"debug":   true,
			"timeout": float64(30),
		},
	},
	
	SSEEvents: map[string]string{
		"RUN_STARTED": `data: {"type":"RUN_STARTED","threadId":"test-thread-123","runId":"test-run-456","timestamp":"2025-01-14T12:00:00Z"}

`,
		"RUN_FINISHED": `data: {"type":"RUN_FINISHED","threadId":"test-thread-123","runId":"test-run-456","status":"completed"}

`,
		"TEXT_MESSAGE_START": `data: {"type":"TEXT_MESSAGE_START","role":"assistant","messageId":"test-msg-001"}

`,
		"TEXT_MESSAGE_CONTENT": `data: {"type":"TEXT_MESSAGE_CONTENT","delta":"Hello, how can I help you?","messageId":"test-msg-001"}

`,
		"TEXT_MESSAGE_END": `data: {"type":"TEXT_MESSAGE_END","messageId":"test-msg-001"}

`,
		"TOOL_CALL_START": `data: {"type":"TOOL_CALL_START","toolCallId":"test-call-001","name":"generate_haiku"}

`,
		"TOOL_CALL_CHUNK": `data: {"type":"TOOL_CALL_CHUNK","toolCallId":"test-call-001","delta":"{\"topic\":\"nature\"}"}

`,
		"TOOL_CALL_END": `data: {"type":"TOOL_CALL_END","toolCallId":"test-call-001"}

`,
		"THINKING_START": `data: {"type":"THINKING_START","messageId":"thinking-001"}

`,
		"THINKING_DELTA": `data: {"type":"THINKING_DELTA","delta":"Processing request...","messageId":"thinking-001"}

`,
		"THINKING_END": `data: {"type":"THINKING_END","messageId":"thinking-001"}

`,
		"STATE_SNAPSHOT": `data: {"type":"STATE_SNAPSHOT","snapshot":{"counter":42,"status":"active"}}

`,
		"STATE_DELTA": `data: {"type":"STATE_DELTA","delta":{"counter":43},"path":"/"}

`,
		"MESSAGES_SNAPSHOT_WITH_TOOL": `data: {"type":"MESSAGES_SNAPSHOT","messages":[{"id":"msg-1","role":"assistant","content":"I'll generate a haiku for you.","toolCalls":[{"id":"call-1","type":"function","function":{"name":"generate_haiku","arguments":"{\"topic\":\"nature\"}"}}]},{"id":"msg-2","role":"tool","content":"Spring rain falling down\\nGentle drops on new green leaves\\nLife begins again","toolCallId":"call-1"}]}

`,
		"MESSAGES_SNAPSHOT_NO_TOOL": `data: {"type":"MESSAGES_SNAPSHOT","messages":[{"id":"msg-1","role":"assistant","content":"Hello! How can I help you today?"}]}

`,
	},
	
	APIResponses: map[string]interface{}{
		"tools_list": map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{
					"name":         "generate_haiku",
					"description":  "Generate a haiku poem",
					"tags":         []string{"creative", "text"},
					"capabilities": []string{"async"},
				},
				map[string]interface{}{
					"name":         "http_get",
					"description":  "Make HTTP GET requests",
					"tags":         []string{"network", "http"},
					"capabilities": []string{"async", "retry"},
				},
			},
		},
		"tool_describe": map[string]interface{}{
			"name":         "generate_haiku",
			"description":  "Generate a haiku poem",
			"tags":         []string{"creative", "text"},
			"capabilities": []string{"async"},
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"topic": map[string]interface{}{
						"type":        "string",
						"description": "The topic for the haiku",
					},
				},
				"required": []string{"topic"},
			},
		},
	},
}

// MockSSEServer creates a mock SSE server that sends predefined events
type MockSSEServer struct {
	Events   []string
	Delay    time.Duration
	CloseAfter int // Close after N events, 0 = send all
}

// NewMockSSEServer creates a new mock SSE server
func NewMockSSEServer(events []string) *httptest.Server {
	mock := &MockSSEServer{
		Events: events,
		Delay:  10 * time.Millisecond,
	}
	return httptest.NewServer(mock)
}

// ServeHTTP handles SSE requests
func (m *MockSSEServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	
	count := 0
	for _, event := range m.Events {
		if m.CloseAfter > 0 && count >= m.CloseAfter {
			break
		}
		
		fmt.Fprint(w, event)
		flusher.Flush()
		
		if m.Delay > 0 {
			time.Sleep(m.Delay)
		}
		count++
	}
}

// MockAPIServer creates a mock API server for testing REST endpoints
type MockAPIServer struct {
	Routes map[string]func(w http.ResponseWriter, r *http.Request)
}

// NewMockAPIServer creates a new mock API server
func NewMockAPIServer() *httptest.Server {
	mock := &MockAPIServer{
		Routes: make(map[string]func(w http.ResponseWriter, r *http.Request)),
	}
	
	// Add default routes
	mock.Routes["/tools"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TestFixtures.APIResponses["tools_list"])
	}
	
	mock.Routes["/tools/generate_haiku"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TestFixtures.APIResponses["tool_describe"])
	}
	
	mock.Routes["/tool_based_generative_ui"] = func(w http.ResponseWriter, r *http.Request) {
		// Return SSE stream for chat endpoint
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		
		events := []string{
			TestFixtures.SSEEvents["RUN_STARTED"],
			TestFixtures.SSEEvents["TEXT_MESSAGE_START"],
			TestFixtures.SSEEvents["TEXT_MESSAGE_CONTENT"],
			TestFixtures.SSEEvents["TEXT_MESSAGE_END"],
			TestFixtures.SSEEvents["RUN_FINISHED"],
		}
		
		for _, event := range events {
			fmt.Fprint(w, event)
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}
	
	mock.Routes["/agentic_chat"] = func(w http.ResponseWriter, r *http.Request) {
		// Return SSE stream with tool events
		w.Header().Set("Content-Type", "text/event-stream")
		
		events := []string{
			TestFixtures.SSEEvents["RUN_STARTED"],
			TestFixtures.SSEEvents["THINKING_START"],
			TestFixtures.SSEEvents["THINKING_DELTA"],
			TestFixtures.SSEEvents["THINKING_END"],
			TestFixtures.SSEEvents["TOOL_CALL_START"],
			TestFixtures.SSEEvents["TOOL_CALL_CHUNK"],
			TestFixtures.SSEEvents["TOOL_CALL_END"],
			TestFixtures.SSEEvents["MESSAGES_SNAPSHOT_WITH_TOOL"],
			TestFixtures.SSEEvents["RUN_FINISHED"],
		}
		
		for _, event := range events {
			fmt.Fprint(w, event)
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}
	
	mock.Routes["/agentic_generative_ui"] = func(w http.ResponseWriter, r *http.Request) {
		// Return SSE stream with state events
		w.Header().Set("Content-Type", "text/event-stream")
		
		events := []string{
			TestFixtures.SSEEvents["RUN_STARTED"],
			TestFixtures.SSEEvents["STATE_SNAPSHOT"],
			TestFixtures.SSEEvents["STATE_DELTA"],
			TestFixtures.SSEEvents["RUN_FINISHED"],
		}
		
		for _, event := range events {
			fmt.Fprint(w, event)
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}
	
	return httptest.NewServer(mock)
}

// ServeHTTP handles API requests
func (m *MockAPIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler, exists := m.Routes[r.URL.Path]
	if !exists {
		http.NotFound(w, r)
		return
	}
	handler(w, r)
}

// CreateMockSSEEvent creates a formatted SSE event string
func CreateMockSSEEvent(eventType string, data map[string]interface{}) string {
	data["type"] = eventType
	jsonData, _ := json.Marshal(data)
	return fmt.Sprintf("data: %s\n\n", jsonData)
}

// CreateMockRequest creates a mock HTTP request with JSON body
func CreateMockRequest(method, path string, body interface{}) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = strings.NewReader(string(jsonBody))
	}
	
	req, err := http.NewRequest(method, path, bodyReader)
	if err != nil {
		return nil, err
	}
	
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "text/event-stream")
	
	return req, nil
}

// CreateChatRequest creates a mock chat request body
func CreateChatRequest(message string, tools []interface{}, state map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"thread_id": TestFixtures.ThreadID,
		"run_id":    TestFixtures.RunID,
		"messages": []interface{}{
			map[string]interface{}{
				"id":      "user-msg-1",
				"role":    "user",
				"content": message,
			},
		},
		"state":          state,
		"tools":          tools,
		"context":        []interface{}{},
		"forwardedProps": map[string]interface{}{},
	}
}

// CreateToolRunRequest creates a mock tool run request
func CreateToolRunRequest(toolName string, args map[string]interface{}) map[string]interface{} {
	argsJSON, _ := json.Marshal(args)
	return map[string]interface{}{
		"thread_id": TestFixtures.ThreadID,
		"run_id":    TestFixtures.RunID,
		"messages": []interface{}{
			map[string]interface{}{
				"id":   "assistant-msg-1",
				"role": "assistant",
				"content": fmt.Sprintf("Executing tool: %s", toolName),
				"toolCalls": []interface{}{
					map[string]interface{}{
						"id":   TestFixtures.ToolCallID,
						"type": "function",
						"function": map[string]interface{}{
							"name":      toolName,
							"arguments": string(argsJSON),
						},
					},
				},
			},
		},
		"state":          map[string]interface{}{},
		"tools":          []interface{}{},
		"context":        []interface{}{},
		"forwardedProps": map[string]interface{}{},
	}
}

// AssertJSONEqual asserts that two JSON strings are equivalent
func AssertJSONEqual(t interface{ Errorf(format string, args ...interface{}) }, expected, actual string) bool {
	var expectedObj, actualObj interface{}
	
	if err := json.Unmarshal([]byte(expected), &expectedObj); err != nil {
		t.Errorf("Failed to parse expected JSON: %v", err)
		return false
	}
	
	if err := json.Unmarshal([]byte(actual), &actualObj); err != nil {
		t.Errorf("Failed to parse actual JSON: %v", err)
		return false
	}
	
	expectedJSON, _ := json.Marshal(expectedObj)
	actualJSON, _ := json.Marshal(actualObj)
	
	if string(expectedJSON) != string(actualJSON) {
		t.Errorf("JSON mismatch:\nExpected: %s\nActual: %s", expectedJSON, actualJSON)
		return false
	}
	
	return true
}