# Common Code Patterns in Encoding System

## 1. Mutex Locking Patterns

### Current Usage
```go
// Read lock pattern (found 15+ times)
r.mu.RLock()
defer r.mu.RUnlock()

// Write lock pattern (found 10+ times)
r.mu.Lock()
defer r.mu.Unlock()

// Conditional locking (found in streaming)
func (e *JSONEncoder) Encode(event events.Event) ([]byte, error) {
    e.mu.Lock()
    defer e.mu.Unlock()
    // ...
}
```

### Extraction Opportunity
Create a `SafeOperation` helper:
```go
type SafeOperation struct {
    mu sync.RWMutex
}

func (s *SafeOperation) Read(fn func() error) error {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return fn()
}

func (s *SafeOperation) Write(fn func() error) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    return fn()
}
```

## 2. Nil Checking Patterns

### Current Usage
```go
// Options nil check (found 20+ times)
if options == nil {
    options = &EncodingOptions{}
}

// Event nil check (found 10+ times)
if event == nil {
    return nil, &encoding.EncodingError{
        Format:  "json",
        Message: "cannot encode nil event",
    }
}

// Factory nil check (found 5+ times)
if factory == nil {
    return fmt.Errorf("factory cannot be nil")
}
```

### Extraction Opportunity
```go
func DefaultIfNil[T any](value *T, defaultValue *T) *T {
    if value == nil {
        return defaultValue
    }
    return value
}

func RequireNonNil[T any](value *T, name string) error {
    if value == nil {
        return fmt.Errorf("%s cannot be nil", name)
    }
    return nil
}
```

## 3. Error Creation Patterns

### Current Usage
```go
// Format-specific errors (found 30+ times)
return &encoding.EncodingError{
    Format:  "json",
    Event:   event,
    Message: "failed to encode event",
    Cause:   err,
}

// Simple errors (found 25+ times)
return fmt.Errorf("no encoder registered for content type: %s", contentType)

// Validation errors (found 15+ times)
return &ValidationError{
    Field:   "contentType",
    Value:   contentType,
    Message: "unsupported content type",
}
```

### Extraction Opportunity
```go
type ErrorFactory struct {
    format string
}

func (ef *ErrorFactory) EncodingError(event events.Event, message string, cause error) error {
    return &encoding.EncodingError{
        Format:  ef.format,
        Event:   event,
        Message: message,
        Cause:   cause,
    }
}

func (ef *ErrorFactory) DecodingError(data []byte, message string, cause error) error {
    return &encoding.DecodingError{
        Format:  ef.format,
        Data:    data,
        Message: message,
        Cause:   cause,
    }
}
```

## 4. Content Type Handling Patterns

### Current Usage
```go
// Alias resolution (found 8+ times)
canonical := r.resolveAlias(mimeType)

// Parameter stripping (found 5+ times)
if idx := strings.Index(mimeType, ";"); idx > 0 {
    base := strings.TrimSpace(mimeType[:idx])
    // ...
}

// Case normalization (found 6+ times)
normalized := strings.ToLower(contentType)
```

### Extraction Opportunity
```go
type ContentTypeHelper struct{}

func (c *ContentTypeHelper) Normalize(contentType string) string {
    base := c.StripParameters(contentType)
    return strings.ToLower(strings.TrimSpace(base))
}

func (c *ContentTypeHelper) StripParameters(contentType string) string {
    if idx := strings.Index(contentType, ";"); idx > 0 {
        return contentType[:idx]
    }
    return contentType
}

func (c *ContentTypeHelper) ParseParameters(contentType string) (string, map[string]string) {
    // Extract base type and parameters
}
```

## 5. Validation Patterns

### Current Usage
```go
// Event validation (found 12+ times)
if e.options.ValidateOutput {
    if err := event.Validate(); err != nil {
        return nil, &encoding.EncodingError{
            Format:  "json",
            Event:   event,
            Message: "event validation failed",
            Cause:   err,
        }
    }
}

// Size checking (found 10+ times)
if e.options.MaxSize > 0 && int64(len(data)) > e.options.MaxSize {
    return nil, &encoding.EncodingError{
        Format:  "json",
        Event:   event,
        Message: fmt.Sprintf("encoded event exceeds max size of %d bytes", e.options.MaxSize),
    }
}
```

### Extraction Opportunity
```go
type ValidationHelper struct {
    format string
}

func (v *ValidationHelper) ValidateEvent(event events.Event, options *EncodingOptions) error {
    if options.ValidateOutput && event != nil {
        if err := event.Validate(); err != nil {
            return &encoding.EncodingError{
                Format:  v.format,
                Event:   event,
                Message: "event validation failed",
                Cause:   err,
            }
        }
    }
    return nil
}

func (v *ValidationHelper) CheckSize(data []byte, maxSize int64, context string) error {
    if maxSize > 0 && int64(len(data)) > maxSize {
        return fmt.Errorf("%s exceeds max size of %d bytes", context, maxSize)
    }
    return nil
}
```

## 6. Stream Management Patterns

### Current Usage
```go
// Stream lifecycle (found 8+ times)
if err := encoder.StartStream(output); err != nil {
    return fmt.Errorf("failed to start output stream: %w", err)
}
defer encoder.EndStream()

// Active state checking (found 6+ times)
usc.mu.Lock()
if usc.active {
    usc.mu.Unlock()
    return fmt.Errorf("stream already active")
}
usc.active = true
usc.mu.Unlock()
```

### Extraction Opportunity
```go
type StreamLifecycle struct {
    mu     sync.Mutex
    active bool
}

func (s *StreamLifecycle) Start() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if s.active {
        return errors.New("stream already active")
    }
    s.active = true
    return nil
}

func (s *StreamLifecycle) End() {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.active = false
}

func (s *StreamLifecycle) WithStream(fn func() error) error {
    if err := s.Start(); err != nil {
        return err
    }
    defer s.End()
    return fn()
}
```

## 7. Registry Pattern

### Current Usage
```go
// Type registration (found 15+ times)
f.mu.Lock()
defer f.mu.Unlock()
f.encoderCtors[contentType] = ctor
f.updateSupportedTypes()

// Type lookup (found 20+ times)
f.mu.RLock()
defer f.mu.RUnlock()
ctor, exists := f.encoderCtors[contentType]
if !exists {
    return nil, fmt.Errorf("no encoder registered for content type: %s", contentType)
}
```

### Extraction Opportunity
```go
type TypeRegistry[T any] struct {
    mu    sync.RWMutex
    items map[string]T
}

func (r *TypeRegistry[T]) Register(key string, value T) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if key == "" {
        return errors.New("key cannot be empty")
    }
    r.items[key] = value
    return nil
}

func (r *TypeRegistry[T]) Get(key string) (T, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    value, exists := r.items[key]
    if !exists {
        var zero T
        return zero, fmt.Errorf("no item registered for key: %s", key)
    }
    return value, nil
}
```

## Summary of Extraction Opportunities

1. **Thread Safety Helpers** - Reduce boilerplate for mutex operations
2. **Nil Handling Utilities** - Consistent nil checking and defaulting
3. **Error Factories** - Standardized error creation
4. **Content Type Utilities** - Common content type operations
5. **Validation Helpers** - Reusable validation logic
6. **Stream Lifecycle Management** - Consistent stream state handling
7. **Generic Registry** - Type-safe registry pattern

These extractions would:
- Reduce code duplication by ~30%
- Improve consistency across the codebase
- Make testing easier with isolated utilities
- Reduce the chance of bugs in common operations
- Improve maintainability