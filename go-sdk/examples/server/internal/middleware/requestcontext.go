package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
)

// RequestContextKey is used to store the logrus.Entry in the Fiber context
const RequestContextKey = "logger_entry"

// RequestContext middleware creates a per-request logrus.Entry with request-scoped fields
func RequestContext(logger *logrus.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Extract request fields
		requestID := ""
		if rid := c.Locals("requestid"); rid != nil {
			requestID = rid.(string)
		}
		method := c.Method()
		path := c.Path()
		remoteIP := c.IP()
		userAgent := c.Get("User-Agent")

		// Create request-scoped logger entry
		entry := logger.WithFields(logrus.Fields{
			"request_id": requestID,
			"method":     method,
			"path":       path,
			"remote_ip":  remoteIP,
			"user_agent": userAgent,
		})

		// Store the logger entry in the context
		c.Locals(RequestContextKey, entry)

		// Continue to next middleware
		return c.Next()
	}
}

// GetLogger retrieves the logrus.Entry from the Fiber context
func GetLogger(c fiber.Ctx) *logrus.Entry {
	if entry, ok := c.Locals(RequestContextKey).(*logrus.Entry); ok {
		return entry
	}
	// Return a basic logger if none is found (shouldn't happen with proper middleware ordering)
	return logrus.NewEntry(logrus.StandardLogger())
}
