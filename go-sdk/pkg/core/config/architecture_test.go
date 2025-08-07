package config

import (
	"context"
	"testing"
	"time"
)

// TestUnifiedConfigurationSystem tests the unified configuration system
func TestUnifiedConfigurationSystem(t *testing.T) {
	// Test configuration provider
	t.Run("ConfigProvider", func(t *testing.T) {
		config := DefaultValidatorConfig()
		provider := NewUnifiedValidatorConfigProvider(config)

		// Test basic configuration access
		nodeID, err := provider.GetString("distributed.NodeID")
		if err != nil {
			t.Errorf("Expected no error getting node_id, got %v", err)
		}

		if nodeID == "" {
			t.Error("Expected non-empty node_id")
		}

		// Test setting configuration
		err = provider.Set("distributed.NodeID", "test-node-123")
		if err != nil {
			t.Errorf("Expected no error setting node_id, got %v", err)
		}

		// Verify the change
		newNodeID, err := provider.GetString("distributed.NodeID")
		if err != nil {
			t.Errorf("Expected no error getting updated node_id, got %v", err)
		}

		if newNodeID != "test-node-123" {
			t.Errorf("Expected node_id to be 'test-node-123', got %s", newNodeID)
		}

		// Test validation
		err = provider.Validate()
		if err != nil {
			t.Errorf("Expected no validation error, got %v", err)
		}
	})

	// Test configuration manager
	t.Run("ConfigurationManager", func(t *testing.T) {
		manager := NewConfigurationManager()

		// Test initialization
		err := manager.Initialize()
		if err != nil {
			t.Errorf("Expected no error initializing manager, got %v", err)
		}

		// Test provider access
		provider := manager.GetProvider()
		if provider == nil {
			t.Error("Expected non-nil provider")
		}

		// Test registry access
		registry := manager.GetRegistry()
		if registry == nil {
			t.Error("Expected non-nil registry")
		}

		// Test factory access
		factory := manager.GetFactory()
		if factory == nil {
			t.Error("Expected non-nil factory")
		}

		// Test container access
		container := manager.GetContainer()
		if container == nil {
			t.Error("Expected non-nil container")
		}

		// Test validation
		err = manager.Validate()
		if err != nil {
			t.Errorf("Expected no validation error, got %v", err)
		}

		// Test health check
		health := manager.GetHealth()
		if health == nil {
			t.Error("Expected non-nil health status")
		}

		if !health["healthy"].(bool) {
			t.Error("Expected system to be healthy")
		}
	})

	// Test service container
	t.Run("ServiceContainer", func(t *testing.T) {
		container := NewServiceRegistry()

		// Test service registration
		mockService := &MockService{name: "test-service"}
		err := container.Register("test-service", mockService)
		if err != nil {
			t.Errorf("Expected no error registering service, got %v", err)
		}

		// Test service retrieval
		service, err := container.Get("test-service")
		if err != nil {
			t.Errorf("Expected no error getting service, got %v", err)
		}

		if service != mockService {
			t.Error("Expected to get the same service instance")
		}

		// Test service existence check
		if !container.Has("test-service") {
			t.Error("Expected service to exist")
		}

		// Test service validation
		err = container.Validate()
		if err != nil {
			t.Errorf("Expected no validation error, got %v", err)
		}
	})

	// Test configuration factory
	t.Run("ConfigurationFactory", func(t *testing.T) {
		factory := NewConfigurationFactory()

		// Test creating configuration from options
		provider, err := factory.CreateValidatorConfig(
			NewEnvironmentOption("test"),
			NewValidationLevelOption(ValidationLevelTesting),
			NewNodeIDOption("test-node"),
		)
		if err != nil {
			t.Errorf("Expected no error creating config, got %v", err)
		}

		// Verify environment
		env := provider.GetEnvironment()
		if env != "test" {
			t.Errorf("Expected environment to be 'test', got %s", env)
		}

		// Verify node ID
		distributedConfig, err := provider.GetDistributedConfig()
		if err != nil {
			t.Errorf("Expected no error getting distributed config, got %v", err)
		}

		if distributedConfig.NodeID != "test-node" {
			t.Errorf("Expected node ID to be 'test-node', got %s", distributedConfig.NodeID)
		}

		// Test validation
		err = provider.Validate()
		if err != nil {
			t.Errorf("Expected no validation error, got %v", err)
		}
	})

	// Test configuration mapper
	t.Run("DistributedConfigMapper", func(t *testing.T) {
		config := DefaultValidatorConfig()
		config.Distributed.NodeID = "test-node"
		config.Distributed.Enabled = true

		provider := NewUnifiedValidatorConfigProvider(config)
		mapper := NewDistributedConfigMapper(provider)

		// Test mapping to distributed config
		distributedConfig, err := mapper.MapToDistributedConfig()
		if err != nil {
			t.Errorf("Expected no error mapping config, got %v", err)
		}

		if distributedConfig.NodeID != "test-node" {
			t.Errorf("Expected node ID to be 'test-node', got %s", distributedConfig.NodeID)
		}

		// Test adapter validation
		err = distributedConfig.Validate()
		if err != nil {
			t.Errorf("Expected no validation error, got %v", err)
		}

		// Test consensus config
		consensusConfig := distributedConfig.GetConsensusConfig()
		if consensusConfig == nil {
			t.Error("Expected non-nil consensus config")
		}

		err = consensusConfig.Validate()
		if err != nil {
			t.Errorf("Expected no consensus config validation error, got %v", err)
		}
	})
}

