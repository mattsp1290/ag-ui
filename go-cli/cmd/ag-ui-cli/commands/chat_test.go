package commands

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserPrompt(t *testing.T) {
	tests := []struct {
		name      string
		setup     func()
		cleanup   func()
		expected  string
		expectErr bool
	}{
		{
			name: "from flag",
			setup: func() {
				prompt = "test prompt from flag"
			},
			cleanup: func() {
				prompt = ""
			},
			expected: "test prompt from flag",
		},
		{
			name: "empty prompt",
			setup: func() {
				prompt = ""
				inputFile = ""
			},
			cleanup: func() {},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer tt.cleanup()

			result, err := getUserPrompt()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestJSONRenderer(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewJSONRenderer(&buf)

	// Test text message start
	err := renderer.OnTextMessageStart(`{"messageId": "123"}`)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "text_message_start")
	assert.Contains(t, output, "messageId")

	// Test that output is valid JSON
	var result map[string]interface{}
	err = json.Unmarshal([]byte(strings.TrimSpace(output)), &result)
	require.NoError(t, err)
	assert.Equal(t, "text_message_start", result["type"])
}

func TestPrettyRenderer(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewPrettyRenderer(&buf, false)

	// Test text message content
	err := renderer.OnTextMessageContent(`{"delta": "Hello, world!"}`)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Hello, world!")
}

func TestIsInteractive(t *testing.T) {
	// This test would need to mock os.Stdin
	// For now, just verify the function exists and doesn't panic
	_ = isInteractive()
}