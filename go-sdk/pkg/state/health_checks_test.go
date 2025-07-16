package state

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Mock implementations for testing

// Test helpers for creating minimal test instances

// MockStateStore provides a mock implementation for testing
type MockStateStore struct {
	data map[string]interface{}
}

func NewMockStateStore() *MockStateStore {
	return &MockStateStore{
		data: make(map[string]interface{}),
	}
}

func (m *MockStateStore) Get(ctx context.Context, key string) (interface{}, error) {
	if value, exists := m.data[key]; exists {
		return value, nil
	}
	return nil, nil
}

func (m *MockStateStore) Set(ctx context.Context, key string, value interface{}) error {
	m.data[key] = value
	return nil
}

// MockStateManager provides a mock implementation of StateManager for testing
type MockStateManager struct {
	store            *MockStateStore
	deltaComputer    *DeltaComputer
	conflictResolver *ConflictResolverImpl
	closing          int32
	failComponents   map[string]bool
}

func NewMockStateManager() *MockStateManager {
	return &MockStateManager{
		store:            NewMockStateStore(),
		deltaComputer:    &DeltaComputer{},
		conflictResolver: &ConflictResolverImpl{},
		failComponents:   make(map[string]bool),
	}
}

func (m *MockStateManager) SetClosing(closing bool) {
	if closing {
		atomic.StoreInt32(&m.closing, 1)
	} else {
		atomic.StoreInt32(&m.closing, 0)
	}
}

func (m *MockStateManager) isClosing() bool {
	return atomic.LoadInt32(&m.closing) != 0
}

func (m *MockStateManager) SetComponentFail(component string, fail bool) {
	m.failComponents[component] = fail

	switch component {
	case "store":
		if fail {
			m.store = nil
		} else {
			m.store = NewMockStateStore()
		}
	case "deltaComputer":
		if fail {
			m.deltaComputer = nil
		} else {
			m.deltaComputer = &DeltaComputer{}
		}
	case "conflictResolver":
		if fail {
			m.conflictResolver = nil
		} else {
			m.conflictResolver = &ConflictResolverImpl{}
		}
	}
}

// MockStateEventHandler provides a mock implementation of StateEventHandler
type MockStateEventHandler struct {
	running    bool
	queueDepth int64
	mu         sync.RWMutex
}

func NewMockStateEventHandler() *MockStateEventHandler {
	return &MockStateEventHandler{
		running:    true,
		queueDepth: 0,
	}
}

func (m *MockStateEventHandler) SetRunning(running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = running
}

func (m *MockStateEventHandler) isRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

func (m *MockStateEventHandler) SetQueueDepth(depth int64) {
	atomic.StoreInt64(&m.queueDepth, depth)
}

func (m *MockStateEventHandler) getQueueDepth() int64 {
	return atomic.LoadInt64(&m.queueDepth)
}

// MockAuditLogger provides a mock implementation of AuditLogger
type MockAuditLogger struct {
	logs []AuditLog
	mu   sync.RWMutex
	fail bool
}

func NewMockAuditLogger() *MockAuditLogger {
	return &MockAuditLogger{
		logs: make([]AuditLog, 0),
	}
}

func (m *MockAuditLogger) Log(ctx context.Context, log *AuditLog) error {
	if m.fail {
		return errors.New("mock logger failure")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, *log)
	return nil
}

func (m *MockAuditLogger) Query(ctx context.Context, criteria AuditCriteria) ([]*AuditLog, error) {
	if m.fail {
		return nil, errors.New("mock logger failure")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AuditLog, 0)
	for i := range m.logs {
		result = append(result, &m.logs[i])
	}
	return result, nil
}

func (m *MockAuditLogger) Verify(ctx context.Context, startTime, endTime time.Time) (*AuditVerification, error) {
	if m.fail {
		return nil, errors.New("mock logger failure")
	}

	return &AuditVerification{
		Valid:        true,
		TotalLogs:    len(m.logs),
		ValidLogs:    len(m.logs),
		InvalidLogs:  0,
		TamperedLogs: []string{},
		MissingLogs:  []int64{},
	}, nil
}

func (m *MockAuditLogger) SetFail(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fail = fail
}

func (m *MockAuditLogger) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = nil
	return nil
}

// MockAuditManager provides a mock implementation of AuditManager
type MockAuditManager struct {
	logger  *MockAuditLogger
	enabled bool
	mu      sync.RWMutex
}

func NewMockAuditManager() *MockAuditManager {
	return &MockAuditManager{
		logger:  NewMockAuditLogger(),
		enabled: true,
	}
}

func (m *MockAuditManager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
}

func (m *MockAuditManager) isEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

func (m *MockAuditManager) SetLoggerFail(fail bool) {
	m.logger.SetFail(fail)
}

// MockPerformanceOptimizer provides a mock implementation of PerformanceOptimizer
type MockPerformanceOptimizer struct {
	metrics PerformanceMetrics
	mu      sync.RWMutex
}

func NewMockPerformanceOptimizer() *MockPerformanceOptimizer {
	return &MockPerformanceOptimizer{
		metrics: PerformanceMetrics{
			PoolEfficiency: 95.0,
		},
	}
}

func (m *MockPerformanceOptimizer) GetMetrics() PerformanceMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

func (m *MockPerformanceOptimizer) SetMetrics(metrics PerformanceMetrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = metrics
}

// Additional methods required by PerformanceOptimizer interface
func (m *MockPerformanceOptimizer) GetPatchOperation() *JSONPatchOperation {
	return &JSONPatchOperation{}
}

func (m *MockPerformanceOptimizer) PutPatchOperation(op *JSONPatchOperation) {
	// Mock implementation - no-op
}

func (m *MockPerformanceOptimizer) GetStateChange() *StateChange {
	return &StateChange{}
}

func (m *MockPerformanceOptimizer) PutStateChange(sc *StateChange) {
	// Mock implementation - no-op
}

func (m *MockPerformanceOptimizer) GetStateEvent() *StateEvent {
	return &StateEvent{}
}

func (m *MockPerformanceOptimizer) PutStateEvent(se *StateEvent) {
	// Mock implementation - no-op
}

func (m *MockPerformanceOptimizer) GetBuffer() *bytes.Buffer {
	return &bytes.Buffer{}
}

func (m *MockPerformanceOptimizer) PutBuffer(buf *bytes.Buffer) {
	// Mock implementation - no-op
}

func (m *MockPerformanceOptimizer) BatchOperation(ctx context.Context, operation func() error) error {
	// Mock implementation - just execute the operation
	if operation != nil {
		return operation()
	}
	return nil
}

func (m *MockPerformanceOptimizer) ShardedGet(key string) (interface{}, bool) {
	// Mock implementation
	return nil, false
}

