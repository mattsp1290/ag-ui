package security

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"
)

// InputValidationConfig represents input validation configuration
type InputValidationConfig struct {
	Enabled             bool     `json:"enabled" yaml:"enabled"`
	MaxRequestSize      int64    `json:"max_request_size" yaml:"max_request_size"`
	MaxHeaderSize       int64    `json:"max_header_size" yaml:"max_header_size"`
	MaxQueryParams      int      `json:"max_query_params" yaml:"max_query_params"`
	MaxFormFields       int      `json:"max_form_fields" yaml:"max_form_fields"`
	AllowedContentTypes []string `json:"allowed_content_types" yaml:"allowed_content_types"`
	BlockedPatterns     []string `json:"blocked_patterns" yaml:"blocked_patterns"`
	SanitizeHTML        bool     `json:"sanitize_html" yaml:"sanitize_html"`
	ValidateJSON        bool     `json:"validate_json" yaml:"validate_json"`
	StripControlChars   bool     `json:"strip_control_chars" yaml:"strip_control_chars"`
	// Enhanced security options
	NormalizeUnicode    bool     `json:"normalize_unicode" yaml:"normalize_unicode"`
	ValidateEncoding    bool     `json:"validate_encoding" yaml:"validate_encoding"`
	FailSecure          bool     `json:"fail_secure" yaml:"fail_secure"`
	MaxStringLength     int      `json:"max_string_length" yaml:"max_string_length"`
}

// InputValidator handles input validation functionality
type InputValidator struct {
	config *InputValidationConfig
}

// NewInputValidator creates a new input validator
func NewInputValidator(config *InputValidationConfig) *InputValidator {
	if config == nil {
		config = &InputValidationConfig{
			Enabled:           true,
			MaxRequestSize:    10 * 1024 * 1024, // 10MB
			MaxHeaderSize:     8192,             // 8KB
			MaxQueryParams:    100,
			MaxFormFields:     100,
			SanitizeHTML:      true,
			ValidateJSON:      true,
			StripControlChars: true,
		// Enhanced security defaults
		NormalizeUnicode: true,
		ValidateEncoding: true,
		FailSecure: true,
		MaxStringLength: 10000,
		}
	}

	return &InputValidator{config: config}
}

// ValidateInput validates request input according to configuration
func (iv *InputValidator) ValidateInput(ctx context.Context, req *Request) error {
	// Fail-secure: if config is nil or disabled, still perform basic validation
	if iv.config == nil {
		return fmt.Errorf("input validation configuration is required")
	}
	
	if !iv.config.Enabled {
		if iv.config.FailSecure {
			// Even when disabled, perform minimal security checks
			return iv.performMinimalValidation(ctx, req)
		}
		return nil
	}
	
	// Check context cancellation before processing
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue processing
	}

	// Validate request size
	if iv.config.MaxRequestSize > 0 {
		// Estimate request size (simplified)
		size := int64(len(req.Method) + len(req.Path))
		for k, v := range req.Headers {
			size += int64(len(k) + len(v))
			if size > iv.config.MaxHeaderSize {
				return fmt.Errorf("header size exceeds limit")
			}
		}

		if size > iv.config.MaxRequestSize {
			return fmt.Errorf("request size exceeds limit")
		}
	}

	// Validate content type
	if len(iv.config.AllowedContentTypes) > 0 {
		contentType := req.Headers["Content-Type"]
		if contentType != "" {
			allowed := false
			for _, allowedType := range iv.config.AllowedContentTypes {
				if strings.Contains(contentType, allowedType) {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("content type not allowed: %s", contentType)
			}
		}
	}

	// Validate and sanitize request data
	if req.Body != nil {
		// First normalize input before validation to prevent encoding bypasses
		if err := iv.normalizeInput(ctx, req); err != nil {
			return fmt.Errorf("input normalization failed: %w", err)
		}
		
		if err := iv.sanitizeDataWithContext(ctx, req); err != nil {
			return fmt.Errorf("data sanitization failed: %w", err)
		}
	}

	return nil
}

// sanitizeData sanitizes request data with context awareness
func (iv *InputValidator) sanitizeData(req *Request) error {
	return iv.sanitizeDataWithContext(context.Background(), req)
}

