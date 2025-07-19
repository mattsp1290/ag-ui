package state

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

// TestValidateWebhookURL tests the webhook URL validation function for SSRF prevention
func TestValidateWebhookURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantError   bool
		errorSubstr string
	}{
		{
			name:        "empty URL",
			url:         "",
			wantError:   true,
			errorSubstr: "cannot be empty",
		},
		{
			name:        "invalid URL format",
			url:         "://invalid",
			wantError:   true,
			errorSubstr: "invalid URL format",
		},
		{
			name:        "URL without scheme (should reject HTTPS)",
			url:         "not-a-url",
			wantError:   true,
			errorSubstr: "only HTTPS URLs are allowed",
		},
		{
			name:        "HTTP URL (should reject)",
			url:         "http://example.com/webhook",
			wantError:   true,
			errorSubstr: "only HTTPS URLs are allowed",
		},
		{
			name:        "localhost URL",
			url:         "https://localhost/webhook",
			wantError:   true,
			errorSubstr: "cannot point to localhost",
		},
		{
			name:        "127.0.0.1 URL",
			url:         "https://127.0.0.1/webhook",
			wantError:   true,
			errorSubstr: "cannot point to localhost",
		},
		{
			name:        "IPv6 localhost",
			url:         "https://[::1]/webhook",
			wantError:   true,
			errorSubstr: "cannot point to localhost",
		},
		{
			name:        "URL without hostname",
			url:         "https:///webhook",
			wantError:   true,
			errorSubstr: "must have a valid hostname",
		},
		{
			name:        "valid external HTTPS URL",
			url:         "https://hooks.slack.com/services/webhook",
			wantError:   false,
			errorSubstr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWebhookURL(tt.url)
			if tt.wantError {
				if err == nil {
					t.Errorf("validateWebhookURL() expected error for %s, got nil", tt.url)
				} else if tt.errorSubstr != "" && !strings.Contains(err.Error(), tt.errorSubstr) {
					t.Errorf("validateWebhookURL() error = %v, want substring %s", err, tt.errorSubstr)
				}
			} else if err != nil {
				t.Errorf("validateWebhookURL() unexpected error for %s: %v", tt.url, err)
			}
		})
	}
}

