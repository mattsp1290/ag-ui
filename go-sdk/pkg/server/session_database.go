package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Database storage constants
const (
	DefaultDatabaseMaxConnections = 10
	DefaultDatabaseTableName      = "sessions"
	DatabaseConnectionTimeout     = 10 * time.Second
	DatabaseConnectionMaxLifetime = 5 * time.Minute
)

// DatabaseSessionStorage implements database-based session storage
type DatabaseSessionStorage struct {
	config *DatabaseSessionConfig
	logger *zap.Logger
	db     *sql.DB
}

// SecureDatabaseSessionStorage implements secure database-based session storage
type SecureDatabaseSessionStorage struct {
	config *SecureDatabaseSessionConfig
	logger *zap.Logger
	db     *sql.DB
}

// NewDatabaseSessionStorage creates a new database session storage
func NewDatabaseSessionStorage(config *DatabaseSessionConfig, logger *zap.Logger) (*DatabaseSessionStorage, error) {
	if config == nil {
		return nil, fmt.Errorf("database config cannot be nil")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Validate and set defaults
	if err := validateDatabaseConfig(config); err != nil {
		return nil, fmt.Errorf("invalid database config: %w", err)
	}

	// Open database connection
	db, err := sql.Open(config.Driver, config.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	configureDBConnectionPool(db, config.MaxConnections)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), DatabaseConnectionTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	storage := &DatabaseSessionStorage{
		config: config,
		logger: logger,
		db:     db,
	}

	// Initialize schema
	if err := storage.initSchema(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logger.Info("Database session storage initialized",
		zap.String("driver", config.Driver),
		zap.String("table_name", config.TableName),
		zap.Int("max_connections", config.MaxConnections))

	return storage, nil
}

// NewSecureDatabaseSessionStorage creates a new secure database session storage
func NewSecureDatabaseSessionStorage(config *SecureDatabaseSessionConfig, logger *zap.Logger) (*SecureDatabaseSessionStorage, error) {
	if config == nil {
		return nil, fmt.Errorf("database config cannot be nil")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Load credentials before creating storage
	if err := config.LoadCredentials(logger); err != nil {
		return nil, fmt.Errorf("failed to load database credentials: %w", err)
	}

	// Validate and set defaults
	if err := validateSecureDatabaseConfig(config); err != nil {
		return nil, fmt.Errorf("invalid secure database config: %w", err)
	}

	// Open database connection using secure credentials
	connectionString := config.GetConnectionString().Value()
	db, err := sql.Open(config.Driver, connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	configureDBConnectionPool(db, config.MaxConnections)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), DatabaseConnectionTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	storage := &SecureDatabaseSessionStorage{
		config: config,
		logger: logger,
		db:     db,
	}

	// Initialize schema
	if err := storage.initSchema(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logger.Info("Secure database session storage initialized",
		zap.String("driver", config.Driver),
		zap.String("table_name", config.TableName),
		zap.Int("max_connections", config.MaxConnections),
		zap.Bool("credentials_loaded", config.GetConnectionString() != nil))

	return storage, nil
}

// Configuration validation functions

func validateDatabaseConfig(config *DatabaseSessionConfig) error {
	if config.Driver == "" {
		return fmt.Errorf("database driver is required")
	}

	if config.ConnectionString == "" {
		return fmt.Errorf("database connection string is required")
	}

	if config.TableName == "" {
		config.TableName = DefaultDatabaseTableName
	}

	if config.MaxConnections <= 0 {
		config.MaxConnections = DefaultDatabaseMaxConnections
	}

	// Validate supported drivers
	switch config.Driver {
	case "postgres", "postgresql", "mysql", "sqlite", "sqlite3":
		// Supported drivers
	default:
		return fmt.Errorf("unsupported database driver: %s", config.Driver)
	}

	return nil
}

func validateSecureDatabaseConfig(config *SecureDatabaseSessionConfig) error {
	if config.Driver == "" {
		return fmt.Errorf("database driver is required")
	}

	if config.ConnectionStringEnv == "" {
		return fmt.Errorf("database connection string environment variable is required")
	}

	if config.TableName == "" {
		config.TableName = DefaultDatabaseTableName
	}

	if config.MaxConnections <= 0 {
		config.MaxConnections = DefaultDatabaseMaxConnections
	}

	// Validate supported drivers
	switch config.Driver {
	case "postgres", "postgresql", "mysql", "sqlite", "sqlite3":
		// Supported drivers
	default:
		return fmt.Errorf("unsupported database driver: %s", config.Driver)
	}

	return nil
}

// configureDBConnectionPool configures database connection pool settings
func configureDBConnectionPool(db *sql.DB, maxConnections int) {
	db.SetMaxOpenConns(maxConnections)
	db.SetMaxIdleConns(maxConnections / 2)
	db.SetConnMaxLifetime(DatabaseConnectionMaxLifetime)
}

// Schema initialization

func (d *DatabaseSessionStorage) initSchema(ctx context.Context) error {
	return initDatabaseSchema(ctx, d.db, d.config.Driver, d.config.TableName, d.logger)
}

func (d *SecureDatabaseSessionStorage) initSchema(ctx context.Context) error {
	return initDatabaseSchema(ctx, d.db, d.config.Driver, d.config.TableName, d.logger)
}

// initDatabaseSchema creates the sessions table with appropriate schema for the database driver
func initDatabaseSchema(ctx context.Context, db *sql.DB, driver, tableName string, logger *zap.Logger) error {
	var createTableSQL string

	switch driver {
	case "postgres", "postgresql":
		createTableSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id VARCHAR(64) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				created_at TIMESTAMP WITH TIME ZONE NOT NULL,
				last_accessed TIMESTAMP WITH TIME ZONE NOT NULL,
				expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
				ip_address INET,
				user_agent TEXT,
				is_active BOOLEAN NOT NULL DEFAULT true,
				data JSONB,
				metadata JSONB
			)`, tableName)

		// Create indexes for PostgreSQL
		indexQueries := []string{
			fmt.Sprintf("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_%s_user_id ON %s (user_id)", tableName, tableName),
			fmt.Sprintf("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_%s_expires_at ON %s (expires_at)", tableName, tableName),
			fmt.Sprintf("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_%s_last_accessed ON %s (last_accessed)", tableName, tableName),
			fmt.Sprintf("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_%s_active_expires ON %s (is_active, expires_at) WHERE is_active = true", tableName, tableName),
		}

		if _, err := db.ExecContext(ctx, createTableSQL); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}

		for _, indexSQL := range indexQueries {
			if _, err := db.ExecContext(ctx, indexSQL); err != nil {
				logger.Warn("Failed to create index", zap.String("sql", indexSQL), zap.Error(err))
			}
		}

	case "mysql":
		createTableSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id VARCHAR(64) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				created_at TIMESTAMP NOT NULL,
				last_accessed TIMESTAMP NOT NULL,
				expires_at TIMESTAMP NOT NULL,
				ip_address VARCHAR(45),
				user_agent TEXT,
				is_active BOOLEAN NOT NULL DEFAULT true,
				data JSON,
				metadata JSON,
				INDEX idx_user_id (user_id),
				INDEX idx_expires_at (expires_at),
				INDEX idx_last_accessed (last_accessed),
				INDEX idx_active_expires (is_active, expires_at)
			) ENGINE=InnoDB`, tableName)

		if _, err := db.ExecContext(ctx, createTableSQL); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}

	case "sqlite3", "sqlite":
		createTableSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL,
				created_at DATETIME NOT NULL,
				last_accessed DATETIME NOT NULL,
				expires_at DATETIME NOT NULL,
				ip_address TEXT,
				user_agent TEXT,
				is_active BOOLEAN NOT NULL DEFAULT 1,
				data TEXT,
				metadata TEXT
			)`, tableName)

		// Create indexes separately for SQLite
		indexQueries := []string{
			fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_user_id ON %s (user_id)", tableName, tableName),
			fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_expires_at ON %s (expires_at)", tableName, tableName),
			fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_last_accessed ON %s (last_accessed)", tableName, tableName),
			fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_active_expires ON %s (is_active, expires_at) WHERE is_active = 1", tableName, tableName),
		}

		if _, err := db.ExecContext(ctx, createTableSQL); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}

		for _, indexSQL := range indexQueries {
			if _, err := db.ExecContext(ctx, indexSQL); err != nil {
				logger.Warn("Failed to create index", zap.String("sql", indexSQL), zap.Error(err))
			}
		}

	default:
		return fmt.Errorf("unsupported database driver: %s", driver)
	}

	return nil
}

// DatabaseSessionStorage implementation

func (d *DatabaseSessionStorage) CreateSession(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	dataJSON, err := json.Marshal(session.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal session metadata: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (id, user_id, created_at, last_accessed, expires_at, ip_address, user_agent, is_active, data, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, d.config.TableName)

	// Adjust placeholders for PostgreSQL
	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$")
		for i := 1; i <= 10; i++ {
			query = strings.Replace(query, "$", fmt.Sprintf("$%d", i), 1)
		}
	}

	_, err = d.db.ExecContext(ctx, query,
		session.ID, session.UserID, session.CreatedAt, session.LastAccessed,
		session.ExpiresAt, session.IPAddress, session.UserAgent, session.IsActive,
		string(dataJSON), string(metadataJSON))

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

func (d *DatabaseSessionStorage) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, created_at, last_accessed, expires_at, ip_address, user_agent, is_active, data, metadata
		FROM %s WHERE id = ?
	`, d.config.TableName)

	// Adjust placeholders for PostgreSQL
	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$1")
	}

	row := d.db.QueryRowContext(ctx, query, sessionID)

	var session Session
	var dataJSON, metadataJSON string

	err := row.Scan(&session.ID, &session.UserID, &session.CreatedAt,
		&session.LastAccessed, &session.ExpiresAt, &session.IPAddress,
		&session.UserAgent, &session.IsActive, &dataJSON, &metadataJSON)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Session not found
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Unmarshal JSON data
	if err := json.Unmarshal([]byte(dataJSON), &session.Data); err != nil {
		d.logger.Warn("Failed to unmarshal session data", zap.String("session_id", sessionID), zap.Error(err))
		session.Data = make(map[string]interface{})
	}

	if err := json.Unmarshal([]byte(metadataJSON), &session.Metadata); err != nil {
		d.logger.Warn("Failed to unmarshal session metadata", zap.String("session_id", sessionID), zap.Error(err))
		session.Metadata = make(map[string]interface{})
	}

	return &session, nil
}

