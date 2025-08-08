package config

import (
	"errors"
	"fmt"
	"strings"
)

// ConfigError represents a structured configuration error with context
type ConfigError struct {
	Op       string // Operation that failed (Load, Set, Get, Validate, etc.)
	Key      string // Config key involved (if applicable)
	Source   string // Source name (if applicable)
	Value    interface{} // Value that caused the error (if applicable)
	Err      error  // Underlying error
	Category Category // Error category for classification
}

// Category represents the type/category of configuration error
type Category int

const (
	// CategoryUnknown represents an unknown error category
	CategoryUnknown Category = iota
	
	// CategoryValidation represents validation failures
	CategoryValidation
	
	// CategorySource represents source-related errors (file not found, parse error, etc.)
	CategorySource
	
	// CategoryAccess represents access/permission errors
	CategoryAccess
	
	// CategoryType represents type conversion/casting errors
	CategoryType
	
	// CategoryKey represents key-related errors (not found, invalid format, etc.)
	CategoryKey
	
	// CategoryNetwork represents network-related errors (for remote sources)
	CategoryNetwork
	
	// CategoryTimeout represents timeout errors
	CategoryTimeout
	
	// CategorySecurity represents security-related errors
	CategorySecurity
)

// String returns the string representation of the error category
func (c Category) String() string {
	switch c {
	case CategoryValidation:
		return "validation"
	case CategorySource:
		return "source"
	case CategoryAccess:
		return "access"
	case CategoryType:
		return "type"
	case CategoryKey:
		return "key"
	case CategoryNetwork:
		return "network"
	case CategoryTimeout:
		return "timeout"
	case CategorySecurity:
		return "security"
	default:
		return "unknown"
	}
}

// IsTemporary returns true if the error is likely temporary and retry might succeed
func (c Category) IsTemporary() bool {
	switch c {
	case CategoryNetwork, CategoryTimeout:
		return true
	default:
		return false
	}
}

// Error implements the error interface
func (e *ConfigError) Error() string {
	var parts []string
	
	// Start with operation
	if e.Op != "" {
		parts = append(parts, fmt.Sprintf("config %s", e.Op))
	} else {
		parts = append(parts, "config operation")
	}
	
	// Add source context
	if e.Source != "" {
		parts = append(parts, fmt.Sprintf("source=%s", e.Source))
	}
	
	// Add key context
	if e.Key != "" {
		parts = append(parts, fmt.Sprintf("key=%s", e.Key))
	}
	
	// Add value context if relevant
	if e.Value != nil && e.Category == CategoryType {
		parts = append(parts, fmt.Sprintf("value=%v", e.Value))
	}
	
	// Add category
	if e.Category != CategoryUnknown {
		parts = append(parts, fmt.Sprintf("category=%s", e.Category))
	}
	
	message := strings.Join(parts, " ")
	
	// Add underlying error
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", message, e.Err)
	}
	
	return message
}

// Unwrap returns the underlying error for error unwrapping
func (e *ConfigError) Unwrap() error {
	return e.Err
}

// Is checks if the error matches the target error
func (e *ConfigError) Is(target error) bool {
	if target == nil {
		return false
	}
	
	// Check if target is also a ConfigError
	if t, ok := target.(*ConfigError); ok {
		return e.Op == t.Op && 
			   e.Key == t.Key && 
			   e.Source == t.Source &&
			   e.Category == t.Category
	}
	
	// Check underlying error
	return errors.Is(e.Err, target)
}

// WithOperation creates a new ConfigError with the specified operation
func WithOperation(op string, err error) *ConfigError {
	if err == nil {
		return nil
	}
	
	// If it's already a ConfigError, update the operation
	if configErr, ok := err.(*ConfigError); ok {
		newErr := *configErr
		if newErr.Op == "" {
			newErr.Op = op
		}
		return &newErr
	}
	
	return &ConfigError{
		Op:       op,
		Err:      err,
		Category: categorizeError(err),
	}
}

// WithKey creates a new ConfigError with the specified key
func WithKey(key string, err error) *ConfigError {
	if err == nil {
		return nil
	}
	
	if configErr, ok := err.(*ConfigError); ok {
		newErr := *configErr
		if newErr.Key == "" {
			newErr.Key = key
		}
		return &newErr
	}
	
	return &ConfigError{
		Key:      key,
		Err:      err,
		Category: categorizeError(err),
	}
}

// WithSource creates a new ConfigError with the specified source
func WithSource(source string, err error) *ConfigError {
	if err == nil {
		return nil
	}
	
	if configErr, ok := err.(*ConfigError); ok {
		newErr := *configErr
		if newErr.Source == "" {
			newErr.Source = source
		}
		return &newErr
	}
	
	return &ConfigError{
		Source:   source,
		Err:      err,
		Category: categorizeError(err),
	}
}

// WithValue creates a new ConfigError with the specified value
func WithValue(value interface{}, err error) *ConfigError {
	if err == nil {
		return nil
	}
	
	if configErr, ok := err.(*ConfigError); ok {
		newErr := *configErr
		if newErr.Value == nil {
			newErr.Value = value
		}
		return &newErr
	}
	
	return &ConfigError{
		Value:    value,
		Err:      err,
		Category: categorizeError(err),
	}
}

// WithCategory creates a new ConfigError with the specified category
func WithCategory(category Category, err error) *ConfigError {
	if err == nil {
		return nil
	}
	
	if configErr, ok := err.(*ConfigError); ok {
		newErr := *configErr
		if newErr.Category == CategoryUnknown {
			newErr.Category = category
		}
		return &newErr
	}
	
	return &ConfigError{
		Category: category,
		Err:      err,
	}
}

