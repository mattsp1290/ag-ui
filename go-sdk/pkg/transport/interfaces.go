// Package transport provides a comprehensive transport abstraction system with
// composable interfaces organized across multiple focused files.
//
// The interfaces have been refactored into smaller, more focused files:
// - interfaces_core.go: Core transport interfaces
// - interfaces_stats.go: Statistics and metrics
// - interfaces_config.go: Configuration interfaces  
// - interfaces_state.go: Connection state management
// - interfaces_middleware.go: Middleware and filtering
// - interfaces_manager.go: Transport management
// - interfaces_serialization.go: Serialization and compression
// - interfaces_health.go: Health checking
// - interfaces_metrics.go: Metrics collection
// - interfaces_auth.go: Authentication
// - interfaces_resilience.go: Retry and circuit breaker
// - interfaces_io.go: I/O abstractions
// - interfaces_events.go: Transport event types
//
// This organization provides better maintainability while keeping
// the core Transport interface simple and composable.
package transport

// Backward Compatibility Notes:
// - Deprecated methods have been removed in favor of the new composable approach
// - Transport interface now composes smaller, focused interfaces
// - StreamingTransport and ReliableTransport extend the core Transport interface
// - All functionality remains available through the new interface composition