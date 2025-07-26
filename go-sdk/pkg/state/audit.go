package state

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// AuditAction represents the type of action being audited
type AuditAction string

const (
	// State management actions
	AuditActionStateUpdate   AuditAction = "STATE_UPDATE"
	AuditActionStateAccess   AuditAction = "STATE_ACCESS"
	AuditActionStateRollback AuditAction = "STATE_ROLLBACK"
	AuditActionCheckpoint    AuditAction = "CHECKPOINT_CREATE"

	// Context management actions
	AuditActionContextCreate AuditAction = "CONTEXT_CREATE"
	AuditActionContextExpire AuditAction = "CONTEXT_EXPIRE"
	AuditActionContextAccess AuditAction = "CONTEXT_ACCESS"

	// Security actions
	AuditActionRateLimit      AuditAction = "RATE_LIMIT_EXCEEDED"
	AuditActionSizeLimit      AuditAction = "SIZE_LIMIT_EXCEEDED"
	AuditActionValidationFail AuditAction = "VALIDATION_FAILED"
	AuditActionSecurityBlock  AuditAction = "SECURITY_BLOCKED"

	// Configuration actions
	AuditActionConfigChange AuditAction = "CONFIG_CHANGE"

	// Error conditions
	AuditActionError AuditAction = "ERROR"
	AuditActionPanic AuditAction = "PANIC_RECOVERED"
)

// AuditResult represents the outcome of an audited action
type AuditResult string

const (
	AuditResultSuccess AuditResult = "SUCCESS"
	AuditResultFailure AuditResult = "FAILURE"
	AuditResultBlocked AuditResult = "BLOCKED"
)

// AuditLog represents a single audit log entry
type AuditLog struct {
	// Core fields
	ID        string      `json:"id"`        // Unique identifier for the log entry
	Timestamp time.Time   `json:"timestamp"` // When the action occurred
	Action    AuditAction `json:"action"`    // What action was performed
	Result    AuditResult `json:"result"`    // Outcome of the action

	// Context information
	UserID    string `json:"user_id,omitempty"`    // User performing the action
	ContextID string `json:"context_id,omitempty"` // State context ID
	StateID   string `json:"state_id,omitempty"`   // State ID being acted upon
	SessionID string `json:"session_id,omitempty"` // Session identifier

	// Resource information
	Resource     string      `json:"resource,omitempty"`      // Resource being accessed
	ResourcePath string      `json:"resource_path,omitempty"` // Full path to resource
	OldValue     interface{} `json:"old_value,omitempty"`     // Previous value (for updates)
	NewValue     interface{} `json:"new_value,omitempty"`     // New value (for updates)

	// Security information
	IPAddress  string `json:"ip_address,omitempty"`  // Client IP address
	UserAgent  string `json:"user_agent,omitempty"`  // Client user agent
	AuthMethod string `json:"auth_method,omitempty"` // Authentication method used

	// Error information
	ErrorCode    string `json:"error_code,omitempty"`    // Error code if applicable
	ErrorMessage string `json:"error_message,omitempty"` // Error message if applicable

	// Additional details
	Details  map[string]interface{} `json:"details,omitempty"`  // Additional context
	Duration time.Duration          `json:"duration,omitempty"` // How long the operation took

	// Tamper-evidence fields
	Hash         string `json:"hash"`                    // SHA256 hash of the log entry
	PreviousHash string `json:"previous_hash,omitempty"` // Hash of the previous log entry
	Sequence     int64  `json:"sequence"`                // Sequence number for ordering
}

// AuditLogger is the interface for different audit log backends
type AuditLogger interface {
	// Log writes an audit log entry
	Log(ctx context.Context, log *AuditLog) error

	// Query retrieves audit logs based on criteria
	Query(ctx context.Context, criteria AuditCriteria) ([]*AuditLog, error)

	// Verify checks the integrity of audit logs
	Verify(ctx context.Context, startTime, endTime time.Time) (*AuditVerification, error)

	// Close cleanly shuts down the audit logger
	Close() error
}

// AuditCriteria defines search criteria for audit logs
type AuditCriteria struct {
	StartTime    *time.Time
	EndTime      *time.Time
	UserID       string
	ContextID    string
	StateID      string
	Action       AuditAction
	Result       AuditResult
	ResourcePath string
	Limit        int
	Offset       int
}

