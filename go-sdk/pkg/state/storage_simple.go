package state

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// StorageBackend defines the interface for pluggable storage backends
type StorageBackend interface {
	// State operations
	GetState(ctx context.Context, stateID string) (map[string]interface{}, error)
	SetState(ctx context.Context, stateID string, state map[string]interface{}) error
	DeleteState(ctx context.Context, stateID string) error
	
	// Version operations
	GetVersion(ctx context.Context, stateID string, versionID string) (*StateVersion, error)
	SaveVersion(ctx context.Context, stateID string, version *StateVersion) error
	GetVersionHistory(ctx context.Context, stateID string, limit int) ([]*StateVersion, error)
	
	// Snapshot operations
	GetSnapshot(ctx context.Context, stateID string, snapshotID string) (*StateSnapshot, error)
	SaveSnapshot(ctx context.Context, stateID string, snapshot *StateSnapshot) error
	ListSnapshots(ctx context.Context, stateID string) ([]*StateSnapshot, error)
	
	// Transaction operations
	BeginTransaction(ctx context.Context) (Transaction, error)
	
	// Housekeeping
	Close() error
	Ping(ctx context.Context) error
	Stats() map[string]interface{}
}

// Transaction represents a storage transaction
type Transaction interface {
	SetState(ctx context.Context, stateID string, state map[string]interface{}) error
	SaveVersion(ctx context.Context, stateID string, version *StateVersion) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// StorageBackendType represents the type of storage backend
type StorageBackendType string

const (
	StorageBackendRedis      StorageBackendType = "redis"
	StorageBackendPostgreSQL StorageBackendType = "postgresql"
	StorageBackendFile       StorageBackendType = "file"
)

// StorageConfig holds configuration for storage backends
type StorageConfig struct {
	Type           StorageBackendType `json:"type"`
	ConnectionURL  string            `json:"connection_url"`
	Database       string            `json:"database"`
	Schema         string            `json:"schema"`
	MaxConnections int               `json:"max_connections"`
	ConnectTimeout time.Duration     `json:"connect_timeout"`
	ReadTimeout    time.Duration     `json:"read_timeout"`
	WriteTimeout   time.Duration     `json:"write_timeout"`
	IdleTimeout    time.Duration     `json:"idle_timeout"`
	MaxRetries     int               `json:"max_retries"`
	
	// File specific
	FileOptions *FileOptions `json:"file_options,omitempty"`
}

// FileOptions holds file-based storage configuration
type FileOptions struct {
	BaseDir         string      `json:"base_dir"`
	EnableSharding  bool        `json:"enable_sharding"`
	ShardCount      int         `json:"shard_count"`
	FileMode        os.FileMode `json:"file_mode"`
	EnableBackups   bool        `json:"enable_backups"`
	BackupCount     int         `json:"backup_count"`
}

// DefaultStorageConfig returns a default storage configuration
func DefaultStorageConfig() *StorageConfig {
	return &StorageConfig{
		Type:           StorageBackendFile,
		MaxConnections: 10,
		ConnectTimeout: 30 * time.Second,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    5 * time.Minute,
		MaxRetries:     3,
		FileOptions: &FileOptions{
			BaseDir:         "/var/lib/ag-ui/state",
			EnableSharding:  true,
			ShardCount:      16,
			FileMode:        0644,
			EnableBackups:   true,
			BackupCount:     5,
		},
	}
}

// ValidateStorageConfig validates the storage configuration
func ValidateStorageConfig(config *StorageConfig) error {
	if config == nil {
		return fmt.Errorf("storage config cannot be nil")
	}
	
	switch config.Type {
	case StorageBackendRedis:
		if config.ConnectionURL == "" {
			return fmt.Errorf("redis connection URL is required")
		}
		
	case StorageBackendPostgreSQL:
		if config.ConnectionURL == "" {
			return fmt.Errorf("postgresql connection URL is required")
		}
		
	case StorageBackendFile:
		if config.FileOptions == nil {
			return fmt.Errorf("file options are required")
		}
		if config.FileOptions.BaseDir == "" {
			return fmt.Errorf("file base directory is required")
		}
		if config.FileOptions.ShardCount <= 0 {
			config.FileOptions.ShardCount = 16
		}
		
	default:
		return fmt.Errorf("unsupported storage backend type: %s", config.Type)
	}
	
	return nil
}

// NewStorageBackend creates a new storage backend based on configuration
func NewStorageBackend(config *StorageConfig, logger Logger) (StorageBackend, error) {
	if err := ValidateStorageConfig(config); err != nil {
		return nil, fmt.Errorf("invalid storage config: %w", err)
	}
	
	switch config.Type {
	case StorageBackendRedis:
		// Use mock implementation for development
		return NewMockRedisBackend(config, logger)
	case StorageBackendPostgreSQL:
		// Use mock implementation for development
		return NewMockPostgreSQLBackend(config, logger)
	case StorageBackendFile:
		return NewFileBackend(config, logger)
	default:
		return nil, fmt.Errorf("unsupported storage backend type: %s", config.Type)
	}
}

// FileBackend implements StorageBackend using file system
type FileBackend struct {
	config *StorageConfig
	logger Logger
	mu     sync.RWMutex
	stats  map[string]interface{}
}

// NewFileBackend creates a new file storage backend
func NewFileBackend(config *StorageConfig, logger Logger) (*FileBackend, error) {
	// Ensure base directory exists
	if err := os.MkdirAll(config.FileOptions.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}
	
	// Create subdirectories
	dirs := []string{"states", "versions", "snapshots"}
	for _, dir := range dirs {
		path := fmt.Sprintf("%s/%s", config.FileOptions.BaseDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}
	
	backend := &FileBackend{
		config: config,
		logger: logger,
		stats:  make(map[string]interface{}),
	}
	
	logger.Info("File storage backend initialized",
		String("base_dir", config.FileOptions.BaseDir),
		Bool("enable_sharding", config.FileOptions.EnableSharding),
		Int("shard_count", config.FileOptions.ShardCount))
	
	return backend, nil
}

func (f *FileBackend) GetState(ctx context.Context, stateID string) (map[string]interface{}, error) {
	path := f.statePath(stateID)
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}
	
	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}
	
	return state, nil
}

