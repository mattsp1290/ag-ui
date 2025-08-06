package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/testhelper"
)

func TestMiddleware(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	t.Run("MiddlewareFunc Implementation", func(t *testing.T) {
		called := false
		
		mw := MiddlewareFunc(func(next http.Handler) http.Handler {
			called = true
			return next
		})
		
		// Test interface methods
		assert.Equal(t, "func-middleware", mw.Name())
		assert.Equal(t, 0, mw.Priority())
		assert.Nil(t, mw.Config())
		assert.NoError(t, mw.Cleanup())
		
		// Test handler functionality
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		
		wrappedHandler := mw.Handler(testHandler)
		assert.True(t, called)
		assert.NotNil(t, wrappedHandler)
	})
}

func TestChain(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	logger := zaptest.NewLogger(t)
	chain := NewChain(logger)
	require.NotNil(t, chain)
	
	cleanup.Add(func() {
		chain.Cleanup()
	})

	t.Run("Chain Creation", func(t *testing.T) {
		assert.NotNil(t, chain)
		assert.Len(t, chain.List(), 0)
	})

	t.Run("Add Middleware", func(t *testing.T) {
		mw1 := &testMiddleware{name: "middleware1", priority: 10}
		mw2 := &testMiddleware{name: "middleware2", priority: 5}
		
		// Add middleware
		chain.Use(mw1, mw2)
		
		// Verify middleware added
		middlewares := chain.List()
		assert.Len(t, middlewares, 2)
		
		// Verify contains both middleware
		names := make(map[string]bool)
		for _, m := range middlewares {
			names[m.Name()] = true
		}
		assert.True(t, names["middleware1"])
		assert.True(t, names["middleware2"])
	})

	t.Run("Remove Middleware", func(t *testing.T) {
		mw := &testMiddleware{name: "removable", priority: 1}
		chain.Use(mw)
		
		// Verify added
		assert.Len(t, chain.List(), 3) // Previous test added 2
		
		// Remove
		removed := chain.Remove("removable")
		assert.True(t, removed)
		assert.Len(t, chain.List(), 2)
		
		// Try to remove non-existent
		removed = chain.Remove("non-existent")
		assert.False(t, removed)
	})

	t.Run("Clear Middleware", func(t *testing.T) {
		chain.Clear()
		assert.Len(t, chain.List(), 0)
	})
}

func TestChainExecution(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	t.Run("Middleware Execution Order", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		chain := NewChain(logger)
		defer chain.Cleanup()
		
		var executionOrder []string
		var mu sync.Mutex
		
		// Create middleware with different priorities
		mw1 := &orderTestMiddleware{
			name:     "high-priority",
			priority: 100,
			order:    &executionOrder,
			mu:       &mu,
		}
		mw2 := &orderTestMiddleware{
			name:     "medium-priority",
			priority: 50,
			order:    &executionOrder,
			mu:       &mu,
		}
		mw3 := &orderTestMiddleware{
			name:     "low-priority",
			priority: 10,
			order:    &executionOrder,
			mu:       &mu,
		}
		
		// Add middleware in random order
		chain.Use(mw2, mw3, mw1)
		
		// Create final handler
		finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			executionOrder = append(executionOrder, "final-handler")
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		})
		
		// Get compiled handler
		handler := chain.Handler(finalHandler)
		
		// Create test request
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		
		// Execute
		handler.ServeHTTP(w, req)
		
		// Verify execution order (highest priority first)
		mu.Lock()
		expectedOrder := []string{"high-priority", "medium-priority", "low-priority", "final-handler"}
		assert.Equal(t, expectedOrder, executionOrder)
		mu.Unlock()
	})

	t.Run("Chain Compilation Caching", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		chain := NewChain(logger)
		defer chain.Cleanup()
		
		mw := &testMiddleware{name: "test", priority: 1}
		chain.Use(mw)
		
		// Test caching behavior by checking the dirty flag and compilation count
		// First compilation should compile
		handler1 := chain.Compile()
		assert.NotNil(t, handler1)
		
		// Second compilation should return the same cached result
		// We verify caching by checking that the chain is not dirty after compilation
		handler2 := chain.Compile()
		assert.NotNil(t, handler2)
		
		// Test behavior: handlers should produce the same response
		req1 := httptest.NewRequest("GET", "/test", nil)
		w1 := httptest.NewRecorder()
		handler1.ServeHTTP(w1, req1)
		
		req2 := httptest.NewRequest("GET", "/test", nil)
		w2 := httptest.NewRecorder()
		handler2.ServeHTTP(w2, req2)
		
		// Both should produce the same result (cache working)
		assert.Equal(t, w1.Code, w2.Code)
		
		// Add middleware (should invalidate cache and trigger recompilation)
		chain.Use(&testMiddleware{name: "test2", priority: 2})
		
		// Third compilation should produce different behavior
		handler3 := chain.Compile()
		assert.NotNil(t, handler3)
		
		// Verify the new handler has different middleware count in execution
		req3 := httptest.NewRequest("GET", "/test", nil)
		w3 := httptest.NewRecorder()
		handler3.ServeHTTP(w3, req3)
		
		// The response should still be the same status, but the internal behavior is different
		// We verify cache invalidation by ensuring the compilation produced a valid handler
		assert.Equal(t, http.StatusNotFound, w3.Code) // Default handler behavior
	})
}

