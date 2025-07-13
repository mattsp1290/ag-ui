package worker

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestWorkerManager_StartWorker(t *testing.T) {
	wm := NewWorkerManager(nil)
	defer wm.Stop()

	var executed int32
	workerFunc := func(ctx context.Context) error {
		atomic.AddInt32(&executed, 1)
		return nil
	}

	workerID, err := wm.StartWorker("test-worker", workerFunc, nil)
	if err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}

	if workerID == "" {
		t.Fatal("Expected non-empty worker ID")
	}

	// Wait for worker to complete
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&executed) != 1 {
		t.Errorf("Expected worker to be executed once, got %d", atomic.LoadInt32(&executed))
	}

	metrics := wm.GetMetrics()
	if metrics.WorkersCreated != 1 {
		t.Errorf("Expected 1 worker created, got %d", metrics.WorkersCreated)
	}
}

func TestWorkerManager_WorkerTimeout(t *testing.T) {
	wm := NewWorkerManager(nil)
	defer wm.Stop()

	workerFunc := func(ctx context.Context) error {
		// This will timeout
		select {
		case <-time.After(time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	opts := &WorkerOptions{
		Name:    "timeout-worker",
		Timeout: 50 * time.Millisecond,
	}

	_, err := wm.StartWorker("timeout-worker", workerFunc, opts)
	if err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}

	// Wait for timeout
	time.Sleep(200 * time.Millisecond)

	// Worker should have been cancelled due to timeout
	metrics := wm.GetMetrics()
	if metrics.WorkersCompleted != 1 {
		t.Errorf("Expected 1 worker completed, got %d", metrics.WorkersCompleted)
	}
}

func TestWorkerManager_PanicRecovery(t *testing.T) {
	// Create logger with observer to capture logs
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	
	config := &WorkerConfig{
		MaxWorkers:      10,
		ShutdownTimeout: 5 * time.Second,
		Logger:          logger,
		PanicHandler:    NewDefaultPanicHandler(logger),
	}

	wm := NewWorkerManager(config)
	defer wm.Stop()

	workerFunc := func(ctx context.Context) error {
		panic("test panic")
	}

	_, err := wm.StartWorker("panic-worker", workerFunc, nil)
	if err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}

	// Wait for panic recovery
	time.Sleep(100 * time.Millisecond)

	metrics := wm.GetMetrics()
	if metrics.PanicsRecovered != 1 {
		t.Errorf("Expected 1 panic recovered, got %d", metrics.PanicsRecovered)
	}
	
	if metrics.WorkersFailed != 1 {
		t.Errorf("Expected 1 worker failed, got %d", metrics.WorkersFailed)
	}

	// Check that panic was logged
	logs := recorded.All()
	found := false
	for _, log := range logs {
		if log.Message == "Worker panic recovered" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected panic to be logged")
	}
}

func TestWorkerManager_WorkerRetries(t *testing.T) {
	wm := NewWorkerManager(nil)
	defer wm.Stop()

	var attempts int32
	workerFunc := func(ctx context.Context) error {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			return fmt.Errorf("attempt %d failed", count)
		}
		return nil
	}

	opts := &WorkerOptions{
		Name:       "retry-worker",
		MaxRetries: 3,
		RetryDelay: 10 * time.Millisecond,
	}

	_, err := wm.StartWorker("retry-worker", workerFunc, opts)
	if err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}

	// Wait for retries to complete
	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("Expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}

	metrics := wm.GetMetrics()
	if metrics.WorkersCompleted != 1 {
		t.Errorf("Expected 1 worker completed, got %d", metrics.WorkersCompleted)
	}
}

func TestWorkerManager_CancelWorker(t *testing.T) {
	wm := NewWorkerManager(nil)
	defer wm.Stop()

	var cancelled int32
	workerFunc := func(ctx context.Context) error {
		select {
		case <-time.After(time.Second):
			return nil
		case <-ctx.Done():
			atomic.AddInt32(&cancelled, 1)
			return ctx.Err()
		}
	}

	workerID, err := wm.StartWorker("cancel-worker", workerFunc, nil)
	if err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}

	// Give worker time to start
	time.Sleep(50 * time.Millisecond)

	err = wm.CancelWorker(workerID)
	if err != nil {
		t.Fatalf("Failed to cancel worker: %v", err)
	}

	// Wait for cancellation
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&cancelled) != 1 {
		t.Errorf("Expected worker to be cancelled, got %d", atomic.LoadInt32(&cancelled))
	}
}

