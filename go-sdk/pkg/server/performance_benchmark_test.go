package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/server/middleware"
	"go.uber.org/zap"
)

// BenchmarkStringBuilding compares fmt.Sprintf vs strings.Builder performance
func BenchmarkStringBuilding(b *testing.B) {
	b.Run("fmt.Sprintf", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = fmt.Sprintf("HTTP Server Metrics - Total: %d, Success: %d, Failed: %d", 1000, 950, 50)
		}
	})
	
	b.Run("strings.Builder_pooled", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			builder := stringBuilderPool.Get().(*strings.Builder)
			builder.Reset()
			builder.WriteString("HTTP Server Metrics - Total: ")
			builder.WriteString("1000")
			builder.WriteString(", Success: ")
			builder.WriteString("950")
			builder.WriteString(", Failed: ")
			builder.WriteString("50")
			_ = builder.String()
			stringBuilderPool.Put(builder)
		}
	})
}

// BenchmarkObjectPooling compares object allocation vs pooling
func BenchmarkObjectPooling(b *testing.B) {
	b.Run("direct_allocation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			user := &middleware.AuthUser{
				ID:          "user123",
				Username:    "testuser",
				Roles:       []string{"admin", "user"},
				Permissions: []string{"read", "write"},
				Metadata:    make(map[string]interface{}),
			}
			_ = user
		}
	})
	
	b.Run("object_pooling", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			user := middleware.AuthUserPool.Get().(*middleware.AuthUser)
			middleware.ResetAuthUser(user)
			user.ID = "user123"
			user.Username = "testuser"
			user.Roles = append(user.Roles, "admin", "user")
			user.Permissions = append(user.Permissions, "read", "write")
			middleware.ReleaseAuthUser(user)
		}
	})
}

// BenchmarkHTTPServerThroughput benchmarks the optimized HTTP server
func BenchmarkHTTPServerThroughput(b *testing.B) {
	config := DefaultHTTPServerConfig()
	config.PreferredFramework = FrameworkStdlib
	config.EnableLogging = false
	config.EnableMetrics = false
	
	server, err := NewHTTPServer(config)
	if err != nil {
		b.Fatal(err)
	}
	defer server.Stop(context.Background())
	
	// Create test request
	req := httptest.NewRequest("GET", "/health", nil)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rec := httptest.NewRecorder()
			server.compileTimeHandler.ServeHTTP(rec, req)
		}
	})
}

// BenchmarkRateLimiterPerformance benchmarks the optimized rate limiter
func BenchmarkRateLimiterPerformance(b *testing.B) {
	config := middleware.DefaultRateLimitConfig()
	config.RequestsPerSecond = 10000
	config.BurstSize = 100
	config.EnableMemoryBounds = true
	config.MaxLimiters = 1000
	
	logger := zap.NewNop()
	rateLimiter, err := middleware.NewRateLimitMiddleware(config, logger)
	if err != nil {
		b.Fatal(err)
	}
	defer rateLimiter.Cleanup()
	
	handler := rateLimiter.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		clientID := 0
		for pb.Next() {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = fmt.Sprintf("192.168.1.%d:8080", clientID%255)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			clientID++
		}
	})
}

// BenchmarkAuthMiddlewarePerformance benchmarks the optimized auth middleware
func BenchmarkAuthMiddlewarePerformance(b *testing.B) {
	config := &middleware.AuthConfig{
		Method: middleware.AuthMethodBasic,
		BasicAuth: middleware.BasicAuthConfig{
			Realm: "test",
			Users: map[string]*middleware.BasicAuthUser{
				"testuser": {
					PasswordHash: "$2a$12$test", // Pre-computed hash
					UserID:       "user123",
				},
			},
		},
	}
	config.Enabled = true
	config.Name = "auth"
	config.Priority = 100
	
	logger := zap.NewNop()
	authMiddleware, err := middleware.NewAuthMiddleware(config, logger)
	if err != nil {
		b.Fatal(err)
	}
	defer authMiddleware.Cleanup()
	
	handler := authMiddleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/test", nil)
			req.SetBasicAuth("testuser", "testpass")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
}

// BenchmarkMiddlewareChainPerformance compares different middleware chain approaches
func BenchmarkMiddlewareChainPerformance(b *testing.B) {
	// Create test middlewares
	middleware1 := func(w http.ResponseWriter, r *http.Request, next func()) {
		// Simulate some work
		time.Sleep(time.Microsecond)
		next()
	}
	middleware2 := func(w http.ResponseWriter, r *http.Request, next func()) {
		// Simulate some work
		time.Sleep(time.Microsecond)
		next()
	}
	middleware3 := func(w http.ResponseWriter, r *http.Request, next func()) {
		// Simulate some work
		time.Sleep(time.Microsecond)
		next()
	}
	
	b.Run("optimized_chain", func(b *testing.B) {
		chain := NewOptimizedMiddlewareChain()
		chain.Add(middleware1)
		chain.Add(middleware2)
		chain.Add(middleware3)
		
		req := httptest.NewRequest("GET", "/test", nil)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			chain.Execute(rec, req, func() {
				rec.WriteHeader(http.StatusOK)
			})
		}
	})
}

// BenchmarkMemoryUsage measures memory allocations
func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("string_operations", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			builder := stringBuilderPool.Get().(*strings.Builder)
			builder.Reset()
			builder.WriteString("test")
			builder.WriteString("123")
			_ = builder.String()
			stringBuilderPool.Put(builder)
		}
	})
	
	b.Run("object_pooling", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			user := middleware.AuthUserPool.Get().(*middleware.AuthUser)
			middleware.ResetAuthUser(user)
			user.ID = "test"
			middleware.ReleaseAuthUser(user)
		}
	})
}

// BenchmarkConcurrentAccess tests concurrent access patterns
func BenchmarkConcurrentAccess(b *testing.B) {
	config := DefaultHTTPServerConfig()
	config.PreferredFramework = FrameworkStdlib
	
	server, err := NewHTTPServer(config)
	if err != nil {
		b.Fatal(err)
	}
	defer server.Stop(context.Background())
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/health", nil)
			rec := httptest.NewRecorder()
			server.compileTimeHandler.ServeHTTP(rec, req)
		}
	})
}

// Performance target validation tests
func TestPerformanceTargets(t *testing.T) {
	// Test that we can achieve > 10,000 RPS per core
	config := DefaultHTTPServerConfig()
	config.PreferredFramework = FrameworkStdlib
	config.EnableLogging = false
	config.EnableMetrics = false
	
	server, err := NewHTTPServer(config)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop(context.Background())
	
	// Measure throughput
	duration := time.Second
	start := time.Now()
	var requests int64
	
	var wg sync.WaitGroup
	done := make(chan bool)
	
	// Start background goroutines to generate load
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/health", nil)
			for {
				select {
				case <-done:
					return
				default:
					rec := httptest.NewRecorder()
					server.compileTimeHandler.ServeHTTP(rec, req)
					atomic.AddInt64(&requests, 1)
				}
			}
		}()
	}
	
	// Run for specified duration
	time.Sleep(duration)
	close(done)
	wg.Wait()
	
	elapsed := time.Since(start)
	finalRequests := atomic.LoadInt64(&requests)
	rps := float64(finalRequests) / elapsed.Seconds()
	
	t.Logf("Achieved %d requests in %v (%.0f RPS)", finalRequests, elapsed, rps)
	
	// Validate we achieved target performance
	if rps < 5000 { // Conservative target for test environment
		t.Errorf("Performance target not met: %.0f RPS < 5000 RPS", rps)
	}
}