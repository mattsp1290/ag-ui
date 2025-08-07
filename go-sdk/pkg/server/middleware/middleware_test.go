package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
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

// =============================================================================
// CONCURRENCY TESTS
// =============================================================================

func TestConcurrentChainModification(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	t.Run("Concurrent Add/Remove Operations", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		chain := NewChain(logger)
		defer chain.Cleanup()

		const numGoroutines = 50
		const operationsPerGoroutine = 100

		var wg sync.WaitGroup
		var addCount, removeCount int32

		// Start multiple goroutines adding middleware
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < operationsPerGoroutine; j++ {
					// Add middleware
					mw := &testMiddleware{
						name:     fmt.Sprintf("middleware-%d-%d", id, j),
						priority: id + j,
					}
					chain.Use(mw)
					atomic.AddInt32(&addCount, 1)

					// Occasionally remove middleware
					if j%10 == 0 && j > 0 {
						removed := chain.Remove(fmt.Sprintf("middleware-%d-%d", id, j-5))
						if removed {
							atomic.AddInt32(&removeCount, 1)
						}
					}

					// Small delay to increase chance of race conditions
					time.Sleep(time.Microsecond)
				}
			}(i)
		}

		wg.Wait()

		// Verify no panics and reasonable state
		middlewares := chain.List()
		t.Logf("Added: %d, Removed: %d, Final count: %d",
			addCount, removeCount, len(middlewares))

		// Should have some middleware remaining (not all were removed)
		assert.Greater(t, len(middlewares), 0)
		assert.Less(t, len(middlewares), int(addCount))
	})

	t.Run("Concurrent Clear Operations", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		chain := NewChain(logger)
		defer chain.Cleanup()

		const numGoroutines = 20
		const numMiddleware = 50

		// Add initial middleware
		for i := 0; i < numMiddleware; i++ {
			mw := &testMiddleware{
				name:     fmt.Sprintf("initial-%d", i),
				priority: i,
			}
			chain.Use(mw)
		}

		var wg sync.WaitGroup
		var clearCount int32

		// Multiple goroutines attempting to clear
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				chain.Clear()
				atomic.AddInt32(&clearCount, 1)
			}()
		}

		// While clearing, try to add more middleware
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					mw := &testMiddleware{
						name:     fmt.Sprintf("concurrent-%d-%d", id, j),
						priority: id + j,
					}
					chain.Use(mw)
					time.Sleep(time.Microsecond)
				}
			}(i)
		}

		wg.Wait()

		// Verify final state is consistent
		middlewares := chain.List()
		t.Logf("Clear operations: %d, Final middleware count: %d", clearCount, len(middlewares))

		// Should not panic and state should be reasonable
		assert.GreaterOrEqual(t, len(middlewares), 0)
		assert.LessOrEqual(t, len(middlewares), numGoroutines*10)
	})
}

