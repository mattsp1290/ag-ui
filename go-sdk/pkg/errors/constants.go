package errors

import (
	"strconv"
	"strings"
)

// Common error message constants to ensure consistency across the codebase.
// These constants prevent duplicate error messages and provide a centralized
// location for maintaining error message consistency.

// Parameter and input validation messages
const (
	// Common parameter validation messages
	MsgCannotBeEmpty   = "cannot be empty"
	MsgCannotBeNil     = "cannot be nil"
	MsgIsRequired      = "is required"
	MsgInvalidFormat   = "invalid format"
	MsgInvalidValue    = "invalid value"
	MsgExceedsMaximum  = "exceeds maximum"
	MsgBelowMinimum    = "below minimum"
	MsgTooLong         = "too long"
	MsgTooShort        = "too short"
	MsgContainsInvalid = "contains invalid characters"

	// Generic validation messages
	MsgValidationFailed      = "validation failed"
	MsgFieldValidationFailed = "field validation failed"
	MsgParameterValidation   = "parameter validation"
	MsgDataValidation        = "data validation"

	// Specific field validation messages
	MsgBaseURLCannotBeEmpty   = "base URL cannot be empty"
	MsgAgentNameCannotBeEmpty = "agent name cannot be empty"
	MsgEventCannotBeNil       = "event cannot be nil"
	MsgConfigCannotBeNil      = "config cannot be nil"
	MsgDataCannotBeEmpty      = "data cannot be empty"
	MsgPathCannotBeEmpty      = "path cannot be empty"
	MsgNameCannotBeEmpty      = "name cannot be empty"
	MsgIDCannotBeEmpty        = "ID cannot be empty"
	MsgTypeCannotBeEmpty      = "type cannot be empty"
	MsgEndpointCannotBeEmpty  = "endpoint cannot be empty"
	MsgParameterIsRequired    = "parameter is required"
	MsgFieldIsRequired        = "field is required"

	// Value constraint messages
	MsgMustBePositive          = "must be positive"
	MsgMustBeNonNegative       = "must be non-negative"
	MsgMustBeBetween           = "must be between"
	MsgCannotBeNegative        = "cannot be negative"
	MsgTimeoutCannotBeNegative = "timeout cannot be negative"
	MsgRetriesCannotBeNegative = "max_retries cannot be negative"
)

// Connection and transport error messages
const (
	// Connection lifecycle messages
	MsgNotConnected       = "not connected"
	MsgAlreadyConnected   = "already connected"
	MsgConnectionFailed   = "connection failed"
	MsgConnectionClosed   = "connection closed"
	MsgConnectionTimeout  = "connection timeout"
	MsgReconnectionFailed = "reconnection failed"

	// Operation status messages
	MsgOperationTimeout   = "operation timeout"
	MsgOperationCancelled = "operation cancelled"
	MsgOperationFailed    = "operation failed"
	MsgExecutionTimeout   = "execution timeout"
	MsgExecutionCancelled = "execution cancelled"
	MsgExecutionFailed    = "execution failed"

	// Transport specific messages
	MsgTransportNotFound     = "transport not found"
	MsgTransportNotSupported = "transport not supported"
	MsgStreamNotFound        = "stream not found"
	MsgStreamClosed          = "stream closed"
	MsgStreamingNotSupported = "streaming not supported"
)

// Encoding and data processing error messages
const (
	// Encoding/Decoding operations
	MsgEncodingFailed        = "encoding failed"
	MsgDecodingFailed        = "decoding failed"
	MsgSerializationFailed   = "serialization failed"
	MsgDeserializationFailed = "deserialization failed"
	MsgCompressionFailed     = "compression failed"
	MsgDecompressionFailed   = "decompression failed"
	MsgChunkingFailed        = "chunking failed"

	// Format and negotiation messages
	MsgFormatNotSupported   = "format not supported"
	MsgFormatNotRegistered  = "format not registered"
	MsgEncodingNotSupported = "encoding not supported"
	MsgMimeTypeInvalid      = "MIME type invalid"
	MsgNegotiationFailed    = "negotiation failed"
	MsgNoSuitableFormat     = "no suitable format found"

	// Data format messages
	MsgInvalidJSONFormat     = "invalid JSON format"
	MsgInvalidXMLFormat      = "invalid XML format"
	MsgInvalidProtobufFormat = "invalid protobuf format"
	MsgInvalidBase64         = "invalid base64"
	MsgInvalidDataFormat     = "invalid data format"
)

