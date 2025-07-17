package testhelper

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// AdvancedCleanupManager provides enhanced cleanup capabilities
type AdvancedCleanupManager struct {
	t                 *testing.T
	mu                sync.Mutex
	cleanupStack      []cleanupItem
	resourceTrackers  map[string]*ResourceCounter
	tempDirs          []string
	tempFiles         []string
	onPanic           func(interface{})
	timeouts          *TimeoutConfig
	parallelCleanup   bool
	maxCleanupWorkers int
	cleanupTimeout    time.Duration
	errorHandler      func(string, error)
}

type cleanupItem struct {
	name     string
	fn       func() error
	priority int // Higher priority runs first
	timeout  time.Duration
}

// ResourceCounter tracks resource usage
type ResourceCounter struct {
	allocated int64
	released  int64
	mu        sync.RWMutex
}

// NewAdvancedCleanupManager creates an enhanced cleanup manager
func NewAdvancedCleanupManager(t *testing.T) *AdvancedCleanupManager {
	acm := &AdvancedCleanupManager{
		t:                 t,
		cleanupStack:      make([]cleanupItem, 0),
		resourceTrackers:  make(map[string]*ResourceCounter),
		tempDirs:          make([]string, 0),
		tempFiles:         make([]string, 0),
		timeouts:          GlobalTimeouts,
		parallelCleanup:   false,
		maxCleanupWorkers: runtime.NumCPU(),
		cleanupTimeout:    GlobalTimeouts.Cleanup,
	}

	t.Cleanup(func() {
		acm.ExecuteAllCleanup()
	})

	return acm
}

// AddCleanup adds a cleanup function with priority and timeout
func (acm *AdvancedCleanupManager) AddCleanup(name string, fn func() error, priority int) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	acm.cleanupStack = append(acm.cleanupStack, cleanupItem{
		name:     name,
		fn:       fn,
		priority: priority,
		timeout:  acm.cleanupTimeout,
	})

	acm.t.Logf("Added cleanup: %s (priority: %d)", name, priority)
}

// AddCleanupWithTimeout adds a cleanup function with custom timeout
func (acm *AdvancedCleanupManager) AddCleanupWithTimeout(name string, fn func() error, priority int, timeout time.Duration) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	acm.cleanupStack = append(acm.cleanupStack, cleanupItem{
		name:     name,
		fn:       fn,
		priority: priority,
		timeout:  timeout,
	})
}

// AddSimpleCleanup adds a simple cleanup function without error handling
func (acm *AdvancedCleanupManager) AddSimpleCleanup(name string, fn func()) {
	acm.AddCleanup(name, func() error {
		fn()
		return nil
	}, 0)
}

// SetParallelCleanup enables/disables parallel cleanup execution
func (acm *AdvancedCleanupManager) SetParallelCleanup(enabled bool) {
	acm.mu.Lock()
	defer acm.mu.Unlock()
	acm.parallelCleanup = enabled
}

// SetErrorHandler sets a custom error handler for cleanup failures
func (acm *AdvancedCleanupManager) SetErrorHandler(handler func(string, error)) {
	acm.mu.Lock()
	defer acm.mu.Unlock()
	acm.errorHandler = handler
}

// TrackResource starts tracking a resource type
func (acm *AdvancedCleanupManager) TrackResource(resourceType string) *ResourceCounter {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	if counter, exists := acm.resourceTrackers[resourceType]; exists {
		return counter
	}

	counter := &ResourceCounter{}
	acm.resourceTrackers[resourceType] = counter
	return counter
}

// Allocate increments the allocation counter for a resource
func (rc *ResourceCounter) Allocate(count int64) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.allocated += count
}

// Release increments the release counter for a resource
func (rc *ResourceCounter) Release(count int64) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.released += count
}

// GetCounts returns the current allocation and release counts
func (rc *ResourceCounter) GetCounts() (allocated, released int64) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.allocated, rc.released
}

// IsBalanced returns true if allocations equal releases
func (rc *ResourceCounter) IsBalanced() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.allocated == rc.released
}

