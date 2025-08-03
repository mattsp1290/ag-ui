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
)

// HTTPConnectionPool provides HTTP connection pooling with health monitoring,
// load balancing, and comprehensive metrics collection.
type HTTPConnectionPool struct {
	// Configuration
	config *HTTPPoolConfig
	
	// Connection management
	pools   map[string]*serverPool // server -> pool mapping
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
	cleanupTicker  *time.Ticker
	metricsTicker  *time.Ticker
	healthTicker   *time.Ticker
	workerGroup    sync.WaitGroup
	
	// Connection semaphore for global limiting
	connSemaphore *semaphore.Weighted
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
	CleanupInterval time.Duration `json:"cleanup_interval" default:"1m"`
	MetricsInterval time.Duration `json:"metrics_interval" default:"10s"`
	
	// TLS configuration
	TLSConfig *tls.Config `json:"-"`
	
	// Custom transport options
	DisableKeepAlives     bool          `json:"disable_keep_alives"`
	DisableCompression    bool          `json:"disable_compression"`
	MaxResponseHeaderSize int64         `json:"max_response_header_size" default:"1048576"`
	WriteBufferSize       int           `json:"write_buffer_size" default:"4096"`
	ReadBufferSize        int           `json:"read_buffer_size" default:"4096"`
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
		return nil, fmt.Errorf("invalid pool configuration: %w", err)
	}
	
	// Apply defaults
	config = mergeWithDefaults(config)
	
	pool := &HTTPConnectionPool{
		config:        config,
		pools:         make(map[string]*serverPool),
		servers:       make([]*ServerTarget, 0),
		metrics:       newHTTPPoolMetrics(),
		shutdown:      make(chan struct{}),
		connSemaphore: semaphore.NewWeighted(int64(config.MaxTotalConnections)),
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
		return fmt.Errorf("connection pool is shutdown")
	}
	
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
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
	p.pools[serverURL] = pool
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
		return fmt.Errorf("connection pool is shutdown")
	}
	
	p.poolsMu.Lock()
	pool, exists := p.pools[serverURL]
	delete(p.pools, serverURL)
	p.poolsMu.Unlock()
	
	if !exists {
		return fmt.Errorf("server not found: %s", serverURL)
	}
	
	// Close server pool
	p.closeServerPool(pool)
	
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
		return nil, fmt.Errorf("connection pool is shutdown")
	}
	
	if req.Context == nil {
		req.Context = context.Background()
	}
	
	startTime := time.Now()
	
	// Select server using load balancing
	server, err := p.selectServer(req)
	if err != nil {
		atomic.AddInt64(&p.metrics.FailedRequests, 1)
		return nil, fmt.Errorf("server selection failed: %w", err)
	}
	
	// Acquire global connection semaphore
	if err := p.connSemaphore.Acquire(req.Context, 1); err != nil {
		atomic.AddInt64(&p.metrics.FailedRequests, 1)
		return nil, fmt.Errorf("failed to acquire connection semaphore: %w", err)
	}
	
	// Get connection from server pool
	conn, fromPool, err := p.getConnectionFromServerPool(req.Context, server)
	if err != nil {
		p.connSemaphore.Release(1)
		atomic.AddInt64(&p.metrics.FailedRequests, 1)
		return nil, fmt.Errorf("failed to get connection: %w", err)
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
		return fmt.Errorf("connection is nil")
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
	pool, exists := p.pools[serverURL]
	p.poolsMu.RUnlock()
	
	if !exists {
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
	for _, pool := range p.pools {
		p.closeServerPool(pool)
	}
	p.pools = make(map[string]*serverPool)
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
	
	return &serverPool{
		server:      server,
		transport:   transport,
		client:      client,
		connections: make(chan *pooledConnection, p.config.MaxIdleConnections),
		maxConns:    int64(server.MaxConnections),
		created:     time.Now(),
	}
}

func (p *HTTPConnectionPool) closeServerPool(pool *serverPool) {
	// Close all connections in pool
	close(pool.connections)
	for conn := range pool.connections {
		p.destroyConnectionUnsafe(conn, true) // already have pools lock
	}
	
	// Close transport
	if pool.transport != nil {
		pool.transport.CloseIdleConnections()
	}
}

func (p *HTTPConnectionPool) getConnectionFromServerPool(ctx context.Context, server *ServerTarget) (*pooledConnection, bool, error) {
	serverURL := server.URL.String()
	
	p.poolsMu.RLock()
	pool, exists := p.pools[serverURL]
	p.poolsMu.RUnlock()
	
	if !exists {
		return nil, false, fmt.Errorf("server pool not found: %s", serverURL)
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
		return nil, false, fmt.Errorf("server connection limit reached: %s", serverURL)
	}
	
	// Create new connection
	conn := &pooledConnection{
		transport:  pool.transport,
		client:     pool.client,
		server:     server,
		created:    time.Now(),
		lastUsed:   time.Now(),
		usageCount: 0,
		isHealthy:  true,
	}
	
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
		if pool, exists := p.pools[serverURL]; exists {
			atomic.AddInt64(&pool.activeConns, -1)
		}
		p.poolsMu.RUnlock()
	}
	
	atomic.AddInt64(&p.metrics.ConnectionsDestroyed, 1)
	
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
		return nil, fmt.Errorf("no servers available")
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
		return nil, fmt.Errorf("no healthy servers available")
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
}