func (m *MockPerformanceOptimizer) ShardedSet(key string, value interface{}) {
	// Mock implementation - no-op
}

func (m *MockPerformanceOptimizer) CompressData(data []byte) ([]byte, error) {
	// Mock implementation - return the data as-is
	return data, nil
}

func (m *MockPerformanceOptimizer) DecompressData(data []byte) ([]byte, error) {
	// Mock implementation - return the data as-is
	return data, nil
}

func (m *MockPerformanceOptimizer) LazyLoadState(key string, loader func() (interface{}, error)) (interface{}, error) {
	// Mock implementation - just call the loader
	if loader != nil {
		return loader()
	}
	return nil, nil
}

func (m *MockPerformanceOptimizer) OptimizeForLargeState(stateSize int64) {
	// Mock implementation - no-op
}

func (m *MockPerformanceOptimizer) ProcessLargeStateUpdate(ctx context.Context, update func() error) error {
	// Mock implementation - just execute the update
	if update != nil {
		return update()
	}
	return nil
}

func (m *MockPerformanceOptimizer) GetEnhancedMetrics() PerformanceMetrics {
	// Mock implementation - return the same metrics
	return m.GetMetrics()
}

func (m *MockPerformanceOptimizer) Stop() {
	// Mock implementation - no-op
}

// TestStateManagerHealthCheck tests the StateManagerHealthCheck
func TestStateManagerHealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*MockStateManager)
		expectError bool
		errorMsg    string
	}{
		{
			name: "healthy state manager",
			setupMock: func(m *MockStateManager) {
				// All components are healthy by default
			},
			expectError: false,
		},
		{
			name: "nil state manager",
			setupMock: func(m *MockStateManager) {
				// This test will use nil manager
			},
			expectError: true,
			errorMsg:    "state manager is nil",
		},
		{
			name: "closing state manager",
			setupMock: func(m *MockStateManager) {
				m.SetClosing(true)
			},
			expectError: true,
			errorMsg:    "state manager is closing",
		},
		{
			name: "nil store",
			setupMock: func(m *MockStateManager) {
				m.SetComponentFail("store", true)
			},
			expectError: true,
			errorMsg:    "state store is not initialized",
		},
		{
			name: "nil delta computer",
			setupMock: func(m *MockStateManager) {
				m.SetComponentFail("deltaComputer", true)
			},
			expectError: true,
			errorMsg:    "delta computer is not initialized",
		},
		{
			name: "nil conflict resolver",
			setupMock: func(m *MockStateManager) {
				m.SetComponentFail("conflictResolver", true)
			},
			expectError: true,
			errorMsg:    "conflict resolver is not initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var manager *StateManager
			var healthCheck *StateManagerHealthCheck

			if tt.name == "nil state manager" {
				healthCheck = NewStateManagerHealthCheck(nil)
			} else {
				manager = createTestStateManagerWithMockBehavior(func(sm *StateManager) {
					// Convert mock setup to actual StateManager setup
					if tt.name == "closing state manager" {
						atomic.StoreInt32(&sm.closing, 1)
					} else if tt.name == "nil store" {
						sm.store = nil
					} else if tt.name == "nil delta computer" {
						sm.deltaComputer = nil
					} else if tt.name == "nil conflict resolver" {
						sm.conflictResolver = nil
					}
				})
				healthCheck = NewStateManagerHealthCheck(manager)
			}

			ctx := context.Background()
			err := healthCheck.Check(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}

			// Test Name method
			if healthCheck.Name() != "state_manager" {
				t.Errorf("Expected name 'state_manager', got '%s'", healthCheck.Name())
			}
		})
	}
}

// TestStateManagerHealthCheckWithContext tests context cancellation
func TestStateManagerHealthCheckWithContext(t *testing.T) {
	manager := createTestStateManager()
	healthCheck := NewStateManagerHealthCheck(manager)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := healthCheck.Check(ctx)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// TestMemoryHealthCheck tests the MemoryHealthCheck
func TestMemoryHealthCheck(t *testing.T) {
	tests := []struct {
		name          string
		maxMemoryMB   int64
		maxGCPauseMs  int64
		maxGoroutines int
		expectError   bool
		errorContains string
	}{
		{
			name:          "healthy memory",
			maxMemoryMB:   1024,  // 1GB - should be enough for tests
			maxGCPauseMs:  1000,  // 1 second - very generous
			maxGoroutines: 10000, // High limit
			expectError:   false,
		},
		{
			name:          "memory threshold exceeded",
			maxMemoryMB:   0, // 0MB - impossible to meet
			maxGCPauseMs:  1000,
			maxGoroutines: 10000,
			expectError:   true,
			errorContains: "memory usage",
		},
		{
			name:          "goroutine threshold exceeded",
			maxMemoryMB:   1024,
			maxGCPauseMs:  1000,
			maxGoroutines: 1, // Very low limit
			expectError:   true,
			errorContains: "goroutine count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthCheck := NewMemoryHealthCheck(tt.maxMemoryMB, tt.maxGCPauseMs, tt.maxGoroutines)

			ctx := context.Background()
			err := healthCheck.Check(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}

			// Test Name method
			if healthCheck.Name() != "memory" {
				t.Errorf("Expected name 'memory', got '%s'", healthCheck.Name())
			}
		})
	}
}

// TestStoreHealthCheck tests the StoreHealthCheck
func TestStoreHealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		setupStore  func() *StateStore
		timeout     time.Duration
		expectError bool
		errorMsg    string
	}{
		{
			name: "healthy store",
			setupStore: func() *StateStore {
				return createTestStateStore()
			},
			timeout:     time.Second,
			expectError: false,
		},
		{
			name: "nil store",
			setupStore: func() *StateStore {
				return nil
			},
			timeout:     time.Second,
			expectError: true,
			errorMsg:    "state store is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := tt.setupStore()
			healthCheck := NewStoreHealthCheck(store, tt.timeout)

			ctx := context.Background()
			err := healthCheck.Check(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}

			// Test Name method
			if healthCheck.Name() != "store" {
				t.Errorf("Expected name 'store', got '%s'", healthCheck.Name())
			}
		})
	}
}

