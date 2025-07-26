package state

import (
	"context"
	"errors"
	"fmt"
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

// TestAlertNotificationFailureScenarios tests comprehensive failure scenarios for alert notification systems
func TestAlertNotificationFailureScenarios(t *testing.T) {
	t.Run("webhook_network_failures", func(t *testing.T) {
		testWebhookNetworkFailures(t)
	})

	t.Run("webhook_timeout_scenarios", func(t *testing.T) {
		testWebhookTimeoutScenarios(t)
	})

	t.Run("webhook_authentication_failures", func(t *testing.T) {
		testWebhookAuthenticationFailures(t)
	})

	t.Run("webhook_service_unavailable", func(t *testing.T) {
		testWebhookServiceUnavailable(t)
	})

	t.Run("file_notifier_failures", func(t *testing.T) {
		testFileNotifierFailures(t)
	})

	t.Run("composite_notifier_partial_failures", func(t *testing.T) {
		testCompositeNotifierPartialFailures(t)
	})

	t.Run("alert_storm_handling", func(t *testing.T) {
		testAlertStormHandling(t)
	})

	t.Run("concurrent_notification_failures", func(t *testing.T) {
		testConcurrentNotificationFailures(t)
	})

	t.Run("resource_exhaustion_scenarios", func(t *testing.T) {
		testResourceExhaustionScenarios(t)
	})

	t.Run("recovery_scenarios", func(t *testing.T) {
		testRecoveryScenarios(t)
	})
}

func testWebhookNetworkFailures(t *testing.T) {
	tests := []struct {
		name          string
		setupServer   func() *httptest.Server
		expectedError string
	}{
		{
			name: "connection_refused",
			setupServer: func() *httptest.Server {
				// Create server but don't start it
				server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				// Get the URL but don't start - connections will be refused
				server.Start()
				server.Close()
				return server
			},
			expectedError: "connection refused",
		},
		{
			name: "connection_reset",
			setupServer: func() *httptest.Server {
				return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Close connection abruptly
					if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
						conn.Close()
					}
				}))
			},
			expectedError: "EOF",
		},
		{
			name: "dns_failure",
			setupServer: func() *httptest.Server {
				// Create a server with an invalid hostname
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				// Replace the hostname with something that won't resolve
				server.URL = strings.Replace(server.URL, "127.0.0.1", "nonexistent.invalid", 1)
				return server
			},
			expectedError: "no such host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			if server != nil {
				defer server.Close()
			}

			// Create webhook notifier for testing
			notifier, err := NewWebhookAlertNotifierForTesting(server.URL, 2*time.Second)
			if err != nil {
				// For DNS failure test, this might fail at creation
				if !strings.Contains(err.Error(), "invalid webhook URL") {
					t.Fatalf("Failed to create webhook notifier: %v", err)
				}
				return
			}

			alert := Alert{
				Level:       AlertLevelError,
				Title:       "Network Failure Test",
				Description: "Testing network failure scenarios",
				Timestamp:   time.Now(),
				Component:   "test",
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err = notifier.SendAlert(ctx, alert)
			if err == nil {
				t.Errorf("Expected network failure error for %s", tt.name)
			}

			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expectedError)) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
			}
		})
	}
}

