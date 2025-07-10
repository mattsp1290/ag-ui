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
		return errors.New("nil event")
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
		return nil, fmt.Errorf("failed to parse JSON for sanitization: %w", err)
	}

	// Sanitize the data structure
	sanitized := v.sanitizeValue(jsonData)

	// Marshal back to JSON
	result, err := json.Marshal(sanitized)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sanitized data: %w", err)
	}

	return result, nil
}

// validateSize validates data size limits
func (v *SecurityValidator) validateSize(data []byte) error {
	size := int64(len(data))
	if size > v.config.MaxInputSize {
		return fmt.Errorf("input size %d exceeds maximum %d", size, v.config.MaxInputSize)
	}
	return nil
}

// validateFormat validates data format for malformed content
func (v *SecurityValidator) validateFormat(data []byte) error {
	// Check for null bytes
	if bytes.Contains(data, []byte{0}) {
		return errors.New("input contains null bytes")
	}

	// Check for extremely long lines (potential DoS)
	lines := bytes.Split(data, []byte("\n"))
	for i, line := range lines {
		if len(line) > 100*1024 { // 100KB per line
			return fmt.Errorf("line %d exceeds maximum length", i+1)
		}
	}

	// Check for valid UTF-8
	if !isValidUTF8(data) {
		return errors.New("input contains invalid UTF-8")
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
			return fmt.Errorf("regex pattern error: %w", err)
		}
		if matched {
			return fmt.Errorf("input matches blocked pattern: %s", pattern)
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
				return fmt.Errorf("script injection detected: pattern %s", pattern)
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
			return fmt.Errorf("SQL injection pattern detected: %s", pattern)
		}
	}

	return nil
}

// validateDOSPatterns validates against DoS attack patterns
func (v *SecurityValidator) validateDOSPatterns(data []byte) error {
	dataStr := string(data)

	// Check for billion laughs attack patterns
	if strings.Contains(dataStr, "&lol;") || strings.Contains(dataStr, "<!ENTITY") {
		return errors.New("XML entity expansion attack detected")
	}

	// Check for zip bomb patterns
	if strings.Contains(dataStr, "PK\x03\x04") {
		return errors.New("potential zip bomb detected")
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
		return fmt.Errorf("nesting depth %d exceeds maximum %d", depth, v.config.MaxNestingDepth)
	}

	switch typed := data.(type) {
	case map[string]interface{}:
		if len(typed) > v.config.MaxFieldCount {
			return fmt.Errorf("object field count %d exceeds maximum %d", len(typed), v.config.MaxFieldCount)
		}
		for _, value := range typed {
			if err := v.validateNestingDepth(value, depth+1); err != nil {
				return err
			}
		}
	case []interface{}:
		if len(typed) > v.config.MaxArrayLength {
			return fmt.Errorf("array length %d exceeds maximum %d", len(typed), v.config.MaxArrayLength)
		}
		for _, value := range typed {
			if err := v.validateNestingDepth(value, depth+1); err != nil {
				return err
			}
		}
	case string:
		if len(typed) > v.config.MaxStringLength {
			return fmt.Errorf("string length %d exceeds maximum %d", len(typed), v.config.MaxStringLength)
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
				return errors.New("excessive repetition detected (potential DoS)")
			}
		}
	}

	return nil
}

// validateEventStructure validates event structure
func (v *SecurityValidator) validateEventStructure(event events.Event) error {
	baseEvent := event.GetBaseEvent()
	if baseEvent == nil {
		return errors.New("missing base event")
	}

	// Validate event type
	if baseEvent.EventType == "" {
		return errors.New("missing event type")
	}

	// Validate timestamp if present
	if baseEvent.TimestampMs != nil && *baseEvent.TimestampMs < 0 {
		return errors.New("negative timestamp")
	}

	// Note: BaseEvent doesn't have SequenceNumber field in this implementation

	return nil
}

// validateEventContent validates event content for security issues
func (v *SecurityValidator) validateEventContent(event events.Event) error {
	switch typed := event.(type) {
	case *events.TextMessageContentEvent:
		if err := v.validateStringContent(typed.Delta); err != nil {
			return fmt.Errorf("invalid message content: %w", err)
		}
		if err := v.validateID(typed.MessageID, "message"); err != nil {
			return err
		}

	case *events.ToolCallStartEvent:
		if err := v.validateStringContent(typed.ToolCallName); err != nil {
			return fmt.Errorf("invalid tool call name: %w", err)
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
			return fmt.Errorf("invalid snapshot: %w", err)
		}

	case *events.CustomEvent:
		if err := v.validateStringContent(typed.Name); err != nil {
			return fmt.Errorf("invalid custom event name: %w", err)
		}
		if typed.Value != nil {
			if err := v.validateCustomData(typed.Value); err != nil {
				return fmt.Errorf("invalid custom event data: %w", err)
			}
		}
	}

	return nil
}

// validateStringContent validates string content for security issues
func (v *SecurityValidator) validateStringContent(content string) error {
	if len(content) > v.config.MaxStringLength {
		return fmt.Errorf("string length %d exceeds maximum %d", len(content), v.config.MaxStringLength)
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
				return fmt.Errorf("XSS pattern detected: %s", pattern)
			}
		}
	}

	// Check for HTML content if not allowed
	if !v.config.AllowHTMLContent && containsHTML(content) {
		return errors.New("HTML content not allowed")
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
		return fmt.Errorf("%s ID too long: %d", idType, len(id))
	}

	// Check for dangerous characters
	dangerousChars := []string{"\x00", "\n", "\r", "\t", "<", ">", "\"", "'", "&"}
	for _, char := range dangerousChars {
		if strings.Contains(id, char) {
			return fmt.Errorf("%s ID contains dangerous character: %s", idType, char)
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