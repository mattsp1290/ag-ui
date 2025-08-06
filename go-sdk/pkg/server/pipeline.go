// Package server provides request/response processing pipeline for the AG-UI Go SDK.
// This pipeline implements high-performance request processing with comprehensive error handling,
// pluggable transformation stages, and metrics/monitoring integration.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
	pkgerrors "github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/transport"
)

// ==============================================================================
// PIPELINE INTERFACES
// ==============================================================================

// PipelineHandler represents a function that handles HTTP requests in the pipeline.
type PipelineHandler func(ctx context.Context, req *http.Request) (*http.Response, error)

// RequestProcessor defines the interface for processing incoming requests.
type RequestProcessor interface {
	// Process processes a raw HTTP request through the pipeline
	Process(ctx context.Context, req *http.Request) (*PipelineResponse, error)
	
	// ProcessWithStages processes a request through specific stages
	ProcessWithStages(ctx context.Context, req *http.Request, stages []ProcessingStage) (*PipelineResponse, error)
	
	// AddStage adds a processing stage to the pipeline
	AddStage(stage ProcessingStage)
	
	// RemoveStage removes a processing stage from the pipeline
	RemoveStage(name string) error
	
	// GetStages returns all configured processing stages
	GetStages() []ProcessingStage
}

// ProcessingStage defines a single stage in the request processing pipeline.
type ProcessingStage interface {
	// Name returns the stage name for identification
	Name() string
	
	// Priority returns the stage priority (higher executes first)
	Priority() int
	
	// Process processes the request context
	Process(ctx context.Context, pipelineCtx *PipelineContext) error
	
	// ShouldProcess returns true if this stage should process the current context
	ShouldProcess(ctx context.Context, pipelineCtx *PipelineContext) bool
	
	// OnError handles errors that occur during processing
	OnError(ctx context.Context, pipelineCtx *PipelineContext, err error) error
}

// ResponseProcessor defines the interface for processing outgoing responses.
type ResponseProcessor interface {
	// ProcessResponse processes the response through the pipeline
	ProcessResponse(ctx context.Context, resp *PipelineResponse) error
	
	// SerializeResponse serializes the response to the appropriate format
	SerializeResponse(ctx context.Context, resp *PipelineResponse, writer io.Writer) error
	
	// AddTransformer adds a response transformer
	AddTransformer(transformer ResponseTransformer)
	
	// RemoveTransformer removes a response transformer
	RemoveTransformer(name string) error
}

// ResponseTransformer defines the interface for transforming responses.
type ResponseTransformer interface {
	// Name returns the transformer name
	Name() string
	
	// Priority returns the transformer priority
	Priority() int
	
	// Transform transforms the response
	Transform(ctx context.Context, resp *PipelineResponse) error
	
	// ShouldTransform returns true if this transformer should process the response
	ShouldTransform(ctx context.Context, resp *PipelineResponse) bool
}

// PipelineValidator validates requests and responses.
type PipelineValidator interface {
	// ValidateRequest validates an incoming request
	ValidateRequest(ctx context.Context, req *PipelineRequest) error
	
	// ValidateResponse validates an outgoing response
	ValidateResponse(ctx context.Context, resp *PipelineResponse) error
}

// ==============================================================================
// DATA STRUCTURES
// ==============================================================================

// PipelineRequest represents a request being processed through the pipeline.
type PipelineRequest struct {
	// Original HTTP request
	HTTPRequest *http.Request
	
	// Request metadata
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Headers     map[string]string      `json:"headers"`
	QueryParams map[string]string      `json:"query_params"`
	
	// Request body and content
	Body        []byte                 `json:"body,omitempty"`
	ContentType string                 `json:"content_type"`
	ContentLength int64                `json:"content_length"`
	
	// AG-UI specific fields
	AgentName   string                 `json:"agent_name,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	Event       events.Event           `json:"event,omitempty"`
	Message     messages.Message       `json:"message,omitempty"`
	
	// Processing metadata
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	SpanID      string                 `json:"span_id,omitempty"`
	
	// Security context
	UserID      string                 `json:"user_id,omitempty"`
	Permissions []string               `json:"permissions,omitempty"`
	AuthContext map[string]interface{} `json:"auth_context,omitempty"`
}

// PipelineResponse represents a response being processed through the pipeline.
type PipelineResponse struct {
	// Response metadata
	ID          string                 `json:"id"`
	RequestID   string                 `json:"request_id"`
	Timestamp   time.Time              `json:"timestamp"`
	StatusCode  int                    `json:"status_code"`
	Headers     map[string]string      `json:"headers"`
	
	// Response data
	Body        []byte                 `json:"body,omitempty"`
	ContentType string                 `json:"content_type"`
	ContentLength int64                `json:"content_length"`
	
	// AG-UI specific fields
	Event       events.Event           `json:"event,omitempty"`
	Message     messages.Message       `json:"message,omitempty"`
	Data        interface{}            `json:"data,omitempty"`
	
	// Processing metadata
	ProcessingTime time.Duration         `json:"processing_time"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	
	// Error information
	Error       error                  `json:"error,omitempty"`
	ErrorCode   string                 `json:"error_code,omitempty"`
	ErrorMessage string                `json:"error_message,omitempty"`
}

// PipelineContext holds the context for request processing.
type PipelineContext struct {
	// Core context
	Context     context.Context
	
	// Request and response
	Request     *PipelineRequest
	Response    *PipelineResponse
	
	// Processing state
	Stage       string
	StageIndex  int
	Completed   bool
	Failed      bool
	
	// Performance tracking
	StartTime   time.Time
	StageTimings map[string]time.Duration
	
	// Processing data
	ProcessingData map[string]interface{}
	
	// Error handling
	Errors      []error
	
	// Security context
	Authenticated bool
	Authorized    bool
	
	// Transport and encoding
	Transport   transport.Transport
	Encoder     encoding.Codec
	Decoder     encoding.Codec
}

