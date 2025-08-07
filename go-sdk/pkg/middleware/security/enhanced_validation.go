package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// EnhancedInputValidator provides comprehensive input validation with security focus
type EnhancedInputValidator struct {
	config    *EnhancedValidationConfig
	patterns  map[string]*regexp.Regexp
	whitelist map[string]bool
}

// EnhancedValidationConfig extends basic input validation with advanced security features
type EnhancedValidationConfig struct {
	// Basic validation settings
	Enabled             bool     `json:"enabled" yaml:"enabled"`
	MaxRequestSize      int64    `json:"max_request_size" yaml:"max_request_size"`
	MaxHeaderSize       int64    `json:"max_header_size" yaml:"max_header_size"`
	MaxQueryParams      int      `json:"max_query_params" yaml:"max_query_params"`
	MaxFormFields       int      `json:"max_form_fields" yaml:"max_form_fields"`
	AllowedContentTypes []string `json:"allowed_content_types" yaml:"allowed_content_types"`

	// Enhanced security patterns
	SQLInjectionPatterns     []string `json:"sql_injection_patterns" yaml:"sql_injection_patterns"`
	XSSPatterns              []string `json:"xss_patterns" yaml:"xss_patterns"`
	CommandInjectionPatterns []string `json:"command_injection_patterns" yaml:"command_injection_patterns"`
	PathTraversalPatterns    []string `json:"path_traversal_patterns" yaml:"path_traversal_patterns"`

	// URL and IP validation
	AllowedDomains   []string `json:"allowed_domains" yaml:"allowed_domains"`
	BlockedDomains   []string `json:"blocked_domains" yaml:"blocked_domains"`
	AllowPrivateIPs  bool     `json:"allow_private_ips" yaml:"allow_private_ips"`
	AllowLoopbackIPs bool     `json:"allow_loopback_ips" yaml:"allow_loopback_ips"`

	// Content validation
	MaxStringLength int `json:"max_string_length" yaml:"max_string_length"`
	MaxArrayLength  int `json:"max_array_length" yaml:"max_array_length"`
	MaxObjectDepth  int `json:"max_object_depth" yaml:"max_object_depth"`

	// Character set restrictions
	AllowedCharsets []string `json:"allowed_charsets" yaml:"allowed_charsets"`
	DisallowedChars []string `json:"disallowed_chars" yaml:"disallowed_chars"`
	RequireUTF8     bool     `json:"require_utf8" yaml:"require_utf8"`

	// File upload validation
	MaxFileSize      int64    `json:"max_file_size" yaml:"max_file_size"`
	AllowedFileTypes []string `json:"allowed_file_types" yaml:"allowed_file_types"`
	BlockedFileTypes []string `json:"blocked_file_types" yaml:"blocked_file_types"`

	// Custom validation rules
	CustomPatterns    map[string]string `json:"custom_patterns" yaml:"custom_patterns"`
	WhitelistPatterns []string          `json:"whitelist_patterns" yaml:"whitelist_patterns"`

	// Behavior settings
	StrictMode       bool `json:"strict_mode" yaml:"strict_mode"`
	LogViolations    bool `json:"log_violations" yaml:"log_violations"`
	BlockOnViolation bool `json:"block_on_violation" yaml:"block_on_violation"`
}

// NewEnhancedInputValidator creates a new enhanced input validator
func NewEnhancedInputValidator(config *EnhancedValidationConfig) (*EnhancedInputValidator, error) {
	if config == nil {
		config = getDefaultEnhancedValidationConfig()
	}

	validator := &EnhancedInputValidator{
		config:    config,
		patterns:  make(map[string]*regexp.Regexp),
		whitelist: make(map[string]bool),
	}

	// Compile security patterns
	if err := validator.compileSecurityPatterns(); err != nil {
		return nil, fmt.Errorf("failed to compile security patterns: %w", err)
	}

	// Build whitelist
	validator.buildWhitelist()

	return validator, nil
}

