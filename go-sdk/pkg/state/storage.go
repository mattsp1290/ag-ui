package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
	// Optional external dependencies - only used if available
	// Comment out these lines if dependencies are not available
	// "github.com/go-redis/redis/v8"
	// "github.com/lib/pq"
	// _ "github.com/lib/pq"
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
	StorageTypeRedis         StorageBackendType = "redis" // Alias for backwards compatibility
)

// CompressionConfig configures compression settings
type CompressionConfig struct {
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	Algorithm    string `json:"algorithm" yaml:"algorithm"`
	Level        int    `json:"level" yaml:"level"`
	MinSizeBytes int    `json:"min_size_bytes" yaml:"min_size_bytes"`
}

// StorageConfig holds configuration for storage backends
type StorageConfig struct {
	Type              StorageBackendType `json:"type" yaml:"type"`
	ConnectionURL     string             `json:"connection_url" yaml:"connection_url"`
	Database          string             `json:"database" yaml:"database"`
	Schema            string             `json:"schema" yaml:"schema"`
	MaxConnections    int                `json:"max_connections" yaml:"max_connections"`
	ConnectTimeout    time.Duration      `json:"connect_timeout" yaml:"connect_timeout"`
	ConnectionTimeout time.Duration      `json:"connection_timeout" yaml:"connection_timeout"` // Alias for backwards compatibility
	ReadTimeout       time.Duration      `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout      time.Duration      `json:"write_timeout" yaml:"write_timeout"`
	IdleTimeout       time.Duration      `json:"idle_timeout" yaml:"idle_timeout"`
	MaxRetries        int                `json:"max_retries" yaml:"max_retries"`
	Compression       CompressionConfig  `json:"compression" yaml:"compression"`

	// Redis specific
	RedisOptions *RedisOptions `json:"redis_options,omitempty" yaml:"redis_options,omitempty"`

	// PostgreSQL specific
	PostgreSQLOptions *PostgreSQLOptions `json:"postgresql_options,omitempty" yaml:"postgresql_options,omitempty"`

	// File specific
	FileOptions *FileOptions `json:"file_options,omitempty" yaml:"file_options,omitempty"`
}

// RedisOptions holds Redis-specific configuration
type RedisOptions struct {
	PoolSize        int    `json:"pool_size" yaml:"pool_size"`
	MinIdleConns    int    `json:"min_idle_conns" yaml:"min_idle_conns"`
	MaxRetries      int    `json:"max_retries" yaml:"max_retries"`
	Password        string `json:"password" yaml:"password"`
	DB              int    `json:"db" yaml:"db"`
	KeyPrefix       string `json:"key_prefix" yaml:"key_prefix"`
	EnableTLS       bool   `json:"enable_tls" yaml:"enable_tls"`
	CompressionType string `json:"compression_type" yaml:"compression_type"`
}

// PostgreSQLOptions holds PostgreSQL-specific configuration
type PostgreSQLOptions struct {
	SSLMode            string `json:"ssl_mode" yaml:"ssl_mode"`
	ApplicationName    string `json:"application_name" yaml:"application_name"`
	StatementTimeout   string `json:"statement_timeout" yaml:"statement_timeout"`
	EnablePartitioning bool   `json:"enable_partitioning" yaml:"enable_partitioning"`
	CompressionType    string `json:"compression_type" yaml:"compression_type"`
}

// FileOptions holds file-based storage configuration
type FileOptions struct {
	BaseDir         string      `json:"base_dir" yaml:"base_dir"`
	EnableSharding  bool        `json:"enable_sharding" yaml:"enable_sharding"`
	ShardCount      int         `json:"shard_count" yaml:"shard_count"`
	FileMode        os.FileMode `json:"file_mode" yaml:"file_mode"`
	EnableBackups   bool        `json:"enable_backups" yaml:"enable_backups"`
	BackupCount     int         `json:"backup_count" yaml:"backup_count"`
	CompressionType string      `json:"compression_type" yaml:"compression_type"`
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
		RedisOptions: &RedisOptions{
			PoolSize:        10,
			MinIdleConns:    5,
			MaxRetries:      3,
			DB:              0,
			KeyPrefix:       "ag-ui:state:",
			EnableTLS:       false,
			CompressionType: "gzip",
		},
		PostgreSQLOptions: &PostgreSQLOptions{
			SSLMode:            "prefer",
			ApplicationName:    "ag-ui-state",
			StatementTimeout:   "30s",
			EnablePartitioning: false,
			CompressionType:    "gzip",
		},
		FileOptions: &FileOptions{
			BaseDir:         "/var/lib/ag-ui/state",
			EnableSharding:  true,
			ShardCount:      16,
			FileMode:        0600,
			EnableBackups:   true,
			BackupCount:     5,
			CompressionType: "gzip",
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
		if config.RedisOptions == nil {
			return fmt.Errorf("redis options are required")
		}
		if config.RedisOptions.PoolSize <= 0 {
			return fmt.Errorf("redis pool size must be positive")
		}

	case StorageBackendPostgreSQL:
		if config.ConnectionURL == "" {
			return fmt.Errorf("postgresql connection URL is required")
		}
		if config.PostgreSQLOptions == nil {
			return fmt.Errorf("postgresql options are required")
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

	if config.MaxConnections <= 0 {
		config.MaxConnections = 10
	}
	if config.ConnectTimeout <= 0 {
		config.ConnectTimeout = 30 * time.Second
	}
	if config.ReadTimeout <= 0 {
		config.ReadTimeout = 10 * time.Second
	}
	if config.WriteTimeout <= 0 {
		config.WriteTimeout = 10 * time.Second
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
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
		// Use stub implementation when Redis dependency is not available
		return NewRedisBackend(config, logger)
	case StorageBackendPostgreSQL:
		// Use stub implementation when PostgreSQL dependency is not available
		return NewPostgreSQLBackend(config, logger)
	case StorageBackendFile:
		return NewFileBackend(config, logger)
	default:
		return nil, fmt.Errorf("unsupported storage backend type: %s", config.Type)
	}
}

// Redis Backend Implementation

// RedisBackend implements StorageBackend using Redis
// NOTE: This implementation is temporarily stubbed until Redis dependency is available
type RedisBackend struct {
	// client  *redis.Client // TODO: Uncomment when redis package is available
	config *StorageConfig
	logger Logger
	mu     sync.RWMutex
	stats  map[string]interface{}
}

// NewRedisBackend creates a new Redis storage backend
func NewRedisBackend(config *StorageConfig, logger Logger) (*RedisBackend, error) {
	if config == nil {
		return nil, fmt.Errorf("storage config cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// TODO: Implement Redis backend when redis package is available
	// TODO: Uncomment when redis package is available
	// opts := &redis.Options{
	// 	Addr:         config.ConnectionURL,
	// 	Password:     config.RedisOptions.Password,
	// 	DB:           config.RedisOptions.DB,
	// 	PoolSize:     config.RedisOptions.PoolSize,
	// 	MinIdleConns: config.RedisOptions.MinIdleConns,
	// 	MaxRetries:   config.RedisOptions.MaxRetries,
	// 	ReadTimeout:  config.ReadTimeout,
	// 	WriteTimeout: config.WriteTimeout,
	// 	IdleTimeout:  config.IdleTimeout,
	// }
	//
	// client := redis.NewClient(opts)
	//
	// // Test connection
	// ctx, cancel := context.WithTimeout(context.Background(), config.ConnectTimeout)
	// defer cancel()
	//
	// if err := client.Ping(ctx).Err(); err != nil {
	// 	return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	// }

	backend := &RedisBackend{
		// client: client,
		config: config,
		logger: logger,
		stats:  make(map[string]interface{}),
	}

	logger.Info("Redis storage backend initialized",
		String("addr", config.ConnectionURL),
		Int("pool_size", config.RedisOptions.PoolSize),
		Int("db", config.RedisOptions.DB))

	return backend, nil
}

// Helper methods for Redis keys
func (r *RedisBackend) stateKey(stateID string) string {
	return fmt.Sprintf("%sstate:%s", r.config.RedisOptions.KeyPrefix, stateID)
}

func (r *RedisBackend) versionKey(stateID, versionID string) string {
	return fmt.Sprintf("%sversion:%s:%s", r.config.RedisOptions.KeyPrefix, stateID, versionID)
}

func (r *RedisBackend) versionListKey(stateID string) string {
	return fmt.Sprintf("%sversions:%s", r.config.RedisOptions.KeyPrefix, stateID)
}

func (r *RedisBackend) snapshotKey(stateID, snapshotID string) string {
	return fmt.Sprintf("%ssnapshot:%s:%s", r.config.RedisOptions.KeyPrefix, stateID, snapshotID)
}

func (r *RedisBackend) snapshotListKey(stateID string) string {
	return fmt.Sprintf("%ssnapshots:%s", r.config.RedisOptions.KeyPrefix, stateID)
}

// RedisTransaction implements Transaction for Redis
type RedisTransaction struct {
	backend *RedisBackend
	// pipe    redis.Pipeliner // TODO: Uncomment when redis package is available
}

// PostgreSQL Backend Implementation

// PostgreSQLBackend implements StorageBackend using PostgreSQL
type PostgreSQLBackend struct {
	db     *sql.DB
	config *StorageConfig
	logger Logger
	mu     sync.RWMutex
	stats  map[string]interface{}
}

// NewPostgreSQLBackend creates a new PostgreSQL storage backend
func NewPostgreSQLBackend(config *StorageConfig, logger Logger) (*PostgreSQLBackend, error) {
	db, err := sql.Open("postgres", config.ConnectionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxConnections)
	db.SetMaxIdleConns(config.MaxConnections / 2)
	db.SetConnMaxLifetime(config.IdleTimeout)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	backend := &PostgreSQLBackend{
		db:     db,
		config: config,
		logger: logger,
		stats:  make(map[string]interface{}),
	}

	// Initialize schema
	if err := backend.initSchema(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logger.Info("PostgreSQL storage backend initialized",
		String("host", config.ConnectionURL),
		Int("max_connections", config.MaxConnections))

	return backend, nil
}

func (p *PostgreSQLBackend) initSchema(ctx context.Context) error {
	schema := p.config.Schema
	if schema == "" {
		schema = "public"
	}

	queries := []string{
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s.ag_states (
				state_id VARCHAR(255) PRIMARY KEY,
				data JSONB NOT NULL,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			)
		`, schema),
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s.ag_versions (
				version_id VARCHAR(255) PRIMARY KEY,
				state_id VARCHAR(255) NOT NULL,
				data JSONB NOT NULL,
				delta JSONB,
				metadata JSONB,
				parent_id VARCHAR(255),
				created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
				INDEX(state_id, created_at)
			)
		`, schema),
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s.ag_snapshots (
				snapshot_id VARCHAR(255) PRIMARY KEY,
				state_id VARCHAR(255) NOT NULL,
				data JSONB NOT NULL,
				version_id VARCHAR(255),
				metadata JSONB,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
				INDEX(state_id, created_at)
			)
		`, schema),
		fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS idx_ag_versions_state_created 
			ON %s.ag_versions (state_id, created_at DESC)
		`, schema),
		fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS idx_ag_snapshots_state_created 
			ON %s.ag_snapshots (state_id, created_at DESC)
		`, schema),
	}

	for _, query := range queries {
		if _, err := p.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	return nil
}

