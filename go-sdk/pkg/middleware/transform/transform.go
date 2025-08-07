package transform

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
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

// BaseTransformer provides a base implementation for transformers
type BaseTransformer struct {
	name    string
	tType   TransformationType
	enabled bool
	config  map[string]interface{}
	mu      sync.RWMutex
}

// NewBaseTransformer creates a new base transformer
func NewBaseTransformer(name string, tType TransformationType) *BaseTransformer {
	return &BaseTransformer{
		name:    name,
		tType:   tType,
		enabled: true,
		config:  make(map[string]interface{}),
	}
}

// Name returns the transformer name
func (bt *BaseTransformer) Name() string {
	return bt.name
}

// Type returns the transformation type
func (bt *BaseTransformer) Type() TransformationType {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	return bt.tType
}

// Configure configures the transformer
func (bt *BaseTransformer) Configure(config map[string]interface{}) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	if enabled, ok := config["enabled"].(bool); ok {
		bt.enabled = enabled
	}

	bt.config = config
	return nil
}

// Enabled returns whether the transformer is enabled
func (bt *BaseTransformer) Enabled() bool {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	return bt.enabled
}

// GetConfig returns configuration value
func (bt *BaseTransformer) GetConfig(key string) (interface{}, bool) {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	value, exists := bt.config[key]
	return value, exists
}

// Transform default implementation (should be overridden)
func (bt *BaseTransformer) Transform(ctx context.Context, data interface{}) (interface{}, error) {
	return data, nil
}

// TransformRequest default implementation (should be overridden)
func (bt *BaseTransformer) TransformRequest(ctx context.Context, req *Request) error {
	return nil
}

// TransformResponse default implementation (should be overridden)
func (bt *BaseTransformer) TransformResponse(ctx context.Context, resp *Response) error {
	return nil
}

// CompressionTransformer handles request/response compression
type CompressionTransformer struct {
	*BaseTransformer
	algorithm string
	level     int
}

// NewCompressionTransformer creates a new compression transformer
func NewCompressionTransformer(algorithm string, level int) *CompressionTransformer {
	return &CompressionTransformer{
		BaseTransformer: NewBaseTransformer("compression", TransformationBoth),
		algorithm:       algorithm,
		level:           level,
	}
}

// TransformRequest compresses request body if applicable
func (ct *CompressionTransformer) TransformRequest(ctx context.Context, req *Request) error {
	if !ct.Enabled() || req.Body == nil {
		return nil
	}

	// Check if compression is requested
	if req.Headers["Content-Encoding"] == ct.algorithm {
		return nil // Already compressed
	}

	// Serialize body to bytes
	var bodyBytes []byte
	var err error

	switch body := req.Body.(type) {
	case []byte:
		bodyBytes = body
	case string:
		bodyBytes = []byte(body)
	default:
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	// Compress the body
	compressed, err := ct.compress(bodyBytes)
	if err != nil {
		return fmt.Errorf("failed to compress request body: %w", err)
	}

	// Update request
	req.Body = compressed
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	req.Headers["Content-Encoding"] = ct.algorithm
	req.Headers["Content-Length"] = fmt.Sprintf("%d", len(compressed))

	return nil
}

// TransformResponse decompresses response body if needed
func (ct *CompressionTransformer) TransformResponse(ctx context.Context, resp *Response) error {
	if !ct.Enabled() || resp.Body == nil {
		return nil
	}

	// Check if response is compressed
	if resp.Headers["Content-Encoding"] != ct.algorithm {
		return nil
	}

	// Convert body to bytes
	var bodyBytes []byte
	switch body := resp.Body.(type) {
	case []byte:
		bodyBytes = body
	case string:
		bodyBytes = []byte(body)
	default:
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal response body: %w", err)
		}
	}

	// Decompress the body
	decompressed, err := ct.decompress(bodyBytes)
	if err != nil {
		return fmt.Errorf("failed to decompress response body: %w", err)
	}

	// Update response
	resp.Body = decompressed
	if resp.Headers == nil {
		resp.Headers = make(map[string]string)
	}
	delete(resp.Headers, "Content-Encoding")
	resp.Headers["Content-Length"] = fmt.Sprintf("%d", len(decompressed))

	return nil
}

// compress compresses data using the configured algorithm
func (ct *CompressionTransformer) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	switch ct.algorithm {
	case "gzip":
		writer, err := gzip.NewWriterLevel(&buf, ct.level)
		if err != nil {
			return nil, err
		}
		defer writer.Close()

		_, err = writer.Write(data)
		if err != nil {
			return nil, err
		}

		err = writer.Close()
		if err != nil {
			return nil, err
		}

	case "deflate":
		writer, err := zlib.NewWriterLevel(&buf, ct.level)
		if err != nil {
			return nil, err
		}
		defer writer.Close()

		_, err = writer.Write(data)
		if err != nil {
			return nil, err
		}

		err = writer.Close()
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported compression algorithm: %s", ct.algorithm)
	}

	return buf.Bytes(), nil
}

