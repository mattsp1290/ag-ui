package server

import (
	"fmt"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/server/middleware"
	"go.uber.org/zap"
)

// SessionConfig configures the session manager
type SessionConfig struct {
	// Backend configuration
	Backend         string        `json:"backend" yaml:"backend"`
	TTL             time.Duration `json:"ttl" yaml:"ttl"`
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`

	// Security settings
	SecureCookies   bool   `json:"secure_cookies" yaml:"secure_cookies"`
	HTTPOnlyCookies bool   `json:"http_only_cookies" yaml:"http_only_cookies"`
	SameSiteCookies string `json:"same_site_cookies" yaml:"same_site_cookies"`
	CookieName      string `json:"cookie_name" yaml:"cookie_name"`
	CookiePath      string `json:"cookie_path" yaml:"cookie_path"`
	CookieDomain    string `json:"cookie_domain" yaml:"cookie_domain"`

	// Session validation
	ValidateIP         bool `json:"validate_ip" yaml:"validate_ip"`
	ValidateUserAgent  bool `json:"validate_user_agent" yaml:"validate_user_agent"`
	MaxSessionsPerUser int  `json:"max_sessions_per_user" yaml:"max_sessions_per_user"`

	// Performance settings
	MaxConcurrentSessions int  `json:"max_concurrent_sessions" yaml:"max_concurrent_sessions"`
	SessionPoolSize       int  `json:"session_pool_size" yaml:"session_pool_size"`
	EnableCompression     bool `json:"enable_compression" yaml:"enable_compression"`

	// Backend-specific configurations with secure credential handling
	Redis    *SecureRedisSessionConfig    `json:"redis,omitempty" yaml:"redis,omitempty"`
	Database *SecureDatabaseSessionConfig `json:"database,omitempty" yaml:"database,omitempty"`
	Memory   *MemorySessionConfig         `json:"memory,omitempty" yaml:"memory,omitempty"`
}

// RedisSessionConfig configures Redis session storage with plain credentials
type RedisSessionConfig struct {
	Address    string `json:"address" yaml:"address"`
	Password   string `json:"password" yaml:"password"` // Plain password (less secure)
	DB         int    `json:"db" yaml:"db"`
	KeyPrefix  string `json:"key_prefix" yaml:"key_prefix"`
	PoolSize   int    `json:"pool_size" yaml:"pool_size"`
	MaxRetries int    `json:"max_retries" yaml:"max_retries"`
	EnableTLS  bool   `json:"enable_tls" yaml:"enable_tls"`
}

// SecureRedisSessionConfig configures Redis session storage with secure credential handling
type SecureRedisSessionConfig struct {
	Address     string `json:"address" yaml:"address"`
	PasswordEnv string `json:"password_env" yaml:"password_env"` // Environment variable name for Redis password
	DB          int    `json:"db" yaml:"db"`
	KeyPrefix   string `json:"key_prefix" yaml:"key_prefix"`
	PoolSize    int    `json:"pool_size" yaml:"pool_size"`
	MaxRetries  int    `json:"max_retries" yaml:"max_retries"`
	EnableTLS   bool   `json:"enable_tls" yaml:"enable_tls"`

	// Runtime secure credentials (populated from environment variables)
	password *middleware.SecureCredential
}

// LoadCredentials loads Redis credentials from environment variables
func (c *SecureRedisSessionConfig) LoadCredentials(logger *zap.Logger) error {
	if c.PasswordEnv != "" {
		var err error
		c.password, err = middleware.NewSecureCredential(c.PasswordEnv, middleware.DefaultPasswordValidator(), logger)
		if err != nil {
			return fmt.Errorf("failed to load Redis password: %w", err)
		}
	}
	return nil
}

// GetPassword returns the password credential
func (c *SecureRedisSessionConfig) GetPassword() *middleware.SecureCredential {
	return c.password
}

// Cleanup securely clears all credentials
func (c *SecureRedisSessionConfig) Cleanup() {
	if c.password != nil {
		c.password.Clear()
	}
}

// DatabaseSessionConfig configures database session storage with plain credentials
type DatabaseSessionConfig struct {
	Driver           string `json:"driver" yaml:"driver"`
	ConnectionString string `json:"connection_string" yaml:"connection_string"` // Plain connection string (less secure)
	TableName        string `json:"table_name" yaml:"table_name"`
	MaxConnections   int    `json:"max_connections" yaml:"max_connections"`
	EnableSSL        bool   `json:"enable_ssl" yaml:"enable_ssl"`
}

// SecureDatabaseSessionConfig configures database session storage with secure credential handling
type SecureDatabaseSessionConfig struct {
	Driver              string `json:"driver" yaml:"driver"`
	ConnectionStringEnv string `json:"connection_string_env" yaml:"connection_string_env"` // Environment variable name for connection string
	TableName           string `json:"table_name" yaml:"table_name"`
	MaxConnections      int    `json:"max_connections" yaml:"max_connections"`
	EnableSSL           bool   `json:"enable_ssl" yaml:"enable_ssl"`

	// Runtime secure credentials (populated from environment variables)
	connectionString *middleware.SecureCredential
}

// LoadCredentials loads database credentials from environment variables
func (c *SecureDatabaseSessionConfig) LoadCredentials(logger *zap.Logger) error {
	if c.ConnectionStringEnv == "" {
		return fmt.Errorf("database connection string environment variable not specified")
	}

	var err error
	c.connectionString, err = middleware.NewSecureCredential(c.ConnectionStringEnv, &middleware.CredentialValidator{MinLength: 10}, logger)
	if err != nil {
		return fmt.Errorf("failed to load database connection string: %w", err)
	}

	return nil
}

// GetConnectionString returns the connection string credential
func (c *SecureDatabaseSessionConfig) GetConnectionString() *middleware.SecureCredential {
	return c.connectionString
}

// Cleanup securely clears all credentials
func (c *SecureDatabaseSessionConfig) Cleanup() {
	if c.connectionString != nil {
		c.connectionString.Clear()
	}
}

// MemorySessionConfig configures in-memory session storage
type MemorySessionConfig struct {
	MaxSessions    int  `json:"max_sessions" yaml:"max_sessions"`
	EnableSharding bool `json:"enable_sharding" yaml:"enable_sharding"`
	ShardCount     int  `json:"shard_count" yaml:"shard_count"`
	
	// Memory management settings to prevent memory leaks
	EnableMapRecreation       bool          `json:"enable_map_recreation" yaml:"enable_map_recreation"`
	RecreationDeletionThreshold int         `json:"recreation_deletion_threshold" yaml:"recreation_deletion_threshold"`
	RecreationTimeThreshold   time.Duration `json:"recreation_time_threshold" yaml:"recreation_time_threshold"`
	MaxMapCapacityRatio       float64       `json:"max_map_capacity_ratio" yaml:"max_map_capacity_ratio"`
}

// DefaultSessionConfig returns default session configuration
func DefaultSessionConfig() *SessionConfig {
	return &SessionConfig{
		Backend:               "memory",
		TTL:                   DefaultSessionTTL,
		CleanupInterval:       DefaultCleanupInterval,
		SecureCookies:         true,
		HTTPOnlyCookies:       true,
		SameSiteCookies:       DefaultSameSiteCookies,
		CookieName:            DefaultCookieName,
		CookiePath:            DefaultCookiePath,
		ValidateIP:            false,
		ValidateUserAgent:     false,
		MaxSessionsPerUser:    DefaultMaxSessionsPerUser,
		MaxConcurrentSessions: DefaultMaxConcurrentSessions,
		SessionPoolSize:       DefaultSessionPoolSize,
		EnableCompression:     true,
		Memory: &MemorySessionConfig{
			MaxSessions:                 DefaultMaxConcurrentSessions,
			EnableSharding:              true,
			ShardCount:                  16,
			EnableMapRecreation:         true,
			RecreationDeletionThreshold: 1000,
			RecreationTimeThreshold:     30 * time.Minute,
			MaxMapCapacityRatio:         2.0,
		},
	}
}

// Validate validates session configuration
func (c *SessionConfig) Validate() error {
	// Basic configuration validation
	if err := c.validateBasicConfig(); err != nil {
		return err
	}

	// Backend-specific validation
	switch c.Backend {
	case "memory":
		return c.validateMemoryConfig()
	case "redis":
		return c.validateRedisConfig()
	case "database":
		return c.validateDatabaseConfig()
	default:
		return fmt.Errorf("unsupported session backend: %s", c.Backend)
	}
}

// validateBasicConfig validates basic session configuration
func (c *SessionConfig) validateBasicConfig() error {
	if c.TTL < MinSessionTTL {
		return fmt.Errorf("session TTL must be at least %v", MinSessionTTL)
	}

	if c.CleanupInterval < MinCleanupInterval {
		return fmt.Errorf("cleanup interval must be at least %v", MinCleanupInterval)
	}

	if c.MaxConcurrentSessions <= 0 {
		return fmt.Errorf("max concurrent sessions must be positive")
	}

	if c.SessionPoolSize <= 0 {
		return fmt.Errorf("session pool size must be positive")
	}

	if c.MaxSessionsPerUser < 0 {
		return fmt.Errorf("max sessions per user cannot be negative")
	}

	// Validate cookie configuration
	if err := c.validateCookieConfig(); err != nil {
		return fmt.Errorf("invalid cookie configuration: %w", err)
	}

	return nil
}

// validateCookieConfig validates cookie configuration
func (c *SessionConfig) validateCookieConfig() error {
	if c.CookieName == "" {
		return fmt.Errorf("cookie name cannot be empty")
	}

	if c.CookiePath == "" {
		return fmt.Errorf("cookie path cannot be empty")
	}

	// Validate SameSite cookie values
	switch c.SameSiteCookies {
	case "Strict", "Lax", "None", "":
		// Valid values
	default:
		return fmt.Errorf("invalid SameSite cookie value: %s", c.SameSiteCookies)
	}

	return nil
}

// validateMemoryConfig validates memory backend configuration
func (c *SessionConfig) validateMemoryConfig() error {
	if c.Memory == nil {
		return fmt.Errorf("memory config required for memory backend")
	}

	if c.Memory.MaxSessions <= 0 {
		return fmt.Errorf("max sessions must be positive")
	}

	if c.Memory.EnableSharding && c.Memory.ShardCount <= 0 {
		return fmt.Errorf("shard count must be positive when sharding is enabled")
	}

	// Validate memory management settings
	if c.Memory.EnableMapRecreation {
		if c.Memory.RecreationDeletionThreshold <= 0 {
			return fmt.Errorf("recreation deletion threshold must be positive when map recreation is enabled")
		}
		
		if c.Memory.RecreationTimeThreshold <= 0 {
			return fmt.Errorf("recreation time threshold must be positive when map recreation is enabled")
		}
		
		if c.Memory.MaxMapCapacityRatio <= 1.0 {
			return fmt.Errorf("max map capacity ratio must be greater than 1.0 when map recreation is enabled")
		}
	}

	return nil
}

// validateRedisConfig validates Redis backend configuration
func (c *SessionConfig) validateRedisConfig() error {
	if c.Redis == nil {
		return fmt.Errorf("redis config required for redis backend")
	}

	if c.Redis.Address == "" {
		return fmt.Errorf("redis address is required")
	}

	if c.Redis.PasswordEnv == "" {
		return fmt.Errorf("redis password environment variable is required for secure configuration")
	}

	if c.Redis.PoolSize <= 0 {
		return fmt.Errorf("redis pool size must be positive")
	}

	if c.Redis.MaxRetries < 0 {
		return fmt.Errorf("redis max retries cannot be negative")
	}

	return nil
}

// validateDatabaseConfig validates database backend configuration
func (c *SessionConfig) validateDatabaseConfig() error {
	if c.Database == nil {
		return fmt.Errorf("database config required for database backend")
	}

	if c.Database.Driver == "" {
		return fmt.Errorf("database driver is required")
	}

	if c.Database.ConnectionStringEnv == "" {
		return fmt.Errorf("database connection string environment variable is required for secure configuration")
	}

	if c.Database.TableName == "" {
		return fmt.Errorf("database table name is required")
	}

	if c.Database.MaxConnections <= 0 {
		return fmt.Errorf("database max connections must be positive")
	}

	// Validate supported database drivers
	switch c.Database.Driver {
	case "postgres", "postgresql", "mysql", "sqlite", "sqlite3":
		// Supported drivers
	default:
		return fmt.Errorf("unsupported database driver: %s", c.Database.Driver)
	}

	return nil
}

// SetDefaults sets default values for configuration fields
func (c *SessionConfig) SetDefaults() {
	if c.Backend == "" {
		c.Backend = "memory"
	}

	if c.TTL == 0 {
		c.TTL = DefaultSessionTTL
	}

	if c.CleanupInterval == 0 {
		c.CleanupInterval = DefaultCleanupInterval
	}

	if c.CookieName == "" {
		c.CookieName = DefaultCookieName
	}

	if c.CookiePath == "" {
		c.CookiePath = DefaultCookiePath
	}

	if c.SameSiteCookies == "" {
		c.SameSiteCookies = DefaultSameSiteCookies
	}

	if c.MaxSessionsPerUser == 0 {
		c.MaxSessionsPerUser = DefaultMaxSessionsPerUser
	}

	if c.MaxConcurrentSessions == 0 {
		c.MaxConcurrentSessions = DefaultMaxConcurrentSessions
	}

	if c.SessionPoolSize == 0 {
		c.SessionPoolSize = DefaultSessionPoolSize
	}

	// Set backend-specific defaults
	switch c.Backend {
	case "memory":
		if c.Memory == nil {
			c.Memory = &MemorySessionConfig{
				MaxSessions:                 DefaultMaxConcurrentSessions,
				EnableSharding:              true,
				ShardCount:                  16,
				EnableMapRecreation:         true,
				RecreationDeletionThreshold: 1000,
				RecreationTimeThreshold:     30 * time.Minute,
				MaxMapCapacityRatio:         2.0,
			}
		}
	case "redis":
		if c.Redis != nil {
			if c.Redis.PoolSize == 0 {
				c.Redis.PoolSize = 10
			}
			if c.Redis.KeyPrefix == "" {
				c.Redis.KeyPrefix = "session:"
			}
		}
	case "database":
		if c.Database != nil {
			if c.Database.MaxConnections == 0 {
				c.Database.MaxConnections = 10
			}
			if c.Database.TableName == "" {
				c.Database.TableName = "sessions"
			}
		}
	}
}
