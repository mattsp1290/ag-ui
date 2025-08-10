package middleware

import (
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
)

// Recovery middleware handles panics with structured logging
func Recovery() fiber.Handler {
	return func(c fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				// Get the logger entry from context
				entry := GetLogger(c)

				// Log the panic with stack trace
				entry.WithFields(map[string]interface{}{
					"panic": r,
					"stack": string(debug.Stack()),
				}).Error("Panic recovered in HTTP handler")

				// Return 500 Internal Server Error
				err := c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error":   true,
					"message": "Internal Server Error",
				})
				if err != nil {
					entry.WithError(err).Error("Failed to send error response after panic")
				}
			}
		}()

		return c.Next()
	}
}