// decompress decompresses data using the configured algorithm
func (ct *CompressionTransformer) decompress(data []byte) ([]byte, error) {
	buf := bytes.NewReader(data)

	switch ct.algorithm {
	case "gzip":
		reader, err := gzip.NewReader(buf)
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		return io.ReadAll(reader)

	case "deflate":
		reader, err := zlib.NewReader(buf)
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		return io.ReadAll(reader)

	default:
		return nil, fmt.Errorf("unsupported compression algorithm: %s", ct.algorithm)
	}
}

// SanitizationTransformer removes sensitive data from requests/responses
type SanitizationTransformer struct {
	*BaseTransformer
	sensitiveFields []string
	replacement     string
	patterns        []*regexp.Regexp
}

// NewSanitizationTransformer creates a new sanitization transformer
func NewSanitizationTransformer(sensitiveFields []string, replacement string) *SanitizationTransformer {
	patterns := make([]*regexp.Regexp, len(sensitiveFields))
	for i, field := range sensitiveFields {
		// Create case-insensitive regex pattern
		pattern, err := regexp.Compile(fmt.Sprintf(`(?i)%s`, regexp.QuoteMeta(field)))
		if err == nil {
			patterns[i] = pattern
		}
	}

	return &SanitizationTransformer{
		BaseTransformer: NewBaseTransformer("sanitization", TransformationBoth),
		sensitiveFields: sensitiveFields,
		replacement:     replacement,
		patterns:        patterns,
	}
}

// TransformRequest sanitizes sensitive data in request
func (st *SanitizationTransformer) TransformRequest(ctx context.Context, req *Request) error {
	if !st.Enabled() {
		return nil
	}

	// Sanitize headers
	req.Headers = st.sanitizeMap(req.Headers)

	// Sanitize body
	if req.Body != nil {
		req.Body = st.sanitizeData(req.Body)
	}

	return nil
}

// TransformResponse sanitizes sensitive data in response
func (st *SanitizationTransformer) TransformResponse(ctx context.Context, resp *Response) error {
	if !st.Enabled() {
		return nil
	}

	// Sanitize headers
	resp.Headers = st.sanitizeMap(resp.Headers)

	// Sanitize body
	if resp.Body != nil {
		resp.Body = st.sanitizeData(resp.Body)
	}

	return nil
}

// sanitizeMap sanitizes sensitive fields in a map
func (st *SanitizationTransformer) sanitizeMap(data map[string]string) map[string]string {
	if data == nil {
		return nil
	}

	result := make(map[string]string)
	for k, v := range data {
		if st.isSensitiveField(k) {
			result[k] = st.replacement
		} else {
			result[k] = st.sanitizeString(v)
		}
	}

	return result
}

// sanitizeData recursively sanitizes sensitive data
func (st *SanitizationTransformer) sanitizeData(data interface{}) interface{} {
	if data == nil {
		return nil
	}

	value := reflect.ValueOf(data)
	switch value.Kind() {
	case reflect.Map:
		return st.sanitizeMapInterface(data)
	case reflect.Slice, reflect.Array:
		return st.sanitizeSlice(data)
	case reflect.String:
		return st.sanitizeString(data.(string))
	default:
		return data
	}
}

// sanitizeMapInterface sanitizes a map[string]interface{}
func (st *SanitizationTransformer) sanitizeMapInterface(data interface{}) interface{} {
	if dataMap, ok := data.(map[string]interface{}); ok {
		result := make(map[string]interface{})
		for k, v := range dataMap {
			if st.isSensitiveField(k) {
				result[k] = st.replacement
			} else {
				result[k] = st.sanitizeData(v)
			}
		}
		return result
	}

	return data
}

// sanitizeSlice sanitizes a slice
func (st *SanitizationTransformer) sanitizeSlice(data interface{}) interface{} {
	value := reflect.ValueOf(data)
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return data
	}

	result := make([]interface{}, value.Len())
	for i := 0; i < value.Len(); i++ {
		result[i] = st.sanitizeData(value.Index(i).Interface())
	}

	return result
}

// sanitizeString sanitizes sensitive patterns in strings
func (st *SanitizationTransformer) sanitizeString(s string) string {
	result := s
	for _, pattern := range st.patterns {
		if pattern != nil {
			result = pattern.ReplaceAllString(result, st.replacement)
		}
	}
	return result
}

// isSensitiveField checks if a field name is sensitive
func (st *SanitizationTransformer) isSensitiveField(field string) bool {
	fieldLower := strings.ToLower(field)
	for _, sensitive := range st.sensitiveFields {
		if strings.ToLower(sensitive) == fieldLower {
			return true
		}
	}
	return false
}

