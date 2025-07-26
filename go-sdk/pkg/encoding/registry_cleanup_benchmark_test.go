package encoding

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkFormatRegistryWithCleanup benchmarks format registry operations with cleanup enabled
func BenchmarkFormatRegistryWithCleanup(b *testing.B) {
	config := &RegistryConfig{
		MaxEntries:              1000,
		TTL:                     1 * time.Hour, // Long TTL to avoid cleanup during benchmark
		CleanupInterval:         30 * time.Minute,
		EnableLRU:               true,
		EnableBackgroundCleanup: false, // Disable background cleanup for consistent benchmarks
	}

	registry := NewFormatRegistryWithConfig(config)
	defer registry.Close()

	b.Run("RegisterFormat", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			info := &FormatInfo{
				MIMEType:    fmt.Sprintf("application/bench-%d", i),
				Name:        fmt.Sprintf("Benchmark Format %d", i),
				Description: fmt.Sprintf("Benchmark test format %d", i),
				Capabilities: FormatCapabilities{
					Streaming:        true,
					BinaryEfficient:  false,
					HumanReadable:    true,
					SelfDescribing:   true,
					SchemaValidation: false,
					Compression:      false,
					Versionable:      true,
				},
			}
			_ = registry.RegisterFormat(info) // Ignore errors due to eviction
		}
	})

	// Pre-populate registry for get benchmarks
	for i := 0; i < 500; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/get-bench-%d", i),
			Name:        fmt.Sprintf("Get Benchmark Format %d", i),
			Description: fmt.Sprintf("Get benchmark test format %d", i),
			Capabilities: FormatCapabilities{
				Streaming:        true,
				BinaryEfficient:  false,
				HumanReadable:    true,
				SelfDescribing:   true,
				SchemaValidation: false,
				Compression:      false,
				Versionable:      true,
			},
		}
		_ = registry.RegisterFormat(info)
	}

	b.Run("GetFormat", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mimeType := fmt.Sprintf("application/get-bench-%d", i%500)
			_, _ = registry.GetFormat(mimeType) // Ignore errors
		}
	})

	b.Run("ListFormats", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.ListFormats()
		}
	})

	b.Run("SupportsFormat", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mimeType := fmt.Sprintf("application/get-bench-%d", i%500)
			_ = registry.SupportsFormat(mimeType)
		}
	})
}

// BenchmarkFormatRegistryWithoutCleanup benchmarks format registry operations without cleanup for comparison
func BenchmarkFormatRegistryWithoutCleanup(b *testing.B) {
	config := &RegistryConfig{
		MaxEntries:              0, // Unlimited
		TTL:                     0, // No TTL
		EnableLRU:               false,
		EnableBackgroundCleanup: false,
	}

	registry := NewFormatRegistryWithConfig(config)
	defer registry.Close()

	b.Run("RegisterFormat", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			info := &FormatInfo{
				MIMEType:    fmt.Sprintf("application/bench-no-cleanup-%d", i),
				Name:        fmt.Sprintf("Benchmark Format No Cleanup %d", i),
				Description: fmt.Sprintf("Benchmark test format no cleanup %d", i),
				Capabilities: FormatCapabilities{
					Streaming:        true,
					BinaryEfficient:  false,
					HumanReadable:    true,
					SelfDescribing:   true,
					SchemaValidation: false,
					Compression:      false,
					Versionable:      true,
				},
			}
			_ = registry.RegisterFormat(info)
		}
	})

	// Pre-populate registry for get benchmarks
	for i := 0; i < 500; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/get-bench-no-cleanup-%d", i),
			Name:        fmt.Sprintf("Get Benchmark Format No Cleanup %d", i),
			Description: fmt.Sprintf("Get benchmark test format no cleanup %d", i),
			Capabilities: FormatCapabilities{
				Streaming:        true,
				BinaryEfficient:  false,
				HumanReadable:    true,
				SelfDescribing:   true,
				SchemaValidation: false,
				Compression:      false,
				Versionable:      true,
			},
		}
		_ = registry.RegisterFormat(info)
	}

	b.Run("GetFormat", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mimeType := fmt.Sprintf("application/get-bench-no-cleanup-%d", i%500)
			_, _ = registry.GetFormat(mimeType) // Ignore errors
		}
	})

	b.Run("ListFormats", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.ListFormats()
		}
	})

	b.Run("SupportsFormat", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mimeType := fmt.Sprintf("application/get-bench-no-cleanup-%d", i%500)
			_ = registry.SupportsFormat(mimeType)
		}
	})
}

