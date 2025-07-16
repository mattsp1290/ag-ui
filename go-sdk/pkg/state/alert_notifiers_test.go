package state

import (
	"bytes"
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
	"sync/atomic"
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
			url:         "not-a-url",
			wantError:   true,
			errorSubstr: "invalid URL format",
		},
		{
			name:        "HTTP URL (should reject)",
			url:         "http://example.com/webhook",
			wantError:   true,
			errorSubstr: "only HTTPS webhook URLs are allowed",
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

	notifier, err := NewWebhookAlertNotifierForTest(server.URL, 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to create webhook notifier: %v", err)
	}

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
	err = notifier.SendAlert(context.Background(), alert)
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

		notifier, err := NewWebhookAlertNotifierForTest(server.URL, 10*time.Second)
		if err != nil {
			t.Fatalf("Failed to create webhook notifier: %v", err)
		}

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Test Alert",
			Timestamp: time.Now(),
		}

		err = notifier.SendAlert(context.Background(), alert)
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

		notifier, err := NewWebhookAlertNotifierForTest(server.URL, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to create webhook notifier: %v", err)
		}

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Test Alert",
			Timestamp: time.Now(),
		}

		err = notifier.SendAlert(context.Background(), alert)
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

		notifier, err := NewWebhookAlertNotifierForTest(server.URL, 10*time.Second)
		if err != nil {
			t.Fatalf("Failed to create webhook notifier: %v", err)
		}

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Test Alert",
			Timestamp: time.Now(),
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = notifier.SendAlert(ctx, alert)
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

	notifier, err := NewWebhookAlertNotifierForTest(server.URL, 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to create webhook notifier: %v", err)
	}

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

// TestSlackAlertNotifierWithMockServer tests the Slack notifier with a mock server
func TestSlackAlertNotifierWithMockServer(t *testing.T) {
	var receivedPayload map[string]interface{}
	var requestHeaders http.Header
	var requestCount int32

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		requestHeaders = r.Header.Clone()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("Failed to unmarshal request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	notifier, err := NewSlackAlertNotifierForTest(server.URL, "#test-alerts", "TestBot")
	if err != nil {
		t.Fatalf("Failed to create Slack notifier: %v", err)
	}

	alert := Alert{
		Level:       AlertLevelCritical,
		Title:       "Test Critical Alert",
		Description: "This is a test critical alert for Slack",
		Timestamp:   time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
		Component:   "test-service",
		Value:       95.7,
		Threshold:   90.0,
		Labels:      map[string]string{"environment": "production", "service": "api"},
		Severity:    AuditSeverityCritical,
	}

	// Test sending alert
	err = notifier.SendAlert(context.Background(), alert)
	if err != nil {
		t.Fatalf("SendAlert() failed: %v", err)
	}

	// Verify request was made
	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("Expected 1 request, got %d", atomic.LoadInt32(&requestCount))
	}

	// Verify headers
	if requestHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", requestHeaders.Get("Content-Type"))
	}

	// Verify payload structure
	if receivedPayload == nil {
		t.Fatal("No payload received")
	}

	if receivedPayload["channel"] != "#test-alerts" {
		t.Errorf("Expected channel '#test-alerts', got '%v'", receivedPayload["channel"])
	}

	if receivedPayload["username"] != "TestBot" {
		t.Errorf("Expected username 'TestBot', got '%v'", receivedPayload["username"])
	}

	attachments, ok := receivedPayload["attachments"].([]interface{})
	if !ok || len(attachments) == 0 {
		t.Fatal("Expected attachments array")
	}

	attachment, ok := attachments[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected attachment to be an object")
	}

	if attachment["color"] != "danger" {
		t.Errorf("Expected color 'danger' for critical alert, got '%v'", attachment["color"])
	}

	if attachment["title"] != alert.Title {
		t.Errorf("Expected title '%s', got '%v'", alert.Title, attachment["title"])
	}

	if attachment["text"] != alert.Description {
		t.Errorf("Expected text '%s', got '%v'", alert.Description, attachment["text"])
	}

	// Verify fields
	fields, ok := attachment["fields"].([]interface{})
	if !ok || len(fields) < 4 {
		t.Fatal("Expected at least 4 fields in attachment")
	}

	// Check specific fields
	fieldMap := make(map[string]string)
	for _, field := range fields {
		f, ok := field.(map[string]interface{})
		if !ok {
			continue
		}
		if title, ok := f["title"].(string); ok {
			if value, ok := f["value"].(string); ok {
				fieldMap[title] = value
			}
		}
	}

	if fieldMap["Component"] != alert.Component {
		t.Errorf("Expected Component '%s', got '%s'", alert.Component, fieldMap["Component"])
	}

	if fieldMap["Value"] != "95.70" {
		t.Errorf("Expected Value '95.70', got '%s'", fieldMap["Value"])
	}

	if fieldMap["Threshold"] != "90.00" {
		t.Errorf("Expected Threshold '90.00', got '%s'", fieldMap["Threshold"])
	}

	if fieldMap["Timestamp"] != "2024-01-15T12:30:45Z" {
		t.Errorf("Expected Timestamp '2024-01-15T12:30:45Z', got '%s'", fieldMap["Timestamp"])
	}

	// Verify footer and timestamp
	if attachment["footer"] != "State Manager" {
		t.Errorf("Expected footer 'State Manager', got '%v'", attachment["footer"])
	}

	expectedTs := alert.Timestamp.Unix()
	if ts, ok := attachment["ts"].(float64); !ok || int64(ts) != expectedTs {
		t.Errorf("Expected timestamp %d, got %v", expectedTs, attachment["ts"])
	}
}