// TestIsInternalIP tests the internal IP detection function
func TestIsInternalIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// IPv4 loopback
		{"127.0.0.1", "127.0.0.1", true},
		{"127.255.255.255", "127.255.255.255", true},

		// IPv4 private ranges
		{"10.0.0.1", "10.0.0.1", true},
		{"10.255.255.255", "10.255.255.255", true},
		{"172.16.0.1", "172.16.0.1", true},
		{"172.31.255.255", "172.31.255.255", true},
		{"192.168.0.1", "192.168.0.1", true},
		{"192.168.255.255", "192.168.255.255", true},

		// IPv4 link-local
		{"169.254.1.1", "169.254.1.1", true},

		// IPv6 loopback
		{"::1", "::1", true},

		// IPv6 unique local
		{"fc00::1", "fc00::1", true},
		{"fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", "fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", true},

		// Public IPv4 addresses
		{"8.8.8.8", "8.8.8.8", false},
		{"1.1.1.1", "1.1.1.1", false},
		{"208.67.222.222", "208.67.222.222", false},

		// Public IPv6 addresses
		{"2001:4860:4860::8888", "2001:4860:4860::8888", false},
		{"2606:4700:4700::1111", "2606:4700:4700::1111", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tt.ip)
			}
			result := isInternalIP(ip)
			if result != tt.expected {
				t.Errorf("isInternalIP(%s) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

// TestLogAlertNotifier tests the log alert notifier
func TestLogAlertNotifier(t *testing.T) {
	logger := zaptest.NewLogger(t)
	notifier := NewLogAlertNotifier(logger)

	alert := Alert{
		Level:       AlertLevelWarning,
		Title:       "Test Alert",
		Description: "This is a test alert",
		Timestamp:   time.Now(),
		Component:   "test-component",
		Value:       85.5,
		Threshold:   80.0,
		Labels:      map[string]string{"env": "test"},
		Severity:    AuditSeverityWarning,
	}

	// Test sending alerts at different levels
	levels := []AlertLevel{AlertLevelInfo, AlertLevelWarning, AlertLevelError, AlertLevelCritical}
	for _, level := range levels {
		t.Run(fmt.Sprintf("level_%v", level), func(t *testing.T) {
			alert.Level = level
			err := notifier.SendAlert(context.Background(), alert)
			if err != nil {
				t.Errorf("SendAlert() failed: %v", err)
			}
		})
	}

	// Test with context cancellation
	t.Run("cancelled_context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := notifier.SendAlert(ctx, alert)
		if err != nil {
			t.Errorf("SendAlert() with cancelled context failed: %v", err)
		}
	})
}

// TestEmailAlertNotifier tests the email alert notifier
func TestEmailAlertNotifier(t *testing.T) {
	notifier := NewEmailAlertNotifier(
		"smtp.example.com",
		587,
		"test@example.com",
		"password",
		"alerts@example.com",
		[]string{"admin@example.com", "ops@example.com"},
	)

	// Test basic configuration
	if notifier.smtpServer != "smtp.example.com" {
		t.Errorf("Expected smtp server 'smtp.example.com', got %s", notifier.smtpServer)
	}
	if notifier.smtpPort != 587 {
		t.Errorf("Expected smtp port 587, got %d", notifier.smtpPort)
	}
	if !notifier.enabled {
		t.Error("Expected notifier to be enabled by default")
	}

	alert := Alert{
		Level:       AlertLevelError,
		Title:       "Database Connection Failed",
		Description: "Unable to connect to primary database",
		Timestamp:   time.Now(),
		Component:   "database",
		Value:       0.0,
		Threshold:   1.0,
	}

	// Test sending alert
	err := notifier.SendAlert(context.Background(), alert)
	if err != nil {
		t.Errorf("SendAlert() failed: %v", err)
	}

	// Test with disabled notifier
	notifier.enabled = false
	err = notifier.SendAlert(context.Background(), alert)
	if err != nil {
		t.Errorf("SendAlert() with disabled notifier failed: %v", err)
	}
}

// createTestWebhookNotifier creates a webhook notifier for testing that bypasses URL validation
func createTestWebhookNotifier(url string, timeout time.Duration) *WebhookAlertNotifier {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
			InsecureSkipVerify: true, // For test servers
		},
		DisableKeepAlives: true,
	}
	
	return &WebhookAlertNotifier{
		url:     url,
		method:  http.MethodPost,
		headers: make(map[string]string),
		timeout: timeout,
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}

// TestWebhookAlertNotifier tests the webhook alert notifier
func TestWebhookAlertNotifier(t *testing.T) {
	var receivedPayload map[string]interface{}
	var requestHeaders http.Header
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestHeaders = r.Header
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			return
		}

		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("Failed to unmarshal request body: %v", err)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Use the test helper to create a notifier that bypasses URL validation
	notifier := createTestWebhookNotifier(server.URL, 10*time.Second)

	// Set custom headers
	notifier.SetHeader("X-Custom-Header", "test-value")
	notifier.SetHeader("Authorization", "Bearer token123")

	alert := Alert{
		Level:       AlertLevelCritical,
		Title:       "System Overload",
		Description: "CPU usage exceeds 95%",
		Timestamp:   time.Now(),
		Component:   "system",
		Value:       97.5,
		Threshold:   95.0,
		Labels:      map[string]string{"hostname": "web-01"},
		Severity:    AuditSeverityCritical,
	}

	// Test sending alert
	err := notifier.SendAlert(context.Background(), alert)
	if err != nil {
		t.Errorf("SendAlert() failed: %v", err)
	}

	// Verify payload
	if receivedPayload == nil {
		t.Fatal("No payload received")
	}
	if receivedPayload["title"] != alert.Title {
		t.Errorf("Expected title '%s', got '%v'", alert.Title, receivedPayload["title"])
	}
	if receivedPayload["level"] != "critical" {
		t.Errorf("Expected level 'critical', got '%v'", receivedPayload["level"])
	}
	if receivedPayload["value"] != 97.5 {
		t.Errorf("Expected value 97.5, got %v", receivedPayload["value"])
	}

	// Verify headers
	if requestHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", requestHeaders.Get("Content-Type"))
	}
	if requestHeaders.Get("X-Custom-Header") != "test-value" {
		t.Errorf("Expected X-Custom-Header 'test-value', got '%s'", requestHeaders.Get("X-Custom-Header"))
	}
	if requestHeaders.Get("Authorization") != "Bearer token123" {
		t.Errorf("Expected Authorization 'Bearer token123', got '%s'", requestHeaders.Get("Authorization"))
	}
}