// BenchmarkFormatRegistryCleanupOperations benchmarks the cleanup operations themselves
func BenchmarkFormatRegistryCleanupOperations(b *testing.B) {
	config := &RegistryConfig{
		MaxEntries:              10000,
		TTL:                     1 * time.Millisecond, // Very short TTL for cleanup testing
		EnableLRU:               true,
		EnableBackgroundCleanup: false,
	}

	registry := NewFormatRegistryWithConfig(config)
	defer registry.Close()

	// Pre-populate registry with formats that will expire
	for i := 0; i < 1000; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/cleanup-%d", i),
			Name:        fmt.Sprintf("Cleanup Format %d", i),
			Description: fmt.Sprintf("Cleanup benchmark test format %d", i),
			Capabilities: FormatCapabilities{
				Streaming:        true,
				BinaryEfficient:  false,
				HumanReadable:    true,
				SelfDescribing:   true,
				SchemaValidation: false,
				Compression:      false,
				Versionable:      true,
			},
		}
		_ = registry.RegisterFormat(info)
	}

	// Wait for formats to expire
	time.Sleep(5 * time.Millisecond)

	b.Run("CleanupExpired", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Re-populate between iterations since cleanup removes formats
			if i > 0 {
				for j := 0; j < 100; j++ {
					info := &FormatInfo{
						MIMEType:    fmt.Sprintf("application/cleanup-%d-%d", i, j),
						Name:        fmt.Sprintf("Cleanup Format %d-%d", i, j),
						Description: fmt.Sprintf("Cleanup benchmark test format %d-%d", i, j),
						Capabilities: FormatCapabilities{
							Streaming:        true,
							BinaryEfficient:  false,
							HumanReadable:    true,
							SelfDescribing:   true,
							SchemaValidation: false,
							Compression:      false,
							Versionable:      true,
						},
					}
					_ = registry.RegisterFormat(info)
				}
				time.Sleep(5 * time.Millisecond) // Let them expire
			}
			
			_, _ = registry.CleanupExpired()
		}
	})

	// Re-populate for access time cleanup benchmark
	for i := 0; i < 1000; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/access-cleanup-%d", i),
			Name:        fmt.Sprintf("Access Cleanup Format %d", i),
			Description: fmt.Sprintf("Access cleanup benchmark test format %d", i),
			Capabilities: FormatCapabilities{
				Streaming:        true,
				BinaryEfficient:  false,
				HumanReadable:    true,
				SelfDescribing:   true,
				SchemaValidation: false,
				Compression:      false,
				Versionable:      true,
			},
		}
		_ = registry.RegisterFormat(info)
	}

	time.Sleep(5 * time.Millisecond) // Age the formats

	b.Run("CleanupByAccessTime", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Re-populate between iterations
			if i > 0 {
				for j := 0; j < 100; j++ {
					info := &FormatInfo{
						MIMEType:    fmt.Sprintf("application/access-cleanup-%d-%d", i, j),
						Name:        fmt.Sprintf("Access Cleanup Format %d-%d", i, j),
						Description: fmt.Sprintf("Access cleanup benchmark test format %d-%d", i, j),
						Capabilities: FormatCapabilities{
							Streaming:        true,
							BinaryEfficient:  false,
							HumanReadable:    true,
							SelfDescribing:   true,
							SchemaValidation: false,
							Compression:      false,
							Versionable:      true,
						},
					}
					_ = registry.RegisterFormat(info)
				}
				time.Sleep(5 * time.Millisecond) // Age them
			}
			
			_, _ = registry.CleanupByAccessTime(1 * time.Millisecond)
		}
	})

	b.Run("ClearAll", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Re-populate between iterations
			if i > 0 {
				for j := 0; j < 100; j++ {
					info := &FormatInfo{
						MIMEType:    fmt.Sprintf("application/clear-%d-%d", i, j),
						Name:        fmt.Sprintf("Clear Format %d-%d", i, j),
						Description: fmt.Sprintf("Clear benchmark test format %d-%d", i, j),
						Capabilities: FormatCapabilities{
							Streaming:        true,
							BinaryEfficient:  false,
							HumanReadable:    true,
							SelfDescribing:   true,
							SchemaValidation: false,
							Compression:      false,
							Versionable:      true,
						},
					}
					_ = registry.RegisterFormat(info)
				}
			}
			
			_ = registry.ClearAll()
		}
	})
}

// BenchmarkFormatRegistryLRUEviction benchmarks LRU eviction performance
func BenchmarkFormatRegistryLRUEviction(b *testing.B) {
	config := &RegistryConfig{
		MaxEntries:              100, // Small limit to force frequent evictions
		TTL:                     0,   // No TTL
		EnableLRU:               true,
		EnableBackgroundCleanup: false,
	}

	registry := NewFormatRegistryWithConfig(config)
	defer registry.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/lru-%d", i),
			Name:        fmt.Sprintf("LRU Format %d", i),
			Description: fmt.Sprintf("LRU benchmark test format %d", i),
			Capabilities: FormatCapabilities{
				Streaming:        true,
				BinaryEfficient:  false,
				HumanReadable:    true,
				SelfDescribing:   true,
				SchemaValidation: false,
				Compression:      false,
				Versionable:      true,
			},
		}
		_ = registry.RegisterFormat(info) // Will trigger evictions after first 100
	}
}

