package state

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestHealthCheckIntegrationScenarios tests comprehensive health check integration scenarios
func TestHealthCheckIntegrationScenarios(t *testing.T) {
	t.Run("concurrent_health_check_failures", func(t *testing.T) {
		testConcurrentHealthCheckFailures(t)
	})

	t.Run("health_check_recovery_patterns", func(t *testing.T) {
		testHealthCheckRecoveryPatterns(t)
	})

	t.Run("cascading_failure_scenarios", func(t *testing.T) {
		testCascadingFailureScenarios(t)
	})

	t.Run("health_check_timeout_handling", func(t *testing.T) {
		testHealthCheckTimeoutHandling(t)
	})

	t.Run("dynamic_health_check_registration", func(t *testing.T) {
		testDynamicHealthCheckRegistration(t)
	})

	t.Run("health_check_performance_degradation", func(t *testing.T) {
		testHealthCheckPerformanceDegradation(t)
	})

	t.Run("composite_health_check_scenarios", func(t *testing.T) {
		testCompositeHealthCheckScenarios(t)
	})

	t.Run("health_check_state_transitions", func(t *testing.T) {
		testHealthCheckStateTransitions(t)
	})

	t.Run("health_check_resource_exhaustion", func(t *testing.T) {
		testHealthCheckResourceExhaustion(t)
	})

	t.Run("health_check_monitoring_integration", func(t *testing.T) {
		testHealthCheckMonitoringIntegration(t)
	})
}

func testConcurrentHealthCheckFailures(t *testing.T) {
	// Create multiple health checks with different failure patterns
	healthChecks := []struct {
		name        string
		check       HealthCheck
		expectFail  bool
		description string
	}{
		{
			name:        "always_pass",
			check:       NewCustomHealthCheck("always_pass", func(ctx context.Context) error { return nil }),
			expectFail:  false,
			description: "Should always pass",
		},
		{
			name:        "always_fail",
			check:       NewCustomHealthCheck("always_fail", func(ctx context.Context) error { return errors.New("always fails") }),
			expectFail:  true,
			description: "Should always fail",
		},
		{
			name:        "intermittent_fail",
			check:       &IntermittentHealthCheck{failureRate: 0.5},
			expectFail:  true, // Will likely fail in some attempts
			description: "Fails 50% of the time",
		},
		{
			name:        "slow_check",
			check:       &SlowHealthCheck{delay: 100 * time.Millisecond},
			expectFail:  false,
			description: "Slow but eventually succeeds",
		},
		{
			name:        "timeout_check",
			check:       &TimeoutHealthCheck{delay: 2 * time.Second},
			expectFail:  true,
			description: "Takes too long and should timeout",
		},
		{
			name:        "panic_check",
			check:       &PanicHealthCheck{},
			expectFail:  true,
			description: "Panics during execution",
		},
		{
			name:        "context_sensitive",
			check:       &ContextSensitiveHealthCheck{},
			expectFail:  false,
			description: "Respects context cancellation",
		},
	}

	// Test concurrent execution
	var wg sync.WaitGroup
	results := make(chan TestHealthCheckResult, len(healthChecks)*10)

	// Run multiple iterations concurrently
	for iteration := 0; iteration < 10; iteration++ {
		for _, hc := range healthChecks {
			wg.Add(1)
			go func(iteration int, name string, check HealthCheck, expectFail bool) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()

				start := time.Now()
				var err error

				// Recover from panics in health checks
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("health check panicked: %v", r)
					}
				}()

				err = check.Check(ctx)
				duration := time.Since(start)

				results <- TestHealthCheckResult{
					Iteration:    iteration,
					Name:         name,
					Error:        err,
					Duration:     duration,
					ExpectedFail: expectFail,
				}
			}(iteration, hc.name, hc.check, hc.expectFail)
		}
	}

	// Wait for all checks to complete
	wg.Wait()
	close(results)

	// Analyze results
	resultMap := make(map[string][]TestHealthCheckResult)
	for result := range results {
		resultMap[result.Name] = append(resultMap[result.Name], result)
	}

	// Verify concurrent execution results
	for name, checkResults := range resultMap {
		t.Run(fmt.Sprintf("verify_%s", name), func(t *testing.T) {
			if len(checkResults) != 10 {
				t.Errorf("Expected 10 results for %s, got %d", name, len(checkResults))
			}

			failureCount := 0
			totalDuration := time.Duration(0)

			for _, result := range checkResults {
				if result.Error != nil {
					failureCount++
				}
				totalDuration += result.Duration
			}

			avgDuration := totalDuration / time.Duration(len(checkResults))
			failureRate := float64(failureCount) / float64(len(checkResults))

			t.Logf("%s: Failure rate: %.2f, Average duration: %v", name, failureRate, avgDuration)

			// Specific verifications based on health check type
			switch name {
			case "always_pass":
				if failureCount > 0 {
					t.Errorf("Expected always_pass to never fail, got %d failures", failureCount)
				}
			case "always_fail":
				if failureCount != 10 {
					t.Errorf("Expected always_fail to fail every time, got %d failures", failureCount)
				}
			case "intermittent_fail":
				if failureCount == 0 || failureCount == 10 {
					t.Errorf("Expected intermittent failures, got %d out of 10", failureCount)
				}
			case "timeout_check":
				if failureCount == 0 {
					t.Error("Expected timeout_check to fail due to timeout")
				}
				// Check that it respected timeout
				if avgDuration > 1*time.Second {
					t.Errorf("Expected timeout to be respected, average duration: %v", avgDuration)
				}
			case "panic_check":
				if failureCount == 0 {
					t.Error("Expected panic_check to fail due to panic recovery")
				}
			}
		})
	}
}