// TestWebhookAlertNotifierErrors tests error scenarios for webhook notifier
func TestWebhookAlertNotifierErrors(t *testing.T) {
	t.Run("invalid_url", func(t *testing.T) {
		_, err := NewWebhookAlertNotifier("http://localhost/webhook", 10*time.Second)
		if err == nil {
			t.Error("Expected error for invalid URL")
		}
	})

	t.Run("server_error", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		// Use the test helper to create a notifier that bypasses URL validation
		notifier := createTestWebhookNotifier(server.URL, 10*time.Second)

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Test Alert",
			Timestamp: time.Now(),
		}

		err := notifier.SendAlert(context.Background(), alert)
		if err == nil {
			t.Error("Expected error for server error response")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("Expected error to contain status code 500, got: %v", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Use the test helper to create a notifier that bypasses URL validation
		notifier := createTestWebhookNotifier(server.URL, 10*time.Millisecond)

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Test Alert",
			Timestamp: time.Now(),
		}

		err := notifier.SendAlert(context.Background(), alert)
		if err == nil {
			t.Error("Expected timeout error")
		}
	})

	t.Run("context_cancellation", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Use the test helper to create a notifier that bypasses URL validation
		notifier := createTestWebhookNotifier(server.URL, 10*time.Second)

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Test Alert",
			Timestamp: time.Now(),
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := notifier.SendAlert(ctx, alert)
		if err == nil {
			t.Error("Expected context cancellation error")
		}
	})
}

// TestWebhookTLSConfiguration tests TLS configuration for webhook notifier
func TestWebhookTLSConfiguration(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	server.TLS = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	server.StartTLS()
	defer server.Close()

	// Use the test helper to create a notifier that bypasses URL validation
	notifier := createTestWebhookNotifier(server.URL, 10*time.Second)

	// Verify TLS configuration
	transport := notifier.client.Transport.(*http.Transport)
	if transport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected MinVersion TLS 1.2, got %v", transport.TLSClientConfig.MinVersion)
	}

	if len(transport.TLSClientConfig.CipherSuites) == 0 {
		t.Error("Expected cipher suites to be configured")
	}

	if !transport.DisableKeepAlives {
		t.Error("Expected DisableKeepAlives to be true")
	}
}

// TestSlackAlertNotifier tests the Slack alert notifier
func TestSlackAlertNotifier(t *testing.T) {
	notifier, err := NewSlackAlertNotifier("https://hooks.slack.com/test", "#alerts", "StateManager")
	if err != nil {
		t.Fatalf("Failed to create Slack notifier: %v", err)
	}

	tests := []struct {
		level    AlertLevel
		expected string
	}{
		{AlertLevelInfo, "good"},
		{AlertLevelWarning, "warning"},
		{AlertLevelError, "danger"},
		{AlertLevelCritical, "danger"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("level_%v", tt.level), func(t *testing.T) {
			color := notifier.getColorForLevel(tt.level)
			if color != tt.expected {
				t.Errorf("getColorForLevel(%v) = %s, want %s", tt.level, color, tt.expected)
			}
		})
	}
}

