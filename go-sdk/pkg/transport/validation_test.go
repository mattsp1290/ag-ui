package transport

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"
)

// TestEvent implements TransportEvent for testing
type TestEvent struct {
	id        string
	eventType string
	timestamp time.Time
	data      map[string]interface{}
}

func (e *TestEvent) ID() string {
	return e.id
}

func (e *TestEvent) Type() string {
	return e.eventType
}

func (e *TestEvent) Timestamp() time.Time {
	return e.timestamp
}

func (e *TestEvent) Data() map[string]interface{} {
	return e.data
}

func TestDefaultValidator(t *testing.T) {
	config := DefaultValidationConfig()
	validator := NewValidator(config)

	// Test valid event
	validEvent := &TestEvent{
		id:        "test-123",
		eventType: "test",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"id":        "test-123",
			"type":      "test",
			"timestamp": time.Now(),
			"message":   "hello world",
		},
	}

	ctx := context.Background()
	if err := validator.Validate(ctx, validEvent); err != nil {
		t.Errorf("Expected valid event to pass validation, got error: %v", err)
	}

	// Test invalid event - missing required field
	invalidEvent := &TestEvent{
		id:        "test-456",
		eventType: "test",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"id":      "test-456",
			"message": "hello world",
			// Missing "type" and "timestamp" fields
		},
	}

	if err := validator.Validate(ctx, invalidEvent); err == nil {
		t.Error("Expected invalid event to fail validation")
	}
}

func TestMessageSizeValidation(t *testing.T) {
	// Test with minimal configuration to isolate message size validation
	config := &ValidationConfig{
		Enabled:        true,
		MaxMessageSize: 50, // Very small size for testing
		RequiredFields: []string{}, // No required fields
		AllowedEventTypes: []string{}, // No event type restrictions
		DeniedEventTypes:  []string{}, // No denied types
		MaxDataDepth:      100, // High depth limit
		MaxArraySize:      1000, // High array size limit
		MaxStringLength:   10000, // High string length limit
		AllowedDataTypes:  []string{"string", "number", "boolean", "object", "array", "null"},
		FailFast:          false,
		CollectAllErrors:  true,
	}
	validator := NewValidator(config)

	// Create a large message
	largeData := make(map[string]interface{})
	largeData["id"] = "test-123"
	largeData["type"] = "test"
	largeData["large_field"] = string(make([]byte, 200)) // Larger than limit

	largeEvent := &TestEvent{
		id:        "test-123",
		eventType: "test",
		timestamp: time.Now(),
		data:      largeData,
	}

	ctx := context.Background()
	
	// Check actual serialized size
	serialized, _ := json.Marshal(largeEvent.Data())
	t.Logf("Serialized size: %d bytes (limit: %d)", len(serialized), config.MaxMessageSize)
	
	// Check if validator rules are enabled
	t.Logf("Validator config enabled: %t", config.Enabled)
	t.Logf("Max message size: %d", config.MaxMessageSize)
	
	if err := validator.Validate(ctx, largeEvent); err == nil {
		t.Error("Expected large message to fail validation")
	} else {
		t.Logf("Validation error: %v", err)
	}
}

func TestEventTypeValidation(t *testing.T) {
	config := &ValidationConfig{
		Enabled:           true,
		AllowedEventTypes: []string{"allowed_type"},
		DeniedEventTypes:  []string{"denied_type"},
		MaxMessageSize:    1024 * 1024, // High limit to avoid size issues
		RequiredFields:    []string{}, // No required fields to avoid other validation issues
		MaxDataDepth:      100,
		MaxArraySize:      1000,
		MaxStringLength:   10000,
		AllowedDataTypes:  []string{"string", "number", "boolean", "object", "array", "null"},
		FailFast:          false,
		CollectAllErrors:  true,
	}
	validator := NewValidator(config)

	ctx := context.Background()

	// Test allowed type
	allowedEvent := &TestEvent{
		id:        "test-123",
		eventType: "allowed_type",
		timestamp: time.Now(),
		data:      map[string]interface{}{},
	}

	if err := validator.Validate(ctx, allowedEvent); err != nil {
		t.Errorf("Expected allowed event type to pass validation, got error: %v", err)
	}

	// Test denied type
	deniedEvent := &TestEvent{
		id:        "test-456",
		eventType: "denied_type",
		timestamp: time.Now(),
		data:      map[string]interface{}{},
	}

	if err := validator.Validate(ctx, deniedEvent); err == nil {
		t.Error("Expected denied event type to fail validation")
	} else {
		t.Logf("Denied event validation error: %v", err)
	}

	// Test disallowed type
	disallowedEvent := &TestEvent{
		id:        "test-789",
		eventType: "disallowed_type",
		timestamp: time.Now(),
		data:      map[string]interface{}{},
	}

	if err := validator.Validate(ctx, disallowedEvent); err == nil {
		t.Error("Expected disallowed event type to fail validation")
	} else {
		t.Logf("Disallowed event validation error: %v", err)
	}
}

