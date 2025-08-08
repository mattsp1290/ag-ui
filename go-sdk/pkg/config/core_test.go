package config

import (
	"strings"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/config/sources"
)

func TestNewConfig(t *testing.T) {
	config := NewConfig()
	
	if config == nil {
		t.Fatal("NewConfig returned nil")
	}
	
	if config.data == nil {
		t.Error("Config data map not initialized")
	}
	
	if config.watchers == nil {
		t.Error("Config watchers map not initialized")
	}
	
	if config.defaults == nil {
		t.Error("Config defaults map not initialized")
	}
	
	if config.keyDelimiter != "." {
		t.Error("Default key delimiter not set correctly")
	}
}

func TestConfigBuilder(t *testing.T) {
	builder := NewConfigBuilder()
	
	if builder == nil {
		t.Fatal("NewConfigBuilder returned nil")
	}
	
	if builder.config == nil {
		t.Error("Builder config not initialized")
	}
	
	if builder.options == nil {
		t.Error("Builder options not initialized")
	}
	
	// Test method chaining
	result := builder.
		WithProfile("test").
		EnableHotReload().
		DisableValidation().
		WithMergeStrategy(MergeStrategyOverride)
	
	if result != builder {
		t.Error("Builder methods should return self for chaining")
	}
	
	if builder.profile != "test" {
		t.Error("Profile not set correctly")
	}
	
	if !builder.options.EnableHotReload {
		t.Error("Hot reload not enabled")
	}
	
	if builder.options.ValidateOnBuild {
		t.Error("Validation should be disabled")
	}
	
	if builder.options.MergeStrategy != MergeStrategyOverride {
		t.Error("Merge strategy not set correctly")
	}
}

func TestConfigBasicOperations(t *testing.T) {
	config := NewConfig()
	
	// Test Set and Get
	err := config.Set("test.key", "test_value")
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}
	
	value := config.Get("test.key")
	if value != "test_value" {
		t.Errorf("Expected 'test_value', got '%v'", value)
	}
	
	// Test GetString
	str := config.GetString("test.key")
	if str != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", str)
	}
	
	// Test IsSet
	if !config.IsSet("test.key") {
		t.Error("IsSet should return true for existing key")
	}
	
	if config.IsSet("nonexistent.key") {
		t.Error("IsSet should return false for nonexistent key")
	}
}

func TestConfigTypeConversions(t *testing.T) {
	config := NewConfig()
	
	// Test integer operations
	config.Set("int_key", 42)
	if config.GetInt("int_key") != 42 {
		t.Error("GetInt failed for integer value")
	}
	
	config.Set("int64_key", int64(1234567890))
	if config.GetInt64("int64_key") != 1234567890 {
		t.Error("GetInt64 failed for int64 value")
	}
	
	// Test float operations
	config.Set("float_key", 3.14)
	if config.GetFloat64("float_key") != 3.14 {
		t.Error("GetFloat64 failed for float value")
	}
	
	// Test boolean operations
	config.Set("bool_key", true)
	if !config.GetBool("bool_key") {
		t.Error("GetBool failed for boolean value")
	}
	
	// Test duration operations
	duration := time.Minute * 5
	config.Set("duration_key", duration)
	if config.GetDuration("duration_key") != duration {
		t.Error("GetDuration failed for duration value")
	}
	
	config.Set("duration_string", "30s")
	expected := time.Second * 30
	if config.GetDuration("duration_string") != expected {
		t.Error("GetDuration failed for duration string")
	}
}

func TestConfigNestedOperations(t *testing.T) {
	config := NewConfig()
	
	// Set nested values
	config.Set("server.host", "localhost")
	config.Set("server.port", 8080)
	config.Set("server.timeout.read", "30s")
	
	// Get nested values
	if config.GetString("server.host") != "localhost" {
		t.Error("Failed to get nested string value")
	}
	
	if config.GetInt("server.port") != 8080 {
		t.Error("Failed to get nested int value")
	}
	
	if config.GetDuration("server.timeout.read") != time.Second*30 {
		t.Error("Failed to get nested duration value")
	}
}

