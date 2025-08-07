package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// AgentUtils provides utilities for agent lifecycle management and monitoring.
type AgentUtils struct {
	healthCheckers map[string]*HealthChecker
	checkersMu     sync.RWMutex
}

// HealthReport represents the health status of an agent.
type HealthReport struct {
	AgentName      string                 `json:"agent_name"`
	Status         string                 `json:"status"` // "healthy", "degraded", "unhealthy"
	LastCheck      time.Time              `json:"last_check"`
	ResponseTime   time.Duration          `json:"response_time"`
	Metrics        map[string]interface{} `json:"metrics"`
	Errors         []string               `json:"errors"`
	Warnings       []string               `json:"warnings"`
	Dependencies   []DependencyStatus     `json:"dependencies"`
	Configuration  map[string]interface{} `json:"configuration"`
}

// DependencyStatus represents the status of an agent dependency.
type DependencyStatus struct {
	Name         string        `json:"name"`
	Type         string        `json:"type"`
	Status       string        `json:"status"`
	ResponseTime time.Duration `json:"response_time"`
	LastCheck    time.Time     `json:"last_check"`
	Error        string        `json:"error,omitempty"`
}

// AgentBackup represents a complete backup of an agent's configuration and state.
type AgentBackup struct {
	Name         string                 `json:"name"`
	Config       map[string]interface{} `json:"config"`
	State        map[string]interface{} `json:"state"`
	Metadata     map[string]interface{} `json:"metadata"`
	BackupTime   time.Time              `json:"backup_time"`
	Version      string                 `json:"version"`
	Dependencies []string               `json:"dependencies"`
}

// HealthChecker performs health checks on agents.
type HealthChecker struct {
	agent       client.Agent
	interval    time.Duration
	timeout     time.Duration
	ctx         context.Context
	cancel      context.CancelFunc
	lastReport  *HealthReport
	reportMu    sync.RWMutex
	isRunning   bool
	runningMu   sync.RWMutex
}

// NewAgentUtils creates a new AgentUtils instance.
func NewAgentUtils() *AgentUtils {
	return &AgentUtils{
		healthCheckers: make(map[string]*HealthChecker),
	}
}

// Clone creates a deep copy of an agent with its current configuration.
// The cloned agent will be in an uninitialized state and must be initialized
// before use.
func (au *AgentUtils) Clone(agent client.Agent) (client.Agent, error) {
	if agent == nil {
		return nil, errors.NewValidationError("agent", "agent cannot be nil")
	}

	// Check if agent supports cloning by checking if it implements the necessary interfaces

	// Create configuration copy
	originalConfig, err := au.extractAgentConfig(agent)
	if err != nil {
		return nil, errors.WrapWithContext(err, "Clone", "failed to extract agent configuration")
	}

	// Deep clone configuration
	clonedConfig, err := au.deepCloneConfig(originalConfig)
	if err != nil {
		return nil, errors.WrapWithContext(err, "Clone", "failed to clone agent configuration")
	}

	// Create new agent instance based on the original type
	clonedAgent, err := au.createAgentFromConfig(agent, clonedConfig)
	if err != nil {
		return nil, errors.WrapWithContext(err, "Clone", "failed to create cloned agent")
	}

	return clonedAgent, nil
}

// Merge combines configurations from base and override configurations.
func (au *AgentUtils) Merge(base, override *client.AgentConfig) (*client.AgentConfig, error) {
	if base == nil {
		return nil, errors.NewValidationError("base", "base configuration cannot be nil")
	}

	if override == nil {
		return base, nil
	}

	// Convert to maps for easier merging
	baseMap, err := au.configToMap(base)
	if err != nil {
		return nil, errors.WrapWithContext(err, "Merge", "failed to convert base config to map")
	}

	overrideMap, err := au.configToMap(override)
	if err != nil {
		return nil, errors.WrapWithContext(err, "Merge", "failed to convert override config to map")
	}

	// Merge configurations
	merged := au.mergeMaps(baseMap, overrideMap)

	// Convert back to AgentConfig
	result, err := au.mapToConfig(merged)
	if err != nil {
		return nil, errors.WrapWithContext(err, "Merge", "failed to convert merged map to config")
	}

	return result, nil
}

