// Package main demonstrates enhanced event handlers with compression, resilience,
// and advanced synchronization features for production use.
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/state"
)

// SimulatedNetwork represents network conditions for testing
type SimulatedNetwork struct {
	latencyMs    int
	packetLoss   float64
	jitter       int
	connected    bool
	mu           sync.RWMutex
}

// RemoteClient simulates a remote client with network conditions
type RemoteClient struct {
	id           string
	store        *state.StateStore
	handler      *state.StateEventHandler
	network      *SimulatedNetwork
	receivedEvents int
	missedEvents   int
	mu           sync.RWMutex
}

func main() {
	ctx := context.Background()
	
	fmt.Println("=== Enhanced Event Handlers Demo ===\n")
	
	// Run demonstrations
	demonstrateCompressionFeatures(ctx)
	demonstrateResilienceFeatures(ctx)
	demonstrateAdvancedSync(ctx)
	demonstrateBatchingOptimization(ctx)
	demonstrateEventOrdering(ctx)
	demonstrateBackpressureHandling(ctx)
}

func demonstrateCompressionFeatures(ctx context.Context) {
	fmt.Println("1. Event Compression Demo")
	fmt.Println("-------------------------")
	
	// Create stores
	sourceStore := state.NewStateStore()
	targetStore := state.NewStateStore()
	
	// Create event handler with compression
	handler := state.NewStateEventHandler(
		targetStore,
		state.WithCompressionThreshold(1024),     // Compress events > 1KB
		state.WithCompressionLevel(6),             // Balanced compression
		state.WithBatchSize(50),
		state.WithBatchTimeout(100*time.Millisecond),
		state.WithSnapshotCallback(func(event *events.StateSnapshotEvent) error {
			fmt.Printf("  Received snapshot event (ID: %s)\n", event.ID[:8])
			return nil
		}),
		state.WithDeltaCallback(func(event *events.StateDeltaEvent) error {
			fmt.Printf("  Received delta event (ID: %s, ops: %d)\n", 
				event.ID[:8], len(event.Delta))
			return nil
		}),
	)
	
	// Generate large state data
	fmt.Println("  Creating large state data...")
	largeData := generateLargeState(100) // 100 documents
	
	// Set initial state
	for key, value := range largeData {
		sourceStore.Set("/"+key, value)
	}
	
	// Create snapshot event
	generator := state.NewStateEventGenerator(sourceStore)
	snapshotEvent, err := generator.GenerateSnapshot()
	if err != nil {
		log.Printf("Failed to generate snapshot: %v", err)
		return
	}
	
	// Show compression results
	originalSize := calculateEventSize(snapshotEvent)
	compressedEvent := compressEvent(snapshotEvent, 6)
	compressedSize := len(compressedEvent)
	
	fmt.Printf("\n  Compression Results:\n")
	fmt.Printf("    Original size: %d bytes (%.2f KB)\n", originalSize, float64(originalSize)/1024)
	fmt.Printf("    Compressed size: %d bytes (%.2f KB)\n", compressedSize, float64(compressedSize)/1024)
	fmt.Printf("    Compression ratio: %.2f%%\n", float64(compressedSize)/float64(originalSize)*100)
	fmt.Printf("    Space saved: %.2f KB\n", float64(originalSize-compressedSize)/1024)
	
	// Test different compression levels
	fmt.Println("\n  Compression Level Comparison:")
	testCompressionLevels(snapshotEvent)
	
	// Process compressed event
	if err := handler.HandleStateSnapshot(snapshotEvent); err != nil {
		log.Printf("Failed to handle snapshot: %v", err)
	}
	
	fmt.Println()
}

