package state

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// This file contains basic implementations of storage backends
// For production use, replace these with actual Redis/PostgreSQL implementations

// MockRedisBackend provides a basic in-memory implementation for Redis backend
// Replace with actual Redis implementation when redis dependency is available
type MockRedisBackend struct {
	data   sync.Map // Thread-safe map for storing data
	config *StorageConfig
	logger Logger
}

// NewMockRedisBackend creates a mock Redis backend for testing/development
func NewMockRedisBackend(config *StorageConfig, logger Logger) (*MockRedisBackend, error) {
	backend := &MockRedisBackend{
		config: config,
		logger: logger,
	}

	logger.Info("Mock Redis storage backend initialized (development mode)",
		String("connection_url", config.ConnectionURL))

	return backend, nil
}

func (m *MockRedisBackend) GetState(ctx context.Context, stateID string) (map[string]interface{}, error) {
	if val, ok := m.data.Load("state:" + stateID); ok {
		if state, ok := val.(map[string]interface{}); ok {
			return state, nil
		}
	}
	return make(map[string]interface{}), nil
}

func (m *MockRedisBackend) SetState(ctx context.Context, stateID string, state map[string]interface{}) error {
	m.data.Store("state:"+stateID, state)
	return nil
}

func (m *MockRedisBackend) DeleteState(ctx context.Context, stateID string) error {
	m.data.Delete("state:" + stateID)
	return nil
}

func (m *MockRedisBackend) GetVersion(ctx context.Context, stateID string, versionID string) (*StateVersion, error) {
	if val, ok := m.data.Load("version:" + stateID + ":" + versionID); ok {
		if version, ok := val.(*StateVersion); ok {
			return version, nil
		}
	}
	return nil, fmt.Errorf("version not found: %s", versionID)
}

func (m *MockRedisBackend) SaveVersion(ctx context.Context, stateID string, version *StateVersion) error {
	m.data.Store("version:"+stateID+":"+version.ID, version)
	return nil
}

func (m *MockRedisBackend) GetVersionHistory(ctx context.Context, stateID string, limit int) ([]*StateVersion, error) {
	var versions []*StateVersion
	// Pre-build the key prefix for performance
	var keyBuilder strings.Builder
	keyBuilder.Grow(12 + len(stateID)) // "version:" + stateID + ":"
	keyBuilder.WriteString("version:")
	keyBuilder.WriteString(stateID)
	keyBuilder.WriteByte(':')
	keyPrefix := keyBuilder.String()
	
	m.data.Range(func(key, value interface{}) bool {
		if keyStr, ok := key.(string); ok {
			if strings.HasPrefix(keyStr, keyPrefix) {
				if version, ok := value.(*StateVersion); ok {
					versions = append(versions, version)
				}
			}
		}
		return len(versions) < limit
	})
	return versions, nil
}

func (m *MockRedisBackend) GetSnapshot(ctx context.Context, stateID string, snapshotID string) (*StateSnapshot, error) {
	// Use strings.Builder for key construction performance
	var keyBuilder strings.Builder
	keyBuilder.Grow(12 + len(stateID) + len(snapshotID)) // "snapshot:" + stateID + ":" + snapshotID
	keyBuilder.WriteString("snapshot:")
	keyBuilder.WriteString(stateID)
	keyBuilder.WriteByte(':')
	keyBuilder.WriteString(snapshotID)
	key := keyBuilder.String()
	
	if val, ok := m.data.Load(key); ok {
		if snapshot, ok := val.(*StateSnapshot); ok {
			return snapshot, nil
		}
	}
	return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
}

func (m *MockRedisBackend) SaveSnapshot(ctx context.Context, stateID string, snapshot *StateSnapshot) error {
	// Use strings.Builder for key construction performance
	var keyBuilder strings.Builder
	keyBuilder.Grow(12 + len(stateID) + len(snapshot.ID)) // "snapshot:" + stateID + ":" + snapshot.ID
	keyBuilder.WriteString("snapshot:")
	keyBuilder.WriteString(stateID)
	keyBuilder.WriteByte(':')
	keyBuilder.WriteString(snapshot.ID)
	key := keyBuilder.String()
	
	m.data.Store(key, snapshot)
	return nil
}

