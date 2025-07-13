package encoding

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatRegistryTTLCleanup tests TTL-based cleanup
func TestFormatRegistryTTLCleanup(t *testing.T) {
	config := &RegistryConfig{
		MaxEntries:              100,
		TTL:                     100 * time.Millisecond, // Very short TTL
		CleanupInterval:         50 * time.Millisecond,  // Frequent cleanup
		EnableLRU:               false,
		EnableBackgroundCleanup: false, // Manual cleanup for testing
	}

	registry := NewFormatRegistryWithConfig(config)
	defer registry.Close()

	// Register some formats
	for i := 0; i < 3; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/test-%d", i),
			Name:        fmt.Sprintf("Test Format %d", i),
			Description: fmt.Sprintf("Test format %d", i),
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
		err := registry.RegisterFormat(info)
		require.NoError(t, err)
	}

	// Verify formats are present initially
	stats := registry.GetRegistryStats()
	assert.Equal(t, 3, stats["formats_count"])

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Run manual cleanup
	cleaned, err := registry.CleanupExpired()
	require.NoError(t, err)
	assert.Equal(t, 3, cleaned, "All formats should be cleaned up due to TTL expiry")

	// Verify formats are gone
	afterStats := registry.GetRegistryStats()
	assert.Equal(t, 0, afterStats["formats_count"])

	// Verify specific formats are gone
	for i := 0; i < 3; i++ {
		_, err := registry.GetFormat(fmt.Sprintf("application/test-%d", i))
		assert.Error(t, err, "Formats should be removed after TTL cleanup")
	}
}

// TestFormatRegistryLRUEviction tests LRU eviction when MaxEntries is reached
func TestFormatRegistryLRUEviction(t *testing.T) {
	config := &RegistryConfig{
		MaxEntries:              3,  // Small limit to test eviction
		TTL:                     0,  // Disable TTL
		EnableLRU:               true,
		EnableBackgroundCleanup: false,
	}

	registry := NewFormatRegistryWithConfig(config)
	defer registry.Close()

	// Register more formats than the limit
	for i := 0; i < 6; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/test-%d", i),
			Name:        fmt.Sprintf("Test Format %d", i),
			Description: fmt.Sprintf("Test format %d", i),
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
		err := registry.RegisterFormat(info)
		require.NoError(t, err)
	}

	// Verify only MaxEntries are kept
	stats := registry.GetRegistryStats()
	assert.Equal(t, config.MaxEntries, stats["formats_count"])

	// Verify oldest formats were evicted (LRU)
	for i := 0; i < 3; i++ {
		_, err := registry.GetFormat(fmt.Sprintf("application/test-%d", i))
		assert.Error(t, err, "Oldest formats should have been evicted")
	}

	// Verify newest formats are still present
	for i := 3; i < 6; i++ {
		format, err := registry.GetFormat(fmt.Sprintf("application/test-%d", i))
		assert.NoError(t, err, "Newest formats should still be present")
		assert.NotNil(t, format)
	}
}

// TestFormatRegistryAliasesCleanup tests cleanup of aliases
func TestFormatRegistryAliasesCleanup(t *testing.T) {
	config := &RegistryConfig{
		MaxEntries:              2,  // Small limit
		TTL:                     0,  // Disable TTL
		EnableLRU:               true,
		EnableBackgroundCleanup: false,
	}

	registry := NewFormatRegistryWithConfig(config)
	defer registry.Close()

	// Register formats with aliases
	for i := 0; i < 4; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/test-%d", i),
			Name:        fmt.Sprintf("Test Format %d", i),
			Description: fmt.Sprintf("Test format %d", i),
			Aliases:     []string{fmt.Sprintf("test-%d", i), fmt.Sprintf("format-%d", i)},
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
		err := registry.RegisterFormat(info)
		require.NoError(t, err)
	}

	// Verify only MaxEntries formats are kept
	stats := registry.GetRegistryStats()
	assert.Equal(t, config.MaxEntries, stats["formats_count"])

	// Verify aliases were also limited/cleaned up
	assert.LessOrEqual(t, stats["aliases_count"], config.MaxEntries*2) // At most 2 aliases per format
}

// TestFormatRegistryAccessTimeCleanup tests cleanup based on last access time
func TestFormatRegistryAccessTimeCleanup(t *testing.T) {
	config := &RegistryConfig{
		MaxEntries:              100,
		TTL:                     0, // Disable TTL
		EnableLRU:               false,
		EnableBackgroundCleanup: false,
	}

	registry := NewFormatRegistryWithConfig(config)
	defer registry.Close()

	// Register some formats
	for i := 0; i < 3; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/test-%d", i),
			Name:        fmt.Sprintf("Test Format %d", i),
			Description: fmt.Sprintf("Test format %d", i),
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
		err := registry.RegisterFormat(info)
		require.NoError(t, err)
	}

	// Access some formats to update their access time
	time.Sleep(50 * time.Millisecond)
	_, _ = registry.GetFormat("application/test-1")

	// Wait a bit more
	time.Sleep(50 * time.Millisecond)

	// Clean up formats not accessed in the last 75ms
	cleaned, err := registry.CleanupByAccessTime(75 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 2, cleaned, "Formats 0 and 2 should be cleaned up")

	// Verify correct format remains
	stats := registry.GetRegistryStats()
	assert.Equal(t, 1, stats["formats_count"])

	// Format 1 should still be present (it was accessed recently)
	_, err = registry.GetFormat("application/test-1")
	assert.NoError(t, err)

	// Formats 0 and 2 should be gone
	_, err = registry.GetFormat("application/test-0")
	assert.Error(t, err)
	_, err = registry.GetFormat("application/test-2")
	assert.Error(t, err)
}

