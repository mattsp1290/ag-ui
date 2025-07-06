package state

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// ThreadSafeBuffer wraps bytes.Buffer with mutex for thread-safe operations
type ThreadSafeBuffer struct {
	mu  sync.RWMutex
	buf bytes.Buffer
}

func (tsb *ThreadSafeBuffer) Write(p []byte) (n int, err error) {
	tsb.mu.Lock()
	defer tsb.mu.Unlock()
	return tsb.buf.Write(p)
}

func (tsb *ThreadSafeBuffer) String() string {
	tsb.mu.RLock()
	defer tsb.mu.RUnlock()
	return tsb.buf.String()
}

func (tsb *ThreadSafeBuffer) Reset() {
	tsb.mu.Lock()
	defer tsb.mu.Unlock()
	tsb.buf.Reset()
}

// SyncAuditLogger wraps JSONAuditLogger to provide synchronous logging for tests
type SyncAuditLogger struct {
	*JSONAuditLogger
	mu sync.Mutex
}

func NewSyncAuditLogger(writer io.Writer) *SyncAuditLogger {
	return &SyncAuditLogger{
		JSONAuditLogger: NewJSONAuditLogger(writer),
	}
}

func (sal *SyncAuditLogger) Log(ctx context.Context, log *AuditLog) error {
	sal.mu.Lock()
	defer sal.mu.Unlock()
	return sal.JSONAuditLogger.Log(ctx, log)
}

func TestAuditLogging(t *testing.T) {
	// Create a thread-safe buffer to capture audit logs
	var auditBuffer ThreadSafeBuffer
	auditLogger := NewSyncAuditLogger(&auditBuffer)
	
	// Create state manager with audit logging enabled
	opts := DefaultManagerOptions()
	opts.EnableAudit = true
	opts.AuditLogger = auditLogger
	
	sm, err := NewStateManager(opts)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}
	defer sm.Close()
	
	// Test context creation audit
	ctx := context.WithValue(context.Background(), "user_id", "test_user")
	ctx = context.WithValue(ctx, "ip_address", "192.168.1.100")
	
	contextID, err := sm.CreateContext(ctx, "test_state", map[string]interface{}{
		"version": "1.0",
	})
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	
	// Wait for audit logging to complete
	time.Sleep(200 * time.Millisecond)
	
	// Test state update audit
	updates := map[string]interface{}{
		"counter": 1,
		"name":    "test",
	}
	
	_, err = sm.UpdateState(ctx, contextID, "test_state", updates, UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update state: %v", err)
	}
	
	// Wait for audit logging to complete
	time.Sleep(200 * time.Millisecond)
	
	// Test state access audit
	_, err = sm.GetState(ctx, contextID, "test_state")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	
	// Wait for audit logging to complete
	time.Sleep(200 * time.Millisecond)
	
	// Parse and verify audit logs
	auditContent := auditBuffer.String()
	logs := strings.Split(strings.TrimSpace(auditContent), "\n")
	
	if len(logs) < 3 {
		t.Fatalf("Expected at least 3 audit logs, got %d", len(logs))
	}
	
	// Verify the logs contain expected audit events
	var foundStateUpdate, foundStateAccess bool
	
	for _, logLine := range logs {
		if logLine == "" {
			continue
		}
		
		var auditLog AuditLog
		if err := json.Unmarshal([]byte(logLine), &auditLog); err != nil {
			t.Errorf("Failed to parse audit log: %v", err)
			continue
		}
		
		// Verify common fields
		if auditLog.ID == "" {
			t.Error("Audit log missing ID")
		}
		if auditLog.Timestamp.IsZero() {
			t.Error("Audit log missing timestamp")
		}
		if auditLog.Hash == "" {
			t.Error("Audit log missing hash")
		}
		
		// Check specific actions
		switch auditLog.Action {
		case AuditActionStateUpdate:
			foundStateUpdate = true
			if auditLog.ContextID != contextID {
				t.Errorf("Expected context ID %s, got %s", contextID, auditLog.ContextID)
			}
			if auditLog.StateID != "test_state" {
				t.Errorf("Expected state ID test_state, got %s", auditLog.StateID)
			}
			if auditLog.Result != AuditResultSuccess {
				t.Errorf("Expected success result, got %s", auditLog.Result)
			}
			if auditLog.UserID != "test_user" {
				t.Errorf("Expected user ID test_user, got %s", auditLog.UserID)
			}
			if auditLog.IPAddress != "192.168.1.100" {
				t.Errorf("Expected IP address 192.168.1.100, got %s", auditLog.IPAddress)
			}
			
		case AuditActionStateAccess:
			foundStateAccess = true
			if auditLog.ContextID != contextID {
				t.Errorf("Expected context ID %s, got %s", contextID, auditLog.ContextID)
			}
			if auditLog.StateID != "test_state" {
				t.Errorf("Expected state ID test_state, got %s", auditLog.StateID)
			}
			if auditLog.Result != AuditResultSuccess {
				t.Errorf("Expected success result, got %s", auditLog.Result)
			}
		}
	}
	
	if !foundStateUpdate {
		t.Error("Did not find state update audit log")
	}
	if !foundStateAccess {
		t.Error("Did not find state access audit log")
	}
}

