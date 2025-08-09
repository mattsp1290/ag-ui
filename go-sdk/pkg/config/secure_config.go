// Package config provides a security-enhanced configuration management system
// with comprehensive resource limits, monitoring, and DoS protection.
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"
)

// SecureConfigImpl extends ConfigImpl with comprehensive security features
type SecureConfigImpl struct {
	// Embedded original config for compatibility
	*ConfigImpl
	
	// Security and resource management
	resourceManager *ResourceManager
	metrics         *MetricsCollector
	errorHandler    *ResourceErrorHandler
	
	// Security configuration
	securityEnabled   bool
	auditingEnabled   bool
	
	// Runtime state for resource tracking
	memoryUsage       int64
	lastOperationTime time.Time
	operationMu       sync.Mutex
}

// SecureConfigBuilder extends ConfigBuilder with security options
type SecureConfigBuilder struct {
	*ConfigBuilder
	resourceLimits  *ResourceLimits
	securityEnabled bool
	auditingEnabled bool
	metricsEnabled  bool
}

// SecurityOptions contains security-related configuration options
type SecurityOptions struct {
	EnableResourceLimits  bool             `json:"enable_resource_limits" yaml:"enable_resource_limits"`
	EnableMonitoring      bool             `json:"enable_monitoring" yaml:"enable_monitoring"`
	EnableAuditing        bool             `json:"enable_auditing" yaml:"enable_auditing"`
	ResourceLimits        *ResourceLimits  `json:"resource_limits,omitempty" yaml:"resource_limits,omitempty"`
	AlertThresholds       *AlertThresholds `json:"alert_thresholds,omitempty" yaml:"alert_thresholds,omitempty"`
	EnableGracefulDegradation bool         `json:"enable_graceful_degradation" yaml:"enable_graceful_degradation"`
}

// DefaultSecurityOptions returns sensible default security options
func DefaultSecurityOptions() *SecurityOptions {
	return &SecurityOptions{
		EnableResourceLimits:      true,
		EnableMonitoring:          true,
		EnableAuditing:            false, // Disabled by default for performance
		ResourceLimits:            DefaultResourceLimits(),
		AlertThresholds:           func() *AlertThresholds { t := DefaultAlertThresholds(); return &t }(),
		EnableGracefulDegradation: true,
	}
}

// NewSecureConfig creates a new secure configuration instance
func NewSecureConfig(options *SecurityOptions) *SecureConfigImpl {
	if options == nil {
		options = DefaultSecurityOptions()
	}
	
	baseConfig := NewConfig()
	
	var resourceManager *ResourceManager
	var metrics *MetricsCollector
	var errorHandler *ResourceErrorHandler
	
	if options.EnableResourceLimits {
		resourceManager = NewResourceManager(options.ResourceLimits)
	}
	
	if options.EnableMonitoring {
		metrics = NewMetricsCollector()
		if options.AlertThresholds != nil {
			metrics.UpdateAlertThresholds(*options.AlertThresholds)
		}
	}
	
	if options.EnableResourceLimits || options.EnableMonitoring {
		errorHandler = NewResourceErrorHandler()
		errorHandler.EnableGracefulDegradation = options.EnableGracefulDegradation
		errorHandler.EnableErrorRecovery = true
		errorHandler.EnableMetrics = options.EnableMonitoring
	}
	
	return &SecureConfigImpl{
		ConfigImpl:      baseConfig,
		resourceManager: resourceManager,
		metrics:         metrics,
		errorHandler:    errorHandler,
		securityEnabled: options.EnableResourceLimits,
		auditingEnabled: options.EnableAuditing,
	}
}

// NewSecureConfigBuilder creates a new secure configuration builder
func NewSecureConfigBuilder() *SecureConfigBuilder {
	return &SecureConfigBuilder{
		ConfigBuilder:   NewConfigBuilder(),
		resourceLimits:  DefaultResourceLimits(),
		securityEnabled: true,
		auditingEnabled: false,
		metricsEnabled:  true,
	}
}

