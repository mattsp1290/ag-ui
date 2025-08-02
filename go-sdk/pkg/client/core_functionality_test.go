package client

import (
	"context"
	"testing"
	"time"
)

// Test core functionality without importing events package to avoid protobuf issues
func TestBasicAgentCreation(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test agent")
	
	if agent == nil {
		t.Fatal("NewBaseAgent returned nil")
	}
	
	if agent.Name() != "test-agent" {
		t.Errorf("Expected name 'test-agent', got '%s'", agent.Name())
	}
	
	if agent.Description() != "Test agent" {
		t.Errorf("Expected description 'Test agent', got '%s'", agent.Description())
	}
}

func TestAgentInitializeAndStart(t *testing.T) {
	agent := NewBaseAgent("test-agent", "Test agent")
	
	// Test initial status
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
	
	// Test state retrieval
	state, err := agent.GetState(ctx)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	
	if state.Name != "test-agent" {
		t.Errorf("Expected state name 'test-agent', got %s", state.Name)
	}
	
	if state.Status != AgentStatusRunning {
		t.Errorf("Expected state status running, got %v", state.Status)
	}
	
	// Test tools list
	tools := agent.ListTools()
	if tools == nil {
		t.Error("Expected non-nil tools list")
	}
}

func TestAgentStateManagement(t *testing.T) {
	agent := NewBaseAgent("state-agent", "State test agent")
	
	config := DefaultAgentConfig()
	config.Name = "state-agent"
	
	ctx := context.Background()
	err := agent.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}
	
	err = agent.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	
	// Get initial state
	state, err := agent.GetState(ctx)
	if err != nil {
		t.Fatalf("Failed to get initial state: %v", err)
	}
	
	// Create a state delta
	delta := &StateDelta{
		Version: state.Version,
		Operations: []StateOperation{
			{
				Op:    StateOpSet,
				Path:  "/test/field",
				Value: "test_value",
			},
		},
		Metadata:  map[string]interface{}{"source": "test"},
		Timestamp: time.Now(),
	}
	
	// Update state
	err = agent.UpdateState(ctx, delta)
	if err != nil {
		t.Fatalf("Failed to update state: %v", err)
	}
	
	// Verify state was updated
	newState, err := agent.GetState(ctx)
	if err != nil {
		t.Fatalf("Failed to get updated state: %v", err)
	}
	
	if newState.Version <= state.Version {
		t.Errorf("Expected state version to increase, got %d -> %d", state.Version, newState.Version)
	}
}

func TestConflictResolutionStrategies(t *testing.T) {
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

func TestAgentContextCancellation(t *testing.T) {
	agent := NewBaseAgent("context-agent", "Context test agent")
	
	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	
	// These should return context.Canceled error
	err := agent.Initialize(ctx, DefaultAgentConfig())
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
	
	_, err = agent.GetState(ctx)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error from GetState, got %v", err)
	}
}

func TestAgentCapabilitiesConfiguration(t *testing.T) {
	agent := NewBaseAgent("cap-agent", "Capabilities test agent")
	config := DefaultAgentConfig()
	config.Name = "cap-agent"
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

func TestAgentCleanShutdown(t *testing.T) {
	agent := NewBaseAgent("shutdown-agent", "Shutdown test agent")
	
	config := DefaultAgentConfig()
	config.Name = "shutdown-agent"
	
	ctx := context.Background()
	err := agent.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Failed to initialize agent: %v", err)
	}
	
	err = agent.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
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