func (d *DatabaseSessionStorage) UpdateSession(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	dataJSON, err := json.Marshal(session.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal session metadata: %w", err)
	}

	query := fmt.Sprintf(`
		UPDATE %s SET last_accessed = ?, expires_at = ?, is_active = ?, data = ?, metadata = ?
		WHERE id = ?
	`, d.config.TableName)

	// Adjust placeholders for PostgreSQL
	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$")
		for i := 1; i <= 6; i++ {
			query = strings.Replace(query, "$", fmt.Sprintf("$%d", i), 1)
		}
	}

	result, err := d.db.ExecContext(ctx, query,
		session.LastAccessed, session.ExpiresAt, session.IsActive,
		string(dataJSON), string(metadataJSON), session.ID)

	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

func (d *DatabaseSessionStorage) DeleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", d.config.TableName)

	// Adjust placeholders for PostgreSQL
	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$1")
	}

	_, err := d.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

func (d *DatabaseSessionStorage) GetUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, created_at, last_accessed, expires_at, ip_address, user_agent, is_active, data, metadata
		FROM %s WHERE user_id = ? ORDER BY last_accessed DESC
	`, d.config.TableName)

	// Adjust placeholders for PostgreSQL
	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$1")
	}

	rows, err := d.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var session Session
		var dataJSON, metadataJSON string

		err := rows.Scan(&session.ID, &session.UserID, &session.CreatedAt,
			&session.LastAccessed, &session.ExpiresAt, &session.IPAddress,
			&session.UserAgent, &session.IsActive, &dataJSON, &metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		// Unmarshal JSON data
		if err := json.Unmarshal([]byte(dataJSON), &session.Data); err != nil {
			d.logger.Warn("Failed to unmarshal session data", zap.String("session_id", session.ID), zap.Error(err))
			session.Data = make(map[string]interface{})
		}

		if err := json.Unmarshal([]byte(metadataJSON), &session.Metadata); err != nil {
			d.logger.Warn("Failed to unmarshal session metadata", zap.String("session_id", session.ID), zap.Error(err))
			session.Metadata = make(map[string]interface{})
		}

		sessions = append(sessions, &session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return sessions, nil
}

func (d *DatabaseSessionStorage) DeleteUserSessions(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE user_id = ?", d.config.TableName)

	// Adjust placeholders for PostgreSQL
	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$1")
	}

	result, err := d.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		d.logger.Warn("Failed to get rows affected for user session deletion", zap.Error(err))
	} else {
		d.logger.Debug("Deleted user sessions",
			zap.String("user_id", userID),
			zap.Int64("count", rowsAffected))
	}

	return nil
}

func (d *DatabaseSessionStorage) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	query := fmt.Sprintf("DELETE FROM %s WHERE expires_at < ? OR is_active = false", d.config.TableName)

	// Adjust placeholders for PostgreSQL
	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$1")
	}

	result, err := d.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

func (d *DatabaseSessionStorage) GetActiveSessions(ctx context.Context, limit int) ([]*Session, error) {
	if limit <= 0 {
		return []*Session{}, nil
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, created_at, last_accessed, expires_at, ip_address, user_agent, is_active, data, metadata
		FROM %s WHERE is_active = ? AND expires_at > ?
		ORDER BY last_accessed DESC LIMIT ?
	`, d.config.TableName)

	// Adjust placeholders and LIMIT syntax for different databases
	switch d.config.Driver {
	case "postgres", "postgresql":
		query = strings.ReplaceAll(query, "?", "$")
		for i := 1; i <= 3; i++ {
			query = strings.Replace(query, "$", fmt.Sprintf("$%d", i), 1)
		}
	case "sqlite3", "sqlite":
		// SQLite uses ? placeholders and standard LIMIT syntax
	case "mysql":
		// MySQL uses ? placeholders and standard LIMIT syntax
	}

	rows, err := d.db.QueryContext(ctx, query, true, time.Now(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var session Session
		var dataJSON, metadataJSON string

		err := rows.Scan(&session.ID, &session.UserID, &session.CreatedAt,
			&session.LastAccessed, &session.ExpiresAt, &session.IPAddress,
			&session.UserAgent, &session.IsActive, &dataJSON, &metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		// Unmarshal JSON data
		if err := json.Unmarshal([]byte(dataJSON), &session.Data); err != nil {
			d.logger.Warn("Failed to unmarshal session data", zap.String("session_id", session.ID), zap.Error(err))
			session.Data = make(map[string]interface{})
		}

		if err := json.Unmarshal([]byte(metadataJSON), &session.Metadata); err != nil {
			d.logger.Warn("Failed to unmarshal session metadata", zap.String("session_id", session.ID), zap.Error(err))
			session.Metadata = make(map[string]interface{})
		}

		sessions = append(sessions, &session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return sessions, nil
}

func (d *DatabaseSessionStorage) CountSessions(ctx context.Context) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE is_active = ? AND expires_at > ?", d.config.TableName)

	// Adjust placeholders for PostgreSQL
	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$")
		for i := 1; i <= 2; i++ {
			query = strings.Replace(query, "$", fmt.Sprintf("$%d", i), 1)
		}
	}

	var count int64
	err := d.db.QueryRowContext(ctx, query, true, time.Now()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions: %w", err)
	}

	return count, nil
}

func (d *DatabaseSessionStorage) Close() error {
	d.logger.Info("Closing database session storage")
	return d.db.Close()
}

func (d *DatabaseSessionStorage) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

func (d *DatabaseSessionStorage) Stats() map[string]interface{} {
	stats := map[string]interface{}{
		"type":       "database",
		"driver":     d.config.Driver,
		"table_name": d.config.TableName,
	}

	// Get database statistics
	if db := d.db; db != nil {
		dbStats := db.Stats()
		stats["open_connections"] = dbStats.OpenConnections
		stats["in_use"] = dbStats.InUse
		stats["idle"] = dbStats.Idle
		stats["wait_count"] = dbStats.WaitCount
		stats["wait_duration_ms"] = dbStats.WaitDuration.Milliseconds()
		stats["max_idle_closed"] = dbStats.MaxIdleClosed
		stats["max_idle_time_closed"] = dbStats.MaxIdleTimeClosed
		stats["max_lifetime_closed"] = dbStats.MaxLifetimeClosed

		// Try to get session count
		count, err := d.CountSessions(context.Background())
		if err == nil {
			stats["session_count"] = count
		}

		// Check connection health
		if err := d.Ping(context.Background()); err != nil {
			stats["status"] = "disconnected"
			stats["error"] = err.Error()
		} else {
			stats["status"] = "connected"
		}
	} else {
		stats["status"] = "not_initialized"
	}

	return stats
}

// SecureDatabaseSessionStorage implementation (delegates to helper functions)

func (d *SecureDatabaseSessionStorage) CreateSession(ctx context.Context, session *Session) error {
	return d.createSession(ctx, session)
}

func (d *SecureDatabaseSessionStorage) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	return d.getSession(ctx, sessionID)
}

