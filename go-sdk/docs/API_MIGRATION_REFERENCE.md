# API Migration Reference

## Overview

This document provides comprehensive before/after examples for migrating from `interface{}` usage to type-safe alternatives in the AG-UI Go SDK.

## Event System Migration

### Basic Event Creation

#### Before (Legacy TransportEvent)
```go
// Manual event struct
type LegacyEvent struct {
    id        string
    eventType string
    timestamp time.Time
    data      map[string]interface{}
}

func (e *LegacyEvent) ID() string                      { return e.id }
func (e *LegacyEvent) Type() string                    { return e.eventType }
func (e *LegacyEvent) Timestamp() time.Time            { return e.timestamp }
func (e *LegacyEvent) Data() map[string]interface{}    { return e.data }

// Usage
event := &LegacyEvent{
    id:        "conn-1",
    eventType: "connection",
    timestamp: time.Now(),
    data: map[string]interface{}{
        "status":  "connected",
        "address": "localhost:8080",
        "protocol": "tcp",
    },
}
```

#### After (Type-Safe Events)
```go
// Using type-safe event creation
event := CreateConnectionEvent("conn-1", "connected", 
    func(data *ConnectionEventData) {
        data.RemoteAddress = "localhost:8080"
        data.LocalAddress = "localhost:0"
        data.Protocol = "tcp"
        data.Version = "1.0"
        data.Metadata = map[string]string{
            "compression": "gzip",
        }
    },
)

// Convert to legacy interface when needed
legacyEvent := NewTransportEventAdapter(event)
```

### Data Events

#### Before
```go
dataEvent := &LegacyEvent{
    id:        "data-1",
    eventType: "data",
    timestamp: time.Now(),
    data: map[string]interface{}{
        "content":     []byte("hello world"),
        "contentType": "text/plain",
        "size":        11,
        "encoding":    "utf-8",
        "streamId":    "stream-1",
    },
}
```

#### After
```go
event := CreateDataEvent("data-1", &DataEventData{
    Content:     []byte("hello world"),
    ContentType: "text/plain",
    Size:        11,
    Encoding:    "utf-8",
    StreamID:    "stream-1",
    Compressed:  false,
    Checksum:    "sha256:...",
})
```

### Error Events

#### Before
```go
errorEvent := &LegacyEvent{
    id:        "error-1",
    eventType: "error",
    timestamp: time.Now(),
    data: map[string]interface{}{
        "message":   "connection failed",
        "code":      "CONN_FAILED",
        "severity":  "error",
        "retryable": true,
        "details": map[string]interface{}{
            "attempts": 3,
            "lastAttempt": time.Now().Add(-time.Minute),
        },
    },
}
```

#### After
```go
event := CreateErrorEvent("error-1", &ErrorEventData{
    Message:    "connection failed",
    Code:       "CONN_FAILED",
    Severity:   ErrorSeverityError,
    Retryable:  true,
    Category:   ErrorCategoryConnection,
    Details: map[string]string{
        "attempts":    "3",
        "lastAttempt": time.Now().Add(-time.Minute).Format(time.RFC3339),
    },
})
```

## Capabilities System Migration

### Basic Capabilities

#### Before
```go
capabilities := Capabilities{
    Streaming:       true,
    Bidirectional:   true,
    Compression:     []CompressionType{CompressionGzip},
    Multiplexing:    true,
    Reconnection:    true,
    MaxMessageSize:  1024 * 1024,
    Security:        []SecurityFeature{SecurityTLS},
    ProtocolVersion: "1.0",
    Features: map[string]interface{}{
        "compression_level": 6,
        "buffer_size":      8192,
        "keepalive":        30,
        "custom_feature":   true,
    },
}
```

