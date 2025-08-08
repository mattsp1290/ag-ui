package utils

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// MockAgent provides a test implementation of the client.Agent interface
type MockAgent struct {
	name         string
	description  string
	capabilities client.AgentCapabilities
	health       client.AgentHealthStatus
	state        *client.AgentState
	mu           sync.RWMutex
}

func NewMockAgent(name, description string) *MockAgent {
	return &MockAgent{
		name:        name,
		description: description,
		capabilities: client.AgentCapabilities{
			Tools:          []string{"test_tool"},
			Streaming:      true,
			StateSync:      true,
			MessageHistory: true,
		},
		health: client.AgentHealthStatus{
			Status:    "healthy",
			LastCheck: time.Now(),
			Errors:    nil,
		},
		state: &client.AgentState{
			Status:       client.AgentStatusRunning,
			Name:         name,
			Version:      1,
			Data:         make(map[string]interface{}),
			Metadata:     make(map[string]interface{}),
			LastModified: time.Now(),
		},
	}
}

// Implement client.Agent interface methods
func (m *MockAgent) Name() string                                    { return m.name }
func (m *MockAgent) Description() string                             { return m.description }
func (m *MockAgent) Capabilities() client.AgentCapabilities          { return m.capabilities }
func (m *MockAgent) Health() client.AgentHealthStatus                { return m.health }
func (m *MockAgent) Initialize(ctx context.Context, config *client.AgentConfig) error { return nil }
func (m *MockAgent) Start(ctx context.Context) error                 { return nil }
func (m *MockAgent) Stop(ctx context.Context) error                  { return nil }
func (m *MockAgent) Cleanup() error                                  { return nil }
func (m *MockAgent) ProcessEvent(ctx context.Context, event events.Event) ([]events.Event, error) {
	return nil, nil
}
func (m *MockAgent) StreamEvents(ctx context.Context) (<-chan events.Event, error) {
	return make(<-chan events.Event), nil
}
func (m *MockAgent) GetState(ctx context.Context) (*client.AgentState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Return a copy to avoid race conditions
	stateCopy := *m.state
	return &stateCopy, nil
}
func (m *MockAgent) UpdateState(ctx context.Context, delta *client.StateDelta) error { return nil }
func (m *MockAgent) ExecuteTool(ctx context.Context, name string, params interface{}) (interface{}, error) {
	return nil, nil
}
func (m *MockAgent) ListTools() []client.ToolDefinition { return []client.ToolDefinition{} }

// SetHealth allows tests to modify health status
func (m *MockAgent) SetHealth(status client.AgentHealthStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.health = status
}

// SetUnhealthy sets the agent to an unhealthy state for testing
func (m *MockAgent) SetUnhealthy(errors []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.health = client.AgentHealthStatus{
		Status:    "unhealthy",
		LastCheck: time.Now(),
		Errors:    errors,
	}
}

func TestNewAgentUtils(t *testing.T) {
	utils := NewAgentUtils()
	if utils == nil {
		t.Fatal("NewAgentUtils returned nil")
	}
	if utils.healthCheckers == nil {
		t.Error("healthCheckers map not initialized")
	}
	if utils.commonUtils == nil {
		t.Error("commonUtils not initialized")
	}
}

func TestAgentUtils_Clone(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		utils := NewAgentUtils()
		agent := NewMockAgent("test-agent", "Test agent")

		// Note: Clone method has placeholder implementation
		// This test verifies the error handling for now
		_, err := utils.Clone(agent)
		if err == nil {
			t.Error("Expected error due to placeholder implementation")
		}
	})

	t.Run("NilAgent", func(t *testing.T) {
		utils := NewAgentUtils()
		_, err := utils.Clone(nil)
		if err == nil {
			t.Error("Expected error for nil agent")
		}
		if err.Error() != "validation error: agent - agent cannot be nil" {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	t.Run("ConcurrentClone", func(t *testing.T) {
		utils := NewAgentUtils()
		agent := NewMockAgent("test-agent", "Test agent")

		var wg sync.WaitGroup
		errors := make(chan error, 10)

		// Test concurrent clone attempts
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := utils.Clone(agent)
				errors <- err
			}()
		}

		wg.Wait()
		close(errors)

		// All should fail consistently (due to placeholder implementation)
		errorCount := 0
		for err := range errors {
			if err != nil {
				errorCount++
			}
		}
		if errorCount != 10 {
			t.Errorf("Expected 10 errors, got %d", errorCount)
		}
	})
}

