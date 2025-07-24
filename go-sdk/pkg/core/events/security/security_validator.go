package security

import (
	"encoding/base64"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// SecurityValidationRule implements comprehensive security validation
type SecurityValidationRule struct {
	*events.BaseValidationRule
	
	// Configuration
	config           *SecurityConfig
	rateLimiter      *RateLimiter
	anomalyDetector  *AnomalyDetector
	encryptionValidator *EncryptionValidator
	
	// Patterns for detection
	xssPatterns          []*regexp.Regexp
	sqlPatterns          []*regexp.Regexp
	commandPatterns      []*regexp.Regexp
	pathTraversalPatterns []*regexp.Regexp
	
	// Metrics
	detectionMetrics *SecurityMetrics
	mutex            sync.RWMutex
}

// SecurityConfig defines security validation configuration
type SecurityConfig struct {
	// Input sanitization
	EnableInputSanitization bool
	MaxContentLength        int
	AllowedHTMLTags         []string
	
	// Detection settings
	EnableXSSDetection        bool
	EnableSQLInjectionDetection bool
	EnableCommandInjection    bool
	EnablePathTraversalDetection bool
	
	// Rate limiting
	EnableRateLimiting     bool
	RateLimitPerMinute     int
	RateLimitPerEventType  map[events.EventType]int
	
	// Anomaly detection
	EnableAnomalyDetection    bool
	AnomalyThreshold         float64
	AnomalyWindowSize        time.Duration
	
	// Encryption
	RequireEncryption        bool
	MinimumEncryptionBits    int
	AllowedEncryptionTypes   []string
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		EnableInputSanitization:     true,
		MaxContentLength:           1048576, // 1MB
		AllowedHTMLTags:           []string{"p", "br", "strong", "em", "code", "pre"},
		EnableXSSDetection:         true,
		EnableSQLInjectionDetection: true,
		EnableCommandInjection:     true,
		EnablePathTraversalDetection: true,
		EnableRateLimiting:         true,
		RateLimitPerMinute:        1000,
		RateLimitPerEventType:     make(map[events.EventType]int),
		EnableAnomalyDetection:    true,
		AnomalyThreshold:         3.0, // 3 standard deviations
		AnomalyWindowSize:        time.Hour,
		RequireEncryption:        false,
		MinimumEncryptionBits:    256,
		AllowedEncryptionTypes:   []string{"AES-256-GCM", "ChaCha20-Poly1305"},
	}
}

// NewSecurityValidationRule creates a new security validation rule
func NewSecurityValidationRule(config *SecurityConfig) *SecurityValidationRule {
	if config == nil {
		config = DefaultSecurityConfig()
	}
	
	rule := &SecurityValidationRule{
		BaseValidationRule: events.NewBaseValidationRule(
			"SECURITY_VALIDATION",
			"Validates security aspects including XSS, SQL injection, rate limiting, and anomaly detection",
			events.ValidationSeverityError,
		),
		config:              config,
		rateLimiter:        NewRateLimiter(config),
		anomalyDetector:    NewAnomalyDetector(config),
		encryptionValidator: NewEncryptionValidator(config),
		detectionMetrics:   NewSecurityMetrics(),
	}
	
	// Initialize detection patterns
	rule.initializePatterns()
	
	return rule
}

