package security

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CORSConfig represents CORS configuration
type CORSConfig struct {
	Enabled          bool     `json:"enabled" yaml:"enabled"`
	AllowedOrigins   []string `json:"allowed_origins" yaml:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods" yaml:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers" yaml:"allowed_headers"`
	ExposedHeaders   []string `json:"exposed_headers" yaml:"exposed_headers"`
	AllowCredentials bool     `json:"allow_credentials" yaml:"allow_credentials"`
	MaxAge           int      `json:"max_age" yaml:"max_age"`
	OptionsSuccess   int      `json:"options_success" yaml:"options_success"`
}

// CORSHandler handles CORS functionality
type CORSHandler struct {
	config *CORSConfig
}

// NewCORSHandler creates a new CORS handler
func NewCORSHandler(config *CORSConfig) *CORSHandler {
	if config == nil {
		config = &CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"*"},
			MaxAge:         86400,
			OptionsSuccess: 204,
		}
	}

	return &CORSHandler{config: config}
}

// HandlePreflight handles CORS preflight requests
func (ch *CORSHandler) HandlePreflight(ctx context.Context, req *Request) (*Response, error) {
	headers := make(map[string]string)

	// Add CORS headers
	origin := req.Headers["Origin"]
	if ch.isOriginAllowed(origin) {
		headers["Access-Control-Allow-Origin"] = origin
	}

	if len(ch.config.AllowedMethods) > 0 {
		headers["Access-Control-Allow-Methods"] = strings.Join(ch.config.AllowedMethods, ", ")
	}

	if len(ch.config.AllowedHeaders) > 0 {
		headers["Access-Control-Allow-Headers"] = strings.Join(ch.config.AllowedHeaders, ", ")
	}

	if ch.config.AllowCredentials {
		headers["Access-Control-Allow-Credentials"] = "true"
	}

	if ch.config.MaxAge > 0 {
		headers["Access-Control-Max-Age"] = fmt.Sprintf("%d", ch.config.MaxAge)
	}

	statusCode := ch.config.OptionsSuccess
	if statusCode == 0 {
		statusCode = 204
	}

	return &Response{
		ID:         req.ID,
		StatusCode: statusCode,
		Headers:    headers,
		Timestamp:  time.Now(),
	}, nil
}

// AddCORSHeaders adds CORS headers to response
func (ch *CORSHandler) AddCORSHeaders(req *Request, resp *Response) {
	if !ch.config.Enabled {
		return
	}

	if resp.Headers == nil {
		resp.Headers = make(map[string]string)
	}

	origin := req.Headers["Origin"]
	if ch.isOriginAllowed(origin) {
		resp.Headers["Access-Control-Allow-Origin"] = origin
	}

	if len(ch.config.ExposedHeaders) > 0 {
		resp.Headers["Access-Control-Expose-Headers"] = strings.Join(ch.config.ExposedHeaders, ", ")
	}

	if ch.config.AllowCredentials {
		resp.Headers["Access-Control-Allow-Credentials"] = "true"
	}
}

// isOriginAllowed checks if origin is allowed
func (ch *CORSHandler) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range ch.config.AllowedOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
		// Support wildcard subdomains (simplified)
		if strings.HasPrefix(allowed, "*.") {
			domain := strings.TrimPrefix(allowed, "*.")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}

	return false
}

// Enabled returns whether CORS is enabled
func (ch *CORSHandler) Enabled() bool {
	return ch.config.Enabled
}
