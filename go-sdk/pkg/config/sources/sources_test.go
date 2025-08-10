package sources

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestEnvSource(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_HOST", "localhost")
	os.Setenv("TEST_PORT", "8080")
	os.Setenv("TEST_DEBUG", "true")
	os.Setenv("TEST_SERVERS", "server1,server2,server3")
	os.Setenv("TEST_NESTED_VALUE", "nested_test")
	
	defer func() {
		os.Unsetenv("TEST_HOST")
		os.Unsetenv("TEST_PORT")
		os.Unsetenv("TEST_DEBUG")
		os.Unsetenv("TEST_SERVERS")
		os.Unsetenv("TEST_NESTED_VALUE")
	}()
	
	source := NewEnvSource("TEST")
	
	if source.Name() != "env:TEST" {
		t.Errorf("Expected name 'env:TEST', got '%s'", source.Name())
	}
	
	if source.Priority() != 10 {
		t.Errorf("Expected priority 10, got %d", source.Priority())
	}
	
	if source.CanWatch() {
		t.Error("EnvSource should not support watching")
	}
	
	// Load configuration
	ctx := context.Background()
	config, err := source.Load(ctx)
	if err != nil {
		t.Errorf("Load failed: %v", err)
	}
	
	// Verify string value
	if config["host"] != "localhost" {
		t.Errorf("Expected 'localhost', got '%v'", config["host"])
	}
	
	// Verify integer parsing
	if config["port"] != 8080 {
		t.Errorf("Expected 8080, got '%v'", config["port"])
	}
	
	// Verify boolean parsing
	if config["debug"] != true {
		t.Errorf("Expected true, got '%v'", config["debug"])
	}
	
	// Verify array parsing
	servers, ok := config["servers"].([]string)
	if !ok || len(servers) != 3 || servers[0] != "server1" {
		t.Errorf("Expected []string{\"server1\", \"server2\", \"server3\"}, got '%v'", config["servers"])
	}
	
	// Verify nested value
	nested, ok := config["nested"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected nested map, got '%T'", config["nested"])
	} else if nested["value"] != "nested_test" {
		t.Errorf("Expected 'nested_test', got '%v'", nested["value"])
	}
}

func TestEnvSourceWithCustomOptions(t *testing.T) {
	os.Setenv("CUSTOM_TEST_VALUE", "custom_value")
	defer os.Unsetenv("CUSTOM_TEST_VALUE")
	
	options := &EnvSourceOptions{
		Prefix:    "CUSTOM",
		Separator: "_",
		Priority:  20,
		KeyMapping: map[string]string{
			"test_value": "mapped.value",
		},
		Transformer: func(key string) string {
			return "transformed." + key
		},
	}
	
	source := NewEnvSourceWithOptions(options)
	
	if source.Priority() != 20 {
		t.Errorf("Expected priority 20, got %d", source.Priority())
	}
	
	ctx := context.Background()
	config, err := source.Load(ctx)
	if err != nil {
		t.Errorf("Load failed: %v", err)
	}
	
	// Check that transformer was applied
	if _, exists := config["transformed"]; !exists {
		t.Error("Transformer should have created 'transformed' key")
	}
}

func TestProgrammaticSource(t *testing.T) {
	source := NewProgrammaticSource()
	
	if source.Name() != "programmatic:programmatic" {
		t.Errorf("Expected name 'programmatic:programmatic', got '%s'", source.Name())
	}
	
	if source.Priority() != 40 {
		t.Errorf("Expected priority 40, got %d", source.Priority())
	}
	
	// Test Set and Get
	err := source.Set("test.key", "test_value")
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}
	
	value := source.Get("test.key")
	if value != "test_value" {
		t.Errorf("Expected 'test_value', got '%v'", value)
	}
	
	// Test nested values
	err = source.Set("nested.deep.value", 42)
	if err != nil {
		t.Errorf("Set nested value failed: %v", err)
	}
	
	nestedValue := source.Get("nested.deep.value")
	if nestedValue != 42 {
		t.Errorf("Expected 42, got '%v'", nestedValue)
	}
	
	// Test Load
	ctx := context.Background()
	config, err := source.Load(ctx)
	if err != nil {
		t.Errorf("Load failed: %v", err)
	}
	
	if config["test"].(map[string]interface{})["key"] != "test_value" {
		t.Error("Load should return set values")
	}
	
	// Test Delete
	err = source.Delete("test.key")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}
	
	value = source.Get("test.key")
	if value != nil {
		t.Error("Value should be nil after deletion")
	}
	
	// Test Clear
	source.Clear()
	all := source.GetAll()
	if len(all) != 0 {
		t.Error("GetAll should return empty map after Clear")
	}
}

