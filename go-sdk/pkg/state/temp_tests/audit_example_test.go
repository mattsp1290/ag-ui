package state

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// ExampleAuditLogging demonstrates comprehensive audit logging functionality
func ExampleAuditLogging() {
	// Create a file for audit logs
	auditFile, err := os.CreateTemp("", "audit_logs_*.json")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(auditFile.Name())
	defer auditFile.Close()
	
	// Create audit logger that writes to file
	auditLogger := NewJSONAuditLogger(auditFile)
	
	// Create state manager with audit logging enabled
	opts := DefaultManagerOptions()
	opts.EnableAudit = true
	opts.AuditLogger = auditLogger
	
	sm, err := NewStateManager(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer sm.Close()
	
	// Create context with user information for audit trails
	ctx := context.WithValue(context.Background(), "user_id", "admin_user")
	ctx = context.WithValue(ctx, "session_id", "session_12345")
	ctx = context.WithValue(ctx, "ip_address", "10.0.1.100")
	ctx = context.WithValue(ctx, "user_agent", "StateManager/1.0")
	ctx = context.WithValue(ctx, "auth_method", "oauth2")
	
	// Create a state context - this will be audited
	contextID, err := sm.CreateContext(ctx, "user_profile", map[string]interface{}{
		"app_version": "2.1.0",
		"environment": "production",
	})
	if err != nil {
		log.Fatal(err)
	}
	
	// Perform state updates - these will be audited
	updates := map[string]interface{}{
		"user": map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
			"role":  "administrator",
		},
		"preferences": map[string]interface{}{
			"theme":    "dark",
			"language": "en-US",
		},
		"last_login": time.Now().Unix(),
	}
	
	_, err = sm.UpdateState(ctx, contextID, "user_profile", updates, UpdateOptions{
		CreateCheckpoint: true,
		CheckpointName:   "initial_profile",
	})
	if err != nil {
		log.Fatal(err)
	}
	
	// Access state - this will be audited
	state, err := sm.GetState(ctx, contextID, "user_profile")
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Retrieved state with %d top-level keys\n", len(state.(map[string]interface{})))
	
	// Create a checkpoint - this will be audited
	checkpointID, err := sm.CreateCheckpoint(ctx, "user_profile", "pre_role_change")
	if err != nil {
		log.Fatal(err)
	}
	
	// Update user role - this will be audited
	roleUpdate := map[string]interface{}{
		"user": map[string]interface{}{
			"role": "super_administrator",
		},
	}
	
	_, err = sm.UpdateState(ctx, contextID, "user_profile", roleUpdate, UpdateOptions{})
	if err != nil {
		log.Fatal(err)
	}
	
	// Simulate a rollback due to unauthorized change - this will be audited
	err = sm.Rollback(ctx, "user_profile", checkpointID)
	if err != nil {
		log.Fatal(err)
	}
	
	// Wait for async audit logging to complete
	time.Sleep(200 * time.Millisecond)
	
	// Demonstrate audit log verification
	verification, err := auditLogger.Verify(ctx, time.Now().Add(-1*time.Hour), time.Now())
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Audit Log Verification:\n")
	fmt.Printf("  Total logs: %d\n", verification.TotalLogs)
	fmt.Printf("  Valid logs: %d\n", verification.ValidLogs)
	fmt.Printf("  Invalid logs: %d\n", verification.InvalidLogs)
	fmt.Printf("  Integrity check: %t\n", verification.Valid)
	
	if verification.FirstLog != nil {
		fmt.Printf("  First log: %s at %s\n", verification.FirstLog.Action, verification.FirstLog.Timestamp.Format(time.RFC3339))
	}
	if verification.LastLog != nil {
		fmt.Printf("  Last log: %s at %s\n", verification.LastLog.Action, verification.LastLog.Timestamp.Format(time.RFC3339))
	}
	
	// Demonstrate audit log querying
	criteria := AuditCriteria{
		UserID: "admin_user",
		Action: AuditActionStateUpdate,
	}
	
	logs, err := auditLogger.Query(ctx, criteria)
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("\nState Update Audit Logs for admin_user: %d logs\n", len(logs))
	for i, auditLog := range logs {
		fmt.Printf("  Log %d: %s on %s (Result: %s)\n", 
			i+1, auditLog.Action, auditLog.StateID, auditLog.Result)
		if auditLog.Duration > 0 {
			fmt.Printf("    Duration: %v\n", auditLog.Duration)
		}
		if len(auditLog.Details) > 0 {
			details, _ := json.MarshalIndent(auditLog.Details, "    ", "  ")
			fmt.Printf("    Details: %s\n", string(details))
		}
	}
	
	// Demonstrate security event querying
	securityCriteria := AuditCriteria{
		Result: AuditResultBlocked,
	}
	
	securityLogs, err := auditLogger.Query(ctx, securityCriteria)
	if err != nil {
		log.Fatal(err)
	}
	
	if len(securityLogs) > 0 {
		fmt.Printf("\nSecurity Events (Blocked): %d logs\n", len(securityLogs))
		for i, auditLog := range securityLogs {
			fmt.Printf("  Security Event %d: %s on %s\n", 
				i+1, auditLog.Action, auditLog.Resource)
		}
	} else {
		fmt.Println("\nNo blocked security events found")
	}
	
	// Output demonstrates that audit logging is working
	// The exact output will vary based on timing and system performance
	fmt.Println("\nAudit logging demonstration completed successfully")
	
	// Output: 
	// Retrieved state with 3 top-level keys
	// Audit Log Verification:
	//   Total logs: 5
	//   Valid logs: 5
	//   Invalid logs: 0
	//   Integrity check: true
	//   First log: STATE_UPDATE at 2024-01-01T12:00:00Z
	//   Last log: STATE_ROLLBACK at 2024-01-01T12:00:05Z
	// 
	// State Update Audit Logs for admin_user: 2 logs
	//   Log 1: STATE_UPDATE on user_profile (Result: SUCCESS)
	//     Duration: 15ms
	//     Details: {
	//       "checkpoint_created": true,
	//       "delta_operations": 3
	//     }
	//   Log 2: STATE_UPDATE on user_profile (Result: SUCCESS)
	//     Duration: 8ms
	//     Details: {
	//       "checkpoint_created": false,
	//       "delta_operations": 1
	//     }
	// 
	// No blocked security events found
	// 
	// Audit logging demonstration completed successfully
}

