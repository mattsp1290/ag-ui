package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"
)

// AccessLog middleware logs HTTP requests with performance metrics
func AccessLog() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Capture start time for duration calculation
		start := time.Now()

		// Process the request
		err := c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Get the logger entry from context
		entry := GetLogger(c)

		// Log the request with access metrics
		entry.WithFields(map[string]interface{}{
			"status":        c.Response().StatusCode(),
			"duration_ms":   float64(duration.Nanoseconds()) / 1e6, // Convert to milliseconds
			"bytes_written": len(c.Response().Body()),
		}).Info("HTTP request completed")

		return err
	}
}