func (d *SecureDatabaseSessionStorage) UpdateSession(ctx context.Context, session *Session) error {
	return d.updateSession(ctx, session)
}

func (d *SecureDatabaseSessionStorage) DeleteSession(ctx context.Context, sessionID string) error {
	return d.deleteSession(ctx, sessionID)
}

func (d *SecureDatabaseSessionStorage) GetUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	return d.getUserSessions(ctx, userID)
}

func (d *SecureDatabaseSessionStorage) DeleteUserSessions(ctx context.Context, userID string) error {
	return d.deleteUserSessions(ctx, userID)
}

func (d *SecureDatabaseSessionStorage) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	return d.cleanupExpiredSessions(ctx)
}

func (d *SecureDatabaseSessionStorage) GetActiveSessions(ctx context.Context, limit int) ([]*Session, error) {
	return d.getActiveSessions(ctx, limit)
}

func (d *SecureDatabaseSessionStorage) CountSessions(ctx context.Context) (int64, error) {
	return d.countSessions(ctx)
}

func (d *SecureDatabaseSessionStorage) Close() error {
	d.logger.Info("Closing secure database session storage")

	// Close database connection
	var err error
	if d.db != nil {
		err = d.db.Close()
	}

	// Cleanup credentials
	if d.config != nil {
		d.config.Cleanup()
	}

	return err
}

