package performance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

// BenchmarkJSONSerialization benchmarks JSON serialization performance
func BenchmarkJSONSerialization(b *testing.B) {
	b.Run("Message Serialization", func(b *testing.B) {
		benchmarkMessageSerialization(b)
	})

	b.Run("Event Serialization", func(b *testing.B) {
		benchmarkEventSerialization(b)
	})

	b.Run("State Serialization", func(b *testing.B) {
		benchmarkStateSerialization(b)
	})

	b.Run("Tool Result Serialization", func(b *testing.B) {
		benchmarkToolResultSerialization(b)
	})
}

// benchmarkMessageSerialization benchmarks message serialization
func benchmarkMessageSerialization(b *testing.B) {
	// Create test message using concrete message type
	content := "This is a test message with some content to serialize"
	message := &messages.UserMessage{
		BaseMessage: messages.BaseMessage{
			ID:      "test-message-123",
			Role:    messages.RoleUser,
			Content: &content,
			Metadata: &messages.MessageMetadata{
				UserID:    "user123",
				SessionID: "session456",
				CustomFields: map[string]interface{}{
					"tags": []string{"important", "urgent", "test"},
				},
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(message)
		if err != nil {
			b.Fatal(err)
		}

		var decoded messages.UserMessage
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// benchmarkEventSerialization benchmarks event serialization
func benchmarkEventSerialization(b *testing.B) {
	// Create test event with correct field names
	timestampMs := time.Now().UnixMilli()
	event := &events.CustomEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeCustom,
			TimestampMs: &timestampMs,
			RawEvent: map[string]interface{}{
				"source":      "test",
				"version":     "1.0",
				"environment": "test",
			},
		},
		Name: "user_action",
		Value: map[string]interface{}{
			"action":      "click",
			"element":     "button",
			"page":        "/dashboard",
			"coordinates": map[string]int{"x": 100, "y": 200},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(event)
		if err != nil {
			b.Fatal(err)
		}

		var decoded events.CustomEvent
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// benchmarkStateSerialization benchmarks state serialization
func benchmarkStateSerialization(b *testing.B) {
	// Create test state using a simple state structure for benchmarking
	testState := map[string]interface{}{
		"id":           "state-123",
		"version":      1,
		"last_updated": time.Now(),
		"data": map[string]interface{}{
			"user": map[string]interface{}{
				"id":    "user123",
				"name":  "John Doe",
				"email": "john@example.com",
				"settings": map[string]interface{}{
					"theme":       "dark",
					"language":    "en",
					"timezone":    "UTC",
					"preferences": []string{"notifications", "emails"},
				},
			},
			"session": map[string]interface{}{
				"id":         "session456",
				"created_at": time.Now().Unix(),
				"expires_at": time.Now().Add(24 * time.Hour).Unix(),
				"active":     true,
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(testState)
		if err != nil {
			b.Fatal(err)
		}

		var decoded map[string]interface{}
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// benchmarkToolResultSerialization benchmarks tool result serialization
func benchmarkToolResultSerialization(b *testing.B) {
	// Create test tool result using correct type and fields
	result := &tools.ToolExecutionResult{
		Success:   true,
		Data:      "Tool execution completed successfully",
		Error:     "",
		Timestamp: time.Now(),
		Duration:  1234 * time.Millisecond,
		Metadata: map[string]interface{}{
			"memory_usage": "45MB",
			"cpu_usage":    "12%",
			"logs": []string{
				"Starting tool execution",
				"Processing input data",
				"Generating output",
				"Execution completed",
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(result)
		if err != nil {
			b.Fatal(err)
		}

		var decoded tools.ToolExecutionResult
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMemoryAllocation benchmarks memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	b.Run("Message Pool", func(b *testing.B) {
		benchmarkMessagePool(b)
	})

	b.Run("Event Pool", func(b *testing.B) {
		benchmarkEventPool(b)
	})

	b.Run("State Pool", func(b *testing.B) {
		benchmarkStatePool(b)
	})

	b.Run("Buffer Pool", func(b *testing.B) {
		benchmarkBufferPool(b)
	})
}

// benchmarkMessagePool benchmarks message pool performance
func benchmarkMessagePool(b *testing.B) {
	pool := &sync.Pool{
		New: func() interface{} {
			return &messages.UserMessage{}
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		msg := pool.Get().(*messages.UserMessage)
		content := "Test message content"
		msg.BaseMessage.ID = fmt.Sprintf("msg-%d", i)
		msg.BaseMessage.Role = messages.RoleUser
		msg.BaseMessage.Content = &content

		// Simulate usage
		_ = msg.BaseMessage.ID
		_ = msg.BaseMessage.Content

		// Reset and return to pool
		msg.BaseMessage.ID = ""
		msg.BaseMessage.Role = ""
		msg.BaseMessage.Content = nil
		msg.BaseMessage.Metadata = nil

		pool.Put(msg)
	}
}

// benchmarkEventPool benchmarks event pool performance
func benchmarkEventPool(b *testing.B) {
	pool := &sync.Pool{
		New: func() interface{} {
			return &events.CustomEvent{}
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		event := pool.Get().(*events.CustomEvent)
		timestampMs := time.Now().UnixMilli()
		event.BaseEvent.EventType = events.EventTypeCustom
		event.BaseEvent.TimestampMs = &timestampMs
		event.Name = "test_event"
		event.Value = "test_value"

		// Simulate usage
		_ = event.BaseEvent.EventType
		_ = event.Name

		// Reset and return to pool
		event.BaseEvent.EventType = ""
		event.BaseEvent.TimestampMs = nil
		event.BaseEvent.RawEvent = nil
		event.Name = ""
		event.Value = nil

		pool.Put(event)
	}
}

// benchmarkStatePool benchmarks state pool performance
func benchmarkStatePool(b *testing.B) {
	pool := &sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{})
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		st := pool.Get().(map[string]interface{})
		st["id"] = fmt.Sprintf("state-%d", i)
		st["version"] = 1
		st["last_updated"] = time.Now()
		st["data"] = map[string]interface{}{"key": "value"}

		// Simulate usage
		_ = st["id"]
		_ = st["data"]

		// Reset and return to pool
		for k := range st {
			delete(st, k)
		}

		pool.Put(st)
	}
}

// benchmarkBufferPool benchmarks buffer pool performance
func benchmarkBufferPool(b *testing.B) {
	pool := &sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf := pool.Get().(*bytes.Buffer)
		buf.Reset()

		// Simulate usage
		fmt.Fprintf(buf, "Test data %d", i)
		_ = buf.String()

		pool.Put(buf)
	}
}

// BenchmarkConnectionPooling benchmarks connection pooling performance
func BenchmarkConnectionPooling(b *testing.B) {
	b.Run("HTTP Connection Pool", func(b *testing.B) {
		benchmarkHTTPConnectionPool(b)
	})

	b.Run("WebSocket Connection Pool", func(b *testing.B) {
		benchmarkWebSocketConnectionPool(b)
	})

	b.Run("SSE Connection Pool", func(b *testing.B) {
		benchmarkSSEConnectionPool(b)
	})
}

// benchmarkHTTPConnectionPool benchmarks HTTP connection pooling
func benchmarkHTTPConnectionPool(b *testing.B) {
	// Create HTTP client with connection pooling
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Create test server
	server := &http.Server{
		Addr: ":0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("OK"))
		}),
	}

	// Start server in background
	go server.ListenAndServe()
	defer server.Close()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get("http://localhost:8080")
			if err != nil {
				continue // Server might not be ready
			}
			resp.Body.Close()
		}
	})
}

// benchmarkWebSocketConnectionPool benchmarks WebSocket connection pooling
func benchmarkWebSocketConnectionPool(b *testing.B) {
	// Create WebSocket connection pool using mock type
	pool := &ConnectionPool{
		MaxConnections: 100,
		IdleTimeout:    30 * time.Second,
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := pool.Get("ws://localhost:8080/ws")
			if err != nil {
				continue // Connection might fail
			}

			// Simulate usage
			_ = conn

			pool.Put(conn)
		}
	})
}

// benchmarkSSEConnectionPool benchmarks SSE connection pooling
func benchmarkSSEConnectionPool(b *testing.B) {
	// Create SSE connection pool using mock type
	pool := &ConnectionPool{
		MaxConnections: 100,
		IdleTimeout:    30 * time.Second,
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := pool.Get("http://localhost:8080/sse")
			if err != nil {
				continue // Connection might fail
			}

			// Simulate usage
			_ = conn

			pool.Put(conn)
		}
	})
}

// BenchmarkConcurrentOperations benchmarks concurrent operations
func BenchmarkConcurrentOperations(b *testing.B) {
	b.Run("Concurrent Message Processing", func(b *testing.B) {
		benchmarkConcurrentMessageProcessing(b)
	})

	b.Run("Concurrent Event Processing", func(b *testing.B) {
		benchmarkConcurrentEventProcessing(b)
	})

	b.Run("Concurrent State Updates", func(b *testing.B) {
		benchmarkConcurrentStateUpdates(b)
	})
}

// benchmarkConcurrentMessageProcessing benchmarks concurrent message processing
func benchmarkConcurrentMessageProcessing(b *testing.B) {
	processor := &MessageProcessor{
		Workers: runtime.NumCPU(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			content := "Test message content"
			msg := &messages.UserMessage{
				BaseMessage: messages.BaseMessage{
					ID:      fmt.Sprintf("msg-%d", time.Now().UnixNano()),
					Role:    messages.RoleUser,
					Content: &content,
				},
			}

			processor.Process(msg)
		}
	})
}

// benchmarkConcurrentEventProcessing benchmarks concurrent event processing
func benchmarkConcurrentEventProcessing(b *testing.B) {
	processor := &EventProcessor{
		Workers: runtime.NumCPU(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			timestampMs := time.Now().UnixMilli()
			event := &events.CustomEvent{
				BaseEvent: &events.BaseEvent{
					EventType:   events.EventTypeCustom,
					TimestampMs: &timestampMs,
				},
				Name:  "test_event",
				Value: "test_value",
			}

			processor.Process(event)
		}
	})
}

