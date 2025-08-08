package utils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"
)

// MockAlertHandler for testing
type MockAlertHandler struct {
	name   string
	alerts []Alert
	mu     sync.Mutex
}

func (h *MockAlertHandler) HandleAlert(alert Alert) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.alerts = append(h.alerts, alert)
	return nil
}

func (h *MockAlertHandler) Name() string {
	return h.name
}

func TestNewDiagnosticsUtils(t *testing.T) {
	du := NewDiagnosticsUtils()
	
	if du == nil {
		t.Fatal("NewDiagnosticsUtils returned nil")
	}
	if du.monitors == nil {
		t.Error("monitors map not initialized")
	}
	if du.benchmarks == nil {
		t.Error("benchmarks map not initialized")
	}
	if du.alerts == nil {
		t.Error("alerts not initialized")
	}
	if du.commonUtils == nil {
		t.Error("commonUtils not initialized")
	}
	if du.httpClient == nil {
		t.Error("httpClient not initialized")
	}
	if du.maxMonitors <= 0 {
		t.Error("maxMonitors should be positive")
	}
	if du.maxBenchmarks <= 0 {
		t.Error("maxBenchmarks should be positive")
	}

	// Clean up
	du.Shutdown()
}

func TestDiagnosticsUtils_CreateMonitor(t *testing.T) {
	du := NewDiagnosticsUtils()
	defer du.Shutdown()

	t.Run("ValidMonitor", func(t *testing.T) {
		monitor, err := du.CreateMonitor("test-monitor", "http://example.com", 1*time.Second)
		if err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}
		if monitor == nil {
			t.Fatal("Monitor is nil")
		}
		if monitor.name != "test-monitor" {
			t.Errorf("Expected monitor name 'test-monitor', got %s", monitor.name)
		}
		if monitor.target != "http://example.com" {
			t.Errorf("Expected target 'http://example.com', got %s", monitor.target)
		}
		if monitor.interval != 1*time.Second {
			t.Errorf("Expected interval 1s, got %v", monitor.interval)
		}
	})

	t.Run("EmptyName", func(t *testing.T) {
		_, err := du.CreateMonitor("", "http://example.com", 1*time.Second)
		if err == nil {
			t.Error("Expected error for empty monitor name")
		}
	})

	t.Run("EmptyTarget", func(t *testing.T) {
		_, err := du.CreateMonitor("test", "", 1*time.Second)
		if err == nil {
			t.Error("Expected error for empty target")
		}
	})

	t.Run("InvalidURL", func(t *testing.T) {
		_, err := du.CreateMonitor("test", "not-a-url", 1*time.Second)
		if err == nil {
			t.Error("Expected error for invalid URL")
		}
	})

	t.Run("MonitorLimit", func(t *testing.T) {
		// Clean up any existing monitors first
		du.monitorsMu.Lock()
		for name := range du.monitors {
			delete(du.monitors, name)
		}
		du.monitorsMu.Unlock()
		
		// Create monitors up to the limit
		originalLimit := du.maxMonitors
		du.maxMonitors = 2 // Set low limit for testing

		_, err := du.CreateMonitor("monitor1", "http://example.com/1", 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create first monitor: %v", err)
		}

		_, err = du.CreateMonitor("monitor2", "http://example.com/2", 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create second monitor: %v", err)
		}

		// Third should fail
		_, err = du.CreateMonitor("monitor3", "http://example.com/3", 1*time.Second)
		if err == nil {
			t.Error("Expected error when exceeding monitor limit")
		}

		du.maxMonitors = originalLimit // Restore
	})

	t.Run("ConcurrentCreation", func(t *testing.T) {
		// Clean up any existing monitors first
		du.monitorsMu.Lock()
		for name := range du.monitors {
			delete(du.monitors, name)
		}
		du.monitorsMu.Unlock()
		
		var wg sync.WaitGroup
		numMonitors := 5
		results := make(chan error, numMonitors)

		for i := 0; i < numMonitors; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				name := fmt.Sprintf("concurrent-monitor-%d", id)
				target := fmt.Sprintf("http://example.com/%d", id)
				_, err := du.CreateMonitor(name, target, 1*time.Second)
				results <- err
			}(i)
		}

		wg.Wait()
		close(results)

		errorCount := 0
		for err := range results {
			if err != nil {
				t.Errorf("Concurrent monitor creation failed: %v", err)
				errorCount++
			}
		}

		if errorCount > 0 {
			t.Errorf("Had %d errors in concurrent monitor creation", errorCount)
		}
	})
}