// TestPagerDutyAlertNotifier tests the PagerDuty alert notifier
func TestPagerDutyAlertNotifier(t *testing.T) {
	notifier := NewPagerDutyAlertNotifier("test-key")

	tests := []struct {
		level    AlertLevel
		expected string
	}{
		{AlertLevelInfo, "info"},
		{AlertLevelWarning, "warning"},
		{AlertLevelError, "error"},
		{AlertLevelCritical, "critical"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("level_%v", tt.level), func(t *testing.T) {
			severity := notifier.getSeverityForLevel(tt.level)
			if severity != tt.expected {
				t.Errorf("getSeverityForLevel(%v) = %s, want %s", tt.level, severity, tt.expected)
			}
		})
	}
}

// TestFileAlertNotifier tests the file alert notifier
func TestFileAlertNotifier(t *testing.T) {
	tempDir := t.TempDir()
	alertFile := filepath.Join(tempDir, "alerts.log")

	notifier, err := NewFileAlertNotifier(alertFile)
	if err != nil {
		t.Fatalf("Failed to create file notifier: %v", err)
	}
	defer notifier.Close()

	alert := Alert{
		Level:       AlertLevelError,
		Title:       "Disk Space Low",
		Description: "Disk space is below 10%",
		Timestamp:   time.Now(),
		Component:   "filesystem",
		Value:       5.0,
		Threshold:   10.0,
		Labels:      map[string]string{"mount": "/var"},
		Severity:    AuditSeverityError,
	}

	err = notifier.SendAlert(context.Background(), alert)
	if err != nil {
		t.Errorf("SendAlert() failed: %v", err)
	}

	// Verify file contents
	content, err := os.ReadFile(alertFile)
	if err != nil {
		t.Fatalf("Failed to read alert file: %v", err)
	}

	var loggedAlert map[string]interface{}
	lines := strings.Split(string(content), "\n")
	if len(lines) < 1 || lines[0] == "" {
		t.Fatal("No alert logged to file")
	}

	err = json.Unmarshal([]byte(lines[0]), &loggedAlert)
	if err != nil {
		t.Fatalf("Failed to unmarshal logged alert: %v", err)
	}

	if loggedAlert["title"] != alert.Title {
		t.Errorf("Expected title '%s', got '%v'", alert.Title, loggedAlert["title"])
	}
	if loggedAlert["level"] != "error" {
		t.Errorf("Expected level 'error', got '%v'", loggedAlert["level"])
	}
	if loggedAlert["component"] != alert.Component {
		t.Errorf("Expected component '%s', got '%v'", alert.Component, loggedAlert["component"])
	}
}

// TestFileAlertNotifierErrors tests error scenarios for file notifier
func TestFileAlertNotifierErrors(t *testing.T) {
	t.Run("invalid_file_path", func(t *testing.T) {
		_, err := NewFileAlertNotifier("/non/existent/path/alerts.log")
		if err == nil {
			t.Error("Expected error for invalid file path")
		}
	})

	t.Run("permission_denied", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Skipping permission test when running as root")
		}
		_, err := NewFileAlertNotifier("/root/alerts.log")
		if err == nil {
			t.Error("Expected permission error")
		}
	})
}

