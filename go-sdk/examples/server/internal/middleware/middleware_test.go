package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestApp() (*fiber.App, *bytes.Buffer) {
	// Create logger with buffer output for testing
	logger := logrus.New()
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)
	logger.SetFormatter(&logrus.JSONFormatter{})

	app := fiber.New()
	app.Use(requestid.New())
	app.Use(RequestContext(logger))
	app.Use(Recovery())
	app.Use(AccessLog())

	return app, buf
}

func TestRequestContextMiddleware(t *testing.T) {
	app, buf := setupTestApp()

	app.Get("/test", func(c fiber.Ctx) error {
		entry := GetLogger(c)
		entry.Info("Test message")
		return c.JSON(fiber.Map{"message": "success"})
	})

	// Make test request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent/1.0")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, 200, resp.StatusCode)

	// Parse log entries
	logs := bytes.Split(buf.Bytes(), []byte("\n"))
	require.GreaterOrEqual(t, len(logs), 2) // At least handler log + access log

	var handlerLog, accessLog map[string]interface{}

	// Find the handler log (contains "Test message")
	for _, logLine := range logs {
		if len(logLine) == 0 {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal(logLine, &entry); err == nil {
			if entry["msg"] == "Test message" {
				handlerLog = entry
			}
			if entry["msg"] == "HTTP request completed" {
				accessLog = entry
			}
		}
	}

	// Verify handler log has request context
	require.NotNil(t, handlerLog)
	assert.Equal(t, "GET", handlerLog["method"])
	assert.Equal(t, "/test", handlerLog["path"])
	assert.Equal(t, "test-agent/1.0", handlerLog["user_agent"])
	assert.Contains(t, handlerLog, "request_id")
	assert.Contains(t, handlerLog, "remote_ip")

	// Verify access log has metrics
	require.NotNil(t, accessLog)
	assert.Equal(t, float64(200), accessLog["status"])
	assert.Contains(t, accessLog, "duration_ms")
	assert.Contains(t, accessLog, "bytes_written")
}

func TestAccessLogMiddleware(t *testing.T) {
	app, buf := setupTestApp()

	app.Get("/test", func(c fiber.Ctx) error {
		// Simulate some processing time
		time.Sleep(1 * time.Millisecond)
		return c.JSON(fiber.Map{"data": "test response"})
	})

	// Make test request
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Parse access log
	logs := bytes.Split(buf.Bytes(), []byte("\n"))
	var accessLog map[string]interface{}

	for _, logLine := range logs {
		if len(logLine) == 0 {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal(logLine, &entry); err == nil {
			if entry["msg"] == "HTTP request completed" {
				accessLog = entry
				break
			}
		}
	}

	require.NotNil(t, accessLog)
	assert.Equal(t, float64(200), accessLog["status"])

	// Verify duration is positive (processing took some time)
	duration, ok := accessLog["duration_ms"].(float64)
	require.True(t, ok)
	assert.Greater(t, duration, 0.0)

	// Verify bytes written is positive
	bytes, ok := accessLog["bytes_written"].(float64)
	require.True(t, ok)
	assert.Greater(t, bytes, 0.0)
}

func TestRecoveryMiddleware(t *testing.T) {
	app, buf := setupTestApp()

	app.Get("/panic", func(c fiber.Ctx) error {
		panic("test panic")
	})

	// Make test request
	req := httptest.NewRequest("GET", "/panic", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify 500 status code
	assert.Equal(t, 500, resp.StatusCode)

	// Verify response body
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)
	assert.True(t, response["error"].(bool))
	assert.Equal(t, "Internal Server Error", response["message"])

	// Parse logs to find panic log
	logs := bytes.Split(buf.Bytes(), []byte("\n"))
	var panicLog map[string]interface{}

	for _, logLine := range logs {
		if len(logLine) == 0 {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal(logLine, &entry); err == nil {
			if entry["level"] == "error" && entry["msg"] == "Panic recovered in HTTP handler" {
				panicLog = entry
				break
			}
		}
	}

	require.NotNil(t, panicLog)
	assert.Equal(t, "test panic", panicLog["panic"])
	assert.Contains(t, panicLog, "stack")
	assert.Contains(t, panicLog, "request_id")
}

func TestGetLoggerFallback(t *testing.T) {
	app := fiber.New()

	app.Get("/test", func(c fiber.Ctx) error {
		// Test GetLogger without RequestContext middleware
		entry := GetLogger(c)
		assert.NotNil(t, entry)

		// Should return a basic logger entry
		assert.Equal(t, logrus.StandardLogger(), entry.Logger)

		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
}