func TestWorkerManager_MaxWorkersLimit(t *testing.T) {
	config := &WorkerConfig{
		MaxWorkers:      2,
		ShutdownTimeout: 5 * time.Second,
		Logger:          zap.NewNop(),
		PanicHandler:    NewDefaultPanicHandler(zap.NewNop()),
	}

	wm := NewWorkerManager(config)
	defer wm.Stop()

	// Start max workers
	for i := 0; i < 2; i++ {
		_, err := wm.StartWorker(fmt.Sprintf("worker-%d", i), func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}, nil)
		if err != nil {
			t.Fatalf("Failed to start worker %d: %v", i, err)
		}
	}

	// Try to start one more worker (should fail)
	_, err := wm.StartWorker("excess-worker", func(ctx context.Context) error {
		return nil
	}, nil)
	if err == nil {
		t.Error("Expected error when exceeding max workers")
	}
}

func TestWorkerManager_ListWorkers(t *testing.T) {
	wm := NewWorkerManager(nil)
	defer wm.Stop()

	workerFunc := func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	// Start multiple workers
	for i := 0; i < 3; i++ {
		_, err := wm.StartWorker(fmt.Sprintf("worker-%d", i), workerFunc, nil)
		if err != nil {
			t.Fatalf("Failed to start worker %d: %v", i, err)
		}
	}

	// List workers
	workers := wm.ListWorkers()
	if len(workers) != 3 {
		t.Errorf("Expected 3 workers, got %d", len(workers))
	}

	// Check worker names
	names := make(map[string]bool)
	for _, worker := range workers {
		names[worker.Name] = true
	}

	for i := 0; i < 3; i++ {
		expectedName := fmt.Sprintf("worker-%d", i)
		if !names[expectedName] {
			t.Errorf("Expected worker %s not found", expectedName)
		}
	}
}

func TestWorkerManager_GracefulShutdown(t *testing.T) {
	wm := NewWorkerManager(nil)

	// Start multiple workers that will complete quickly
	for i := 0; i < 5; i++ {
		_, err := wm.StartWorker(fmt.Sprintf("worker-%d", i), func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		}, nil)
		if err != nil {
			t.Fatalf("Failed to start worker %d: %v", i, err)
		}
	}

	// Give workers time to complete
	time.Sleep(50 * time.Millisecond)

	// Stop worker manager
	err := wm.Stop()
	if err != nil {
		t.Fatalf("Failed to stop worker manager: %v", err)
	}

	// Check that all workers completed
	metrics := wm.GetMetrics()
	if metrics.WorkersCompleted != 5 {
		t.Errorf("Expected 5 workers completed, got %d", metrics.WorkersCompleted)
	}

	// Manager should not be running
	if wm.IsRunning() {
		t.Error("Expected worker manager to be stopped")
	}
}

func TestWorkerManager_HealthCheck(t *testing.T) {
	wm := NewWorkerManager(nil)
	defer wm.Stop()

	// Health check should pass initially
	err := wm.HealthCheck()
	if err != nil {
		t.Errorf("Expected health check to pass: %v", err)
	}

	// Stop worker manager
	wm.Stop()

	// Health check should fail after stopping
	err = wm.HealthCheck()
	if err == nil {
		t.Error("Expected health check to fail after stopping")
	}
}

func TestWorkerManager_ConvenienceMethods(t *testing.T) {
	wm := NewWorkerManager(nil)
	defer wm.Stop()

	var executed int32
	workerFunc := func(ctx context.Context) error {
		atomic.AddInt32(&executed, 1)
		return nil
	}

	// Test StartBackgroundWorker
	_, err := wm.StartBackgroundWorker("bg-worker", workerFunc)
	if err != nil {
		t.Fatalf("Failed to start background worker: %v", err)
	}

	// Test StartTimedWorker
	_, err = wm.StartTimedWorker("timed-worker", 100*time.Millisecond, workerFunc)
	if err != nil {
		t.Fatalf("Failed to start timed worker: %v", err)
	}

	// Test StartOneOffWorker
	_, err = wm.StartOneOffWorker("oneoff-worker", workerFunc)
	if err != nil {
		t.Fatalf("Failed to start one-off worker: %v", err)
	}

	// Wait for workers to complete
	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&executed) != 3 {
		t.Errorf("Expected 3 workers executed, got %d", atomic.LoadInt32(&executed))
	}
}

