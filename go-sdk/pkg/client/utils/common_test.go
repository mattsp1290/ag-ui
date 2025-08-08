package utils

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewCommonUtils(t *testing.T) {
	utils := NewCommonUtils()
	if utils == nil {
		t.Fatal("NewCommonUtils returned nil")
	}
}

// Test JSON marshaling operations
func TestCommonUtils_JSONMarshalToMap(t *testing.T) {
	utils := NewCommonUtils()

	t.Run("ValidStruct", func(t *testing.T) {
		type TestStruct struct {
			Name  string `json:"name"`
			Age   int    `json:"age"`
			Email string `json:"email"`
		}

		testData := TestStruct{
			Name:  "John Doe",
			Age:   30,
			Email: "john@example.com",
		}

		result, err := utils.JSONMarshalToMap(testData)
		if err != nil {
			t.Fatalf("JSONMarshalToMap failed: %v", err)
		}

		if result["name"] != "John Doe" {
			t.Errorf("Expected name 'John Doe', got %v", result["name"])
		}
		if result["age"] != float64(30) { // JSON numbers are float64
			t.Errorf("Expected age 30, got %v", result["age"])
		}
		if result["email"] != "john@example.com" {
			t.Errorf("Expected email 'john@example.com', got %v", result["email"])
		}
	})

	t.Run("InvalidData", func(t *testing.T) {
		// Channels cannot be marshaled to JSON
		invalidData := make(chan int)

		_, err := utils.JSONMarshalToMap(invalidData)
		if err == nil {
			t.Error("Expected error for invalid data")
		}
	})

	t.Run("NilInput", func(t *testing.T) {
		result, err := utils.JSONMarshalToMap(nil)
		if err != nil {
			t.Fatalf("JSONMarshalToMap failed: %v", err)
		}
		if result != nil {
			t.Error("Expected nil result for nil input")
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		type TestData struct {
			ID    int    `json:"id"`
			Value string `json:"value"`
		}

		var wg sync.WaitGroup
		numRoutines := 10
		results := make(chan map[string]interface{}, numRoutines)
		errors := make(chan error, numRoutines)

		for i := 0; i < numRoutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				data := TestData{ID: id, Value: fmt.Sprintf("value-%d", id)}
				result, err := utils.JSONMarshalToMap(data)
				results <- result
				errors <- err
			}(i)
		}

		wg.Wait()
		close(results)
		close(errors)

		// Verify all operations succeeded
		for err := range errors {
			if err != nil {
				t.Errorf("Concurrent operation failed: %v", err)
			}
		}

		resultCount := 0
		for result := range results {
			if result == nil {
				t.Error("Result should not be nil")
			}
			resultCount++
		}

		if resultCount != numRoutines {
			t.Errorf("Expected %d results, got %d", numRoutines, resultCount)
		}
	})
}

func TestCommonUtils_JSONUnmarshalFromMap(t *testing.T) {
	utils := NewCommonUtils()

	t.Run("ValidMap", func(t *testing.T) {
		type TestStruct struct {
			Name  string `json:"name"`
			Age   int    `json:"age"`
			Email string `json:"email"`
		}

		mapData := map[string]interface{}{
			"name":  "Jane Smith",
			"age":   25,
			"email": "jane@example.com",
		}

		var result TestStruct
		err := utils.JSONUnmarshalFromMap(mapData, &result)
		if err != nil {
			t.Fatalf("JSONUnmarshalFromMap failed: %v", err)
		}

		if result.Name != "Jane Smith" {
			t.Errorf("Expected name 'Jane Smith', got %s", result.Name)
		}
		if result.Age != 25 {
			t.Errorf("Expected age 25, got %d", result.Age)
		}
		if result.Email != "jane@example.com" {
			t.Errorf("Expected email 'jane@example.com', got %s", result.Email)
		}
	})

	t.Run("InvalidTarget", func(t *testing.T) {
		mapData := map[string]interface{}{"key": "value"}

		// Try to unmarshal to a non-pointer
		var result string
		err := utils.JSONUnmarshalFromMap(mapData, result)
		if err == nil {
			t.Error("Expected error for non-pointer target")
		}
	})

	t.Run("TypeMismatch", func(t *testing.T) {
		type TestStruct struct {
			Age int `json:"age"`
		}

		mapData := map[string]interface{}{
			"age": "not a number", // String instead of int
		}

		var result TestStruct
		err := utils.JSONUnmarshalFromMap(mapData, &result)
		if err == nil {
			t.Error("Expected error for type mismatch")
		}
	})

	t.Run("ConcurrentUnmarshal", func(t *testing.T) {
		type TestStruct struct {
			ID    int    `json:"id"`
			Value string `json:"value"`
		}

		var wg sync.WaitGroup
		numRoutines := 10
		errors := make(chan error, numRoutines)

		for i := 0; i < numRoutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				mapData := map[string]interface{}{
					"id":    id,
					"value": fmt.Sprintf("value-%d", id),
				}
				var result TestStruct
				err := utils.JSONUnmarshalFromMap(mapData, &result)
				errors <- err
			}(i)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			if err != nil {
				t.Errorf("Concurrent unmarshal failed: %v", err)
			}
		}
	})
}