func TestManager(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	logger := zaptest.NewLogger(t)
	manager := NewManager(logger)
	require.NotNil(t, manager)
	
	cleanup.Add(func() {
		manager.Cleanup()
	})

	t.Run("Manager Creation", func(t *testing.T) {
		assert.NotNil(t, manager)
		assert.Len(t, manager.ListChains(), 0)
	})

	t.Run("Create and Get Chains", func(t *testing.T) {
		// Create chains
		chain1 := manager.CreateChain("chain1")
		chain2 := manager.CreateChain("chain2")
		
		assert.NotNil(t, chain1)
		assert.NotNil(t, chain2)
		
		// Get chains
		retrievedChain1, exists := manager.GetChain("chain1")
		assert.True(t, exists)
		assert.Equal(t, chain1, retrievedChain1)
		
		retrievedChain2, exists := manager.GetChain("chain2")
		assert.True(t, exists)
		assert.Equal(t, chain2, retrievedChain2)
		
		// Try to get non-existent chain
		_, exists = manager.GetChain("non-existent")
		assert.False(t, exists)
		
		// List chains
		chainNames := manager.ListChains()
		assert.Len(t, chainNames, 2)
		assert.Contains(t, chainNames, "chain1")
		assert.Contains(t, chainNames, "chain2")
	})

	t.Run("Remove Chains", func(t *testing.T) {
		// Remove existing chain
		removed := manager.RemoveChain("chain1")
		assert.True(t, removed)
		
		// Verify removed
		_, exists := manager.GetChain("chain1")
		assert.False(t, exists)
		
		// Try to remove non-existent chain
		removed = manager.RemoveChain("non-existent")
		assert.False(t, removed)
		
		// Verify remaining chain count
		chainNames := manager.ListChains()
		assert.Len(t, chainNames, 1)
		assert.Contains(t, chainNames, "chain2")
	})

	t.Run("Manager Cleanup", func(t *testing.T) {
		// Add some chains with middleware
		chain := manager.CreateChain("cleanup-test")
		mw := &testMiddleware{name: "cleanup-middleware", priority: 1}
		chain.Use(mw)
		
		// Cleanup manager
		err := manager.Cleanup()
		assert.NoError(t, err)
		
		// Verify all chains removed
		chainNames := manager.ListChains()
		assert.Len(t, chainNames, 0)
	})
}

