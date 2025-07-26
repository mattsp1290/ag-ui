package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Validator defines the interface for validating transport events and messages
type Validator interface {
	// Validate validates a transport event
	Validate(ctx context.Context, event TransportEvent) error
	
	// ValidateIncoming validates an incoming event
	ValidateIncoming(ctx context.Context, event TransportEvent) error
	
	// ValidateOutgoing validates an outgoing event
	ValidateOutgoing(ctx context.Context, event TransportEvent) error
}

// ValidationRule defines a single validation rule
type ValidationRule interface {
	// Name returns the name of the validation rule
	Name() string
	
	// Validate validates the event against this rule
	Validate(ctx context.Context, event TransportEvent) error
	
	// IsEnabled returns whether this rule is enabled
	IsEnabled() bool
	
	// Priority returns the priority of this rule (higher = earlier execution)
	Priority() int
}

// ValidationConfig holds configuration for validation
type ValidationConfig struct {
	// Enabled controls whether validation is enabled
	Enabled bool
	
	// MaxMessageSize is the maximum allowed message size in bytes
	MaxMessageSize int64
	
	// RequiredFields lists fields that must be present in event data
	RequiredFields []string
	
	// AllowedEventTypes lists allowed event types (empty = all allowed)
	AllowedEventTypes []string
	
	// DeniedEventTypes lists denied event types
	DeniedEventTypes []string
	
	// MaxDataDepth is the maximum nesting depth for event data
	MaxDataDepth int
	
	// MaxArraySize is the maximum size for arrays in event data
	MaxArraySize int
	
	// MaxStringLength is the maximum length for string values
	MaxStringLength int
	
	// AllowedDataTypes lists allowed data types for event data values
	AllowedDataTypes []string
	
	// CustomValidators are custom validation functions
	CustomValidators []ValidationRule
	
	// FieldValidators are field-specific validation rules
	FieldValidators map[string][]ValidationRule
	
	// PatternValidators are regex-based validation rules
	PatternValidators map[string]*regexp.Regexp
	
	// SkipValidationOnIncoming skips validation for incoming events
	SkipValidationOnIncoming bool
	
	// SkipValidationOnOutgoing skips validation for outgoing events
	SkipValidationOnOutgoing bool
	
	// FailFast stops validation on first error
	FailFast bool
	
	// CollectAllErrors collects all validation errors
	CollectAllErrors bool
	
	// ValidateTimestamps enables timestamp validation
	ValidateTimestamps bool
	
	// StrictMode enables strict validation mode
	StrictMode bool
	
	// MaxEventSize is the maximum size of an event in bytes
	MaxEventSize int
}

// DefaultValidationConfig returns a default validation configuration
func DefaultValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		Enabled:           true,
		MaxMessageSize:    1024 * 1024, // 1MB
		RequiredFields:    []string{"id", "type", "timestamp"},
		AllowedEventTypes: []string{},
		DeniedEventTypes:  []string{},
		MaxDataDepth:      10,
		MaxArraySize:      1000,
		MaxStringLength:   10000,
		AllowedDataTypes:  []string{"string", "number", "boolean", "object", "array", "null"},
		CustomValidators:  []ValidationRule{},
		FieldValidators:   make(map[string][]ValidationRule),
		PatternValidators: make(map[string]*regexp.Regexp),
		SkipValidationOnIncoming: false,
		SkipValidationOnOutgoing: false,
		FailFast:                 false,
		CollectAllErrors:         true,
		ValidateTimestamps:       true,
		StrictMode:               false,
		MaxEventSize:             1024 * 1024, // 1MB
	}
}

// DefaultValidator is the default implementation of Validator
type DefaultValidator struct {
	config *ValidationConfig
	rules  []ValidationRule
}

// NewValidator creates a new validator with the given configuration
func NewValidator(config *ValidationConfig) *DefaultValidator {
	if config == nil {
		config = DefaultValidationConfig()
	}
	
	validator := &DefaultValidator{
		config: config,
		rules:  make([]ValidationRule, 0),
	}
	
	// Add built-in validation rules
	validator.addBuiltinRules()
	
	// Add custom validation rules
	validator.rules = append(validator.rules, config.CustomValidators...)
	
	return validator
}