// PipelineConfig contains configuration for the request processing pipeline.
type PipelineConfig struct {
	// Performance settings
	MaxConcurrentRequests int           `json:"max_concurrent_requests" yaml:"max_concurrent_requests"`
	RequestTimeout        time.Duration `json:"request_timeout" yaml:"request_timeout"`
	MaxRequestSize        int64         `json:"max_request_size" yaml:"max_request_size"`
	MaxResponseSize       int64         `json:"max_response_size" yaml:"max_response_size"`
	
	// Pipeline behavior
	EnableAsyncProcessing bool          `json:"enable_async_processing" yaml:"enable_async_processing"`
	EnableStageParallel   bool          `json:"enable_stage_parallel" yaml:"enable_stage_parallel"`
	FailFast              bool          `json:"fail_fast" yaml:"fail_fast"`
	
	// Error handling
	EnableRecovery        bool          `json:"enable_recovery" yaml:"enable_recovery"`
	EnableErrorDetails    bool          `json:"enable_error_details" yaml:"enable_error_details"`
	MaxRetries            int           `json:"max_retries" yaml:"max_retries"`
	RetryDelay            time.Duration `json:"retry_delay" yaml:"retry_delay"`
	
	// Validation
	EnableRequestValidation  bool       `json:"enable_request_validation" yaml:"enable_request_validation"`
	EnableResponseValidation bool       `json:"enable_response_validation" yaml:"enable_response_validation"`
	StrictValidation         bool       `json:"strict_validation" yaml:"strict_validation"`
	
	// Monitoring
	EnableMetrics         bool          `json:"enable_metrics" yaml:"enable_metrics"`
	EnableTracing         bool          `json:"enable_tracing" yaml:"enable_tracing"`
	EnableDetailedLogs    bool          `json:"enable_detailed_logs" yaml:"enable_detailed_logs"`
	
	// Encoding
	DefaultContentType    string        `json:"default_content_type" yaml:"default_content_type"`
	SupportedContentTypes []string      `json:"supported_content_types" yaml:"supported_content_types"`
	EnableCompression     bool          `json:"enable_compression" yaml:"enable_compression"`
	
	// Security
	EnableAuthentication  bool          `json:"enable_authentication" yaml:"enable_authentication"`
	EnableAuthorization   bool          `json:"enable_authorization" yaml:"enable_authorization"`
	EnableRateLimit       bool          `json:"enable_rate_limit" yaml:"enable_rate_limit"`
	RateLimitPerSecond    int           `json:"rate_limit_per_second" yaml:"rate_limit_per_second"`
}

// PipelineMetrics contains metrics for the processing pipeline.
type PipelineMetrics struct {
	// Request metrics
	RequestCounter       metric.Int64Counter
	RequestDuration      metric.Float64Histogram
	RequestSize          metric.Int64Histogram
	ResponseSize         metric.Int64Histogram
	ActiveRequests       metric.Int64UpDownCounter
	
	// Stage metrics
	StageCounter         metric.Int64Counter
	StageDuration        metric.Float64Histogram
	StageErrors          metric.Int64Counter
	
	// Error metrics
	ErrorCounter         metric.Int64Counter
	ValidationErrors     metric.Int64Counter
	TimeoutErrors        metric.Int64Counter
	
	// Performance metrics
	ThroughputCounter    metric.Float64Counter
	LatencyPercentiles   metric.Float64Histogram
	ConcurrencyGauge     metric.Int64Gauge
}

// ==============================================================================
// PIPELINE IMPLEMENTATION
// ==============================================================================

// RequestProcessingPipeline implements the RequestProcessor interface.
type RequestProcessingPipeline struct {
	// Configuration
	config *PipelineConfig
	
	// Processing stages
	stages    []ProcessingStage
	stagesMu  sync.RWMutex
	
	// Response processing
	responseProcessor *ResponseProcessingPipeline
	
	// Components
	validator    PipelineValidator
	codecFactory encoding.CodecFactory
	
	// Metrics and monitoring
	metrics *PipelineMetrics
	tracer  trace.Tracer
	logger  *logrus.Logger
	
	// Performance tracking
	activeRequests int64
	totalRequests  int64
	totalErrors    int64
	
	// Synchronization
	mu sync.RWMutex
}

// NewRequestProcessingPipeline creates a new request processing pipeline.
func NewRequestProcessingPipeline(config *PipelineConfig) (*RequestProcessingPipeline, error) {
	if config == nil {
		config = DefaultPipelineConfig()
	}
	
	if err := validatePipelineConfig(config); err != nil {
		return nil, pkgerrors.NewValidationError("invalid_config", "invalid pipeline configuration").WithCause(err)
	}
	
	// Create logger
	logger := logrus.New()
	if config.EnableDetailedLogs {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}
	
	// Create tracer
	var tracer trace.Tracer
	if config.EnableTracing {
		tracer = otel.Tracer("ag-ui-pipeline")
	}
	
	// Create codec factory
	codecFactory := encoding.NewDefaultCodecFactory()
	
	// Register default codecs
	if err := registerDefaultCodecs(codecFactory); err != nil {
		return nil, pkgerrors.NewStateError("codec_registration_failed", "failed to register default codecs").WithCause(err)
	}
	
	pipeline := &RequestProcessingPipeline{
		config:            config,
		stages:            make([]ProcessingStage, 0),
		responseProcessor: NewResponseProcessingPipeline(config),
		validator:         NewDefaultPipelineValidator(config),
		codecFactory:      codecFactory,
		logger:            logger,
		tracer:            tracer,
	}
	
	// Initialize metrics
	if config.EnableMetrics {
		metrics, err := initializePipelineMetrics()
		if err != nil {
			return nil, pkgerrors.NewStateError("metrics_init_failed", "failed to initialize pipeline metrics").WithCause(err)
		}
		pipeline.metrics = metrics
	}
	
	// Register default stages
	if err := pipeline.registerDefaultStages(); err != nil {
		return nil, pkgerrors.NewStateError("stage_registration_failed", "failed to register default stages").WithCause(err)
	}
	
	return pipeline, nil
}

