package memory

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// MemoryManager handles memory pressure monitoring and adaptive behavior
type MemoryManager struct {
	mu                sync.RWMutex
	logger            *zap.Logger
	
	// Memory thresholds
	lowMemoryThreshold  uint64 // Bytes
	highMemoryThreshold uint64 // Bytes
	criticalThreshold   uint64 // Bytes
	
	// Current state
	currentMemoryUsage  atomic.Uint64
	memoryPressureLevel atomic.Int32 // 0=normal, 1=low, 2=high, 3=critical
	
	// Adaptive settings
	adaptiveBufferSizes map[string]int
	bufferSizeMutex     sync.RWMutex
	
	// Monitoring
	monitorInterval time.Duration
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	
	// Callbacks for memory pressure events
	onMemoryPressure []func(level MemoryPressureLevel)
	callbacksMutex   sync.RWMutex
	
	// Metrics
	metrics *MemoryMetrics
}

// MemoryPressureLevel represents the current memory pressure
type MemoryPressureLevel int

const (
	MemoryPressureNormal MemoryPressureLevel = iota
	MemoryPressureLow
	MemoryPressureHigh
	MemoryPressureCritical
)

// String returns the string representation of memory pressure level
func (m MemoryPressureLevel) String() string {
	switch m {
	case MemoryPressureNormal:
		return "normal"
	case MemoryPressureLow:
		return "low"
	case MemoryPressureHigh:
		return "high"
	case MemoryPressureCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// MemoryMetrics tracks memory usage metrics
type MemoryMetrics struct {
	mu                     sync.RWMutex
	TotalAllocated         uint64
	HeapInUse              uint64
	StackInUse             uint64
	NumGC                  uint32
	LastGCTime             time.Time
	PressureEvents         map[MemoryPressureLevel]uint64
	BufferResizeEvents     uint64
	LastPressureChangeTime time.Time
	GCPauseTotal           time.Duration
}

// MemoryManagerConfig configures the memory manager
type MemoryManagerConfig struct {
	// Memory thresholds as percentage of system memory
	LowMemoryPercent      float64       // Default: 70%
	HighMemoryPercent     float64       // Default: 85%
	CriticalMemoryPercent float64       // Default: 95%
	MonitorInterval       time.Duration // Default: 5s
	Logger                *zap.Logger
}

// DefaultMemoryManagerConfig returns default configuration
func DefaultMemoryManagerConfig() *MemoryManagerConfig {
	return &MemoryManagerConfig{
		LowMemoryPercent:      70.0,
		HighMemoryPercent:     85.0,
		CriticalMemoryPercent: 95.0,
		MonitorInterval:       5 * time.Second,
		Logger:                zap.NewNop(),
	}
}

// NewMemoryManager creates a new memory manager
func NewMemoryManager(config *MemoryManagerConfig) *MemoryManager {
	if config == nil {
		config = DefaultMemoryManagerConfig()
	}

	// Get system memory
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	totalMemory := memStats.Sys

	ctx, cancel := context.WithCancel(context.Background())

	mm := &MemoryManager{
		logger:              config.Logger,
		lowMemoryThreshold:  uint64(float64(totalMemory) * config.LowMemoryPercent / 100),
		highMemoryThreshold: uint64(float64(totalMemory) * config.HighMemoryPercent / 100),
		criticalThreshold:   uint64(float64(totalMemory) * config.CriticalMemoryPercent / 100),
		monitorInterval:     config.MonitorInterval,
		ctx:                 ctx,
		cancel:              cancel,
		adaptiveBufferSizes: make(map[string]int),
		metrics: &MemoryMetrics{
			PressureEvents: make(map[MemoryPressureLevel]uint64),
		},
		onMemoryPressure: make([]func(MemoryPressureLevel), 0),
	}

	// Initialize current memory usage
	mm.updateMemoryUsage()

	return mm
}

// Start begins memory monitoring
func (mm *MemoryManager) Start() {
	mm.wg.Add(1)
	go mm.monitorMemory()
}

// Stop stops memory monitoring
func (mm *MemoryManager) Stop() {
	mm.cancel()
	mm.wg.Wait()
}

// OnMemoryPressure registers a callback for memory pressure events
func (mm *MemoryManager) OnMemoryPressure(callback func(MemoryPressureLevel)) {
	mm.callbacksMutex.Lock()
	defer mm.callbacksMutex.Unlock()
	mm.onMemoryPressure = append(mm.onMemoryPressure, callback)
}

// GetAdaptiveBufferSize returns the adaptive buffer size for a given key
func (mm *MemoryManager) GetAdaptiveBufferSize(key string, defaultSize int) int {
	// Check current memory pressure
	pressure := MemoryPressureLevel(mm.memoryPressureLevel.Load())
	
	// Adjust buffer size based on memory pressure
	adjustedSize := defaultSize
	switch pressure {
	case MemoryPressureLow:
		adjustedSize = defaultSize * 3 / 4 // 75% of default
	case MemoryPressureHigh:
		adjustedSize = defaultSize / 2 // 50% of default
	case MemoryPressureCritical:
		adjustedSize = defaultSize / 4 // 25% of default
	}

	// Store the adjusted size
	mm.bufferSizeMutex.Lock()
	if currentSize, exists := mm.adaptiveBufferSizes[key]; !exists || currentSize != adjustedSize {
		mm.adaptiveBufferSizes[key] = adjustedSize
		mm.metrics.mu.Lock()
		mm.metrics.BufferResizeEvents++
		mm.metrics.mu.Unlock()
	}
	mm.bufferSizeMutex.Unlock()

	return adjustedSize
}

// GetMemoryPressureLevel returns the current memory pressure level
func (mm *MemoryManager) GetMemoryPressureLevel() MemoryPressureLevel {
	return MemoryPressureLevel(mm.memoryPressureLevel.Load())
}

// GetMetrics returns current memory metrics
func (mm *MemoryManager) GetMetrics() MemoryMetrics {
	mm.metrics.mu.RLock()
	defer mm.metrics.mu.RUnlock()
	return *mm.metrics
}

// ForceGC forces a garbage collection if memory pressure is high
func (mm *MemoryManager) ForceGC() {
	pressure := MemoryPressureLevel(mm.memoryPressureLevel.Load())
	if pressure >= MemoryPressureHigh {
		runtime.GC()
		if mm.logger != nil {
			mm.logger.Info("Forced garbage collection due to memory pressure",
				zap.String("pressure", pressure.String()))
		}
	}
}

// monitorMemory continuously monitors memory usage
func (mm *MemoryManager) monitorMemory() {
	defer mm.wg.Done()

	ticker := time.NewTicker(mm.monitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mm.ctx.Done():
			return
		case <-ticker.C:
			mm.updateMemoryUsage()
			mm.checkMemoryPressure()
		}
	}
}

// updateMemoryUsage updates current memory usage statistics
func (mm *MemoryManager) updateMemoryUsage() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	mm.currentMemoryUsage.Store(memStats.Alloc)

	mm.metrics.mu.Lock()
	mm.metrics.TotalAllocated = memStats.TotalAlloc
	mm.metrics.HeapInUse = memStats.HeapInuse
	mm.metrics.StackInUse = memStats.StackInuse
	mm.metrics.NumGC = memStats.NumGC
	mm.metrics.GCPauseTotal = time.Duration(memStats.PauseTotalNs)
	if memStats.NumGC > 0 {
		mm.metrics.LastGCTime = time.Unix(0, int64(memStats.LastGC))
	}
	mm.metrics.mu.Unlock()
}