#### After
```go
// Type-safe compression features
compressionFeatures := CompressionFeatures{
    SupportedAlgorithms: []CompressionType{CompressionGzip, CompressionZstd},
    DefaultAlgorithm:    CompressionGzip,
    CompressionLevel:    6,
    MinSizeThreshold:    1024,
    MaxCompressionRatio: 0.9,
}

baseCaps := Capabilities{
    Streaming:       true,
    Bidirectional:   true,
    Compression:     []CompressionType{CompressionGzip, CompressionZstd},
    Multiplexing:    true,
    Reconnection:    true,
    MaxMessageSize:  1024 * 1024,
    Security:        []SecurityFeature{SecurityTLS},
    ProtocolVersion: "1.0",
}

typedCaps := NewCompressionCapabilities(baseCaps, compressionFeatures)
capabilities := ToCapabilities(typedCaps)
```

### Security Capabilities

#### Before
```go
capabilities := Capabilities{
    Security: []SecurityFeature{SecurityTLS, SecurityJWT},
    Features: map[string]interface{}{
        "tls_version":    "1.3",
        "cipher_suites":  []string{"TLS_AES_256_GCM_SHA384"},
        "jwt_algorithm":  "RS256",
        "token_lifetime": 3600,
    },
}
```

#### After
```go
securityFeatures := SecurityFeatures{
    SupportedFeatures: []SecurityFeature{SecurityTLS, SecurityJWT},
    DefaultFeature:    SecurityTLS,
    TLSConfig: &TLSConfig{
        MinVersion:   "1.3",
        CipherSuites: []string{"TLS_AES_256_GCM_SHA384"},
        RequireSNI:   true,
    },
    JWTConfig: &JWTConfig{
        Algorithm:     "RS256",
        TokenLifetime: 3600 * time.Second,
        Issuer:        "ag-ui",
    },
}

typedCaps := NewSecurityCapabilities(baseCaps, securityFeatures)
capabilities := ToCapabilities(typedCaps)
```

## Logging System Migration

### Basic Logging

#### Before
```go
import "log"

logger := log.New(os.Stdout, "", log.LstdFlags)

// Unsafe logging with Any()
logger.Info("Processing request",
    Any("requestId", requestId),
    Any("userId", userId),
    Any("data", requestData), // Could be anything!
    Any("timestamp", time.Now()),
    Any("headers", headers),
)
```

#### After
```go
import "github.com/ag-ui/go-sdk/pkg/transport"

logger := transport.NewLogger()

// Type-safe logging
logger.InfoTyped("Processing request",
    SafeString("requestId", requestId),
    SafeString("userId", userId),
    SafeString("dataSize", fmt.Sprintf("%d bytes", len(requestData))),
    SafeTime("timestamp", time.Now()),
    SafeInt("headerCount", len(headers)),
)
```

### Error Logging

#### Before
```go
logger.Error("Operation failed",
    Any("error", err),
    Any("context", operationContext), // map[string]interface{}
    Any("retryCount", retryCount),
    Any("duration", duration),
)
```

#### After
```go
logger.ErrorTyped("Operation failed",
    SafeErr(err),
    SafeString("operation", operationContext.Name),
    SafeString("resource", operationContext.ResourceID),
    SafeInt("retryCount", retryCount),
    SafeDuration("duration", duration),
)
```

### Complex Data Logging

#### Before
```go
// Logging complex data structures
complexData := map[string]interface{}{
    "metrics": map[string]interface{}{
        "cpu":    85.5,
        "memory": 1024,
        "disk":   []interface{}{"/dev/sda1", "/dev/sda2"},
    },
    "status": "healthy",
    "count":  42,
}

logger.Info("System status", Any("data", complexData))
```