// Process processes a raw HTTP request through the pipeline.
func (p *RequestProcessingPipeline) Process(ctx context.Context, req *http.Request) (*PipelineResponse, error) {
	// Create pipeline context
	pipelineCtx, err := p.createPipelineContext(ctx, req)
	if err != nil {
		return nil, err
	}
	
	// Start tracing
	if p.tracer != nil {
		var span trace.Span
		ctx, span = p.tracer.Start(ctx, "pipeline.process",
			trace.WithAttributes(
				attribute.String("request.id", pipelineCtx.Request.ID),
				attribute.String("request.method", pipelineCtx.Request.Method),
				attribute.String("request.path", pipelineCtx.Request.Path),
			),
		)
		defer span.End()
		pipelineCtx.Context = ctx
	}
	
	// Update metrics
	if p.metrics != nil {
		atomic.AddInt64(&p.activeRequests, 1)
		p.metrics.ActiveRequests.Add(ctx, 1)
		defer func() {
			atomic.AddInt64(&p.activeRequests, -1)
			p.metrics.ActiveRequests.Add(ctx, -1)
		}()
		
		atomic.AddInt64(&p.totalRequests, 1)
		p.metrics.RequestCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("method", pipelineCtx.Request.Method),
				attribute.String("path", pipelineCtx.Request.Path),
			),
		)
	}
	
	// Check rate limits
	if p.config.EnableRateLimit {
		if err := p.checkRateLimit(ctx, pipelineCtx); err != nil {
			return p.createErrorResponse(pipelineCtx, err, http.StatusTooManyRequests), nil
		}
	}
	
	// Check concurrent request limits
	if p.config.MaxConcurrentRequests > 0 && atomic.LoadInt64(&p.activeRequests) > int64(p.config.MaxConcurrentRequests) {
		err := pkgerrors.NewStateError("too_many_requests", "maximum concurrent requests exceeded").
			WithDetail("max_concurrent", p.config.MaxConcurrentRequests).
			WithDetail("current_active", atomic.LoadInt64(&p.activeRequests))
		return p.createErrorResponse(pipelineCtx, err, http.StatusTooManyRequests), nil
	}
	
	// Validate request
	if p.config.EnableRequestValidation {
		if err := p.validator.ValidateRequest(ctx, pipelineCtx.Request); err != nil {
			return p.createErrorResponse(pipelineCtx, err, http.StatusBadRequest), nil
		}
	}
	
	// Process through stages
	if err := p.processStages(ctx, pipelineCtx); err != nil {
		if p.config.EnableRecovery {
			p.logger.WithError(err).Error("Pipeline processing failed, attempting recovery")
			if recoveryErr := p.recoverFromError(ctx, pipelineCtx, err); recoveryErr == nil {
				// Recovery successful, continue processing
			} else {
				return p.createErrorResponse(pipelineCtx, err, http.StatusInternalServerError), nil
			}
		} else {
			return p.createErrorResponse(pipelineCtx, err, http.StatusInternalServerError), nil
		}
	}
	
	// Validate response
	if p.config.EnableResponseValidation {
		if err := p.validator.ValidateResponse(ctx, pipelineCtx.Response); err != nil {
			return p.createErrorResponse(pipelineCtx, err, http.StatusInternalServerError), nil
		}
	}
	
	// Process response
	if err := p.responseProcessor.ProcessResponse(ctx, pipelineCtx.Response); err != nil {
		return p.createErrorResponse(pipelineCtx, err, http.StatusInternalServerError), nil
	}
	
	// Update final metrics
	if p.metrics != nil {
		duration := time.Since(pipelineCtx.StartTime)
		p.metrics.RequestDuration.Record(ctx, duration.Seconds())
		
		if pipelineCtx.Request.ContentLength > 0 {
			p.metrics.RequestSize.Record(ctx, pipelineCtx.Request.ContentLength)
		}
		
		if pipelineCtx.Response.ContentLength > 0 {
			p.metrics.ResponseSize.Record(ctx, pipelineCtx.Response.ContentLength)
		}
	}
	
	// Log completion
	if p.config.EnableDetailedLogs {
		p.logger.WithFields(logrus.Fields{
			"request_id":      pipelineCtx.Request.ID,
			"method":          pipelineCtx.Request.Method,
			"path":            pipelineCtx.Request.Path,
			"status_code":     pipelineCtx.Response.StatusCode,
			"processing_time": time.Since(pipelineCtx.StartTime),
			"stages":          len(p.stages),
		}).Info("Request processing completed")
	}
	
	return pipelineCtx.Response, nil
}

// ProcessWithStages processes a request through specific stages.
func (p *RequestProcessingPipeline) ProcessWithStages(ctx context.Context, req *http.Request, stages []ProcessingStage) (*PipelineResponse, error) {
	// Create pipeline context
	pipelineCtx, err := p.createPipelineContext(ctx, req)
	if err != nil {
		return nil, err
	}
	
	// Process through specified stages
	for i, stage := range stages {
		pipelineCtx.Stage = stage.Name()
		pipelineCtx.StageIndex = i
		
		if !stage.ShouldProcess(ctx, pipelineCtx) {
			continue
		}
		
		stageStart := time.Now()
		
		if err := stage.Process(ctx, pipelineCtx); err != nil {
			if stageErr := stage.OnError(ctx, pipelineCtx, err); stageErr != nil {
				return p.createErrorResponse(pipelineCtx, stageErr, http.StatusInternalServerError), nil
			}
			return p.createErrorResponse(pipelineCtx, err, http.StatusInternalServerError), nil
		}
		
		// Record stage timing
		stageDuration := time.Since(stageStart)
		if pipelineCtx.StageTimings == nil {
			pipelineCtx.StageTimings = make(map[string]time.Duration)
		}
		pipelineCtx.StageTimings[stage.Name()] = stageDuration
		
		// Update stage metrics
		if p.metrics != nil {
			p.metrics.StageDuration.Record(ctx, stageDuration.Seconds(),
				metric.WithAttributes(attribute.String("stage", stage.Name())),
			)
		}
	}
	
	return pipelineCtx.Response, nil
}

// AddStage adds a processing stage to the pipeline.
func (p *RequestProcessingPipeline) AddStage(stage ProcessingStage) {
	p.stagesMu.Lock()
	defer p.stagesMu.Unlock()
	
	p.stages = append(p.stages, stage)
	
	// Sort stages by priority (higher priority first)
	p.sortStages()
}

// RemoveStage removes a processing stage from the pipeline.
func (p *RequestProcessingPipeline) RemoveStage(name string) error {
	p.stagesMu.Lock()
	defer p.stagesMu.Unlock()
	
	for i, stage := range p.stages {
		if stage.Name() == name {
			p.stages = append(p.stages[:i], p.stages[i+1:]...)
			return nil
		}
	}
	
	return pkgerrors.NewStateError("stage_not_found", "processing stage not found").
		WithDetail("stage_name", name)
}