func testHealthCheckRecoveryPatterns(t *testing.T) {
	// Test health checks that recover over time
	recoveryChecks := []struct {
		name  string
		check HealthCheck
	}{
		{
			name:  "quick_recovery",
			check: NewRecoveringHealthCheck(3, 100*time.Millisecond), // Fails 3 times, then recovers
		},
		{
			name:  "slow_recovery",
			check: NewRecoveringHealthCheck(5, 50*time.Millisecond), // Fails 5 times, then recovers
		},
		{
			name:  "gradual_recovery",
			check: NewGradualRecoveringHealthCheck(0.8, 0.1), // Starts with 80% failure rate, improves by 10% each check
		},
	}

	for _, rc := range recoveryChecks {
		t.Run(rc.name, func(t *testing.T) {
			ctx := context.Background()

			// Track recovery progress
			results := make([]bool, 20)
			for i := 0; i < 20; i++ {
				err := rc.check.Check(ctx)
				results[i] = err == nil
				time.Sleep(10 * time.Millisecond) // Small delay between checks
			}

			// Analyze recovery pattern
			initialFailures := 0
			finalSuccesses := 0

			// Count failures in first half
			for i := 0; i < 10; i++ {
				if !results[i] {
					initialFailures++
				}
			}

			// Count successes in second half
			for i := 10; i < 20; i++ {
				if results[i] {
					finalSuccesses++
				}
			}

			t.Logf("%s: Initial failures: %d/10, Final successes: %d/10", rc.name, initialFailures, finalSuccesses)

			// Should show improvement over time
			if finalSuccesses <= initialFailures {
				t.Errorf("Expected improvement over time for %s", rc.name)
			}
		})
	}
}