// TestArchitectureDecoupling tests that the architecture improvements reduce coupling
func TestArchitectureDecoupling(t *testing.T) {
	// Test that configuration is decoupled from implementation
	t.Run("ConfigurationDecoupling", func(t *testing.T) {
		// Create configuration without importing distributed package
		config := DefaultValidatorConfig()
		provider := NewUnifiedValidatorConfigProvider(config)

		// Test that configuration can be accessed without knowledge of implementation details
		enabled := provider.IsEnabled("distributed")
		if enabled {
			t.Log("Distributed validation is enabled")
		}

		// Test that configuration can be modified without affecting implementation
		err := provider.Set("distributed.Enabled", true)
		if err != nil {
			t.Errorf("Expected no error setting config, got %v", err)
		}

		// Verify the change
		newEnabled, err := provider.GetBool("distributed.Enabled")
		if err != nil {
			t.Errorf("Expected no error getting enabled status, got %v", err)
		}
		if !newEnabled {
			t.Error("Expected distributed validation to be enabled")
		}
	})

	// Test that services can be registered and discovered without tight coupling
	t.Run("ServiceDecoupling", func(t *testing.T) {
		container := NewServiceRegistry()

		// Register services without importing their implementations
		mockValidator := &MockValidator{name: "test-validator"}
		err := container.Register("validator", mockValidator)
		if err != nil {
			t.Errorf("Expected no error registering validator, got %v", err)
		}

		mockConsensus := &MockConsensus{algorithm: "test-consensus"}
		err = container.Register("consensus", mockConsensus)
		if err != nil {
			t.Errorf("Expected no error registering consensus, got %v", err)
		}

		// Retrieve services by interface
		validator, err := container.Get("validator")
		if err != nil {
			t.Errorf("Expected no error getting validator, got %v", err)
		}

		if validator == nil {
			t.Error("Expected non-nil validator")
		}

		consensus, err := container.Get("consensus")
		if err != nil {
			t.Errorf("Expected no error getting consensus, got %v", err)
		}

		if consensus == nil {
			t.Error("Expected non-nil consensus")
		}
	})

	// Test that components can be configured independently
	t.Run("IndependentConfiguration", func(t *testing.T) {
		// Create separate configuration providers for different components
		coreConfig := DefaultCoreValidationConfig()
		distributedConfig := DefaultDistributedValidationConfig()
		securityConfig := DefaultSecurityValidationConfig()

		// Test that each component can be configured independently
		coreConfig.ValidationTimeout = 10 * time.Second
		distributedConfig.NodeID = "independent-node"
		securityConfig.EnableRateLimiting = true

		// Verify configurations don't interfere with each other
		if coreConfig.ValidationTimeout != 10*time.Second {
			t.Error("Core config should maintain independent settings")
		}

		if distributedConfig.NodeID != "independent-node" {
			t.Error("Distributed config should maintain independent settings")
		}

		if !securityConfig.EnableRateLimiting {
			t.Error("Security config should maintain independent settings")
		}
	})

	// Test that the factory pattern reduces coupling
	t.Run("FactoryDecoupling", func(t *testing.T) {
		factory := NewConfigurationFactory()

		// Test that factory can create configurations without tight coupling
		config1, err := factory.CreateValidatorConfig(
			NewEnvironmentOption("development"),
		)
		if err != nil {
			t.Errorf("Expected no error creating config1, got %v", err)
		}

		config2, err := factory.CreateValidatorConfig(
			NewEnvironmentOption("production"),
		)
		if err != nil {
			t.Errorf("Expected no error creating config2, got %v", err)
		}

		// Verify that configurations are independent
		if config1.GetEnvironment() == config2.GetEnvironment() {
			t.Error("Configurations should be independent")
		}

		// Test that factory can create different types of configurations
		_, err = factory.CreateConfig("test-module")
		if err != nil {
			t.Errorf("Expected no error creating module config, got %v", err)
		}
	})
}