// GetStages returns all configured processing stages.
func (p *RequestProcessingPipeline) GetStages() []ProcessingStage {
	p.stagesMu.RLock()
	defer p.stagesMu.RUnlock()
	
	stages := make([]ProcessingStage, len(p.stages))
	copy(stages, p.stages)
	return stages
}

// ==============================================================================
// RESPONSE PROCESSING PIPELINE
// ==============================================================================

// ResponseProcessingPipeline handles response processing.
type ResponseProcessingPipeline struct {
	config       *PipelineConfig
	transformers []ResponseTransformer
	mu           sync.RWMutex
}

// NewResponseProcessingPipeline creates a new response processing pipeline.
func NewResponseProcessingPipeline(config *PipelineConfig) *ResponseProcessingPipeline {
	return &ResponseProcessingPipeline{
		config:       config,
		transformers: make([]ResponseTransformer, 0),
	}
}

// ProcessResponse processes the response through the pipeline.
func (rp *ResponseProcessingPipeline) ProcessResponse(ctx context.Context, resp *PipelineResponse) error {
	rp.mu.RLock()
	transformers := make([]ResponseTransformer, len(rp.transformers))
	copy(transformers, rp.transformers)
	rp.mu.RUnlock()
	
	for _, transformer := range transformers {
		if transformer.ShouldTransform(ctx, resp) {
			if err := transformer.Transform(ctx, resp); err != nil {
				return pkgerrors.NewStateError("response_transform_failed", "response transformation failed").
					WithCause(err).
					WithDetail("transformer", transformer.Name())
			}
		}
	}
	
	return nil
}

// SerializeResponse serializes the response to the appropriate format.
func (rp *ResponseProcessingPipeline) SerializeResponse(ctx context.Context, resp *PipelineResponse, writer io.Writer) error {
	// Determine content type
	contentType := resp.ContentType
	if contentType == "" {
		contentType = rp.config.DefaultContentType
		if contentType == "" {
			contentType = "application/json"
		}
	}
	
	// If body already exists, write it directly
	if len(resp.Body) > 0 {
		_, err := writer.Write(resp.Body)
		return err
	}
	
	// Otherwise, serialize the data
	if resp.Data != nil {
		if contentType == "application/json" {
			return rp.serializeAsJSON(resp.Data, writer)
		}
		// Add other content type serialization as needed
	}
	
	return nil
}

// AddTransformer adds a response transformer.
func (rp *ResponseProcessingPipeline) AddTransformer(transformer ResponseTransformer) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	
	rp.transformers = append(rp.transformers, transformer)
	
	// Sort transformers by priority
	rp.sortTransformers()
}

// RemoveTransformer removes a response transformer.
func (rp *ResponseProcessingPipeline) RemoveTransformer(name string) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	
	for i, transformer := range rp.transformers {
		if transformer.Name() == name {
			rp.transformers = append(rp.transformers[:i], rp.transformers[i+1:]...)
			return nil
		}
	}
	
	return pkgerrors.NewStateError("transformer_not_found", "response transformer not found").
		WithDetail("transformer_name", name)
}

// ==============================================================================
// PROCESSING STAGES
// ==============================================================================

// AuthenticationStage handles request authentication.
type AuthenticationStage struct {
	name     string
	priority int
	config   *PipelineConfig
}

// NewAuthenticationStage creates a new authentication stage.
func NewAuthenticationStage(config *PipelineConfig) *AuthenticationStage {
	return &AuthenticationStage{
		name:     "authentication",
		priority: 1000, // High priority
		config:   config,
	}
}

// Name returns the stage name.
func (s *AuthenticationStage) Name() string {
	return s.name
}

// Priority returns the stage priority.
func (s *AuthenticationStage) Priority() int {
	return s.priority
}

// Process processes the authentication.
func (s *AuthenticationStage) Process(ctx context.Context, pipelineCtx *PipelineContext) error {
	// Check for authentication headers
	authHeader := pipelineCtx.Request.Headers["Authorization"]
	if authHeader == "" {
		if s.config.EnableAuthentication {
			return pkgerrors.NewSecurityError("auth_required", "authentication required")
		}
		return nil
	}
	
	// TODO: Implement actual authentication logic
	// For now, mark as authenticated if header is present
	pipelineCtx.Authenticated = true
	
	// Extract user information from token/auth header
	// This is a placeholder - implement actual token validation
	pipelineCtx.Request.UserID = "authenticated_user"
	
	return nil
}

// ShouldProcess returns true if authentication should be processed.
func (s *AuthenticationStage) ShouldProcess(ctx context.Context, pipelineCtx *PipelineContext) bool {
	return s.config.EnableAuthentication
}

// OnError handles authentication errors.
func (s *AuthenticationStage) OnError(ctx context.Context, pipelineCtx *PipelineContext, err error) error {
	pipelineCtx.Authenticated = false
	return err
}

// ValidationStage handles request validation.
type ValidationStage struct {
	name      string
	priority  int
	config    *PipelineConfig
	validator PipelineValidator
}

// NewValidationStage creates a new validation stage.
func NewValidationStage(config *PipelineConfig, validator PipelineValidator) *ValidationStage {
	return &ValidationStage{
		name:      "validation",
		priority:  900, // High priority
		config:    config,
		validator: validator,
	}
}

// Name returns the stage name.
func (s *ValidationStage) Name() string {
	return s.name
}

// Priority returns the stage priority.
func (s *ValidationStage) Priority() int {
	return s.priority
}

// Process processes the validation.
func (s *ValidationStage) Process(ctx context.Context, pipelineCtx *PipelineContext) error {
	return s.validator.ValidateRequest(ctx, pipelineCtx.Request)
}

// ShouldProcess returns true if validation should be processed.
func (s *ValidationStage) ShouldProcess(ctx context.Context, pipelineCtx *PipelineContext) bool {
	return s.config.EnableRequestValidation
}

// OnError handles validation errors.
func (s *ValidationStage) OnError(ctx context.Context, pipelineCtx *PipelineContext, err error) error {
	return err
}

// DecodingStage handles request body decoding.
type DecodingStage struct {
	name         string
	priority     int
	codecFactory encoding.CodecFactory
}

// NewDecodingStage creates a new decoding stage.
func NewDecodingStage(codecFactory encoding.CodecFactory) *DecodingStage {
	return &DecodingStage{
		name:         "decoding",
		priority:     800,
		codecFactory: codecFactory,
	}
}

