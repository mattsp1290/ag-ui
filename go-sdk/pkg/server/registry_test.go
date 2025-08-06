package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/testhelper"
)

func TestAgentRegistry(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultRegistryConfig()
	config.HealthCheckInterval = 100 * time.Millisecond // Fast for testing
	config.MetricsCollectionInterval = 50 * time.Millisecond
	
	registry := NewAgentRegistry(config)
	require.NotNil(t, registry)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		registry.Stop(ctx)
	})

	t.Run("Registry Start and Stop", func(t *testing.T) {
		ctx := context.Background()
		
		// Start registry
		err := registry.Start(ctx)
		require.NoError(t, err)
		
		// Stop registry
		err = registry.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("Registry Double Start", func(t *testing.T) {
		ctx := context.Background()
		
		// Start registry
		err := registry.Start(ctx)
		require.NoError(t, err)
		defer registry.Stop(ctx)
		
		// Try to start again - should error
		err = registry.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already running")
	})
}

func TestAgentRegistration(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultRegistryConfig()
	config.HealthCheckInterval = time.Hour // Disable for testing
	
	registry := NewAgentRegistry(config)
	require.NotNil(t, registry)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		registry.Stop(ctx)
	})

	ctx := context.Background()
	err := registry.Start(ctx)
	require.NoError(t, err)
	defer registry.Stop(ctx)

	t.Run("Register Agent", func(t *testing.T) {
		agent := &mockClientAgent{
			name:        "test-agent-1",
			description: "Test agent for registration",
			capabilities: client.AgentCapabilities{
				Tools: []string{"calculator", "text-processor"},
			},
		}
		
		metadata := &AgentRegistrationMetadata{
			Tags:        []string{"test", "calculator"},
			Environment: "development",
			Region:      "us-west-1",
			Priority:    10,
			Weight:      100,
		}
		
		// Register agent
		err := registry.RegisterAgent(ctx, agent, metadata)
		require.NoError(t, err)
		
		// Verify agent is registered
		registration, err := registry.GetAgent(ctx, agent.Name())
		require.NoError(t, err)
		assert.Equal(t, agent.Name(), registration.AgentID)
		assert.Equal(t, agent.Description(), registration.Description)
		assert.Equal(t, metadata.Tags, registration.Metadata.Tags)
		assert.Equal(t, metadata.Environment, registration.Metadata.Environment)
		assert.Equal(t, AgentStatusActive, registration.Status)
	})

	t.Run("Register Duplicate Agent", func(t *testing.T) {
		agent := &mockClientAgent{
			name:        "duplicate-agent",
			description: "Test duplicate registration",
		}
		
		// Register agent first time
		err := registry.RegisterAgent(ctx, agent, nil)
		require.NoError(t, err)
		
		// Try to register again - should error
		err = registry.RegisterAgent(ctx, agent, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("Unregister Agent", func(t *testing.T) {
		agent := &mockClientAgent{
			name:        "unregister-agent",
			description: "Test agent unregistration",
		}
		
		// Register agent
		err := registry.RegisterAgent(ctx, agent, nil)
		require.NoError(t, err)
		
		// Unregister agent
		err = registry.UnregisterAgent(ctx, agent.Name())
		require.NoError(t, err)
		
		// Verify agent is unregistered
		_, err = registry.GetAgent(ctx, agent.Name())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Unregister Non-existent Agent", func(t *testing.T) {
		err := registry.UnregisterAgent(ctx, "non-existent-agent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestAgentListing(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultRegistryConfig()
	config.HealthCheckInterval = time.Hour // Disable for testing
	
	registry := NewAgentRegistry(config)
	require.NotNil(t, registry)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		registry.Stop(ctx)
	})

	ctx := context.Background()
	err := registry.Start(ctx)
	require.NoError(t, err)
	defer registry.Stop(ctx)

	// Register multiple agents
	agents := []*mockClientAgent{
		{
			name: "list-agent-1",
			capabilities: client.AgentCapabilities{
				Tools: []string{"calculator"},
			},
		},
		{
			name: "list-agent-2",
			capabilities: client.AgentCapabilities{
				Tools: []string{"text-processor"},
			},
		},
		{
			name: "list-agent-3",
			capabilities: client.AgentCapabilities{
				Tools: []string{"calculator", "text-processor"},
			},
		},
	}
	
	metadatas := []*AgentRegistrationMetadata{
		{Tags: []string{"calculator"}, Environment: "dev"},
		{Tags: []string{"text"}, Environment: "prod"},
		{Tags: []string{"multi"}, Environment: "dev"},
	}
	
	for i, agent := range agents {
		err := registry.RegisterAgent(ctx, agent, metadatas[i])
		require.NoError(t, err)
	}

	t.Run("List All Agents", func(t *testing.T) {
		registrations, err := registry.ListAgents(ctx, nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(registrations), 3)
		
		// Verify all test agents are included
		agentNames := make(map[string]bool)
		for _, reg := range registrations {
			agentNames[reg.AgentID] = true
		}
		
		for _, agent := range agents {
			assert.True(t, agentNames[agent.Name()], "missing agent: %s", agent.Name())
		}
	})

	t.Run("List Agents with Environment Filter", func(t *testing.T) {
		filter := &AgentFilter{
			Environment: "dev",
		}
		
		registrations, err := registry.ListAgents(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, registrations, 2) // list-agent-1 and list-agent-3
		
		for _, reg := range registrations {
			assert.Equal(t, "dev", reg.Metadata.Environment)
		}
	})

	t.Run("List Agents with Capability Filter", func(t *testing.T) {
		filter := &AgentFilter{
			Capabilities: []string{"calculator"},
		}
		
		registrations, err := registry.ListAgents(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, registrations, 2) // list-agent-1 and list-agent-3
		
		for _, reg := range registrations {
			assert.Contains(t, reg.Capabilities.Tools, "calculator")
		}
	})

	t.Run("List Agents with Tag Filter", func(t *testing.T) {
		filter := &AgentFilter{
			Tags: []string{"multi"},
		}
		
		registrations, err := registry.ListAgents(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, registrations, 1) // list-agent-3
		assert.Equal(t, "list-agent-3", registrations[0].AgentID)
	})
}

func TestAgentCapabilities(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultRegistryConfig()
	config.HealthCheckInterval = time.Hour // Disable for testing
	
	registry := NewAgentRegistry(config)
	require.NotNil(t, registry)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		registry.Stop(ctx)
	})

	ctx := context.Background()
	err := registry.Start(ctx)
	require.NoError(t, err)
	defer registry.Stop(ctx)

	// Register agent
	agent := &mockClientAgent{
		name: "capability-agent",
		capabilities: client.AgentCapabilities{
			Tools: []string{"calculator", "text-processor"},
		},
	}
	
	err = registry.RegisterAgent(ctx, agent, nil)
	require.NoError(t, err)

	t.Run("Find Agents by Capability", func(t *testing.T) {
		// Find agents with calculator capability
		agents, err := registry.FindAgentsByCapability(ctx, "calculator")
		require.NoError(t, err)
		assert.Len(t, agents, 1)
		assert.Equal(t, "capability-agent", agents[0].AgentID)
		
		// Find agents with non-existent capability
		agents, err = registry.FindAgentsByCapability(ctx, "non-existent")
		require.NoError(t, err)
		assert.Len(t, agents, 0)
	})

	t.Run("Get Capability Matrix", func(t *testing.T) {
		matrix, err := registry.GetCapabilityMatrix(ctx)
		require.NoError(t, err)
		
		// Verify capabilities are mapped
		calculatorAgents, exists := matrix["calculator"]
		assert.True(t, exists)
		assert.Contains(t, calculatorAgents, "capability-agent")
		
		textProcessorAgents, exists := matrix["text-processor"]
		assert.True(t, exists)
		assert.Contains(t, textProcessorAgents, "capability-agent")
	})

	t.Run("Update Agent Capabilities", func(t *testing.T) {
		// Update capabilities
		newCapabilities := &client.AgentCapabilities{
			Tools: []string{"calculator", "image-processor"},
		}
		
		err := registry.UpdateAgentCapabilities(ctx, "capability-agent", newCapabilities)
		require.NoError(t, err)
		
		// Verify updated capabilities
		registration, err := registry.GetAgent(ctx, "capability-agent")
		require.NoError(t, err)
		assert.Contains(t, registration.Capabilities.Tools, "calculator")
		assert.Contains(t, registration.Capabilities.Tools, "image-processor")
		assert.NotContains(t, registration.Capabilities.Tools, "text-processor")
		
		// Verify capability matrix is updated
		matrix, err := registry.GetCapabilityMatrix(ctx)
		require.NoError(t, err)
		
		imageProcessorAgents, exists := matrix["image-processor"]
		assert.True(t, exists)
		assert.Contains(t, imageProcessorAgents, "capability-agent")
		
		// Old capability should not have this agent anymore
		textProcessorAgents, exists := matrix["text-processor"]
		if exists {
			assert.NotContains(t, textProcessorAgents, "capability-agent")
		}
	})
}

func TestAgentHealth(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultRegistryConfig()
	config.HealthCheckInterval = time.Hour // Disable automatic checks
	
	registry := NewAgentRegistry(config)
	require.NotNil(t, registry)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		registry.Stop(ctx)
	})

	ctx := context.Background()
	err := registry.Start(ctx)
	require.NoError(t, err)
	defer registry.Stop(ctx)

	// Register agent
	agent := &mockClientAgent{
		name:   "health-agent",
		health: client.AgentHealthStatus{Status: "healthy"},
	}
	
	err = registry.RegisterAgent(ctx, agent, nil)
	require.NoError(t, err)

	t.Run("Get Agent Health", func(t *testing.T) {
		health, err := registry.GetAgentHealth(ctx, "health-agent")
		require.NoError(t, err)
		assert.NotNil(t, health)
		// Initial health status depends on health checker implementation
	})

	t.Run("Update Agent Health", func(t *testing.T) {
		newHealth := &AgentHealthStatus{
			Status:       HealthStatusHealthy,
			LastCheck:    time.Now(),
			ResponseTime: 100 * time.Millisecond,
			ErrorCount:   0,
			Uptime:       time.Hour,
		}
		
		err := registry.UpdateAgentHealth(ctx, "health-agent", newHealth)
		require.NoError(t, err)
		
		// Verify updated health
		health, err := registry.GetAgentHealth(ctx, "health-agent")
		require.NoError(t, err)
		assert.Equal(t, HealthStatusHealthy, health.Status)
		assert.Equal(t, newHealth.ResponseTime, health.ResponseTime)
	})

	t.Run("Get Healthy Agents", func(t *testing.T) {
		// Register another agent
		healthyAgent := &mockClientAgent{
			name: "healthy-agent",
			capabilities: client.AgentCapabilities{
				Tools: []string{"calculator"},
			},
		}
		
		err := registry.RegisterAgent(ctx, healthyAgent, nil)
		require.NoError(t, err)
		
		// Set as healthy
		healthyStatus := &AgentHealthStatus{
			Status:    HealthStatusHealthy,
			LastCheck: time.Now(),
		}
		err = registry.UpdateAgentHealth(ctx, "healthy-agent", healthyStatus)
		require.NoError(t, err)
		
		// Get healthy agents with calculator capability
		agents, err := registry.GetHealthyAgents(ctx, []string{"calculator"})
		require.NoError(t, err)
		
		// Should include the healthy agent with calculator capability
		found := false
		for _, agent := range agents {
			if agent.AgentID == "healthy-agent" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestAgentSelection(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultRegistryConfig()
	config.HealthCheckInterval = time.Hour // Disable for testing
	
	registry := NewAgentRegistry(config)
	require.NotNil(t, registry)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		registry.Stop(ctx)
	})

	ctx := context.Background()
	err := registry.Start(ctx)
	require.NoError(t, err)
	defer registry.Stop(ctx)

	// Register multiple agents with different capabilities
	agents := []*mockClientAgent{
		{
			name: "select-agent-1",
			capabilities: client.AgentCapabilities{
				Tools: []string{"calculator"},
			},
		},
		{
			name: "select-agent-2",
			capabilities: client.AgentCapabilities{
				Tools: []string{"calculator", "text-processor"},
			},
		},
		{
			name: "select-agent-3",
			capabilities: client.AgentCapabilities{
				Tools: []string{"image-processor"},
			},
		},
	}
	
	for _, agent := range agents {
		err := registry.RegisterAgent(ctx, agent, nil)
		require.NoError(t, err)
		
		// Set all agents as healthy
		healthStatus := &AgentHealthStatus{
			Status:    HealthStatusHealthy,
			LastCheck: time.Now(),
		}
		err = registry.UpdateAgentHealth(ctx, agent.Name(), healthStatus)
		require.NoError(t, err)
	}

	t.Run("Select Agent with Required Capability", func(t *testing.T) {
		request := &AgentSelectionRequest{
			RequiredCapabilities: []string{"calculator"},
		}
		
		selected, err := registry.SelectAgent(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, selected)
		assert.Contains(t, selected.Capabilities.Tools, "calculator")
		
		// Should be either select-agent-1 or select-agent-2
		assert.Contains(t, []string{"select-agent-1", "select-agent-2"}, selected.AgentID)
	})

	t.Run("Select Agent with Multiple Required Capabilities", func(t *testing.T) {
		request := &AgentSelectionRequest{
			RequiredCapabilities: []string{"calculator", "text-processor"},
		}
		
		selected, err := registry.SelectAgent(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, selected)
		assert.Equal(t, "select-agent-2", selected.AgentID)
	})

	t.Run("Select Agent with Exclusion", func(t *testing.T) {
		request := &AgentSelectionRequest{
			RequiredCapabilities: []string{"calculator"},
			ExcludeAgents:        []string{"select-agent-1"},
		}
		
		selected, err := registry.SelectAgent(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, selected)
		assert.Equal(t, "select-agent-2", selected.AgentID)
	})

	t.Run("Select Agent with No Matches", func(t *testing.T) {
		request := &AgentSelectionRequest{
			RequiredCapabilities: []string{"non-existent-capability"},
		}
		
		_, err := registry.SelectAgent(ctx, request)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no suitable agents found")
	})
}

func TestAgentDiscovery(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultRegistryConfig()
	config.HealthCheckInterval = time.Hour // Disable for testing
	config.EnableChangeNotifications = true
	
	registry := NewAgentRegistry(config)
	require.NotNil(t, registry)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		registry.Stop(ctx)
	})

	ctx := context.Background()
	err := registry.Start(ctx)
	require.NoError(t, err)
	defer registry.Stop(ctx)

	t.Run("Discover Agents", func(t *testing.T) {
		// Register agents
		agent1 := &mockClientAgent{
			name: "discovery-agent-1",
			capabilities: client.AgentCapabilities{
				Tools: []string{"calculator"},
			},
		}
		metadata1 := &AgentRegistrationMetadata{
			Environment: "production",
			Tags:        []string{"math"},
		}
		
		agent2 := &mockClientAgent{
			name: "discovery-agent-2",
			capabilities: client.AgentCapabilities{
				Tools: []string{"text-processor"},
			},
		}
		metadata2 := &AgentRegistrationMetadata{
			Environment: "development",
			Tags:        []string{"text"},
		}
		
		err := registry.RegisterAgent(ctx, agent1, metadata1)
		require.NoError(t, err)
		err = registry.RegisterAgent(ctx, agent2, metadata2)
		require.NoError(t, err)
		
		// Discover all agents
		query := &DiscoveryQuery{
			MaxResults: 10,
		}
		
		result, err := registry.DiscoverAgents(ctx, query)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result.Agents), 2)
		assert.GreaterOrEqual(t, result.TotalCount, 2)
		assert.Greater(t, result.QueryTime, time.Duration(0))
		
		// Discover agents by capability
		query = &DiscoveryQuery{
			Capabilities: []string{"calculator"},
			MaxResults:   10,
		}
		
		result, err = registry.DiscoverAgents(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Agents, 1)
		assert.Equal(t, "discovery-agent-1", result.Agents[0].AgentID)
		
		// Discover agents by environment
		query = &DiscoveryQuery{
			Environment: "production",
			MaxResults:  10,
		}
		
		result, err = registry.DiscoverAgents(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Agents, 1)
		assert.Equal(t, "discovery-agent-1", result.Agents[0].AgentID)
	})

	t.Run("Watch Agent Changes", func(t *testing.T) {
		// Start watching for changes
		changesChan, err := registry.WatchAgentChanges(ctx)
		require.NoError(t, err)
		require.NotNil(t, changesChan)
		
		// Register a new agent
		agent := &mockClientAgent{
			name: "watch-agent",
		}
		
		go func() {
			time.Sleep(100 * time.Millisecond)
			registry.RegisterAgent(ctx, agent, nil)
		}()
		
		// Wait for change event
		select {
		case event := <-changesChan:
			assert.NotNil(t, event)
			assert.Equal(t, ChangeTypeRegistered, event.Type)
			assert.Equal(t, "watch-agent", event.AgentID)
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for change event")
		}
	})
}