func TestDataFormatValidation(t *testing.T) {
	config := &ValidationConfig{
		Enabled:          true,
		MaxDataDepth:     2,
		MaxArraySize:     3,
		MaxStringLength:  10,
		AllowedDataTypes: []string{"string", "number", "boolean", "object", "array"},
		MaxMessageSize:   1024 * 1024, // High limit to avoid size issues
		RequiredFields:   []string{}, // No required fields to avoid other validation issues
		AllowedEventTypes: []string{}, // No event type restrictions
		DeniedEventTypes:  []string{}, // No denied types
		FailFast:         false,
		CollectAllErrors: true,
	}
	validator := NewValidator(config)

	ctx := context.Background()

	// Test valid data
	validEvent := &TestEvent{
		id:        "test-123",
		eventType: "test",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"string_field":  "hello",
			"number_field":  42,
			"boolean_field": true,
			"object_field": map[string]interface{}{
				"nested": "value",
			},
			"array_field": []interface{}{1, 2, 3},
		},
	}

	if err := validator.Validate(ctx, validEvent); err != nil {
		t.Errorf("Expected valid data to pass validation, got error: %v", err)
	}

	// Test string too long
	longStringEvent := &TestEvent{
		id:        "test-456",
		eventType: "test",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"long_string": "this string is too long",
		},
	}

	if err := validator.Validate(ctx, longStringEvent); err == nil {
		t.Error("Expected long string to fail validation")
	} else {
		t.Logf("Long string validation error: %v", err)
	}

	// Test array too large
	largeArrayEvent := &TestEvent{
		id:        "test-789",
		eventType: "test",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"large_array": []interface{}{1, 2, 3, 4, 5},
		},
	}

	if err := validator.Validate(ctx, largeArrayEvent); err == nil {
		t.Error("Expected large array to fail validation")
	} else {
		t.Logf("Large array validation error: %v", err)
	}

	// Test depth too deep
	deepEvent := &TestEvent{
		id:        "test-101",
		eventType: "test",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"level3": "too deep",
				},
			},
		},
	}

	if err := validator.Validate(ctx, deepEvent); err == nil {
		t.Error("Expected deep nesting to fail validation")
	} else {
		t.Logf("Deep nesting validation error: %v", err)
	}
}

