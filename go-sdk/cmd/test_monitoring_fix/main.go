package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/ag-ui/go-sdk/pkg/state"
	"go.uber.org/zap/zapcore"
)

func main() {
	fmt.Println("Testing monitoring system resource leak fix...")

	// Record initial goroutine count
	initialGoroutines := runtime.NumGoroutine()
	fmt.Printf("Initial goroutines: %d\n", initialGoroutines)

	// Create and run monitoring system
	config := state.DefaultMonitoringConfig()
	config.ResourceSampleInterval = 100 * time.Millisecond
	config.HealthCheckInterval = 100 * time.Millisecond
	config.MetricsInterval = 100 * time.Millisecond
	config.LogLevel = zapcore.InfoLevel

	ms, err := state.NewMonitoringSystem(config)
	if err != nil {
		log.Fatalf("Failed to create monitoring system: %v", err)
	}

	// Register a health check
	ms.RegisterHealthCheck(state.NewCustomHealthCheck("test", func(ctx context.Context) error {
		return nil
	}))

	// Let it run
	fmt.Println("Running monitoring system for 2 seconds...")
	time.Sleep(2 * time.Second)

	// Check goroutines while running
	runningGoroutines := runtime.NumGoroutine()
	fmt.Printf("Goroutines while running: %d\n", runningGoroutines)

	// Shutdown
	fmt.Println("Shutting down monitoring system...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := ms.Shutdown(ctx); err != nil {
		log.Fatalf("Shutdown failed: %v", err)
	}

	// Wait a bit for goroutines to fully terminate
	time.Sleep(1 * time.Second)

	// Check final goroutine count
	finalGoroutines := runtime.NumGoroutine()
	fmt.Printf("Final goroutines: %d\n", finalGoroutines)

	leaked := finalGoroutines - initialGoroutines
	if leaked > 0 {
		fmt.Printf("WARNING: Potential goroutine leak detected: %d goroutines leaked\n", leaked)
	} else {
		fmt.Println("SUCCESS: No goroutine leaks detected!")
	}
}