// WithResourceLimits sets custom resource limits
func (b *SecureConfigBuilder) WithResourceLimits(limits *ResourceLimits) *SecureConfigBuilder {
	b.resourceLimits = limits
	return b
}

// WithSecurity enables or disables security features
func (b *SecureConfigBuilder) WithSecurity(enabled bool) *SecureConfigBuilder {
	b.securityEnabled = enabled
	return b
}

// WithAuditing enables or disables auditing
func (b *SecureConfigBuilder) WithAuditing(enabled bool) *SecureConfigBuilder {
	b.auditingEnabled = enabled
	return b
}

// WithMetrics enables or disables metrics collection
func (b *SecureConfigBuilder) WithMetrics(enabled bool) *SecureConfigBuilder {
	b.metricsEnabled = enabled
	return b
}

// BuildSecure builds a secure configuration instance
func (b *SecureConfigBuilder) BuildSecure() (*SecureConfigImpl, error) {
	return b.BuildSecureWithContext(context.Background())
}

// BuildSecureWithContext builds a secure configuration with context
func (b *SecureConfigBuilder) BuildSecureWithContext(ctx context.Context) (*SecureConfigImpl, error) {
	// Create security options
	options := &SecurityOptions{
		EnableResourceLimits:      b.securityEnabled,
		EnableMonitoring:          b.metricsEnabled,
		EnableAuditing:            b.auditingEnabled,
		ResourceLimits:            b.resourceLimits,
		AlertThresholds:           func() *AlertThresholds { t := DefaultAlertThresholds(); return &t }(),
		EnableGracefulDegradation: true,
	}
	
	// Create secure config
	secureConfig := NewSecureConfig(options)
	
	// Copy builder state to secure config
	secureConfig.ConfigImpl.profile = b.profile
	secureConfig.ConfigImpl.keyDelimiter = b.options.KeyDelimiter
	secureConfig.ConfigImpl.caseMapping = !b.options.CaseSensitive
	secureConfig.ConfigImpl.envPrefix = b.options.EnvPrefix
	
	if b.metadata != nil {
		secureConfig.ConfigImpl.metadata = b.metadata
		secureConfig.ConfigImpl.metadata.Profile = b.profile
	}
	
	// Sort sources by priority
	sort.Slice(b.sources, func(i, j int) bool {
		return b.sources[i].Priority() < b.sources[j].Priority()
	})
	
	secureConfig.ConfigImpl.sources = b.sources
	secureConfig.ConfigImpl.validators = b.validators
	
	// Load configuration from all sources with security checks
	if err := secureConfig.loadFromSourcesSecurely(ctx, b.options); err != nil {
		return nil, WithOperation("build", err)
	}
	
	// Validate configuration if enabled
	if b.options.ValidateOnBuild {
		if err := secureConfig.ValidateSecurely(); err != nil {
			return nil, WithOperation("build", err)
		}
	}
	
	// Start hot-reloading if enabled
	if b.options.EnableHotReload {
		if err := secureConfig.startSecureHotReload(ctx, b.options); err != nil {
			return nil, WithOperation("build", WithCategory(CategorySource, fmt.Errorf("failed to start secure hot-reloading: %w", err)))
		}
	}
	
	secureConfig.ConfigImpl.metadata.UpdatedAt = time.Now()
	return secureConfig, nil
}

// loadFromSourcesSecurely loads configuration from all sources with security checks
func (c *SecureConfigImpl) loadFromSourcesSecurely(ctx context.Context, options *BuilderOptions) error {
	if c.metrics != nil {
		return c.metrics.WithMonitoring("load", func() error {
			return c.doLoadFromSourcesSecurely(ctx, options)
		})
	}
	return c.doLoadFromSourcesSecurely(ctx, options)
}