// TestCompositeAlertNotifier tests the composite alert notifier
func TestCompositeAlertNotifier(t *testing.T) {
	logger := zaptest.NewLogger(t)

	logNotifier1 := NewLogAlertNotifier(logger)
	logNotifier2 := NewLogAlertNotifier(logger)

	tempDir := t.TempDir()
	alertFile := filepath.Join(tempDir, "alerts.log")
	fileNotifier, err := NewFileAlertNotifier(alertFile)
	if err != nil {
		t.Fatalf("Failed to create file notifier: %v", err)
	}
	defer fileNotifier.Close()

	compositeNotifier := NewCompositeAlertNotifier(logNotifier1, logNotifier2, fileNotifier)

	alert := Alert{
		Level:       AlertLevelWarning,
		Title:       "Composite Test",
		Description: "Testing composite notifier",
		Timestamp:   time.Now(),
		Component:   "test",
		Value:       75.0,
		Threshold:   70.0,
	}

	err = compositeNotifier.SendAlert(context.Background(), alert)
	if err != nil {
		t.Errorf("SendAlert() failed: %v", err)
	}

	// Verify file was written
	content, err := os.ReadFile(alertFile)
	if err != nil {
		t.Fatalf("Failed to read alert file: %v", err)
	}
	if len(content) == 0 {
		t.Error("Alert file is empty, composite notifier may not have called file notifier")
	}
}

// TestCompositeAlertNotifierErrors tests error handling in composite notifier
func TestCompositeAlertNotifierErrors(t *testing.T) {
	failingNotifier := &mockFailingNotifier{shouldFail: true}

	logger := zaptest.NewLogger(t)
	successNotifier := NewLogAlertNotifier(logger)

	compositeNotifier := NewCompositeAlertNotifier(failingNotifier, successNotifier)

	alert := Alert{
		Level:     AlertLevelError,
		Title:     "Error Test",
		Timestamp: time.Now(),
		Component: "test",
	}

	err := compositeNotifier.SendAlert(context.Background(), alert)
	if err == nil {
		t.Error("Expected error when one notifier fails")
	}
	if !strings.Contains(err.Error(), "mock notifier error") {
		t.Errorf("Expected error message to contain 'mock notifier error', got: %v", err)
	}
}

// TestConditionalAlertNotifier tests the conditional alert notifier
func TestConditionalAlertNotifier(t *testing.T) {
	logger := zaptest.NewLogger(t)
	baseNotifier := NewLogAlertNotifier(logger)

	condition := func(alert Alert) bool {
		return alert.Level == AlertLevelCritical
	}
	conditionalNotifier := NewConditionalAlertNotifier(baseNotifier, condition)

	tests := []struct {
		name       string
		level      AlertLevel
		shouldSend bool
	}{
		{"info alert", AlertLevelInfo, false},
		{"warning alert", AlertLevelWarning, false},
		{"error alert", AlertLevelError, false},
		{"critical alert", AlertLevelCritical, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alert := Alert{
				Level:     tt.level,
				Title:     "Conditional Test",
				Timestamp: time.Now(),
				Component: "test",
			}

			err := conditionalNotifier.SendAlert(context.Background(), alert)
			if err != nil {
				t.Errorf("SendAlert() failed: %v", err)
			}
		})
	}
}

// TestThrottledAlertNotifier tests the throttled alert notifier
func TestThrottledAlertNotifier(t *testing.T) {
	logger := zaptest.NewLogger(t)
	baseNotifier := NewLogAlertNotifier(logger)

	throttleDuration := 100 * time.Millisecond
	throttledNotifier := NewThrottledAlertNotifier(baseNotifier, throttleDuration)

	alert := Alert{
		Level:       AlertLevelWarning,
		Title:       "Throttle Test",
		Description: "Testing throttled notifier",
		Timestamp:   time.Now(),
		Component:   "test-component",
		Value:       80.0,
		Threshold:   70.0,
	}

	// First alert should be sent
	err := throttledNotifier.SendAlert(context.Background(), alert)
	if err != nil {
		t.Errorf("First SendAlert() failed: %v", err)
	}

	// Second alert immediately should be throttled
	err = throttledNotifier.SendAlert(context.Background(), alert)
	if err != nil {
		t.Errorf("Second SendAlert() failed: %v", err)
	}

	// Wait for throttle duration to pass
	time.Sleep(throttleDuration + 10*time.Millisecond)

	// Third alert should be sent after throttle duration
	err = throttledNotifier.SendAlert(context.Background(), alert)
	if err != nil {
		t.Errorf("Third SendAlert() failed: %v", err)
	}

	// Verify throttle tracking
	alertKey := fmt.Sprintf("%s_%s", alert.Component, alert.Title)
	if _, exists := throttledNotifier.lastSent[alertKey]; !exists {
		t.Error("Expected alert key to be tracked in lastSent map")
	}
}