// AuditVerification contains the results of an audit log verification
type AuditVerification struct {
	Valid        bool
	TotalLogs    int
	ValidLogs    int
	InvalidLogs  int
	MissingLogs  []int64  // Sequence numbers of missing logs
	TamperedLogs []string // IDs of tampered logs
	FirstLog     *AuditLog
	LastLog      *AuditLog
}

// JSONAuditLogger implements AuditLogger with JSON output
type JSONAuditLogger struct {
	mu           sync.Mutex
	writer       io.Writer
	encoder      *json.Encoder
	sequence     int64
	previousHash string
	closed       bool

	// For verification
	logCache     map[string]*AuditLog
	sequenceMap  map[int64]string
	maxCacheSize int
}

// NewJSONAuditLogger creates a new JSON audit logger
func NewJSONAuditLogger(writer io.Writer) *JSONAuditLogger {
	if writer == nil {
		writer = os.Stdout
	}

	return &JSONAuditLogger{
		writer:       writer,
		encoder:      json.NewEncoder(writer),
		sequence:     0,
		previousHash: "",
		logCache:     make(map[string]*AuditLog),
		sequenceMap:  make(map[int64]string),
		maxCacheSize: 10000, // Keep last 10k logs for verification
	}
}

// Log writes an audit log entry
func (l *JSONAuditLogger) Log(ctx context.Context, log *AuditLog) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return fmt.Errorf("audit logger is closed")
	}

	// Set sequence number
	l.sequence++
	log.Sequence = l.sequence

	// Set previous hash for chain integrity
	log.PreviousHash = l.previousHash

	// Calculate hash
	hash, err := l.calculateHash(log)
	if err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}
	log.Hash = hash
	l.previousHash = hash

	// Write to output (handle closed stdout gracefully)
	if err := l.encoder.Encode(log); err != nil {
		// Check if this is a "file already closed" error on stdout/stderr
		if strings.Contains(err.Error(), "file already closed") && 
		   (l.writer == os.Stdout || l.writer == os.Stderr) {
			// Log is being written to closed stdout/stderr during test shutdown
			// This is expected behavior, silently ignore
			return nil
		}
		return fmt.Errorf("failed to write audit log: %w", err)
	}

	// Update cache for verification
	l.updateCache(log)

	return nil
}

// Query retrieves audit logs based on criteria
func (l *JSONAuditLogger) Query(ctx context.Context, criteria AuditCriteria) ([]*AuditLog, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// For JSON logger, we only query from cache
	// A production implementation would read from the JSON file
	var results []*AuditLog

	for _, log := range l.logCache {
		if l.matchesCriteria(log, criteria) {
			results = append(results, log)
		}

		if criteria.Limit > 0 && len(results) >= criteria.Limit {
			break
		}
	}

	return results, nil
}

// Verify checks the integrity of audit logs
func (l *JSONAuditLogger) Verify(ctx context.Context, startTime, endTime time.Time) (*AuditVerification, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	verification := &AuditVerification{
		Valid:        true,
		MissingLogs:  []int64{},
		TamperedLogs: []string{},
	}

	var firstLog, lastLog *AuditLog
	var previousHash string

	// Check sequence continuity and hash chain
	for seq := int64(1); seq <= l.sequence; seq++ {
		logID, exists := l.sequenceMap[seq]
		if !exists {
			verification.MissingLogs = append(verification.MissingLogs, seq)
			verification.Valid = false
			continue
		}

		log, exists := l.logCache[logID]
		if !exists {
			verification.MissingLogs = append(verification.MissingLogs, seq)
			verification.Valid = false
			continue
		}

		// Skip logs outside time range
		if !log.Timestamp.IsZero() {
			if log.Timestamp.Before(startTime) || log.Timestamp.After(endTime) {
				continue
			}
		}

		verification.TotalLogs++

		// Verify hash chain
		if log.PreviousHash != previousHash {
			verification.TamperedLogs = append(verification.TamperedLogs, log.ID)
			verification.InvalidLogs++
			verification.Valid = false
		} else {
			// Recalculate hash to verify integrity
			originalHash := log.Hash
			log.Hash = "" // Clear hash for recalculation

			calculatedHash, err := l.calculateHash(log)
			if err != nil || calculatedHash != originalHash {
				verification.TamperedLogs = append(verification.TamperedLogs, log.ID)
				verification.InvalidLogs++
				verification.Valid = false
			} else {
				verification.ValidLogs++
			}

			log.Hash = originalHash // Restore original hash
		}

		previousHash = log.Hash

		if firstLog == nil {
			firstLog = log
		}
		lastLog = log
	}

	verification.FirstLog = firstLog
	verification.LastLog = lastLog

	return verification, nil
}