// TestFormatRegistryBackgroundCleanup tests background TTL cleanup
func TestFormatRegistryBackgroundCleanup(t *testing.T) {
	config := &RegistryConfig{
		MaxEntries:              100,
		TTL:                     100 * time.Millisecond, // Short TTL
		CleanupInterval:         50 * time.Millisecond,  // Frequent cleanup
		EnableLRU:               false,
		EnableBackgroundCleanup: true, // Enable background cleanup
	}

	registry := NewFormatRegistryWithConfig(config)
	defer registry.Close()

	// Register some formats
	for i := 0; i < 2; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/test-%d", i),
			Name:        fmt.Sprintf("Test Format %d", i),
			Description: fmt.Sprintf("Test format %d", i),
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
		err := registry.RegisterFormat(info)
		require.NoError(t, err)
	}

	// Verify formats are present initially
	stats := registry.GetRegistryStats()
	assert.Equal(t, 2, stats["formats_count"])

	// Wait for background cleanup to remove expired formats
	// Wait longer than TTL + cleanup interval
	time.Sleep(200 * time.Millisecond)

	// Verify formats have been cleaned up by background process
	afterStats := registry.GetRegistryStats()
	assert.Equal(t, 0, afterStats["formats_count"], "Background cleanup should have removed all expired formats")
}

// TestFormatRegistryLRUAccessPattern tests that LRU correctly tracks access patterns
func TestFormatRegistryLRUAccessPattern(t *testing.T) {
	config := &RegistryConfig{
		MaxEntries:              2,  // Small limit
		TTL:                     0,  // Disable TTL
		EnableLRU:               true,
		EnableBackgroundCleanup: false,
	}

	registry := NewFormatRegistryWithConfig(config)
	defer registry.Close()

	// Register 2 formats (at limit)
	for i := 0; i < 2; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/test-%d", i),
			Name:        fmt.Sprintf("Test Format %d", i),
			Description: fmt.Sprintf("Test format %d", i),
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
		err := registry.RegisterFormat(info)
		require.NoError(t, err)
	}

	// Access format-0 to make it most recently used
	_, err := registry.GetFormat("application/test-0")
	require.NoError(t, err)

	// Register a new format, should evict format-1 (least recently used)
	newInfo := &FormatInfo{
		MIMEType:    "application/test-2",
		Name:        "Test Format 2",
		Description: "New test format",
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
	err = registry.RegisterFormat(newInfo)
	require.NoError(t, err)

	// Verify format-0 and format-2 are present
	_, err = registry.GetFormat("application/test-0")
	assert.NoError(t, err, "format-0 should still be present (recently accessed)")
	_, err = registry.GetFormat("application/test-2")
	assert.NoError(t, err, "format-2 should be present (newly added)")

	// Verify format-1 was evicted
	_, err = registry.GetFormat("application/test-1")
	assert.Error(t, err, "format-1 should have been evicted (least recently used)")
}

// TestFormatRegistryClearAll tests clearing all entries
func TestFormatRegistryClearAll(t *testing.T) {
	registry := NewFormatRegistry()
	defer registry.Close()

	// Register some formats and aliases
	for i := 0; i < 3; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/test-%d", i),
			Name:        fmt.Sprintf("Test Format %d", i),
			Description: fmt.Sprintf("Test format %d", i),
			Aliases:     []string{fmt.Sprintf("test-%d", i)},
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
		err := registry.RegisterFormat(info)
		require.NoError(t, err)
	}

	// Verify entries are present
	stats := registry.GetRegistryStats()
	assert.Equal(t, 3, stats["formats_count"])
	assert.Equal(t, 3, stats["aliases_count"])

	// Clear all entries
	err := registry.ClearAll()
	require.NoError(t, err)

	// Verify all entries are gone
	afterStats := registry.GetRegistryStats()
	assert.Equal(t, 0, afterStats["formats_count"])
	assert.Equal(t, 0, afterStats["aliases_count"])
	assert.Equal(t, 0, afterStats["total_entries"])

	// Verify specific formats are gone
	for i := 0; i < 3; i++ {
		_, err := registry.GetFormat(fmt.Sprintf("application/test-%d", i))
		assert.Error(t, err)
	}
}