// TestArchitectureMaintainability tests that the architecture improvements improve maintainability
func TestArchitectureMaintainability(t *testing.T) {
	// Test that configuration can be easily extended
	t.Run("ConfigurationExtensibility", func(t *testing.T) {
		config := DefaultValidatorConfig()
		provider := NewUnifiedValidatorConfigProvider(config)

		// Test adding new configuration keys to existing structure
		err := provider.Set("global.Environment", "testing")
		if err != nil {
			t.Errorf("Expected no error adding new config, got %v", err)
		}

		// Verify the new configuration can be retrieved
		env, err := provider.GetString("global.Environment")
		if err != nil {
			t.Errorf("Expected no error getting new config, got %v", err)
		}

		if env != "testing" {
			t.Errorf("Expected environment to be 'testing', got %s", env)
		}

		// Test that existing configuration is not affected
		origEnv := provider.GetEnvironment()
		if origEnv == "" {
			t.Error("Expected existing configuration to remain intact")
		}
	})

	// Test that services can be easily swapped
	t.Run("ServiceSwappability", func(t *testing.T) {
		container := NewServiceRegistry()

		// Register initial service
		service1 := &MockService{name: "service-v1"}
		err := container.Register("test-service", service1)
		if err != nil {
			t.Errorf("Expected no error registering service1, got %v", err)
		}

		// Verify service is registered
		retrieved1, err := container.Get("test-service")
		if err != nil {
			t.Errorf("Expected no error getting service1, got %v", err)
		}

		if retrieved1 != service1 {
			t.Error("Expected to get service1")
		}

		// Swap with new service
		service2 := &MockService{name: "service-v2"}
		err = container.Register("test-service", service2)
		if err != nil {
			t.Errorf("Expected no error registering service2, got %v", err)
		}

		// Verify service is swapped
		retrieved2, err := container.Get("test-service")
		if err != nil {
			t.Errorf("Expected no error getting service2, got %v", err)
		}

		if retrieved2 != service2 {
			t.Error("Expected to get service2")
		}
	})

	// Test that configuration can be validated at different levels
	t.Run("HierarchicalValidation", func(t *testing.T) {
		manager := NewConfigurationManager()
		err := manager.Initialize()
		if err != nil {
			t.Errorf("Expected no error initializing manager, got %v", err)
		}

		// Test provider-level validation
		provider := manager.GetProvider()
		err = provider.Validate()
		if err != nil {
			t.Errorf("Expected no provider validation error, got %v", err)
		}

		// Test registry-level validation
		registry := manager.GetRegistry()
		err = registry.Validate()
		if err != nil {
			t.Errorf("Expected no registry validation error, got %v", err)
		}

		// Test container-level validation
		container := manager.GetContainer()
		err = container.Validate()
		if err != nil {
			t.Errorf("Expected no container validation error, got %v", err)
		}

		// Test system-level validation
		err = manager.Validate()
		if err != nil {
			t.Errorf("Expected no system validation error, got %v", err)
		}
	})

	// Test that the system supports lifecycle management
	t.Run("LifecycleManagement", func(t *testing.T) {
		manager := NewConfigurationManager()

		// Test initialization
		err := manager.Initialize()
		if err != nil {
			t.Errorf("Expected no error initializing, got %v", err)
		}

		// Test starting
		err = manager.Start()
		if err != nil {
			t.Errorf("Expected no error starting, got %v", err)
		}

		// Test health check
		health := manager.GetHealth()
		if !health["healthy"].(bool) {
			t.Error("Expected system to be healthy after start")
		}

		// Test stopping
		err = manager.Stop()
		if err != nil {
			t.Errorf("Expected no error stopping, got %v", err)
		}

		// Test that system can be restarted
		err = manager.Start()
		if err != nil {
			t.Errorf("Expected no error restarting, got %v", err)
		}

		err = manager.Stop()
		if err != nil {
			t.Errorf("Expected no error stopping after restart, got %v", err)
		}
	})
}