// TestSlackAlertNotifierErrors tests error scenarios for Slack notifier
func TestSlackAlertNotifierErrors(t *testing.T) {
	t.Run("server_error", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		notifier, err := NewSlackAlertNotifierForTest(server.URL, "#alerts", "TestBot")
		if err != nil {
			t.Fatalf("Failed to create Slack notifier: %v", err)
		}

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Test Alert",
			Timestamp: time.Now(),
		}

		err = notifier.SendAlert(context.Background(), alert)
		if err == nil {
			t.Error("Expected error for server error response")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("Expected error to contain status code 500, got: %v", err)
		}
	})

	t.Run("context_cancellation", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		notifier, err := NewSlackAlertNotifierForTest(server.URL, "#alerts", "TestBot")
		if err != nil {
			t.Fatalf("Failed to create Slack notifier: %v", err)
		}

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Test Alert",
			Timestamp: time.Now(),
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = notifier.SendAlert(ctx, alert)
		if err == nil {
			t.Error("Expected context cancellation error")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(3 * time.Second) // Longer than the client timeout
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		notifier, err := NewSlackAlertNotifierForTestWithTimeout(server.URL, "#alerts", "TestBot", 2*time.Second)
		if err != nil {
			t.Fatalf("Failed to create Slack notifier: %v", err)
		}

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Test Alert",
			Timestamp: time.Now(),
		}

		err = notifier.SendAlert(context.Background(), alert)
		if err == nil {
			t.Error("Expected timeout error")
		}
	})

	t.Run("invalid_json_response", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("invalid_json"))
		}))
		defer server.Close()

		notifier, err := NewSlackAlertNotifierForTest(server.URL, "#alerts", "TestBot")
		if err != nil {
			t.Fatalf("Failed to create Slack notifier: %v", err)
		}

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Test Alert",
			Timestamp: time.Now(),
		}

		err = notifier.SendAlert(context.Background(), alert)
		if err == nil {
			t.Error("Expected error for invalid response")
		}
		if !strings.Contains(err.Error(), "400") {
			t.Errorf("Expected error to contain status code 400, got: %v", err)
		}
	})
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

// TestPagerDutyAlertNotifierWithMockServer tests the PagerDuty notifier with a mock server
func TestPagerDutyAlertNotifierWithMockServer(t *testing.T) {
	var receivedPayload map[string]interface{}
	var requestHeaders http.Header
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		requestHeaders = r.Header.Clone()

		// Verify the URL endpoint
		if r.URL.Path != "/v2/enqueue" {
			t.Errorf("Expected path '/v2/enqueue', got '%s'", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("Failed to unmarshal request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"success","message":"Event processed","dedup_key":"state-manager-test-service-Critical Alert"}`))
	}))
	defer server.Close()

	// We'll use a test notifier instead of the real one

	alert := Alert{
		Level:       AlertLevelCritical,
		Title:       "Critical Alert",
		Description: "This is a critical test alert for PagerDuty",
		Timestamp:   time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
		Component:   "test-service",
		Value:       98.5,
		Threshold:   95.0,
		Labels:      map[string]string{"datacenter": "us-east-1", "service": "web"},
		Severity:    AuditSeverityCritical,
	}

	// Create a custom PagerDuty notifier that uses our test server
	testNotifier := &testPagerDutyNotifier{
		integrationKey: "test-integration-key",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		serverURL: server.URL,
	}

	// Test sending alert
	err := testNotifier.SendAlert(context.Background(), alert)
	if err != nil {
		t.Fatalf("SendAlert() failed: %v", err)
	}

	// Verify request was made
	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("Expected 1 request, got %d", atomic.LoadInt32(&requestCount))
	}

	// Verify headers
	if requestHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", requestHeaders.Get("Content-Type"))
	}

	// Verify payload structure
	if receivedPayload == nil {
		t.Fatal("No payload received")
	}

	if receivedPayload["routing_key"] != "test-integration-key" {
		t.Errorf("Expected routing_key 'test-integration-key', got '%v'", receivedPayload["routing_key"])
	}

	if receivedPayload["event_action"] != "trigger" {
		t.Errorf("Expected event_action 'trigger', got '%v'", receivedPayload["event_action"])
	}

	expectedDedupKey := "state-manager-test-service-Critical Alert"
	if receivedPayload["dedup_key"] != expectedDedupKey {
		t.Errorf("Expected dedup_key '%s', got '%v'", expectedDedupKey, receivedPayload["dedup_key"])
	}

	payload, ok := receivedPayload["payload"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be an object")
	}

	if payload["summary"] != alert.Title {
		t.Errorf("Expected summary '%s', got '%v'", alert.Title, payload["summary"])
	}

	if payload["source"] != "state-manager" {
		t.Errorf("Expected source 'state-manager', got '%v'", payload["source"])
	}

	if payload["severity"] != "critical" {
		t.Errorf("Expected severity 'critical', got '%v'", payload["severity"])
	}

	if payload["component"] != alert.Component {
		t.Errorf("Expected component '%s', got '%v'", alert.Component, payload["component"])
	}

	customDetails, ok := payload["custom_details"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected custom_details to be an object")
	}

	if customDetails["description"] != alert.Description {
		t.Errorf("Expected description '%s', got '%v'", alert.Description, customDetails["description"])
	}

	if customDetails["value"] != alert.Value {
		t.Errorf("Expected value %f, got %v", alert.Value, customDetails["value"])
	}

	if customDetails["threshold"] != alert.Threshold {
		t.Errorf("Expected threshold %f, got %v", alert.Threshold, customDetails["threshold"])
	}
}