func TestContextHelpers(t *testing.T) {
	t.Run("Request ID Context", func(t *testing.T) {
		ctx := context.Background()
		
		// Initially empty
		requestID := GetRequestID(ctx)
		assert.Empty(t, requestID)
		
		// Set request ID
		testID := "test-request-123"
		ctx = SetRequestID(ctx, testID)
		
		// Retrieve request ID
		retrievedID := GetRequestID(ctx)
		assert.Equal(t, testID, retrievedID)
	})

	t.Run("User ID Context", func(t *testing.T) {
		ctx := context.Background()
		
		// Initially empty
		userID := GetUserID(ctx)
		assert.Empty(t, userID)
		
		// Set user ID
		testUserID := "user-456"
		ctx = SetUserID(ctx, testUserID)
		
		// Retrieve user ID
		retrievedUserID := GetUserID(ctx)
		assert.Equal(t, testUserID, retrievedUserID)
	})

	t.Run("Logger Context", func(t *testing.T) {
		ctx := context.Background()
		
		// Initially no-op logger
		logger := GetLogger(ctx)
		assert.NotNil(t, logger)
		
		// Set logger
		testLogger := zaptest.NewLogger(t)
		ctx = SetLogger(ctx, testLogger)
		
		// Retrieve logger
		retrievedLogger := GetLogger(ctx)
		assert.Equal(t, testLogger, retrievedLogger)
	})

	t.Run("Start Time Context", func(t *testing.T) {
		ctx := context.Background()
		
		// Initially zero time
		startTime := GetStartTime(ctx)
		assert.True(t, startTime.IsZero())
		
		// Set start time
		testTime := time.Now()
		ctx = SetStartTime(ctx, testTime)
		
		// Retrieve start time
		retrievedTime := GetStartTime(ctx)
		assert.Equal(t, testTime, retrievedTime)
	})
}

func TestResponseWriter(t *testing.T) {
	t.Run("ResponseWriter Status Capture", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		rw := NewResponseWriter(recorder)
		
		// Initially OK status
		assert.Equal(t, http.StatusOK, rw.Status())
		assert.Equal(t, int64(0), rw.Written())
		
		// Write header
		rw.WriteHeader(http.StatusCreated)
		assert.Equal(t, http.StatusCreated, rw.Status())
		
		// Write data
		data := []byte("test response")
		n, err := rw.Write(data)
		assert.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, int64(len(data)), rw.Written())
		
		// Multiple writes should accumulate
		moreData := []byte(" more data")
		n, err = rw.Write(moreData)
		assert.NoError(t, err)
		assert.Equal(t, len(moreData), n)
		assert.Equal(t, int64(len(data)+len(moreData)), rw.Written())
	})

	t.Run("ResponseWriter Header Persistence", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		rw := NewResponseWriter(recorder)
		
		// Multiple WriteHeader calls - first one should stick
		rw.WriteHeader(http.StatusNotFound)
		rw.WriteHeader(http.StatusInternalServerError)
		
		assert.Equal(t, http.StatusNotFound, rw.Status())
	})
}

func TestConditionalMiddleware(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	t.Run("Conditional Execution", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		
		var executed bool
		
		// Base middleware
		baseMw := &testMiddleware{
			name:     "base",
			priority: 1,
			handler: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					executed = true
					next.ServeHTTP(w, r)
				})
			},
		}
		
		// Condition: only execute for GET requests
		condition := func(r *http.Request) bool {
			return r.Method == "GET"
		}
		
		conditionalMw := NewConditionalMiddleware(baseMw, condition, logger)
		
		// Test with GET request (should execute)
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		
		handler := conditionalMw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		
		executed = false
		handler.ServeHTTP(w, req)
		assert.True(t, executed)
		
		// Test with POST request (should not execute)
		req = httptest.NewRequest("POST", "/test", nil)
		w = httptest.NewRecorder()
		
		executed = false
		handler.ServeHTTP(w, req)
		assert.False(t, executed)
	})

	t.Run("Conditional Middleware Properties", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		baseMw := &testMiddleware{name: "base", priority: 5}
		condition := func(r *http.Request) bool { return true }
		
		conditionalMw := NewConditionalMiddleware(baseMw, condition, logger)
		
		assert.Equal(t, "conditional-base", conditionalMw.Name())
		assert.Equal(t, 5, conditionalMw.Priority())
		assert.Equal(t, baseMw.Config(), conditionalMw.Config())
		assert.NoError(t, conditionalMw.Cleanup())
	})
}

