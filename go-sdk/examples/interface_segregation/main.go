package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
)

// This file demonstrates how the refactored Agent interface now follows
// the Interface Segregation Principle by allowing code to depend only
// on the specific functionality it needs.

// LifecycleService only needs to manage agent lifecycle
type LifecycleService struct{}

func (ls *LifecycleService) StartAgent(ctx context.Context, mgr client.LifecycleManager, config *client.AgentConfig) error {
	if err := mgr.Initialize(ctx, config); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}
	
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("start failed: %w", err)
	}
	
	fmt.Println("Agent started successfully")
	return nil
}

// EventService only needs to process events
type EventService struct{}

func (es *EventService) ProcessEvents(ctx context.Context, processor client.AgentEventProcessor, events []interface{}) error {
	fmt.Printf("Processing %d events...\n", len(events))
	// Event processing logic would go here
	fmt.Println("Events processed successfully")
	return nil
}

// StateService only needs to manage state
type StateService struct{}

func (ss *StateService) GetAgentState(ctx context.Context, mgr client.StateManager) (*client.AgentState, error) {
	state, err := mgr.GetState(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}
	
	fmt.Printf("Agent state retrieved: %s\n", state.Name)
	return state, nil
}

// ToolService only needs to execute tools
type ToolService struct{}

func (ts *ToolService) ListAvailableTools(runner client.ToolRunner) []client.ToolDefinition {
	tools := runner.ListTools()
	fmt.Printf("Found %d available tools\n", len(tools))
	return tools
}

// MetadataService only needs to access metadata
type MetadataService struct{}

func (ms *MetadataService) GetAgentInfo(metadata client.AgentMetadata) {
	fmt.Printf("Agent: %s - %s\n", metadata.Name(), metadata.Description())
	fmt.Printf("Capabilities: %+v\n", metadata.Capabilities())
	fmt.Printf("Health: %s\n", metadata.Health().Status)
}

func main() {
	fmt.Println("Interface Segregation Principle Demonstration")
	fmt.Println("==============================================")
	
	// Create a concrete agent that implements all interfaces
	baseAgent := client.NewBaseAgent("demo-agent", "Demonstration agent for interface segregation")
	
	// Configure the agent
	config := client.DefaultAgentConfig()
	config.Name = "demo-agent"
	config.Description = "Demonstration agent"
	
	ctx := context.Background()
	
	// Each service only depends on the interface it needs
	lifecycleService := &LifecycleService{}
	eventService := &EventService{}
	stateService := &StateService{}
	toolService := &ToolService{}
	metadataService := &MetadataService{}
	
	// Initialize and start agent using only LifecycleManager interface
	if err := lifecycleService.StartAgent(ctx, baseAgent, config); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}
	
	// Process events using only AgentEventProcessor interface
	if err := eventService.ProcessEvents(ctx, baseAgent, []interface{}{"event1", "event2"}); err != nil {
		log.Fatalf("Failed to process events: %v", err)
	}
	
	// Get state using only StateManager interface
	if _, err := stateService.GetAgentState(ctx, baseAgent); err != nil {
		log.Fatalf("Failed to get state: %v", err)
	}
	
	// List tools using only ToolRunner interface
	toolService.ListAvailableTools(baseAgent)
	
	// Get metadata using only AgentMetadata interface
	metadataService.GetAgentInfo(baseAgent)
	
	// Clean up
	if err := baseAgent.Stop(ctx); err != nil {
		log.Printf("Warning: Failed to stop agent: %v", err)
	}
	
	if err := baseAgent.Cleanup(); err != nil {
		log.Printf("Warning: Failed to cleanup agent: %v", err)
	}
	
	fmt.Println("\nDemonstration completed successfully!")
	fmt.Println("Each service only depends on the specific interface it needs,")
	fmt.Println("following the Interface Segregation Principle.")
}