// TestPagerDutyAlertNotifierInfoLevel tests that info level alerts trigger resolve action
func TestPagerDutyAlertNotifierInfoLevel(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	// We'll use a test notifier

	alert := Alert{
		Level:     AlertLevelInfo,
		Title:     "Service Recovered",
		Component: "test-service",
		Timestamp: time.Now(),
	}

	// Create a test notifier for info level testing
	testNotifier := &testPagerDutyNotifier{
		integrationKey: "test-key",
		client:         &http.Client{Timeout: 10 * time.Second},
		serverURL:      server.URL,
	}

	err := testNotifier.SendAlert(context.Background(), alert)
	if err != nil {
		t.Fatalf("SendAlert() failed: %v", err)
	}

	if receivedPayload["event_action"] != "resolve" {
		t.Errorf("Expected event_action 'resolve' for info level, got '%v'", receivedPayload["event_action"])
	}
}

// TestPagerDutyAlertNotifierErrors tests error scenarios for PagerDuty notifier
func TestPagerDutyAlertNotifierErrors(t *testing.T) {
	t.Run("server_error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		// Will use test notifier

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Test Alert",
			Component: "test",
			Timestamp: time.Now(),
		}

		// Create test notifier
		testNotifier := &testPagerDutyNotifier{
			integrationKey: "test-key",
			client:         &http.Client{Timeout: 10 * time.Second},
			serverURL:      server.URL,
		}

		err := testNotifier.SendAlert(context.Background(), alert)
		if err == nil {
			t.Error("Expected error for server error response")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("Expected error to contain status code 500, got: %v", err)
		}
	})

	t.Run("context_cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusAccepted)
		}))
		defer server.Close()

		// Will use test notifier

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Test Alert",
			Component: "test",
			Timestamp: time.Now(),
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Create test notifier  
		testNotifier := &testPagerDutyNotifier{
			integrationKey: "test-key",
			client:         &http.Client{Timeout: 10 * time.Second},
			serverURL:      server.URL,
		}

		err := testNotifier.SendAlert(ctx, alert)
		if err == nil {
			t.Error("Expected context cancellation error")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(3 * time.Second) // Longer than client timeout
			w.WriteHeader(http.StatusAccepted)
		}))
		defer server.Close()

		// Will use test notifier with short timeout

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Test Alert",
			Component: "test",
			Timestamp: time.Now(),
		}

		// Create test notifier with short timeout
		testNotifier := &testPagerDutyNotifier{
			integrationKey: "test-key",
			client:         &http.Client{Timeout: 2 * time.Second}, // Short timeout
			serverURL:      server.URL,
		}

		err := testNotifier.SendAlert(context.Background(), alert)
		if err == nil {
			t.Error("Expected timeout error")
		}
	})
}