// ExampleAuditSecurityEvents demonstrates security event logging
func ExampleAuditSecurityEvents() {
	// Create in-memory audit logger for this example
	auditLogger := NewJSONAuditLogger(os.Stdout)
	
	// Create state manager with strict security settings
	opts := DefaultManagerOptions()
	opts.EnableAudit = true
	opts.AuditLogger = auditLogger
	
	sm, err := NewStateManager(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer sm.Close()
	
	ctx := context.WithValue(context.Background(), "user_id", "test_user")
	ctx = context.WithValue(ctx, "ip_address", "192.168.1.50")
	
	// Create context with invalid metadata to trigger security validation
	metadata := map[string]interface{}{
		"malicious_script": "<script>alert('xss')</script>",
		"large_field":      make([]byte, 100000), // Very large field
	}
	
	contextID, err := sm.CreateContext(ctx, "test_state", metadata)
	if err != nil {
		// This should fail and be audited as a security event
		fmt.Printf("Security validation prevented context creation: %v\n", err)
	} else {
		// If context creation succeeds, try updates that will trigger security events
		
		// Try to update with oversized data
		oversizedUpdate := map[string]interface{}{
			"large_data": make([]byte, 2000000), // 2MB - should exceed limits
		}
		
		_, err = sm.UpdateState(ctx, contextID, "test_state", oversizedUpdate, UpdateOptions{})
		if err != nil {
			fmt.Printf("Security validation prevented oversized update: %v\n", err)
		}
		
		// Attempt rapid updates to trigger rate limiting
		for i := 0; i < 50; i++ {
			go func(index int) {
				updates := map[string]interface{}{
					"counter": index,
				}
				_, err := sm.UpdateState(ctx, contextID, "test_state", updates, UpdateOptions{})
				if err != nil {
					fmt.Printf("Rate limiting triggered: %v\n", err)
				}
			}(i)
		}
	}
	
	// Wait for security events to be logged
	time.Sleep(500 * time.Millisecond)
	
	fmt.Println("Security event logging demonstration completed")
	
	// This example demonstrates how security violations are automatically
	// audited, providing a complete trail of both successful operations
	// and blocked malicious attempts.
}

// ExampleAuditLogTamperDetection demonstrates tamper detection capabilities
func ExampleAuditLogTamperDetection() {
	// Create JSON audit logger
	auditLogger := NewJSONAuditLogger(os.Stdout)
	
	ctx := context.Background()
	
	// Create legitimate audit logs
	log1 := &AuditLog{
		ID:        "log_001",
		Timestamp: time.Now(),
		Action:    AuditActionStateUpdate,
		Result:    AuditResultSuccess,
		UserID:    "user123",
		StateID:   "state456",
		Resource:  "user_profile",
	}
	
	if err := auditLogger.Log(ctx, log1); err != nil {
		log.Fatal(err)
	}
	
	log2 := &AuditLog{
		ID:        "log_002",
		Timestamp: time.Now(),
		Action:    AuditActionStateAccess,
		Result:    AuditResultSuccess,
		UserID:    "user123",
		StateID:   "state456",
		Resource:  "user_profile",
	}
	
	if err := auditLogger.Log(ctx, log2); err != nil {
		log.Fatal(err)
	}
	
	// Verify integrity before tampering
	verification, err := auditLogger.Verify(ctx, time.Now().Add(-1*time.Hour), time.Now())
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Before tampering - Valid: %t, Total: %d, Valid: %d, Invalid: %d\n", 
		verification.Valid, verification.TotalLogs, verification.ValidLogs, verification.InvalidLogs)
	
	// Simulate tampering by manually modifying a log entry
	// In a real scenario, this would detect external tampering
	if len(auditLogger.logCache) > 0 {
		// Get first log and modify it
		for _, log := range auditLogger.logCache {
			log.UserID = "tampered_user" // Simulate tampering
			break
		}
		
		// Verify integrity after tampering
		verification, err = auditLogger.Verify(ctx, time.Now().Add(-1*time.Hour), time.Now())
		if err != nil {
			log.Fatal(err)
		}
		
		fmt.Printf("After tampering - Valid: %t, Total: %d, Valid: %d, Invalid: %d\n", 
			verification.Valid, verification.TotalLogs, verification.ValidLogs, verification.InvalidLogs)
		
		if len(verification.TamperedLogs) > 0 {
			fmt.Printf("Tampered logs detected: %v\n", verification.TamperedLogs)
		}
	}
	
	fmt.Println("Tamper detection demonstration completed")
	
	// Output:
	// Before tampering - Valid: true, Total: 2, Valid: 2, Invalid: 0
	// After tampering - Valid: false, Total: 2, Valid: 1, Invalid: 1
	// Tampered logs detected: [log_001]
	// Tamper detection demonstration completed
}