func TestDiagnosticsUtils_StartStopMonitoring(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	du := NewDiagnosticsUtils()
	defer du.Shutdown()

	t.Run("StartMonitoring", func(t *testing.T) {
		_, err := du.CreateMonitor("start-test", server.URL, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		err = du.StartMonitoring("start-test")
		if err != nil {
			t.Fatalf("StartMonitoring failed: %v", err)
		}

		// Wait for at least one health check
		time.Sleep(200 * time.Millisecond)

		// Verify monitor is running
		du.monitorsMu.RLock()
		monitor, exists := du.monitors["start-test"]
		du.monitorsMu.RUnlock()

		if !exists {
			t.Fatal("Monitor not found")
		}
		if !monitor.IsRunning() {
			t.Error("Monitor should be running")
		}

		// Stop monitoring
		err = du.StopMonitoring("start-test")
		if err != nil {
			t.Errorf("StopMonitoring failed: %v", err)
		}

		// Give it time to stop
		time.Sleep(50 * time.Millisecond)

		if monitor.IsRunning() {
			t.Error("Monitor should be stopped")
		}
	})

	t.Run("StartNonExistentMonitor", func(t *testing.T) {
		err := du.StartMonitoring("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent monitor")
		}
	})

	t.Run("StopNonExistentMonitor", func(t *testing.T) {
		err := du.StopMonitoring("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent monitor")
		}
	})

	t.Run("ConcurrentStartStop", func(t *testing.T) {
		// Create multiple monitors
		monitorNames := make([]string, 5)
		for i := 0; i < 5; i++ {
			name := fmt.Sprintf("concurrent-startstop-%d", i)
			monitorNames[i] = name
			_, err := du.CreateMonitor(name, server.URL, 50*time.Millisecond)
			if err != nil {
				t.Fatalf("CreateMonitor failed: %v", err)
			}
		}

		var wg sync.WaitGroup
		errors := make(chan error, 10)

		// Start all monitors concurrently
		for _, name := range monitorNames {
			wg.Add(1)
			go func(monitorName string) {
				defer wg.Done()
				err := du.StartMonitoring(monitorName)
				errors <- err
			}(name)
		}

		// Stop all monitors concurrently
		for _, name := range monitorNames {
			wg.Add(1)
			go func(monitorName string) {
				defer wg.Done()
				time.Sleep(100 * time.Millisecond) // Let it run briefly
				err := du.StopMonitoring(monitorName)
				errors <- err
			}(name)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			if err != nil {
				t.Errorf("Concurrent start/stop operation failed: %v", err)
			}
		}
	})
}

func TestDiagnosticsUtils_GetMonitorMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	du := NewDiagnosticsUtils()
	defer du.Shutdown()

	t.Run("ValidMetrics", func(t *testing.T) {
		_, err := du.CreateMonitor("metrics-test", server.URL, 50*time.Millisecond)
		if err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		err = du.StartMonitoring("metrics-test")
		if err != nil {
			t.Fatalf("StartMonitoring failed: %v", err)
		}

		// Wait for some health checks
		time.Sleep(150 * time.Millisecond)

		metrics, err := du.GetMonitorMetrics("metrics-test")
		if err != nil {
			t.Fatalf("GetMonitorMetrics failed: %v", err)
		}

		if metrics == nil {
			t.Fatal("Metrics is nil")
		}
		if metrics.TotalRequests == 0 {
			t.Error("Expected some requests to have been made")
		}

		du.StopMonitoring("metrics-test")
	})

	t.Run("NonExistentMonitor", func(t *testing.T) {
		_, err := du.GetMonitorMetrics("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent monitor")
		}
	})
}