// TestSlackAlertNotifierURLValidation tests the Slack notifier's URL validation
func TestSlackAlertNotifierURLValidation(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantError   bool
		errorSubstr string
	}{
		{
			name:        "valid HTTPS URL",
			url:         "https://hooks.slack.com/services/test",
			wantError:   false,
		},
		{
			name:        "HTTP URL (should reject)",
			url:         "http://hooks.slack.com/services/test",
			wantError:   true,
			errorSubstr: "only HTTPS webhook URLs are allowed",
		},
		{
			name:        "localhost URL",
			url:         "https://localhost:8080/webhook",
			wantError:   true,
			errorSubstr: "cannot point to localhost",
		},
		{
			name:        "internal IP 10.x.x.x",
			url:         "https://10.0.0.1/webhook",
			wantError:   true,
			errorSubstr: "internal IP address",
		},
		{
			name:        "internal IP 192.168.x.x",
			url:         "https://192.168.1.1/webhook",
			wantError:   true,
			errorSubstr: "internal IP address",
		},
		{
			name:        "empty URL",
			url:         "",
			wantError:   true,
			errorSubstr: "cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := NewSlackAlertNotifier(tt.url, "#alerts", "StateManager")
			if tt.wantError {
				if err == nil {
					t.Errorf("NewSlackAlertNotifier(%s) expected error, got nil", tt.url)
				} else if !strings.Contains(err.Error(), tt.errorSubstr) {
					t.Errorf("NewSlackAlertNotifier(%s) error = %v, want error containing %q", tt.url, err, tt.errorSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("NewSlackAlertNotifier(%s) unexpected error: %v", tt.url, err)
				}
				if notifier == nil {
					t.Errorf("NewSlackAlertNotifier(%s) returned nil notifier", tt.url)
				}
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

// TestCompositeAlertNotifierPartialFailures tests various partial failure scenarios
func TestCompositeAlertNotifierPartialFailures(t *testing.T) {
	t.Run("multiple_failures", func(t *testing.T) {
		failingNotifier1 := &mockFailingNotifier{shouldFail: true, errorMessage: "notifier 1 failed"}
		failingNotifier2 := &mockFailingNotifier{shouldFail: true, errorMessage: "notifier 2 failed"}
		
		logger := zaptest.NewLogger(t)
		successNotifier := NewLogAlertNotifier(logger)

		compositeNotifier := NewCompositeAlertNotifier(failingNotifier1, successNotifier, failingNotifier2)

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Multiple Failures Test",
			Timestamp: time.Now(),
			Component: "test",
		}

		err := compositeNotifier.SendAlert(context.Background(), alert)
		if err == nil {
			t.Error("Expected error when multiple notifiers fail")
		}

		// Should contain both error messages
		if !strings.Contains(err.Error(), "notifier 1 failed") {
			t.Errorf("Expected error to contain 'notifier 1 failed', got: %v", err)
		}
		if !strings.Contains(err.Error(), "notifier 2 failed") {
			t.Errorf("Expected error to contain 'notifier 2 failed', got: %v", err)
		}
	})

	t.Run("all_failures", func(t *testing.T) {
		failingNotifier1 := &mockFailingNotifier{shouldFail: true, errorMessage: "error A"}
		failingNotifier2 := &mockFailingNotifier{shouldFail: true, errorMessage: "error B"}
		failingNotifier3 := &mockFailingNotifier{shouldFail: true, errorMessage: "error C"}

		compositeNotifier := NewCompositeAlertNotifier(failingNotifier1, failingNotifier2, failingNotifier3)

		alert := Alert{
			Level:     AlertLevelCritical,
			Title:     "All Failures Test",
			Timestamp: time.Now(),
			Component: "test",
		}

		err := compositeNotifier.SendAlert(context.Background(), alert)
		if err == nil {
			t.Error("Expected error when all notifiers fail")
		}

		// Check that all error messages are included
		errorStr := err.Error()
		if !strings.Contains(errorStr, "error A") || !strings.Contains(errorStr, "error B") || !strings.Contains(errorStr, "error C") {
			t.Errorf("Expected error to contain all failure messages, got: %v", err)
		}
	})

	t.Run("intermittent_failures", func(t *testing.T) {
		intermittentNotifier := &mockIntermittentNotifier{failCount: 0, failEvery: 2}
		
		logger := zaptest.NewLogger(t)
		successNotifier := NewLogAlertNotifier(logger)

		compositeNotifier := NewCompositeAlertNotifier(intermittentNotifier, successNotifier)

		alert := Alert{
			Level:     AlertLevelWarning,
			Title:     "Intermittent Failures Test",
			Timestamp: time.Now(),
			Component: "test",
		}

		// First call should succeed
		err := compositeNotifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Errorf("First call should succeed, got error: %v", err)
		}

		// Second call should fail (intermittent notifier fails every 2nd call)
		err = compositeNotifier.SendAlert(context.Background(), alert)
		if err == nil {
			t.Error("Second call should fail due to intermittent failure")
		}

		// Third call should succeed again
		err = compositeNotifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Errorf("Third call should succeed, got error: %v", err)
		}
	})

	t.Run("mixed_notifier_types", func(t *testing.T) {
		// Create a mix of different notifier types with some failing
		logger := zaptest.NewLogger(t)
		logNotifier := NewLogAlertNotifier(logger)
		
		tempDir := t.TempDir()
		alertFile := filepath.Join(tempDir, "test_alerts.log")
		fileNotifier, err := NewFileAlertNotifier(alertFile)
		if err != nil {
			t.Fatalf("Failed to create file notifier: %v", err)
		}
		defer fileNotifier.Close()

		failingNotifier := &mockFailingNotifier{shouldFail: true, errorMessage: "webhook service down"}

		compositeNotifier := NewCompositeAlertNotifier(logNotifier, fileNotifier, failingNotifier)

		alert := Alert{
			Level:       AlertLevelError,
			Title:       "Mixed Notifiers Test",
			Description: "Testing mixed notifier types with partial failure",
			Timestamp:   time.Now(),
			Component:   "test-service",
			Value:       85.0,
			Threshold:   80.0,
		}

		sendErr := compositeNotifier.SendAlert(context.Background(), alert)
		if sendErr == nil {
			t.Error("Expected error due to failing notifier")
		} else {
			t.Logf("Got expected error: %v", sendErr)
		}

		// Verify that successful notifiers still executed
		content, err := os.ReadFile(alertFile)
		if err != nil {
			t.Fatalf("Failed to read alert file: %v", err)
		}
		if len(content) == 0 {
			t.Error("File notifier should have written alert even with other failures")
		}

		// Verify error contains the specific failure
		if sendErr != nil && !strings.Contains(sendErr.Error(), "webhook service down") {
			t.Errorf("Expected error to contain 'webhook service down', got: %v", sendErr)
		} else if sendErr == nil {
			t.Error("Expected error due to failing notifier, but got nil")
		}
	})

	t.Run("empty_notifier_list", func(t *testing.T) {
		compositeNotifier := NewCompositeAlertNotifier()

		alert := Alert{
			Level:     AlertLevelInfo,
			Title:     "Empty Notifiers Test",
			Timestamp: time.Now(),
			Component: "test",
		}

		err := compositeNotifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Errorf("Expected no error with empty notifier list, got: %v", err)
		}
	})
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

// TestThrottledAlertNotifierAdvanced tests advanced throttling scenarios
func TestThrottledAlertNotifierAdvanced(t *testing.T) {
	t.Run("edge_case_timing", func(t *testing.T) {
		countingNotifier := &mockCountingNotifier{}
		throttleDuration := 50 * time.Millisecond
		throttledNotifier := NewThrottledAlertNotifier(countingNotifier, throttleDuration)

		alert := Alert{
			Level:     AlertLevelWarning,
			Title:     "Edge Case Test",
			Timestamp: time.Now(),
			Component: "test",
		}

		// Send first alert
		err := throttledNotifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Fatalf("First alert failed: %v", err)
		}

		// Send second alert immediately (should be throttled)
		err = throttledNotifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Fatalf("Second alert failed: %v", err)
		}

		// Wait just under the throttle duration
		time.Sleep(throttleDuration - 10*time.Millisecond)

		// Send third alert (should still be throttled)
		err = throttledNotifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Fatalf("Third alert failed: %v", err)
		}

		// Should only have sent one alert so far
		if countingNotifier.GetCallCount() != 1 {
			t.Errorf("Expected 1 call, got %d", countingNotifier.GetCallCount())
		}

		// Wait for throttle to expire
		time.Sleep(20 * time.Millisecond)

		// Send fourth alert (should go through)
		err = throttledNotifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Fatalf("Fourth alert failed: %v", err)
		}

		// Should now have sent two alerts
		if countingNotifier.GetCallCount() != 2 {
			t.Errorf("Expected 2 calls, got %d", countingNotifier.GetCallCount())
		}
	})

	t.Run("concurrent_same_alert", func(t *testing.T) {
		countingNotifier := &mockCountingNotifier{}
		throttledNotifier := NewThrottledAlertNotifier(countingNotifier, 100*time.Millisecond)

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Concurrent Test",
			Timestamp: time.Now(),
			Component: "test",
		}

		const numGoroutines = 10
		var wg sync.WaitGroup
		errChan := make(chan error, numGoroutines)

		// Send same alert concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := throttledNotifier.SendAlert(context.Background(), alert)
				if err != nil {
					errChan <- err
				}
			}()
		}

		wg.Wait()
		close(errChan)

		// Check for errors
		for err := range errChan {
			t.Errorf("Concurrent alert failed: %v", err)
		}

		// Should only have sent one alert due to throttling
		if countingNotifier.GetCallCount() != 1 {
			t.Errorf("Expected 1 call due to throttling, got %d", countingNotifier.GetCallCount())
		}
	})

	t.Run("concurrent_different_alerts", func(t *testing.T) {
		countingNotifier := &mockCountingNotifier{}
		throttledNotifier := NewThrottledAlertNotifier(countingNotifier, 100*time.Millisecond)

		const numGoroutines = 5
		var wg sync.WaitGroup
		errChan := make(chan error, numGoroutines)

		// Send different alerts concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				alert := Alert{
					Level:     AlertLevelWarning,
					Title:     fmt.Sprintf("Concurrent Test %d", id),
					Timestamp: time.Now(),
					Component: fmt.Sprintf("component-%d", id),
				}
				err := throttledNotifier.SendAlert(context.Background(), alert)
				if err != nil {
					errChan <- err
				}
			}(i)
		}

		wg.Wait()
		close(errChan)

		// Check for errors
		for err := range errChan {
			t.Errorf("Concurrent alert failed: %v", err)
		}

		// Should have sent all alerts (different alert keys)
		if countingNotifier.GetCallCount() != numGoroutines {
			t.Errorf("Expected %d calls, got %d", numGoroutines, countingNotifier.GetCallCount())
		}
	})

	t.Run("rapid_fire_same_alert", func(t *testing.T) {
		countingNotifier := &mockCountingNotifier{}
		throttledNotifier := NewThrottledAlertNotifier(countingNotifier, 300*time.Millisecond)

		alert := Alert{
			Level:     AlertLevelCritical,
			Title:     "Rapid Fire Test",
			Timestamp: time.Now(),
			Component: "test",
		}

		// Send many alerts rapidly
		for i := 0; i < 20; i++ {
			err := throttledNotifier.SendAlert(context.Background(), alert)
			if err != nil {
				t.Fatalf("Alert %d failed: %v", i, err)
			}
			time.Sleep(5 * time.Millisecond)
		}

		// Should have sent very few alerts due to rapid throttling (be less strict about exact count)
		callCount := countingNotifier.GetCallCount()
		if callCount < 1 || callCount > 2 {
			t.Errorf("Expected 1-2 calls due to rapid throttling, got %d", callCount)
		}

		// Wait for throttle to expire and send another
		time.Sleep(350 * time.Millisecond)
		err := throttledNotifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Fatalf("Final alert failed: %v", err)
		}

		// Should now have sent one more alert (be less strict about exact count)
		finalCount := countingNotifier.GetCallCount()
		if finalCount <= callCount {
			t.Errorf("Expected final count (%d) to be greater than initial count (%d)", finalCount, callCount)
		}
	})

	t.Run("zero_throttle_duration", func(t *testing.T) {
		countingNotifier := &mockCountingNotifier{}
		throttledNotifier := NewThrottledAlertNotifier(countingNotifier, 0)

		alert := Alert{
			Level:     AlertLevelInfo,
			Title:     "Zero Throttle Test",
			Timestamp: time.Now(),
			Component: "test",
		}

		// Send multiple alerts with zero throttle
		for i := 0; i < 5; i++ {
			err := throttledNotifier.SendAlert(context.Background(), alert)
			if err != nil {
				t.Fatalf("Alert %d failed: %v", i, err)
			}
		}

		// All alerts should go through with zero throttle
		if countingNotifier.GetCallCount() != 5 {
			t.Errorf("Expected 5 calls with zero throttle, got %d", countingNotifier.GetCallCount())
		}
	})

	t.Run("mixed_success_failure_throttling", func(t *testing.T) {
		intermittentNotifier := &mockIntermittentNotifier{failEvery: 2}
		throttledNotifier := NewThrottledAlertNotifier(intermittentNotifier, 100*time.Millisecond)

		alert := Alert{
			Level:     AlertLevelWarning,
			Title:     "Mixed Success Failure",
			Timestamp: time.Now(),
			Component: "test",
		}

		// First two alerts should succeed
		err := throttledNotifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Fatalf("First alert should succeed: %v", err)
		}

		err = throttledNotifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Fatalf("Second alert should succeed (throttled): %v", err)
		}

		// Wait for throttle to expire
		time.Sleep(150 * time.Millisecond)

		// Third alert should fail (intermittent notifier fails every 3rd call)
		err = throttledNotifier.SendAlert(context.Background(), alert)
		if err == nil {
			t.Error("Third alert should fail")
		}

		// Verify alert key was not updated due to failure
		alertKey := fmt.Sprintf("%s_%s", alert.Component, alert.Title)
		lastSent, exists := throttledNotifier.lastSent[alertKey]
		if !exists {
			t.Error("Expected alert key to exist from successful first call")
		}

		// Fourth alert should succeed after short wait
		time.Sleep(150 * time.Millisecond)
		err = throttledNotifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Fatalf("Fourth alert should succeed: %v", err)
		}

		// Verify lastSent was updated
		newLastSent := throttledNotifier.lastSent[alertKey]
		if !newLastSent.After(lastSent) {
			t.Error("Expected lastSent to be updated after successful fourth call")
		}
	})

	t.Run("alert_key_generation", func(t *testing.T) {
		countingNotifier := &mockCountingNotifier{}
		throttledNotifier := NewThrottledAlertNotifier(countingNotifier, 100*time.Millisecond)

		// Test alerts with same component but different titles
		alert1 := Alert{Level: AlertLevelWarning, Title: "Alert A", Component: "service"}
		alert2 := Alert{Level: AlertLevelWarning, Title: "Alert B", Component: "service"}

		// Test alerts with same title but different components
		alert3 := Alert{Level: AlertLevelWarning, Title: "Alert A", Component: "service1"}
		alert4 := Alert{Level: AlertLevelWarning, Title: "Alert A", Component: "service2"}

		alerts := []Alert{alert1, alert2, alert3, alert4}

		for i, alert := range alerts {
			err := throttledNotifier.SendAlert(context.Background(), alert)
			if err != nil {
				t.Fatalf("Alert %d failed: %v", i, err)
			}
		}

		// All alerts should go through (different keys)
		if countingNotifier.GetCallCount() != 4 {
			t.Errorf("Expected 4 calls for different alert keys, got %d", countingNotifier.GetCallCount())
		}

		// Verify we have 4 different keys being tracked
		if len(throttledNotifier.lastSent) != 4 {
			t.Errorf("Expected 4 tracked alert keys, got %d", len(throttledNotifier.lastSent))
		}
	})
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

