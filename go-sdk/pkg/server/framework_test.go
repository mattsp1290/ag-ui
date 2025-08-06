package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/testhelper"
)

func TestServerFramework(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultFrameworkConfig()
	
	// Create server framework
	framework := NewFramework()
	err := framework.Initialize(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, framework)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		framework.Stop(ctx)
	})

	t.Run("ServerFramework Configuration", func(t *testing.T) {
		assert.Equal(t, config.Name, framework.config.Name)
		assert.Equal(t, config.HTTP.Host, framework.config.HTTP.Host)
		assert.Equal(t, config.HTTP.Port, framework.config.HTTP.Port)
	})

	t.Run("ServerFramework Start and Stop", func(t *testing.T) {
		ctx := context.Background()
		
		// Start server
		err := framework.Start(ctx)
		require.NoError(t, err)
		
		// Verify running state
		assert.True(t, framework.IsRunning())
		
		// Stop server
		err = framework.Stop(ctx)
		require.NoError(t, err)
		
		// Verify stopped state
		assert.False(t, framework.IsRunning())
	})

	t.Run("ServerFramework Double Start Error", func(t *testing.T) {
		ctx := context.Background()
		
		// Start server
		err := framework.Start(ctx)
		require.NoError(t, err)
		defer framework.Stop(ctx)
		
		// Try to start again - should error
		err = framework.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in a startable state")
	})

	t.Run("ServerFramework Stop Without Start", func(t *testing.T) {
		ctx := context.Background()
		
		// Stop without starting should not error
		err := framework.Stop(ctx)
		assert.NoError(t, err)
	})
}

func TestServerFrameworkConfig(t *testing.T) {
	t.Run("DefaultFrameworkConfig", func(t *testing.T) {
		config := DefaultFrameworkConfig()
		
		assert.NotEmpty(t, config.Name)
		assert.NotEmpty(t, config.HTTP.Host)
		assert.Greater(t, config.HTTP.Port, 0)
		assert.Greater(t, config.HTTP.ReadTimeout, time.Duration(0))
		assert.Greater(t, config.HTTP.WriteTimeout, time.Duration(0))
		assert.Greater(t, config.Performance.RequestTimeout, time.Duration(0))
	})

	t.Run("FrameworkConfig Validation", func(t *testing.T) {
		tests := []struct {
			name    string
			config  *FrameworkConfig
			wantErr bool
		}{
			{
				name:    "nil config",
				config:  nil,
				wantErr: false, // Should use default config
			},
			{
				name: "empty name",
				config: &FrameworkConfig{
					Name: "",
					HTTP: HTTPConfig{
						Host: "localhost",
						Port: 8080,
					},
				},
				wantErr: true,
			},
			{
				name: "invalid port",
				config: &FrameworkConfig{
					Name: "test-server",
					HTTP: HTTPConfig{
						Host: "localhost",
						Port: -1,
					},
				},
				wantErr: true,
			},
			{
				name: "port too high",
				config: &FrameworkConfig{
					Name: "test-server",
					HTTP: HTTPConfig{
						Host: "localhost",
						Port: 70000,
					},
				},
				wantErr: true,
			},
			{
				name: "valid config",
				config: &FrameworkConfig{
					Name: "test-server",
					HTTP: HTTPConfig{
						Host:         "localhost",
						Port:         8080,
						ReadTimeout:  5 * time.Second,
						WriteTimeout: 5 * time.Second,
					},
					Performance: PerformanceConfig{
						RequestTimeout: 10 * time.Second,
					},
				},
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				framework := NewFramework()
				err := framework.Initialize(context.Background(), tt.config)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestServerFrameworkStatus(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultFrameworkConfig()
	
	framework := NewFramework()
	err := framework.Initialize(context.Background(), config)
	require.NoError(t, err)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		framework.Stop(ctx)
	})

	t.Run("Status Before Start", func(t *testing.T) {
		status := framework.GetStatus()
		assert.NotNil(t, status)
		assert.Equal(t, "initialized", status.State.String())
		assert.Zero(t, status.AgentCount)
		assert.Zero(t, status.RequestCount)
	})

	t.Run("Status After Start", func(t *testing.T) {
		ctx := context.Background()
		
		err := framework.Start(ctx)
		require.NoError(t, err)
		defer framework.Stop(ctx)
		
		status := framework.GetStatus()
		assert.NotNil(t, status)
		assert.Equal(t, "running", status.State.String())
		assert.NotZero(t, status.StartTime)
	})
}

func TestServerFrameworkMiddleware(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultFrameworkConfig()
	
	framework := NewFramework()
	err := framework.Initialize(context.Background(), config)
	require.NoError(t, err)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		framework.Stop(ctx)
	})

	t.Run("Register Middleware", func(t *testing.T) {
		// Mock middleware
		mockMiddleware := &mockMiddleware{name: "test-middleware"}
		
		// Register middleware
		err := framework.RegisterMiddleware(mockMiddleware)
		assert.NoError(t, err)
		
		// Try to register nil middleware
		err = framework.RegisterMiddleware(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "middleware cannot be nil")
	})
}

func TestServerFrameworkHealth(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultFrameworkConfig()
	
	framework := NewFramework()
	err := framework.Initialize(context.Background(), config)
	require.NoError(t, err)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		framework.Stop(ctx)
	})

	t.Run("Health Check", func(t *testing.T) {
		ctx := context.Background()
		
		// Health check before start
		health := framework.HealthCheck(ctx)
		assert.NotNil(t, health)
		assert.Equal(t, "unhealthy", health.Status)
		assert.False(t, health.Healthy)
		assert.NotEmpty(t, health.Errors)
		
		// Start server
		err := framework.Start(ctx)
		require.NoError(t, err)
		defer framework.Stop(ctx)
		
		// Health check after start
		health = framework.HealthCheck(ctx)
		assert.NotNil(t, health)
		assert.Equal(t, "healthy", health.Status)
		assert.True(t, health.Healthy)
		assert.Empty(t, health.Errors)
	})
}