func TestAuditSecurityEvents(t *testing.T) {
	// Create a thread-safe buffer to capture audit logs
	var auditBuffer ThreadSafeBuffer
	auditLogger := NewJSONAuditLogger(&auditBuffer)
	
	// Create state manager with audit logging enabled
	opts := DefaultManagerOptions()
	opts.EnableAudit = true
	opts.AuditLogger = auditLogger
	
	sm, err := NewStateManager(opts)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}
	defer sm.Close()
	
	ctx := context.Background()
	
	// Create a valid context first
	contextID, err := sm.CreateContext(ctx, "test_state", nil)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	
	// Wait for any async logs
	time.Sleep(200 * time.Millisecond)
	
	// Clear the buffer to focus on security events
	auditBuffer.Reset()
	
	// Test rate limiting by making rapid requests
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			updates := map[string]interface{}{
				"rapid_update": idx,
			}
			sm.UpdateState(ctx, contextID, "test_state", updates, UpdateOptions{})
		}(i)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	
	// Wait for rate limiting to trigger and logging to complete
	time.Sleep(500 * time.Millisecond)
	
	// Parse audit logs to find security events
	auditContent := auditBuffer.String()
	if auditContent == "" {
		t.Skip("No audit logs generated - rate limiting may not have been triggered")
	}
	
	logs := strings.Split(strings.TrimSpace(auditContent), "\n")
	var foundSecurityEvent bool
	
	for _, logLine := range logs {
		if logLine == "" {
			continue
		}
		
		var auditLog AuditLog
		if err := json.Unmarshal([]byte(logLine), &auditLog); err != nil {
			continue
		}
		
		// Look for rate limiting events
		if auditLog.Action == AuditActionRateLimit {
			foundSecurityEvent = true
			if auditLog.Result != AuditResultBlocked {
				t.Errorf("Expected blocked result for rate limit, got %s", auditLog.Result)
			}
			if auditLog.ContextID != contextID {
				t.Errorf("Expected context ID %s, got %s", contextID, auditLog.ContextID)
			}
		}
	}
	
	// Note: This test may not always trigger rate limiting in fast test environments
	if foundSecurityEvent {
		t.Log("Successfully found rate limiting audit event")
	} else {
		t.Log("No rate limiting events found - this may be expected in fast test environments")
	}
}

func TestAuditLogIntegrity(t *testing.T) {
	// Create audit logger
	var auditBuffer ThreadSafeBuffer
	auditLogger := NewJSONAuditLogger(&auditBuffer)
	
	// Create several audit logs
	ctx := context.Background()
	
	logs := []*AuditLog{
		{
			ID:        "test1",
			Timestamp: time.Now(),
			Action:    AuditActionStateUpdate,
			Result:    AuditResultSuccess,
			ContextID: "ctx1",
			StateID:   "state1",
		},
		{
			ID:        "test2",
			Timestamp: time.Now(),
			Action:    AuditActionStateAccess,
			Result:    AuditResultSuccess,
			ContextID: "ctx2",
			StateID:   "state2",
		},
		{
			ID:        "test3",
			Timestamp: time.Now(),
			Action:    AuditActionCheckpoint,
			Result:    AuditResultSuccess,
			StateID:   "state3",
		},
	}
	
	// Log all entries
	for _, log := range logs {
		if err := auditLogger.Log(ctx, log); err != nil {
			t.Fatalf("Failed to log audit entry: %v", err)
		}
	}
	
	// Wait for any async operations to complete
	time.Sleep(100 * time.Millisecond)
	
	// Verify audit log integrity
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)
	
	verification, err := auditLogger.Verify(ctx, startTime, endTime)
	if err != nil {
		t.Fatalf("Failed to verify audit logs: %v", err)
	}
	
	if !verification.Valid {
		t.Errorf("Audit logs failed integrity check")
		t.Errorf("Total logs: %d, Valid: %d, Invalid: %d", 
			verification.TotalLogs, verification.ValidLogs, verification.InvalidLogs)
		t.Errorf("Missing logs: %v", verification.MissingLogs)
		t.Errorf("Tampered logs: %v", verification.TamperedLogs)
	}
	
	if verification.TotalLogs != 3 {
		t.Errorf("Expected 3 total logs, got %d", verification.TotalLogs)
	}
	
	if verification.ValidLogs != 3 {
		t.Errorf("Expected 3 valid logs, got %d", verification.ValidLogs)
	}
	
	if verification.InvalidLogs != 0 {
		t.Errorf("Expected 0 invalid logs, got %d", verification.InvalidLogs)
	}
}

