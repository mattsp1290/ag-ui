// Package config provides comprehensive resource management and security limits
// for the configuration system to prevent DoS attacks and resource exhaustion.
package config

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ResourceLimits defines configurable limits to prevent resource exhaustion attacks
type ResourceLimits struct {
	// File size limits
	MaxFileSize          int64         `json:"max_file_size" yaml:"max_file_size"`                   // Maximum file size in bytes (default: 10MB)
	MaxMemoryUsage       int64         `json:"max_memory_usage" yaml:"max_memory_usage"`             // Maximum memory usage in bytes (default: 50MB)
	
	// Configuration structure limits
	MaxNestingDepth      int           `json:"max_nesting_depth" yaml:"max_nesting_depth"`           // Maximum nesting depth (default: 20)
	MaxKeys              int           `json:"max_keys" yaml:"max_keys"`                             // Maximum number of keys (default: 10000)
	MaxArraySize         int           `json:"max_array_size" yaml:"max_array_size"`                 // Maximum array size (default: 1000)
	MaxStringLength      int           `json:"max_string_length" yaml:"max_string_length"`           // Maximum string length (default: 64KB)
	
	// Watcher limits
	MaxWatchers          int           `json:"max_watchers" yaml:"max_watchers"`                     // Maximum watchers per instance (default: 100)
	MaxWatchersPerKey    int           `json:"max_watchers_per_key" yaml:"max_watchers_per_key"`     // Maximum watchers per key (default: 10)
	
	// Rate limiting
	ReloadRateLimit      time.Duration `json:"reload_rate_limit" yaml:"reload_rate_limit"`           // Minimum time between reloads (default: 1s)
	UpdateRateLimit      time.Duration `json:"update_rate_limit" yaml:"update_rate_limit"`           // Minimum time between updates (default: 100ms)
	ValidationRateLimit  time.Duration `json:"validation_rate_limit" yaml:"validation_rate_limit"`   // Minimum time between validations (default: 500ms)
	
	// Processing timeouts
	LoadTimeout          time.Duration `json:"load_timeout" yaml:"load_timeout"`                     // Maximum time to load configuration (default: 30s)
	ValidationTimeout    time.Duration `json:"validation_timeout" yaml:"validation_timeout"`         // Maximum time for validation (default: 10s)
	WatcherTimeout       time.Duration `json:"watcher_timeout" yaml:"watcher_timeout"`               // Maximum time for watcher callbacks (default: 5s)
}

// DefaultResourceLimits returns sensible default resource limits
func DefaultResourceLimits() *ResourceLimits {
	return &ResourceLimits{
		// File and memory limits
		MaxFileSize:         10 * 1024 * 1024,  // 10MB
		MaxMemoryUsage:      50 * 1024 * 1024,  // 50MB
		
		// Structure limits
		MaxNestingDepth:     20,
		MaxKeys:             10000,
		MaxArraySize:        1000,
		MaxStringLength:     64 * 1024,          // 64KB
		
		// Watcher limits
		MaxWatchers:         100,
		MaxWatchersPerKey:   10,
		
		// Rate limits
		ReloadRateLimit:     1 * time.Second,
		UpdateRateLimit:     100 * time.Millisecond,
		ValidationRateLimit: 500 * time.Millisecond,
		
		// Timeouts
		LoadTimeout:         30 * time.Second,
		ValidationTimeout:   10 * time.Second,
		WatcherTimeout:      5 * time.Second,
	}
}

// ResourceManager manages resource usage and enforces limits
type ResourceManager struct {
	limits   *ResourceLimits
	stats    *ResourceStats
	mu       sync.RWMutex
	
	// Rate limiting state
	lastReload     time.Time
	lastUpdate     time.Time
	lastValidation time.Time
	rateMu         sync.Mutex
}