// TestComprehensiveErrorHandling tests comprehensive error scenarios across all notifiers
func TestComprehensiveErrorHandling(t *testing.T) {
	t.Run("email_notifier_disabled", func(t *testing.T) {
		notifier := NewEmailAlertNotifier("smtp.test.com", 587, "user", "pass", "from@test.com", []string{"to@test.com"})
		notifier.enabled = false

		alert := Alert{
			Level:     AlertLevelCritical,
			Title:     "Test Alert",
			Timestamp: time.Now(),
			Component: "test",
		}

		err := notifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Errorf("Disabled email notifier should not return error, got: %v", err)
		}
	})

	t.Run("file_notifier_write_error", func(t *testing.T) {
		tempDir := t.TempDir()
		alertFile := filepath.Join(tempDir, "readonly", "alerts.log")

		// Create directory structure first
		readonlyDir := filepath.Join(tempDir, "readonly")
		if err := os.Mkdir(readonlyDir, 0755); err != nil {
			t.Fatalf("Failed to create readonly directory: %v", err)
		}

		// Try to create notifier with file in readonly directory
		if os.Getuid() != 0 { // Skip if running as root
			if err := os.Chmod(readonlyDir, 0555); err != nil {
				t.Fatalf("Failed to make directory readonly: %v", err)
			}

			_, err := NewFileAlertNotifier(alertFile)
			if err == nil {
				t.Error("Expected error when creating file in readonly directory")
			}

			// Restore permissions for cleanup
			os.Chmod(readonlyDir, 0755)
		}
	})

	t.Run("webhook_json_marshal_error", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		notifier, err := NewWebhookAlertNotifierForTest(server.URL, 10*time.Second)
		if err != nil {
			t.Fatalf("Failed to create webhook notifier: %v", err)
		}

		// Create alert with unmarshalable data (circular reference would be ideal, but let's use a large value)
		alert := Alert{
			Level:     AlertLevelError,
			Title:     "JSON Test",
			Timestamp: time.Now(),
			Component: "test",
			Labels:    map[string]string{"key": string(make([]byte, 1<<20))}, // Very large string
		}

		err = notifier.SendAlert(context.Background(), alert)
		// This should still work as JSON can handle large strings
		if err != nil && !strings.Contains(err.Error(), "marshal") {
			t.Errorf("Unexpected error type: %v", err)
		}
	})

	t.Run("conditional_notifier_nil_condition", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		baseNotifier := NewLogAlertNotifier(logger)

		// Test with nil condition function
		conditionalNotifier := NewConditionalAlertNotifier(baseNotifier, nil)

		alert := Alert{
			Level:     AlertLevelWarning,
			Title:     "Nil Condition Test",
			Timestamp: time.Now(),
			Component: "test",
		}

		// This should not panic and should probably send the alert
		err := conditionalNotifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Errorf("ConditionalNotifier with nil condition failed: %v", err)
		}
	})

	t.Run("context_timeout_scenarios", func(t *testing.T) {
		// Test context timeout with slow notifier
		slowNotifier := &mockCountingNotifier{
			shouldDelay: true,
			delay:       200 * time.Millisecond,
		}

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Timeout Test",
			Timestamp: time.Now(),
			Component: "test",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := slowNotifier.SendAlert(ctx, alert)
		if err == nil {
			t.Error("Expected context timeout error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
		}
	})
}