func TestDiagnosticsUtils_RunBenchmark(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some processing time
		time.Sleep(1 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	du := NewDiagnosticsUtils()
	defer du.Shutdown()

	t.Run("ValidBenchmark", func(t *testing.T) {
		result, err := du.RunBenchmark("test-benchmark", server.URL, 200*time.Millisecond, 2)
		if err != nil {
			t.Fatalf("RunBenchmark failed: %v", err)
		}

		if result == nil {
			t.Fatal("Benchmark result is nil")
		}
		if result.Name != "test-benchmark" {
			t.Errorf("Expected name 'test-benchmark', got %s", result.Name)
		}
		if result.TotalRequests == 0 {
			t.Error("Expected some requests to be made")
		}
		if result.Duration <= 0 {
			t.Error("Expected positive duration")
		}
		if result.RequestsPerSec <= 0 {
			t.Error("Expected positive requests per second")
		}
	})

	t.Run("ZeroConcurrency", func(t *testing.T) {
		result, err := du.RunBenchmark("zero-concurrency", server.URL, 100*time.Millisecond, 0)
		if err != nil {
			t.Fatalf("RunBenchmark failed: %v", err)
		}
		// Should default to 1
		if result.TotalRequests == 0 {
			t.Error("Expected at least one request")
		}
	})

	t.Run("HighConcurrency", func(t *testing.T) {
		result, err := du.RunBenchmark("high-concurrency", server.URL, 100*time.Millisecond, 200)
		if err != nil {
			t.Fatalf("RunBenchmark failed: %v", err)
		}
		// Should be capped at 100
		if result.TotalRequests == 0 {
			t.Error("Expected some requests")
		}
	})

	t.Run("BenchmarkLimit", func(t *testing.T) {
		// Clean up existing benchmarks to start fresh
		du.benchmarkMu.Lock()
		for name := range du.benchmarks {
			delete(du.benchmarks, name)
		}
		du.benchmarkMu.Unlock()
		
		originalLimit := du.maxBenchmarks
		du.maxBenchmarks = 1 // Set low limit

		// First benchmark should succeed
		_, err := du.RunBenchmark("limit-test-1", server.URL, 50*time.Millisecond, 1)
		if err != nil {
			t.Fatalf("First benchmark failed: %v", err)
		}

		// Second should fail due to limit
		_, err = du.RunBenchmark("limit-test-2", server.URL, 50*time.Millisecond, 1)
		if err == nil {
			t.Error("Expected error when exceeding benchmark limit")
		}

		du.maxBenchmarks = originalLimit // Restore
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		// Clean up existing benchmarks to ensure we don't hit the limit
		du.benchmarkMu.Lock()
		for name := range du.benchmarks {
			delete(du.benchmarks, name)
		}
		du.benchmarkMu.Unlock()
		
		// Use invalid URL to cause errors
		result, err := du.RunBenchmark("error-test", "http://invalid.localhost", 100*time.Millisecond, 1)
		if err != nil {
			t.Fatalf("RunBenchmark failed: %v", err)
		}

		if result.ErrorCount == 0 {
			t.Error("Expected some errors for invalid URL")
		}
		if result.ErrorRate == 0 {
			t.Error("Expected non-zero error rate")
		}
	})
}

func TestDiagnosticsUtils_DiagnoseNetwork(t *testing.T) {
	du := NewDiagnosticsUtils()
	defer du.Shutdown()

	t.Run("ValidURL", func(t *testing.T) {
		diag, err := du.DiagnoseNetwork("http://google.com")
		if err != nil {
			t.Fatalf("DiagnoseNetwork failed: %v", err)
		}

		if diag == nil {
			t.Fatal("Diagnostic result is nil")
		}
		if diag.Target != "http://google.com" {
			t.Errorf("Expected target 'http://google.com', got %s", diag.Target)
		}
		if diag.Port != 80 {
			t.Errorf("Expected port 80, got %d", diag.Port)
		}
		if diag.DNSResolutionTime <= 0 {
			t.Error("Expected positive DNS resolution time")
		}
	})

	t.Run("HTTPSUrl", func(t *testing.T) {
		diag, err := du.DiagnoseNetwork("https://google.com")
		if err != nil {
			t.Fatalf("DiagnoseNetwork failed: %v", err)
		}

		if diag.Port != 443 {
			t.Errorf("Expected port 443 for HTTPS, got %d", diag.Port)
		}
	})

	t.Run("InvalidURL", func(t *testing.T) {
		_, err := du.DiagnoseNetwork("not-a-url")
		if err == nil {
			t.Error("Expected error for invalid URL")
		}
	})

	t.Run("UnreachableHost", func(t *testing.T) {
		diag, err := du.DiagnoseNetwork("http://invalid.localhost.nonexistent")
		if err != nil {
			t.Fatalf("DiagnoseNetwork failed: %v", err)
		}

		if diag.IsReachable {
			t.Error("Invalid host should not be reachable")
		}
		if diag.Error == "" {
			t.Error("Expected error message for unreachable host")
		}
	})
}

func TestConnectionMonitor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	du := NewDiagnosticsUtils()
	defer du.Shutdown()

	t.Run("MonitorLifecycle", func(t *testing.T) {
		monitor, err := du.CreateMonitor("lifecycle-test", server.URL, 50*time.Millisecond)
		if err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		// Start monitoring
		err = monitor.Start()
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		if !monitor.IsRunning() {
			t.Error("Monitor should be running")
		}

		// Let it run for a while
		time.Sleep(150 * time.Millisecond)

		// Check metrics were updated
		metrics := monitor.GetMetrics()
		if metrics.TotalRequests == 0 {
			t.Error("Expected some requests to have been made")
		}

		// Stop monitoring
		err = monitor.Stop()
		if err != nil {
			t.Fatalf("Stop failed: %v", err)
		}

		if monitor.IsRunning() {
			t.Error("Monitor should not be running")
		}
	})

	t.Run("DoubleStart", func(t *testing.T) {
		monitor, err := du.CreateMonitor("double-start", server.URL, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		err = monitor.Start()
		if err != nil {
			t.Fatalf("First start failed: %v", err)
		}

		err = monitor.Start()
		if err == nil {
			t.Error("Expected error for double start")
		}

		monitor.Stop()
	})

	t.Run("DoubleStop", func(t *testing.T) {
		monitor, err := du.CreateMonitor("double-stop", server.URL, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		err = monitor.Stop()
		if err == nil {
			t.Error("Expected error for stopping non-running monitor")
		}
	})

	t.Run("AlertThresholds", func(t *testing.T) {
		monitor, err := du.CreateMonitor("alert-test", server.URL, 50*time.Millisecond)
		if err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		// Set custom alert thresholds
		thresholds := &AlertThresholds{
			MaxLatency:          100 * time.Millisecond,
			MinSuccessRate:      0.9,
			MaxErrorRate:        0.1,
			MaxConsecutiveFails: 2,
		}
		monitor.SetAlertThresholds(thresholds)

		if monitor.alertThresholds != thresholds {
			t.Error("Alert thresholds not set correctly")
		}
	})
}

func TestMemoryLeak_DiagnosticsUtils(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Test creating and destroying many monitors
	for i := 0; i < 50; i++ {
		du := NewDiagnosticsUtils()
		
		// Create multiple monitors
		for j := 0; j < 5; j++ {
			name := fmt.Sprintf("leak-test-monitor-%d-%d", i, j)
			monitor, err := du.CreateMonitor(name, server.URL, 10*time.Millisecond)
			if err != nil {
				t.Fatalf("CreateMonitor failed: %v", err)
			}
			
			// Start and quickly stop
			monitor.Start()
			time.Sleep(20 * time.Millisecond)
			monitor.Stop()
		}

		// Run a benchmark
		du.RunBenchmark(fmt.Sprintf("leak-test-bench-%d", i), server.URL, 50*time.Millisecond, 2)

		// Shutdown should clean up everything
		du.Shutdown()
	}

	// Force garbage collection
	runtime.GC()
}

func TestRaceCondition_DiagnosticsUtils(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	du := NewDiagnosticsUtils()
	defer du.Shutdown()

	var wg sync.WaitGroup
	numRoutines := 20

	// Test concurrent operations
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			name := fmt.Sprintf("race-test-%d", id)
			
			// Create monitor
			monitor, err := du.CreateMonitor(name, server.URL, 25*time.Millisecond)
			if err != nil {
				return // Skip if creation fails due to concurrency
			}

			// Start monitoring
			monitor.Start()

			// Get metrics while running
			time.Sleep(50 * time.Millisecond)
			du.GetMonitorMetrics(name)

			// Stop monitoring
			du.StopMonitoring(name)
		}(i)
	}

	// Concurrent benchmarks
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			benchName := fmt.Sprintf("race-benchmark-%d", id)
			du.RunBenchmark(benchName, server.URL, 100*time.Millisecond, 1)
		}(i)
	}

	wg.Wait()
}

