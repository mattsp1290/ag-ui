package websocket

import (
	"context"
	"log"
	"reflect"
	"runtime"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// startGoroutine starts a goroutine with tracking
func (t *Transport) startGoroutine(name string, fn func()) {
	goroutineCtx, goroutineCancel := context.WithCancel(t.ctx)
	
	// Track the goroutine
	goroutineInfo := &GoroutineInfo{
		Name:      name,
		StartTime: time.Now(),
		LastSeen:  time.Now(),
		Function:  runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name(),
		Context:   goroutineCtx,
		Cancel:    goroutineCancel,
	}
	
	t.goroutinesMutex.Lock()
	t.activeGoroutines[name] = goroutineInfo
	t.stats.mutex.Lock()
	t.stats.ActiveGoroutines++
	t.stats.mutex.Unlock()
	t.goroutinesMutex.Unlock()
	
	go func() {
		defer func() {
			// Clean up goroutine tracking
			t.goroutinesMutex.Lock()
			delete(t.activeGoroutines, name)
			t.stats.mutex.Lock()
			t.stats.ActiveGoroutines--
			t.stats.mutex.Unlock()
			t.goroutinesMutex.Unlock()
			
			// Standard cleanup
			t.wg.Done()
			
			// Handle panic recovery
			if r := recover(); r != nil {
				t.config.Logger.Error("Goroutine panic recovered",
					zap.String("goroutine", name),
					zap.Any("panic", r))
			}
		}()
		
		// Update last seen time periodically
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		go func() {
			for {
				select {
				case <-goroutineCtx.Done():
					return
				case <-ticker.C:
					t.goroutinesMutex.Lock()
					if info, exists := t.activeGoroutines[name]; exists {
						info.LastSeen = time.Now()
					}
					t.goroutinesMutex.Unlock()
				}
			}
		}()
		
		// Run the actual function
		fn()
	}()
}

// handleEventWithBackpressure handles events with proper backpressure control
func (t *Transport) handleEventWithBackpressure(data []byte) {
	// Check if backpressure threshold is reached
	currentUsage := float64(len(t.eventCh)) / float64(cap(t.eventCh)) * 100
	
	if int(currentUsage) >= t.config.BackpressureConfig.BackpressureThresholdPercent {
		t.activateBackpressure()
	}
	
	select {
	case t.eventCh <- data:
		// Successfully sent event
	case <-t.ctx.Done():
		// Transport is shutting down
		return
	default:
		// Event channel is full, apply backpressure
		t.handleDroppedEvent(data)
	}
}

// handleDroppedEvent handles dropped events due to channel backpressure
func (t *Transport) handleDroppedEvent(data []byte) {
	droppedCount := atomic.AddInt64(&t.droppedEvents, 1)
	t.lastDropTime = time.Now()
	
	// Update stats
	t.stats.mutex.Lock()
	t.stats.EventsDropped++
	t.stats.EventsFailed++
	t.stats.mutex.Unlock()
	
	if t.config.BackpressureConfig.EnableBackpressureLogging {
		t.config.Logger.Warn("WebSocket Transport: Dropped event due to backpressure",
			zap.Int64("total_dropped_events", droppedCount),
			zap.Int("data_size", len(data)),
			zap.Int("channel_size", len(t.eventCh)),
			zap.Int("channel_capacity", cap(t.eventCh)))
	}
	
	// Check if we need to take action
	if droppedCount >= t.config.BackpressureConfig.MaxDroppedEvents {
		t.handleBackpressureAction()
	}
}

// activateBackpressure activates backpressure mode
func (t *Transport) activateBackpressure() {
	t.backpressureMutex.Lock()
	defer t.backpressureMutex.Unlock()
	
	if !t.backpressureActive {
		t.backpressureActive = true
		t.stats.mutex.Lock()
		t.stats.BackpressureEvents++
		t.stats.mutex.Unlock()
		
		if t.config.BackpressureConfig.EnableBackpressureLogging {
			t.config.Logger.Warn("WebSocket Transport: Backpressure activated",
				zap.Int("threshold_percent", t.config.BackpressureConfig.BackpressureThresholdPercent),
				zap.Int("channel_size", len(t.eventCh)),
				zap.Int("channel_capacity", cap(t.eventCh)))
		}
	}
}

// deactivateBackpressure deactivates backpressure mode
func (t *Transport) deactivateBackpressure() {
	t.backpressureMutex.Lock()
	defer t.backpressureMutex.Unlock()
	
	if t.backpressureActive {
		t.backpressureActive = false
		if t.config.BackpressureConfig.EnableBackpressureLogging {
			t.config.Logger.Info("WebSocket Transport: Backpressure deactivated")
		}
	}
}

// handleBackpressureAction takes action when maximum dropped items is reached
func (t *Transport) handleBackpressureAction() {
	switch t.config.BackpressureConfig.DropActionType {
	case DropActionLog:
		t.config.Logger.Error("WebSocket Transport: Maximum dropped events reached, continuing with logging only",
			zap.Int64("max_dropped_events", t.config.BackpressureConfig.MaxDroppedEvents))
		
	case DropActionReconnect:
		t.config.Logger.Error("WebSocket Transport: Maximum dropped events reached, reconnecting pool",
			zap.Int64("max_dropped_events", t.config.BackpressureConfig.MaxDroppedEvents))
		
		// Restart the connection pool
		if err := t.pool.Stop(); err != nil {
			t.config.Logger.Error("Error stopping pool for reconnection", zap.Error(err))
		}
		if err := t.pool.Start(t.ctx); err != nil {
			t.config.Logger.Error("Error restarting pool", zap.Error(err))
		} else {
			// Reset counters on successful reconnection
			atomic.StoreInt64(&t.droppedEvents, 0)
		}
		
	case DropActionStop:
		t.config.Logger.Error("WebSocket Transport: Maximum dropped events reached, stopping transport",
			zap.Int64("max_dropped_events", t.config.BackpressureConfig.MaxDroppedEvents))
		if err := t.Stop(); err != nil {
			t.config.Logger.Error("Error during shutdown", zap.Error(err))
		}
		
	case DropActionSlowDown:
		t.config.Logger.Warn("WebSocket Transport: Maximum dropped events reached, applying flow control",
			zap.Int64("max_dropped_events", t.config.BackpressureConfig.MaxDroppedEvents))
		
		// Apply flow control by sleeping briefly
		time.Sleep(100 * time.Millisecond)
		
		// Reset counter to allow gradual recovery
		atomic.StoreInt64(&t.droppedEvents, t.config.BackpressureConfig.MaxDroppedEvents/2)
	}
}

// channelMonitoringLoop monitors channel usage and applies backpressure
func (t *Transport) channelMonitoringLoop() {
	ticker := time.NewTicker(t.config.BackpressureConfig.MonitoringInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-t.monitoringCtx.Done():
			return
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.monitorChannelUsage()
		}
	}
}

