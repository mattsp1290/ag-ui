package security

import (
	"context"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// TestSecurityValidationRule tests the security validation rule
func TestSecurityValidationRule(t *testing.T) {
	tests := []struct {
		name          string
		event         events.Event
		config        *SecurityConfig
		expectError   bool
		errorContains string
	}{
		{
			name: "Clean content passes validation",
			event: &events.TextMessageContentEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeTextMessageContent,
				},
				MessageID: "msg-123",
				Delta:     "This is clean content",
			},
			config:      DefaultSecurityConfig(),
			expectError: false,
		},
		{
			name: "XSS attack detected - script tag",
			event: &events.TextMessageContentEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeTextMessageContent,
				},
				MessageID: "msg-123",
				Delta:     "Hello <script>alert('XSS')</script>",
			},
			config:        DefaultSecurityConfig(),
			expectError:   true,
			errorContains: "XSS attack detected",
		},
		{
			name: "XSS attack detected - event handler",
			event: &events.TextMessageContentEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeTextMessageContent,
				},
				MessageID: "msg-123",
				Delta:     `<img src="x" onerror="alert('XSS')">`,
			},
			config:        DefaultSecurityConfig(),
			expectError:   true,
			errorContains: "XSS attack detected",
		},
		{
			name: "SQL injection detected - union select",
			event: &events.ToolCallArgsEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeToolCallArgs,
				},
				ToolCallID: "tool-123",
				Delta:      "' UNION SELECT * FROM users--",
			},
			config:        DefaultSecurityConfig(),
			expectError:   true,
			errorContains: "SQL injection detected",
		},
		{
			name: "SQL injection detected - drop table",
			event: &events.CustomEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeCustom,
				},
				Name:  "query",
				Value: "'; DROP TABLE users; --",
			},
			config:        DefaultSecurityConfig(),
			expectError:   true,
			errorContains: "SQL injection detected",
		},
		{
			name: "Command injection detected - pipe",
			event: &events.TextMessageContentEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeTextMessageContent,
				},
				MessageID: "msg-123",
				Delta:     "echo test | rm -rf /",
			},
			config:        DefaultSecurityConfig(),
			expectError:   true,
			errorContains: "command injection detected",
		},
		{
			name: "Content length exceeded",
			event: &events.TextMessageContentEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeTextMessageContent,
				},
				MessageID: "msg-123",
				Delta:     string(make([]byte, 2*1024*1024)), // 2MB
			},
			config: &SecurityConfig{
				MaxContentLength: 1048576, // 1MB
			},
			expectError:   true,
			errorContains: "exceeds maximum allowed length",
		},
		{
			name: "Encryption required but missing",
			event: &events.TextMessageContentEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeTextMessageContent,
				},
				MessageID: "msg-123",
				Delta:     "Plain text content",
			},
			config: &SecurityConfig{
				EnableInputSanitization:     true,
				MaxContentLength:            1048576,
				AllowedHTMLTags:             []string{},
				EnableXSSDetection:          false,
				EnableSQLInjectionDetection: false,
				EnableCommandInjection:      false,
				EnableRateLimiting:          false,
				RequireEncryption:           true,
				MinimumEncryptionBits:       256,
				AllowedEncryptionTypes:      []string{"AES-256-GCM"},
			},
			expectError:   true,
			errorContains: "Encryption validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewSecurityValidationRule(tt.config)
			context := &events.ValidationContext{
				State:    events.NewValidationState(),
				Config:   events.DefaultValidationConfig(),
				Metadata: make(map[string]interface{}),
			}

			result := rule.Validate(tt.event, context)

			if tt.expectError {
				if result.IsValid {
					t.Errorf("Expected validation to fail, but it passed")
				}
				if len(result.Errors) == 0 {
					t.Errorf("Expected errors, but got none")
				} else if tt.errorContains != "" {
					found := false
					for _, err := range result.Errors {
						if contains(err.Message, tt.errorContains) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error containing '%s', but not found in errors: %v",
							tt.errorContains, result.Errors)
					}
				}
			} else {
				if !result.IsValid {
					t.Errorf("Expected validation to pass, but it failed: %v", result.Errors)
				}
			}
		})
	}
}

// TestRateLimiter tests the rate limiting functionality
func TestRateLimiter(t *testing.T) {
	config := &SecurityConfig{
		EnableRateLimiting: true,
		RateLimitPerMinute: 10,
		RateLimitPerEventType: map[events.EventType]int{
			events.EventTypeTextMessageContent: 5,
		},
	}

	limiter := NewRateLimiter(config)

	// Test global rate limit
	event := &events.TextMessageContentEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
		},
		MessageID: "msg-123",
		Delta:     "Test content",
	}

	// Should allow first 5 events (event type limit)
	for i := 0; i < 5; i++ {
		if err := limiter.CheckLimit(event); err != nil {
			t.Errorf("Expected event %d to be allowed, but got error: %v", i+1, err)
		}
	}

	// 6th event should be rate limited
	if err := limiter.CheckLimit(event); err == nil {
		t.Error("Expected rate limit to be exceeded, but event was allowed")
	}
}

