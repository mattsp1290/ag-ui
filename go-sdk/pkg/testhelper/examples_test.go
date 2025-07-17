package testhelper

import (
	"net"
	"sync"
	"testing"
	"time"
)

// TestComprehensiveExample demonstrates using all test helpers together
func TestComprehensiveExample(t *testing.T) {
	// Always add goroutine leak detection first
	defer VerifyNoGoroutineLeaks(t)

	// Set up all cleanup helpers
	cleanup := NewCleanupManager(t)
	channelCleanup := NewChannelCleanup(t)
	networkCleanup := NewNetworkCleanup(t)
	resourceTracker := NewResourceTracker(t)

	// Create test context with timeout
	ctx := NewTestContextWithTimeout(t, 30*time.Second)

	// 1. Network resources
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	networkCleanup.AddListener(listener)
	resourceTracker.Allocated("listener")

	// 2. Channels for communication
	workCh := make(chan int, 10)
	doneCh := make(chan struct{})
	resultCh := make(chan string, 5)

	AddChan(channelCleanup, "work-channel", workCh)
	AddChan(channelCleanup, "done-channel", doneCh)
	AddChan(channelCleanup, "result-channel", resultCh)

	resourceTracker.Allocated("work-channel")
	resourceTracker.Allocated("done-channel")
	resourceTracker.Allocated("result-channel")

	// 3. Start workers
	var wg sync.WaitGroup
	workerCount := 3

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for {
				select {
				case work := <-workCh:
					// Simulate work
					time.Sleep(10 * time.Millisecond)
					resultCh <- "completed-" + string(rune(work))
				case <-doneCh:
					t.Logf("Worker %d shutting down", id)
					return
				case <-ctx.Done():
					t.Logf("Worker %d cancelled", id)
					return
				}
			}
		}(i)
	}

	// Register worker cleanup
	cleanup.Register("workers", func() {
		close(doneCh)
		if !WaitGroupTimeout(t, &wg, 2*time.Second) {
			t.Log("Workers did not shut down gracefully")
		}
		resourceTracker.Cleaned("workers")
	})

	// 4. Send some work
	for i := 0; i < 5; i++ {
		select {
		case workCh <- i:
		case <-ctx.Done():
			t.Fatal("Context cancelled while sending work")
		}
	}

	// 5. Collect results with timeout
	guard := NewTimeoutGuard(t, 5*time.Second)
	err = guard.Run("collect-results", func() error {
		for i := 0; i < 5; i++ {
			select {
			case result := <-resultCh:
				t.Logf("Got result: %s", result)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})

	if err != nil {
		t.Errorf("Failed to collect results: %v", err)
	}

	// 6. Test error handling
	EnsureCleanup(t, "error-prone-operation", func() {
		// Simulate an operation that might panic
		time.Sleep(10 * time.Millisecond)
	}, func() {
		t.Log("Cleanup from error-prone operation")
	})

	// Mark resources as cleaned up
	resourceTracker.Cleaned("work-channel")
	resourceTracker.Cleaned("done-channel")
	resourceTracker.Cleaned("result-channel")
	resourceTracker.Cleaned("listener")

	// All cleanup happens automatically via t.Cleanup()
}

// ExampleDistributedSystemTest shows how to test distributed systems
func TestDistributedSystemExample(t *testing.T) {
	defer VerifyNoGoroutineLeaks(t)

	// Set up monitoring of goroutines during test
	stopMonitor := MonitorGoroutines(t, 1*time.Second)
	defer stopMonitor()

	// Set up parallel contexts for multiple components
	pc := NewParallelContexts(t)

	// Component 1: Message processor
	ctx1 := pc.Create("processor")
	go func() {
		<-ctx1.Done()
		t.Log("Message processor stopped")
	}()

	// Component 2: Event handler
	ctx2 := pc.Create("handler")
	go func() {
		<-ctx2.Done()
		t.Log("Event handler stopped")
	}()

	// Component 3: State synchronizer
	ctx3 := pc.Create("sync")
	go func() {
		<-ctx3.Done()
		t.Log("State synchronizer stopped")
	}()

	// Simulate some distributed work
	time.Sleep(100 * time.Millisecond)

	// Stop components in order
	pc.Cancel("processor")
	pc.Cancel("handler")
	pc.Cancel("sync")

	// Wait for goroutines to settle
	WaitForGoroutines(t, 0, 2*time.Second)

	// Check for blocked goroutines
	DetectBlockedGoroutines(t)
}

// ExampleNetworkServiceTest shows testing network services
func TestNetworkServiceExample(t *testing.T) {
	defer VerifyNoGoroutineLeaks(t)

	ctx := NewTestContext(t)
	cleanup := NewCleanupManager(t)
	networkCleanup := NewNetworkCleanup(t)

	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	networkCleanup.AddListener(listener)

	// Server goroutine
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					select {
					case <-ctx.Done():
						return // Expected during shutdown
					default:
						t.Logf("Accept error: %v", err)
						return
					}
				}

				networkCleanup.AddConnection(conn)

				// Handle connection
				go func() {
					defer conn.Close()
					<-ctx.Done()
				}()
			}
		}
	}()

	// Register server cleanup
	cleanup.Register("server", func() {
		ctx.Cancel()
		select {
		case <-serverDone:
			t.Log("Server stopped gracefully")
		case <-time.After(2 * time.Second):
			t.Log("Server stop timeout")
		}
	})

	// Create some client connections
	for i := 0; i < 3; i++ {
		conn, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			t.Errorf("Failed to connect: %v", err)
			continue
		}
		networkCleanup.AddConnection(conn)
	}

	// Simulate some network activity
	time.Sleep(100 * time.Millisecond)

	// All cleanup happens automatically
}