// monitorChannelUsage monitors channel usage and manages backpressure
func (t *Transport) monitorChannelUsage() {
	eventChannelUsage := float64(len(t.eventCh)) / float64(cap(t.eventCh)) * 100
	
	// Activate backpressure if threshold is exceeded
	if int(eventChannelUsage) >= t.config.BackpressureConfig.BackpressureThresholdPercent {
		t.activateBackpressure()
	} else if eventChannelUsage < float64(t.config.BackpressureConfig.BackpressureThresholdPercent)*0.7 {
		// Deactivate backpressure if usage drops significantly below threshold
		t.deactivateBackpressure()
	}
	
	// Log channel usage if backpressure logging is enabled
	if t.config.BackpressureConfig.EnableBackpressureLogging && eventChannelUsage > 50 {
		t.config.Logger.Debug("Channel usage monitoring",
			zap.Float64("event_channel_usage_percent", eventChannelUsage),
			zap.Int("event_channel_size", len(t.eventCh)),
			zap.Int("event_channel_capacity", cap(t.eventCh)),
			zap.Int64("dropped_events", atomic.LoadInt64(&t.droppedEvents)))
	}
}

// resourceCleanupLoop performs periodic resource cleanup
func (t *Transport) resourceCleanupLoop() {
	ticker := time.NewTicker(t.config.ResourceCleanupConfig.CleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-t.monitoringCtx.Done():
			return
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.performResourceCleanup()
		}
	}
}