func testCascadingFailureScenarios(t *testing.T) {
	// Create health checks that can cause cascading failures
	// Dependency chain: Database -> Cache -> API -> Frontend
	database := &ResourceHealthCheck{name: "database", healthy: true}
	cache := &DependentHealthCheck{name: "cache", dependency: database}
	api := &DependentHealthCheck{name: "api", dependency: cache}
	frontend := &DependentHealthCheck{name: "frontend", dependency: api}

	checks := []HealthCheck{database, cache, api, frontend}

	// Test normal operation
	t.Run("normal_operation", func(t *testing.T) {
		ctx := context.Background()

		for _, check := range checks {
			err := check.Check(ctx)
			if err != nil {
				t.Errorf("Expected %s to be healthy initially: %v", check.Name(), err)
			}
		}
	})

	// Test cascading failure
	t.Run("cascading_failure", func(t *testing.T) {
		ctx := context.Background()

		// Cause database failure
		database.SetHealthy(false)

		// All dependent services should fail
		expectedFailures := []string{"database", "cache", "api", "frontend"}

		for i, check := range checks {
			err := check.Check(ctx)
			if err == nil {
				t.Errorf("Expected %s to fail due to cascading failure", expectedFailures[i])
			}
		}
	})

	// Test recovery propagation
	t.Run("recovery_propagation", func(t *testing.T) {
		ctx := context.Background()

		// Restore all services to healthy state
		database.SetHealthy(true)
		cache.SetFailureMode(false)
		api.SetFailureMode(false)
		frontend.SetFailureMode(false)

		// All services should recover
		time.Sleep(50 * time.Millisecond) // Small delay for recovery

		for _, check := range checks {
			err := check.Check(ctx)
			if err != nil {
				t.Errorf("Expected %s to recover: %v", check.Name(), err)
			}
		}
	})

	// Test partial failure
	t.Run("partial_failure", func(t *testing.T) {
		ctx := context.Background()

		// Only cache fails - reset database to healthy and set cache to failure mode
		database.SetHealthy(true)
		cache.SetFailureMode(true)
		api.SetFailureMode(false)
		frontend.SetFailureMode(false)

		// Database should be healthy, others should fail
		if err := database.Check(ctx); err != nil {
			t.Errorf("Database should remain healthy: %v", err)
		}

		if err := cache.Check(ctx); err == nil {
			t.Error("Cache should fail")
		}

		if err := api.Check(ctx); err == nil {
			t.Error("API should fail due to cache dependency")
		}

		if err := frontend.Check(ctx); err == nil {
			t.Error("Frontend should fail due to API dependency")
		}
	})
}

func testHealthCheckTimeoutHandling(t *testing.T) {
	timeoutScenarios := []struct {
		name           string
		checkDuration  time.Duration
		contextTimeout time.Duration
		expectTimeout  bool
	}{
		{
			name:           "quick_check",
			checkDuration:  10 * time.Millisecond,
			contextTimeout: 100 * time.Millisecond,
			expectTimeout:  false,
		},
		{
			name:           "timeout_exceeded",
			checkDuration:  200 * time.Millisecond,
			contextTimeout: 50 * time.Millisecond,
			expectTimeout:  true,
		},
		{
			name:           "boundary_case",
			checkDuration:  100 * time.Millisecond,
			contextTimeout: 100 * time.Millisecond,
			expectTimeout:  false, // Should just make it
		},
	}

	for _, scenario := range timeoutScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			check := &TimedHealthCheck{duration: scenario.checkDuration}

			ctx, cancel := context.WithTimeout(context.Background(), scenario.contextTimeout)
			defer cancel()

			start := time.Now()
			err := check.Check(ctx)
			elapsed := time.Since(start)

			if scenario.expectTimeout {
				if err == nil {
					t.Error("Expected timeout error")
				}
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("Expected deadline exceeded error, got: %v", err)
				}

				// Should return within reasonable time of timeout
				if elapsed > scenario.contextTimeout+50*time.Millisecond {
					t.Errorf("Timeout took too long: %v > %v", elapsed, scenario.contextTimeout)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func testDynamicHealthCheckRegistration(t *testing.T) {
	// Simulate dynamic service registration/deregistration
	checks := make(map[string]HealthCheck)
	var mu sync.RWMutex

	// Function to register a health check
	registerCheck := func(name string, check HealthCheck) {
		mu.Lock()
		defer mu.Unlock()
		checks[name] = check
	}

	// Function to unregister a health check
	unregisterCheck := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		delete(checks, name)
	}

	// Function to run all registered checks
	runChecks := func(ctx context.Context) map[string]error {
		mu.RLock()
		defer mu.RUnlock()

		results := make(map[string]error)
		for name, check := range checks {
			results[name] = check.Check(ctx)
		}
		return results
	}

	ctx := context.Background()

	// Initial state - no checks
	results := runChecks(ctx)
	if len(results) != 0 {
		t.Errorf("Expected no checks initially, got %d", len(results))
	}

	// Register some checks
	registerCheck("service1", NewCustomHealthCheck("service1", func(ctx context.Context) error { return nil }))
	registerCheck("service2", NewCustomHealthCheck("service2", func(ctx context.Context) error { return errors.New("service2 down") }))

	// Run checks
	results = runChecks(ctx)
	if len(results) != 2 {
		t.Errorf("Expected 2 checks, got %d", len(results))
	}

	if results["service1"] != nil {
		t.Error("Expected service1 to be healthy")
	}

	if results["service2"] == nil {
		t.Error("Expected service2 to be unhealthy")
	}

	// Add more services concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			serviceName := fmt.Sprintf("dynamic_service_%d", id)

			// Simulate service with intermittent health
			check := &IntermittentHealthCheck{failureRate: 0.3}
			registerCheck(serviceName, check)
		}(i)
	}
	wg.Wait()

	// Run checks again
	results = runChecks(ctx)
	if len(results) != 12 { // 2 + 10
		t.Errorf("Expected 12 checks after dynamic registration, got %d", len(results))
	}

	// Unregister some services
	unregisterCheck("service2")
	for i := 0; i < 5; i++ {
		unregisterCheck(fmt.Sprintf("dynamic_service_%d", i))
	}

	// Final check
	results = runChecks(ctx)
	if len(results) != 6 { // 1 + 5
		t.Errorf("Expected 6 checks after unregistration, got %d", len(results))
	}
}

