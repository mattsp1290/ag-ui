package transport

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ValidationSeverity defines the severity level of validation errors
type ValidationSeverity int

const (
	// SeverityInfo represents informational messages
	SeverityInfo ValidationSeverity = iota
	// SeverityWarning represents warnings that don't prevent processing
	SeverityWarning
	// SeverityError represents errors that prevent processing
	SeverityError
	// SeverityFatal represents critical errors that require immediate attention
	SeverityFatal
)

// String returns the string representation of the severity level
func (s ValidationSeverity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	case SeverityFatal:
		return "fatal"
	default:
		return "unknown"
	}
}

// ParseSeverity parses a string into a ValidationSeverity
func ParseSeverity(s string) ValidationSeverity {
	switch strings.ToLower(s) {
	case "info":
		return SeverityInfo
	case "warning":
		return SeverityWarning
	case "error":
		return SeverityError
	case "fatal":
		return SeverityFatal
	default:
		return SeverityError
	}
}

// ValidationIssue represents a single validation issue with rich context
type ValidationIssue struct {
	// Message is the human-readable error message
	Message string `json:"message"`
	
	// Code is a machine-readable error code
	Code string `json:"code,omitempty"`
	
	// Severity indicates the severity level of this issue
	Severity ValidationSeverity `json:"severity"`
	
	// Field is the field path where the issue occurred
	Field string `json:"field,omitempty"`
	
	// Value is the actual value that caused the issue (for debugging)
	Value interface{} `json:"value,omitempty"`
	
	// ExpectedValue is what was expected (for comparison errors)
	ExpectedValue interface{} `json:"expected_value,omitempty"`
	
	// Suggestion provides a hint on how to fix the issue
	Suggestion string `json:"suggestion,omitempty"`
	
	// Context provides additional context about the validation
	Context map[string]interface{} `json:"context,omitempty"`
	
	// Timestamp when the issue was detected
	Timestamp time.Time `json:"timestamp"`
	
	// Validator is the name of the validator that generated this issue
	Validator string `json:"validator,omitempty"`
	
	// RuleID identifies the specific validation rule that failed
	RuleID string `json:"rule_id,omitempty"`
	
	// Category groups related validation issues
	Category string `json:"category,omitempty"`
	
	// Tags allow for flexible categorization and filtering
	Tags []string `json:"tags,omitempty"`
	
	// Details contains additional structured information about the issue
	Details map[string]interface{} `json:"details,omitempty"`
}

// NewValidationIssue creates a new validation issue
func NewValidationIssue(message string, severity ValidationSeverity) *ValidationIssue {
	return &ValidationIssue{
		Message:   message,
		Severity:  severity,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
		Details:   make(map[string]interface{}),
	}
}

// WithCode sets the error code
func (i *ValidationIssue) WithCode(code string) *ValidationIssue {
	i.Code = code
	return i
}

// WithField sets the field path
func (i *ValidationIssue) WithField(field string) *ValidationIssue {
	i.Field = field
	return i
}

// WithValue sets the problematic value
func (i *ValidationIssue) WithValue(value interface{}) *ValidationIssue {
	i.Value = value
	return i
}

// WithExpectedValue sets the expected value
func (i *ValidationIssue) WithExpectedValue(expected interface{}) *ValidationIssue {
	i.ExpectedValue = expected
	return i
}

// WithSuggestion sets a suggestion for fixing the issue
func (i *ValidationIssue) WithSuggestion(suggestion string) *ValidationIssue {
	i.Suggestion = suggestion
	return i
}

// WithValidator sets the validator name
func (i *ValidationIssue) WithValidator(validator string) *ValidationIssue {
	i.Validator = validator
	return i
}

// WithRuleID sets the rule ID
func (i *ValidationIssue) WithRuleID(ruleID string) *ValidationIssue {
	i.RuleID = ruleID
	return i
}

// WithCategory sets the category
func (i *ValidationIssue) WithCategory(category string) *ValidationIssue {
	i.Category = category
	return i
}

// WithTags sets the tags
func (i *ValidationIssue) WithTags(tags ...string) *ValidationIssue {
	i.Tags = tags
	return i
}

// AddTag adds a tag
func (i *ValidationIssue) AddTag(tag string) *ValidationIssue {
	i.Tags = append(i.Tags, tag)
	return i
}

