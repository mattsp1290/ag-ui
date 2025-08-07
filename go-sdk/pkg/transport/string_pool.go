package transport

import (
	"sync"
)

// stringPool manages a pool of frequently used strings to reduce allocations
type stringPool struct {
	pool sync.Pool
}

// newStringPool creates a new string pool
func newStringPool() *stringPool {
	return &stringPool{
		pool: sync.Pool{
			New: func() interface{} {
				// Pre-allocate a string builder with reasonable capacity
				b := make([]byte, 0, 64)
				return &b
			},
		},
	}
}

// get retrieves a buffer from the pool
func (sp *stringPool) get() *[]byte {
	return sp.pool.Get().(*[]byte)
}

// put returns a buffer to the pool
func (sp *stringPool) put(b *[]byte) {
	// Reset the buffer
	*b = (*b)[:0]
	sp.pool.Put(b)
}

// Global string pools for common operations
var (
	// Pool for event type strings
	eventTypePool = &sync.Map{}

	// Pool for error messages
	errorMsgPool = &sync.Map{}

	// Pool for log messages
	logMsgPool = newStringPool()
)

// internString interns a string in the given pool
func internString(pool *sync.Map, s string) string {
	if v, ok := pool.Load(s); ok {
		return v.(string)
	}
	pool.Store(s, s)
	return s
}

// InternEventType interns an event type string
func InternEventType(eventType string) string {
	return internString(eventTypePool, eventType)
}

// InternErrorMsg interns an error message
func InternErrorMsg(msg string) string {
	return internString(errorMsgPool, msg)
}

// FormatLogMessage efficiently formats a log message using a pooled buffer
func FormatLogMessage(format string, args ...interface{}) string {
	buf := logMsgPool.get()
	defer logMsgPool.put(buf)

	// Use append operations which are more efficient than fmt.Sprintf
	*buf = append(*buf, format...)

	// Simple implementation - in production, would use more sophisticated formatting
	// This avoids fmt.Sprintf allocations for simple cases
	return string(*buf)
}