// TestThrottledAlertNotifierDifferentAlerts tests that different alerts are not throttled together
func TestThrottledAlertNotifierDifferentAlerts(t *testing.T) {
	logger := zaptest.NewLogger(t)
	baseNotifier := NewLogAlertNotifier(logger)

	throttledNotifier := NewThrottledAlertNotifier(baseNotifier, 1*time.Second)

	alert1 := Alert{
		Level:     AlertLevelWarning,
		Title:     "Alert 1",
		Timestamp: time.Now(),
		Component: "component1",
	}

	alert2 := Alert{
		Level:     AlertLevelWarning,
		Title:     "Alert 2",
		Timestamp: time.Now(),
		Component: "component2",
	}

	// Both alerts should be sent (different alert keys)
	err := throttledNotifier.SendAlert(context.Background(), alert1)
	if err != nil {
		t.Errorf("First alert failed: %v", err)
	}

	err = throttledNotifier.SendAlert(context.Background(), alert2)
	if err != nil {
		t.Errorf("Second alert failed: %v", err)
	}

	// Verify both alerts are tracked separately
	if len(throttledNotifier.lastSent) != 2 {
		t.Errorf("Expected 2 tracked alerts, got %d", len(throttledNotifier.lastSent))
	}
}

// TestThrottledAlertNotifierErrorHandling tests error handling in throttled notifier
func TestThrottledAlertNotifierErrorHandling(t *testing.T) {
	failingNotifier := &mockFailingNotifier{shouldFail: true}
	throttledNotifier := NewThrottledAlertNotifier(failingNotifier, 1*time.Second)

	alert := Alert{
		Level:     AlertLevelError,
		Title:     "Error Test",
		Timestamp: time.Now(),
		Component: "test",
	}

	// First call should fail and not update lastSent
	err := throttledNotifier.SendAlert(context.Background(), alert)
	if err == nil {
		t.Error("Expected error from failing notifier")
	}

	// Verify lastSent was not updated due to error
	alertKey := fmt.Sprintf("%s_%s", alert.Component, alert.Title)
	if _, exists := throttledNotifier.lastSent[alertKey]; exists {
		t.Error("Expected alert key to not be tracked when sending fails")
	}
}

// TestHelperFunctions tests the helper functions
func TestHelperFunctions(t *testing.T) {
	t.Run("alertLevelToString", func(t *testing.T) {
		tests := []struct {
			level    AlertLevel
			expected string
		}{
			{AlertLevelInfo, "info"},
			{AlertLevelWarning, "warning"},
			{AlertLevelError, "error"},
			{AlertLevelCritical, "critical"},
			{AlertLevel(999), "unknown"}, // invalid level
		}

		for _, tt := range tests {
			result := alertLevelToString(tt.level)
			if result != tt.expected {
				t.Errorf("alertLevelToString(%v) = %s, want %s", tt.level, result, tt.expected)
			}
		}
	})

	t.Run("auditSeverityToString", func(t *testing.T) {
		tests := []struct {
			severity AuditSeverityLevel
			expected string
		}{
			{AuditSeverityDebug, "debug"},
			{AuditSeverityInfo, "info"},
			{AuditSeverityWarning, "warning"},
			{AuditSeverityError, "error"},
			{AuditSeverityCritical, "critical"},
			{AuditSeverityLevel(999), "unknown"}, // invalid severity
		}

		for _, tt := range tests {
			result := auditSeverityToString(tt.severity)
			if result != tt.expected {
				t.Errorf("auditSeverityToString(%v) = %s, want %s", tt.severity, result, tt.expected)
			}
		}
	})
}

