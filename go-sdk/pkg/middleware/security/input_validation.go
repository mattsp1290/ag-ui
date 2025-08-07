package security

import (
	"context"
	"fmt"
	"html"
	"reflect"
	"regexp"
	"strings"
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
		}
	}

	return &InputValidator{config: config}
}

// ValidateInput validates request input according to configuration
func (iv *InputValidator) ValidateInput(ctx context.Context, req *Request) error {
	if !iv.config.Enabled {
		return nil
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
		if err := iv.sanitizeData(req); err != nil {
			return fmt.Errorf("data sanitization failed: %w", err)
		}
	}

	return nil
}

// sanitizeData sanitizes request data
func (iv *InputValidator) sanitizeData(req *Request) error {
	// HTML sanitization
	if iv.config.SanitizeHTML {
		req.Body = iv.sanitizeHTML(req.Body)
	}

	// Strip control characters
	if iv.config.StripControlChars {
		req.Body = iv.stripControlChars(req.Body)
	}

	return nil
}

// sanitizeHTML sanitizes HTML content
func (iv *InputValidator) sanitizeHTML(data interface{}) interface{} {
	switch v := data.(type) {
	case string:
		return html.EscapeString(v)
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			result[k] = iv.sanitizeHTML(val)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = iv.sanitizeHTML(val)
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
func (iv *InputValidator) recursiveValidation(data interface{}) error {
	if data == nil {
		return nil
	}

	value := reflect.ValueOf(data)
	switch value.Kind() {
	case reflect.Map:
		return iv.validateMap(data)
	case reflect.Slice, reflect.Array:
		return iv.validateSlice(data)
	case reflect.String:
		return iv.validateString(data.(string))
	default:
		return nil
	}
}

// validateMap validates map data
func (iv *InputValidator) validateMap(data interface{}) error {
	if dataMap, ok := data.(map[string]interface{}); ok {
		for k, v := range dataMap {
			if err := iv.validateString(k); err != nil {
				return fmt.Errorf("invalid key '%s': %w", k, err)
			}
			if err := iv.recursiveValidation(v); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateSlice validates slice data
func (iv *InputValidator) validateSlice(data interface{}) error {
	value := reflect.ValueOf(data)
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return nil
	}

	for i := 0; i < value.Len(); i++ {
		if err := iv.recursiveValidation(value.Index(i).Interface()); err != nil {
			return err
		}
	}

	return nil
}

// validateString validates string data against blocked patterns
func (iv *InputValidator) validateString(s string) error {
	for _, pattern := range iv.config.BlockedPatterns {
		if matched, _ := regexp.MatchString(pattern, s); matched {
			return fmt.Errorf("string contains blocked pattern: %s", pattern)
		}
	}
	return nil
}

// Enabled returns whether input validation is enabled
func (iv *InputValidator) Enabled() bool {
	return iv.config.Enabled
}