// doLoadFromSourcesSecurely performs the actual loading with security checks
func (c *SecureConfigImpl) doLoadFromSourcesSecurely(ctx context.Context, options *BuilderOptions) error {
	merger := NewMerger(options.MergeStrategy)
	
	for _, source := range c.ConfigImpl.sources {
		// Apply timeout if resource manager is available
		loadCtx := ctx
		if c.resourceManager != nil {
			var cancel context.CancelFunc
			loadCtx, cancel = c.resourceManager.WithTimeout(ctx, "load")
			defer cancel()
		}
		
		// Check rate limits before loading
		if c.resourceManager != nil {
			if err := c.resourceManager.CanReload(); err != nil {
				if c.errorHandler != nil {
					if handledErr := c.errorHandler.HandleError(err); handledErr != nil {
						return WithOperation("load", WithSource(source.Name(), handledErr))
					}
					// Rate limit error was handled gracefully, skip this load
					continue
				}
				return WithOperation("load", WithSource(source.Name(), err))
			}
		}
		
		// Load data from source
		data, err := source.Load(loadCtx)
		if err != nil {
			if c.errorHandler != nil {
				if handledErr := c.errorHandler.HandleError(err); handledErr != nil {
					return WithOperation("load", WithSource(source.Name(), handledErr))
				}
				// Error was handled gracefully, use empty data
				data = make(map[string]interface{})
			} else {
				return WithOperation("load", WithSource(source.Name(), err))
			}
		}
		
		// Validate data size and structure if security is enabled
		if c.securityEnabled && c.resourceManager != nil {
			// Estimate memory usage of the loaded data
			estimatedSize := c.estimateMemoryUsage(data)
			if err := c.resourceManager.ValidateMemoryUsage(estimatedSize); err != nil {
				if c.errorHandler != nil {
					if handledErr := c.errorHandler.HandleError(err); handledErr != nil {
						return WithOperation("load", WithSource(source.Name(), handledErr))
					}
					// Memory limit error was handled gracefully, skip this source
					continue
				}
				return WithOperation("load", WithSource(source.Name(), err))
			}
			
			// Validate configuration structure
			if err := c.resourceManager.ValidateConfigStructure(data); err != nil {
				if c.errorHandler != nil {
					if handledErr := c.errorHandler.HandleError(err); handledErr != nil {
						return WithOperation("load", WithSource(source.Name(), handledErr))
					}
					// Structure limit error was handled gracefully, skip this source
					continue
				}
				return WithOperation("load", WithSource(source.Name(), err))
			}
			
			// Update resource tracking
			c.resourceManager.UpdateMemoryUsage(estimatedSize)
			c.updateMemoryUsage(estimatedSize)
		}
		
		// Apply profile filtering if data contains profiles
		if c.ConfigImpl.profile != "" {
			if profiles, ok := data["profiles"]; ok {
				if profileMap, ok := profiles.(map[string]interface{}); ok {
					if profileData, ok := profileMap[c.ConfigImpl.profile]; ok {
						if profileDataMap, ok := profileData.(map[string]interface{}); ok {
							data = merger.Merge(data, profileDataMap)
						}
					}
				}
			}
		}
		
		// Merge with existing configuration
		c.ConfigImpl.mu.Lock()
		oldSize := c.estimateMemoryUsage(c.ConfigImpl.data)
		c.ConfigImpl.data = merger.Merge(c.ConfigImpl.data, data)
		newSize := c.estimateMemoryUsage(c.ConfigImpl.data)
		c.ConfigImpl.mu.Unlock()
		
		// Update memory tracking
		if c.resourceManager != nil {
			c.resourceManager.UpdateMemoryUsage(newSize - oldSize)
		}
		if c.metrics != nil {
			c.metrics.RecordMemoryUsage(newSize)
		}
	}
	
	return nil
}

// ValidateSecurely performs validation with resource limits
func (c *SecureConfigImpl) ValidateSecurely() error {
	if c.metrics != nil {
		return c.metrics.WithMonitoring("validate", func() error {
			return c.doValidateSecurely()
		})
	}
	return c.doValidateSecurely()
}

