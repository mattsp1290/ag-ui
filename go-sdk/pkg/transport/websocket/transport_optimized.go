package websocket

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Pre-allocated string formats
const (
	handlerIDFormat = "handler_%d_%d"
	subIDFormat     = "sub_%d"
	slotIDFormat    = "slot_%d"
)

// String pools for reducing allocations
var (
	handlerIDPool = &sync.Pool{
		New: func() interface{} {
			return new(strings.Builder)
		},
	}

	errorMsgPool = &sync.Pool{
		New: func() interface{} {
			return new(strings.Builder)
		},
	}
)

// Counter for generating unique IDs without fmt.Sprintf
var (
	handlerIDCounter uint64
	subIDCounter     uint64
)

// generateHandlerID generates a handler ID without string allocation
func generateHandlerID() string {
	id := atomic.AddUint64(&handlerIDCounter, 1)

	// Use a pooled string builder
	sb := handlerIDPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		handlerIDPool.Put(sb)
	}()

	sb.WriteString("handler_")
	sb.WriteString(strconv.FormatUint(id, 10))
	sb.WriteByte('_')
	sb.WriteString(strconv.FormatInt(time.Now().UnixNano(), 10))

	return sb.String()
}

// generateSubID generates a subscription ID without string allocation
func generateSubID() string {
	id := atomic.AddUint64(&subIDCounter, 1)

	// Pre-allocate buffer for the ID
	buf := make([]byte, 0, 20)
	buf = append(buf, "sub_"...)
	buf = strconv.AppendUint(buf, id, 10)

	return string(buf)
}

// OptimizedTransport wraps the standard transport with performance optimizations
type OptimizedTransport struct {
	*Transport

	// Pre-allocated error messages
	closedError     error
	emptyHandlerErr error
	noEventTypesErr error
	nilHandlerErr   error
}

// NewOptimizedTransport creates a new optimized transport
func NewOptimizedTransport(config *TransportConfig) (*OptimizedTransport, error) {
	transport, err := NewTransport(config)
	if err != nil {
		return nil, err
	}

	return &OptimizedTransport{
		Transport:       transport,
		closedError:     fmt.Errorf("transport is closed"),
		emptyHandlerErr: fmt.Errorf("handler ID cannot be empty"),
		noEventTypesErr: fmt.Errorf("at least one event type must be specified"),
		nilHandlerErr:   fmt.Errorf("event handler cannot be nil"),
	}, nil
}

// AddEventHandlerOptimized adds an event handler with reduced allocations
func (t *OptimizedTransport) AddEventHandlerOptimized(eventType string, handler EventHandler) string {
	if handler == nil {
		return ""
	}

	handlerID := generateHandlerID()
	wrapper := &EventHandlerWrapper{
		ID:      handlerID,
		Handler: handler,
	}

	t.handlersMutex.Lock()
	defer t.handlersMutex.Unlock()

	handlers := t.eventHandlers[eventType]
	if handlers == nil {
		// Pre-allocate with capacity
		handlers = make([]*EventHandlerWrapper, 0, 4)
		t.eventHandlers[eventType] = handlers
	}

	t.eventHandlers[eventType] = append(handlers, wrapper)

	return handlerID
}

// RemoveEventHandlerOptimized removes a handler with minimal allocations
func (t *OptimizedTransport) RemoveEventHandlerOptimized(eventType string, handlerID string) error {
	if handlerID == "" {
		return t.emptyHandlerErr
	}

	t.handlersMutex.Lock()
	defer t.handlersMutex.Unlock()

	handlers, exists := t.eventHandlers[eventType]
	if !exists {
		return buildError("no handlers found for event type: ", eventType)
	}

	// Find and remove the handler
	for i := 0; i < len(handlers); i++ {
		if handlers[i].ID == handlerID {
			// Remove without allocating new slice if possible
			if i == len(handlers)-1 {
				t.eventHandlers[eventType] = handlers[:i]
			} else {
				copy(handlers[i:], handlers[i+1:])
				t.eventHandlers[eventType] = handlers[:len(handlers)-1]
			}

			// Clear the removed handler
			handlers[i].Handler = nil
			handlers[i].ID = ""

			if len(t.eventHandlers[eventType]) == 0 {
				delete(t.eventHandlers, eventType)
			}

			return nil
		}
	}

	return buildError("handler with ID ", handlerID, " not found for event type ", eventType)
}