// Validate validates a transport event
func (v *DefaultValidator) Validate(ctx context.Context, event TransportEvent) error {
	if !v.config.Enabled {
		return nil
	}
	
	var errors []error
	
	for _, rule := range v.rules {
		if !rule.IsEnabled() {
			continue
		}
		
		if err := rule.Validate(ctx, event); err != nil {
			if v.config.FailFast {
				return err
			}
			// Always collect the first error
			errors = append(errors, err)
			// If not collecting all errors, stop after first error
			if !v.config.CollectAllErrors {
				break
			}
		}
	}
	
	if len(errors) > 0 {
		return NewValidationError("validation failed", errors)
	}
	
	return nil
}

// ValidateIncoming validates an incoming event
func (v *DefaultValidator) ValidateIncoming(ctx context.Context, event TransportEvent) error {
	if v.config.SkipValidationOnIncoming {
		return nil
	}
	return v.Validate(ctx, event)
}

// ValidateOutgoing validates an outgoing event
func (v *DefaultValidator) ValidateOutgoing(ctx context.Context, event TransportEvent) error {
	if v.config.SkipValidationOnOutgoing {
		return nil
	}
	return v.Validate(ctx, event)
}

// addBuiltinRules adds built-in validation rules
func (v *DefaultValidator) addBuiltinRules() {
	// Add message size validation
	v.rules = append(v.rules, &MessageSizeRule{
		maxSize: v.config.MaxMessageSize,
		enabled: v.config.MaxMessageSize > 0,
	})
	
	// Add required fields validation
	v.rules = append(v.rules, &RequiredFieldsRule{
		requiredFields: v.config.RequiredFields,
		enabled:        len(v.config.RequiredFields) > 0,
	})
	
	// Add event type validation
	v.rules = append(v.rules, &EventTypeRule{
		allowedTypes: v.config.AllowedEventTypes,
		deniedTypes:  v.config.DeniedEventTypes,
		enabled:      len(v.config.AllowedEventTypes) > 0 || len(v.config.DeniedEventTypes) > 0,
	})
	
	// Add data format validation
	v.rules = append(v.rules, &DataFormatRule{
		maxDepth:         v.config.MaxDataDepth,
		maxArraySize:     v.config.MaxArraySize,
		maxStringLength:  v.config.MaxStringLength,
		allowedDataTypes: v.config.AllowedDataTypes,
		enabled:          true,
	})
	
	// Add field-specific validators
	for field, validators := range v.config.FieldValidators {
		v.rules = append(v.rules, &FieldValidatorRule{
			field:      field,
			validators: validators,
			enabled:    len(validators) > 0,
		})
	}
	
	// Add pattern validators
	for field, pattern := range v.config.PatternValidators {
		v.rules = append(v.rules, &PatternValidatorRule{
			field:   field,
			pattern: pattern,
			enabled: pattern != nil,
		})
	}
}

// MessageSizeRule validates message size
type MessageSizeRule struct {
	maxSize int64
	enabled bool
}

func (r *MessageSizeRule) Name() string {
	return "message_size"
}

func (r *MessageSizeRule) Validate(ctx context.Context, event TransportEvent) error {
	if !r.enabled {
		return nil
	}
	
	// Calculate message size by serializing the event data
	data, err := json.Marshal(event.Data())
	if err != nil {
		return NewValidationError("failed to serialize event data for size validation", []error{err})
	}
	
	size := int64(len(data))
	if size > r.maxSize {
		return ErrInvalidMessageSize
	}
	
	return nil
}

func (r *MessageSizeRule) IsEnabled() bool {
	return r.enabled
}

func (r *MessageSizeRule) Priority() int {
	return 100 // High priority
}

// RequiredFieldsRule validates required fields
type RequiredFieldsRule struct {
	requiredFields []string
	enabled        bool
}

func (r *RequiredFieldsRule) Name() string {
	return "required_fields"
}

func (r *RequiredFieldsRule) Validate(ctx context.Context, event TransportEvent) error {
	if !r.enabled {
		return nil
	}
	
	data := event.Data()
	var missingFields []string
	
	for _, field := range r.requiredFields {
		if _, exists := data[field]; !exists {
			missingFields = append(missingFields, field)
		}
	}
	
	if len(missingFields) > 0 {
		return ErrMissingRequiredFields
	}
	
	return nil
}

func (r *RequiredFieldsRule) IsEnabled() bool {
	return r.enabled
}

func (r *RequiredFieldsRule) Priority() int {
	return 90 // High priority
}

