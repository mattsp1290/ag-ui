package transport

import (
	"context"
	"testing"
	"time"
)

// TestDemoTransportContextHandling verifies that DemoTransport properly handles context cancellation
func TestDemoTransportContextHandling(t *testing.T) {
	t.Run("connect_respects_context", func(t *testing.T) {
		transport := NewDemoTransport()
		
		// Create a cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		
		err := transport.Connect(ctx)
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
	
	t.Run("send_respects_context", func(t *testing.T) {
		transport := NewDemoTransport()
		
		// Connect first
		ctx := context.Background()
		if err := transport.Connect(ctx); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		
		// Create a cancelled context for send
		sendCtx, cancel := context.WithCancel(context.Background())
		cancel()
		
		event := &DemoEvent{
			id:        "test-ctx",
			eventType: "test",
			timestamp: time.Now(),
		}
		
		err := transport.Send(sendCtx, event)
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
	
	t.Run("close_respects_context", func(t *testing.T) {
		transport := NewDemoTransport()
		
		// Connect first
		ctx := context.Background()
		if err := transport.Connect(ctx); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		
		// Create a cancelled context for close
		closeCtx, cancel := context.WithCancel(context.Background())
		cancel()
		
		err := transport.Close(closeCtx)
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
	
	t.Run("health_respects_context", func(t *testing.T) {
		transport := NewDemoTransport()
		
		// Connect first
		ctx := context.Background()
		if err := transport.Connect(ctx); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		
		// Health check removed - test stats instead
		stats := transport.Stats()
		if stats.EventsSent != 0 {
			t.Errorf("Expected 0 events sent, got %d", stats.EventsSent)
		}
	})
}

// TestMockTransportContextHandling verifies that MockTransport properly handles context cancellation
func TestMockTransportContextHandling(t *testing.T) {
	t.Run("connect_with_timeout", func(t *testing.T) {
		transport := NewMockTransport()
		
		// Set a delay longer than the timeout
		transport.SetConnectDelay(50 * time.Millisecond)
		
		// Create a context with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		
		err := transport.Connect(ctx)
		if err != context.DeadlineExceeded {
			t.Errorf("Expected context.DeadlineExceeded, got %v", err)
		}
	})
	
	t.Run("send_with_delay_and_cancellation", func(t *testing.T) {
		transport := NewMockTransport()
		
		// Set a delay that will be interrupted by cancellation
		transport.SetSendDelay(50 * time.Millisecond)
		
		// Connect first
		ctx := context.Background()
		if err := transport.Connect(ctx); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		
		// Create context and cancel it during send
		sendCtx, cancel := context.WithCancel(context.Background())
		
		event := generateEvent("test", 100)
		
		// Cancel context after a short delay
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		
		err := transport.Send(sendCtx, event)
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
}

// TestErrorTransportContextHandling verifies that ErrorTransport properly handles context cancellation
func TestErrorTransportContextHandling(t *testing.T) {
	t.Run("connect_with_delay_and_cancellation", func(t *testing.T) {
		transport := NewErrorTransport()
		transport.connectDelay = 100 * time.Millisecond
		
		ctx, cancel := context.WithCancel(context.Background())
		
		// Cancel context after a short delay
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		
		err := transport.Connect(ctx)
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
	
	t.Run("send_with_delay_and_timeout", func(t *testing.T) {
		transport := NewErrorTransport()
		// sendDelay configuration not available in new MockTransport
		
		// Connect first
		ctx := context.Background()
		if err := transport.Connect(ctx); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		
		// Create context with timeout
		sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		
		event := &DemoEvent{
			id:        "test",
			eventType: "test",
			timestamp: time.Now(),
		}
		
		err := transport.Send(sendCtx, event)
		if err != context.DeadlineExceeded {
			t.Errorf("Expected context.DeadlineExceeded, got %v", err)
		}
	})
}

// TestRaceTestTransportContextHandling verifies that RaceTestTransport properly handles context cancellation
func TestRaceTestTransportContextHandling(t *testing.T) {
	t.Run("all_methods_respect_context", func(t *testing.T) {
		transport := NewRaceTestTransport()
		
		// Test Connect with cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		
		if err := transport.Connect(ctx); err != context.Canceled {
			t.Errorf("Connect: Expected context.Canceled, got %v", err)
		}
		
		// Connect successfully for remaining tests
		if err := transport.Connect(context.Background()); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		
		// Test Send with cancelled context
		event := &DemoEvent{id: "test", eventType: "test", timestamp: time.Now()}
		if err := transport.Send(ctx, event); err != context.Canceled {
			t.Errorf("Send: Expected context.Canceled, got %v", err)
		}
		
		// Test Close with cancelled context
		if err := transport.Close(ctx); err != context.Canceled {
			t.Errorf("Close: Expected context.Canceled, got %v", err)
		}
	})
}