// ResourceStats tracks current resource usage
type ResourceStats struct {
	// Current usage counters (atomic for thread safety)
	CurrentMemoryUsage  int64 `json:"current_memory_usage"`
	CurrentKeys         int64 `json:"current_keys"`
	CurrentWatchers     int64 `json:"current_watchers"`
	
	// Tracking by key
	WatchersByKey map[string]int `json:"watchers_by_key"`
	watchersMu    sync.RWMutex
	
	// Rate limiting stats
	ReloadAttempts      int64 `json:"reload_attempts"`
	ReloadBlocked       int64 `json:"reload_blocked"`
	UpdateAttempts      int64 `json:"update_attempts"`
	UpdateBlocked       int64 `json:"update_blocked"`
	ValidationAttempts  int64 `json:"validation_attempts"`
	ValidationBlocked   int64 `json:"validation_blocked"`
	
	// Error counts
	LimitExceeded       int64 `json:"limit_exceeded"`
	TimeoutExceeded     int64 `json:"timeout_exceeded"`
	StructureViolations int64 `json:"structure_violations"`
	
	// Performance metrics
	AverageLoadTime     time.Duration `json:"average_load_time"`
	AverageValidateTime time.Duration `json:"average_validate_time"`
	PeakMemoryUsage     int64         `json:"peak_memory_usage"`
}

// NewResourceManager creates a new resource manager with the given limits
func NewResourceManager(limits *ResourceLimits) *ResourceManager {
	if limits == nil {
		limits = DefaultResourceLimits()
	}
	
	return &ResourceManager{
		limits: limits,
		stats: &ResourceStats{
			WatchersByKey: make(map[string]int),
		},
	}
}

// ValidateFileSize checks if a file size is within limits
func (rm *ResourceManager) ValidateFileSize(size int64) error {
	if size > rm.limits.MaxFileSize {
		atomic.AddInt64(&rm.stats.LimitExceeded, 1)
		return NewResourceLimitError("file_size", size, rm.limits.MaxFileSize, 
			fmt.Sprintf("file size %d bytes exceeds maximum allowed %d bytes", size, rm.limits.MaxFileSize))
	}
	return nil
}

// ValidateMemoryUsage checks if memory usage is within limits  
func (rm *ResourceManager) ValidateMemoryUsage(usage int64) error {
	current := atomic.LoadInt64(&rm.stats.CurrentMemoryUsage)
	if current+usage > rm.limits.MaxMemoryUsage {
		atomic.AddInt64(&rm.stats.LimitExceeded, 1)
		return NewResourceLimitError("memory_usage", current+usage, rm.limits.MaxMemoryUsage,
			fmt.Sprintf("memory usage %d bytes would exceed maximum allowed %d bytes", current+usage, rm.limits.MaxMemoryUsage))
	}
	return nil
}

// ValidateConfigStructure checks if configuration structure is within limits
func (rm *ResourceManager) ValidateConfigStructure(config map[string]interface{}) error {
	validator := &structureValidator{
		limits: rm.limits,
		stats:  rm.stats,
	}
	
	if err := validator.validateStructure(config, 0); err != nil {
		atomic.AddInt64(&rm.stats.StructureViolations, 1)
		return err
	}
	
	return nil
}

// CanAddWatcher checks if a new watcher can be added
func (rm *ResourceManager) CanAddWatcher(key string) error {
	current := atomic.LoadInt64(&rm.stats.CurrentWatchers)
	
	// Check total watcher limit
	if current >= int64(rm.limits.MaxWatchers) {
		atomic.AddInt64(&rm.stats.LimitExceeded, 1)
		return NewResourceLimitError("total_watchers", current+1, int64(rm.limits.MaxWatchers),
			fmt.Sprintf("cannot add watcher: total watchers %d would exceed maximum %d", current+1, rm.limits.MaxWatchers))
	}
	
	// Check per-key watcher limit
	rm.stats.watchersMu.RLock()
	keyCount := rm.stats.WatchersByKey[key]
	rm.stats.watchersMu.RUnlock()
	
	if keyCount >= rm.limits.MaxWatchersPerKey {
		atomic.AddInt64(&rm.stats.LimitExceeded, 1)
		return NewResourceLimitError("key_watchers", int64(keyCount+1), int64(rm.limits.MaxWatchersPerKey),
			fmt.Sprintf("cannot add watcher for key '%s': key watchers %d would exceed maximum %d", key, keyCount+1, rm.limits.MaxWatchersPerKey))
	}
	
	return nil
}