func TestServerFrameworkConcurrency(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultFrameworkConfig()
	
	framework := NewFramework()
	err := framework.Initialize(context.Background(), config)
	require.NoError(t, err)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		framework.Stop(ctx)
	})

	t.Run("Concurrent Start Stop", func(t *testing.T) {
		ctx := context.Background()
		
		// Start multiple goroutines trying to start/stop
		done := make(chan bool, 10)
		
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				
				// Try to start
				framework.Start(ctx)
				time.Sleep(10 * time.Millisecond)
				
				// Try to stop
				framework.Stop(ctx)
			}()
		}
		
		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for goroutines")
			}
		}
		
		// Ensure server is in consistent state
		assert.False(t, framework.IsRunning())
	})

	t.Run("Concurrent Status Access", func(t *testing.T) {
		ctx := context.Background()
		
		err := framework.Start(ctx)
		require.NoError(t, err)
		defer framework.Stop(ctx)
		
		// Multiple goroutines accessing status
		done := make(chan bool, 20)
		
		for i := 0; i < 20; i++ {
			go func() {
				defer func() { done <- true }()
				
				for j := 0; j < 10; j++ {
					status := framework.GetStatus()
					assert.NotNil(t, status)
					time.Sleep(time.Millisecond)
				}
			}()
		}
		
		// Wait for all goroutines to complete
		for i := 0; i < 20; i++ {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for goroutines")
			}
		}
	})
}

func TestServerFrameworkEdgeCases(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	t.Run("New Framework with Nil Config", func(t *testing.T) {
		framework := NewFramework()
		err := framework.Initialize(context.Background(), nil)
		assert.NoError(t, err) // Should use default config
		assert.NotNil(t, framework)
		
		if framework != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			framework.Stop(ctx)
		}
	})

	t.Run("Framework Creation", func(t *testing.T) {
		framework := NewFramework()
		assert.NotNil(t, framework)
		config := DefaultFrameworkConfig()
		
		err := framework.Initialize(context.Background(), config)
		assert.NoError(t, err)
		
		if framework != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			framework.Stop(ctx)
		}
	})

	t.Run("Context Cancellation During Start", func(t *testing.T) {
		config := DefaultFrameworkConfig()
		
		framework := NewFramework()
		err := framework.Initialize(context.Background(), config)
		require.NoError(t, err)
		
		// Create context that will be cancelled
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		// Try to start with cancelled context
		err = framework.Start(ctx)
		// Behavior depends on implementation - might succeed or fail
		
		// Clean shutdown regardless
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		framework.Stop(shutdownCtx)
	})
}

