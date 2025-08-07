package security

import (
	"strings"
	"testing"
)

// BenchmarkStringConcatenation tests the old string concatenation approach
func BenchmarkStringConcatenation(b *testing.B) {
	parts := []string{"Hello", "world", "this", "is", "a", "test", "of", "string", "performance"}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var result string
			for _, part := range parts {
				result += part + " "
			}
			_ = result
		}
	})
}

// BenchmarkStringBuilder tests the optimized strings.Builder approach
func BenchmarkStringBuilder(b *testing.B) {
	parts := []string{"Hello", "world", "this", "is", "a", "test", "of", "string", "performance"}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var builder strings.Builder
			// Pre-allocate capacity for better performance
			totalLen := 0
			for _, part := range parts {
				totalLen += len(part) + 1 // +1 for space
			}
			builder.Grow(totalLen)
			
			for _, part := range parts {
				builder.WriteString(part)
				builder.WriteString(" ")
			}
			_ = builder.String()
		}
	})
}

// BenchmarkMapPathLookup tests the old map-based path lookup
func BenchmarkMapPathLookup(b *testing.B) {
	// Create a map with many paths for realistic comparison
	pathMap := make(map[string]bool)
	paths := []string{
		"/health", "/healthz", "/ping", "/ready", "/live",
		"/api/v1/users", "/api/v1/orders", "/api/v1/products",
		"/static/css", "/static/js", "/static/images",
		"/admin/dashboard", "/admin/users", "/admin/settings",
		"/public/docs", "/public/help", "/public/about",
	}
	
	for _, path := range paths {
		pathMap[path] = true
	}
	
	testPaths := []string{
		"/health", "/api/v1/users", "/static/css", "/admin/dashboard", "/nonexistent",
		"/healthz", "/api/v1/orders", "/static/js", "/admin/users", "/also/nonexistent",
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, testPath := range testPaths {
				_ = pathMap[testPath]
			}
		}
	})
}

// BenchmarkTriePathLookup tests the new trie-based path lookup
func BenchmarkTriePathLookup(b *testing.B) {
	// Create a trie with the same paths for comparison
	trie := NewPathTrie()
	paths := []string{
		"/health", "/healthz", "/ping", "/ready", "/live",
		"/api/v1/users", "/api/v1/orders", "/api/v1/products",
		"/static/css", "/static/js", "/static/images",
		"/admin/dashboard", "/admin/users", "/admin/settings",
		"/public/docs", "/public/help", "/public/about",
	}
	
	for _, path := range paths {
		trie.AddPath(path)
	}
	
	testPaths := []string{
		"/health", "/api/v1/users", "/static/css", "/admin/dashboard", "/nonexistent",
		"/healthz", "/api/v1/orders", "/static/js", "/admin/users", "/also/nonexistent",
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, testPath := range testPaths {
				_ = trie.MatchesPath(testPath)
			}
		}
	})
}

// BenchmarkTrieVsMapScalability tests scalability with many paths
func BenchmarkTrieVsMapScalability_Map(b *testing.B) {
	// Create 1000 paths for scalability testing
	pathMap := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		pathMap["/api/v"+string(rune(i%10))+"/resource"+string(rune(i%100))] = true
	}
	
	testPath := "/api/v5/resource50"
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = pathMap[testPath]
		}
	})
}

func BenchmarkTrieVsMapScalability_Trie(b *testing.B) {
	// Create trie with 1000 paths for scalability testing
	trie := NewPathTrie()
	for i := 0; i < 1000; i++ {
		trie.AddPath("/api/v" + string(rune(i%10)) + "/resource" + string(rune(i%100)))
	}
	
	testPath := "/api/v5/resource50"
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = trie.MatchesPath(testPath)
		}
	})
}

// BenchmarkPathPrefixMatching tests prefix matching capability (trie advantage)
func BenchmarkPathPrefixMatching_Map(b *testing.B) {
	// Map cannot efficiently do prefix matching - must check all possibilities
	pathMap := make(map[string]bool)
	prefixes := []string{"/api/v1", "/api/v2", "/admin", "/public", "/static"}
	for _, prefix := range prefixes {
		pathMap[prefix] = true
	}
	
	testPath := "/api/v1/users/123/profile"
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			found := false
			// Simulate prefix matching with map - very inefficient
			for prefix := range pathMap {
				if strings.HasPrefix(testPath, prefix) {
					found = true
					break
				}
			}
			_ = found
		}
	})
}

func BenchmarkPathPrefixMatching_Trie(b *testing.B) {
	// Trie excels at prefix matching
	trie := NewPathTrie()
	prefixes := []string{"/api/v1", "/api/v2", "/admin", "/public", "/static"}
	for _, prefix := range prefixes {
		trie.AddPath(prefix)
	}
	
	testPath := "/api/v1/users/123/profile"
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = trie.MatchesPath(testPath)
		}
	})
}