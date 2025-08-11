package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"
)

type accessLogFields struct {
	Status       int
	DurationMs   float64
	BytesWritten int
}

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

		// Prepare structured fields without using map[string]interface{}
		fields := accessLogFields{
			Status:       c.Response().StatusCode(),
			DurationMs:   float64(duration.Nanoseconds()) / 1e6,
			BytesWritten: len(c.Response().Body()),
		}
		entry.WithFields(map[string]interface{}{
			"status":        fields.Status,
			"duration_ms":   fields.DurationMs,
			"bytes_written": fields.BytesWritten,
		}).Info("HTTP request completed")

		return err
	}
}
