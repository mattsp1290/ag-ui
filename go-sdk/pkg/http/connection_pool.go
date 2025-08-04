package httppool

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"
	"github.com/mattsp1290/ag-ui/go-sdk/internal"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"github.com/sirupsen/logrus"
)

// HTTPConnectionPool provides HTTP connection pooling with health monitoring,
// load balancing, and comprehensive metrics collection.
type HTTPConnectionPool struct {
	// Configuration
	config *HTTPPoolConfig
	
	// Connection management
	pools   *internal.BoundedMap // Bounded map for server pools to prevent memory leaks
	poolsMu sync.RWMutex
	
	// Load balancing
	servers   []*ServerTarget
	serversMu sync.RWMutex
	
	// Metrics and monitoring
	metrics   *HTTPPoolMetrics
	metricsMu sync.RWMutex
	
	// Health monitoring
	healthChecker *HealthChecker
	
	// Lifecycle management
	shutdown     chan struct{}
	shutdownOnce sync.Once
	isShutdown   int32
	
	// Background goroutines
	cleanupTicker    *time.Ticker
	metricsTicker    *time.Ticker
	healthTicker     *time.Ticker
	monitoringTicker *time.Ticker
	workerGroup      sync.WaitGroup
	
	// Connection semaphore for global limiting
	connSemaphore *semaphore.Weighted
	
	// Connection reuse pool to prevent memory leaks
	connPool sync.Pool
}

// HTTPPoolConfig contains configuration options for the connection pool.
type HTTPPoolConfig struct {
	// Pool sizing
	MaxConnectionsPerServer int           `json:"max_connections_per_server" default:"100"`
	MaxTotalConnections     int           `json:"max_total_connections" default:"1000"`
	MaxIdleConnections      int           `json:"max_idle_connections" default:"50"`
	MaxIdleTime             time.Duration `json:"max_idle_time" default:"5m"`
	
	// Connection timeouts
	ConnectTimeout    time.Duration `json:"connect_timeout" default:"10s"`
	RequestTimeout    time.Duration `json:"request_timeout" default:"30s"`
	KeepAliveTimeout  time.Duration `json:"keep_alive_timeout" default:"30s"`
	IdleConnTimeout   time.Duration `json:"idle_conn_timeout" default:"90s"`
	
	// Health checking
	HealthCheckInterval   time.Duration `json:"health_check_interval" default:"30s"`
	HealthCheckTimeout    time.Duration `json:"health_check_timeout" default:"5s"`
	HealthCheckPath       string        `json:"health_check_path" default:"/health"`
	UnhealthyThreshold    int           `json:"unhealthy_threshold" default:"3"`
	HealthyThreshold      int           `json:"healthy_threshold" default:"2"`
	
	// Load balancing
	LoadBalanceStrategy LoadBalanceStrategy `json:"load_balance_strategy" default:"round_robin"`
	
	// Cleanup and monitoring
	CleanupInterval   time.Duration `json:"cleanup_interval" default:"1m"`
	MetricsInterval   time.Duration `json:"metrics_interval" default:"10s"`
	MonitoringInterval time.Duration `json:"monitoring_interval" default:"30s"`
	
	// TLS configuration
	TLSConfig *tls.Config `json:"-"`
	
	// Custom transport options
	DisableKeepAlives     bool          `json:"disable_keep_alives"`
	DisableCompression    bool          `json:"disable_compression"`
	MaxResponseHeaderSize int64         `json:"max_response_header_size" default:"1048576"`
	WriteBufferSize       int           `json:"write_buffer_size" default:"4096"`
	ReadBufferSize        int           `json:"read_buffer_size" default:"4096"`
	
	// Bounded pool configuration to prevent memory leaks
	BoundedPool BoundedPoolConfig `json:"bounded_pool"`
}

// BoundedPoolConfig configures the bounded connection pool behavior
type BoundedPoolConfig struct {
	// MaxServerPools is the maximum number of server pools to maintain (default: 1000)
	MaxServerPools int `json:"max_server_pools"`
	
	// ServerPoolTTL is the time-to-live for unused server pools (default: 30 minutes)
	ServerPoolTTL time.Duration `json:"server_pool_ttl"`
	
	// PoolCleanupInterval is how often to run pool cleanup (default: 5 minutes)
	PoolCleanupInterval time.Duration `json:"pool_cleanup_interval"`
	
	// EnablePoolMetrics enables server pool metrics collection (default: true)
	EnablePoolMetrics bool `json:"enable_pool_metrics"`
}

// LoadBalanceStrategy defines the load balancing algorithm to use.
type LoadBalanceStrategy string

const (
	RoundRobin    LoadBalanceStrategy = "round_robin"
	LeastConn     LoadBalanceStrategy = "least_conn"
	WeightedRound LoadBalanceStrategy = "weighted_round"
	Random        LoadBalanceStrategy = "random"
	IPHash        LoadBalanceStrategy = "ip_hash"
)

// ServerTarget represents a target server for load balancing.
type ServerTarget struct {
	URL             *url.URL      `json:"url"`
	Weight          int           `json:"weight" default:"1"`
	MaxConnections  int           `json:"max_connections" default:"100"`
	CurrentConnections int64      `json:"current_connections"`
	IsHealthy       bool          `json:"is_healthy"`
	LastHealthCheck time.Time     `json:"last_health_check"`
	FailureCount    int           `json:"failure_count"`
	TotalRequests   int64         `json:"total_requests"`
	ResponseTime    time.Duration `json:"response_time"`
	mu              sync.RWMutex
}

// serverPool manages connections for a specific server.
type serverPool struct {
	server       *ServerTarget
	transport    *http.Transport
	client       *http.Client
	connections  chan *pooledConnection
	activeConns  int64
	totalConns   int64
	maxConns     int64
	mu           sync.RWMutex
	created      time.Time
	
	// Cleanup management
	cleanupCtx    context.Context
	cleanupCancel context.CancelFunc
}

// pooledConnection wraps an HTTP connection with metadata.
type pooledConnection struct {
	transport   *http.Transport
	client      *http.Client
	server      *ServerTarget
	created     time.Time
	lastUsed    time.Time
	usageCount  int64
	isHealthy   bool
	mu          sync.RWMutex
}