func TestConfigSliceOperations(t *testing.T) {
	config := NewConfig()
	
	// Test string slice
	stringSlice := []string{"item1", "item2", "item3"}
	config.Set("string_slice", stringSlice)
	
	result := config.GetStringSlice("string_slice")
	if len(result) != len(stringSlice) {
		t.Errorf("Expected slice length %d, got %d", len(stringSlice), len(result))
	}
	
	for i, expected := range stringSlice {
		if result[i] != expected {
			t.Errorf("Expected %s at index %d, got %s", expected, i, result[i])
		}
	}
	
	// Test generic slice
	genericSlice := []interface{}{"item1", 42, true}
	config.Set("generic_slice", genericSlice)
	
	resultGeneric := config.GetSlice("generic_slice")
	if len(resultGeneric) != len(genericSlice) {
		t.Errorf("Expected slice length %d, got %d", len(genericSlice), len(resultGeneric))
	}
}

func TestConfigMapOperations(t *testing.T) {
	config := NewConfig()
	
	// Test string map
	stringMap := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	config.Set("string_map", stringMap)
	
	result := config.GetStringMapString("string_map")
	if len(result) != len(stringMap) {
		t.Errorf("Expected map length %d, got %d", len(stringMap), len(result))
	}
	
	for key, expected := range stringMap {
		if result[key] != expected {
			t.Errorf("Expected %s for key %s, got %s", expected, key, result[key])
		}
	}
	
	// Test generic string map
	genericMap := map[string]interface{}{
		"string_key": "string_value",
		"int_key":    42,
		"bool_key":   true,
	}
	config.Set("generic_map", genericMap)
	
	resultGeneric := config.GetStringMap("generic_map")
	if len(resultGeneric) != len(genericMap) {
		t.Errorf("Expected map length %d, got %d", len(genericMap), len(resultGeneric))
	}
}

func TestConfigAllOperations(t *testing.T) {
	config := NewConfig()
	
	// Set multiple values
	config.Set("key1", "value1")
	config.Set("key2", 42)
	config.Set("nested.key", "nested_value")
	
	// Test AllKeys
	keys := config.AllKeys()
	expectedKeys := []string{"key1", "key2", "nested.key"}
	
	if len(keys) != len(expectedKeys) {
		t.Errorf("Expected %d keys, got %d", len(expectedKeys), len(keys))
	}
	
	// Test AllSettings
	settings := config.AllSettings()
	if len(settings) == 0 {
		t.Error("AllSettings should not be empty")
	}
	
	if settings["key1"] != "value1" {
		t.Error("AllSettings missing key1")
	}
	
	if settings["key2"] != 42 {
		t.Error("AllSettings missing key2")
	}
}

func TestConfigWatchers(t *testing.T) {
	config := NewConfig()
	
	called := false
	var receivedValue interface{}
	
	// Add watcher
	config.Watch("test.key", func(value interface{}) {
		called = true
		receivedValue = value
	})
	
	// Set value to trigger watcher
	config.Set("test.key", "test_value")
	
	// Give watcher goroutine time to execute
	time.Sleep(time.Millisecond * 10)
	
	if !called {
		t.Error("Watcher was not called")
	}
	
	if receivedValue != "test_value" {
		t.Errorf("Expected 'test_value', got '%v'", receivedValue)
	}
}

func TestConfigClone(t *testing.T) {
	original := NewConfig()
	original.Set("key1", "value1")
	original.Set("nested.key", "nested_value")
	
	clone := original.Clone()
	
	// Verify clone has same values
	if clone.GetString("key1") != "value1" {
		t.Error("Clone missing key1")
	}
	
	if clone.GetString("nested.key") != "nested_value" {
		t.Error("Clone missing nested.key")
	}
	
	// Verify independence
	clone.Set("key1", "modified_value")
	
	if original.GetString("key1") != "value1" {
		t.Error("Original should not be modified by clone changes")
	}
	
	if clone.GetString("key1") != "modified_value" {
		t.Error("Clone should have modified value")
	}
}

func TestConfigMerge(t *testing.T) {
	config1 := NewConfig()
	config1.Set("key1", "value1")
	config1.Set("shared.key", "original")
	
	config2 := NewConfig()
	config2.Set("key2", "value2")
	config2.Set("shared.key", "overridden")
	
	err := config1.Merge(config2)
	if err != nil {
		t.Errorf("Merge failed: %v", err)
	}
	
	// Verify merged values
	if config1.GetString("key1") != "value1" {
		t.Error("Original key1 should be preserved")
	}
	
	if config1.GetString("key2") != "value2" {
		t.Error("New key2 should be added")
	}
	
	if config1.GetString("shared.key") != "overridden" {
		t.Error("Shared key should be overridden")
	}
}