// Close cleanly shuts down the audit logger
func (l *JSONAuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}

	l.closed = true

	// Flush any buffered data
	if flusher, ok := l.writer.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			return err
		}
	}

	// Close writer if it's closeable, but not if it's stdout or stderr
	if closer, ok := l.writer.(io.Closer); ok {
		// Don't close stdout or stderr
		if l.writer != os.Stdout && l.writer != os.Stderr && l.writer != os.Stdin {
			return closer.Close()
		}
	}

	return nil
}

// calculateHash computes the SHA256 hash of a log entry
func (l *JSONAuditLogger) calculateHash(log *AuditLog) (string, error) {
	// Create a copy without the hash field
	logCopy := *log
	logCopy.Hash = ""

	// Marshal to JSON for consistent hashing
	data, err := json.Marshal(logCopy)
	if err != nil {
		return "", err
	}

	// Calculate SHA256
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// updateCache updates the internal cache for verification
func (l *JSONAuditLogger) updateCache(log *AuditLog) {
	// Add to cache
	l.logCache[log.ID] = log
	l.sequenceMap[log.Sequence] = log.ID

	// Prune cache if it's too large
	if len(l.logCache) > l.maxCacheSize {
		// Find and remove oldest entries
		// In production, this would be more efficient
		minSeq := l.sequence - int64(l.maxCacheSize)
		for seq := int64(1); seq < minSeq; seq++ {
			if logID, exists := l.sequenceMap[seq]; exists {
				delete(l.logCache, logID)
				delete(l.sequenceMap, seq)
			}
		}
	}
}

// matchesCriteria checks if a log entry matches the search criteria
func (l *JSONAuditLogger) matchesCriteria(log *AuditLog, criteria AuditCriteria) bool {
	// Check time range
	if criteria.StartTime != nil && log.Timestamp.Before(*criteria.StartTime) {
		return false
	}
	if criteria.EndTime != nil && log.Timestamp.After(*criteria.EndTime) {
		return false
	}

	// Check other fields
	if criteria.UserID != "" && log.UserID != criteria.UserID {
		return false
	}
	if criteria.ContextID != "" && log.ContextID != criteria.ContextID {
		return false
	}
	if criteria.StateID != "" && log.StateID != criteria.StateID {
		return false
	}
	if criteria.Action != "" && log.Action != criteria.Action {
		return false
	}
	if criteria.Result != "" && log.Result != criteria.Result {
		return false
	}
	if criteria.ResourcePath != "" && log.ResourcePath != criteria.ResourcePath {
		return false
	}

	return true
}

// AuditManager integrates audit logging with the StateManager
type AuditManager struct {
	logger  AuditLogger
	enabled bool
	mu      sync.RWMutex

	// Configuration
	logStateValues bool // Whether to log old/new state values
	maxValueSize   int  // Maximum size of values to log
	
	// Lifecycle management
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewAuditManager creates a new audit manager
func NewAuditManager(logger AuditLogger) *AuditManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &AuditManager{
		logger:         logger,
		enabled:        true,
		logStateValues: true,
		maxValueSize:   1024, // 1KB default
		ctx:            ctx,
		cancel:         cancel,
	}
}

// LogStateUpdate logs a state update event
func (am *AuditManager) LogStateUpdate(ctx context.Context, contextID, stateID, userID string, oldValue, newValue interface{}, result AuditResult, err error) {
	if !am.isEnabled() {
		return
	}

	log := &AuditLog{
		ID:        generateAuditID(),
		Timestamp: time.Now(),
		Action:    AuditActionStateUpdate,
		Result:    result,
		UserID:    userID,
		ContextID: contextID,
		StateID:   stateID,
		Resource:  "state",
		Details:   make(map[string]interface{}),
	}

	// Add values if configured
	if am.logStateValues {
		log.OldValue = am.truncateValue(oldValue)
		log.NewValue = am.truncateValue(newValue)
	}

	// Add error information if present
	if err != nil {
		log.ErrorMessage = err.Error()
		log.Details["error_type"] = categorizeError(err)
	}

	// Extract additional context from context
	am.enrichFromContext(ctx, log)

	// Log asynchronously to avoid blocking
	am.wg.Add(1)
	go func() {
		defer am.wg.Done()
		
		// Check if context is cancelled
		select {
		case <-am.ctx.Done():
			return
		default:
		}
		
		// Use a timeout context for the log operation
		logCtx, cancel := context.WithTimeout(am.ctx, 5*time.Second)
		defer cancel()
		
		if err := am.logger.Log(logCtx, log); err != nil {
			// In production, this would go to a fallback logger
			fmt.Fprintf(os.Stderr, "Failed to write audit log: %v\n", err)
		}
	}()
}

// LogSecurityEvent logs a security-related event
func (am *AuditManager) LogSecurityEvent(ctx context.Context, action AuditAction, contextID, userID, resource string, details map[string]interface{}) {
	if !am.isEnabled() {
		return
	}

	log := &AuditLog{
		ID:        generateAuditID(),
		Timestamp: time.Now(),
		Action:    action,
		Result:    AuditResultBlocked,
		UserID:    userID,
		ContextID: contextID,
		Resource:  resource,
		Details:   details,
	}

	// Extract additional context
	am.enrichFromContext(ctx, log)

	// Security events are always logged synchronously
	if err := am.logger.Log(ctx, log); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write security audit log: %v\n", err)
	}
}