// Name returns the stage name.
func (s *DecodingStage) Name() string {
	return s.name
}

// Priority returns the stage priority.
func (s *DecodingStage) Priority() int {
	return s.priority
}

// Process processes the decoding.
func (s *DecodingStage) Process(ctx context.Context, pipelineCtx *PipelineContext) error {
	if len(pipelineCtx.Request.Body) == 0 {
		return nil // No body to decode
	}
	
	contentType := pipelineCtx.Request.ContentType
	if contentType == "" {
		return nil // No content type specified
	}
	
	// Get appropriate decoder
	decoder, err := s.codecFactory.CreateCodec(ctx, contentType, nil, &encoding.DecodingOptions{
		Strict: true,
	})
	if err != nil {
		return pkgerrors.NewEncodingError("decoder_not_available", "no decoder available for content type").
			WithCause(err).
			WithMimeType(contentType)
	}
	
	// Decode based on content type
	if contentType == "application/json" {
		// Try to decode as AG-UI event first
		if event, err := s.decodeAsEvent(ctx, decoder, pipelineCtx.Request.Body); err == nil {
			pipelineCtx.Request.Event = event
			return nil
		}
		
		// Try to decode as message
		if message, err := s.decodeAsMessage(pipelineCtx.Request.Body); err == nil {
			pipelineCtx.Request.Message = message
			return nil
		}
	}
	
	return nil
}

// ShouldProcess returns true if decoding should be processed.
func (s *DecodingStage) ShouldProcess(ctx context.Context, pipelineCtx *PipelineContext) bool {
	return len(pipelineCtx.Request.Body) > 0 && pipelineCtx.Request.ContentType != ""
}

// OnError handles decoding errors.
func (s *DecodingStage) OnError(ctx context.Context, pipelineCtx *PipelineContext, err error) error {
	return err
}

// RouteResolutionStage handles route resolution and agent selection.
type RouteResolutionStage struct {
	name     string
	priority int
}

// NewRouteResolutionStage creates a new route resolution stage.
func NewRouteResolutionStage() *RouteResolutionStage {
	return &RouteResolutionStage{
		name:     "route_resolution",
		priority: 700,
	}
}

// Name returns the stage name.
func (s *RouteResolutionStage) Name() string {
	return s.name
}

// Priority returns the stage priority.
func (s *RouteResolutionStage) Priority() int {
	return s.priority
}

// Process processes the route resolution.
func (s *RouteResolutionStage) Process(ctx context.Context, pipelineCtx *PipelineContext) error {
	// Extract agent name from path, query params, or headers
	path := pipelineCtx.Request.Path
	
	// Try to extract from path (e.g., /agents/{agent_name}/...)
	if len(path) > 8 && path[:8] == "/agents/" {
		parts := strings.Split(path[8:], "/")
		if len(parts) > 0 && parts[0] != "" {
			pipelineCtx.Request.AgentName = parts[0]
		}
	}
	
	// Try to extract from query parameters
	if agentName := pipelineCtx.Request.QueryParams["agent"]; agentName != "" {
		pipelineCtx.Request.AgentName = agentName
	}
	
	// Try to extract from headers
	if agentName := pipelineCtx.Request.Headers["X-Agent-Name"]; agentName != "" {
		pipelineCtx.Request.AgentName = agentName
	}
	
	return nil
}

// ShouldProcess returns true if route resolution should be processed.
func (s *RouteResolutionStage) ShouldProcess(ctx context.Context, pipelineCtx *PipelineContext) bool {
	return true // Always process route resolution
}

// OnError handles route resolution errors.
func (s *RouteResolutionStage) OnError(ctx context.Context, pipelineCtx *PipelineContext, err error) error {
	return err
}

// ==============================================================================
// RESPONSE TRANSFORMERS
// ==============================================================================

// CompressionTransformer handles response compression.
type CompressionTransformer struct {
	name     string
	priority int
	config   *PipelineConfig
}

// NewCompressionTransformer creates a new compression transformer.
func NewCompressionTransformer(config *PipelineConfig) *CompressionTransformer {
	return &CompressionTransformer{
		name:     "compression",
		priority: 100,
		config:   config,
	}
}

// Name returns the transformer name.
func (t *CompressionTransformer) Name() string {
	return t.name
}

// Priority returns the transformer priority.
func (t *CompressionTransformer) Priority() int {
	return t.priority
}

// Transform transforms the response with compression.
func (t *CompressionTransformer) Transform(ctx context.Context, resp *PipelineResponse) error {
	if !t.config.EnableCompression || len(resp.Body) == 0 {
		return nil
	}
	
	// TODO: Implement actual compression logic
	// For now, just add the header
	if resp.Headers == nil {
		resp.Headers = make(map[string]string)
	}
	resp.Headers["Content-Encoding"] = "gzip"
	
	return nil
}

// ShouldTransform returns true if compression should be applied.
func (t *CompressionTransformer) ShouldTransform(ctx context.Context, resp *PipelineResponse) bool {
	return t.config.EnableCompression && len(resp.Body) > 1024 // Compress responses > 1KB
}

// ==============================================================================
// VALIDATOR IMPLEMENTATION
// ==============================================================================

// DefaultPipelineValidator implements basic request/response validation.
type DefaultPipelineValidator struct {
	config *PipelineConfig
}

// NewDefaultPipelineValidator creates a new default pipeline validator.
func NewDefaultPipelineValidator(config *PipelineConfig) *DefaultPipelineValidator {
	return &DefaultPipelineValidator{
		config: config,
	}
}

// ValidateRequest validates an incoming request.
func (v *DefaultPipelineValidator) ValidateRequest(ctx context.Context, req *PipelineRequest) error {
	// Check request size limits
	if v.config.MaxRequestSize > 0 && req.ContentLength > v.config.MaxRequestSize {
		return pkgerrors.NewValidationError("request_too_large", "request exceeds maximum size limit").
			WithDetail("max_size", v.config.MaxRequestSize).
			WithDetail("actual_size", req.ContentLength)
	}
	
	// Validate content type
	if len(v.config.SupportedContentTypes) > 0 && req.ContentType != "" {
		supported := false
		for _, ct := range v.config.SupportedContentTypes {
			if req.ContentType == ct {
				supported = true
				break
			}
		}
		if !supported {
			return pkgerrors.NewValidationError("unsupported_content_type", "content type not supported").
				WithDetail("content_type", req.ContentType).
				WithDetail("supported_types", v.config.SupportedContentTypes)
		}
	}
	
	// Validate required headers
	requiredHeaders := []string{"Content-Type", "User-Agent"}
	for _, header := range requiredHeaders {
		if req.Headers[header] == "" {
			return pkgerrors.NewValidationError("missing_required_header", "required header missing").
				WithDetail("header", header)
		}
	}
	
	return nil
}

