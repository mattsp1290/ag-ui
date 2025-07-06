# Context Cancellation Fix Summary

## Overview
Added context cancellation checks to all long-running goroutines in the state management system to ensure proper shutdown behavior and prevent goroutine leaks.

## Changes Made

### manager.go
1. **processUpdates()** - Added context check before processing loop
2. **processEvents()** - Added context check before processing loop  
3. **autoCheckpoint()** - Added context check before processing loop
4. **collectMetrics()** - Added context check before processing loop
5. **handleErrors()** - Added context check before processing loop
6. **contextCleanup()** - Added context check before processing loop
7. **maybeCleanupContexts() goroutine** - Added context check before cleanup

All goroutines now:
- Check `ctx.Err()` before processing work
- Log debug messages when shutting down
- Exit promptly when context is cancelled

### event_handlers.go
1. **streamLoop()** - Added check for stopCh before processing to ensure immediate exit

## Pattern Applied

For each long-running goroutine:
```go
for {
    // Check context cancellation before processing
    if err := sm.ctx.Err(); err != nil {
        sm.logger.Debug("goroutine shutting down", Err(err))
        return
    }

    select {
    case <-ticker.C:
        // Process work
    case <-sm.ctx.Done():
        sm.logger.Debug("goroutine context cancelled", Err(sm.ctx.Err()))
        return
    }
}
```

## Files Not Modified

### Short-lived/Fire-and-forget goroutines (no changes needed):
- **store.go** - notification goroutines and cleanup goroutines are short-lived
- **client_ratelimit.go** - cleanup goroutine is short-lived
- **event_handlers.go** - emit goroutines are fire-and-forget
- **manager.go** - drain goroutines in Close() are specifically for shutdown

### Already had proper context handling:
- **tools/executor.go** - already checks context
- **tools/streaming.go** - already checks context  
- **core/events/validator.go** - already checks context

### Uses different shutdown mechanism:
- **messages/streaming.go** - uses closeChan instead of context (appropriate for its design)

## Benefits

1. **Graceful Shutdown**: All goroutines now exit promptly when the StateManager is closed
2. **No Goroutine Leaks**: Context cancellation ensures goroutines don't continue running after shutdown
3. **Better Debugging**: Debug logs show when and why goroutines are shutting down
4. **Consistent Pattern**: All long-running goroutines follow the same shutdown pattern

## Testing

The changes ensure:
- Goroutines check context before processing each iteration
- Goroutines log their shutdown for debugging
- No work is lost during shutdown (existing behavior preserved)
- Fast shutdown response to context cancellation