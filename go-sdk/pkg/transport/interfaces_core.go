package transport

import (
	"context"
	"time"
	
	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// TransportEvent represents a transport event with metadata.
//
// Example usage:
//
//	// Creating a custom transport event
//	type CustomEvent struct {
//		id        string
//		eventType string
//		timestamp time.Time
//		data      map[string]interface{}
//	}
//
//	func (e *CustomEvent) ID() string { return e.id }
//	func (e *CustomEvent) Type() string { return e.eventType }
//	func (e *CustomEvent) Timestamp() time.Time { return e.timestamp }
//	func (e *CustomEvent) Data() map[string]interface{} { return e.data }
//
//	// Usage in transport
//	event := &CustomEvent{
//		id:        "evt-123",
//		eventType: "user.action",
//		timestamp: time.Now(),
//		data:      map[string]interface{}{"action": "click", "target": "button"},
//	}
type TransportEvent interface {
	// ID returns the unique identifier for this event
	ID() string
	
	// Type returns the event type
	Type() string
	
	// Timestamp returns when the event was created
	Timestamp() time.Time
	
	// Data returns the event data as a map for backward compatibility
	// Deprecated: Data will be removed on 2025-03-31. Use specific typed methods when available for better type safety and performance.
	Data() map[string]interface{}
}

// Connector handles connection lifecycle
//
// Example usage:
//
//	// Basic connection management
//	func connectTransport(transport Connector) error {
//		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//		defer cancel()
//		
//		if transport.IsConnected() {
//			return nil // Already connected
//		}
//		
//		if err := transport.Connect(ctx); err != nil {
//			return fmt.Errorf("failed to connect: %w", err)
//		}
//		
//		log.Println("Transport connected successfully")
//		return nil
//	}
//
//	// Graceful shutdown
//	func shutdownTransport(transport Connector) error {
//		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//		defer cancel()
//		
//		if !transport.IsConnected() {
//			return nil // Already disconnected
//		}
//		
//		return transport.Close(ctx)
//	}
type Connector interface {
	// Connect establishes a connection to the remote endpoint.
	Connect(ctx context.Context) error
	
	// Close closes the connection and releases resources.
	Close(ctx context.Context) error
	
	// IsConnected returns true if currently connected.
	IsConnected() bool
}

// Sender handles sending events
//
// Example usage:
//
//	// Sending a single event
//	func sendUserAction(sender Sender, userID string, action string) error {
//		event := &CustomEvent{
//			id:        fmt.Sprintf("action-%d", time.Now().UnixNano()),
//			eventType: "user.action",
//			timestamp: time.Now(),
//			data: map[string]interface{}{
//				"user_id": userID,
//				"action":  action,
//			},
//		}
//		
//		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//		defer cancel()
//		
//		return sender.Send(ctx, event)
//	}
//
//	// Sending with retry logic
//	func sendWithRetry(sender Sender, event TransportEvent, maxRetries int) error {
//		for i := 0; i < maxRetries; i++ {
//			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//			err := sender.Send(ctx, event)
//			cancel()
//			
//			if err == nil {
//				return nil
//			}
//			
//			if i < maxRetries-1 {
//				time.Sleep(time.Duration(i+1) * time.Second)
//			}
//		}
//		return fmt.Errorf("failed to send after %d retries", maxRetries)
//	}
type Sender interface {
	// Send sends an event to the remote endpoint.
	Send(ctx context.Context, event TransportEvent) error
}

// Receiver handles receiving events
//
// Example usage:
//
//	// Basic event processing
//	func processEvents(receiver Receiver) {
//		eventCh, errorCh := receiver.Channels()
//		
//		for {
//			select {
//			case event := <-eventCh:
//				if event != nil {
//					log.Printf("Received event: %s", event.Type())
//					// Process the event
//				}
//			case err := <-errorCh:
//				if err != nil {
//					log.Printf("Transport error: %v", err)
//					// Handle the error
//				}
//			}
//		}
//	}
//
//	// Event processing with graceful shutdown
//	func processEventsWithShutdown(receiver Receiver, ctx context.Context) {
//		eventCh, errorCh := receiver.Channels()
//		
//		for {
//			select {
//			case <-ctx.Done():
//				log.Println("Shutting down event processing")
//				return
//			case event := <-eventCh:
//				if event != nil {
//					handleEvent(event)
//				}
//			case err := <-errorCh:
//				if err != nil {
//					handleError(err)
//				}
//			}
//		}
//	}
type Receiver interface {
	// Channels returns event and error channels.
	Channels() (<-chan events.Event, <-chan error)
}

// ConfigProvider provides configuration access
type ConfigProvider interface {
	// Config returns the transport's configuration.
	Config() Config
}

// StatsProvider provides statistics access
type StatsProvider interface {
	// Stats returns transport statistics and metrics.
	Stats() TransportStats
}

// Transport is the core transport interface composed of smaller interfaces
//
// Example usage:
//
//	// Complete transport usage pattern
//	func useTransport(transport Transport) error {
//		// Connect
//		ctx := context.Background()
//		if err := transport.Connect(ctx); err != nil {
//			return fmt.Errorf("connection failed: %w", err)
//		}
//		defer transport.Close(ctx)
//		
//		// Start receiving events
//		go func() {
//			eventCh, errorCh := transport.Channels()
//			for {
//				select {
//				case event := <-eventCh:
//					handleReceivedEvent(event)
//				case err := <-errorCh:
//					log.Printf("Transport error: %v", err)
//				}
//			}
//		}()
//		
//		// Send an event
//		event := createEvent("test.event", map[string]interface{}{"data": "test"})
//		if err := transport.Send(ctx, event); err != nil {
//			return fmt.Errorf("send failed: %w", err)
//		}
//		
//		// Check stats
//		stats := transport.Stats()
//		log.Printf("Messages sent: %d, received: %d", stats.MessagesSent, stats.MessagesReceived)
//		
//		return nil
//	}
//
//	// Creating a transport factory
//	func createHTTPTransport(config Config) (Transport, error) {
//		// Implementation would return a concrete transport
//		return nil, nil
//	}
type Transport interface {
	Connector
	Sender
	Receiver
	ConfigProvider
	StatsProvider
}

// BatchSender handles batch operations
type BatchSender interface {
	// SendBatch sends multiple events in a single batch operation.
	SendBatch(ctx context.Context, events []TransportEvent) error
}

// EventHandlerProvider allows setting event handlers
type EventHandlerProvider interface {
	// SetEventHandler sets a callback function to handle received events.
	SetEventHandler(handler EventHandler)
}

// StreamController controls streaming operations
type StreamController interface {
	// StartStreaming begins streaming events in both directions.
	StartStreaming(ctx context.Context) (send chan<- TransportEvent, receive <-chan events.Event, errors <-chan error, err error)
}

// StreamingStatsProvider provides streaming-specific statistics
type StreamingStatsProvider interface {
	// GetStreamingStats returns streaming-specific statistics.
	GetStreamingStats() StreamingStats
}

// StreamingTransport extends Transport with streaming capabilities
//
// Example usage:
//
//	// Full streaming transport usage
//	func useStreamingTransport(transport StreamingTransport) error {
//		ctx := context.Background()
//		
//		// Connect
//		if err := transport.Connect(ctx); err != nil {
//			return err
//		}
//		defer transport.Close(ctx)
//		
//		// Set up event handler
//		transport.SetEventHandler(func(ctx context.Context, event events.Event) error {
//			log.Printf("Handled event: %s", event.Type())
//			return nil
//		})
//		
//		// Start streaming
//		sendCh, receiveCh, errorCh, err := transport.StartStreaming(ctx)
//		if err != nil {
//			return fmt.Errorf("failed to start streaming: %w", err)
//		}
//		
//		// Send events via streaming
//		go func() {
//			for i := 0; i < 10; i++ {
//				event := createEvent("stream.event", map[string]interface{}{"index": i})
//				select {
//				case sendCh <- event:
//					log.Printf("Sent streaming event %d", i)
//				case <-ctx.Done():
//					return
//				}
//			}
//		}()
//		
//		// Process streaming events
//		go func() {
//			for {
//				select {
//				case event := <-receiveCh:
//					log.Printf("Received streaming event: %s", event.Type())
//				case err := <-errorCh:
//					log.Printf("Streaming error: %v", err)
//				case <-ctx.Done():
//					return
//				}
//			}
//		}()
//		
//		// Send batch events
//		batchEvents := make([]TransportEvent, 5)
//		for i := range batchEvents {
//			batchEvents[i] = createEvent("batch.event", map[string]interface{}{"index": i})
//		}
//		if err := transport.SendBatch(ctx, batchEvents); err != nil {
//			return fmt.Errorf("batch send failed: %w", err)
//		}
//		
//		// Check streaming stats
//		streamStats := transport.GetStreamingStats()
//		log.Printf("Active streams: %d, total throughput: %f/s", 
//			streamStats.ActiveStreams, streamStats.TotalThroughput)
//		
//		return nil
//	}
type StreamingTransport interface {
	Transport
	BatchSender
	EventHandlerProvider
	StreamController
	StreamingStatsProvider
}

// ReliableSender handles reliable event delivery
type ReliableSender interface {
	// SendEventWithAck sends an event and waits for acknowledgment.
	SendEventWithAck(ctx context.Context, event TransportEvent, timeout time.Duration) error
}

// AckHandlerProvider allows setting acknowledgment handlers
type AckHandlerProvider interface {
	// SetAckHandler sets a callback for handling acknowledgments.
	SetAckHandler(handler AckHandler)
}

// ReliabilityStatsProvider provides reliability statistics
type ReliabilityStatsProvider interface {
	// GetReliabilityStats returns reliability-specific statistics.
	GetReliabilityStats() ReliabilityStats
}

// ReliableTransport extends Transport with reliability features
//
// Example usage:
//
//	// Using reliable transport with acknowledgments
//	func useReliableTransport(transport ReliableTransport) error {
//		ctx := context.Background()
//		
//		// Connect
//		if err := transport.Connect(ctx); err != nil {
//			return err
//		}
//		defer transport.Close(ctx)
//		
//		// Set up acknowledgment handler
//		transport.SetAckHandler(func(ctx context.Context, eventID string, success bool) error {
//			if success {
//				log.Printf("Event %s acknowledged successfully", eventID)
//			} else {
//				log.Printf("Event %s failed acknowledgment", eventID)
//			}
//			return nil
//		})
//		
//		// Send event with acknowledgment
//		event := createEvent("reliable.event", map[string]interface{}{"critical": true})
//		timeout := 10 * time.Second
//		
//		if err := transport.SendEventWithAck(ctx, event, timeout); err != nil {
//			log.Printf("Failed to send with ack: %v", err)
//			return err
//		}
//		
//		log.Printf("Event sent and acknowledged: %s", event.ID())
//		
//		// Check reliability stats
//		reliabilityStats := transport.GetReliabilityStats()
//		log.Printf("Ack rate: %.2f%%, retry count: %d", 
//			reliabilityStats.AckRate*100, reliabilityStats.RetryCount)
//		
//		return nil
//	}
//
//	// Sending critical events with retry
//	func sendCriticalEvent(transport ReliableTransport, event TransportEvent) error {
//		ctx := context.Background()
//		maxRetries := 3
//		baseTimeout := 5 * time.Second
//		
//		for attempt := 0; attempt < maxRetries; attempt++ {
//			timeout := baseTimeout * time.Duration(attempt+1)
//			err := transport.SendEventWithAck(ctx, event, timeout)
//			
//			if err == nil {
//				return nil // Success
//			}
//			
//			log.Printf("Attempt %d failed: %v", attempt+1, err)
//			if attempt < maxRetries-1 {
//				time.Sleep(time.Duration(attempt+1) * time.Second)
//			}
//		}
//		
//		return fmt.Errorf("failed to send critical event after %d attempts", maxRetries)
//	}
type ReliableTransport interface {
	Transport
	ReliableSender
	AckHandlerProvider
	ReliabilityStatsProvider
}

// EventHandler is a callback function for handling received events.
type EventHandler func(ctx context.Context, event events.Event) error

// AckHandler is a callback function for handling acknowledgments.
type AckHandler func(ctx context.Context, eventID string, success bool) error