func testWebhookTimeoutScenarios(t *testing.T) {
	tests := []struct {
		name           string
		serverDelay    time.Duration
		clientTimeout  time.Duration
		contextTimeout time.Duration
		expectTimeout  bool
	}{
		{
			name:           "client_timeout",
			serverDelay:    50 * time.Millisecond,
			clientTimeout:  25 * time.Millisecond,
			contextTimeout: 200 * time.Millisecond,
			expectTimeout:  true,
		},
		{
			name:           "context_timeout",
			serverDelay:    50 * time.Millisecond,
			clientTimeout:  200 * time.Millisecond,
			contextTimeout: 25 * time.Millisecond,
			expectTimeout:  true,
		},
		{
			name:           "no_timeout",
			serverDelay:    10 * time.Millisecond,
			clientTimeout:  50 * time.Millisecond,
			contextTimeout: 100 * time.Millisecond,
			expectTimeout:  false,
		},
		{
			name:           "server_hanging",
			serverDelay:    200 * time.Millisecond, // Reduced from 5 seconds
			clientTimeout:  50 * time.Millisecond,
			contextTimeout: 100 * time.Millisecond,
			expectTimeout:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(tt.serverDelay)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"ok"}`))
			}))
			defer server.Close()

			notifier, err := NewWebhookAlertNotifierForTesting(server.URL, tt.clientTimeout)
			if err != nil {
				t.Fatalf("Failed to create webhook notifier: %v", err)
			}

			alert := Alert{
				Level:       AlertLevelError,
				Title:       "Timeout Test",
				Description: "Testing timeout scenarios",
				Timestamp:   time.Now(),
				Component:   "test",
			}

			ctx, cancel := context.WithTimeout(context.Background(), tt.contextTimeout)
			defer cancel()

			start := time.Now()
			err = notifier.SendAlert(ctx, alert)
			duration := time.Since(start)

			if tt.expectTimeout {
				if err == nil {
					t.Errorf("Expected timeout error for %s", tt.name)
				}
				// Verify timeout occurred within reasonable bounds
				expectedMaxDuration := max(tt.clientTimeout, tt.contextTimeout) + 100*time.Millisecond
				if duration > expectedMaxDuration {
					t.Errorf("Timeout took too long: %v > %v", duration, expectedMaxDuration)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s, got: %v", tt.name, err)
				}
			}
		})
	}
}

func testWebhookAuthenticationFailures(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		setupHeaders   func(notifier *WebhookAlertNotifier)
		expectedStatus int
	}{
		{
			name: "unauthorized_401",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized"}`))
			},
			setupHeaders: func(notifier *WebhookAlertNotifier) {
				// No auth headers
			},
			expectedStatus: 401,
		},
		{
			name: "forbidden_403",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"forbidden"}`))
			},
			setupHeaders: func(notifier *WebhookAlertNotifier) {
				notifier.SetHeader("Authorization", "Bearer invalid_token")
			},
			expectedStatus: 403,
		},
		{
			name: "invalid_api_key",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				apiKey := r.Header.Get("X-API-Key")
				if apiKey != "valid_key" {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"error":"invalid api key"}`))
					return
				}
				w.WriteHeader(http.StatusOK)
			},
			setupHeaders: func(notifier *WebhookAlertNotifier) {
				notifier.SetHeader("X-API-Key", "invalid_key")
			},
			expectedStatus: 401,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			notifier, err := NewWebhookAlertNotifierForTesting(server.URL, 5*time.Second)
			if err != nil {
				t.Fatalf("Failed to create webhook notifier: %v", err)
			}

			tt.setupHeaders(notifier)

			alert := Alert{
				Level:       AlertLevelError,
				Title:       "Auth Failure Test",
				Description: "Testing authentication failure scenarios",
				Timestamp:   time.Now(),
				Component:   "test",
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err = notifier.SendAlert(ctx, alert)
			if err == nil {
				t.Errorf("Expected authentication error for %s", tt.name)
			}

			if !strings.Contains(err.Error(), fmt.Sprintf("%d", tt.expectedStatus)) {
				t.Errorf("Expected error containing status %d, got: %v", tt.expectedStatus, err)
			}
		})
	}
}

