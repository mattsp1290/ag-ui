package state

import "errors"

// Common errors for state management
var (
	// Security errors
	ErrPatchTooLarge    = errors.New("patch size exceeds maximum allowed size")
	ErrStateTooLarge    = errors.New("state size exceeds maximum allowed size")
	ErrJSONTooDeep      = errors.New("JSON structure exceeds maximum allowed depth")
	ErrPathTooLong      = errors.New("JSON pointer path exceeds maximum allowed length")
	ErrValueTooLarge    = errors.New("value size exceeds maximum allowed size")
	ErrStringTooLong    = errors.New("string length exceeds maximum allowed length")
	ErrArrayTooLong     = errors.New("array length exceeds maximum allowed length")
	ErrTooManyKeys      = errors.New("object has too many keys")
	ErrInvalidOperation = errors.New("invalid patch operation")
	ErrForbiddenPath    = errors.New("access to path is forbidden")

	// Rate limiting errors
	ErrRateLimited     = errors.New("rate limit exceeded")
	ErrTooManyContexts = errors.New("too many active contexts")

	// Validation errors
	ErrInvalidPatch    = errors.New("invalid patch format")
	ErrInvalidState    = errors.New("invalid state format")
	ErrInvalidMetadata = errors.New("invalid metadata format")

	// General errors
	ErrContextNotFound = errors.New("context not found")
	ErrStateNotFound   = errors.New("state not found")
	ErrManagerShutdown = errors.New("manager is shutting down")
)
