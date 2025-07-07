// Package main demonstrates a real-time dashboard with high-frequency state updates
// using the AG-UI state management system.
//
// This example shows:
// - High-frequency state updates from multiple data sources
// - Efficient batching and throttling of updates
// - Performance optimization for real-time scenarios
// - State streaming with delta compression
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/state"
)

// DashboardState represents the complete dashboard state
type DashboardState struct {
	SystemMetrics   SystemMetrics              `json:"systemMetrics"`
	NetworkStats    NetworkStats               `json:"networkStats"`
	ServiceHealth   map[string]ServiceStatus   `json:"serviceHealth"`
	ActivityFeed    []ActivityEvent            `json:"activityFeed"`
	Alerts          []Alert                    `json:"alerts"`
	Analytics       AnalyticsData              `json:"analytics"`
	LastUpdate      time.Time                  `json:"lastUpdate"`
}

// SystemMetrics contains system performance metrics
type SystemMetrics struct {
	CPUUsage       float64   `json:"cpuUsage"`
	MemoryUsage    float64   `json:"memoryUsage"`
	DiskUsage      float64   `json:"diskUsage"`
	Temperature    float64   `json:"temperature"`
	ProcessCount   int       `json:"processCount"`
	ThreadCount    int       `json:"threadCount"`
	Timestamp      time.Time `json:"timestamp"`
}

// NetworkStats contains network statistics
type NetworkStats struct {
	BytesIn        int64     `json:"bytesIn"`
	BytesOut       int64     `json:"bytesOut"`
	PacketsIn      int64     `json:"packetsIn"`
	PacketsOut     int64     `json:"packetsOut"`
	Connections    int       `json:"connections"`
	Bandwidth      float64   `json:"bandwidth"`
	Latency        float64   `json:"latency"`
	PacketLoss     float64   `json:"packetLoss"`
	Timestamp      time.Time `json:"timestamp"`
}

// ServiceStatus represents a service's health status
type ServiceStatus struct {
	Name           string    `json:"name"`
	Status         string    `json:"status"` // healthy, degraded, unhealthy
	Uptime         float64   `json:"uptime"`
	ResponseTime   float64   `json:"responseTime"`
	ErrorRate      float64   `json:"errorRate"`
	LastCheck      time.Time `json:"lastCheck"`
	Message        string    `json:"message"`
}

// ActivityEvent represents a system activity
type ActivityEvent struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	Source      string    `json:"source"`
	Timestamp   time.Time `json:"timestamp"`
}

