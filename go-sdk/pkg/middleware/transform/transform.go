package transform

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TransformationConfig represents transformation middleware configuration
type TransformationConfig struct {
	Pipelines       []PipelineConfig `json:"pipelines" yaml:"pipelines"`
	DefaultPipeline string           `json:"default_pipeline" yaml:"default_pipeline"`
	SkipPaths       []string         `json:"skip_paths" yaml:"skip_paths"`
	SkipHealthCheck bool             `json:"skip_health_check" yaml:"skip_health_check"`
}

// PipelineConfig represents pipeline configuration
type PipelineConfig struct {
	Name         string                 `json:"name" yaml:"name"`
	Enabled      bool                   `json:"enabled" yaml:"enabled"`
	Transformers []TransformerConfig    `json:"transformers" yaml:"transformers"`
	Conditions   map[string]interface{} `json:"conditions" yaml:"conditions"`
}

// TransformerConfig represents transformer configuration
type TransformerConfig struct {
	Type    string                 `json:"type" yaml:"type"`
	Name    string                 `json:"name" yaml:"name"`
	Enabled bool                   `json:"enabled" yaml:"enabled"`
	Config  map[string]interface{} `json:"config" yaml:"config"`
}

// TransformationMiddleware implements transformation middleware
type TransformationMiddleware struct {
	config    *TransformationConfig
	pipelines map[string]*Pipeline
	enabled   bool
	priority  int
	skipMap   map[string]bool
	mu        sync.RWMutex
}

// NewTransformationMiddleware creates a new transformation middleware
func NewTransformationMiddleware(config *TransformationConfig) (*TransformationMiddleware, error) {
	if config == nil {
		config = &TransformationConfig{
			DefaultPipeline: "default",
			SkipHealthCheck: true,
		}
	}

	skipMap := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipMap[path] = true
	}

	// Add common health check paths
	if config.SkipHealthCheck {
		skipMap["/health"] = true
		skipMap["/healthz"] = true
		skipMap["/ping"] = true
		skipMap["/ready"] = true
		skipMap["/live"] = true
	}

	tm := &TransformationMiddleware{
		config:    config,
		pipelines: make(map[string]*Pipeline),
		enabled:   true,
		priority:  20, // Medium-low priority
		skipMap:   skipMap,
	}

	// Initialize pipelines
	if err := tm.initializePipelines(); err != nil {
		return nil, err
	}

	return tm, nil
}

// Name returns middleware name
func (tm *TransformationMiddleware) Name() string {
	return "transformation"
}

// Process processes the request through transformation middleware
func (tm *TransformationMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Skip transformation for configured paths
	if tm.skipMap[req.Path] {
		return next(ctx, req)
	}

	// Select pipeline based on request
	pipeline := tm.selectPipeline(ctx, req)
	if pipeline == nil || !pipeline.Enabled() {
		return next(ctx, req)
	}

	// Transform request
	if err := pipeline.TransformRequest(ctx, req); err != nil {
		return &Response{
			ID:         req.ID,
			StatusCode: 400,
			Error:      fmt.Errorf("request transformation failed: %w", err),
			Timestamp:  time.Now(),
		}, nil
	}

	// Process request through next middleware
	resp, err := next(ctx, req)
	if err != nil {
		return resp, err
	}

	// Transform response
	if resp != nil {
		if err := pipeline.TransformResponse(ctx, resp); err != nil {
			resp.Error = fmt.Errorf("response transformation failed: %w", err)
			resp.StatusCode = 500
		}
	}

	return resp, err
}

// Configure configures the middleware
func (tm *TransformationMiddleware) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		tm.enabled = enabled
	}

	if priority, ok := config["priority"].(int); ok {
		tm.priority = priority
	}

	return nil
}

// Enabled returns whether the middleware is enabled
func (tm *TransformationMiddleware) Enabled() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.enabled
}

// Priority returns the middleware priority
func (tm *TransformationMiddleware) Priority() int {
	return tm.priority
}

// AddPipeline adds a pipeline to the middleware
func (tm *TransformationMiddleware) AddPipeline(pipeline *Pipeline) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.pipelines[pipeline.Name()] = pipeline
}

// GetPipeline returns a pipeline by name
func (tm *TransformationMiddleware) GetPipeline(name string) *Pipeline {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.pipelines[name]
}

// RemovePipeline removes a pipeline
func (tm *TransformationMiddleware) RemovePipeline(name string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.pipelines[name]; exists {
		delete(tm.pipelines, name)
		return true
	}
	return false
}

// ListPipelines returns all pipeline names
func (tm *TransformationMiddleware) ListPipelines() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	names := make([]string, 0, len(tm.pipelines))
	for name := range tm.pipelines {
		names = append(names, name)
	}
	return names
}

// initializePipelines initializes pipelines from configuration
func (tm *TransformationMiddleware) initializePipelines() error {
	for _, pipelineConfig := range tm.config.Pipelines {
		pipeline := NewPipeline(pipelineConfig.Name)
		pipeline.SetEnabled(pipelineConfig.Enabled)

		// Create transformers for the pipeline
		for _, transformerConfig := range pipelineConfig.Transformers {
			transformer, err := tm.createTransformer(transformerConfig)
			if err != nil {
				return fmt.Errorf("failed to create transformer %s: %w", transformerConfig.Name, err)
			}

			if transformer != nil {
				pipeline.AddTransformer(transformer)
			}
		}

		tm.pipelines[pipeline.Name()] = pipeline
	}

	return nil
}

// createTransformer creates a transformer from configuration
func (tm *TransformationMiddleware) createTransformer(config TransformerConfig) (Transformer, error) {
	switch config.Type {
	case "compression":
		algorithm := "gzip"
		level := 6

		if alg, ok := config.Config["algorithm"].(string); ok {
			algorithm = alg
		}
		if lvl, ok := config.Config["level"].(int); ok {
			level = lvl
		}

		transformer := NewCompressionTransformer(algorithm, level)
		transformer.Configure(config.Config)
		return transformer, nil

	case "sanitization":
		fields := []string{"password", "token", "secret", "key"}
		replacement := "[REDACTED]"

		if f, ok := config.Config["sensitive_fields"].([]interface{}); ok {
			fields = make([]string, len(f))
			for i, field := range f {
				if fieldStr, ok := field.(string); ok {
					fields[i] = fieldStr
				}
			}
		}

		if r, ok := config.Config["replacement"].(string); ok {
			replacement = r
		}

		transformer := NewSanitizationTransformer(fields, replacement)
		transformer.Configure(config.Config)
		return transformer, nil

	case "validation":
		transformer := NewValidationTransformer()
		transformer.Configure(config.Config)
		return transformer, nil

	default:
		return nil, fmt.Errorf("unknown transformer type: %s", config.Type)
	}
}

// selectPipeline selects the appropriate pipeline for a request
func (tm *TransformationMiddleware) selectPipeline(ctx context.Context, req *Request) *Pipeline {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// For now, use the default pipeline
	// In a more advanced implementation, you could select based on request properties
	if defaultPipeline, exists := tm.pipelines[tm.config.DefaultPipeline]; exists {
		return defaultPipeline
	}

	// Return first available pipeline
	for _, pipeline := range tm.pipelines {
		if pipeline.Enabled() {
			return pipeline
		}
	}

	return nil
}
