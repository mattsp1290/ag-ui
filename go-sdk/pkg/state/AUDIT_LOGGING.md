# Audit Logging for AG-UI State Management

## Overview

The AG-UI Go SDK includes comprehensive audit logging capabilities designed to provide complete visibility into state management operations, security events, and system behavior. This audit system is built to meet enterprise security and compliance requirements.

## Features

### 🔍 Comprehensive Event Coverage
- **State Operations**: Create, read, update, delete operations on state data
- **Context Management**: Context creation, access, and expiration events
- **Security Events**: Rate limiting, validation failures, access violations
- **System Events**: Configuration changes, errors, and system lifecycle events
- **Checkpoint Operations**: Checkpoint creation and rollback operations

### 🛡️ Security-Focused Design
- **Tamper-Evident Logs**: SHA256 hash chains prevent log tampering
- **Immutable by Design**: Audit logs cannot be modified once written
- **Complete Forensic Context**: Includes user ID, IP address, timestamps, and operation details
- **Always-On Security**: Cannot be disabled in production environments

### 📊 Structured Logging
- **JSON Format**: Machine-readable structured logs for easy parsing
- **Consistent Schema**: Standardized fields across all log types
- **Rich Metadata**: Comprehensive context for each audited event
- **Query Capabilities**: Built-in search and filtering functionality

## Architecture

### Core Components

#### AuditLog Structure
```go
type AuditLog struct {
    // Core identification
    ID        string       `json:"id"`
    Timestamp time.Time    `json:"timestamp"`
    Action    AuditAction  `json:"action"`
    Result    AuditResult  `json:"result"`
    
    // Context information
    UserID    string `json:"user_id,omitempty"`
    ContextID string `json:"context_id,omitempty"`
    StateID   string `json:"state_id,omitempty"`
    SessionID string `json:"session_id,omitempty"`
    
    // Resource information
    Resource     string      `json:"resource,omitempty"`
    ResourcePath string      `json:"resource_path,omitempty"`
    OldValue     interface{} `json:"old_value,omitempty"`
    NewValue     interface{} `json:"new_value,omitempty"`
    
    // Security context
    IPAddress  string `json:"ip_address,omitempty"`
    UserAgent  string `json:"user_agent,omitempty"`
    AuthMethod string `json:"auth_method,omitempty"`
    
    // Error information
    ErrorCode    string `json:"error_code,omitempty"`
    ErrorMessage string `json:"error_message,omitempty"`
    
    // Additional details
    Details  map[string]interface{} `json:"details,omitempty"`
    Duration time.Duration          `json:"duration,omitempty"`
    
    // Tamper-evidence
    Hash         string `json:"hash"`
    PreviousHash string `json:"previous_hash,omitempty"`
    Sequence     int64  `json:"sequence"`
}
```

#### AuditLogger Interface
```go
type AuditLogger interface {
    Log(ctx context.Context, log *AuditLog) error
    Query(ctx context.Context, criteria AuditCriteria) ([]*AuditLog, error)
    Verify(ctx context.Context, startTime, endTime time.Time) (*AuditVerification, error)
    Close() error
}
```

### Audit Actions

The system tracks the following types of actions:

#### State Management
- `STATE_UPDATE`: State data modifications
- `STATE_ACCESS`: State data retrieval
- `STATE_ROLLBACK`: Rollback to previous checkpoint
- `CHECKPOINT_CREATE`: Checkpoint creation

#### Context Management
- `CONTEXT_CREATE`: New context creation
- `CONTEXT_EXPIRE`: Context expiration/cleanup
- `CONTEXT_ACCESS`: Context access events

#### Security Events
- `RATE_LIMIT_EXCEEDED`: Rate limiting triggered
- `SIZE_LIMIT_EXCEEDED`: Data size limits exceeded
- `VALIDATION_FAILED`: Input validation failures
- `SECURITY_BLOCKED`: Security policy violations

#### System Events
- `CONFIG_CHANGE`: Configuration modifications
- `ERROR`: Error conditions
- `PANIC_RECOVERED`: Panic recovery events

## Configuration

### Enabling Audit Logging