// HTTPPoolMetrics contains comprehensive metrics for the connection pool.
type HTTPPoolMetrics struct {
	// Connection metrics
	TotalConnections     int64 `json:"total_connections"`
	ActiveConnections    int64 `json:"active_connections"`
	IdleConnections      int64 `json:"idle_connections"`
	ConnectionsCreated   int64 `json:"connections_created"`
	ConnectionsDestroyed int64 `json:"connections_destroyed"`
	ConnectionsReused    int64 `json:"connections_reused"`
	
	// Request metrics
	TotalRequests        int64         `json:"total_requests"`
	SuccessfulRequests   int64         `json:"successful_requests"`
	FailedRequests       int64         `json:"failed_requests"`
	AverageResponseTime  time.Duration `json:"average_response_time"`
	
	// Health metrics
	HealthyServers       int           `json:"healthy_servers"`
	UnhealthyServers     int           `json:"unhealthy_servers"`
	HealthChecksFailed   int64         `json:"health_checks_failed"`
	HealthChecksSuccess  int64         `json:"health_checks_success"`
	
	// Performance metrics
	PoolUtilization      float64       `json:"pool_utilization"`
	AverageWaitTime      time.Duration `json:"average_wait_time"`
	MaxWaitTime          time.Duration `json:"max_wait_time"`
	
	// Load balancing metrics
	RequestsPerServer    map[string]int64 `json:"requests_per_server"`
	
	// Error tracking
	ConnectionErrors     int64         `json:"connection_errors"`
	TimeoutErrors        int64         `json:"timeout_errors"`
	
	// Resource usage
	MemoryUsage          int64         `json:"memory_usage"`
	
	// Timing
	LastUpdated          time.Time     `json:"last_updated"`
	StartTime            time.Time     `json:"start_time"`
}

// HealthChecker manages health checking for servers.
type HealthChecker struct {
	config    *HTTPPoolConfig
	pool      *HTTPConnectionPool
	transport *http.Transport
	client    *http.Client
}

// ConnectionRequest represents a request for a connection.
type ConnectionRequest struct {
	Context     context.Context
	ServerHint  string // Hint for server selection
	ClientID    string // For IP hash load balancing
	Priority    int    // Request priority
}

// ConnectionResponse contains the result of a connection request.
type ConnectionResponse struct {
	Connection *pooledConnection
	Server     *ServerTarget
	WaitTime   time.Duration
	FromPool   bool
}

// NewHTTPConnectionPool creates a new connection pool with the specified configuration.
func NewHTTPConnectionPool(config *HTTPPoolConfig) (*HTTPConnectionPool, error) {
	if config == nil {
		config = DefaultHTTPPoolConfig()
	}
	
	// Validate configuration
	if err := validateHTTPPoolConfig(config); err != nil {
		return nil, errors.NewAgentError(
			errors.ErrorTypeValidation,
			"invalid pool configuration",
			"ConnectionPool",
		).WithCause(err).WithDetail("operation", "NewHTTPConnectionPool")
	}
	
	// Apply defaults
	config = mergeWithDefaults(config)
	
	// Set bounded pool defaults
	if config.BoundedPool.MaxServerPools == 0 {
		config.BoundedPool.MaxServerPools = 1000
	}
	if config.BoundedPool.ServerPoolTTL == 0 {
		config.BoundedPool.ServerPoolTTL = 30 * time.Minute
	}
	if config.BoundedPool.PoolCleanupInterval == 0 {
		config.BoundedPool.PoolCleanupInterval = 5 * time.Minute
	}
	config.BoundedPool.EnablePoolMetrics = true // Always enable metrics

	// Create bounded map for server pools
	boundedMapConfig := internal.BoundedMapConfig{
		MaxSize:         config.BoundedPool.MaxServerPools,
		TTL:             config.BoundedPool.ServerPoolTTL,
		CleanupInterval: config.BoundedPool.PoolCleanupInterval,
		EnableMetrics:   config.BoundedPool.EnablePoolMetrics,
		MetricsPrefix:   "http_connection_pool_servers",
		EvictionCallback: func(key, value interface{}, reason internal.EvictionReason) {
			// Clean up server pool when evicted
			if pool, ok := value.(*serverPool); ok {
				logrus.WithFields(logrus.Fields{
					"server_url": key,
					"reason": reason.String(),
				}).Debug("Server pool evicted from bounded map")
				// Cancel cleanup worker and close connections
				if pool.cleanupCancel != nil {
					pool.cleanupCancel()
				}
				go func() {
					// Close all connections in pool
					select {
					case <-pool.connections:
						// Drain and close connections
						for {
							select {
							case conn := <-pool.connections:
								if conn != nil {
									// Connection cleanup would go here
								}
							default:
								return
							}
						}
					default:
						// No connections to drain
					}
					// Close transport
					if pool.transport != nil {
						pool.transport.CloseIdleConnections()
					}
				}()
			}
		},
	}

	pool := &HTTPConnectionPool{
		config:        config,
		pools:         internal.NewBoundedMap(boundedMapConfig),
		servers:       make([]*ServerTarget, 0),
		metrics:       newHTTPPoolMetrics(),
		shutdown:      make(chan struct{}),
		connSemaphore: semaphore.NewWeighted(int64(config.MaxTotalConnections)),
	}
	
	// Initialize connection pool for reuse
	pool.connPool.New = func() interface{} {
		return &pooledConnection{
			created:    time.Now(),
			lastUsed:   time.Now(),
			usageCount: 0,
			isHealthy:  true,
		}
	}
	
	// Initialize health checker
	pool.healthChecker = &HealthChecker{
		config: config,
		pool:   pool,
		transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   config.HealthCheckTimeout,
				KeepAlive: 0, // Disable keep-alive for health checks
			}).DialContext,
			MaxIdleConns:        1,
			IdleConnTimeout:     config.HealthCheckTimeout,
			DisableKeepAlives:   true,
			DisableCompression:  true,
		},
	}
	pool.healthChecker.client = &http.Client{
		Transport: pool.healthChecker.transport,
		Timeout:   config.HealthCheckTimeout,
	}
	
	// Start background workers
	pool.startBackgroundWorkers()
	
	return pool, nil
}

// AddServer adds a server target to the pool.
func (p *HTTPConnectionPool) AddServer(serverURL string, weight int) error {
	if atomic.LoadInt32(&p.isShutdown) == 1 {
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"connection pool is shutdown",
			"ConnectionPool",
		).WithDetail("operation", "AddServer")
	}
	
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		validationErr := errors.NewValidationError("invalid_server_url", "invalid server URL").
			WithField("serverURL", serverURL)
		return validationErr.WithCause(err)
	}
	
	if weight <= 0 {
		weight = 1
	}
	
	server := &ServerTarget{
		URL:                parsedURL,
		Weight:             weight,
		MaxConnections:     p.config.MaxConnectionsPerServer,
		IsHealthy:          true, // Start as healthy
		LastHealthCheck:    time.Now(),
		FailureCount:       0,
		TotalRequests:      0,
	}
	
	// Create server pool
	pool := p.createServerPool(server)
	
	p.poolsMu.Lock()
	p.pools.Set(serverURL, pool)
	p.poolsMu.Unlock()
	
	p.serversMu.Lock()
	p.servers = append(p.servers, server)
	p.serversMu.Unlock()
	
	// Trigger immediate health check
	go p.healthChecker.checkServerHealth(server)
	
	return nil
}

