package main

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
)

// ResourceMonitorTool demonstrates advanced resource monitoring and optimization techniques.
// This example shows CPU, memory, goroutine tracking, GC monitoring, and performance profiling.
type ResourceMonitorTool struct{}

// ResourceMonitorParams defines the parameters for resource monitoring
type ResourceMonitorParams struct {
	Duration      int     `json:"duration_seconds" validate:"min=1,max=300"`
	SampleRate    int     `json:"sample_rate_ms" validate:"min=10,max=5000"`
	EnableCPU     bool    `json:"enable_cpu_monitoring"`
	EnableMemory  bool    `json:"enable_memory_monitoring"`
	EnableGC      bool    `json:"enable_gc_monitoring"`
	EnableProfile bool    `json:"enable_profiling"`
	Workload      string  `json:"workload" validate:"oneof=idle light medium heavy custom"`
	GCTarget      int     `json:"gc_target_percent" validate:"min=50,max=500"`
	MemLimit      int64   `json:"memory_limit_mb"`
	AlertThresholds map[string]float64 `json:"alert_thresholds"`
}

// SystemMonitor provides comprehensive system resource monitoring
type SystemMonitor struct {
	mu              sync.RWMutex
	running         bool
	startTime       time.Time
	sampleInterval  time.Duration
	samples         []ResourceSnapshot
	maxSamples      int
	cpuTracker      *CPUTracker
	memoryTracker   *MemoryTracker
	gcTracker       *GCTracker
	goroutineTracker *GoroutineTracker
	profiler        *SystemProfiler
	alertManager    *AlertManager
	ctx             context.Context
	cancel          context.CancelFunc
}

// ResourceSnapshot captures system state at a point in time
type ResourceSnapshot struct {
	Timestamp      time.Time `json:"timestamp"`
	CPU            CPUMetrics `json:"cpu"`
	Memory         MemoryMetrics `json:"memory"`
	GC             GCMetrics `json:"gc"`
	Goroutines     GoroutineMetrics `json:"goroutines"`
	System         SystemMetrics `json:"system"`
}

// CPUMetrics tracks CPU usage and performance
type CPUMetrics struct {
	UserPercent     float64       `json:"user_percent"`
	SystemPercent   float64       `json:"system_percent"`
	IdlePercent     float64       `json:"idle_percent"`
	LoadAverage1    float64       `json:"load_average_1m"`
	LoadAverage5    float64       `json:"load_average_5m"`
	LoadAverage15   float64       `json:"load_average_15m"`
	CPUCount        int           `json:"cpu_count"`
	ContextSwitches int64         `json:"context_switches"`
	Interrupts      int64         `json:"interrupts"`
	ProcessCPUTime  time.Duration `json:"process_cpu_time"`
}

// MemoryMetrics tracks memory usage patterns
type MemoryMetrics struct {
	// Go runtime memory stats
	Alloc         uint64 `json:"alloc"`
	TotalAlloc    uint64 `json:"total_alloc"`
	Sys           uint64 `json:"sys"`
	Lookups       uint64 `json:"lookups"`
	Mallocs       uint64 `json:"mallocs"`
	Frees         uint64 `json:"frees"`
	
	// Heap statistics
	HeapAlloc    uint64 `json:"heap_alloc"`
	HeapSys      uint64 `json:"heap_sys"`
	HeapIdle     uint64 `json:"heap_idle"`
	HeapInuse    uint64 `json:"heap_inuse"`
	HeapReleased uint64 `json:"heap_released"`
	HeapObjects  uint64 `json:"heap_objects"`
	
	// Stack statistics
	StackInuse  uint64 `json:"stack_inuse"`
	StackSys    uint64 `json:"stack_sys"`
	
	// Other memory areas
	MSpanInuse  uint64 `json:"mspan_inuse"`
	MSpanSys    uint64 `json:"mspan_sys"`
	MCacheInuse uint64 `json:"mcache_inuse"`
	MCacheSys   uint64 `json:"mcache_sys"`
	BuckHashSys uint64 `json:"buck_hash_sys"`
	GCSys       uint64 `json:"gc_sys"`
	OtherSys    uint64 `json:"other_sys"`
	
	// System memory (if available)
	SystemTotal     uint64  `json:"system_total"`
	SystemAvailable uint64  `json:"system_available"`
	SystemUsed      uint64  `json:"system_used"`
	SystemPercent   float64 `json:"system_percent"`
}

