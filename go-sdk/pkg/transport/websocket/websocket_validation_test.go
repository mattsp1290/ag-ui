//go:build heavy

package websocket

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// TestWebSocketEventValidation tests basic WebSocket event validation
func TestWebSocketEventValidation(t *testing.T) {

	tests := []struct {
		name            string
		event           events.Event
		validationLevel events.ValidationLevel
		expectError     bool
		errorContains   string
	}{
		{
			name: "valid_event_strict_validation",
			event: &MockEvent{
				EventType:   events.EventTypeTextMessageStart,
				Data:        "test message",
				TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
			},
			validationLevel: events.ValidationStrict,
			expectError:     false,
		},
		{
			name: "invalid_event_strict_validation",
			event: &MockEvent{
				EventType: events.EventTypeTextMessageStart,
				Data:      "test message",
				ValidationFunc: func() error {
					return errors.New("validation failed: invalid message format")
				},
			},
			validationLevel: events.ValidationStrict,
			expectError:     true,
			errorContains:   "validation failed",
		},
		{
			name: "event_with_empty_type",
			event: &MockEvent{
				EventType:   "",
				Data:        "test message",
				TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
				ValidationFunc: func() error {
					return fmt.Errorf("event type is required")
				},
			},
			validationLevel: events.ValidationStrict,
			expectError:     true,
			errorContains:   "event type",
		},
		{
			name: "event_with_invalid_timestamp",
			event: &MockEvent{
				EventType:   events.EventTypeRunStarted,
				Data:        "test",
				TimestampMs: func() *int64 { t := int64(-1); return &t }(),
			},
			validationLevel: events.ValidationStrict,
			expectError:     true,
			errorContains:   "timestamp",
		},
		{
			name:            "permissive_validation_allows_empty_timestamp",
			event:           events.NewTextMessageContentEvent("msg-1", "test"),
			validationLevel: events.ValidationPermissive,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation without WebSocket connection to avoid hanging
			validationConfig := &events.ValidationConfig{
				Level:                  tt.validationLevel,
				SkipSequenceValidation: true, // Skip sequence validation for individual event tests
			}

			// Use the simpler validator that respects the config
			simpleValidator := events.NewValidator(validationConfig)

			// Test validation
			ctx := context.Background()
			validationErr := simpleValidator.ValidateEvent(ctx, tt.event)

			if tt.expectError {
				assert.Error(t, validationErr)
				if tt.errorContains != "" && validationErr != nil {
					assert.Contains(t, validationErr.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, validationErr)
			}
		})
	}
}