// Health performs a comprehensive health check on an agent.
func (au *AgentUtils) Health(agent client.Agent) (*HealthReport, error) {
	if agent == nil {
		return nil, errors.NewValidationError("agent", "agent cannot be nil")
	}

	startTime := time.Now()
	report := &HealthReport{
		AgentName:     agent.Name(),
		LastCheck:     startTime,
		Metrics:       make(map[string]interface{}),
		Errors:        make([]string, 0),
		Warnings:      make([]string, 0),
		Dependencies:  make([]DependencyStatus, 0),
		Configuration: make(map[string]interface{}),
	}

	// Check basic agent health
	healthStatus := agent.Health()
	report.Status = healthStatus.Status
	
	// Add any health errors
	for _, err := range healthStatus.Errors {
		report.Errors = append(report.Errors, err)
	}

	// Collect metrics
	capabilities := agent.Capabilities()
	report.Metrics["capabilities"] = capabilities

	// Check agent configuration
	config, err := au.extractAgentConfig(agent)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Failed to extract configuration: %v", err))
	} else {
		configMap, _ := au.configToMap(config)
		report.Configuration = configMap
	}

	// Calculate response time
	report.ResponseTime = time.Since(startTime)

	// Determine overall status
	if len(report.Errors) > 0 {
		report.Status = "unhealthy"
	} else if len(report.Warnings) > 0 {
		report.Status = "degraded"
	} else {
		report.Status = "healthy"
	}

	return report, nil
}

// StartHealthMonitoring starts continuous health monitoring for an agent.
func (au *AgentUtils) StartHealthMonitoring(agent client.Agent, interval time.Duration) error {
	if agent == nil {
		return errors.NewValidationError("agent", "agent cannot be nil")
	}

	if interval <= 0 {
		interval = 30 * time.Second // Default interval
	}

	agentName := agent.Name()
	
	au.checkersMu.Lock()
	defer au.checkersMu.Unlock()

	// Stop existing checker if present
	if existing, exists := au.healthCheckers[agentName]; exists {
		existing.Stop()
	}

	// Create new health checker
	ctx, cancel := context.WithCancel(context.Background())
	checker := &HealthChecker{
		agent:    agent,
		interval: interval,
		timeout:  10 * time.Second,
		ctx:      ctx,
		cancel:   cancel,
	}

	au.healthCheckers[agentName] = checker
	go checker.Run(au)

	return nil
}

// StopHealthMonitoring stops health monitoring for an agent.
func (au *AgentUtils) StopHealthMonitoring(agentName string) error {
	au.checkersMu.Lock()
	defer au.checkersMu.Unlock()

	if checker, exists := au.healthCheckers[agentName]; exists {
		checker.Stop()
		delete(au.healthCheckers, agentName)
	}

	return nil
}

// GetHealthReport returns the latest health report for an agent.
func (au *AgentUtils) GetHealthReport(agentName string) (*HealthReport, error) {
	au.checkersMu.RLock()
	defer au.checkersMu.RUnlock()

	checker, exists := au.healthCheckers[agentName]
	if !exists {
		return nil, errors.NewNotFoundError("health_checker not found: " + agentName, nil)
	}

	return checker.GetLastReport(), nil
}

// Backup creates a complete backup of an agent's configuration and state.
func (au *AgentUtils) Backup(agent client.Agent) (*AgentBackup, error) {
	if agent == nil {
		return nil, errors.NewValidationError("agent", "agent cannot be nil")
	}

	backup := &AgentBackup{
		Name:       agent.Name(),
		BackupTime: time.Now(),
		Version:    "1.0",
		Metadata:   make(map[string]interface{}),
	}

	// Backup configuration
	config, err := au.extractAgentConfig(agent)
	if err != nil {
		return nil, errors.WrapWithContext(err, "Backup", "failed to extract configuration")
	}

	configMap, err := au.configToMap(config)
	if err != nil {
		return nil, errors.WrapWithContext(err, "Backup", "failed to convert configuration to map")
	}
	backup.Config = configMap

	// Backup state if available
	if stateManager, ok := agent.(client.StateManager); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		state, err := stateManager.GetState(ctx)
		if err != nil {
			backup.Metadata["state_backup_error"] = err.Error()
		} else if state != nil {
			stateMap, err := au.stateToMap(state)
			if err != nil {
				backup.Metadata["state_conversion_error"] = err.Error()
			} else {
				backup.State = stateMap
			}
		}
	}

	// Add metadata
	backup.Metadata["capabilities"] = agent.Capabilities()
	backup.Metadata["description"] = agent.Description()

	return backup, nil
}