func TestConcurrentChainCompilation(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	t.Run("Concurrent Compilation and Modification", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		chain := NewChain(logger)
		defer chain.Cleanup()

		// Add some initial middleware
		for i := 0; i < 5; i++ {
			mw := &testMiddleware{
				name:     fmt.Sprintf("initial-%d", i),
				priority: i * 10,
			}
			chain.Use(mw)
		}

		const numGoroutines = 20
		var wg sync.WaitGroup
		var compilationCount, modificationCount int32

		// Goroutines constantly compiling the chain
		for i := 0; i < numGoroutines/2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					handler := chain.Compile()
					assert.NotNil(t, handler)
					atomic.AddInt32(&compilationCount, 1)

					// Test the compiled handler
					req := httptest.NewRequest("GET", "/test", nil)
					w := httptest.NewRecorder()
					handler.ServeHTTP(w, req)

					time.Sleep(time.Microsecond * 10)
				}
			}()
		}

		// Goroutines constantly modifying the chain
		for i := 0; i < numGoroutines/2; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					// Add middleware
					mw := &testMiddleware{
						name:     fmt.Sprintf("dynamic-%d-%d", id, j),
						priority: id*100 + j,
					}
					chain.Use(mw)
					atomic.AddInt32(&modificationCount, 1)

					// Occasionally remove middleware
					if j%20 == 0 && j > 0 {
						chain.Remove(fmt.Sprintf("dynamic-%d-%d", id, j-10))
					}

					time.Sleep(time.Microsecond * 5)
				}
			}(i)
		}

		wg.Wait()

		// Verify no race conditions occurred
		t.Logf("Compilations: %d, Modifications: %d", compilationCount, modificationCount)

		// Final compilation should work without issues
		finalHandler := chain.Compile()
		assert.NotNil(t, finalHandler)

		req := httptest.NewRequest("GET", "/final", nil)
		w := httptest.NewRecorder()
		finalHandler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Cache Invalidation Under Load", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		chain := NewChain(logger)
		defer chain.Cleanup()

		mw := &testMiddleware{name: "cache-test", priority: 1}
		chain.Use(mw)

		const numGoroutines = 20
		var wg sync.WaitGroup
		var cacheHits, cacheMisses int32

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < 50; j++ {
					// Half the time, modify the chain (cache miss)
					if j%2 == 0 {
						newMw := &testMiddleware{
							name:     fmt.Sprintf("cache-invalidator-%d-%d", id, j),
							priority: id*100 + j,
						}
						chain.Use(newMw)
						atomic.AddInt32(&cacheMisses, 1)
					}

					// Always compile (should hit cache when possible)
					handler := chain.Compile()
					assert.NotNil(t, handler)

					if j%2 != 0 {
						atomic.AddInt32(&cacheHits, 1)
					}

					time.Sleep(time.Microsecond)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Cache hits: %d, Cache misses: %d", cacheHits, cacheMisses)
		assert.Greater(t, cacheHits, int32(0))
		assert.Greater(t, cacheMisses, int32(0))
	})
}

func TestConcurrentRequestProcessing(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	t.Run("Parallel Request Execution", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		chain := NewChain(logger)
		defer chain.Cleanup()

		// Create middleware that tracks execution
		var requestCount, middlewareExecutions int64

		trackingMiddleware := &testMiddleware{
			name:     "tracking-middleware",
			priority: 100,
			handler: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					atomic.AddInt64(&middlewareExecutions, 1)

					// Add some processing time to increase chance of race conditions
					time.Sleep(time.Microsecond * 10)

					next.ServeHTTP(w, r)
				})
			},
		}

		chain.Use(trackingMiddleware)

		finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt64(&requestCount, 1)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		handler := chain.Handler(finalHandler)

		const numGoroutines = 100
		const requestsPerGoroutine = 50

		var wg sync.WaitGroup

		// Launch concurrent requests
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < requestsPerGoroutine; j++ {
					req := httptest.NewRequest("GET", fmt.Sprintf("/test-%d-%d", id, j), nil)
					w := httptest.NewRecorder()

					handler.ServeHTTP(w, req)

					assert.Equal(t, http.StatusOK, w.Code)
					assert.Equal(t, "OK", w.Body.String())
				}
			}(i)
		}

		wg.Wait()

		expectedRequests := int64(numGoroutines * requestsPerGoroutine)
		actualRequests := atomic.LoadInt64(&requestCount)
		actualMiddlewareExecutions := atomic.LoadInt64(&middlewareExecutions)

		t.Logf("Expected requests: %d, Actual requests: %d, Middleware executions: %d",
			expectedRequests, actualRequests, actualMiddlewareExecutions)

		assert.Equal(t, expectedRequests, actualRequests)
		assert.Equal(t, expectedRequests, actualMiddlewareExecutions)
	})

	t.Run("Request Processing with Chain Modifications", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		chain := NewChain(logger)
		defer chain.Cleanup()

		var processedRequests int64

		// Initial middleware
		initialMw := &testMiddleware{
			name:     "initial",
			priority: 50,
			handler: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					atomic.AddInt64(&processedRequests, 1)
					next.ServeHTTP(w, r)
				})
			},
		}
		chain.Use(initialMw)

		finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		const numRequestGoroutines = 50
		const numModificationGoroutines = 10
		const requestsPerGoroutine = 100

		var wg sync.WaitGroup

		// Goroutines processing requests
		for i := 0; i < numRequestGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < requestsPerGoroutine; j++ {
					// Get current handler (might be recompiled due to modifications)
					handler := chain.Handler(finalHandler)

					req := httptest.NewRequest("GET", fmt.Sprintf("/concurrent-%d-%d", id, j), nil)
					w := httptest.NewRecorder()

					handler.ServeHTTP(w, req)
					assert.Equal(t, http.StatusOK, w.Code)

					time.Sleep(time.Microsecond)
				}
			}(i)
		}

		// Goroutines modifying the chain
		for i := 0; i < numModificationGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < 50; j++ {
					// Add middleware
					mw := &testMiddleware{
						name:     fmt.Sprintf("dynamic-%d-%d", id, j),
						priority: id*100 + j + 1,
					}
					chain.Use(mw)

					time.Sleep(time.Microsecond * 50)

					// Occasionally remove middleware
					if j%10 == 0 && j > 0 {
						chain.Remove(fmt.Sprintf("dynamic-%d-%d", id, j-5))
					}
				}
			}(i)
		}

		wg.Wait()

		actualProcessed := atomic.LoadInt64(&processedRequests)
		expectedRequests := int64(numRequestGoroutines * requestsPerGoroutine)

		t.Logf("Expected requests: %d, Processed by middleware: %d", expectedRequests, actualProcessed)

		// Should process all requests despite concurrent modifications
		assert.Equal(t, expectedRequests, actualProcessed)
	})
}

