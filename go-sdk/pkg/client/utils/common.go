package utils

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// CommonUtils provides shared utility functions to reduce code duplication.
type CommonUtils struct{}

// JSONMarshalToMap converts a struct to a map using JSON marshaling.
// This is a common pattern used across multiple files.
func (cu *CommonUtils) JSONMarshalToMap(data interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, errors.WrapWithContext(err, "JSONMarshalToMap", "failed to marshal data to JSON")
	}

	var result map[string]interface{}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		return nil, errors.WrapWithContext(err, "JSONMarshalToMap", "failed to unmarshal JSON to map")
	}

	return result, nil
}

// JSONUnmarshalFromMap converts a map to a struct using JSON marshaling.
// This is a common pattern used across multiple files.
func (cu *CommonUtils) JSONUnmarshalFromMap(data map[string]interface{}, target interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return errors.WrapWithContext(err, "JSONUnmarshalFromMap", "failed to marshal map to JSON")
	}

	err = json.Unmarshal(jsonData, target)
	if err != nil {
		return errors.WrapWithContext(err, "JSONUnmarshalFromMap", "failed to unmarshal JSON to target")
	}

	return nil
}

// DeepCopyJSON performs a deep copy of an object using JSON marshaling.
// This is used in multiple places for cloning objects.
func (cu *CommonUtils) DeepCopyJSON(src, dst interface{}) error {
	data, err := json.Marshal(src)
	if err != nil {
		return errors.WrapWithContext(err, "DeepCopyJSON", "failed to marshal source")
	}

	err = json.Unmarshal(data, dst)
	if err != nil {
		return errors.WrapWithContext(err, "DeepCopyJSON", "failed to unmarshal to destination")
	}

	return nil
}

// CalculateChecksum calculates a simple MD5 checksum for data.
// Used across multiple files for data integrity verification.
func (cu *CommonUtils) CalculateChecksum(data []byte) string {
	return fmt.Sprintf("%x", md5.Sum(data))
}

// SafeStringBuilder provides a pre-allocated string builder for performance.
// Used in multiple files for efficient string concatenation.
type SafeStringBuilder struct {
	builder *strings.Builder
}

// NewSafeStringBuilder creates a new SafeStringBuilder with the given capacity.
func NewSafeStringBuilder(capacity int) *SafeStringBuilder {
	var builder strings.Builder
	builder.Grow(capacity)
	return &SafeStringBuilder{builder: &builder}
}

// WriteString writes a string to the builder.
func (ssb *SafeStringBuilder) WriteString(s string) {
	ssb.builder.WriteString(s)
}

// WriteByte writes a byte to the builder.
func (ssb *SafeStringBuilder) WriteByte(b byte) {
	ssb.builder.WriteByte(b)
}

// String returns the built string.
func (ssb *SafeStringBuilder) String() string {
	return ssb.builder.String()
}

// Reset resets the builder for reuse.
func (ssb *SafeStringBuilder) Reset() {
	ssb.builder.Reset()
}

// ContextWithDefaultTimeout creates a context with a default timeout if none exists.
// Used across multiple files for consistent timeout handling.
func (cu *CommonUtils) ContextWithDefaultTimeout(ctx context.Context, defaultTimeout time.Duration) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		// Context already has a timeout, use it as-is
		return ctx, func() {}
	}
	
	return context.WithTimeout(ctx, defaultTimeout)
}

// ValidateNotNil validates that a value is not nil, returning a validation error if it is.
// This reduces repetitive nil checking code.
func (cu *CommonUtils) ValidateNotNil(value interface{}, fieldName string) error {
	if value == nil {
		return errors.NewValidationError(fieldName, fieldName+" cannot be nil")
	}
	return nil
}

// ValidateNotEmpty validates that a string is not empty or whitespace-only.
func (cu *CommonUtils) ValidateNotEmpty(value, fieldName string) error {
	if strings.TrimSpace(value) == "" {
		return errors.NewValidationError(fieldName, fieldName+" cannot be empty")
	}
	return nil
}

// MergeMaps recursively merges two maps, with override values taking precedence.
// Used in multiple files for configuration merging.
func (cu *CommonUtils) MergeMaps(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(base)+len(override))

	// Copy base map
	for k, v := range base {
		result[k] = v
	}

	// Override with values from override map
	for k, v := range override {
		if baseVal, exists := result[k]; exists {
			// If both values are maps, merge recursively
			if baseMap, baseIsMap := baseVal.(map[string]interface{}); baseIsMap {
				if overrideMap, overrideIsMap := v.(map[string]interface{}); overrideIsMap {
					result[k] = cu.MergeMaps(baseMap, overrideMap)
					continue
				}
			}
		}
		result[k] = v
	}

	return result
}

// SafeClose safely closes a channel, recovering from any panics.
// Used for safe cleanup in multiple files.
func (cu *CommonUtils) SafeClose(ch chan<- interface{}) {
	defer func() {
		if r := recover(); r != nil {
			// Channel was already closed, ignore
		}
	}()
	close(ch)
}

// RetryWithBackoff executes a function with exponential backoff retry logic.
// Used across multiple files for resilient operations.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	BackoffFunc func(int, time.Duration) time.Duration
}

// DefaultRetryConfig returns a sensible default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		BackoffFunc: ExponentialBackoff,
	}
}

// ExponentialBackoff implements exponential backoff delay calculation.
func ExponentialBackoff(attempt int, baseDelay time.Duration) time.Duration {
	delay := baseDelay
	for i := 0; i < attempt; i++ {
		delay *= 2
	}
	return delay
}

// RetryWithConfig executes a function with the specified retry configuration.
func (cu *CommonUtils) RetryWithConfig(ctx context.Context, config *RetryConfig, fn func() error) error {
	var lastErr error
	
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if attempt < config.MaxAttempts-1 {
			delay := config.BackoffFunc(attempt, config.BaseDelay)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return errors.WrapWithContext(lastErr, "RetryWithConfig", "all retry attempts failed")
}

// NewCommonUtils creates a new CommonUtils instance.
func NewCommonUtils() *CommonUtils {
	return &CommonUtils{}
}