// checkMemoryPressure checks and updates memory pressure level
func (mm *MemoryManager) checkMemoryPressure() {
	currentUsage := mm.currentMemoryUsage.Load()
	oldLevel := MemoryPressureLevel(mm.memoryPressureLevel.Load())
	newLevel := oldLevel

	// Determine new pressure level
	if currentUsage >= mm.criticalThreshold {
		newLevel = MemoryPressureCritical
	} else if currentUsage >= mm.highMemoryThreshold {
		newLevel = MemoryPressureHigh
	} else if currentUsage >= mm.lowMemoryThreshold {
		newLevel = MemoryPressureLow
	} else {
		newLevel = MemoryPressureNormal
	}

	// Update if changed
	if newLevel != oldLevel {
		mm.memoryPressureLevel.Store(int32(newLevel))
		
		mm.metrics.mu.Lock()
		mm.metrics.PressureEvents[newLevel]++
		mm.metrics.LastPressureChangeTime = time.Now()
		mm.metrics.mu.Unlock()

		if mm.logger != nil {
			mm.logger.Info("Memory pressure level changed",
				zap.String("old_level", oldLevel.String()),
				zap.String("new_level", newLevel.String()),
				zap.Uint64("memory_usage", currentUsage),
				zap.Uint64("threshold", mm.getThresholdForLevel(newLevel)))
		}

		// Notify callbacks
		mm.notifyPressureChange(newLevel)

		// Take adaptive actions
		mm.adaptToMemoryPressure(newLevel)
	}
}