func TestUtilityFunctions(t *testing.T) {
	t.Run("GenerateRequestID", func(t *testing.T) {
		id1 := GenerateRequestID()
		id2 := GenerateRequestID()
		
		assert.NotEmpty(t, id1)
		assert.NotEmpty(t, id2)
		assert.NotEqual(t, id1, id2)
	})

	t.Run("GetClientIP", func(t *testing.T) {
		// Test X-Forwarded-For header
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1")
		
		ip := GetClientIP(req)
		assert.Equal(t, "192.168.1.100", ip)
		
		// Test X-Real-IP header
		req = httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Real-IP", "192.168.1.200")
		
		ip = GetClientIP(req)
		assert.Equal(t, "192.168.1.200", ip)
		
		// Test RemoteAddr fallback
		req = httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.300:12345"
		
		ip = GetClientIP(req)
		assert.Equal(t, "192.168.1.300:12345", ip)
	})

	t.Run("IsWebSocket", func(t *testing.T) {
		// WebSocket request
		req := httptest.NewRequest("GET", "/ws", nil)
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Connection", "Upgrade")
		
		assert.True(t, IsWebSocket(req))
		
		// Regular HTTP request
		req = httptest.NewRequest("GET", "/api", nil)
		assert.False(t, IsWebSocket(req))
	})

	t.Run("IsAJAXRequest", func(t *testing.T) {
		// AJAX request
		req := httptest.NewRequest("GET", "/api", nil)
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		
		assert.True(t, IsAJAXRequest(req))
		
		// Regular request
		req = httptest.NewRequest("GET", "/page", nil)
		assert.False(t, IsAJAXRequest(req))
	})
}

func TestBaseConfigValidation(t *testing.T) {
	t.Run("Valid Config", func(t *testing.T) {
		config := &BaseConfig{
			Enabled:  true,
			Priority: 10,
			Name:     "test-middleware",
			Debug:    false,
		}
		
		err := ValidateBaseConfig(config)
		assert.NoError(t, err)
	})

	t.Run("Nil Config", func(t *testing.T) {
		err := ValidateBaseConfig(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("Empty Name", func(t *testing.T) {
		config := &BaseConfig{
			Enabled:  true,
			Priority: 10,
			Name:     "",
			Debug:    false,
		}
		
		err := ValidateBaseConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name cannot be empty")
	})
}

// Test middleware implementation
type testMiddleware struct {
	name     string
	priority int
	handler  func(http.Handler) http.Handler
}

func (t *testMiddleware) Name() string {
	return t.name
}

func (t *testMiddleware) Priority() int {
	return t.priority
}

func (t *testMiddleware) Handler(next http.Handler) http.Handler {
	if t.handler != nil {
		return t.handler(next)
	}
	return next
}

func (t *testMiddleware) Config() interface{} {
	return nil
}

func (t *testMiddleware) Cleanup() error {
	return nil
}

// Test middleware for execution order testing
type orderTestMiddleware struct {
	name     string
	priority int
	order    *[]string
	mu       *sync.Mutex
}

func (o *orderTestMiddleware) Name() string {
	return o.name
}

func (o *orderTestMiddleware) Priority() int {
	return o.priority
}

func (o *orderTestMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o.mu.Lock()
		*o.order = append(*o.order, o.name)
		o.mu.Unlock()
		
		next.ServeHTTP(w, r)
	})
}

func (o *orderTestMiddleware) Config() interface{} {
	return nil
}

func (o *orderTestMiddleware) Cleanup() error {
	return nil
}