// getDefaultEnhancedValidationConfig returns secure default configuration
func getDefaultEnhancedValidationConfig() *EnhancedValidationConfig {
	return &EnhancedValidationConfig{
		Enabled:             true,
		MaxRequestSize:      10 * 1024 * 1024, // 10MB
		MaxHeaderSize:       8192,             // 8KB
		MaxQueryParams:      50,
		MaxFormFields:       50,
		MaxStringLength:     10000,
		MaxArrayLength:      1000,
		MaxObjectDepth:      10,
		MaxFileSize:         50 * 1024 * 1024, // 50MB
		AllowPrivateIPs:     false,
		AllowLoopbackIPs:    true,
		RequireUTF8:         true,
		StrictMode:          true,
		LogViolations:       true,
		BlockOnViolation:    true,
		AllowedContentTypes: []string{"application/json", "application/x-www-form-urlencoded", "multipart/form-data"},

		// Enhanced security patterns
		SQLInjectionPatterns: []string{
			`(?i)(union\s+select)`,
			`(?i)(select\s+.*\s+from)`,
			`(?i)(insert\s+into)`,
			`(?i)(delete\s+from)`,
			`(?i)(update\s+.*\s+set)`,
			`(?i)(drop\s+table)`,
			`(?i)(alter\s+table)`,
			`(?i)(create\s+table)`,
			`(?i)(\-\-|\#|\/\*)`,
			`(?i)(exec\s*\(|execute\s*\()`,
			`(?i)(sp_|xp_)`,
			`(?i)('.*'.*=.*'.*')`,
			`(?i)(or\s+1\s*=\s*1)`,
			`(?i)(and\s+1\s*=\s*1)`,
			`(?i)('\s+or\s+'[^']*'\s*=\s*'[^']*')`,
			`(?i)'\s*or\s*'[^']*'\s*=\s*'[^']*`,
		},

		XSSPatterns: []string{
			`(?i)<script[^>]*>.*?</script>`,
			`(?i)<iframe[^>]*>.*?</iframe>`,
			`(?i)<object[^>]*>.*?</object>`,
			`(?i)<embed[^>]*>`,
			`(?i)<applet[^>]*>.*?</applet>`,
			`(?i)javascript:`,
			`(?i)vbscript:`,
			`(?i)data:text/html`,
			`(?i)on\w+\s*=`,
			`(?i)<svg[^>]*on\w+`,
			`(?i)expression\s*\(`,
			`(?i)<meta[^>]*http-equiv`,
		},

		CommandInjectionPatterns: []string{
			`(?i)(\||\&|\;|\$\(|<|>)`,
			`(?i)(rm\s+|del\s+|delete\s+)`,
			`(?i)(cat\s+|type\s+)`,
			`(?i)(wget\s+|curl\s+)`,
			`(?i)(nc\s+|netcat\s+)`,
			`(?i)(sh\s+|bash\s+|cmd\s+|powershell\s+)`,
			`(?i)(eval\s*\(|exec\s*\()`,
			`(?i)(\$\{.*\})`,
		},

		PathTraversalPatterns: []string{
			`\.\.\/`,
			`\.\.\\`,
			`%2e%2e%2f`,
			`%2e%2e%5c`,
			`%252e%252e%252f`,
			`%c0%ae%c0%ae%c0%af`,
			`(?i)\/etc\/passwd`,
			`(?i)\/windows\/system32`,
			`(?i)\.\.%2f`,
			`(?i)\.\.%5c`,
		},

		BlockedFileTypes: []string{
			".exe", ".bat", ".cmd", ".com", ".scr", ".pif", ".vbs", ".js", ".jar", ".sh", ".php", ".asp", ".aspx", ".jsp",
		},

		AllowedFileTypes: []string{
			".txt", ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".png", ".jpg", ".jpeg", ".gif", ".csv",
		},

		DisallowedChars: []string{
			"\x00", "\x01", "\x02", "\x03", "\x04", "\x05", "\x06", "\x07", "\x08", "\x0b", "\x0c", "\x0e", "\x0f",
			"\x10", "\x11", "\x12", "\x13", "\x14", "\x15", "\x16", "\x17", "\x18", "\x19", "\x1a", "\x1b", "\x1c", "\x1d", "\x1e", "\x1f",
		},
	}
}

