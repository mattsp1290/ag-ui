package security

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// ThreatDetectionConfig represents threat detection configuration
type ThreatDetectionConfig struct {
	Enabled          bool     `json:"enabled" yaml:"enabled"`
	SQLInjection     bool     `json:"sql_injection" yaml:"sql_injection"`
	XSSDetection     bool     `json:"xss_detection" yaml:"xss_detection"`
	PathTraversal    bool     `json:"path_traversal" yaml:"path_traversal"`
	CommandInjection bool     `json:"command_injection" yaml:"command_injection"`
	BlockSuspicious  bool     `json:"block_suspicious" yaml:"block_suspicious"`
	LogThreats       bool     `json:"log_threats" yaml:"log_threats"`
	ThreatPatterns   []string `json:"threat_patterns" yaml:"threat_patterns"`
}

// ThreatDetector handles threat detection functionality
type ThreatDetector struct {
	config       *ThreatDetectionConfig
	threatRegexs []*regexp.Regexp
}

// NewThreatDetector creates a new threat detector
func NewThreatDetector(config *ThreatDetectionConfig) (*ThreatDetector, error) {
	if config == nil {
		config = &ThreatDetectionConfig{
			Enabled:          true,
			SQLInjection:     true,
			XSSDetection:     true,
			PathTraversal:    true,
			CommandInjection: true,
			BlockSuspicious:  true,
			LogThreats:       true,
		}
	}

	td := &ThreatDetector{config: config}

	// Initialize threat patterns
	if err := td.initThreatPatterns(); err != nil {
		return nil, fmt.Errorf("failed to initialize threat patterns: %w", err)
	}

	return td, nil
}

// DetectThreats detects security threats in the request
func (td *ThreatDetector) DetectThreats(ctx context.Context, req *Request) (string, error) {
	if !td.config.Enabled {
		return "", nil
	}

	// Check for SQL injection
	if td.config.SQLInjection {
		if threat := td.detectSQLInjection(req); threat != "" {
			return threat, nil
		}
	}

	// Check for XSS
	if td.config.XSSDetection {
		if threat := td.detectXSS(req); threat != "" {
			return threat, nil
		}
	}

	// Check for path traversal
	if td.config.PathTraversal {
		if threat := td.detectPathTraversal(req); threat != "" {
			return threat, nil
		}
	}

	// Check for command injection
	if td.config.CommandInjection {
		if threat := td.detectCommandInjection(req); threat != "" {
			return threat, nil
		}
	}

	// Check custom threat patterns
	if threat := td.detectCustomThreats(req); threat != "" {
		return threat, nil
	}

	return "", nil
}

// detectSQLInjection detects SQL injection attempts
func (td *ThreatDetector) detectSQLInjection(req *Request) string {
	sqlPatterns := []string{
		`(?i)(union\s+select)`,
		`(?i)(select\s+.*\s+from)`,
		`(?i)(insert\s+into)`,
		`(?i)(update\s+.*\s+set)`,
		`(?i)(delete\s+from)`,
		`(?i)(drop\s+table)`,
		`(?i)(or\s+1\s*=\s*1)`,
		`(?i)(and\s+1\s*=\s*1)`,
		`(?i)'.*'`,
		`(?i);\s*--`,
		`(?i)/\*.*\*/`,
	}

	for _, pattern := range sqlPatterns {
		regex := regexp.MustCompile(pattern)
		if td.checkPatternInRequest(req, regex) {
			return "SQL injection attempt detected"
		}
	}

	return ""
}

// detectXSS detects XSS attempts
func (td *ThreatDetector) detectXSS(req *Request) string {
	xssPatterns := []string{
		`(?i)<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>`,
		`(?i)<iframe\b[^<]*(?:(?!<\/iframe>)<[^<]*)*<\/iframe>`,
		`(?i)<object\b[^<]*(?:(?!<\/object>)<[^<]*)*<\/object>`,
		`(?i)<embed\b[^>]*>`,
		`(?i)<link\b[^>]*>`,
		`(?i)<meta\b[^>]*>`,
		`(?i)javascript:`,
		`(?i)vbscript:`,
		`(?i)on\w+\s*=`,
		`(?i)expression\s*\(`,
	}

	for _, pattern := range xssPatterns {
		regex := regexp.MustCompile(pattern)
		if td.checkPatternInRequest(req, regex) {
			return "XSS attempt detected"
		}
	}

	return ""
}