// CreateTempDir creates a temporary directory that will be automatically cleaned up
func (acm *AdvancedCleanupManager) CreateTempDir(prefix string) (string, error) {
	tempDir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", err
	}

	acm.mu.Lock()
	acm.tempDirs = append(acm.tempDirs, tempDir)
	acm.mu.Unlock()

	acm.AddCleanup(fmt.Sprintf("temp-dir-%s", filepath.Base(tempDir)), func() error {
		return os.RemoveAll(tempDir)
	}, 10) // Lower priority for file cleanup

	acm.t.Logf("Created temp directory: %s", tempDir)
	return tempDir, nil
}

// CreateTempFile creates a temporary file that will be automatically cleaned up
func (acm *AdvancedCleanupManager) CreateTempFile(pattern string) (*os.File, error) {
	tempFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return nil, err
	}

	acm.mu.Lock()
	acm.tempFiles = append(acm.tempFiles, tempFile.Name())
	acm.mu.Unlock()

	acm.AddCleanup(fmt.Sprintf("temp-file-%s", filepath.Base(tempFile.Name())), func() error {
		tempFile.Close()
		return os.Remove(tempFile.Name())
	}, 10)

	acm.t.Logf("Created temp file: %s", tempFile.Name())
	return tempFile, nil
}

// ExecuteAllCleanup executes all registered cleanup functions
func (acm *AdvancedCleanupManager) ExecuteAllCleanup() {
	acm.mu.Lock()
	items := make([]cleanupItem, len(acm.cleanupStack))
	copy(items, acm.cleanupStack)
	parallel := acm.parallelCleanup
	maxWorkers := acm.maxCleanupWorkers
	acm.mu.Unlock()

	if len(items) == 0 {
		return
	}

	acm.t.Logf("Executing %d cleanup items", len(items))

	// Sort by priority (higher first)
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].priority < items[j].priority {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	if parallel && len(items) > 1 {
		acm.executeParallelCleanup(items, maxWorkers)
	} else {
		acm.executeSequentialCleanup(items)
	}

	acm.reportResourceLeaks()
}

// executeSequentialCleanup executes cleanup items one by one
func (acm *AdvancedCleanupManager) executeSequentialCleanup(items []cleanupItem) {
	for _, item := range items {
		acm.executeCleanupItem(item)
	}
}

// executeParallelCleanup executes cleanup items in parallel with worker pool
func (acm *AdvancedCleanupManager) executeParallelCleanup(items []cleanupItem, maxWorkers int) {
	workers := maxWorkers
	if len(items) < workers {
		workers = len(items)
	}

	itemChan := make(chan cleanupItem, len(items))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range itemChan {
				acm.executeCleanupItem(item)
			}
		}()
	}

	// Send items to workers
	for _, item := range items {
		itemChan <- item
	}
	close(itemChan)

	// Wait for completion
	wg.Wait()
}

// executeCleanupItem executes a single cleanup item with timeout and error handling
func (acm *AdvancedCleanupManager) executeCleanupItem(item cleanupItem) {
	defer func() {
		if r := recover(); r != nil {
			acm.t.Logf("Panic during cleanup %s: %v", item.name, r)
			if acm.onPanic != nil {
				acm.onPanic(r)
			}
		}
	}()

	start := time.Now()

	done := make(chan error, 1)
	go func() {
		done <- item.fn()
	}()

	select {
	case err := <-done:
		duration := time.Since(start)
		if err != nil {
			acm.t.Logf("Cleanup %s failed after %v: %v", item.name, duration, err)
			if acm.errorHandler != nil {
				acm.errorHandler(item.name, err)
			}
		} else {
			acm.t.Logf("Cleanup %s completed in %v", item.name, duration)
		}
	case <-time.After(item.timeout):
		acm.t.Logf("Cleanup %s timed out after %v", item.name, item.timeout)
		if acm.errorHandler != nil {
			acm.errorHandler(item.name, fmt.Errorf("cleanup timeout"))
		}
	}
}

// reportResourceLeaks reports any detected resource leaks
func (acm *AdvancedCleanupManager) reportResourceLeaks() {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	hasLeaks := false
	for resourceType, counter := range acm.resourceTrackers {
		allocated, released := counter.GetCounts()
		if allocated != released {
			hasLeaks = true
			acm.t.Errorf("Resource leak detected for %s: allocated=%d, released=%d (leak=%d)",
				resourceType, allocated, released, allocated-released)
		}
	}

	if !hasLeaks {
		acm.t.Log("No resource leaks detected")
	}
}