func TestRegistryStats(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultRegistryConfig()
	config.HealthCheckInterval = time.Hour // Disable for testing
	
	registry := NewAgentRegistry(config)
	require.NotNil(t, registry)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		registry.Stop(ctx)
	})

	ctx := context.Background()
	err := registry.Start(ctx)
	require.NoError(t, err)
	defer registry.Stop(ctx)

	t.Run("Registry Stats", func(t *testing.T) {
		initialStats, err := registry.GetRegistryStats(ctx)
		require.NoError(t, err)
		assert.NotNil(t, initialStats)
		
		// Register agents
		agent1 := &mockClientAgent{name: "stats-agent-1"}
		agent2 := &mockClientAgent{name: "stats-agent-2"}
		
		err = registry.RegisterAgent(ctx, agent1, nil)
		require.NoError(t, err)
		err = registry.RegisterAgent(ctx, agent2, nil)
		require.NoError(t, err)
		
		// Check updated stats
		updatedStats, err := registry.GetRegistryStats(ctx)
		require.NoError(t, err)
		assert.Greater(t, updatedStats.TotalAgents, initialStats.TotalAgents)
		assert.NotZero(t, updatedStats.StartTime)
		assert.Greater(t, updatedStats.Uptime, time.Duration(0))
	})

	t.Run("Load Balancing Stats", func(t *testing.T) {
		stats, err := registry.GetLoadBalancingStats(ctx)
		require.NoError(t, err)
		assert.NotNil(t, stats)
		assert.NotEmpty(t, stats.Algorithm)
	})
}