func TestAgentUtils_Merge(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		utils := NewAgentUtils()
		base := &client.AgentConfig{
			Name:        "base-agent",
			Description: "Base description",
		}
		override := &client.AgentConfig{
			Name:        "override-agent",
			Description: "Override description",
		}

		result, err := utils.Merge(base, override)
		if err != nil {
			t.Fatalf("Merge failed: %v", err)
		}
		if result == nil {
			t.Fatal("Merge returned nil result")
		}
	})

	t.Run("NilBase", func(t *testing.T) {
		utils := NewAgentUtils()
		override := &client.AgentConfig{Name: "test"}

		_, err := utils.Merge(nil, override)
		if err == nil {
			t.Error("Expected error for nil base")
		}
	})

	t.Run("NilOverride", func(t *testing.T) {
		utils := NewAgentUtils()
		base := &client.AgentConfig{Name: "test"}

		result, err := utils.Merge(base, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != base {
			t.Error("Expected base config when override is nil")
		}
	})

	t.Run("ConcurrentMerge", func(t *testing.T) {
		utils := NewAgentUtils()
		base := &client.AgentConfig{Name: "base"}
		override := &client.AgentConfig{Name: "override"}

		var wg sync.WaitGroup
		results := make(chan *client.AgentConfig, 10)
		errors := make(chan error, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result, err := utils.Merge(base, override)
				results <- result
				errors <- err
			}()
		}

		wg.Wait()
		close(results)
		close(errors)

		// Verify all operations completed successfully
		for err := range errors {
			if err != nil {
				t.Errorf("Concurrent merge failed: %v", err)
			}
		}
	})
}

func TestAgentUtils_Health(t *testing.T) {
	t.Run("HealthyAgent", func(t *testing.T) {
		utils := NewAgentUtils()
		agent := NewMockAgent("healthy-agent", "A healthy agent")

		report, err := utils.Health(agent)
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		if report == nil {
			t.Fatal("Health report is nil")
		}
		if report.AgentName != "healthy-agent" {
			t.Errorf("Expected agent name 'healthy-agent', got %s", report.AgentName)
		}
		if report.Status != "healthy" {
			t.Errorf("Expected status 'healthy', got %s", report.Status)
		}
	})

	t.Run("UnhealthyAgent", func(t *testing.T) {
		utils := NewAgentUtils()
		agent := NewMockAgent("unhealthy-agent", "An unhealthy agent")
		agent.SetUnhealthy([]string{"connection failed", "timeout"})

		report, err := utils.Health(agent)
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		if report.Status != "unhealthy" {
			t.Errorf("Expected status 'unhealthy', got %s", report.Status)
		}
		if len(report.Errors) != 2 {
			t.Errorf("Expected 2 errors, got %d", len(report.Errors))
		}
	})

	t.Run("NilAgent", func(t *testing.T) {
		utils := NewAgentUtils()
		_, err := utils.Health(nil)
		if err == nil {
			t.Error("Expected error for nil agent")
		}
	})

	t.Run("ConcurrentHealthChecks", func(t *testing.T) {
		utils := NewAgentUtils()
		agent := NewMockAgent("concurrent-agent", "Test agent")

		var wg sync.WaitGroup
		reports := make(chan *HealthReport, 10)
		errors := make(chan error, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				report, err := utils.Health(agent)
				reports <- report
				errors <- err
			}()
		}

		wg.Wait()
		close(reports)
		close(errors)

		// Verify all health checks completed successfully
		for err := range errors {
			if err != nil {
				t.Errorf("Concurrent health check failed: %v", err)
			}
		}

		reportCount := 0
		for range reports {
			reportCount++
		}
		if reportCount != 10 {
			t.Errorf("Expected 10 reports, got %d", reportCount)
		}
	})
}