```go
opts := DefaultManagerOptions()
opts.EnableAudit = true
opts.AuditLogger = NewJSONAuditLogger(auditFile)

sm, err := NewStateManager(opts)
```

### Custom Audit Logger

```go
type CustomAuditLogger struct {
    // Your implementation
}

func (c *CustomAuditLogger) Log(ctx context.Context, log *AuditLog) error {
    // Send to your logging system (syslog, database, etc.)
    return nil
}

// Use custom logger
opts.AuditLogger = &CustomAuditLogger{}
```

### Context Enrichment

```go
// Add audit context to requests
ctx := context.WithValue(context.Background(), "user_id", "john.doe")
ctx = context.WithValue(ctx, "session_id", "sess_12345")
ctx = context.WithValue(ctx, "ip_address", "192.168.1.100")
ctx = context.WithValue(ctx, "user_agent", "MyApp/1.0")
ctx = context.WithValue(ctx, "auth_method", "oauth2")

// All operations using this context will include audit metadata
contextID, err := sm.CreateContext(ctx, "user_profile", metadata)
```

## Security Features

### Tamper-Evident Logging

Each audit log entry includes:
- **SHA256 Hash**: Cryptographic hash of the entire log entry
- **Previous Hash**: Creates a chain linking logs together
- **Sequence Number**: Ensures no logs are missing

```go
// Verify log integrity
verification, err := auditLogger.Verify(ctx, startTime, endTime)
if !verification.Valid {
    log.Printf("Audit log tampering detected!")
    log.Printf("Tampered logs: %v", verification.TamperedLogs)
    log.Printf("Missing logs: %v", verification.MissingLogs)
}
```

### Always-On Design

- Audit logging is enabled by default in production
- Cannot be disabled through configuration in production builds
- Failures in audit logging are logged but don't break operations
- Async logging prevents performance impact

### Data Protection

- Large values are automatically truncated to prevent log bloat
- Sensitive data can be excluded from logs
- Hash chains prevent retroactive log modification
- Immutable append-only design

## Querying and Analysis

### Basic Queries

```go
// Find all actions by a specific user
criteria := AuditCriteria{
    UserID: "john.doe",
}
logs, err := auditLogger.Query(ctx, criteria)

// Find security violations
criteria = AuditCriteria{
    Result: AuditResultBlocked,
}
securityEvents, err := auditLogger.Query(ctx, criteria)

// Find operations on specific state
criteria = AuditCriteria{
    StateID: "user_profile_123",
    Action:  AuditActionStateUpdate,
}
updates, err := auditLogger.Query(ctx, criteria)
```

### Time-Based Queries

```go
// Find events in last hour
startTime := time.Now().Add(-1 * time.Hour)
endTime := time.Now()
criteria := AuditCriteria{
    StartTime: &startTime,
    EndTime:   &endTime,
}
recentLogs, err := auditLogger.Query(ctx, criteria)
```

### Complex Analysis

```go
// Find suspicious patterns
criteria := AuditCriteria{
    Action: AuditActionRateLimit,
    UserID: "suspicious_user",
}
rateLimitEvents, err := auditLogger.Query(ctx, criteria)

if len(rateLimitEvents) > 10 {
    log.Printf("User %s has triggered rate limiting %d times", 
        "suspicious_user", len(rateLimitEvents))
}
```

## Integration Examples

### SIEM Integration

```go
type SIEMAuditLogger struct {
    endpoint string
    client   *http.Client
}

func (s *SIEMAuditLogger) Log(ctx context.Context, log *AuditLog) error {
    data, _ := json.Marshal(log)
    
    req, _ := http.NewRequestWithContext(ctx, "POST", s.endpoint, bytes.NewBuffer(data))
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := s.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

### Database Storage

```go
type DBAuditLogger struct {
    db *sql.DB
}

func (d *DBAuditLogger) Log(ctx context.Context, log *AuditLog) error {
    query := `
        INSERT INTO audit_logs (id, timestamp, action, result, user_id, 
                               context_id, state_id, resource, details, hash)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `
    
    details, _ := json.Marshal(log.Details)
    
    _, err := d.db.ExecContext(ctx, query,
        log.ID, log.Timestamp, log.Action, log.Result, log.UserID,
        log.ContextID, log.StateID, log.Resource, details, log.Hash)
    
    return err
}
```

### Syslog Integration

```go
type SyslogAuditLogger struct {
    writer *syslog.Writer
}