func (m *MockRedisBackend) ListSnapshots(ctx context.Context, stateID string) ([]*StateSnapshot, error) {
	var snapshots []*StateSnapshot
	// Pre-build the key prefix for performance
	var keyBuilder strings.Builder
	keyBuilder.Grow(12 + len(stateID)) // "snapshot:" + stateID + ":"
	keyBuilder.WriteString("snapshot:")
	keyBuilder.WriteString(stateID)
	keyBuilder.WriteByte(':')
	keyPrefix := keyBuilder.String()
	
	m.data.Range(func(key, value interface{}) bool {
		if keyStr, ok := key.(string); ok {
			if strings.HasPrefix(keyStr, keyPrefix) {
				if snapshot, ok := value.(*StateSnapshot); ok {
					snapshots = append(snapshots, snapshot)
				}
			}
		}
		return true
	})
	return snapshots, nil
}

func (m *MockRedisBackend) BeginTransaction(ctx context.Context) (Transaction, error) {
	return &MockTransaction{backend: m}, nil
}

func (m *MockRedisBackend) Close() error {
	return nil
}

func (m *MockRedisBackend) Ping(ctx context.Context) error {
	return nil
}

func (m *MockRedisBackend) Stats() map[string]interface{} {
	count := 0
	m.data.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	return map[string]interface{}{
		"type":        "mock_redis",
		"total_keys":  count,
		"backend_url": m.config.ConnectionURL,
	}
}

// MockPostgreSQLBackend provides a basic in-memory implementation for PostgreSQL backend
type MockPostgreSQLBackend struct {
	data   sync.Map // Thread-safe map for storing data
	config *StorageConfig
	logger Logger
}

// NewMockPostgreSQLBackend creates a mock PostgreSQL backend for testing/development
func NewMockPostgreSQLBackend(config *StorageConfig, logger Logger) (*MockPostgreSQLBackend, error) {
	backend := &MockPostgreSQLBackend{
		config: config,
		logger: logger,
	}

	logger.Info("Mock PostgreSQL storage backend initialized (development mode)",
		String("connection_url", config.ConnectionURL))

	return backend, nil
}

func (m *MockPostgreSQLBackend) GetState(ctx context.Context, stateID string) (map[string]interface{}, error) {
	// Use strings.Builder for key construction performance
	var keyBuilder strings.Builder
	keyBuilder.Grow(6 + len(stateID)) // "state:" + stateID
	keyBuilder.WriteString("state:")
	keyBuilder.WriteString(stateID)
	key := keyBuilder.String()
	
	if val, ok := m.data.Load(key); ok {
		if state, ok := val.(map[string]interface{}); ok {
			return state, nil
		}
	}
	return make(map[string]interface{}), nil
}

func (m *MockPostgreSQLBackend) SetState(ctx context.Context, stateID string, state map[string]interface{}) error {
	// Use strings.Builder for key construction performance
	var keyBuilder strings.Builder
	keyBuilder.Grow(6 + len(stateID)) // "state:" + stateID
	keyBuilder.WriteString("state:")
	keyBuilder.WriteString(stateID)
	key := keyBuilder.String()
	
	m.data.Store(key, state)
	return nil
}

func (m *MockPostgreSQLBackend) DeleteState(ctx context.Context, stateID string) error {
	// Use strings.Builder for key construction performance
	var keyBuilder strings.Builder
	keyBuilder.Grow(6 + len(stateID)) // "state:" + stateID
	keyBuilder.WriteString("state:")
	keyBuilder.WriteString(stateID)
	key := keyBuilder.String()
	
	m.data.Delete(key)
	return nil
}

