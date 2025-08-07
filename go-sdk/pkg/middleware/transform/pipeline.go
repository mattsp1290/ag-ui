package transform

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Local type definitions to avoid circular imports
type Request struct {
	ID        string                 `json:"id"`
	Method    string                 `json:"method"`
	Path      string                 `json:"path"`
	Headers   map[string]string      `json:"headers"`
	Body      interface{}            `json:"body"`
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
}

type Response struct {
	ID         string                 `json:"id"`
	StatusCode int                    `json:"status_code"`
	Headers    map[string]string      `json:"headers"`
	Body       interface{}            `json:"body"`
	Error      error                  `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata"`
	Timestamp  time.Time              `json:"timestamp"`
	Duration   time.Duration          `json:"duration"`
}

type NextHandler func(ctx context.Context, req *Request) (*Response, error)

// TransformationType represents different types of transformations
type TransformationType string

const (
	TransformationRequest  TransformationType = "request"
	TransformationResponse TransformationType = "response"
	TransformationBoth     TransformationType = "both"
)

// Transformer interface defines transformation operations
type Transformer interface {
	// Name returns the transformer name
	Name() string

	// Transform applies transformation to data
	Transform(ctx context.Context, data interface{}) (interface{}, error)

	// TransformRequest transforms request data
	TransformRequest(ctx context.Context, req *Request) error

	// TransformResponse transforms response data
	TransformResponse(ctx context.Context, resp *Response) error

	// Type returns the transformation type
	Type() TransformationType

	// Configure configures the transformer
	Configure(config map[string]interface{}) error

	// Enabled returns whether the transformer is enabled
	Enabled() bool
}

// Pipeline represents a transformation pipeline
type Pipeline struct {
	name         string
	transformers []Transformer
	enabled      bool
	mu           sync.RWMutex
}

// NewPipeline creates a new transformation pipeline
func NewPipeline(name string) *Pipeline {
	return &Pipeline{
		name:         name,
		transformers: make([]Transformer, 0),
		enabled:      true,
	}
}

// Name returns the pipeline name
func (p *Pipeline) Name() string {
	return p.name
}

// AddTransformer adds a transformer to the pipeline
func (p *Pipeline) AddTransformer(transformer Transformer) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if transformer != nil && transformer.Enabled() {
		p.transformers = append(p.transformers, transformer)
	}
}

// RemoveTransformer removes a transformer from the pipeline
func (p *Pipeline) RemoveTransformer(name string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, transformer := range p.transformers {
		if transformer.Name() == name {
			p.transformers = append(p.transformers[:i], p.transformers[i+1:]...)
			return true
		}
	}
	return false
}

// Transform applies all transformations in the pipeline
func (p *Pipeline) Transform(ctx context.Context, data interface{}) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.enabled {
		return data, nil
	}

	result := data
	for _, transformer := range p.transformers {
		if !transformer.Enabled() {
			continue
		}

		var err error
		result, err = transformer.Transform(ctx, result)
		if err != nil {
			return nil, fmt.Errorf("transformation failed in %s: %w", transformer.Name(), err)
		}
	}

	return result, nil
}

// TransformRequest applies request transformations
func (p *Pipeline) TransformRequest(ctx context.Context, req *Request) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.enabled {
		return nil
	}

	for _, transformer := range p.transformers {
		if !transformer.Enabled() {
			continue
		}

		if transformer.Type() == TransformationRequest || transformer.Type() == TransformationBoth {
			if err := transformer.TransformRequest(ctx, req); err != nil {
				return fmt.Errorf("request transformation failed in %s: %w", transformer.Name(), err)
			}
		}
	}

	return nil
}

// TransformResponse applies response transformations
func (p *Pipeline) TransformResponse(ctx context.Context, resp *Response) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.enabled {
		return nil
	}

	for _, transformer := range p.transformers {
		if !transformer.Enabled() {
			continue
		}

		if transformer.Type() == TransformationResponse || transformer.Type() == TransformationBoth {
			if err := transformer.TransformResponse(ctx, resp); err != nil {
				return fmt.Errorf("response transformation failed in %s: %w", transformer.Name(), err)
			}
		}
	}

	return nil
}

// Enabled returns whether the pipeline is enabled
func (p *Pipeline) Enabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.enabled
}

// SetEnabled sets the pipeline enabled state
func (p *Pipeline) SetEnabled(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enabled = enabled
}

// ListTransformers returns all transformer names in the pipeline
func (p *Pipeline) ListTransformers() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	names := make([]string, len(p.transformers))
	for i, transformer := range p.transformers {
		names[i] = transformer.Name()
	}
	return names
}