func (f *FileBackend) SetState(ctx context.Context, stateID string, state map[string]interface{}) error {
	path := f.statePath(stateID)
	
	// Ensure directory exists
	if err := os.MkdirAll(fmt.Sprintf("%s/states", f.config.FileOptions.BaseDir), 0755); err != nil {
		return fmt.Errorf("failed to create states directory: %w", err)
	}
	
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	
	// Create backup if enabled
	if f.config.FileOptions.EnableBackups {
		if err := f.createBackup(path); err != nil {
			f.logger.Warn("failed to create backup", String("path", path), Err(err))
		}
	}
	
	if err := os.WriteFile(path, data, f.config.FileOptions.FileMode); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}
	
	return nil
}

func (f *FileBackend) DeleteState(ctx context.Context, stateID string) error {
	path := f.statePath(stateID)
	
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete state file: %w", err)
	}
	
	return nil
}

func (f *FileBackend) GetVersion(ctx context.Context, stateID string, versionID string) (*StateVersion, error) {
	path := f.versionPath(stateID, versionID)
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("version not found: %s", versionID)
		}
		return nil, fmt.Errorf("failed to read version file: %w", err)
	}
	
	var version StateVersion
	if err := json.Unmarshal(data, &version); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version: %w", err)
	}
	
	return &version, nil
}