func TestProgrammaticSourceWithOptions(t *testing.T) {
	initialData := map[string]interface{}{
		"initial.key": "initial_value",
		"number":      42,
	}
	
	options := &ProgrammaticSourceOptions{
		Name:        "custom-source",
		Priority:    50,
		InitialData: initialData,
	}
	
	source := NewProgrammaticSourceWithOptions(options)
	
	if source.Name() != "programmatic:custom-source" {
		t.Errorf("Expected name 'programmatic:custom-source', got '%s'", source.Name())
	}
	
	if source.Priority() != 50 {
		t.Errorf("Expected priority 50, got %d", source.Priority())
	}
	
	// Check initial data - the key was stored as a flat key, not nested
	// So we need to access it directly
	all := source.GetAll()
	if val, ok := all["initial.key"]; !ok || val != "initial_value" {
		t.Errorf("Expected 'initial_value' for key 'initial.key', got '%v'", val)
	}
	
	if val, ok := all["number"]; !ok || val != 42 {
		t.Errorf("Expected 42 for key 'number', got '%v'", val)
	}
}

func TestProgrammaticSourceClone(t *testing.T) {
	source := NewProgrammaticSource()
	source.Set("original.key", "original_value")
	
	// Test SetAll (which is used for cloning)
	newData := map[string]interface{}{
		"new": map[string]interface{}{
			"key": "new_value",
		},
	}
	
	source.SetAll(newData)
	
	// Original key should be gone
	if source.Get("original.key") != nil {
		t.Error("Original key should be gone after SetAll")
	}
	
	// New key should be present
	if source.Get("new.key") != "new_value" {
		t.Error("New key should be present after SetAll")
	}
}

func TestProgrammaticSourceDeepCopy(t *testing.T) {
	source := NewProgrammaticSource()
	
	// Set complex nested data
	nestedData := map[string]interface{}{
		"array": []interface{}{
			map[string]interface{}{"item": "value1"},
			map[string]interface{}{"item": "value2"},
		},
		"nested": map[string]interface{}{
			"deep": map[string]interface{}{
				"value": "deep_value",
			},
		},
	}
	
	source.SetAll(nestedData)
	
	// Get all data (which should be deep copied)
	all := source.GetAll()
	
	// Modify the returned data
	if nestedMap, ok := all["nested"].(map[string]interface{}); ok {
		if deepMap, ok := nestedMap["deep"].(map[string]interface{}); ok {
			deepMap["value"] = "modified"
		}
	}
	
	// Original should not be affected
	originalValue := source.Get("nested.deep.value")
	if originalValue != "deep_value" {
		t.Error("Original value should not be affected by modifications to returned copy")
	}
}

func TestProgrammaticSourceLastModified(t *testing.T) {
	source := NewProgrammaticSource()
	
	initialTime := source.LastModified()
	time.Sleep(time.Millisecond * 2) // Ensure different timestamp
	
	source.Set("test.key", "test_value")
	
	modifiedTime := source.LastModified()
	if !modifiedTime.After(initialTime) {
		t.Error("LastModified should be updated after Set operation")
	}
}

func TestFlagSource(t *testing.T) {
	// Note: Testing FlagSource is tricky because it depends on command-line flags
	// In a real application, you'd typically test this with a custom FlagSet
	source := NewFlagSource()
	
	if source.Name() != "flags" {
		t.Errorf("Expected name 'flags', got '%s'", source.Name())
	}
	
	if source.Priority() != 30 {
		t.Errorf("Expected priority 30, got %d", source.Priority())
	}
	
	if source.CanWatch() {
		t.Error("FlagSource should not support watching")
	}
	
	// Test Load (will be empty since no flags are set in test)
	ctx := context.Background()
	config, err := source.Load(ctx)
	if err != nil {
		t.Errorf("Load failed: %v", err)
	}
	
	// Should be empty in test environment
	if len(config) > 0 {
		t.Logf("Unexpected config entries: %v", config)
		// Don't fail, as some test environments might have flags
	}
}

func BenchmarkEnvSourceLoad(b *testing.B) {
	// Set up test environment variables
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("BENCH_TEST_%d", i)
		os.Setenv(key, fmt.Sprintf("value_%d", i))
	}
	
	source := NewEnvSource("BENCH")
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source.Load(ctx)
	}
	
	b.StopTimer()
	// Clean up
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("BENCH_TEST_%d", i)
		os.Unsetenv(key)
	}
}

func BenchmarkProgrammaticSourceSet(b *testing.B) {
	source := NewProgrammaticSource()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source.Set(fmt.Sprintf("bench.key.%d", i), fmt.Sprintf("value_%d", i))
	}
}

func BenchmarkProgrammaticSourceGet(b *testing.B) {
	source := NewProgrammaticSource()
	source.Set("bench.key", "bench_value")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source.Get("bench.key")
	}
}

func BenchmarkProgrammaticSourceLoad(b *testing.B) {
	source := NewProgrammaticSource()
	
	// Set up test data
	for i := 0; i < 100; i++ {
		source.Set(fmt.Sprintf("bench.key.%d", i), fmt.Sprintf("value_%d", i))
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source.Load(ctx)
	}
}