func (p *PostgreSQLBackend) GetState(ctx context.Context, stateID string) (map[string]interface{}, error) {
	schema := p.config.Schema
	if schema == "" {
		schema = "public"
	}

	query := fmt.Sprintf(`
		SELECT data FROM %s.ag_states WHERE state_id = $1
	`, schema)

	var dataBytes []byte
	err := p.db.QueryRowContext(ctx, query, stateID).Scan(&dataBytes)
	if err != nil {
		if err == sql.ErrNoRows {
			return make(map[string]interface{}), nil
		}
		return nil, fmt.Errorf("failed to get state from PostgreSQL: %w", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(dataBytes, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return state, nil
}

func (p *PostgreSQLBackend) SetState(ctx context.Context, stateID string, state map[string]interface{}) error {
	schema := p.config.Schema
	if schema == "" {
		schema = "public"
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.ag_states (state_id, data) 
		VALUES ($1, $2) 
		ON CONFLICT (state_id) DO UPDATE SET 
			data = EXCLUDED.data, 
			updated_at = NOW()
	`, schema)

	if _, err := p.db.ExecContext(ctx, query, stateID, data); err != nil {
		return fmt.Errorf("failed to set state in PostgreSQL: %w", err)
	}

	return nil
}

func (p *PostgreSQLBackend) DeleteState(ctx context.Context, stateID string) error {
	schema := p.config.Schema
	if schema == "" {
		schema = "public"
	}

	query := fmt.Sprintf(`
		DELETE FROM %s.ag_states WHERE state_id = $1
	`, schema)

	if _, err := p.db.ExecContext(ctx, query, stateID); err != nil {
		return fmt.Errorf("failed to delete state from PostgreSQL: %w", err)
	}

	return nil
}

func (p *PostgreSQLBackend) GetVersion(ctx context.Context, stateID string, versionID string) (*StateVersion, error) {
	schema := p.config.Schema
	if schema == "" {
		schema = "public"
	}

	query := fmt.Sprintf(`
		SELECT version_id, state_id, data, delta, metadata, parent_id, created_at 
		FROM %s.ag_versions 
		WHERE state_id = $1 AND version_id = $2
	`, schema)

	var version StateVersion
	var dataBytes, deltaBytes, metadataBytes []byte
	var parentID sql.NullString
	var createdAt time.Time

	err := p.db.QueryRowContext(ctx, query, stateID, versionID).Scan(
		&version.ID, &dataBytes, &deltaBytes, &metadataBytes, &parentID, &createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("version not found: %s", versionID)
		}
		return nil, fmt.Errorf("failed to get version from PostgreSQL: %w", err)
	}

	version.Timestamp = createdAt
	if parentID.Valid {
		version.ParentID = parentID.String
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(dataBytes, &version.State); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version state: %w", err)
	}

	if len(deltaBytes) > 0 {
		if err := json.Unmarshal(deltaBytes, &version.Delta); err != nil {
			return nil, fmt.Errorf("failed to unmarshal version delta: %w", err)
		}
	}

	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &version.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal version metadata: %w", err)
		}
	}

	return &version, nil
}

func (p *PostgreSQLBackend) SaveVersion(ctx context.Context, stateID string, version *StateVersion) error {
	schema := p.config.Schema
	if schema == "" {
		schema = "public"
	}

	// Marshal JSON fields
	dataBytes, err := json.Marshal(version.State)
	if err != nil {
		return fmt.Errorf("failed to marshal version state: %w", err)
	}

	var deltaBytes []byte
	if version.Delta != nil {
		deltaBytes, err = json.Marshal(version.Delta)
		if err != nil {
			return fmt.Errorf("failed to marshal version delta: %w", err)
		}
	}

	var metadataBytes []byte
	if version.Metadata != nil {
		metadataBytes, err = json.Marshal(version.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal version metadata: %w", err)
		}
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.ag_versions (version_id, state_id, data, delta, metadata, parent_id, created_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (version_id) DO UPDATE SET 
			data = EXCLUDED.data,
			delta = EXCLUDED.delta,
			metadata = EXCLUDED.metadata
	`, schema)

	var parentID sql.NullString
	if version.ParentID != "" {
		parentID.String = version.ParentID
		parentID.Valid = true
	}

	if _, err := p.db.ExecContext(ctx, query, version.ID, stateID, dataBytes, deltaBytes, metadataBytes, parentID, version.Timestamp); err != nil {
		return fmt.Errorf("failed to save version to PostgreSQL: %w", err)
	}

	return nil
}

func (p *PostgreSQLBackend) GetVersionHistory(ctx context.Context, stateID string, limit int) ([]*StateVersion, error) {
	schema := p.config.Schema
	if schema == "" {
		schema = "public"
	}

	query := fmt.Sprintf(`
		SELECT version_id, state_id, data, delta, metadata, parent_id, created_at 
		FROM %s.ag_versions 
		WHERE state_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2
	`, schema)

	rows, err := p.db.QueryContext(ctx, query, stateID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get version history from PostgreSQL: %w", err)
	}
	defer rows.Close()

	var versions []*StateVersion
	for rows.Next() {
		var version StateVersion
		var dataBytes, deltaBytes, metadataBytes []byte
		var parentID sql.NullString
		var createdAt time.Time

		if err := rows.Scan(&version.ID, &dataBytes, &deltaBytes, &metadataBytes, &parentID, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan version row: %w", err)
		}

		version.Timestamp = createdAt
		if parentID.Valid {
			version.ParentID = parentID.String
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(dataBytes, &version.State); err != nil {
			return nil, fmt.Errorf("failed to unmarshal version state: %w", err)
		}

		if len(deltaBytes) > 0 {
			if err := json.Unmarshal(deltaBytes, &version.Delta); err != nil {
				return nil, fmt.Errorf("failed to unmarshal version delta: %w", err)
			}
		}

		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &version.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal version metadata: %w", err)
			}
		}

		versions = append(versions, &version)
	}

	return versions, nil
}

func (p *PostgreSQLBackend) GetSnapshot(ctx context.Context, stateID string, snapshotID string) (*StateSnapshot, error) {
	schema := p.config.Schema
	if schema == "" {
		schema = "public"
	}

	query := fmt.Sprintf(`
		SELECT snapshot_id, state_id, data, version_id, metadata, created_at 
		FROM %s.ag_snapshots 
		WHERE state_id = $1 AND snapshot_id = $2
	`, schema)

	var snapshot StateSnapshot
	var dataBytes, metadataBytes []byte
	var versionID sql.NullString
	var createdAt time.Time

	err := p.db.QueryRowContext(ctx, query, stateID, snapshotID).Scan(
		&snapshot.ID, &dataBytes, &versionID, &metadataBytes, &createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
		}
		return nil, fmt.Errorf("failed to get snapshot from PostgreSQL: %w", err)
	}

	snapshot.Timestamp = createdAt
	if versionID.Valid {
		snapshot.Version = versionID.String
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(dataBytes, &snapshot.State); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot state: %w", err)
	}

	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &snapshot.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal snapshot metadata: %w", err)
		}
	}

	return &snapshot, nil
}

