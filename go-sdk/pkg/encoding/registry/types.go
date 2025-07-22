package registry

import (
	"fmt"
	"time"
)

// RegistryEntry wraps registry data with metadata for cleanup
type RegistryEntry struct {
	Value       interface{}
	CreatedAt   time.Time
	LastAccess  time.Time
	AccessCount int64
}

// RegistryConfig holds configuration for registry cleanup behavior
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
}

// DefaultRegistryConfig returns sensible defaults for registry cleanup
func DefaultRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		MaxEntries:              1000,                // Limit to 1000 entries per map
		TTL:                     1 * time.Hour,       // 1 hour TTL
		CleanupInterval:         10 * time.Minute,    // Cleanup every 10 minutes
		EnableLRU:               true,
		EnableBackgroundCleanup: true,
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