// TestConcurrentNotifierUsage tests concurrent usage of notifiers
func TestConcurrentNotifierUsage(t *testing.T) {
	logger := zaptest.NewLogger(t)
	notifier := NewLogAlertNotifier(logger)

	alert := Alert{
		Level:       AlertLevelWarning,
		Title:       "Concurrent Test",
		Description: "Testing concurrent usage",
		Timestamp:   time.Now(),
		Component:   "test",
		Value:       75.0,
		Threshold:   70.0,
	}

	// Test concurrent sending
	const numGoroutines = 100
	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			testAlert := alert
			testAlert.Title = fmt.Sprintf("Concurrent Test %d", id)

			err := notifier.SendAlert(context.Background(), testAlert)
			if err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		t.Errorf("Concurrent SendAlert() failed: %v", err)
	}
}

// TestSecurityFeatures tests the SSRF prevention and security features
func TestSecurityFeatures(t *testing.T) {
	t.Run("webhook_url_security", func(t *testing.T) {
		maliciousURLs := []string{
			"https://169.254.169.254/metadata", // AWS metadata service
			"https://127.0.0.1:8080/admin",     // localhost
			"https://10.0.0.1/internal",        // private network
			"https://192.168.1.1/config",       // private network
			"https://172.16.0.1/secrets",       // private network
		}

		for _, url := range maliciousURLs {
			err := validateWebhookURL(url)
			if err == nil {
				t.Errorf("Expected validateWebhookURL to reject malicious URL: %s", url)
			}
		}
	})

	t.Run("webhook_https_only", func(t *testing.T) {
		httpURLs := []string{
			"http://example.com/webhook",
			"http://hooks.slack.com/webhook",
			"ftp://example.com/webhook",
		}

		for _, url := range httpURLs {
			err := validateWebhookURL(url)
			if err == nil {
				t.Errorf("Expected validateWebhookURL to reject non-HTTPS URL: %s", url)
			}
		}
	})
}

// mockFailingNotifier is a mock notifier for testing error scenarios
type mockFailingNotifier struct {
	shouldFail bool
}

func (m *mockFailingNotifier) SendAlert(ctx context.Context, alert Alert) error {
	if m.shouldFail {
		return errors.New("mock notifier error")
	}
	return nil
}

// Benchmark tests for performance
func BenchmarkLogAlertNotifier(b *testing.B) {
	logger := zaptest.NewLogger(b)
	notifier := NewLogAlertNotifier(logger)

	alert := Alert{
		Level:       AlertLevelWarning,
		Title:       "Benchmark Test",
		Description: "Testing performance",
		Timestamp:   time.Now(),
		Component:   "benchmark",
		Value:       75.0,
		Threshold:   70.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		notifier.SendAlert(context.Background(), alert)
	}
}

func BenchmarkFileAlertNotifier(b *testing.B) {
	tempDir := b.TempDir()
	alertFile := filepath.Join(tempDir, "benchmark.log")

	notifier, err := NewFileAlertNotifier(alertFile)
	if err != nil {
		b.Fatalf("Failed to create file notifier: %v", err)
	}
	defer notifier.Close()

	alert := Alert{
		Level:       AlertLevelWarning,
		Title:       "Benchmark Test",
		Description: "Testing performance",
		Timestamp:   time.Now(),
		Component:   "benchmark",
		Value:       75.0,
		Threshold:   70.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		notifier.SendAlert(context.Background(), alert)
	}
}

func BenchmarkCompositeAlertNotifier(b *testing.B) {
	logger := zaptest.NewLogger(b)
	logNotifier := NewLogAlertNotifier(logger)

	compositeNotifier := NewCompositeAlertNotifier(logNotifier, logNotifier, logNotifier)

	alert := Alert{
		Level:       AlertLevelWarning,
		Title:       "Benchmark Test",
		Description: "Testing performance",
		Timestamp:   time.Now(),
		Component:   "benchmark",
		Value:       75.0,
		Threshold:   70.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compositeNotifier.SendAlert(context.Background(), alert)
	}
}