#### After
```go
// Type-safe structured logging
logger.InfoTyped("System status",
    SafeFloat64("cpuUsage", 85.5),
    SafeInt64("memoryMB", 1024),
    SafeString("diskPrimary", "/dev/sda1"),
    SafeString("diskSecondary", "/dev/sda2"),
    SafeString("status", "healthy"),
    SafeInt("count", 42),
)

// Or create a typed structure
type SystemMetrics struct {
    CPUUsage     float64
    MemoryMB     int64
    DiskDevices  []string
    Status       string
    Count        int
}

metrics := SystemMetrics{
    CPUUsage:    85.5,
    MemoryMB:    1024,
    DiskDevices: []string{"/dev/sda1", "/dev/sda2"},
    Status:      "healthy",
    Count:       42,
}

logger.InfoTyped("System status",
    SafeFloat64("cpuUsage", metrics.CPUUsage),
    SafeInt64("memoryMB", metrics.MemoryMB),
    SafeString("status", metrics.Status),
    SafeInt("diskCount", len(metrics.DiskDevices)),
)
```

## Validation System Migration

### Basic Type Validation

#### Before
```go
func validateField(field string, value interface{}) error {
    switch field {
    case "name":
        if str, ok := value.(string); ok {
            if len(str) < 1 || len(str) > 255 {
                return errors.New("name length must be 1-255 characters")
            }
            if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(str) {
                return errors.New("name contains invalid characters")
            }
            return nil
        }
        return errors.New("name must be a string")
    
    case "age":
        if num, ok := value.(int); ok {
            if num < 0 || num > 150 {
                return errors.New("age must be 0-150")
            }
            return nil
        }
        return errors.New("age must be an integer")
    
    default:
        return errors.New("unknown field")
    }
}
```

#### After
```go
// Type-safe validators
func validateName(name string) error {
    validator := NewStringValidator().
        WithMinLength(1).
        WithMaxLength(255).
        WithPattern(`^[a-zA-Z0-9_-]+$`)
    return validator.Validate(name)
}

func validateAge(age int) error {
    validator := NewIntValidatorWithRange(0, 150)
    return validator.Validate(age)
}

// Or use generic validation
func ValidateField[T ValidatableValue](field string, value T) error {
    return ValidateGenericValue(field, value)
}
```

### Complex Validation Rules

#### Before
```go
func validateConfiguration(config map[string]interface{}) error {
    // Validate timeout
    if timeoutVal, ok := config["timeout"]; ok {
        if timeout, ok := timeoutVal.(int); ok {
            if timeout < 1 || timeout > 300 {
                return errors.New("timeout must be 1-300 seconds")
            }
        } else {
            return errors.New("timeout must be an integer")
        }
    }
    
    // Validate retries with dependency on timeout
    if retriesVal, ok := config["retries"]; ok {
        if retries, ok := retriesVal.(int); ok {
            if retries < 0 || retries > 10 {
                return errors.New("retries must be 0-10")
            }
            // Complex dependency validation
            if timeoutVal, ok := config["timeout"]; ok {
                if timeout, ok := timeoutVal.(int); ok {
                    if retries*timeout > 600 {
                        return errors.New("total retry time cannot exceed 600 seconds")
                    }
                }
            }
        } else {
            return errors.New("retries must be an integer")
        }
    }
    
    return nil
}
```

#### After
```go
type ConnectionConfig struct {
    Timeout time.Duration `validate:"min=1s,max=5m"`
    Retries int           `validate:"min=0,max=10"`
    Address string        `validate:"required,url"`
}

func (c *ConnectionConfig) Validate() error {
    // Basic field validation
    if c.Timeout < time.Second || c.Timeout > 5*time.Minute {
        return NewDurationConfigError("timeout", c.Timeout)
    }
    
    if c.Retries < 0 || c.Retries > 10 {
        return NewIntConfigError("retries", c.Retries)
    }
    
    // Complex dependency validation
    totalRetryTime := time.Duration(c.Retries) * c.Timeout
    if totalRetryTime > 10*time.Minute {
        return NewDurationConfigError("totalRetryTime", totalRetryTime)
    }
    
    return nil
}

// Usage
config := ConnectionConfig{
    Timeout: 30 * time.Second,
    Retries: 3,
    Address: "https://api.example.com",
}

if err := config.Validate(); err != nil {
    return err
}
```