func TestAgentUtils_StartStopHealthMonitoring(t *testing.T) {
	t.Run("StartMonitoring", func(t *testing.T) {
		utils := NewAgentUtils()
		agent := NewMockAgent("monitor-agent", "Test agent")

		err := utils.StartHealthMonitoring(agent, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to start monitoring: %v", err)
		}

		// Verify checker was created
		utils.checkersMu.RLock()
		checker, exists := utils.healthCheckers["monitor-agent"]
		utils.checkersMu.RUnlock()

		if !exists {
			t.Error("Health checker not created")
		}
		if !checker.IsRunning() {
			t.Error("Health checker not running")
		}

		// Clean up
		err = utils.StopHealthMonitoring("monitor-agent")
		if err != nil {
			t.Errorf("Failed to stop monitoring: %v", err)
		}
	})

	t.Run("StopNonExistentMonitoring", func(t *testing.T) {
		utils := NewAgentUtils()
		err := utils.StopHealthMonitoring("non-existent")
		// Should not return error for non-existent monitor
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("ConcurrentMonitoringOperations", func(t *testing.T) {
		utils := NewAgentUtils()
		agents := make([]*MockAgent, 5)
		for i := 0; i < 5; i++ {
			agents[i] = NewMockAgent(fmt.Sprintf("agent-%d", i), "Test agent")
		}

		var wg sync.WaitGroup

		// Start monitoring for all agents concurrently
		for i, agent := range agents {
			wg.Add(1)
			go func(idx int, a client.Agent) {
				defer wg.Done()
				err := utils.StartHealthMonitoring(a, 50*time.Millisecond)
				if err != nil {
					t.Errorf("Failed to start monitoring for agent-%d: %v", idx, err)
				}
			}(i, agent)
		}

		wg.Wait()

		// Verify all monitors are running
		utils.checkersMu.RLock()
		if len(utils.healthCheckers) != 5 {
			t.Errorf("Expected 5 health checkers, got %d", len(utils.healthCheckers))
		}
		utils.checkersMu.RUnlock()

		// Stop all monitoring concurrently
		for i := range agents {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				err := utils.StopHealthMonitoring(fmt.Sprintf("agent-%d", idx))
				if err != nil {
					t.Errorf("Failed to stop monitoring for agent-%d: %v", idx, err)
				}
			}(i)
		}

		wg.Wait()

		// Verify all monitors are stopped
		utils.checkersMu.RLock()
		if len(utils.healthCheckers) != 0 {
			t.Errorf("Expected 0 health checkers, got %d", len(utils.healthCheckers))
		}
		utils.checkersMu.RUnlock()
	})

	t.Run("NilAgent", func(t *testing.T) {
		utils := NewAgentUtils()
		err := utils.StartHealthMonitoring(nil, time.Second)
		if err == nil {
			t.Error("Expected error for nil agent")
		}
	})

	t.Run("DefaultInterval", func(t *testing.T) {
		utils := NewAgentUtils()
		agent := NewMockAgent("default-interval-agent", "Test agent")

		err := utils.StartHealthMonitoring(agent, 0)
		if err != nil {
			t.Fatalf("Failed to start monitoring: %v", err)
		}

		// Verify default interval was used
		utils.checkersMu.RLock()
		checker, exists := utils.healthCheckers["default-interval-agent"]
		utils.checkersMu.RUnlock()

		if !exists {
			t.Error("Health checker not created")
		}
		if checker.interval != 30*time.Second {
			t.Errorf("Expected default interval 30s, got %v", checker.interval)
		}

		// Clean up
		utils.StopHealthMonitoring("default-interval-agent")
	})
}

func TestAgentUtils_GetHealthReport(t *testing.T) {
	t.Run("ExistingMonitor", func(t *testing.T) {
		utils := NewAgentUtils()
		agent := NewMockAgent("report-agent", "Test agent")

		err := utils.StartHealthMonitoring(agent, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to start monitoring: %v", err)
		}

		// Wait for at least one health check
		time.Sleep(200 * time.Millisecond)

		report, err := utils.GetHealthReport("report-agent")
		if err != nil {
			t.Fatalf("Failed to get health report: %v", err)
		}
		if report == nil {
			t.Error("Health report is nil")
		}

		// Clean up
		utils.StopHealthMonitoring("report-agent")
	})

	t.Run("NonExistentMonitor", func(t *testing.T) {
		utils := NewAgentUtils()
		_, err := utils.GetHealthReport("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent monitor")
		}
	})
}

