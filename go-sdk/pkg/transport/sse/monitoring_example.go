package sse

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap/zapcore"
)

// Example_monitoringBasic demonstrates basic monitoring setup
func Example_monitoringBasic() {
	// Create monitoring configuration
	config := DefaultMonitoringConfig()
	config.Logging.Level = zapcore.DebugLevel

	// Create monitoring system
	monitoring, err := NewMonitoringSystem(config)
	if err != nil {
		log.Fatal(err)
	}
	defer monitoring.Shutdown(context.Background())

	// Use the monitoring system
	connID := "example-conn-1"
	monitoring.RecordConnectionEstablished(connID, "192.168.1.100:12345", "ExampleClient/1.0")
	monitoring.RecordEventReceived(connID, "message", 1024)
	monitoring.RecordEventProcessed("message", 5*time.Millisecond, nil)
	monitoring.RecordConnectionClosed(connID, "example complete")

	// Output:
}

// Example_monitoringWithHealthChecks demonstrates health check setup
func Example_monitoringWithHealthChecks() {
	// Create monitoring system
	monitoring, err := NewMonitoringSystem(DefaultMonitoringConfig())
	if err != nil {
		log.Fatal(err)
	}
	defer monitoring.Shutdown(context.Background())

	// Register health checks
	monitoring.RegisterHealthCheck(&customHealthCheck{
		name: "database",
		checkFunc: func(ctx context.Context) error {
			// Check database connectivity
			return nil
		},
	})

	monitoring.RegisterHealthCheck(&customHealthCheck{
		name: "sse-endpoint",
		checkFunc: func(ctx context.Context) error {
			// Check SSE endpoint availability
			return nil
		},
	})

	// Get health status
	status := monitoring.GetHealthStatus()
	for name, health := range status {
		fmt.Printf("Health check %s: healthy=%v\n", name, health.Healthy)
	}

	// Output:
}

// Example_monitoringWithPrometheus demonstrates Prometheus integration
func Example_monitoringWithPrometheus() {
	// Create monitoring configuration with Prometheus enabled
	config := DefaultMonitoringConfig()
	config.Metrics.Prometheus.Enabled = true
	config.Metrics.Prometheus.Namespace = "myapp"
	config.Metrics.Prometheus.Subsystem = "sse"

	// Create monitoring system
	monitoring, err := NewMonitoringSystem(config)
	if err != nil {
		log.Fatal(err)
	}
	defer monitoring.Shutdown(context.Background())

	// Expose Prometheus metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	// Start HTTP server in a goroutine
	go func() {
		if err := http.ListenAndServe(":9090", nil); err != nil {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// Use the monitoring system
	// Metrics will be automatically exposed at http://localhost:9090/metrics

	// Output:
}

// Example_monitoringWithCustomAlerts demonstrates custom alert setup
func Example_monitoringWithCustomAlerts() {
	// Create monitoring configuration with alerting
	config := DefaultMonitoringConfig()
	config.Alerting.Enabled = true
	config.Alerting.Thresholds.ErrorRate = 5.0        // 5% error rate threshold
	config.Alerting.Thresholds.ConnectionCount = 1000 // 1000 connections threshold

	// Create monitoring system
	monitoring, err := NewMonitoringSystem(config)
	if err != nil {
		log.Fatal(err)
	}
	defer monitoring.Shutdown(context.Background())

	// Add custom alert notifier
	_ = &slackAlertNotifier{
		webhookURL: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
	}
	// In real implementation, you would add this to the alert manager

	// Simulate high error rate
	for i := 0; i < 100; i++ {
		if i < 10 {
			// 10% error rate
			monitoring.RecordEventProcessed("test", 1*time.Millisecond, fmt.Errorf("error"))
		} else {
			monitoring.RecordEventProcessed("test", 1*time.Millisecond, nil)
		}
	}

	// Output:
}

// Example_monitoringPerformanceBenchmark demonstrates performance benchmarking
func Example_monitoringPerformanceBenchmark() {
	// Create monitoring system
	monitoring, err := NewMonitoringSystem(DefaultMonitoringConfig())
	if err != nil {
		log.Fatal(err)
	}
	defer monitoring.Shutdown(context.Background())

	// Start a benchmark
	benchmark := monitoring.StartBenchmark("sse-throughput-test")

	// Simulate SSE operations
	connID := "bench-conn-1"
	monitoring.RecordConnectionEstablished(connID, "127.0.0.1:8080", "BenchmarkClient")

	startTime := time.Now()
	eventCount := 10000
	totalBytes := int64(0)

	for i := 0; i < eventCount; i++ {
		eventSize := int64(1024) // 1KB per event
		monitoring.RecordEventReceived(connID, "benchmark-event", eventSize)
		monitoring.RecordEventProcessed("benchmark-event", 1*time.Millisecond, nil)
		totalBytes += eventSize
	}

	duration := time.Since(startTime)

	// Complete benchmark
	benchmark.operations = int64(eventCount)
	benchmark.bytes = totalBytes
	monitoring.CompleteBenchmark(benchmark)

	// Get performance metrics
	metrics := monitoring.GetPerformanceMetrics()
	fmt.Printf("Throughput: %.2f events/sec, %.2f MB/sec\n",
		metrics.Throughput.EventsPerSecond,
		metrics.Throughput.BytesPerSecond/1024/1024)

	fmt.Printf("Total time: %v, Events: %d, Bytes: %d\n",
		duration, eventCount, totalBytes)

	// Output:
}

// Example_monitoringDashboard demonstrates creating a monitoring dashboard endpoint
func Example_monitoringDashboard() {
	// Create monitoring system
	monitoring, err := NewMonitoringSystem(DefaultMonitoringConfig())
	if err != nil {
		log.Fatal(err)
	}
	defer monitoring.Shutdown(context.Background())

	// Create HTTP handler for monitoring dashboard
	http.HandleFunc("/monitoring/connections", func(w http.ResponseWriter, r *http.Request) {
		stats := monitoring.GetConnectionStats()
		fmt.Fprintf(w, "Active Connections: %d\n", stats.ActiveConnections)
		fmt.Fprintf(w, "Total Connections: %d\n", stats.TotalConnections)
		fmt.Fprintf(w, "Failed Connections: %d\n", stats.FailedConnections)

		for _, conn := range stats.ActiveConnectionList {
			fmt.Fprintf(w, "\nConnection %s:\n", conn.ID)
			fmt.Fprintf(w, "  Remote: %s\n", conn.RemoteAddr)
			fmt.Fprintf(w, "  Duration: %v\n", time.Since(conn.StartTime))
			fmt.Fprintf(w, "  Events In/Out: %d/%d\n", conn.EventsReceived, conn.EventsSent)
			fmt.Fprintf(w, "  Bytes In/Out: %d/%d\n", conn.BytesReceived, conn.BytesSent)
		}
	})

	http.HandleFunc("/monitoring/events", func(w http.ResponseWriter, r *http.Request) {
		stats := monitoring.GetEventStats()
		for eventType, stat := range stats {
			fmt.Fprintf(w, "Event Type: %s\n", eventType)
			fmt.Fprintf(w, "  Count: %d\n", stat.Count)
			fmt.Fprintf(w, "  Total Size: %d bytes\n", stat.TotalSize)
			fmt.Fprintf(w, "  Avg Size: %d bytes\n", stat.AvgSize)
			fmt.Fprintf(w, "  Error Count: %d\n", stat.ErrorCount)
			fmt.Fprintf(w, "\n")
		}
	})

	http.HandleFunc("/monitoring/health", func(w http.ResponseWriter, r *http.Request) {
		status := monitoring.GetHealthStatus()
		allHealthy := true

		for name, health := range status {
			if !health.Healthy {
				allHealthy = false
			}
			fmt.Fprintf(w, "%s: %v\n", name, health.Healthy)
		}

		if !allHealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})

	// Start server in a goroutine
	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Printf("Dashboard server error: %v", err)
		}
	}()

	// Output:
}

