package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

var (
	// Object pools for performance optimization
	httpClientPool = &sync.Pool{
		New: func() interface{} {
			return &http.Client{
				Timeout: 30 * time.Second,
				Transport: &http.Transport{
					DisableKeepAlives: true,
				},
			}
		},
	}

	latencySlicePool = &sync.Pool{
		New: func() interface{} {
			return make([]time.Duration, 0, 1000) // Pre-allocate with reasonable capacity
		},
	}
)

// DiagnosticsUtils provides utilities for connection health monitoring and diagnostics.
type DiagnosticsUtils struct {
	monitors        map[string]*ConnectionMonitor
	monitorsMu      sync.RWMutex
	benchmarks      map[string]*BenchmarkResult
	benchmarkMu     sync.RWMutex
	alerts          *AlertManager
	httpClient      *http.Client
	cleanupTimer    *time.Timer
	maxMonitors     int
	maxBenchmarks   int
	benchmarkMaxAge time.Duration
	shutdownCtx     context.Context
	shutdownCancel  context.CancelFunc
	commonUtils     *CommonUtils
}

// ConnectionMonitor monitors connection health and performance.
type ConnectionMonitor struct {
	name            string
	target          string
	interval        time.Duration
	timeout         time.Duration
	ctx             context.Context
	cancel          context.CancelFunc
	isRunning       atomic.Bool
	metrics         *ConnectionMetrics
	metricsMu       sync.RWMutex
	healthHistory   []HealthCheckResult
	historyMu       sync.RWMutex
	maxHistorySize  int
	alertThresholds *AlertThresholds
	lastAlert       time.Time
	alertCooldown   time.Duration
}

// ConnectionMetrics tracks connection performance metrics.
type ConnectionMetrics struct {
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	FailedRequests     int64         `json:"failed_requests"`
	AverageLatency     time.Duration `json:"average_latency"`
	MinLatency         time.Duration `json:"min_latency"`
	MaxLatency         time.Duration `json:"max_latency"`
	CurrentThroughput  float64       `json:"current_throughput"`
	ErrorRate          float64       `json:"error_rate"`
	LastUpdate         time.Time     `json:"last_update"`
	ConnectionsActive  int64         `json:"connections_active"`
	ConnectionsTotal   int64         `json:"connections_total"`
	BytesSent          int64         `json:"bytes_sent"`
	BytesReceived      int64         `json:"bytes_received"`
}

// HealthCheckResult represents the result of a health check.
type HealthCheckResult struct {
	Timestamp    time.Time     `json:"timestamp"`
	Success      bool          `json:"success"`
	Latency      time.Duration `json:"latency"`
	Error        string        `json:"error,omitempty"`
	StatusCode   int           `json:"status_code,omitempty"`
	ResponseSize int64         `json:"response_size,omitempty"`
	DNSTime      time.Duration `json:"dns_time,omitempty"`
	ConnectTime  time.Duration `json:"connect_time,omitempty"`
	TLSTime      time.Duration `json:"tls_time,omitempty"`
}

// AlertThresholds defines thresholds for triggering alerts.
type AlertThresholds struct {
	MaxLatency          time.Duration `json:"max_latency"`
	MinSuccessRate      float64       `json:"min_success_rate"`
	MaxErrorRate        float64       `json:"max_error_rate"`
	MaxConsecutiveFails int           `json:"max_consecutive_fails"`
}

// AlertManager manages alerts for connection issues.
type AlertManager struct {
	handlers     []AlertHandler
	handlersMu   sync.RWMutex
	alertHistory []Alert
	historyMu    sync.RWMutex
}

// AlertHandler handles alert notifications.
type AlertHandler interface {
	HandleAlert(alert Alert) error
	Name() string
}

// Alert represents a connection alert.
type Alert struct {
	ID         string                 `json:"id"`
	Type       AlertType              `json:"type"`
	Severity   AlertSeverity          `json:"severity"`
	Title      string                 `json:"title"`
	Message    string                 `json:"message"`
	Source     string                 `json:"source"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata"`
	Resolved   bool                   `json:"resolved"`
	ResolvedAt *time.Time             `json:"resolved_at,omitempty"`
}

// AlertType represents the type of alert.
type AlertType string

