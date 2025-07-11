package validation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	agerrors "github.com/ag-ui/go-sdk/pkg/errors"
)

// SecurityValidator provides security validation for encoding/decoding operations
type SecurityValidator struct {
	config SecurityConfig
}

// SecurityConfig defines security validation configuration
type SecurityConfig struct {
	// Size limits
	MaxInputSize      int64  // Maximum input data size in bytes
	MaxOutputSize     int64  // Maximum output data size in bytes
	MaxStringLength   int    // Maximum string field length
	MaxArrayLength    int    // Maximum array field length
	MaxNestingDepth   int    // Maximum nesting depth for objects
	MaxFieldCount     int    // Maximum number of fields in an object

	// Content validation
	AllowHTMLContent      bool     // Allow HTML tags in string fields
	AllowScriptContent    bool     // Allow script tags and javascript: URLs
	AllowedURLSchemes     []string // Allowed URL schemes (http, https, etc.)
	BlockedPatterns       []string // Blocked regex patterns
	SanitizeInput         bool     // Enable input sanitization

	// Resource limits
	MaxProcessingTime     time.Duration // Maximum processing time
	MaxMemoryUsage        int64         // Maximum memory usage in bytes
	EnableResourceMonitor bool          // Enable resource monitoring

	// Attack prevention
	EnableInjectionPrevention bool // Enable injection attack prevention
	EnableDOSPrevention       bool // Enable DoS attack prevention
	EnableXSSPrevention       bool // Enable XSS attack prevention
}

// DefaultSecurityConfig returns the default security configuration
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		MaxInputSize:      10 * 1024 * 1024, // 10MB
		MaxOutputSize:     10 * 1024 * 1024, // 10MB
		MaxStringLength:   1024 * 1024,      // 1MB
		MaxArrayLength:    10000,
		MaxNestingDepth:   50,
		MaxFieldCount:     1000,
		AllowHTMLContent:  false,
		AllowScriptContent: false,
		AllowedURLSchemes: []string{"http", "https"},
		BlockedPatterns: []string{
			`<script[^>]*>.*?</script>`,
			`javascript:`,
			`data:text/html`,
			`vbscript:`,
			`onload\s*=`,
			`onerror\s*=`,
		},
		SanitizeInput:             true,
		MaxProcessingTime:         30 * time.Second,
		MaxMemoryUsage:           100 * 1024 * 1024, // 100MB
		EnableResourceMonitor:     true,
		EnableInjectionPrevention: true,
		EnableDOSPrevention:      true,
		EnableXSSPrevention:      true,
	}
}

// StrictSecurityConfig returns a strict security configuration
func StrictSecurityConfig() SecurityConfig {
	config := DefaultSecurityConfig()
	config.MaxInputSize = 1 * 1024 * 1024      // 1MB
	config.MaxOutputSize = 1 * 1024 * 1024     // 1MB
	config.MaxStringLength = 64 * 1024         // 64KB
	config.MaxArrayLength = 1000
	config.MaxNestingDepth = 20
	config.MaxFieldCount = 100
	config.MaxProcessingTime = 10 * time.Second
	config.MaxMemoryUsage = 50 * 1024 * 1024   // 50MB
	return config
}

// NewSecurityValidator creates a new security validator
func NewSecurityValidator(config SecurityConfig) *SecurityValidator {
	return &SecurityValidator{
		config: config,
	}
}

// ValidateInput validates input data for security issues
func (v *SecurityValidator) ValidateInput(ctx context.Context, data []byte) error {
	// Check size limits
	if err := v.validateSize(data); err != nil {
		return err
	}

	// Check for malformed data
	if err := v.validateFormat(data); err != nil {
		return err
	}

	// Check for injection attacks
	if v.config.EnableInjectionPrevention {
		if err := v.validateInjectionPatterns(data); err != nil {
			return err
		}
	}

	// Check for DoS patterns
	if v.config.EnableDOSPrevention {
		if err := v.validateDOSPatterns(data); err != nil {
			return err
		}
	}

	return nil
}

// ValidateEvent validates an event for security issues
func (v *SecurityValidator) ValidateEvent(ctx context.Context, event events.Event) error {
	if event == nil {
		return agerrors.NewSecurityError(agerrors.CodeMissingEvent, "nil event provided for validation")
	}

	// Validate event structure
	if err := v.validateEventStructure(event); err != nil {
		return err
	}

	// Validate event content
	if err := v.validateEventContent(event); err != nil {
		return err
	}

	return nil
}

