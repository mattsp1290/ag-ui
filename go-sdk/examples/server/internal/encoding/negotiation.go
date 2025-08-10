package encoding

import (
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// ContentNegotiationConfig holds configuration for content negotiation middleware
type ContentNegotiationConfig struct {
	// DefaultContentType is used when no Accept header is present
	DefaultContentType string
	// SupportedTypes lists all supported content types
	SupportedTypes []string
	// EnableLogging enables debug logging for content negotiation
	EnableLogging bool
}

// DefaultContentNegotiationConfig returns sensible defaults
func DefaultContentNegotiationConfig() ContentNegotiationConfig {
	return ContentNegotiationConfig{
		DefaultContentType: "application/json",
		SupportedTypes:     []string{"application/json", "application/vnd.ag-ui+json"},
		EnableLogging:      false,
	}
}

// ContentNegotiationMiddleware creates a Fiber middleware for content negotiation
func ContentNegotiationMiddleware(config ...ContentNegotiationConfig) fiber.Handler {
	var cfg ContentNegotiationConfig
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultContentNegotiationConfig()
	}

	logger := slog.Default()

	return func(c fiber.Ctx) error {
		// Get Accept header from request
		acceptHeader := c.Get("Accept", "")

		// Perform content negotiation
		negotiatedType := negotiateContentType(acceptHeader, cfg.SupportedTypes, cfg.DefaultContentType)

		// Store the negotiated content type in context for handlers to use
		c.Locals("content_type", negotiatedType)

		// Set response content type header for SSE streams
		if isSSEEndpoint(c.Path()) {
			// For SSE, always use text/event-stream as the transport
			c.Set("Content-Type", "text/event-stream; charset=utf-8")
			// But store the negotiated payload type for event encoding
			c.Locals("event_content_type", negotiatedType)
		}

		// Log content negotiation if enabled
		if cfg.EnableLogging {
			logger.Debug("Content negotiation performed",
				"path", c.Path(),
				"accept_header", acceptHeader,
				"negotiated_type", negotiatedType,
				"request_id", c.Locals("requestid"))
		}

		return c.Next()
	}
}

// negotiateContentType performs simple content type negotiation
func negotiateContentType(acceptHeader string, supportedTypes []string, defaultType string) string {
	if acceptHeader == "" {
		return defaultType
	}

	// Parse Accept header (simplified version)
	acceptedTypes := parseAcceptHeader(acceptHeader)

	// Find the best match
	for _, acceptedType := range acceptedTypes {
		for _, supportedType := range supportedTypes {
			if matches(acceptedType, supportedType) {
				return supportedType
			}
		}
	}

	// No match found, return default
	return defaultType
}

// parseAcceptHeader parses the Accept header into a slice of content types
// This is a simplified parser - production code should handle q-values and priorities
func parseAcceptHeader(acceptHeader string) []string {
	types := strings.Split(acceptHeader, ",")
	var result []string

	for _, t := range types {
		// Remove q-value and whitespace for simple matching
		parts := strings.Split(strings.TrimSpace(t), ";")
		if len(parts) > 0 {
			cleanType := strings.TrimSpace(parts[0])
			if cleanType != "" {
				result = append(result, cleanType)
			}
		}
	}

	return result
}

// matches checks if an accepted type matches a supported type
func matches(acceptedType, supportedType string) bool {
	// Handle wildcards
	if acceptedType == "*/*" {
		return true
	}

	// Handle subtype wildcards (e.g., application/*)
	if strings.HasSuffix(acceptedType, "/*") {
		mainType := strings.TrimSuffix(acceptedType, "/*")
		return strings.HasPrefix(supportedType, mainType+"/")
	}

	// Exact match
	return acceptedType == supportedType
}

// isSSEEndpoint checks if the path is an SSE endpoint
func isSSEEndpoint(path string) bool {
	sseEndpoints := []string{
		"/examples/_internal/stream",
		"/events",
		"/stream",
	}

	for _, endpoint := range sseEndpoints {
		if strings.Contains(path, endpoint) {
			return true
		}
	}

	return false
}

// GetNegotiatedContentType retrieves the negotiated content type from Fiber context
func GetNegotiatedContentType(c fiber.Ctx) string {
	if contentType, ok := c.Locals("content_type").(string); ok {
		return contentType
	}
	return "application/json" // fallback
}

// GetEventContentType retrieves the event content type for SSE payloads from Fiber context
func GetEventContentType(c fiber.Ctx) string {
	if contentType, ok := c.Locals("event_content_type").(string); ok {
		return contentType
	}
	return "application/json" // fallback
}

// HandleUnsupportedContentType returns a 406 Not Acceptable response
func HandleUnsupportedContentType(c fiber.Ctx, acceptedType string) error {
	return c.Status(fiber.StatusNotAcceptable).JSON(fiber.Map{
		"error":           "Not Acceptable",
		"message":         "The requested content type is not supported",
		"accepted_type":   acceptedType,
		"supported_types": []string{"application/json", "application/vnd.ag-ui+json"},
	})
}