func TestWorkerManager_ConcurrentOperations(t *testing.T) {
	wm := NewWorkerManager(nil)
	defer wm.Stop()

	var wg sync.WaitGroup
	var errors []error
	var mu sync.Mutex

	// Start multiple workers concurrently
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			
			_, err := wm.StartWorker(fmt.Sprintf("concurrent-worker-%d", i), func(ctx context.Context) error {
				time.Sleep(time.Millisecond)
				return nil
			}, nil)
			
			if err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	if len(errors) > 0 {
		t.Errorf("Expected no errors, got %d errors: %v", len(errors), errors[0])
	}

	// Wait for all workers to complete
	time.Sleep(200 * time.Millisecond)

	metrics := wm.GetMetrics()
	if metrics.WorkersCreated != 50 {
		t.Errorf("Expected 50 workers created, got %d", metrics.WorkersCreated)
	}
}

func TestWorkerManager_MemoryLeaks(t *testing.T) {
	// This test checks for potential memory leaks by creating and destroying many workers
	initialGoroutines := runtime.NumGoroutine()

	for i := 0; i < 10; i++ {
		wm := NewWorkerManager(nil)
		
		// Start some workers
		for j := 0; j < 5; j++ {
			_, err := wm.StartWorker(fmt.Sprintf("worker-%d", j), func(ctx context.Context) error {
				time.Sleep(time.Millisecond)
				return nil
			}, nil)
			if err != nil {
				t.Fatalf("Failed to start worker: %v", err)
			}
		}

		// Stop the manager
		if err := wm.Stop(); err != nil {
			t.Fatalf("Failed to stop worker manager: %v", err)
		}
	}

	// Force garbage collection
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	
	// Allow some variance in goroutine count
	if finalGoroutines > initialGoroutines+5 {
		t.Errorf("Potential goroutine leak detected: initial=%d, final=%d", initialGoroutines, finalGoroutines)
	}
}

func TestWorkerManager_ErrorHandling(t *testing.T) {
	wm := NewWorkerManager(nil)
	defer wm.Stop()

	// Test getting info for non-existent worker
	_, err := wm.GetWorkerInfo("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent worker")
	}

	// Test cancelling non-existent worker
	err = wm.CancelWorker("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent worker")
	}
}

func TestWorkerManager_CustomPanicHandler(t *testing.T) {
	var panicHandled bool
	var panicValue interface{}
	
	customHandler := &testPanicHandler{
		handleFunc: func(workerID string, panic interface{}, stackTrace []byte) {
			panicHandled = true
			panicValue = panic
		},
	}

	config := &WorkerConfig{
		MaxWorkers:      10,
		ShutdownTimeout: 5 * time.Second,
		Logger:          zap.NewNop(),
		PanicHandler:    customHandler,
	}

	wm := NewWorkerManager(config)
	defer wm.Stop()

	expectedPanic := "custom panic"
	workerFunc := func(ctx context.Context) error {
		panic(expectedPanic)
	}

	_, err := wm.StartWorker("custom-panic-worker", workerFunc, nil)
	if err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}

	// Wait for panic to be handled
	time.Sleep(100 * time.Millisecond)

	if !panicHandled {
		t.Error("Expected custom panic handler to be called")
	}

	if panicValue != expectedPanic {
		t.Errorf("Expected panic value %v, got %v", expectedPanic, panicValue)
	}
}

// testPanicHandler is a test implementation of PanicHandler
type testPanicHandler struct {
	handleFunc func(workerID string, panicValue interface{}, stackTrace []byte)
}

func (h *testPanicHandler) HandlePanic(workerID string, panicValue interface{}, stackTrace []byte) {
	if h.handleFunc != nil {
		h.handleFunc(workerID, panicValue, stackTrace)
	}
}

func BenchmarkWorkerManager_StartWorker(b *testing.B) {
	wm := NewWorkerManager(nil)
	defer wm.Stop()

	workerFunc := func(ctx context.Context) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := wm.StartWorker(fmt.Sprintf("bench-worker-%d", i), workerFunc, nil)
		if err != nil {
			b.Fatalf("Failed to start worker: %v", err)
		}
	}
}

func BenchmarkWorkerManager_WorkerExecution(b *testing.B) {
	wm := NewWorkerManager(nil)
	defer wm.Stop()

	var counter int64
	workerFunc := func(ctx context.Context) error {
		atomic.AddInt64(&counter, 1)
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := wm.StartWorker(fmt.Sprintf("bench-exec-worker-%d", i), workerFunc, nil)
		if err != nil {
			b.Fatalf("Failed to start worker: %v", err)
		}
	}

	// Wait for all workers to complete
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt64(&counter) != int64(b.N) {
		b.Errorf("Expected %d workers executed, got %d", b.N, atomic.LoadInt64(&counter))
	}
}