// AddWatcher registers a new watcher
func (rm *ResourceManager) AddWatcher(key string) {
	atomic.AddInt64(&rm.stats.CurrentWatchers, 1)
	
	rm.stats.watchersMu.Lock()
	rm.stats.WatchersByKey[key]++
	rm.stats.watchersMu.Unlock()
}

// RemoveWatcher unregisters a watcher
func (rm *ResourceManager) RemoveWatcher(key string) {
	if current := atomic.LoadInt64(&rm.stats.CurrentWatchers); current > 0 {
		atomic.AddInt64(&rm.stats.CurrentWatchers, -1)
	}
	
	rm.stats.watchersMu.Lock()
	if rm.stats.WatchersByKey[key] > 0 {
		rm.stats.WatchersByKey[key]--
		if rm.stats.WatchersByKey[key] == 0 {
			delete(rm.stats.WatchersByKey, key)
		}
	}
	rm.stats.watchersMu.Unlock()
}

// CanReload checks if a reload operation is allowed based on rate limits
func (rm *ResourceManager) CanReload() error {
	rm.rateMu.Lock()
	defer rm.rateMu.Unlock()
	
	atomic.AddInt64(&rm.stats.ReloadAttempts, 1)
	
	if time.Since(rm.lastReload) < rm.limits.ReloadRateLimit {
		atomic.AddInt64(&rm.stats.ReloadBlocked, 1)
		return NewRateLimitError("reload", rm.limits.ReloadRateLimit, time.Since(rm.lastReload),
			fmt.Sprintf("reload rate limit exceeded: minimum interval %v, time since last reload %v", 
				rm.limits.ReloadRateLimit, time.Since(rm.lastReload)))
	}
	
	rm.lastReload = time.Now()
	return nil
}

// CanUpdate checks if an update operation is allowed based on rate limits
func (rm *ResourceManager) CanUpdate() error {
	rm.rateMu.Lock()
	defer rm.rateMu.Unlock()
	
	atomic.AddInt64(&rm.stats.UpdateAttempts, 1)
	
	if time.Since(rm.lastUpdate) < rm.limits.UpdateRateLimit {
		atomic.AddInt64(&rm.stats.UpdateBlocked, 1)
		return NewRateLimitError("update", rm.limits.UpdateRateLimit, time.Since(rm.lastUpdate),
			fmt.Sprintf("update rate limit exceeded: minimum interval %v, time since last update %v", 
				rm.limits.UpdateRateLimit, time.Since(rm.lastUpdate)))
	}
	
	rm.lastUpdate = time.Now()
	return nil
}

// CanValidate checks if a validation operation is allowed based on rate limits
func (rm *ResourceManager) CanValidate() error {
	rm.rateMu.Lock()
	defer rm.rateMu.Unlock()
	
	atomic.AddInt64(&rm.stats.ValidationAttempts, 1)
	
	if time.Since(rm.lastValidation) < rm.limits.ValidationRateLimit {
		atomic.AddInt64(&rm.stats.ValidationBlocked, 1)
		return NewRateLimitError("validation", rm.limits.ValidationRateLimit, time.Since(rm.lastValidation),
			fmt.Sprintf("validation rate limit exceeded: minimum interval %v, time since last validation %v", 
				rm.limits.ValidationRateLimit, time.Since(rm.lastValidation)))
	}
	
	rm.lastValidation = time.Now()
	return nil
}