func TestAgentUtils_Backup(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		utils := NewAgentUtils()
		agent := NewMockAgent("backup-agent", "Agent for backup testing")

		backup, err := utils.Backup(agent)
		if err != nil {
			t.Fatalf("Backup failed: %v", err)
		}
		if backup == nil {
			t.Fatal("Backup is nil")
		}
		if backup.Name != "backup-agent" {
			t.Errorf("Expected backup name 'backup-agent', got %s", backup.Name)
		}
		if backup.Version != "1.0" {
			t.Errorf("Expected version '1.0', got %s", backup.Version)
		}
	})

	t.Run("NilAgent", func(t *testing.T) {
		utils := NewAgentUtils()
		_, err := utils.Backup(nil)
		if err == nil {
			t.Error("Expected error for nil agent")
		}
	})

	t.Run("ConcurrentBackups", func(t *testing.T) {
		utils := NewAgentUtils()
		agent := NewMockAgent("concurrent-backup-agent", "Test agent")

		var wg sync.WaitGroup
		backups := make(chan *AgentBackup, 5)
		errors := make(chan error, 5)

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				backup, err := utils.Backup(agent)
				backups <- backup
				errors <- err
			}()
		}

		wg.Wait()
		close(backups)
		close(errors)

		// Verify all backups completed successfully
		for err := range errors {
			if err != nil {
				t.Errorf("Concurrent backup failed: %v", err)
			}
		}

		backupCount := 0
		for range backups {
			backupCount++
		}
		if backupCount != 5 {
			t.Errorf("Expected 5 backups, got %d", backupCount)
		}
	})
}

func TestAgentUtils_Restore(t *testing.T) {
	t.Run("PlaceholderImplementation", func(t *testing.T) {
		utils := NewAgentUtils()
		backup := &AgentBackup{
			Name:       "test-agent",
			Config:     make(map[string]interface{}),
			BackupTime: time.Now(),
			Version:    "1.0",
		}

		_, err := utils.Restore(backup)
		if err == nil {
			t.Error("Expected error due to placeholder implementation")
		}
	})

	t.Run("NilBackup", func(t *testing.T) {
		utils := NewAgentUtils()
		_, err := utils.Restore(nil)
		if err == nil {
			t.Error("Expected error for nil backup")
		}
	})
}

func TestHealthChecker(t *testing.T) {
	t.Run("RunAndStop", func(t *testing.T) {
		utils := NewAgentUtils()
		agent := NewMockAgent("checker-test-agent", "Test agent")

		err := utils.StartHealthMonitoring(agent, 50*time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to start monitoring: %v", err)
		}

		utils.checkersMu.RLock()
		checker, exists := utils.healthCheckers["checker-test-agent"]
		utils.checkersMu.RUnlock()

		if !exists {
			t.Fatal("Health checker not found")
		}

		// Let it run for a bit
		time.Sleep(150 * time.Millisecond)

		if !checker.IsRunning() {
			t.Error("Health checker should be running")
		}

		// Stop the checker
		checker.Stop()

		// Give it time to stop
		time.Sleep(50 * time.Millisecond)

		if checker.IsRunning() {
			t.Error("Health checker should be stopped")
		}
	})

	t.Run("GetLastReport", func(t *testing.T) {
		utils := NewAgentUtils()
		agent := NewMockAgent("report-test-agent", "Test agent")

		err := utils.StartHealthMonitoring(agent, 50*time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to start monitoring: %v", err)
		}

		utils.checkersMu.RLock()
		checker, exists := utils.healthCheckers["report-test-agent"]
		utils.checkersMu.RUnlock()

		if !exists {
			t.Fatal("Health checker not found")
		}

		// Get initial report (should be default)
		report := checker.GetLastReport()
		if report == nil {
			t.Error("Report should not be nil")
		}
		if report.Status != "unknown" {
			t.Errorf("Initial status should be 'unknown', got %s", report.Status)
		}

		// Wait for a health check to run
		time.Sleep(100 * time.Millisecond)

		report = checker.GetLastReport()
		if report.Status == "unknown" {
			t.Error("Status should have been updated after health check")
		}

		// Clean up
		utils.StopHealthMonitoring("report-test-agent")
	})
}