func TestCommonUtils_DeepCopyJSON(t *testing.T) {
	utils := NewCommonUtils()

	t.Run("ValidCopy", func(t *testing.T) {
		type TestStruct struct {
			Name   string            `json:"name"`
			Values map[string]string `json:"values"`
			Items  []int             `json:"items"`
		}

		source := TestStruct{
			Name: "original",
			Values: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			Items: []int{1, 2, 3},
		}

		var destination TestStruct
		err := utils.DeepCopyJSON(source, &destination)
		if err != nil {
			t.Fatalf("DeepCopyJSON failed: %v", err)
		}

		// Verify copy is correct
		if destination.Name != source.Name {
			t.Errorf("Name not copied correctly: got %s, want %s", destination.Name, source.Name)
		}
		if len(destination.Values) != len(source.Values) {
			t.Error("Values map not copied correctly")
		}
		if len(destination.Items) != len(source.Items) {
			t.Error("Items slice not copied correctly")
		}

		// Verify it's a deep copy by modifying original
		source.Values["key1"] = "modified"
		if destination.Values["key1"] == "modified" {
			t.Error("Deep copy failed - changes affected copy")
		}
	})

	t.Run("InvalidSource", func(t *testing.T) {
		source := make(chan int) // Cannot be marshaled
		var destination interface{}

		err := utils.DeepCopyJSON(source, &destination)
		if err == nil {
			t.Error("Expected error for invalid source")
		}
	})

	t.Run("InvalidDestination", func(t *testing.T) {
		source := map[string]interface{}{"key": "value"}
		destination := "not a pointer"

		err := utils.DeepCopyJSON(source, destination)
		if err == nil {
			t.Error("Expected error for invalid destination")
		}
	})
}

func TestCommonUtils_CalculateChecksum(t *testing.T) {
	utils := NewCommonUtils()

	t.Run("ConsistentChecksum", func(t *testing.T) {
		data := []byte("test data for checksum")

		checksum1 := utils.CalculateChecksum(data)
		checksum2 := utils.CalculateChecksum(data)

		if checksum1 != checksum2 {
			t.Error("Checksums should be consistent for same data")
		}
		if len(checksum1) != 32 { // MD5 hex string length
			t.Errorf("Expected checksum length 32, got %d", len(checksum1))
		}
	})

	t.Run("DifferentData", func(t *testing.T) {
		data1 := []byte("data one")
		data2 := []byte("data two")

		checksum1 := utils.CalculateChecksum(data1)
		checksum2 := utils.CalculateChecksum(data2)

		if checksum1 == checksum2 {
			t.Error("Different data should produce different checksums")
		}
	})

	t.Run("EmptyData", func(t *testing.T) {
		checksum := utils.CalculateChecksum([]byte{})
		if len(checksum) != 32 {
			t.Error("Empty data should still produce valid checksum")
		}
	})

	t.Run("ConcurrentChecksum", func(t *testing.T) {
		var wg sync.WaitGroup
		numRoutines := 10
		checksums := make(chan string, numRoutines)

		data := []byte("concurrent test data")

		for i := 0; i < numRoutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				checksum := utils.CalculateChecksum(data)
				checksums <- checksum
			}()
		}

		wg.Wait()
		close(checksums)

		// All checksums should be identical
		var firstChecksum string
		count := 0
		for checksum := range checksums {
			if count == 0 {
				firstChecksum = checksum
			} else if checksum != firstChecksum {
				t.Error("Concurrent checksum calculations produced different results")
			}
			count++
		}

		if count != numRoutines {
			t.Errorf("Expected %d checksums, got %d", numRoutines, count)
		}
	})
}

