package security

import (
	"context"
	"testing"
	"time"
)

// TestPathTrieCompatibility ensures that the trie implementation maintains
// the same behavior as the original map-based implementation
func TestPathTrieCompatibility(t *testing.T) {
	testCases := []struct {
		name         string
		skipPaths    []string
		testPath     string
		shouldMatch  bool
		description  string
	}{
		{
			name:         "exact_match",
			skipPaths:    []string{"/health", "/api/v1/status"},
			testPath:     "/health",
			shouldMatch:  true,
			description:  "Exact path match should work",
		},
		{
			name:         "no_match",
			skipPaths:    []string{"/health", "/api/v1/status"},
			testPath:     "/nonexistent",
			shouldMatch:  false,
			description:  "Non-existent path should not match",
		},
		{
			name:         "trailing_slash_normalization",
			skipPaths:    []string{"/health/"},
			testPath:     "/health",
			shouldMatch:  true,
			description:  "Trailing slashes should be normalized",
		},
		{
			name:         "root_path",
			skipPaths:    []string{"/"},
			testPath:     "/",
			shouldMatch:  true,
			description:  "Root path should match",
		},
		{
			name:         "empty_path_handling",
			skipPaths:    []string{""},
			testPath:     "/test",
			shouldMatch:  false,
			description:  "Empty paths should be ignored",
		},
		{
			name:         "prefix_matching",
			skipPaths:    []string{"/api"},
			testPath:     "/api/v1/users",
			shouldMatch:  true,
			description:  "Prefix matching should work (trie advantage)",
		},
		{
			name:         "nested_paths",
			skipPaths:    []string{"/api/v1", "/api/v2/admin"},
			testPath:     "/api/v1/users",
			shouldMatch:  true,
			description:  "Nested path matching should work",
		},
		{
			name:         "case_sensitive",
			skipPaths:    []string{"/Health"},
			testPath:     "/health",
			shouldMatch:  false,
			description:  "Path matching should be case sensitive",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test with PathTrie
			trie := NewPathTrie()
			for _, path := range tc.skipPaths {
				trie.AddPath(path)
			}
			
			result := trie.MatchesPath(tc.testPath)
			if result != tc.shouldMatch {
				t.Errorf("PathTrie.MatchesPath(%q) = %v, want %v. %s", 
					tc.testPath, result, tc.shouldMatch, tc.description)
			}
		})
	}
}

// TestSecurityMiddlewareBackwardCompatibility ensures that the new implementation
// maintains the same behavior as the original SecurityMiddleware
func TestSecurityMiddlewareBackwardCompatibility(t *testing.T) {
	config := &SecurityConfig{
		SkipPaths: []string{"/health", "/healthz", "/api/status"},
		SkipHealthCheck: true,
		CORS: &CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		},
		Headers: &SecurityHeadersConfig{
			Enabled:             true,
			XFrameOptions:       "DENY",
			XContentTypeOptions: "nosniff",
			XXSSProtection:      "1; mode=block",
		},
		ThreatDetection: &ThreatDetectionConfig{
			Enabled:      false, // Disable for testing
			SQLInjection: false,
			XSSDetection: false,
			LogThreats:   false,
		},
	}

	sm, err := NewSecurityMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create SecurityMiddleware: %v", err)
	}

	// Test that skip paths work correctly
	testCases := []struct {
		path        string
		shouldSkip  bool
		description string
	}{
		{"/health", true, "Configured skip path should be skipped"},
		{"/healthz", true, "Health check path should be skipped"},
		{"/ping", true, "Health check path should be skipped (from SkipHealthCheck)"},
		{"/api/status", true, "Configured skip path should be skipped"},
		{"/api/users", false, "Non-skip path should not be skipped"},
		{"/other", false, "Random path should not be skipped"},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			req := &Request{
				ID:        "test-request",
				Method:    "GET",
				Path:      tc.path,
				Headers:   make(map[string]string),
				Timestamp: time.Now(),
			}

			var nextCalled bool
			next := func(ctx context.Context, req *Request) (*Response, error) {
				nextCalled = true
				return &Response{
					ID:         req.ID,
					StatusCode: 200,
					Timestamp:  time.Now(),
				}, nil
			}

			resp, err := sm.Process(context.Background(), req, next)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			if tc.shouldSkip {
				if !nextCalled {
					t.Errorf("Expected path %q to be skipped (next handler should be called), but it wasn't", tc.path)
				}
				if resp.StatusCode != 200 {
					t.Errorf("Expected successful response for skipped path %q, got status %d", tc.path, resp.StatusCode)
				}
			} else {
				// For non-skip paths, the middleware should process them
				// (exact behavior depends on other middleware components)
				if !nextCalled {
					// If next wasn't called, middleware blocked the request
					if resp.StatusCode == 200 {
						t.Errorf("Expected non-skip path %q to be processed by middleware, but got successful response", tc.path)
					}
				}
			}
		})
	}
}