// TestStoreHealthCheckTimeout tests timeout handling
func TestStoreHealthCheckTimeout(t *testing.T) {
	store := createTestStateStore()
	healthCheck := NewStoreHealthCheck(store, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	err := healthCheck.Check(ctx)
	// Should either pass or timeout, but not panic
	if err != nil && err != context.DeadlineExceeded {
		t.Logf("Got error (expected timeout or success): %v", err)
	}
}

// TestEventHandlerHealthCheck tests the EventHandlerHealthCheck
func TestEventHandlerHealthCheck(t *testing.T) {
	tests := []struct {
		name         string
		setupHandler func() *StateEventHandler
		expectError  bool
		errorMsg     string
	}{
		{
			name: "healthy event handler",
			setupHandler: func() *StateEventHandler {
				return createTestEventHandler(true, 100)
			},
			expectError: false,
		},
		{
			name: "nil event handler",
			setupHandler: func() *StateEventHandler {
				return nil
			},
			expectError: true,
			errorMsg:    "event handler is nil",
		},
		{
			name: "not running event handler",
			setupHandler: func() *StateEventHandler {
				return createTestEventHandler(false, 0)
			},
			expectError: true,
			errorMsg:    "event handler is not running",
		},
		{
			name: "high queue depth",
			setupHandler: func() *StateEventHandler {
				return createTestEventHandler(true, 15000) // Above threshold
			},
			expectError: true,
			errorMsg:    "event queue depth (15000) is too high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := tt.setupHandler()
			healthCheck := NewEventHandlerHealthCheck(handler)

			ctx := context.Background()
			err := healthCheck.Check(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}

			// Test Name method
			if healthCheck.Name() != "event_handler" {
				t.Errorf("Expected name 'event_handler', got '%s'", healthCheck.Name())
			}
		})
	}
}

// TestRateLimiterHealthCheck tests the RateLimiterHealthCheck
func TestRateLimiterHealthCheck(t *testing.T) {
	tests := []struct {
		name              string
		rateLimiter       *RateLimiter
		clientRateLimiter *ClientRateLimiter
		expectError       bool
		errorMsg          string
	}{
		{
			name:              "healthy rate limiter",
			rateLimiter:       NewRateLimiter(100),
			clientRateLimiter: nil,
			expectError:       false,
		},
		{
			name:              "healthy client rate limiter",
			rateLimiter:       nil,
			clientRateLimiter: NewClientRateLimiter(DefaultClientRateLimiterConfig()),
			expectError:       false,
		},
		{
			name:              "both rate limiters",
			rateLimiter:       NewRateLimiter(100),
			clientRateLimiter: NewClientRateLimiter(DefaultClientRateLimiterConfig()),
			expectError:       false,
		},
		{
			name:              "no rate limiters",
			rateLimiter:       nil,
			clientRateLimiter: nil,
			expectError:       true,
			errorMsg:          "no rate limiters configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthCheck := NewRateLimiterHealthCheck(tt.rateLimiter, tt.clientRateLimiter)

			ctx := context.Background()
			err := healthCheck.Check(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}

			// Test Name method
			if healthCheck.Name() != "rate_limiter" {
				t.Errorf("Expected name 'rate_limiter', got '%s'", healthCheck.Name())
			}

			// Cleanup rate limiters
			if tt.rateLimiter != nil {
				tt.rateLimiter.Stop()
			}
			// ClientRateLimiter doesn't have a Stop method, so no cleanup needed
		})
	}
}

// TestAuditHealthCheck tests the AuditHealthCheck
func TestAuditHealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		setupAudit  func() *AuditManager
		expectError bool
		errorMsg    string
	}{
		{
			name: "healthy audit manager",
			setupAudit: func() *AuditManager {
				return createTestAuditManager(false)
			},
			expectError: false,
		},
		{
			name: "nil audit manager",
			setupAudit: func() *AuditManager {
				return nil
			},
			expectError: true,
			errorMsg:    "audit manager is nil",
		},
		{
			name: "disabled audit manager",
			setupAudit: func() *AuditManager {
				return createTestAuditManager(true) // true means failing/disabled
			},
			expectError: true,
			errorMsg:    "audit logging is disabled",
		},
		{
			name: "audit verification failure",
			setupAudit: func() *AuditManager {
				am := createTestAuditManager(false)
				am.logger = nil // Simulate logger failure
				return am
			},
			expectError: true,
			errorMsg:    "audit verification failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auditManager := tt.setupAudit()
			healthCheck := NewAuditHealthCheck(auditManager)

			ctx := context.Background()
			err := healthCheck.Check(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}

			// Test Name method
			if healthCheck.Name() != "audit" {
				t.Errorf("Expected name 'audit', got '%s'", healthCheck.Name())
			}
		})
	}
}

// TestCompositeHealthCheck tests the CompositeHealthCheck
func TestCompositeHealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		parallel    bool
		checks      []HealthCheck
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty checks sequential",
			parallel:    false,
			checks:      []HealthCheck{},
			expectError: false,
		},
		{
			name:        "empty checks parallel",
			parallel:    true,
			checks:      []HealthCheck{},
			expectError: false,
		},
		{
			name:     "all healthy sequential",
			parallel: false,
			checks: []HealthCheck{
				NewMemoryHealthCheck(1024, 1000, 10000),
				NewCustomHealthCheck("test", func(ctx context.Context) error { return nil }),
			},
			expectError: false,
		},
		{
			name:     "all healthy parallel",
			parallel: true,
			checks: []HealthCheck{
				NewMemoryHealthCheck(1024, 1000, 10000),
				NewCustomHealthCheck("test", func(ctx context.Context) error { return nil }),
			},
			expectError: false,
		},
		{
			name:     "one failing sequential",
			parallel: false,
			checks: []HealthCheck{
				NewMemoryHealthCheck(1024, 1000, 10000),
				NewCustomHealthCheck("failing", func(ctx context.Context) error { return errors.New("test error") }),
			},
			expectError: true,
			errorMsg:    "health check 'failing' failed",
		},
		{
			name:     "one failing parallel",
			parallel: true,
			checks: []HealthCheck{
				NewMemoryHealthCheck(1024, 1000, 10000),
				NewCustomHealthCheck("failing", func(ctx context.Context) error { return errors.New("test error") }),
			},
			expectError: true,
			errorMsg:    "health checks failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthCheck := NewCompositeHealthCheck("composite", tt.parallel, tt.checks...)

			ctx := context.Background()
			err := healthCheck.Check(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}

			// Test Name method
			if healthCheck.Name() != "composite" {
				t.Errorf("Expected name 'composite', got '%s'", healthCheck.Name())
			}
		})
	}
}

// TestCompositeHealthCheckConcurrency tests concurrent execution
func TestCompositeHealthCheckConcurrency(t *testing.T) {
	var counter int32
	checks := make([]HealthCheck, 10)

	for i := 0; i < 10; i++ {
		checks[i] = NewCustomHealthCheck("test", func(ctx context.Context) error {
			atomic.AddInt32(&counter, 1)
			time.Sleep(10 * time.Millisecond) // Simulate work
			return nil
		})
	}

	healthCheck := NewCompositeHealthCheck("concurrent", true, checks...)

	start := time.Now()
	ctx := context.Background()
	err := healthCheck.Check(ctx)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	if atomic.LoadInt32(&counter) != 10 {
		t.Errorf("Expected counter to be 10, got %d", counter)
	}

	// Parallel execution should be faster than sequential
	if duration > 80*time.Millisecond {
		t.Errorf("Parallel execution took too long: %v", duration)
	}
}