// ==============================================================================
// INTERFACE SEGREGATION TESTS
// ==============================================================================

func TestInterfaceSegregation(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := DefaultFrameworkConfig()
	framework := NewFramework()
	err := framework.Initialize(context.Background(), config)
	require.NoError(t, err)
	
	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		framework.Stop(ctx)
	})

	t.Run("Framework satisfies all composed interfaces", func(t *testing.T) {
		// Test that BaseFramework satisfies all the decomposed interfaces
		var lifecycle FrameworkLifecycle = framework
		var agentRegistry AgentRegistry = framework
		var routeRegistry RouteRegistry = framework
		var statusProvider FrameworkStatusProvider = framework
		
		assert.NotNil(t, lifecycle)
		assert.NotNil(t, agentRegistry)
		assert.NotNil(t, routeRegistry)
		assert.NotNil(t, statusProvider)
	})

	t.Run("Framework satisfies composed interfaces", func(t *testing.T) {
		// Test that BaseFramework satisfies the composed interfaces
		var minimalFramework MinimalFramework = framework
		var agentFramework AgentFramework = framework
		var routingFramework RoutingFramework = framework
		var serverFramework ServerFramework = framework
		
		assert.NotNil(t, minimalFramework)
		assert.NotNil(t, agentFramework)
		assert.NotNil(t, routingFramework)
		assert.NotNil(t, serverFramework)
	})

	t.Run("Can use interfaces independently", func(t *testing.T) {
		// Test that we can use individual interfaces without full framework
		useOnlyRunning := func(sp FrameworkStatusProvider) bool {
			return sp.IsRunning()
		}
		
		useOnlyAgentRegistry := func(ar AgentRegistry) int {
			return len(ar.ListAgents())
		}
		
		useOnlyStatusProvider := func(sp FrameworkStatusProvider) FrameworkStatus {
			return sp.GetStatus()
		}
		
		// These should work with the framework
		assert.False(t, useOnlyRunning(framework))
		assert.Equal(t, 0, useOnlyAgentRegistry(framework))
		assert.NotEmpty(t, useOnlyStatusProvider(framework).State.String())
	})
}

// Mock agent for testing
type mockAgent struct {
	name        string
	description string
}

func (m *mockAgent) Name() string {
	return m.name
}

func (m *mockAgent) Description() string {
	return m.description
}

func (m *mockAgent) HandleEvent(ctx context.Context, event any) ([]any, error) {
	return nil, nil
}

func TestDecomposedAgentInterface(t *testing.T) {
	t.Run("Agent satisfies decomposed interfaces", func(t *testing.T) {
		agent := &mockAgent{
			name:        "test-agent",
			description: "A test agent",
		}
		
		// Test that our agent satisfies the decomposed interfaces
		var identity core.AgentIdentity = agent
		var handler core.AgentEventHandler = agent
		var fullAgent core.Agent = agent
		
		assert.NotNil(t, identity)
		assert.NotNil(t, handler)
		assert.NotNil(t, fullAgent)
		
		assert.Equal(t, "test-agent", identity.Name())
		assert.Equal(t, "A test agent", identity.Description())
	})
}

// Mock middleware for testing
type mockMiddleware struct {
	name string
}

func (m *mockMiddleware) Name() string {
	return m.name
}

func (m *mockMiddleware) Priority() int {
	return 0
}

func (m *mockMiddleware) Process(ctx context.Context, req *Request, resp ResponseWriter, next NextHandler) error {
	return next(ctx, req, resp)
}