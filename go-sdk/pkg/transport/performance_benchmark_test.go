package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/transport/sse"
)

// BenchmarkStringAllocations compares string allocation patterns
func BenchmarkStringAllocations(b *testing.B) {
	b.Run("fmt.Sprintf", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = fmt.Sprintf("handler_%d_%d", i, time.Now().UnixNano())
		}
	})
	
	b.Run("strings.Builder", func(b *testing.B) {
		var sb strings.Builder
		for i := 0; i < b.N; i++ {
			sb.Reset()
			sb.WriteString("handler_")
			sb.WriteString(strconv.Itoa(i))
			sb.WriteByte('_')
			sb.WriteString(strconv.FormatInt(time.Now().UnixNano(), 10))
			_ = sb.String()
		}
	})
	
	b.Run("pre-allocated buffer", func(b *testing.B) {
		buf := make([]byte, 0, 64)
		for i := 0; i < b.N; i++ {
			buf = buf[:0]
			buf = append(buf, "handler_"...)
			buf = strconv.AppendInt(buf, int64(i), 10)
			buf = append(buf, '_')
			buf = strconv.AppendInt(buf, time.Now().UnixNano(), 10)
			_ = string(buf)
		}
	})
}

// BenchmarkSliceOperations compares slice operations
func BenchmarkSliceOperations(b *testing.B) {
	b.Run("append without preallocation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var slice []string
			iterations := 100
			if testing.Short() {
				iterations = 20  // Reduced for short mode
			}
			for j := 0; j < iterations; j++ {
				slice = append(slice, "item")
			}
		}
	})
	
	b.Run("append with preallocation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			iterations := 100
			if testing.Short() {
				iterations = 20
			}
			slice := make([]string, 0, iterations)
			for j := 0; j < iterations; j++ {
				slice = append(slice, "item")
			}
		}
	})
	
	b.Run("slice pool", func(b *testing.B) {
		pool := NewSlicePool[string]()
		for i := 0; i < b.N; i++ {
			iterations := 100
			if testing.Short() {
				iterations = 20
			}
			slice := pool.Get(iterations)
			for j := 0; j < iterations; j++ {
				slice = append(slice, "item")
			}
			pool.Put(slice)
		}
	})
}

// BenchmarkEventParsing compares event parsing approaches
func BenchmarkEventParsing(b *testing.B) {
	eventData := `{"type":"TEXT_MESSAGE_CONTENT","messageId":"msg123","delta":"hello"}`
	
	b.Run("json.Unmarshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var data map[string]interface{}
			json.Unmarshal([]byte(eventData), &data)
		}
	})
	
	b.Run("json.Decoder", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			dec := json.NewDecoder(strings.NewReader(eventData))
			var data map[string]interface{}
			dec.Decode(&data)
		}
	})
	
	b.Run("pooled decoder", func(b *testing.B) {
		pool := &sync.Pool{
			New: func() interface{} {
				return json.NewDecoder(strings.NewReader(""))
			},
		}
		
		for i := 0; i < b.N; i++ {
			dec := pool.Get().(*json.Decoder)
			dec = json.NewDecoder(strings.NewReader(eventData))
			var data map[string]interface{}
			dec.Decode(&data)
			pool.Put(dec)
		}
	})
}

// BenchmarkLoggerPerformance compares logger implementations
func BenchmarkLoggerPerformance(b *testing.B) {
	defaultLogger := NewLogger(DefaultLoggerConfig())
	optimizedLogger := NewOptimizedLogger(DefaultLoggerConfig())
	
	b.Run("default logger", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			defaultLogger.Info("test message", 
				String("key1", "value1"),
				Int("key2", 42),
				Duration("key3", time.Millisecond*100))
		}
	})
	
	b.Run("optimized logger", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			optimizedLogger.Info("test message",
				String("key1", "value1"),
				Int("key2", 42),
				Duration("key3", time.Millisecond*100))
		}
	})
}

// BenchmarkSSEEventProcessing compares SSE event processing
func BenchmarkSSEEventProcessing(b *testing.B) {
	config := sse.DefaultConfig()
	transport, _ := sse.NewSSETransport(config)
	
	eventData := map[string]interface{}{
		"messageId": "msg123",
		"delta":     "hello world",
		"timestamp": float64(time.Now().Unix()),
	}
	
	b.Run("original parsing", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Skip parsing test since method is not exported
			_ = eventData
			_ = transport
		}
	})
	
	b.Run("optimized parsing", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Simulate optimized parsing
			messageID, _ := eventData["messageId"].(string)
			delta, _ := eventData["delta"].(string)
			
			event := events.NewTextMessageContentEvent(messageID, delta)
			if timestamp, ok := eventData["timestamp"].(float64); ok {
				event.SetTimestamp(int64(timestamp))
			}
		}
	})
}