// WithContext adds context information
func (i *ValidationIssue) WithContext(key string, value interface{}) *ValidationIssue {
	if i.Context == nil {
		i.Context = make(map[string]interface{})
	}
	i.Context[key] = value
	return i
}

// WithDetail adds detail information
func (i *ValidationIssue) WithDetail(key string, value interface{}) *ValidationIssue {
	if i.Details == nil {
		i.Details = make(map[string]interface{})
	}
	i.Details[key] = value
	return i
}

// Error implements the error interface
func (i *ValidationIssue) Error() string {
	var parts []string
	
	// Add severity prefix
	if i.Severity != SeverityError {
		parts = append(parts, fmt.Sprintf("[%s]", i.Severity.String()))
	}
	
	// Add field context
	if i.Field != "" {
		parts = append(parts, fmt.Sprintf("field '%s':", i.Field))
	}
	
	// Add main message
	parts = append(parts, i.Message)
	
	// Add code if present
	if i.Code != "" {
		parts = append(parts, fmt.Sprintf("(code: %s)", i.Code))
	}
	
	return strings.Join(parts, " ")
}

// String returns a detailed string representation
func (i *ValidationIssue) String() string {
	var parts []string
	
	// Basic error information
	parts = append(parts, i.Error())
	
	// Add value information if present
	if i.Value != nil {
		parts = append(parts, fmt.Sprintf("actual: %v", i.Value))
	}
	if i.ExpectedValue != nil {
		parts = append(parts, fmt.Sprintf("expected: %v", i.ExpectedValue))
	}
	
	// Add suggestion if present
	if i.Suggestion != "" {
		parts = append(parts, fmt.Sprintf("suggestion: %s", i.Suggestion))
	}
	
	return strings.Join(parts, ", ")
}

// IsError returns true if this is an error-level or fatal-level issue
func (i *ValidationIssue) IsError() bool {
	return i.Severity >= SeverityError
}

// IsWarning returns true if this is a warning-level issue
func (i *ValidationIssue) IsWarning() bool {
	return i.Severity == SeverityWarning
}

// IsFatal returns true if this is a fatal-level issue
func (i *ValidationIssue) IsFatal() bool {
	return i.Severity == SeverityFatal
}

// ValidationResult aggregates validation results with rich error reporting
type ValidationResult struct {
	// Valid indicates whether validation passed
	valid bool
	
	// Issues contains all validation issues found
	issues []*ValidationIssue
	
	// FieldIssues maps field paths to their specific issues
	fieldIssues map[string][]*ValidationIssue
	
	// Summary provides a high-level summary of the validation
	summary *ValidationSummary
	
	// Metadata contains additional information about the validation process
	metadata map[string]interface{}
	
	// Timestamp when the validation was performed
	timestamp time.Time
}

// ValidationSummary provides a high-level overview of validation results
type ValidationSummary struct {
	// TotalIssues is the total number of issues found
	TotalIssues int `json:"total_issues"`
	
	// ErrorCount is the number of error-level issues
	ErrorCount int `json:"error_count"`
	
	// WarningCount is the number of warning-level issues
	WarningCount int `json:"warning_count"`
	
	// InfoCount is the number of info-level issues
	InfoCount int `json:"info_count"`
	
	// FatalCount is the number of fatal-level issues
	FatalCount int `json:"fatal_count"`
	
	// FieldsWithIssues is the number of fields that have issues
	FieldsWithIssues int `json:"fields_with_issues"`
	
	// Categories lists all issue categories found
	Categories []string `json:"categories"`
	
	// Validators lists all validators that reported issues
	Validators []string `json:"validators"`
	
	// HasErrors indicates if there are any error-level or fatal-level issues
	HasErrors bool `json:"has_errors"`
	
	// HasWarnings indicates if there are any warning-level issues
	HasWarnings bool `json:"has_warnings"`
}

// NewValidationResult creates a new validation result
func NewValidationResult(valid bool) ValidationResult {
	return ValidationResult{
		valid:       valid,
		issues:      make([]*ValidationIssue, 0),
		fieldIssues: make(map[string][]*ValidationIssue),
		metadata:    make(map[string]interface{}),
		timestamp:   time.Now(),
	}
}

// IsValid returns whether the validation passed
func (r *ValidationResult) IsValid() bool {
	return r.valid && len(r.issues) == 0
}

// SetValid sets the validation status
func (r *ValidationResult) SetValid(valid bool) {
	r.valid = valid
}