// NewConfigError creates a new ConfigError with all context
func NewConfigError(op, key, source string, value interface{}, category Category, err error) *ConfigError {
	return &ConfigError{
		Op:       op,
		Key:      key,
		Source:   source,
		Value:    value,
		Category: category,
		Err:      err,
	}
}

// categorizeError attempts to categorize an error based on its type and message
func categorizeError(err error) Category {
	if err == nil {
		return CategoryUnknown
	}
	
	errMsg := strings.ToLower(err.Error())
	
	// Check for validation errors
	if strings.Contains(errMsg, "validation") || 
	   strings.Contains(errMsg, "invalid") ||
	   strings.Contains(errMsg, "required") ||
	   strings.Contains(errMsg, "constraint") {
		return CategoryValidation
	}
	
	// Check for source errors
	if strings.Contains(errMsg, "no such file") ||
	   strings.Contains(errMsg, "parse") ||
	   strings.Contains(errMsg, "unmarshal") ||
	   strings.Contains(errMsg, "syntax") {
		return CategorySource
	}
	
	// Check for access errors
	if strings.Contains(errMsg, "permission denied") ||
	   strings.Contains(errMsg, "access") ||
	   strings.Contains(errMsg, "forbidden") {
		return CategoryAccess
	}
	
	// Check for type errors
	if strings.Contains(errMsg, "type") ||
	   strings.Contains(errMsg, "convert") ||
	   strings.Contains(errMsg, "cast") {
		return CategoryType
	}
	
	// Check for key errors
	if strings.Contains(errMsg, "key") ||
	   strings.Contains(errMsg, "not found") ||
	   strings.Contains(errMsg, "missing") {
		return CategoryKey
	}
	
	// Check for network errors
	if strings.Contains(errMsg, "network") ||
	   strings.Contains(errMsg, "connection") ||
	   strings.Contains(errMsg, "dial") {
		return CategoryNetwork
	}
	
	// Check for timeout errors
	if strings.Contains(errMsg, "timeout") ||
	   strings.Contains(errMsg, "deadline") {
		return CategoryTimeout
	}
	
	// Check for security errors
	if strings.Contains(errMsg, "security") ||
	   strings.Contains(errMsg, "path traversal") ||
	   strings.Contains(errMsg, "dangerous") {
		return CategorySecurity
	}
	
	return CategoryUnknown
}

// Common error sentinels
var (
	// ErrKeyNotFound indicates a configuration key was not found
	ErrKeyNotFound = errors.New("configuration key not found")
	
	// ErrInvalidValue indicates a configuration value is invalid
	ErrInvalidValue = errors.New("invalid configuration value")
	
	// ErrSourceNotFound indicates a configuration source was not found
	ErrSourceNotFound = errors.New("configuration source not found")
	
	// ErrValidationFailed indicates configuration validation failed
	ErrValidationFailed = errors.New("configuration validation failed")
	
	// ErrTypeMismatch indicates a type conversion failed
	ErrTypeMismatch = errors.New("configuration type mismatch")
	
	// ErrReadOnly indicates an attempt to modify read-only configuration
	ErrReadOnly = errors.New("configuration is read-only")
	
	// ErrShutdown indicates the configuration system has been shut down
	ErrShutdown = errors.New("configuration system is shut down")
	
	// ErrCircularDependency indicates a circular dependency in profiles
	ErrCircularDependency = errors.New("circular dependency detected")
	
	// ErrPathTraversal indicates a path traversal security violation
	ErrPathTraversal = errors.New("path traversal attempt detected")
	
	// ErrTimeout indicates an operation timed out
	ErrTimeout = errors.New("operation timed out")
)

// Error type checking helpers

// IsKeyNotFound checks if an error indicates a key was not found
func IsKeyNotFound(err error) bool {
	return errors.Is(err, ErrKeyNotFound)
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	if errors.Is(err, ErrValidationFailed) {
		return true
	}
	
	if configErr, ok := err.(*ConfigError); ok {
		return configErr.Category == CategoryValidation
	}
	
	return false
}

// IsSourceError checks if an error is a source-related error
func IsSourceError(err error) bool {
	if errors.Is(err, ErrSourceNotFound) {
		return true
	}
	
	if configErr, ok := err.(*ConfigError); ok {
		return configErr.Category == CategorySource
	}
	
	return false
}

// IsTemporary checks if an error is temporary and retry might succeed
func IsTemporary(err error) bool {
	if configErr, ok := err.(*ConfigError); ok {
		return configErr.Category.IsTemporary()
	}
	
	// Check for common temporary error types
	if errors.Is(err, ErrTimeout) {
		return true
	}
	
	return false
}

// IsSecurityError checks if an error is security-related
func IsSecurityError(err error) bool {
	if errors.Is(err, ErrPathTraversal) {
		return true
	}
	
	if configErr, ok := err.(*ConfigError); ok {
		return configErr.Category == CategorySecurity
	}
	
	return false
}

// GetCategory returns the error category, or CategoryUnknown if not a ConfigError
func GetCategory(err error) Category {
	if configErr, ok := err.(*ConfigError); ok {
		return configErr.Category
	}
	return CategoryUnknown
}

// GetOperation returns the operation that failed, or empty string if not a ConfigError
func GetOperation(err error) string {
	if configErr, ok := err.(*ConfigError); ok {
		return configErr.Op
	}
	return ""
}

// GetKey returns the configuration key involved, or empty string if not available
func GetKey(err error) string {
	if configErr, ok := err.(*ConfigError); ok {
		return configErr.Key
	}
	return ""
}

// GetSource returns the configuration source involved, or empty string if not available
func GetSource(err error) string {
	if configErr, ok := err.(*ConfigError); ok {
		return configErr.Source
	}
	return ""
}

