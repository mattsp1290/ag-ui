package config

import (
	"sync"
	"time"
)

// CachedValidator wraps an existing validator with caching capabilities
type CachedValidator struct {
	validator Validator
	cache     *ValidationCache
	mu        sync.RWMutex
	enabled   bool
}

// NewCachedValidator creates a new cached validator wrapper
func NewCachedValidator(validator Validator, cacheConfig *ValidationCacheConfig) *CachedValidator {
	if cacheConfig == nil {
		cacheConfig = DefaultValidationCacheConfig()
	}

	return &CachedValidator{
		validator: validator,
		cache:     NewValidationCache(cacheConfig),
		enabled:   cacheConfig.Enabled,
	}
}

// NewCachedValidatorWithCache creates a cached validator with an existing cache
func NewCachedValidatorWithCache(validator Validator, cache *ValidationCache) *CachedValidator {
	return &CachedValidator{
		validator: validator,
		cache:     cache,
		enabled:   cache.config.Enabled,
	}
}

// Name returns the name of the underlying validator with cache indicator
func (cv *CachedValidator) Name() string {
	if cv.enabled {
		return cv.validator.Name() + " (cached)"
	}
	return cv.validator.Name()
}

// Validate validates the configuration with caching
func (cv *CachedValidator) Validate(config map[string]interface{}) error {
	if !cv.enabled {
		return cv.validator.Validate(config)
	}

	// Try to get cached result first
	if result, found := cv.cache.Get(cv.validator.Name(), config); found {
		return result
	}

	// Cache miss - perform actual validation
	result := cv.validator.Validate(config)

	// Store result in cache
	cv.cache.Put(cv.validator.Name(), config, result)

	return result
}

// ValidateField validates a specific field with caching
// Note: Field validation is typically fast, so we may want to make this configurable
func (cv *CachedValidator) ValidateField(key string, value interface{}) error {
	if !cv.enabled {
		return cv.validator.ValidateField(key, value)
	}

	// For field validation, we create a minimal config with just this field
	fieldConfig := map[string]interface{}{key: value}
	fieldValidatorName := cv.validator.Name() + ":field:" + key

	// Try to get cached result
	if result, found := cv.cache.Get(fieldValidatorName, fieldConfig); found {
		return result
	}

	// Cache miss - perform actual validation
	result := cv.validator.ValidateField(key, value)

	// Store result in cache
	cv.cache.Put(fieldValidatorName, fieldConfig, result)

	return result
}

// GetSchema returns the validation schema from the underlying validator
func (cv *CachedValidator) GetSchema() map[string]interface{} {
	return cv.validator.GetSchema()
}

// GetCache returns the validation cache (useful for metrics and management)
func (cv *CachedValidator) GetCache() *ValidationCache {
	return cv.cache
}

// GetMetrics returns cache performance metrics
func (cv *CachedValidator) GetMetrics() ValidationCacheMetrics {
	return cv.cache.GetMetrics()
}

// GetHitRatio returns the cache hit ratio as a percentage
func (cv *CachedValidator) GetHitRatio() float64 {
	return cv.cache.GetHitRatio()
}

// InvalidateCache clears all cached validation results
func (cv *CachedValidator) InvalidateCache() {
	cv.cache.Invalidate()
}

// EnableCache enables caching for this validator
func (cv *CachedValidator) EnableCache() {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	cv.enabled = true
	cv.cache.config.Enabled = true
}

// DisableCache disables caching for this validator
func (cv *CachedValidator) DisableCache() {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	cv.enabled = false
	cv.cache.config.Enabled = false
}

// IsCacheEnabled returns whether caching is currently enabled
func (cv *CachedValidator) IsCacheEnabled() bool {
	cv.mu.RLock()
	defer cv.mu.RUnlock()
	return cv.enabled
}

// SetCacheMaxSize updates the maximum cache size
func (cv *CachedValidator) SetCacheMaxSize(maxSize int) {
	cv.cache.SetMaxSize(maxSize)
}

// SetCacheTTL updates the cache time-to-live
func (cv *CachedValidator) SetCacheTTL(ttl time.Duration) {
	cv.cache.SetTTL(ttl)
}

// Stop gracefully stops the cached validator and its cache
func (cv *CachedValidator) Stop() {
	cv.cache.Stop()
}

// CachedValidatorManager manages multiple cached validators with a shared cache
type CachedValidatorManager struct {
	mu         sync.RWMutex
	validators map[string]*CachedValidator
	sharedCache *ValidationCache
}

// NewCachedValidatorManager creates a new cached validator manager
func NewCachedValidatorManager(cacheConfig *ValidationCacheConfig) *CachedValidatorManager {
	if cacheConfig == nil {
		cacheConfig = DefaultValidationCacheConfig()
	}

	return &CachedValidatorManager{
		validators:  make(map[string]*CachedValidator),
		sharedCache: NewValidationCache(cacheConfig),
	}
}

// WrapValidator wraps an existing validator with caching using the shared cache
func (cvm *CachedValidatorManager) WrapValidator(validator Validator) *CachedValidator {
	cvm.mu.Lock()
	defer cvm.mu.Unlock()

	name := validator.Name()
	if existing, exists := cvm.validators[name]; exists {
		return existing
	}

	cached := NewCachedValidatorWithCache(validator, cvm.sharedCache)
	cvm.validators[name] = cached
	return cached
}

// GetValidator returns a cached validator by name
func (cvm *CachedValidatorManager) GetValidator(name string) (*CachedValidator, bool) {
	cvm.mu.RLock()
	defer cvm.mu.RUnlock()

	validator, exists := cvm.validators[name]
	return validator, exists
}

// GetAllValidators returns all cached validators
func (cvm *CachedValidatorManager) GetAllValidators() map[string]*CachedValidator {
	cvm.mu.RLock()
	defer cvm.mu.RUnlock()

	result := make(map[string]*CachedValidator)
	for name, validator := range cvm.validators {
		result[name] = validator
	}
	return result
}

// GetSharedCache returns the shared cache used by all validators
func (cvm *CachedValidatorManager) GetSharedCache() *ValidationCache {
	return cvm.sharedCache
}

// GetAggregatedMetrics returns aggregated metrics across all validators
func (cvm *CachedValidatorManager) GetAggregatedMetrics() ValidationCacheMetrics {
	return cvm.sharedCache.GetMetrics()
}

// InvalidateAll invalidates all cached results across all validators
func (cvm *CachedValidatorManager) InvalidateAll() {
	cvm.sharedCache.Invalidate()
}

// EnableAllCaches enables caching for all managed validators
func (cvm *CachedValidatorManager) EnableAllCaches() {
	cvm.mu.RLock()
	defer cvm.mu.RUnlock()

	for _, validator := range cvm.validators {
		validator.EnableCache()
	}
}

// DisableAllCaches disables caching for all managed validators
func (cvm *CachedValidatorManager) DisableAllCaches() {
	cvm.mu.RLock()
	defer cvm.mu.RUnlock()

	for _, validator := range cvm.validators {
		validator.DisableCache()
	}
}

// Stop gracefully stops the manager and all cached validators
func (cvm *CachedValidatorManager) Stop() {
	cvm.mu.Lock()
	defer cvm.mu.Unlock()

	cvm.sharedCache.Stop()

	for _, validator := range cvm.validators {
		validator.Stop()
	}
}

// Size returns the number of managed validators
func (cvm *CachedValidatorManager) Size() int {
	cvm.mu.RLock()
	defer cvm.mu.RUnlock()
	return len(cvm.validators)
}