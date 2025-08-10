package config

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// MockValidator for testing
type MockValidator struct {
	name            string
	validateFunc    func(map[string]interface{}) error
	validateFieldFunc func(string, interface{}) error
	schema          map[string]interface{}
	callCount       int
	mu              sync.Mutex
}

func NewMockValidator(name string) *MockValidator {
	return &MockValidator{
		name:   name,
		schema: map[string]interface{}{"type": "object"},
		validateFunc: func(config map[string]interface{}) error {
			// Default: return nil (validation passes)
			return nil
		},
		validateFieldFunc: func(key string, value interface{}) error {
			// Default: return nil (validation passes)
			return nil
		},
	}
}

func (m *MockValidator) Name() string {
	return m.name
}

func (m *MockValidator) Validate(config map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.validateFunc(config)
}

func (m *MockValidator) ValidateField(key string, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.validateFieldFunc(key, value)
}

func (m *MockValidator) GetSchema() map[string]interface{} {
	return m.schema
}

func (m *MockValidator) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func (m *MockValidator) SetValidateFunc(f func(map[string]interface{}) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validateFunc = f
}

func (m *MockValidator) SetValidateFieldFunc(f func(string, interface{}) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validateFieldFunc = f
}

func TestCachedValidator_BasicCaching(t *testing.T) {
	mockValidator := NewMockValidator("test-validator")
	mockValidator.SetValidateFunc(func(config map[string]interface{}) error {
		// Simulate an expensive validation that returns an error
		time.Sleep(1 * time.Millisecond)
		return fmt.Errorf("validation failed")
	})

	cachedValidator := NewCachedValidator(mockValidator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()

	config := map[string]interface{}{
		"name": "test",
		"value": 123,
	}

	// First call should hit the underlying validator
	start := time.Now()
	err1 := cachedValidator.Validate(config)
	duration1 := time.Since(start)

	if err1 == nil {
		t.Error("Expected validation error")
	}
	if mockValidator.GetCallCount() != 1 {
		t.Errorf("Expected 1 call to underlying validator, got %d", mockValidator.GetCallCount())
	}

	// Second call should use cache (faster)
	start = time.Now()
	err2 := cachedValidator.Validate(config)
	duration2 := time.Since(start)

	if err2 == nil {
		t.Error("Expected validation error from cache")
	}
	if err1.Error() != err2.Error() {
		t.Error("Cached error should match original error")
	}
	if mockValidator.GetCallCount() != 1 {
		t.Errorf("Expected still 1 call to underlying validator (cached), got %d", mockValidator.GetCallCount())
	}

	// Cache should be significantly faster
	if duration2 >= duration1 {
		t.Logf("Warning: Cached call (%v) should be faster than original call (%v)", duration2, duration1)
	}

	// Verify cache metrics
	metrics := cachedValidator.GetMetrics()
	if metrics.Hits != 1 {
		t.Errorf("Expected 1 cache hit, got %d", metrics.Hits)
	}
	if metrics.Misses != 1 {
		t.Errorf("Expected 1 cache miss, got %d", metrics.Misses)
	}
}

func TestCachedValidator_SuccessfulValidationCaching(t *testing.T) {
	mockValidator := NewMockValidator("test-validator")
	mockValidator.SetValidateFunc(func(config map[string]interface{}) error {
		return nil // Validation passes
	})

	cachedValidator := NewCachedValidator(mockValidator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()

	config := map[string]interface{}{
		"name": "test",
		"value": 123,
	}

	// First call
	err1 := cachedValidator.Validate(config)
	if err1 != nil {
		t.Errorf("Expected no validation error, got %v", err1)
	}

	// Second call should use cache
	err2 := cachedValidator.Validate(config)
	if err2 != nil {
		t.Errorf("Expected no validation error from cache, got %v", err2)
	}

	if mockValidator.GetCallCount() != 1 {
		t.Errorf("Expected 1 call to underlying validator (second should be cached), got %d", mockValidator.GetCallCount())
	}
}

func TestCachedValidator_FieldValidation(t *testing.T) {
	mockValidator := NewMockValidator("test-validator")
	mockValidator.SetValidateFieldFunc(func(key string, value interface{}) error {
		if key == "invalid" {
			return fmt.Errorf("field validation failed")
		}
		return nil
	})

	cachedValidator := NewCachedValidator(mockValidator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()

	// Test valid field
	err1 := cachedValidator.ValidateField("valid", "value")
	if err1 != nil {
		t.Errorf("Expected no field validation error, got %v", err1)
	}

	// Test cached valid field
	err2 := cachedValidator.ValidateField("valid", "value")
	if err2 != nil {
		t.Errorf("Expected no field validation error from cache, got %v", err2)
	}

	// Test invalid field
	err3 := cachedValidator.ValidateField("invalid", "value")
	if err3 == nil {
		t.Error("Expected field validation error")
	}

	// Test cached invalid field
	err4 := cachedValidator.ValidateField("invalid", "value")
	if err4 == nil {
		t.Error("Expected field validation error from cache")
	}

	// Should have called underlying validator only twice (once for each unique field)
	expectedCalls := 2
	if mockValidator.GetCallCount() != expectedCalls {
		t.Errorf("Expected %d calls to underlying validator, got %d", expectedCalls, mockValidator.GetCallCount())
	}
}

func TestCachedValidator_CacheInvalidation(t *testing.T) {
	mockValidator := NewMockValidator("test-validator")
	cachedValidator := NewCachedValidator(mockValidator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()

	config := map[string]interface{}{
		"name": "test",
		"value": 123,
	}

	// First validation to populate cache
	cachedValidator.Validate(config)
	
	// Verify cache has entry
	metrics := cachedValidator.GetMetrics()
	if metrics.Size != 1 {
		t.Errorf("Expected cache size 1, got %d", metrics.Size)
	}

	// Invalidate cache
	cachedValidator.InvalidateCache()

	// Verify cache is empty
	metrics = cachedValidator.GetMetrics()
	if metrics.Size != 0 {
		t.Errorf("Expected cache size 0 after invalidation, got %d", metrics.Size)
	}
	if metrics.Invalidations == 0 {
		t.Error("Expected invalidations count to be greater than 0")
	}

	// Next validation should call underlying validator again
	oldCallCount := mockValidator.GetCallCount()
	cachedValidator.Validate(config)
	
	if mockValidator.GetCallCount() != oldCallCount+1 {
		t.Error("Expected underlying validator to be called after cache invalidation")
	}
}

func TestCachedValidator_EnableDisableCache(t *testing.T) {
	mockValidator := NewMockValidator("test-validator")
	cachedValidator := NewCachedValidator(mockValidator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()

	config := map[string]interface{}{
		"name": "test",
		"value": 123,
	}

	// Verify caching is enabled by default
	if !cachedValidator.IsCacheEnabled() {
		t.Error("Expected cache to be enabled by default")
	}

	// First validation
	cachedValidator.Validate(config)
	firstCallCount := mockValidator.GetCallCount()

	// Second validation (should be cached)
	cachedValidator.Validate(config)
	if mockValidator.GetCallCount() != firstCallCount {
		t.Error("Expected second validation to be cached")
	}

	// Disable cache
	cachedValidator.DisableCache()
	if cachedValidator.IsCacheEnabled() {
		t.Error("Expected cache to be disabled")
	}

	// Third validation (should call underlying validator)
	cachedValidator.Validate(config)
	if mockValidator.GetCallCount() == firstCallCount {
		t.Error("Expected third validation to call underlying validator when cache is disabled")
	}

	// Re-enable cache
	cachedValidator.EnableCache()
	if !cachedValidator.IsCacheEnabled() {
		t.Error("Expected cache to be re-enabled")
	}
}

func TestCachedValidator_ConcurrentAccess(t *testing.T) {
	mockValidator := NewMockValidator("test-validator")
	mockValidator.SetValidateFunc(func(config map[string]interface{}) error {
		// Simulate some work
		time.Sleep(1 * time.Millisecond)
		return nil
	})

	cachedValidator := NewCachedValidator(mockValidator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()

	const numGoroutines = 50
	const numOperationsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch multiple goroutines performing validations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperationsPerGoroutine; j++ {
				config := map[string]interface{}{
					"id":    id,
					"value": j,
				}
				
				err := cachedValidator.Validate(config)
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify the validator is still functional
	finalConfig := map[string]interface{}{"final": "test"}
	err := cachedValidator.Validate(finalConfig)
	if err != nil {
		t.Errorf("Validator should be functional after concurrent access: %v", err)
	}
}

func TestCachedValidatorManager(t *testing.T) {
	manager := NewCachedValidatorManager(DefaultValidationCacheConfig())
	defer manager.Stop()

	// Create mock validators
	validator1 := NewMockValidator("validator-1")
	validator2 := NewMockValidator("validator-2")

	// Wrap validators
	cachedValidator1 := manager.WrapValidator(validator1)
	cachedValidator2 := manager.WrapValidator(validator2)

	// Verify they were added to manager
	if manager.Size() != 2 {
		t.Errorf("Expected manager size 2, got %d", manager.Size())
	}

	// Verify we can retrieve them
	retrieved1, found1 := manager.GetValidator("validator-1")
	if !found1 {
		t.Error("Expected to find validator-1")
	}
	if retrieved1 != cachedValidator1 {
		t.Error("Retrieved validator should match original")
	}

	// Test validation with shared cache
	config := map[string]interface{}{"shared": "config"}
	
	err1 := cachedValidator1.Validate(config)
	if err1 != nil {
		t.Errorf("Unexpected validation error: %v", err1)
	}

	err2 := cachedValidator2.Validate(config)
	if err2 != nil {
		t.Errorf("Unexpected validation error: %v", err2)
	}

	// Test aggregated metrics
	metrics := manager.GetAggregatedMetrics()
	if metrics.TotalChecks < 2 {
		t.Errorf("Expected at least 2 total checks, got %d", metrics.TotalChecks)
	}

	// Test manager-wide invalidation
	manager.InvalidateAll()
	metrics = manager.GetAggregatedMetrics()
	if metrics.Size != 0 {
		t.Errorf("Expected cache size 0 after invalidation, got %d", metrics.Size)
	}
}

func TestCachedValidator_Name(t *testing.T) {
	mockValidator := NewMockValidator("test-validator")
	
	// Test with cache enabled
	cachedValidator := NewCachedValidator(mockValidator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()
	
	expectedName := "test-validator (cached)"
	if cachedValidator.Name() != expectedName {
		t.Errorf("Expected name '%s', got '%s'", expectedName, cachedValidator.Name())
	}

	// Test with cache disabled
	cachedValidator.DisableCache()
	expectedName = "test-validator"
	if cachedValidator.Name() != expectedName {
		t.Errorf("Expected name '%s' when cache disabled, got '%s'", expectedName, cachedValidator.Name())
	}
}

func TestCachedValidator_HitRatio(t *testing.T) {
	mockValidator := NewMockValidator("test-validator")
	cachedValidator := NewCachedValidator(mockValidator, DefaultValidationCacheConfig())
	defer cachedValidator.Stop()

	config := map[string]interface{}{"key": "value"}

	// Initial hit ratio should be 0
	if cachedValidator.GetHitRatio() != 0.0 {
		t.Errorf("Expected initial hit ratio 0%%, got %.1f%%", cachedValidator.GetHitRatio())
	}

	// First call (cache miss)
	cachedValidator.Validate(config)
	hitRatio := cachedValidator.GetHitRatio()
	if hitRatio != 0.0 {
		t.Errorf("Expected hit ratio 0%% after first call, got %.1f%%", hitRatio)
	}

	// Second call (cache hit)
	cachedValidator.Validate(config)
	hitRatio = cachedValidator.GetHitRatio()
	if hitRatio != 50.0 {
		t.Errorf("Expected hit ratio 50%% after second call, got %.1f%%", hitRatio)
	}

	// Third call (cache hit)
	cachedValidator.Validate(config)
	hitRatio = cachedValidator.GetHitRatio()
	expectedRatio := 2.0/3.0 * 100.0 // 2 hits out of 3 total checks
	tolerance := 0.1 // Allow small floating point precision differences
	if hitRatio < expectedRatio-tolerance || hitRatio > expectedRatio+tolerance {
		t.Errorf("Expected hit ratio %.1f%% after third call, got %.1f%%", expectedRatio, hitRatio)
	}
}