func testHealthCheckPerformanceDegradation(t *testing.T) {
	// Test health checks under increasing load
	loadLevels := []struct {
		name             string
		concurrentChecks int
		iterations       int
		expectedDuration time.Duration
	}{
		{
			name:             "low_load",
			concurrentChecks: 5,
			iterations:       10,
			expectedDuration: 100 * time.Millisecond,
		},
		{
			name:             "medium_load",
			concurrentChecks: 20,
			iterations:       25,
			expectedDuration: 200 * time.Millisecond,
		},
		{
			name:             "high_load",
			concurrentChecks: 50,
			iterations:       50,
			expectedDuration: 500 * time.Millisecond,
		},
	}

	for _, level := range loadLevels {
		t.Run(level.name, func(t *testing.T) {
			// Create checks with varying complexity
			checks := make([]HealthCheck, level.concurrentChecks)
			for i := 0; i < level.concurrentChecks; i++ {
				switch i % 4 {
				case 0:
					checks[i] = NewCustomHealthCheck(fmt.Sprintf("fast_%d", i), func(ctx context.Context) error { return nil })
				case 1:
					checks[i] = &SlowHealthCheck{delay: 10 * time.Millisecond}
				case 2:
					checks[i] = &CPUIntensiveHealthCheck{iterations: 1000}
				case 3:
					checks[i] = &MemoryIntensiveHealthCheck{allocSize: 1024 * 1024} // 1MB
				}
			}

			var wg sync.WaitGroup
			start := time.Now()
			errorChan := make(chan error, level.concurrentChecks*level.iterations)

			// Run checks concurrently
			for iteration := 0; iteration < level.iterations; iteration++ {
				for i, check := range checks {
					wg.Add(1)
					go func(iteration, checkIndex int, hc HealthCheck) {
						defer wg.Done()

						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()

						err := hc.Check(ctx)
						if err != nil {
							errorChan <- fmt.Errorf("check %d iteration %d: %w", checkIndex, iteration, err)
						}
					}(iteration, i, check)
				}
			}

			wg.Wait()
			totalDuration := time.Since(start)
			close(errorChan)

			// Count errors
			errorCount := 0
			for err := range errorChan {
				errorCount++
				t.Logf("Health check error: %v", err)
			}

			totalChecks := level.concurrentChecks * level.iterations
			errorRate := float64(errorCount) / float64(totalChecks)

			t.Logf("%s: Total duration: %v, Error rate: %.2f%% (%d/%d)",
				level.name, totalDuration, errorRate*100, errorCount, totalChecks)

			// Performance should degrade gracefully
			if totalDuration > level.expectedDuration*2 {
				t.Errorf("Performance degraded too much: %v > %v", totalDuration, level.expectedDuration*2)
			}

			// Error rate should remain reasonable even under load
			if errorRate > 0.1 { // Max 10% error rate
				t.Errorf("Error rate too high under load: %.2f%%", errorRate*100)
			}
		})
	}
}