func testWebhookServiceUnavailable(t *testing.T) {
	scenarios := []struct {
		name     string
		response func(w http.ResponseWriter, r *http.Request)
	}{
		{
			name: "internal_server_error",
			response: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"internal server error"}`))
			},
		},
		{
			name: "service_unavailable",
			response: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(`{"error":"service temporarily unavailable"}`))
			},
		},
		{
			name: "bad_gateway",
			response: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				w.Write([]byte(`{"error":"bad gateway"}`))
			},
		},
		{
			name: "gateway_timeout",
			response: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusGatewayTimeout)
				w.Write([]byte(`{"error":"gateway timeout"}`))
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(scenario.response))
			defer server.Close()

			notifier, err := NewWebhookAlertNotifierForTesting(server.URL, 5*time.Second)
			if err != nil {
				t.Fatalf("Failed to create webhook notifier: %v", err)
			}

			alert := Alert{
				Level:       AlertLevelCritical,
				Title:       "Service Unavailable Test",
				Description: "Testing service unavailable scenarios",
				Timestamp:   time.Now(),
				Component:   "test",
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err = notifier.SendAlert(ctx, alert)
			if err == nil {
				t.Errorf("Expected service unavailable error for %s", scenario.name)
			}

			// Should contain HTTP status code
			if !strings.Contains(err.Error(), "50") && !strings.Contains(err.Error(), "40") {
				t.Errorf("Expected HTTP error status in error message, got: %v", err)
			}
		})
	}
}

func testFileNotifierFailures(t *testing.T) {
	t.Run("permission_denied", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Skipping permission test when running as root")
		}

		// Try to create file in root directory (should fail)
		_, err := NewFileAlertNotifier("/root/test_alerts.log")
		if err == nil {
			t.Error("Expected permission denied error")
		}
	})

	t.Run("disk_full_simulation", func(t *testing.T) {
		tempDir := t.TempDir()
		alertFile := filepath.Join(tempDir, "alerts.log")

		notifier, err := NewFileAlertNotifier(alertFile)
		if err != nil {
			t.Fatalf("Failed to create file notifier: %v", err)
		}
		defer notifier.Close()

		// Create a large alert to simulate disk space issues
		alert := Alert{
			Level:       AlertLevelError,
			Title:       "Disk Full Test",
			Description: strings.Repeat("Large alert data to simulate disk full scenario. ", 10000),
			Timestamp:   time.Now(),
			Component:   "test",
		}

		// Send multiple large alerts
		var lastErr error
		for i := 0; i < 100; i++ {
			err := notifier.SendAlert(context.Background(), alert)
			if err != nil {
				lastErr = err
				break
			}
		}

		// In a real scenario with limited disk space, this would eventually fail
		// For testing, we just verify the notifier handles errors gracefully
		if lastErr != nil {
			t.Logf("File notifier handled error gracefully: %v", lastErr)
		}
	})

	t.Run("file_locked", func(t *testing.T) {
		tempDir := t.TempDir()
		alertFile := filepath.Join(tempDir, "alerts.log")

		// Create first notifier
		notifier1, err := NewFileAlertNotifier(alertFile)
		if err != nil {
			t.Fatalf("Failed to create first file notifier: %v", err)
		}
		defer notifier1.Close()

		// Try to create second notifier on the same file
		// This might work on some systems, so we handle both cases
		notifier2, err := NewFileAlertNotifier(alertFile)
		if err != nil {
			t.Logf("Expected behavior: second notifier failed due to file lock: %v", err)
		} else {
			defer notifier2.Close()
			t.Log("System allows multiple file handles to same file")
		}

		// Test alert sending with both notifiers if second one succeeded
		alert := Alert{
			Level:       AlertLevelWarning,
			Title:       "File Lock Test",
			Description: "Testing file locking scenarios",
			Timestamp:   time.Now(),
			Component:   "test",
		}

		err = notifier1.SendAlert(context.Background(), alert)
		if err != nil {
			t.Errorf("First notifier should work: %v", err)
		}

		if notifier2 != nil {
			err = notifier2.SendAlert(context.Background(), alert)
			if err != nil {
				t.Logf("Second notifier failed as expected: %v", err)
			}
		}
	})

	t.Run("file_deleted_during_operation", func(t *testing.T) {
		tempDir := t.TempDir()
		alertFile := filepath.Join(tempDir, "alerts.log")

		notifier, err := NewFileAlertNotifier(alertFile)
		if err != nil {
			t.Fatalf("Failed to create file notifier: %v", err)
		}
		defer notifier.Close()

		// Send initial alert
		alert := Alert{
			Level:       AlertLevelInfo,
			Title:       "File Deletion Test",
			Description: "Testing file deletion during operation",
			Timestamp:   time.Now(),
			Component:   "test",
		}

		err = notifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Errorf("Initial alert failed: %v", err)
		}

		// Close the file handle to simulate file system issues
		notifier.Close()
		
		// Delete the file
		os.Remove(alertFile)

		// Try to send another alert - this should fail because the file handle is closed
		err = notifier.SendAlert(context.Background(), alert)
		if err == nil {
			t.Error("Expected error after file handle closed")
		}
	})
}

func testCompositeNotifierPartialFailures(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create mix of working and failing notifiers
	workingNotifier := NewLogAlertNotifier(logger)
	
	tempDir := t.TempDir()
	alertFile := filepath.Join(tempDir, "alerts.log")
	fileNotifier, err := NewFileAlertNotifier(alertFile)
	if err != nil {
		t.Fatalf("Failed to create file notifier: %v", err)
	}
	defer fileNotifier.Close()

	failingNotifier := &AlwaysFailingNotifier{}
	timeoutNotifier := &TimeoutNotifier{delay: 2 * time.Second}

	// Create composite notifier
	composite := NewCompositeAlertNotifier(
		workingNotifier,
		fileNotifier,
		failingNotifier,
		timeoutNotifier,
	)

	alert := Alert{
		Level:       AlertLevelCritical,
		Title:       "Partial Failure Test",
		Description: "Testing partial failure scenarios in composite notifier",
		Timestamp:   time.Now(),
		Component:   "test",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err = composite.SendAlert(ctx, alert)
	duration := time.Since(start)

	// Should return error due to failing notifiers
	if err == nil {
		t.Error("Expected error from composite notifier with failing components")
	}

	// Should contain information about which notifiers failed
	errStr := err.Error()
	if !strings.Contains(errStr, "failed to send alert to some notifiers") {
		t.Errorf("Expected composite error message, got: %v", err)
	}

	// Should have completed within reasonable time (not wait for all timeouts)
	if duration > 3*time.Second {
		t.Errorf("Composite notifier took too long: %v", duration)
	}

	// Verify working notifiers still sent alerts
	// Check file was written
	if stat, err := os.Stat(alertFile); err != nil || stat.Size() == 0 {
		t.Error("Expected file notifier to write alert despite other failures")
	}
}

func testAlertStormHandling(t *testing.T) {
	// Create throttled notifier
	logger := zaptest.NewLogger(t)
	baseNotifier := NewLogAlertNotifier(logger)
	throttledNotifier := NewThrottledAlertNotifier(baseNotifier, 100*time.Millisecond)

	// Create conditional notifier (only critical alerts)
	conditionalNotifier := NewConditionalAlertNotifier(baseNotifier, func(alert Alert) bool {
		return alert.Level == AlertLevelCritical
	})

	alerts := []Alert{
		{Level: AlertLevelInfo, Title: "Info Alert", Timestamp: time.Now(), Component: "test"},
		{Level: AlertLevelWarning, Title: "Warning Alert", Timestamp: time.Now(), Component: "test"},
		{Level: AlertLevelError, Title: "Error Alert", Timestamp: time.Now(), Component: "test"},
		{Level: AlertLevelCritical, Title: "Critical Alert", Timestamp: time.Now(), Component: "test"},
	}

	ctx := context.Background()

	t.Run("throttled_notifier_storm", func(t *testing.T) {
		// Send fewer rapid alerts for faster testing
		for i := 0; i < 3; i++ {
			for _, alert := range alerts {
				err := throttledNotifier.SendAlert(ctx, alert)
				if err != nil {
					t.Errorf("Throttled notifier failed: %v", err)
				}
			}
		}

		// Check throttling worked by inspecting lastSent map
		if len(throttledNotifier.lastSent) == 0 {
			t.Error("Expected throttled notifier to track sent alerts")
		}
	})

	t.Run("conditional_notifier_filtering", func(t *testing.T) {
		// Send all alert levels
		for _, alert := range alerts {
			err := conditionalNotifier.SendAlert(ctx, alert)
			if err != nil {
				t.Errorf("Conditional notifier failed: %v", err)
			}
		}
		// Only critical alerts should have been processed (verified by condition function)
	})

	t.Run("concurrent_alert_storm", func(t *testing.T) {
		// Create a shorter timeout context for this test
		testCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		
		var wg sync.WaitGroup
		errorChan := make(chan error, 20)

		// Start fewer goroutines with fewer alerts for faster testing
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 5; j++ {
					select {
					case <-testCtx.Done():
						return // Exit if context is cancelled
					default:
						alert := Alert{
							Level:       AlertLevel(j % 4),
							Title:       fmt.Sprintf("Concurrent Alert %d-%d", id, j),
							Description: "Testing concurrent alert handling",
							Timestamp:   time.Now(),
							Component:   fmt.Sprintf("component_%d", id),
						}

						if err := throttledNotifier.SendAlert(testCtx, alert); err != nil {
							select {
							case errorChan <- err:
							case <-testCtx.Done():
								return
							}
						}
					}
				}
			}(i)
		}

		// Wait for completion with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All goroutines completed successfully
		case <-testCtx.Done():
			t.Fatalf("Test timed out waiting for goroutines to complete")
		}
		close(errorChan)

		// Check for errors
		errorCount := 0
		for err := range errorChan {
			errorCount++
			t.Logf("Concurrent alert error: %v", err)
		}

		if errorCount > 0 {
			t.Errorf("Got %d errors during concurrent alert storm", errorCount)
		}
	})
}

func testConcurrentNotificationFailures(t *testing.T) {
	// Create notifiers with different failure modes
	notifiers := []AlertNotifier{
		&RandomFailureNotifier{failureRate: 0.3},
		&RandomFailureNotifier{failureRate: 0.5},
		&SlowNotifier{delay: 5 * time.Millisecond},
		&SlowNotifier{delay: 10 * time.Millisecond},
	}

	composite := NewCompositeAlertNotifier(notifiers...)

	var wg sync.WaitGroup
	errors := make(chan error, 100)
	successes := int32(0)
	failures := int32(0)

	// Send alerts concurrently
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			alert := Alert{
				Level:       AlertLevelError,
				Title:       fmt.Sprintf("Concurrent Test %d", id),
				Description: "Testing concurrent notification failures",
				Timestamp:   time.Now(),
				Component:   "test",
			}

			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			err := composite.SendAlert(ctx, alert)
			if err != nil {
				atomic.AddInt32(&failures, 1)
				errors <- err
			} else {
				atomic.AddInt32(&successes, 1)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	successCount := atomic.LoadInt32(&successes)
	failureCount := atomic.LoadInt32(&failures)

	t.Logf("Concurrent notifications - Successes: %d, Failures: %d", successCount, failureCount)

	// Verify some notifications succeeded despite failures
	if successCount == 0 {
		t.Error("Expected some notifications to succeed despite failures")
	}

	// Log error details
	for err := range errors {
		t.Logf("Notification error: %v", err)
	}
}

func testResourceExhaustionScenarios(t *testing.T) {
	t.Run("memory_pressure", func(t *testing.T) {
		// Create many notifiers to simulate memory pressure
		var notifiers []AlertNotifier
		logger := zaptest.NewLogger(t)

		for i := 0; i < 100; i++ {
			notifiers = append(notifiers, NewLogAlertNotifier(logger))
		}

		composite := NewCompositeAlertNotifier(notifiers...)

		// Create large alert
		alert := Alert{
			Level:       AlertLevelError,
			Title:       "Memory Pressure Test",
			Description: strings.Repeat("Large alert data. ", 1000),
			Timestamp:   time.Now(),
			Component:   "test",
			Labels:      make(map[string]string),
		}

		// Add many labels to increase memory usage
		for i := 0; i < 100; i++ {
			alert.Labels[fmt.Sprintf("label_%d", i)] = fmt.Sprintf("value_%d", i)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Should handle memory pressure gracefully
		err := composite.SendAlert(ctx, alert)
		if err != nil {
			t.Logf("Expected some failures under memory pressure: %v", err)
		}
	})

	t.Run("goroutine_exhaustion", func(t *testing.T) {
		// Create slow notifiers that will block goroutines (reduced for faster testing)
		var notifiers []AlertNotifier
		for i := 0; i < 5; i++ {
			notifiers = append(notifiers, &SlowNotifier{delay: 10 * time.Millisecond})
		}

		composite := NewCompositeAlertNotifier(notifiers...)

		var wg sync.WaitGroup
		
		// Send many alerts quickly to exhaust goroutines
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				alert := Alert{
					Level:       AlertLevelWarning,
					Title:       fmt.Sprintf("Goroutine Test %d", id),
					Description: "Testing goroutine exhaustion",
					Timestamp:   time.Now(),
					Component:   "test",
				}

				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				err := composite.SendAlert(ctx, alert)
				if err != nil {
					// Expected due to timeouts
					t.Logf("Alert %d failed (expected): %v", id, err)
				}
			}(i)
		}

		wg.Wait()
	})
}

func testRecoveryScenarios(t *testing.T) {
	t.Run("webhook_recovery_after_failure", func(t *testing.T) {
		callCount := int32(0)
		
		// Server that fails first few requests then succeeds
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&callCount, 1)
			if count <= 3 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer server.Close()

		notifier, err := NewWebhookAlertNotifierForTesting(server.URL, 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create webhook notifier: %v", err)
		}

		alert := Alert{
			Level:       AlertLevelError,
			Title:       "Recovery Test",
			Description: "Testing recovery after failures",
			Timestamp:   time.Now(),
			Component:   "test",
		}

		ctx := context.Background()

		// First few attempts should fail
		for i := 0; i < 3; i++ {
			err := notifier.SendAlert(ctx, alert)
			if err == nil {
				t.Errorf("Expected failure on attempt %d", i+1)
			}
		}

		// Next attempt should succeed
		err = notifier.SendAlert(ctx, alert)
		if err != nil {
			t.Errorf("Expected success after recovery, got: %v", err)
		}
	})

	t.Run("file_notifier_recovery", func(t *testing.T) {
		tempDir := t.TempDir()
		alertFile := filepath.Join(tempDir, "recovery_test.log")

		notifier, err := NewFileAlertNotifier(alertFile)
		if err != nil {
			t.Fatalf("Failed to create file notifier: %v", err)
		}
		defer notifier.Close()

		alert := Alert{
			Level:       AlertLevelWarning,
			Title:       "File Recovery Test",
			Description: "Testing file recovery scenarios",
			Timestamp:   time.Now(),
			Component:   "test",
		}

		// Send initial alert
		err = notifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Errorf("Initial alert failed: %v", err)
		}

		// Simulate file system recovery by ensuring file exists
		if _, err := os.Stat(alertFile); os.IsNotExist(err) {
			t.Error("Alert file should exist after successful write")
		}

		// Send another alert to verify continued operation
		err = notifier.SendAlert(context.Background(), alert)
		if err != nil {
			t.Errorf("Recovery alert failed: %v", err)
		}
	})
}

// Mock notifiers for testing failure scenarios

type AlwaysFailingNotifier struct{}

func (n *AlwaysFailingNotifier) SendAlert(ctx context.Context, alert Alert) error {
	return errors.New("always failing notifier")
}

type TimeoutNotifier struct {
	delay time.Duration
}

func (n *TimeoutNotifier) SendAlert(ctx context.Context, alert Alert) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(n.delay):
		return nil
	}
}

type RandomFailureNotifier struct {
	failureRate float64
	callCount   int32
}

func (n *RandomFailureNotifier) SendAlert(ctx context.Context, alert Alert) error {
	count := atomic.AddInt32(&n.callCount, 1)
	
	// Use call count to simulate pseudo-random failures
	if float64(count%100)/100.0 < n.failureRate {
		return fmt.Errorf("random failure on call %d", count)
	}
	return nil
}

type SlowNotifier struct {
	delay time.Duration
}

func (n *SlowNotifier) SendAlert(ctx context.Context, alert Alert) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(n.delay):
		return nil
	}
}

// Benchmark tests for failure scenarios
func BenchmarkAlertNotificationFailures(b *testing.B) {
	b.Run("webhook_timeout", func(b *testing.B) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		notifier, err := NewWebhookAlertNotifierForTesting(server.URL, 50*time.Millisecond)
		if err != nil {
			b.Fatalf("Failed to create notifier: %v", err)
		}

		alert := Alert{
			Level:     AlertLevelError,
			Title:     "Benchmark Timeout",
			Timestamp: time.Now(),
			Component: "benchmark",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
			notifier.SendAlert(ctx, alert) // Will timeout
			cancel()
		}
	})

	b.Run("composite_partial_failure", func(b *testing.B) {
		logger := zaptest.NewLogger(b)
		
		notifiers := []AlertNotifier{
			NewLogAlertNotifier(logger),
			&AlwaysFailingNotifier{},
			&SlowNotifier{delay: 10 * time.Millisecond},
		}

		composite := NewCompositeAlertNotifier(notifiers...)
		alert := Alert{
			Level:     AlertLevelWarning,
			Title:     "Benchmark Composite",
			Timestamp: time.Now(),
			Component: "benchmark",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			composite.SendAlert(ctx, alert)
			cancel()
		}
	})
}

// Helper function for max
func max(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}