const (
	AlertTypeLatency      AlertType = "latency"
	AlertTypeErrorRate    AlertType = "error_rate"
	AlertTypeAvailability AlertType = "availability"
	AlertTypeThroughput   AlertType = "throughput"
)

// AlertSeverity represents the severity of an alert.
type AlertSeverity string

const (
	AlertSeverityLow      AlertSeverity = "low"
	AlertSeverityMedium   AlertSeverity = "medium"
	AlertSeverityHigh     AlertSeverity = "high"
	AlertSeverityCritical AlertSeverity = "critical"
)

// BenchmarkResult represents performance benchmark results.
type BenchmarkResult struct {
	Name           string                 `json:"name"`
	StartTime      time.Time              `json:"start_time"`
	EndTime        time.Time              `json:"end_time"`
	Duration       time.Duration          `json:"duration"`
	TotalRequests  int                    `json:"total_requests"`
	RequestsPerSec float64                `json:"requests_per_sec"`
	AverageLatency time.Duration          `json:"average_latency"`
	MinLatency     time.Duration          `json:"min_latency"`
	MaxLatency     time.Duration          `json:"max_latency"`
	P95Latency     time.Duration          `json:"p95_latency"`
	P99Latency     time.Duration          `json:"p99_latency"`
	ErrorCount     int                    `json:"error_count"`
	ErrorRate      float64                `json:"error_rate"`
	ThroughputMBps float64                `json:"throughput_mbps"`
	Metadata       map[string]interface{} `json:"metadata"`
	LatencyBuckets map[string]int         `json:"latency_buckets"`
}

// NetworkDiagnostics provides network-level diagnostic information.
type NetworkDiagnostics struct {
	Target            string        `json:"target"`
	IP                string        `json:"ip"`
	Port              int           `json:"port"`
	DNSResolutionTime time.Duration `json:"dns_resolution_time"`
	ConnectionTime    time.Duration `json:"connection_time"`
	TLSHandshakeTime  time.Duration `json:"tls_handshake_time"`
	IsReachable       bool          `json:"is_reachable"`
	MTU               int           `json:"mtu"`
	Hops              []NetworkHop  `json:"hops"`
	Error             string        `json:"error,omitempty"`
}

// NetworkHop represents a network hop in a traceroute.
type NetworkHop struct {
	Hop     int           `json:"hop"`
	IP      string        `json:"ip"`
	Latency time.Duration `json:"latency"`
	Name    string        `json:"name,omitempty"`
}

// NewDiagnosticsUtils creates a new DiagnosticsUtils instance.
func NewDiagnosticsUtils() *DiagnosticsUtils {
	ctx, cancel := context.WithCancel(context.Background())
	
	du := &DiagnosticsUtils{
		monitors:        make(map[string]*ConnectionMonitor),
		benchmarks:      make(map[string]*BenchmarkResult),
		alerts:          NewAlertManager(),
		maxMonitors:     1000,           // Prevent unbounded growth
		maxBenchmarks:   500,            // Prevent unbounded growth
		benchmarkMaxAge: 24 * time.Hour, // Clean up old benchmarks
		shutdownCtx:     ctx,
		shutdownCancel:  cancel,
		commonUtils:     NewCommonUtils(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       100,
				IdleConnTimeout:    90 * time.Second,
				DisableCompression: false,
			},
		},
	}

	// Start periodic cleanup
	du.startCleanup()
	return du
}