func testCompositeHealthCheckScenarios(t *testing.T) {
	// Create nested composite health checks
	groupA := NewCompositeHealthCheck("group_a", false,
		NewCustomHealthCheck("service_a1", func(ctx context.Context) error { return nil }),
		NewCustomHealthCheck("service_a2", func(ctx context.Context) error { return nil }),
		&IntermittentHealthCheck{failureRate: 0.2},
	)

	groupB := NewCompositeHealthCheck("group_b", true, // Parallel execution
		NewCustomHealthCheck("service_b1", func(ctx context.Context) error { return nil }),
		&SlowHealthCheck{delay: 50 * time.Millisecond},
		NewCustomHealthCheck("service_b3", func(ctx context.Context) error { return errors.New("always fails") }),
	)

	masterComposite := NewCompositeHealthCheck("master", false, groupA, groupB)

	t.Run("nested_composite_execution", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		start := time.Now()
		err := masterComposite.Check(ctx)
		duration := time.Since(start)

		// Should fail due to service_b3
		if err == nil {
			t.Error("Expected composite check to fail due to nested failure")
		}

		// Should complete in reasonable time
		if duration > 2*time.Second {
			t.Errorf("Composite check took too long: %v", duration)
		}

		t.Logf("Nested composite check completed in %v with error: %v", duration, err)
	})

	t.Run("partial_failure_isolation", func(t *testing.T) {
		// Test individual groups
		ctx := context.Background()

		errA := groupA.Check(ctx)
		errB := groupB.Check(ctx)

		// Group A might pass (depends on intermittent check)
		t.Logf("Group A result: %v", errA)

		// Group B should fail
		if errB == nil {
			t.Error("Expected group B to fail due to always-failing service")
		}
	})

	t.Run("timeout_propagation", func(t *testing.T) {
		// Create a composite with a very slow check
		slowComposite := NewCompositeHealthCheck("slow", false,
			&TimedHealthCheck{duration: 2 * time.Second},
			NewCustomHealthCheck("fast", func(ctx context.Context) error { return nil }),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		err := slowComposite.Check(ctx)
		duration := time.Since(start)

		// Should timeout
		if err == nil {
			t.Error("Expected timeout error")
		}

		// Should respect timeout
		if duration > 200*time.Millisecond {
			t.Errorf("Timeout not respected: %v", duration)
		}
	})
}

func testHealthCheckStateTransitions(t *testing.T) {
	// Test health check that transitions between states
	statefulCheck := &StatefulHealthCheck{
		states:          []string{"initializing", "unhealthy", "degraded", "healthy"},
		currentState:    0,
		transitionDelay: 50 * time.Millisecond,
		lastTransition:  time.Now(), // Initialize lastTransition to current time
		checkCount:      0,          // Start with 0 checks
		checksPerState:  2,          // Stay in each state for 2 checks
	}

	ctx := context.Background()

	// Track state transitions
	var states []string
	var errors []error

	for i := 0; i < 8; i++ {
		err := statefulCheck.Check(ctx)
		states = append(states, statefulCheck.GetCurrentState())
		errors = append(errors, err)

		time.Sleep(60 * time.Millisecond) // Allow transition
	}

	// Verify state progression
	expectedStates := []string{
		"initializing", "initializing", "unhealthy", "unhealthy",
		"degraded", "degraded", "healthy", "healthy",
	}

	if len(states) != len(expectedStates) {
		t.Errorf("Expected %d states, got %d", len(expectedStates), len(states))
	}

	for i, expected := range expectedStates {
		if i < len(states) && states[i] != expected {
			t.Errorf("State %d: expected %s, got %s", i, expected, states[i])
		}
	}

	// Verify error patterns
	healthyCount := 0
	for i, err := range errors {
		if err == nil {
			healthyCount++
		}

		// Last two checks should be healthy
		if i >= 6 && err != nil {
			t.Errorf("Expected healthy state at check %d, got error: %v", i, err)
		}
	}

	if healthyCount < 2 {
		t.Errorf("Expected at least 2 healthy checks, got %d", healthyCount)
	}
}