// RemoveServer removes a server target from the pool.
func (p *HTTPConnectionPool) RemoveServer(serverURL string) error {
	if atomic.LoadInt32(&p.isShutdown) == 1 {
		return errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"connection pool is shutdown",
			"ConnectionPool",
		).WithDetail("operation", "RemoveServer")
	}
	
	p.poolsMu.Lock()
	poolInterface, exists := p.pools.Get(serverURL)
	if exists {
		p.pools.Delete(serverURL)
	}
	p.poolsMu.Unlock()
	
	if !exists {
		return errors.NewResourceNotFoundError("server", serverURL)
	}
	
	// Close server pool
	if pool, ok := poolInterface.(*serverPool); ok {
		p.closeServerPool(pool)
	}
	
	// Remove from servers list
	p.serversMu.Lock()
	for i, server := range p.servers {
		if server.URL.String() == serverURL {
			p.servers = append(p.servers[:i], p.servers[i+1:]...)
			break
		}
	}
	p.serversMu.Unlock()
	
	return nil
}

// GetConnection acquires a connection from the pool using load balancing.
func (p *HTTPConnectionPool) GetConnection(req *ConnectionRequest) (*ConnectionResponse, error) {
	if atomic.LoadInt32(&p.isShutdown) == 1 {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"connection pool is shutdown",
			"ConnectionPool",
		).WithDetail("operation", "GetConnection")
	}
	
	if req.Context == nil {
		req.Context = context.Background()
	}
	
	startTime := time.Now()
	
	// Select server using load balancing
	server, err := p.selectServer(req)
	if err != nil {
		atomic.AddInt64(&p.metrics.FailedRequests, 1)
		return nil, errors.NewAgentError(
			errors.ErrorTypeExternal,
			"server selection failed",
			"ConnectionPool",
		).WithCause(err).WithDetail("operation", "GetConnection")
	}
	
	// Acquire global connection semaphore
	if err := p.connSemaphore.Acquire(req.Context, 1); err != nil {
		atomic.AddInt64(&p.metrics.FailedRequests, 1)
		return nil, errors.NewAgentError(
			errors.ErrorTypeTimeout,
			"failed to acquire connection semaphore",
			"ConnectionPool",
		).WithCause(err).WithDetail("operation", "GetConnection")
	}
	
	// Get connection from server pool
	conn, fromPool, err := p.getConnectionFromServerPool(req.Context, server)
	if err != nil {
		p.connSemaphore.Release(1)
		atomic.AddInt64(&p.metrics.FailedRequests, 1)
		return nil, errors.NewAgentError(
			errors.ErrorTypeExternal,
			"failed to get connection",
			"ConnectionPool",
		).WithCause(err).WithDetail("operation", "GetConnection")
	}
	
	waitTime := time.Since(startTime)
	
	// Update metrics
	atomic.AddInt64(&p.metrics.TotalRequests, 1)
	if fromPool {
		atomic.AddInt64(&p.metrics.ConnectionsReused, 1)
	}
	
	p.updateWaitTimeMetrics(waitTime)
	
	// Update server metrics
	server.mu.Lock()
	atomic.AddInt64(&server.TotalRequests, 1)
	atomic.AddInt64(&server.CurrentConnections, 1)
	server.mu.Unlock()
	
	return &ConnectionResponse{
		Connection: conn,
		Server:     server,
		WaitTime:   waitTime,
		FromPool:   fromPool,
	}, nil
}

// ReleaseConnection returns a connection to the pool.
func (p *HTTPConnectionPool) ReleaseConnection(conn *pooledConnection) error {
	if atomic.LoadInt32(&p.isShutdown) == 1 {
		return p.destroyConnection(conn)
	}
	
	if conn == nil {
		return errors.NewValidationError("connection_nil", "connection is nil").
			WithField("connection", conn).WithDetail("operation", "ReleaseConnection")
	}
	
	conn.mu.Lock()
	conn.lastUsed = time.Now()
	atomic.AddInt64(&conn.usageCount, 1)
	conn.mu.Unlock()
	
	// Update server metrics
	if conn.server != nil {
		atomic.AddInt64(&conn.server.CurrentConnections, -1)
	}
	
	// Check if connection is still healthy and within limits
	if !conn.isHealthy || time.Since(conn.created) > p.config.MaxIdleTime {
		p.connSemaphore.Release(1)
		return p.destroyConnection(conn)
	}
	
	// Return to pool
	serverURL := conn.server.URL.String()
	p.poolsMu.RLock()
	poolInterface, exists := p.pools.Get(serverURL)
	p.poolsMu.RUnlock()
	
	if !exists {
		p.connSemaphore.Release(1)
		return p.destroyConnection(conn)
	}
	
	pool, ok := poolInterface.(*serverPool)
	if !ok {
		p.connSemaphore.Release(1)
		return p.destroyConnection(conn)
	}
	
	// Try to return to pool
	select {
	case pool.connections <- conn:
		// Successfully returned to pool
		p.connSemaphore.Release(1)
		return nil
	default:
		// Pool is full, destroy connection
		p.connSemaphore.Release(1)
		return p.destroyConnection(conn)
	}
}

// GetMetrics returns current pool metrics.
func (p *HTTPConnectionPool) GetMetrics() *HTTPPoolMetrics {
	p.metricsMu.RLock()
	defer p.metricsMu.RUnlock()
	
	// Create a deep copy of metrics
	metrics := &HTTPPoolMetrics{
		TotalConnections:     atomic.LoadInt64(&p.metrics.TotalConnections),
		ActiveConnections:    atomic.LoadInt64(&p.metrics.ActiveConnections),
		IdleConnections:      atomic.LoadInt64(&p.metrics.IdleConnections),
		ConnectionsCreated:   atomic.LoadInt64(&p.metrics.ConnectionsCreated),
		ConnectionsDestroyed: atomic.LoadInt64(&p.metrics.ConnectionsDestroyed),
		ConnectionsReused:    atomic.LoadInt64(&p.metrics.ConnectionsReused),
		TotalRequests:        atomic.LoadInt64(&p.metrics.TotalRequests),
		SuccessfulRequests:   atomic.LoadInt64(&p.metrics.SuccessfulRequests),
		FailedRequests:       atomic.LoadInt64(&p.metrics.FailedRequests),
		AverageResponseTime:  p.metrics.AverageResponseTime,
		HealthyServers:       p.metrics.HealthyServers,
		UnhealthyServers:     p.metrics.UnhealthyServers,
		HealthChecksFailed:   atomic.LoadInt64(&p.metrics.HealthChecksFailed),
		HealthChecksSuccess:  atomic.LoadInt64(&p.metrics.HealthChecksSuccess),
		PoolUtilization:      p.metrics.PoolUtilization,
		AverageWaitTime:      p.metrics.AverageWaitTime,
		MaxWaitTime:          p.metrics.MaxWaitTime,
		RequestsPerServer:    make(map[string]int64),
		ConnectionErrors:     atomic.LoadInt64(&p.metrics.ConnectionErrors),
		TimeoutErrors:        atomic.LoadInt64(&p.metrics.TimeoutErrors),
		MemoryUsage:          p.metrics.MemoryUsage,
		LastUpdated:          p.metrics.LastUpdated,
		StartTime:            p.metrics.StartTime,
	}
	
	// Copy requests per server
	for k, v := range p.metrics.RequestsPerServer {
		metrics.RequestsPerServer[k] = v
	}
	
	return metrics
}

