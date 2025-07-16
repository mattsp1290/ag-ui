package testhelper

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// CleanupHelper provides utilities for cleaning up resources in tests
type CleanupHelper struct {
	t       *testing.T
	mu      sync.Mutex
	cleanup []func()
}

// NewCleanupHelper creates a new cleanup helper
func NewCleanupHelper(t *testing.T) *CleanupHelper {
	ch := &CleanupHelper{
		t: t,
	}
	
	t.Cleanup(func() {
		ch.RunCleanup()
	})
	
	return ch
}

// Add registers a cleanup function
func (ch *CleanupHelper) Add(cleanup func()) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.cleanup = append(ch.cleanup, cleanup)
}

// RunCleanup executes all cleanup functions in reverse order
func (ch *CleanupHelper) RunCleanup() {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	
	for i := len(ch.cleanup) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					ch.t.Logf("Cleanup panic: %v", r)
				}
			}()
			ch.cleanup[i]()
		}()
	}
	
	ch.cleanup = nil
}

// CloseChannel safely closes a channel with panic recovery
func CloseChannel[T any](t *testing.T, ch chan T, name string) {
	t.Helper()
	
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Panic closing channel %s: %v", name, r)
		}
	}()
	
	if ch != nil {
		close(ch)
		t.Logf("Closed channel: %s", name)
	}
}

// DrainChannel drains all values from a channel before closing
func DrainChannel[T any](t *testing.T, ch chan T, name string, timeout time.Duration) {
	t.Helper()
	
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	
	drained := 0
	for {
		select {
		case <-ch:
			drained++
		case <-deadline.C:
			t.Logf("Drained %d items from channel %s before timeout", drained, name)
			CloseChannel(t, ch, name)
			return
		default:
			t.Logf("Drained %d items from channel %s", drained, name)
			CloseChannel(t, ch, name)
			return
		}
	}
}

// CloseConnection safely closes any io.Closer (connections, files, etc.)
func CloseConnection(t *testing.T, conn io.Closer, name string) {
	t.Helper()
	
	if conn == nil {
		return
	}
	
	if err := conn.Close(); err != nil {
		t.Logf("Error closing %s: %v", name, err)
	} else {
		t.Logf("Closed connection: %s", name)
	}
}

// StopWorker stops a worker goroutine using a done channel
func StopWorker(t *testing.T, done chan<- struct{}, name string, timeout time.Duration) {
	t.Helper()
	
	select {
	case done <- struct{}{}:
		t.Logf("Sent stop signal to worker: %s", name)
	case <-time.After(timeout):
		t.Logf("Timeout sending stop signal to worker: %s", name)
	}
}

// WaitGroupTimeout waits for a WaitGroup with timeout
func WaitGroupTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) bool {
	t.Helper()
	
	done := make(chan struct{})
	
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		t.Logf("WaitGroup timeout after %v", timeout)
		return false
	}
}

// CleanupManager manages multiple cleanup operations
type CleanupManager struct {
	t         *testing.T
	mu        sync.Mutex
	resources map[string]func()
}

// NewCleanupManager creates a new cleanup manager
func NewCleanupManager(t *testing.T) *CleanupManager {
	cm := &CleanupManager{
		t:         t,
		resources: make(map[string]func()),
	}
	
	t.Cleanup(func() {
		cm.CleanupAll()
	})
	
	return cm
}

// Register adds a named resource for cleanup
func (cm *CleanupManager) Register(name string, cleanup func()) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if _, exists := cm.resources[name]; exists {
		cm.t.Logf("Warning: overwriting cleanup for resource: %s", name)
	}
	
	cm.resources[name] = cleanup
}

// Cleanup cleans up a specific resource
func (cm *CleanupManager) Cleanup(name string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if cleanup, exists := cm.resources[name]; exists {
		cm.runCleanup(name, cleanup)
		delete(cm.resources, name)
	}
}

// CleanupAll cleans up all resources
func (cm *CleanupManager) CleanupAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.t.Logf("Cleaning up %d resources", len(cm.resources))
	
	for name, cleanup := range cm.resources {
		cm.runCleanup(name, cleanup)
	}
	
	cm.resources = make(map[string]func())
}

// runCleanup executes a cleanup function with panic recovery
func (cm *CleanupManager) runCleanup(name string, cleanup func()) {
	defer func() {
		if r := recover(); r != nil {
			cm.t.Logf("Panic during cleanup of %s: %v", name, r)
		}
	}()
	
	start := time.Now()
	cleanup()
	cm.t.Logf("Cleaned up %s in %v", name, time.Since(start))
}