// SanitizeInput sanitizes input data
func (v *SecurityValidator) SanitizeInput(data []byte) ([]byte, error) {
	if !v.config.SanitizeInput {
		return data, nil
	}

	// Parse as JSON for sanitization
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, agerrors.NewEncodingError(agerrors.CodeDecodingFailed, "failed to parse JSON for sanitization").WithOperation("sanitize").WithCause(err)
	}

	// Sanitize the data structure
	sanitized := v.sanitizeValue(jsonData)

	// Marshal back to JSON
	result, err := json.Marshal(sanitized)
	if err != nil {
		return nil, agerrors.NewEncodingError(agerrors.CodeEncodingFailed, "failed to marshal sanitized data").WithOperation("sanitize").WithCause(err)
	}

	return result, nil
}

// validateSize validates data size limits
func (v *SecurityValidator) validateSize(data []byte) error {
	size := int64(len(data))
	if size > v.config.MaxInputSize {
		return agerrors.NewSecurityError(agerrors.CodeSizeExceeded, fmt.Sprintf("input size %d exceeds maximum %d", size, v.config.MaxInputSize)).
			WithViolationType("size_limit").
			WithDetail("size", size).
			WithDetail("max_size", v.config.MaxInputSize)
	}
	return nil
}

// validateFormat validates data format for malformed content
func (v *SecurityValidator) validateFormat(data []byte) error {
	// Check for null bytes
	if bytes.Contains(data, []byte{0}) {
		return agerrors.NewSecurityError(agerrors.CodeNullByteDetected, "input contains null bytes").
			WithViolationType("null_byte_injection").
			WithRiskLevel("high")
	}

	// Check for extremely long lines (potential DoS)
	lines := bytes.Split(data, []byte("\n"))
	for i, line := range lines {
		if len(line) > 100*1024 { // 100KB per line
			return agerrors.NewSecurityError(agerrors.CodeSizeExceeded, fmt.Sprintf("line %d exceeds maximum length", i+1)).
				WithViolationType("line_length").
				WithLocation(fmt.Sprintf("line_%d", i+1)).
				WithDetail("line_length", len(line)).
				WithDetail("max_length", 100*1024)
		}
	}

	// Check for valid UTF-8
	if !isValidUTF8(data) {
		return agerrors.NewSecurityError(agerrors.CodeInvalidUTF8, "input contains invalid UTF-8").
			WithViolationType("encoding_violation").
			WithRiskLevel("medium")
	}

	return nil
}

// validateInjectionPatterns validates against injection attack patterns
func (v *SecurityValidator) validateInjectionPatterns(data []byte) error {
	dataStr := string(data)

	// Check blocked patterns
	for _, pattern := range v.config.BlockedPatterns {
		matched, err := regexp.MatchString(pattern, dataStr)
		if err != nil {
			return agerrors.NewSecurityError(agerrors.CodeSecurityViolation, "regex pattern error").WithPattern(pattern).WithCause(err)
		}
		if matched {
			return agerrors.NewSecurityError(agerrors.CodeSecurityViolation, "input matches blocked pattern").
				WithViolationType("blocked_pattern").
				WithPattern(pattern).
				WithRiskLevel("high")
		}
	}

	// Check for script injection
	if !v.config.AllowScriptContent {
		scriptPatterns := []string{
			`(?i)<script[^>]*>`,
			`(?i)javascript:`,
			`(?i)vbscript:`,
			`(?i)data:text/html`,
			`(?i)on\w+\s*=`,
		}
		for _, pattern := range scriptPatterns {
			matched, _ := regexp.MatchString(pattern, dataStr)
			if matched {
				return agerrors.NewScriptInjectionError("script injection detected", pattern)
			}
		}
	}

	// Check for SQL injection patterns
	sqlPatterns := []string{
		`(?i)(union\s+select)`,
		`(?i)(drop\s+table)`,
		`(?i)(insert\s+into)`,
		`(?i)(delete\s+from)`,
		`(?i)(\'\s*or\s*\'\s*=\s*\')`,
		`(?i)(\'\s*;\s*drop)`,
	}
	for _, pattern := range sqlPatterns {
		matched, _ := regexp.MatchString(pattern, dataStr)
		if matched {
			return agerrors.NewSQLInjectionError("SQL injection pattern detected", pattern)
		}
	}

	return nil
}