// UpdateMemoryUsage updates the current memory usage
func (rm *ResourceManager) UpdateMemoryUsage(delta int64) {
	newUsage := atomic.AddInt64(&rm.stats.CurrentMemoryUsage, delta)
	
	// Update peak memory usage if necessary
	for {
		peak := atomic.LoadInt64(&rm.stats.PeakMemoryUsage)
		if newUsage <= peak {
			break
		}
		if atomic.CompareAndSwapInt64(&rm.stats.PeakMemoryUsage, peak, newUsage) {
			break
		}
	}
}

// UpdateKeyCount updates the current key count
func (rm *ResourceManager) UpdateKeyCount(delta int64) {
	atomic.AddInt64(&rm.stats.CurrentKeys, delta)
}

// GetStats returns a copy of the current resource statistics
func (rm *ResourceManager) GetStats() ResourceStats {
	rm.stats.watchersMu.RLock()
	watchersByKey := make(map[string]int)
	for k, v := range rm.stats.WatchersByKey {
		watchersByKey[k] = v
	}
	rm.stats.watchersMu.RUnlock()
	
	return ResourceStats{
		CurrentMemoryUsage:  atomic.LoadInt64(&rm.stats.CurrentMemoryUsage),
		CurrentKeys:         atomic.LoadInt64(&rm.stats.CurrentKeys),
		CurrentWatchers:     atomic.LoadInt64(&rm.stats.CurrentWatchers),
		WatchersByKey:       watchersByKey,
		ReloadAttempts:      atomic.LoadInt64(&rm.stats.ReloadAttempts),
		ReloadBlocked:       atomic.LoadInt64(&rm.stats.ReloadBlocked),
		UpdateAttempts:      atomic.LoadInt64(&rm.stats.UpdateAttempts),
		UpdateBlocked:       atomic.LoadInt64(&rm.stats.UpdateBlocked),
		ValidationAttempts:  atomic.LoadInt64(&rm.stats.ValidationAttempts),
		ValidationBlocked:   atomic.LoadInt64(&rm.stats.ValidationBlocked),
		LimitExceeded:       atomic.LoadInt64(&rm.stats.LimitExceeded),
		TimeoutExceeded:     atomic.LoadInt64(&rm.stats.TimeoutExceeded),
		StructureViolations: atomic.LoadInt64(&rm.stats.StructureViolations),
		AverageLoadTime:     rm.stats.AverageLoadTime,
		AverageValidateTime: rm.stats.AverageValidateTime,
		PeakMemoryUsage:     atomic.LoadInt64(&rm.stats.PeakMemoryUsage),
	}
}

// GetLimits returns a copy of the current resource limits
func (rm *ResourceManager) GetLimits() ResourceLimits {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	return *rm.limits
}

// UpdateLimits updates the resource limits (for runtime configuration)
func (rm *ResourceManager) UpdateLimits(newLimits *ResourceLimits) error {
	if newLimits == nil {
		return fmt.Errorf("resource limits cannot be nil")
	}
	
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	// Validate that new limits are reasonable
	if err := rm.validateLimits(newLimits); err != nil {
		return fmt.Errorf("invalid resource limits: %w", err)
	}
	
	rm.limits = newLimits
	return nil
}

// validateLimits validates that resource limits are reasonable
func (rm *ResourceManager) validateLimits(limits *ResourceLimits) error {
	if limits.MaxFileSize <= 0 {
		return fmt.Errorf("max file size must be positive")
	}
	if limits.MaxMemoryUsage <= 0 {
		return fmt.Errorf("max memory usage must be positive")
	}
	if limits.MaxNestingDepth <= 0 {
		return fmt.Errorf("max nesting depth must be positive")
	}
	if limits.MaxKeys <= 0 {
		return fmt.Errorf("max keys must be positive")
	}
	if limits.MaxWatchers <= 0 {
		return fmt.Errorf("max watchers must be positive")
	}
	if limits.MaxWatchersPerKey <= 0 {
		return fmt.Errorf("max watchers per key must be positive")
	}
	if limits.ReloadRateLimit <= 0 {
		return fmt.Errorf("reload rate limit must be positive")
	}
	if limits.UpdateRateLimit <= 0 {
		return fmt.Errorf("update rate limit must be positive")
	}
	if limits.ValidationRateLimit <= 0 {
		return fmt.Errorf("validation rate limit must be positive")
	}
	
	return nil
}

