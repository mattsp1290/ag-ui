# UI Renderer Package

The `ui` package provides real-time rendering capabilities for AG-UI SSE events in the Go CLI client.

## Features

### Message Streaming
- Tracks assistant messages by `messageId`
- Accumulates content deltas incrementally
- Bounded message buffers to prevent memory issues
- Support for TEXT_MESSAGE_START, TEXT_MESSAGE_CONTENT, TEXT_MESSAGE_END events
- Optional TEXT_MESSAGE_CHUNK support

### State Management
- STATE_SNAPSHOT: Replaces entire state with new snapshot
- STATE_DELTA: Applies JSON Patch operations to existing state
- Thread-safe concurrent access to state
- Pretty-printed state summaries in human-readable mode

### Tool Call Integration
- Visual cards for tool calls in pretty mode
- Shows tool name, arguments, and results
- Error handling for failed tool calls
- Compact, readable output format

### Output Modes

#### Pretty Mode (Default)
- Colorized output when terminal supports it
- Incremental streaming without flicker
- Compact tool call cards
- State change summaries
- Thinking phase visualization

#### JSON Mode
- Line-delimited JSON output (one object per line)
- Deterministic formatting for testing
- Machine-parseable format
- Preserves all event data

### Configuration Options
- `--output`: Choose between pretty and json modes
- `--no-color`: Disable colored output
- `--quiet`: Suppress all output except errors

## Usage

```go
// Create a renderer
renderer := ui.NewRenderer(ui.RendererConfig{
    OutputMode:    ui.OutputModePretty,
    NoColor:       false,
    Quiet:         false,
    Writer:        os.Stdout,
    MaxBufferSize: 1024 * 1024, // 1MB
})

// Handle SSE events
err := renderer.HandleEvent(eventType, eventData)

// Access accumulated state
msg, exists := renderer.GetMessage("msg-1")
state := renderer.GetState()

// Clear for new session
renderer.Clear()
```

## Performance

Benchmarks on Apple M4 Max:
- Pretty Mode: ~550,000 events/second
- JSON Mode: ~280,000 events/second
- Memory efficient with bounded buffers
- Thread-safe for concurrent access

## Testing

The package includes:
- Comprehensive unit tests for all event types
- Integration tests with realistic SSE transcripts
- Concurrent access tests
- Performance benchmarks
- Error handling tests

Run tests:
```bash
go test ./internal/ui/...
go test ./internal/ui/... -bench=.
```

## Event Support

### Core Events
- TEXT_MESSAGE_START/CONTENT/END
- STATE_SNAPSHOT/DELTA
- TOOL_CALL_START/ARGS/END/RESULT
- THINKING_START/TEXT_MESSAGE_CONTENT/END
- MESSAGES_SNAPSHOT

### Graceful Handling
- Unknown events are logged but don't cause errors
- Optional events (TEXT_MESSAGE_CHUNK) are supported
- Resilient to missing or malformed data

## Implementation Details

### Thread Safety
- Uses sync.RWMutex for concurrent access
- Safe for multiple readers, single writer
- No deadlocks or race conditions

### Memory Management
- Bounded message buffers (configurable MaxBufferSize)
- Efficient string building with strings.Builder
- Clears old data to prevent leaks

### Color Support
- Uses fatih/color for terminal colors
- Automatic detection of color support
- Respects --no-color flag
- Graceful fallback for non-terminal output