func TestRegistryConcurrency(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultRegistryConfig()
	config.HealthCheckInterval = time.Hour // Disable for testing
	
	registry := NewAgentRegistry(config)
	require.NotNil(t, registry)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		registry.Stop(ctx)
	})

	ctx := context.Background()
	err := registry.Start(ctx)
	require.NoError(t, err)
	defer registry.Stop(ctx)

	t.Run("Concurrent Agent Registration", func(t *testing.T) {
		const numAgents = 20
		var wg sync.WaitGroup
		
		for i := 0; i < numAgents; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				agent := &mockClientAgent{
					name: fmt.Sprintf("concurrent-agent-%d", id),
				}
				
				err := registry.RegisterAgent(ctx, agent, nil)
				if err != nil {
					t.Errorf("failed to register agent %d: %v", id, err)
				}
			}(i)
		}
		
		wg.Wait()
		
		// Verify all agents are registered
		agents, err := registry.ListAgents(ctx, nil)
		require.NoError(t, err)
		
		// Count our test agents
		concurrentAgentCount := 0
		for _, agent := range agents {
			if strings.HasPrefix(agent.AgentID, "concurrent-agent-") {
				concurrentAgentCount++
			}
		}
		assert.Equal(t, numAgents, concurrentAgentCount)
	})

	t.Run("Concurrent Operations", func(t *testing.T) {
		// Register an agent for concurrent operations
		agent := &mockClientAgent{
			name: "concurrent-ops-agent",
		}
		err := registry.RegisterAgent(ctx, agent, nil)
		require.NoError(t, err)
		
		var wg sync.WaitGroup
		const numOperations = 10
		
		// Concurrent get operations
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				_, err := registry.GetAgent(ctx, "concurrent-ops-agent")
				if err != nil {
					t.Errorf("failed to get agent: %v", err)
				}
			}()
		}
		
		// Concurrent list operations
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				_, err := registry.ListAgents(ctx, nil)
				if err != nil {
					t.Errorf("failed to list agents: %v", err)
				}
			}()
		}
		
		// Concurrent stats operations
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				_, err := registry.GetRegistryStats(ctx)
				if err != nil {
					t.Errorf("failed to get stats: %v", err)
				}
			}()
		}
		
		wg.Wait()
	})
}

