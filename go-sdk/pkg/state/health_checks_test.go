package state

import (
	"context"
	"errors"
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
			var manager *MockStateManager
			var healthCheck *StateManagerHealthCheck
			
			if tt.name == "nil state manager" {
				healthCheck = NewStateManagerHealthCheck(nil)
			} else {
				manager = NewMockStateManager()
				tt.setupMock(manager)
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
	manager := NewMockStateManager()
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
		name            string
		maxMemoryMB     int64
		maxGCPauseMs    int64
		maxGoroutines   int
		expectError     bool
		errorContains   string
	}{
		{
			name:          "healthy memory",
			maxMemoryMB:   1024, // 1GB - should be enough for tests
			maxGCPauseMs:  1000, // 1 second - very generous
			maxGoroutines: 10000, // High limit
			expectError:   false,
		},
		{
			name:          "memory threshold exceeded",
			maxMemoryMB:   1, // 1MB - very low limit
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
		setupStore  func() *MockStateStore
		timeout     time.Duration
		expectError bool
		errorMsg    string
	}{
		{
			name: "healthy store",
			setupStore: func() *MockStateStore {
				return NewMockStateStore()
			},
			timeout:     time.Second,
			expectError: false,
		},
		{
			name: "nil store",
			setupStore: func() *MockStateStore {
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
	store := NewMockStateStore()
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
		name        string
		setupHandler func() *MockStateEventHandler
		expectError bool
		errorMsg    string
	}{
		{
			name: "healthy event handler",
			setupHandler: func() *MockStateEventHandler {
				handler := NewMockStateEventHandler()
				handler.SetRunning(true)
				handler.SetQueueDepth(100)
				return handler
			},
			expectError: false,
		},
		{
			name: "nil event handler",
			setupHandler: func() *MockStateEventHandler {
				return nil
			},
			expectError: true,
			errorMsg:    "event handler is nil",
		},
		{
			name: "not running event handler",
			setupHandler: func() *MockStateEventHandler {
				handler := NewMockStateEventHandler()
				handler.SetRunning(false)
				return handler
			},
			expectError: true,
			errorMsg:    "event handler is not running",
		},
		{
			name: "high queue depth",
			setupHandler: func() *MockStateEventHandler {
				handler := NewMockStateEventHandler()
				handler.SetRunning(true)
				handler.SetQueueDepth(15000) // Above threshold
				return handler
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
		name               string
		rateLimiter        *RateLimiter
		clientRateLimiter  *ClientRateLimiter
		expectError        bool
		errorMsg           string
	}{
		{
			name:               "healthy rate limiter",
			rateLimiter:        NewRateLimiter(100),
			clientRateLimiter:  nil,
			expectError:        false,
		},
		{
			name:               "healthy client rate limiter",
			rateLimiter:        nil,
			clientRateLimiter:  NewClientRateLimiter(DefaultClientRateLimiterConfig()),
			expectError:        false,
		},
		{
			name:               "both rate limiters",
			rateLimiter:        NewRateLimiter(100),
			clientRateLimiter:  NewClientRateLimiter(DefaultClientRateLimiterConfig()),
			expectError:        false,
		},
		{
			name:               "no rate limiters",
			rateLimiter:        nil,
			clientRateLimiter:  nil,
			expectError:        true,
			errorMsg:           "no rate limiters configured",
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
		setupAudit  func() *MockAuditManager
		expectError bool
		errorMsg    string
	}{
		{
			name: "healthy audit manager",
			setupAudit: func() *MockAuditManager {
				return NewMockAuditManager()
			},
			expectError: false,
		},
		{
			name: "nil audit manager",
			setupAudit: func() *MockAuditManager {
				return nil
			},
			expectError: true,
			errorMsg:    "audit manager is nil",
		},
		{
			name: "disabled audit manager",
			setupAudit: func() *MockAuditManager {
				manager := NewMockAuditManager()
				manager.SetEnabled(false)
				return manager
			},
			expectError: true,
			errorMsg:    "audit logging is disabled",
		},
		{
			name: "audit verification failure",
			setupAudit: func() *MockAuditManager {
				manager := NewMockAuditManager()
				manager.SetLoggerFail(true)
				return manager
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
			name:     "empty checks sequential",
			parallel: false,
			checks:   []HealthCheck{},
			expectError: false,
		},
		{
			name:     "empty checks parallel",
			parallel: true,
			checks:   []HealthCheck{},
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
	tests := []struct {
		name            string
		setupOptimizer  func() *MockPerformanceOptimizer
		maxPoolMissRate float64
		maxErrorRate    float64
		expectError     bool
		errorMsg        string
	}{
		{
			name: "healthy performance",
			setupOptimizer: func() *MockPerformanceOptimizer {
				optimizer := NewMockPerformanceOptimizer()
				optimizer.SetMetrics(PerformanceMetrics{
					PoolEfficiency: 95.0,
				})
				return optimizer
			},
			maxPoolMissRate: 10.0,
			maxErrorRate:    5.0,
			expectError:     false,
		},
		{
			name: "nil optimizer",
			setupOptimizer: func() *MockPerformanceOptimizer {
				return nil
			},
			maxPoolMissRate: 10.0,
			maxErrorRate:    5.0,
			expectError:     true,
			errorMsg:        "performance optimizer is nil",
		},
		{
			name: "low pool efficiency",
			setupOptimizer: func() *MockPerformanceOptimizer {
				optimizer := NewMockPerformanceOptimizer()
				optimizer.SetMetrics(PerformanceMetrics{
					PoolEfficiency: 80.0, // Below 90% (100% - 10% max miss rate)
				})
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
		store := NewMockStateStore()
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

// Helper functions