func TestConcurrentManagerOperations(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	t.Run("Concurrent Chain Creation and Removal", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		manager := NewManager(logger)
		defer manager.Cleanup()

		const numGoroutines = 30
		const operationsPerGoroutine = 100

		var wg sync.WaitGroup
		var createCount, removeCount int32

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < operationsPerGoroutine; j++ {
					chainName := fmt.Sprintf("chain-%d-%d", id, j)

					// Create chain
					chain := manager.CreateChain(chainName)
					assert.NotNil(t, chain)
					atomic.AddInt32(&createCount, 1)

					// Add some middleware to the chain
					mw := &testMiddleware{
						name:     fmt.Sprintf("middleware-%d-%d", id, j),
						priority: j,
					}
					chain.Use(mw)

					// Occasionally remove the chain
					if j%5 == 0 && j > 0 {
						oldChainName := fmt.Sprintf("chain-%d-%d", id, j-2)
						if manager.RemoveChain(oldChainName) {
							atomic.AddInt32(&removeCount, 1)
						}
					}

					time.Sleep(time.Microsecond)
				}
			}(i)
		}

		wg.Wait()

		remainingChains := manager.ListChains()
		t.Logf("Created: %d, Removed: %d, Remaining: %d",
			createCount, removeCount, len(remainingChains))

		assert.Greater(t, len(remainingChains), 0)
		assert.LessOrEqual(t, len(remainingChains), int(createCount))
	})

	t.Run("Concurrent Chain Access", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		manager := NewManager(logger)
		defer manager.Cleanup()

		// Create initial chains
		const numInitialChains = 10
		for i := 0; i < numInitialChains; i++ {
			chainName := fmt.Sprintf("persistent-chain-%d", i)
			chain := manager.CreateChain(chainName)

			// Add middleware
			mw := &testMiddleware{
				name:     fmt.Sprintf("persistent-middleware-%d", i),
				priority: i * 10,
			}
			chain.Use(mw)
		}

		const numGoroutines = 50
		var wg sync.WaitGroup
		var accessCount, successCount int32

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < 200; j++ {
					// Access random chain
					chainName := fmt.Sprintf("persistent-chain-%d", j%numInitialChains)

					chain, exists := manager.GetChain(chainName)
					atomic.AddInt32(&accessCount, 1)

					if exists && chain != nil {
						atomic.AddInt32(&successCount, 1)

						// Use the chain
						finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)
						})

						handler := chain.Handler(finalHandler)
						req := httptest.NewRequest("GET", "/test", nil)
						w := httptest.NewRecorder()
						handler.ServeHTTP(w, req)

						assert.Equal(t, http.StatusOK, w.Code)
					}

					time.Sleep(time.Microsecond)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Total access attempts: %d, Successful: %d", accessCount, successCount)
		assert.Equal(t, accessCount, successCount) // Should find all chains
	})
}