// GetServerStats returns statistics for all servers.
func (p *HTTPConnectionPool) GetServerStats() []*ServerTarget {
	p.serversMu.RLock()
	defer p.serversMu.RUnlock()
	
	stats := make([]*ServerTarget, len(p.servers))
	for i, server := range p.servers {
		server.mu.RLock()
		stats[i] = &ServerTarget{
			URL:                server.URL,
			Weight:             server.Weight,
			MaxConnections:     server.MaxConnections,
			CurrentConnections: atomic.LoadInt64(&server.CurrentConnections),
			IsHealthy:          server.IsHealthy,
			LastHealthCheck:    server.LastHealthCheck,
			FailureCount:       server.FailureCount,
			TotalRequests:      atomic.LoadInt64(&server.TotalRequests),
			ResponseTime:       server.ResponseTime,
		}
		server.mu.RUnlock()
	}
	
	return stats
}

// Shutdown gracefully shuts down the connection pool.
func (p *HTTPConnectionPool) Shutdown(ctx context.Context) error {
	p.shutdownOnce.Do(func() {
		atomic.StoreInt32(&p.isShutdown, 1)
		close(p.shutdown)
	})
	
	// Stop background workers
	if p.cleanupTicker != nil {
		p.cleanupTicker.Stop()
	}
	if p.metricsTicker != nil {
		p.metricsTicker.Stop()
	}
	if p.healthTicker != nil {
		p.healthTicker.Stop()
	}
	if p.monitoringTicker != nil {
		p.monitoringTicker.Stop()
	}
	
	// Wait for background workers to finish
	done := make(chan struct{})
	go func() {
		p.workerGroup.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Workers finished gracefully
	case <-ctx.Done():
		// Context timeout, force shutdown
	}
	
	// Close all server pools
	p.poolsMu.Lock()
	for _, keyInterface := range p.pools.Keys() {
		if poolInterface, exists := p.pools.Get(keyInterface); exists {
			if pool, ok := poolInterface.(*serverPool); ok {
				p.closeServerPool(pool)
			}
		}
	}
	// Close bounded map
	p.pools.Close()
	p.poolsMu.Unlock()
	
	// Close health checker
	if p.healthChecker.transport != nil {
		p.healthChecker.transport.CloseIdleConnections()
	}
	
	return nil
}

// Helper methods

func (p *HTTPConnectionPool) createServerPool(server *ServerTarget) *serverPool {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   p.config.ConnectTimeout,
			KeepAlive: p.config.KeepAliveTimeout,
		}).DialContext,
		MaxIdleConns:          p.config.MaxIdleConnections,
		MaxIdleConnsPerHost:   p.config.MaxConnectionsPerServer,
		IdleConnTimeout:       p.config.IdleConnTimeout,
		TLSHandshakeTimeout:   p.config.ConnectTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     p.config.DisableKeepAlives,
		DisableCompression:    p.config.DisableCompression,
		MaxResponseHeaderBytes: p.config.MaxResponseHeaderSize,
		WriteBufferSize:       p.config.WriteBufferSize,
		ReadBufferSize:        p.config.ReadBufferSize,
		TLSClientConfig:       p.config.TLSConfig,
	}
	
	client := &http.Client{
		Transport: transport,
		Timeout:   p.config.RequestTimeout,
	}
	
	pool := &serverPool{
		server:      server,
		transport:   transport,
		client:      client,
		connections: make(chan *pooledConnection, p.config.MaxIdleConnections),
		maxConns:    int64(server.MaxConnections),
		created:     time.Now(),
	}
	
	// Initialize connection cleanup with timeout
	pool.cleanupCtx, pool.cleanupCancel = context.WithCancel(context.Background())
	go pool.startCleanupWorker(p.config.MaxIdleTime)
	
	return pool
}

func (p *HTTPConnectionPool) closeServerPool(pool *serverPool) {
	// Cancel cleanup worker
	if pool.cleanupCancel != nil {
		pool.cleanupCancel()
	}
	
	// Close all connections in pool with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	done := make(chan struct{})
	go func() {
		defer close(done)
		close(pool.connections)
		for conn := range pool.connections {
			p.destroyConnectionUnsafe(conn, true) // already have pools lock
		}
	}()
	
	select {
	case <-done:
		// Cleanup completed successfully
	case <-ctx.Done():
		// Timeout reached, force cleanup
	}
	
	// Close transport
	if pool.transport != nil {
		pool.transport.CloseIdleConnections()
	}
}

func (p *HTTPConnectionPool) getConnectionFromServerPool(ctx context.Context, server *ServerTarget) (*pooledConnection, bool, error) {
	serverURL := server.URL.String()
	
	p.poolsMu.RLock()
	poolInterface, exists := p.pools.Get(serverURL)
	p.poolsMu.RUnlock()
	
	if !exists {
		return nil, false, errors.NewResourceNotFoundError("server_pool", serverURL)
	}
	
	pool, ok := poolInterface.(*serverPool)
	if !ok {
		return nil, false, errors.NewInternalErrorWithComponent("ConnectionPool", "invalid server pool type", nil)
	}
	
	// Try to get existing connection from pool
	select {
	case conn := <-pool.connections:
		// Check if connection is still valid
		if p.isConnectionValid(conn) {
			atomic.AddInt64(&pool.activeConns, 1)
			return conn, true, nil
		}
		// Connection is invalid, destroy it and create new one
		p.destroyConnection(conn)
	default:
		// No connections available in pool
	}
	
	// Check connection limits
	if atomic.LoadInt64(&pool.activeConns) >= pool.maxConns {
		return nil, false, errors.NewOperationError("getConnectionFromServerPool", "ConnectionPool", 
			fmt.Errorf("server connection limit reached for %s", serverURL))
	}
	
	// Create new connection using sync.Pool for reuse
	conn := p.connPool.Get().(*pooledConnection)
	conn.transport = pool.transport
	conn.client = pool.client
	conn.server = server
	conn.created = time.Now()
	conn.lastUsed = time.Now()
	atomic.StoreInt64(&conn.usageCount, 0)
	conn.isHealthy = true
	
	atomic.AddInt64(&pool.activeConns, 1)
	atomic.AddInt64(&pool.totalConns, 1)
	atomic.AddInt64(&p.metrics.ConnectionsCreated, 1)
	
	return conn, false, nil
}