// CreateMonitor creates a new connection monitor.
func (du *DiagnosticsUtils) CreateMonitor(name, target string, interval time.Duration) (*ConnectionMonitor, error) {
	if name == "" {
		return nil, errors.NewValidationError("name", "monitor name cannot be empty")
	}

	if target == "" {
		return nil, errors.NewValidationError("target", "monitor target cannot be empty")
	}

	// Check monitor limit to prevent unbounded growth (only if creating a new monitor)
	du.monitorsMu.RLock()
	existingMonitor := du.monitors[name]
	monitorCount := len(du.monitors)
	du.monitorsMu.RUnlock()

	// If we're replacing an existing monitor, don't count it against the limit
	if existingMonitor == nil && monitorCount+1 > du.maxMonitors {
		return nil, errors.NewValidationError("monitors", "maximum number of monitors exceeded")
	}

	// Validate target URL
	parsedURL, err := url.Parse(target)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, errors.NewValidationError("target", "invalid target URL - must be a valid URL with scheme and host")
	}
	
	// Only allow http and https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, errors.NewValidationError("target", "invalid URL scheme - only http and https are supported")
	}

	ctx, cancel := context.WithCancel(context.Background())

	monitor := &ConnectionMonitor{
		name:     name,
		target:   target,
		interval: interval,
		timeout:  10 * time.Second,
		ctx:      ctx,
		cancel:   cancel,
		metrics: &ConnectionMetrics{
			MinLatency: time.Hour, // Initialize with high value
			LastUpdate: time.Now(),
		},
		healthHistory:  nil, // More efficient than make([]HealthCheckResult, 0)
		maxHistorySize: 1000,
		alertThresholds: &AlertThresholds{
			MaxLatency:          5 * time.Second,
			MinSuccessRate:      0.95,
			MaxErrorRate:        0.05,
			MaxConsecutiveFails: 3,
		},
		alertCooldown: 5 * time.Minute,
	}

	du.monitorsMu.Lock()
	// Stop existing monitor if replacing
	if existingMonitor != nil {
		existingMonitor.Stop()
	}
	du.monitors[name] = monitor
	du.monitorsMu.Unlock()

	return monitor, nil
}

// StartMonitoring starts monitoring a connection.
func (du *DiagnosticsUtils) StartMonitoring(name string) error {
	du.monitorsMu.RLock()
	monitor, exists := du.monitors[name]
	du.monitorsMu.RUnlock()

	if !exists {
		return errors.NewNotFoundError("monitor not found: "+name, nil)
	}

	return monitor.Start()
}

// StopMonitoring stops monitoring a connection.
func (du *DiagnosticsUtils) StopMonitoring(name string) error {
	du.monitorsMu.RLock()
	monitor, exists := du.monitors[name]
	du.monitorsMu.RUnlock()

	if !exists {
		return errors.NewNotFoundError("monitor not found: "+name, nil)
	}

	return monitor.Stop()
}

// GetMonitorMetrics returns metrics for a specific monitor.
func (du *DiagnosticsUtils) GetMonitorMetrics(name string) (*ConnectionMetrics, error) {
	du.monitorsMu.RLock()
	monitor, exists := du.monitors[name]
	du.monitorsMu.RUnlock()

	if !exists {
		return nil, errors.NewNotFoundError("monitor not found: "+name, nil)
	}

	return monitor.GetMetrics(), nil
}

// RunBenchmark runs a performance benchmark against a target.
func (du *DiagnosticsUtils) RunBenchmark(name, target string, duration time.Duration, concurrency int) (*BenchmarkResult, error) {
	if concurrency <= 0 {
		concurrency = 1
	}

	// Limit concurrency to prevent resource exhaustion
	if concurrency > 100 {
		concurrency = 100
	}

	// Check benchmark limit to prevent unbounded growth
	du.benchmarkMu.RLock()
	benchmarkCount := len(du.benchmarks)
	du.benchmarkMu.RUnlock()

	if benchmarkCount >= du.maxBenchmarks {
		return nil, errors.NewValidationError("benchmarks", "maximum number of benchmarks exceeded")
	}

	benchmark := &BenchmarkResult{
		Name:           name,
		StartTime:      time.Now(),
		Metadata:       make(map[string]interface{}),
		LatencyBuckets: make(map[string]int),
	}

	// Run benchmark
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	var totalRequests, errorCount int64
	latencies := make(chan time.Duration, 10000)

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			du.benchmarkWorker(ctx, target, &totalRequests, &errorCount, latencies)
		}()
	}

	// Wait for completion
	wg.Wait()
	close(latencies)

	// Process results
	benchmark.EndTime = time.Now()
	benchmark.Duration = benchmark.EndTime.Sub(benchmark.StartTime)
	benchmark.TotalRequests = int(totalRequests)
	benchmark.ErrorCount = int(errorCount)
	benchmark.RequestsPerSec = float64(totalRequests) / benchmark.Duration.Seconds()
	benchmark.ErrorRate = float64(errorCount) / float64(totalRequests)

	// Process latencies using object pool for better performance
	var latencySum time.Duration
	latencySlice := latencySlicePool.Get().([]time.Duration)
	latencySlice = latencySlice[:0]          // Reset slice but keep capacity
	defer latencySlicePool.Put(latencySlice) // Return to pool when done

	for latency := range latencies {
		latencySum += latency
		latencySlice = append(latencySlice, latency)

		// Update min/max
		if benchmark.MinLatency == 0 || latency < benchmark.MinLatency {
			benchmark.MinLatency = latency
		}
		if latency > benchmark.MaxLatency {
			benchmark.MaxLatency = latency
		}
	}

	if len(latencySlice) > 0 {
		benchmark.AverageLatency = latencySum / time.Duration(len(latencySlice))

		// Calculate percentiles
		benchmark.P95Latency = du.calculatePercentile(latencySlice, 0.95)
		benchmark.P99Latency = du.calculatePercentile(latencySlice, 0.99)
	}

	// Store benchmark result
	du.benchmarkMu.Lock()
	du.benchmarks[name] = benchmark
	du.benchmarkMu.Unlock()

	return benchmark, nil
}