// performResourceCleanup performs resource cleanup tasks
func (t *Transport) performResourceCleanup() {
	now := time.Now()
	
	// Clean up idle goroutines
	if t.config.ResourceCleanupConfig.EnableGoroutineTracking {
		t.goroutinesMutex.Lock()
		for name, info := range t.activeGoroutines {
			if now.Sub(info.LastSeen) > t.config.ResourceCleanupConfig.MaxGoroutineIdleTime {
				t.config.Logger.Warn("Cancelling idle goroutine",
					zap.String("name", name),
					zap.Duration("idle_time", now.Sub(info.LastSeen)))
				
				if info.Cancel != nil {
					info.Cancel()
				}
				delete(t.activeGoroutines, name)
			}
		}
		t.goroutinesMutex.Unlock()
	}
	
	// Log resource usage if monitoring is enabled
	if t.config.ResourceCleanupConfig.EnableResourceMonitoring {
		var m runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m)
		
		t.config.Logger.Debug("Resource usage monitoring",
			zap.Int64("active_goroutines", t.stats.ActiveGoroutines),
			zap.Uint64("memory_alloc_mb", m.Alloc/1024/1024),
			zap.Uint64("memory_sys_mb", m.Sys/1024/1024),
			zap.Uint32("gc_cycles", m.NumGC))
	}
}

// RegisterCleanupFunc registers a function to be called during shutdown
func (t *Transport) RegisterCleanupFunc(cleanup func() error) {
	t.cleanupMutex.Lock()
	defer t.cleanupMutex.Unlock()
	t.resourceCleanup = append(t.resourceCleanup, cleanup)
}

// GetBackpressureStats returns current backpressure statistics
func (t *Transport) GetBackpressureStats() BackpressureStats {
	t.backpressureMutex.RLock()
	defer t.backpressureMutex.RUnlock()
	
	eventChannelUsage := float64(len(t.eventCh)) / float64(cap(t.eventCh)) * 100
	
	return BackpressureStats{
		DroppedEvents:        atomic.LoadInt64(&t.droppedEvents),
		BackpressureActive:   t.backpressureActive,
		EventChannelUsage:    eventChannelUsage,
		LastDropTime:         t.lastDropTime,
		EventChannelCapacity: cap(t.eventCh),
		ThresholdPercent:     t.config.BackpressureConfig.BackpressureThresholdPercent,
	}
}

// BackpressureStats contains backpressure monitoring statistics
type BackpressureStats struct {
	DroppedEvents        int64     `json:"dropped_events"`
	BackpressureActive   bool      `json:"backpressure_active"`
	EventChannelUsage    float64   `json:"event_channel_usage_percent"`
	LastDropTime         time.Time `json:"last_drop_time"`
	EventChannelCapacity int       `json:"event_channel_capacity"`
	ThresholdPercent     int       `json:"threshold_percent"`
}

// GetGoroutineStats returns current goroutine statistics
func (t *Transport) GetGoroutineStats() map[string]GoroutineStats {
	t.goroutinesMutex.RLock()
	defer t.goroutinesMutex.RUnlock()
	
	stats := make(map[string]GoroutineStats)
	now := time.Now()
	
	for name, info := range t.activeGoroutines {
		stats[name] = GoroutineStats{
			Name:      info.Name,
			StartTime: info.StartTime,
			LastSeen:  info.LastSeen,
			Duration:  now.Sub(info.StartTime),
			IdleTime:  now.Sub(info.LastSeen),
		}
	}
	
	return stats
}

// GoroutineStats contains goroutine monitoring statistics
type GoroutineStats struct {
	Name      string        `json:"name"`
	StartTime time.Time     `json:"start_time"`
	LastSeen  time.Time     `json:"last_seen"`
	Duration  time.Duration `json:"duration"`
	IdleTime  time.Duration `json:"idle_time"`
}

// ResetBackpressureStats resets backpressure statistics
func (t *Transport) ResetBackpressureStats() {
	atomic.StoreInt64(&t.droppedEvents, 0)
	t.deactivateBackpressure()
}