// ValidationTransformer validates request/response data
type ValidationTransformer struct {
	*BaseTransformer
	requestValidators  []Validator
	responseValidators []Validator
}

// Validator interface for data validation
type Validator interface {
	// Validate validates data and returns error if invalid
	Validate(ctx context.Context, data interface{}) error

	// Name returns validator name
	Name() string
}

// JSONSchemaValidator validates data against JSON schema (simplified)
type JSONSchemaValidator struct {
	name   string
	schema map[string]interface{}
}

// NewJSONSchemaValidator creates a new JSON schema validator
func NewJSONSchemaValidator(name string, schema map[string]interface{}) *JSONSchemaValidator {
	return &JSONSchemaValidator{
		name:   name,
		schema: schema,
	}
}

// Name returns validator name
func (jsv *JSONSchemaValidator) Name() string {
	return jsv.name
}

// Validate validates data against schema (simplified implementation)
func (jsv *JSONSchemaValidator) Validate(ctx context.Context, data interface{}) error {
	// This is a simplified validation - in production, use a proper JSON schema library
	if data == nil {
		return fmt.Errorf("data cannot be nil")
	}

	// Basic type validation
	if expectedType, ok := jsv.schema["type"].(string); ok {
		actualType := reflect.TypeOf(data).Kind().String()
		if actualType != expectedType && !jsv.isCompatibleType(expectedType, actualType) {
			return fmt.Errorf("expected type %s, got %s", expectedType, actualType)
		}
	}

	// Required fields validation
	if requiredFields, ok := jsv.schema["required"].([]interface{}); ok {
		if dataMap, ok := data.(map[string]interface{}); ok {
			for _, field := range requiredFields {
				if fieldName, ok := field.(string); ok {
					if _, exists := dataMap[fieldName]; !exists {
						return fmt.Errorf("required field '%s' is missing", fieldName)
					}
				}
			}
		}
	}

	return nil
}

// isCompatibleType checks if types are compatible
func (jsv *JSONSchemaValidator) isCompatibleType(expected, actual string) bool {
	compatibleTypes := map[string][]string{
		"object": {"map"},
		"array":  {"slice"},
		"string": {"string"},
		"number": {"int", "int64", "float64", "float32"},
		"boolean": {"bool"},
	}

	if compatible, ok := compatibleTypes[expected]; ok {
		for _, t := range compatible {
			if t == actual {
				return true
			}
		}
	}

	return false
}

// NewValidationTransformer creates a new validation transformer
func NewValidationTransformer() *ValidationTransformer {
	return &ValidationTransformer{
		BaseTransformer:    NewBaseTransformer("validation", TransformationBoth),
		requestValidators:  make([]Validator, 0),
		responseValidators: make([]Validator, 0),
	}
}

// AddRequestValidator adds a request validator
func (vt *ValidationTransformer) AddRequestValidator(validator Validator) {
	vt.requestValidators = append(vt.requestValidators, validator)
}

// AddResponseValidator adds a response validator
func (vt *ValidationTransformer) AddResponseValidator(validator Validator) {
	vt.responseValidators = append(vt.responseValidators, validator)
}

// TransformRequest validates request data
func (vt *ValidationTransformer) TransformRequest(ctx context.Context, req *Request) error {
	if !vt.Enabled() {
		return nil
	}

	for _, validator := range vt.requestValidators {
		if err := validator.Validate(ctx, req.Body); err != nil {
			return fmt.Errorf("request validation failed in %s: %w", validator.Name(), err)
		}
	}

	return nil
}

// TransformResponse validates response data
func (vt *ValidationTransformer) TransformResponse(ctx context.Context, resp *Response) error {
	if !vt.Enabled() {
		return nil
	}

	for _, validator := range vt.responseValidators {
		if err := validator.Validate(ctx, resp.Body); err != nil {
			return fmt.Errorf("response validation failed in %s: %w", validator.Name(), err)
		}
	}

	return nil
}

// TransformationConfig represents transformation middleware configuration
type TransformationConfig struct {
	Pipelines       []PipelineConfig `json:"pipelines" yaml:"pipelines"`
	DefaultPipeline string           `json:"default_pipeline" yaml:"default_pipeline"`
	SkipPaths       []string         `json:"skip_paths" yaml:"skip_paths"`
	SkipHealthCheck bool             `json:"skip_health_check" yaml:"skip_health_check"`
}

// PipelineConfig represents pipeline configuration
type PipelineConfig struct {
	Name         string                   `json:"name" yaml:"name"`
	Enabled      bool                     `json:"enabled" yaml:"enabled"`
	Transformers []TransformerConfig      `json:"transformers" yaml:"transformers"`
	Conditions   map[string]interface{}   `json:"conditions" yaml:"conditions"`
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
	config       *TransformationConfig
	pipelines    map[string]*Pipeline
	enabled      bool
	priority     int
	skipMap      map[string]bool
	mu           sync.RWMutex
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