func TestPatternValidation(t *testing.T) {
	config := &ValidationConfig{
		Enabled: true,
		PatternValidators: map[string]*regexp.Regexp{
			"email": regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
			"phone": regexp.MustCompile(`^\+?[1-9]\d{1,14}$`),
		},
		MaxMessageSize:   1024 * 1024, // High limit to avoid size issues
		RequiredFields:   []string{}, // No required fields to avoid other validation issues
		AllowedEventTypes: []string{}, // No event type restrictions
		DeniedEventTypes:  []string{}, // No denied types
		MaxDataDepth:     100,
		MaxArraySize:     1000,
		MaxStringLength:  10000,
		AllowedDataTypes: []string{"string", "number", "boolean", "object", "array", "null"},
		FailFast:         false,
		CollectAllErrors: true,
	}
	validator := NewValidator(config)

	ctx := context.Background()

	// Test valid email
	validEmailEvent := &TestEvent{
		id:        "test-123",
		eventType: "test",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"email": "test@example.com",
		},
	}

	if err := validator.Validate(ctx, validEmailEvent); err != nil {
		t.Errorf("Expected valid email to pass validation, got error: %v", err)
	}

	// Test invalid email
	invalidEmailEvent := &TestEvent{
		id:        "test-456",
		eventType: "test",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"email": "invalid-email",
		},
	}

	if err := validator.Validate(ctx, invalidEmailEvent); err == nil {
		t.Error("Expected invalid email to fail validation")
	} else {
		t.Logf("Invalid email validation error: %v", err)
	}

	// Test valid phone
	validPhoneEvent := &TestEvent{
		id:        "test-789",
		eventType: "test",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"phone": "+1234567890",
		},
	}

	if err := validator.Validate(ctx, validPhoneEvent); err != nil {
		t.Errorf("Expected valid phone to pass validation, got error: %v", err)
	}

	// Test invalid phone
	invalidPhoneEvent := &TestEvent{
		id:        "test-101",
		eventType: "test",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"phone": "invalid-phone",
		},
	}

	if err := validator.Validate(ctx, invalidPhoneEvent); err == nil {
		t.Error("Expected invalid phone to fail validation")
	} else {
		t.Logf("Invalid phone validation error: %v", err)
	}
}

func TestValidationError(t *testing.T) {
	err := NewValidationError("test error", []error{
		NewValidationError("sub error 1", nil),
		NewValidationError("sub error 2", nil),
	})

	if err.Error() == "" {
		t.Error("Expected validation error to have message")
	}

	if !IsValidationError(err) {
		t.Error("Expected error to be identified as validation error")
	}

	if len(err.Errors()) != 2 {
		t.Errorf("Expected 2 sub-errors, got %d", len(err.Errors()))
	}
}

func TestValidationMiddleware(t *testing.T) {
	config := &ValidationConfig{
		Enabled:        true,
		RequiredFields: []string{"id", "type"},
	}
	
	middleware := NewValidationMiddleware(config)
	
	// Test that middleware can be created and configured
	if !middleware.IsEnabled() {
		t.Error("Expected middleware to be enabled")
	}

	metrics := middleware.GetMetrics()
	if metrics.TotalValidations != 0 {
		t.Error("Expected initial metrics to be zero")
	}

	// Test enabling/disabling
	middleware.SetEnabled(false)
	if middleware.IsEnabled() {
		t.Error("Expected middleware to be disabled")
	}

	middleware.SetEnabled(true)
	if !middleware.IsEnabled() {
		t.Error("Expected middleware to be enabled")
	}
}

func TestFastValidator(t *testing.T) {
	config := &ValidationConfig{
		Enabled:           true,
		MaxMessageSize:    1000,
		RequiredFields:    []string{"id", "type"},
		AllowedEventTypes: []string{"test"},
	}
	
	validator := NewFastValidator(config)
	ctx := context.Background()

	// Test valid event
	validEvent := &TestEvent{
		id:        "test-123",
		eventType: "test",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"id":   "test-123",
			"type": "test",
		},
	}

	if err := validator.Validate(ctx, validEvent); err != nil {
		t.Errorf("Expected valid event to pass fast validation, got error: %v", err)
	}

	// Test invalid event type
	invalidEvent := &TestEvent{
		id:        "test-456",
		eventType: "invalid",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"id":   "test-456",
			"type": "invalid",
		},
	}

	if err := validator.Validate(ctx, invalidEvent); err == nil {
		t.Error("Expected invalid event type to fail fast validation")
	}
}

func TestCachedValidator(t *testing.T) {
	baseValidator := NewValidator(DefaultValidationConfig())
	cachedValidator := NewCachedValidator(baseValidator, 100, time.Minute)

	ctx := context.Background()
	fixedTime := time.Now()
	event := &TestEvent{
		id:        "test-123",
		eventType: "test",
		timestamp: fixedTime,
		data: map[string]interface{}{
			"id":        "test-123",
			"type":      "test",
			"timestamp": fixedTime, // Use the same timestamp
		},
	}

	// First validation - should be a cache miss
	if err := cachedValidator.Validate(ctx, event); err != nil {
		t.Errorf("Expected validation to succeed, got error: %v", err)
	}

	// Second validation - should be a cache hit
	if err := cachedValidator.Validate(ctx, event); err != nil {
		t.Errorf("Expected cached validation to succeed, got error: %v", err)
	}

	stats := cachedValidator.GetCacheStats()
	if stats.TotalOps != 2 {
		t.Errorf("Expected 2 total operations, got %d", stats.TotalOps)
	}
	if stats.Hits != 1 {
		t.Errorf("Expected 1 cache hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 cache miss, got %d", stats.Misses)
	}
}