// TestMemoryLeak tests for memory leaks in health monitoring
func TestMemoryLeak_HealthMonitoring(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	utils := NewAgentUtils()

	// Start and stop monitoring many times
	for i := 0; i < 100; i++ {
		agentName := fmt.Sprintf("memory-agent-%d", i)
		testAgent := NewMockAgent(agentName, "Memory test agent")

		err := utils.StartHealthMonitoring(testAgent, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to start monitoring: %v", err)
		}

		// Let it run briefly
		time.Sleep(20 * time.Millisecond)

		err = utils.StopHealthMonitoring(agentName)
		if err != nil {
			t.Errorf("Failed to stop monitoring: %v", err)
		}
	}

	// Verify all monitors are cleaned up
	utils.checkersMu.RLock()
	monitorCount := len(utils.healthCheckers)
	utils.checkersMu.RUnlock()

	if monitorCount != 0 {
		t.Errorf("Expected 0 monitors after cleanup, got %d", monitorCount)
	}
}

// TestRaceCondition tests for race conditions in concurrent operations
func TestRaceCondition_ConcurrentAccess(t *testing.T) {
	utils := NewAgentUtils()
	agent := NewMockAgent("race-test-agent", "Test agent")

	var wg sync.WaitGroup
	errChan := make(chan error, 20)

	// Concurrent start operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := utils.StartHealthMonitoring(agent, 50*time.Millisecond)
			errChan <- err
		}()
	}

	// Concurrent stop operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := utils.StopHealthMonitoring("race-test-agent")
			errChan <- err
		}()
	}

	wg.Wait()
	close(errChan)

	// Should not panic or deadlock
	for err := range errChan {
		// Errors are expected due to concurrent start/stop
		_ = err
	}
}

// Benchmark tests
func BenchmarkAgentUtils_Health(b *testing.B) {
	utils := NewAgentUtils()
	agent := NewMockAgent("bench-agent", "Benchmark agent")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := utils.Health(agent)
		if err != nil {
			b.Fatalf("Health check failed: %v", err)
		}
	}
}

func BenchmarkAgentUtils_Backup(b *testing.B) {
	utils := NewAgentUtils()
	agent := NewMockAgent("bench-backup-agent", "Benchmark agent")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := utils.Backup(agent)
		if err != nil {
			b.Fatalf("Backup failed: %v", err)
		}
	}
}

func BenchmarkHealthChecker_Concurrent(b *testing.B) {
	utils := NewAgentUtils()
	agents := make([]*MockAgent, 10)
	for i := 0; i < 10; i++ {
		agents[i] = NewMockAgent(fmt.Sprintf("bench-concurrent-agent-%d", i), "Benchmark agent")
	}

	// Start monitoring for all agents
	for i, agent := range agents {
		err := utils.StartHealthMonitoring(agent, 100*time.Millisecond)
		if err != nil {
			b.Fatalf("Failed to start monitoring for agent %d: %v", i, err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < 10; i++ {
				_, err := utils.GetHealthReport(fmt.Sprintf("bench-concurrent-agent-%d", i))
				if err != nil {
					continue // Monitor might not have started yet
				}
			}
		}
	})

	// Clean up
	for i := range agents {
		utils.StopHealthMonitoring(fmt.Sprintf("bench-concurrent-agent-%d", i))
	}
}