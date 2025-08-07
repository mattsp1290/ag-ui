package client

import (
	"context"
	"testing"
	"time"
)

func TestNewBaseAgent(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test agent for unit testing")

	if agent == nil {
		t.Fatal("NewBaseAgent returned nil")
	}

	if agent.Name() != "test-agent" {
		t.Errorf("Expected agent name 'test-agent', got '%s'", agent.Name())
	}

	if agent.Description() != "Test agent for unit testing" {
		t.Errorf("Expected agent description 'Test agent for unit testing', got '%s'", agent.Description())
	}
}

func TestAgentLifecycle(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test agent")

	// Test initial state
	if agent.getStatus() != AgentStatusUninitialized {
		t.Errorf("Expected initial status %s, got %s", AgentStatusUninitialized, agent.getStatus())
	}

	// Test initialization
	config := DefaultAgentConfig()
	config.Name = "test-agent"

	ctx := context.Background()
	err := agent.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}

	if agent.getStatus() != AgentStatusInitialized {
		t.Errorf("Expected status %s after initialization, got %s", AgentStatusInitialized, agent.getStatus())
	}

	// Test start
	err = agent.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}

	if agent.getStatus() != AgentStatusRunning {
		t.Errorf("Expected status %s after start, got %s", AgentStatusRunning, agent.getStatus())
	}

	// Test health
	health := agent.Health()
	if health.Status != "healthy" {
		t.Errorf("Expected healthy status, got %s", health.Status)
	}

	// Test stop
	stopCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err = agent.Stop(stopCtx)
	if err != nil {
		t.Fatalf("Failed to stop agent: %v", err)
	}

	if agent.getStatus() != AgentStatusStopped {
		t.Errorf("Expected status %s after stop, got %s", AgentStatusStopped, agent.getStatus())
	}

	// Test cleanup
	err = agent.Cleanup()
	if err != nil {
		t.Fatalf("Failed to cleanup agent: %v", err)
	}
}

func TestDefaultAgentConfig(t *testing.T) {
	config := DefaultAgentConfig()

	if config == nil {
		t.Fatal("DefaultAgentConfig returned nil")
	}

	if config.EventProcessing.BufferSize <= 0 {
		t.Error("Default buffer size should be positive")
	}

	if config.EventProcessing.BatchSize <= 0 {
		t.Error("Default batch size should be positive")
	}

	if config.EventProcessing.Timeout <= 0 {
		t.Error("Default timeout should be positive")
	}

	if config.Tools.MaxConcurrent <= 0 {
		t.Error("Default max concurrent should be positive")
	}

	if config.History.MaxMessages <= 0 {
		t.Error("Default max messages should be positive")
	}
}

func TestAgentCapabilities(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test agent")
	config := DefaultAgentConfig()
	config.Name = "test-agent"
	config.Capabilities.Streaming = true
	config.Capabilities.StateSync = true
	config.Capabilities.MessageHistory = true
	config.Capabilities.Tools = []string{"http", "file"}

	ctx := context.Background()
	err := agent.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}

	capabilities := agent.Capabilities()

	if !capabilities.Streaming {
		t.Error("Expected streaming capability to be enabled")
	}

	if !capabilities.StateSync {
		t.Error("Expected state sync capability to be enabled")
	}

	if !capabilities.MessageHistory {
		t.Error("Expected message history capability to be enabled")
	}

	if len(capabilities.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(capabilities.Tools))
	}
}

// Test comprehensive agent functionality including new features
func TestComprehensiveAgentFunctionality(t *testing.T) {
	agent := NewBaseAgent("comprehensive-agent", "Comprehensive test agent")
	ctx := context.Background()

	// Initialize and start
	config := DefaultAgentConfig()
	config.Name = "comprehensive-agent"
	config.Capabilities.Streaming = true

	err := agent.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}

	err = agent.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}

	// Test GetState with new typed return
	state, err := agent.GetState(ctx)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if state.Name != "comprehensive-agent" {
		t.Errorf("Expected state name to be 'comprehensive-agent', got %s", state.Name)
	}

	if state.Status != AgentStatusRunning {
		t.Errorf("Expected state status to be running, got %v", state.Status)
	}

	if state.Data == nil {
		t.Error("Expected state data to be initialized")
	}

	if state.Checksum == "" {
		t.Error("Expected state checksum to be set")
	}

	// Test UpdateState with new typed delta
	delta := &StateDelta{
		Version: state.Version,
		Operations: []StateOperation{
			{
				Op:    StateOpSet,
				Path:  "/custom/test_field",
				Value: "test_value",
			},
		},
		Metadata:  map[string]interface{}{"source": "test"},
		Timestamp: time.Now(),
	}

	err = agent.UpdateState(ctx, delta)
	if err != nil {
		t.Fatalf("Failed to update state: %v", err)
	}

	// Test ListTools
	tools := agent.ListTools()
	if tools == nil {
		t.Error("Expected non-nil tools list")
	}

	// Test StreamEvents with proper race condition handling
	stream, err := agent.StreamEvents(ctx)
	if err != nil {
		t.Fatalf("Failed to get event stream: %v", err)
	}

	if stream == nil {
		t.Error("Expected non-nil stream from StreamEvents")
	}

	// Test Health method memory optimization
	health1 := agent.Health()
	health2 := agent.Health()

	// These should be separate instances (no memory leak)
	if &health1.Details == &health2.Details {
		t.Error("Health method is returning shared map references - memory leak")
	}

	// Clean shutdown
	err = agent.Stop(ctx)
	if err != nil {
		t.Fatalf("Failed to stop agent: %v", err)
	}

	err = agent.Cleanup()
	if err != nil {
		t.Fatalf("Failed to cleanup agent: %v", err)
	}
}

// Test context cancellation handling
func TestContextCancellation(t *testing.T) {
	agent := NewBaseAgent("context-agent", "Context test agent")

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// These should return context.Canceled error
	err := agent.Initialize(ctx, DefaultAgentConfig())
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}

	_, err = agent.StreamEvents(ctx)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error from StreamEvents, got %v", err)
	}

	_, err = agent.GetState(ctx)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error from GetState, got %v", err)
	}
}

// Test typed enums
func TestConflictResolutionStrategy(t *testing.T) {
	strategies := []ConflictResolutionStrategy{
		ConflictResolutionLastWriterWins,
		ConflictResolutionFirstWriterWins,
		ConflictResolutionMerge,
		ConflictResolutionReject,
	}

	for _, strategy := range strategies {
		config := DefaultAgentConfig()
		config.State.ConflictResolution = strategy

		if config.State.ConflictResolution != strategy {
			t.Errorf("Failed to set conflict resolution strategy to %v", strategy)
		}
	}
}