// EventTypeRule validates event types
type EventTypeRule struct {
	allowedTypes []string
	deniedTypes  []string
	enabled      bool
}

func (r *EventTypeRule) Name() string {
	return "event_type"
}

func (r *EventTypeRule) Validate(ctx context.Context, event TransportEvent) error {
	if !r.enabled {
		return nil
	}
	
	eventType := event.Type()
	
	// Check denied types first
	for _, deniedType := range r.deniedTypes {
		if eventType == deniedType {
			return ErrInvalidEventType
		}
	}
	
	// Check allowed types if specified
	if len(r.allowedTypes) > 0 {
		for _, allowedType := range r.allowedTypes {
			if eventType == allowedType {
				return nil
			}
		}
		return ErrInvalidEventType
	}
	
	return nil
}

func (r *EventTypeRule) IsEnabled() bool {
	return r.enabled
}

func (r *EventTypeRule) Priority() int {
	return 80 // Medium-high priority
}

// DataFormatRule validates data format
type DataFormatRule struct {
	maxDepth         int
	maxArraySize     int
	maxStringLength  int
	allowedDataTypes []string
	enabled          bool
}

func (r *DataFormatRule) Name() string {
	return "data_format"
}

func (r *DataFormatRule) Validate(ctx context.Context, event TransportEvent) error {
	if !r.enabled {
		return nil
	}
	
	data := event.Data()
	return r.validateValue(data, 0)
}

func (r *DataFormatRule) validateValue(value interface{}, depth int) error {
	if depth > r.maxDepth {
		return NewValidationError(fmt.Sprintf("data depth %d exceeds maximum allowed depth %d", depth, r.maxDepth), nil)
	}
	
	switch v := value.(type) {
	case string:
		if len(v) > r.maxStringLength {
			return NewValidationError(fmt.Sprintf("string length %d exceeds maximum allowed length %d", len(v), r.maxStringLength), nil)
		}
		if !r.isAllowedDataType("string") {
			return NewValidationError("string data type is not allowed", nil)
		}
	case []interface{}:
		if len(v) > r.maxArraySize {
			return NewValidationError(fmt.Sprintf("array size %d exceeds maximum allowed size %d", len(v), r.maxArraySize), nil)
		}
		if !r.isAllowedDataType("array") {
			return NewValidationError("array data type is not allowed", nil)
		}
		for _, item := range v {
			if err := r.validateValue(item, depth+1); err != nil {
				return err
			}
		}
	case map[string]interface{}:
		if !r.isAllowedDataType("object") {
			return NewValidationError("object data type is not allowed", nil)
		}
		for _, val := range v {
			if err := r.validateValue(val, depth+1); err != nil {
				return err
			}
		}
	case float64, int, int32, int64, float32:
		if !r.isAllowedDataType("number") {
			return NewValidationError("number data type is not allowed", nil)
		}
	case bool:
		if !r.isAllowedDataType("boolean") {
			return NewValidationError("boolean data type is not allowed", nil)
		}
	case nil:
		if !r.isAllowedDataType("null") {
			return NewValidationError("null data type is not allowed", nil)
		}
	case time.Time:
		// time.Time is commonly used in events, treat as string for validation
		timeStr := v.Format(time.RFC3339)
		if len(timeStr) > r.maxStringLength {
			return NewValidationError(fmt.Sprintf("timestamp string length %d exceeds maximum allowed length %d", len(timeStr), r.maxStringLength), nil)
		}
		if !r.isAllowedDataType("string") {
			return NewValidationError("timestamp data type is not allowed", nil)
		}
	default:
		// For other types, try to convert to string and validate as string
		strValue := fmt.Sprintf("%v", v)
		if len(strValue) > r.maxStringLength {
			return NewValidationError(fmt.Sprintf("converted string length %d exceeds maximum allowed length %d", len(strValue), r.maxStringLength), nil)
		}
		if !r.isAllowedDataType("string") {
			return NewValidationError("converted string data type is not allowed", nil)
		}
	}
	
	return nil
}

func (r *DataFormatRule) isAllowedDataType(dataType string) bool {
	if len(r.allowedDataTypes) == 0 {
		return true
	}
	for _, allowed := range r.allowedDataTypes {
		if allowed == dataType {
			return true
		}
	}
	return false
}

func (r *DataFormatRule) IsEnabled() bool {
	return r.enabled
}

func (r *DataFormatRule) Priority() int {
	return 70 // Medium priority
}