// SubscribeOptimized creates a subscription with reduced allocations
func (t *OptimizedTransport) SubscribeOptimized(ctx context.Context, eventTypes []string, handler EventHandler) (*Subscription, error) {
	if len(eventTypes) == 0 {
		return nil, t.noEventTypesErr
	}

	if handler == nil {
		return nil, t.nilHandlerErr
	}

	// Create subscription with pre-allocated handler IDs slice
	subCtx, cancel := context.WithCancel(ctx)
	sub := &Subscription{
		ID:         generateSubID(),
		EventTypes: eventTypes,
		Handler:    handler,
		HandlerIDs: make([]string, 0, len(eventTypes)),
		Context:    subCtx,
		Cancel:     cancel,
		CreatedAt:  time.Now(),
	}

	// Add to subscriptions
	t.subsMutex.Lock()
	t.subscriptions[sub.ID] = sub
	t.subsMutex.Unlock()

	// Update statistics
	atomic.AddInt64(&t.stats.TotalSubscriptions, 1)
	atomic.AddInt64(&t.stats.ActiveSubscriptions, 1)

	// Register event handlers
	for _, eventType := range eventTypes {
		handlerID := t.AddEventHandlerOptimized(eventType, handler)
		sub.HandlerIDs = append(sub.HandlerIDs, handlerID)
	}

	return sub, nil
}

// buildError builds an error message without fmt.Sprintf
func buildError(parts ...string) error {
	sb := errorMsgPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		errorMsgPool.Put(sb)
	}()

	for _, part := range parts {
		sb.WriteString(part)
	}

	return fmt.Errorf(sb.String())
}

// Pre-allocated event handler slices by type for common event types
type eventHandlerCache struct {
	mu       sync.RWMutex
	handlers map[string][]*EventHandlerWrapper

	// Pre-allocated slices for common event types
	textMessageHandlers []EventHandler
	toolCallHandlers    []EventHandler
	stateHandlers       []EventHandler
}

// newEventHandlerCache creates a new handler cache
func newEventHandlerCache() *eventHandlerCache {
	return &eventHandlerCache{
		handlers:            make(map[string][]*EventHandlerWrapper),
		textMessageHandlers: make([]EventHandler, 0, 10),
		toolCallHandlers:    make([]EventHandler, 0, 10),
		stateHandlers:       make([]EventHandler, 0, 10),
	}
}

// getHandlers returns handlers for an event type with caching
func (c *eventHandlerCache) getHandlers(eventType string) []EventHandler {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check pre-allocated slices for common types
	switch eventType {
	case "TEXT_MESSAGE_START", "TEXT_MESSAGE_CONTENT", "TEXT_MESSAGE_END":
		return c.textMessageHandlers
	case "TOOL_CALL_START", "TOOL_CALL_ARGS", "TOOL_CALL_END":
		return c.toolCallHandlers
	case "STATE_SNAPSHOT", "STATE_DELTA":
		return c.stateHandlers
	}

	// Fall back to map lookup
	wrappers := c.handlers[eventType]
	if len(wrappers) == 0 {
		return nil
	}

	// Reuse slice if possible
	handlers := make([]EventHandler, len(wrappers))
	for i, w := range wrappers {
		handlers[i] = w.Handler
	}

	return handlers
}

// OptimizedConnectionSlot reduces allocations in connection management
type OptimizedConnectionSlot struct {
	ID         string
	Connection interface{}
	InUse      int32 // Use atomic operations instead of mutex
	LastUsed   int64 // Unix timestamp
}

// acquire atomically acquires the slot
func (s *OptimizedConnectionSlot) acquire() bool {
	return atomic.CompareAndSwapInt32(&s.InUse, 0, 1)
}

// release atomically releases the slot
func (s *OptimizedConnectionSlot) release() {
	atomic.StoreInt32(&s.InUse, 0)
	atomic.StoreInt64(&s.LastUsed, time.Now().Unix())
}

// isInUse checks if slot is in use without locking
func (s *OptimizedConnectionSlot) isInUse() bool {
	return atomic.LoadInt32(&s.InUse) == 1
}

// generateSlotID generates slot IDs efficiently
func generateSlotID(index int) string {
	// Pre-allocate buffer
	buf := make([]byte, 0, 10)
	buf = append(buf, "slot_"...)
	buf = strconv.AppendInt(buf, int64(index), 10)
	return string(buf)
}