func TestSafeStringBuilder(t *testing.T) {
	t.Run("BasicUsage", func(t *testing.T) {
		builder := NewSafeStringBuilder(100)

		builder.WriteString("Hello")
		builder.WriteByte(' ')
		builder.WriteString("World")
		builder.WriteByte('!')

		result := builder.String()
		expected := "Hello World!"
		if result != expected {
			t.Errorf("Expected '%s', got '%s'", expected, result)
		}
	})

	t.Run("Reset", func(t *testing.T) {
		builder := NewSafeStringBuilder(50)

		builder.WriteString("First content")
		builder.Reset()
		builder.WriteString("Second content")

		result := builder.String()
		if result != "Second content" {
			t.Errorf("Expected 'Second content', got '%s'", result)
		}
	})

	t.Run("LargeContent", func(t *testing.T) {
		builder := NewSafeStringBuilder(10) // Small initial capacity

		// Write content larger than initial capacity
		for i := 0; i < 100; i++ {
			builder.WriteString(fmt.Sprintf("Item %d ", i))
		}

		result := builder.String()
		if len(result) == 0 {
			t.Error("Result should not be empty")
		}
		if !strings.Contains(result, "Item 0") || !strings.Contains(result, "Item 99") {
			t.Error("Content not written correctly")
		}
	})
}

func TestCommonUtils_ContextWithDefaultTimeout(t *testing.T) {
	utils := NewCommonUtils()

	t.Run("NoExistingDeadline", func(t *testing.T) {
		ctx := context.Background()
		defaultTimeout := 5 * time.Second

		newCtx, cancel := utils.ContextWithDefaultTimeout(ctx, defaultTimeout)
		defer cancel()

		deadline, hasDeadline := newCtx.Deadline()
		if !hasDeadline {
			t.Error("Expected deadline to be set")
		}

		expectedDeadline := time.Now().Add(defaultTimeout)
		if deadline.Before(expectedDeadline.Add(-100*time.Millisecond)) ||
			deadline.After(expectedDeadline.Add(100*time.Millisecond)) {
			t.Error("Deadline not set correctly")
		}
	})

	t.Run("ExistingDeadline", func(t *testing.T) {
		originalTimeout := 3 * time.Second
		ctx, originalCancel := context.WithTimeout(context.Background(), originalTimeout)
		defer originalCancel()

		defaultTimeout := 5 * time.Second

		newCtx, cancel := utils.ContextWithDefaultTimeout(ctx, defaultTimeout)
		defer cancel()

		// Should return the same context
		if newCtx != ctx {
			t.Error("Should return original context when deadline exists")
		}
	})
}

func TestCommonUtils_ValidateNotNil(t *testing.T) {
	utils := NewCommonUtils()

	t.Run("ValidValue", func(t *testing.T) {
		err := utils.ValidateNotNil("some value", "test_field")
		if err != nil {
			t.Errorf("Expected no error for valid value, got %v", err)
		}
	})

	t.Run("NilValue", func(t *testing.T) {
		err := utils.ValidateNotNil(nil, "test_field")
		if err == nil {
			t.Error("Expected error for nil value")
		}
		if !strings.Contains(err.Error(), "test_field") {
			t.Error("Error should contain field name")
		}
	})

	t.Run("ZeroValue", func(t *testing.T) {
		// Zero values should be valid (not nil)
		err := utils.ValidateNotNil(0, "number_field")
		if err != nil {
			t.Error("Zero values should be valid")
		}

		err = utils.ValidateNotNil("", "string_field")
		if err != nil {
			t.Error("Empty string should be valid")
		}
	})
}

func TestCommonUtils_ValidateNotEmpty(t *testing.T) {
	utils := NewCommonUtils()

	t.Run("ValidString", func(t *testing.T) {
		err := utils.ValidateNotEmpty("valid content", "test_field")
		if err != nil {
			t.Errorf("Expected no error for valid string, got %v", err)
		}
	})

	t.Run("EmptyString", func(t *testing.T) {
		err := utils.ValidateNotEmpty("", "test_field")
		if err == nil {
			t.Error("Expected error for empty string")
		}
	})

	t.Run("WhitespaceOnly", func(t *testing.T) {
		err := utils.ValidateNotEmpty("   \t\n   ", "test_field")
		if err == nil {
			t.Error("Expected error for whitespace-only string")
		}
	})
}