// FieldValidatorRule validates specific fields
type FieldValidatorRule struct {
	field      string
	validators []ValidationRule
	enabled    bool
}

func (r *FieldValidatorRule) Name() string {
	return fmt.Sprintf("field_%s", r.field)
}

func (r *FieldValidatorRule) Validate(ctx context.Context, event TransportEvent) error {
	if !r.enabled {
		return nil
	}
	
	data := event.Data()
	value, exists := data[r.field]
	if !exists {
		return nil // Field is optional, let RequiredFieldsRule handle required fields
	}
	
	// Create a temporary event with just this field for validation
	fieldEvent := &simpleEvent{
		id:        event.ID(),
		eventType: event.Type(),
		timestamp: event.Timestamp(),
		data:      map[string]interface{}{r.field: value},
	}
	
	var errors []error
	for _, validator := range r.validators {
		if err := validator.Validate(ctx, fieldEvent); err != nil {
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return NewValidationError(fmt.Sprintf("field '%s' validation failed", r.field), errors)
	}
	
	return nil
}

func (r *FieldValidatorRule) IsEnabled() bool {
	return r.enabled
}

func (r *FieldValidatorRule) Priority() int {
	return 60 // Medium priority
}

// PatternValidatorRule validates fields against regex patterns
type PatternValidatorRule struct {
	field   string
	pattern *regexp.Regexp
	enabled bool
}

func (r *PatternValidatorRule) Name() string {
	return fmt.Sprintf("pattern_%s", r.field)
}

func (r *PatternValidatorRule) Validate(ctx context.Context, event TransportEvent) error {
	if !r.enabled {
		return nil
	}
	
	data := event.Data()
	value, exists := data[r.field]
	if !exists {
		return nil // Field is optional
	}
	
	// Convert value to string for pattern matching
	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	case fmt.Stringer:
		strValue = v.String()
	default:
		strValue = fmt.Sprintf("%v", v)
	}
	
	if !r.pattern.MatchString(strValue) {
		return NewValidationError(fmt.Sprintf("field '%s' value '%s' does not match pattern '%s'", r.field, strValue, r.pattern.String()), nil)
	}
	
	return nil
}

func (r *PatternValidatorRule) IsEnabled() bool {
	return r.enabled
}

func (r *PatternValidatorRule) Priority() int {
	return 50 // Medium priority
}

// simpleEvent is a simple implementation of TransportEvent for testing
type simpleEvent struct {
	id        string
	eventType string
	timestamp time.Time
	data      map[string]interface{}
}

func (e *simpleEvent) ID() string {
	return e.id
}

func (e *simpleEvent) Type() string {
	return e.eventType
}

func (e *simpleEvent) Timestamp() time.Time {
	return e.timestamp
}

func (e *simpleEvent) Data() map[string]interface{} {
	return e.data
}

// ValidationError represents a validation error
type ValidationError struct {
	message string
	errors  []error
}

func (e *ValidationError) Error() string {
	if len(e.errors) == 0 {
		return e.message
	}
	
	var errorMessages []string
	for _, err := range e.errors {
		errorMessages = append(errorMessages, err.Error())
	}
	
	return fmt.Sprintf("%s: %s", e.message, strings.Join(errorMessages, "; "))
}

func (e *ValidationError) Errors() []error {
	return e.errors
}

func (e *ValidationError) Unwrap() error {
	if len(e.errors) == 1 {
		return e.errors[0]
	}
	return nil
}

// NewValidationError creates a new validation error
func NewValidationError(message string, errors []error) *ValidationError {
	return &ValidationError{
		message: message,
		errors:  errors,
	}
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// ValidatedTransportEvent wraps a TransportEvent with validation metadata
type ValidatedTransportEvent struct {
	TransportEvent
	ValidatedAt time.Time
	Validator   string
}

func (e *ValidatedTransportEvent) Data() map[string]interface{} {
	data := e.TransportEvent.Data()
	if data == nil {
		data = make(map[string]interface{})
	}
	
	// Add validation metadata
	data["_validated_at"] = e.ValidatedAt
	data["_validator"] = e.Validator
	
	return data
}

// NewValidatedTransportEvent creates a new validated transport event
func NewValidatedTransportEvent(event TransportEvent, validator string) *ValidatedTransportEvent {
	return &ValidatedTransportEvent{
		TransportEvent: event,
		ValidatedAt:    time.Now(),
		Validator:      validator,
	}
}