// initializePatterns initializes regex patterns for security detection
func (r *SecurityValidationRule) initializePatterns() {
	// XSS patterns
	r.xssPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`),
		regexp.MustCompile(`(?i)javascript:`),
		regexp.MustCompile(`(?i)on\w+\s*=`),
		regexp.MustCompile(`(?i)<iframe[^>]*>`),
		regexp.MustCompile(`(?i)<object[^>]*>`),
		regexp.MustCompile(`(?i)<embed[^>]*>`),
		regexp.MustCompile(`(?i)eval\s*\(`),
		regexp.MustCompile(`(?i)expression\s*\(`),
		regexp.MustCompile(`(?i)vbscript:`),
		regexp.MustCompile(`(?i)<img[^>]+src\s*=\s*["\']javascript:`),
	}
	
	// SQL injection patterns
	r.sqlPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(\b(union|select|insert|update|delete|drop|create|alter|exec|execute)\b.*\b(from|into|where|table|database)\b)`),
		// Updated pattern to catch ' OR '1'='1 style injections and numeric comparisons
		regexp.MustCompile(`(?i)'[^']*\s+(or|and)\s+[^']*'[^']*=|(?i)'\s+(or|and)\s+'[^']*'='`),
		// Pattern for numeric OR/AND conditions like ' OR 1=1
		regexp.MustCompile(`(?i)'\s+(or|and)\s+\d+\s*=\s*\d+`),
		// Pattern for parentheses-based injections like ') OR ('1'='1
		regexp.MustCompile(`(?i)'\)\s*(or|and)\s*\('[^']*'='`),
		// Pattern for HAVING clauses
		regexp.MustCompile(`(?i)'\s+having\s+\d+\s*=\s*\d+`),
		regexp.MustCompile(`(?i);.*?(drop|delete|truncate|update)\s+(table|database)`),
		regexp.MustCompile(`(?i)\b(waitfor\s+delay|benchmark|sleep)\b`),
		regexp.MustCompile(`(?i)(\b(sys|information_schema)\.\w+)`),
		regexp.MustCompile(`(?i)(xp_cmdshell|sp_executesql)`),
		regexp.MustCompile(`(?i)(\bunion\b.*\bselect\b|\bselect\b.*\bunion\b)`),
		// Comment-based SQL injection patterns
		regexp.MustCompile(`(?i)('|")\s*--`),
		regexp.MustCompile(`(?i)('|")\s*/\*`),
		regexp.MustCompile(`(?i)('|")\s*#`),
		regexp.MustCompile(`(?i)\badmin\s*'?\s*(--|#|/\*)`),
	}
	
	// Command injection patterns
	r.commandPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(\||;|&|&&|\|\||>|<|>>|<<)`),
		regexp.MustCompile(`(?i)(rm\s+-rf|format\s+c:|del\s+/f)`),
		regexp.MustCompile(`(?i)(\$\(|` + "`" + `)`),
		regexp.MustCompile(`(?i)(nc\s+-|\btelnet\b|\bssh\b)`),
		regexp.MustCompile(`(?i)(/etc/passwd|/etc/shadow|C:\\\\Windows\\\\System32)`),
	}
	
	// Path traversal patterns
	r.pathTraversalPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(\.\.\/|\.\.\\)`),                    // ../ or ..\
		regexp.MustCompile(`(?i)(\.\.%2[fF]|\.\.%5[cC])`),           // URL encoded ../ or ..\
		regexp.MustCompile(`(?i)(\.\.%252[fF]|\.\.%255[cC])`),       // Double URL encoded
		regexp.MustCompile(`(?i)(\.{2,}[\/\\])`),                    // Multiple dots with slash
		regexp.MustCompile(`(?i)(\.\.[\/\\]){2,}`),                  // Multiple traversals
		regexp.MustCompile(`(?i)(\.\.%c0%af|\.\.%c1%9c)`),          // Unicode encoding attempts
		regexp.MustCompile(`(?i)(\.\.%c0%ae)`),                      // Overlong UTF-8 encoding for ../
		regexp.MustCompile(`(?i)(%c0%ae)`),                          // Overlong UTF-8 encoding for /
		regexp.MustCompile(`(?i)(/\.\./|/\.\.\\|\\\.\.\\|\\\.\./)`), // Variations with leading slash
	}
}

// Validate implements the ValidationRule interface
func (r *SecurityValidationRule) Validate(event events.Event, context *events.ValidationContext) *events.ValidationResult {
	result := &events.ValidationResult{
		IsValid:   true,
		Timestamp: time.Now(),
	}
	
	if !r.IsEnabled() {
		return result
	}
	
	// Extract content from event
	content := r.extractEventContent(event)
	if content == "" {
		return result
	}
	
	// Check content length
	if len(content) > r.config.MaxContentLength {
		result.AddError(r.CreateError(event,
			fmt.Sprintf("Content exceeds maximum allowed length of %d bytes", r.config.MaxContentLength),
			map[string]interface{}{
				"content_length": len(content),
				"max_length":    r.config.MaxContentLength,
			},
			[]string{"Reduce content size or split into multiple events"}))
		return result
	}
	
	// Rate limiting check
	if r.config.EnableRateLimiting {
		if err := r.rateLimiter.CheckLimit(event); err != nil {
			result.AddError(r.CreateError(event,
				"Rate limit exceeded",
				map[string]interface{}{
					"error": err.Error(),
				},
				[]string{"Reduce event frequency", "Implement client-side throttling"}))
			r.detectionMetrics.RecordRateLimitExceeded(event.Type())
		}
	}
	
	// Input sanitization and XSS detection
	if r.config.EnableXSSDetection {
		if violations := r.detectXSS(content); len(violations) > 0 {
			result.AddError(r.CreateError(event,
				"Potential XSS attack detected",
				map[string]interface{}{
					"violations": violations,
					"content":    r.sanitizeForLogging(content),
				},
				[]string{"Remove or escape HTML/JavaScript content", "Use proper content encoding"}))
			r.detectionMetrics.RecordXSSDetection(event.Type())
		}
	}
	
	// SQL injection detection
	if r.config.EnableSQLInjectionDetection {
		if violations := r.detectSQLInjection(content); len(violations) > 0 {
			result.AddError(r.CreateError(event,
				"Potential SQL injection detected",
				map[string]interface{}{
					"violations": violations,
					"content":    r.sanitizeForLogging(content),
				},
				[]string{"Use parameterized queries", "Sanitize input data", "Avoid dynamic SQL construction"}))
			r.detectionMetrics.RecordSQLInjectionDetection(event.Type())
		}
	}
	
	// Command injection detection
	if r.config.EnableCommandInjection {
		if violations := r.detectCommandInjection(content); len(violations) > 0 {
			result.AddError(r.CreateError(event,
				"Potential command injection detected",
				map[string]interface{}{
					"violations": violations,
					"content":    r.sanitizeForLogging(content),
				},
				[]string{"Avoid executing system commands with user input", "Use safe APIs instead of shell commands"}))
			r.detectionMetrics.RecordCommandInjectionDetection(event.Type())
		}
	}
	
	// Path traversal detection
	if r.config.EnablePathTraversalDetection {
		if violations := r.detectPathTraversal(content); len(violations) > 0 {
			result.AddError(r.CreateError(event,
				"Potential path traversal detected",
				map[string]interface{}{
					"violations": violations,
					"content":    r.sanitizeForLogging(content),
				},
				[]string{"Validate and sanitize file paths", "Use safe path resolution APIs"}))
			r.detectionMetrics.RecordPathTraversalDetection(event.Type())
		}
	}
	
	// Anomaly detection
	if r.config.EnableAnomalyDetection {
		if anomaly := r.anomalyDetector.DetectAnomaly(event, context); anomaly != nil {
			result.AddWarning(r.CreateError(event,
				"Anomalous event pattern detected",
				map[string]interface{}{
					"anomaly_type":  anomaly.Type,
					"anomaly_score": anomaly.Score,
					"details":       anomaly.Details,
				},
				[]string{"Review event patterns", "Check for automated or malicious activity"}))
			r.detectionMetrics.RecordAnomalyDetection(event.Type())
		}
	}
	
	// Encryption validation
	if r.config.RequireEncryption {
		if err := r.encryptionValidator.ValidateEncryption(event); err != nil {
			result.AddError(r.CreateError(event,
				"Encryption validation failed",
				map[string]interface{}{
					"error": err.Error(),
				},
				[]string{"Ensure content is properly encrypted", "Use approved encryption algorithms"}))
			r.detectionMetrics.RecordEncryptionFailure(event.Type())
		}
	}
	
	return result
}

// extractEventContent extracts content from various event types
func (r *SecurityValidationRule) extractEventContent(event events.Event) string {
	switch e := event.(type) {
	case *events.TextMessageContentEvent:
		return e.Delta
	case *events.ToolCallArgsEvent:
		return e.Delta
	case *events.RunErrorEvent:
		return e.Message
	case *events.CustomEvent:
		// Convert value to string for analysis
		if e.Value != nil {
			return fmt.Sprintf("%v", e.Value)
		}
		return e.Name
	default:
		// For other event types, check if they have content fields
		return ""
	}
}

// detectXSS detects potential XSS attacks
func (r *SecurityValidationRule) detectXSS(content string) []string {
	var violations []string
	
	// First, check the original content
	for _, pattern := range r.xssPatterns {
		if matches := pattern.FindAllString(content, -1); len(matches) > 0 {
			for _, match := range matches {
				violations = append(violations, fmt.Sprintf("XSS pattern detected: %s", r.sanitizeForLogging(match)))
			}
		}
	}
	
	// Check for URL-encoded XSS attempts
	urlDecodedContent, err := url.QueryUnescape(content)
	if err == nil && urlDecodedContent != content {
		for _, pattern := range r.xssPatterns {
			if matches := pattern.FindAllString(urlDecodedContent, -1); len(matches) > 0 {
				violations = append(violations, "URL-encoded XSS pattern detected")
				break
			}
		}
	}
	
	// Check for HTML-encoded XSS attempts
	htmlDecodedContent := html.UnescapeString(content)
	if htmlDecodedContent != content {
		for _, pattern := range r.xssPatterns {
			if matches := pattern.FindAllString(htmlDecodedContent, -1); len(matches) > 0 {
				violations = append(violations, "HTML-encoded XSS pattern detected")
				break
			}
		}
	}
	
	// Check for double-encoded attempts (URL then HTML or vice versa)
	if err == nil && urlDecodedContent != content {
		doubleDecoded := html.UnescapeString(urlDecodedContent)
		if doubleDecoded != urlDecodedContent {
			for _, pattern := range r.xssPatterns {
				if matches := pattern.FindAllString(doubleDecoded, -1); len(matches) > 0 {
					violations = append(violations, "Double-encoded XSS pattern detected")
					break
				}
			}
		}
	}
	
	return violations
}

// detectSQLInjection detects potential SQL injection attacks
func (r *SecurityValidationRule) detectSQLInjection(content string) []string {
	var violations []string
	
	for _, pattern := range r.sqlPatterns {
		if matches := pattern.FindAllString(content, -1); len(matches) > 0 {
			for _, match := range matches {
				violations = append(violations, fmt.Sprintf("SQL pattern detected: %s", r.sanitizeForLogging(match)))
			}
		}
	}
	
	// Check for encoded SQL injection attempts
	if decoded, err := base64.StdEncoding.DecodeString(content); err == nil {
		decodedStr := string(decoded)
		for _, pattern := range r.sqlPatterns {
			if pattern.MatchString(decodedStr) {
				violations = append(violations, "Base64 encoded SQL injection pattern detected")
				break
			}
		}
	}
	
	return violations
}

// detectCommandInjection detects potential command injection attacks
func (r *SecurityValidationRule) detectCommandInjection(content string) []string {
	var violations []string
	
	for _, pattern := range r.commandPatterns {
		if matches := pattern.FindAllString(content, -1); len(matches) > 0 {
			for _, match := range matches {
				violations = append(violations, fmt.Sprintf("Command injection pattern detected: %s", r.sanitizeForLogging(match)))
			}
		}
	}
	
	return violations
}

// detectPathTraversal detects potential path traversal attacks
func (r *SecurityValidationRule) detectPathTraversal(content string) []string {
	var violations []string
	
	// Check the original content
	for _, pattern := range r.pathTraversalPatterns {
		if matches := pattern.FindAllString(content, -1); len(matches) > 0 {
			for _, match := range matches {
				violations = append(violations, fmt.Sprintf("Path traversal pattern detected: %s", r.sanitizeForLogging(match)))
			}
		}
	}
	
	// Check for URL-encoded path traversal attempts
	urlDecodedContent, err := url.QueryUnescape(content)
	if err == nil && urlDecodedContent != content {
		for _, pattern := range r.pathTraversalPatterns {
			if matches := pattern.FindAllString(urlDecodedContent, -1); len(matches) > 0 {
				violations = append(violations, "URL-encoded path traversal pattern detected")
				break
			}
		}
	}
	
	// Check for double-encoded attempts
	if err == nil && urlDecodedContent != content {
		doubleDecoded, err2 := url.QueryUnescape(urlDecodedContent)
		if err2 == nil && doubleDecoded != urlDecodedContent {
			for _, pattern := range r.pathTraversalPatterns {
				if matches := pattern.FindAllString(doubleDecoded, -1); len(matches) > 0 {
					violations = append(violations, "Double-encoded path traversal pattern detected")
					break
				}
			}
		}
	}
	
	return violations
}

// sanitizeForLogging sanitizes content for safe logging
func (r *SecurityValidationRule) sanitizeForLogging(content string) string {
	if len(content) > 100 {
		content = content[:100] + "..."
	}
	// Replace potentially dangerous characters
	content = strings.ReplaceAll(content, "<", "&lt;")
	content = strings.ReplaceAll(content, ">", "&gt;")
	content = strings.ReplaceAll(content, "&", "&amp;")
	content = strings.ReplaceAll(content, "\"", "&quot;")
	content = strings.ReplaceAll(content, "'", "&#39;")
	return content
}

// GetMetrics returns security detection metrics
func (r *SecurityValidationRule) GetMetrics() *SecurityMetrics {
	return r.detectionMetrics
}

// UpdateConfig updates the security configuration
func (r *SecurityValidationRule) UpdateConfig(config *SecurityConfig) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.config = config
	r.rateLimiter.UpdateConfig(config)
	r.anomalyDetector.UpdateConfig(config)
	r.encryptionValidator.UpdateConfig(config)
}

// SanitizeContent provides content sanitization functionality
func (r *SecurityValidationRule) SanitizeContent(content string) string {
	if !r.config.EnableInputSanitization {
		return content
	}
	
	// Basic HTML escaping
	sanitized := html.EscapeString(content)
	
	// Additional sanitization for specific patterns
	for _, pattern := range r.xssPatterns {
		sanitized = pattern.ReplaceAllString(sanitized, "")
	}
	
	return sanitized
}