// DatabaseCleanup provides database-specific cleanup utilities
type DatabaseCleanup struct {
	acm    *AdvancedCleanupManager
	db     *sql.DB
	tables []string
}

// NewDatabaseCleanup creates a database cleanup helper
func NewDatabaseCleanup(t *testing.T, db *sql.DB) *DatabaseCleanup {
	return &DatabaseCleanup{
		acm:    NewAdvancedCleanupManager(t),
		db:     db,
		tables: make([]string, 0),
	}
}

// AddTable adds a table to be truncated during cleanup
func (dc *DatabaseCleanup) AddTable(tableName string) {
	dc.tables = append(dc.tables, tableName)
	dc.acm.AddCleanup(fmt.Sprintf("truncate-table-%s", tableName), func() error {
		_, err := dc.db.Exec(fmt.Sprintf("TRUNCATE TABLE %s", tableName))
		return err
	}, 50) // Medium priority
}

// AddTransaction adds a transaction rollback to cleanup
func (dc *DatabaseCleanup) AddTransaction(tx *sql.Tx) {
	dc.acm.AddCleanup("rollback-transaction", func() error {
		return tx.Rollback()
	}, 100) // High priority
}

// ProcessCleanup manages cleanup for external processes
type ProcessCleanup struct {
	acm       *AdvancedCleanupManager
	processes []ProcessInfo
}

type ProcessInfo struct {
	Name string
	PID  int
	Cmd  string
}

// NewProcessCleanup creates a process cleanup helper
func NewProcessCleanup(t *testing.T) *ProcessCleanup {
	return &ProcessCleanup{
		acm:       NewAdvancedCleanupManager(t),
		processes: make([]ProcessInfo, 0),
	}
}

// AddProcess adds a process to be terminated during cleanup
func (pc *ProcessCleanup) AddProcess(name string, pid int, cmd string) {
	pc.processes = append(pc.processes, ProcessInfo{
		Name: name,
		PID:  pid,
		Cmd:  cmd,
	})

	pc.acm.AddCleanup(fmt.Sprintf("kill-process-%s-%d", name, pid), func() error {
		// Implementation would depend on the OS
		// This is a placeholder for process termination
		return fmt.Errorf("process termination not implemented")
	}, 90) // High priority
}

// CleanupGuard provides automatic cleanup with defer patterns
type CleanupGuard struct {
	acm     *AdvancedCleanupManager
	active  bool
	name    string
	cleanup func() error
}

// NewCleanupGuard creates a cleanup guard that can be deferred
func NewCleanupGuard(acm *AdvancedCleanupManager, name string, cleanup func() error) *CleanupGuard {
	return &CleanupGuard{
		acm:     acm,
		active:  true,
		name:    name,
		cleanup: cleanup,
	}
}

// Defer registers the cleanup to be executed later
func (cg *CleanupGuard) Defer() {
	if cg.active {
		cg.acm.AddCleanup(cg.name, cg.cleanup, 0)
	}
}

// Cancel cancels the cleanup (prevents execution)
func (cg *CleanupGuard) Cancel() {
	cg.active = false
}

// Execute immediately executes the cleanup
func (cg *CleanupGuard) Execute() error {
	if !cg.active {
		return nil
	}

	cg.active = false
	return cg.cleanup()
}

// WithCleanup executes a function with automatic cleanup
func WithCleanup[T any](acm *AdvancedCleanupManager, name string, fn func() (T, error), cleanup func() error) (T, error) {
	guard := NewCleanupGuard(acm, name, cleanup)
	defer guard.Defer()

	result, err := fn()
	if err != nil {
		// If the function failed, execute cleanup immediately
		guard.Execute()
	}

	return result, err
}

// CleanupAfterDelay schedules cleanup to run after a delay
func (acm *AdvancedCleanupManager) CleanupAfterDelay(name string, fn func() error, delay time.Duration) {
	acm.AddCleanup(name, func() error {
		time.Sleep(delay)
		return fn()
	}, 0)
}

// CleanupOnCondition adds conditional cleanup that only runs if condition is true
func (acm *AdvancedCleanupManager) CleanupOnCondition(name string, fn func() error, condition func() bool) {
	acm.AddCleanup(name, func() error {
		if condition() {
			return fn()
		}
		return nil
	}, 0)
}
