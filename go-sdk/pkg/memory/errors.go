package memory

import "errors"

// Common memory management errors
var (
	// ErrAlreadyStarted is returned when trying to start an already started component
	ErrAlreadyStarted = errors.New("already started")

	// ErrInvalidTaskName is returned when a task name is invalid
	ErrInvalidTaskName = errors.New("invalid task name")

	// ErrInvalidCleanupFunc is returned when a cleanup function is invalid
	ErrInvalidCleanupFunc = errors.New("invalid cleanup function")

	// ErrTaskNotFound is returned when a cleanup task is not found
	ErrTaskNotFound = errors.New("task not found")

	// ErrBackpressureActive is returned when backpressure is active and blocking operations
	ErrBackpressureActive = errors.New("backpressure active")

	// ErrBackpressureTimeout is returned when backpressure timeout is exceeded
	ErrBackpressureTimeout = errors.New("backpressure timeout exceeded")

	// ErrConnectionClosed is returned when the connection is closed unexpectedly
	ErrConnectionClosed = errors.New("connection closed")
)