func (m *MockPostgreSQLBackend) GetVersion(ctx context.Context, stateID string, versionID string) (*StateVersion, error) {
	// Use strings.Builder for key construction performance
	var keyBuilder strings.Builder
	keyBuilder.Grow(10 + len(stateID) + len(versionID)) // "version:" + stateID + ":" + versionID
	keyBuilder.WriteString("version:")
	keyBuilder.WriteString(stateID)
	keyBuilder.WriteByte(':')
	keyBuilder.WriteString(versionID)
	key := keyBuilder.String()
	
	if val, ok := m.data.Load(key); ok {
		if version, ok := val.(*StateVersion); ok {
			return version, nil
		}
	}
	return nil, fmt.Errorf("version not found: %s", versionID)
}

func (m *MockPostgreSQLBackend) SaveVersion(ctx context.Context, stateID string, version *StateVersion) error {
	// Use strings.Builder for key construction performance
	var keyBuilder strings.Builder
	keyBuilder.Grow(10 + len(stateID) + len(version.ID)) // "version:" + stateID + ":" + version.ID
	keyBuilder.WriteString("version:")
	keyBuilder.WriteString(stateID)
	keyBuilder.WriteByte(':')
	keyBuilder.WriteString(version.ID)
	key := keyBuilder.String()
	
	m.data.Store(key, version)
	return nil
}

func (m *MockPostgreSQLBackend) GetVersionHistory(ctx context.Context, stateID string, limit int) ([]*StateVersion, error) {
	var versions []*StateVersion
	// Pre-build the key prefix for performance
	var keyBuilder strings.Builder
	keyBuilder.Grow(9 + len(stateID)) // "version:" + stateID + ":"
	keyBuilder.WriteString("version:")
	keyBuilder.WriteString(stateID)
	keyBuilder.WriteByte(':')
	keyPrefix := keyBuilder.String()
	
	m.data.Range(func(key, value interface{}) bool {
		if keyStr, ok := key.(string); ok {
			if strings.HasPrefix(keyStr, keyPrefix) {
				if version, ok := value.(*StateVersion); ok {
					versions = append(versions, version)
				}
			}
		}
		return len(versions) < limit
	})
	return versions, nil
}

func (m *MockPostgreSQLBackend) GetSnapshot(ctx context.Context, stateID string, snapshotID string) (*StateSnapshot, error) {
	// Use strings.Builder for key construction performance
	var keyBuilder strings.Builder
	keyBuilder.Grow(10 + len(stateID) + len(snapshotID)) // "snapshot:" + stateID + ":" + snapshotID
	keyBuilder.WriteString("snapshot:")
	keyBuilder.WriteString(stateID)
	keyBuilder.WriteByte(':')
	keyBuilder.WriteString(snapshotID)
	key := keyBuilder.String()
	
	if val, ok := m.data.Load(key); ok {
		if snapshot, ok := val.(*StateSnapshot); ok {
			return snapshot, nil
		}
	}
	return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
}

func (m *MockPostgreSQLBackend) SaveSnapshot(ctx context.Context, stateID string, snapshot *StateSnapshot) error {
	// Use strings.Builder for key construction performance
	var keyBuilder strings.Builder
	keyBuilder.Grow(10 + len(stateID) + len(snapshot.ID)) // "snapshot:" + stateID + ":" + snapshot.ID
	keyBuilder.WriteString("snapshot:")
	keyBuilder.WriteString(stateID)
	keyBuilder.WriteByte(':')
	keyBuilder.WriteString(snapshot.ID)
	key := keyBuilder.String()
	
	m.data.Store(key, snapshot)
	return nil
}

func (m *MockPostgreSQLBackend) ListSnapshots(ctx context.Context, stateID string) ([]*StateSnapshot, error) {
	var snapshots []*StateSnapshot
	// Pre-build the key prefix for performance
	var keyBuilder strings.Builder
	keyBuilder.Grow(10 + len(stateID)) // "snapshot:" + stateID + ":"
	keyBuilder.WriteString("snapshot:")
	keyBuilder.WriteString(stateID)
	keyBuilder.WriteByte(':')
	keyPrefix := keyBuilder.String()
	
	m.data.Range(func(key, value interface{}) bool {
		if keyStr, ok := key.(string); ok {
			if strings.HasPrefix(keyStr, keyPrefix) {
				if snapshot, ok := value.(*StateSnapshot); ok {
					snapshots = append(snapshots, snapshot)
				}
			}
		}
		return true
	})
	return snapshots, nil
}