// doValidateSecurely performs the actual validation
func (c *SecureConfigImpl) doValidateSecurely() error {
	// Check rate limits
	if c.resourceManager != nil {
		if err := c.resourceManager.CanValidate(); err != nil {
			if c.errorHandler != nil {
				return c.errorHandler.HandleError(err)
			}
			return err
		}
	}
	
	// Apply timeout
	ctx := context.Background()
	if c.resourceManager != nil {
		var cancel context.CancelFunc
		ctx, cancel = c.resourceManager.WithTimeout(ctx, "validate")
		defer cancel()
	}
	
	// Run validation in a goroutine to respect timeouts
	resultCh := make(chan error, 1)
	go func() {
		resultCh <- c.ConfigImpl.Validate()
	}()
	
	select {
	case err := <-resultCh:
		if err != nil && c.errorHandler != nil {
			return c.errorHandler.HandleError(err)
		}
		return err
	case <-ctx.Done():
		timeoutErr := NewTimeoutError("validate", c.resourceManager.GetLimits().ValidationTimeout, 
			c.resourceManager.GetLimits().ValidationTimeout, "validation operation timed out")
		if c.errorHandler != nil {
			return c.errorHandler.HandleError(timeoutErr)
		}
		return timeoutErr
	}
}

// Watch adds a secure watcher with resource limits
func (c *SecureConfigImpl) Watch(key string, callback func(interface{})) (CallbackID, error) {
	// Check if we can add another watcher
	if c.resourceManager != nil {
		if err := c.resourceManager.CanAddWatcher(key); err != nil {
			if c.errorHandler != nil {
				if handledErr := c.errorHandler.HandleError(err); handledErr != nil {
					return "", handledErr
				}
				// Error was handled gracefully, but we still can't add the watcher
				return "", fmt.Errorf("cannot add watcher: resource limit exceeded (handled gracefully)")
			}
			return "", err
		}
	}
	
	// Add the watcher using the base implementation
	callbackID, err := c.ConfigImpl.Watch(key, callback)
	if err != nil {
		return callbackID, err
	}
	
	// Update resource tracking
	if c.resourceManager != nil {
		c.resourceManager.AddWatcher(key)
	}
	if c.metrics != nil {
		c.metrics.RecordWatcherAdded()
	}
	
	return callbackID, nil
}

// UnWatch removes a watcher with resource tracking
func (c *SecureConfigImpl) UnWatch(key string, callbackID CallbackID) error {
	err := c.ConfigImpl.UnWatch(key, callbackID)
	if err == nil {
		// Update resource tracking
		if c.resourceManager != nil {
			c.resourceManager.RemoveWatcher(key)
		}
		if c.metrics != nil {
			c.metrics.RecordWatcherRemoved()
		}
	}
	return err
}

// Set sets a configuration value with security checks
func (c *SecureConfigImpl) Set(key string, value interface{}) error {
	// Check rate limits
	if c.resourceManager != nil {
		if err := c.resourceManager.CanUpdate(); err != nil {
			if c.errorHandler != nil {
				if handledErr := c.errorHandler.HandleError(err); handledErr != nil {
					return handledErr
				}
				// Rate limit error was handled gracefully, skip this update
				return nil
			}
			return err
		}
	}
	
	// Validate the new value's structure and size
	if c.securityEnabled && c.resourceManager != nil {
		// Create a temporary map to validate the structure
		tempData := map[string]interface{}{key: value}
		if err := c.resourceManager.ValidateConfigStructure(tempData); err != nil {
			if c.errorHandler != nil {
				return c.errorHandler.HandleError(err)
			}
			return err
		}
		
		// Estimate memory impact
		estimatedSize := c.estimateMemoryUsage(tempData)
		if err := c.resourceManager.ValidateMemoryUsage(estimatedSize); err != nil {
			if c.errorHandler != nil {
				return c.errorHandler.HandleError(err)
			}
			return err
		}
	}
	
	// Perform the actual set operation
	err := c.ConfigImpl.Set(key, value)
	if err == nil {
		// Update resource tracking
		c.updateLastOperationTime()
		if c.metrics != nil {
			c.metrics.RecordOperation()
		}
		
		// Update memory usage tracking after successful set
		if c.securityEnabled && c.resourceManager != nil {
			tempData := map[string]interface{}{key: value}
			estimatedSize := c.estimateMemoryUsage(tempData)
			c.resourceManager.UpdateMemoryUsage(estimatedSize)
			c.updateMemoryUsage(estimatedSize)
		}
	}
	
	return err
}