// ValidateRequest performs comprehensive request validation
func (v *EnhancedInputValidator) ValidateRequest(ctx context.Context, req *Request) error {
	if !v.config.Enabled {
		return nil
	}

	// Basic size validation
	if err := v.validateRequestSize(req); err != nil {
		return v.handleViolation("request_size", err)
	}

	// Header validation
	if err := v.validateHeaders(req.Headers); err != nil {
		return v.handleViolation("headers", err)
	}

	// Content type validation
	if err := v.validateContentType(req.Headers); err != nil {
		return v.handleViolation("content_type", err)
	}

	// Path validation
	if err := v.validatePath(req.Path); err != nil {
		return v.handleViolation("path", err)
	}

	// Body validation
	if req.Body != nil {
		if err := v.validateRequestBody(req.Body); err != nil {
			return v.handleViolation("body", err)
		}
	}

	return nil
}

// validateRequestSize validates request size limits
func (v *EnhancedInputValidator) validateRequestSize(req *Request) error {
	size := int64(len(req.Method) + len(req.Path))

	for k, val := range req.Headers {
		size += int64(len(k) + len(val))
		if size > v.config.MaxHeaderSize {
			return fmt.Errorf("header size exceeds limit: %d > %d", size, v.config.MaxHeaderSize)
		}
	}

	if size > v.config.MaxRequestSize {
		return fmt.Errorf("request size exceeds limit: %d > %d", size, v.config.MaxRequestSize)
	}

	return nil
}

// validateHeaders validates request headers
func (v *EnhancedInputValidator) validateHeaders(headers map[string]string) error {
	headerCount := 0

	for name, value := range headers {
		headerCount++

		// Check header count limit
		if headerCount > v.config.MaxQueryParams { // Reuse this limit for headers
			return fmt.Errorf("too many headers: %d", headerCount)
		}

		// Validate header name
		if err := v.validateHeaderName(name); err != nil {
			return fmt.Errorf("invalid header name '%s': %w", name, err)
		}

		// Validate header value
		if err := v.validateString(value); err != nil {
			return fmt.Errorf("invalid header value for '%s': %w", name, err)
		}

		// Check for suspicious headers
		if err := v.checkSuspiciousHeader(name, value); err != nil {
			return fmt.Errorf("suspicious header '%s': %w", name, err)
		}
	}

	return nil
}

// validateHeaderName validates HTTP header names
func (v *EnhancedInputValidator) validateHeaderName(name string) error {
	if name == "" {
		return fmt.Errorf("header name cannot be empty")
	}

	// HTTP header names should only contain token characters
	for _, char := range name {
		if !isTokenChar(char) {
			return fmt.Errorf("invalid character in header name: %c", char)
		}
	}

	return nil
}

// isTokenChar checks if a character is valid in an HTTP token
func isTokenChar(r rune) bool {
	return r > 32 && r < 127 && !strings.ContainsRune("()<>@,;:\\\"/[]?={} \t", r)
}

// checkSuspiciousHeader checks for suspicious header patterns
func (v *EnhancedInputValidator) checkSuspiciousHeader(name, value string) error {
	suspiciousHeaders := map[string]bool{
		"x-forwarded-for": true,
		"x-real-ip":       true,
		"x-remote-addr":   true,
	}

	lowerName := strings.ToLower(name)
	if suspiciousHeaders[lowerName] && v.config.StrictMode {
		// Validate IP addresses in these headers
		if err := v.validateIPAddress(value); err != nil {
			return fmt.Errorf("invalid IP address: %w", err)
		}
	}

	return nil
}