// Restore creates a new agent from a backup.
func (au *AgentUtils) Restore(backup *AgentBackup) (client.Agent, error) {
	if backup == nil {
		return nil, errors.NewValidationError("backup", "backup cannot be nil")
	}

	// Convert config map back to AgentConfig
	_, err := au.mapToConfig(backup.Config)
	if err != nil {
		return nil, errors.WrapWithContext(err, "Restore", "failed to restore configuration")
	}

	// Create a new agent (this would need to be implemented based on your agent factory)
	// For now, we return an error indicating this needs implementation
	return nil, errors.NewOperationError("Restore", "agent", fmt.Errorf("agent restoration not implemented - requires agent factory"))
}

// Helper methods

func (au *AgentUtils) extractAgentConfig(agent client.Agent) (*client.AgentConfig, error) {
	// This is a placeholder - the actual implementation would depend on
	// how your agents expose their configuration
	return &client.AgentConfig{}, nil
}

func (au *AgentUtils) deepCloneConfig(config *client.AgentConfig) (*client.AgentConfig, error) {
	// Use JSON marshaling/unmarshaling for deep cloning
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var cloned client.AgentConfig
	err = json.Unmarshal(data, &cloned)
	if err != nil {
		return nil, err
	}

	return &cloned, nil
}

func (au *AgentUtils) createAgentFromConfig(original client.Agent, config *client.AgentConfig) (client.Agent, error) {
	// This is a placeholder - the actual implementation would use your agent factory
	// to create a new agent of the same type as the original
	return nil, fmt.Errorf("agent creation from config not implemented - requires agent factory")
}

func (au *AgentUtils) configToMap(config *client.AgentConfig) (map[string]interface{}, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	return result, err
}

func (au *AgentUtils) mapToConfig(data map[string]interface{}) (*client.AgentConfig, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var config client.AgentConfig
	err = json.Unmarshal(jsonData, &config)
	return &config, err
}

func (au *AgentUtils) stateToMap(state *client.AgentState) (map[string]interface{}, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	return result, err
}

func (au *AgentUtils) mergeMaps(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy base map
	for k, v := range base {
		result[k] = v
	}

	// Override with values from override map
	for k, v := range override {
		if baseVal, exists := result[k]; exists {
			// If both values are maps, merge recursively
			if baseMap, baseIsMap := baseVal.(map[string]interface{}); baseIsMap {
				if overrideMap, overrideIsMap := v.(map[string]interface{}); overrideIsMap {
					result[k] = au.mergeMaps(baseMap, overrideMap)
					continue
				}
			}
		}
		result[k] = v
	}

	return result
}

// HealthChecker methods

// Run starts the health checking loop.
func (hc *HealthChecker) Run(utils *AgentUtils) {
	hc.runningMu.Lock()
	hc.isRunning = true
	hc.runningMu.Unlock()

	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.ctx.Done():
			hc.runningMu.Lock()
			hc.isRunning = false
			hc.runningMu.Unlock()
			return
		case <-ticker.C:
			report, err := utils.Health(hc.agent)
			if err != nil {
				// Create error report
				report = &HealthReport{
					AgentName: hc.agent.Name(),
					Status:    "unhealthy",
					LastCheck: time.Now(),
					Errors:    []string{err.Error()},
				}
			}

			hc.reportMu.Lock()
			hc.lastReport = report
			hc.reportMu.Unlock()
		}
	}
}

// Stop stops the health checker.
func (hc *HealthChecker) Stop() {
	if hc.cancel != nil {
		hc.cancel()
	}
}

// GetLastReport returns the last health report.
func (hc *HealthChecker) GetLastReport() *HealthReport {
	hc.reportMu.RLock()
	defer hc.reportMu.RUnlock()
	
	if hc.lastReport == nil {
		return &HealthReport{
			AgentName: hc.agent.Name(),
			Status:    "unknown",
			LastCheck: time.Now(),
		}
	}
	
	return hc.lastReport
}

// IsRunning returns whether the health checker is currently running.
func (hc *HealthChecker) IsRunning() bool {
	hc.runningMu.RLock()
	defer hc.runningMu.RUnlock()
	return hc.isRunning
}