// startSecureHotReload starts hot-reloading with security features
func (c *SecureConfigImpl) startSecureHotReload(ctx context.Context, options *BuilderOptions) error {
	// Use the config's hot-reload context
	watchCtx := c.ConfigImpl.hotReloadCtx
	
	for _, source := range c.ConfigImpl.sources {
		if source.CanWatch() {
			go func(s Source) {
				defer func() {
					if r := recover(); r != nil {
						// Log the panic but don't crash the system
						if c.metrics != nil {
							c.metrics.RecordError()
						}
					}
				}()
				
				s.Watch(watchCtx, func(data map[string]interface{}) {
					// Check if config is still active before processing
					select {
					case <-c.ConfigImpl.hotReloadCtx.Done():
						return
					default:
					}
					
					// Apply security checks to reload data
					if c.securityEnabled && c.resourceManager != nil {
						// Check rate limits
						if err := c.resourceManager.CanReload(); err != nil {
							if c.metrics != nil {
								c.metrics.RecordError()
							}
							return // Skip this reload due to rate limiting
						}
						
						// Validate structure and size
						if err := c.resourceManager.ValidateConfigStructure(data); err != nil {
							if c.metrics != nil {
								c.metrics.RecordError()
								c.metrics.RecordStructureLimitHit()
							}
							return // Skip this reload due to structure violations
						}
						
						estimatedSize := c.estimateMemoryUsage(data)
						if err := c.resourceManager.ValidateMemoryUsage(estimatedSize); err != nil {
							if c.metrics != nil {
								c.metrics.RecordError()
								c.metrics.RecordResourceLimitHit()
							}
							return // Skip this reload due to memory limits
						}
					}
					
					c.ConfigImpl.mu.Lock()
					merger := NewMerger(options.MergeStrategy)
					
					// Apply profile filtering
					if c.ConfigImpl.profile != "" {
						if profiles, ok := data["profiles"]; ok {
							if profileMap, ok := profiles.(map[string]interface{}); ok {
								if profileData, ok := profileMap[c.ConfigImpl.profile]; ok {
									if profileDataMap, ok := profileData.(map[string]interface{}); ok {
										data = merger.Merge(data, profileDataMap)
									}
								}
							}
						}
					}
					
					oldData := c.ConfigImpl.data
					c.ConfigImpl.data = merger.Merge(c.ConfigImpl.data, data)
					c.ConfigImpl.metadata.UpdatedAt = time.Now()
					c.ConfigImpl.mu.Unlock()
					
					// Update metrics
					if c.metrics != nil {
						c.metrics.RecordReload()
					}
					
					// Trigger watchers asynchronously
					c.triggerWatchersAsync(oldData, c.ConfigImpl.data)
				})
			}(source)
		}
	}
	return nil
}

// GetResourceStats returns current resource usage statistics
func (c *SecureConfigImpl) GetResourceStats() ResourceStats {
	if c.resourceManager != nil {
		return c.resourceManager.GetStats()
	}
	return ResourceStats{}
}

// GetMetricsSnapshot returns current metrics
func (c *SecureConfigImpl) GetMetricsSnapshot() MetricsSnapshot {
	if c.metrics != nil {
		return c.metrics.GetMetrics()
	}
	return MetricsSnapshot{}
}

// GetHealthStatus returns current health status
func (c *SecureConfigImpl) GetHealthStatus() HealthStatus {
	if c.metrics != nil {
		return c.metrics.HealthCheck()
	}
	return HealthStatus{
		Overall:   "unknown",
		Timestamp: time.Now(),
		Checks:    make(map[string]CheckResult),
	}
}