func TestCleanup_DiagnosticsUtils(t *testing.T) {
	du := NewDiagnosticsUtils()

	// Set very short benchmark max age for testing
	du.benchmarkMaxAge = 100 * time.Millisecond

	// Create some benchmarks
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create benchmarks
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("cleanup-test-%d", i)
		du.RunBenchmark(name, server.URL, 20*time.Millisecond, 1)
	}

	// Verify benchmarks exist
	du.benchmarkMu.RLock()
	initialCount := len(du.benchmarks)
	du.benchmarkMu.RUnlock()

	if initialCount == 0 {
		t.Fatal("No benchmarks created")
	}

	// Wait for benchmarks to age out
	time.Sleep(200 * time.Millisecond)

	// Trigger cleanup manually
	du.cleanupOldBenchmarks()

	// Check if old benchmarks were cleaned up
	du.benchmarkMu.RLock()
	finalCount := len(du.benchmarks)
	du.benchmarkMu.RUnlock()

	if finalCount >= initialCount {
		t.Error("Old benchmarks not cleaned up")
	}

	du.Shutdown()
}

func TestShutdown_DiagnosticsUtils(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	du := NewDiagnosticsUtils()

	// Create and start some monitors
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("shutdown-test-%d", i)
		monitor, err := du.CreateMonitor(name, server.URL, 50*time.Millisecond)
		if err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}
		monitor.Start()
	}

	// Verify monitors are running
	du.monitorsMu.RLock()
	monitorCount := len(du.monitors)
	du.monitorsMu.RUnlock()

	if monitorCount != 3 {
		t.Errorf("Expected 3 monitors, got %d", monitorCount)
	}

	// Shutdown should stop all monitors
	du.Shutdown()

	// Verify all monitors are stopped
	du.monitorsMu.RLock()
	for _, monitor := range du.monitors {
		if monitor.IsRunning() {
			t.Error("Monitor should be stopped after shutdown")
		}
	}
	du.monitorsMu.RUnlock()
}

