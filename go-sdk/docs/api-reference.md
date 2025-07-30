# AG-UI Go SDK API Reference

A comprehensive reference for all public APIs in the AG-UI Go SDK. This documentation covers all interfaces, types, and functions that are part of the public API surface.

## Table of Contents

- [Client APIs](#client-apis)
- [Event System APIs](#event-system-apis)
- [State Management APIs](#state-management-apis)
- [Tools Framework APIs](#tools-framework-apis)
- [Transport Layer APIs](#transport-layer-apis)
- [Monitoring & Observability APIs](#monitoring--observability-apis)
- [Error Handling APIs](#error-handling-apis)
- [Type Definitions](#type-definitions)

---

## Client APIs

The client APIs provide functionality for connecting to AG-UI servers and sending events.

### client Package

#### Client

```go
type Client struct {
    // baseURL is the base URL of the AG-UI server
    baseURL *url.URL
}
```

The main client for communicating with AG-UI servers.

**Example:**
```go
// Create a new client
client, err := client.New(client.Config{
    BaseURL: "https://api.example.com/ag-ui",
})
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

#### Config

```go
type Config struct {
    // BaseURL is the base URL of the AG-UI server
    BaseURL string
}
```

Configuration options for creating a client instance.

**Fields:**
- `BaseURL` (string): Required. The base URL of the AG-UI server

**Example:**
```go
config := client.Config{
    BaseURL: "https://api.example.com/ag-ui",
}
```

#### Methods

##### New(config Config) (*Client, error)

Creates a new AG-UI client with the specified configuration.

**Parameters:**
- `config` (Config): Client configuration

**Returns:**
- `*Client`: The created client instance
- `error`: Configuration validation error

**Errors:**
- Returns `ConfigError` if BaseURL is empty or invalid

**Example:**
```go
client, err := client.New(client.Config{
    BaseURL: "https://api.example.com/ag-ui",
})
if err != nil {
    return fmt.Errorf("failed to create client: %w", err)
}
```

##### SendEvent(ctx context.Context, agentName string, event any) ([]any, error)

Sends an event to the specified agent and returns the response.

**Parameters:**
- `ctx` (context.Context): Request context for cancellation and timeout
- `agentName` (string): Name of the target agent
- `event` (any): Event data to send

**Returns:**
- `[]any`: Response events from the agent
- `error`: Send error

**Errors:**
- Returns `ConfigError` if agentName is empty or event is nil
- Returns `ErrNotImplemented` (currently being implemented)

**Example:**
```go
ctx := context.Background()
responses, err := client.SendEvent(ctx, "my-agent", map[string]interface{}{
    "type": "message",
    "content": "Hello, agent!",
})
if err != nil {
    return fmt.Errorf("failed to send event: %w", err)
}
```

##### Stream(ctx context.Context, agentName string) (<-chan any, error)

Opens a streaming connection to the specified agent.

**Parameters:**
- `ctx` (context.Context): Connection context for cancellation
- `agentName` (string): Name of the target agent

**Returns:**
- `<-chan any`: Channel that receives streaming events
- `error`: Connection error

**Errors:**
- Returns `ConfigError` if agentName is empty
- Returns `ErrNotImplemented` (currently being implemented)

**Example:**
```go
ctx := context.Background()
eventChan, err := client.Stream(ctx, "my-agent")
if err != nil {
    return fmt.Errorf("failed to open stream: %w", err)
}

for event := range eventChan {
    fmt.Printf("Received event: %v\n", event)
}
```

##### Close() error

Closes the client and releases any resources.

**Returns:**
- `error`: Cleanup error (currently always returns nil)

**Example:**
```go
if err := client.Close(); err != nil {
    log.Printf("Error closing client: %v", err)
}
```

---

## Event System APIs

The event system provides comprehensive event validation, processing, and lifecycle management.

### events Package

#### Event Interface

```go
type Event interface {
    Type() EventType
    Timestamp() *int64
    SetTimestamp(timestamp int64)
    ThreadID() string
    RunID() string
    Validate() error
    ToJSON() ([]byte, error)
    ToProtobuf() (*generated.Event, error)
    GetBaseEvent() *BaseEvent
}
```

The core interface that all AG-UI events must implement.

**Methods:**
- `Type()`: Returns the event type
- `Timestamp()`: Returns the event timestamp in Unix milliseconds
- `SetTimestamp(int64)`: Sets the event timestamp
- `ThreadID()`: Returns the thread ID associated with this event
- `RunID()`: Returns the run ID associated with this event
- `Validate()`: Validates the event structure and content
- `ToJSON()`: Serializes the event to JSON for cross-SDK compatibility
- `ToProtobuf()`: Converts the event to its protobuf representation
- `GetBaseEvent()`: Returns the underlying base event

#### EventType

```go
type EventType string
```

Represents the type of AG-UI event.

**Constants:**
```go
const (
    EventTypeTextMessageStart   EventType = "TEXT_MESSAGE_START"
    EventTypeTextMessageContent EventType = "TEXT_MESSAGE_CONTENT"
    EventTypeTextMessageEnd     EventType = "TEXT_MESSAGE_END"
    EventTypeToolCallStart      EventType = "TOOL_CALL_START"
    EventTypeToolCallArgs       EventType = "TOOL_CALL_ARGS"
    EventTypeToolCallEnd        EventType = "TOOL_CALL_END"
    EventTypeStateSnapshot      EventType = "STATE_SNAPSHOT"
    EventTypeStateDelta         EventType = "STATE_DELTA"
    EventTypeMessagesSnapshot   EventType = "MESSAGES_SNAPSHOT"
    EventTypeRaw                EventType = "RAW"
    EventTypeCustom             EventType = "CUSTOM"
    EventTypeRunStarted         EventType = "RUN_STARTED"
    EventTypeRunFinished        EventType = "RUN_FINISHED"
    EventTypeRunError           EventType = "RUN_ERROR"
    EventTypeStepStarted        EventType = "STEP_STARTED"
    EventTypeStepFinished       EventType = "STEP_FINISHED"
    EventTypeUnknown            EventType = "UNKNOWN"
)
```

#### BaseEvent

```go
type BaseEvent struct {
    EventType   EventType `json:"type"`
    TimestampMs *int64    `json:"timestamp,omitempty"`
    RawEvent    any       `json:"rawEvent,omitempty"`
}
```

Provides common fields and functionality for all events.

**Example:**
```go
// Create a new base event
baseEvent := events.NewBaseEvent(events.EventTypeTextMessageStart)
fmt.Printf("Event type: %s\n", baseEvent.Type())
fmt.Printf("Timestamp: %d\n", *baseEvent.Timestamp())
```

#### Functions

##### NewBaseEvent(eventType EventType) *BaseEvent

Creates a new base event with the given type and current timestamp.

**Parameters:**
- `eventType` (EventType): The type of event to create

**Returns:**
- `*BaseEvent`: New base event instance

**Example:**
```go
event := events.NewBaseEvent(events.EventTypeTextMessageStart)
```

##### ValidateSequence(events []Event) error

Validates a sequence of events according to AG-UI protocol rules.

**Parameters:**
- `events` ([]Event): Sequence of events to validate

**Returns:**
- `error`: Validation error if the sequence is invalid

**Validation Rules:**
- Ensures proper event lifecycle (start/end pairs)
- Validates run and step state transitions
- Checks for orphaned events

**Example:**
```go
eventSequence := []events.Event{
    runStartEvent,
    messageStartEvent,
    messageContentEvent,
    messageEndEvent,
    runFinishEvent,
}

if err := events.ValidateSequence(eventSequence); err != nil {
    return fmt.Errorf("invalid event sequence: %w", err)
}
```

### Event Validation System

#### Enhanced Validation Features

The event validation system provides advanced validation capabilities:

**Parallel Validation:**
```go
// Validate multiple events concurrently
validator := events.NewParallelValidator(events.ParallelValidatorConfig{
    MaxWorkers: 10,
    BufferSize: 100,
})

results := validator.ValidateParallel(ctx, eventBatch)
for result := range results {
    if result.Error != nil {
        log.Printf("Validation error for event %s: %v", result.EventID, result.Error)
    }
}
```

**Custom Validation Rules:**
```go
// Register custom validation rules
rule := &events.CustomValidationRule{
    Name: "business-logic-validation",
    Validator: func(ctx context.Context, event events.Event) error {
        // Custom business logic validation
        return validateBusinessRules(event)
    },
}

validator.RegisterRule(rule)
```

**Validation Metrics:**
```go
// Access validation metrics
metrics := validator.GetMetrics()
fmt.Printf("Total validations: %d\n", metrics.TotalValidations)
fmt.Printf("Success rate: %.2f%%\n", metrics.SuccessRate)
fmt.Printf("Average latency: %v\n", metrics.AverageLatency)
```

---

## State Management APIs

The state management system provides versioned state with history, transactions, and efficient copy-on-write semantics.

### state Package

#### StateStore

```go
type StateStore struct {
    // Implementation uses sharded state for fine-grained locking
    shards     []*stateShard
    shardCount uint32
    version    int64
    history    []*StateVersion
    // ... other fields
}
```

The main state store that provides versioned state management with history and transactions.

**Example:**
```go
// Create a new state store
store := state.NewStateStore(
    state.WithMaxHistory(100),
    state.WithShardCount(16),
    state.WithLogger(logger),
)
```

#### StateStoreOption

```go
type StateStoreOption func(*StateStore)
```

Configuration options for StateStore.

**Available Options:**
- `WithMaxHistory(int)`: Sets the maximum number of history entries
- `WithShardCount(uint32)`: Sets the number of shards (must be power of 2)
- `WithLogger(Logger)`: Sets the logger for the store
- `WithSubscriptionTTL(time.Duration)`: Sets subscription time-to-live
- `WithCleanupInterval(time.Duration)`: Sets cleanup interval

#### Methods

##### NewStateStore(options ...StateStoreOption) *StateStore

Creates a new state store instance with optional configuration.

**Parameters:**
- `options` (...StateStoreOption): Configuration options

**Returns:**
- `*StateStore`: New state store instance

**Example:**
```go
store := state.NewStateStore(
    state.WithMaxHistory(500),
    state.WithShardCount(32),
    state.WithSubscriptionTTL(30*time.Minute),
)
```

##### Get(path string) (interface{}, error)

Retrieves a value at the specified path using JSON Pointer syntax.

**Parameters:**
- `path` (string): JSON Pointer path to the value

**Returns:**
- `interface{}`: The value at the specified path
- `error`: Retrieval error

**Path Examples:**
- `""` or `"/"`: Root object
- `"/users"`: Top-level users object
- `"/users/123"`: User with ID 123
- `"/users/123/name"`: Name field of user 123

**Example:**
```go
// Get a nested value
name, err := store.Get("/users/123/name")
if err != nil {
    return fmt.Errorf("failed to get user name: %w", err)
}
fmt.Printf("User name: %s\n", name)
```

##### Set(path string, value interface{}) error

Updates a value at the specified path.

**Parameters:**
- `path` (string): JSON Pointer path to update
- `value` (interface{}): New value to set

**Returns:**
- `error`: Update error

**Example:**
```go
// Set a nested value
err := store.Set("/users/123/name", "John Doe")
if err != nil {
    return fmt.Errorf("failed to set user name: %w", err)
}
```

##### Delete(path string) error

Removes a value at the specified path.

**Parameters:**
- `path` (string): JSON Pointer path to delete

**Returns:**
- `error`: Deletion error

**Example:**
```go
// Delete a user
err := store.Delete("/users/123")
if err != nil {
    return fmt.Errorf("failed to delete user: %w", err)
}
```

##### ApplyPatch(patch JSONPatch) error

Applies a JSON Patch to the state.

**Parameters:**
- `patch` (JSONPatch): JSON Patch operations to apply

**Returns:**
- `error`: Patch application error

**Example:**
```go
patch := state.JSONPatch{
    {
        Op:    state.JSONPatchOpAdd,
        Path:  "/users/456",
        Value: map[string]interface{}{
            "name":  "Jane Smith",
            "email": "jane@example.com",
        },
    },
    {
        Op:    state.JSONPatchOpReplace,
        Path:  "/users/123/email",
        Value: "john.doe@example.com",
    },
}

err := store.ApplyPatch(patch)
if err != nil {
    return fmt.Errorf("failed to apply patch: %w", err)
}
```

##### Subscribe(path string, callback SubscriptionCallback) func()

Registers a callback for state changes at the specified path.

**Parameters:**
- `path` (string): Path to monitor for changes
- `callback` (SubscriptionCallback): Function to call on changes

**Returns:**
- `func()`: Unsubscribe function

**Example:**
```go
// Subscribe to user changes
unsubscribe := store.Subscribe("/users/*", func(change state.StateChange) {
    fmt.Printf("User changed: %s %s\n", change.Path, change.Operation)
})

// Later, unsubscribe
defer unsubscribe()
```

##### Begin() *StateTransaction

Starts a new transaction for atomic state changes.

**Returns:**
- `*StateTransaction`: New transaction instance

**Example:**
```go
// Start a transaction
tx := store.Begin()

// Apply changes within transaction
patch := state.JSONPatch{
    {Op: state.JSONPatchOpAdd, Path: "/temp", Value: "data"},
}
err := tx.Apply(patch)
if err != nil {
    tx.Rollback()
    return err
}

// Commit the transaction
err = tx.Commit()
if err != nil {
    return fmt.Errorf("failed to commit transaction: %w", err)
}
```

#### StateTransaction

```go
type StateTransaction struct {
    store     *StateStore
    patches   JSONPatch
    snapshot  map[string]interface{}
    committed bool
    mu        sync.Mutex
}
```

Represents an atomic transaction for state changes.

##### Methods

###### Apply(patch JSONPatch) error

Adds a patch to the transaction.

**Parameters:**
- `patch` (JSONPatch): Patch operations to add

**Returns:**
- `error`: Application error

###### Commit() error

Commits the transaction, applying all patches atomically.

**Returns:**
- `error`: Commit error

###### Rollback() error

Discards the transaction without applying changes.

**Returns:**
- `error`: Rollback error

#### StateChange

```go
type StateChange struct {
    Path      string
    OldValue  interface{}
    NewValue  interface{}
    Operation string
    Timestamp time.Time
}
```

Represents a change to the state.

#### JSONPatch Operations

```go
type JSONPatchOperation struct {
    Op    JSONPatchOp `json:"op"`
    Path  string      `json:"path"`
    Value interface{} `json:"value,omitempty"`
    From  string      `json:"from,omitempty"`
}

type JSONPatchOp string

const (
    JSONPatchOpAdd     JSONPatchOp = "add"
    JSONPatchOpRemove  JSONPatchOp = "remove"
    JSONPatchOpReplace JSONPatchOp = "replace"
    JSONPatchOpMove    JSONPatchOp = "move"
    JSONPatchOpCopy    JSONPatchOp = "copy"
    JSONPatchOpTest    JSONPatchOp = "test"
)
```

**Example:**
```go
// Create a comprehensive patch
patch := state.JSONPatch{
    // Add a new user
    {
        Op:   state.JSONPatchOpAdd,
        Path: "/users/789",
        Value: map[string]interface{}{
            "name":   "Bob Wilson",
            "email":  "bob@example.com",
            "active": true,
        },
    },
    // Update existing user's status
    {
        Op:    state.JSONPatchOpReplace,
        Path:  "/users/123/active",
        Value: false,
    },
    // Remove a user
    {
        Op:   state.JSONPatchOpRemove,
        Path: "/users/456",
    },
    // Move data
    {
        Op:   state.JSONPatchOpMove,
        Path: "/archive/users/456",
        From: "/users/456",
    },
}
```

---

## Tools Framework APIs

The tools framework provides a comprehensive system for defining, registering, and executing AI agent tools.

### tools Package

#### Tool

```go
type Tool struct {
    ID           string             `json:"id"`
    Name         string             `json:"name"`
    Description  string             `json:"description"`
    Version      string             `json:"version"`
    Schema       *ToolSchema        `json:"schema"`
    Metadata     *ToolMetadata      `json:"metadata,omitempty"`
    Executor     ToolExecutor       `json:"-"`
    Capabilities *ToolCapabilities  `json:"capabilities,omitempty"`
}
```

Represents a function that can be called by an AI agent.

**Example:**
```go
// Create a text analysis tool
tool := &tools.Tool{
    ID:          "text-analyzer",
    Name:        "Text Analyzer",
    Description: "Analyzes text for sentiment, keywords, and statistics",
    Version:     "1.0.0",
    Schema: &tools.ToolSchema{
        Type: "object",
        Properties: map[string]*tools.Property{
            "text": {
                Type:        "string",
                Description: "Text to analyze",
                MinLength:   intPtr(1),
                MaxLength:   intPtr(10000),
            },
        },
        Required: []string{"text"},
    },
    Executor: &TextAnalyzerExecutor{},
    Capabilities: &tools.ToolCapabilities{
        Cacheable: true,
        Timeout:   30 * time.Second,
    },
}
```

#### ToolExecutor Interface

```go
type ToolExecutor interface {
    Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error)
}
```

The interface that tool implementations must satisfy.

**Example Implementation:**
```go
type TextAnalyzerExecutor struct{}

func (e *TextAnalyzerExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
    // Check context cancellation
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    
    // Extract and validate parameters
    text, ok := params["text"].(string)
    if !ok {
        return &tools.ToolExecutionResult{
            Success: false,
            Error:   "text parameter must be a string",
        }, nil
    }
    
    // Perform analysis
    analysis := analyzeText(text)
    
    return &tools.ToolExecutionResult{
        Success:   true,
        Data:      analysis,
        Duration:  time.Since(start),
        Timestamp: time.Now(),
    }, nil
}
```

#### StreamingToolExecutor Interface

```go
type StreamingToolExecutor interface {
    ToolExecutor
    ExecuteStream(ctx context.Context, params map[string]interface{}) (<-chan *ToolStreamChunk, error)
}
```

Extended interface for tools that produce streaming output.

**Example Implementation:**
```go
type LogReaderExecutor struct{}

func (e *LogReaderExecutor) ExecuteStream(ctx context.Context, params map[string]interface{}) (<-chan *tools.ToolStreamChunk, error) {
    stream := make(chan *tools.ToolStreamChunk)
    
    go func() {
        defer close(stream)
        
        file := params["file"].(string)
        lines := readFileByLine(file)
        
        for i, line := range lines {
            select {
            case <-ctx.Done():
                return
            case stream <- &tools.ToolStreamChunk{
                Type:      "data",
                Data:      line,
                Index:     i,
                Timestamp: time.Now(),
            }:
            }
        }
    }()
    
    return stream, nil
}
```

#### ToolSchema

```go
type ToolSchema struct {
    Type                 string                  `json:"type"`
    Properties           map[string]*Property    `json:"properties,omitempty"`
    Required             []string                `json:"required,omitempty"`
    AdditionalProperties *bool                   `json:"additionalProperties,omitempty"`
    Description          string                  `json:"description,omitempty"`
}
```

Defines the JSON Schema for tool parameters.

#### Property

```go
type Property struct {
    Type        string        `json:"type,omitempty"`
    Description string        `json:"description,omitempty"`
    Format      string        `json:"format,omitempty"`
    Enum        []interface{} `json:"enum,omitempty"`
    Default     interface{}   `json:"default,omitempty"`
    Minimum     *float64      `json:"minimum,omitempty"`
    Maximum     *float64      `json:"maximum,omitempty"`
    MinLength   *int          `json:"minLength,omitempty"`
    MaxLength   *int          `json:"maxLength,omitempty"`
    Pattern     string        `json:"pattern,omitempty"`
    Items       *Property     `json:"items,omitempty"`
    // ... additional JSON Schema features
}
```

Represents a single parameter in the tool schema.

**Example Schema:**
```go
schema := &tools.ToolSchema{
    Type: "object",
    Properties: map[string]*tools.Property{
        "query": {
            Type:        "string",
            Description: "Search query",
            MinLength:   intPtr(1),
            MaxLength:   intPtr(500),
        },
        "limit": {
            Type:        "integer",
            Description: "Maximum results",
            Minimum:     float64Ptr(1),
            Maximum:     float64Ptr(100),
            Default:     10,
        },
        "filters": {
            Type: "array",
            Items: &tools.Property{
                Type: "string",
                Enum: []interface{}{"active", "inactive", "pending"},
            },
        },
    },
    Required: []string{"query"},
}
```

#### ToolExecutionResult

```go
type ToolExecutionResult struct {
    Success   bool                   `json:"success"`
    Data      interface{}            `json:"data,omitempty"`
    Error     string                 `json:"error,omitempty"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
    Duration  time.Duration          `json:"duration,omitempty"`
    Timestamp time.Time              `json:"timestamp"`
}
```

Represents the outcome of a tool execution.

#### Tool Registry

```go
// Register a tool
registry := tools.NewRegistry()
err := registry.Register(tool)
if err != nil {
    return fmt.Errorf("failed to register tool: %w", err)
}

// Find tools
results := registry.Find(&tools.ToolFilter{
    Tags: []string{"text", "analysis"},
    Capabilities: &tools.ToolCapabilities{
        Cacheable: true,
    },
})

// Execute a tool
result, err := registry.Execute(ctx, "text-analyzer", map[string]interface{}{
    "text": "Hello, world!",
})
```

---

## Transport Layer APIs

The transport layer provides multiple communication protocols for connecting clients and servers.

### transport Package

#### Common Types

```go
type Config struct {
    Address     string        `json:"address"`
    TLSConfig   *tls.Config   `json:"-"`
    CertFile    string        `json:"cert_file,omitempty"`
    KeyFile     string        `json:"key_file,omitempty"`
    Timeout     time.Duration `json:"timeout"`
    BufferSize  int           `json:"buffer_size"`
}
```

#### WebSocket Transport

##### SecurityConfig

```go
type SecurityConfig struct {
    RequireAuth       bool                 `json:"require_auth"`
    AuthTimeout       time.Duration        `json:"auth_timeout"`
    TokenValidator    TokenValidator       `json:"-"`
    AllowedOrigins    []string            `json:"allowed_origins"`
    StrictOriginCheck bool                `json:"strict_origin_check"`
    GlobalRateLimit   float64             `json:"global_rate_limit"`
    ClientRateLimit   float64             `json:"client_rate_limit"`
    MaxConnections    int                 `json:"max_connections"`
    RequireTLS        bool                `json:"require_tls"`
    MinTLSVersion     uint16              `json:"min_tls_version"`
    MaxMessageSize    int64               `json:"max_message_size"`
    AuditLogger       WSAuditLogger       `json:"-"`
}
```

**Example Configuration:**
```go
securityConfig := &websocket.SecurityConfig{
    RequireAuth:       true,
    AuthTimeout:       30 * time.Second,
    StrictOriginCheck: true,
    AllowedOrigins: []string{
        "https://app.example.com",
        "https://admin.example.com",
    },
    GlobalRateLimit:   1000.0,
    ClientRateLimit:   100.0,
    MaxConnections:    10000,
    RequireTLS:        true,
    MinTLSVersion:     tls.VersionTLS12,
    MaxMessageSize:    1024 * 1024, // 1MB
    TokenValidator:    jwtValidator,
    AuditLogger:       auditLogger,
}
```

##### JWT Token Validation

```go
type JWTTokenValidator struct {
    secretKey     []byte
    publicKey     *rsa.PublicKey
    issuer        string
    audience      string
    signingMethod jwt.SigningMethod
}

// Create HMAC-based validator
validator := websocket.NewJWTTokenValidator(
    []byte("your-secret-key"),
    "your-issuer",
)

// Create RSA-based validator
validator := websocket.NewJWTTokenValidatorRSA(
    publicKey,
    "your-issuer",
    "your-audience",
)
```

##### Security Manager

```go
// Create security manager
securityManager := websocket.NewSecurityManager(securityConfig)

// Validate upgrade request
authContext, err := securityManager.ValidateUpgrade(w, r)
if err != nil {
    http.Error(w, "Upgrade validation failed", http.StatusUnauthorized)
    return
}

// Create upgrader
upgrader := securityManager.CreateUpgrader()

// Upgrade connection
conn, err := upgrader.Upgrade(w, r, nil)
if err != nil {
    return
}

// Secure the connection
secureConn := securityManager.SecureConnection(conn, authContext, r)
```

#### Server-Sent Events (SSE) Transport

##### SSE Configuration

```go
type Config struct {
    MaxConnections    int           `json:"max_connections"`
    HeartbeatInterval time.Duration `json:"heartbeat_interval"`
    BufferSize        int           `json:"buffer_size"`
    EnableGZip        bool          `json:"enable_gzip"`
    AllowedOrigins    []string      `json:"allowed_origins"`
    RetryInterval     time.Duration `json:"retry_interval"`
}
```

**Example:**
```go
sseConfig := &sse.Config{
    MaxConnections:    1000,
    HeartbeatInterval: 30 * time.Second,
    BufferSize:        256,
    EnableGZip:        true,
    AllowedOrigins: []string{
        "https://app.example.com",
    },
    RetryInterval: 5 * time.Second,
}
```

---

## Monitoring & Observability APIs

Comprehensive monitoring capabilities with OpenTelemetry integration, Prometheus metrics, and alerting.

### monitoring Package

#### MonitoringIntegration

```go
type MonitoringIntegration struct {
    config            *Config
    metricsCollector  events.MetricsCollector
    prometheusExporter *PrometheusExporter
    alertManager      *AlertManager
    grafanaGenerator  *GrafanaDashboardGenerator
    tracerProvider    *sdktrace.TracerProvider
    meterProvider     metric.MeterProvider
    tracer            trace.Tracer
    meter             metric.Meter
    slaMonitor        *SLAMonitor
}
```

The main monitoring integration that provides enterprise-grade monitoring capabilities.

#### Config

```go
type Config struct {
    // Base metrics configuration
    MetricsConfig     *events.MetricsConfig `json:"metrics_config"`
    
    // Prometheus configuration
    PrometheusPort    int                   `json:"prometheus_port"`
    PrometheusPath    string                `json:"prometheus_path"`
    CustomLabels      map[string]string     `json:"custom_labels"`
    
    // OpenTelemetry configuration
    OTLPEndpoint      string                `json:"otlp_endpoint"`
    OTLPHeaders       map[string]string     `json:"otlp_headers"`
    TraceSampleRate   float64               `json:"trace_sample_rate"`
    EnableTracing     bool                  `json:"enable_tracing"`
    EnableMetrics     bool                  `json:"enable_metrics"`
    ServiceName       string                `json:"service_name"`
    ServiceVersion    string                `json:"service_version"`
    Environment       string                `json:"environment"`
    
    // Alerting configuration
    AlertThresholds   AlertThresholds       `json:"alert_thresholds"`
    AlertWebhookURL   string                `json:"alert_webhook_url"`
    EnableRunbooks    bool                  `json:"enable_runbooks"`
    
    // SLA configuration
    SLATargets        map[string]SLATarget  `json:"sla_targets"`
    SLAWindowSize     time.Duration         `json:"sla_window_size"`
    EnableSLAReports  bool                  `json:"enable_sla_reports"`
    
    // Dashboard configuration
    GrafanaURL        string                `json:"grafana_url"`
    GrafanaAPIKey     string                `json:"grafana_api_key"`
    AutoGenerateDash  bool                  `json:"auto_generate_dash"`
}
```

**Example Configuration:**
```go
config := &monitoring.Config{
    MetricsConfig: events.ProductionMetricsConfig(),
    
    // Prometheus
    PrometheusPort: 9090,
    PrometheusPath: "/metrics",
    CustomLabels: map[string]string{
        "component": "event-validation",
        "version":   "1.0.0",
    },
    
    // OpenTelemetry
    OTLPEndpoint:    "jaeger:4317",
    TraceSampleRate: 0.1,
    EnableTracing:   true,
    EnableMetrics:   true,
    ServiceName:     "ag-ui-events",
    ServiceVersion:  "1.0.0",
    Environment:     "production",
    
    // Alerting
    AlertThresholds: monitoring.AlertThresholds{
        ErrorRatePercent:    5.0,
        LatencyP99Millis:    100.0,
        MemoryUsagePercent:  80.0,
        ThroughputMinEvents: 100.0,
    },
    AlertWebhookURL: "https://alerts.example.com/webhook",
    EnableRunbooks:  true,
    
    // SLA Targets
    SLATargets: map[string]monitoring.SLATarget{
        "validation_latency": {
            Name:              "Event Validation Latency",
            TargetValue:       100.0,
            Unit:              "milliseconds",
            MeasurementWindow: 5 * time.Minute,
            AlertOnViolation:  true,
        },
    },
    
    // Grafana
    GrafanaURL:       "https://grafana.example.com",
    GrafanaAPIKey:    "your-api-key",
    AutoGenerateDash: true,
}
```

#### Methods

##### NewMonitoringIntegration(config *Config) (*MonitoringIntegration, error)

Creates a new monitoring integration with the specified configuration.

**Example:**
```go
monitoring, err := monitoring.NewMonitoringIntegration(config)
if err != nil {
    return fmt.Errorf("failed to create monitoring: %w", err)
}
defer monitoring.Shutdown()
```

##### RecordEventWithContext(ctx context.Context, duration time.Duration, success bool, attributes map[string]string)

Records an event with OpenTelemetry context and tracing.

**Example:**
```go
attributes := map[string]string{
    "event_type": "validation",
    "agent_id":   "user-agent-123",
}

monitoring.RecordEventWithContext(ctx, duration, true, attributes)
```

##### GetEnhancedDashboardData() *EnhancedDashboardData

Returns enhanced dashboard data with SLA information and active alerts.

**Example:**
```go
dashboardData := monitoring.GetEnhancedDashboardData()
fmt.Printf("Active alerts: %d\n", len(dashboardData.ActiveAlerts))
fmt.Printf("SLA status: %+v\n", dashboardData.SLAStatus)
```

#### Alert System

```go
type Alert struct {
    Name        string            `json:"name"`
    Severity    string            `json:"severity"`
    Description string            `json:"description"`
    TriggeredAt time.Time         `json:"triggered_at"`
    Labels      map[string]string `json:"labels"`
    RunbookID   string            `json:"runbook_id"`
}

type AlertThresholds struct {
    ErrorRatePercent      float64 `json:"error_rate_percent"`
    LatencyP99Millis      float64 `json:"latency_p99_millis"`
    MemoryUsagePercent    float64 `json:"memory_usage_percent"`
    ThroughputMinEvents   float64 `json:"throughput_min_events"`
}
```

#### SLA Monitoring

```go
type SLATarget struct {
    Name              string        `json:"name"`
    Description       string        `json:"description"`
    TargetValue       float64       `json:"target_value"`
    Unit              string        `json:"unit"`
    MeasurementWindow time.Duration `json:"measurement_window"`
    AlertOnViolation  bool          `json:"alert_on_violation"`
}

type SLAStatus struct {
    Target           SLATarget      `json:"target"`
    CurrentValue     float64        `json:"current_value"`
    IsViolated       bool           `json:"is_violated"`
    ViolationPercent float64        `json:"violation_percent"`
    LastUpdated      time.Time      `json:"last_updated"`
    TrendDirection   string         `json:"trend_direction"`
    WindowData       []SLADataPoint `json:"window_data"`
}
```

---

## Error Handling APIs

Comprehensive error handling system with typed errors and context preservation.

### errors Package

#### Error Types

```go
type ValidationError struct {
    Field   string `json:"field"`
    Value   string `json:"value"`
    Message string `json:"message"`
    Code    string `json:"code"`
}

type AuthenticationError struct {
    Reason    string            `json:"reason"`
    Context   map[string]string `json:"context"`
    Timestamp time.Time         `json:"timestamp"`
}

type RateLimitError struct {
    Limit     int           `json:"limit"`
    Remaining int           `json:"remaining"`
    ResetTime time.Time     `json:"reset_time"`
    Window    time.Duration `json:"window"`
}
```

#### Error Context

```go
type ErrorContext struct {
    RequestID   string            `json:"request_id"`
    UserID      string            `json:"user_id,omitempty"`
    Operation   string            `json:"operation"`
    Component   string            `json:"component"`
    Timestamp   time.Time         `json:"timestamp"`
    Metadata    map[string]string `json:"metadata,omitempty"`
    StackTrace  []string          `json:"stack_trace,omitempty"`
}
```

**Example Usage:**
```go
// Create error with context
ctx := &errors.ErrorContext{
    RequestID: "req-123",
    UserID:    "user-456",
    Operation: "event-validation",
    Component: "validator",
    Timestamp: time.Now(),
    Metadata: map[string]string{
        "event_type": "TEXT_MESSAGE_START",
        "agent_id":   "agent-789",
    },
}

err := &errors.ValidationError{
    Field:   "content",
    Value:   "invalid-content",
    Message: "Content contains prohibited elements",
    Code:    "CONTENT_VALIDATION_FAILED",
}

contextualError := errors.WithContext(err, ctx)
```

---

## Type Definitions

### Common Types

#### Duration Extensions

```go
// Custom duration type with JSON marshaling
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error)
func (d *Duration) UnmarshalJSON(data []byte) error
```

#### Utility Functions

```go
// Helper functions for pointer types
func StringPtr(s string) *string
func IntPtr(i int) *int
func Float64Ptr(f float64) *float64
func BoolPtr(b bool) *bool

// Deep copy functions
func DeepCopy(src interface{}) interface{}
func DeepCopyMap(src map[string]interface{}) map[string]interface{}
func DeepCopySlice(src []interface{}) []interface{}
```

### Configuration Types

#### Common Configuration Pattern

```go
type Config struct {
    // Network settings
    Address    string        `json:"address" env:"ADDRESS" default:":8080"`
    TLSConfig  *tls.Config   `json:"-"`
    
    // Timeouts
    ReadTimeout  Duration `json:"read_timeout" env:"READ_TIMEOUT" default:"30s"`
    WriteTimeout Duration `json:"write_timeout" env:"WRITE_TIMEOUT" default:"30s"`
    
    // Security
    EnableAuth  bool     `json:"enable_auth" env:"ENABLE_AUTH" default:"true"`
    SecretKey   string   `json:"-" env:"SECRET_KEY,required"`
    
    // Observability
    LogLevel    string   `json:"log_level" env:"LOG_LEVEL" default:"info"`
    MetricsPort int      `json:"metrics_port" env:"METRICS_PORT" default:"9090"`
}
```

**Configuration Loading:**
```go
// Load configuration from environment and defaults
func LoadConfig() (*Config, error) {
    var cfg Config
    
    // Set defaults
    cfg.Address = ":8080"
    cfg.ReadTimeout = Duration(30 * time.Second)
    cfg.WriteTimeout = Duration(30 * time.Second)
    cfg.EnableAuth = true
    cfg.LogLevel = "info"
    cfg.MetricsPort = 9090
    
    // Override with environment variables
    if addr := os.Getenv("ADDRESS"); addr != "" {
        cfg.Address = addr
    }
    
    if secretKey := os.Getenv("SECRET_KEY"); secretKey == "" {
        return nil, errors.New("SECRET_KEY environment variable is required")
    } else {
        cfg.SecretKey = secretKey
    }
    
    return &cfg, nil
}
```

---

## Best Practices

### Error Handling

1. **Always use typed errors** for different error categories
2. **Preserve error context** throughout the call stack
3. **Log errors with sufficient detail** for debugging
4. **Return user-friendly messages** while logging technical details

```go
func (s *Service) ProcessEvent(ctx context.Context, event Event) error {
    // Add operation context
    ctx = errors.WithOperation(ctx, "process-event")
    
    if err := s.validator.Validate(event); err != nil {
        // Wrap with context
        return errors.Wrap(err, "event validation failed")
    }
    
    if err := s.processor.Process(ctx, event); err != nil {
        // Log detailed error
        s.logger.Error("event processing failed",
            "error", err,
            "event_id", event.ID(),
            "event_type", event.Type(),
        )
        
        // Return user-friendly error
        return errors.New("failed to process event")
    }
    
    return nil
}
```

### Context Usage

1. **Always pass context** to methods that might be canceled
2. **Use context for request-scoped values** (user ID, request ID)
3. **Respect context cancellation** in long-running operations

```go
func (s *Service) LongRunningOperation(ctx context.Context) error {
    for i := 0; i < 1000; i++ {
        // Check for cancellation
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
        
        // Do work
        if err := s.processItem(ctx, i); err != nil {
            return err
        }
    }
    
    return nil
}
```

### Resource Management

1. **Always close resources** using defer
2. **Use context for timeout control**
3. **Implement graceful shutdown**

```go
func (s *Service) Start(ctx context.Context) error {
    // Start background workers
    workerCtx, workerCancel := context.WithCancel(ctx)
    defer workerCancel()
    
    // Start HTTP server
    server := &http.Server{
        Addr:    s.config.Address,
        Handler: s.handler,
    }
    
    go func() {
        <-ctx.Done()
        // Graceful shutdown
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        server.Shutdown(shutdownCtx)
    }()
    
    return server.ListenAndServe()
}
```

---

This API reference provides comprehensive documentation for all public APIs in the AG-UI Go SDK. For more examples and usage patterns, see the [examples directory](../examples/) and the [development guide](../DEVELOPMENT.md).