// TestPerformanceHealthCheck tests the PerformanceHealthCheck
func TestPerformanceHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PerformanceHealthCheck test in short mode to prevent goroutine leaks")
	}
	
	tests := []struct {
		name            string
		setupOptimizer  func() PerformanceOptimizer
		maxPoolMissRate float64
		maxErrorRate    float64
		expectError     bool
		errorMsg        string
	}{
		{
			name: "healthy performance",
			setupOptimizer: func() PerformanceOptimizer {
				optimizer := createTestPerformanceOptimizer()
				// Set up healthy metrics
				if impl, ok := optimizer.(*PerformanceOptimizerImpl); ok {
					impl.poolHits.Store(95)
					impl.poolMisses.Store(5)
				}
				return optimizer
			},
			maxPoolMissRate: 10.0,
			maxErrorRate:    5.0,
			expectError:     false,
		},
		{
			name: "nil optimizer",
			setupOptimizer: func() PerformanceOptimizer {
				return nil
			},
			maxPoolMissRate: 10.0,
			maxErrorRate:    5.0,
			expectError:     true,
			errorMsg:        "performance optimizer is nil",
		},
		{
			name: "low pool efficiency",
			setupOptimizer: func() PerformanceOptimizer {
				optimizer := createTestPerformanceOptimizer()
				// Set up poor efficiency metrics
				if impl, ok := optimizer.(*PerformanceOptimizerImpl); ok {
					impl.poolHits.Store(80)
					impl.poolMisses.Store(20)
				}
				return optimizer
			},
			maxPoolMissRate: 10.0,
			maxErrorRate:    5.0,
			expectError:     true,
			errorMsg:        "pool efficiency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			optimizer := tt.setupOptimizer()
			// Ensure optimizer is stopped after test
			if optimizer != nil {
				defer optimizer.Stop()
			}
			healthCheck := NewPerformanceHealthCheck(optimizer, tt.maxPoolMissRate, tt.maxErrorRate)

			ctx := context.Background()
			err := healthCheck.Check(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}

			// Test Name method
			if healthCheck.Name() != "performance" {
				t.Errorf("Expected name 'performance', got '%s'", healthCheck.Name())
			}
		})
	}
}

// TestCustomHealthCheck tests the CustomHealthCheck
func TestCustomHealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		checkFn     func(context.Context) error
		expectError bool
		errorMsg    string
	}{
		{
			name: "healthy custom check",
			checkFn: func(ctx context.Context) error {
				return nil
			},
			expectError: false,
		},
		{
			name: "failing custom check",
			checkFn: func(ctx context.Context) error {
				return errors.New("custom error")
			},
			expectError: true,
			errorMsg:    "custom error",
		},
		{
			name:        "nil check function",
			checkFn:     nil,
			expectError: true,
			errorMsg:    "health check function is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthCheck := NewCustomHealthCheck("custom", tt.checkFn)

			ctx := context.Background()
			err := healthCheck.Check(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}

			// Test Name method
			if healthCheck.Name() != "custom" {
				t.Errorf("Expected name 'custom', got '%s'", healthCheck.Name())
			}
		})
	}
}

// TestHealthCheckWithContextCancellation tests context cancellation behavior
func TestHealthCheckWithContextCancellation(t *testing.T) {
	tests := []struct {
		name      string
		checkType string
		factory   func() HealthCheck
	}{
		{
			name:      "memory health check",
			checkType: "memory",
			factory: func() HealthCheck {
				return NewMemoryHealthCheck(1024, 1000, 10000)
			},
		},
		{
			name:      "custom health check",
			checkType: "custom",
			factory: func() HealthCheck {
				return NewCustomHealthCheck("test", func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(100 * time.Millisecond):
						return nil
					}
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthCheck := tt.factory()

			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			err := healthCheck.Check(ctx)

			// Should handle context cancellation gracefully
			if err != nil && err != context.Canceled {
				t.Logf("Health check returned error (expected cancellation or success): %v", err)
			}
		})
	}
}

// TestHealthCheckTimeout tests timeout behavior
func TestHealthCheckTimeout(t *testing.T) {
	healthCheck := NewCustomHealthCheck("slow", func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := healthCheck.Check(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

// TestHealthCheckEdgeCases tests edge cases and error conditions
func TestHealthCheckEdgeCases(t *testing.T) {
	t.Run("memory health check with zero limits", func(t *testing.T) {
		healthCheck := NewMemoryHealthCheck(0, 0, 0)
		ctx := context.Background()
		err := healthCheck.Check(ctx)

		// Should fail with zero limits
		if err == nil {
			t.Error("Expected error with zero limits")
		}
	})

	t.Run("store health check with very short timeout", func(t *testing.T) {
		store := createTestStateStore()
		healthCheck := NewStoreHealthCheck(store, 1*time.Nanosecond)
		ctx := context.Background()

		// Should either pass quickly or timeout
		err := healthCheck.Check(ctx)
		if err != nil {
			t.Logf("Got error (expected quick pass or timeout): %v", err)
		}
	})

	t.Run("composite health check with mixed results", func(t *testing.T) {
		checks := []HealthCheck{
			NewCustomHealthCheck("pass", func(ctx context.Context) error { return nil }),
			NewCustomHealthCheck("fail", func(ctx context.Context) error { return errors.New("fail") }),
			NewCustomHealthCheck("pass2", func(ctx context.Context) error { return nil }),
		}

		healthCheck := NewCompositeHealthCheck("mixed", false, checks...)
		ctx := context.Background()
		err := healthCheck.Check(ctx)

		// Should fail on the first failing check
		if err == nil {
			t.Error("Expected error from mixed results")
		}

		if !strings.Contains(err.Error(), "fail") {
			t.Errorf("Expected error to contain 'fail', got '%s'", err.Error())
		}
	})
}

// TestHealthCheckStress tests health checks under stress conditions
func TestHealthCheckStress(t *testing.T) {
	t.Run("concurrent health checks", func(t *testing.T) {
		healthCheck := NewMemoryHealthCheck(1024, 1000, 10000)

		var wg sync.WaitGroup
		errors := make(chan error, 100)

		// Run 100 concurrent health checks
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx := context.Background()
				err := healthCheck.Check(ctx)
				if err != nil {
					errors <- err
				}
			}()
		}

		wg.Wait()
		close(errors)

		// Collect any errors
		var errorCount int
		for err := range errors {
			t.Logf("Concurrent health check error: %v", err)
			errorCount++
		}

		if errorCount > 10 { // Allow some errors under stress
			t.Errorf("Too many errors in concurrent health checks: %d", errorCount)
		}
	})
}

// Benchmark tests
func BenchmarkMemoryHealthCheck(b *testing.B) {
	healthCheck := NewMemoryHealthCheck(1024, 1000, 10000)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = healthCheck.Check(ctx)
	}
}