// WithTimeout creates a context with the appropriate timeout for the operation type
func (rm *ResourceManager) WithTimeout(ctx context.Context, operation string) (context.Context, context.CancelFunc) {
	var timeout time.Duration
	
	switch operation {
	case "load":
		timeout = rm.limits.LoadTimeout
	case "validate":
		timeout = rm.limits.ValidationTimeout
	case "watcher":
		timeout = rm.limits.WatcherTimeout
	default:
		timeout = rm.limits.LoadTimeout // Default timeout
	}
	
	return context.WithTimeout(ctx, timeout)
}

// structureValidator validates configuration structure against limits
type structureValidator struct {
	limits      *ResourceLimits
	stats       *ResourceStats
	keyCount    int
	arrayCount  int
}

// validateStructure recursively validates configuration structure
func (sv *structureValidator) validateStructure(data interface{}, depth int) error {
	// Check nesting depth
	if depth > sv.limits.MaxNestingDepth {
		return NewStructureLimitError("nesting_depth", depth, sv.limits.MaxNestingDepth,
			fmt.Sprintf("nesting depth %d exceeds maximum allowed %d", depth, sv.limits.MaxNestingDepth))
	}
	
	switch v := data.(type) {
	case map[string]interface{}:
		return sv.validateObject(v, depth)
	case []interface{}:
		return sv.validateArray(v, depth)
	case string:
		return sv.validateString(v)
	default:
		return nil
	}
}

// validateObject validates an object (map)
func (sv *structureValidator) validateObject(obj map[string]interface{}, depth int) error {
	sv.keyCount += len(obj)
	
	// Check total key count
	if sv.keyCount > sv.limits.MaxKeys {
		return NewStructureLimitError("key_count", sv.keyCount, sv.limits.MaxKeys,
			fmt.Sprintf("key count %d exceeds maximum allowed %d", sv.keyCount, sv.limits.MaxKeys))
	}
	
	// Recursively validate nested structures
	for key, value := range obj {
		// Validate key length
		if len(key) > sv.limits.MaxStringLength {
			return NewStructureLimitError("string_length", len(key), sv.limits.MaxStringLength,
				fmt.Sprintf("key length %d exceeds maximum allowed %d", len(key), sv.limits.MaxStringLength))
		}
		
		// Recursively validate value
		if err := sv.validateStructure(value, depth+1); err != nil {
			return err
		}
	}
	
	return nil
}

// validateArray validates an array
func (sv *structureValidator) validateArray(arr []interface{}, depth int) error {
	sv.arrayCount++
	
	// Check array size
	if len(arr) > sv.limits.MaxArraySize {
		return NewStructureLimitError("array_size", len(arr), sv.limits.MaxArraySize,
			fmt.Sprintf("array size %d exceeds maximum allowed %d", len(arr), sv.limits.MaxArraySize))
	}
	
	// Recursively validate array elements
	for _, item := range arr {
		if err := sv.validateStructure(item, depth+1); err != nil {
			return err
		}
	}
	
	return nil
}

// validateString validates a string
func (sv *structureValidator) validateString(str string) error {
	if len(str) > sv.limits.MaxStringLength {
		return NewStructureLimitError("string_length", len(str), sv.limits.MaxStringLength,
			fmt.Sprintf("string length %d exceeds maximum allowed %d", len(str), sv.limits.MaxStringLength))
	}
	return nil
}