func demonstrateResilienceFeatures(ctx context.Context) {
	fmt.Println("2. Connection Resilience Demo")
	fmt.Println("-----------------------------")
	
	// Create main store
	mainStore := state.NewStateStore()
	
	// Create remote clients with different network conditions
	clients := []*RemoteClient{
		createRemoteClient("client-1", &SimulatedNetwork{
			latencyMs:  20,
			packetLoss: 0.05, // 5% packet loss
			jitter:     10,
			connected:  true,
		}),
		createRemoteClient("client-2", &SimulatedNetwork{
			latencyMs:  100,
			packetLoss: 0.15, // 15% packet loss
			jitter:     50,
			connected:  true,
		}),
		createRemoteClient("client-3", &SimulatedNetwork{
			latencyMs:  500,
			packetLoss: 0.30, // 30% packet loss
			jitter:     200,
			connected:  true,
		}),
	}
	
	// Create resilient event handlers for each client
	for _, client := range clients {
		client.handler = state.NewStateEventHandler(
			client.store,
			state.WithMaxRetries(5),
			state.WithRetryDelay(100*time.Millisecond),
			state.WithRetryBackoffMultiplier(1.5),
			state.WithClientID(client.id),
			state.WithConnectionHealth(state.NewConnectionHealth()),
		)
	}
	
	fmt.Println("  Simulating state changes with network issues...")
	
	// Generate state changes
	generator := state.NewStateEventGenerator(mainStore)
	eventCount := 50
	
	for i := 0; i < eventCount; i++ {
		// Make state change
		path := fmt.Sprintf("/resilience/item_%d", i)
		mainStore.Set(path, map[string]interface{}{
			"id":        i,
			"value":     rand.Float64() * 100,
			"timestamp": time.Now().Unix(),
		})
		
		// Generate delta event
		deltaEvent, err := generator.GenerateDelta(
			mainStore.GetState(),
			mainStore.GetState(),
		)
		if err != nil {
			continue
		}
		
		// Send to each client
		for _, client := range clients {
			go func(c *RemoteClient, event *events.StateDeltaEvent) {
				// Simulate network conditions
				if !c.simulateNetworkTransmission() {
					c.mu.Lock()
					c.missedEvents++
					c.mu.Unlock()
					return
				}
				
				// Handle event with retries
				if err := c.handler.HandleStateDelta(event); err != nil {
					log.Printf("Client %s failed to handle event: %v", c.id, err)
				} else {
					c.mu.Lock()
					c.receivedEvents++
					c.mu.Unlock()
				}
			}(client, deltaEvent)
		}
		
		// Simulate intermittent disconnections
		if i == 20 {
			fmt.Println("  Simulating network outage for client-2...")
			clients[1].network.mu.Lock()
			clients[1].network.connected = false
			clients[1].network.mu.Unlock()
		}
		if i == 30 {
			fmt.Println("  Restoring network for client-2...")
			clients[1].network.mu.Lock()
			clients[1].network.connected = true
			clients[1].network.mu.Unlock()
		}
		
		time.Sleep(50 * time.Millisecond)
	}
	
	// Wait for completion
	time.Sleep(2 * time.Second)
	
	// Show results
	fmt.Println("\n  Resilience Test Results:")
	fmt.Printf("    Total events sent: %d\n\n", eventCount)
	
	for _, client := range clients {
		client.mu.RLock()
		received := client.receivedEvents
		missed := client.missedEvents
		client.mu.RUnlock()
		
		successRate := float64(received) / float64(eventCount) * 100
		fmt.Printf("    %s:\n", client.id)
		fmt.Printf("      Network: %dms latency, %.0f%% loss\n", 
			client.network.latencyMs, client.network.packetLoss*100)
		fmt.Printf("      Events received: %d/%d (%.1f%%)\n", 
			received, eventCount, successRate)
		fmt.Printf("      Events missed: %d\n", missed)
		fmt.Printf("      Connection health: %s\n", 
			client.handler.GetConnectionHealth().Status)
		fmt.Println()
	}
}