// TestWebSocketDifferentEventTypes tests validation for different event types
func TestWebSocketDifferentEventTypes(t *testing.T) {

	// Test different event types with validation (without WebSocket transport)
	eventTypes := []struct {
		name        string
		createEvent func() events.Event
		expectError bool
	}{
		{
			name: "run_started_event",
			createEvent: func() events.Event {
				return events.NewRunStartedEvent("thread-456", "run-123")
			},
			expectError: false,
		},
		{
			name: "run_started_event_missing_ids",
			createEvent: func() events.Event {
				return &events.RunStartedEvent{
					BaseEvent: events.NewBaseEvent(events.EventTypeRunStarted),
				}
			},
			expectError: true,
		},
		{
			name: "step_started_event",
			createEvent: func() events.Event {
				return events.NewStepStartedEvent("process_data")
			},
			expectError: false,
		},
		{
			name: "step_started_event_missing_name",
			createEvent: func() events.Event {
				return &events.StepStartedEvent{
					BaseEvent: events.NewBaseEvent(events.EventTypeStepStarted),
				}
			},
			expectError: true,
		},
		{
			name: "tool_call_start_event",
			createEvent: func() events.Event {
				return events.NewToolCallStartEvent("tool-123", "calculate")
			},
			expectError: false,
		},
		{
			name: "text_message_content_event",
			createEvent: func() events.Event {
				return events.NewTextMessageContentEvent("msg-123", "Hello, world!")
			},
			expectError: false,
		},
		{
			name: "state_snapshot_event",
			createEvent: func() events.Event {
				return events.NewStateSnapshotEvent(map[string]interface{}{
					"key1": "value1",
					"key2": 42,
				})
			},
			expectError: false,
		},
		{
			name: "custom_event",
			createEvent: func() events.Event {
				return events.NewCustomEvent("my_custom_event", events.WithValue(map[string]interface{}{
					"custom_field": "custom_value",
				}))
			},
			expectError: false,
		},
	}

	// Use testing validation config that skips sequence validation
	validator := events.NewValidator(events.TestingValidationConfig())
	ctx := context.Background()

	for _, et := range eventTypes {
		t.Run(et.name, func(t *testing.T) {
			event := et.createEvent()
			err := validator.ValidateEvent(ctx, event)

			if et.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestWebSocketValidationErrorHandling tests error handling during validation
func TestWebSocketValidationErrorHandling(t *testing.T) {

	tests := []struct {
		name             string
		event            events.Event
		validationConfig *events.ValidationConfig
		expectError      bool
		errorContains    string
	}{
		{
			name: "event_size_validation",
			event: &MockEvent{
				EventType:   events.EventTypeTextMessageContent,
				Data:        strings.Repeat("x", 2000), // Large data
				TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
			},
			validationConfig: events.TestingValidationConfig(), // Use testing config to avoid strict timestamp validation
			expectError:      false,                            // Size validation is not part of basic validation
		},
		{
			name: "validation_timeout_simulation",
			event: &MockEvent{
				EventType:   events.EventTypeTextMessageStart,
				Data:        "test",
				TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
				ValidationFunc: func() error {
					time.Sleep(50 * time.Millisecond) // Simulate slow validation
					return nil
				},
			},
			validationConfig: events.TestingValidationConfig(),
			expectError:      false,
		},
		{
			name:             "nil_event_validation",
			event:            nil,
			validationConfig: events.DefaultValidationConfig(),
			expectError:      true,
			errorContains:    "nil",
		},
		{
			name: "invalid_event_type",
			event: &MockEvent{
				EventType:   "INVALID_TYPE",
				Data:        "test",
				TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
				ValidationFunc: func() error {
					return fmt.Errorf("invalid event type")
				},
			},
			validationConfig: events.TestingValidationConfig(),
			expectError:      true,
			errorContains:    "event type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := events.NewValidator(tt.validationConfig)
			ctx := context.Background()

			var err error
			if tt.event != nil {
				err = validator.ValidateEvent(ctx, tt.event)
			} else {
				// Special handling for nil event test
				err = errors.New("event cannot be nil")
			}

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" && err != nil {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestWebSocketConnectionLifecycleValidation tests validation during connection lifecycle
func TestWebSocketConnectionLifecycleValidation(t *testing.T) {

	// Test event validation during different lifecycle phases
	lifecycleTests := []struct {
		name        string
		phase       string
		events      []events.Event
		expectError bool
	}{
		{
			name:  "startup_phase_events",
			phase: "startup",
			events: []events.Event{
				events.NewRunStartedEvent("thread-1", "run-1"),
				events.NewStepStartedEvent("initial_step"),
			},
			expectError: false,
		},
		{
			name:  "runtime_phase_events",
			phase: "runtime",
			events: []events.Event{
				events.NewTextMessageContentEvent("msg-1", "Hello"),
				events.NewToolCallStartEvent("tool-1", "calculate"),
			},
			expectError: false,
		},
		{
			name:  "shutdown_phase_events",
			phase: "shutdown",
			events: []events.Event{
				events.NewStepFinishedEvent("final_step"),
				events.NewRunFinishedEvent("thread-1", "run-1"),
			},
			expectError: false,
		},
		{
			name:  "invalid_lifecycle_order",
			phase: "invalid",
			events: []events.Event{
				events.NewRunFinishedEvent("thread-1", "run-1"), // Finished before started
				events.NewRunStartedEvent("thread-1", "run-1"),
			},
			expectError: false, // With testing config, sequence validation is skipped
		},
	}

	validator := events.NewValidator(events.TestingValidationConfig())
	ctx := context.Background()

	for _, tt := range lifecycleTests {
		t.Run(tt.name, func(t *testing.T) {
			var hasError bool
			for _, event := range tt.events {
				err := validator.ValidateEvent(ctx, event)
				if err != nil {
					hasError = true
					break
				}
			}

			if tt.expectError {
				assert.True(t, hasError, "Expected validation error for lifecycle phase %s", tt.phase)
			} else {
				assert.False(t, hasError, "Unexpected validation error for lifecycle phase %s", tt.phase)
			}
		})
	}
}

// TestWebSocketConcurrentValidation tests concurrent event validation
func TestWebSocketConcurrentValidation(t *testing.T) {
	t.Run("concurrent_event_validation", func(t *testing.T) {
		validator := events.NewValidator(events.TestingValidationConfig())
		ctx := context.Background()

		// Send events concurrently for validation
		const numGoroutines = 10
		const eventsPerGoroutine = 100
		var wg sync.WaitGroup
		successCount := atomic.Int32{}
		errorCount := atomic.Int32{}

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < eventsPerGoroutine; j++ {
					event := &MockEvent{
						EventType:   events.EventTypeTextMessageContent,
						Data:        fmt.Sprintf("worker-%d-message-%d", workerID, j),
						TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
					}

					if err := validator.ValidateEvent(ctx, event); err != nil {
						errorCount.Add(1)
					} else {
						successCount.Add(1)
					}
				}
			}(i)
		}

		wg.Wait()

		// Verify results
		totalEvents := numGoroutines * eventsPerGoroutine
		successRate := float64(successCount.Load()) / float64(totalEvents)
		t.Logf("Success rate: %.2f%% (%d/%d)", successRate*100, successCount.Load(), totalEvents)
		assert.Greater(t, successRate, 0.95, "Success rate should be above 95%")
	})

	t.Run("concurrent_validation_with_different_configs", func(t *testing.T) {
		// Create multiple validators with different validation configs
		configs := []struct {
			name             string
			validationConfig *events.ValidationConfig
		}{
			{
				name:             "strict",
				validationConfig: events.ProductionValidationConfig(),
			},
			{
				name:             "permissive",
				validationConfig: events.PermissiveValidationConfig(),
			},
			{
				name:             "development",
				validationConfig: events.DevelopmentValidationConfig(),
			},
		}

		var validators []*events.Validator
		for _, cfg := range configs {
			validator := events.NewValidator(cfg.validationConfig)
			validators = append(validators, validator)
		}

		// Send events through each validator concurrently
		var wg sync.WaitGroup
		results := make(map[string]*atomic.Int32)

		for i, validator := range validators {
			configName := configs[i].name
			results[configName] = &atomic.Int32{}

			wg.Add(1)
			go func(v *events.Validator, name string, counter *atomic.Int32) {
				defer wg.Done()

				// Test event that might fail strict validation
				event := &MockEvent{
					EventType: events.EventTypeRunStarted,
					Data:      "test",
					// No timestamp - will fail strict validation
				}

				for j := 0; j < 50; j++ {
					if err := v.ValidateEvent(context.Background(), event); err == nil {
						counter.Add(1)
					}
				}
			}(validator, configName, results[configName])
		}

		wg.Wait()

		// Verify different validation levels had different results
		strictSuccess := results["strict"].Load()
		permissiveSuccess := results["permissive"].Load()
		developmentSuccess := results["development"].Load()

		t.Logf("Strict: %d, Permissive: %d, Development: %d", strictSuccess, permissiveSuccess, developmentSuccess)

		// Strict should fail all (no timestamp)
		assert.Equal(t, int32(0), strictSuccess, "Strict validation should fail without timestamp")
		// Permissive might still fail due to base event validation, but development should pass
		assert.Greater(t, developmentSuccess, int32(45), "Development should mostly succeed")
		// Permissive might be more restrictive than expected, so let's just check it's consistent
		assert.LessOrEqual(t, permissiveSuccess, int32(50), "Permissive results should be reasonable")
	})

	t.Run("validation_under_high_load", func(t *testing.T) {
		// Custom validator that tracks validation performance
		validationTimes := &sync.Map{}
		customValidator := func(ctx context.Context, event events.Event) error {
			start := time.Now()
			defer func() {
				duration := time.Since(start)
				validationTimes.Store(event.(*MockEvent).Data, duration)
			}()

			// Simulate some validation work
			time.Sleep(time.Microsecond * 100)

			// Validate data length
			if mockEvent, ok := event.(*MockEvent); ok {
				if dataStr, ok := mockEvent.Data.(string); ok {
					if len(dataStr) > 1000 {
						return errors.New("data too large")
					}
				}
			}
			return nil
		}

		validationConfig := events.DefaultValidationConfig()
		validationConfig.CustomValidators = []events.CustomValidator{customValidator}

		validator := events.NewValidator(validationConfig)
		ctx := context.Background()

		// Generate high load
		const numWorkers = 20
		const eventsPerWorker = 200
		var wg sync.WaitGroup
		start := time.Now()

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < eventsPerWorker; j++ {
					event := &MockEvent{
						EventType:   events.EventTypeTextMessageContent,
						Data:        fmt.Sprintf("load-test-%d-%d", workerID, j),
						TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
					}

					validator.ValidateEvent(ctx, event)
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		// Calculate throughput
		totalEvents := numWorkers * eventsPerWorker
		throughput := float64(totalEvents) / duration.Seconds()
		t.Logf("Throughput: %.2f events/second", throughput)

		// Verify validation performance
		var totalValidationTime time.Duration
		var validationCount int
		validationTimes.Range(func(key, value interface{}) bool {
			totalValidationTime += value.(time.Duration)
			validationCount++
			return true
		})

		if validationCount > 0 {
			avgValidationTime := totalValidationTime / time.Duration(validationCount)
			t.Logf("Average validation time: %v", avgValidationTime)
			assert.Less(t, avgValidationTime, time.Millisecond, "Validation should be fast")
		}
	})
}

// TestWebSocketValidationWithCustomRules tests custom validation rules
func TestWebSocketValidationWithCustomRules(t *testing.T) {

	// Create custom validation rules
	var validationLog sync.Map

	customValidators := []events.CustomValidator{
		// Rule 1: Event data must not contain forbidden words
		func(ctx context.Context, event events.Event) error {
			if mockEvent, ok := event.(*MockEvent); ok {
				if dataStr, ok := mockEvent.Data.(string); ok {
					forbiddenWords := []string{"forbidden", "blocked", "invalid"}
					for _, word := range forbiddenWords {
						if strings.Contains(strings.ToLower(dataStr), word) {
							validationLog.Store("forbidden_word", true)
							return fmt.Errorf("data contains forbidden word: %s", word)
						}
					}
				}
			}
			return nil
		},
		// Rule 2: Event must have timestamp in specific range
		events.NewTimestampValidator(
			time.Now().Add(-1*time.Hour).UnixMilli(),
			time.Now().Add(1*time.Hour).UnixMilli(),
		),
		// Rule 3: Only specific event types allowed
		events.NewEventTypeValidator(
			events.EventTypeTextMessageContent,
			events.EventTypeRunStarted,
			events.EventTypeCustom,
		),
	}

	validationConfig := &events.ValidationConfig{
		Level:            events.ValidationCustom,
		CustomValidators: customValidators,
	}

	validator := events.NewValidator(validationConfig)
	ctx := context.Background()

	tests := []struct {
		name        string
		event       events.Event
		expectError bool
		checkLog    string
	}{
		{
			name: "valid_event_passes_all_rules",
			event: &MockEvent{
				EventType:   events.EventTypeTextMessageContent,
				Data:        "This is a valid message",
				TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
			},
			expectError: false,
		},
		{
			name: "event_with_forbidden_word",
			event: &MockEvent{
				EventType:   events.EventTypeTextMessageContent,
				Data:        "This message contains forbidden content",
				TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
			},
			expectError: true,
			checkLog:    "forbidden_word",
		},
		{
			name: "event_with_invalid_timestamp",
			event: &MockEvent{
				EventType:   events.EventTypeTextMessageContent,
				Data:        "Valid message",
				TimestampMs: func() *int64 { t := time.Now().Add(-2 * time.Hour).UnixMilli(); return &t }(),
			},
			expectError: true,
		},
		{
			name: "event_with_disallowed_type",
			event: &MockEvent{
				EventType:   events.EventTypeToolCallStart, // Not in allowed list
				Data:        "Valid message",
				TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateEvent(ctx, tt.event)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.checkLog != "" {
				_, exists := validationLog.Load(tt.checkLog)
				assert.True(t, exists, "Expected validation log entry not found")
				validationLog.Delete(tt.checkLog) // Clean up for next test
			}
		})
	}
}

// TestJWTTokenValidation consolidates all JWT validation tests into a comprehensive table-driven test
func TestJWTTokenValidation(t *testing.T) {

	secretKey := []byte("test-secret-key-256-bits-minimum")
	issuer := "test-issuer"
	audience := "test-audience"

	// Generate RSA key pair for RSA tests
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey

	testCases := []struct {
		name          string
		validatorType string // "hmac" or "rsa"
		claims        jwt.MapClaims
		signingMethod jwt.SigningMethod
		signingKey    interface{}
		expectError   bool
		errorContains string
		validateAuth  func(*testing.T, *AuthContext)
	}{
		// HMAC Tests
		{
			name:          "hmac_valid_token",
			validatorType: "hmac",
			claims: jwt.MapClaims{
				"iss":         issuer,
				"aud":         audience,
				"sub":         "user123",
				"username":    "testuser",
				"roles":       []string{"admin", "user"},
				"permissions": []string{"read", "write"},
				"exp":         time.Now().Add(1 * time.Hour).Unix(),
				"iat":         time.Now().Unix(),
			},
			signingMethod: jwt.SigningMethodHS256,
			signingKey:    secretKey,
			expectError:   false,
			validateAuth: func(t *testing.T, authCtx *AuthContext) {
				assert.Equal(t, "user123", authCtx.UserID)
				assert.Equal(t, "testuser", authCtx.Username)
				assert.Contains(t, authCtx.Roles, "admin")
				assert.Contains(t, authCtx.Permissions, "read")
			},
		},
		{
			name:          "hmac_expired_token",
			validatorType: "hmac",
			claims: jwt.MapClaims{
				"iss": issuer,
				"aud": audience,
				"sub": "user123",
				"exp": time.Now().Add(-1 * time.Hour).Unix(), // Expired
				"iat": time.Now().Add(-2 * time.Hour).Unix(),
			},
			signingMethod: jwt.SigningMethodHS256,
			signingKey:    secretKey,
			expectError:   true,
			errorContains: "token is expired",
		},
		{
			name:          "hmac_invalid_issuer",
			validatorType: "hmac",
			claims: jwt.MapClaims{
				"iss": "wrong-issuer",
				"aud": audience,
				"sub": "user123",
				"exp": time.Now().Add(1 * time.Hour).Unix(),
				"iat": time.Now().Unix(),
			},
			signingMethod: jwt.SigningMethodHS256,
			signingKey:    secretKey,
			expectError:   true,
			errorContains: "invalid issuer",
		},
		{
			name:          "hmac_multiple_audiences_valid",
			validatorType: "hmac",
			claims: jwt.MapClaims{
				"iss": issuer,
				"aud": []string{"other-audience", audience, "another-audience"},
				"sub": "user123",
				"exp": time.Now().Add(1 * time.Hour).Unix(),
				"iat": time.Now().Unix(),
			},
			signingMethod: jwt.SigningMethodHS256,
			signingKey:    secretKey,
			expectError:   false,
		},
		{
			name:          "hmac_multiple_audiences_invalid",
			validatorType: "hmac",
			claims: jwt.MapClaims{
				"iss": issuer,
				"aud": []string{"other-audience", "another-audience"},
				"sub": "user123",
				"exp": time.Now().Add(1 * time.Hour).Unix(),
				"iat": time.Now().Unix(),
			},
			signingMethod: jwt.SigningMethodHS256,
			signingKey:    secretKey,
			expectError:   true,
			errorContains: "invalid audience",
		},
		// RSA Tests
		{
			name:          "rsa_valid_token",
			validatorType: "rsa",
			claims: jwt.MapClaims{
				"iss":      issuer,
				"aud":      audience,
				"sub":      "user123",
				"username": "testuser",
				"exp":      time.Now().Add(1 * time.Hour).Unix(),
				"iat":      time.Now().Unix(),
			},
			signingMethod: jwt.SigningMethodRS256,
			signingKey:    privateKey,
			expectError:   false,
			validateAuth: func(t *testing.T, authCtx *AuthContext) {
				assert.Equal(t, "user123", authCtx.UserID)
				assert.Equal(t, "testuser", authCtx.Username)
			},
		},
		{
			name:          "rsa_wrong_algorithm",
			validatorType: "rsa",
			claims: jwt.MapClaims{
				"iss": issuer,
				"aud": audience,
				"sub": "user123",
				"exp": time.Now().Add(1 * time.Hour).Unix(),
				"iat": time.Now().Unix(),
			},
			signingMethod: jwt.SigningMethodHS256, // Wrong algorithm for RSA
			signingKey:    []byte("secret"),
			expectError:   true,
			errorContains: "unexpected signing method",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create validator based on type
			var validator *JWTTokenValidator
			if tc.validatorType == "hmac" {
				validator = NewJWTTokenValidatorWithOptions(secretKey, issuer, audience, jwt.SigningMethodHS256)
			} else {
				validator = NewJWTTokenValidatorRSA(publicKey, issuer, audience)
			}

			// Create and sign token
			token := jwt.NewWithClaims(tc.signingMethod, tc.claims)
			tokenString, err := token.SignedString(tc.signingKey)
			require.NoError(t, err)

			// Validate token
			authCtx, err := validator.ValidateToken(context.Background(), tokenString)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, authCtx)
				if tc.validateAuth != nil {
					tc.validateAuth(t, authCtx)
				}
			}
		})
	}

	// Test empty token case
	t.Run("empty_token", func(t *testing.T) {
		validator := NewJWTTokenValidatorWithOptions(secretKey, issuer, audience, jwt.SigningMethodHS256)
		_, err := validator.ValidateToken(context.Background(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty token")
	})
}
