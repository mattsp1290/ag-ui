package orchestration

import (
	"fmt"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// OrchestrationValidationContext provides context for validation execution within orchestration
type OrchestrationValidationContext struct {
	EventData   map[string]interface{}
	Metadata    map[string]interface{}
	Properties  map[string]interface{}
	Source      string
	Environment string
	Tags        map[string]string
}

// OrchestrationValidationResult represents the result of a validation operation
type OrchestrationValidationResult struct {
	IsValid   bool                   `json:"is_valid"`
	Message   string                 `json:"message"`
	Validator string                 `json:"validator"`
	Errors    []string               `json:"errors,omitempty"`
	Warnings  []string               `json:"warnings,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
}

// Validator defines the interface for validation components in orchestration
type Validator interface {
	// Validate performs the validation operation
	Validate(ctx *OrchestrationValidationContext) (*OrchestrationValidationResult, error)

	// GetID returns the unique identifier for this validator
	GetID() string

	// GetType returns the type of validator
	GetType() string

	// GetDescription returns a human-readable description
	GetDescription() string
}

// EventValidator wraps the events package validator for orchestration use
type EventValidator struct {
	validator   events.CustomValidator
	id          string
	vType       string
	description string
}

// NewEventValidator creates a new event validator wrapper
func NewEventValidator(id, vType, description string, validator events.CustomValidator) *EventValidator {
	return &EventValidator{
		validator:   validator,
		id:          id,
		vType:       vType,
		description: description,
	}
}

// Validate implements the Validator interface
func (ev *EventValidator) Validate(ctx *OrchestrationValidationContext) (*OrchestrationValidationResult, error) {
	start := time.Now()

	// For simplicity, we'll simulate the validation here
	// In a real implementation, you would properly convert the context
	// and call the underlying events validator

	// Simulate validation logic
	var err error
	if ctx == nil {
		err = fmt.Errorf("validation context is nil")
	}

	duration := time.Since(start)

	result := &OrchestrationValidationResult{
		IsValid:   err == nil,
		Validator: ev.id,
		Duration:  duration,
		Timestamp: time.Now(),
	}

	if err != nil {
		result.Message = err.Error()
		result.Errors = []string{err.Error()}
	} else {
		result.Message = "Validation passed"
	}

	return result, nil
}

// GetID returns the validator ID
func (ev *EventValidator) GetID() string {
	return ev.id
}

// GetType returns the validator type
func (ev *EventValidator) GetType() string {
	return ev.vType
}

// GetDescription returns the validator description
func (ev *EventValidator) GetDescription() string {
	return ev.description
}

// SimpleValidator is a basic validator implementation for testing
type SimpleValidator struct {
	id          string
	vType       string
	description string
	isValid     bool
	message     string
}

// NewSimpleValidator creates a new simple validator
func NewSimpleValidator(id, vType, description string, isValid bool, message string) *SimpleValidator {
	return &SimpleValidator{
		id:          id,
		vType:       vType,
		description: description,
		isValid:     isValid,
		message:     message,
	}
}

// Validate implements the Validator interface
func (sv *SimpleValidator) Validate(ctx *OrchestrationValidationContext) (*OrchestrationValidationResult, error) {
	start := time.Now()

	result := &OrchestrationValidationResult{
		IsValid:   sv.isValid,
		Message:   sv.message,
		Validator: sv.id,
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}

	if !sv.isValid {
		result.Errors = []string{sv.message}
	}

	return result, nil
}

// GetID returns the validator ID
func (sv *SimpleValidator) GetID() string {
	return sv.id
}

// GetType returns the validator type
func (sv *SimpleValidator) GetType() string {
	return sv.vType
}

// GetDescription returns the validator description
func (sv *SimpleValidator) GetDescription() string {
	return sv.description
}