// LogError logs an error condition
func (am *AuditManager) LogError(ctx context.Context, action AuditAction, err error, details map[string]interface{}) {
	if !am.isEnabled() {
		return
	}

	log := &AuditLog{
		ID:           generateAuditID(),
		Timestamp:    time.Now(),
		Action:       action,
		Result:       AuditResultFailure,
		ErrorMessage: err.Error(),
		Details:      details,
	}

	// Extract additional context
	am.enrichFromContext(ctx, log)

	// Error logs are written asynchronously
	am.wg.Add(1)
	go func() {
		defer am.wg.Done()
		
		// Check if context is cancelled
		select {
		case <-am.ctx.Done():
			return
		default:
		}
		
		// Use a timeout context for the log operation
		logCtx, cancel := context.WithTimeout(am.ctx, 5*time.Second)
		defer cancel()
		
		if err := am.logger.Log(logCtx, log); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write error audit log: %v\n", err)
		}
	}()
}

// SetEnabled enables or disables audit logging
func (am *AuditManager) SetEnabled(enabled bool) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.enabled = enabled
}

// isEnabled checks if audit logging is enabled
func (am *AuditManager) isEnabled() bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.enabled
}

// truncateValue truncates large values to avoid bloating audit logs
func (am *AuditManager) truncateValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	// Convert to JSON to check size
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("<error marshaling: %v>", err)
	}

	if len(data) <= am.maxValueSize {
		return value
	}

	// Return truncated string representation
	return fmt.Sprintf("<truncated %d bytes>", len(data))
}

// enrichFromContext extracts additional information from the context
func (am *AuditManager) enrichFromContext(ctx context.Context, log *AuditLog) {
	if ctx == nil {
		return
	}

	// Extract standard context values
	if userID, ok := ctx.Value("user_id").(string); ok && log.UserID == "" {
		log.UserID = userID
	}

	if sessionID, ok := ctx.Value("session_id").(string); ok {
		log.SessionID = sessionID
	}

	if ipAddress, ok := ctx.Value("ip_address").(string); ok {
		log.IPAddress = ipAddress
	}

	if userAgent, ok := ctx.Value("user_agent").(string); ok {
		log.UserAgent = userAgent
	}

	if authMethod, ok := ctx.Value("auth_method").(string); ok {
		log.AuthMethod = authMethod
	}
}

// Close gracefully shuts down the AuditManager
func (am *AuditManager) Close() error {
	// Cancel context to signal all goroutines to stop
	am.cancel()
	
	// Wait for all goroutines to complete
	am.wg.Wait()
	
	return nil
}

// generateAuditID generates a unique ID for an audit log entry
func generateAuditID() string {
	return fmt.Sprintf("audit_%d_%s", time.Now().UnixNano(), generateRandomString(8))
}

// generateRandomString generates a random string of the specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// NoOpAuditLogger is an audit logger that discards all logs (for testing)
type NoOpAuditLogger struct{}

func (n *NoOpAuditLogger) Log(ctx context.Context, log *AuditLog) error { return nil }
func (n *NoOpAuditLogger) Query(ctx context.Context, criteria AuditCriteria) ([]*AuditLog, error) {
	return []*AuditLog{}, nil
}
func (n *NoOpAuditLogger) Verify(ctx context.Context, startTime, endTime time.Time) (*AuditVerification, error) {
	return &AuditVerification{Valid: true}, nil
}
func (n *NoOpAuditLogger) Close() error { return nil }