func (p *PostgreSQLBackend) SaveSnapshot(ctx context.Context, stateID string, snapshot *StateSnapshot) error {
	schema := p.config.Schema
	if schema == "" {
		schema = "public"
	}

	// Marshal JSON fields
	dataBytes, err := json.Marshal(snapshot.State)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot state: %w", err)
	}

	var metadataBytes []byte
	if snapshot.Metadata != nil {
		metadataBytes, err = json.Marshal(snapshot.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal snapshot metadata: %w", err)
		}
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.ag_snapshots (snapshot_id, state_id, data, version_id, metadata, created_at) 
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (snapshot_id) DO UPDATE SET 
			data = EXCLUDED.data,
			version_id = EXCLUDED.version_id,
			metadata = EXCLUDED.metadata
	`, schema)

	var versionID sql.NullString
	if snapshot.Version != "" {
		versionID.String = snapshot.Version
		versionID.Valid = true
	}

	if _, err := p.db.ExecContext(ctx, query, snapshot.ID, stateID, dataBytes, versionID, metadataBytes, snapshot.Timestamp); err != nil {
		return fmt.Errorf("failed to save snapshot to PostgreSQL: %w", err)
	}

	return nil
}

func (p *PostgreSQLBackend) ListSnapshots(ctx context.Context, stateID string) ([]*StateSnapshot, error) {
	schema := p.config.Schema
	if schema == "" {
		schema = "public"
	}

	query := fmt.Sprintf(`
		SELECT snapshot_id, state_id, data, version_id, metadata, created_at 
		FROM %s.ag_snapshots 
		WHERE state_id = $1 
		ORDER BY created_at DESC
	`, schema)

	rows, err := p.db.QueryContext(ctx, query, stateID)
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots from PostgreSQL: %w", err)
	}
	defer rows.Close()

	var snapshots []*StateSnapshot
	for rows.Next() {
		var snapshot StateSnapshot
		var dataBytes, metadataBytes []byte
		var versionID sql.NullString
		var createdAt time.Time

		if err := rows.Scan(&snapshot.ID, &dataBytes, &versionID, &metadataBytes, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan snapshot row: %w", err)
		}

		snapshot.Timestamp = createdAt
		if versionID.Valid {
			snapshot.Version = versionID.String
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(dataBytes, &snapshot.State); err != nil {
			return nil, fmt.Errorf("failed to unmarshal snapshot state: %w", err)
		}

		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &snapshot.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal snapshot metadata: %w", err)
			}
		}

		snapshots = append(snapshots, &snapshot)
	}

	return snapshots, nil
}

func (p *PostgreSQLBackend) BeginTransaction(ctx context.Context) (Transaction, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	return &PostgreSQLTransaction{
		backend: p,
		tx:      tx,
	}, nil
}

func (p *PostgreSQLBackend) Close() error {
	return p.db.Close()
}

func (p *PostgreSQLBackend) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

func (p *PostgreSQLBackend) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]interface{})
	for k, v := range p.stats {
		stats[k] = v
	}

	dbStats := p.db.Stats()
	stats["max_open_connections"] = dbStats.MaxOpenConnections
	stats["open_connections"] = dbStats.OpenConnections
	stats["in_use"] = dbStats.InUse
	stats["idle"] = dbStats.Idle
	stats["wait_count"] = dbStats.WaitCount
	stats["wait_duration"] = dbStats.WaitDuration
	stats["max_idle_closed"] = dbStats.MaxIdleClosed
	stats["max_idle_time_closed"] = dbStats.MaxIdleTimeClosed
	stats["max_lifetime_closed"] = dbStats.MaxLifetimeClosed

	return stats
}

// PostgreSQLTransaction implements Transaction for PostgreSQL
type PostgreSQLTransaction struct {
	backend *PostgreSQLBackend
	tx      *sql.Tx
}

func (t *PostgreSQLTransaction) SetState(ctx context.Context, stateID string, state map[string]interface{}) error {
	schema := t.backend.config.Schema
	if schema == "" {
		schema = "public"
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.ag_states (state_id, data) 
		VALUES ($1, $2) 
		ON CONFLICT (state_id) DO UPDATE SET 
			data = EXCLUDED.data, 
			updated_at = NOW()
	`, schema)

	if _, err := t.tx.ExecContext(ctx, query, stateID, data); err != nil {
		return fmt.Errorf("failed to set state in transaction: %w", err)
	}

	return nil
}