// validateContentType validates the Content-Type header
func (v *EnhancedInputValidator) validateContentType(headers map[string]string) error {
	if len(v.config.AllowedContentTypes) == 0 {
		return nil
	}

	contentType := headers["Content-Type"]
	if contentType == "" {
		return nil // No content type is OK for GET requests
	}

	// Extract main content type (ignore charset and other parameters)
	mainType := strings.Split(contentType, ";")[0]
	mainType = strings.TrimSpace(strings.ToLower(mainType))

	for _, allowed := range v.config.AllowedContentTypes {
		if strings.ToLower(allowed) == mainType {
			return nil
		}
	}

	return fmt.Errorf("content type not allowed: %s", mainType)
}

// validatePath validates request path for security issues
func (v *EnhancedInputValidator) validatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Check for path traversal
	if err := v.checkPathTraversal(path); err != nil {
		return err
	}

	// URL decode and validate
	decoded, err := url.QueryUnescape(path)
	if err != nil {
		return fmt.Errorf("invalid URL encoding in path: %w", err)
	}

	// Check decoded path for traversal attempts
	if err := v.checkPathTraversal(decoded); err != nil {
		return err
	}

	// Check for null bytes and control characters
	if err := v.validateString(path); err != nil {
		return fmt.Errorf("invalid characters in path: %w", err)
	}

	return nil
}

// checkPathTraversal checks for path traversal patterns
func (v *EnhancedInputValidator) checkPathTraversal(path string) error {
	for _, pattern := range v.config.PathTraversalPatterns {
		if matched, _ := regexp.MatchString(pattern, path); matched {
			return fmt.Errorf("path traversal attempt detected")
		}
	}
	return nil
}

// validateRequestBody validates request body content
func (v *EnhancedInputValidator) validateRequestBody(body interface{}) error {
	return v.validateValue(body, 0)
}

// validateValue recursively validates values with depth tracking
func (ev *EnhancedInputValidator) validateValue(value interface{}, depth int) error {
	if depth > ev.config.MaxObjectDepth {
		return fmt.Errorf("object depth exceeds limit: %d", depth)
	}

	switch v := value.(type) {
	case string:
		return ev.validateString(v)
	case map[string]interface{}:
		return ev.validateObject(v, depth+1)
	case []interface{}:
		return ev.validateArray(v, depth+1)
	case json.Number:
		return ev.validateNumber(string(v))
	case float64, int, int64, bool:
		return nil // Basic types are OK
	default:
		return fmt.Errorf("unsupported value type: %T", value)
	}
}

// validateString validates string content
func (v *EnhancedInputValidator) validateString(s string) error {
	// Length check
	if len(s) > v.config.MaxStringLength {
		return fmt.Errorf("string too long: %d > %d", len(s), v.config.MaxStringLength)
	}

	// UTF-8 validation
	if v.config.RequireUTF8 && !isValidUTF8(s) {
		return fmt.Errorf("invalid UTF-8 encoding")
	}

	// Check for disallowed characters
	for _, char := range v.config.DisallowedChars {
		if strings.Contains(s, char) {
			return fmt.Errorf("contains disallowed character")
		}
	}

	// Security pattern checks
	if err := v.checkSecurityPatterns(s); err != nil {
		return err
	}

	return nil
}

// validateObject validates object/map content
func (ev *EnhancedInputValidator) validateObject(obj map[string]interface{}, depth int) error {
	if len(obj) > ev.config.MaxFormFields {
		return fmt.Errorf("too many object fields: %d > %d", len(obj), ev.config.MaxFormFields)
	}

	for key, val := range obj {
		// Validate key
		if err := ev.validateString(key); err != nil {
			return fmt.Errorf("invalid object key '%s': %w", key, err)
		}

		// Validate value
		if err := ev.validateValue(val, depth); err != nil {
			return fmt.Errorf("invalid value for key '%s': %w", key, err)
		}
	}

	return nil
}

// validateArray validates array content
func (ev *EnhancedInputValidator) validateArray(arr []interface{}, depth int) error {
	if len(arr) > ev.config.MaxArrayLength {
		return fmt.Errorf("array too long: %d > %d", len(arr), ev.config.MaxArrayLength)
	}

	for i, val := range arr {
		if err := ev.validateValue(val, depth); err != nil {
			return fmt.Errorf("invalid array element at index %d: %w", i, err)
		}
	}

	return nil
}