func testHealthCheckResourceExhaustion(t *testing.T) {
	// Test health checks under resource constraints
	t.Run("memory_exhaustion_simulation", func(t *testing.T) {
		memoryIntensiveChecks := make([]HealthCheck, 20)
		for i := 0; i < 20; i++ {
			memoryIntensiveChecks[i] = &MemoryIntensiveHealthCheck{
				allocSize: 10 * 1024 * 1024, // 10MB per check
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		errors := make(chan error, 20)

		// Run all checks concurrently
		for i, check := range memoryIntensiveChecks {
			wg.Add(1)
			go func(id int, hc HealthCheck) {
				defer wg.Done()
				err := hc.Check(ctx)
				if err != nil {
					errors <- fmt.Errorf("memory check %d: %w", id, err)
				}
			}(i, check)
		}

		wg.Wait()
		close(errors)

		// Should handle memory pressure gracefully
		errorCount := 0
		for err := range errors {
			errorCount++
			t.Logf("Memory intensive check error: %v", err)
		}

		// Some failures might be acceptable under memory pressure
		if errorCount > 5 {
			t.Errorf("Too many failures under memory pressure: %d/20", errorCount)
		}
	})

	t.Run("cpu_exhaustion_simulation", func(t *testing.T) {
		cpuIntensiveChecks := make([]HealthCheck, 10)
		for i := 0; i < 10; i++ {
			cpuIntensiveChecks[i] = &CPUIntensiveHealthCheck{
				iterations: 10000, // CPU intensive work - reduced for test performance
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		start := time.Now()

		var wg sync.WaitGroup
		for _, check := range cpuIntensiveChecks {
			wg.Add(1)
			go func(hc HealthCheck) {
				defer wg.Done()
				hc.Check(ctx)
			}(check)
		}

		wg.Wait()
		duration := time.Since(start)

		// Should complete within reasonable time even under CPU load
		if duration > 8*time.Second {
			t.Errorf("CPU intensive checks took too long: %v", duration)
		}

		t.Logf("CPU intensive checks completed in %v", duration)
	})
}

func testHealthCheckMonitoringIntegration(t *testing.T) {
	// Create monitoring system
	config := DefaultMonitoringConfig()
	config.EnableHealthChecks = true
	config.HealthCheckInterval = 50 * time.Millisecond
	config.HealthCheckTimeout = 200 * time.Millisecond

	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	// Register various health checks
	checks := []HealthCheck{
		NewCustomHealthCheck("stable_service", func(ctx context.Context) error { return nil }),
		&IntermittentHealthCheck{failureRate: 0.3},
		&SlowHealthCheck{delay: 30 * time.Millisecond},
		&RecoveringHealthCheck{failuresLeft: 5, recoveryDelay: 20 * time.Millisecond},
	}

	for _, check := range checks {
		ms.RegisterHealthCheck(check)
	}

	// Let monitoring system run health checks
	time.Sleep(500 * time.Millisecond)

	// Get health status
	status := ms.GetHealthStatus()

	// Verify all checks were registered and executed
	if len(status) != len(checks) {
		t.Errorf("Expected %d health check results, got %d", len(checks), len(status))
	}

	// Verify status structure
	for name, healthStatus := range status {
		t.Logf("Health check %s: healthy=%v", name, healthStatus)
	}

	// Test dynamic registration during monitoring
	dynamicCheck := NewCustomHealthCheck("dynamic", func(ctx context.Context) error {
		return errors.New("dynamic failure")
	})
	ms.RegisterHealthCheck(dynamicCheck)

	// Wait for next health check cycle
	time.Sleep(200 * time.Millisecond)

	// Verify dynamic check was included
	updatedStatus := ms.GetHealthStatus()
	if len(updatedStatus) != len(checks)+1 {
		t.Errorf("Expected %d health checks after dynamic registration, got %d",
			len(checks)+1, len(updatedStatus))
	}

	if dynamicStatus, exists := updatedStatus["dynamic"]; !exists {
		t.Error("Dynamic health check should be registered")
	} else if dynamicStatus {
		t.Error("Dynamic health check should be unhealthy")
	}

	// Test unregistration
	ms.UnregisterHealthCheck("dynamic")
	time.Sleep(100 * time.Millisecond)

	finalStatus := ms.GetHealthStatus()
	if _, exists := finalStatus["dynamic"]; exists {
		t.Error("Dynamic health check should be unregistered")
	}
}

// Custom Health Check Implementations for Testing

// TestHealthCheckResult is used for integration testing
type TestHealthCheckResult struct {
	Iteration    int
	Name         string
	Error        error
	Duration     time.Duration
	ExpectedFail bool
}

type IntermittentHealthCheck struct {
	failureRate float64
	callCount   int32
}

func (hc *IntermittentHealthCheck) Name() string {
	return "intermittent"
}

func (hc *IntermittentHealthCheck) Check(ctx context.Context) error {
	count := atomic.AddInt32(&hc.callCount, 1)
	// Use call count for deterministic "randomness" - fail on even counts for 50% failure rate
	if hc.failureRate >= 0.5 && count%2 == 0 {
		return fmt.Errorf("intermittent failure on call %d", count)
	} else if hc.failureRate < 0.5 && count%4 == 0 {
		return fmt.Errorf("intermittent failure on call %d", count)
	}
	return nil
}

type SlowHealthCheck struct {
	delay time.Duration
}

func (hc *SlowHealthCheck) Name() string {
	return "slow"
}

func (hc *SlowHealthCheck) Check(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(hc.delay):
		return nil
	}
}

type TimeoutHealthCheck struct {
	delay time.Duration
}

func (hc *TimeoutHealthCheck) Name() string {
	return "timeout"
}

func (hc *TimeoutHealthCheck) Check(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(hc.delay):
		return nil
	}
}

type PanicHealthCheck struct{}

func (hc *PanicHealthCheck) Name() string {
	return "panic"
}

func (hc *PanicHealthCheck) Check(ctx context.Context) error {
	panic("health check panic")
}

type ContextSensitiveHealthCheck struct{}

func (hc *ContextSensitiveHealthCheck) Name() string {
	return "context_sensitive"
}

func (hc *ContextSensitiveHealthCheck) Check(ctx context.Context) error {
	// Simulate work that can be cancelled
	for i := 0; i < 100; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			time.Sleep(1 * time.Millisecond)
		}
	}
	return nil
}