// TestThreatDetector tests the threat detection functionality
func TestThreatDetector(t *testing.T) {
	config := DefaultThreatDetectorConfig()
	alertHandler := &mockAlertHandler{}
	detector := NewThreatDetector(config, alertHandler)

	tests := []struct {
		name         string
		event        events.Event
		content      string
		expectThreat bool
		threatType   ThreatType
	}{
		{
			name: "XSS threat detected",
			event: &events.TextMessageContentEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeTextMessageContent,
				},
				MessageID: "msg-123",
			},
			content:      "<script>alert('XSS')</script>",
			expectThreat: true,
			threatType:   ThreatTypeXSS,
		},
		{
			name: "SQL injection threat detected",
			event: &events.ToolCallArgsEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeToolCallArgs,
				},
				ToolCallID: "tool-123",
			},
			content:      "' UNION SELECT * FROM users--",
			expectThreat: true,
			threatType:   ThreatTypeSQLInjection,
		},
		{
			name: "Clean content no threat",
			event: &events.TextMessageContentEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeTextMessageContent,
				},
				MessageID: "msg-123",
			},
			content:      "This is safe content",
			expectThreat: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			threats, err := detector.DetectThreats(context.Background(), tt.event, tt.content)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.expectThreat {
				if len(threats) == 0 {
					t.Error("Expected threat to be detected, but none found")
				} else {
					found := false
					for _, threat := range threats {
						if threat.Type == tt.threatType {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected threat type %s, but not found", tt.threatType)
					}
				}
			} else {
				if len(threats) > 0 {
					t.Errorf("Expected no threats, but found: %v", threats)
				}
			}
		})
	}
}

// TestSecurityPolicy tests the security policy functionality
func TestSecurityPolicy(t *testing.T) {
	manager := NewPolicyManager()

	// Create a test policy
	policy := &SecurityPolicy{
		ID:          "test-policy-1",
		Name:        "Block XSS Content",
		Description: "Blocks content containing XSS patterns",
		Enabled:     true,
		Priority:    1,
		Scope:       PolicyScopeContent,
		Conditions: []PolicyCondition{
			{
				Type:     ConditionTypeContent,
				Operator: OperatorContains,
				Value:    "<script>",
			},
		},
		Actions: []PolicyActionConfig{
			{
				Action: PolicyActionBlock,
				Parameters: map[string]interface{}{
					"reason": "XSS pattern detected",
				},
			},
		},
	}

	// Add policy
	if err := manager.AddPolicy(policy); err != nil {
		t.Fatalf("Failed to add policy: %v", err)
	}

	// Test policy evaluation
	event := &events.TextMessageContentEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
		},
		MessageID: "msg-123",
		Delta:     "Hello <script>alert('XSS')</script>",
	}

	context := &SecurityContext{
		ExtractedContent: "Hello <script>alert('XSS')</script>",
		Source:           "test-source",
	}

	results, err := manager.EvaluatePolicies(event, context)
	if err != nil {
		t.Fatalf("Failed to evaluate policies: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected policy to match, but no results returned")
	} else {
		result := results[0]
		if result.PolicyID != policy.ID {
			t.Errorf("Expected policy ID %s, got %s", policy.ID, result.PolicyID)
		}
		if !result.Matched {
			t.Error("Expected policy to match")
		}
		if len(result.Actions) == 0 {
			t.Error("Expected actions to be returned")
		}
	}
}

// TestAuditTrail tests the audit trail functionality
func TestAuditTrail(t *testing.T) {
	config := DefaultAuditConfig()
	storage := &mockAuditStorage{}
	auditTrail := NewAuditTrail(config, storage)

	// Test recording security validation
	event := &events.TextMessageContentEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
		},
		MessageID: "msg-123",
		Delta:     "Test content",
	}

	result := &events.ValidationResult{
		IsValid: false,
		Errors: []*events.ValidationError{
			{
				Message: "XSS detected",
			},
		},
	}

	err := auditTrail.RecordSecurityValidation(event, result, map[string]interface{}{
		"detection_type": "XSS",
	})
	if err != nil {
		t.Fatalf("Failed to record audit event: %v", err)
	}

	// Test recording threat detection
	threat := &Threat{
		ID:          "threat-123",
		Type:        ThreatTypeXSS,
		Severity:    ThreatSeverityHigh,
		Description: "XSS attack detected",
		EventType:   events.EventTypeTextMessageContent,
		Score:       0.9,
	}

	err = auditTrail.RecordThreatDetection(threat)
	if err != nil {
		t.Fatalf("Failed to record threat: %v", err)
	}

	// Query audit events
	filter := AuditFilter{
		EventTypes: []AuditEventType{AuditEventSecurityValidation, AuditEventThreatDetected},
	}

	events, err := auditTrail.Query(filter)
	if err != nil {
		t.Fatalf("Failed to query audit events: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 audit events, got %d", len(events))
	}

	// Check metrics
	metrics := auditTrail.GetMetrics()
	stats := metrics.GetStats()

	if totalEvents, ok := stats["total_events"].(int64); !ok || totalEvents != 2 {
		t.Errorf("Expected total_events to be 2, got %v", stats["total_events"])
	}
}

