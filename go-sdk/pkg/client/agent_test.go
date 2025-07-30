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