func TestAlertManager(t *testing.T) {
	am := NewAlertManager()

	if am == nil {
		t.Fatal("NewAlertManager returned nil")
	}

	handler := &MockAlertHandler{name: "test-handler"}

	am.AddHandler(handler)

	alert := Alert{
		ID:        "test-alert-1",
		Type:      AlertTypeLatency,
		Severity:  AlertSeverityHigh,
		Title:     "High Latency Alert",
		Message:   "Latency exceeded threshold",
		Source:    "test-monitor",
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	err := am.TriggerAlert(alert)
	if err != nil {
		t.Fatalf("TriggerAlert failed: %v", err)
	}

	// Verify alert was handled
	handler.mu.Lock()
	handledCount := len(handler.alerts)
	handler.mu.Unlock()

	if handledCount != 1 {
		t.Errorf("Expected 1 handled alert, got %d", handledCount)
	}
}

// Benchmark tests

func BenchmarkDiagnosticsUtils_CreateMonitor(b *testing.B) {
	du := NewDiagnosticsUtils()
	defer du.Shutdown()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := fmt.Sprintf("bench-monitor-%d", i)
		du.CreateMonitor(name, "http://example.com", 1*time.Second)
	}
}

func BenchmarkConnectionMonitor_HealthCheck(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	du := NewDiagnosticsUtils()
	defer du.Shutdown()

	monitor, err := du.CreateMonitor("bench-health", server.URL, 1*time.Second)
	if err != nil {
		b.Fatalf("CreateMonitor failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := monitor.performHealthCheck()
		_ = result
	}
}

func BenchmarkDiagnosticsUtils_RunBenchmark(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	du := NewDiagnosticsUtils()
	defer du.Shutdown()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := fmt.Sprintf("bench-benchmark-%d", i)
		du.RunBenchmark(name, server.URL, 50*time.Millisecond, 1)
	}
}