// BenchmarkFormatRegistryAliasResolution benchmarks alias resolution performance
func BenchmarkFormatRegistryAliasResolution(b *testing.B) {
	registry := NewFormatRegistry()
	defer registry.Close()

	// Pre-populate with formats and aliases
	for i := 0; i < 500; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/alias-test-%d", i),
			Name:        fmt.Sprintf("Alias Test Format %d", i),
			Description: fmt.Sprintf("Alias benchmark test format %d", i),
			Aliases:     []string{fmt.Sprintf("alias-%d", i), fmt.Sprintf("alt-%d", i)},
			Capabilities: FormatCapabilities{
				Streaming:        true,
				BinaryEfficient:  false,
				HumanReadable:    true,
				SelfDescribing:   true,
				SchemaValidation: false,
				Compression:      false,
				Versionable:      true,
			},
		}
		_ = registry.RegisterFormat(info)
	}

	b.Run("ResolveAlias", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			alias := fmt.Sprintf("alias-%d", i%500)
			_, _ = registry.GetFormat(alias) // This will test alias resolution
		}
	})

	b.Run("ResolveCanonical", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mimeType := fmt.Sprintf("application/alias-test-%d", i%500)
			_, _ = registry.GetFormat(mimeType) // This tests direct lookup
		}
	})
}

// BenchmarkFormatRegistryStats benchmarks registry statistics operations
func BenchmarkFormatRegistryStats(b *testing.B) {
	registry := NewFormatRegistry()
	defer registry.Close()

	// Pre-populate with some formats
	for i := 0; i < 100; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/stats-%d", i),
			Name:        fmt.Sprintf("Stats Format %d", i),
			Description: fmt.Sprintf("Stats benchmark test format %d with longer description for more realistic memory usage", i),
			Aliases:     []string{fmt.Sprintf("stats-%d", i), fmt.Sprintf("s%d", i)},
			Capabilities: FormatCapabilities{
				Streaming:        true,
				BinaryEfficient:  false,
				HumanReadable:    true,
				SelfDescribing:   true,
				SchemaValidation: false,
				Compression:      false,
				Versionable:      true,
			},
		}
		_ = registry.RegisterFormat(info)
	}

	b.Run("GetRegistryStats", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.GetRegistryStats()
		}
	})
}

// BenchmarkFormatRegistryConcurrentOperations benchmarks concurrent operations with cleanup
func BenchmarkFormatRegistryConcurrentOperations(b *testing.B) {
	config := &RegistryConfig{
		MaxEntries:              1000,
		TTL:                     1 * time.Hour, // Long TTL
		EnableLRU:               true,
		EnableBackgroundCleanup: false,
	}

	registry := NewFormatRegistryWithConfig(config)
	defer registry.Close()

	// Pre-populate for get operations
	for i := 0; i < 500; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/concurrent-%d", i),
			Name:        fmt.Sprintf("Concurrent Format %d", i),
			Description: fmt.Sprintf("Concurrent benchmark test format %d", i),
			Capabilities: FormatCapabilities{
				Streaming:        true,
				BinaryEfficient:  false,
				HumanReadable:    true,
				SelfDescribing:   true,
				SchemaValidation: false,
				Compression:      false,
				Versionable:      true,
			},
		}
		_ = registry.RegisterFormat(info)
	}

	b.Run("ConcurrentRegister", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				info := &FormatInfo{
					MIMEType:    fmt.Sprintf("application/parallel-%d", i),
					Name:        fmt.Sprintf("Parallel Format %d", i),
					Description: fmt.Sprintf("Parallel benchmark test format %d", i),
					Capabilities: FormatCapabilities{
						Streaming:        true,
						BinaryEfficient:  false,
						HumanReadable:    true,
						SelfDescribing:   true,
						SchemaValidation: false,
						Compression:      false,
						Versionable:      true,
					},
				}
				_ = registry.RegisterFormat(info)
				i++
			}
		})
	})

	b.Run("ConcurrentGet", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				mimeType := fmt.Sprintf("application/concurrent-%d", i%500)
				_, _ = registry.GetFormat(mimeType)
				i++
			}
		})
	})

	b.Run("ConcurrentList", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = registry.ListFormats()
			}
		})
	})

	b.Run("ConcurrentSupports", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				mimeType := fmt.Sprintf("application/concurrent-%d", i%500)
				_ = registry.SupportsFormat(mimeType)
				i++
			}
		})
	})
}