// AddIssue adds a validation issue
func (r *ValidationResult) AddIssue(issue *ValidationIssue) {
	r.issues = append(r.issues, issue)
	
	// Mark as invalid if this is an error-level issue
	if issue.IsError() {
		r.valid = false
	}
	
	// Add to field-specific issues if field is specified
	if issue.Field != "" {
		r.fieldIssues[issue.Field] = append(r.fieldIssues[issue.Field], issue)
	}
	
	// Invalidate cached summary
	r.summary = nil
}

// AddError adds an error-level validation issue
func (r *ValidationResult) AddError(err error) {
	var issue *ValidationIssue
	
	// Try to extract ValidationIssue if the error is already one
	if validationErr, ok := err.(*ValidationError); ok {
		// Convert ValidationError to ValidationIssue
		issue = NewValidationIssue(validationErr.Error(), SeverityError)
	} else if existingIssue, ok := err.(*ValidationIssue); ok {
		issue = existingIssue
	} else {
		issue = NewValidationIssue(err.Error(), SeverityError)
	}
	
	r.AddIssue(issue)
}

// AddWarning adds a warning-level validation issue
func (r *ValidationResult) AddWarning(message string) {
	issue := NewValidationIssue(message, SeverityWarning)
	r.AddIssue(issue)
}

// AddInfo adds an info-level validation issue
func (r *ValidationResult) AddInfo(message string) {
	issue := NewValidationIssue(message, SeverityInfo)
	r.AddIssue(issue)
}

// AddFieldError adds an error for a specific field
func (r *ValidationResult) AddFieldError(field string, err error) {
	var issue *ValidationIssue
	
	if validationErr, ok := err.(*ValidationError); ok {
		issue = NewValidationIssue(validationErr.Error(), SeverityError).WithField(field)
	} else if existingIssue, ok := err.(*ValidationIssue); ok {
		existingIssue.Field = field
		issue = existingIssue
	} else {
		issue = NewValidationIssue(err.Error(), SeverityError).WithField(field)
	}
	
	r.AddIssue(issue)
}

// AddFieldWarning adds a warning for a specific field
func (r *ValidationResult) AddFieldWarning(field, message string) {
	issue := NewValidationIssue(message, SeverityWarning).WithField(field)
	r.AddIssue(issue)
}

// AddFieldInfo adds an info message for a specific field
func (r *ValidationResult) AddFieldInfo(field, message string) {
	issue := NewValidationIssue(message, SeverityInfo).WithField(field)
	r.AddIssue(issue)
}

// Issues returns all validation issues
func (r *ValidationResult) Issues() []*ValidationIssue {
	return r.issues
}

// Errors returns all error-level and fatal-level issues
func (r *ValidationResult) Errors() []error {
	var errors []error
	for _, issue := range r.issues {
		if issue.IsError() {
			errors = append(errors, issue)
		}
	}
	return errors
}

// Warnings returns all warning-level issues
func (r *ValidationResult) Warnings() []*ValidationIssue {
	var warnings []*ValidationIssue
	for _, issue := range r.issues {
		if issue.IsWarning() {
			warnings = append(warnings, issue)
		}
	}
	return warnings
}

// FieldIssues returns issues for a specific field
func (r *ValidationResult) FieldIssues(field string) []*ValidationIssue {
	return r.fieldIssues[field]
}

// AllFieldIssues returns all field-specific issues
func (r *ValidationResult) FieldErrors() map[string][]error {
	result := make(map[string][]error)
	for field, issues := range r.fieldIssues {
		errors := make([]error, 0, len(issues))
		for _, issue := range issues {
			if issue.IsError() {
				errors = append(errors, issue)
			}
		}
		if len(errors) > 0 {
			result[field] = errors
		}
	}
	return result
}

// GetSummary returns a summary of the validation results
func (r *ValidationResult) GetSummary() *ValidationSummary {
	if r.summary == nil {
		r.summary = r.computeSummary()
	}
	return r.summary
}

