package common

import (
	"context"
	"fmt"
	"time"
)

// ExampleTimeoutHandlerUsage demonstrates how to use TimeoutHandler for proper context cancellation
func ExampleTimeoutHandlerUsage() {
	// Example 1: Basic timeout handler with cleanup
	cleanup := func() {
		fmt.Println("Cleaning up resources...")
	}

	handler := NewTimeoutHandler("example-operation", 5*time.Second, cleanup)

	err := handler.Execute(context.Background(), func(ctx context.Context) error {
		// Simulate some work that might take time
		select {
		case <-time.After(2 * time.Second):
			fmt.Println("Operation completed successfully")
			return nil
		case <-ctx.Done():
			fmt.Println("Operation cancelled")
			return ctx.Err()
		}
	})

	if err != nil {
		fmt.Printf("Operation failed: %v\n", err)
	}
}

// ExampleRetryableTimeoutHandlerUsage demonstrates retry logic with timeouts
func ExampleRetryableTimeoutHandlerUsage() {
	cleanup := func() {
		fmt.Println("Cleaning up after failed attempt...")
	}

	retryHandler := NewRetryableTimeoutHandler(
		"retryable-operation",
		2*time.Second,    // timeout per attempt
		3,                // max retries
		500*time.Millisecond, // delay between retries
		cleanup,
	)

	attempt := 0
	err := retryHandler.Execute(context.Background(), func(ctx context.Context) error {
		attempt++
		fmt.Printf("Attempt %d\n", attempt)
		
		// Simulate operation that fails first two times, succeeds on third
		if attempt < 3 {
			return NewTimeoutError("mock-operation", 2*time.Second, 2*time.Second)
		}
		
		fmt.Println("Operation succeeded on retry")
		return nil
	})

	if err != nil {
		fmt.Printf("All retries failed: %v\n", err)
	}
}

// TransportConnectionExample shows how to use timeout patterns for transport connections
func TransportConnectionExample() error {
	// Create a timeout handler for connection establishment
	connectionCleanup := func() {
		fmt.Println("Cleaning up connection resources...")
		// Close sockets, release resources, etc.
	}

	connectHandler := NewTimeoutHandler("transport-connect", 10*time.Second, connectionCleanup)

	// Example connection logic with proper timeout handling
	return connectHandler.Execute(context.Background(), func(ctx context.Context) error {
		// Simulate connection establishment
		fmt.Println("Establishing transport connection...")
		
		// Use context for cancellation within the operation
		select {
		case <-time.After(1 * time.Second): // Simulate connection time
			fmt.Println("Connection established successfully")
			return nil
		case <-ctx.Done():
			fmt.Println("Connection cancelled due to timeout")
			return ctx.Err()
		}
	})
}

// EventProcessingExample shows timeout handling for event processing
func EventProcessingExample() error {
	eventCleanup := func() {
		fmt.Println("Cleaning up event processing resources...")
		// Close channels, stop goroutines, etc.
	}

	processHandler := NewRetryableTimeoutHandler(
		"event-processing",
		5*time.Second,        // timeout per attempt
		2,                    // max retries
		1*time.Second,        // retry delay
		eventCleanup,
	)

	return processHandler.Execute(context.Background(), func(ctx context.Context) error {
		fmt.Println("Processing event...")
		
		// Simulate event processing with cancellation support
		processDone := make(chan error, 1)
		
		go func() {
			// Simulate processing work
			time.Sleep(2 * time.Second)
			processDone <- nil
		}()
		
		select {
		case err := <-processDone:
			if err != nil {
				fmt.Printf("Event processing failed: %v\n", err)
				return err
			}
			fmt.Println("Event processed successfully")
			return nil
		case <-ctx.Done():
			fmt.Println("Event processing cancelled")
			return ctx.Err()
		}
	})
}