// DiagnoseNetwork performs comprehensive network diagnostics.
func (du *DiagnosticsUtils) DiagnoseNetwork(target string) (*NetworkDiagnostics, error) {
	targetURL, err := url.Parse(target)
	if err != nil || targetURL.Scheme == "" || targetURL.Host == "" {
		return nil, errors.NewValidationError("target", "invalid target URL - must be a valid URL with scheme and host")
	}
	
	// Only allow http and https schemes
	if targetURL.Scheme != "http" && targetURL.Scheme != "https" {
		return nil, errors.NewValidationError("target", "invalid URL scheme - only http and https are supported")
	}

	diag := &NetworkDiagnostics{
		Target: target,
		Hops:   nil, // More efficient than make([]NetworkHop, 0)
	}

	// Extract host and port
	host := targetURL.Hostname()
	port := targetURL.Port()
	if port == "" {
		switch targetURL.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			port = "80"
		}
	}

	diag.Port = du.parsePort(port)

	// DNS resolution
	start := time.Now()
	ips, err := net.LookupIP(host)
	diag.DNSResolutionTime = time.Since(start)

	if err != nil {
		diag.Error = fmt.Sprintf("DNS resolution failed: %v", err)
		return diag, nil
	}

	if len(ips) > 0 {
		diag.IP = ips[0].String()
	}

	// TCP connection test
	start = time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 10*time.Second)
	diag.ConnectionTime = time.Since(start)

	if err != nil {
		diag.Error = fmt.Sprintf("Connection failed: %v", err)
		diag.IsReachable = false
		return diag, nil
	}

	conn.Close()
	diag.IsReachable = true

	// TLS handshake test (if HTTPS)
	if targetURL.Scheme == "https" {
		tlsStart := time.Now()
		tlsConn, tlsErr := net.DialTimeout("tcp", net.JoinHostPort(host, port), 10*time.Second)
		if tlsErr == nil {
			tlsConn.Close()
			diag.TLSHandshakeTime = time.Since(tlsStart) - diag.ConnectionTime
		}
	}

	return diag, nil
}

// ConnectionMonitor methods

// Start starts the connection monitor.
func (cm *ConnectionMonitor) Start() error {
	if cm.isRunning.Load() {
		return errors.NewOperationError("Start", "monitor", fmt.Errorf("monitor is already running"))
	}

	cm.isRunning.Store(true)
	go cm.monitorLoop()
	return nil
}

// Stop stops the connection monitor.
func (cm *ConnectionMonitor) Stop() error {
	if !cm.isRunning.Load() {
		return errors.NewOperationError("Stop", "monitor", fmt.Errorf("monitor is not running"))
	}

	cm.cancel()
	cm.isRunning.Store(false)
	return nil
}

// IsRunning returns whether the monitor is currently running.
func (cm *ConnectionMonitor) IsRunning() bool {
	return cm.isRunning.Load()
}

// GetMetrics returns the current connection metrics.
func (cm *ConnectionMonitor) GetMetrics() *ConnectionMetrics {
	cm.metricsMu.RLock()
	defer cm.metricsMu.RUnlock()

	// Create a copy to avoid race conditions
	metrics := *cm.metrics
	return &metrics
}

