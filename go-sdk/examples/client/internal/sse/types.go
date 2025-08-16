package sse

type RunAgentInput struct {
	SessionID       string                 `json:"session_id,omitempty"`
	ConversationID  string                 `json:"conversation_id,omitempty"`
	State           map[string]interface{} `json:"state,omitempty"`
	Tools           []ToolDefinition       `json:"tools,omitempty"`
	SystemPrompt    string                 `json:"system_prompt,omitempty"`
	Messages        []Message              `json:"messages,omitempty"`
	Model           string                 `json:"model,omitempty"`
	Temperature     *float64               `json:"temperature,omitempty"`
	MaxTokens       *int                   `json:"max_tokens,omitempty"`
	Stream          bool                   `json:"stream"`
}

type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type SSEEvent struct {
	Event string                 `json:"event"`
	Data  map[string]interface{} `json:"data,omitempty"`
}