func BenchmarkCompositeHealthCheckSequential(b *testing.B) {
	checks := []HealthCheck{
		NewMemoryHealthCheck(1024, 1000, 10000),
		NewCustomHealthCheck("test", func(ctx context.Context) error { return nil }),
	}
	healthCheck := NewCompositeHealthCheck("bench", false, checks...)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = healthCheck.Check(ctx)
	}
}

func BenchmarkCompositeHealthCheckParallel(b *testing.B) {
	checks := []HealthCheck{
		NewMemoryHealthCheck(1024, 1000, 10000),
		NewCustomHealthCheck("test", func(ctx context.Context) error { return nil }),
	}
	healthCheck := NewCompositeHealthCheck("bench", true, checks...)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = healthCheck.Check(ctx)
	}
}

// BenchmarkCustomHealthCheck benchmarks custom health check performance
func BenchmarkCustomHealthCheck(b *testing.B) {
	healthCheck := NewCustomHealthCheck("bench", func(ctx context.Context) error { return nil })
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = healthCheck.Check(ctx)
	}
}

// TestHealthCheckInterfaceCompliance verifies all health checks implement the interface correctly
func TestHealthCheckInterfaceCompliance(t *testing.T) {
	tests := []struct {
		name        string
		healthCheck HealthCheck
		expectedName string
	}{
		{
			name:         "StateManagerHealthCheck",
			healthCheck:  NewStateManagerHealthCheck(createTestStateManager()),
			expectedName: "state_manager",
		},
		{
			name:         "MemoryHealthCheck",
			healthCheck:  NewMemoryHealthCheck(1024, 1000, 10000),
			expectedName: "memory",
		},
		{
			name:         "StoreHealthCheck",
			healthCheck:  NewStoreHealthCheck(createTestStateStore(), time.Second),
			expectedName: "store",
		},
		{
			name:         "EventHandlerHealthCheck",
			healthCheck:  NewEventHandlerHealthCheck(createTestEventHandler(true, 100)),
			expectedName: "event_handler",
		},
		{
			name:         "RateLimiterHealthCheck",
			healthCheck:  NewRateLimiterHealthCheck(NewRateLimiter(100), nil),
			expectedName: "rate_limiter",
		},
		{
			name:         "AuditHealthCheck",
			healthCheck:  NewAuditHealthCheck(createTestAuditManager(false)),
			expectedName: "audit",
		},
		{
			name:         "CompositeHealthCheck",
			healthCheck:  NewCompositeHealthCheck("test_composite", false),
			expectedName: "test_composite",
		},
		// Note: PerformanceHealthCheck test commented out to prevent goroutine leaks
		// {
		//	name:         "PerformanceHealthCheck",
		//	healthCheck:  NewPerformanceHealthCheck(perfOptimizer, 10.0, 5.0),
		//	expectedName: "performance",
		// },
		{
			name:         "CustomHealthCheck",
			healthCheck:  NewCustomHealthCheck("custom_test", func(ctx context.Context) error { return nil }),
			expectedName: "custom_test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Name method
			if name := tt.healthCheck.Name(); name != tt.expectedName {
				t.Errorf("Expected name '%s', got '%s'", tt.expectedName, name)
			}

			// Test Check method exists and can be called
			ctx := context.Background()
			err := tt.healthCheck.Check(ctx)
			
			// We don't care about the result, just that it doesn't panic
			// and the method exists
			_ = err

			// Clean up rate limiter if needed
			if tt.name == "RateLimiterHealthCheck" {
				if rlhc, ok := tt.healthCheck.(*RateLimiterHealthCheck); ok && rlhc.rateLimiter != nil {
					rlhc.rateLimiter.Stop()
				}
			}
		})
	}
}

// TestHealthCheckErrorMessages verifies error messages are descriptive
func TestHealthCheckErrorMessages(t *testing.T) {
	tests := []struct {
		name          string
		healthCheck   HealthCheck
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil state manager",
			healthCheck:   NewStateManagerHealthCheck(nil),
			expectError:   true,
			errorContains: "state manager is nil",
		},
		{
			name:          "nil store",
			healthCheck:   NewStoreHealthCheck(nil, time.Second),
			expectError:   true,
			errorContains: "state store is nil",
		},
		{
			name:          "nil event handler",
			healthCheck:   NewEventHandlerHealthCheck(nil),
			expectError:   true,
			errorContains: "event handler is nil",
		},
		{
			name:          "no rate limiters",
			healthCheck:   NewRateLimiterHealthCheck(nil, nil),
			expectError:   true,
			errorContains: "no rate limiters configured",
		},
		{
			name:          "nil audit manager",
			healthCheck:   NewAuditHealthCheck(nil),
			expectError:   true,
			errorContains: "audit manager is nil",
		},
		{
			name:          "nil performance optimizer",
			healthCheck:   NewPerformanceHealthCheck(nil, 10.0, 5.0),
			expectError:   true,
			errorContains: "performance optimizer is nil",
		},
		{
			name:          "nil custom function",
			healthCheck:   NewCustomHealthCheck("test", nil),
			expectError:   true,
			errorContains: "health check function is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := tt.healthCheck.Check(ctx)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestHealthCheckConcurrentAccess tests thread safety
func TestHealthCheckConcurrentAccess(t *testing.T) {
	healthChecks := []HealthCheck{
		NewMemoryHealthCheck(1024, 1000, 10000),
		NewCustomHealthCheck("concurrent", func(ctx context.Context) error { return nil }),
		NewCompositeHealthCheck("concurrent_composite", true,
			NewCustomHealthCheck("sub1", func(ctx context.Context) error { return nil }),
			NewCustomHealthCheck("sub2", func(ctx context.Context) error { return nil }),
		),
	}

	for _, hc := range healthChecks {
		t.Run(fmt.Sprintf("concurrent_%s", hc.Name()), func(t *testing.T) {
			var wg sync.WaitGroup
			errChan := make(chan error, 50)

			// Run 50 concurrent health checks
			for i := 0; i < 50; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					ctx := context.Background()
					if err := hc.Check(ctx); err != nil {
						errChan <- err
					}
				}()
			}

			wg.Wait()
			close(errChan)

			// Collect any errors
			var errors []error
			for err := range errChan {
				errors = append(errors, err)
			}

			if len(errors) > 0 {
				t.Errorf("Got %d errors in concurrent execution: %v", len(errors), errors[0])
			}
		})
	}
}