func (p *HTTPConnectionPool) destroyConnection(conn *pooledConnection) error {
	return p.destroyConnectionUnsafe(conn, false)
}

func (p *HTTPConnectionPool) destroyConnectionUnsafe(conn *pooledConnection, alreadyLocked bool) error {
	if conn == nil {
		return nil
	}
	
	// Update pool metrics
	if conn.server != nil && !alreadyLocked {
		serverURL := conn.server.URL.String()
		p.poolsMu.RLock()
		if poolInterface, exists := p.pools.Get(serverURL); exists {
			if pool, ok := poolInterface.(*serverPool); ok {
				atomic.AddInt64(&pool.activeConns, -1)
			}
		}
		p.poolsMu.RUnlock()
	}
	
	atomic.AddInt64(&p.metrics.ConnectionsDestroyed, 1)
	
	// Return connection to sync.Pool for reuse
	if conn != nil {
		// Reset connection state
		conn.transport = nil
		conn.client = nil
		conn.server = nil
		conn.isHealthy = false
		atomic.StoreInt64(&conn.usageCount, 0)
		p.connPool.Put(conn)
	}
	
	return nil
}

func (p *HTTPConnectionPool) isConnectionValid(conn *pooledConnection) bool {
	if conn == nil {
		return false
	}
	
	conn.mu.RLock()
	defer conn.mu.RUnlock()
	
	// Check if connection is healthy
	if !conn.isHealthy {
		return false
	}
	
	// Check if connection has exceeded max idle time
	if time.Since(conn.lastUsed) > p.config.MaxIdleTime {
		return false
	}
	
	// Check if server is healthy
	if conn.server != nil && !conn.server.IsHealthy {
		return false
	}
	
	return true
}

func (p *HTTPConnectionPool) selectServer(req *ConnectionRequest) (*ServerTarget, error) {
	p.serversMu.RLock()
	servers := make([]*ServerTarget, len(p.servers))
	copy(servers, p.servers)
	p.serversMu.RUnlock()
	
	if len(servers) == 0 {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"no servers available",
			"ConnectionPool",
		).WithDetail("operation", "selectServer")
	}
	
	// Filter healthy servers
	healthyServers := make([]*ServerTarget, 0, len(servers))
	for _, server := range servers {
		server.mu.RLock()
		if server.IsHealthy {
			healthyServers = append(healthyServers, server)
		}
		server.mu.RUnlock()
	}
	
	if len(healthyServers) == 0 {
		return nil, errors.NewAgentError(
			errors.ErrorTypeInvalidState,
			"no healthy servers available",
			"ConnectionPool",
		).WithDetail("operation", "selectServer")
	}
	
	// Apply load balancing strategy
	switch p.config.LoadBalanceStrategy {
	case RoundRobin:
		return p.selectRoundRobin(healthyServers), nil
	case LeastConn:
		return p.selectLeastConnections(healthyServers), nil
	case WeightedRound:
		return p.selectWeightedRoundRobin(healthyServers), nil
	case Random:
		return p.selectRandom(healthyServers), nil
	case IPHash:
		return p.selectIPHash(healthyServers, req.ClientID), nil
	default:
		return p.selectRoundRobin(healthyServers), nil
	}
}

func (p *HTTPConnectionPool) selectRoundRobin(servers []*ServerTarget) *ServerTarget {
	if len(servers) == 0 {
		return nil
	}
	
	// Simple round-robin using current time as seed
	index := int(time.Now().UnixNano()) % len(servers)
	return servers[index]
}

func (p *HTTPConnectionPool) selectLeastConnections(servers []*ServerTarget) *ServerTarget {
	if len(servers) == 0 {
		return nil
	}
	
	var selected *ServerTarget
	minConnections := int64(^uint64(0) >> 1) // Max int64
	
	for _, server := range servers {
		connCount := atomic.LoadInt64(&server.CurrentConnections)
		if connCount < minConnections {
			minConnections = connCount
			selected = server
		}
	}
	
	return selected
}

func (p *HTTPConnectionPool) selectWeightedRoundRobin(servers []*ServerTarget) *ServerTarget {
	if len(servers) == 0 {
		return nil
	}
	
	// Calculate total weight
	totalWeight := 0
	for _, server := range servers {
		totalWeight += server.Weight
	}
	
	if totalWeight == 0 {
		return p.selectRoundRobin(servers)
	}
	
	// Select based on weight
	target := rand.Intn(totalWeight)
	current := 0
	
	for _, server := range servers {
		current += server.Weight
		if current > target {
			return server
		}
	}
	
	return servers[len(servers)-1]
}

func (p *HTTPConnectionPool) selectRandom(servers []*ServerTarget) *ServerTarget {
	if len(servers) == 0 {
		return nil
	}
	
	index := rand.Intn(len(servers))
	return servers[index]
}

func (p *HTTPConnectionPool) selectIPHash(servers []*ServerTarget, clientID string) *ServerTarget {
	if len(servers) == 0 {
		return nil
	}
	
	if clientID == "" {
		return p.selectRoundRobin(servers)
	}
	
	// Simple hash of client ID
	hash := 0
	for _, c := range clientID {
		hash = hash*31 + int(c)
	}
	
	index := hash % len(servers)
	if index < 0 {
		index = -index
	}
	
	return servers[index]
}

func (p *HTTPConnectionPool) startBackgroundWorkers() {
	// Cleanup worker
	p.cleanupTicker = time.NewTicker(p.config.CleanupInterval)
	p.workerGroup.Add(1)
	go func() {
		defer p.workerGroup.Done()
		for {
			select {
			case <-p.shutdown:
				return
			case <-p.cleanupTicker.C:
				p.performCleanup()
			}
		}
	}()
	
	// Metrics worker
	p.metricsTicker = time.NewTicker(p.config.MetricsInterval)
	p.workerGroup.Add(1)
	go func() {
		defer p.workerGroup.Done()
		for {
			select {
			case <-p.shutdown:
				return
			case <-p.metricsTicker.C:
				p.updateMetrics()
			}
		}
	}()
	
	// Health check worker
	p.healthTicker = time.NewTicker(p.config.HealthCheckInterval)
	p.workerGroup.Add(1)
	go func() {
		defer p.workerGroup.Done()
		for {
			select {
			case <-p.shutdown:
				return
			case <-p.healthTicker.C:
				p.performHealthChecks()
			}
		}
	}()
	
	// Start monitoring worker
	p.startMonitoring()
}