func TestConfigProfile(t *testing.T) {
	config := NewConfig()
	
	// Test profile operations
	if config.GetProfile() != "" {
		t.Error("Default profile should be empty")
	}
	
	err := config.SetProfile("development")
	if err != nil {
		t.Errorf("SetProfile failed: %v", err)
	}
	
	if config.GetProfile() != "development" {
		t.Error("Profile not set correctly")
	}
}

func TestConfigBuilderWithSources(t *testing.T) {
	// Create a programmatic source with test data
	progSource := sources.NewProgrammaticSource()
	progSource.Set("test.key", "test_value")
	progSource.Set("server.port", 8080)
	
	// Build configuration
	builder := NewConfigBuilder()
	cfg, err := builder.
		AddSource(progSource).
		DisableValidation().
		Build()
	
	if err != nil {
		t.Errorf("Build failed: %v", err)
	}
	
	// Verify loaded values
	if cfg.GetString("test.key") != "test_value" {
		t.Error("Failed to load from programmatic source")
	}
	
	if cfg.GetInt("server.port") != 8080 {
		t.Error("Failed to load int value from source")
	}
}

func TestConfigBuilderWithValidator(t *testing.T) {
	// Create a custom validator
	validator := NewCustomValidator("test-validator")
	validator.AddRule("required.field", RequiredRule)
	
	progSource := sources.NewProgrammaticSource()
	progSource.Set("required.field", "present")
	
	builder := NewConfigBuilder()
	_, err := builder.
		AddSource(progSource).
		AddValidator(validator).
		Build()
	
	if err != nil {
		t.Errorf("Build with valid data should succeed: %v", err)
	}
	
	// Test with invalid data
	progSource2 := sources.NewProgrammaticSource()
	// Don't set required.field
	
	builder2 := NewConfigBuilder()
	_, err = builder2.
		AddSource(progSource2).
		AddValidator(validator).
		Build()
	
	if err == nil {
		t.Error("Build with invalid data should fail")
	}
	
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("Error should mention required field: %v", err)
	}
}

func TestConfigBuilderWithMultipleSources(t *testing.T) {
	// Create multiple sources with different priorities
	source1 := sources.NewProgrammaticSourceWithOptions(&sources.ProgrammaticSourceOptions{
		Priority: 10,
		InitialData: map[string]interface{}{
			"key1":    "value1_from_source1",
			"unique1": "only_in_source1",
		},
	})
	
	source2 := sources.NewProgrammaticSourceWithOptions(&sources.ProgrammaticSourceOptions{
		Priority: 20, // Higher priority
		InitialData: map[string]interface{}{
			"key1":    "value1_from_source2", // Should override source1
			"unique2": "only_in_source2",
		},
	})
	
	builder := NewConfigBuilder()
	config, err := builder.
		AddSource(source1).
		AddSource(source2).
		DisableValidation().
		Build()
	
	if err != nil {
		t.Errorf("Build failed: %v", err)
	}
	
	// Source2 has higher priority, so its values should win
	if config.GetString("key1") != "value1_from_source2" {
		t.Errorf("Expected value from source2, got: %s", config.GetString("key1"))
	}
	
	// Unique values should be preserved from both sources
	if config.GetString("unique1") != "only_in_source1" {
		t.Error("Unique value from source1 should be preserved")
	}
	
	if config.GetString("unique2") != "only_in_source2" {
		t.Error("Unique value from source2 should be preserved")
	}
}

func TestConfigString(t *testing.T) {
	config := NewConfig()
	config.Set("key1", "value1")
	config.Set("nested.key", map[string]interface{}{
		"subkey": "subvalue",
	})
	
	str := config.String()
	
	if !strings.Contains(str, "key1") {
		t.Error("String representation should contain key1")
	}
	
	if !strings.Contains(str, "value1") {
		t.Error("String representation should contain value1")
	}
	
	if !strings.Contains(str, "nested") {
		t.Error("String representation should contain nested structure")
	}
}

func BenchmarkConfigGet(b *testing.B) {
	config := NewConfig()
	config.Set("benchmark.key", "benchmark_value")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.Get("benchmark.key")
	}
}

func BenchmarkConfigSet(b *testing.B) {
	config := NewConfig()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.Set("benchmark.key", i)
	}
}

func BenchmarkConfigNestedGet(b *testing.B) {
	config := NewConfig()
	config.Set("deeply.nested.benchmark.key", "benchmark_value")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.Get("deeply.nested.benchmark.key")
	}
}