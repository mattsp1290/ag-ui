package config

import (
	"strings"
	"testing"
)

// TestProfileConditionImportExport tests the complete profile condition import/export functionality
func TestProfileConditionImportExport(t *testing.T) {
	config := NewConfig()
	pm := NewProfileManager(config)

	// Create test profile data with conditions
	profileData := map[string]interface{}{
		"name":        "test-profile",
		"environment": "test",
		"description": "Test profile with conditions",
		"enabled":     true,
		"config": map[string]interface{}{
			"server.port": 3000,
			"debug":       true,
		},
		"conditions": []interface{}{
			map[string]interface{}{
				"type":      "env",
				"key":       "NODE_ENV",
				"value":     "test",
				"operation": "equals",
			},
			map[string]interface{}{
				"type":      "config",
				"key":       "debug.enabled",
				"value":     true,
				"operation": "equals",
			},
			map[string]interface{}{
				"type":      "system",
				"key":       "hostname",
				"value":     "test-host",
				"operation": "contains",
			},
		},
	}

	// Test ImportProfile with conditions
	err := pm.ImportProfile(profileData)
	if err != nil {
		t.Fatalf("ImportProfile failed: %v", err)
	}

	// Verify profile was imported correctly
	profile, exists := pm.GetProfile("test-profile")
	if !exists {
		t.Fatal("Profile should exist after import")
	}

	// Verify basic profile fields
	if profile.Name != "test-profile" {
		t.Errorf("Expected profile name 'test-profile', got '%s'", profile.Name)
	}
	if profile.Environment != "test" {
		t.Errorf("Expected environment 'test', got '%s'", profile.Environment)
	}
	if !profile.Enabled {
		t.Error("Profile should be enabled")
	}

	// Verify conditions were imported correctly
	if len(profile.Conditions) != 3 {
		t.Fatalf("Expected 3 conditions, got %d", len(profile.Conditions))
	}

	// Check first condition (env type)
	cond1 := profile.Conditions[0]
	if cond1.Type != "env" {
		t.Errorf("Expected condition type 'env', got '%s'", cond1.Type)
	}
	if cond1.Key != "NODE_ENV" {
		t.Errorf("Expected condition key 'NODE_ENV', got '%s'", cond1.Key)
	}
	if cond1.Value != "test" {
		t.Errorf("Expected condition value 'test', got '%v'", cond1.Value)
	}
	if cond1.Operation != "equals" {
		t.Errorf("Expected condition operation 'equals', got '%s'", cond1.Operation)
	}

	// Check second condition (config type)
	cond2 := profile.Conditions[1]
	if cond2.Type != "config" {
		t.Errorf("Expected condition type 'config', got '%s'", cond2.Type)
	}
	if cond2.Value != true {
		t.Errorf("Expected condition value true, got '%v'", cond2.Value)
	}

	// Check third condition (system type with different operation)
	cond3 := profile.Conditions[2]
	if cond3.Type != "system" {
		t.Errorf("Expected condition type 'system', got '%s'", cond3.Type)
	}
	if cond3.Operation != "contains" {
		t.Errorf("Expected condition operation 'contains', got '%s'", cond3.Operation)
	}

	// Test ExportProfile to ensure round-trip consistency
	exported, err := pm.ExportProfile("test-profile")
	if err != nil {
		t.Fatalf("ExportProfile failed: %v", err)
	}

	// Verify exported conditions
	exportedConditionsInterface, ok := exported["conditions"].([]interface{})
	if !ok {
		t.Fatal("Exported conditions should be of type []interface{}")
	}

	if len(exportedConditionsInterface) != 3 {
		t.Fatalf("Expected 3 exported conditions, got %d", len(exportedConditionsInterface))
	}

	// Verify exported condition details match original
	firstCondition := exportedConditionsInterface[0].(map[string]interface{})
	if firstCondition["type"] != "env" {
		t.Error("Exported condition type mismatch")
	}
	if firstCondition["key"] != "NODE_ENV" {
		t.Error("Exported condition key mismatch")
	}
	if firstCondition["value"] != "test" {
		t.Error("Exported condition value mismatch")
	}
}

// TestProfileConditionImportWithDefaults tests condition import with default values
func TestProfileConditionImportWithDefaults(t *testing.T) {
	config := NewConfig()
	pm := NewProfileManager(config)

	// Create profile data with minimal condition (should use defaults)
	profileData := map[string]interface{}{
		"name":    "minimal-profile",
		"enabled": true,
		"conditions": []interface{}{
			map[string]interface{}{
				"type":  "env",
				"key":   "TEST_VAR",
				"value": "test_value",
				// operation omitted - should default to "equals"
			},
		},
	}

	err := pm.ImportProfile(profileData)
	if err != nil {
		t.Fatalf("ImportProfile failed: %v", err)
	}

	profile, _ := pm.GetProfile("minimal-profile")
	if len(profile.Conditions) != 1 {
		t.Fatal("Expected 1 condition")
	}

	condition := profile.Conditions[0]
	if condition.Operation != "equals" {
		t.Errorf("Expected default operation 'equals', got '%s'", condition.Operation)
	}
}