func (p *HTTPConnectionPool) performCleanup() {
	p.poolsMu.RLock()
	pools := make([]*serverPool, 0)
	for _, keyInterface := range p.pools.Keys() {
		if poolInterface, exists := p.pools.Get(keyInterface); exists {
			if pool, ok := poolInterface.(*serverPool); ok {
				pools = append(pools, pool)
			}
		}
	}
	p.poolsMu.RUnlock()
	
	for _, pool := range pools {
		p.cleanupServerPool(pool)
	}
}

func (p *HTTPConnectionPool) cleanupServerPool(pool *serverPool) {
	// Clean up idle connections that have exceeded max idle time
	select {
	case conn := <-pool.connections:
		if !p.isConnectionValid(conn) {
			p.destroyConnection(conn)
		} else {
			// Put back valid connection
			select {
			case pool.connections <- conn:
			default:
				// Pool is full, destroy connection
				p.destroyConnection(conn)
			}
		}
	default:
		// No connections to clean up
	}
}

// startCleanupWorker starts a background worker for connection cleanup with timeout
func (sp *serverPool) startCleanupWorker(maxIdleTime time.Duration) {
	ticker := time.NewTicker(maxIdleTime / 2) // Check twice as often as max idle time
	defer ticker.Stop()
	
	for {
		select {
		case <-sp.cleanupCtx.Done():
			return
		case <-ticker.C:
			sp.cleanupIdleConnections(maxIdleTime)
		}
	}
}

// cleanupIdleConnections removes idle connections that have exceeded max idle time
func (sp *serverPool) cleanupIdleConnections(maxIdleTime time.Duration) {
	// Use timeout to prevent blocking indefinitely
	timeout := time.NewTimer(100 * time.Millisecond)
	defer timeout.Stop()
	
	for {
		select {
		case conn := <-sp.connections:
			if conn != nil {
				conn.mu.RLock()
				isExpired := time.Since(conn.lastUsed) > maxIdleTime
				conn.mu.RUnlock()
				
				if isExpired {
					// Connection expired, destroy it
					atomic.AddInt64(&sp.activeConns, -1)
				} else {
					// Connection still valid, try to put it back
					select {
					case sp.connections <- conn:
						// Successfully returned to pool
					default:
						// Pool is full, destroy connection anyway
						atomic.AddInt64(&sp.activeConns, -1)
					}
				}
			}
		case <-timeout.C:
			return // Exit cleanup cycle after timeout
		default:
			return // No more connections to check
		}
	}
}

// UpdateMetrics manually updates the pool metrics. Useful for testing.
func (p *HTTPConnectionPool) UpdateMetrics() {
	p.updateMetrics()
}

func (p *HTTPConnectionPool) updateMetrics() {
	p.metricsMu.Lock()
	defer p.metricsMu.Unlock()
	
	// Update server counts
	healthyCount := 0
	unhealthyCount := 0
	
	p.serversMu.RLock()
	for _, server := range p.servers {
		server.mu.RLock()
		if server.IsHealthy {
			healthyCount++
		} else {
			unhealthyCount++
		}
		server.mu.RUnlock()
	}
	p.serversMu.RUnlock()
	
	p.metrics.HealthyServers = healthyCount
	p.metrics.UnhealthyServers = unhealthyCount
	
	// Calculate pool utilization
	totalConnections := atomic.LoadInt64(&p.metrics.TotalConnections)
	if p.config.MaxTotalConnections > 0 {
		p.metrics.PoolUtilization = float64(totalConnections) / float64(p.config.MaxTotalConnections)
	}
	
	p.metrics.LastUpdated = time.Now()
}

func (p *HTTPConnectionPool) updateWaitTimeMetrics(waitTime time.Duration) {
	p.metricsMu.Lock()
	defer p.metricsMu.Unlock()
	
	// Update average wait time using exponential moving average
	const alpha = 0.1
	if p.metrics.AverageWaitTime == 0 {
		p.metrics.AverageWaitTime = waitTime
	} else {
		p.metrics.AverageWaitTime = time.Duration(
			alpha*float64(waitTime) + (1-alpha)*float64(p.metrics.AverageWaitTime),
		)
	}
	
	// Update max wait time
	if waitTime > p.metrics.MaxWaitTime {
		p.metrics.MaxWaitTime = waitTime
	}
}

func (p *HTTPConnectionPool) performHealthChecks() {
	p.serversMu.RLock()
	servers := make([]*ServerTarget, len(p.servers))
	copy(servers, p.servers)
	p.serversMu.RUnlock()
	
	for _, server := range servers {
		go p.healthChecker.checkServerHealth(server)
	}
}

// Health checker methods

func (h *HealthChecker) checkServerHealth(server *ServerTarget) {
	startTime := time.Now()
	
	// Build health check URL
	healthURL := *server.URL
	healthURL.Path = h.config.HealthCheckPath
	
	// Create health check request
	req, err := http.NewRequest("GET", healthURL.String(), nil)
	if err != nil {
		err = errors.NewAgentError(
			errors.ErrorTypeValidation,
			"failed to create health check request",
			"HealthChecker",
		).WithCause(err).WithDetail("operation", "checkServerHealth")
		h.markServerUnhealthy(server, err)
		return
	}
	
	// Set timeout context
	ctx, cancel := context.WithTimeout(context.Background(), h.config.HealthCheckTimeout)
	defer cancel()
	req = req.WithContext(ctx)
	
	// Perform health check
	resp, err := h.client.Do(req)
	if err != nil {
		err = errors.NewAgentError(
			errors.ErrorTypeExternal,
			"health check request failed",
			"HealthChecker",
		).WithCause(err).WithDetail("operation", "checkServerHealth")
		h.markServerUnhealthy(server, err)
		return
	}
	defer resp.Body.Close()
	
	responseTime := time.Since(startTime)
	
	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = errors.NewAgentError(
			errors.ErrorTypeExternal,
			"health check returned non-success status",
			"HealthChecker",
		).WithDetail("status_code", resp.StatusCode).WithDetail("operation", "checkServerHealth")
		h.markServerUnhealthy(server, err)
		return
	}
	
	// Health check passed
	h.markServerHealthy(server, responseTime)
}

func (h *HealthChecker) markServerHealthy(server *ServerTarget, responseTime time.Duration) {
	server.mu.Lock()
	defer server.mu.Unlock()
	
	wasUnhealthy := !server.IsHealthy
	
	server.FailureCount = 0
	server.LastHealthCheck = time.Now()
	server.ResponseTime = responseTime
	
	// Mark as healthy if it meets the threshold
	if wasUnhealthy {
		// For now, mark as healthy immediately
		// In production, you might want to require multiple successful checks
		server.IsHealthy = true
	} else {
		server.IsHealthy = true
	}
	
	atomic.AddInt64(&h.pool.metrics.HealthChecksSuccess, 1)
}

