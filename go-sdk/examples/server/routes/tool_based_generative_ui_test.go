package routes

import (
	"testing"

	"github.com/mattsp1290/ag-ui/go-sdk/examples/server/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestToolBasedGenerativeUIHandler(t *testing.T) {
	t.Run("handler creation with valid config", func(t *testing.T) {
		cfg := &config.Config{
			Host:               "localhost",
			Port:               8090,
			LogLevel:           "info",
			EnableSSE:          true,
			CORSEnabled:        true,
			CORSAllowedOrigins: []string{"*"},
		}

		handler := ToolBasedGenerativeUIHandler(cfg)
		assert.NotNil(t, handler, "Handler should not be nil")
	})

	t.Run("handler creation with nil config", func(t *testing.T) {
		// Should not panic with nil config
		assert.NotPanics(t, func() {
			handler := ToolBasedGenerativeUIHandler(nil)
			assert.NotNil(t, handler)
		})
	})

	t.Run("handler creation with empty config", func(t *testing.T) {
		cfg := &config.Config{}
		handler := ToolBasedGenerativeUIHandler(cfg)
		assert.NotNil(t, handler)
	})
}

func TestToolBasedGenerativeUIImplementation(t *testing.T) {
	t.Run("route function exists and is callable", func(t *testing.T) {
		cfg := &config.Config{EnableSSE: true}

		// This should not panic and should return a valid function
		var handler interface{}
		assert.NotPanics(t, func() {
			handler = ToolBasedGenerativeUIHandler(cfg)
		})
		assert.NotNil(t, handler)
	})
}