// Example_monitoringWithTracing demonstrates distributed tracing integration
func Example_monitoringWithTracing() {
	// Create monitoring configuration with tracing enabled
	config := DefaultMonitoringConfig()
	config.Tracing.Enabled = true
	config.Tracing.ServiceName = "sse-service"
	config.Tracing.SamplingRate = 0.1 // Sample 10% of traces

	// Create monitoring system
	monitoring, err := NewMonitoringSystem(config)
	if err != nil {
		log.Fatal(err)
	}
	defer monitoring.Shutdown(context.Background())

	// Use tracing in your SSE handlers
	ctx := context.Background()

	// Start a trace for connection establishment
	ctx, span := monitoring.StartTrace(ctx, "sse.connection.establish")
	if span != nil {
		defer span.End()
	}

	// Record connection with trace context
	connID := "traced-conn-1"
	monitoring.RecordConnectionEstablished(connID, "192.168.1.100:12345", "TracedClient/1.0")

	// Start a child span for event processing
	eventCtx, eventSpan := monitoring.StartTrace(ctx, "sse.event.process")
	if eventSpan != nil {
		defer eventSpan.End()
	}

	// Process event with tracing
	monitoring.RecordEventReceived(connID, "traced-event", 2048)
	time.Sleep(10 * time.Millisecond) // Simulate processing
	monitoring.RecordEventProcessed("traced-event", 10*time.Millisecond, nil)

	_ = eventCtx // Use the context in actual implementation

	// Output:
}

// Custom implementations for examples

type customHealthCheck struct {
	name      string
	checkFunc func(ctx context.Context) error
}

func (c *customHealthCheck) Name() string {
	return c.name
}

func (c *customHealthCheck) Check(ctx context.Context) error {
	if c.checkFunc != nil {
		return c.checkFunc(ctx)
	}
	return nil
}

type slackAlertNotifier struct {
	webhookURL string
}

func (s *slackAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	// In a real implementation, this would send to Slack
	log.Printf("Slack Alert: [%v] %s - %s", alert.Level, alert.Title, alert.Description)
	return nil
}