func (h *HealthChecker) markServerUnhealthy(server *ServerTarget, err error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	
	server.FailureCount++
	server.LastHealthCheck = time.Now()
	
	// Mark as unhealthy if it exceeds the threshold
	if server.FailureCount >= h.config.UnhealthyThreshold {
		server.IsHealthy = false
	}
	
	atomic.AddInt64(&h.pool.metrics.HealthChecksFailed, 1)
}

// Utility functions

func newHTTPPoolMetrics() *HTTPPoolMetrics {
	return &HTTPPoolMetrics{
		RequestsPerServer: make(map[string]int64),
		StartTime:         time.Now(),
		LastUpdated:       time.Now(),
	}
}

func DefaultHTTPPoolConfig() *HTTPPoolConfig {
	return &HTTPPoolConfig{
		MaxConnectionsPerServer: 100,
		MaxTotalConnections:     1000,
		MaxIdleConnections:      50,
		MaxIdleTime:             5 * time.Minute,
		ConnectTimeout:          10 * time.Second,
		RequestTimeout:          30 * time.Second,
		KeepAliveTimeout:        30 * time.Second,
		IdleConnTimeout:         90 * time.Second,
		HealthCheckInterval:     30 * time.Second,
		HealthCheckTimeout:      5 * time.Second,
		HealthCheckPath:         "/health",
		UnhealthyThreshold:      3,
		HealthyThreshold:        2,
		LoadBalanceStrategy:     RoundRobin,
		CleanupInterval:         1 * time.Minute,
		MetricsInterval:         10 * time.Second,
		MonitoringInterval:      30 * time.Second,
		DisableKeepAlives:       false,
		DisableCompression:      false,
		MaxResponseHeaderSize:   1048576, // 1MB
		WriteBufferSize:         4096,
		ReadBufferSize:          4096,
		BoundedPool: BoundedPoolConfig{
			MaxServerPools:      1000,
			ServerPoolTTL:       30 * time.Minute,
			PoolCleanupInterval: 5 * time.Minute,
			EnablePoolMetrics:   true,
		},
	}
}

func validateHTTPPoolConfig(config *HTTPPoolConfig) error {
	if config.MaxConnectionsPerServer <= 0 {
		return errors.NewValidationError("max_connections_per_server_invalid", "MaxConnectionsPerServer must be positive").
			WithField("MaxConnectionsPerServer", config.MaxConnectionsPerServer)
	}
	if config.MaxTotalConnections <= 0 {
		return errors.NewValidationError("max_total_connections_invalid", "MaxTotalConnections must be positive").
			WithField("MaxTotalConnections", config.MaxTotalConnections)
	}
	if config.MaxIdleConnections < 0 {
		return errors.NewValidationError("max_idle_connections_invalid", "MaxIdleConnections cannot be negative").
			WithField("MaxIdleConnections", config.MaxIdleConnections)
	}
	if config.MaxIdleTime <= 0 {
		return errors.NewValidationError("max_idle_time_invalid", "MaxIdleTime must be positive").
			WithField("MaxIdleTime", config.MaxIdleTime.String())
	}
	if config.ConnectTimeout <= 0 {
		return errors.NewValidationError("connect_timeout_invalid", "ConnectTimeout must be positive").
			WithField("ConnectTimeout", config.ConnectTimeout.String())
	}
	if config.RequestTimeout <= 0 {
		return errors.NewValidationError("request_timeout_invalid", "RequestTimeout must be positive").
			WithField("RequestTimeout", config.RequestTimeout.String())
	}
	if config.HealthCheckInterval <= 0 {
		return errors.NewValidationError("health_check_interval_invalid", "HealthCheckInterval must be positive").
			WithField("HealthCheckInterval", config.HealthCheckInterval.String())
	}
	if config.HealthCheckTimeout <= 0 {
		return errors.NewValidationError("health_check_timeout_invalid", "HealthCheckTimeout must be positive").
			WithField("HealthCheckTimeout", config.HealthCheckTimeout.String())
	}
	if config.UnhealthyThreshold <= 0 {
		return errors.NewValidationError("unhealthy_threshold_invalid", "UnhealthyThreshold must be positive").
			WithField("UnhealthyThreshold", config.UnhealthyThreshold)
	}
	if config.HealthyThreshold <= 0 {
		return errors.NewValidationError("healthy_threshold_invalid", "HealthyThreshold must be positive").
			WithField("HealthyThreshold", config.HealthyThreshold)
	}
	return nil
}

func mergeWithDefaults(config *HTTPPoolConfig) *HTTPPoolConfig {
	defaults := DefaultHTTPPoolConfig()
	
	if config.MaxConnectionsPerServer == 0 {
		config.MaxConnectionsPerServer = defaults.MaxConnectionsPerServer
	}
	if config.MaxTotalConnections == 0 {
		config.MaxTotalConnections = defaults.MaxTotalConnections
	}
	if config.MaxIdleConnections == 0 {
		config.MaxIdleConnections = defaults.MaxIdleConnections
	}
	if config.MaxIdleTime == 0 {
		config.MaxIdleTime = defaults.MaxIdleTime
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = defaults.ConnectTimeout
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = defaults.RequestTimeout
	}
	if config.KeepAliveTimeout == 0 {
		config.KeepAliveTimeout = defaults.KeepAliveTimeout
	}
	if config.IdleConnTimeout == 0 {
		config.IdleConnTimeout = defaults.IdleConnTimeout
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = defaults.HealthCheckInterval
	}
	if config.HealthCheckTimeout == 0 {
		config.HealthCheckTimeout = defaults.HealthCheckTimeout
	}
	if config.HealthCheckPath == "" {
		config.HealthCheckPath = defaults.HealthCheckPath
	}
	if config.UnhealthyThreshold == 0 {
		config.UnhealthyThreshold = defaults.UnhealthyThreshold
	}
	if config.HealthyThreshold == 0 {
		config.HealthyThreshold = defaults.HealthyThreshold
	}
	if config.LoadBalanceStrategy == "" {
		config.LoadBalanceStrategy = defaults.LoadBalanceStrategy
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = defaults.CleanupInterval
	}
	if config.MetricsInterval == 0 {
		config.MetricsInterval = defaults.MetricsInterval
	}
	if config.MonitoringInterval == 0 {
		config.MonitoringInterval = defaults.MonitoringInterval
	}
	if config.MaxResponseHeaderSize == 0 {
		config.MaxResponseHeaderSize = defaults.MaxResponseHeaderSize
	}
	if config.WriteBufferSize == 0 {
		config.WriteBufferSize = defaults.WriteBufferSize
	}
	if config.ReadBufferSize == 0 {
		config.ReadBufferSize = defaults.ReadBufferSize
	}
	
	return config
}

