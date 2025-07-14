//go:build !redis
// +build !redis

package state

import (
	"context"
	"fmt"
)

// Redis Backend stub implementation when Redis is not available

func (r *RedisBackend) GetState(ctx context.Context, stateID string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("Redis backend not implemented")
}

func (r *RedisBackend) SetState(ctx context.Context, stateID string, state map[string]interface{}) error {
	return fmt.Errorf("Redis backend not implemented")
}

func (r *RedisBackend) DeleteState(ctx context.Context, stateID string) error {
	return fmt.Errorf("Redis backend not implemented")
}

func (r *RedisBackend) GetVersion(ctx context.Context, stateID string, versionID string) (*StateVersion, error) {
	return nil, fmt.Errorf("Redis backend not implemented")
}

func (r *RedisBackend) SaveVersion(ctx context.Context, stateID string, version *StateVersion) error {
	return fmt.Errorf("Redis backend not implemented")
}

func (r *RedisBackend) GetVersionHistory(ctx context.Context, stateID string, limit int) ([]*StateVersion, error) {
	return nil, fmt.Errorf("Redis backend not implemented")
}

func (r *RedisBackend) GetSnapshot(ctx context.Context, stateID string, snapshotID string) (*StateSnapshot, error) {
	return nil, fmt.Errorf("Redis backend not implemented")
}

func (r *RedisBackend) SaveSnapshot(ctx context.Context, stateID string, snapshot *StateSnapshot) error {
	return fmt.Errorf("Redis backend not implemented")
}

func (r *RedisBackend) ListSnapshots(ctx context.Context, stateID string) ([]*StateSnapshot, error) {
	return nil, fmt.Errorf("Redis backend not implemented")
}

func (r *RedisBackend) BeginTransaction(ctx context.Context) (Transaction, error) {
	return nil, fmt.Errorf("Redis backend not implemented")
}

func (r *RedisBackend) Close() error {
	return nil
}

func (r *RedisBackend) Ping(ctx context.Context) error {
	return nil
}

func (r *RedisBackend) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return basic stats without Redis-specific metrics
	stats := make(map[string]interface{})
	for k, v := range r.stats {
		stats[k] = v
	}

	return stats
}

// RedisTransaction stub methods
func (t *RedisTransaction) SetState(ctx context.Context, stateID string, state map[string]interface{}) error {
	return fmt.Errorf("Redis backend not implemented")
}

func (t *RedisTransaction) SaveVersion(ctx context.Context, stateID string, version *StateVersion) error {
	return fmt.Errorf("Redis backend not implemented")
}

func (t *RedisTransaction) Commit(ctx context.Context) error {
	return fmt.Errorf("Redis backend not implemented")
}

func (t *RedisTransaction) Rollback(ctx context.Context) error {
	return fmt.Errorf("Redis backend not implemented")
}