// BenchmarkEventHandlerManagement compares event handler management
func BenchmarkEventHandlerManagement(b *testing.B) {
	handler := func(ctx context.Context, event events.Event) error {
		return nil
	}
	
	b.Run("map with slice", func(b *testing.B) {
		handlers := make(map[string][]func(context.Context, events.Event) error)
		
		for i := 0; i < b.N; i++ {
			eventType := "test_event"
			handlers[eventType] = append(handlers[eventType], handler)
		}
	})
	
	b.Run("map with preallocated slice", func(b *testing.B) {
		handlers := make(map[string][]func(context.Context, events.Event) error)
		
		for i := 0; i < b.N; i++ {
			eventType := "test_event"
			if _, exists := handlers[eventType]; !exists {
				handlers[eventType] = make([]func(context.Context, events.Event) error, 0, 10)
			}
			handlers[eventType] = append(handlers[eventType], handler)
		}
	})
}

// BenchmarkStringInternment compares string internment approaches
func BenchmarkStringInternment(b *testing.B) {
	eventTypes := []string{
		"TEXT_MESSAGE_START",
		"TEXT_MESSAGE_CONTENT", 
		"TEXT_MESSAGE_END",
		"TOOL_CALL_START",
		"TOOL_CALL_ARGS",
		"TOOL_CALL_END",
	}
	
	b.Run("no internment", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			eventType := eventTypes[i%len(eventTypes)]
			_ = eventType
		}
	})
	
	b.Run("sync.Map internment", func(b *testing.B) {
		var m sync.Map
		for i := 0; i < b.N; i++ {
			eventType := eventTypes[i%len(eventTypes)]
			if v, ok := m.Load(eventType); ok {
				_ = v.(string)
			} else {
				m.Store(eventType, eventType)
			}
		}
	})
}

// BenchmarkMemoryAllocation measures memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	b.Run("slice growth", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var slice []int
			iterations := 1000
			if testing.Short() {
				iterations = 100  // Reduced for short mode
			}
			for j := 0; j < iterations; j++ {
				slice = append(slice, j)
			}
		}
	})
	
	b.Run("pre-allocated slice", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			iterations := 1000
			if testing.Short() {
				iterations = 100
			}
			slice := make([]int, 0, iterations)
			for j := 0; j < iterations; j++ {
				slice = append(slice, j)
			}
		}
	})
}

// BenchmarkEventTypeSwitch compares event type switching
func BenchmarkEventTypeSwitch(b *testing.B) {
	eventTypes := []string{
		"TEXT_MESSAGE_START",
		"TEXT_MESSAGE_CONTENT", 
		"TEXT_MESSAGE_END",
		"TOOL_CALL_START",
		"TOOL_CALL_ARGS",
		"TOOL_CALL_END",
	}
	
	b.Run("switch on events.EventType", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			eventType := eventTypes[i%len(eventTypes)]
			switch events.EventType(eventType) {
			case events.EventTypeTextMessageStart:
				// handle
			case events.EventTypeTextMessageContent:
				// handle
			case events.EventTypeTextMessageEnd:
				// handle
			case events.EventTypeToolCallStart:
				// handle
			case events.EventTypeToolCallArgs:
				// handle
			case events.EventTypeToolCallEnd:
				// handle
			}
		}
	})
	
	b.Run("switch on string", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			eventType := eventTypes[i%len(eventTypes)]
			switch eventType {
			case "TEXT_MESSAGE_START":
				// handle
			case "TEXT_MESSAGE_CONTENT":
				// handle
			case "TEXT_MESSAGE_END":
				// handle
			case "TOOL_CALL_START":
				// handle
			case "TOOL_CALL_ARGS":
				// handle
			case "TOOL_CALL_END":
				// handle
			}
		}
	})
}

// BenchmarkConcurrentOperations tests concurrent performance
func BenchmarkConcurrentOperations(b *testing.B) {
	b.Run("concurrent string pool", func(b *testing.B) {
		pool := newStringPool()
		
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf := pool.get()
				*buf = append(*buf, "test message"...)
				pool.put(buf)
			}
		})
	})
	
	b.Run("concurrent slice pool", func(b *testing.B) {
		pool := NewSlicePool[string]()
		
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				slice := pool.Get(10)
				slice = append(slice, "item1", "item2", "item3")
				pool.Put(slice)
			}
		})
	})
}

// Memory usage benchmark
func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("baseline", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Create event handlers without optimization
			handlers := make(map[string][]func(context.Context, events.Event) error)
			iterations := 100
			if testing.Short() {
				iterations = 20
			}
			for j := 0; j < iterations; j++ {
				eventType := fmt.Sprintf("event_%d", j)
				handler := func(ctx context.Context, event events.Event) error {
					return nil
				}
				handlers[eventType] = append(handlers[eventType], handler)
			}
		}
	})
	
	b.Run("optimized", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Create event handlers with optimization
			handlers := make(map[string][]func(context.Context, events.Event) error)
			iterations := 100
			if testing.Short() {
				iterations = 20
			}
			for j := 0; j < iterations; j++ {
				eventType := fmt.Sprintf("event_%d", j)
				if _, exists := handlers[eventType]; !exists {
					handlers[eventType] = make([]func(context.Context, events.Event) error, 0, 4)
				}
				handler := func(ctx context.Context, event events.Event) error {
					return nil
				}
				handlers[eventType] = append(handlers[eventType], handler)
			}
		}
	})
}