// ValidateResponse validates an outgoing response.
func (v *DefaultPipelineValidator) ValidateResponse(ctx context.Context, resp *PipelineResponse) error {
	// Check response size limits
	if v.config.MaxResponseSize > 0 && resp.ContentLength > v.config.MaxResponseSize {
		return pkgerrors.NewValidationError("response_too_large", "response exceeds maximum size limit").
			WithDetail("max_size", v.config.MaxResponseSize).
			WithDetail("actual_size", resp.ContentLength)
	}
	
	// Validate status code
	if resp.StatusCode < 100 || resp.StatusCode > 599 {
		return pkgerrors.NewValidationError("invalid_status_code", "invalid HTTP status code").
			WithDetail("status_code", resp.StatusCode)
	}
	
	return nil
}

// ==============================================================================
// HELPER FUNCTIONS
// ==============================================================================

// createPipelineContext creates a new pipeline context from an HTTP request.
func (p *RequestProcessingPipeline) createPipelineContext(ctx context.Context, req *http.Request) (*PipelineContext, error) {
	// Create pipeline request
	pipelineReq, err := p.createPipelineRequest(req)
	if err != nil {
		return nil, err
	}
	
	// Create pipeline response
	pipelineResp := &PipelineResponse{
		ID:         uuid.New().String(),
		RequestID:  pipelineReq.ID,
		Timestamp:  time.Now(),
		StatusCode: http.StatusOK,
		Headers:    make(map[string]string),
	}
	
	// Create pipeline context
	pipelineCtx := &PipelineContext{
		Context:        ctx,
		Request:        pipelineReq,
		Response:       pipelineResp,
		StartTime:      time.Now(),
		StageTimings:   make(map[string]time.Duration),
		ProcessingData: make(map[string]interface{}),
		Errors:         make([]error, 0),
	}
	
	return pipelineCtx, nil
}

// createPipelineRequest creates a pipeline request from an HTTP request.
func (p *RequestProcessingPipeline) createPipelineRequest(req *http.Request) (*PipelineRequest, error) {
	// Read request body
	var body []byte
	var err error
	if req.Body != nil {
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, pkgerrors.NewEncodingError("body_read_failed", "failed to read request body").WithCause(err)
		}
		req.Body.Close()
	}
	
	// Extract headers
	headers := make(map[string]string)
	for key, values := range req.Header {
		if len(values) > 0 {
			headers[key] = values[0] // Take first value
		}
	}
	
	// Extract query parameters
	queryParams := make(map[string]string)
	for key, values := range req.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0] // Take first value
		}
	}
	
	// Create pipeline request
	pipelineReq := &PipelineRequest{
		HTTPRequest:   req,
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		Method:        req.Method,
		Path:          req.URL.Path,
		Headers:       headers,
		QueryParams:   queryParams,
		Body:          body,
		ContentType:   req.Header.Get("Content-Type"),
		ContentLength: int64(len(body)),
		Metadata:      make(map[string]interface{}),
	}
	
	// Extract session ID from headers or cookies
	if sessionID := req.Header.Get("X-Session-ID"); sessionID != "" {
		pipelineReq.SessionID = sessionID
	}
	
	return pipelineReq, nil
}

// processStages processes the request through all configured stages.
func (p *RequestProcessingPipeline) processStages(ctx context.Context, pipelineCtx *PipelineContext) error {
	p.stagesMu.RLock()
	stages := make([]ProcessingStage, len(p.stages))
	copy(stages, p.stages)
	p.stagesMu.RUnlock()
	
	for i, stage := range stages {
		if pipelineCtx.Failed && p.config.FailFast {
			break
		}
		
		pipelineCtx.Stage = stage.Name()
		pipelineCtx.StageIndex = i
		
		if !stage.ShouldProcess(ctx, pipelineCtx) {
			continue
		}
		
		stageStart := time.Now()
		
		// Add timeout for stage processing
		stageCtx := ctx
		if p.config.RequestTimeout > 0 {
			var cancel context.CancelFunc
			stageCtx, cancel = context.WithTimeout(ctx, p.config.RequestTimeout)
			defer cancel()
		}
		
		// Process stage with recovery
		err := p.processStageWithRecovery(stageCtx, stage, pipelineCtx)
		
		// Record stage timing
		stageDuration := time.Since(stageStart)
		pipelineCtx.StageTimings[stage.Name()] = stageDuration
		
		// Update stage metrics
		if p.metrics != nil {
			p.metrics.StageCounter.Add(ctx, 1,
				metric.WithAttributes(attribute.String("stage", stage.Name())),
			)
			p.metrics.StageDuration.Record(ctx, stageDuration.Seconds(),
				metric.WithAttributes(attribute.String("stage", stage.Name())),
			)
			
			if err != nil {
				p.metrics.StageErrors.Add(ctx, 1,
					metric.WithAttributes(attribute.String("stage", stage.Name())),
				)
			}
		}
		
		if err != nil {
			pipelineCtx.Failed = true
			pipelineCtx.Errors = append(pipelineCtx.Errors, err)
			
			// Try stage-specific error handling
			if stageErr := stage.OnError(ctx, pipelineCtx, err); stageErr != nil {
				return stageErr
			}
			
			if p.config.FailFast {
				return err
			}
		}
	}
	
	pipelineCtx.Completed = true
	return nil
}

// processStageWithRecovery processes a stage with panic recovery.
func (p *RequestProcessingPipeline) processStageWithRecovery(ctx context.Context, stage ProcessingStage, pipelineCtx *PipelineContext) (err error) {
	if p.config.EnableRecovery {
		defer func() {
			if r := recover(); r != nil {
				p.logger.WithFields(logrus.Fields{
					"stage":      stage.Name(),
					"request_id": pipelineCtx.Request.ID,
					"panic":      r,
					"stack":      string(debug.Stack()),
				}).Error("Stage processing panic recovered")
				
				err = pkgerrors.NewStateError("stage_panic", "stage processing panic").
					WithDetail("stage", stage.Name()).
					WithDetail("panic", fmt.Sprintf("%v", r))
			}
		}()
	}
	
	return stage.Process(ctx, pipelineCtx)
}