// System and resource error messages
const (
	// Not found messages
	MsgNotFound        = "not found"
	MsgToolNotFound    = "tool not found"
	MsgStateNotFound   = "state not found"
	MsgSessionNotFound = "session not found"
	MsgContextNotFound = "context not found"
	MsgHandlerNotFound = "handler not found"
	MsgMarkerNotFound  = "marker not found"
	MsgVersionNotFound = "version not found"
	MsgTaskNotFound    = "task not found"

	// Resource and limit messages
	MsgRateLimitExceeded     = "rate limit exceeded"
	MsgMaxConcurrencyReached = "maximum concurrent executions reached"
	MsgResourceExhausted     = "resource exhausted"
	MsgMemoryLimitExceeded   = "memory limit exceeded"
	MsgSizeLimitExceeded     = "size limit exceeded"
	MsgDepthLimitExceeded    = "depth limit exceeded"
	MsgMessageTooLarge       = "message too large"
	MsgPayloadTooLarge       = "payload too large"

	// State and lifecycle messages
	MsgAlreadyStarted     = "already started"
	MsgAlreadyStopped     = "already stopped"
	MsgAlreadyInitialized = "already initialized"
	MsgNotInitialized     = "not initialized"
	MsgManagerShutdown    = "manager is shutting down"
	MsgServiceUnavailable = "service unavailable"
)

// Implementation and feature messages
const (
	// Implementation status
	MsgNotImplemented         = "not implemented"
	MsgMethodNotImplemented   = "method not implemented"
	MsgFeatureNotImplemented  = "feature not implemented"
	MsgNotSupported           = "not supported"
	MsgCapabilityNotSupported = "capability not supported"
	MsgUnsupportedOperation   = "unsupported operation"

	// Configuration messages
	MsgInvalidConfiguration = "invalid configuration"
	MsgConfigurationError   = "configuration error"
	MsgMissingConfiguration = "missing configuration"
	MsgIncompatibleConfig   = "incompatible configuration"
)

// Security and validation error messages
const (
	// Security violations
	MsgSecurityViolation = "security violation"
	MsgAccessDenied      = "access denied"
	MsgPermissionDenied  = "permission denied"
	MsgUnauthorized      = "unauthorized"
	MsgForbidden         = "forbidden"
	MsgPathForbidden     = "access to path is forbidden"

	// Security threats
	MsgXSSDetected             = "XSS pattern detected"
	MsgSQLInjectionDetected    = "SQL injection pattern detected"
	MsgScriptInjectionDetected = "script injection pattern detected"
	MsgPathTraversalDetected   = "path traversal detected"
	MsgHTMLNotAllowed          = "HTML content not allowed"
	MsgNullByteDetected        = "null byte detected"
	MsgInvalidUTF8             = "invalid UTF-8 encoding"

	// Attack patterns
	MsgDOSAttackDetected   = "denial of service attack detected"
	MsgEntityExpansion     = "XML entity expansion attack detected"
	MsgZipBombDetected     = "zip bomb detected"
	MsgExcessiveRepetition = "excessive character repetition detected"
	MsgSuspiciousPattern   = "suspicious pattern detected"
)

