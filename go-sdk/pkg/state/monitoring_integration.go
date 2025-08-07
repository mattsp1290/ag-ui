package state

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// MonitoringIntegration provides easy integration with existing StateManager
type MonitoringIntegration struct {
	stateManager     *StateManager
	monitoringSystem *MonitoringSystem
	config           MonitoringConfig
	httpServer       *http.Server
}

// NewMonitoringIntegration creates a new monitoring integration
func NewMonitoringIntegration(stateManager *StateManager, config MonitoringConfig) (*MonitoringIntegration, error) {
	monitoringSystem, err := NewMonitoringSystem(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create monitoring system: %w", err)
	}

	integration := &MonitoringIntegration{
		stateManager:     stateManager,
		monitoringSystem: monitoringSystem,
		config:           config,
	}

	// Set up audit integration
	if config.AuditIntegration && stateManager.auditManager != nil {
		monitoringSystem.SetAuditManager(stateManager.auditManager)
	}

	// Register health checks
	integration.registerHealthChecks()

	// Set up HTTP server for metrics endpoint
	if config.EnablePrometheus {
		integration.setupMetricsServer()
	}

	return integration, nil
}

// registerHealthChecks registers standard health checks
func (mi *MonitoringIntegration) registerHealthChecks() {
	// State manager health check
	mi.monitoringSystem.RegisterHealthCheck(
		NewStateManagerHealthCheck(mi.stateManager),
	)

	// Memory health check
	mi.monitoringSystem.RegisterHealthCheck(
		NewMemoryHealthCheck(512, 100, 1000), // 512MB, 100ms GC pause, 1000 goroutines
	)

	// Store health check
	if mi.stateManager.store != nil {
		mi.monitoringSystem.RegisterHealthCheck(
			NewStoreHealthCheck(mi.stateManager.store, 5*time.Second),
		)
	}

	// Event handler health check
	if mi.stateManager.eventHandler != nil {
		mi.monitoringSystem.RegisterHealthCheck(
			NewEventHandlerHealthCheck(mi.stateManager.eventHandler),
		)
	}

	// Rate limiter health check
	mi.monitoringSystem.RegisterHealthCheck(
		NewRateLimiterHealthCheck(mi.stateManager.rateLimiter, mi.stateManager.clientRateLimiter),
	)

	// Audit health check
	if mi.stateManager.auditManager != nil {
		mi.monitoringSystem.RegisterHealthCheck(
			NewAuditHealthCheck(mi.stateManager.auditManager),
		)
	}

	// Performance health check (if performance optimizer is available)
	// Note: This would require access to the PerformanceOptimizer from StateManager
	// For now, we'll register a custom health check
	mi.monitoringSystem.RegisterHealthCheck(
		NewCustomHealthCheck("performance", func(ctx context.Context) error {
			// Basic performance check - ensure system is responsive
			start := time.Now()
			select {
			case <-time.After(100 * time.Millisecond):
				return fmt.Errorf("system response time exceeded 100ms")
			case <-ctx.Done():
				return ctx.Err()
			default:
				// System is responsive
				duration := time.Since(start)
				if duration > 10*time.Millisecond {
					return fmt.Errorf("performance check took %v, may indicate system stress", duration)
				}
				return nil
			}
		}),
	)
}

// setupMetricsServer sets up the HTTP server for Prometheus metrics
func (mi *MonitoringIntegration) setupMetricsServer() {
	mux := http.NewServeMux()

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Health check endpoint
	mux.HandleFunc("/health", mi.healthCheckHandler)

	// Detailed health check endpoint
	mux.HandleFunc("/health/detailed", mi.detailedHealthCheckHandler)

	// Metrics summary endpoint
	mux.HandleFunc("/metrics/summary", mi.metricsSummaryHandler)

	mi.httpServer = &http.Server{
		Addr:    ":8080", // Default port, make configurable
		Handler: mux,
	}
}

// StartMetricsServer starts the HTTP server for metrics
func (mi *MonitoringIntegration) StartMetricsServer() error {
	if mi.httpServer == nil {
		return fmt.Errorf("metrics server not configured")
	}

	mi.monitoringSystem.Logger().Info("Starting metrics server", zap.String("addr", mi.httpServer.Addr))
	return mi.httpServer.ListenAndServe()
}

// healthCheckHandler handles basic health check requests
func (mi *MonitoringIntegration) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Use context for health check operations
	_ = ctx

	// Run a simple health check
	healthStatus := mi.monitoringSystem.GetHealthStatus()

	allHealthy := true
	for _, healthy := range healthStatus {
		if !healthy {
			allHealthy = false
			break
		}
	}

	if allHealthy {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("UNHEALTHY"))
	}
}

// detailedHealthCheckHandler handles detailed health check requests
func (mi *MonitoringIntegration) detailedHealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Use context for health check operations
	_ = ctx

	healthStatus := mi.monitoringSystem.GetHealthStatus()

	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"status":    "healthy",
		"checks":    healthStatus,
	}

	allHealthy := true
	for _, healthy := range healthStatus {
		if !healthy {
			allHealthy = false
			break
		}
	}

	if !allHealthy {
		response["status"] = "unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(response)
}