// validateDOSPatterns validates against DoS attack patterns
func (v *SecurityValidator) validateDOSPatterns(data []byte) error {
	dataStr := string(data)

	// Check for billion laughs attack patterns
	if strings.Contains(dataStr, "&lol;") || strings.Contains(dataStr, "<!ENTITY") {
		return agerrors.NewSecurityError(agerrors.CodeEntityExpansion, "XML entity expansion attack detected").
			WithViolationType("entity_expansion").
			WithRiskLevel("critical")
	}

	// Check for zip bomb patterns
	if strings.Contains(dataStr, "PK\x03\x04") {
		return agerrors.NewSecurityError(agerrors.CodeZipBomb, "potential zip bomb detected").
			WithViolationType("zip_bomb").
			WithRiskLevel("high")
	}

	// Check for nested structures (JSON bomb)
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err == nil {
		if err := v.validateNestingDepth(jsonData, 0); err != nil {
			return err
		}
	}

	// Check for excessive repetition
	if err := v.validateRepetition(dataStr); err != nil {
		return err
	}

	return nil
}

// validateNestingDepth validates nesting depth to prevent stack overflow
func (v *SecurityValidator) validateNestingDepth(data interface{}, depth int) error {
	if depth > v.config.MaxNestingDepth {
		return agerrors.NewSecurityError(agerrors.CodeDepthExceeded, fmt.Sprintf("nesting depth %d exceeds maximum %d", depth, v.config.MaxNestingDepth)).
			WithViolationType("depth_limit").
			WithDetail("depth", depth).
			WithDetail("max_depth", v.config.MaxNestingDepth)
	}

	switch typed := data.(type) {
	case map[string]interface{}:
		if len(typed) > v.config.MaxFieldCount {
			return agerrors.NewSecurityError(agerrors.CodeSizeExceeded, fmt.Sprintf("object field count %d exceeds maximum %d", len(typed), v.config.MaxFieldCount)).
				WithViolationType("field_count").
				WithDetail("field_count", len(typed)).
				WithDetail("max_fields", v.config.MaxFieldCount)
		}
		for _, value := range typed {
			if err := v.validateNestingDepth(value, depth+1); err != nil {
				return err
			}
		}
	case []interface{}:
		if len(typed) > v.config.MaxArrayLength {
			return agerrors.NewSecurityError(agerrors.CodeSizeExceeded, fmt.Sprintf("array length %d exceeds maximum %d", len(typed), v.config.MaxArrayLength)).
				WithViolationType("array_length").
				WithDetail("array_length", len(typed)).
				WithDetail("max_length", v.config.MaxArrayLength)
		}
		for _, value := range typed {
			if err := v.validateNestingDepth(value, depth+1); err != nil {
				return err
			}
		}
	case string:
		if len(typed) > v.config.MaxStringLength {
			return agerrors.NewSecurityError(agerrors.CodeSizeExceeded, fmt.Sprintf("string length %d exceeds maximum %d", len(typed), v.config.MaxStringLength)).
				WithViolationType("string_length").
				WithDetail("string_length", len(typed)).
				WithDetail("max_length", v.config.MaxStringLength)
		}
	}

	return nil
}

// validateRepetition validates against excessive repetition
func (v *SecurityValidator) validateRepetition(data string) error {
	// Check for excessive character repetition
	for i := 0; i < len(data)-1000; i++ {
		if data[i] == data[i+1000] {
			// Check if this pattern repeats
			pattern := data[i : i+1000]
			count := 0
			for j := i; j < len(data)-1000; j += 1000 {
				if strings.HasPrefix(data[j:], pattern) {
					count++
				} else {
					break
				}
			}
			if count > 10 { // More than 10 repetitions of 1KB pattern
				return agerrors.NewDOSError("excessive repetition detected (potential DoS)", "repetition_check")
			}
		}
	}

	return nil
}

// validateEventStructure validates event structure
func (v *SecurityValidator) validateEventStructure(event events.Event) error {
	baseEvent := event.GetBaseEvent()
	if baseEvent == nil {
		return agerrors.NewSecurityError(agerrors.CodeMissingEvent, "missing base event").
			WithViolationType("structure_validation")
	}

	// Validate event type
	if baseEvent.EventType == "" {
		return agerrors.NewSecurityError(agerrors.CodeMissingEventType, "missing event type").
			WithViolationType("structure_validation")
	}

	// Validate timestamp if present
	if baseEvent.TimestampMs != nil && *baseEvent.TimestampMs < 0 {
		return agerrors.NewSecurityError(agerrors.CodeNegativeTimestamp, "negative timestamp").
			WithViolationType("timestamp_validation").
			WithDetail("timestamp", *baseEvent.TimestampMs)
	}

	// Note: BaseEvent doesn't have SequenceNumber field in this implementation

	return nil
}