// Process and execution error messages
const (
	// Execution states
	MsgExecutionPanicked = "execution panicked"
	MsgToolPanicked      = "tool execution panicked"
	MsgProcessFailed     = "process failed"
	MsgTaskFailed        = "task failed"
	MsgCleanupFailed     = "cleanup failed"

	// Dependencies and circular references
	MsgCircularDependency = "circular dependency detected"
	MsgDependencyNotFound = "dependency not found"
	MsgDependencyFailed   = "dependency failed"

	// Retry and recovery
	MsgRetryExhausted   = "retry attempts exhausted"
	MsgBackoffExhausted = "backoff attempts exhausted"
	MsgRecoveryFailed   = "recovery failed"
)

// Backpressure and flow control messages
const (
	// Backpressure states
	MsgBackpressureActive  = "backpressure active"
	MsgBackpressureTimeout = "backpressure timeout exceeded"
	MsgFlowControlActive   = "flow control active"
	MsgBufferFull          = "buffer full"
	MsgQueueFull           = "queue full"
	MsgChannelFull         = "channel full"
)

// Context and cancellation messages
const (
	// Context states
	MsgContextCancelled        = "context cancelled"
	MsgContextDeadlineExceeded = "context deadline exceeded"
	MsgContextMissing          = "required context missing"
	MsgContextInvalid          = "invalid context"
)

// Health check and monitoring messages
const (
	// Health states
	MsgHealthCheckFailed  = "health check failed"
	MsgServiceUnhealthy   = "service unhealthy"
	MsgMetricsUnavailable = "metrics unavailable"
	MsgMonitoringDisabled = "monitoring disabled"
)

// Common error message prefixes and suffixes
const (
	// Operation context prefixes
	PrefixValidation    = "validation"
	PrefixEncoding      = "encoding"
	PrefixDecoding      = "decoding"
	PrefixSerialization = "serialization"
	PrefixConnection    = "connection"
	PrefixExecution     = "execution"
	PrefixConfiguration = "configuration"
	PrefixSecurity      = "security"
	PrefixState         = "state"
	PrefixTransport     = "transport"
	PrefixTool          = "tool"
	PrefixWebsocket     = "websocket"
	PrefixSSE           = "sse"
	PrefixHTTP          = "http"

	// Component context prefixes
	PrefixManager   = "manager"
	PrefixClient    = "client"
	PrefixServer    = "server"
	PrefixRegistry  = "registry"
	PrefixFactory   = "factory"
	PrefixProvider  = "provider"
	PrefixHandler   = "handler"
	PrefixProcessor = "processor"
	PrefixValidator = "validator"
	PrefixMonitor   = "monitor"

	// Common suffixes
	SuffixFailed       = "failed"
	SuffixTimeout      = "timeout"
	SuffixCancelled    = "cancelled"
	SuffixExceeded     = "exceeded"
	SuffixNotFound     = "not found"
	SuffixNotSupported = "not supported"
	SuffixInvalid      = "invalid"
	SuffixMissing      = "missing"
)

// Helper functions for consistent error message formatting

// FormatComponentError creates a consistent component error message
func FormatComponentError(component, operation, message string) string {
	if component == "" {
		var b strings.Builder
		b.Grow(len(operation) + 2 + len(message)) // operation + ": " + message
		b.WriteString(operation)
		b.WriteString(": ")
		b.WriteString(message)
		return b.String()
	}
	var b strings.Builder
	b.Grow(len(component) + 1 + len(operation) + 2 + len(message)) // component + " " + operation + ": " + message
	b.WriteString(component)
	b.WriteString(" ")
	b.WriteString(operation)
	b.WriteString(": ")
	b.WriteString(message)
	return b.String()
}

// FormatFieldError creates a consistent field validation error message
func FormatFieldError(field, message string) string {
	var b strings.Builder
	b.Grow(7 + len(field) + 2 + len(message)) // "field '" + field + "' " + message
	b.WriteString("field '")
	b.WriteString(field)
	b.WriteString("' ")
	b.WriteString(message)
	return b.String()
}