## Error Handling Migration

### Configuration Errors

#### Before
```go
type ConfigurationError struct {
    Field   string
    Value   interface{} // Could be anything
    Message string
}

func (e *ConfigurationError) Error() string {
    return fmt.Sprintf("configuration error in field '%s': %s (value: %v)", 
        e.Field, e.Message, e.Value)
}

// Usage
err := &ConfigurationError{
    Field:   "timeout",
    Value:   "invalid", // runtime type assertion needed
    Message: "must be a positive integer",
}
```

#### After
```go
// Type-safe error with generic value type
configErr := NewStringConfigError("timeout", "invalid")

// Or for specific types
timeoutErr := NewIntConfigError("timeout", -5)
enabledErr := NewBoolConfigError("enabled", false) // Note: false might be valid
addressErr := NewStringConfigError("address", "invalid-url")

// Usage with type safety
if configError, ok := err.(*ConfigurationError[StringValue]); ok {
    invalidValue := configError.Value.Value // Type-safe access
    // invalidValue is guaranteed to be string
}
```

### Validation Errors

#### Before
```go
type ValidationError struct {
    Field  string
    Value  interface{}
    Rule   string
    Detail map[string]interface{}
}

// Manual type checking everywhere
func handleValidationError(err error) {
    if valErr, ok := err.(*ValidationError); ok {
        switch v := valErr.Value.(type) {
        case string:
            fmt.Printf("String validation failed: %s\n", v)
        case int:
            fmt.Printf("Integer validation failed: %d\n", v)
        default:
            fmt.Printf("Unknown type validation failed: %v\n", v)
        }
    }
}
```

#### After
```go
// Type-safe validation errors
stringErr := &ValidationError[string]{
    Field: "name",
    Value: "invalid-name!",
    Rule:  "pattern",
    Detail: "must contain only alphanumeric characters",
}

intErr := &ValidationError[int64]{
    Field: "age",
    Value: -5,
    Rule:  "range",
    Detail: "must be between 0 and 150",
}

// Type-safe error handling
func handleValidationError[T any](err *ValidationError[T]) {
    // T is known at compile time
    fmt.Printf("Validation failed for %s: %v (rule: %s)\n", 
        err.Field, err.Value, err.Rule)
    
    // Type-specific handling without type assertions
    switch any(err.Value).(type) {
    case string:
        // Handle string validation error
    case int64:
        // Handle integer validation error
    }
}
```

## Provider Integration Migration

### Message Provider APIs

#### Before
```go
// Generic provider interface with interface{} parameters
type MessageProvider interface {
    SendMessage(params map[string]interface{}) (map[string]interface{}, error)
    Configure(config map[string]interface{}) error
}

// Usage
params := map[string]interface{}{
    "message":     "Hello, world!",
    "model":       "gpt-4",
    "temperature": 0.7,
    "max_tokens":  150,
    "stream":      true,
}

response, err := provider.SendMessage(params)
if err != nil {
    return err
}

// Type assertions required
if content, ok := response["content"].(string); ok {
    fmt.Println(content)
}
```

#### After
```go
// Type-safe provider interface
type MessageProvider interface {
    SendMessage(params MessageParams) (*MessageResponse, error)
    Configure(config ProviderConfig) error
}

type MessageParams struct {
    Message     string  `json:"message" validate:"required"`
    Model       string  `json:"model" validate:"required"`
    Temperature float64 `json:"temperature" validate:"min=0,max=2"`
    MaxTokens   int     `json:"max_tokens" validate:"min=1,max=4096"`
    Stream      bool    `json:"stream"`
}

type MessageResponse struct {
    Content   string            `json:"content"`
    Model     string            `json:"model"`
    Usage     TokenUsage        `json:"usage"`
    FinishReason string         `json:"finish_reason"`
    Metadata  map[string]string `json:"metadata,omitempty"`
}

type TokenUsage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}

// Type-safe usage
params := MessageParams{
    Message:     "Hello, world!",
    Model:       "gpt-4",
    Temperature: 0.7,
    MaxTokens:   150,
    Stream:      true,
}

response, err := provider.SendMessage(params)
if err != nil {
    return err
}

// No type assertions needed
fmt.Println(response.Content)
fmt.Printf("Used %d tokens\n", response.Usage.TotalTokens)
```