// GCMetrics tracks garbage collection performance
type GCMetrics struct {
	NumGC           uint32        `json:"num_gc"`
	NumForcedGC     uint32        `json:"num_forced_gc"`
	GCCPUFraction   float64       `json:"gc_cpu_fraction"`
	LastGC          time.Time     `json:"last_gc"`
	PauseTotal      time.Duration `json:"pause_total"`
	PauseAvg        time.Duration `json:"pause_avg"`
	PauseMax        time.Duration `json:"pause_max"`
	PauseMin        time.Duration `json:"pause_min"`
	PauseP95        time.Duration `json:"pause_p95"`
	PauseP99        time.Duration `json:"pause_p99"`
	EnablePercent   int           `json:"enable_percent"`
	NextGC          uint64        `json:"next_gc"`
	TargetHeapSize  uint64        `json:"target_heap_size"`
	TriggeredBy     string        `json:"triggered_by"`
}

// GoroutineMetrics tracks goroutine lifecycle and performance
type GoroutineMetrics struct {
	Total        int                        `json:"total"`
	Running      int                        `json:"running"`
	Runnable     int                        `json:"runnable"`
	Waiting      int                        `json:"waiting"`
	Syscall      int                        `json:"syscall"`
	Dead         int                        `json:"dead"`
	StateChanges map[string]int             `json:"state_changes"`
	StackSizes   GoroutineStackStats        `json:"stack_sizes"`
	LifetimeStats GoroutineLifetimeStats    `json:"lifetime_stats"`
}

// GoroutineStackStats tracks goroutine stack usage
type GoroutineStackStats struct {
	MinSize    int     `json:"min_size"`
	MaxSize    int     `json:"max_size"`
	AvgSize    int     `json:"avg_size"`
	TotalSize  int64   `json:"total_size"`
	P95Size    int     `json:"p95_size"`
	Histogram  map[string]int `json:"histogram"`
}

// GoroutineLifetimeStats tracks goroutine creation and destruction
type GoroutineLifetimeStats struct {
	Created     int64         `json:"created"`
	Destroyed   int64         `json:"destroyed"`
	AvgLifetime time.Duration `json:"avg_lifetime"`
	MaxLifetime time.Duration `json:"max_lifetime"`
	CreationRate float64      `json:"creation_rate"`
}

// SystemMetrics tracks overall system performance
type SystemMetrics struct {
	ProcessID       int           `json:"process_id"`
	ThreadCount     int           `json:"thread_count"`
	HandleCount     int           `json:"handle_count"`
	ProcessUptime   time.Duration `json:"process_uptime"`
	SystemUptime    time.Duration `json:"system_uptime"`
	OpenFiles       int           `json:"open_files"`
	NetworkConnections int        `json:"network_connections"`
	DiskIO          DiskIOMetrics `json:"disk_io"`
	NetworkIO       NetworkIOMetrics `json:"network_io"`
}

// DiskIOMetrics tracks disk I/O performance
type DiskIOMetrics struct {
	ReadBytes    uint64  `json:"read_bytes"`
	WriteBytes   uint64  `json:"write_bytes"`
	ReadOps      uint64  `json:"read_ops"`
	WriteOps     uint64  `json:"write_ops"`
	ReadLatency  time.Duration `json:"read_latency"`
	WriteLatency time.Duration `json:"write_latency"`
	IOPS         float64 `json:"iops"`
	Utilization  float64 `json:"utilization"`
}

// NetworkIOMetrics tracks network I/O performance
type NetworkIOMetrics struct {
	BytesSent     uint64  `json:"bytes_sent"`
	BytesReceived uint64  `json:"bytes_received"`
	PacketsSent   uint64  `json:"packets_sent"`
	PacketsReceived uint64 `json:"packets_received"`
	Errors        uint64  `json:"errors"`
	Drops         uint64  `json:"drops"`
	Bandwidth     float64 `json:"bandwidth_mbps"`
}

// Tracker interfaces for different monitoring components
type CPUTracker struct {
	samples []CPUMetrics
	lastSample time.Time
}

type MemoryTracker struct {
	baseline runtime.MemStats
	samples  []MemoryMetrics
	leakDetector *MemoryLeakDetector
}

type GCTracker struct {
	samples []GCMetrics
	gcEvents []GCEvent
	lastGCStats debug.GCStats
}

type GoroutineTracker struct {
	samples []GoroutineMetrics
	goroutineMap map[uint64]*GoroutineInfo
}

// Alert system
type AlertManager struct {
	thresholds map[string]float64
	alerts     []Alert
	handlers   []AlertHandler
}

