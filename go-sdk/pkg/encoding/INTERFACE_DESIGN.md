# Interface Hierarchy Simplification - Design Documentation

## Overview

This document explains the interface hierarchy simplification that was implemented to address violations of the Interface Segregation Principle (ISP) and reduce architectural complexity in the encoding package.

## Problem Analysis

### Original Interface Violations

The original interface design suffered from several architectural problems:

1. **Interface Segregation Principle Violations**:
   - The `Codec` interface combined encoding AND decoding operations, forcing implementations to support both directions even when only one was needed
   - The `StreamCodec` interface was massive (12+ methods) covering multiple concerns: streaming, session management, and data processing
   - Redundant methods: `SupportsStreaming()` and `CanStream()` performed the same function

2. **"God Object" Anti-patterns**:
   - Registry had 8+ different entry types with excessive complexity
   - Factory patterns were overly complex with multiple inheritance layers
   - Too many responsibilities concentrated in single components

3. **Backward Compatibility Burden**:
   - Deprecated interfaces created unnecessary abstraction layers
   - Multiple interfaces performed identical functions
   - Excessive adapter code required to maintain compatibility

## New Interface Design

### Core Principles

1. **Single Responsibility**: Each interface has one clear, focused purpose
2. **Composition over Inheritance**: Build complex functionality by composing simple interfaces
3. **Optional Capabilities**: Use separate interfaces for advanced features
4. **Backward Compatibility**: Maintain compatibility through adapter patterns

### New Interface Hierarchy

#### Core Single-Purpose Interfaces

```go
// Basic data processing
type Encoder interface {
    Encode(ctx context.Context, event events.Event) ([]byte, error)
    EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error)
}

type Decoder interface {
    Decode(ctx context.Context, data []byte) (events.Event, error)
    DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error)
}

// Metadata providers
type ContentTypeProvider interface {
    ContentType() string
}

type StreamingCapabilityProvider interface {
    SupportsStreaming() bool
}
```

#### Streaming Interfaces

```go
// Focused streaming operations
type StreamEncoder interface {
    EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error
}

type StreamDecoder interface {
    DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error
}

// Session management (separated concern)
type StreamSessionManager interface {
    StartEncodingSession(ctx context.Context, w io.Writer) error
    StartDecodingSession(ctx context.Context, r io.Reader) error
    EndSession(ctx context.Context) error
}

// Event-level processing (separated concern)
type StreamEventProcessor interface {
    WriteEvent(ctx context.Context, event events.Event) error
    ReadEvent(ctx context.Context) (events.Event, error)
}
```

#### Validation Interfaces

```go
type Validator interface {
    Validate(ctx context.Context, data interface{}) error
}

type OutputValidator interface {
    ValidateOutput(ctx context.Context, data []byte) error
}

type InputValidator interface {
    ValidateInput(ctx context.Context, data []byte) error
}
```

#### Composite Interfaces (Built through Composition)

```go
// Convenience interfaces that combine focused interfaces
type Codec interface {
    Encoder
    Decoder
    ContentTypeProvider
    StreamingCapabilityProvider
}

type StreamCodec interface {
    StreamEncoder
    StreamDecoder
    ContentTypeProvider
    StreamingCapabilityProvider
}

type FullStreamCodec interface {
    Codec                    // Basic operations
    StreamCodec             // Stream operations
    StreamSessionManager    // Session management
    StreamEventProcessor    // Event-level streaming
}
```

### Factory Interface Simplification

#### Separated Factory Concerns

```go
// Basic codec creation
type CodecFactory interface {
    CreateCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error)
    SupportedTypes() []string
}

// Streaming codec creation (separate interface)
type StreamCodecFactory interface {
    CreateStreamCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (StreamCodec, error)
    SupportsStreaming(contentType string) bool
}

// Combined convenience interface
type FullCodecFactory interface {
    CodecFactory
    StreamCodecFactory
}
```

## Implementation Strategy

### Backward Compatibility

To maintain backward compatibility, the implementation provides:

1. **Legacy Interface Support**: All old interfaces are preserved and marked as deprecated
2. **Adapter Patterns**: Universal adapters bridge between old and new interfaces
3. **Composite Interface Mapping**: Old monolithic interfaces map to new composite interfaces

#### Universal Adapters

```go
// Adapter functions ensure any basic interface can be used where composite interfaces are required
func EnsureFullEncoder(encoder Encoder) interface {
    Encoder
    ContentTypeProvider  
    StreamingCapabilityProvider
}

func EnsureFullDecoder(decoder Decoder) interface {
    Decoder
    StreamingCapabilityProvider
}
```

### Migration Path

1. **Immediate**: New code should use focused interfaces
2. **Gradual**: Existing code continues working through adapters
3. **Future**: Legacy interfaces will be removed in a future major version

## Benefits Achieved

### Interface Segregation Compliance

- **Before**: Single interface with 10+ methods covering multiple concerns
- **After**: Multiple focused interfaces with 1-3 methods each

### Reduced Complexity

- **Before**: Massive interfaces forced implementation of unused functionality
- **After**: Components implement only the interfaces they need

### Improved Testability

- **Before**: Mocking required implementing entire large interfaces
- **After**: Test mocks need only implement specific focused interfaces

### Better Composability

- **Before**: Inheritance-heavy design with rigid hierarchies  
- **After**: Composition-based design with flexible combinations

## Example Usage

### Old Approach (Still Supported)

```go
// Old way - forces implementation of everything
type MyCodec struct{}
func (c *MyCodec) Encode(...) ([]byte, error) { /* implementation */ }
func (c *MyCodec) Decode(...) (events.Event, error) { /* implementation */ }
func (c *MyCodec) ContentType() string { return "application/my-format" }
func (c *MyCodec) SupportsStreaming() bool { return false }
// ...10+ more methods required
```

### New Focused Approach

```go
// New way - implement only what you need
type MyEncoder struct{}
func (e *MyEncoder) Encode(...) ([]byte, error) { /* implementation */ }
func (e *MyEncoder) EncodeMultiple(...) ([]byte, error) { /* implementation */ }

type MyContentTypeProvider struct{}
func (p *MyContentTypeProvider) ContentType() string { return "application/my-format" }

// Combine via composition when needed
type MyCodec struct {
    MyEncoder
    MyDecoder  // Similar focused decoder
    MyContentTypeProvider
    MyStreamingCapabilityProvider
}
```

## Architecture Impact

### Registry Simplification

The registry can now work with focused factory interfaces, reducing complexity:

```go
// Register focused factories
registry.RegisterFullCodecFactory("application/json", myFactory)

// Get focused codecs
encoder := registry.GetFocusedEncoder(ctx, "application/json", options)
decoder := registry.GetFocusedDecoder(ctx, "application/json", options)
```

### Reduced God Object Anti-patterns

- **Registry**: Now works with specific factory types instead of managing everything
- **Factories**: Separated into focused factory interfaces 
- **Codecs**: Split into focused operational interfaces

## Future Improvements

1. **Validation Integration**: Add validation interfaces to provide structured error handling
2. **Plugin Architecture**: Use focused interfaces for better plugin system design
3. **Performance Optimization**: Focused interfaces enable better performance optimizations
4. **Testing Framework**: Create interface-specific testing utilities

## Migration Recommendations

For new development:

1. **Use Focused Interfaces**: Implement only the interfaces you need
2. **Compose When Necessary**: Use composition to build complex functionality
3. **Leverage Adapters**: Use provided adapters when interfacing with legacy code
4. **Validate Early**: Use validation interfaces to catch errors early

For existing code:

1. **No Immediate Changes Required**: Legacy interfaces continue working
2. **Gradual Migration**: Migrate to focused interfaces over time
3. **Test Thoroughly**: Use adapters to ensure compatibility during migration

## Conclusion

This interface hierarchy simplification addresses fundamental architectural issues while maintaining full backward compatibility. The new design follows SOLID principles, reduces complexity, and provides a clear path for future evolution of the encoding system.

The implementation demonstrates how to refactor complex interface hierarchies without breaking existing code, using adapter patterns and interface composition to achieve both architectural cleanliness and practical compatibility.