// TestProfileConditionImportValidationErrors tests validation errors during import
func TestProfileConditionImportValidationErrors(t *testing.T) {
	config := NewConfig()
	pm := NewProfileManager(config)

	testCases := []struct {
		name          string
		conditionData map[string]interface{}
		expectError   string
	}{
		{
			name: "missing type",
			conditionData: map[string]interface{}{
				"key":   "TEST_KEY",
				"value": "test_value",
			},
			expectError: "condition type is required",
		},
		{
			name: "missing key",
			conditionData: map[string]interface{}{
				"type":  "env",
				"value": "test_value",
			},
			expectError: "condition key is required",
		},
		{
			name: "missing value",
			conditionData: map[string]interface{}{
				"type": "env",
				"key":  "TEST_KEY",
			},
			expectError: "condition value is required",
		},
		{
			name: "invalid type",
			conditionData: map[string]interface{}{
				"type":  "invalid_type",
				"key":   "TEST_KEY",
				"value": "test_value",
			},
			expectError: "invalid condition type",
		},
		{
			name: "invalid operation",
			conditionData: map[string]interface{}{
				"type":      "env",
				"key":       "TEST_KEY",
				"value":     "test_value",
				"operation": "invalid_operation",
			},
			expectError: "invalid condition operation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			profileData := map[string]interface{}{
				"name":    "error-test-profile",
				"enabled": true,
				"conditions": []interface{}{
					tc.conditionData,
				},
			}

			err := pm.ImportProfile(profileData)
			if err == nil {
				t.Error("Expected error but got none")
				return
			}

			if !strings.Contains(err.Error(), tc.expectError) {
				t.Errorf("Expected error containing '%s', got: %v", tc.expectError, err)
			}
		})
	}
}

// TestProfileConditionImportComplexTypes tests importing conditions with complex value types
func TestProfileConditionImportComplexTypes(t *testing.T) {
	config := NewConfig()
	pm := NewProfileManager(config)

	profileData := map[string]interface{}{
		"name":    "complex-types-profile",
		"enabled": true,
		"conditions": []interface{}{
			map[string]interface{}{
				"type":      "env",
				"key":       "NUMBER_VAR",
				"value":     42,
				"operation": "equals",
			},
			map[string]interface{}{
				"type":      "config",
				"key":       "flag",
				"value":     true,
				"operation": "equals",
			},
			map[string]interface{}{
				"type":      "env",
				"key":       "FLOAT_VAR",
				"value":     3.14,
				"operation": "equals",
			},
		},
	}

	err := pm.ImportProfile(profileData)
	if err != nil {
		t.Fatalf("ImportProfile failed: %v", err)
	}

	profile, _ := pm.GetProfile("complex-types-profile")
	if len(profile.Conditions) != 3 {
		t.Fatal("Expected 3 conditions")
	}

	// Check that different value types are preserved
	if profile.Conditions[0].Value != 42 {
		t.Errorf("Expected integer value 42, got %v", profile.Conditions[0].Value)
	}
	if profile.Conditions[1].Value != true {
		t.Errorf("Expected boolean value true, got %v", profile.Conditions[1].Value)
	}
	if profile.Conditions[2].Value != 3.14 {
		t.Errorf("Expected float value 3.14, got %v", profile.Conditions[2].Value)
	}
}

// TestProfileConditionRoundTrip tests importing and then exporting profile conditions
func TestProfileConditionRoundTrip(t *testing.T) {
	config := NewConfig()
	pm := NewProfileManager(config)

	// Create original profile data
	originalData := map[string]interface{}{
		"name":        "roundtrip-profile",
		"environment": "test",
		"enabled":     true,
		"conditions": []interface{}{
			map[string]interface{}{
				"type":      "env",
				"key":       "TEST_ENV",
				"value":     "production",
				"operation": "equals",
			},
			map[string]interface{}{
				"type":      "config",
				"key":       "debug",
				"value":     false,
				"operation": "equals",
			},
		},
	}

	// Import the profile
	err := pm.ImportProfile(originalData)
	if err != nil {
		t.Fatalf("ImportProfile failed: %v", err)
	}

	// Export the profile
	exported, err := pm.ExportProfile("roundtrip-profile")
	if err != nil {
		t.Fatalf("ExportProfile failed: %v", err)
	}

	// Import the exported data into a new profile manager
	pm2 := NewProfileManager(config)
	exported["name"] = "roundtrip-profile-2" // Change name to avoid conflict
	err = pm2.ImportProfile(exported)
	if err != nil {
		t.Fatalf("ImportProfile from exported data failed: %v", err)
	}

	// Verify the round-trip worked correctly
	profile, exists := pm2.GetProfile("roundtrip-profile-2")
	if !exists {
		t.Fatal("Round-trip profile should exist")
	}

	if len(profile.Conditions) != 2 {
		t.Fatalf("Expected 2 conditions after round-trip, got %d", len(profile.Conditions))
	}

	// Verify first condition
	cond1 := profile.Conditions[0]
	if cond1.Type != "env" || cond1.Key != "TEST_ENV" || cond1.Value != "production" || cond1.Operation != "equals" {
		t.Errorf("First condition not preserved correctly: %+v", cond1)
	}

	// Verify second condition  
	cond2 := profile.Conditions[1]
	if cond2.Type != "config" || cond2.Key != "debug" || cond2.Value != false || cond2.Operation != "equals" {
		t.Errorf("Second condition not preserved correctly: %+v", cond2)
	}
}