type Alert struct {
	Type      string    `json:"type"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
	Resolved  bool      `json:"resolved"`
}

type AlertHandler interface {
	HandleAlert(alert Alert) error
}

// Supporting structures
type GCEvent struct {
	Timestamp   time.Time     `json:"timestamp"`
	Duration    time.Duration `json:"duration"`
	HeapBefore  uint64        `json:"heap_before"`
	HeapAfter   uint64        `json:"heap_after"`
	Freed       uint64        `json:"freed"`
	TriggerType string        `json:"trigger_type"`
}

type GoroutineInfo struct {
	ID          uint64
	State       string
	StartTime   time.Time
	StackSize   int
	Function    string
	LastUpdated time.Time
}

type MemoryLeakDetector struct {
	objects map[string]*ObjectTracker
	threshold uint64
}

type ObjectTracker struct {
	Count     int64
	Size      uint64
	LastSeen  time.Time
	Growing   bool
	GrowthRate float64
}

type SystemProfiler struct {
	enabled    bool
	cpuProfile []byte
	memProfile []byte
	blockProfile []byte
	mutexProfile []byte
	goroutineProfile []byte
	heapProfile []byte
}

// CreateResourceMonitorTool creates and registers the resource monitor tool
func CreateResourceMonitorTool() *tools.Tool {
	return &tools.Tool{
		ID:          "resource-monitor",
		Name:        "ResourceMonitor",
		Description: "Advanced system resource monitoring and performance analysis tool",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"duration_seconds": {
					Type:        "integer",
					Description: "Duration to monitor resources in seconds",
					Default:     60,
					Minimum:     func() *float64 { v := 1.0; return &v }(),
					Maximum:     func() *float64 { v := 300.0; return &v }(),
				},
				"sample_rate_ms": {
					Type:        "integer",
					Description: "Sample rate in milliseconds",
					Default:     1000,
					Minimum:     func() *float64 { v := 10.0; return &v }(),
					Maximum:     func() *float64 { v := 5000.0; return &v }(),
				},
				"enable_cpu_monitoring": {
					Type:        "boolean",
					Description: "Enable CPU usage monitoring",
					Default:     true,
				},
				"enable_memory_monitoring": {
					Type:        "boolean",
					Description: "Enable memory usage monitoring",
					Default:     true,
				},
				"enable_gc_monitoring": {
					Type:        "boolean",
					Description: "Enable garbage collection monitoring",
					Default:     true,
				},
				"enable_profiling": {
					Type:        "boolean",
					Description: "Enable performance profiling",
					Default:     false,
				},
				"workload": {
					Type:        "string",
					Description: "Workload to simulate during monitoring",
					Enum:        []interface{}{"idle", "light", "medium", "heavy", "custom"},
					Default:     "light",
				},
				"gc_target_percent": {
					Type:        "integer",
					Description: "Target GC percentage (GOGC)",
					Default:     100,
					Minimum:     func() *float64 { v := 50.0; return &v }(),
					Maximum:     func() *float64 { v := 500.0; return &v }(),
				},
				"memory_limit_mb": {
					Type:        "integer",
					Description: "Memory limit in MB (0 = no limit)",
					Default:     0,
				},
				"alert_thresholds": {
					Type:        "object",
					Description: "Alert thresholds for various metrics",
					Properties: map[string]*tools.Property{
						"cpu_percent": {
							Type:    "number",
							Default: 80.0,
						},
						"memory_percent": {
							Type:    "number",
							Default: 90.0,
						},
						"gc_pause_ms": {
							Type:    "number",
							Default: 100.0,
						},
						"goroutine_count": {
							Type:    "number",
							Default: 10000.0,
						},
					},
				},
			},
			Required: []string{},
		},
		Executor: &ResourceMonitorTool{},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      true,
			Cancelable: true,
			Cacheable:  false,
			Timeout:    10 * time.Minute,
		},
		Metadata: &tools.ToolMetadata{
			Author:   "Performance Team",
			License:  "MIT",
			Tags:     []string{"performance", "monitoring", "resources", "profiling"},
			Examples: []tools.ToolExample{
				{
					Name:        "Basic Monitoring",
					Description: "Monitor system resources for 30 seconds",
					Input: map[string]interface{}{
						"duration_seconds": 30,
						"workload":        "light",
						"enable_profiling": false,
					},
				},
				{
					Name:        "Intensive Monitoring",
					Description: "Monitor during heavy workload with profiling",
					Input: map[string]interface{}{
						"duration_seconds":   60,
						"sample_rate_ms":     500,
						"workload":          "heavy",
						"enable_profiling":   true,
						"gc_target_percent":  50,
					},
				},
			},
		},
	}
}

// Execute runs the resource monitoring
func (t *ResourceMonitorTool) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	// Parse parameters
	p, err := parseResourceMonitorParams(params)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Create monitor
	monitor := NewSystemMonitor(ctx, p)
	
	// Start monitoring
	if err := monitor.Start(); err != nil {
		return &tools.ToolExecutionResult{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}, nil
	}

	// Run workload if specified
	if p.Workload != "idle" {
		go t.runWorkload(ctx, p.Workload, time.Duration(p.Duration)*time.Second)
	}

	// Wait for monitoring duration
	time.Sleep(time.Duration(p.Duration) * time.Second)

	// Stop monitoring and collect results
	results := monitor.Stop()
	
	// Analyze results
	analysis := analyzeResults(results)
	
	// Generate recommendations
	recommendations := generateResourceRecommendations(analysis)

	response := map[string]interface{}{
		"monitoring_summary": results.Summary,
		"resource_analysis":  analysis,
		"performance_trends": results.Trends,
		"alerts":            results.Alerts,
		"recommendations":   recommendations,
		"configuration":     p,
		"profiles":          results.Profiles,
	}

	return &tools.ToolExecutionResult{
		Success:   true,
		Data:      response,
		Timestamp: time.Now(),
		Duration:  time.Duration(p.Duration) * time.Second,
		Metadata: map[string]interface{}{
			"samples_collected": len(results.Summary.Samples),
			"alerts_triggered":  len(results.Alerts),
			"peak_memory":       results.Trends.Memory.PeakUsage,
			"avg_cpu":          results.Trends.CPU.AverageUsage,
		},
	}, nil
}

// NewSystemMonitor creates a new system monitor
func NewSystemMonitor(ctx context.Context, params *ResourceMonitorParams) *SystemMonitor {
	monitorCtx, cancel := context.WithCancel(ctx)
	
	monitor := &SystemMonitor{
		sampleInterval: time.Duration(params.SampleRate) * time.Millisecond,
		maxSamples:     int(params.Duration * 1000 / params.SampleRate),
		samples:        make([]ResourceSnapshot, 0),
		ctx:            monitorCtx,
		cancel:         cancel,
	}

	// Initialize trackers based on enabled features
	if params.EnableCPU {
		monitor.cpuTracker = &CPUTracker{}
	}
	if params.EnableMemory {
		monitor.memoryTracker = &MemoryTracker{
			leakDetector: &MemoryLeakDetector{
				objects: make(map[string]*ObjectTracker),
				threshold: uint64(params.MemLimit * 1024 * 1024),
			},
		}
		runtime.ReadMemStats(&monitor.memoryTracker.baseline)
	}
	if params.EnableGC {
		monitor.gcTracker = &GCTracker{}
		debug.ReadGCStats(&monitor.gcTracker.lastGCStats)
	}
	
	monitor.goroutineTracker = &GoroutineTracker{
		goroutineMap: make(map[uint64]*GoroutineInfo),
	}
	
	if params.EnableProfile {
		monitor.profiler = &SystemProfiler{enabled: true}
	}

	// Setup alert manager
	monitor.alertManager = &AlertManager{
		thresholds: params.AlertThresholds,
		alerts:     make([]Alert, 0),
	}

	// Set GC target if specified
	if params.GCTarget > 0 {
		debug.SetGCPercent(params.GCTarget)
	}

	return monitor
}

// Start begins resource monitoring
func (m *SystemMonitor) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.running {
		return fmt.Errorf("monitor already running")
	}
	
	m.running = true
	m.startTime = time.Now()
	
	// Start monitoring goroutine
	go m.monitorLoop()
	
	return nil
}

// monitorLoop is the main monitoring loop
func (m *SystemMonitor) monitorLoop() {
	ticker := time.NewTicker(m.sampleInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			snapshot := m.takeSample()
			m.processSample(snapshot)
			
		case <-m.ctx.Done():
			return
		}
	}
}

// takeSample captures current system state
func (m *SystemMonitor) takeSample() ResourceSnapshot {
	snapshot := ResourceSnapshot{
		Timestamp: time.Now(),
	}
	
	// Collect CPU metrics
	if m.cpuTracker != nil {
		snapshot.CPU = m.collectCPUMetrics()
	}
	
	// Collect memory metrics
	if m.memoryTracker != nil {
		snapshot.Memory = m.collectMemoryMetrics()
	}
	
	// Collect GC metrics
	if m.gcTracker != nil {
		snapshot.GC = m.collectGCMetrics()
	}
	
	// Collect goroutine metrics
	snapshot.Goroutines = m.collectGoroutineMetrics()
	
	// Collect system metrics
	snapshot.System = m.collectSystemMetrics()
	
	return snapshot
}

// Helper methods for collecting different types of metrics

func (m *SystemMonitor) collectCPUMetrics() CPUMetrics {
	// Note: In a real implementation, you would use system calls or libraries
	// to get actual CPU usage. This is a simplified version.
	return CPUMetrics{
		UserPercent:    float64(runtime.NumGoroutine()) / 1000.0 * 10, // Approximation
		SystemPercent:  5.0, // Placeholder
		IdlePercent:    85.0, // Placeholder
		CPUCount:       runtime.NumCPU(),
		ProcessCPUTime: time.Since(m.startTime),
	}
}

func (m *SystemMonitor) collectMemoryMetrics() MemoryMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	return MemoryMetrics{
		Alloc:        memStats.Alloc,
		TotalAlloc:   memStats.TotalAlloc,
		Sys:          memStats.Sys,
		Lookups:      memStats.Lookups,
		Mallocs:      memStats.Mallocs,
		Frees:        memStats.Frees,
		HeapAlloc:    memStats.HeapAlloc,
		HeapSys:      memStats.HeapSys,
		HeapIdle:     memStats.HeapIdle,
		HeapInuse:    memStats.HeapInuse,
		HeapReleased: memStats.HeapReleased,
		HeapObjects:  memStats.HeapObjects,
		StackInuse:   memStats.StackInuse,
		StackSys:     memStats.StackSys,
		MSpanInuse:   memStats.MSpanInuse,
		MSpanSys:     memStats.MSpanSys,
		MCacheInuse:  memStats.MCacheInuse,
		MCacheSys:    memStats.MCacheSys,
		BuckHashSys:  memStats.BuckHashSys,
		GCSys:        memStats.GCSys,
		OtherSys:     memStats.OtherSys,
	}
}

func (m *SystemMonitor) collectGCMetrics() GCMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// Calculate pause statistics
	var pauseTotal time.Duration
	var pauseMax time.Duration
	var pauseMin time.Duration = time.Hour // Initialize to large value
	
	recentPauses := memStats.PauseNs[(memStats.NumGC+255)%256:]
	if len(recentPauses) > 0 {
		for _, pause := range recentPauses {
			if pause > 0 {
				pauseDuration := time.Duration(pause)
				pauseTotal += pauseDuration
				if pauseDuration > pauseMax {
					pauseMax = pauseDuration
				}
				if pauseDuration < pauseMin {
					pauseMin = pauseDuration
				}
			}
		}
	}
	
	var pauseAvg time.Duration
	if memStats.NumGC > 0 {
		pauseAvg = time.Duration(memStats.PauseTotalNs) / time.Duration(memStats.NumGC)
	}
	
	return GCMetrics{
		NumGC:         memStats.NumGC,
		NumForcedGC:   memStats.NumForcedGC,
		GCCPUFraction: memStats.GCCPUFraction,
		LastGC:        time.Unix(0, int64(memStats.LastGC)),
		PauseTotal:    time.Duration(memStats.PauseTotalNs),
		PauseAvg:      pauseAvg,
		PauseMax:      pauseMax,
		PauseMin:      pauseMin,
		NextGC:        memStats.NextGC,
		EnablePercent: int(debug.SetGCPercent(-1)), // Get current value
	}
}

func (m *SystemMonitor) collectGoroutineMetrics() GoroutineMetrics {
	total := runtime.NumGoroutine()
	
	// In a real implementation, you would parse goroutine stacks
	// This is a simplified version
	return GoroutineMetrics{
		Total:   total,
		Running: total / 4,     // Approximation
		Runnable: total / 4,    // Approximation
		Waiting: total / 2,     // Approximation
		StackSizes: GoroutineStackStats{
			AvgSize:   8192, // Typical stack size
			TotalSize: int64(total * 8192),
		},
	}
}

func (m *SystemMonitor) collectSystemMetrics() SystemMetrics {
	return SystemMetrics{
		ProcessUptime: time.Since(m.startTime),
		ThreadCount:   runtime.NumGoroutine(),
	}
}

// processSample processes and stores a sample
func (m *SystemMonitor) processSample(snapshot ResourceSnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Store sample
	if len(m.samples) >= m.maxSamples {
		// Remove oldest sample
		m.samples = m.samples[1:]
	}
	m.samples = append(m.samples, snapshot)
	
	// Check for alerts
	m.checkAlerts(snapshot)
}

// checkAlerts checks if any metrics exceed thresholds
func (m *SystemMonitor) checkAlerts(snapshot ResourceSnapshot) {
	if m.alertManager == nil {
		return
	}
	
	// Check CPU alerts
	if threshold, ok := m.alertManager.thresholds["cpu_percent"]; ok {
		if snapshot.CPU.UserPercent+snapshot.CPU.SystemPercent > threshold {
			alert := Alert{
				Type:      "cpu",
				Level:     "warning",
				Message:   fmt.Sprintf("CPU usage %.2f%% exceeds threshold %.2f%%", snapshot.CPU.UserPercent+snapshot.CPU.SystemPercent, threshold),
				Value:     snapshot.CPU.UserPercent + snapshot.CPU.SystemPercent,
				Threshold: threshold,
				Timestamp: snapshot.Timestamp,
			}
			m.alertManager.alerts = append(m.alertManager.alerts, alert)
		}
	}
	
	// Check memory alerts
	if threshold, ok := m.alertManager.thresholds["memory_percent"]; ok {
		memPercent := float64(snapshot.Memory.HeapInuse) / float64(snapshot.Memory.HeapSys) * 100
		if memPercent > threshold {
			alert := Alert{
				Type:      "memory",
				Level:     "warning", 
				Message:   fmt.Sprintf("Memory usage %.2f%% exceeds threshold %.2f%%", memPercent, threshold),
				Value:     memPercent,
				Threshold: threshold,
				Timestamp: snapshot.Timestamp,
			}
			m.alertManager.alerts = append(m.alertManager.alerts, alert)
		}
	}
	
	// Check GC pause alerts
	if threshold, ok := m.alertManager.thresholds["gc_pause_ms"]; ok {
		pauseMs := float64(snapshot.GC.PauseMax) / float64(time.Millisecond)
		if pauseMs > threshold {
			alert := Alert{
				Type:      "gc_pause",
				Level:     "warning",
				Message:   fmt.Sprintf("GC pause %.2fms exceeds threshold %.2fms", pauseMs, threshold),
				Value:     pauseMs,
				Threshold: threshold,
				Timestamp: snapshot.Timestamp,
			}
			m.alertManager.alerts = append(m.alertManager.alerts, alert)
		}
	}
	
	// Check goroutine count alerts
	if threshold, ok := m.alertManager.thresholds["goroutine_count"]; ok {
		if float64(snapshot.Goroutines.Total) > threshold {
			alert := Alert{
				Type:      "goroutines",
				Level:     "warning",
				Message:   fmt.Sprintf("Goroutine count %d exceeds threshold %.0f", snapshot.Goroutines.Total, threshold),
				Value:     float64(snapshot.Goroutines.Total),
				Threshold: threshold,
				Timestamp: snapshot.Timestamp,
			}
			m.alertManager.alerts = append(m.alertManager.alerts, alert)
		}
	}
}

// Stop stops monitoring and returns results
func (m *SystemMonitor) Stop() *MonitoringResults {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.running = false
	m.cancel()
	
	endTime := time.Now()
	duration := endTime.Sub(m.startTime)
	
	return &MonitoringResults{
		Summary: MonitoringSummary{
			StartTime:    m.startTime,
			EndTime:      endTime,
			Duration:     duration,
			Samples:      m.samples,
			SampleCount:  len(m.samples),
		},
		Trends:   calculateTrends(m.samples),
		Alerts:   m.alertManager.alerts,
		Profiles: m.collectProfiles(),
	}
}

// MonitoringResults contains the results of monitoring
type MonitoringResults struct {
	Summary  MonitoringSummary `json:"summary"`
	Trends   TrendAnalysis     `json:"trends"`
	Alerts   []Alert          `json:"alerts"`
	Profiles ProfileData      `json:"profiles"`
}

type MonitoringSummary struct {
	StartTime   time.Time          `json:"start_time"`
	EndTime     time.Time          `json:"end_time"`
	Duration    time.Duration      `json:"duration"`
	Samples     []ResourceSnapshot `json:"samples"`
	SampleCount int               `json:"sample_count"`
}

type TrendAnalysis struct {
	CPU     CPUTrend     `json:"cpu"`
	Memory  MemoryTrend  `json:"memory"`
	GC      GCTrend      `json:"gc"`
	System  SystemTrend  `json:"system"`
}

type CPUTrend struct {
	AverageUsage   float64 `json:"average_usage"`
	PeakUsage      float64 `json:"peak_usage"`
	TrendDirection string  `json:"trend_direction"`
	Volatility     float64 `json:"volatility"`
}

type MemoryTrend struct {
	AverageUsage   uint64  `json:"average_usage"`
	PeakUsage      uint64  `json:"peak_usage"`
	GrowthRate     float64 `json:"growth_rate"`
	LeaksDetected  bool    `json:"leaks_detected"`
	FragmentationLevel float64 `json:"fragmentation_level"`
}

type GCTrend struct {
	Frequency      float64       `json:"frequency"`
	AveragePause   time.Duration `json:"average_pause"`
	TrendDirection string        `json:"trend_direction"`
	Efficiency     float64       `json:"efficiency"`
}

type SystemTrend struct {
	GoroutineGrowth  float64 `json:"goroutine_growth"`
	ResourceStability float64 `json:"resource_stability"`
}

type ProfileData struct {
	CPUProfile       []byte `json:"cpu_profile,omitempty"`
	MemoryProfile    []byte `json:"memory_profile,omitempty"`
	GoroutineProfile []byte `json:"goroutine_profile,omitempty"`
}

// Helper functions

func parseResourceMonitorParams(params map[string]interface{}) (*ResourceMonitorParams, error) {
	p := &ResourceMonitorParams{
		Duration:      60,
		SampleRate:    1000,
		EnableCPU:     true,
		EnableMemory:  true,
		EnableGC:      true,
		EnableProfile: false,
		Workload:      "light",
		GCTarget:      100,
		MemLimit:      0,
		AlertThresholds: map[string]float64{
			"cpu_percent":     80.0,
			"memory_percent":  90.0,
			"gc_pause_ms":     100.0,
			"goroutine_count": 10000.0,
		},
	}
	
	// Parse parameters (simplified version)
	if v, ok := params["duration_seconds"].(int); ok {
		p.Duration = v
	}
	if v, ok := params["sample_rate_ms"].(int); ok {
		p.SampleRate = v
	}
	if v, ok := params["workload"].(string); ok {
		p.Workload = v
	}
	// ... parse other parameters
	
	return p, nil
}

func (t *ResourceMonitorTool) runWorkload(ctx context.Context, workloadType string, duration time.Duration) {
	endTime := time.Now().Add(duration)
	
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		switch workloadType {
		case "light":
			t.lightWorkload()
		case "medium":
			t.mediumWorkload()
		case "heavy":
			t.heavyWorkload()
		}
		
		time.Sleep(time.Millisecond * 10) // Small pause between work
	}
}

func (t *ResourceMonitorTool) lightWorkload() {
	// Light CPU work
	for i := 0; i < 1000; i++ {
		_ = i * i
	}
}

func (t *ResourceMonitorTool) mediumWorkload() {
	// Medium CPU and memory work
	data := make([]int, 10000)
	for i := range data {
		data[i] = i * i
	}
}

func (t *ResourceMonitorTool) heavyWorkload() {
	// Heavy CPU and memory work
	data := make([][]int, 100)
	for i := range data {
		data[i] = make([]int, 1000)
		for j := range data[i] {
			data[i][j] = i * j
		}
	}
	runtime.GC() // Force GC
}

func calculateTrends(samples []ResourceSnapshot) TrendAnalysis {
	if len(samples) == 0 {
		return TrendAnalysis{}
	}
	
	// Calculate CPU trends
	var totalCPU float64
	var peakCPU float64
	for _, sample := range samples {
		cpu := sample.CPU.UserPercent + sample.CPU.SystemPercent
		totalCPU += cpu
		if cpu > peakCPU {
			peakCPU = cpu
		}
	}
	
	cpuTrend := CPUTrend{
		AverageUsage: totalCPU / float64(len(samples)),
		PeakUsage:    peakCPU,
		TrendDirection: "stable", // Simplified
	}
	
	// Calculate memory trends
	var totalMemory uint64
	var peakMemory uint64
	for _, sample := range samples {
		mem := sample.Memory.HeapInuse
		totalMemory += mem
		if mem > peakMemory {
			peakMemory = mem
		}
	}
	
	memoryTrend := MemoryTrend{
		AverageUsage: totalMemory / uint64(len(samples)),
		PeakUsage:    peakMemory,
		GrowthRate:   0.0, // Simplified
	}
	
	// Calculate GC trends
	var totalPause time.Duration
	var gcCount int
	for _, sample := range samples {
		totalPause += sample.GC.PauseAvg
		if sample.GC.NumGC > 0 {
			gcCount++
		}
	}
	
	gcTrend := GCTrend{
		Frequency:    float64(gcCount) / float64(len(samples)),
		AveragePause: totalPause / time.Duration(max(gcCount, 1)),
		TrendDirection: "stable",
	}
	
	return TrendAnalysis{
		CPU:    cpuTrend,
		Memory: memoryTrend,
		GC:     gcTrend,
	}
}

func analyzeResults(results *MonitoringResults) map[string]interface{} {
	return map[string]interface{}{
		"cpu_analysis":    results.Trends.CPU,
		"memory_analysis": results.Trends.Memory,
		"gc_analysis":     results.Trends.GC,
		"stability_score": calculateStabilityScore(results),
		"efficiency_score": calculateEfficiencyScore(results),
	}
}

func calculateStabilityScore(results *MonitoringResults) float64 {
	// Simplified stability calculation
	score := 100.0
	
	// Reduce score for high volatility
	if results.Trends.CPU.Volatility > 20 {
		score -= 20
	}
	
	// Reduce score for memory growth
	if results.Trends.Memory.GrowthRate > 10 {
		score -= 30
	}
	
	// Reduce score for alerts
	score -= float64(len(results.Alerts)) * 5
	
	if score < 0 {
		score = 0
	}
	
	return score
}

func calculateEfficiencyScore(results *MonitoringResults) float64 {
	// Simplified efficiency calculation
	score := 100.0
	
	// Reduce score for high GC frequency
	if results.Trends.GC.Frequency > 0.1 {
		score -= 20
	}
	
	// Reduce score for long GC pauses
	if results.Trends.GC.AveragePause > time.Millisecond*50 {
		score -= 25
	}
	
	if score < 0 {
		score = 0
	}
	
	return score
}

func generateResourceRecommendations(analysis map[string]interface{}) []string {
	recommendations := []string{}
	
	if cpuAnalysis, ok := analysis["cpu_analysis"].(CPUTrend); ok {
		if cpuAnalysis.AverageUsage > 70 {
			recommendations = append(recommendations, "Consider optimizing CPU-intensive operations or scaling horizontally")
		}
	}
	
	if memAnalysis, ok := analysis["memory_analysis"].(MemoryTrend); ok {
		if memAnalysis.GrowthRate > 5 {
			recommendations = append(recommendations, "Memory usage is growing - check for memory leaks")
		}
		if memAnalysis.FragmentationLevel > 30 {
			recommendations = append(recommendations, "High memory fragmentation detected - consider object pooling")
		}
	}
	
	if gcAnalysis, ok := analysis["gc_analysis"].(GCTrend); ok {
		if gcAnalysis.AveragePause > time.Millisecond*100 {
			recommendations = append(recommendations, "GC pauses are high - consider tuning GOGC or reducing allocation rate")
		}
	}
	
	return recommendations
}

func (m *SystemMonitor) collectProfiles() ProfileData {
	if m.profiler == nil || !m.profiler.enabled {
		return ProfileData{}
	}
	
	// In a real implementation, you would collect actual profiles
	// This is a placeholder
	return ProfileData{
		CPUProfile:       []byte("cpu profile data"),
		MemoryProfile:    []byte("memory profile data"),
		GoroutineProfile: []byte("goroutine profile data"),
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	// Example usage
	params := map[string]interface{}{
		"duration_seconds":      30,
		"sample_rate_ms":       1000,
		"workload":             "medium",
		"enable_profiling":     true,
	}
	
	ctx := context.Background()
	monitor := &ResourceMonitorTool{}
	
	result, err := monitor.Execute(ctx, params)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Printf("Resource Monitoring Results:\n")
	fmt.Printf("Success: %t\n", result.Success)
	fmt.Printf("Duration: %v\n", result.Duration)
	
	if data, ok := result.Data.(map[string]interface{}); ok {
		if analysis, ok := data["resource_analysis"].(map[string]interface{}); ok {
			fmt.Printf("Analysis: %+v\n", analysis)
		}
		if recs, ok := data["recommendations"].([]string); ok {
			fmt.Printf("Recommendations:\n")
			for _, rec := range recs {
				fmt.Printf("  - %s\n", rec)
			}
		}
	}
}