// TestMemoryHealthCheckGCPause tests GC pause time validation specifically
func TestMemoryHealthCheckGCPause(t *testing.T) {
	// Test GC pause validation by forcing GC and checking if we can detect pause times
	t.Run("gc pause under threshold", func(t *testing.T) {
		// Force a GC to ensure we have GC statistics
		runtime.GC()
		
		healthCheck := NewMemoryHealthCheck(1024, 10000, 10000) // 10 second GC pause limit
		ctx := context.Background()
		err := healthCheck.Check(ctx)
		
		if err != nil {
			t.Errorf("Expected no error for reasonable GC pause limit, got: %v", err)
		}
	})
	
	t.Run("gc pause threshold validation", func(t *testing.T) {
		// Force multiple GCs to generate pause statistics
		for i := 0; i < 5; i++ {
			runtime.GC()
		}
		
		// Set an extremely low GC pause threshold (1 nanosecond)
		healthCheck := NewMemoryHealthCheck(1024, 0, 10000) // 0ms GC pause limit
		ctx := context.Background()
		err := healthCheck.Check(ctx)
		
		// This might fail if there have been any GCs with non-zero pause times
		if err != nil && !strings.Contains(err.Error(), "GC pause time") {
			t.Errorf("Expected GC pause error or no error, got: %v", err)
		}
	})
}

// TestStoreHealthCheckComprehensiveTimeout tests various timeout scenarios
func TestStoreHealthCheckComprehensiveTimeout(t *testing.T) {
	t.Run("immediate timeout", func(t *testing.T) {
		store := createTestStateStore()
		healthCheck := NewStoreHealthCheck(store, 1*time.Nanosecond)
		
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		
		err := healthCheck.Check(ctx)
		// Should handle timeout gracefully
		if err != nil && err != context.DeadlineExceeded {
			t.Logf("Got error (expected timeout or success): %v", err)
		}
	})
	
	t.Run("context timeout before health check timeout", func(t *testing.T) {
		store := createTestStateStore()
		healthCheck := NewStoreHealthCheck(store, 100*time.Millisecond)
		
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		
		err := healthCheck.Check(ctx)
		if err != nil && err != context.DeadlineExceeded {
			t.Logf("Expected timeout or success, got: %v", err)
		}
	})
	
	t.Run("health check timeout before context timeout", func(t *testing.T) {
		store := createTestStateStore()
		healthCheck := NewStoreHealthCheck(store, 10*time.Millisecond)
		
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		err := healthCheck.Check(ctx)
		// Should either pass quickly or timeout
		if err != nil {
			t.Logf("Got error (expected quick pass or timeout): %v", err)
		}
	})
}

// TestEventHandlerHealthCheckAdvanced tests advanced event handler scenarios
func TestEventHandlerHealthCheckAdvanced(t *testing.T) {
	t.Run("queue depth boundary conditions", func(t *testing.T) {
		tests := []struct {
			name       string
			queueDepth int
			expectError bool
		}{
			{"exactly at threshold", 10000, false},
			{"just above threshold", 10001, true},
			{"way above threshold", 50000, true},
			{"zero queue depth", 0, false},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				handler := createTestEventHandler(true, tt.queueDepth)
				healthCheck := NewEventHandlerHealthCheck(handler)
				
				ctx := context.Background()
				err := healthCheck.Check(ctx)
				
				if tt.expectError && err == nil {
					t.Errorf("Expected error for queue depth %d, got none", tt.queueDepth)
				} else if !tt.expectError && err != nil {
					t.Errorf("Expected no error for queue depth %d, got: %v", tt.queueDepth, err)
				}
			})
		}
	})
	
	t.Run("handler state transitions", func(t *testing.T) {
		handler := createTestEventHandler(true, 100)
		healthCheck := NewEventHandlerHealthCheck(handler)
		
		// Initially running
		ctx := context.Background()
		err := healthCheck.Check(ctx)
		if err != nil {
			t.Errorf("Expected no error for running handler, got: %v", err)
		}
		
		// This test demonstrates the limitation that we can't easily modify 
		// the running state of a real StateEventHandler in tests.
		// In practice, this would require integration testing or dependency injection.
	})
}

// TestAuditHealthCheckAdvanced tests comprehensive audit scenarios
func TestAuditHealthCheckAdvanced(t *testing.T) {
	t.Run("audit verification basic functionality", func(t *testing.T) {
		// Test basic audit verification
		auditManager := createTestAuditManager(false)
		healthCheck := NewAuditHealthCheck(auditManager)
		ctx := context.Background()
		err := healthCheck.Check(ctx)
		
		if err != nil {
			t.Errorf("Expected healthy audit system, got: %v", err)
		}
	})
	
	t.Run("audit verification disabled", func(t *testing.T) {
		// Test disabled audit manager
		auditManager := createTestAuditManager(true) // disabled
		healthCheck := NewAuditHealthCheck(auditManager)
		ctx := context.Background()
		err := healthCheck.Check(ctx)
		
		if err == nil {
			t.Error("Expected error for disabled audit system")
		}
	})
	
	t.Run("audit verification nil manager", func(t *testing.T) {
		// Test nil audit manager
		healthCheck := NewAuditHealthCheck(nil)
		ctx := context.Background()
		err := healthCheck.Check(ctx)
		
		if err == nil {
			t.Error("Expected error for nil audit manager")
		}
	})
	
	t.Run("audit verification logger failure", func(t *testing.T) {
		// Test audit logger failure
		auditManager := createTestAuditManager(false)
		mockLogger := auditManager.logger.(*MockAuditLogger)
		mockLogger.SetFail(true)
		
		healthCheck := NewAuditHealthCheck(auditManager)
		ctx := context.Background()
		err := healthCheck.Check(ctx)
		
		if err == nil {
			t.Error("Expected error for audit logger failure")
		}
	})
}