// BatchOperationExample shows timeout handling for batch operations
func BatchOperationExample(items []string) error {
	batchCleanup := func() {
		fmt.Println("Cleaning up batch operation...")
		// Rollback partial changes, close resources, etc.
	}

	batchHandler := NewTimeoutHandler("batch-operation", 30*time.Second, batchCleanup)

	return batchHandler.Execute(context.Background(), func(ctx context.Context) error {
		fmt.Printf("Processing batch of %d items...\n", len(items))
		
		for i, item := range items {
			// Check for cancellation before each item
			select {
			case <-ctx.Done():
				fmt.Printf("Batch operation cancelled at item %d\n", i)
				return ctx.Err()
			default:
			}
			
			// Process item with timeout awareness
			itemCtx, itemCancel := context.WithTimeout(ctx, 2*time.Second)
			err := processItemWithTimeout(itemCtx, item)
			itemCancel()
			
			if err != nil {
				fmt.Printf("Failed to process item %d: %v\n", i, err)
				return err
			}
		}
		
		fmt.Println("Batch operation completed successfully")
		return nil
	})
}

func processItemWithTimeout(ctx context.Context, item string) error {
	// Simulate item processing
	select {
	case <-time.After(100 * time.Millisecond):
		fmt.Printf("Processed item: %s\n", item)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ContextHierarchyExample demonstrates proper context hierarchy for nested operations
func ContextHierarchyExample() error {
	// Top-level context with overall timeout
	rootCtx, rootCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer rootCancel()

	// Operation-specific context derived from root
	connectHandler := NewTimeoutHandler("parent-operation", 25*time.Second, func() {
		fmt.Println("Cleaning up parent operation...")
	})

	return connectHandler.Execute(rootCtx, func(parentCtx context.Context) error {
		fmt.Println("Starting parent operation...")
		
		// Child operation with its own timeout (shorter than parent)
		childHandler := NewTimeoutHandler("child-operation", 10*time.Second, func() {
			fmt.Println("Cleaning up child operation...")
		})
		
		err := childHandler.Execute(parentCtx, func(childCtx context.Context) error {
			fmt.Println("Starting child operation...")
			
			// Simulate work that respects context cancellation
			select {
			case <-time.After(5 * time.Second):
				fmt.Println("Child operation completed")
				return nil
			case <-childCtx.Done():
				fmt.Println("Child operation cancelled")
				return childCtx.Err()
			}
		})
		
		if err != nil {
			fmt.Printf("Child operation failed: %v\n", err)
			return err
		}
		
		fmt.Println("Parent operation completed")
		return nil
	})
}

// ConcurrentOperationsExample shows timeout handling for concurrent operations
func ConcurrentOperationsExample() error {
	mainCleanup := func() {
		fmt.Println("Cleaning up main operation...")
	}

	mainHandler := NewTimeoutHandler("concurrent-operations", 15*time.Second, mainCleanup)

	return mainHandler.Execute(context.Background(), func(ctx context.Context) error {
		fmt.Println("Starting concurrent operations...")
		
		// Channel to collect results from concurrent operations
		results := make(chan error, 3)
		
		// Start multiple concurrent operations
		for i := 0; i < 3; i++ {
			go func(operationID int) {
				opCleanup := func() {
					fmt.Printf("Cleaning up operation %d...\n", operationID)
				}
				
				opHandler := NewTimeoutHandler(
					fmt.Sprintf("operation-%d", operationID), 
					5*time.Second, 
					opCleanup,
				)
				
				err := opHandler.Execute(ctx, func(opCtx context.Context) error {
					fmt.Printf("Operation %d starting...\n", operationID)
					
					// Simulate work with different durations
					workDuration := time.Duration(operationID+1) * time.Second
					
					select {
					case <-time.After(workDuration):
						fmt.Printf("Operation %d completed\n", operationID)
						return nil
					case <-opCtx.Done():
						fmt.Printf("Operation %d cancelled\n", operationID)
						return opCtx.Err()
					}
				})
				
				results <- err
			}(i)
		}
		
		// Wait for all operations to complete or timeout
		var errors []error
		for i := 0; i < 3; i++ {
			select {
			case err := <-results:
				if err != nil {
					errors = append(errors, err)
				}
			case <-ctx.Done():
				fmt.Println("Main operation timed out while waiting for concurrent operations")
				return ctx.Err()
			}
		}
		
		if len(errors) > 0 {
			return fmt.Errorf("some operations failed: %v", errors)
		}
		
		fmt.Println("All concurrent operations completed successfully")
		return nil
	})
}