// computeSummary computes the validation summary
func (r *ValidationResult) computeSummary() *ValidationSummary {
	summary := &ValidationSummary{
		TotalIssues: len(r.issues),
		Categories:  make([]string, 0),
		Validators:  make([]string, 0),
	}
	
	categorySet := make(map[string]bool)
	validatorSet := make(map[string]bool)
	
	// Count issues by severity and collect categories/validators
	for _, issue := range r.issues {
		switch issue.Severity {
		case SeverityInfo:
			summary.InfoCount++
		case SeverityWarning:
			summary.WarningCount++
			summary.HasWarnings = true
		case SeverityError:
			summary.ErrorCount++
			summary.HasErrors = true
		case SeverityFatal:
			summary.FatalCount++
			summary.HasErrors = true
		}
		
		if issue.Category != "" && !categorySet[issue.Category] {
			categorySet[issue.Category] = true
			summary.Categories = append(summary.Categories, issue.Category)
		}
		
		if issue.Validator != "" && !validatorSet[issue.Validator] {
			validatorSet[issue.Validator] = true
			summary.Validators = append(summary.Validators, issue.Validator)
		}
	}
	
	// Count fields with issues
	summary.FieldsWithIssues = len(r.fieldIssues)
	
	// Sort categories and validators for consistent output
	sort.Strings(summary.Categories)
	sort.Strings(summary.Validators)
	
	return summary
}

// WithMetadata adds metadata to the validation result
func (r *ValidationResult) WithMetadata(key string, value interface{}) *ValidationResult {
	r.metadata[key] = value
	return r
}

// GetMetadata retrieves metadata from the validation result
func (r *ValidationResult) GetMetadata(key string) (interface{}, bool) {
	value, exists := r.metadata[key]
	return value, exists
}

// Merge combines another validation result into this one
func (r *ValidationResult) Merge(other ValidationResult) {
	// Merge validity (false if either is false)
	r.valid = r.valid && other.valid
	
	// Merge issues
	r.issues = append(r.issues, other.issues...)
	
	// Merge field issues
	for field, issues := range other.fieldIssues {
		r.fieldIssues[field] = append(r.fieldIssues[field], issues...)
	}
	
	// Merge metadata
	for key, value := range other.metadata {
		r.metadata[key] = value
	}
	
	// Invalidate cached summary
	r.summary = nil
}

// Filter returns a new ValidationResult containing only issues that match the filter
func (r *ValidationResult) Filter(filter func(*ValidationIssue) bool) ValidationResult {
	filtered := NewValidationResult(r.valid)
	
	for _, issue := range r.issues {
		if filter(issue) {
			filtered.AddIssue(issue)
		}
	}
	
	// Copy metadata
	for key, value := range r.metadata {
		filtered.metadata[key] = value
	}
	
	return filtered
}