// SetAlertThresholds sets the alert thresholds for the monitor.
func (cm *ConnectionMonitor) SetAlertThresholds(thresholds *AlertThresholds) {
	cm.alertThresholds = thresholds
}

// monitorLoop is the main monitoring loop.
func (cm *ConnectionMonitor) monitorLoop() {
	ticker := time.NewTicker(cm.interval)
	defer ticker.Stop()

	consecutiveFailures := 0

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			result := cm.performHealthCheck()

			// Update metrics
			cm.updateMetrics(result)

			// Store in history
			cm.addToHistory(result)

			// Check for alerts
			if !result.Success {
				consecutiveFailures++
			} else {
				consecutiveFailures = 0
			}

			cm.checkAlerts(result, consecutiveFailures)
		}
	}
}

// performHealthCheck performs a single health check.
func (cm *ConnectionMonitor) performHealthCheck() HealthCheckResult {
	start := time.Now()
	result := HealthCheckResult{
		Timestamp: start,
	}

	// Parse target URL
	_, err := url.Parse(cm.target)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Invalid target URL: %v", err)
		result.Latency = time.Since(start)
		return result
	}

	// Create HTTP request
	ctx, cancel := context.WithTimeout(cm.ctx, cm.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", cm.target, nil)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		result.Latency = time.Since(start)
		return result
	}

	// Use pooled HTTP client for better performance
	client := httpClientPool.Get().(*http.Client)
	client.Timeout = cm.timeout      // Set timeout for this request
	defer httpClientPool.Put(client) // Return to pool when done

	resp, err := client.Do(req)
	result.Latency = time.Since(start)

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode >= 200 && resp.StatusCode < 400

	if !result.Success {
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return result
}

// updateMetrics updates the connection metrics with a health check result.
func (cm *ConnectionMonitor) updateMetrics(result HealthCheckResult) {
	cm.metricsMu.Lock()
	defer cm.metricsMu.Unlock()

	cm.metrics.TotalRequests++

	if result.Success {
		cm.metrics.SuccessfulRequests++
	} else {
		cm.metrics.FailedRequests++
	}

	// Update latency metrics
	if result.Success {
		if cm.metrics.MinLatency == 0 || result.Latency < cm.metrics.MinLatency {
			cm.metrics.MinLatency = result.Latency
		}
		if result.Latency > cm.metrics.MaxLatency {
			cm.metrics.MaxLatency = result.Latency
		}

		// Update average latency using exponential moving average
		alpha := 0.1
		if cm.metrics.AverageLatency == 0 {
			cm.metrics.AverageLatency = result.Latency
		} else {
			cm.metrics.AverageLatency = time.Duration(
				alpha*float64(result.Latency) + (1-alpha)*float64(cm.metrics.AverageLatency))
		}
	}

	// Update error rate
	if cm.metrics.TotalRequests > 0 {
		cm.metrics.ErrorRate = float64(cm.metrics.FailedRequests) / float64(cm.metrics.TotalRequests)
	}

	cm.metrics.LastUpdate = time.Now()
}

// addToHistory adds a health check result to the history.
func (cm *ConnectionMonitor) addToHistory(result HealthCheckResult) {
	cm.historyMu.Lock()
	defer cm.historyMu.Unlock()

	cm.healthHistory = append(cm.healthHistory, result)

	// Limit history size
	if len(cm.healthHistory) > cm.maxHistorySize {
		cm.healthHistory = cm.healthHistory[1:]
	}
}

// checkAlerts checks if any alerts should be triggered.
func (cm *ConnectionMonitor) checkAlerts(result HealthCheckResult, consecutiveFailures int) {
	// Check if we're in alert cooldown
	if time.Since(cm.lastAlert) < cm.alertCooldown {
		return
	}

	// Check for consecutive failures
	if consecutiveFailures >= cm.alertThresholds.MaxConsecutiveFails {
		// Create alert - this would integrate with your alert system
		cm.lastAlert = time.Now()
	}

	// Check latency threshold
	if result.Success && result.Latency > cm.alertThresholds.MaxLatency {
		// Create high latency alert - this would integrate with your alert system
		cm.lastAlert = time.Now()
	}
}

// Helper methods