// Alert represents a system alert
type Alert struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Message     string    `json:"message"`
	Severity    string    `json:"severity"` // info, warning, error, critical
	Source      string    `json:"source"`
	Acknowledged bool      `json:"acknowledged"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// AnalyticsData contains aggregated analytics
type AnalyticsData struct {
	RequestsPerSecond float64              `json:"requestsPerSecond"`
	AverageLatency    float64              `json:"averageLatency"`
	ErrorRate         float64              `json:"errorRate"`
	ActiveUsers       int                  `json:"activeUsers"`
	TopEndpoints      []EndpointStats      `json:"topEndpoints"`
	TimeSeriesData    map[string][]float64 `json:"timeSeriesData"`
}

// EndpointStats contains endpoint statistics
type EndpointStats struct {
	Path         string  `json:"path"`
	RequestCount int64   `json:"requestCount"`
	AvgLatency   float64 `json:"avgLatency"`
	ErrorRate    float64 `json:"errorRate"`
}

// MetricsCollector simulates collecting metrics from various sources
type MetricsCollector struct {
	store        *state.StateStore
	eventGen     *state.StateEventGenerator
	eventStream  *state.StateEventStream
	updateCount  int64
	errorCount   int64
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// DashboardServer simulates a dashboard server handling client connections
type DashboardServer struct {
	collector    *MetricsCollector
	clients      map[string]*DashboardClient
	mu           sync.RWMutex
	eventHandler *state.StateEventHandler
}

// DashboardClient represents a connected dashboard client
type DashboardClient struct {
	ID            string
	Connected     time.Time
	LastHeartbeat time.Time
	EventCount    int64
}

func main() {
	// Initialize the dashboard
	fmt.Println("=== Real-Time Dashboard Demo ===")
	fmt.Println("Starting high-frequency state update simulation...")

	// Create state store with optimizations for high-frequency updates
	store := state.NewStateStore(
		state.WithMaxHistory(500), // Limited history for performance
	)

	// Initialize dashboard state
	initialState := &DashboardState{
		SystemMetrics: SystemMetrics{
			CPUUsage:     0.0,
			MemoryUsage:  0.0,
			DiskUsage:    0.0,
			Temperature:  20.0,
			ProcessCount: 100,
			ThreadCount:  500,
			Timestamp:    time.Now(),
		},
		NetworkStats: NetworkStats{
			BytesIn:     0,
			BytesOut:    0,
			PacketsIn:   0,
			PacketsOut:  0,
			Connections: 0,
			Bandwidth:   100.0,
			Latency:     0.0,
			PacketLoss:  0.0,
			Timestamp:   time.Now(),
		},
		ServiceHealth: initializeServices(),
		ActivityFeed:  make([]ActivityEvent, 0, 100),
		Alerts:        make([]Alert, 0, 50),
		Analytics: AnalyticsData{
			RequestsPerSecond: 0.0,
			AverageLatency:    0.0,
			ErrorRate:         0.0,
			ActiveUsers:       0,
			TopEndpoints:      make([]EndpointStats, 0),
			TimeSeriesData:    make(map[string][]float64),
		},
		LastUpdate: time.Now(),
	}

	// Set initial state
	if err := setDashboardState(store, initialState); err != nil {
		log.Fatal("Failed to set initial state:", err)
	}

	// Create metrics collector
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	collector := &MetricsCollector{
		store:       store,
		eventGen:    state.NewStateEventGenerator(store),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Create event stream for real-time updates
	collector.eventStream = state.NewStateEventStream(
		store,
		collector.eventGen,
		state.WithStreamInterval(50*time.Millisecond), // High frequency updates
		state.WithDeltaOnly(true),                      // Only send deltas for efficiency
	)

	// Create dashboard server
	server := &DashboardServer{
		collector: collector,
		clients:   make(map[string]*DashboardClient),
		eventHandler: state.NewStateEventHandler(
			store,
			state.WithBatchSize(50),                    // Larger batch for high frequency
			state.WithBatchTimeout(25*time.Millisecond), // Shorter timeout for real-time
		),
	}

	// Simulate client connections
	fmt.Println("\n=== Simulating Client Connections ===")
	clientIDs := []string{"client-1", "client-2", "client-3"}
	for _, clientID := range clientIDs {
		server.ConnectClient(clientID)
	}

	// Subscribe to state changes for monitoring
	fmt.Println("\n=== Starting Real-Time Updates ===")
	
	// Monitor update frequency
	var updateFrequency int64
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		var lastCount int64
		for range ticker.C {
			currentCount := atomic.LoadInt64(&collector.updateCount)
			frequency := currentCount - lastCount
			atomic.StoreInt64(&updateFrequency, frequency)
			lastCount = currentCount
			
			if frequency > 0 {
				fmt.Printf("Update frequency: %d updates/sec, Total: %d, Errors: %d\n",
					frequency, currentCount, atomic.LoadInt64(&collector.errorCount))
			}
		}
	}()

	// Start metrics collection from multiple sources
	var wg sync.WaitGroup

	// System metrics collector (10Hz)
	wg.Add(1)
	go func() {
		defer wg.Done()
		collector.collectSystemMetrics(100 * time.Millisecond)
	}()

	// Network stats collector (5Hz)
	wg.Add(1)
	go func() {
		defer wg.Done()
		collector.collectNetworkStats(200 * time.Millisecond)
	}()

	// Service health checker (1Hz)
	wg.Add(1)
	go func() {
		defer wg.Done()
		collector.checkServiceHealth(1 * time.Second)
	}()

	// Activity feed generator (Variable rate)
	wg.Add(1)
	go func() {
		defer wg.Done()
		collector.generateActivityEvents()
	}()

	// Analytics aggregator (2Hz)
	wg.Add(1)
	go func() {
		defer wg.Done()
		collector.aggregateAnalytics(500 * time.Millisecond)
	}()

	// Start event streaming
	if err := collector.eventStream.Start(); err != nil {
		log.Printf("Failed to start event stream: %v", err)
	}

	// Subscribe to events and show samples
	unsubscribe := collector.eventStream.Subscribe(func(event events.Event) error {
		// Process events (in real app, this would send to clients)
		switch e := event.(type) {
		case *events.StateDeltaEvent:
			for _, client := range server.clients {
				atomic.AddInt64(&client.EventCount, 1)
			}
			
			// Sample logging (every 100th event)
			if atomic.LoadInt64(&collector.updateCount)%100 == 0 {
				fmt.Printf("Delta event: %d operations\n", len(e.Delta))
			}
		}
		return nil
	})
	defer unsubscribe()

	// Run for demonstration period
	fmt.Println("\nRunning high-frequency updates for 30 seconds...")
	time.Sleep(30 * time.Second)

	// Generate some alerts during runtime
	go func() {
		time.Sleep(5 * time.Second)
		collector.generateAlert("High CPU Usage", "CPU usage exceeded 80%", "warning")
		
		time.Sleep(10 * time.Second)
		collector.generateAlert("Service Degradation", "API service response time increased", "error")
		
		time.Sleep(5 * time.Second)
		collector.generateAlert("Network Congestion", "Packet loss detected on primary link", "critical")
	}()

	// Show performance statistics periodically
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			collector.showPerformanceStats()
		}
	}()

	// Wait for demonstration to complete
	time.Sleep(30 * time.Second)

	// Stop collectors
	fmt.Println("\n=== Stopping Collectors ===")
	cancel()
	collector.eventStream.Stop()

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("All collectors stopped")
	case <-time.After(5 * time.Second):
		fmt.Println("Timeout waiting for collectors")
	}

	// Show final statistics
	fmt.Println("\n=== Final Statistics ===")
	showFinalStats(collector, server)

	// Demonstrate state compression
	fmt.Println("\n=== State Compression Analysis ===")
	demonstrateCompression(store)

	// Show optimization techniques
	fmt.Println("\n=== Optimization Techniques Used ===")
	showOptimizationTechniques()
}

// Initialize services for monitoring
func initializeServices() map[string]ServiceStatus {
	services := []string{"api", "database", "cache", "queue", "storage"}
	serviceHealth := make(map[string]ServiceStatus)
	
	for _, name := range services {
		serviceHealth[name] = ServiceStatus{
			Name:         name,
			Status:       "healthy",
			Uptime:       100.0,
			ResponseTime: rand.Float64() * 50,
			ErrorRate:    0.0,
			LastCheck:    time.Now(),
			Message:      "Service operating normally",
		}
	}
	
	return serviceHealth
}

// MetricsCollector methods

func (c *MetricsCollector) collectSystemMetrics(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			metrics := SystemMetrics{
				CPUUsage:     math.Min(100, math.Max(0, 50+rand.Float64()*50+math.Sin(float64(time.Now().Unix())*0.1)*20)),
				MemoryUsage:  math.Min(100, math.Max(0, 60+rand.Float64()*30)),
				DiskUsage:    math.Min(100, math.Max(0, 40+rand.Float64()*20)),
				Temperature:  20 + rand.Float64()*40,
				ProcessCount: 100 + rand.Intn(50),
				ThreadCount:  500 + rand.Intn(200),
				Timestamp:    time.Now(),
			}

			if err := c.updateMetrics("/systemMetrics", metrics); err != nil {
				atomic.AddInt64(&c.errorCount, 1)
			}
		}
	}
}

func (c *MetricsCollector) collectNetworkStats(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var bytesIn, bytesOut int64

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			// Simulate network traffic
			bytesIn += int64(rand.Intn(1000000))
			bytesOut += int64(rand.Intn(800000))

			stats := NetworkStats{
				BytesIn:     bytesIn,
				BytesOut:    bytesOut,
				PacketsIn:   bytesIn / 1500,
				PacketsOut:  bytesOut / 1500,
				Connections: 100 + rand.Intn(500),
				Bandwidth:   math.Max(0, 100-rand.Float64()*20),
				Latency:     math.Max(1, rand.Float64()*100),
				PacketLoss:  math.Max(0, rand.Float64()*5),
				Timestamp:   time.Now(),
			}

			if err := c.updateMetrics("/networkStats", stats); err != nil {
				atomic.AddInt64(&c.errorCount, 1)
			}
		}
	}
}

func (c *MetricsCollector) checkServiceHealth(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	services := []string{"api", "database", "cache", "queue", "storage"}

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			for _, service := range services {
				// Simulate service health checks
				status := "healthy"
				responseTime := rand.Float64() * 100
				errorRate := rand.Float64() * 5

				if responseTime > 80 {
					status = "degraded"
				}
				if errorRate > 3 {
					status = "unhealthy"
				}

				health := ServiceStatus{
					Name:         service,
					Status:       status,
					Uptime:       99.0 + rand.Float64(),
					ResponseTime: responseTime,
					ErrorRate:    errorRate,
					LastCheck:    time.Now(),
					Message:      fmt.Sprintf("Service %s check completed", service),
				}

				path := fmt.Sprintf("/serviceHealth/%s", service)
				if err := c.updateMetrics(path, health); err != nil {
					atomic.AddInt64(&c.errorCount, 1)
				}
			}
		}
	}
}

func (c *MetricsCollector) generateActivityEvents() {
	eventTypes := []string{"user_login", "api_call", "data_sync", "backup_complete", "deployment"}
	severities := []string{"info", "warning", "error"}
	sources := []string{"web", "api", "worker", "scheduler", "monitor"}

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			// Variable rate based on time of day simulation
			delay := time.Duration(100+rand.Intn(900)) * time.Millisecond
			time.Sleep(delay)

			event := ActivityEvent{
				ID:          fmt.Sprintf("evt-%d", time.Now().UnixNano()),
				Type:        eventTypes[rand.Intn(len(eventTypes))],
				Description: fmt.Sprintf("Event from %s", sources[rand.Intn(len(sources))]),
				Severity:    severities[rand.Intn(len(severities))],
				Source:      sources[rand.Intn(len(sources))],
				Timestamp:   time.Now(),
			}

			// Add to activity feed (keep last 100)
			c.mu.Lock()
			currentFeed, _ := c.store.Get("/activityFeed")
			feed, ok := currentFeed.([]interface{})
			if !ok {
				feed = make([]interface{}, 0)
			}

			// Convert event to interface{}
			eventData, _ := json.Marshal(event)
			var eventInterface interface{}
			json.Unmarshal(eventData, &eventInterface)

			feed = append(feed, eventInterface)
			if len(feed) > 100 {
				feed = feed[len(feed)-100:]
			}

			c.store.Set("/activityFeed", feed)
			c.mu.Unlock()

			atomic.AddInt64(&c.updateCount, 1)
		}
	}
}

func (c *MetricsCollector) aggregateAnalytics(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initialize time series data
	metrics := []string{"cpu", "memory", "requests", "errors"}
	timeSeriesData := make(map[string][]float64)
	for _, metric := range metrics {
		timeSeriesData[metric] = make([]float64, 0, 60) // Keep last 60 points
	}

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			// Get current metrics
			sysMetrics, _ := c.store.Get("/systemMetrics")
			
			// Update time series data
			if sm, ok := sysMetrics.(map[string]interface{}); ok {
				if cpu, ok := sm["cpuUsage"].(float64); ok {
					timeSeriesData["cpu"] = appendToTimeSeries(timeSeriesData["cpu"], cpu, 60)
				}
				if mem, ok := sm["memoryUsage"].(float64); ok {
					timeSeriesData["memory"] = appendToTimeSeries(timeSeriesData["memory"], mem, 60)
				}
			}

			// Generate analytics
			analytics := AnalyticsData{
				RequestsPerSecond: math.Max(0, 1000+rand.Float64()*500+math.Sin(float64(time.Now().Unix())*0.05)*200),
				AverageLatency:    math.Max(10, 50+rand.Float64()*50),
				ErrorRate:         math.Max(0, rand.Float64()*2),
				ActiveUsers:       100 + rand.Intn(400),
				TopEndpoints: []EndpointStats{
					{Path: "/api/users", RequestCount: rand.Int63n(10000), AvgLatency: rand.Float64() * 100, ErrorRate: rand.Float64() * 2},
					{Path: "/api/data", RequestCount: rand.Int63n(8000), AvgLatency: rand.Float64() * 150, ErrorRate: rand.Float64() * 3},
					{Path: "/api/metrics", RequestCount: rand.Int63n(5000), AvgLatency: rand.Float64() * 80, ErrorRate: rand.Float64() * 1},
				},
				TimeSeriesData: timeSeriesData,
			}

			if err := c.updateMetrics("/analytics", analytics); err != nil {
				atomic.AddInt64(&c.errorCount, 1)
			}
		}
	}
}

func (c *MetricsCollector) updateMetrics(path string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	atomic.AddInt64(&c.updateCount, 1)
	
	// Update last update timestamp
	c.store.Set("/lastUpdate", time.Now())
	
	return c.store.Set(path, value)
}

func (c *MetricsCollector) generateAlert(title, message, severity string) {
	alert := Alert{
		ID:           fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		Title:        title,
		Message:      message,
		Severity:     severity,
		Source:       "monitor",
		Acknowledged: false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	currentAlerts, _ := c.store.Get("/alerts")
	alerts, ok := currentAlerts.([]interface{})
	if !ok {
		alerts = make([]interface{}, 0)
	}

	// Convert alert to interface{}
	alertData, _ := json.Marshal(alert)
	var alertInterface interface{}
	json.Unmarshal(alertData, &alertInterface)

	alerts = append(alerts, alertInterface)
	if len(alerts) > 50 {
		alerts = alerts[len(alerts)-50:]
	}

	c.store.Set("/alerts", alerts)
	fmt.Printf("Alert generated: %s - %s\n", title, message)
}

func (c *MetricsCollector) showPerformanceStats() {
	fmt.Println("\n--- Performance Statistics ---")
	fmt.Printf("Total updates: %d\n", atomic.LoadInt64(&c.updateCount))
	fmt.Printf("Error count: %d\n", atomic.LoadInt64(&c.errorCount))
	fmt.Printf("Store version: %d\n", c.store.GetVersion())
	
	// Get state size estimate
	exported, _ := c.store.Export()
	fmt.Printf("State size: %d bytes\n", len(exported))
	
	// Show event generator stats
	if c.eventGen != nil {
		// In a real implementation, we'd have metrics from the generator
		fmt.Println("Event generation: Active")
	}
}

// DashboardServer methods

func (s *DashboardServer) ConnectClient(clientID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client := &DashboardClient{
		ID:            clientID,
		Connected:     time.Now(),
		LastHeartbeat: time.Now(),
		EventCount:    0,
	}

	s.clients[clientID] = client
	fmt.Printf("Client connected: %s\n", clientID)
}

// Helper functions

func setDashboardState(store *state.StateStore, dashboard *DashboardState) error {
	data, err := json.Marshal(dashboard)
	if err != nil {
		return err
	}

	var stateMap map[string]interface{}
	if err := json.Unmarshal(data, &stateMap); err != nil {
		return err
	}

	for key, value := range stateMap {
		if err := store.Set("/"+key, value); err != nil {
			return err
		}
	}

	return nil
}

func appendToTimeSeries(series []float64, value float64, maxLen int) []float64 {
	series = append(series, value)
	if len(series) > maxLen {
		series = series[len(series)-maxLen:]
	}
	return series
}

func showFinalStats(collector *MetricsCollector, server *DashboardServer) {
	fmt.Printf("Total updates processed: %d\n", atomic.LoadInt64(&collector.updateCount))
	fmt.Printf("Total errors: %d\n", atomic.LoadInt64(&collector.errorCount))
	fmt.Printf("Error rate: %.2f%%\n", float64(atomic.LoadInt64(&collector.errorCount))/float64(atomic.LoadInt64(&collector.updateCount))*100)
	
	fmt.Println("\nClient statistics:")
	for _, client := range server.clients {
		fmt.Printf("  %s: %d events received\n", client.ID, atomic.LoadInt64(&client.EventCount))
	}
	
	// Show final state summary
	finalState, _ := collector.store.Get("/")
	if state, ok := finalState.(map[string]interface{}); ok {
		if analytics, ok := state["analytics"].(map[string]interface{}); ok {
			fmt.Println("\nFinal analytics:")
			fmt.Printf("  Requests/sec: %.2f\n", analytics["requestsPerSecond"])
			fmt.Printf("  Active users: %v\n", analytics["activeUsers"])
			fmt.Printf("  Error rate: %.2f%%\n", analytics["errorRate"])
		}
	}
}

func demonstrateCompression(store *state.StateStore) {
	// Export full state
	fullExport, _ := store.Export()
	fmt.Printf("Full state size: %d bytes\n", len(fullExport))
	
	// Create snapshot for comparison
	snapshot, _ := store.CreateSnapshot()
	snapshotData, _ := json.Marshal(snapshot.State)
	fmt.Printf("Snapshot size: %d bytes\n", len(snapshotData))
	
	// Show compression ratio
	if len(fullExport) > 0 {
		ratio := float64(len(snapshotData)) / float64(len(fullExport)) * 100
		fmt.Printf("Size ratio: %.2f%%\n", ratio)
	}
	
	// Demonstrate delta efficiency
	history, _ := store.GetHistory()
	if len(history) > 10 {
		var totalDeltaSize int
		for i := len(history) - 10; i < len(history); i++ {
			if history[i].Delta != nil {
				deltaData, _ := json.Marshal(history[i].Delta)
				totalDeltaSize += len(deltaData)
			}
		}
		fmt.Printf("Average delta size (last 10): %d bytes\n", totalDeltaSize/10)
	}
}

func showOptimizationTechniques() {
	techniques := []struct {
		Name        string
		Description string
	}{
		{
			Name:        "Delta Compression",
			Description: "Only transmit changes instead of full state",
		},
		{
			Name:        "Batch Processing",
			Description: "Group multiple updates into single operations",
		},
		{
			Name:        "Throttling",
			Description: "Limit update frequency to prevent overload",
		},
		{
			Name:        "Selective Updates",
			Description: "Only update changed paths in the state tree",
		},
		{
			Name:        "Event Streaming",
			Description: "Use server-sent events for real-time updates",
		},
		{
			Name:        "State Sharding",
			Description: "Partition state for parallel processing",
		},
		{
			Name:        "Circular Buffers",
			Description: "Fixed-size buffers for time series data",
		},
		{
			Name:        "Async Processing",
			Description: "Non-blocking updates with goroutines",
		},
	}
	
	for _, tech := range techniques {
		fmt.Printf("- %s: %s\n", tech.Name, tech.Description)
	}
}

func printJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}