// FilterBySeverity returns issues of a specific severity level
func (r *ValidationResult) FilterBySeverity(severity ValidationSeverity) []*ValidationIssue {
	var filtered []*ValidationIssue
	for _, issue := range r.issues {
		if issue.Severity == severity {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// FilterByField returns issues for a specific field
func (r *ValidationResult) FilterByField(field string) []*ValidationIssue {
	return r.fieldIssues[field]
}

// FilterByCategory returns issues of a specific category
func (r *ValidationResult) FilterByCategory(category string) []*ValidationIssue {
	var filtered []*ValidationIssue
	for _, issue := range r.issues {
		if issue.Category == category {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// FilterByValidator returns issues from a specific validator
func (r *ValidationResult) FilterByValidator(validator string) []*ValidationIssue {
	var filtered []*ValidationIssue
	for _, issue := range r.issues {
		if issue.Validator == validator {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// Error implements the error interface
func (r *ValidationResult) Error() string {
	if r.IsValid() {
		return ""
	}
	
	var messages []string
	for _, issue := range r.issues {
		if issue.IsError() {
			messages = append(messages, issue.Error())
		}
	}
	
	if len(messages) == 0 {
		return "validation failed"
	}
	
	return fmt.Sprintf("validation failed: %s", strings.Join(messages, "; "))
}

// ToJSON serializes the validation result to JSON
func (r *ValidationResult) ToJSON() ([]byte, error) {
	result := map[string]interface{}{
		"valid":        r.IsValid(),
		"issues":       r.issues,
		"summary":      r.GetSummary(),
		"metadata":     r.metadata,
		"timestamp":    r.timestamp,
		"field_issues": r.fieldIssues,
	}
	
	return json.Marshal(result)
}

// FromJSON deserializes a validation result from JSON
func FromJSON(data []byte) (*ValidationResult, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	
	result := NewValidationResult(true)
	
	// Parse basic fields
	if valid, ok := raw["valid"].(bool); ok {
		result.valid = valid
	}
	
	if timestamp, ok := raw["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
			result.timestamp = t
		}
	}
	
	// Parse metadata
	if metadata, ok := raw["metadata"].(map[string]interface{}); ok {
		result.metadata = metadata
	}
	
	// Parse issues
	if issuesRaw, ok := raw["issues"].([]interface{}); ok {
		for _, issueRaw := range issuesRaw {
			if issueData, ok := issueRaw.(map[string]interface{}); ok {
				issue := parseIssueFromMap(issueData)
				if issue != nil {
					result.AddIssue(issue)
				}
			}
		}
	}
	
	return &result, nil
}

// parseIssueFromMap parses a ValidationIssue from a map
func parseIssueFromMap(data map[string]interface{}) *ValidationIssue {
	issue := &ValidationIssue{
		Context: make(map[string]interface{}),
		Details: make(map[string]interface{}),
	}
	
	// Parse basic fields
	if message, ok := data["message"].(string); ok {
		issue.Message = message
	}
	
	if code, ok := data["code"].(string); ok {
		issue.Code = code
	}
	
	if severity, ok := data["severity"].(string); ok {
		issue.Severity = ParseSeverity(severity)
	} else if severityNum, ok := data["severity"].(float64); ok {
		issue.Severity = ValidationSeverity(int(severityNum))
	}
	
	if field, ok := data["field"].(string); ok {
		issue.Field = field
	}
	
	if value, ok := data["value"]; ok {
		issue.Value = value
	}
	
	if expected, ok := data["expected_value"]; ok {
		issue.ExpectedValue = expected
	}
	
	if suggestion, ok := data["suggestion"].(string); ok {
		issue.Suggestion = suggestion
	}
	
	if validator, ok := data["validator"].(string); ok {
		issue.Validator = validator
	}
	
	if ruleID, ok := data["rule_id"].(string); ok {
		issue.RuleID = ruleID
	}
	
	if category, ok := data["category"].(string); ok {
		issue.Category = category
	}
	
	if timestamp, ok := data["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
			issue.Timestamp = t
		}
	}
	
	// Parse tags
	if tagsRaw, ok := data["tags"].([]interface{}); ok {
		for _, tagRaw := range tagsRaw {
			if tag, ok := tagRaw.(string); ok {
				issue.Tags = append(issue.Tags, tag)
			}
		}
	}
	
	// Parse context
	if context, ok := data["context"].(map[string]interface{}); ok {
		issue.Context = context
	}
	
	// Parse details
	if details, ok := data["details"].(map[string]interface{}); ok {
		issue.Details = details
	}
	
	return issue
}

// Localized error messages support

// LocalizationProvider defines an interface for providing localized error messages
type LocalizationProvider interface {
	// GetMessage returns a localized message for the given key and parameters
	GetMessage(key string, params map[string]interface{}) string
	
	// GetSuggestion returns a localized suggestion for the given key and parameters
	GetSuggestion(key string, params map[string]interface{}) string
	
	// SupportedLanguages returns a list of supported language codes
	SupportedLanguages() []string
}

// DefaultLocalizationProvider provides English messages
type DefaultLocalizationProvider struct{}

// GetMessage returns the default English message
func (p *DefaultLocalizationProvider) GetMessage(key string, params map[string]interface{}) string {
	// Default messages for common validation scenarios
	messages := map[string]string{
		"required":           "field is required",
		"invalid_format":     "field has invalid format",
		"out_of_range":       "value is out of allowed range",
		"too_short":          "value is too short",
		"too_long":           "value is too long",
		"invalid_email":      "invalid email address",
		"invalid_url":        "invalid URL format",
		"passwords_mismatch": "passwords do not match",
		"invalid_date":       "invalid date format",
		"future_date":        "date must be in the future",
		"past_date":          "date must be in the past",
	}
	
	if message, exists := messages[key]; exists {
		return message
	}
	
	return key // Fallback to key if no message found
}

// GetSuggestion returns a default suggestion
func (p *DefaultLocalizationProvider) GetSuggestion(key string, params map[string]interface{}) string {
	suggestions := map[string]string{
		"required":           "provide a value for this field",
		"invalid_email":      "use format: user@example.com",
		"invalid_url":        "use format: https://example.com",
		"passwords_mismatch": "ensure both password fields match",
		"too_short":          "enter more characters",
		"too_long":           "enter fewer characters",
	}
	
	if suggestion, exists := suggestions[key]; exists {
		return suggestion
	}
	
	return ""
}

// SupportedLanguages returns supported languages
func (p *DefaultLocalizationProvider) SupportedLanguages() []string {
	return []string{"en"}
}

// Global localization provider
var globalLocalizationProvider LocalizationProvider = &DefaultLocalizationProvider{}

// SetLocalizationProvider sets the global localization provider
func SetLocalizationProvider(provider LocalizationProvider) {
	globalLocalizationProvider = provider
}

// GetLocalizationProvider returns the current localization provider
func GetLocalizationProvider() LocalizationProvider {
	return globalLocalizationProvider
}

// NewLocalizedValidationIssue creates a validation issue with localized messages
func NewLocalizedValidationIssue(messageKey string, severity ValidationSeverity, params map[string]interface{}) *ValidationIssue {
	provider := GetLocalizationProvider()
	
	issue := &ValidationIssue{
		Message:   provider.GetMessage(messageKey, params),
		Code:      messageKey,
		Severity:  severity,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
		Details:   make(map[string]interface{}),
	}
	
	// Add suggestion if available
	if suggestion := provider.GetSuggestion(messageKey, params); suggestion != "" {
		issue.Suggestion = suggestion
	}
	
	// Add parameters to context
	if params != nil {
		for key, value := range params {
			issue.WithContext(key, value)
		}
	}
	
	return issue
}

// Validation result aggregator for complex validation scenarios

// ValidationResultAggregator aggregates multiple validation results
type ValidationResultAggregator struct {
	results []ValidationResult
	options AggregatorOptions
}

// AggregatorOptions configures how validation results are aggregated
type AggregatorOptions struct {
	// StopOnFirstError stops aggregation when the first error is encountered
	StopOnFirstError bool
	
	// MaxIssues limits the total number of issues to collect
	MaxIssues int
	
	// IncludeWarnings includes warning-level issues in the aggregation
	IncludeWarnings bool
	
	// IncludeInfo includes info-level issues in the aggregation
	IncludeInfo bool
	
	// DuplicateDetection enables detection and removal of duplicate issues
	DuplicateDetection bool
}

// NewValidationResultAggregator creates a new result aggregator
func NewValidationResultAggregator(options AggregatorOptions) *ValidationResultAggregator {
	return &ValidationResultAggregator{
		results: make([]ValidationResult, 0),
		options: options,
	}
}

// Add adds a validation result to the aggregator
func (a *ValidationResultAggregator) Add(result ValidationResult) bool {
	// Check if we should stop on first error
	if a.options.StopOnFirstError && !result.IsValid() {
		a.results = append(a.results, result)
		return false // Signal to stop
	}
	
	a.results = append(a.results, result)
	return true // Continue aggregating
}

// Aggregate returns the final aggregated validation result
func (a *ValidationResultAggregator) Aggregate() ValidationResult {
	if len(a.results) == 0 {
		return NewValidationResult(true)
	}
	
	// Start with a valid result
	aggregated := NewValidationResult(true)
	
	// Track issue count
	issueCount := 0
	
	// Track duplicates if enabled
	var seenIssues map[string]bool
	if a.options.DuplicateDetection {
		seenIssues = make(map[string]bool)
	}
	
	// Merge all results
	for _, result := range a.results {
		// Check validity
		if !result.IsValid() {
			aggregated.SetValid(false)
		}
		
		// Add issues based on options
		for _, issue := range result.Issues() {
			// Check issue count limit
			if a.options.MaxIssues > 0 && issueCount >= a.options.MaxIssues {
				break
			}
			
			// Filter by severity
			include := true
			switch issue.Severity {
			case SeverityWarning:
				include = a.options.IncludeWarnings
			case SeverityInfo:
				include = a.options.IncludeInfo
			case SeverityError, SeverityFatal:
				include = true // Always include errors
			}
			
			if !include {
				continue
			}
			
			// Check for duplicates
			if a.options.DuplicateDetection {
				issueKey := fmt.Sprintf("%s:%s:%s", issue.Field, issue.Code, issue.Message)
				if seenIssues[issueKey] {
					continue
				}
				seenIssues[issueKey] = true
			}
			
			aggregated.AddIssue(issue)
			issueCount++
		}
		
		// Merge metadata
		for key, value := range result.metadata {
			aggregated.WithMetadata(key, value)
		}
	}
	
	return aggregated
}