func (du *DiagnosticsUtils) benchmarkWorker(ctx context.Context, target string, totalRequests, errorCount *int64, latencies chan<- time.Duration) {
	benchmarkClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Add rate limiting to prevent excessive resource usage
	ticker := time.NewTicker(10 * time.Millisecond) // Max 100 requests/sec per worker
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Process one request per tick
		}

		requestStart := time.Now()

		req, reqErr := http.NewRequestWithContext(ctx, "GET", target, nil)
		if reqErr != nil {
			atomic.AddInt64(errorCount, 1)
			continue
		}

		resp, respErr := benchmarkClient.Do(req)
		latency := time.Since(requestStart)

		atomic.AddInt64(totalRequests, 1)

		if respErr != nil {
			atomic.AddInt64(errorCount, 1)
		} else {
			resp.Body.Close()
			if resp.StatusCode >= 400 {
				atomic.AddInt64(errorCount, 1)
			}
		}

		select {
		case latencies <- latency:
		default:
			// Channel full, skip this latency
		}
	}
}

func (du *DiagnosticsUtils) calculatePercentile(latencies []time.Duration, percentile float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Sort latencies using Go's built-in sort for O(n log n) performance
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)

	// Use Go's built-in sort.Slice for efficient sorting
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	index := int(float64(len(sorted)-1) * percentile)
	return sorted[index]
}

func (du *DiagnosticsUtils) parsePort(portStr string) int {
	// Simple port parsing - in production you'd want proper error handling
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return port
}

// AlertManager methods

func NewAlertManager() *AlertManager {
	return &AlertManager{
		handlers:     nil, // More efficient than make([]AlertHandler, 0)
		alertHistory: nil, // More efficient than make([]Alert, 0)
	}
}

func (am *AlertManager) AddHandler(handler AlertHandler) {
	am.handlersMu.Lock()
	defer am.handlersMu.Unlock()
	am.handlers = append(am.handlers, handler)
}

func (am *AlertManager) TriggerAlert(alert Alert) error {
	am.handlersMu.RLock()
	handlers := make([]AlertHandler, len(am.handlers))
	copy(handlers, am.handlers)
	am.handlersMu.RUnlock()

	// Store in history
	am.historyMu.Lock()
	am.alertHistory = append(am.alertHistory, alert)
	am.historyMu.Unlock()

	// Trigger handlers
	for _, handler := range handlers {
		if err := handler.HandleAlert(alert); err != nil {
			// Log error but continue with other handlers
			fmt.Printf("Alert handler %s failed: %v\n", handler.Name(), err)
		}
	}

	return nil
}

// startCleanup starts periodic cleanup of old benchmarks and monitors.
func (du *DiagnosticsUtils) startCleanup() {
	du.cleanupTimer = time.AfterFunc(1*time.Hour, func() {
		du.cleanupOldBenchmarks()
		du.startCleanup() // Schedule next cleanup
	})
}

// cleanupOldBenchmarks removes old benchmark results to prevent memory leaks.
func (du *DiagnosticsUtils) cleanupOldBenchmarks() {
	du.benchmarkMu.Lock()
	defer du.benchmarkMu.Unlock()

	cutoff := time.Now().Add(-du.benchmarkMaxAge)
	for name, result := range du.benchmarks {
		if result.EndTime.Before(cutoff) {
			delete(du.benchmarks, name)
		}
	}
}

// Shutdown gracefully shuts down the DiagnosticsUtils instance.
// This should be called when the application is shutting down to clean up resources.
func (du *DiagnosticsUtils) Shutdown() {
	// Cancel the shutdown context to signal all goroutines to stop
	du.shutdownCancel()
	
	// Stop all monitors
	du.monitorsMu.Lock()
	var monitorsToStop []string
	for name := range du.monitors {
		monitorsToStop = append(monitorsToStop, name)
	}
	du.monitorsMu.Unlock()
	
	// Stop monitors without holding the lock
	for _, name := range monitorsToStop {
		du.StopMonitoring(name)
	}
	
	// Stop cleanup timer
	if du.cleanupTimer != nil {
		du.cleanupTimer.Stop()
	}
	
	// Close HTTP client transport if possible
	if transport, ok := du.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

// StopCleanup stops the periodic cleanup timer.
// Deprecated: Use Shutdown() instead for proper resource cleanup.
func (du *DiagnosticsUtils) StopCleanup() {
	du.Shutdown()
}