// detectPathTraversal detects path traversal attempts
func (td *ThreatDetector) detectPathTraversal(req *Request) string {
	pathTraversalPatterns := []string{
		`\.\.\/`,
		`\.\.\\\\ `,
		`%2e%2e%2f`,
		`%2e%2e%5c`,
		`..%2f`,
		`..%5c`,
		`%252e%252e%252f`,
	}

	for _, pattern := range pathTraversalPatterns {
		if strings.Contains(strings.ToLower(req.Path), strings.ToLower(pattern)) {
			return "Path traversal attempt detected"
		}

		// Check in headers and body
		regex := regexp.MustCompile(pattern)
		if td.checkPatternInRequest(req, regex) {
			return "Path traversal attempt detected"
		}
	}

	return ""
}

// detectCommandInjection detects command injection attempts
func (td *ThreatDetector) detectCommandInjection(req *Request) string {
	cmdPatterns := []string{
		`(?i);\s*rm\s+`,
		`(?i);\s*cat\s+`,
		`(?i);\s*ls\s+`,
		`(?i);\s*ps\s+`,
		`(?i);\s*id\s*`,
		`(?i);\s*whoami\s*`,
		`(?i);\s*pwd\s*`,
		`(?i)\|\s*nc\s+`,
		`(?i)\|\s*wget\s+`,
		`(?i)\|\s*curl\s+`,
		`(?i)&&\s*rm\s+`,
		`(?i)&&\s*cat\s+`,
	}

	for _, pattern := range cmdPatterns {
		regex := regexp.MustCompile(pattern)
		if td.checkPatternInRequest(req, regex) {
			return "Command injection attempt detected"
		}
	}

	return ""
}

// detectCustomThreats detects custom threat patterns
func (td *ThreatDetector) detectCustomThreats(req *Request) string {
	for _, regex := range td.threatRegexs {
		if td.checkPatternInRequest(req, regex) {
			return "Custom threat pattern detected"
		}
	}

	return ""
}

// checkPatternInRequest checks if a pattern exists in the request
func (td *ThreatDetector) checkPatternInRequest(req *Request, pattern *regexp.Regexp) bool {
	// Check in path
	if pattern.MatchString(req.Path) {
		return true
	}

	// Check in headers
	for k, v := range req.Headers {
		if pattern.MatchString(k) || pattern.MatchString(v) {
			return true
		}
	}

	// Check in body
	if req.Body != nil {
		bodyStr := fmt.Sprintf("%v", req.Body)
		if pattern.MatchString(bodyStr) {
			return true
		}
	}

	return false
}

// initThreatPatterns initializes threat detection regex patterns
func (td *ThreatDetector) initThreatPatterns() error {
	if len(td.config.ThreatPatterns) == 0 {
		return nil
	}

	patterns := make([]*regexp.Regexp, 0, len(td.config.ThreatPatterns))
	for _, pattern := range td.config.ThreatPatterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("failed to compile threat pattern '%s': %w", pattern, err)
		}
		patterns = append(patterns, regex)
	}

	td.threatRegexs = patterns
	return nil
}

// Enabled returns whether threat detection is enabled
func (td *ThreatDetector) Enabled() bool {
	return td.config.Enabled
}

// ShouldBlock returns whether suspicious requests should be blocked
func (td *ThreatDetector) ShouldBlock() bool {
	return td.config.BlockSuspicious
}

// ShouldLog returns whether threats should be logged
func (td *ThreatDetector) ShouldLog() bool {
	return td.config.LogThreats
}