func demonstrateAdvancedSync(ctx context.Context) {
	fmt.Println("3. Advanced Synchronization Demo")
	fmt.Println("--------------------------------")
	
	// Create sync manager for coordination
	syncManager := state.NewSyncManager()
	
	// Create master store
	masterStore := state.NewStateStore()
	
	// Create replica stores
	replicas := make([]*state.StateStore, 3)
	handlers := make([]*state.StateEventHandler, 3)
	
	for i := 0; i < 3; i++ {
		replicas[i] = state.NewStateStore()
		handlers[i] = state.NewStateEventHandler(
			replicas[i],
			state.WithSyncManager(syncManager),
			state.WithClientID(fmt.Sprintf("replica-%d", i+1)),
			state.WithConflictResolver(state.NewLastWriteWinsResolver()),
		)
	}
	
	fmt.Println("  Setting up master-replica synchronization...")
	
	// Subscribe replicas to master changes
	masterStore.Subscribe("/", func(change state.StateChange) {
		// Generate event for change
		generator := state.NewStateEventGenerator(masterStore)
		deltaEvent, err := generator.GenerateDelta(
			change.OldValue.(map[string]interface{}),
			change.NewValue.(map[string]interface{}),
		)
		if err != nil {
			return
		}
		
		// Broadcast to replicas
		for _, handler := range handlers {
			go handler.HandleStateDelta(deltaEvent)
		}
	})
	
	// Demonstrate synchronized updates
	fmt.Println("\n  Performing synchronized updates...")
	
	// Update 1: Simple update
	masterStore.Set("/sync/counter", 0)
	time.Sleep(100 * time.Millisecond)
	
	// Update 2: Concurrent increments
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			
			current, _ := masterStore.Get("/sync/counter")
			if val, ok := current.(int); ok {
				masterStore.Set("/sync/counter", val+1)
			}
			
			// Also update a unique key
			masterStore.Set(fmt.Sprintf("/sync/worker_%d", n), map[string]interface{}{
				"id":        n,
				"timestamp": time.Now().Unix(),
			})
		}(i)
	}
	wg.Wait()
	time.Sleep(500 * time.Millisecond)
	
	// Verify synchronization
	fmt.Println("\n  Synchronization Results:")
	masterCounter, _ := masterStore.Get("/sync/counter")
	fmt.Printf("    Master counter: %v\n", masterCounter)
	
	for i, replica := range replicas {
		replicaCounter, _ := replica.Get("/sync/counter")
		workerKeys := 0
		replica.Subscribe("/sync/worker_", func(change state.StateChange) {
			workerKeys++
		})
		
		fmt.Printf("    Replica %d - counter: %v, synced: %v\n", 
			i+1, replicaCounter, replicaCounter == masterCounter)
	}
	
	// Demonstrate conflict resolution
	fmt.Println("\n  Testing conflict resolution...")
	
	// Create conflicting updates
	masterStore.Set("/sync/conflict_test", map[string]interface{}{
		"value":     "master",
		"timestamp": time.Now().Unix(),
	})
	
	// Simulate delayed replica update
	replicas[0].Set("/sync/conflict_test", map[string]interface{}{
		"value":     "replica",
		"timestamp": time.Now().Add(-1 * time.Second).Unix(),
	})
	
	// Force sync
	time.Sleep(200 * time.Millisecond)
	
	// Check resolution
	masterValue, _ := masterStore.Get("/sync/conflict_test")
	replicaValue, _ := replicas[0].Get("/sync/conflict_test")
	
	fmt.Printf("    Master value: %v\n", masterValue)
	fmt.Printf("    Replica value after resolution: %v\n", replicaValue)
	fmt.Printf("    Conflict resolved: %v\n", 
		masterValue.(map[string]interface{})["value"] == 
		replicaValue.(map[string]interface{})["value"])
	
	fmt.Println()
}