func (f *FileBackend) SaveVersion(ctx context.Context, stateID string, version *StateVersion) error {
	path := f.versionPath(stateID, version.ID)
	
	// Ensure directory exists
	dir := fmt.Sprintf("%s/versions/%s", f.config.FileOptions.BaseDir, f.getShardDir(stateID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create version directory: %w", err)
	}
	
	data, err := json.MarshalIndent(version, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal version: %w", err)
	}
	
	if err := os.WriteFile(path, data, f.config.FileOptions.FileMode); err != nil {
		return fmt.Errorf("failed to write version file: %w", err)
	}
	
	return nil
}

func (f *FileBackend) GetVersionHistory(ctx context.Context, stateID string, limit int) ([]*StateVersion, error) {
	dir := fmt.Sprintf("%s/versions/%s", f.config.FileOptions.BaseDir, f.getShardDir(stateID))
	
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*StateVersion{}, nil
		}
		return nil, fmt.Errorf("failed to read version directory: %w", err)
	}
	
	var versions []*StateVersion
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		// Extract version ID from filename
		versionID := entry.Name()
		if len(versionID) > 5 && versionID[len(versionID)-5:] == ".json" {
			versionID = versionID[:len(versionID)-5]
		}
		
		version, err := f.GetVersion(ctx, stateID, versionID)
		if err != nil {
			f.logger.Error("failed to get version", 
				String("state_id", stateID),
				String("version_id", versionID),
				Err(err))
			continue
		}
		
		versions = append(versions, version)
		
		if len(versions) >= limit {
			break
		}
	}
	
	return versions, nil
}

func (f *FileBackend) GetSnapshot(ctx context.Context, stateID string, snapshotID string) (*StateSnapshot, error) {
	path := f.snapshotPath(stateID, snapshotID)
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
		}
		return nil, fmt.Errorf("failed to read snapshot file: %w", err)
	}
	
	var snapshot StateSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}
	
	return &snapshot, nil
}

func (f *FileBackend) SaveSnapshot(ctx context.Context, stateID string, snapshot *StateSnapshot) error {
	path := f.snapshotPath(stateID, snapshot.ID)
	
	// Ensure directory exists
	dir := fmt.Sprintf("%s/snapshots/%s", f.config.FileOptions.BaseDir, f.getShardDir(stateID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}
	
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}
	
	if err := os.WriteFile(path, data, f.config.FileOptions.FileMode); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}
	
	return nil
}

func (f *FileBackend) ListSnapshots(ctx context.Context, stateID string) ([]*StateSnapshot, error) {
	dir := fmt.Sprintf("%s/snapshots/%s", f.config.FileOptions.BaseDir, f.getShardDir(stateID))
	
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*StateSnapshot{}, nil
		}
		return nil, fmt.Errorf("failed to read snapshot directory: %w", err)
	}
	
	var snapshots []*StateSnapshot
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		// Extract snapshot ID from filename
		snapshotID := entry.Name()
		if len(snapshotID) > 5 && snapshotID[len(snapshotID)-5:] == ".json" {
			snapshotID = snapshotID[:len(snapshotID)-5]
		}
		
		snapshot, err := f.GetSnapshot(ctx, stateID, snapshotID)
		if err != nil {
			f.logger.Error("failed to get snapshot", 
				String("state_id", stateID),
				String("snapshot_id", snapshotID),
				Err(err))
			continue
		}
		
		snapshots = append(snapshots, snapshot)
	}
	
	return snapshots, nil
}

func (f *FileBackend) BeginTransaction(ctx context.Context) (Transaction, error) {
	return &FileTransaction{
		backend: f,
		ops:     make([]fileOp, 0),
	}, nil
}

func (f *FileBackend) Close() error {
	return nil
}

func (f *FileBackend) Ping(ctx context.Context) error {
	// Check if base directory is accessible
	_, err := os.Stat(f.config.FileOptions.BaseDir)
	return err
}

func (f *FileBackend) Stats() map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	stats := make(map[string]interface{})
	for k, v := range f.stats {
		stats[k] = v
	}
	
	stats["base_dir"] = f.config.FileOptions.BaseDir
	stats["enable_sharding"] = f.config.FileOptions.EnableSharding
	stats["shard_count"] = f.config.FileOptions.ShardCount
	
	return stats
}

// Helper methods for file paths
func (f *FileBackend) getShardDir(stateID string) string {
	if !f.config.FileOptions.EnableSharding {
		return ""
	}
	
	// Simple hash-based sharding
	hash := 0
	for _, ch := range stateID {
		hash = hash*31 + int(ch)
	}
	shardIndex := hash % f.config.FileOptions.ShardCount
	if shardIndex < 0 {
		shardIndex = -shardIndex
	}
	
	return fmt.Sprintf("shard_%d", shardIndex)
}

