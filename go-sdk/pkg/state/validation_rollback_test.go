package state

import (
	"encoding/json"
	"testing"
	"time"
)

// TestStateValidation tests state validation functionality.
func TestStateValidation(t *testing.T) {
	// Create schema
	schema := &StateSchema{
		Type: "object",
		Properties: map[string]*SchemaProperty{
			"user": {
				Type: "object",
				Properties: map[string]*SchemaProperty{
					"id": {
						Type:    "string",
						Pattern: "^[a-zA-Z0-9-]+$",
					},
					"name": {
						Type:      "string",
						MinLength: intPtr(1),
						MaxLength: intPtr(100),
					},
					"age": {
						Type:    "integer",
						Minimum: float64Ptr(0),
						Maximum: float64Ptr(150),
					},
					"email": {
						Type:   "string",
						Format: "email",
					},
				},
				Required: []string{"id", "name"},
			},
			"settings": {
				Type: "object",
				Properties: map[string]*SchemaProperty{
					"theme": {
						Type: "string",
						Enum: []interface{}{"light", "dark", "auto"},
					},
					"notifications": {
						Type: "boolean",
					},
				},
			},
		},
		Required: []string{"user"},
	}

	validator := NewStateValidator(schema)

	t.Run("Valid State", func(t *testing.T) {
		state := map[string]interface{}{
			"user": map[string]interface{}{
				"id":    "user-123",
				"name":  "John Doe",
				"age":   30,
				"email": "john@example.com",
			},
			"settings": map[string]interface{}{
				"theme":         "dark",
				"notifications": true,
			},
		}

		result, err := validator.Validate(state)
		if err != nil {
			t.Fatalf("Validation failed with error: %v", err)
		}

		if !result.Valid {
			t.Errorf("Expected valid state, got invalid. Errors: %v", result.Errors)
		}
	})

	t.Run("Missing Required Field", func(t *testing.T) {
		state := map[string]interface{}{
			"user": map[string]interface{}{
				"id": "user-123",
				// Missing required "name" field
			},
		}

		result, err := validator.Validate(state)
		if err != nil {
			t.Fatalf("Validation failed with error: %v", err)
		}

		if result.Valid {
			t.Error("Expected invalid state due to missing required field")
		}

		foundError := false
		for _, e := range result.Errors {
			if e.Code == "REQUIRED_PROPERTY_MISSING" && e.Path == "/user/name" {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error for missing required field, got: %v", result.Errors)
		}
	})

	t.Run("Invalid Type", func(t *testing.T) {
		state := map[string]interface{}{
			"user": map[string]interface{}{
				"id":   "user-123",
				"name": "John Doe",
				"age":  "thirty", // Should be integer
			},
		}

		result, err := validator.Validate(state)
		if err != nil {
			t.Fatalf("Validation failed with error: %v", err)
		}

		if result.Valid {
			t.Error("Expected invalid state due to type mismatch")
		}

		foundError := false
		for _, e := range result.Errors {
			if e.Code == "TYPE_MISMATCH" && e.Path == "/user/age" {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected type mismatch error, got: %v", result.Errors)
		}
	})

	t.Run("Enum Violation", func(t *testing.T) {
		state := map[string]interface{}{
			"user": map[string]interface{}{
				"id":   "user-123",
				"name": "John Doe",
			},
			"settings": map[string]interface{}{
				"theme": "blue", // Not in enum
			},
		}

		result, err := validator.Validate(state)
		if err != nil {
			t.Fatalf("Validation failed with error: %v", err)
		}

		if result.Valid {
			t.Error("Expected invalid state due to enum violation")
		}

		foundError := false
		for _, e := range result.Errors {
			if e.Code == "ENUM_VIOLATION" && e.Path == "/settings/theme" {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected enum violation error, got: %v", result.Errors)
		}
	})

	t.Run("Custom Validation Rule", func(t *testing.T) {
		// Add custom rule that requires user ID to start with "user-"
		rule := NewFuncValidationRule(
			"user-id-format",
			"User ID must start with 'user-'",
			func(state map[string]interface{}) []ValidationError {
				errors := []ValidationError{}

				if user, ok := state["user"].(map[string]interface{}); ok {
					if id, ok := user["id"].(string); ok {
						if len(id) < 5 || id[:5] != "user-" {
							errors = append(errors, ValidationError{
								Path:    "/user/id",
								Message: "User ID must start with 'user-'",
								Code:    "CUSTOM_USER_ID_FORMAT",
							})
						}
					}
				}

				return errors
			},
		)

		err := validator.AddRule(rule)
		if err != nil {
			t.Fatalf("Failed to add rule: %v", err)
		}

		// Test with invalid user ID
		state := map[string]interface{}{
			"user": map[string]interface{}{
				"id":   "123", // Should start with "user-"
				"name": "John Doe",
			},
		}

		result, err := validator.Validate(state)
		if err != nil {
			t.Fatalf("Validation failed with error: %v", err)
		}

		if result.Valid {
			t.Error("Expected invalid state due to custom rule violation")
		}

		foundError := false
		for _, e := range result.Errors {
			if e.Code == "CUSTOM_USER_ID_FORMAT" {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected custom rule error, got: %v", result.Errors)
		}
	})
}

// TestStateRollback tests state rollback functionality.
func TestStateRollback(t *testing.T) {
	store := TestStore(t)
	validator := NewStateValidator(nil) // No schema for these tests
	rollback := NewStateRollback(store, WithValidator(validator))

	t.Run("Rollback to Version", func(t *testing.T) {

		// Set initial state using root path to ensure version creation

		// Set initial state

		err := store.Set("/", map[string]interface{}{
			"data": map[string]interface{}{
				"value": "initial",
			},
		})
		if err != nil {
			t.Fatalf("Failed to set initial state: %v", err)
		}

		// Get initial version
		history, _ := store.GetHistory()
		if len(history) == 0 {
			t.Fatalf("No history available after setting initial state")
		}
		initialVersion := history[len(history)-1].ID

		// Make changes
		err = store.Set("/", map[string]interface{}{
			"data": map[string]interface{}{
				"value": "changed",
			},
		})
		if err != nil {
			t.Fatalf("Failed to update state: %v", err)
		}

		// Verify change
		data, _ := store.Get("/data/value")
		if data != "changed" {
			t.Errorf("Expected 'changed', got %v", data)
		}

		// Rollback to initial version
		err = rollback.RollbackToVersion(initialVersion)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify rollback
		data, _ = store.Get("/data/value")
		if data != "initial" {
			t.Errorf("Expected 'initial' after rollback, got %v", data)
		}
	})

	t.Run("Rollback with Markers", func(t *testing.T) {
		// Clear store
		store.Clear()

		// Set initial state using root path to ensure version creation

		// Set initial state

		err := store.Set("/", map[string]interface{}{
			"config": map[string]interface{}{
				"version": "1.0",
				"enabled": true,
			},
		})
		if err != nil {
			t.Fatalf("Failed to set initial state: %v", err)
		}

		// Create marker
		err = rollback.CreateMarker("stable-v1")
		if err != nil {
			t.Fatalf("Failed to create marker: %v", err)
		}

		// Make changes
		err = store.Set("/", map[string]interface{}{
			"config": map[string]interface{}{
				"version": "2.0",
				"enabled": false,
			},
		})
		if err != nil {
			t.Fatalf("Failed to update config: %v", err)
		}

		// Verify changes
		config, _ := store.Get("/config")
		configMap := config.(map[string]interface{})
		if configMap["version"] != "2.0" || configMap["enabled"] != false {
			t.Errorf("Unexpected state before rollback: %v", config)
		}

		// Rollback to marker
		err = rollback.RollbackToMarker("stable-v1")
		if err != nil {
			t.Fatalf("Rollback to marker failed: %v", err)
		}

		// Verify rollback
		config, _ = store.Get("/config")
		configMap = config.(map[string]interface{})
		if configMap["version"] != "1.0" || configMap["enabled"] != true {
			t.Errorf("Expected original state after rollback, got: %v", config)
		}

		// List markers
		markers, err := rollback.ListMarkers()
		if err != nil {
			t.Fatalf("Failed to list markers: %v", err)
		}
		if len(markers) != 1 || markers[0].Name != "stable-v1" {
			t.Errorf("Expected one marker 'stable-v1', got: %v", markers)
		}
	})

	t.Run("Rollback History", func(t *testing.T) {
		// Get rollback history
		history, err := rollback.GetRollbackHistory()
		if err != nil {
			t.Fatalf("Failed to get rollback history: %v", err)
		}

		// Should have at least 2 operations from previous tests
		if len(history) < 2 {
			t.Errorf("Expected at least 2 rollback operations, got %d", len(history))
		}

		// Check last operation was successful
		if len(history) > 0 {
			lastOp := history[len(history)-1]
			if !lastOp.Success {
				t.Errorf("Expected last rollback to be successful, got: %v", lastOp)
			}
		}
	})

	t.Run("Rollback Strategies", func(t *testing.T) {
		// Test with different strategies
		strategies := []RollbackStrategy{
			NewSafeRollbackStrategy(),
			NewFastRollbackStrategy(),
			NewIncrementalRollbackStrategy(2),
		}

		for _, strategy := range strategies {
			t.Run(strategy.Name(), func(t *testing.T) {
				// Create new rollback with strategy
				rb := NewStateRollback(store, WithStrategy(strategy))

				// Clear and set initial state
				store.Clear()

				// Use root path to ensure version is created

				err := store.Set("/", map[string]interface{}{
					"test": map[string]interface{}{
						"strategy": strategy.Name(),
						"data":     []interface{}{1, 2, 3},
					},
				})
				if err != nil {
					t.Fatalf("Failed to set initial state: %v", err)
				}

				// Get initial version
				history, _ := store.GetHistory()
				if len(history) == 0 {

					t.Fatalf("No history available after setting initial state for strategy %s", strategy.Name())

					t.Fatalf("No history available after setting initial state")

				}
				initialVersion := history[len(history)-1].ID

				// Make changes
				err = store.Set("/", map[string]interface{}{
					"test": map[string]interface{}{
						"strategy": strategy.Name(),
						"data":     []interface{}{4, 5, 6},
					},
				})
				if err != nil {
					t.Fatalf("Failed to update state: %v", err)
				}

				// Rollback
				err = rb.RollbackToVersion(initialVersion)
				if err != nil {
					t.Fatalf("Rollback with %s strategy failed: %v", strategy.Name(), err)
				}

				// Verify
				data, _ := store.Get("/test/data")
				dataArr := data.([]interface{})

				if len(dataArr) != 3 {
					t.Errorf("Rollback with %s strategy failed, expected 3 elements, got: %v", strategy.Name(), data)
				} else {
					// Check first element - handle both int and json.Number types
					firstElem := dataArr[0]
					var firstValue int
					switch v := firstElem.(type) {
					case int:
						firstValue = v
					case json.Number:
						if intVal, err := v.Int64(); err == nil {
							firstValue = int(intVal)
						}
					case float64:
						firstValue = int(v)
					}
					if firstValue != 1 {
						t.Errorf("Rollback with %s strategy failed, expected first element to be 1, got: %v (type: %T)", strategy.Name(), firstElem, firstElem)
					}
				}
			})
		}
	})

	t.Run("Rollback Validation", func(t *testing.T) {
		// Create schema that requires positive numbers
		schema := &StateSchema{
			Type: "object",
			Properties: map[string]*SchemaProperty{
				"count": {
					Type:    "integer",
					Minimum: float64Ptr(0),
				},
			},
		}

		validator := NewStateValidator(schema)
		rb := NewStateRollback(store, WithValidator(validator))

		// Clear and set valid state
		store.Clear()

		// Use root path to ensure version is created
		err := store.Set("/", map[string]interface{}{"count": 10})

		err = store.Set("/", map[string]interface{}{
			"count": 10,
		})

		if err != nil {
			t.Fatalf("Failed to set initial state: %v", err)
		}

		// Get valid version
		history, _ := store.GetHistory()
		if len(history) == 0 {

			t.Fatalf("No history available after setting valid state")

			t.Fatalf("No history available after setting initial state")

		}
		validVersion := history[len(history)-1].ID

		// Set invalid state (negative number)
		// Note: We bypass validation by directly manipulating history
		store.history = append(store.history, &StateVersion{
			ID:        "invalid-version",
			Timestamp: time.Now(),
			State:     map[string]interface{}{"count": -5},
		})

		// Try to rollback to invalid version
		err = rb.RollbackToVersion("invalid-version")
		if err == nil {
			t.Error("Expected rollback to fail due to validation")
		}

		// Rollback to valid version should succeed
		err = rb.RollbackToVersion(validVersion)
		if err != nil {
			t.Errorf("Rollback to valid version failed: %v", err)
		}
	})
}

// TestIntegration tests validation and rollback working together.
func TestIntegration(t *testing.T) {
	// Create schema for a user management system
	schema := &StateSchema{
		Type: "object",
		Properties: map[string]*SchemaProperty{
			"users": {
				Type: "object",
				PatternProperties: map[string]*SchemaProperty{
					"^user-[0-9]+$": {
						Type: "object",
						Properties: map[string]*SchemaProperty{
							"name": {
								Type:      "string",
								MinLength: intPtr(1),
							},
							"role": {
								Type: "string",
								Enum: []interface{}{"admin", "user", "guest"},
							},
							"active": {
								Type: "boolean",
							},
						},
						Required: []string{"name", "role"},
					},
				},
			},
			"metadata": {
				Type: "object",
				Properties: map[string]*SchemaProperty{
					"version": {
						Type:    "string",
						Pattern: `^\d+\.\d+\.\d+$`,
					},
					"lastUpdated": {
						Type:   "string",
						Format: "date-time",
					},
				},
			},
		},
	}

	store := TestStore(t)
	validator := NewStateValidator(schema)
	rollback := NewStateRollback(store, WithValidator(validator))

	// Set initial valid state
	initialState := map[string]interface{}{
		"users": map[string]interface{}{
			"user-1": map[string]interface{}{
				"name":   "Admin User",
				"role":   "admin",
				"active": true,
			},
			"user-2": map[string]interface{}{
				"name":   "Regular User",
				"role":   "user",
				"active": true,
			},
		},
		"metadata": map[string]interface{}{
			"version":     "1.0.0",
			"lastUpdated": time.Now().Format(time.RFC3339),
		},
	}

	err := store.Set("/", initialState)
	if err != nil {
		t.Fatalf("Failed to set initial state: %v", err)
	}

	// Create a stable marker
	err = rollback.CreateMarker("stable-release")
	if err != nil {
		t.Fatalf("Failed to create marker: %v", err)
	}

	// Make valid changes
	err = store.Set("/users/user-3", map[string]interface{}{
		"name":   "New User",
		"role":   "guest",
		"active": false,
	})
	if err != nil {
		t.Fatalf("Failed to add new user: %v", err)
	}

	// Update metadata
	err = store.Set("/metadata/version", "1.1.0")
	if err != nil {
		t.Fatalf("Failed to update version: %v", err)
	}

	// Validate current state
	currentState := store.GetState()
	result, err := validator.Validate(currentState)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}
	if !result.Valid {
		t.Errorf("Current state should be valid, got errors: %v", result.Errors)
	}

	// Try to make invalid change (this would normally be prevented by middleware)
	// For testing, we'll check validation separately
	invalidUser := map[string]interface{}{
		"name": "Invalid User",
		"role": "superadmin", // Not in enum
	}

	testState := deepCopy(currentState).(map[string]interface{})
	users := testState["users"].(map[string]interface{})
	users["user-4"] = invalidUser

	result, err = validator.Validate(testState)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}
	if result.Valid {
		t.Error("State with invalid role should fail validation")
	}

	// Rollback to stable release
	err = rollback.RollbackToMarker("stable-release")
	if err != nil {
		t.Fatalf("Failed to rollback to stable release: %v", err)
	}

	// Verify we're back to initial state
	state := store.GetState()
	users = state["users"].(map[string]interface{})
	if len(users) != 2 {
		t.Errorf("Expected 2 users after rollback, got %d", len(users))
	}

	metadata := state["metadata"].(map[string]interface{})
	if metadata["version"] != "1.0.0" {
		t.Errorf("Expected version 1.0.0 after rollback, got %v", metadata["version"])
	}
}

// Helper functions for tests
func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}