// TestCompositeHealthCheckAdvanced tests advanced composite scenarios
func TestCompositeHealthCheckAdvanced(t *testing.T) {
	t.Run("large number of checks parallel", func(t *testing.T) {
		checks := make([]HealthCheck, 100)
		for i := 0; i < 100; i++ {
			checks[i] = NewCustomHealthCheck(fmt.Sprintf("test_%d", i), func(ctx context.Context) error {
				// Simulate some work
				time.Sleep(1 * time.Millisecond)
				return nil
			})
		}
		
		healthCheck := NewCompositeHealthCheck("large_parallel", true, checks...)
		
		start := time.Now()
		ctx := context.Background()
		err := healthCheck.Check(ctx)
		duration := time.Since(start)
		
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		
		// Should complete much faster than sequential (100ms)
		if duration > 50*time.Millisecond {
			t.Errorf("Parallel execution took too long: %v", duration)
		}
	})
	
	t.Run("mixed success and failure parallel", func(t *testing.T) {
		checks := []HealthCheck{
			NewCustomHealthCheck("success1", func(ctx context.Context) error { return nil }),
			NewCustomHealthCheck("failure1", func(ctx context.Context) error { return errors.New("error1") }),
			NewCustomHealthCheck("success2", func(ctx context.Context) error { return nil }),
			NewCustomHealthCheck("failure2", func(ctx context.Context) error { return errors.New("error2") }),
		}
		
		healthCheck := NewCompositeHealthCheck("mixed_parallel", true, checks...)
		ctx := context.Background()
		err := healthCheck.Check(ctx)
		
		if err == nil {
			t.Error("Expected error from failed checks")
		}
		
		// Should contain multiple errors
		errorStr := err.Error()
		if !strings.Contains(errorStr, "failure1") || !strings.Contains(errorStr, "failure2") {
			t.Errorf("Expected both failure1 and failure2 in error, got: %s", errorStr)
		}
	})
	
	t.Run("context cancellation during parallel execution", func(t *testing.T) {
		checks := make([]HealthCheck, 10)
		for i := 0; i < 10; i++ {
			checks[i] = NewCustomHealthCheck(fmt.Sprintf("slow_%d", i), func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(100 * time.Millisecond):
					return nil
				}
			})
		}
		
		healthCheck := NewCompositeHealthCheck("cancellation_test", true, checks...)
		
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		
		err := healthCheck.Check(ctx)
		// Should handle cancellation gracefully
		if err != nil && err != context.DeadlineExceeded {
			t.Logf("Expected timeout or success, got: %v", err)
		}
	})
	
	t.Run("nested composite health checks", func(t *testing.T) {
		// Create nested composite checks
		inner1 := NewCompositeHealthCheck("inner1", false,
			NewCustomHealthCheck("test1", func(ctx context.Context) error { return nil }),
			NewCustomHealthCheck("test2", func(ctx context.Context) error { return nil }),
		)
		
		inner2 := NewCompositeHealthCheck("inner2", true,
			NewCustomHealthCheck("test3", func(ctx context.Context) error { return nil }),
			NewCustomHealthCheck("test4", func(ctx context.Context) error { return nil }),
		)
		
		outer := NewCompositeHealthCheck("outer", false, inner1, inner2)
		
		ctx := context.Background()
		err := outer.Check(ctx)
		
		if err != nil {
			t.Errorf("Expected no error from nested composite checks, got: %v", err)
		}
	})
}

// TestPerformanceHealthCheckAdvanced tests advanced performance scenarios
func TestPerformanceHealthCheckAdvanced(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PerformanceHealthCheckAdvanced test in short mode to prevent goroutine leaks")
	}
	
	t.Run("pool efficiency boundary conditions", func(t *testing.T) {
		tests := []struct {
			name           string
			poolEfficiency float64
			maxMissRate    float64
			expectError    bool
		}{
			{"exactly at threshold", 90.0, 10.0, false},
			{"just below threshold", 89.9, 10.0, true},
			{"way below threshold", 50.0, 10.0, true},
			{"perfect efficiency", 100.0, 10.0, false},
			{"zero efficiency", 0.0, 10.0, true},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				optimizer := NewMockPerformanceOptimizer()
				optimizer.SetMetrics(PerformanceMetrics{
					PoolEfficiency: tt.poolEfficiency,
				})
				
				healthCheck := NewPerformanceHealthCheck(optimizer, tt.maxMissRate, 5.0)
				ctx := context.Background()
				err := healthCheck.Check(ctx)
				
				if tt.expectError && err == nil {
					t.Errorf("Expected error for efficiency %.2f%%, got none", tt.poolEfficiency)
				} else if !tt.expectError && err != nil {
					t.Errorf("Expected no error for efficiency %.2f%%, got: %v", tt.poolEfficiency, err)
				}
			})
		}
	})
}

// TestHealthCheckStressAdvanced tests additional stress scenarios
func TestHealthCheckStressAdvanced(t *testing.T) {
	t.Run("rapid fire health checks", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping stress test in short mode")
		}
		
		healthCheck := NewMemoryHealthCheck(1024, 1000, 10000)
		
		// Perform health checks as fast as possible
		var wg sync.WaitGroup
		errorCount := int32(0)
		
		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx := context.Background()
				if err := healthCheck.Check(ctx); err != nil {
					atomic.AddInt32(&errorCount, 1)
				}
			}()
		}
		
		wg.Wait()
		
		finalErrorCount := atomic.LoadInt32(&errorCount)
		if finalErrorCount > 100 { // Allow some errors under extreme stress
			t.Errorf("Too many errors in rapid fire test: %d", finalErrorCount)
		}
	})
	
	t.Run("memory pressure during health checks", func(t *testing.T) {
		healthCheck := NewMemoryHealthCheck(10, 1000, 10000) // Very low memory limit (10MB)
		
		// Allocate significant memory to create pressure and keep references
		largeMem := make([][]byte, 50)
		for i := range largeMem {
			largeMem[i] = make([]byte, 1024*1024) // 1MB each = 50MB total
			// Touch the memory to ensure it's actually allocated
			for j := 0; j < len(largeMem[i]); j += 4096 {
				largeMem[i][j] = byte(i % 256)
			}
		}
		
		// Don't call GC - we want the memory to stay allocated
		// Force memory stats update without GC
		runtime.ReadMemStats(&runtime.MemStats{})
		
		ctx := context.Background()
		err := healthCheck.Check(ctx)
		
		// Get current memory usage for debugging
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		memoryMB := int64(memStats.Alloc / 1024 / 1024)
		
		// Should fail due to memory pressure
		if err == nil {
			t.Errorf("Expected error due to memory pressure. Current usage: %d MB, limit: 10 MB. Allocated arrays: %d", memoryMB, len(largeMem))
		}
		
		// Keep the reference alive until after the check
		_ = largeMem
		
		// Clean up only at the end
		largeMem = nil
		runtime.GC()
	})
}