func BenchmarkDiagnosticsUtils_DiagnoseNetwork(b *testing.B) {
	du := NewDiagnosticsUtils()
	defer du.Shutdown()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		du.DiagnoseNetwork("http://google.com")
	}
}

// Performance regression tests

func TestPerformanceRegression_MonitorCreation(t *testing.T) {
	du := NewDiagnosticsUtils()
	defer du.Shutdown()

	start := time.Now()
	numMonitors := 100

	for i := 0; i < numMonitors; i++ {
		name := fmt.Sprintf("perf-monitor-%d", i)
		_, err := du.CreateMonitor(name, "http://example.com", 1*time.Second)
		if err != nil {
			t.Fatalf("CreateMonitor %d failed: %v", i, err)
		}
	}

	elapsed := time.Since(start)
	avgTime := elapsed / time.Duration(numMonitors)

	// Expect creation to be fast (less than 1ms per monitor on average)
	if avgTime > 1*time.Millisecond {
		t.Errorf("Monitor creation too slow: average %v per monitor", avgTime)
	}

	t.Logf("Created %d monitors in %v (avg: %v per monitor)", numMonitors, elapsed, avgTime)
}

func TestPerformanceRegression_ConcurrentMonitoring(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	du := NewDiagnosticsUtils()
	defer du.Shutdown()

	numMonitors := 10
	monitorNames := make([]string, numMonitors)

	// Create monitors
	for i := 0; i < numMonitors; i++ {
		name := fmt.Sprintf("concurrent-perf-%d", i)
		monitorNames[i] = name
		_, err := du.CreateMonitor(name, server.URL, 20*time.Millisecond)
		if err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}
	}

	// Start all monitors
	start := time.Now()
	for _, name := range monitorNames {
		err := du.StartMonitoring(name)
		if err != nil {
			t.Fatalf("StartMonitoring failed: %v", err)
		}
	}

	// Let them run for a while
	time.Sleep(200 * time.Millisecond)

	// Check that all monitors are performing well
	totalRequests := int64(0)
	totalErrors := int64(0)

	for _, name := range monitorNames {
		metrics, err := du.GetMonitorMetrics(name)
		if err != nil {
			t.Errorf("GetMonitorMetrics failed for %s: %v", name, err)
			continue
		}
		totalRequests += metrics.TotalRequests
		totalErrors += metrics.FailedRequests
	}

	elapsed := time.Since(start)

	// Stop all monitors
	for _, name := range monitorNames {
		du.StopMonitoring(name)
	}

	errorRate := float64(totalErrors) / float64(totalRequests)
	
	t.Logf("Concurrent monitoring: %d monitors, %d total requests, %.2f%% error rate in %v",
		numMonitors, totalRequests, errorRate*100, elapsed)

	// Performance expectations
	if totalRequests == 0 {
		t.Error("No requests made during monitoring period")
	}
	if errorRate > 0.1 { // Allow up to 10% error rate
		t.Errorf("Error rate too high: %.2f%%", errorRate*100)
	}
}

// Channel safety tests

func TestChannelSafety_DiagnosticsUtils(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Millisecond) // Small delay
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	du := NewDiagnosticsUtils()
	defer du.Shutdown()

	// Test that channels are properly managed during benchmark
	t.Run("BenchmarkChannelSafety", func(t *testing.T) {
		result, err := du.RunBenchmark("channel-safety", server.URL, 100*time.Millisecond, 3)
		if err != nil {
			t.Fatalf("RunBenchmark failed: %v", err)
		}
		
		if result.TotalRequests == 0 {
			t.Error("Expected some requests to be made")
		}
	})

	// Test monitor channel safety
	t.Run("MonitorChannelSafety", func(t *testing.T) {
		monitor, err := du.CreateMonitor("channel-safety-monitor", server.URL, 50*time.Millisecond)
		if err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		// Start and stop quickly multiple times
		for i := 0; i < 5; i++ {
			err = monitor.Start()
			if err != nil && i == 0 {
				t.Fatalf("Start failed: %v", err)
			}
			
			time.Sleep(25 * time.Millisecond)
			
			err = monitor.Stop()
			if err != nil {
				t.Logf("Stop failed on iteration %d: %v", i, err)
			}
		}
	})
}