// validateEventContent validates event content for security issues
func (v *SecurityValidator) validateEventContent(event events.Event) error {
	switch typed := event.(type) {
	case *events.TextMessageContentEvent:
		if err := v.validateStringContent(typed.Delta); err != nil {
			return agerrors.Wrap(err, "invalid message content")
		}
		if err := v.validateID(typed.MessageID, "message"); err != nil {
			return err
		}

	case *events.ToolCallStartEvent:
		if err := v.validateStringContent(typed.ToolCallName); err != nil {
			return agerrors.Wrap(err, "invalid tool call name")
		}
		if err := v.validateID(typed.ToolCallID, "tool call"); err != nil {
			return err
		}

	case *events.RunStartedEvent:
		if err := v.validateID(typed.RunID, "run"); err != nil {
			return err
		}
		if err := v.validateID(typed.ThreadID, "thread"); err != nil {
			return err
		}

	case *events.StateSnapshotEvent:
		// Validate snapshot data structure
		if err := v.validateSnapshot(typed.Snapshot); err != nil {
			return agerrors.Wrap(err, "invalid snapshot")
		}

	case *events.CustomEvent:
		if err := v.validateStringContent(typed.Name); err != nil {
			return agerrors.Wrap(err, "invalid custom event name")
		}
		if typed.Value != nil {
			if err := v.validateCustomData(typed.Value); err != nil {
				return agerrors.Wrap(err, "invalid custom event data")
			}
		}
	}

	return nil
}

// validateStringContent validates string content for security issues
func (v *SecurityValidator) validateStringContent(content string) error {
	if len(content) > v.config.MaxStringLength {
		return agerrors.NewSecurityError(agerrors.CodeSizeExceeded, fmt.Sprintf("string length %d exceeds maximum %d", len(content), v.config.MaxStringLength)).
			WithViolationType("string_length").
			WithDetail("length", len(content)).
			WithDetail("max_length", v.config.MaxStringLength)
	}

	// Check for XSS patterns if XSS prevention is enabled
	if v.config.EnableXSSPrevention {
		xssPatterns := []string{
			`(?i)<script[^>]*>`,
			`(?i)javascript:`,
			`(?i)vbscript:`,
			`(?i)on\w+\s*=`,
			`(?i)<iframe[^>]*>`,
			`(?i)<object[^>]*>`,
			`(?i)<embed[^>]*>`,
		}
		for _, pattern := range xssPatterns {
			matched, _ := regexp.MatchString(pattern, content)
			if matched {
				return agerrors.NewXSSError("XSS pattern detected in content", pattern)
			}
		}
	}

	// Check for HTML content if not allowed
	if !v.config.AllowHTMLContent && containsHTML(content) {
		return agerrors.NewSecurityError(agerrors.CodeHTMLNotAllowed, "HTML content not allowed").
			WithViolationType("html_content").
			WithRiskLevel("medium")
	}

	return nil
}

// validateID validates ID format and content
func (v *SecurityValidator) validateID(id, idType string) error {
	if id == "" {
		return nil // Empty IDs are handled by event validation
	}

	// Check length
	if len(id) > 1000 { // Reasonable ID length limit
		return agerrors.NewSecurityError(agerrors.CodeIDTooLong, fmt.Sprintf("%s ID too long: %d", idType, len(id))).
			WithViolationType("id_length").
			WithDetail("id_type", idType).
			WithDetail("length", len(id)).
			WithDetail("max_length", 1000)
	}

	// Check for dangerous characters
	dangerousChars := []string{"\x00", "\n", "\r", "\t", "<", ">", "\"", "'", "&"}
	for _, char := range dangerousChars {
		if strings.Contains(id, char) {
			return agerrors.NewSecurityError(agerrors.CodeInvalidData, fmt.Sprintf("%s ID contains dangerous character: %s", idType, char)).
				WithViolationType("dangerous_character").
				WithRiskLevel("medium").
				WithDetail("id_type", idType).
				WithDetail("character", char)
		}
	}

	return nil
}

