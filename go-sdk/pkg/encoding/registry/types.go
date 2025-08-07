package registry

import (
	"fmt"
	"sync/atomic"
	"time"
)

// RegistryEntry wraps registry data with metadata for cleanup
type RegistryEntry struct {
	Value       interface{}
	CreatedAt   time.Time
	lastAccess  int64 // atomic: Unix nano timestamp
	AccessCount int64 // atomic
}

// GetLastAccess atomically gets the last access time
func (e *RegistryEntry) GetLastAccess() time.Time {
	nanos := atomic.LoadInt64(&e.lastAccess)
	return time.Unix(0, nanos)
}

// SetLastAccess atomically sets the last access time
func (e *RegistryEntry) SetLastAccess(t time.Time) {
	atomic.StoreInt64(&e.lastAccess, t.UnixNano())
}

// RegistryConfig holds configuration for registry cleanup behavior with enhanced leak prevention
type RegistryConfig struct {
	// MaxEntries limits the total number of entries per map (0 = unlimited)
	MaxEntries int
	// TTL is the time-to-live for entries (0 = no TTL)
	TTL time.Duration
	// CleanupInterval is how often to run background cleanup
	CleanupInterval time.Duration
	// EnableLRU enables LRU eviction when max entries is reached
	EnableLRU bool
	// EnableBackgroundCleanup enables automatic TTL-based cleanup
	EnableBackgroundCleanup bool
	// MemoryPressureThreshold percentage (0-100) at which to trigger pressure cleanup
	MemoryPressureThreshold int
	// BatchEvictionSize number of entries to evict in a single batch operation
	BatchEvictionSize int
	// PreventativeCleanupInterval how often to run preventative cleanup
	PreventativeCleanupInterval time.Duration
	// EnableMemoryPressureLogging enables detailed logging of memory pressure events
	EnableMemoryPressureLogging bool
	// MaxMemoryPressureLevel maximum memory pressure level to handle (1-3)
	MaxMemoryPressureLevel int
}

// DefaultRegistryConfig returns sensible defaults for registry cleanup with leak prevention
func DefaultRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		MaxEntries:                      1000,             // Limit to 1000 entries per map
		TTL:                             1 * time.Hour,    // 1 hour TTL
		CleanupInterval:                 10 * time.Minute, // Cleanup every 10 minutes
		EnableLRU:                       true,
		EnableBackgroundCleanup:         true,
		MemoryPressureThreshold:         80,               // Trigger cleanup at 80% capacity
		BatchEvictionSize:               50,               // Evict 50 entries per batch
		PreventativeCleanupInterval:     5 * time.Minute,  // Preventative cleanup every 5 minutes
		EnableMemoryPressureLogging:     true,             // Enable detailed logging
		MaxMemoryPressureLevel:          3,                // Handle up to level 3 pressure
	}
}

// ConservativeRegistryConfig returns a more conservative configuration for memory-constrained environments
func ConservativeRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		MaxEntries:                      500,              // Lower limit for constrained environments
		TTL:                             30 * time.Minute, // Shorter TTL
		CleanupInterval:                 5 * time.Minute,  // More frequent cleanup
		EnableLRU:                       true,
		EnableBackgroundCleanup:         true,
		MemoryPressureThreshold:         70,               // More aggressive threshold
		BatchEvictionSize:               25,               // Smaller batches
		PreventativeCleanupInterval:     2 * time.Minute,  // Very frequent preventative cleanup
		EnableMemoryPressureLogging:     true,
		MaxMemoryPressureLevel:          3,
	}
}

// AggressiveRegistryConfig returns a configuration for high-throughput environments
func AggressiveRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		MaxEntries:                      2000,             // Higher limit for high-throughput
		TTL:                             2 * time.Hour,    // Longer TTL
		CleanupInterval:                 15 * time.Minute, // Less frequent cleanup
		EnableLRU:                       true,
		EnableBackgroundCleanup:         true,
		MemoryPressureThreshold:         90,               // Higher threshold
		BatchEvictionSize:               100,              // Larger batches
		PreventativeCleanupInterval:     10 * time.Minute, // Less frequent preventative cleanup
		EnableMemoryPressureLogging:     false,            // Disable verbose logging for performance
		MaxMemoryPressureLevel:          3,
	}
}

// RegistryEntryType represents the type of registry entry for composite keys
type RegistryEntryType int

const (
	EntryTypeFormat RegistryEntryType = iota
	EntryTypeEncoderFactory
	EntryTypeDecoderFactory
	EntryTypeCodecFactory
	EntryTypeLegacyEncoderFactory
	EntryTypeLegacyDecoderFactory
	EntryTypeLegacyCodecFactory
	EntryTypeAlias
)

// RegistryKey represents a composite key for the sync.Map
type RegistryKey struct {
	EntryType RegistryEntryType
	MimeType  string
}

// String returns a string representation of the key for debugging
func (k RegistryKey) String() string {
	return fmt.Sprintf("%d:%s", k.EntryType, k.MimeType)
}
