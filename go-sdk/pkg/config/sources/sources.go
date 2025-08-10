// Package sources provides various configuration sources for the config system
package sources

import (
	"context"
	"time"
)

// Source represents a configuration source interface
// This is a duplicate of the interface in the parent package to avoid circular imports
type Source interface {
	Name() string
	Priority() int
	Load(ctx context.Context) (map[string]interface{}, error)
	Watch(ctx context.Context, callback func(map[string]interface{})) error
	CanWatch() bool
	LastModified() time.Time
}