package cache

import (
	"context"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// CacheStats provides detailed cache statistics
type CacheStats struct {
	L1Hits          uint64
	L1Misses        uint64
	L2Hits          uint64
	L2Misses        uint64
	TotalHits       uint64
	TotalMisses     uint64
	Evictions       uint64
	Expirations     uint64
	CompressionRate float64
	AvgHitLatency   time.Duration
	AvgMissLatency  time.Duration
}

// ValidationCacheKey represents a cache key for validation results
type ValidationCacheKey struct {
	EventType   events.EventType
	EventHash   string
	ConfigHash  string
	ValidatorID string
}

// ValidationCacheEntry represents a cached validation result
type ValidationCacheEntry struct {
	Key              ValidationCacheKey
	Valid            bool
	Errors           []error
	Metadata         map[string]interface{}
	CreatedAt        time.Time
	ExpiresAt        time.Time
	AccessCount      uint64
	LastAccessedAt   time.Time
	CompressionRatio float64
}

// NodeInfo represents information about a cache node
type NodeInfo struct {
	ID            string
	Address       string
	State         NodeState
	LastHeartbeat time.Time
	Metrics       CacheStats
	Shards        []int
}

// NodeState represents the state of a node
type NodeState int

const (
	NodeStateActive NodeState = iota
	NodeStateInactive
	NodeStateSuspect
	NodeStateFailed
)

// CacheValidatorInterface defines the interface for cache invalidation coordination
type CacheValidatorInterface interface {
	InvalidateByKeys(ctx context.Context, keys []string) error
	InvalidateEventType(ctx context.Context, eventType string) error
}

// BasicCache defines the fundamental cache operations
// Following Interface Segregation Principle - only basic read/write operations
type BasicCache interface {
	// Get retrieves a value from the cache
	Get(ctx context.Context, key string) ([]byte, error)
	
	// Set stores a value in the cache with TTL
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	
	// Delete removes a value from the cache
	Delete(ctx context.Context, key string) error
}

// AdvancedCache extends BasicCache with advanced operations
// Separated from BasicCache to follow Interface Segregation Principle
type AdvancedCache interface {
	BasicCache
	
	// Exists checks if a key exists in the cache
	Exists(ctx context.Context, key string) (bool, error)
	
	// TTL returns the time-to-live for a key
	TTL(ctx context.Context, key string) (time.Duration, error)
}

// DistributedCache extends AdvancedCache with distributed operations
// Highest level interface for full distributed cache functionality
type DistributedCache interface {
	AdvancedCache
	
	// Scan searches for keys matching a pattern (distributed operation)
	Scan(ctx context.Context, pattern string) ([]string, error)
}

// CacheMetrics provides metrics collection interface
// Separate interface for metrics to follow single responsibility
type CacheMetrics interface {
	// RecordHit records a cache hit
	RecordHit(level string)
	
	// RecordMiss records a cache miss
	RecordMiss()
	
	// RecordEviction records a cache eviction
	RecordEviction()
	
	// GetStats returns current cache statistics
	GetStats() CacheStats
}

// CacheInvalidator handles cache invalidation operations
// Separate interface for invalidation to follow single responsibility
type CacheInvalidator interface {
	// InvalidateKey invalidates a specific cache key
	InvalidateKey(ctx context.Context, key string) error
	
	// InvalidatePattern invalidates keys matching a pattern
	InvalidatePattern(ctx context.Context, pattern string) error
	
	// InvalidateAll invalidates all cache entries
	InvalidateAll(ctx context.Context) error
}

// CacheWarmup handles cache warming operations
// Separate interface for warmup operations
type CacheWarmup interface {
	// Warmup pre-populates the cache with frequently accessed data
	Warmup(ctx context.Context, keys []string) error
	
	// PrefetchKey prefetches a single key
	PrefetchKey(ctx context.Context, key string) error
}

// L1CacheInterface represents the in-memory cache layer interface
type L1CacheInterface interface {
	BasicCache
	CacheMetrics
	
	// SetEvictionCallback sets a callback for when items are evicted
	SetEvictionCallback(callback func(key string, value interface{}))
	
	// Size returns the current number of items in the cache
	Size() int
	
	// Clear removes all items from the cache
	Clear()
}

// L2CacheInterface represents the distributed cache layer interface
type L2CacheInterface interface {
	DistributedCache
	CacheMetrics
	CacheInvalidator
	
	// GetNodes returns the list of cache nodes
	GetNodes(ctx context.Context) ([]string, error)
	
	// GetShardInfo returns sharding information for a key
	GetShardInfo(ctx context.Context, key string) (ShardInfo, error)
}

// ShardInfo contains information about cache sharding
type ShardInfo struct {
	ShardID   int      `json:"shard_id"`
	NodeID    string   `json:"node_id"`
	Replicas  []string `json:"replicas"`
	IsHealthy bool     `json:"is_healthy"`
}

// CacheFactory creates cache instances
// Factory pattern to abstract cache creation
type CacheFactory interface {
	// CreateL1Cache creates an L1 (in-memory) cache
	CreateL1Cache(config L1CacheConfig) (L1CacheInterface, error)
	
	// CreateL2Cache creates an L2 (distributed) cache
	CreateL2Cache(config L2CacheConfig) (L2CacheInterface, error)
	
	// CreateMultiLevelCache creates a multi-level cache
	CreateMultiLevelCache(l1Config L1CacheConfig, l2Config L2CacheConfig) (MultiLevelCache, error)
}

// L1CacheConfig configuration for L1 cache
type L1CacheConfig struct {
	MaxSize           int           `json:"max_size"`
	TTL               time.Duration `json:"ttl"`
	EvictionPolicy    string        `json:"eviction_policy"` // LRU, LFU, etc.
	EnableMetrics     bool          `json:"enable_metrics"`
	CleanupInterval   time.Duration `json:"cleanup_interval"`
}

// L2CacheConfig configuration for L2 cache
type L2CacheConfig struct {
	Endpoints         []string      `json:"endpoints"`
	ConnectTimeout    time.Duration `json:"connect_timeout"`
	RequestTimeout    time.Duration `json:"request_timeout"`
	MaxRetries        int           `json:"max_retries"`
	EnableCompression bool          `json:"enable_compression"`
	ShardCount        int           `json:"shard_count"`
	EnableMetrics     bool          `json:"enable_metrics"`
}

// MultiLevelCache provides a multi-level cache interface
type MultiLevelCache interface {
	BasicCache
	CacheMetrics
	CacheInvalidator
	CacheWarmup
	
	// GetL1 returns the L1 cache instance
	GetL1() L1CacheInterface
	
	// GetL2 returns the L2 cache instance
	GetL2() L2CacheInterface
	
	// PromoteToL1 promotes a key from L2 to L1
	PromoteToL1(ctx context.Context, key string) error
	
	// SetConsistencyMode sets the consistency mode for multi-level operations
	SetConsistencyMode(mode ConsistencyMode)
}

// ConsistencyMode defines cache consistency levels
type ConsistencyMode int

const (
	// EventualConsistency allows temporary inconsistency between levels
	EventualConsistency ConsistencyMode = iota
	// StrongConsistency ensures all levels are consistent
	StrongConsistency
	// ReadThrough reads from L2 if not in L1
	ReadThrough
	// WriteThrough writes to both L1 and L2
	WriteThrough
	// WriteBack writes to L1 immediately, L2 asynchronously
	WriteBack
)

// String returns the string representation of the consistency mode
func (c ConsistencyMode) String() string {
	switch c {
	case EventualConsistency:
		return "eventual"
	case StrongConsistency:
		return "strong"
	case ReadThrough:
		return "read_through"
	case WriteThrough:
		return "write_through"
	case WriteBack:
		return "write_back"
	default:
		return "unknown"
	}
}