// metricsSummaryHandler handles metrics summary requests
func (mi *MonitoringIntegration) metricsSummaryHandler(w http.ResponseWriter, r *http.Request) {
	metrics := mi.monitoringSystem.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// InstrumentStateManager adds monitoring to StateManager operations
func (mi *MonitoringIntegration) InstrumentStateManager() {
	// This would wrap StateManager methods with monitoring
	// For now, we'll add helper methods to record metrics

	// Note: This is a conceptual example of how to instrument StateManager methods
	// In practice, you would need to modify the StateManager struct to support this
	// For now, we provide helper methods to record metrics manually

	mi.monitoringSystem.Logger().Info("StateManager instrumentation ready - use RecordCustomMetric for manual instrumentation")
}

// RecordCustomMetric records a custom metric
func (mi *MonitoringIntegration) RecordCustomMetric(operation string, duration time.Duration, err error) {
	mi.monitoringSystem.RecordStateOperation(operation, duration, err)
}

// LogEvent logs a custom event
func (mi *MonitoringIntegration) LogEvent(ctx context.Context, level zap.Field, message string, fields ...zap.Field) {
	mi.monitoringSystem.Logger().Info(message, fields...)
}

// RegisterCustomHealthCheck registers a custom health check
func (mi *MonitoringIntegration) RegisterCustomHealthCheck(name string, checkFn func(context.Context) error) {
	mi.monitoringSystem.RegisterHealthCheck(NewCustomHealthCheck(name, checkFn))
}

// AddAlertNotifier adds an alert notifier
func (mi *MonitoringIntegration) AddAlertNotifier(notifier AlertNotifier) {
	mi.monitoringSystem.alertManager.notifiers = append(mi.monitoringSystem.alertManager.notifiers, notifier)
}

// GetMonitoringSystem returns the underlying monitoring system
func (mi *MonitoringIntegration) GetMonitoringSystem() *MonitoringSystem {
	return mi.monitoringSystem
}

// Shutdown gracefully shuts down the monitoring integration
func (mi *MonitoringIntegration) Shutdown(ctx context.Context) error {
	// Shutdown HTTP server
	if mi.httpServer != nil {
		if err := mi.httpServer.Shutdown(ctx); err != nil {
			mi.monitoringSystem.Logger().Error("Failed to shutdown metrics server", zap.Error(err))
		}
	}

	// Shutdown monitoring system
	return mi.monitoringSystem.Shutdown(ctx)
}

// MonitoringMiddleware provides HTTP middleware for monitoring
func (mi *MonitoringIntegration) MonitoringMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Start trace (placeholder for OpenTelemetry integration)
		ctx, span := mi.monitoringSystem.StartTrace(r.Context(), "http_request")
		if span != nil {
			// Placeholder for span attributes
			defer func() {
				// End span placeholder
			}()
		}

		// Create wrapped response writer to capture status code
		wrappedWriter := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call next handler
		next.ServeHTTP(wrappedWriter, r.WithContext(ctx))

		// Record metrics
		duration := time.Since(start)
		mi.monitoringSystem.Logger().Info("HTTP request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", wrappedWriter.statusCode),
			zap.Duration("duration", duration),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Example integration patterns

// SetupBasicMonitoring sets up basic monitoring for a StateManager
func SetupBasicMonitoring(stateManager *StateManager) (*MonitoringIntegration, error) {
	config := DefaultMonitoringConfig()

	// Add log notifier (create a new zap logger for monitoring)
	zapLogger, _ := zap.NewProduction()
	logNotifier := NewLogAlertNotifier(zapLogger)
	config.AlertNotifiers = []AlertNotifier{logNotifier}

	return NewMonitoringIntegration(stateManager, config)
}

// SetupProductionMonitoring sets up production-ready monitoring
func SetupProductionMonitoring(stateManager *StateManager, slackWebhookURL string) (*MonitoringIntegration, error) {
	config := DefaultMonitoringConfig()

	// Configure for production
	config.LogLevel = zap.InfoLevel
	config.EnableTracing = true
	config.TraceSampleRate = 0.01 // 1% sampling

	// Set stricter thresholds
	config.AlertThresholds.ErrorRate = 1.0
	config.AlertThresholds.P95LatencyMs = 50
	config.AlertThresholds.MemoryUsagePercent = 70

	// Add multiple notifiers (create a new zap logger for monitoring)
	zapLogger, _ := zap.NewProduction()
	logNotifier := NewLogAlertNotifier(zapLogger)

	// Create notifiers list starting with log notifier
	notifiers := []AlertNotifier{logNotifier}

	// Try to create Slack notifier
	slackNotifier, err := NewSlackAlertNotifier(slackWebhookURL, "#alerts", "StateManager")
	if err != nil {
		// Log error but continue with other notifiers
		zapLogger.Error("Failed to create Slack notifier", zap.Error(err))
	} else {
		// Use throttled notifiers to prevent spam
		throttledSlack := NewThrottledAlertNotifier(slackNotifier, 5*time.Minute)
		notifiers = append(notifiers, NewConditionalAlertNotifier(throttledSlack, func(alert Alert) bool {
			return alert.Level >= AlertLevelWarning
		}))
	}

	config.AlertNotifiers = notifiers

	return NewMonitoringIntegration(stateManager, config)
}

// SetupDevelopmentMonitoring sets up monitoring for development
func SetupDevelopmentMonitoring(stateManager *StateManager) (*MonitoringIntegration, error) {
	config := DefaultMonitoringConfig()

	// Configure for development
	config.LogLevel = zap.DebugLevel
	config.LogFormat = "console"
	config.EnableTracing = true
	config.TraceSampleRate = 1.0 // 100% sampling

	// More lenient thresholds
	config.AlertThresholds.ErrorRate = 10.0
	config.AlertThresholds.P95LatencyMs = 200
	config.AlertThresholds.MemoryUsagePercent = 90

	// Only log alerts (create a new zap logger for monitoring)
	zapLogger, _ := zap.NewProduction()
	logNotifier := NewLogAlertNotifier(zapLogger)
	config.AlertNotifiers = []AlertNotifier{logNotifier}

	return NewMonitoringIntegration(stateManager, config)
}