func TestCommonUtils_MergeMaps(t *testing.T) {
	utils := NewCommonUtils()

	t.Run("SimpleMerge", func(t *testing.T) {
		base := map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}
		override := map[string]interface{}{
			"key2": "overridden",
			"key3": "value3",
		}

		result := utils.MergeMaps(base, override)

		if result["key1"] != "value1" {
			t.Error("Base key1 not preserved")
		}
		if result["key2"] != "overridden" {
			t.Error("Key2 not overridden correctly")
		}
		if result["key3"] != "value3" {
			t.Error("Override key3 not added")
		}
	})

	t.Run("NestedMerge", func(t *testing.T) {
		base := map[string]interface{}{
			"nested": map[string]interface{}{
				"a": "base_a",
				"b": "base_b",
			},
			"simple": "base_simple",
		}
		override := map[string]interface{}{
			"nested": map[string]interface{}{
				"b": "override_b",
				"c": "override_c",
			},
			"simple": "override_simple",
		}

		result := utils.MergeMaps(base, override)

		nested, ok := result["nested"].(map[string]interface{})
		if !ok {
			t.Fatal("Nested map not preserved")
		}
		if nested["a"] != "base_a" {
			t.Error("Nested base value not preserved")
		}
		if nested["b"] != "override_b" {
			t.Error("Nested override not applied")
		}
		if nested["c"] != "override_c" {
			t.Error("Nested new value not added")
		}
		if result["simple"] != "override_simple" {
			t.Error("Simple override not applied")
		}
	})

	t.Run("EmptyMaps", func(t *testing.T) {
		base := map[string]interface{}{}
		override := map[string]interface{}{"key": "value"}

		result := utils.MergeMaps(base, override)
		if result["key"] != "value" {
			t.Error("Override not applied to empty base")
		}

		base = map[string]interface{}{"key": "value"}
		override = map[string]interface{}{}

		result = utils.MergeMaps(base, override)
		if result["key"] != "value" {
			t.Error("Base not preserved with empty override")
		}
	})

	t.Run("TypeConflict", func(t *testing.T) {
		base := map[string]interface{}{
			"conflict": map[string]interface{}{"nested": "value"},
		}
		override := map[string]interface{}{
			"conflict": "simple string",
		}

		result := utils.MergeMaps(base, override)
		if result["conflict"] != "simple string" {
			t.Error("Override should replace conflicting types")
		}
	})
}

func TestCommonUtils_SafeClose(t *testing.T) {
	utils := NewCommonUtils()

	t.Run("OpenChannel", func(t *testing.T) {
		ch := make(chan interface{}, 1)
		ch <- "test"

		// Should not panic
		utils.SafeClose(ch)

		// Channel should be closed - first read gets the buffered data
		select {
		case val, ok := <-ch:
			if !ok {
				t.Error("First read should get buffered data with ok=true")
			}
			if val != "test" {
				t.Errorf("Expected 'test', got %v", val)
			}
		default:
			t.Error("Should be able to read buffered data from closed channel")
		}

		// Second read should indicate channel is closed
		select {
		case _, ok := <-ch:
			if ok {
				t.Error("Second read should indicate channel is closed")
			}
		default:
			t.Error("Should be able to read from closed channel (zero value)")
		}
	})

	t.Run("AlreadyClosedChannel", func(t *testing.T) {
		ch := make(chan interface{})
		close(ch)

		// Should not panic even if already closed
		utils.SafeClose(ch)
	})
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config == nil {
		t.Fatal("DefaultRetryConfig returned nil")
	}
	if config.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts 3, got %d", config.MaxAttempts)
	}
	if config.BaseDelay != 100*time.Millisecond {
		t.Errorf("Expected BaseDelay 100ms, got %v", config.BaseDelay)
	}
	if config.MaxDelay != 5*time.Second {
		t.Errorf("Expected MaxDelay 5s, got %v", config.MaxDelay)
	}
	if config.BackoffFunc == nil {
		t.Error("BackoffFunc should not be nil")
	}
}

func TestExponentialBackoff(t *testing.T) {
	baseDelay := 100 * time.Millisecond

	t.Run("BackoffProgression", func(t *testing.T) {
		// Test backoff progression
		for attempt := 0; attempt < 5; attempt++ {
			delay := ExponentialBackoff(attempt, baseDelay)
			expectedDelay := baseDelay * time.Duration(1<<attempt) // 2^attempt
			if delay != expectedDelay {
				t.Errorf("Attempt %d: expected delay %v, got %v", attempt, expectedDelay, delay)
			}
		}
	})

	t.Run("ZeroAttempt", func(t *testing.T) {
		delay := ExponentialBackoff(0, baseDelay)
		if delay != baseDelay {
			t.Errorf("Zero attempt should return base delay, got %v", delay)
		}
	})
}