// createErrorResponse creates an error response.
func (p *RequestProcessingPipeline) createErrorResponse(pipelineCtx *PipelineContext, err error, statusCode int) *PipelineResponse {
	// Update error metrics
	if p.metrics != nil {
		atomic.AddInt64(&p.totalErrors, 1)
		p.metrics.ErrorCounter.Add(pipelineCtx.Context, 1,
			metric.WithAttributes(
				attribute.String("error_type", "pipeline_error"),
				attribute.Int("status_code", statusCode),
			),
		)
	}
	
	// Create error response
	resp := &PipelineResponse{
		ID:           uuid.New().String(),
		RequestID:    pipelineCtx.Request.ID,
		Timestamp:    time.Now(),
		StatusCode:   statusCode,
		Headers:      make(map[string]string),
		Error:        err,
		ErrorMessage: err.Error(),
	}
	
	// Set error code if available
	if baseErr, ok := err.(*pkgerrors.BaseError); ok {
		resp.ErrorCode = baseErr.Code
	}
	
	// Create error response body
	errorData := map[string]interface{}{
		"error":      true,
		"message":    err.Error(),
		"request_id": pipelineCtx.Request.ID,
		"timestamp":  resp.Timestamp.Unix(),
	}
	
	if p.config.EnableErrorDetails {
		errorData["details"] = map[string]interface{}{
			"stage":           pipelineCtx.Stage,
			"stage_index":     pipelineCtx.StageIndex,
			"processing_time": time.Since(pipelineCtx.StartTime).String(),
		}
		
		if len(pipelineCtx.Errors) > 0 {
			errors := make([]string, len(pipelineCtx.Errors))
			for i, e := range pipelineCtx.Errors {
				errors[i] = e.Error()
			}
			errorData["all_errors"] = errors
		}
	}
	
	// Serialize error response
	if bodyBytes, err := json.Marshal(errorData); err == nil {
		resp.Body = bodyBytes
		resp.ContentType = "application/json"
		resp.ContentLength = int64(len(bodyBytes))
	}
	
	resp.Headers["Content-Type"] = "application/json"
	
	return resp
}

// checkRateLimit checks if the request is within rate limits.
func (p *RequestProcessingPipeline) checkRateLimit(ctx context.Context, pipelineCtx *PipelineContext) error {
	// TODO: Implement actual rate limiting logic
	// This is a placeholder implementation
	
	if p.config.RateLimitPerSecond <= 0 {
		return nil
	}
	
	// Simple check based on current active requests
	if atomic.LoadInt64(&p.activeRequests) > int64(p.config.RateLimitPerSecond) {
		return pkgerrors.NewStateError("rate_limit_exceeded", "rate limit exceeded").
			WithDetail("limit", p.config.RateLimitPerSecond).
			WithDetail("current", atomic.LoadInt64(&p.activeRequests))
	}
	
	return nil
}

// recoverFromError attempts to recover from processing errors.
func (p *RequestProcessingPipeline) recoverFromError(ctx context.Context, pipelineCtx *PipelineContext, err error) error {
	// TODO: Implement actual recovery logic
	// This could include retry logic, fallback processing, etc.
	
	p.logger.WithFields(logrus.Fields{
		"request_id": pipelineCtx.Request.ID,
		"error":      err.Error(),
		"stage":      pipelineCtx.Stage,
	}).Warn("Attempting error recovery")
	
	// Simple recovery: reset failed state and continue
	pipelineCtx.Failed = false
	
	return nil
}

// registerDefaultStages registers the default processing stages.
func (p *RequestProcessingPipeline) registerDefaultStages() error {
	// Authentication stage
	if p.config.EnableAuthentication {
		authStage := NewAuthenticationStage(p.config)
		p.AddStage(authStage)
	}
	
	// Validation stage
	validationStage := NewValidationStage(p.config, p.validator)
	p.AddStage(validationStage)
	
	// Decoding stage
	decodingStage := NewDecodingStage(p.codecFactory)
	p.AddStage(decodingStage)
	
	// Route resolution stage
	routeStage := NewRouteResolutionStage()
	p.AddStage(routeStage)
	
	// Register default response transformers
	if p.config.EnableCompression {
		compressionTransformer := NewCompressionTransformer(p.config)
		p.responseProcessor.AddTransformer(compressionTransformer)
	}
	
	return nil
}

// sortStages sorts stages by priority.
func (p *RequestProcessingPipeline) sortStages() {
	// Sort by priority (higher priority first)
	for i := 0; i < len(p.stages)-1; i++ {
		for j := i + 1; j < len(p.stages); j++ {
			if p.stages[i].Priority() < p.stages[j].Priority() {
				p.stages[i], p.stages[j] = p.stages[j], p.stages[i]
			}
		}
	}
}

// sortTransformers sorts transformers by priority.
func (rp *ResponseProcessingPipeline) sortTransformers() {
	// Sort by priority (higher priority first)
	for i := 0; i < len(rp.transformers)-1; i++ {
		for j := i + 1; j < len(rp.transformers); j++ {
			if rp.transformers[i].Priority() < rp.transformers[j].Priority() {
				rp.transformers[i], rp.transformers[j] = rp.transformers[j], rp.transformers[i]
			}
		}
	}
}

// decodeAsEvent attempts to decode body as an AG-UI event.
func (s *DecodingStage) decodeAsEvent(ctx context.Context, decoder encoding.Codec, body []byte) (events.Event, error) {
	// This is a placeholder - actual implementation would depend on event structure
	return nil, fmt.Errorf("event decoding not implemented")
}

// decodeAsMessage attempts to decode body as a message.
func (s *DecodingStage) decodeAsMessage(body []byte) (messages.Message, error) {
	// This is a placeholder - actual implementation would decode JSON to message
	return nil, fmt.Errorf("message decoding not implemented")
}