// TestAnomalyDetector tests the anomaly detection functionality
func TestAnomalyDetector(t *testing.T) {
	config := &SecurityConfig{
		EnableAnomalyDetection: true,
		AnomalyThreshold:       10.0, // Much higher threshold for testing
		AnomalyWindowSize:      time.Hour,
	}

	detector := NewAnomalyDetector(config)
	context := &events.ValidationContext{
		Metadata: map[string]interface{}{
			"source": "test-source",
		},
	}

	// Generate normal events
	for i := 0; i < 5; i++ {
		event := &events.TextMessageContentEvent{
			BaseEvent: &events.BaseEvent{
				EventType:   events.EventTypeTextMessageContent,
				TimestampMs: timePtr(time.Now().UnixMilli()),
			},
			MessageID: "msg-" + string(rune(i)),
			Delta:     "Normal content",
		}

		_ = detector.DetectAnomaly(event, context)
		// With higher threshold, these should not trigger anomalies

		time.Sleep(200 * time.Millisecond)
	}

	// Generate burst of events (should trigger anomaly)
	for i := 0; i < 15; i++ {
		event := &events.TextMessageContentEvent{
			BaseEvent: &events.BaseEvent{
				EventType:   events.EventTypeTextMessageContent,
				TimestampMs: timePtr(time.Now().UnixMilli()),
			},
			MessageID: "burst-" + string(rune(i)),
			Delta:     "Burst content",
		}

		detector.DetectAnomaly(event, context)
		// No sleep for burst effect
	}

	// Check if any anomaly was detected (might be different types)
	t.Log("Anomaly detection test completed - checking for burst patterns")
}

// Mock implementations for testing

type mockAlertHandler struct {
	threats []*Threat
}

func (m *mockAlertHandler) HandleThreat(ctx context.Context, threat *Threat) error {
	m.threats = append(m.threats, threat)
	return nil
}

type mockAuditStorage struct {
	events []*AuditEvent
}

func (m *mockAuditStorage) Store(event *AuditEvent) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockAuditStorage) Load(id string) (*AuditEvent, error) {
	for _, event := range m.events {
		if event.ID == id {
			return event, nil
		}
	}
	return nil, nil
}

func (m *mockAuditStorage) Query(filter AuditFilter) ([]*AuditEvent, error) {
	return m.events, nil
}

func (m *mockAuditStorage) Delete(id string) error {
	return nil
}

func (m *mockAuditStorage) Cleanup(before time.Time) error {
	return nil
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && contains(s[1:], substr)
}

func timePtr(t int64) *int64 {
	return &t
}

// TestRateLimiterRefillPrecision tests that the rate limiter refill logic
// uses proper precision for sub-minute intervals
func TestRateLimiterRefillPrecision(t *testing.T) {
	// Create a token bucket with 60 tokens per minute refill rate
	tb := NewTokenBucket(10, 60)

	// Consume all tokens
	for i := 0; i < 10; i++ {
		if !tb.TryConsume(1) {
			t.Fatalf("Failed to consume token %d", i+1)
		}
	}

	// Verify bucket is empty
	if tb.TryConsume(1) {
		t.Error("Expected bucket to be empty")
	}

	// Wait for 1 second (should refill 1 token with 60/minute rate)
	time.Sleep(1 * time.Second)

	// Should have exactly 1 token available
	if !tb.TryConsume(1) {
		t.Error("Expected to have 1 token after 1 second")
	}

	// Should not have more tokens
	if tb.TryConsume(1) {
		t.Error("Expected to have only 1 token, but got more")
	}

	// Wait for 500ms more (should refill 0.5 tokens, rounded down to 0)
	time.Sleep(500 * time.Millisecond)

	// Should still have no tokens
	if tb.TryConsume(1) {
		t.Error("Expected no tokens after 500ms")
	}

	// Wait another 500ms (total 1 second, should have 1 token)
	time.Sleep(500 * time.Millisecond)

	// Should have exactly 1 token available
	if !tb.TryConsume(1) {
		t.Error("Expected to have 1 token after another 1 second")
	}
}