func (f *FileBackend) statePath(stateID string) string {
	shardDir := f.getShardDir(stateID)
	if shardDir == "" {
		return fmt.Sprintf("%s/states/%s.json", f.config.FileOptions.BaseDir, stateID)
	}
	return fmt.Sprintf("%s/states/%s/%s.json", f.config.FileOptions.BaseDir, shardDir, stateID)
}

func (f *FileBackend) versionPath(stateID, versionID string) string {
	shardDir := f.getShardDir(stateID)
	if shardDir == "" {
		return fmt.Sprintf("%s/versions/%s.json", f.config.FileOptions.BaseDir, versionID)
	}
	return fmt.Sprintf("%s/versions/%s/%s.json", f.config.FileOptions.BaseDir, shardDir, versionID)
}

func (f *FileBackend) snapshotPath(stateID, snapshotID string) string {
	shardDir := f.getShardDir(stateID)
	if shardDir == "" {
		return fmt.Sprintf("%s/snapshots/%s.json", f.config.FileOptions.BaseDir, snapshotID)
	}
	return fmt.Sprintf("%s/snapshots/%s/%s.json", f.config.FileOptions.BaseDir, shardDir, snapshotID)
}

func (f *FileBackend) createBackup(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // No file to backup
	}
	
	backupPath := fmt.Sprintf("%s.backup", path)
	
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	
	dst, err := os.Create(backupPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	
	_, err = io.Copy(dst, src)
	return err
}

// FileTransaction implements Transaction for file backend
type FileTransaction struct {
	backend *FileBackend
	ops     []fileOp
}

type fileOp struct {
	Type    string
	Path    string
	Data    []byte
	StateID string
}

func (t *FileTransaction) SetState(ctx context.Context, stateID string, state map[string]interface{}) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	
	t.ops = append(t.ops, fileOp{
		Type:    "set_state",
		Path:    t.backend.statePath(stateID),
		Data:    data,
		StateID: stateID,
	})
	
	return nil
}

func (t *FileTransaction) SaveVersion(ctx context.Context, stateID string, version *StateVersion) error {
	data, err := json.MarshalIndent(version, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal version: %w", err)
	}
	
	t.ops = append(t.ops, fileOp{
		Type:    "save_version",
		Path:    t.backend.versionPath(stateID, version.ID),
		Data:    data,
		StateID: stateID,
	})
	
	return nil
}

func (t *FileTransaction) Commit(ctx context.Context) error {
	// Execute all operations
	for _, op := range t.ops {
		switch op.Type {
		case "set_state":
			// Ensure directory exists
			if err := os.MkdirAll(fmt.Sprintf("%s/states", t.backend.config.FileOptions.BaseDir), 0755); err != nil {
				return fmt.Errorf("failed to create states directory: %w", err)
			}
			
			// Create backup if enabled
			if t.backend.config.FileOptions.EnableBackups {
				if err := t.backend.createBackup(op.Path); err != nil {
					t.backend.logger.Warn("failed to create backup", String("path", op.Path), Err(err))
				}
			}
			
			if err := os.WriteFile(op.Path, op.Data, t.backend.config.FileOptions.FileMode); err != nil {
				return fmt.Errorf("failed to write state file: %w", err)
			}
			
		case "save_version":
			// Ensure directory exists
			dir := fmt.Sprintf("%s/versions/%s", t.backend.config.FileOptions.BaseDir, t.backend.getShardDir(op.StateID))
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create version directory: %w", err)
			}
			
			if err := os.WriteFile(op.Path, op.Data, t.backend.config.FileOptions.FileMode); err != nil {
				return fmt.Errorf("failed to write version file: %w", err)
			}
		}
	}
	
	return nil
}

func (t *FileTransaction) Rollback(ctx context.Context) error {
	// For file backend, we don't need to do anything for rollback
	// since we haven't written anything yet
	t.ops = nil
	return nil
}