// serializeAsJSON serializes data as JSON.
func (rp *ResponseProcessingPipeline) serializeAsJSON(data interface{}, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// registerDefaultCodecs registers default codecs with the factory.
func registerDefaultCodecs(factory *encoding.DefaultCodecFactory) error {
	// Register JSON codec
	jsonCtor := func(encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) (encoding.Codec, error) {
		// This would return an actual JSON codec implementation
		return nil, fmt.Errorf("JSON codec not implemented")
	}
	
	if err := factory.RegisterCodec("application/json", jsonCtor); err != nil {
		return err
	}
	
	return nil
}

// initializePipelineMetrics initializes OpenTelemetry metrics.
func initializePipelineMetrics() (*PipelineMetrics, error) {
	meter := otel.Meter("ag-ui-pipeline")
	
	requestCounter, err := meter.Int64Counter(
		"pipeline_requests_total",
		metric.WithDescription("Total number of pipeline requests"),
	)
	if err != nil {
		return nil, err
	}
	
	requestDuration, err := meter.Float64Histogram(
		"pipeline_request_duration_seconds",
		metric.WithDescription("Pipeline request duration in seconds"),
	)
	if err != nil {
		return nil, err
	}
	
	requestSize, err := meter.Int64Histogram(
		"pipeline_request_size_bytes",
		metric.WithDescription("Pipeline request size in bytes"),
	)
	if err != nil {
		return nil, err
	}
	
	responseSize, err := meter.Int64Histogram(
		"pipeline_response_size_bytes",
		metric.WithDescription("Pipeline response size in bytes"),
	)
	if err != nil {
		return nil, err
	}
	
	activeRequests, err := meter.Int64UpDownCounter(
		"pipeline_active_requests",
		metric.WithDescription("Number of active pipeline requests"),
	)
	if err != nil {
		return nil, err
	}
	
	stageCounter, err := meter.Int64Counter(
		"pipeline_stages_total",
		metric.WithDescription("Total number of stage executions"),
	)
	if err != nil {
		return nil, err
	}
	
	stageDuration, err := meter.Float64Histogram(
		"pipeline_stage_duration_seconds",
		metric.WithDescription("Pipeline stage duration in seconds"),
	)
	if err != nil {
		return nil, err
	}
	
	stageErrors, err := meter.Int64Counter(
		"pipeline_stage_errors_total",
		metric.WithDescription("Total number of stage errors"),
	)
	if err != nil {
		return nil, err
	}
	
	errorCounter, err := meter.Int64Counter(
		"pipeline_errors_total",
		metric.WithDescription("Total number of pipeline errors"),
	)
	if err != nil {
		return nil, err
	}
	
	validationErrors, err := meter.Int64Counter(
		"pipeline_validation_errors_total",
		metric.WithDescription("Total number of validation errors"),
	)
	if err != nil {
		return nil, err
	}
	
	timeoutErrors, err := meter.Int64Counter(
		"pipeline_timeout_errors_total",
		metric.WithDescription("Total number of timeout errors"),
	)
	if err != nil {
		return nil, err
	}
	
	throughputCounter, err := meter.Float64Counter(
		"pipeline_throughput_requests_per_second",
		metric.WithDescription("Pipeline throughput in requests per second"),
	)
	if err != nil {
		return nil, err
	}
	
	latencyPercentiles, err := meter.Float64Histogram(
		"pipeline_latency_percentiles_seconds",
		metric.WithDescription("Pipeline latency percentiles in seconds"),
	)
	if err != nil {
		return nil, err
	}
	
	concurrencyGauge, err := meter.Int64Gauge(
		"pipeline_concurrency_current",
		metric.WithDescription("Current pipeline concurrency level"),
	)
	if err != nil {
		return nil, err
	}
	
	return &PipelineMetrics{
		RequestCounter:       requestCounter,
		RequestDuration:      requestDuration,
		RequestSize:          requestSize,
		ResponseSize:         responseSize,
		ActiveRequests:       activeRequests,
		StageCounter:         stageCounter,
		StageDuration:        stageDuration,
		StageErrors:          stageErrors,
		ErrorCounter:         errorCounter,
		ValidationErrors:     validationErrors,
		TimeoutErrors:        timeoutErrors,
		ThroughputCounter:    throughputCounter,
		LatencyPercentiles:   latencyPercentiles,
		ConcurrencyGauge:     concurrencyGauge,
	}, nil
}

// validatePipelineConfig validates the pipeline configuration.
func validatePipelineConfig(config *PipelineConfig) error {
	if config.MaxConcurrentRequests < 0 {
		return fmt.Errorf("max concurrent requests cannot be negative")
	}
	
	if config.RequestTimeout < 0 {
		return fmt.Errorf("request timeout cannot be negative")
	}
	
	if config.MaxRequestSize < 0 {
		return fmt.Errorf("max request size cannot be negative")
	}
	
	if config.MaxResponseSize < 0 {
		return fmt.Errorf("max response size cannot be negative")
	}
	
	if config.RateLimitPerSecond < 0 {
		return fmt.Errorf("rate limit per second cannot be negative")
	}
	
	return nil
}

// DefaultPipelineConfig returns a default pipeline configuration.
func DefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		MaxConcurrentRequests:    1000,
		RequestTimeout:           30 * time.Second,
		MaxRequestSize:           10 * 1024 * 1024, // 10MB
		MaxResponseSize:          10 * 1024 * 1024, // 10MB
		EnableAsyncProcessing:    true,
		EnableStageParallel:      false,
		FailFast:                 true,
		EnableRecovery:           true,
		EnableErrorDetails:       true,
		MaxRetries:               3,
		RetryDelay:               1 * time.Second,
		EnableRequestValidation:  true,
		EnableResponseValidation: true,
		StrictValidation:         false,
		EnableMetrics:            true,
		EnableTracing:            true,
		EnableDetailedLogs:       true,
		DefaultContentType:       "application/json",
		SupportedContentTypes:    []string{"application/json", "text/plain", "application/xml"},
		EnableCompression:        true,
		EnableAuthentication:     false,
		EnableAuthorization:      false,
		EnableRateLimit:          true,
		RateLimitPerSecond:       5000, // Target > 5000 requests/second
	}
}

// ==============================================================================
// UTILITY FUNCTIONS FOR STRINGS PACKAGE
// ==============================================================================

func init() {
	// This ensures strings package is imported and available for use
	// (strings package should be imported at the top of the file)
}