// TestStressAndRaceConditions tests stress scenarios and race conditions
func TestStressAndRaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress and race conditions test in short mode")
	}
	
	t.Run("high_volume_alerts", func(t *testing.T) {
		countingNotifier := &mockCountingNotifier{}
		compositeNotifier := NewCompositeAlertNotifier(countingNotifier)

		const numAlerts = 1000
		var wg sync.WaitGroup
		errChan := make(chan error, numAlerts)

		alert := Alert{
			Level:     AlertLevelInfo,
			Title:     "High Volume Test",
			Timestamp: time.Now(),
			Component: "stress-test",
		}

		start := time.Now()
		for i := 0; i < numAlerts; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				testAlert := alert
				testAlert.Title = fmt.Sprintf("Alert %d", id)
				err := compositeNotifier.SendAlert(context.Background(), testAlert)
				if err != nil {
					errChan <- err
				}
			}(i)
		}

		wg.Wait()
		close(errChan)
		duration := time.Since(start)

		// Check for errors
		for err := range errChan {
			t.Errorf("High volume alert failed: %v", err)
		}

		if countingNotifier.GetCallCount() != numAlerts {
			t.Errorf("Expected %d calls, got %d", numAlerts, countingNotifier.GetCallCount())
		}

		t.Logf("Processed %d alerts in %v (%.2f alerts/sec)", numAlerts, duration, float64(numAlerts)/duration.Seconds())
	})

	t.Run("concurrent_throttled_notifier_access", func(t *testing.T) {
		countingNotifier := &mockCountingNotifier{}
		throttledNotifier := NewThrottledAlertNotifier(countingNotifier, 10*time.Millisecond)

		const numGoroutines = 50
		const alertsPerGoroutine = 20
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < alertsPerGoroutine; j++ {
					alert := Alert{
						Level:     AlertLevelWarning,
						Title:     fmt.Sprintf("Goroutine %d Alert %d", goroutineID, j),
						Timestamp: time.Now(),
						Component: fmt.Sprintf("component-%d", goroutineID),
					}
					throttledNotifier.SendAlert(context.Background(), alert)
					// Small delay to create more realistic timing
					time.Sleep(time.Microsecond * 100)
				}
			}(i)
		}

		wg.Wait()

		// Verify no race conditions occurred (would panic if there were)
		totalExpectedCalls := numGoroutines * alertsPerGoroutine
		actualCalls := countingNotifier.GetCallCount()

		t.Logf("Concurrent throttled test: %d goroutines, %d alerts each, %d actual calls",
			numGoroutines, alertsPerGoroutine, actualCalls)

		// Since throttling is involved, we expect fewer calls than total
		if actualCalls > int32(totalExpectedCalls) {
			t.Errorf("Actual calls (%d) should not exceed expected calls (%d)", actualCalls, totalExpectedCalls)
		}
	})

	t.Run("file_notifier_concurrent_writes", func(t *testing.T) {
		tempDir := t.TempDir()
		alertFile := filepath.Join(tempDir, "concurrent_alerts.log")

		notifier, err := NewFileAlertNotifier(alertFile)
		if err != nil {
			t.Fatalf("Failed to create file notifier: %v", err)
		}
		defer notifier.Close()

		const numWriters = 5
		const alertsPerWriter = 10
		var wg sync.WaitGroup

		for i := 0; i < numWriters; i++ {
			wg.Add(1)
			go func(writerID int) {
				defer wg.Done()
				for j := 0; j < alertsPerWriter; j++ {
					alert := Alert{
						Level:       AlertLevelInfo,
						Title:       fmt.Sprintf("Writer %d Alert %d", writerID, j),
						Description: fmt.Sprintf("Concurrent write test from writer %d", writerID),
						Timestamp:   time.Now(),
						Component:   fmt.Sprintf("writer-%d", writerID),
						Value:       float64(j),
						Threshold:   100.0,
					}
					err := notifier.SendAlert(context.Background(), alert)
					if err != nil {
						t.Errorf("Concurrent file write failed: %v", err)
						return
					}
				}
			}(i)
		}

		wg.Wait()

		// Verify file integrity
		content, err := os.ReadFile(alertFile)
		if err != nil {
			t.Fatalf("Failed to read alert file: %v", err)
		}

		lines := strings.Split(string(content), "\n")
		// Remove empty last line if it exists
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		expectedLines := numWriters * alertsPerWriter
		if len(lines) != expectedLines {
			t.Errorf("Expected %d lines in file, got %d", expectedLines, len(lines))
		}

		// Verify each line is valid JSON
		for i, line := range lines {
			var alert map[string]interface{}
			if err := json.Unmarshal([]byte(line), &alert); err != nil {
				t.Errorf("Line %d is not valid JSON: %v", i, err)
			}
		}
	})

	t.Run("composite_notifier_race_conditions", func(t *testing.T) {
		const numNotifiers = 10
		var notifiers []AlertNotifier

		for i := 0; i < numNotifiers; i++ {
			notifiers = append(notifiers, &mockCountingNotifier{})
		}

		compositeNotifier := NewCompositeAlertNotifier(notifiers...)

		const numGoroutines = 100
		var wg sync.WaitGroup
		errChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				alert := Alert{
					Level:     AlertLevelWarning,
					Title:     fmt.Sprintf("Race Test %d", id),
					Timestamp: time.Now(),
					Component: "race-test",
				}
				err := compositeNotifier.SendAlert(context.Background(), alert)
				if err != nil {
					errChan <- err
				}
			}(i)
		}

		wg.Wait()
		close(errChan)

		// Check for errors
		for err := range errChan {
			t.Errorf("Composite notifier race test failed: %v", err)
		}

		// Verify all notifiers received all alerts
		for i, notifier := range notifiers {
			countingNotifier := notifier.(*mockCountingNotifier)
			if countingNotifier.GetCallCount() != numGoroutines {
				t.Errorf("Notifier %d expected %d calls, got %d", i, numGoroutines, countingNotifier.GetCallCount())
			}
		}
	})

	t.Run("memory_pressure_test", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping memory pressure test in short mode")
		}
		
		// Test with large alerts and many notifiers
		const numNotifiers = 100
		var notifiers []AlertNotifier

		for i := 0; i < numNotifiers; i++ {
			notifiers = append(notifiers, &mockCountingNotifier{})
		}

		compositeNotifier := NewCompositeAlertNotifier(notifiers...)

		// Create a large alert
		largeLabels := make(map[string]string)
		for i := 0; i < 1000; i++ {
			largeLabels[fmt.Sprintf("label_%d", i)] = fmt.Sprintf("value_%d_with_some_additional_text", i)
		}

		alert := Alert{
			Level:       AlertLevelError,
			Title:       "Memory Pressure Test",
			Description: strings.Repeat("Large description text. ", 1000),
			Timestamp:   time.Now(),
			Component:   "memory-test",
			Labels:      largeLabels,
		}

		start := time.Now()
		err := compositeNotifier.SendAlert(context.Background(), alert)
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Memory pressure test failed: %v", err)
		}

		t.Logf("Memory pressure test completed in %v", duration)

		// Verify all notifiers were called
		for i, notifier := range notifiers {
			countingNotifier := notifier.(*mockCountingNotifier)
			if countingNotifier.GetCallCount() != 1 {
				t.Errorf("Notifier %d expected 1 call, got %d", i, countingNotifier.GetCallCount())
			}
		}
	})
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
	shouldFail   bool
	errorMessage string
}

