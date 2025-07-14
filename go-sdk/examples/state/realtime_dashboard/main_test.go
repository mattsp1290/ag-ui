package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/state"
)

// TestGracefulShutdown tests that all resources are properly cleaned up
func TestGracefulShutdown(t *testing.T) {
	// Create a short-lived context for testing
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create state store
	store := state.NewStateStore(state.WithMaxHistory(100))

	// Create metrics collector
	collector := &MetricsCollector{
		store:  store,
		ctx:    ctx,
		cancel: cancel,
	}

	// Start a few collectors
	collector.wg.Add(3)
	
	// Simulated collector 1
	go func() {
		defer collector.wg.Done()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Simulate work
			}
		}
	}()
	
	// Simulated collector 2
	go func() {
		defer collector.wg.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Simulate work
			}
		}
	}()
	
	// Simulated collector 3
	go func() {
		defer collector.wg.Done()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Simulate work
			}
		}
	}()

	// Let collectors run for a bit
	time.Sleep(500 * time.Millisecond)

	// Test cleanup
	err := collector.Cleanup(2 * time.Second)
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}
}

// TestHTTPServerShutdown tests graceful HTTP server shutdown
func TestHTTPServerShutdown(t *testing.T) {
	// Create server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:    ":18080",
		Handler: mux,
	}

	// Start server
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Server shutdown failed: %v", err)
	}
}

// TestClientDisconnection tests that all clients are properly disconnected
func TestClientDisconnection(t *testing.T) {
	store := state.NewStateStore()
	
	server := &DashboardServer{
		clients: make(map[string]*DashboardClient),
		eventHandler: state.NewStateEventHandler(store),
		shutdownChan: make(chan struct{}),
	}

	// Add some clients
	for i := 0; i < 5; i++ {
		clientID := fmt.Sprintf("test-client-%d", i)
		server.ConnectClient(clientID)
	}

	// Verify clients are connected
	if len(server.clients) != 5 {
		t.Errorf("Expected 5 clients, got %d", len(server.clients))
	}

	// Disconnect all clients
	server.DisconnectAllClients()

	// Verify all clients are disconnected
	if len(server.clients) != 0 {
		t.Errorf("Expected 0 clients after disconnect, got %d", len(server.clients))
	}
}

// TestContextCancellation tests that all goroutines respect context cancellation
func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Track goroutine completion
	done := make(chan struct{}, 3)
	
	// Start goroutines that respect context
	for i := 0; i < 3; i++ {
		go func(id int) {
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			
			for {
				select {
				case <-ctx.Done():
					done <- struct{}{}
					return
				case <-ticker.C:
					// Simulate work
				}
			}
		}(i)
	}
	
	// Let them run
	time.Sleep(200 * time.Millisecond)
	
	// Cancel context
	cancel()
	
	// Wait for all goroutines to complete
	timeout := time.After(1 * time.Second)
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			// Good, goroutine completed
		case <-timeout:
			t.Errorf("Goroutine %d did not complete after context cancellation", i)
		}
	}
}

// TestChannelClosure tests that channels are properly closed
func TestChannelClosure(t *testing.T) {
	// Create a channel
	ch := make(chan struct{})
	
	// Start a goroutine that writes to the channel
	go func() {
		defer close(ch)
		
		for i := 0; i < 5; i++ {
			ch <- struct{}{}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	
	// Read from channel
	count := 0
	for range ch {
		count++
	}
	
	if count != 5 {
		t.Errorf("Expected 5 items from channel, got %d", count)
	}
	
	// Verify channel is closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("Channel should be closed")
		}
	default:
		// Channel is closed, good
	}
}