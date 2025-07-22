package registry

import (
	"sync"
	"sync/atomic"
	"time"
)

// LifecycleManager handles cleanup and resource management
type LifecycleManager struct {
	config      *RegistryConfig
	cleanupStop chan struct{}
	cleanupOnce sync.Once
	closed      int32
}

// NewLifecycleManager creates a new lifecycle manager
func NewLifecycleManager(config *RegistryConfig) *LifecycleManager {
	return &LifecycleManager{
		config:      config,
		cleanupStop: make(chan struct{}),
	}
}

// StartBackgroundCleanup starts background cleanup process
func (lm *LifecycleManager) StartBackgroundCleanup(cleanupCallback func()) {
	go lm.backgroundCleanup(cleanupCallback)
}

// backgroundCleanup runs periodic cleanup
func (lm *LifecycleManager) backgroundCleanup(cleanupCallback func()) {
	ticker := time.NewTicker(lm.config.CleanupInterval)
	defer ticker.Stop()

	for {
		// Check if we're closed before entering blocking select
		if atomic.LoadInt32(&lm.closed) != 0 {
			return
		}
		
		select {
		case <-ticker.C:
			// Double-check if we're closed before running cleanup
			if atomic.LoadInt32(&lm.closed) != 0 {
				return
			}
			cleanupCallback()
		case <-lm.cleanupStop:
			return
		}
	}
}

// Close stops background cleanup
func (lm *LifecycleManager) Close() error {
	if !atomic.CompareAndSwapInt32(&lm.closed, 0, 1) {
		return nil // Already closed
	}

	// Close the stop channel to signal the goroutine to exit
	lm.cleanupOnce.Do(func() {
		close(lm.cleanupStop)
	})

	return nil
}

// IsClosed returns whether the lifecycle is closed
func (lm *LifecycleManager) IsClosed() bool {
	return atomic.LoadInt32(&lm.closed) != 0
}