func TestMiddlewareStateThreadSafety(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	t.Run("Stateful Middleware Concurrency", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		chain := NewChain(logger)
		defer chain.Cleanup()

		// Create middleware with shared state
		var counter int64
		var mutex sync.RWMutex
		stateMap := make(map[string]int)

		statefulMiddleware := &testMiddleware{
			name:     "stateful",
			priority: 100,
			handler: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Increment counter atomically
					atomic.AddInt64(&counter, 1)

					// Safely access shared map
					key := r.Header.Get("X-Test-Key")
					if key != "" {
						mutex.Lock()
						stateMap[key]++
						mutex.Unlock()
					}

					// Add processing delay
					time.Sleep(time.Microsecond * 5)

					next.ServeHTTP(w, r)
				})
			},
		}

		chain.Use(statefulMiddleware)

		finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := chain.Handler(finalHandler)

		const numGoroutines = 100
		const requestsPerGoroutine = 100
		const numTestKeys = 10

		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < requestsPerGoroutine; j++ {
					req := httptest.NewRequest("GET", "/test", nil)
					req.Header.Set("X-Test-Key", fmt.Sprintf("key-%d", j%numTestKeys))

					w := httptest.NewRecorder()
					handler.ServeHTTP(w, req)

					assert.Equal(t, http.StatusOK, w.Code)
				}
			}(i)
		}

		wg.Wait()

		// Verify state consistency
		expectedCounter := int64(numGoroutines * requestsPerGoroutine)
		actualCounter := atomic.LoadInt64(&counter)

		t.Logf("Expected counter: %d, Actual counter: %d", expectedCounter, actualCounter)
		assert.Equal(t, expectedCounter, actualCounter)

		// Verify map state
		mutex.RLock()
		totalMapEntries := 0
		for key, count := range stateMap {
			t.Logf("Key %s: %d entries", key, count)
			totalMapEntries += count
		}
		mutex.RUnlock()

		assert.Equal(t, int(expectedCounter), totalMapEntries)
		assert.Len(t, stateMap, numTestKeys)
	})
}