## Testing Migration

### Test Data Structures

#### Before
```go
func TestTransportManager(t *testing.T) {
    testCases := []struct {
        name   string
        config map[string]interface{}
        events []map[string]interface{}
        expect map[string]interface{}
    }{
        {
            name: "basic connection",
            config: map[string]interface{}{
                "address": "localhost:8080",
                "timeout": 30,
                "retries": 3,
            },
            events: []map[string]interface{}{
                {
                    "type": "connection",
                    "data": map[string]interface{}{
                        "status":   "connected",
                        "protocol": "tcp",
                    },
                },
            },
            expect: map[string]interface{}{
                "connected": true,
                "events":    1,
            },
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Type assertions everywhere
            address := tc.config["address"].(string)
            timeout := tc.config["timeout"].(int)
            // ... more assertions
        })
    }
}
```

#### After
```go
type TransportTestConfig struct {
    Address string `json:"address"`
    Timeout int    `json:"timeout"`
    Retries int    `json:"retries"`
}

type TransportTestExpectation struct {
    Connected bool `json:"connected"`
    Events    int  `json:"events"`
}

func TestTransportManager(t *testing.T) {
    testCases := []struct {
        name   string
        config TransportTestConfig
        events []TypedTransportEvent[ConnectionEventData]
        expect TransportTestExpectation
    }{
        {
            name: "basic connection",
            config: TransportTestConfig{
                Address: "localhost:8080",
                Timeout: 30,
                Retries: 3,
            },
            events: []TypedTransportEvent[ConnectionEventData]{
                CreateConnectionEvent("conn-1", "connected", func(data *ConnectionEventData) {
                    data.RemoteAddress = "localhost:8080"
                    data.Protocol = "tcp"
                }),
            },
            expect: TransportTestExpectation{
                Connected: true,
                Events:    1,
            },
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // No type assertions needed
            manager := NewTransportManager(tc.config)
            for _, event := range tc.events {
                manager.ProcessEvent(event)
            }
            
            assert.Equal(t, tc.expect.Connected, manager.IsConnected())
            assert.Equal(t, tc.expect.Events, manager.EventCount())
        })
    }
}
```

## Performance Considerations

### Memory Allocation Patterns

#### Before (High Allocation)
```go
// Creates multiple interface{} allocations
data := map[string]interface{}{
    "string_field":  "value",      // String allocation + interface{} boxing
    "int_field":     42,           // Int boxing to interface{}
    "float_field":   3.14,         // Float boxing to interface{}
    "bool_field":    true,         // Bool boxing to interface{}
    "slice_field":   []interface{}{1, 2, 3}, // Slice + element boxing
}

// Multiple type assertions (runtime cost)
if str, ok := data["string_field"].(string); ok {
    // Process string
}
if num, ok := data["int_field"].(int); ok {
    // Process int
}
```

#### After (Low Allocation)
```go
// Direct struct allocation (more efficient)
type DataStruct struct {
    StringField string
    IntField    int
    FloatField  float64
    BoolField   bool
    SliceField  []int
}

data := DataStruct{
    StringField: "value",
    IntField:    42,
    FloatField:  3.14,
    BoolField:   true,
    SliceField:  []int{1, 2, 3},
}

// Direct field access (compile-time optimized)
processString(data.StringField)
processInt(data.IntField)
```

This migration reference provides comprehensive examples for transforming `interface{}` usage to type-safe alternatives while maintaining functionality and improving performance.