func TestValidationPool(t *testing.T) {
	factory := func() Validator {
		return NewValidator(DefaultValidationConfig())
	}

	pool := NewValidationPool(3, factory)

	// Get validators from pool
	v1 := pool.Get()
	v2 := pool.Get()
	v3 := pool.Get()

	if v1 == nil || v2 == nil || v3 == nil {
		t.Error("Expected validators to be created")
	}

	// Return validators to pool
	pool.Put(v1)
	pool.Put(v2)
	pool.Put(v3)

	// Get validator from pool again - should reuse
	v4 := pool.Get()
	if v4 == nil {
		t.Error("Expected validator to be reused from pool")
	}
}

func TestBatchValidator(t *testing.T) {
	baseValidator := NewValidator(DefaultValidationConfig())
	batchValidator := NewBatchValidator(baseValidator, 2)

	ctx := context.Background()
	events := []TransportEvent{
		&TestEvent{
			id:        "test-1",
			eventType: "test",
			timestamp: time.Now(),
			data: map[string]interface{}{
				"id":        "test-1",
				"type":      "test",
				"timestamp": time.Now(),
			},
		},
		&TestEvent{
			id:        "test-2",
			eventType: "test",
			timestamp: time.Now(),
			data: map[string]interface{}{
				"id":        "test-2",
				"type":      "test",
				"timestamp": time.Now(),
			},
		},
		&TestEvent{
			id:        "test-3",
			eventType: "test",
			timestamp: time.Now(),
			data: map[string]interface{}{
				"id":   "test-3",
				"type": "test",
				// Missing timestamp - should fail
			},
		},
	}

	results := batchValidator.ValidateBatch(ctx, events)
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// First two should pass
	if results[0] != nil {
		t.Errorf("Expected first event to pass validation, got error: %v", results[0])
	}
	if results[1] != nil {
		t.Errorf("Expected second event to pass validation, got error: %v", results[1])
	}

	// Third should fail
	if results[2] == nil {
		t.Error("Expected third event to fail validation")
	}
}

func BenchmarkDefaultValidator(b *testing.B) {
	config := DefaultValidationConfig()
	validator := NewValidator(config)

	event := &TestEvent{
		id:        "test-123",
		eventType: "test",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"id":        "test-123",
			"type":      "test",
			"timestamp": time.Now(),
			"message":   "hello world",
			"number":    42,
			"boolean":   true,
		},
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		validator.Validate(ctx, event)
	}
}

func BenchmarkFastValidator(b *testing.B) {
	config := &ValidationConfig{
		Enabled:        true,
		MaxMessageSize: 1024,
		RequiredFields: []string{"id", "type"},
	}
	validator := NewFastValidator(config)

	event := &TestEvent{
		id:        "test-123",
		eventType: "test",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"id":        "test-123",
			"type":      "test",
			"timestamp": time.Now(),
			"message":   "hello world",
			"number":    42,
			"boolean":   true,
		},
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		validator.Validate(ctx, event)
	}
}

func BenchmarkCachedValidator(b *testing.B) {
	baseValidator := NewValidator(DefaultValidationConfig())
	cachedValidator := NewCachedValidator(baseValidator, 1000, time.Minute)

	event := &TestEvent{
		id:        "test-123",
		eventType: "test",
		timestamp: time.Now(),
		data: map[string]interface{}{
			"id":        "test-123",
			"type":      "test",
			"timestamp": time.Now(),
			"message":   "hello world",
			"number":    42,
			"boolean":   true,
		},
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cachedValidator.Validate(ctx, event)
	}
}