func (m *MockPostgreSQLBackend) BeginTransaction(ctx context.Context) (Transaction, error) {
	return &MockTransaction{backend: m}, nil
}

func (m *MockPostgreSQLBackend) Close() error {
	return nil
}

func (m *MockPostgreSQLBackend) Ping(ctx context.Context) error {
	return nil
}

func (m *MockPostgreSQLBackend) Stats() map[string]interface{} {
	count := 0
	m.data.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	return map[string]interface{}{
		"type":        "mock_postgresql",
		"total_keys":  count,
		"backend_url": m.config.ConnectionURL,
	}
}

// MockTransaction implements Transaction for mock backends
type MockTransaction struct {
	backend   interface{} // Can be either MockRedisBackend or MockPostgreSQLBackend
	changes   []mockTxOp
	committed bool
}

type mockTxOp struct {
	Op      string
	Key     string
	Value   interface{}
	StateID string
}

func (t *MockTransaction) SetState(ctx context.Context, stateID string, state map[string]interface{}) error {
	if t.committed {
		return fmt.Errorf("transaction already committed")
	}

	t.changes = append(t.changes, mockTxOp{
		Op:      "set_state",
		Key:     "state:" + stateID,
		Value:   state,
		StateID: stateID,
	})
	return nil
}

func (t *MockTransaction) SaveVersion(ctx context.Context, stateID string, version *StateVersion) error {
	if t.committed {
		return fmt.Errorf("transaction already committed")
	}

	t.changes = append(t.changes, mockTxOp{
		Op:      "save_version",
		Key:     "version:" + stateID + ":" + version.ID,
		Value:   version,
		StateID: stateID,
	})
	return nil
}

func (t *MockTransaction) Commit(ctx context.Context) error {
	if t.committed {
		return fmt.Errorf("transaction already committed")
	}

	// Apply all changes atomically
	var dataStore *sync.Map

	switch backend := t.backend.(type) {
	case *MockRedisBackend:
		dataStore = &backend.data
	case *MockPostgreSQLBackend:
		dataStore = &backend.data
	default:
		return fmt.Errorf("unsupported backend type")
	}

	for _, change := range t.changes {
		dataStore.Store(change.Key, change.Value)
	}

	t.committed = true
	return nil
}

func (t *MockTransaction) Rollback(ctx context.Context) error {
	if t.committed {
		return fmt.Errorf("transaction already committed")
	}

	t.changes = nil
	t.committed = true
	return nil
}

// UpdateNewStorageBackend updates the factory function to use mock backends by default
func UpdateNewStorageBackend() {
	// This function can be called to switch from mock to real backends
	// when dependencies become available
}

// CreateProductionRedisBackend creates a real Redis backend (requires redis dependency)
func CreateProductionRedisBackend(config *StorageConfig, logger Logger) (StorageBackend, error) {
	// Placeholder for production Redis implementation
	// When redis dependency is available, replace this with:
	// return NewRedisBackend(config, logger)

	logger.Warn("Production Redis backend not available, using mock implementation",
		String("reason", "redis dependency not available"))
	return NewMockRedisBackend(config, logger)
}

// CreateProductionPostgreSQLBackend creates a real PostgreSQL backend (requires pq dependency)
func CreateProductionPostgreSQLBackend(config *StorageConfig, logger Logger) (StorageBackend, error) {
	// Placeholder for production PostgreSQL implementation
	// When pq dependency is available, replace this with:
	// return NewPostgreSQLBackend(config, logger)

	logger.Warn("Production PostgreSQL backend not available, using mock implementation",
		String("reason", "postgresql dependency not available"))
	return NewMockPostgreSQLBackend(config, logger)
}