// validateSnapshot validates state snapshot data
func (v *SecurityValidator) validateSnapshot(snapshot interface{}) error {
	if snapshot == nil {
		return errors.New("nil snapshot")
	}

	// Convert to JSON for validation
	_, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	return v.validateNestingDepth(snapshot, 0)
}

// validateCustomData validates custom event data
func (v *SecurityValidator) validateCustomData(data interface{}) error {
	if data == nil {
		return nil
	}

	// Convert to JSON for validation
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal custom data: %w", err)
	}

	if int64(len(jsonData)) > v.config.MaxInputSize {
		return fmt.Errorf("custom data too large: %d bytes", len(jsonData))
	}

	return v.validateNestingDepth(data, 0)
}

// sanitizeValue recursively sanitizes data values
func (v *SecurityValidator) sanitizeValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case string:
		return v.sanitizeString(typed)
	case map[string]interface{}:
		sanitized := make(map[string]interface{})
		for k, val := range typed {
			sanitizedKey := v.sanitizeString(k)
			sanitized[sanitizedKey] = v.sanitizeValue(val)
		}
		return sanitized
	case []interface{}:
		sanitized := make([]interface{}, len(typed))
		for i, val := range typed {
			sanitized[i] = v.sanitizeValue(val)
		}
		return sanitized
	default:
		return value
	}
}

// sanitizeString sanitizes a string value
func (v *SecurityValidator) sanitizeString(s string) string {
	// Remove null bytes
	s = strings.ReplaceAll(s, "\x00", "")

	// Remove dangerous HTML if not allowed
	if !v.config.AllowHTMLContent {
		s = stripHTML(s)
	}

	// Remove script content if not allowed
	if !v.config.AllowScriptContent {
		s = stripScript(s)
	}

	return s
}

// Helper functions

func isValidUTF8(data []byte) bool {
	// Simple UTF-8 validation - in production, use utf8.Valid()
	for len(data) > 0 {
		if data[0] < 0x80 {
			data = data[1:]
		} else if data[0] < 0xC0 {
			return false
		} else if data[0] < 0xE0 {
			if len(data) < 2 || data[1]&0xC0 != 0x80 {
				return false
			}
			data = data[2:]
		} else if data[0] < 0xF0 {
			if len(data) < 3 || data[1]&0xC0 != 0x80 || data[2]&0xC0 != 0x80 {
				return false
			}
			data = data[3:]
		} else {
			if len(data) < 4 || data[1]&0xC0 != 0x80 || data[2]&0xC0 != 0x80 || data[3]&0xC0 != 0x80 {
				return false
			}
			data = data[4:]
		}
	}
	return true
}

func containsHTML(s string) bool {
	htmlPattern := `<[^>]+>`
	matched, _ := regexp.MatchString(htmlPattern, s)
	return matched
}

func stripHTML(s string) string {
	htmlPattern := `<[^>]*>`
	re := regexp.MustCompile(htmlPattern)
	return re.ReplaceAllString(s, "")
}

func stripScript(s string) string {
	scriptPatterns := []string{
		`(?i)<script[^>]*>.*?</script>`,
		`(?i)javascript:[^"'\s]*`,
		`(?i)vbscript:[^"'\s]*`,
		`(?i)on\w+\s*=[^"'\s]*`,
	}
	
	for _, pattern := range scriptPatterns {
		re := regexp.MustCompile(pattern)
		s = re.ReplaceAllString(s, "")
	}
	
	return s
}

// ResourceMonitor monitors resource usage during validation
type ResourceMonitor struct {
	config        SecurityConfig
	startTime     time.Time
	maxMemory     int64
	currentMemory int64
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(config SecurityConfig) *ResourceMonitor {
	return &ResourceMonitor{
		config:    config,
		startTime: time.Now(),
	}
}

// CheckLimits checks if resource limits are exceeded
func (m *ResourceMonitor) CheckLimits() error {
	// Check time limit
	if time.Since(m.startTime) > m.config.MaxProcessingTime {
		return errors.New("processing time limit exceeded")
	}

	// Check memory limit (simplified - in production, use runtime.MemStats)
	if m.currentMemory > m.config.MaxMemoryUsage {
		return fmt.Errorf("memory limit exceeded: %d > %d", m.currentMemory, m.config.MaxMemoryUsage)
	}

	return nil
}

// UpdateMemoryUsage updates the current memory usage
func (m *ResourceMonitor) UpdateMemoryUsage(usage int64) {
	m.currentMemory = usage
	if usage > m.maxMemory {
		m.maxMemory = usage
	}
}