// UpdateResourceLimits updates the resource limits at runtime
func (c *SecureConfigImpl) UpdateResourceLimits(limits *ResourceLimits) error {
	if c.resourceManager != nil {
		return c.resourceManager.UpdateLimits(limits)
	}
	return fmt.Errorf("resource manager not enabled")
}

// Helper methods

// estimateMemoryUsage estimates the memory usage of configuration data
func (c *SecureConfigImpl) estimateMemoryUsage(data map[string]interface{}) int64 {
	// This is a rough estimation - in production you might want a more accurate measurement
	jsonData, err := json.Marshal(data)
	if err != nil {
		return 0
	}
	return int64(len(jsonData))
}

// updateMemoryUsage updates the tracked memory usage
func (c *SecureConfigImpl) updateMemoryUsage(delta int64) {
	c.operationMu.Lock()
	c.memoryUsage += delta
	c.operationMu.Unlock()
}

// updateLastOperationTime updates the last operation timestamp
func (c *SecureConfigImpl) updateLastOperationTime() {
	c.operationMu.Lock()
	c.lastOperationTime = time.Now()
	c.operationMu.Unlock()
}

// triggerWatchersAsync triggers watchers asynchronously with timeout protection
func (c *SecureConfigImpl) triggerWatchersAsync(oldData, newData map[string]interface{}) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if c.metrics != nil {
					c.metrics.RecordError()
				}
			}
		}()
		
		// Create timeout context for watcher callbacks
		ctx := context.Background()
		if c.resourceManager != nil {
			var cancel context.CancelFunc
			ctx, cancel = c.resourceManager.WithTimeout(ctx, "watcher")
			defer cancel()
		}
		
		// Take a snapshot of watchers to avoid holding locks during comparison
		c.ConfigImpl.watcherMu.RLock()
		watchersCopy := make(map[string][]WatcherCallback)
		for key, callbacks := range c.ConfigImpl.watchers {
			callbacksCopy := make([]WatcherCallback, len(callbacks))
			copy(callbacksCopy, callbacks)
			watchersCopy[key] = callbacksCopy
		}
		c.ConfigImpl.watcherMu.RUnlock()
		
		// Process watcher notifications with timeout protection
		for key := range watchersCopy {
			oldVal := c.getNestedValue(oldData, key)
			newVal := c.getNestedValue(newData, key)
			
			if !reflect.DeepEqual(oldVal, newVal) {
				select {
				case c.ConfigImpl.notificationCh <- watcherNotification{key: key, value: newVal}:
				case <-ctx.Done():
					if c.metrics != nil {
						c.metrics.RecordTimeoutHit()
					}
					return // Timeout exceeded
				case <-c.ConfigImpl.hotReloadCtx.Done():
					return // Config has been shut down
				default:
					// Channel is full, skip this notification
					if c.metrics != nil {
						c.metrics.RecordError()
					}
				}
			}
		}
	}()
}

// getNestedValue retrieves a nested value from a map using dot notation
func (c *SecureConfigImpl) getNestedValue(data map[string]interface{}, key string) interface{} {
	keys := c.splitKey(key)
	current := data
	
	for i, k := range keys {
		if i == len(keys)-1 {
			return current[k]
		}
		
		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}
	
	return nil
}

// splitKey splits a configuration key by delimiter
func (c *SecureConfigImpl) splitKey(key string) []string {
	if c.ConfigImpl.keyDelimiter == "" {
		return []string{key}
	}
	
	result := []string{}
	current := ""
	
	for _, char := range key {
		if string(char) == c.ConfigImpl.keyDelimiter {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	
	if current != "" {
		result = append(result, current)
	}
	
	return result
}

// IsSecurityEnabled returns whether security features are enabled
func (c *SecureConfigImpl) IsSecurityEnabled() bool {
	return c.securityEnabled
}

// IsMonitoringEnabled returns whether monitoring is enabled
func (c *SecureConfigImpl) IsMonitoringEnabled() bool {
	return c.metrics != nil
}

// IsAuditingEnabled returns whether auditing is enabled
func (c *SecureConfigImpl) IsAuditingEnabled() bool {
	return c.auditingEnabled
}