// getThresholdForLevel returns the threshold for a given level
func (mm *MemoryManager) getThresholdForLevel(level MemoryPressureLevel) uint64 {
	switch level {
	case MemoryPressureLow:
		return mm.lowMemoryThreshold
	case MemoryPressureHigh:
		return mm.highMemoryThreshold
	case MemoryPressureCritical:
		return mm.criticalThreshold
	default:
		return 0
	}
}

// notifyPressureChange notifies all registered callbacks
func (mm *MemoryManager) notifyPressureChange(level MemoryPressureLevel) {
	mm.callbacksMutex.RLock()
	callbacks := make([]func(MemoryPressureLevel), len(mm.onMemoryPressure))
	copy(callbacks, mm.onMemoryPressure)
	mm.callbacksMutex.RUnlock()

	// Use a WaitGroup to ensure all callbacks complete before returning
	var wg sync.WaitGroup
	for _, callback := range callbacks {
		wg.Add(1)
		go func(cb func(MemoryPressureLevel)) {
			defer wg.Done()
			// Add timeout to prevent hanging on stuck callbacks
			done := make(chan struct{})
			go func() {
				defer close(done)
				cb(level)
			}()
			
			select {
			case <-done:
				// Callback completed successfully
			case <-time.After(5 * time.Second):
				// Callback timed out, log warning
				if mm.logger != nil {
					mm.logger.Warn("Memory pressure callback timed out")
				}
			case <-mm.ctx.Done():
				// Memory manager is shutting down
				return
			}
		}(callback)
	}
	
	// Wait for all callbacks to complete or timeout, but don't block shutdown
	go func() {
		wg.Wait()
	}()
}

// adaptToMemoryPressure takes adaptive actions based on memory pressure
func (mm *MemoryManager) adaptToMemoryPressure(level MemoryPressureLevel) {
	switch level {
	case MemoryPressureCritical:
		// Force immediate GC
		runtime.GC()
		// Force finalizers to run
		runtime.GC()
		if mm.logger != nil {
			mm.logger.Warn("Critical memory pressure - forced GC")
		}
		
	case MemoryPressureHigh:
		// Schedule GC with proper context cancellation
		go func() {
			select {
			case <-time.After(100 * time.Millisecond):
				runtime.GC()
			case <-mm.ctx.Done():
				// Memory manager is shutting down, abort scheduled GC
				return
			}
		}()
		
	case MemoryPressureLow:
		// Reduce GC frequency
		runtime.GC()
	}
}

// GetMemoryStats returns current runtime memory statistics
func (mm *MemoryManager) GetMemoryStats() runtime.MemStats {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats
}