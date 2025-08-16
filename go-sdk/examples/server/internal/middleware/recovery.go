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

				// Log the panic with stack trace without using map[string]interface{}
				type panicFields struct {
					Panic any
					Stack string
				}
				pf := panicFields{Panic: r, Stack: string(debug.Stack())}
				entry.WithFields(map[string]interface{}{
					"panic": pf.Panic,
					"stack": pf.Stack,
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