type RecoveringHealthCheck struct {
	failuresLeft  int32
	recoveryDelay time.Duration
	mu            sync.Mutex
}

func NewRecoveringHealthCheck(initialFailures int, delay time.Duration) *RecoveringHealthCheck {
	return &RecoveringHealthCheck{
		failuresLeft:  int32(initialFailures),
		recoveryDelay: delay,
	}
}

func (hc *RecoveringHealthCheck) Name() string {
	return "recovering"
}

func (hc *RecoveringHealthCheck) Check(ctx context.Context) error {
	time.Sleep(hc.recoveryDelay)

	remaining := atomic.LoadInt32(&hc.failuresLeft)
	if remaining > 0 {
		atomic.AddInt32(&hc.failuresLeft, -1)
		return fmt.Errorf("still recovering, %d failures left", remaining)
	}
	return nil
}

type GradualRecoveringHealthCheck struct {
	failureRate     float64
	improvementRate float64
	callCount       int32
	mu              sync.Mutex
}

func NewGradualRecoveringHealthCheck(initialFailureRate, improvementRate float64) *GradualRecoveringHealthCheck {
	return &GradualRecoveringHealthCheck{
		failureRate:     initialFailureRate,
		improvementRate: improvementRate,
	}
}

func (hc *GradualRecoveringHealthCheck) Name() string {
	return "gradual_recovery"
}

func (hc *GradualRecoveringHealthCheck) Check(ctx context.Context) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	count := atomic.AddInt32(&hc.callCount, 1)

	// Improve failure rate over time
	if hc.failureRate > 0 {
		hc.failureRate -= hc.improvementRate
		if hc.failureRate < 0 {
			hc.failureRate = 0
		}
	}

	// Use current failure rate
	if rand.Float64() < hc.failureRate {
		return fmt.Errorf("gradual recovery failure on call %d (rate: %.2f)", count, hc.failureRate)
	}
	return nil
}

type ResourceHealthCheck struct {
	name    string
	healthy bool
	mu      sync.RWMutex
}

func (hc *ResourceHealthCheck) Name() string {
	return hc.name
}

func (hc *ResourceHealthCheck) Check(ctx context.Context) error {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if !hc.healthy {
		return fmt.Errorf("%s is unhealthy", hc.name)
	}
	return nil
}

func (hc *ResourceHealthCheck) SetHealthy(healthy bool) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.healthy = healthy
}

