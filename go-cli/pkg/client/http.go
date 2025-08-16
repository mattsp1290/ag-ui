package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient wraps HTTP operations for AG-UI server
type HTTPClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewHTTPClient creates a new HTTP client
func NewHTTPClient(baseURL, apiKey string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SendMessage sends a chat message to the server
func (c *HTTPClient) SendMessage(ctx context.Context, sessionID string, message Message) error {
	payload := map[string]interface{}{
		"session_id": sessionID,
		"message":    message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", 
		fmt.Sprintf("%s/chat", c.baseURL), 
		bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// CreateSession creates a new session
func (c *HTTPClient) CreateSession(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", 
		fmt.Sprintf("%s/sessions", c.baseURL), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.SessionID, nil
}

// CloseSession closes a session
func (c *HTTPClient) CloseSession(ctx context.Context, sessionID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", 
		fmt.Sprintf("%s/sessions/%s", c.baseURL, sessionID), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListSessions lists all sessions
func (c *HTTPClient) ListSessions(ctx context.Context) ([]Session, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", 
		fmt.Sprintf("%s/sessions", c.baseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var sessions []Session
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return sessions, nil
}

// SubmitToolResult submits a tool call result
func (c *HTTPClient) SubmitToolResult(ctx context.Context, sessionID, threadID, runID string, result ToolResult) error {
	payload := map[string]interface{}{
		"session_id": sessionID,
		"thread_id":  threadID,
		"run_id":     runID,
		"result":     result,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", 
		fmt.Sprintf("%s/tool_result", c.baseURL), 
		bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *HTTPClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Session represents a chat session
type Session struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"`
}

// ToolResult represents a tool execution result
type ToolResult struct {
	ToolCallID string      `json:"tool_call_id"`
	Output     interface{} `json:"output"`
	Error      string      `json:"error,omitempty"`
}

// GetTools retrieves the list of available tools from the server
func (c *HTTPClient) GetTools(ctx context.Context) ([]Tool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", 
		fmt.Sprintf("%s/tools", c.baseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var tools []Tool
	if err := json.NewDecoder(resp.Body).Decode(&tools); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return tools, nil
}

// InvokeTool invokes a tool directly on the server
func (c *HTTPClient) InvokeTool(ctx context.Context, sessionID, toolName, arguments, toolCallID string) error {
	// Create RunAgentInput matching Python server expectations
	payload := map[string]interface{}{
		"thread_id": sessionID,
		"run_id":    fmt.Sprintf("run_%d", time.Now().UnixNano()),
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": fmt.Sprintf("Execute tool: %s", toolName),
			},
			{
				"role": "assistant",
				"tool_calls": []map[string]interface{}{
					{
						"id":   toolCallID,
						"type": "function",
						"function": map[string]string{
							"name":      toolName,
							"arguments": arguments,
						},
					},
				},
			},
		},
		"tools": []map[string]interface{}{}, // Will be populated by server
		"state": map[string]interface{}{},   // Empty state
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use tool_based_generative_ui endpoint for tool invocation
	req, err := http.NewRequestWithContext(ctx, "POST", 
		fmt.Sprintf("%s/tool_based_generative_ui", c.baseURL), 
		bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// For SSE endpoints, we expect 200 OK with event stream
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	// The actual events will be consumed via SSE client
	return nil
}

// Tool represents a tool definition from the server
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  *ToolParameters        `json:"parameters"`
}

// ToolParameters represents tool parameter schema
type ToolParameters struct {
	Type       string                    `json:"type"`
	Properties map[string]*ToolProperty  `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
}

// ToolProperty represents a single tool parameter
type ToolProperty struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}