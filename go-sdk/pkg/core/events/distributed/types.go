package distributed

import "time"

// This file contains local type definitions to avoid circular imports

// ValidationError represents a validation error with context
type ValidationError struct {
	RuleID      string                 `json:"rule_id"`
	EventID     string                 `json:"event_id,omitempty"`
	EventType   string                 `json:"event_type"`
	Message     string                 `json:"message"`
	Severity    ValidationSeverity     `json:"severity"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ValidationSeverity defines the severity level of validation errors
type ValidationSeverity int

const (
	ValidationSeverityError ValidationSeverity = iota
	ValidationSeverityWarning
	ValidationSeverityInfo
)

func (s ValidationSeverity) String() string {
	switch s {
	case ValidationSeverityError:
		return "ERROR"
	case ValidationSeverityWarning:
		return "WARNING"
	case ValidationSeverityInfo:
		return "INFO"
	default:
		return "UNKNOWN"
	}
}

// ValidationResult represents the result of validation
type ValidationResult struct {
	IsValid     bool               `json:"is_valid"`
	Errors      []*ValidationError `json:"errors,omitempty"`
	Warnings    []*ValidationError `json:"warnings,omitempty"`
	Information []*ValidationError `json:"information,omitempty"`
	EventCount  int                `json:"event_count"`
	Duration    time.Duration      `json:"duration"`
	Timestamp   time.Time          `json:"timestamp"`
}

// AddError adds a validation error to the result
func (r *ValidationResult) AddError(err *ValidationError) {
	r.Errors = append(r.Errors, err)
	r.IsValid = false
}

// AddWarning adds a validation warning to the result
func (r *ValidationResult) AddWarning(warning *ValidationError) {
	r.Warnings = append(r.Warnings, warning)
}

// AddInfo adds a validation info to the result
func (r *ValidationResult) AddInfo(info *ValidationError) {
	r.Information = append(r.Information, info)
}