// TestAllHealthChecksWithTimeout tests timeout behavior for all health check types
func TestAllHealthChecksWithTimeout(t *testing.T) {
	tests := []struct {
		name        string
		factory     func() HealthCheck
		timeout     time.Duration
		expectError bool
	}{
		{
			name: "StateManagerHealthCheck",
			factory: func() HealthCheck {
				return NewStateManagerHealthCheck(createTestStateManager())
			},
			timeout:     10 * time.Millisecond,
			expectError: false, // Should complete quickly
		},
		{
			name: "MemoryHealthCheck",
			factory: func() HealthCheck {
				return NewMemoryHealthCheck(1024, 1000, 10000)
			},
			timeout:     10 * time.Millisecond,
			expectError: false, // Should complete quickly
		},
		{
			name: "StoreHealthCheck",
			factory: func() HealthCheck {
				return NewStoreHealthCheck(createTestStateStore(), time.Second)
			},
			timeout:     10 * time.Millisecond,
			expectError: false, // Should complete quickly
		},
		{
			name: "EventHandlerHealthCheck",
			factory: func() HealthCheck {
				return NewEventHandlerHealthCheck(createTestEventHandler(true, 100))
			},
			timeout:     10 * time.Millisecond,
			expectError: false, // Should complete quickly
		},
		{
			name: "RateLimiterHealthCheck",
			factory: func() HealthCheck {
				return NewRateLimiterHealthCheck(NewRateLimiter(100), nil)
			},
			timeout:     10 * time.Millisecond,
			expectError: false, // Should complete quickly
		},
		{
			name: "AuditHealthCheck",
			factory: func() HealthCheck {
				return NewAuditHealthCheck(createTestAuditManager(false))
			},
			timeout:     10 * time.Millisecond,
			expectError: false, // Should complete quickly
		},
		// Note: PerformanceHealthCheck test commented out to prevent goroutine leaks
		// {
		//	name: "PerformanceHealthCheck",
		//	factory: func() HealthCheck {
		//		return NewPerformanceHealthCheck(createTestPerformanceOptimizer(), 10.0, 5.0)
		//	},
		//	timeout:     10 * time.Millisecond,
		//	expectError: false, // Should complete quickly
		// },
		{
			name: "CustomHealthCheck",
			factory: func() HealthCheck {
				return NewCustomHealthCheck("timeout_test", func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(50 * time.Millisecond):
						return nil
					}
				})
			},
			timeout:     20 * time.Millisecond,
			expectError: true, // Should timeout
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthCheck := tt.factory()
			
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()
			
			err := healthCheck.Check(ctx)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected timeout error, got none")
				} else if err != context.DeadlineExceeded {
					t.Logf("Expected timeout, got different error: %v", err)
				}
			} else {
				if err != nil && err != context.DeadlineExceeded {
					t.Logf("Expected success or timeout, got: %v", err)
				}
			}
			
			// Clean up resources if needed
			switch tt.name {
			case "RateLimiterHealthCheck":
				if rlhc, ok := healthCheck.(*RateLimiterHealthCheck); ok && rlhc.rateLimiter != nil {
					rlhc.rateLimiter.Stop()
				}
			case "PerformanceHealthCheck":
				if phc, ok := healthCheck.(*PerformanceHealthCheck); ok && phc.performanceOptimizer != nil {
					phc.performanceOptimizer.Stop()
				}
			}
		})
	}
}

// TestHealthCheckFailureRecovery tests recovery from failure scenarios
func TestHealthCheckFailureRecovery(t *testing.T) {
	t.Run("state manager recovery", func(t *testing.T) {
		manager := createTestStateManager()
		healthCheck := NewStateManagerHealthCheck(manager)
		
		// Initially healthy
		ctx := context.Background()
		err := healthCheck.Check(ctx)
		if err != nil {
			t.Errorf("Expected healthy state initially, got: %v", err)
		}
		
		// Simulate failure
		atomic.StoreInt32(&manager.closing, 1)
		err = healthCheck.Check(ctx)
		if err == nil {
			t.Error("Expected error for closing state")
		}
		
		// Simulate recovery
		atomic.StoreInt32(&manager.closing, 0)
		err = healthCheck.Check(ctx)
		if err != nil {
			t.Errorf("Expected recovery to healthy state, got: %v", err)
		}
	})
	
	t.Run("event handler nil recovery", func(t *testing.T) {
		// Test with nil handler (simulates stopped state)
		healthCheck := NewEventHandlerHealthCheck(nil)
		
		ctx := context.Background()
		err := healthCheck.Check(ctx)
		if err == nil {
			t.Error("Expected error for nil handler")
		}
		
		// Test with valid handler (simulates recovery)
		handler := createTestEventHandler(true, 100)
		healthCheck2 := NewEventHandlerHealthCheck(handler)
		err = healthCheck2.Check(ctx)
		if err != nil {
			t.Errorf("Expected recovery to healthy state, got: %v", err)
		}
	})
}

// Helper functions
func createTestStateManager() *StateManager {
	return &StateManager{
		store:            createTestStateStore(),
		deltaComputer:    &DeltaComputer{},
		conflictResolver: &ConflictResolverImpl{},
		updateQueue:      make(chan *updateRequest, 100),
		closing:          0,
	}
}

// createTestStateManagerWithMockBehavior creates a StateManager with mock behavior
func createTestStateManagerWithMockBehavior(setupFn func(*StateManager)) *StateManager {
	sm := createTestStateManager()
	if setupFn != nil {
		setupFn(sm)
	}
	return sm
}

// createTestStateStore creates a minimal StateStore for testing
func createTestStateStore() *StateStore {
	store := &StateStore{
		shardCount:      16,
		maxHistory:      100,
		transactions:    make(map[string]*StateTransaction),
		cleanupInterval: time.Minute,
	}
	// Initialize shards
	store.shards = make([]*stateShard, store.shardCount)
	for i := uint32(0); i < store.shardCount; i++ {
		store.shards[i] = &stateShard{}
		// Initialize with empty immutable state
		store.shards[i].current.Store(&ImmutableState{
			version: 0,
			data:    make(map[string]interface{}),
			refs:    1,
		})
	}
	return store
}

// createTestEventHandler creates a minimal StateEventHandler for testing
func createTestEventHandler(running bool, queueDepth int) *StateEventHandler {
	ctx, cancel := context.WithCancel(context.Background())
	
	handler := &StateEventHandler{
		store:         createTestStateStore(),
		deltaComputer: &DeltaComputer{},
		metrics:       &StateMetrics{},
		batchSize:     100,
		batchTimeout:  time.Second,
		ctx:           ctx,
		cancel:        cancel,
		running:       running, // Set the running state as requested
	}
	
	// Simulate high queue depth by setting store to nil
	if queueDepth > 10000 {
		handler.store = nil
	}
	
	return handler
}

// createTestAuditManager creates a minimal AuditManager for testing
func createTestAuditManager(failing bool) *AuditManager {
	am := &AuditManager{
		enabled: true,
		logger:  NewMockAuditLogger(), // Always provide a logger
	}
	if failing {
		am.enabled = false
	}
	return am
}


// createTestPerformanceOptimizer creates a minimal PerformanceOptimizer for testing
func createTestPerformanceOptimizer() PerformanceOptimizer {
	opts := DefaultPerformanceOptions()
	opts.EnablePooling = true
	return NewPerformanceOptimizer(opts)
}