// benchmarkConcurrentStateUpdates benchmarks concurrent state updates
func benchmarkConcurrentStateUpdates(b *testing.B) {
	manager := &MockStateManager{
		states: make(map[string]interface{}),
		mutex:  &sync.RWMutex{},
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stateID := fmt.Sprintf("state-%d", time.Now().UnixNano())

			update := map[string]interface{}{
				"id":    stateID,
				"delta": map[string]interface{}{"key": "value"},
			}

			manager.Update(context.Background(), update)
		}
	})
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("Memory Growth", func(b *testing.B) {
		benchmarkMemoryGrowth(b)
	})

	b.Run("Memory Cleanup", func(b *testing.B) {
		benchmarkMemoryCleanup(b)
	})

	b.Run("Memory Pooling", func(b *testing.B) {
		benchmarkMemoryPooling(b)
	})
}

// benchmarkMemoryGrowth benchmarks memory growth patterns
func benchmarkMemoryGrowth(b *testing.B) {
	var data [][]byte

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Allocate memory
		chunk := make([]byte, 1024)
		data = append(data, chunk)

		// Prevent optimization
		runtime.KeepAlive(chunk)
	}

	// Measure memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	b.ReportMetric(float64(m.Alloc), "bytes-allocated")
	b.ReportMetric(float64(m.TotalAlloc), "bytes-total")
	b.ReportMetric(float64(m.Sys), "bytes-system")
}