func demonstrateBatchingOptimization(ctx context.Context) {
	fmt.Println("4. Batching Optimization Demo")
	fmt.Println("-----------------------------")
	
	// Create stores
	sourceStore := state.NewStateStore()
	targetStore := state.NewStateStore()
	
	// Test different batch configurations
	batchConfigs := []struct {
		name        string
		batchSize   int
		batchTimeout time.Duration
	}{
		{"No Batching", 1, 0},
		{"Small Batch", 10, 50 * time.Millisecond},
		{"Medium Batch", 50, 100 * time.Millisecond},
		{"Large Batch", 100, 200 * time.Millisecond},
	}
	
	for _, config := range batchConfigs {
		fmt.Printf("\n  Testing %s (size: %d, timeout: %v):\n", 
			config.name, config.batchSize, config.batchTimeout)
		
		// Reset target store
		targetStore.Clear()
		
		// Create handler with config
		var processedBatches int
		var totalLatency time.Duration
		
		handler := state.NewStateEventHandler(
			targetStore,
			state.WithBatchSize(config.batchSize),
			state.WithBatchTimeout(config.batchTimeout),
			state.WithDeltaCallback(func(event *events.StateDeltaEvent) error {
				processedBatches++
				return nil
			}),
		)
		
		// Generate rapid updates
		updateCount := 1000
		start := time.Now()
		
		for i := 0; i < updateCount; i++ {
			path := fmt.Sprintf("/batch/item_%d", i)
			sourceStore.Set(path, i)
			
			// Generate and send delta
			generator := state.NewStateEventGenerator(sourceStore)
			deltaEvent, _ := generator.GenerateDelta(nil, map[string]interface{}{path: i})
			
			eventStart := time.Now()
			handler.HandleStateDelta(deltaEvent)
			totalLatency += time.Since(eventStart)
		}
		
		// Wait for batch processing
		time.Sleep(config.batchTimeout + 100*time.Millisecond)
		
		duration := time.Since(start)
		avgLatency := totalLatency / time.Duration(updateCount)
		
		fmt.Printf("    Total time: %v\n", duration)
		fmt.Printf("    Throughput: %.0f events/sec\n", float64(updateCount)/duration.Seconds())
		fmt.Printf("    Batches processed: %d\n", processedBatches)
		fmt.Printf("    Avg events per batch: %.1f\n", float64(updateCount)/float64(processedBatches))
		fmt.Printf("    Avg latency: %v\n", avgLatency)
	}
	
	fmt.Println()
}

func demonstrateEventOrdering(ctx context.Context) {
	fmt.Println("5. Event Ordering Guarantee Demo")
	fmt.Println("--------------------------------")
	
	// Create stores
	sourceStore := state.NewStateStore()
	targetStore := state.NewStateStore()
	
	// Create handler with sequence tracking
	handler := state.NewStateEventHandler(
		targetStore,
		state.WithSequenceTracking(true),
		state.WithOutOfOrderBufferSize(100),
	)
	
	fmt.Println("  Sending events with simulated out-of-order delivery...")
	
	// Generate ordered events
	events := make([]*events.StateDeltaEvent, 20)
	generator := state.NewStateEventGenerator(sourceStore)
	
	for i := 0; i < 20; i++ {
		path := fmt.Sprintf("/order/seq_%d", i)
		sourceStore.Set(path, i)
		
		event, _ := generator.GenerateDelta(nil, map[string]interface{}{path: i})
		event.SequenceNumber = int64(i)
		events[i] = event
	}
	
	// Send events out of order
	sendOrder := []int{0, 1, 3, 2, 5, 4, 7, 6, 8, 10, 9, 12, 11, 14, 13, 15, 17, 16, 19, 18}
	
	for _, idx := range sendOrder {
		fmt.Printf("  Sending event %d (seq: %d)\n", idx, events[idx].SequenceNumber)
		
		if err := handler.HandleStateDelta(events[idx]); err != nil {
			log.Printf("Failed to handle event: %v", err)
		}
		
		time.Sleep(50 * time.Millisecond)
	}
	
	// Verify ordering
	fmt.Println("\n  Verifying event ordering in target store...")
	
	ordered := true
	for i := 0; i < 20; i++ {
		path := fmt.Sprintf("/order/seq_%d", i)
		value, err := targetStore.Get(path)
		if err != nil || value != i {
			ordered = false
			fmt.Printf("    Missing or incorrect: %s (expected: %d, got: %v)\n", path, i, value)
		}
	}
	
	if ordered {
		fmt.Println("    ✓ All events processed in correct order")
	} else {
		fmt.Println("    ✗ Event ordering failed")
	}
	
	fmt.Println()
}