// ExampleContextManagementTest shows advanced context management
func TestContextManagementExample(t *testing.T) {
	defer VerifyNoGoroutineLeaks(t)

	cm := NewContextManager(t)

	// Create different contexts for different operations
	fastCtx := cm.CreateWithTimeout("fast-operation", 1*time.Second)
	slowCtx := cm.CreateWithTimeout("slow-operation", 5*time.Second)
	backgroundCtx := cm.Create("background-work")

	// Start operations
	fastDone := make(chan bool)
	go func() {
		select {
		case <-time.After(500 * time.Millisecond):
			fastDone <- true
		case <-fastCtx.Done():
			fastDone <- false
		}
	}()

	slowDone := make(chan bool)
	go func() {
		select {
		case <-time.After(2 * time.Second):
			slowDone <- true
		case <-slowCtx.Done():
			slowDone <- false
		}
	}()

	backgroundDone := make(chan bool)
	go func() {
		select {
		case <-time.After(100 * time.Millisecond):
			backgroundDone <- true
		case <-backgroundCtx.Done():
			backgroundDone <- false
		}
	}()

	// Wait for operations
	if !<-fastDone {
		t.Error("Fast operation should have completed")
	}

	if !<-backgroundDone {
		t.Error("Background operation should have completed")
	}

	if !<-slowDone {
		t.Error("Slow operation should have completed")
	}

	// Test context assertions
	cm.Cancel("background-work")
	AssertContextCancelled(t, backgroundCtx, 100*time.Millisecond)
}

// BenchmarkGoroutineLeakDetection benchmarks the leak detector
func BenchmarkGoroutineLeakDetection(b *testing.B) {
	for i := 0; i < b.N; i++ {
		detector := NewGoroutineLeakDetector(&testing.T{})
		detector.Start()

		// Simulate some goroutines
		done := make(chan struct{})
		for j := 0; j < 10; j++ {
			go func() {
				<-done
			}()
		}

		// Clean up
		close(done)
		time.Sleep(10 * time.Millisecond)

		detector.Check()
	}
}