type DependentHealthCheck struct {
	name        string
	dependency  HealthCheck
	failureMode bool
	mu          sync.RWMutex
}

func (hc *DependentHealthCheck) Name() string {
	return hc.name
}

func (hc *DependentHealthCheck) Check(ctx context.Context) error {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if hc.failureMode {
		return fmt.Errorf("%s is in failure mode", hc.name)
	}

	if err := hc.dependency.Check(ctx); err != nil {
		return fmt.Errorf("%s failed due to dependency: %w", hc.name, err)
	}
	return nil
}

func (hc *DependentHealthCheck) SetFailureMode(failure bool) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.failureMode = failure
}

type TimedHealthCheck struct {
	duration time.Duration
}

func (hc *TimedHealthCheck) Name() string {
	return "timed"
}

func (hc *TimedHealthCheck) Check(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(hc.duration):
		return nil
	}
}

type CPUIntensiveHealthCheck struct {
	iterations int
}

func (hc *CPUIntensiveHealthCheck) Name() string {
	return "cpu_intensive"
}

func (hc *CPUIntensiveHealthCheck) Check(ctx context.Context) error {
	// CPU intensive work with optimized context checking
	sum := 0
	for i := 0; i < hc.iterations; i++ {
		// Check context cancellation every 1000 iterations to reduce overhead
		if i%1000 == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		sum += i * i
	}
	return nil
}

type MemoryIntensiveHealthCheck struct {
	allocSize int
}

func (hc *MemoryIntensiveHealthCheck) Name() string {
	return "memory_intensive"
}

func (hc *MemoryIntensiveHealthCheck) Check(ctx context.Context) error {
	// Allocate and immediately release memory
	data := make([]byte, hc.allocSize)

	// Do something with the data to prevent optimization
	for i := 0; i < len(data); i += 1024 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			data[i] = byte(i % 256)
		}
	}

	// Force GC
	data = nil
	return nil
}

type StatefulHealthCheck struct {
	states          []string
	currentState    int
	transitionDelay time.Duration
	lastTransition  time.Time
	checkCount      int // Track number of checks in current state
	checksPerState  int // Number of checks to stay in each state
	mu              sync.RWMutex
}

func (hc *StatefulHealthCheck) Name() string {
	return "stateful"
}

func (hc *StatefulHealthCheck) Check(ctx context.Context) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	// Increment check count for current state
	hc.checkCount++

	// Check if it's time to transition based on check count and time delay
	if hc.checkCount > hc.checksPerState && time.Since(hc.lastTransition) >= hc.transitionDelay {
		if hc.currentState < len(hc.states)-1 {
			hc.currentState++
			hc.lastTransition = time.Now()
			hc.checkCount = 1 // Reset check count for new state (count the current check)
		}
	}

	// Return error for non-healthy states
	switch hc.states[hc.currentState] {
	case "healthy":
		return nil
	default:
		return fmt.Errorf("service is in %s state", hc.states[hc.currentState])
	}
}

func (hc *StatefulHealthCheck) GetCurrentState() string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.states[hc.currentState]
}

// Benchmark tests

func BenchmarkHealthCheckIntegration(b *testing.B) {
	b.Run("concurrent_health_checks", func(b *testing.B) {
		checks := []HealthCheck{
			NewCustomHealthCheck("fast1", func(ctx context.Context) error { return nil }),
			NewCustomHealthCheck("fast2", func(ctx context.Context) error { return nil }),
			&SlowHealthCheck{delay: 1 * time.Millisecond},
			&IntermittentHealthCheck{failureRate: 0.1},
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)

				var wg sync.WaitGroup
				for _, check := range checks {
					wg.Add(1)
					go func(hc HealthCheck) {
						defer wg.Done()
						hc.Check(ctx)
					}(check)
				}
				wg.Wait()
				cancel()
			}
		})
	})

	b.Run("composite_health_check", func(b *testing.B) {
		composite := NewCompositeHealthCheck("benchmark", true,
			NewCustomHealthCheck("check1", func(ctx context.Context) error { return nil }),
			NewCustomHealthCheck("check2", func(ctx context.Context) error { return nil }),
			NewCustomHealthCheck("check3", func(ctx context.Context) error { return nil }),
		)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			composite.Check(ctx)
			cancel()
		}
	})
}