func demonstrateBackpressureHandling(ctx context.Context) {
	fmt.Println("6. Backpressure Handling Demo")
	fmt.Println("-----------------------------")
	
	// Create stores
	sourceStore := state.NewStateStore()
	targetStore := state.NewStateStore()
	
	// Create handler with backpressure management
	backpressureManager := state.NewBackpressureManager(
		1000,  // Max queue size
		0.8,   // High watermark (80%)
		0.2,   // Low watermark (20%)
	)
	
	handler := state.NewStateEventHandler(
		targetStore,
		state.WithBackpressureManager(backpressureManager),
		state.WithMaxConcurrency(5),
	)
	
	fmt.Println("  Generating high-volume event stream...")
	
	// Metrics
	var sent, dropped, processed int64
	var mu sync.Mutex
	
	// Start monitoring
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			stats := backpressureManager.GetStats()
			mu.Lock()
			fmt.Printf("  Queue: %d/%d (%.1f%%) | Sent: %d | Dropped: %d | Processed: %d\n",
				stats.QueueSize, stats.QueueCapacity, 
				float64(stats.QueueSize)/float64(stats.QueueCapacity)*100,
				sent, dropped, processed)
			mu.Unlock()
			
			if sent >= 5000 {
				return
			}
		}
	}()
	
	// Generate events rapidly
	generator := state.NewStateEventGenerator(sourceStore)
	
	for i := 0; i < 5000; i++ {
		path := fmt.Sprintf("/pressure/item_%d", i)
		sourceStore.Set(path, map[string]interface{}{
			"id":    i,
			"data":  generateRandomData(1024), // 1KB per event
		})
		
		event, _ := generator.GenerateDelta(nil, map[string]interface{}{path: i})
		
		// Try to send with backpressure check
		if backpressureManager.ShouldDrop() {
			mu.Lock()
			dropped++
			mu.Unlock()
		} else {
			mu.Lock()
			sent++
			mu.Unlock()
			
			go func(e *events.StateDeltaEvent) {
				// Simulate slow processing
				time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
				
				if err := handler.HandleStateDelta(e); err == nil {
					mu.Lock()
					processed++
					mu.Unlock()
				}
			}(event)
		}
		
		// Vary sending rate
		if i < 1000 {
			time.Sleep(1 * time.Millisecond) // Fast
		} else if i < 3000 {
			time.Sleep(100 * time.Microsecond) // Very fast
		} else {
			// Burst mode - no delay
		}
	}
	
	// Wait for processing
	time.Sleep(3 * time.Second)
	
	// Final stats
	stats := backpressureManager.GetStats()
	fmt.Println("\n  Backpressure Handling Results:")
	fmt.Printf("    Total events generated: 5000\n")
	fmt.Printf("    Events sent: %d\n", sent)
	fmt.Printf("    Events dropped: %d (%.1f%%)\n", dropped, float64(dropped)/5000*100)
	fmt.Printf("    Events processed: %d\n", processed)
	fmt.Printf("    Peak queue utilization: %.1f%%\n", stats.PeakUtilization*100)
	fmt.Printf("    Backpressure activated: %d times\n", stats.BackpressureCount)
	
	fmt.Println()
}

// Helper functions