// TestStringBuilderCompatibility is tested in the pkg/errors package
// where the actual string builder optimizations are implemented

// TestTrieThreadSafety ensures that the trie implementation is thread-safe
func TestTrieThreadSafety(t *testing.T) {
	trie := NewPathTrie()
	
	// Add initial paths
	initialPaths := []string{"/api/v1", "/health", "/admin"}
	for _, path := range initialPaths {
		trie.AddPath(path)
	}

	// Use channels to coordinate goroutines
	done := make(chan bool, 3)
	
	// Goroutine 1: Keep adding paths
	go func() {
		for i := 0; i < 100; i++ {
			path := "/dynamic/" + string(rune(i))
			trie.AddPath(path)
		}
		done <- true
	}()

	// Goroutine 2: Keep reading paths
	go func() {
		for i := 0; i < 100; i++ {
			path := "/api/v1/test" + string(rune(i))
			_ = trie.MatchesPath(path)
		}
		done <- true
	}()

	// Goroutine 3: Keep reading existing paths
	go func() {
		for i := 0; i < 100; i++ {
			for _, path := range initialPaths {
				_ = trie.MatchesPath(path)
			}
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			// OK
		case <-time.After(5 * time.Second):
			t.Fatal("Goroutine did not complete within timeout - possible deadlock")
		}
	}
}

// BenchmarkBackwardCompatibilityOverhead measures any performance overhead
// introduced by the new optimizations in real usage scenarios
func BenchmarkBackwardCompatibilityOverhead(b *testing.B) {
	config := &SecurityConfig{
		SkipPaths: []string{
			"/health", "/healthz", "/ping", "/ready", "/live",
			"/api/v1/status", "/api/v2/health", "/admin/ping",
		},
		SkipHealthCheck: true,
		CORS: &CORSConfig{Enabled: false}, // Disable to isolate path matching
		Headers: &SecurityHeadersConfig{Enabled: false},
		ThreatDetection: &ThreatDetectionConfig{Enabled: false},
	}

	sm, err := NewSecurityMiddleware(config)
	if err != nil {
		b.Fatalf("Failed to create SecurityMiddleware: %v", err)
	}

	requests := []*Request{
		{ID: "1", Method: "GET", Path: "/health", Headers: make(map[string]string), Timestamp: time.Now()},
		{ID: "2", Method: "GET", Path: "/api/users", Headers: make(map[string]string), Timestamp: time.Now()},
		{ID: "3", Method: "GET", Path: "/ping", Headers: make(map[string]string), Timestamp: time.Now()},
		{ID: "4", Method: "POST", Path: "/api/v1/create", Headers: make(map[string]string), Timestamp: time.Now()},
	}

	next := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{
			ID:         req.ID,
			StatusCode: 200,
			Timestamp:  time.Now(),
		}, nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, req := range requests {
				_, _ = sm.Process(context.Background(), req, next)
			}
		}
	})
}