# Advanced Security Validation System

This package provides a comprehensive security validation system for the AG-UI events framework, implementing multiple layers of security checks including input sanitization, threat detection, rate limiting, anomaly detection, and audit trails.

## Components

### 1. SecurityValidationRule (`security_validator.go`)

The main security validation rule that integrates with the existing validation framework.

**Features:**
- **Input Sanitization & XSS Prevention**: Detects and prevents cross-site scripting attacks
- **SQL Injection Detection**: Identifies potential SQL injection attempts in event content  
- **Command Injection Detection**: Detects command injection patterns
- **Rate Limiting**: Prevents abuse through excessive event generation
- **Content Length Validation**: Enforces maximum content size limits
- **Encryption Validation**: Ensures content meets encryption requirements

**Usage:**
```go
config := DefaultSecurityConfig()
rule := NewSecurityValidationRule(config)

// Add to event validator
validator.AddRule(rule)
```

### 2. ThreatDetector (`threat_detector.go`)

Real-time threat detection engine that analyzes events for security threats.

**Features:**
- **Pattern-based Detection**: Uses regex patterns to identify threats
- **Behavioral Analysis**: Detects anomalous behavior patterns
- **DDoS Detection**: Identifies potential distributed denial of service attacks
- **Threat Scoring**: Assigns severity scores to detected threats
- **Alert System**: Configurable alerting for high-severity threats

**Usage:**
```go
config := DefaultThreatDetectorConfig()
alertHandler := &MyAlertHandler{}
detector := NewThreatDetector(config, alertHandler)

threats, err := detector.DetectThreats(ctx, event, content)
```

### 3. SecurityPolicy (`security_policy.go`)

Configurable security policy engine for flexible security rule management.

**Features:**
- **Policy-based Security**: Define custom security policies
- **Multiple Scopes**: Global, event-type, source, and content-based policies
- **Conditional Logic**: Complex condition evaluation
- **Action System**: Configurable actions (block, allow, warn, redact, throttle)
- **Policy Import/Export**: JSON-based policy management

**Usage:**
```go
manager := NewPolicyManager()

policy := &SecurityPolicy{
    ID:          "block-xss",
    Name:        "Block XSS Content",
    Scope:       PolicyScopeContent,
    Conditions:  []PolicyCondition{...},
    Actions:     []PolicyActionConfig{...},
}

manager.AddPolicy(policy)
results, err := manager.EvaluatePolicies(event, context)
```

### 4. AuditTrail (`audit_trail.go`)

Comprehensive audit logging for all security-related events.

**Features:**
- **Security Event Logging**: Records all security validations and violations
- **Threat Tracking**: Maintains history of detected threats
- **Policy Violation Logging**: Tracks policy violations and actions taken
- **Query Interface**: Flexible querying with filters
- **Alert Integration**: Configurable alerts for audit events
- **Persistent Storage**: Pluggable storage backend

**Usage:**
```go
config := DefaultAuditConfig()
storage := &MyAuditStorage{}
auditTrail := NewAuditTrail(config, storage)

// Record security validation
err := auditTrail.RecordSecurityValidation(event, result, details)

// Query audit events
events, err := auditTrail.Query(filter)
```

### 5. Supporting Components

#### RateLimiter (`rate_limiter.go`)
- Token bucket algorithm implementation
- Per-event-type and global rate limiting
- Configurable refill rates

#### AnomalyDetector (`anomaly_detector.go`)
- Statistical anomaly detection
- Pattern analysis
- Behavioral profiling
- Burst detection

#### EncryptionValidator (`encryption_validator.go`)
- Content encryption validation
- Algorithm verification
- Key size validation
- Encryption/decryption utilities

#### SecurityMetrics (`security_metrics.go`)
- Comprehensive security metrics
- Performance tracking
- Detection statistics
- Hourly and event-type breakdowns

## Configuration

### SecurityConfig
```go
type SecurityConfig struct {
    // Input sanitization
    EnableInputSanitization bool
    MaxContentLength        int
    AllowedHTMLTags         []string
    
    // Detection settings
    EnableXSSDetection        bool
    EnableSQLInjectionDetection bool
    EnableCommandInjection    bool
    
    // Rate limiting
    EnableRateLimiting     bool
    RateLimitPerMinute     int
    RateLimitPerEventType  map[events.EventType]int
    
    // Anomaly detection
    EnableAnomalyDetection    bool
    AnomalyThreshold         float64
    AnomalyWindowSize        time.Duration
    
    // Encryption
    RequireEncryption        bool
    MinimumEncryptionBits    int
    AllowedEncryptionTypes   []string
}
```

## Integration with Validation Framework

The security validation system integrates seamlessly with the existing AG-UI validation framework:

```go
// Add security validation to your event validator
validator := events.NewEventValidator(events.ProductionValidationConfig())

// Create and configure security rule
securityConfig := security.DefaultSecurityConfig()
securityConfig.EnableXSSDetection = true
securityConfig.EnableSQLInjectionDetection = true
securityRule := security.NewSecurityValidationRule(securityConfig)

// Add to validator
validator.AddRule(securityRule)

// Validate events as normal
result := validator.ValidateEvent(ctx, event)
```

## Security Features

### 1. XSS Prevention
- Script tag detection
- Event handler injection detection
- JavaScript URL detection
- HTML sanitization

### 2. SQL Injection Protection
- UNION SELECT detection
- Comment-based injection detection
- Database function detection
- Parameterized query validation

### 3. Command Injection Detection
- Shell metacharacter detection
- System command detection
- File path traversal detection

### 4. Rate Limiting
- Token bucket algorithm
- Per-event-type limits
- Global rate limits
- Configurable refill rates

### 5. Anomaly Detection
- Statistical analysis
- Pattern recognition
- Burst detection
- Behavioral profiling

### 6. Encryption Validation
- Content encryption verification
- Algorithm compliance
- Key size validation
- Format validation

## Best Practices

1. **Configure According to Environment**:
   - Use strict settings in production
   - Relax validation in development/testing
   - Enable comprehensive logging in all environments

2. **Monitor Security Metrics**:
   - Track detection rates
   - Monitor false positives
   - Analyze trends over time

3. **Implement Proper Alerting**:
   - Configure alerts for critical threats
   - Set up escalation procedures
   - Integrate with security incident response

4. **Regular Policy Updates**:
   - Review and update security policies
   - Add new threat patterns
   - Adjust thresholds based on experience

5. **Audit Trail Management**:
   - Implement proper log retention
   - Ensure audit log integrity
   - Regular audit log analysis

## Performance Considerations

- Pattern matching is optimized with compiled regex
- Rate limiting uses efficient token bucket algorithm
- Anomaly detection uses statistical caching
- Audit trails support persistent storage backends
- Metrics collection is thread-safe and efficient

## Testing

The package includes comprehensive tests covering:
- Security validation rules
- Threat detection patterns
- Rate limiting functionality
- Policy evaluation
- Audit trail management
- Anomaly detection

Run tests with:
```bash
go test ./pkg/core/events/security/... -v
```

## Security Considerations

1. **Regular Updates**: Keep threat patterns updated
2. **Configuration Review**: Regularly review security configurations
3. **Log Monitoring**: Monitor security logs for trends
4. **Performance Impact**: Balance security vs. performance
5. **False Positives**: Monitor and tune to reduce false positives