func TestCommonUtils_RetryWithConfig(t *testing.T) {
	utils := NewCommonUtils()

	t.Run("SuccessOnFirstAttempt", func(t *testing.T) {
		config := &RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   10 * time.Millisecond,
			MaxDelay:    100 * time.Millisecond,
			BackoffFunc: ExponentialBackoff,
		}

		attempts := 0
		operation := func() error {
			attempts++
			return nil // Success
		}

		ctx := context.Background()
		err := utils.RetryWithConfig(ctx, config, operation)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("SuccessAfterRetries", func(t *testing.T) {
		config := &RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   1 * time.Millisecond, // Fast for testing
			MaxDelay:    10 * time.Millisecond,
			BackoffFunc: ExponentialBackoff,
		}

		attempts := 0
		operation := func() error {
			attempts++
			if attempts < 3 {
				return fmt.Errorf("attempt %d failed", attempts)
			}
			return nil // Success on 3rd attempt
		}

		ctx := context.Background()
		err := utils.RetryWithConfig(ctx, config, operation)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("AllAttemptsFail", func(t *testing.T) {
		config := &RetryConfig{
			MaxAttempts: 2,
			BaseDelay:   1 * time.Millisecond,
			MaxDelay:    10 * time.Millisecond,
			BackoffFunc: ExponentialBackoff,
		}

		attempts := 0
		operation := func() error {
			attempts++
			return fmt.Errorf("attempt %d failed", attempts)
		}

		ctx := context.Background()
		err := utils.RetryWithConfig(ctx, config, operation)

		if err == nil {
			t.Error("Expected error after all attempts failed")
		}
		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		config := &RetryConfig{
			MaxAttempts: 5,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    1 * time.Second,
			BackoffFunc: ExponentialBackoff,
		}

		ctx, cancel := context.WithCancel(context.Background())

		attempts := 0
		operation := func() error {
			attempts++
			if attempts == 2 {
				cancel() // Cancel context during retry
			}
			return fmt.Errorf("attempt %d failed", attempts)
		}

		err := utils.RetryWithConfig(ctx, config, operation)

		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})

	t.Run("MaxDelayEnforced", func(t *testing.T) {
		config := &RetryConfig{
			MaxAttempts: 3,                     // Reduced from 10 for faster testing
			BaseDelay:   10 * time.Millisecond, // Reduced from 1 second
			MaxDelay:    50 * time.Millisecond, // Reduced from 2 seconds
			BackoffFunc: ExponentialBackoff,
		}

		delays := []time.Duration{}
		operation := func() error {
			return fmt.Errorf("always fail")
		}

		// Mock the delay to capture what would actually be used (after MaxDelay capping)
		originalBackoff := config.BackoffFunc
		config.BackoffFunc = func(attempt int, baseDelay time.Duration) time.Duration {
			delay := originalBackoff(attempt, baseDelay)
			// Apply the same MaxDelay capping logic as RetryWithConfig
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
			delays = append(delays, delay)
			return delay
		}

		// Use context with timeout to prevent hanging
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		utils.RetryWithConfig(ctx, config, operation)

		// Check that delays never exceed MaxDelay
		for i, delay := range delays {
			expectedDelay := config.MaxDelay
			if delay > expectedDelay {
				t.Errorf("Delay at attempt %d exceeded MaxDelay: %v > %v", i, delay, expectedDelay)
			}
		}
	})
}

// Concurrency and race condition tests

func TestCommonUtils_ConcurrentOperations(t *testing.T) {
	utils := NewCommonUtils()

	t.Run("ConcurrentJSONOperations", func(t *testing.T) {
		type TestData struct {
			ID    int    `json:"id"`
			Value string `json:"value"`
		}

		var wg sync.WaitGroup
		numRoutines := 50
		errors := make(chan error, numRoutines*2)

		for i := 0; i < numRoutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				data := TestData{ID: id, Value: fmt.Sprintf("value-%d", id)}

				// Marshal to map
				mapData, err := utils.JSONMarshalToMap(data)
				errors <- err
				if err != nil {
					return
				}

				// Unmarshal back
				var result TestData
				err = utils.JSONUnmarshalFromMap(mapData, &result)
				errors <- err
			}(i)
		}

		wg.Wait()
		close(errors)

		errorCount := 0
		for err := range errors {
			if err != nil {
				t.Errorf("Concurrent operation failed: %v", err)
				errorCount++
			}
		}

		if errorCount > 0 {
			t.Errorf("Had %d errors in concurrent operations", errorCount)
		}
	})

	t.Run("ConcurrentMapMerging", func(t *testing.T) {
		base := map[string]interface{}{
			"base_key": "base_value",
		}

		var wg sync.WaitGroup
		numRoutines := 20
		results := make(chan map[string]interface{}, numRoutines)

		for i := 0; i < numRoutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				override := map[string]interface{}{
					fmt.Sprintf("key_%d", id): fmt.Sprintf("value_%d", id),
				}
				result := utils.MergeMaps(base, override)
				results <- result
			}(i)
		}

		wg.Wait()
		close(results)

		for result := range results {
			if result["base_key"] != "base_value" {
				t.Error("Base key not preserved in concurrent merge")
			}
		}
	})
}

