package transport

import (
	"context"
	"time"
)

// AuthProvider handles authentication for transport connections.
type AuthProvider interface {
	// GetCredentials returns authentication credentials.
	GetCredentials(ctx context.Context) (map[string]string, error)

	// RefreshCredentials refreshes authentication credentials.
	RefreshCredentials(ctx context.Context) error

	// IsValid returns true if the credentials are valid.
	IsValid() bool

	// ExpiresAt returns when the credentials expire.
	ExpiresAt() time.Time
}