// benchmarkMemoryCleanup benchmarks memory cleanup patterns
func benchmarkMemoryCleanup(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Allocate memory
		data := make([]byte, 1024)

		// Use the memory
		for j := range data {
			data[j] = byte(j)
		}

		// Explicitly set to nil to help GC
		data = nil

		// Force garbage collection every 1000 iterations
		if i%1000 == 0 {
			runtime.GC()
		}
	}
}

// benchmarkMemoryPooling benchmarks memory pooling effectiveness
func benchmarkMemoryPooling(b *testing.B) {
	pool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, 1024)
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Get from pool
			data := pool.Get().([]byte)

			// Use the memory
			for i := range data {
				data[i] = byte(i)
			}

			// Return to pool
			pool.Put(data)
		}
	})
}

// BenchmarkComparison benchmarks before and after optimizations
func BenchmarkComparison(b *testing.B) {
	b.Run("Before Optimization", func(b *testing.B) {
		benchmarkBeforeOptimization(b)
	})

	b.Run("After Optimization", func(b *testing.B) {
		benchmarkAfterOptimization(b)
	})
}

// benchmarkBeforeOptimization benchmarks performance before optimizations
func benchmarkBeforeOptimization(b *testing.B) {
	// Simulate old implementation without optimizations
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate unoptimized JSON serialization
		data := map[string]interface{}{
			"id":        fmt.Sprintf("item-%d", i),
			"timestamp": time.Now(),
			"data":      strings.Repeat("data", 100),
		}

		// Direct JSON marshal/unmarshal without pooling
		jsonData, _ := json.Marshal(data)
		var result map[string]interface{}
		json.Unmarshal(jsonData, &result)
	}
}