// sanitizeDataWithContext sanitizes request data with context cancellation support
func (iv *InputValidator) sanitizeDataWithContext(ctx context.Context, req *Request) error {
	// Check context cancellation before sanitization
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	// HTML sanitization
	if iv.config.SanitizeHTML {
		req.Body = iv.sanitizeHTMLWithContext(ctx, req.Body)
	}

	// Check context again before next operation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Strip control characters
	if iv.config.StripControlChars {
		req.Body = iv.stripControlChars(req.Body)
	}

	return nil
}

// sanitizeHTML sanitizes HTML content
func (iv *InputValidator) sanitizeHTML(data interface{}) interface{} {
	return iv.sanitizeHTMLWithContext(context.Background(), data)
}

// sanitizeHTMLWithContext sanitizes HTML content with context cancellation support
func (iv *InputValidator) sanitizeHTMLWithContext(ctx context.Context, data interface{}) interface{} {
	// For large data structures, check context cancellation periodically
	switch v := data.(type) {
	case string:
		return html.EscapeString(v)
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			// Check context cancellation for large maps
			select {
			case <-ctx.Done():
				// Return partial results to prevent data corruption
				return result
			default:
			}
			result[k] = iv.sanitizeHTMLWithContext(ctx, val)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			// Check context cancellation for large arrays
			select {
			case <-ctx.Done():
				// Return partial results to prevent data corruption
				return result[:i]
			default:
			}
			result[i] = iv.sanitizeHTMLWithContext(ctx, val)
		}
		return result
	default:
		return data
	}
}

// stripControlChars removes control characters from data
func (iv *InputValidator) stripControlChars(data interface{}) interface{} {
	controlCharsRegex := regexp.MustCompile(`[\x00-\x1f\x7f-\x9f]`)

	switch v := data.(type) {
	case string:
		return controlCharsRegex.ReplaceAllString(v, "")
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			result[k] = iv.stripControlChars(val)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = iv.stripControlChars(val)
		}
		return result
	default:
		return data
	}
}

// recursiveValidation performs deep validation on complex data structures
func (iv *InputValidator) recursiveValidation(ctx context.Context, data interface{}) error {
	if data == nil {
		return nil
	}

	// Check context cancellation before processing
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	value := reflect.ValueOf(data)
	switch value.Kind() {
	case reflect.Map:
		return iv.validateMap(ctx, data)
	case reflect.Slice, reflect.Array:
		return iv.validateSlice(ctx, data)
	case reflect.String:
		return iv.validateString(data.(string))
	default:
		return nil
	}
}

