package security

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CSRFConfig represents CSRF protection configuration
type CSRFConfig struct {
	Enabled        bool     `json:"enabled" yaml:"enabled"`
	TokenHeader    string   `json:"token_header" yaml:"token_header"`
	TokenField     string   `json:"token_field" yaml:"token_field"`
	TokenLength    int      `json:"token_length" yaml:"token_length"`
	SecretKey      string   `json:"secret_key" yaml:"secret_key"`
	ExemptPaths    []string `json:"exempt_paths" yaml:"exempt_paths"`
	SafeMethods    []string `json:"safe_methods" yaml:"safe_methods"`
	ValidateOrigin bool     `json:"validate_origin" yaml:"validate_origin"`
}

// CSRFHandler handles CSRF protection functionality
type CSRFHandler struct {
	config     *CSRFConfig
	csrfTokens map[string]time.Time
	mu         sync.RWMutex
}

// NewCSRFHandler creates a new CSRF handler
func NewCSRFHandler(config *CSRFConfig) *CSRFHandler {
	if config == nil {
		config = &CSRFConfig{
			Enabled:        false, // Disabled by default
			TokenHeader:    "X-CSRF-Token",
			TokenField:     "csrf_token",
			TokenLength:    32,
			SafeMethods:    []string{"GET", "HEAD", "OPTIONS", "TRACE"},
			ValidateOrigin: true,
		}
	}

	return &CSRFHandler{
		config:     config,
		csrfTokens: make(map[string]time.Time),
	}
}

// ValidateCSRF validates CSRF tokens
func (ch *CSRFHandler) ValidateCSRF(ctx context.Context, req *Request) error {
	if !ch.config.Enabled {
		return nil
	}

	// Skip safe methods
	safeMethods := ch.config.SafeMethods
	if len(safeMethods) == 0 {
		safeMethods = []string{"GET", "HEAD", "OPTIONS", "TRACE"}
	}

	for _, method := range safeMethods {
		if req.Method == method {
			return nil
		}
	}

	// Skip exempt paths
	for _, path := range ch.config.ExemptPaths {
		if req.Path == path {
			return nil
		}
	}

	// Get CSRF token from request
	token := ""
	if ch.config.TokenHeader != "" {
		token = req.Headers[ch.config.TokenHeader]
	}

	if token == "" && ch.config.TokenField != "" {
		// Try to get from body (simplified)
		if bodyMap, ok := req.Body.(map[string]interface{}); ok {
			if tokenVal, ok := bodyMap[ch.config.TokenField].(string); ok {
				token = tokenVal
			}
		}
	}

	if token == "" {
		return fmt.Errorf("CSRF token missing")
	}

	// Validate token (simplified validation)
	ch.mu.RLock()
	tokenTime, exists := ch.csrfTokens[token]
	ch.mu.RUnlock()

	if !exists {
		return fmt.Errorf("invalid CSRF token")
	}

	// Check token expiration (1 hour default)
	if time.Since(tokenTime) > time.Hour {
		ch.mu.Lock()
		delete(ch.csrfTokens, token)
		ch.mu.Unlock()
		return fmt.Errorf("CSRF token expired")
	}

	return nil
}

// GenerateCSRFToken generates a new CSRF token
func (ch *CSRFHandler) GenerateCSRFToken() string {
	// Generate a simple token (in production, use a cryptographically secure method)
	token := fmt.Sprintf("csrf_%d_%d", time.Now().UnixNano(), time.Now().Unix())

	ch.mu.Lock()
	ch.csrfTokens[token] = time.Now()
	ch.mu.Unlock()

	return token
}

// CleanupExpiredTokens removes expired CSRF tokens
func (ch *CSRFHandler) CleanupExpiredTokens() {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	now := time.Now()
	for token, tokenTime := range ch.csrfTokens {
		if now.Sub(tokenTime) > time.Hour {
			delete(ch.csrfTokens, token)
		}
	}
}

// Enabled returns whether CSRF protection is enabled
func (ch *CSRFHandler) Enabled() bool {
	return ch.config.Enabled
}