// FormatOperationError creates a consistent operation error message
func FormatOperationError(operation, message string) string {
	var b strings.Builder
	b.Grow(len(operation) + 1 + len(SuffixFailed) + 2 + len(message)) // operation + " " + SuffixFailed + ": " + message
	b.WriteString(operation)
	b.WriteString(" ")
	b.WriteString(SuffixFailed)
	b.WriteString(": ")
	b.WriteString(message)
	return b.String()
}

// FormatResourceError creates a consistent resource error message
func FormatResourceError(resourceType, resourceID, message string) string {
	if resourceID == "" {
		var b strings.Builder
		b.Grow(len(resourceType) + 1 + len(message)) // resourceType + " " + message
		b.WriteString(resourceType)
		b.WriteString(" ")
		b.WriteString(message)
		return b.String()
	}
	var b strings.Builder
	b.Grow(len(resourceType) + 2 + len(resourceID) + 2 + len(message)) // resourceType + " '" + resourceID + "' " + message
	b.WriteString(resourceType)
	b.WriteString(" '")
	b.WriteString(resourceID)
	b.WriteString("' ")
	b.WriteString(message)
	return b.String()
}

// FormatSecurityError creates a consistent security error message
func FormatSecurityError(violationType, location, message string) string {
	if location == "" {
		var b strings.Builder
		b.Grow(len(MsgSecurityViolation) + 2 + len(violationType) + 3 + len(message)) // MsgSecurityViolation + " (" + violationType + "): " + message
		b.WriteString(MsgSecurityViolation)
		b.WriteString(" (")
		b.WriteString(violationType)
		b.WriteString("): ")
		b.WriteString(message)
		return b.String()
	}
	var b strings.Builder
	b.Grow(len(MsgSecurityViolation) + 2 + len(violationType) + 5 + len(location) + 2 + len(message)) // MsgSecurityViolation + " (" + violationType + ") at " + location + ": " + message
	b.WriteString(MsgSecurityViolation)
	b.WriteString(" (")
	b.WriteString(violationType)
	b.WriteString(") at ")
	b.WriteString(location)
	b.WriteString(": ")
	b.WriteString(message)
	return b.String()
}

// FormatLimitError creates a consistent limit exceeded error message
func FormatLimitError(limitType string, current, maximum int64) string {
	var b strings.Builder
	// Estimate size: limitType + " limit exceeded: current=" + current + ", maximum=" + maximum
	// Numbers can be up to 20 chars each for int64
	b.Grow(len(limitType) + 41 + 40) // 41 for fixed text, 40 for two 20-char numbers
	b.WriteString(limitType)
	b.WriteString(" limit exceeded: current=")
	b.WriteString(strconv.FormatInt(current, 10))
	b.WriteString(", maximum=")
	b.WriteString(strconv.FormatInt(maximum, 10))
	return b.String()
}

// FormatNotImplementedError creates a consistent not implemented error message
func FormatNotImplementedError(component, method string) string {
	if component == "" {
		var b strings.Builder
		b.Grow(len(method) + 2 + len(MsgMethodNotImplemented)) // method + ": " + MsgMethodNotImplemented
		b.WriteString(method)
		b.WriteString(": ")
		b.WriteString(MsgMethodNotImplemented)
		return b.String()
	}
	var b strings.Builder
	b.Grow(len(component) + 1 + len(method) + 2 + len(MsgMethodNotImplemented)) // component + "." + method + ": " + MsgMethodNotImplemented
	b.WriteString(component)
	b.WriteString(".")
	b.WriteString(method)
	b.WriteString(": ")
	b.WriteString(MsgMethodNotImplemented)
	return b.String()
}

// FormatTimeoutError creates a consistent timeout error message
func FormatTimeoutError(operation, timeout string) string {
	var b strings.Builder
	b.Grow(len(operation) + 1 + len(MsgOperationTimeout) + 7 + len(timeout)) // operation + " " + MsgOperationTimeout + " after " + timeout
	b.WriteString(operation)
	b.WriteString(" ")
	b.WriteString(MsgOperationTimeout)
	b.WriteString(" after ")
	b.WriteString(timeout)
	return b.String()
}