// Mock implementations for testing

type MockService struct {
	name string
}

func (m *MockService) GetName() string {
	return m.name
}

func (m *MockService) Validate() error {
	return nil
}

type MockValidator struct {
	name string
}

func (m *MockValidator) GetName() string {
	return m.name
}

func (m *MockValidator) ValidateEvent(ctx context.Context, event interface{}) (interface{}, error) {
	return nil, nil
}

func (m *MockValidator) ValidateSequence(ctx context.Context, events []interface{}) (interface{}, error) {
	return nil, nil
}

func (m *MockValidator) GetVersion() string {
	return "1.0.0"
}

func (m *MockValidator) IsHealthy() bool {
	return true
}

func (m *MockValidator) GetMetrics() interface{} {
	return nil
}

type MockConsensus struct {
	algorithm string
}

func (m *MockConsensus) GetAlgorithm() string {
	return m.algorithm
}

func (m *MockConsensus) Start(ctx context.Context) error {
	return nil
}

func (m *MockConsensus) Stop() error {
	return nil
}

func (m *MockConsensus) IsHealthy() bool {
	return true
}

// BenchmarkConfigurationSystem benchmarks the configuration system performance
func BenchmarkConfigurationSystem(b *testing.B) {
	config := DefaultValidatorConfig()
	provider := NewUnifiedValidatorConfigProvider(config)

	b.Run("ConfigGet", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			provider.GetString("distributed.node_id")
		}
	})

	b.Run("ConfigSet", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			provider.Set("test.key", "test.value")
		}
	})

	b.Run("ConfigValidation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			provider.Validate()
		}
	})
}

// BenchmarkServiceContainer benchmarks the service container performance
func BenchmarkServiceContainer(b *testing.B) {
	container := NewServiceRegistry()
	service := &MockService{name: "test"}

	b.Run("ServiceRegister", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			container.Register("test-service", service)
		}
	})

	b.Run("ServiceGet", func(b *testing.B) {
		container.Register("test-service", service)
		for i := 0; i < b.N; i++ {
			container.Get("test-service")
		}
	})

	b.Run("ServiceValidate", func(b *testing.B) {
		container.Register("test-service", service)
		for i := 0; i < b.N; i++ {
			container.Validate()
		}
	})
}