// NetworkCleanup helps clean up network resources
type NetworkCleanup struct {
	t          *testing.T
	listeners  []net.Listener
	conns      []net.Conn
	mu         sync.Mutex
}

// NewNetworkCleanup creates a network cleanup helper
func NewNetworkCleanup(t *testing.T) *NetworkCleanup {
	nc := &NetworkCleanup{t: t}
	
	t.Cleanup(func() {
		nc.CleanupAll()
	})
	
	return nc
}

// AddListener registers a listener for cleanup
func (nc *NetworkCleanup) AddListener(l net.Listener) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	nc.listeners = append(nc.listeners, l)
}

// AddConnection registers a connection for cleanup
func (nc *NetworkCleanup) AddConnection(conn net.Conn) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	nc.conns = append(nc.conns, conn)
}

// CleanupAll closes all network resources
func (nc *NetworkCleanup) CleanupAll() {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	
	// Close connections first
	for _, conn := range nc.conns {
		if err := conn.Close(); err != nil {
			nc.t.Logf("Error closing connection: %v", err)
		}
	}
	
	// Then close listeners
	for _, l := range nc.listeners {
		if err := l.Close(); err != nil {
			nc.t.Logf("Error closing listener: %v", err)
		}
	}
	
	nc.conns = nil
	nc.listeners = nil
}

// ChannelCleanup manages channel cleanup
type ChannelCleanup struct {
	t        *testing.T
	mu       sync.Mutex
	channels []channelInfo
}

type channelInfo struct {
	name    string
	cleanup func()
}

// NewChannelCleanup creates a channel cleanup helper
func NewChannelCleanup(t *testing.T) *ChannelCleanup {
	cc := &ChannelCleanup{t: t}
	
	t.Cleanup(func() {
		cc.CleanupAll()
	})
	
	return cc
}

// Add registers a channel for cleanup
func (cc *ChannelCleanup) Add(name string, cleanup func()) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	
	cc.channels = append(cc.channels, channelInfo{
		name:    name,
		cleanup: cleanup,
	})
}

// AddChan registers a typed channel for cleanup
func AddChan[T any](cc *ChannelCleanup, name string, ch chan T) {
	cc.Add(name, func() {
		CloseChannel(cc.t, ch, name)
	})
}

// CleanupAll closes all registered channels
func (cc *ChannelCleanup) CleanupAll() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	
	cc.t.Logf("Cleaning up %d channels", len(cc.channels))
	
	for _, ch := range cc.channels {
		func() {
			defer func() {
				if r := recover(); r != nil {
					cc.t.Logf("Panic cleaning up channel %s: %v", ch.name, r)
				}
			}()
			ch.cleanup()
		}()
	}
	
	cc.channels = nil
}

// ResourceTracker tracks resource allocation and cleanup
type ResourceTracker struct {
	t         *testing.T
	mu        sync.Mutex
	allocated map[string]time.Time
	cleaned   map[string]time.Time
}

// NewResourceTracker creates a resource tracker
func NewResourceTracker(t *testing.T) *ResourceTracker {
	rt := &ResourceTracker{
		t:         t,
		allocated: make(map[string]time.Time),
		cleaned:   make(map[string]time.Time),
	}
	
	t.Cleanup(func() {
		rt.Report()
	})
	
	return rt
}

// Allocated marks a resource as allocated
func (rt *ResourceTracker) Allocated(name string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.allocated[name] = time.Now()
}

// Cleaned marks a resource as cleaned
func (rt *ResourceTracker) Cleaned(name string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.cleaned[name] = time.Now()
}

// Report logs resource tracking information
func (rt *ResourceTracker) Report() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	
	var leaks []string
	for name, allocTime := range rt.allocated {
		if cleanTime, cleaned := rt.cleaned[name]; !cleaned {
			leaks = append(leaks, name)
		} else {
			duration := cleanTime.Sub(allocTime)
			rt.t.Logf("Resource %s lived for %v", name, duration)
		}
	}
	
	if len(leaks) > 0 {
		rt.t.Errorf("Resource leaks detected: %v", leaks)
	}
}

// EnsureCleanup wraps a function to ensure cleanup happens
func EnsureCleanup(t *testing.T, name string, fn func(), cleanup func()) {
	t.Helper()
	
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Panic in %s: %v", name, r)
			cleanup()
			panic(r)
		} else {
			cleanup()
		}
	}()
	
	fn()
}