func createRemoteClient(id string, network *SimulatedNetwork) *RemoteClient {
	return &RemoteClient{
		id:      id,
		store:   state.NewStateStore(),
		network: network,
	}
}

func (c *RemoteClient) simulateNetworkTransmission() bool {
	c.network.mu.RLock()
	defer c.network.mu.RUnlock()
	
	// Check if connected
	if !c.network.connected {
		return false
	}
	
	// Simulate packet loss
	if rand.Float64() < c.network.packetLoss {
		return false
	}
	
	// Simulate latency with jitter
	latency := c.network.latencyMs + rand.Intn(c.network.jitter*2) - c.network.jitter
	time.Sleep(time.Duration(latency) * time.Millisecond)
	
	return true
}

func generateLargeState(documentCount int) map[string]interface{} {
	state := make(map[string]interface{})
	
	for i := 0; i < documentCount; i++ {
		docID := fmt.Sprintf("document_%d", i)
		state[docID] = map[string]interface{}{
			"id":          docID,
			"title":       fmt.Sprintf("Document %d", i),
			"content":     generateRandomText(500), // 500 words
			"metadata":    generateMetadata(),
			"tags":        generateTags(),
			"created_at":  time.Now().Add(-time.Duration(rand.Intn(365)) * 24 * time.Hour),
			"modified_at": time.Now().Add(-time.Duration(rand.Intn(30)) * 24 * time.Hour),
			"version":     rand.Intn(10) + 1,
		}
	}
	
	return state
}

func generateRandomText(words int) string {
	wordList := []string{"lorem", "ipsum", "dolor", "sit", "amet", "consectetur", 
		"adipiscing", "elit", "sed", "do", "eiusmod", "tempor", "incididunt", 
		"ut", "labore", "et", "dolore", "magna", "aliqua"}
	
	text := ""
	for i := 0; i < words; i++ {
		text += wordList[rand.Intn(len(wordList))] + " "
	}
	
	return text
}

func generateMetadata() map[string]interface{} {
	return map[string]interface{}{
		"author":     fmt.Sprintf("user_%d", rand.Intn(100)),
		"department": []string{"engineering", "sales", "marketing", "hr"}[rand.Intn(4)],
		"priority":   []string{"low", "medium", "high", "critical"}[rand.Intn(4)],
		"status":     []string{"draft", "review", "approved", "published"}[rand.Intn(4)],
	}
}

func generateTags() []string {
	allTags := []string{"important", "urgent", "review", "archive", "public", 
		"private", "confidential", "draft", "final", "todo"}
	
	tagCount := rand.Intn(5) + 1
	tags := make([]string, tagCount)
	
	for i := 0; i < tagCount; i++ {
		tags[i] = allTags[rand.Intn(len(allTags))]
	}
	
	return tags
}

func generateRandomData(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}

func calculateEventSize(event *events.StateSnapshotEvent) int {
	data, _ := json.Marshal(event)
	return len(data)
}

func compressEvent(event *events.StateSnapshotEvent, level int) []byte {
	data, _ := json.Marshal(event)
	
	var buf bytes.Buffer
	writer, _ := gzip.NewWriterLevel(&buf, level)
	writer.Write(data)
	writer.Close()
	
	return buf.Bytes()
}

func testCompressionLevels(event *events.StateSnapshotEvent) {
	originalSize := calculateEventSize(event)
	
	fmt.Printf("    Level | Size (KB) | Ratio | Time\n")
	fmt.Printf("    ------|-----------|-------|------\n")
	
	for level := 1; level <= 9; level++ {
		start := time.Now()
		compressed := compressEvent(event, level)
		duration := time.Since(start)
		
		size := len(compressed)
		ratio := float64(size) / float64(originalSize) * 100
		
		fmt.Printf("    %5d | %9.2f | %5.1f%% | %v\n", 
			level, float64(size)/1024, ratio, duration)
	}
}