// validateMap validates map data
func (iv *InputValidator) validateMap(ctx context.Context, data interface{}) error {
	if dataMap, ok := data.(map[string]interface{}); ok {
		for k, v := range dataMap {
			// Check context cancellation in long-running map validation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			
			if err := iv.validateString(k); err != nil {
				return fmt.Errorf("invalid key '%s': %w", k, err)
			}
			if err := iv.recursiveValidation(ctx, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateSlice validates slice data
func (iv *InputValidator) validateSlice(ctx context.Context, data interface{}) error {
	value := reflect.ValueOf(data)
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return nil
	}

	for i := 0; i < value.Len(); i++ {
		// Check context cancellation in long-running slice validation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		if err := iv.recursiveValidation(ctx, value.Index(i).Interface()); err != nil {
			return err
		}
	}

	return nil
}

// validateString validates string data against blocked patterns
func (iv *InputValidator) validateString(s string) error {
	// Check string length limits first
	if iv.config.MaxStringLength > 0 && len(s) > iv.config.MaxStringLength {
		return fmt.Errorf("string exceeds maximum length: %d", len(s))
	}
	
	for _, pattern := range iv.config.BlockedPatterns {
		if matched, _ := regexp.MatchString(pattern, s); matched {
			return fmt.Errorf("string contains blocked pattern: %s", pattern)
		}
	}
	return nil
}

// normalizeInput normalizes input to prevent encoding bypass attacks
func (iv *InputValidator) normalizeInput(ctx context.Context, req *Request) error {
	if !iv.config.NormalizeUnicode && !iv.config.ValidateEncoding {
		return nil
	}
	
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	// Normalize Unicode characters to prevent bypass through different encodings
	if iv.config.NormalizeUnicode {
		req.Body = iv.normalizeUnicode(req.Body)
	}
	
	// Validate UTF-8 encoding
	if iv.config.ValidateEncoding {
		if err := iv.validateEncoding(req.Body); err != nil {
			return err
		}
	}
	
	// URL decode multiple times to catch double/triple encoding
	if err := iv.detectEncodingManipulation(req); err != nil {
		return err
	}
	
	return nil
}

// normalizeUnicode normalizes Unicode characters in the data
func (iv *InputValidator) normalizeUnicode(data interface{}) interface{} {
	switch v := data.(type) {
	case string:
		// Convert to normalized form and remove dangerous characters
		return strings.Map(func(r rune) rune {
			// Convert fullwidth characters to halfwidth
			if r >= 0xFF01 && r <= 0xFF5E {
				return r - 0xFEE0
			}
			// Remove zero-width characters that could be used for bypass
			if r == 0x200B || r == 0x200C || r == 0x200D || r == 0xFEFF {
				return -1 // Remove character
			}
			return r
		}, v)
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			normalizedKey := iv.normalizeUnicode(k).(string)
			result[normalizedKey] = iv.normalizeUnicode(val)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = iv.normalizeUnicode(val)
		}
		return result
	default:
		return data
	}
}

// validateEncoding validates that data is properly UTF-8 encoded
func (iv *InputValidator) validateEncoding(data interface{}) error {
	switch v := data.(type) {
	case string:
		if !utf8.ValidString(v) {
			return fmt.Errorf("invalid UTF-8 encoding detected")
		}
		// Check for overlong sequences and other encoding attacks
		if iv.detectOverlongSequences(v) {
			return fmt.Errorf("overlong UTF-8 sequence detected")
		}
	case map[string]interface{}:
		for k, val := range v {
			if !utf8.ValidString(k) {
				return fmt.Errorf("invalid UTF-8 encoding in key: %s", k)
			}
			if err := iv.validateEncoding(val); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, val := range v {
			if err := iv.validateEncoding(val); err != nil {
				return err
			}
		}
	}
	return nil
}

// detectOverlongSequences detects UTF-8 overlong sequences
func (iv *InputValidator) detectOverlongSequences(s string) bool {
	// Look for patterns that could indicate overlong sequences
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError {
			return true
		}
		// Check if the rune could have been encoded in fewer bytes
		if size > 1 && r < 0x80 {
			return true // ASCII character encoded as multi-byte
		}
		if size > 2 && r < 0x800 {
			return true // Character that could fit in 2 bytes encoded as 3+
		}
		i += size
	}
	return false
}

// detectEncodingManipulation detects multiple levels of URL encoding
func (iv *InputValidator) detectEncodingManipulation(req *Request) error {
	// Check URL path and query parameters for multiple encoding levels
	originalPath := req.Path
	for i := 0; i < 5; i++ { // Limit to prevent infinite loops
		decoded, err := url.QueryUnescape(originalPath)
		if err != nil {
			break
		}
		if decoded == originalPath {
			break // No more decoding needed
		}
		if i > 2 { // More than 2 levels of encoding is suspicious
			return fmt.Errorf("excessive URL encoding detected")
		}
		originalPath = decoded
	}
	
	return nil
}

// performMinimalValidation performs basic security checks even when validation is disabled
func (iv *InputValidator) performMinimalValidation(ctx context.Context, req *Request) error {
	// Check for null bytes and other dangerous characters
	if strings.Contains(req.Path, "\x00") {
		return fmt.Errorf("null byte detected in path")
	}
	
	// Check for extremely large requests
	if len(req.Path) > 2000 {
		return fmt.Errorf("path too long")
	}
	
	// Check headers for dangerous content
	for k, v := range req.Headers {
		if strings.Contains(k, "\x00") || strings.Contains(v, "\x00") {
			return fmt.Errorf("null byte detected in headers")
		}
	}
	
	return nil
}

// Enabled returns whether input validation is enabled
func (iv *InputValidator) Enabled() bool {
	return iv.config != nil && iv.config.Enabled
}