// validateNumber validates numeric strings
func (ev *EnhancedInputValidator) validateNumber(s string) error {
	if len(s) > 50 { // Prevent extremely long numbers
		return fmt.Errorf("number string too long")
	}

	// Try parsing as float
	if _, err := strconv.ParseFloat(s, 64); err != nil {
		return fmt.Errorf("invalid number format: %w", err)
	}

	return nil
}

// validateIPAddress validates IP address format
func (v *EnhancedInputValidator) validateIPAddress(ipStr string) error {
	// Handle comma-separated IPs (common in X-Forwarded-For)
	ips := strings.Split(ipStr, ",")

	for _, ipPart := range ips {
		ip := strings.TrimSpace(ipPart)
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			return fmt.Errorf("invalid IP address: %s", ip)
		}

		// Check for private/loopback restrictions
		if !v.config.AllowPrivateIPs && isPrivateIP(parsedIP) {
			return fmt.Errorf("private IP addresses not allowed: %s", ip)
		}

		if !v.config.AllowLoopbackIPs && parsedIP.IsLoopback() {
			return fmt.Errorf("loopback IP addresses not allowed: %s", ip)
		}
	}

	return nil
}

// checkSecurityPatterns checks string against security patterns
func (v *EnhancedInputValidator) checkSecurityPatterns(s string) error {
	// Check SQL injection patterns
	for _, pattern := range v.config.SQLInjectionPatterns {
		if matched, _ := regexp.MatchString(pattern, s); matched {
			return fmt.Errorf("SQL injection pattern detected")
		}
	}

	// Check XSS patterns
	for _, pattern := range v.config.XSSPatterns {
		if matched, _ := regexp.MatchString(pattern, s); matched {
			return fmt.Errorf("XSS pattern detected")
		}
	}

	// Check command injection patterns
	for _, pattern := range v.config.CommandInjectionPatterns {
		if matched, _ := regexp.MatchString(pattern, s); matched {
			return fmt.Errorf("command injection pattern detected")
		}
	}

	return nil
}

// compileSecurityPatterns compiles regular expressions for security patterns
func (v *EnhancedInputValidator) compileSecurityPatterns() error {
	allPatterns := make(map[string][]string)
	allPatterns["sql"] = v.config.SQLInjectionPatterns
	allPatterns["xss"] = v.config.XSSPatterns
	allPatterns["cmd"] = v.config.CommandInjectionPatterns
	allPatterns["path"] = v.config.PathTraversalPatterns

	for category, patterns := range allPatterns {
		for i, pattern := range patterns {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				return fmt.Errorf("failed to compile %s pattern %d: %w", category, i, err)
			}
			v.patterns[fmt.Sprintf("%s_%d", category, i)] = compiled
		}
	}

	return nil
}

// buildWhitelist builds whitelist for allowed patterns
func (v *EnhancedInputValidator) buildWhitelist() {
	for _, pattern := range v.config.WhitelistPatterns {
		v.whitelist[pattern] = true
	}
}

// handleViolation handles validation violations based on configuration
func (v *EnhancedInputValidator) handleViolation(violationType string, err error) error {
	if v.config.LogViolations {
		fmt.Printf("SECURITY VIOLATION [%s]: %v\n", violationType, err)
	}

	if v.config.BlockOnViolation {
		return err
	}

	// In non-blocking mode, just log and continue
	return nil
}

// isValidUTF8 checks if a string is valid UTF-8
func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == unicode.ReplacementChar {
			return false
		}
	}
	return true
}

// isPrivateIP checks if an IP is in a private range
func isPrivateIP(ip net.IP) bool {
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16", // Link-local
		"fc00::/7",       // IPv6 unique local
	}

	for _, rangeStr := range privateRanges {
		_, network, _ := net.ParseCIDR(rangeStr)
		if network != nil && network.Contains(ip) {
			return true
		}
	}

	return false
}

// Enabled returns whether enhanced validation is enabled
func (v *EnhancedInputValidator) Enabled() bool {
	return v.config.Enabled
}