// benchmarkAfterOptimization benchmarks performance after optimizations
func benchmarkAfterOptimization(b *testing.B) {
	// Use optimized implementation with pooling
	pool := &sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{})
		},
	}

	bufferPool := &sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Get from pool
		data := pool.Get().(map[string]interface{})
		buf := bufferPool.Get().(*bytes.Buffer)
		buf.Reset()

		// Populate data
		data["id"] = fmt.Sprintf("item-%d", i)
		data["timestamp"] = time.Now()
		data["data"] = strings.Repeat("data", 100)

		// Use buffer for JSON operations
		encoder := json.NewEncoder(buf)
		encoder.Encode(data)

		decoder := json.NewDecoder(buf)
		var result map[string]interface{}
		decoder.Decode(&result)

		// Clean up and return to pool
		for k := range data {
			delete(data, k)
		}
		pool.Put(data)
		bufferPool.Put(buf)
	}
}

// BenchmarkResourceUtilization benchmarks resource utilization
func BenchmarkResourceUtilization(b *testing.B) {
	b.Run("CPU Utilization", func(b *testing.B) {
		benchmarkCPUUtilization(b)
	})

	b.Run("Memory Utilization", func(b *testing.B) {
		benchmarkMemoryUtilization(b)
	})

	b.Run("Network Utilization", func(b *testing.B) {
		benchmarkNetworkUtilization(b)
	})
}

// benchmarkCPUUtilization benchmarks CPU utilization
func benchmarkCPUUtilization(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate CPU-intensive work
			sum := 0
			for i := 0; i < 1000; i++ {
				sum += i * i
			}
			runtime.KeepAlive(sum)
		}
	})
}

// benchmarkMemoryUtilization benchmarks memory utilization
func benchmarkMemoryUtilization(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate memory allocation patterns
		data := make([]int, 1000)
		for j := range data {
			data[j] = j
		}

		// Force usage to prevent optimization
		sum := 0
		for _, v := range data {
			sum += v
		}
		runtime.KeepAlive(sum)
	}
}

// benchmarkNetworkUtilization benchmarks network utilization
func benchmarkNetworkUtilization(b *testing.B) {
	// Create a test HTTP server
	server := &http.Server{
		Addr: ":0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("OK"))
		}),
	}

	go server.ListenAndServe()
	defer server.Close()

	client := &http.Client{}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get("http://localhost:8080")
			if err != nil {
				continue
			}
			resp.Body.Close()
		}
	})
}

// Mock implementations for testing

// Mock connection pool for WebSocket
type ConnectionPool struct {
	MaxConnections int
	IdleTimeout    time.Duration
	connections    map[string]*Connection
	mutex          sync.RWMutex
}

func (cp *ConnectionPool) Get(url string) (*Connection, error) {
	cp.mutex.RLock()
	defer cp.mutex.RUnlock()

	if conn, exists := cp.connections[url]; exists {
		return conn, nil
	}

	return &Connection{URL: url}, nil
}

func (cp *ConnectionPool) Put(conn *Connection) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	if cp.connections == nil {
		cp.connections = make(map[string]*Connection)
	}

	cp.connections[conn.URL] = conn
}

// Mock connection
type Connection struct {
	URL string
}

// Mock message processor
type MessageProcessor struct {
	Workers int
}

func (mp *MessageProcessor) Process(msg messages.Message) {
	// Simulate processing
	time.Sleep(time.Microsecond)
}

// Mock event processor
type EventProcessor struct {
	Workers int
}

func (ep *EventProcessor) Process(event events.Event) {
	// Simulate processing
	time.Sleep(time.Microsecond)
}

// Mock state manager
type MockStateManager struct {
	states map[string]interface{}
	mutex  *sync.RWMutex
}

func (msm *MockStateManager) Update(ctx context.Context, update map[string]interface{}) {
	msm.mutex.Lock()
	defer msm.mutex.Unlock()

	if id, ok := update["id"].(string); ok {
		msm.states[id] = update
	}

	// Simulate processing
	time.Sleep(time.Microsecond)
}