func (t *PostgreSQLTransaction) SaveVersion(ctx context.Context, stateID string, version *StateVersion) error {
	schema := t.backend.config.Schema
	if schema == "" {
		schema = "public"
	}

	// Marshal JSON fields
	dataBytes, err := json.Marshal(version.State)
	if err != nil {
		return fmt.Errorf("failed to marshal version state: %w", err)
	}

	var deltaBytes []byte
	if version.Delta != nil {
		deltaBytes, err = json.Marshal(version.Delta)
		if err != nil {
			return fmt.Errorf("failed to marshal version delta: %w", err)
		}
	}

	var metadataBytes []byte
	if version.Metadata != nil {
		metadataBytes, err = json.Marshal(version.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal version metadata: %w", err)
		}
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.ag_versions (version_id, state_id, data, delta, metadata, parent_id, created_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (version_id) DO UPDATE SET 
			data = EXCLUDED.data,
			delta = EXCLUDED.delta,
			metadata = EXCLUDED.metadata
	`, schema)

	var parentID sql.NullString
	if version.ParentID != "" {
		parentID.String = version.ParentID
		parentID.Valid = true
	}

	if _, err := t.tx.ExecContext(ctx, query, version.ID, stateID, dataBytes, deltaBytes, metadataBytes, parentID, version.Timestamp); err != nil {
		return fmt.Errorf("failed to save version in transaction: %w", err)
	}

	return nil
}

func (t *PostgreSQLTransaction) Commit(ctx context.Context) error {
	return t.tx.Commit()
}

func (t *PostgreSQLTransaction) Rollback(ctx context.Context) error {
	return t.tx.Rollback()
}

// File Backend Implementation

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

		// Create shard subdirectories if sharding is enabled
		if config.FileOptions.EnableSharding && config.FileOptions.ShardCount > 0 {
			for i := 0; i < config.FileOptions.ShardCount; i++ {
				shardPath := fmt.Sprintf("%s/shard_%d", path, i)
				if err := os.MkdirAll(shardPath, 0755); err != nil {
					return nil, fmt.Errorf("failed to create shard directory %s: %w", shardPath, err)
				}
			}
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

// PersistentStateStore wraps StateStore with pluggable storage backend
type PersistentStateStore struct {
	*StateStore                    // Embed StateStore for in-memory operations
	backend     StorageBackend     // Persistent storage backend
	config      *StorageConfig     // Storage configuration
	logger      Logger             // Logger instance
	mu          sync.RWMutex       // Protect operations
	enableSync  bool               // Enable synchronous persistence
	syncChannel chan persistOp     // Channel for async persistence operations
	ctx         context.Context    // Context for operations
	cancel      context.CancelFunc // Cancel function
	wg          sync.WaitGroup     // Wait group for cleanup
}

// persistOp represents a persistence operation
type persistOp struct {
	Type     string
	StateID  string
	Data     interface{}
	Callback func(error)
}

// PersistentStateStoreOption configures the PersistentStateStore
type PersistentStateStoreOption func(*PersistentStateStore)

// WithSynchronousPersistence enables synchronous persistence
func WithSynchronousPersistence(enabled bool) PersistentStateStoreOption {
	return func(s *PersistentStateStore) {
		s.enableSync = enabled
	}
}

// WithPersistentLogger sets the logger for persistent operations
func WithPersistentLogger(logger Logger) PersistentStateStoreOption {
	return func(s *PersistentStateStore) {
		s.logger = logger
	}
}

// NewPersistentStateStore creates a new state store with persistent storage
func NewPersistentStateStore(config *StorageConfig, storeOpts []StateStoreOption, persistOpts ...PersistentStateStoreOption) (*PersistentStateStore, error) {
	// Validate configuration
	if err := ValidateStorageConfig(config); err != nil {
		return nil, fmt.Errorf("invalid storage config: %w", err)
	}

	// Create in-memory state store first to get logger
	store := NewStateStore(storeOpts...)

	// Use logger from store (which may have been set via WithLogger option)
	logger := store.logger
	if logger == nil {
		logger = DefaultLogger()
	}

	// Create storage backend with the same logger
	backend, err := NewStorageBackend(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage backend: %w", err)
	}

	// Create context for operations
	ctx, cancel := context.WithCancel(context.Background())

	// Create persistent store
	persistentStore := &PersistentStateStore{
		StateStore:  store,
		backend:     backend,
		config:      config,
		logger:      logger,
		enableSync:  false,
		syncChannel: make(chan persistOp, 1000), // Buffer for async operations
		ctx:         ctx,
		cancel:      cancel,
	}

	// Apply options
	for _, opt := range persistOpts {
		opt(persistentStore)
	}

	// Start async persistence worker if not synchronous
	if !persistentStore.enableSync {
		persistentStore.wg.Add(1)
		go persistentStore.persistenceWorker()
	}

	// Load existing state from backend
	if err := persistentStore.loadFromBackend(); err != nil {
		persistentStore.logger.Warn("failed to load initial state from backend", Err(err))
	}

	persistentStore.logger.Info("Persistent state store initialized",
		String("backend_type", string(config.Type)),
		Bool("synchronous", persistentStore.enableSync))

	return persistentStore, nil
}

// loadFromBackend loads existing state from the storage backend
func (p *PersistentStateStore) loadFromBackend() error {
	// For now, we'll implement a simple single-state model
	// In a production system, you might want to load multiple states
	stateID := "default"

	ctx, cancel := context.WithTimeout(p.ctx, p.config.ReadTimeout)
	defer cancel()

	state, err := p.backend.GetState(ctx, stateID)
	if err != nil {
		return fmt.Errorf("failed to load state from backend: %w", err)
	}

	if len(state) > 0 {
		// Import the state into the in-memory store
		data, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("failed to marshal loaded state: %w", err)
		}

		if err := p.StateStore.Import(data); err != nil {
			return fmt.Errorf("failed to import loaded state: %w", err)
		}

		p.logger.Info("Loaded state from backend",
			String("state_id", stateID),
			Int("keys", len(state)))
	}

	return nil
}

// Set overrides StateStore.Set to persist changes
func (p *PersistentStateStore) Set(path string, value interface{}) error {
	// Apply to in-memory store first
	if err := p.StateStore.Set(path, value); err != nil {
		return err
	}

	// Persist the change
	return p.persistState("default")
}

// ApplyPatch overrides StateStore.ApplyPatch to persist changes
func (p *PersistentStateStore) ApplyPatch(patch JSONPatch) error {
	// Apply to in-memory store first
	if err := p.StateStore.ApplyPatch(patch); err != nil {
		return err
	}

	// Persist the change
	return p.persistState("default")
}

// Import overrides StateStore.Import to persist changes
func (p *PersistentStateStore) Import(data []byte) error {
	// Apply to in-memory store first
	if err := p.StateStore.Import(data); err != nil {
		return err
	}

	// Persist the change
	return p.persistState("default")
}

// CreateSnapshot creates a persistent snapshot
func (p *PersistentStateStore) CreatePersistentSnapshot() (*StateSnapshot, error) {
	// Create snapshot from in-memory store
	snapshot, err := p.StateStore.CreateSnapshot()
	if err != nil {
		return nil, err
	}

	// Persist the snapshot
	if err := p.persistSnapshot("default", snapshot); err != nil {
		p.logger.Error("failed to persist snapshot", Err(err))
		// Don't fail the operation, just log the error
	}

	return snapshot, nil
}

// GetPersistentSnapshots retrieves snapshots from the backend
func (p *PersistentStateStore) GetPersistentSnapshots(stateID string) ([]*StateSnapshot, error) {
	ctx, cancel := context.WithTimeout(p.ctx, p.config.ReadTimeout)
	defer cancel()

	return p.backend.ListSnapshots(ctx, stateID)
}

// RestorePersistentSnapshot restores from a persistent snapshot
func (p *PersistentStateStore) RestorePersistentSnapshot(stateID, snapshotID string) error {
	ctx, cancel := context.WithTimeout(p.ctx, p.config.ReadTimeout)
	defer cancel()

	// Get snapshot from backend
	snapshot, err := p.backend.GetSnapshot(ctx, stateID, snapshotID)
	if err != nil {
		return fmt.Errorf("failed to get snapshot from backend: %w", err)
	}

	// Restore to in-memory store
	if err := p.StateStore.RestoreSnapshot(snapshot); err != nil {
		return fmt.Errorf("failed to restore snapshot to in-memory store: %w", err)
	}

	// Persist the restored state
	return p.persistState(stateID)
}

// GetVersionHistory retrieves version history from the backend
func (p *PersistentStateStore) GetPersistentHistory(stateID string, limit int) ([]*StateVersion, error) {
	ctx, cancel := context.WithTimeout(p.ctx, p.config.ReadTimeout)
	defer cancel()

	return p.backend.GetVersionHistory(ctx, stateID, limit)
}

// persistState persists the current state to the backend
func (p *PersistentStateStore) persistState(stateID string) error {
	if p.enableSync {
		return p.persistStateSync(stateID)
	}

	// Async persistence
	op := persistOp{
		Type:    "state",
		StateID: stateID,
		Data:    p.StateStore.GetState(),
	}

	select {
	case p.syncChannel <- op:
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("persistence cancelled: %w", p.ctx.Err())
	default:
		return fmt.Errorf("persistence queue full")
	}
}

// persistStateSync persists state synchronously
func (p *PersistentStateStore) persistStateSync(stateID string) error {
	ctx, cancel := context.WithTimeout(p.ctx, p.config.WriteTimeout)
	defer cancel()

	state := p.StateStore.GetState()
	return p.backend.SetState(ctx, stateID, state)
}

// persistSnapshot persists a snapshot to the backend
func (p *PersistentStateStore) persistSnapshot(stateID string, snapshot *StateSnapshot) error {
	if p.enableSync {
		return p.persistSnapshotSync(stateID, snapshot)
	}

	// Async persistence
	op := persistOp{
		Type:    "snapshot",
		StateID: stateID,
		Data:    snapshot,
	}

	select {
	case p.syncChannel <- op:
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("persistence cancelled: %w", p.ctx.Err())
	default:
		return fmt.Errorf("persistence queue full")
	}
}

// persistSnapshotSync persists snapshot synchronously
func (p *PersistentStateStore) persistSnapshotSync(stateID string, snapshot *StateSnapshot) error {
	ctx, cancel := context.WithTimeout(p.ctx, p.config.WriteTimeout)
	defer cancel()

	return p.backend.SaveSnapshot(ctx, stateID, snapshot)
}

// persistenceWorker handles async persistence operations
func (p *PersistentStateStore) persistenceWorker() {
	defer p.wg.Done()

	for {
		select {
		case op := <-p.syncChannel:
			var err error
			ctx, cancel := context.WithTimeout(p.ctx, p.config.WriteTimeout)
			defer cancel()

			switch op.Type {
			case "state":
				if state, ok := op.Data.(map[string]interface{}); ok {
					err = p.backend.SetState(ctx, op.StateID, state)
				} else {
					err = fmt.Errorf("invalid state data type")
				}

			case "snapshot":
				if snapshot, ok := op.Data.(*StateSnapshot); ok {
					err = p.backend.SaveSnapshot(ctx, op.StateID, snapshot)
				} else {
					err = fmt.Errorf("invalid snapshot data type")
				}

			case "version":
				if version, ok := op.Data.(*StateVersion); ok {
					err = p.backend.SaveVersion(ctx, op.StateID, version)
				} else {
					err = fmt.Errorf("invalid version data type")
				}
			}

			if err != nil {
				p.logger.Error("async persistence failed",
					String("type", op.Type),
					String("state_id", op.StateID),
					Err(err))
			}

			// Call callback if provided
			if op.Callback != nil {
				op.Callback(err)
			}

		case <-p.ctx.Done():
			p.logger.Debug("persistence worker shutting down")
			return
		}
	}
}

// BeginPersistentTransaction starts a transaction that spans both memory and storage
func (p *PersistentStateStore) BeginPersistentTransaction() (*PersistentTransaction, error) {
	// Begin in-memory transaction
	memTx := p.StateStore.Begin()

	// Begin storage transaction
	ctx, cancel := context.WithTimeout(p.ctx, p.config.WriteTimeout)
	defer cancel()

	storageTx, err := p.backend.BeginTransaction(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin storage transaction: %w", err)
	}

	return &PersistentTransaction{
		memTx:     memTx,
		storageTx: storageTx,
		store:     p,
		committed: false,
	}, nil
}

// Close shuts down the persistent state store
func (p *PersistentStateStore) Close() error {
	p.logger.Info("shutting down persistent state store")

	// Cancel context to stop workers
	p.cancel()

	// Wait for workers to finish
	p.wg.Wait()

	// Close the underlying state store
	if p.StateStore != nil {
		p.StateStore.Close()
	}

	// Close storage backend
	if err := p.backend.Close(); err != nil {
		p.logger.Error("failed to close storage backend", Err(err))
		return err
	}

	p.logger.Info("persistent state store shutdown complete")
	return nil
}

// Ping checks the health of the storage backend
func (p *PersistentStateStore) Ping() error {
	ctx, cancel := context.WithTimeout(p.ctx, p.config.ConnectTimeout)
	defer cancel()

	return p.backend.Ping(ctx)
}

// Stats returns statistics from both memory and storage
func (p *PersistentStateStore) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]interface{})

	// Add memory stats
	stats["memory_version"] = p.StateStore.GetVersion()
	stats["memory_ref_count"] = p.StateStore.GetReferenceCount()

	// Add storage stats
	storageStats := p.backend.Stats()
	for k, v := range storageStats {
		stats["storage_"+k] = v
	}

	// Add persistence stats
	stats["sync_enabled"] = p.enableSync
	stats["sync_queue_size"] = len(p.syncChannel)
	stats["backend_type"] = string(p.config.Type)

	return stats
}

// PersistentTransaction represents a transaction that spans memory and storage
type PersistentTransaction struct {
	memTx     *StateTransaction
	storageTx Transaction
	store     *PersistentStateStore
	committed bool
	mu        sync.Mutex
}

// Apply adds a patch to both memory and storage transactions
func (pt *PersistentTransaction) Apply(patch JSONPatch) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.committed {
		return fmt.Errorf("transaction already committed")
	}

	// Apply to memory transaction
	if err := pt.memTx.Apply(patch); err != nil {
		return fmt.Errorf("failed to apply patch to memory transaction: %w", err)
	}

	// Note: Storage transaction will be updated on commit with the final state
	return nil
}

// Commit commits both memory and storage transactions
func (pt *PersistentTransaction) Commit() error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.committed {
		return fmt.Errorf("transaction already committed")
	}

	// Commit memory transaction first
	if err := pt.memTx.Commit(); err != nil {
		return fmt.Errorf("failed to commit memory transaction: %w", err)
	}

	// Get the updated state
	state := pt.store.StateStore.GetState()

	// Apply to storage transaction
	ctx, cancel := context.WithTimeout(pt.store.ctx, pt.store.config.WriteTimeout)
	defer cancel()

	if err := pt.storageTx.SetState(ctx, "default", state); err != nil {
		// Try to rollback memory transaction (though this might not be possible)
		pt.store.logger.Error("failed to apply state to storage transaction, attempting rollback", Err(err))
		return fmt.Errorf("failed to apply state to storage transaction: %w", err)
	}

	// Commit storage transaction
	if err := pt.storageTx.Commit(ctx); err != nil {
		pt.store.logger.Error("failed to commit storage transaction", Err(err))
		return fmt.Errorf("failed to commit storage transaction: %w", err)
	}

	pt.committed = true
	return nil
}

// Rollback rolls back both memory and storage transactions
func (pt *PersistentTransaction) Rollback() error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.committed {
		return fmt.Errorf("transaction already committed")
	}

	// Rollback storage transaction first
	ctx, cancel := context.WithTimeout(pt.store.ctx, pt.store.config.WriteTimeout)
	defer cancel()

	if err := pt.storageTx.Rollback(ctx); err != nil {
		pt.store.logger.Error("failed to rollback storage transaction", Err(err))
	}

	// Rollback memory transaction
	if err := pt.memTx.Rollback(); err != nil {
		return fmt.Errorf("failed to rollback memory transaction: %w", err)
	}

	pt.committed = true
	return nil
}
