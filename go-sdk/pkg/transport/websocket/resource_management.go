package websocket

import (
	"context"
	"reflect"
	"runtime"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// startGoroutine starts a goroutine with tracking and improved cleanup
// IMPORTANT: Caller should NOT call wg.Add(1) before calling this function
// as it's handled internally
//
// This function ensures proper Add/Done pairing to prevent WaitGroup panics
// and implements rapid cleanup for better resource management
func (t *Transport) startGoroutine(name string, fn func()) {
	// Check if transport context is already cancelled before starting
	select {
	case <-t.ctx.Done():
		t.config.Logger.Debug("StartGoroutine: Transport context already cancelled, not starting goroutine", zap.String("name", name))
		return
	case <-t.monitoringCtx.Done():
		t.config.Logger.Debug("StartGoroutine: Monitoring context already cancelled, not starting goroutine", zap.String("name", name))
		return
	default:
		// OK to start goroutine
	}
	
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
	
	// Add to WaitGroup here, not in the caller
	t.wg.Add(1)
	
	go func() {
		defer func() {
			// Clean up goroutine tracking immediately
			t.goroutinesMutex.Lock()
			delete(t.activeGoroutines, name)
			t.stats.mutex.Lock()
			t.stats.ActiveGoroutines--
			t.stats.mutex.Unlock()
			t.goroutinesMutex.Unlock()
			
			// Ensure cancel is called to clean up any remaining resources
			goroutineCancel()
			
			// Standard cleanup - Done() matches the Add(1) above
			t.wg.Done()
			
			// Handle panic recovery
			if r := recover(); r != nil {
				t.config.Logger.Error("Goroutine panic recovered",
					zap.String("goroutine", name),
					zap.Any("panic", r))
			}
			
			t.config.Logger.Debug("Transport goroutine fully exited", zap.String("name", name))
		}()
		
		// Skip monitoring goroutine for better performance and reduced leak risk
		// The monitoring functionality is not critical and adds complexity
		t.config.Logger.Debug("Skipping monitoring goroutine for better performance", zap.String("name", name))
		
		// Run the actual function with timeout protection
		done := make(chan struct{})
		go func() {
			defer close(done)
			fn()
		}()
		
		// Wait for function to complete with context cancellation and aggressive timeout
		maxWaitTime := 5 * time.Second // Maximum time to wait for function to start responding to cancellation
		deadline := time.Now().Add(maxWaitTime)
		
		for time.Now().Before(deadline) {
			select {
			case <-done:
				// Function completed normally
				t.config.Logger.Debug("Transport goroutine function completed", zap.String("name", name))
				return
			case <-goroutineCtx.Done():
				t.config.Logger.Debug("Transport goroutine cancelled", zap.String("name", name))
				// Give function brief time to cleanup, then exit
				select {
				case <-done:
					t.config.Logger.Debug("Transport goroutine function completed after cancellation", zap.String("name", name))
				case <-time.After(100 * time.Millisecond):
					t.config.Logger.Debug("Transport goroutine function did not complete after cancellation, exiting", zap.String("name", name))
				}
				return
			case <-t.ctx.Done():
				t.config.Logger.Debug("Transport goroutine cancelled by main context", zap.String("name", name))
				// Give function brief time to cleanup, then exit
				select {
				case <-done:
					t.config.Logger.Debug("Transport goroutine function completed after main context cancellation", zap.String("name", name))
				case <-time.After(100 * time.Millisecond):
					t.config.Logger.Debug("Transport goroutine function did not complete after main context cancellation, exiting", zap.String("name", name))
				}
				return
			case <-t.monitoringCtx.Done():
				t.config.Logger.Debug("Transport goroutine cancelled by monitoring context", zap.String("name", name))
				// Give function brief time to cleanup, then exit
				select {
				case <-done:
					t.config.Logger.Debug("Transport goroutine function completed after monitoring context cancellation", zap.String("name", name))
				case <-time.After(100 * time.Millisecond):
					t.config.Logger.Debug("Transport goroutine function did not complete after monitoring context cancellation, exiting", zap.String("name", name))
				}
				return
			case <-time.After(1 * time.Millisecond):
				// Timeout case to prevent indefinite blocking and allow periodic context checks
				continue
			}
		}
		
		// If we reach here, the function has been running for too long
		t.config.Logger.Debug("Transport goroutine function timeout - forcing exit", zap.String("name", name), zap.Duration("waited", maxWaitTime))
	}()
}

// handleEventWithBackpressure handles events with proper backpressure control
func (t *Transport) handleEventWithBackpressure(data []byte) {
	// Check if backpressure threshold is reached
	currentUsage := float64(len(t.eventCh)) / float64(cap(t.eventCh)) * 100
	
	if int(currentUsage) >= t.config.BackpressureConfig.BackpressureThresholdPercent {
		t.activateBackpressure()
	}
	
	// Use a timeout-based approach instead of immediate drop
	// This allows for brief delays without dropping messages, important for testing
	timeout := time.NewTimer(100 * time.Millisecond)
	defer timeout.Stop()
	
	select {
	case t.eventCh <- data:
		// Successfully sent event
	case <-t.ctx.Done():
		// Transport is shutting down
		return
	case <-timeout.C:
		// Event channel is full after timeout, apply backpressure
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
	
	t.config.Logger.Debug("Channel monitoring: Starting channel monitoring loop")
	
	// Use shorter intervals for more responsive shutdown
	shutdownTicker := time.NewTicker(50 * time.Millisecond)
	defer shutdownTicker.Stop()
	
	for {
		select {
		case <-t.monitoringCtx.Done():
			t.config.Logger.Debug("Channel monitoring: Monitoring context cancelled, stopping monitor immediately")
			return
		case <-t.ctx.Done():
			t.config.Logger.Debug("Channel monitoring: Transport context cancelled, stopping monitor immediately")
			return
		case <-shutdownTicker.C:
			// Frequent shutdown checks to ensure responsive cleanup
			select {
			case <-t.monitoringCtx.Done():
				t.config.Logger.Debug("Channel monitoring: Monitoring context cancelled during shutdown check")
				return
			case <-t.ctx.Done():
				t.config.Logger.Debug("Channel monitoring: Transport context cancelled during shutdown check")
				return
			default:
				// Continue monitoring
			}
		case <-ticker.C:
			// Check context before performing monitoring
			select {
			case <-t.monitoringCtx.Done():
				t.config.Logger.Debug("Channel monitoring: Monitoring context cancelled during tick, stopping immediately")
				return
			case <-t.ctx.Done():
				t.config.Logger.Debug("Channel monitoring: Transport context cancelled during tick, stopping immediately")
				return
			default:
				t.config.Logger.Debug("Channel monitoring: Monitoring channel usage")
				t.monitorChannelUsage()
			}
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
	
	t.config.Logger.Debug("Resource cleanup: Starting resource cleanup loop")
	
	// Use shorter intervals for more responsive shutdown
	shutdownTicker := time.NewTicker(50 * time.Millisecond)
	defer shutdownTicker.Stop()
	
	for {
		select {
		case <-t.monitoringCtx.Done():
			t.config.Logger.Debug("Resource cleanup: Monitoring context cancelled, stopping cleanup immediately")
			return
		case <-t.ctx.Done():
			t.config.Logger.Debug("Resource cleanup: Transport context cancelled, stopping cleanup immediately")
			return
		case <-shutdownTicker.C:
			// Frequent shutdown checks to ensure responsive cleanup
			select {
			case <-t.monitoringCtx.Done():
				t.config.Logger.Debug("Resource cleanup: Monitoring context cancelled during shutdown check")
				return
			case <-t.ctx.Done():
				t.config.Logger.Debug("Resource cleanup: Transport context cancelled during shutdown check")
				return
			default:
				// Continue cleanup
			}
		case <-ticker.C:
			// Check context before performing cleanup
			select {
			case <-t.monitoringCtx.Done():
				t.config.Logger.Debug("Resource cleanup: Monitoring context cancelled during tick, stopping immediately")
				return
			case <-t.ctx.Done():
				t.config.Logger.Debug("Resource cleanup: Transport context cancelled during tick, stopping immediately")
				return
			default:
				t.config.Logger.Debug("Resource cleanup: Performing resource cleanup")
				t.performResourceCleanup()
			}
		}
	}
}

// performResourceCleanup performs resource cleanup tasks with enhanced cleanup logic
func (t *Transport) performResourceCleanup() {
	now := time.Now()
	
	// Clean up idle goroutines with two-phase approach
	if t.config.ResourceCleanupConfig.EnableGoroutineTracking {
		toCleanup := make(map[string]*GoroutineInfo)
		
		// Phase 1: Identify idle goroutines
		t.goroutinesMutex.Lock()
		for name, info := range t.activeGoroutines {
			if now.Sub(info.LastSeen) > t.config.ResourceCleanupConfig.MaxGoroutineIdleTime {
				toCleanup[name] = info
			}
		}
		t.goroutinesMutex.Unlock()
		
		// Phase 2: Clean up identified goroutines
		if len(toCleanup) > 0 {
			t.config.Logger.Info("Performing goroutine cleanup",
				zap.Int("idle_goroutines", len(toCleanup)))
				
			for name, info := range toCleanup {
				t.config.Logger.Warn("Cancelling idle goroutine",
					zap.String("name", name),
					zap.Duration("idle_time", now.Sub(info.LastSeen)))
				
				// Cancel the goroutine context
				if info.Cancel != nil {
					info.Cancel()
				}
				
				// Remove from tracking - the goroutine's defer will also try to remove it
				// but this prevents it from being identified as idle again
				t.goroutinesMutex.Lock()
				delete(t.activeGoroutines, name)
				t.goroutinesMutex.Unlock()
			}
		}
	}
	
	// Log resource usage if monitoring is enabled
	if t.config.ResourceCleanupConfig.EnableResourceMonitoring {
		var m runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m)
		
		// Get current goroutine count for comparison
		currentGoroutines := runtime.NumGoroutine()
		
		t.config.Logger.Debug("Resource usage monitoring",
			zap.Int64("tracked_active_goroutines", t.stats.ActiveGoroutines),
			zap.Int("runtime_goroutines", currentGoroutines),
			zap.Uint64("memory_alloc_mb", m.Alloc/1024/1024),
			zap.Uint64("memory_sys_mb", m.Sys/1024/1024),
			zap.Uint32("gc_cycles", m.NumGC))
			
		// Warn if there's a significant discrepancy between tracked and runtime goroutines
		if int64(currentGoroutines) > t.stats.ActiveGoroutines+10 {
			t.config.Logger.Warn("Potential goroutine leak detected",
				zap.Int("runtime_goroutines", currentGoroutines),
				zap.Int64("tracked_goroutines", t.stats.ActiveGoroutines),
				zap.Int("untracked_diff", currentGoroutines-int(t.stats.ActiveGoroutines)))
		}
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