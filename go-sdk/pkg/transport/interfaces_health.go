package transport

import (
	"context"
	"time"
)

// HealthChecker provides health check capabilities for transports.
type HealthChecker interface {
	// CheckHealth performs a health check on the transport.
	CheckHealth(ctx context.Context) error

	// IsHealthy returns true if the transport is healthy.
	IsHealthy() bool

	// GetHealthStatus returns detailed health status information.
	GetHealthStatus() HealthStatus
}

// HealthStatus represents the health status of a transport.
type HealthStatus struct {
	Healthy    bool          `json:"healthy"`
	Timestamp  time.Time     `json:"timestamp"`
	Latency    time.Duration `json:"latency"`
	Error      string        `json:"error,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}