func (d *SecureDatabaseSessionStorage) Ping(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return d.db.PingContext(ctx)
}

func (d *SecureDatabaseSessionStorage) Stats() map[string]interface{} {
	stats := map[string]interface{}{
		"type":               "secure_database",
		"driver":             d.config.Driver,
		"table_name":         d.config.TableName,
		"credentials_loaded": d.config.GetConnectionString() != nil,
	}

	// Get database statistics
	if db := d.db; db != nil {
		dbStats := db.Stats()
		stats["open_connections"] = dbStats.OpenConnections
		stats["in_use"] = dbStats.InUse
		stats["idle"] = dbStats.Idle
		stats["wait_count"] = dbStats.WaitCount
		stats["wait_duration_ms"] = dbStats.WaitDuration.Milliseconds()

		// Try to get session count
		count, err := d.countSessions(context.Background())
		if err == nil {
			stats["session_count"] = count
		}

		// Check connection health
		if err := d.Ping(context.Background()); err != nil {
			stats["status"] = "disconnected"
			stats["error"] = err.Error()
		} else {
			stats["status"] = "connected"
		}
	} else {
		stats["status"] = "not_initialized"
	}

	return stats
}

// Helper methods for SecureDatabaseSessionStorage (same implementation as regular database storage)

func (d *SecureDatabaseSessionStorage) createSession(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	dataJSON, err := json.Marshal(session.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal session metadata: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (id, user_id, created_at, last_accessed, expires_at, ip_address, user_agent, is_active, data, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, d.config.TableName)

	// Adjust placeholders for PostgreSQL
	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$")
		for i := 1; i <= 10; i++ {
			query = strings.Replace(query, "$", fmt.Sprintf("$%d", i), 1)
		}
	}

	_, err = d.db.ExecContext(ctx, query,
		session.ID, session.UserID, session.CreatedAt, session.LastAccessed,
		session.ExpiresAt, session.IPAddress, session.UserAgent, session.IsActive,
		string(dataJSON), string(metadataJSON))

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

func (d *SecureDatabaseSessionStorage) getSession(ctx context.Context, sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, created_at, last_accessed, expires_at, ip_address, user_agent, is_active, data, metadata
		FROM %s WHERE id = ?
	`, d.config.TableName)

	// Adjust placeholders for PostgreSQL
	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$1")
	}

	row := d.db.QueryRowContext(ctx, query, sessionID)

	var session Session
	var dataJSON, metadataJSON string

	err := row.Scan(&session.ID, &session.UserID, &session.CreatedAt,
		&session.LastAccessed, &session.ExpiresAt, &session.IPAddress,
		&session.UserAgent, &session.IsActive, &dataJSON, &metadataJSON)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Session not found
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Unmarshal JSON data
	if err := json.Unmarshal([]byte(dataJSON), &session.Data); err != nil {
		d.logger.Warn("Failed to unmarshal session data", zap.String("session_id", sessionID), zap.Error(err))
		session.Data = make(map[string]interface{})
	}

	if err := json.Unmarshal([]byte(metadataJSON), &session.Metadata); err != nil {
		d.logger.Warn("Failed to unmarshal session metadata", zap.String("session_id", sessionID), zap.Error(err))
		session.Metadata = make(map[string]interface{})
	}

	return &session, nil
}

// Additional helper methods for SecureDatabaseSessionStorage would follow the same pattern...
// (updateSession, deleteSession, getUserSessions, deleteUserSessions, etc.)
// These are implemented identically to the regular database storage methods

func (d *SecureDatabaseSessionStorage) updateSession(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	dataJSON, err := json.Marshal(session.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal session metadata: %w", err)
	}

	query := fmt.Sprintf(`
		UPDATE %s SET last_accessed = ?, expires_at = ?, is_active = ?, data = ?, metadata = ?
		WHERE id = ?
	`, d.config.TableName)

	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$")
		for i := 1; i <= 6; i++ {
			query = strings.Replace(query, "$", fmt.Sprintf("$%d", i), 1)
		}
	}

	result, err := d.db.ExecContext(ctx, query,
		session.LastAccessed, session.ExpiresAt, session.IsActive,
		string(dataJSON), string(metadataJSON), session.ID)

	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

func (d *SecureDatabaseSessionStorage) deleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", d.config.TableName)

	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$1")
	}

	_, err := d.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

func (d *SecureDatabaseSessionStorage) getUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, created_at, last_accessed, expires_at, ip_address, user_agent, is_active, data, metadata
		FROM %s WHERE user_id = ? ORDER BY last_accessed DESC
	`, d.config.TableName)

	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$1")
	}

	rows, err := d.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var session Session
		var dataJSON, metadataJSON string

		err := rows.Scan(&session.ID, &session.UserID, &session.CreatedAt,
			&session.LastAccessed, &session.ExpiresAt, &session.IPAddress,
			&session.UserAgent, &session.IsActive, &dataJSON, &metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		if err := json.Unmarshal([]byte(dataJSON), &session.Data); err != nil {
			d.logger.Warn("Failed to unmarshal session data", zap.String("session_id", session.ID), zap.Error(err))
			session.Data = make(map[string]interface{})
		}

		if err := json.Unmarshal([]byte(metadataJSON), &session.Metadata); err != nil {
			d.logger.Warn("Failed to unmarshal session metadata", zap.String("session_id", session.ID), zap.Error(err))
			session.Metadata = make(map[string]interface{})
		}

		sessions = append(sessions, &session)
	}

	return sessions, nil
}