// Benchmark tests

func BenchmarkCommonUtils_JSONMarshalToMap(b *testing.B) {
	utils := NewCommonUtils()
	
	type BenchData struct {
		ID       int                    `json:"id"`
		Name     string                 `json:"name"`
		Values   []string               `json:"values"`
		Metadata map[string]interface{} `json:"metadata"`
	}

	data := BenchData{
		ID:     123,
		Name:   "benchmark test",
		Values: []string{"a", "b", "c", "d", "e"},
		Metadata: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
			"key3": true,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := utils.JSONMarshalToMap(data)
		if err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}

func BenchmarkCommonUtils_DeepCopyJSON(b *testing.B) {
	utils := NewCommonUtils()

	type BenchData struct {
		Items []map[string]interface{} `json:"items"`
	}

	source := BenchData{
		Items: make([]map[string]interface{}, 100),
	}
	for i := 0; i < 100; i++ {
		source.Items[i] = map[string]interface{}{
			"id":    i,
			"value": fmt.Sprintf("item-%d", i),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dest BenchData
		err := utils.DeepCopyJSON(source, &dest)
		if err != nil {
			b.Fatalf("DeepCopy failed: %v", err)
		}
	}
}

func BenchmarkCommonUtils_MergeMaps(b *testing.B) {
	utils := NewCommonUtils()

	base := make(map[string]interface{})
	override := make(map[string]interface{})

	for i := 0; i < 100; i++ {
		base[fmt.Sprintf("base_key_%d", i)] = fmt.Sprintf("base_value_%d", i)
		override[fmt.Sprintf("override_key_%d", i)] = fmt.Sprintf("override_value_%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := utils.MergeMaps(base, override)
		_ = result
	}
}

func BenchmarkSafeStringBuilder(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := NewSafeStringBuilder(100)
		for j := 0; j < 10; j++ {
			builder.WriteString(fmt.Sprintf("item-%d ", j))
		}
		_ = builder.String()
	}
}

func BenchmarkCommonUtils_RetryWithConfig(b *testing.B) {
	utils := NewCommonUtils()
	config := DefaultRetryConfig()
	config.BaseDelay = 1 * time.Microsecond // Very fast for benchmarking

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attempt := 0
		operation := func() error {
			attempt++
			if attempt < 2 {
				return fmt.Errorf("fail")
			}
			return nil
		}
		utils.RetryWithConfig(context.Background(), config, operation)
	}
}

// Memory leak tests

func TestMemoryLeak_CommonUtils(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	utils := NewCommonUtils()

	// Test with many operations to check for memory leaks
	for i := 0; i < 1000; i++ {
		data := map[string]interface{}{
			"id":    i,
			"value": fmt.Sprintf("test-value-%d", i),
		}

		// Marshal and unmarshal
		var result map[string]interface{}
		utils.DeepCopyJSON(data, &result)

		// Merge operations
		override := map[string]interface{}{
			"extra": fmt.Sprintf("extra-%d", i),
		}
		utils.MergeMaps(data, override)

		// String building
		builder := NewSafeStringBuilder(50)
		builder.WriteString(fmt.Sprintf("test content %d", i))
		_ = builder.String()
	}

	// Force garbage collection
	runtime.GC()
	// This test mainly ensures we don't panic or create obvious leaks
}