// Monitoring and alerting methods

// startMonitoring starts the comprehensive monitoring goroutine
func (p *HTTPConnectionPool) startMonitoring() {
	logrus.WithFields(logrus.Fields{
		"monitoring_interval": p.config.MonitoringInterval,
		"component":          "ConnectionPool",
	}).Debug("Starting connection pool monitoring")

	p.monitoringTicker = time.NewTicker(p.config.MonitoringInterval)
	p.workerGroup.Add(1)
	go func() {
		defer p.workerGroup.Done()
		defer logrus.WithFields(logrus.Fields{
			"component": "ConnectionPool",
		}).Debug("Connection pool monitoring stopped")

		for {
			select {
			case <-p.shutdown:
				return
			case <-p.monitoringTicker.C:
				p.logPoolMetrics()
				p.checkPoolHealth()
			}
		}
	}()
}

// logPoolMetrics logs comprehensive pool metrics with structured fields
func (p *HTTPConnectionPool) logPoolMetrics() {
	poolCount := p.pools.Len()
	totalConnections := p.getTotalConnectionCount()
	utilizationPercentage := float64(0)
	
	if p.config.MaxTotalConnections > 0 {
		utilizationPercentage = (float64(totalConnections) / float64(p.config.MaxTotalConnections)) * 100
	}

	// Get server statistics
	p.serversMu.RLock()
	healthyServers := 0
	unhealthyServers := 0
	for _, server := range p.servers {
		server.mu.RLock()
		if server.IsHealthy {
			healthyServers++
		} else {
			unhealthyServers++
		}
		server.mu.RUnlock()
	}
	p.serversMu.RUnlock()

	// Log general pool metrics
	logrus.WithFields(logrus.Fields{
		"component":             "ConnectionPool",
		"pool_count":            poolCount,
		"total_connections":     totalConnections,
		"max_connections":       p.config.MaxTotalConnections,
		"utilization_percent":   utilizationPercentage,
		"healthy_servers":       healthyServers,
		"unhealthy_servers":     unhealthyServers,
		"total_requests":        atomic.LoadInt64(&p.metrics.TotalRequests),
		"successful_requests":   atomic.LoadInt64(&p.metrics.SuccessfulRequests),
		"failed_requests":       atomic.LoadInt64(&p.metrics.FailedRequests),
		"connections_created":   atomic.LoadInt64(&p.metrics.ConnectionsCreated),
		"connections_destroyed": atomic.LoadInt64(&p.metrics.ConnectionsDestroyed),
		"connections_reused":    atomic.LoadInt64(&p.metrics.ConnectionsReused),
	}).Info("Connection pool metrics")

	// Check for high utilization alert
	if utilizationPercentage >= 80.0 {
		logrus.WithFields(logrus.Fields{
			"component":           "ConnectionPool",
			"utilization_percent": utilizationPercentage,
			"total_connections":   totalConnections,
			"max_connections":     p.config.MaxTotalConnections,
			"alert":              "HIGH_UTILIZATION",
		}).Warn("Connection pool utilization exceeds 80% threshold")
	}
}

// checkPoolHealth identifies and removes unhealthy pools
func (p *HTTPConnectionPool) checkPoolHealth() {
	p.poolsMu.RLock()
	poolKeys := p.pools.Keys()
	p.poolsMu.RUnlock()

	unhealthyPools := make([]string, 0)

	for _, keyInterface := range poolKeys {
		key, ok := keyInterface.(string)
		if !ok {
			continue
		}

		p.poolsMu.RLock()
		poolInterface, exists := p.pools.Get(key)
		p.poolsMu.RUnlock()

		if !exists {
			continue
		}

		pool, ok := poolInterface.(*serverPool)
		if !ok {
			continue
		}

		// Check if pool is unhealthy (e.g., old, unused, or has errors)
		isUnhealthy := false
		
		// Check if pool is too old and unused
		if time.Since(pool.created) > p.config.BoundedPool.ServerPoolTTL {
			activeConns := atomic.LoadInt64(&pool.activeConns)
			if activeConns == 0 {
				isUnhealthy = true
				logrus.WithFields(logrus.Fields{
					"component": "ConnectionPool",
					"server_url": key,
					"reason": "pool_expired_and_unused",
					"age": time.Since(pool.created),
					"active_connections": activeConns,
				}).Debug("Identified unhealthy pool for cleanup")
			}
		}

		// Check if corresponding server is unhealthy
		p.serversMu.RLock()
		for _, server := range p.servers {
			if server.URL.String() == key && !server.IsHealthy {
				server.mu.RLock()
				failureCount := server.FailureCount
				lastHealthCheck := server.LastHealthCheck
				server.mu.RUnlock()
				
				// If server has been unhealthy for too long, mark pool as unhealthy
				if failureCount >= p.config.UnhealthyThreshold && 
				   time.Since(lastHealthCheck) > p.config.HealthCheckInterval*5 {
					isUnhealthy = true
					logrus.WithFields(logrus.Fields{
						"component": "ConnectionPool",
						"server_url": key,
						"reason": "server_unhealthy_too_long",
						"failure_count": failureCount,
						"last_health_check": lastHealthCheck,
					}).Debug("Identified unhealthy pool due to server health")
				}
				break
			}
		}
		p.serversMu.RUnlock()

		if isUnhealthy {
			unhealthyPools = append(unhealthyPools, key)
		}
	}

	// Clean up unhealthy pools
	for _, poolKey := range unhealthyPools {
		p.poolsMu.Lock()
		if poolInterface, exists := p.pools.Get(poolKey); exists {
			if pool, ok := poolInterface.(*serverPool); ok {
				p.closeServerPool(pool)
				p.pools.Delete(poolKey)
				
				logrus.WithFields(logrus.Fields{
					"component": "ConnectionPool",
					"server_url": poolKey,
				}).Info("Cleaned up unhealthy pool")
			}
		}
		p.poolsMu.Unlock()
	}
}

// getTotalConnectionCount returns the total number of active connections across all pools
func (p *HTTPConnectionPool) getTotalConnectionCount() int64 {
	var totalConnections int64

	p.poolsMu.RLock()
	poolKeys := p.pools.Keys()
	p.poolsMu.RUnlock()

	for _, keyInterface := range poolKeys {
		p.poolsMu.RLock()
		poolInterface, exists := p.pools.Get(keyInterface)
		p.poolsMu.RUnlock()

		if !exists {
			continue
		}

		if pool, ok := poolInterface.(*serverPool); ok {
			activeConns := atomic.LoadInt64(&pool.activeConns)
			totalConnections += activeConns
		}
	}

	return totalConnections
}