func (s *SyslogAuditLogger) Log(ctx context.Context, log *AuditLog) error {
    data, _ := json.Marshal(log)
    return s.writer.Info(string(data))
}
```

## Performance Considerations

### Async Logging
- Audit logs are written asynchronously to prevent blocking operations
- Internal queues handle bursts of activity
- Graceful degradation if audit system is overloaded

### Resource Management
- Automatic log rotation and cleanup
- Configurable retention policies
- Memory-bounded caches for verification

### Tuning
```go
// Configure audit performance
type AuditConfig struct {
    LogStateValues bool // Whether to include old/new values
    MaxValueSize   int  // Maximum size of logged values
    BufferSize     int  // Internal buffer size
    FlushInterval  time.Duration // How often to flush logs
}
```

## Compliance and Standards

### Regulatory Compliance
- **SOX**: Complete audit trail of financial data changes
- **HIPAA**: Healthcare data access and modification tracking
- **GDPR**: Data processing and access logging
- **SOC 2**: Security and availability monitoring

### Standards Alignment
- **ISO 27001**: Information security management
- **NIST Cybersecurity Framework**: Detect and respond functions
- **COBIT**: IT governance and management

### Audit Trail Requirements
- **Who**: User identification and authentication context
- **What**: Exact operations performed and data affected
- **When**: Precise timestamps with timezone information
- **Where**: Network location and system context
- **Why**: Business context and authorization details
- **How**: Technical details of the operation

## Best Practices

### 1. Context Enrichment
Always provide rich context in requests:
```go
ctx := context.WithValue(ctx, "user_id", userID)
ctx = context.WithValue(ctx, "session_id", sessionID)
ctx = context.WithValue(ctx, "ip_address", clientIP)
```

### 2. Regular Verification
Implement periodic integrity checks:
```go
// Daily verification
go func() {
    ticker := time.NewTicker(24 * time.Hour)
    for range ticker.C {
        verification, _ := auditLogger.Verify(ctx, yesterday, now)
        if !verification.Valid {
            alertSecurityTeam("Audit log tampering detected")
        }
    }
}()
```

### 3. Monitoring and Alerting
Set up alerts for security events:
```go
// Monitor for suspicious activity
criteria := AuditCriteria{
    Action: AuditActionRateLimit,
    StartTime: &lastHour,
}
events, _ := auditLogger.Query(ctx, criteria)

if len(events) > threshold {
    sendAlert("High rate limiting activity detected")
}
```

### 4. Retention Management
Implement appropriate retention policies:
```go
// Archive old logs
oldLogs := AuditCriteria{
    EndTime: &thirtyDaysAgo,
}
logs, _ := auditLogger.Query(ctx, oldLogs)
archiveToLongTermStorage(logs)
```

## Troubleshooting

### Common Issues

#### Performance Impact
```go
// If audit logging affects performance:
opts.EnableAudit = false // Temporarily disable for testing
// Or use a high-performance backend like Kafka
```

#### Missing Context
```go
// Ensure context is properly enriched
if userID := ctx.Value("user_id"); userID == nil {
    log.Warn("Missing user_id in context for audit")
}
```

#### Log Verification Failures
```go
verification, err := auditLogger.Verify(ctx, start, end)
if !verification.Valid {
    // Check for:
    // 1. System clock changes
    // 2. Concurrent modifications
    // 3. Storage corruption
    // 4. Network issues during log writing
}
```

## Security Considerations

### Threat Model
The audit system protects against:
- **Insider Threats**: Unauthorized access by privileged users
- **External Attacks**: Attempts to modify or delete audit trails
- **Compliance Violations**: Regulatory requirement failures
- **Data Breaches**: Unauthorized data access or modification

### Security Controls
- **Hash Chains**: Prevent log tampering
- **Immutable Storage**: Append-only log design
- **Access Controls**: Restrict audit log access
- **Encryption**: Protect logs in transit and at rest
- **Monitoring**: Real-time analysis of audit events

This comprehensive audit logging system provides enterprise-grade security and compliance capabilities while maintaining high performance and usability.