func TestDefaultRegistryConfig(t *testing.T) {
	t.Run("Default Configuration", func(t *testing.T) {
		config := DefaultRegistryConfig()
		
		assert.Greater(t, config.HealthCheckInterval, time.Duration(0))
		assert.Greater(t, config.HealthCheckTimeout, time.Duration(0))
		assert.Greater(t, config.UnhealthyThreshold, int32(0))
		assert.Greater(t, config.HealthyThreshold, int32(0))
		assert.NotEmpty(t, string(config.DefaultLoadBalancingAlgorithm))
		assert.True(t, config.EnableMetricsCollection)
		assert.Greater(t, config.MetricsCollectionInterval, time.Duration(0))
		assert.Greater(t, config.MaxAgents, int32(0))
		assert.True(t, config.EnableChangeNotifications)
	})
}

// Mock client agent implementation for testing
type mockClientAgent struct {
	name         string
	description  string
	capabilities client.AgentCapabilities
	health       client.AgentHealthStatus
}

func (m *mockClientAgent) Name() string {
	return m.name
}

func (m *mockClientAgent) Description() string {
	return m.description
}

func (m *mockClientAgent) Capabilities() client.AgentCapabilities {
	return m.capabilities
}

func (m *mockClientAgent) Health() client.AgentHealthStatus {
	return m.health
}

func (m *mockClientAgent) Start(ctx context.Context) error {
	return nil
}

func (m *mockClientAgent) Stop(ctx context.Context) error {
	return nil
}

// LifecycleManager interface methods
func (m *mockClientAgent) Initialize(ctx context.Context, config *client.AgentConfig) error {
	return nil
}

func (m *mockClientAgent) Cleanup() error {
	return nil
}

// AgentEventProcessor interface methods
func (m *mockClientAgent) ProcessEvent(ctx context.Context, event events.Event) ([]events.Event, error) {
	return nil, nil
}

func (m *mockClientAgent) StreamEvents(ctx context.Context) (<-chan events.Event, error) {
	return nil, nil
}

// StateManager interface methods
func (m *mockClientAgent) GetState(ctx context.Context) (*client.AgentState, error) {
	return nil, nil
}

func (m *mockClientAgent) UpdateState(ctx context.Context, delta *client.StateDelta) error {
	return nil
}

// ToolRunner interface methods
func (m *mockClientAgent) ExecuteTool(ctx context.Context, name string, params interface{}) (interface{}, error) {
	return nil, nil
}

func (m *mockClientAgent) ListTools() []client.ToolDefinition {
	return nil
}