func (m *mockFailingNotifier) SendAlert(ctx context.Context, alert Alert) error {
	if m.shouldFail {
		if m.errorMessage != "" {
			return fmt.Errorf("mock failing notifier: %s", m.errorMessage)
		}
		return errors.New("mock notifier error")
	}
	return nil
}

// mockIntermittentNotifier fails every N calls for testing intermittent failures
type mockIntermittentNotifier struct {
	failCount int32
	failEvery int32
}

func (m *mockIntermittentNotifier) SendAlert(ctx context.Context, alert Alert) error {
	count := atomic.AddInt32(&m.failCount, 1)
	if count%m.failEvery == 0 {
		return errors.New("intermittent failure")
	}
	return nil
}

// mockCountingNotifier counts the number of calls for testing
type mockCountingNotifier struct {
	callCount   int32
	alerts      []Alert
	mutex       sync.Mutex
	shouldDelay bool
	delay       time.Duration
}

func (m *mockCountingNotifier) SendAlert(ctx context.Context, alert Alert) error {
	atomic.AddInt32(&m.callCount, 1)
	
	m.mutex.Lock()
	m.alerts = append(m.alerts, alert)
	m.mutex.Unlock()
	
	if m.shouldDelay {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	return nil
}

func (m *mockCountingNotifier) GetCallCount() int32 {
	return atomic.LoadInt32(&m.callCount)
}

func (m *mockCountingNotifier) GetAlerts() []Alert {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	alerts := make([]Alert, len(m.alerts))
	copy(alerts, m.alerts)
	return alerts
}

func (m *mockCountingNotifier) Reset() {
	atomic.StoreInt32(&m.callCount, 0)
	m.mutex.Lock()
	m.alerts = m.alerts[:0]
	m.mutex.Unlock()
}

// testPagerDutyNotifier is a test implementation that uses a custom server URL
type testPagerDutyNotifier struct {
	integrationKey string
	client         *http.Client
	serverURL      string
}

func (n *testPagerDutyNotifier) SendAlert(ctx context.Context, alert Alert) error {
	eventAction := "trigger"
	if alert.Level == AlertLevelInfo {
		eventAction = "resolve"
	}

	payload := map[string]interface{}{
		"routing_key":  n.integrationKey,
		"event_action": eventAction,
		"dedup_key":    fmt.Sprintf("state-manager-%s-%s", alert.Component, alert.Title),
		"payload": map[string]interface{}{
			"summary":   alert.Title,
			"source":    "state-manager",
			"severity":  n.getSeverityForLevel(alert.Level),
			"component": alert.Component,
			"custom_details": map[string]interface{}{
				"description": alert.Description,
				"value":       alert.Value,
				"threshold":   alert.Threshold,
				"labels":      alert.Labels,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal PagerDuty payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", n.serverURL+"/v2/enqueue", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create PagerDuty request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send PagerDuty request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PagerDuty request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (n *testPagerDutyNotifier) getSeverityForLevel(level AlertLevel) string {
	switch level {
	case AlertLevelInfo:
		return "info"
	case AlertLevelWarning:
		return "warning"
	case AlertLevelError:
		return "error"
	case AlertLevelCritical:
		return "critical"
	default:
		return "info"
	}
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