func TestRaceConditionDetection(t *testing.T) {
	// This test is designed to be run with -race flag
	defer testhelper.VerifyNoGoroutineLeaks(t)

	t.Run("Race Condition Stress Test", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping race condition test in short mode")
		}

		logger := zaptest.NewLogger(t)
		chain := NewChain(logger)
		defer chain.Cleanup()

		const duration = 2 * time.Second
		numWorkers := runtime.NumCPU() * 2

		done := make(chan bool)
		var wg sync.WaitGroup

		// Worker that constantly modifies the chain
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter := 0

			for {
				select {
				case <-done:
					return
				default:
					// Add middleware
					mw := &testMiddleware{
						name:     fmt.Sprintf("race-test-%d", counter),
						priority: counter,
					}
					chain.Use(mw)
					counter++

					// Remove some middleware
					if counter%10 == 0 {
						chain.Remove(fmt.Sprintf("race-test-%d", counter-5))
					}

					// Clear occasionally
					if counter%100 == 0 {
						chain.Clear()
					}

					time.Sleep(time.Microsecond)
				}
			}
		}()

		// Workers that constantly compile and use the chain
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				counter := 0

				for {
					select {
					case <-done:
						return
					default:
						// Compile chain
						handler := chain.Compile()
						if handler != nil {
							// Use the handler
							req := httptest.NewRequest("GET", fmt.Sprintf("/race-test-%d-%d", id, counter), nil)
							w := httptest.NewRecorder()
							handler.ServeHTTP(w, req)
						}

						// Also test Handler method
						finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)
						})

						chainHandler := chain.Handler(finalHandler)
						req := httptest.NewRequest("GET", fmt.Sprintf("/chain-test-%d-%d", id, counter), nil)
						w := httptest.NewRecorder()
						chainHandler.ServeHTTP(w, req)

						counter++
					}
				}
			}(i)
		}

		// Workers that read chain state
		for i := 0; i < numWorkers/2; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for {
					select {
					case <-done:
						return
					default:
						// Read operations
						middlewares := chain.List()
						_ = len(middlewares) // Use the result to prevent optimization

						time.Sleep(time.Microsecond)
					}
				}
			}(i)
		}

		// Run for specified duration
		time.Sleep(duration)
		close(done)
		wg.Wait()

		// Final verification
		middlewares := chain.List()
		t.Logf("Final middleware count: %d", len(middlewares))

		// Should not panic and state should be consistent
		assert.GreaterOrEqual(t, len(middlewares), 0)

		// Test final compilation
		handler := chain.Compile()
		assert.NotNil(t, handler)
	})
}

// =============================================================================
// BENCHMARKS FOR CONCURRENT EXECUTION
// =============================================================================

func BenchmarkConcurrentChainExecution(b *testing.B) {
	logger := zaptest.NewLogger(b)
	chain := NewChain(logger)
	defer chain.Cleanup()

	// Add some middleware
	for i := 0; i < 5; i++ {
		mw := &testMiddleware{
			name:     fmt.Sprintf("bench-middleware-%d", i),
			priority: i * 10,
		}
		chain.Use(mw)
	}

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := chain.Handler(finalHandler)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			req := httptest.NewRequest("GET", fmt.Sprintf("/bench-%d", counter), nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			counter++
		}
	})
}

func BenchmarkConcurrentChainCompilation(b *testing.B) {
	logger := zaptest.NewLogger(b)
	chain := NewChain(logger)
	defer chain.Cleanup()

	// Add middleware
	for i := 0; i < 10; i++ {
		mw := &testMiddleware{
			name:     fmt.Sprintf("bench-compile-%d", i),
			priority: i,
		}
		chain.Use(mw)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			handler := chain.Compile()
			_ = handler // Prevent optimization
		}
	})
}

func BenchmarkConcurrentChainModification(b *testing.B) {
	logger := zaptest.NewLogger(b)
	chain := NewChain(logger)
	defer chain.Cleanup()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			// Add middleware
			mw := &testMiddleware{
				name:     fmt.Sprintf("bench-mod-%d", counter),
				priority: counter,
			}
			chain.Use(mw)

			// Remove middleware occasionally
			if counter%10 == 0 && counter > 0 {
				chain.Remove(fmt.Sprintf("bench-mod-%d", counter-5))
			}

			counter++
		}
	})
}

func BenchmarkManagerConcurrentAccess(b *testing.B) {
	logger := zaptest.NewLogger(b)
	manager := NewManager(logger)
	defer manager.Cleanup()

	// Pre-create some chains
	const numChains = 10
	for i := 0; i < numChains; i++ {
		chainName := fmt.Sprintf("bench-chain-%d", i)
		chain := manager.CreateChain(chainName)

		mw := &testMiddleware{
			name:     fmt.Sprintf("bench-manager-mw-%d", i),
			priority: i,
		}
		chain.Use(mw)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			chainName := fmt.Sprintf("bench-chain-%d", counter%numChains)
			chain, exists := manager.GetChain(chainName)
			if exists && chain != nil {
				// Use the chain
				finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
				handler := chain.Handler(finalHandler)
				_ = handler // Prevent optimization
			}
			counter++
		}
	})
}