func (p *HTTPConnectionPool) performCleanup() {
	p.poolsMu.RLock()
	pools := make([]*serverPool, 0, len(p.pools))
	for _, pool := range p.pools {
		pools = append(pools, pool)
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
		h.markServerUnhealthy(server, fmt.Errorf("failed to create health check request: %w", err))
		return
	}
	
	// Set timeout context
	ctx, cancel := context.WithTimeout(context.Background(), h.config.HealthCheckTimeout)
	defer cancel()
	req = req.WithContext(ctx)
	
	// Perform health check
	resp, err := h.client.Do(req)
	if err != nil {
		h.markServerUnhealthy(server, fmt.Errorf("health check request failed: %w", err))
		return
	}
	defer resp.Body.Close()
	
	responseTime := time.Since(startTime)
	
	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		h.markServerUnhealthy(server, fmt.Errorf("health check returned status %d", resp.StatusCode))
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
		DisableKeepAlives:       false,
		DisableCompression:      false,
		MaxResponseHeaderSize:   1048576, // 1MB
		WriteBufferSize:         4096,
		ReadBufferSize:          4096,
	}
}

func validateHTTPPoolConfig(config *HTTPPoolConfig) error {
	if config.MaxConnectionsPerServer <= 0 {
		return fmt.Errorf("MaxConnectionsPerServer must be positive")
	}
	if config.MaxTotalConnections <= 0 {
		return fmt.Errorf("MaxTotalConnections must be positive")
	}
	if config.MaxIdleConnections < 0 {
		return fmt.Errorf("MaxIdleConnections cannot be negative")
	}
	if config.MaxIdleTime <= 0 {
		return fmt.Errorf("MaxIdleTime must be positive")
	}
	if config.ConnectTimeout <= 0 {
		return fmt.Errorf("ConnectTimeout must be positive")
	}
	if config.RequestTimeout <= 0 {
		return fmt.Errorf("RequestTimeout must be positive")
	}
	if config.HealthCheckInterval <= 0 {
		return fmt.Errorf("HealthCheckInterval must be positive")
	}
	if config.HealthCheckTimeout <= 0 {
		return fmt.Errorf("HealthCheckTimeout must be positive")
	}
	if config.UnhealthyThreshold <= 0 {
		return fmt.Errorf("UnhealthyThreshold must be positive")
	}
	if config.HealthyThreshold <= 0 {
		return fmt.Errorf("HealthyThreshold must be positive")
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