func (d *SecureDatabaseSessionStorage) deleteUserSessions(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE user_id = ?", d.config.TableName)

	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$1")
	}

	result, err := d.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		d.logger.Warn("Failed to get rows affected for user session deletion", zap.Error(err))
	} else {
		d.logger.Debug("Deleted user sessions",
			zap.String("user_id", userID),
			zap.Int64("count", rowsAffected))
	}

	return nil
}

func (d *SecureDatabaseSessionStorage) cleanupExpiredSessions(ctx context.Context) (int64, error) {
	query := fmt.Sprintf("DELETE FROM %s WHERE expires_at < ? OR is_active = false", d.config.TableName)

	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$1")
	}

	result, err := d.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

func (d *SecureDatabaseSessionStorage) getActiveSessions(ctx context.Context, limit int) ([]*Session, error) {
	if limit <= 0 {
		return []*Session{}, nil
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, created_at, last_accessed, expires_at, ip_address, user_agent, is_active, data, metadata
		FROM %s WHERE is_active = ? AND expires_at > ?
		ORDER BY last_accessed DESC LIMIT ?
	`, d.config.TableName)

	switch d.config.Driver {
	case "postgres", "postgresql":
		query = strings.ReplaceAll(query, "?", "$")
		for i := 1; i <= 3; i++ {
			query = strings.Replace(query, "$", fmt.Sprintf("$%d", i), 1)
		}
	}

	rows, err := d.db.QueryContext(ctx, query, true, time.Now(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var session Session
		var dataJSON, metadataJSON string

		err := rows.Scan(&session.ID, &session.UserID, &session.CreatedAt,
			&session.LastAccessed, &session.ExpiresAt, &session.IPAddress,
			&session.UserAgent, &session.IsActive, &dataJSON, &metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		if err := json.Unmarshal([]byte(dataJSON), &session.Data); err != nil {
			d.logger.Warn("Failed to unmarshal session data", zap.Error(err))
			session.Data = make(map[string]interface{})
		}

		if err := json.Unmarshal([]byte(metadataJSON), &session.Metadata); err != nil {
			d.logger.Warn("Failed to unmarshal session metadata", zap.Error(err))
			session.Metadata = make(map[string]interface{})
		}

		sessions = append(sessions, &session)
	}

	return sessions, nil
}

func (d *SecureDatabaseSessionStorage) countSessions(ctx context.Context) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE is_active = ? AND expires_at > ?", d.config.TableName)

	if d.config.Driver == "postgres" || d.config.Driver == "postgresql" {
		query = strings.ReplaceAll(query, "?", "$")
		for i := 1; i <= 2; i++ {
			query = strings.Replace(query, "$", fmt.Sprintf("$%d", i), 1)
		}
	}

	var count int64
	err := d.db.QueryRowContext(ctx, query, true, time.Now()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions: %w", err)
	}

	return count, nil
}