// TestFormatRegistryStats tests registry statistics
func TestFormatRegistryStats(t *testing.T) {
	config := &RegistryConfig{
		MaxEntries:              10,
		TTL:                     1 * time.Hour, // Long TTL
		CleanupInterval:         30 * time.Minute,
		EnableLRU:               true,
		EnableBackgroundCleanup: false,
	}

	registry := NewFormatRegistryWithConfig(config)
	defer registry.Close()

	// Register some formats with aliases
	for i := 0; i < 3; i++ {
		info := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/test-%d", i),
			Name:        fmt.Sprintf("Test Format %d", i),
			Description: fmt.Sprintf("Test format %d", i),
			Aliases:     []string{fmt.Sprintf("test-%d", i), fmt.Sprintf("format-%d", i)},
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
		err := registry.RegisterFormat(info)
		require.NoError(t, err)
	}

	// Access some formats to vary access counts
	_, _ = registry.GetFormat("application/test-0")
	_, _ = registry.GetFormat("application/test-0") // accessed twice total

	// Get registry stats
	stats := registry.GetRegistryStats()
	require.NotNil(t, stats)

	// Verify expected stats
	assert.Equal(t, 3, stats["formats_count"])
	assert.Equal(t, 6, stats["aliases_count"]) // 2 aliases per format
	assert.Equal(t, 9, stats["total_entries"])  // 3 formats + 6 aliases
	assert.Equal(t, config.MaxEntries, stats["max_entries_per_map"])
	assert.Equal(t, 3600.0, stats["ttl_seconds"]) // 1 hour in seconds
	assert.Equal(t, 1800.0, stats["cleanup_interval_seconds"]) // 30 minutes
	assert.True(t, stats["lru_enabled"].(bool))
	assert.False(t, stats["background_cleanup_enabled"].(bool))
}

// TestFormatRegistryConfigUpdate tests updating registry configuration
func TestFormatRegistryConfigUpdate(t *testing.T) {
	initialConfig := &RegistryConfig{
		MaxEntries:              10,
		TTL:                     1 * time.Hour,
		CleanupInterval:         30 * time.Minute,
		EnableLRU:               true,
		EnableBackgroundCleanup: false,
	}

	registry := NewFormatRegistryWithConfig(initialConfig)
	defer registry.Close()

	// Register a format
	info := &FormatInfo{
		MIMEType:    "application/test",
		Name:        "Test Format",
		Description: "Test format",
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
	err := registry.RegisterFormat(info)
	require.NoError(t, err)

	// Update configuration
	newConfig := &RegistryConfig{
		MaxEntries:              5,   // Smaller limit
		TTL:                     10 * time.Minute, // Shorter TTL
		CleanupInterval:         5 * time.Minute,  // More frequent cleanup
		EnableLRU:               true,
		EnableBackgroundCleanup: true, // Enable background cleanup
	}

	err = registry.UpdateConfig(newConfig)
	require.NoError(t, err)

	// Verify configuration was updated
	stats := registry.GetRegistryStats()
	assert.Equal(t, 5, stats["max_entries_per_map"])
	assert.Equal(t, 600.0, stats["ttl_seconds"])   // 10 minutes
	assert.Equal(t, 300.0, stats["cleanup_interval_seconds"]) // 5 minutes
	assert.True(t, stats["background_cleanup_enabled"].(bool))

	// Format should still be present
	retrievedFormat, err := registry.GetFormat("application/test")
	require.NoError(t, err)
	assert.Equal(t, "application/test", retrievedFormat.MIMEType)
}

// TestFormatRegistryAliasResolution tests alias resolution with cleanup
func TestFormatRegistryAliasResolution(t *testing.T) {
	config := &RegistryConfig{
		MaxEntries:              2,  // Small limit to test alias cleanup
		TTL:                     0,  // Disable TTL
		EnableLRU:               true,
		EnableBackgroundCleanup: false,
	}

	registry := NewFormatRegistryWithConfig(config)
	defer registry.Close()

	// Register format with aliases
	info := &FormatInfo{
		MIMEType:    "application/test",
		Name:        "Test Format",
		Description: "Test format",
		Aliases:     []string{"test", "testformat"},
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
	err := registry.RegisterFormat(info)
	require.NoError(t, err)

	// Test alias resolution
	format1, err := registry.GetFormat("application/test")
	require.NoError(t, err)
	format2, err := registry.GetFormat("test")
	require.NoError(t, err)
	format3, err := registry.GetFormat("testformat")
	require.NoError(t, err)

	// All should resolve to the same format
	assert.Equal(t, format1.MIMEType, format2.MIMEType)
	assert.Equal(t, format2.MIMEType, format3.MIMEType)
	assert.Equal(t, "application/test", format1.MIMEType)

	// Register more aliases to trigger cleanup
	for i := 0; i < 5; i++ {
		moreInfo := &FormatInfo{
			MIMEType:    fmt.Sprintf("application/more-%d", i),
			Name:        fmt.Sprintf("More Format %d", i),
			Description: fmt.Sprintf("More format %d", i),
			Aliases:     []string{fmt.Sprintf("more-%d", i)},
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
		_ = registry.RegisterFormat(moreInfo) // May fail due to limits, that's OK
	}

	// Verify registry respects limits
	stats := registry.GetRegistryStats()
	assert.LessOrEqual(t, stats["formats_count"].(int), config.MaxEntries)
	assert.LessOrEqual(t, stats["aliases_count"].(int), config.MaxEntries*2) // Rough estimate
}