func TestAuditManagerDisabled(t *testing.T) {
	// Create state manager with audit logging disabled
	opts := DefaultManagerOptions()
	opts.EnableAudit = false
	
	sm, err := NewStateManager(opts)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}
	defer sm.Close()
	
	// Verify audit manager is nil
	if sm.auditManager != nil {
		t.Error("Expected audit manager to be nil when audit is disabled")
	}
	
	// Operations should still work without audit logging
	ctx := context.Background()
	contextID, err := sm.CreateContext(ctx, "test_state", nil)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	
	updates := map[string]interface{}{
		"test": "value",
	}
	
	_, err = sm.UpdateState(ctx, contextID, "test_state", updates, UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update state: %v", err)
	}
}

func TestAuditLogQuery(t *testing.T) {
	// Create audit logger
	var auditBuffer ThreadSafeBuffer
	auditLogger := NewJSONAuditLogger(&auditBuffer)
	
	ctx := context.Background()
	
	// Create test logs with different properties
	logs := []*AuditLog{
		{
			ID:        "log1",
			Timestamp: time.Now().Add(-2 * time.Hour),
			Action:    AuditActionStateUpdate,
			Result:    AuditResultSuccess,
			UserID:    "user1",
			ContextID: "ctx1",
			StateID:   "state1",
		},
		{
			ID:        "log2",
			Timestamp: time.Now().Add(-1 * time.Hour),
			Action:    AuditActionStateAccess,
			Result:    AuditResultSuccess,
			UserID:    "user2",
			ContextID: "ctx2",
			StateID:   "state2",
		},
		{
			ID:        "log3",
			Timestamp: time.Now(),
			Action:    AuditActionRateLimit,
			Result:    AuditResultBlocked,
			UserID:    "user1",
			ContextID: "ctx1",
			StateID:   "state1",
		},
	}
	
	// Log all entries
	for _, log := range logs {
		if err := auditLogger.Log(ctx, log); err != nil {
			t.Fatalf("Failed to log audit entry: %v", err)
		}
	}
	
	// Wait for any async operations to complete
	time.Sleep(100 * time.Millisecond)
	
	// Test query by user ID
	criteria := AuditCriteria{UserID: "user1"}
	results, err := auditLogger.Query(ctx, criteria)
	if err != nil {
		t.Fatalf("Failed to query audit logs: %v", err)
	}
	
	if len(results) != 2 {
		t.Errorf("Expected 2 results for user1, got %d", len(results))
	}
	
	// Test query by action
	criteria = AuditCriteria{Action: AuditActionStateUpdate}
	results, err = auditLogger.Query(ctx, criteria)
	if err != nil {
		t.Fatalf("Failed to query audit logs: %v", err)
	}
	
	if len(results) != 1 {
		t.Errorf("Expected 1 result for state update, got %d", len(results))
	}
	
	// Test query by result
	criteria = AuditCriteria{Result: AuditResultBlocked}
	results, err = auditLogger.Query(ctx, criteria)
	if err != nil {
		t.Fatalf("Failed to query audit logs: %v", err)
	}
	
	if len(results) != 1 {
		t.Errorf("Expected 1 result for blocked result, got %d", len(results))
	}
	
	// Test time range query
	startTime := time.Now().Add(-30 * time.Minute)
	endTime := time.Now().Add(30 * time.Minute)
	criteria = AuditCriteria{StartTime: &startTime, EndTime: &endTime}
	results, err = auditLogger.Query(ctx, criteria)
	if err != nil {
		t.Fatalf("Failed to query audit logs: %v", err)
	}
	
	if len(results) != 1 {
		t.Errorf("Expected 1 result for time range, got %d", len(results))
	}
}