package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	pkgerrors "github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// ConnectionPoolConfig contains configuration for connection pooling
type ConnectionPoolConfig struct {
	// Pool sizing
	InitialSize    int           // Initial number of connections
	MaxSize        int           // Maximum number of connections
	MinIdleSize    int           // Minimum number of idle connections
	MaxIdleSize    int           // Maximum number of idle connections
	
	// Connection management
	MaxIdleTime    time.Duration // Maximum time a connection can be idle
	MaxLifetime    time.Duration // Maximum lifetime of a connection
	AcquireTimeout time.Duration // Timeout for acquiring a connection
	
	// Health checking
	HealthCheckInterval time.Duration // Interval for health checks
	HealthCheckTimeout  time.Duration // Timeout for health checks
	
	// Network settings
	DialTimeout      time.Duration // Timeout for dialing new connections
	KeepAlive        time.Duration // TCP keep-alive interval
	MaxConnLifetime  time.Duration // Maximum connection lifetime
	
	// Validation
	ValidateOnAcquire bool // Validate connection when acquired
	ValidateOnReturn  bool // Validate connection when returned
}

// DefaultConnectionPoolConfig returns default connection pool configuration
func DefaultConnectionPoolConfig() *ConnectionPoolConfig {
	return &ConnectionPoolConfig{
		InitialSize:         5,
		MaxSize:            50,
		MinIdleSize:        5,
		MaxIdleSize:        10,
		MaxIdleTime:        30 * time.Minute,
		MaxLifetime:        1 * time.Hour,
		AcquireTimeout:     30 * time.Second,
		HealthCheckInterval: 1 * time.Minute,
		HealthCheckTimeout:  5 * time.Second,
		DialTimeout:        10 * time.Second,
		KeepAlive:          30 * time.Second,
		MaxConnLifetime:    1 * time.Hour,
		ValidateOnAcquire:  true,
		ValidateOnReturn:   false,
	}
}

// PooledConnection represents a connection in the pool
type PooledConnection struct {
	ID          string
	Conn        net.Conn
	CreatedAt   time.Time
	LastUsedAt  time.Time
	IsHealthy   bool
	InUse       bool
	UseCount    int64
	ErrorCount  int64
	
	// Connection-specific data
	RemoteAddr  string
	LocalAddr   string
	Protocol    string
	
	// Pool management
	pool        *ConnectionPool
	returnCh    chan *PooledConnection
	mutex       sync.RWMutex
}

// NewPooledConnection creates a new pooled connection
func NewPooledConnection(id string, conn net.Conn, pool *ConnectionPool) *PooledConnection {
	return &PooledConnection{
		ID:         id,
		Conn:       conn,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		IsHealthy:  true,
		InUse:      false,
		UseCount:   0,
		ErrorCount: 0,
		RemoteAddr: conn.RemoteAddr().String(),
		LocalAddr:  conn.LocalAddr().String(),
		Protocol:   conn.RemoteAddr().Network(),
		pool:       pool,
		returnCh:   make(chan *PooledConnection, 1),
	}
}

// Use marks the connection as in use
func (pc *PooledConnection) Use() {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()
	
	pc.InUse = true
	pc.LastUsedAt = time.Now()
	pc.UseCount++
}

// Return returns the connection to the pool
func (pc *PooledConnection) Return() {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()
	
	pc.InUse = false
	pc.LastUsedAt = time.Now()
	
	// Send to return channel (non-blocking)
	select {
	case pc.returnCh <- pc:
	default:
		// Channel full, connection will be closed
		pc.Close()
	}
}

// Close closes the connection
func (pc *PooledConnection) Close() error {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()
	
	if pc.Conn != nil {
		err := pc.Conn.Close()
		pc.Conn = nil
		return err
	}
	return nil
}

// IsExpired checks if the connection has expired
func (pc *PooledConnection) IsExpired(maxLifetime, maxIdleTime time.Duration) bool {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()
	
	now := time.Now()
	
	// Check lifetime
	if maxLifetime > 0 && now.Sub(pc.CreatedAt) > maxLifetime {
		return true
	}
	
	// Check idle time
	if maxIdleTime > 0 && !pc.InUse && now.Sub(pc.LastUsedAt) > maxIdleTime {
		return true
	}
	
	return false
}

// Validate checks if the connection is healthy
func (pc *PooledConnection) Validate() bool {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()
	
	if pc.Conn == nil {
		return false
	}
	
	// Simple validation - check if connection is not closed
	// More sophisticated validation could be added here
	return pc.IsHealthy
}

// GetStats returns connection statistics
func (pc *PooledConnection) GetStats() map[string]interface{} {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()
	
	return map[string]interface{}{
		"id":          pc.ID,
		"created_at":  pc.CreatedAt,
		"last_used":   pc.LastUsedAt,
		"is_healthy":  pc.IsHealthy,
		"in_use":      pc.InUse,
		"use_count":   pc.UseCount,
		"error_count": pc.ErrorCount,
		"remote_addr": pc.RemoteAddr,
		"local_addr":  pc.LocalAddr,
		"protocol":    pc.Protocol,
	}
}

// ConnectionPool manages a pool of network connections
type ConnectionPool struct {
	config      *ConnectionPoolConfig
	connections map[string]*PooledConnection
	idleConns   chan *PooledConnection
	
	// Pool state
	totalConns   int32 // atomic
	activeConns  int32 // atomic
	idleCount    int32 // atomic
	
	// Statistics
	stats       *PoolStats
	
	// Synchronization
	mutex       sync.RWMutex
	
	// Lifecycle
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	closed      bool
	
	// Connection factory
	factory     ConnectionFactory
}

// ConnectionFactory creates new connections
type ConnectionFactory interface {
	CreateConnection(ctx context.Context) (net.Conn, error)
	ValidateConnection(conn net.Conn) bool
	CloseConnection(conn net.Conn) error
}

// PoolStats tracks pool statistics
type PoolStats struct {
	TotalAcquired    int64
	TotalReturned    int64
	TotalCreated     int64
	TotalClosed      int64
	TotalErrors      int64
	AcquireTimeouts  int64
	ValidationErrors int64
	
	// Timing statistics
	AvgAcquireTime   time.Duration
	AvgCreateTime    time.Duration
	
	mutex sync.RWMutex
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config *ConnectionPoolConfig, factory ConnectionFactory) (*ConnectionPool, error) {
	if config == nil {
		config = DefaultConnectionPoolConfig()
	}
	
	if factory == nil {
		return nil, errors.New("connection factory is required")
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	pool := &ConnectionPool{
		config:      config,
		connections: make(map[string]*PooledConnection),
		idleConns:   make(chan *PooledConnection, config.MaxIdleSize),
		stats:       &PoolStats{},
		ctx:         ctx,
		cancel:      cancel,
		factory:     factory,
	}
	
	// Pre-create initial connections
	if err := pool.initialize(); err != nil {
		pool.Close()
		return nil, pkgerrors.WithOperation("initialize", "connection_pool", err)
	}
	
	// Start background workers
	pool.wg.Add(2)
	go pool.healthCheckWorker()
	go pool.maintenanceWorker()
	
	return pool, nil
}

// initialize creates initial connections
func (p *ConnectionPool) initialize() error {
	for i := 0; i < p.config.InitialSize; i++ {
		if err := p.createConnection(); err != nil {
			return pkgerrors.WithOperation("create_initial_connection", fmt.Sprintf("connection_%d", i), err)
		}
	}
	return nil
}

// Acquire gets a connection from the pool
func (p *ConnectionPool) Acquire(ctx context.Context) (*PooledConnection, error) {
	start := time.Now()
	defer func() {
		p.updateAcquireStats(time.Since(start))
	}()
	
	p.stats.mutex.Lock()
	p.stats.TotalAcquired++
	p.stats.mutex.Unlock()
	
	// Create timeout context
	acquireCtx := ctx
	if p.config.AcquireTimeout > 0 {
		var cancel context.CancelFunc
		acquireCtx, cancel = context.WithTimeout(ctx, p.config.AcquireTimeout)
		defer cancel()
	}
	
	// Try to get an idle connection first
	select {
	case conn := <-p.idleConns:
		atomic.AddInt32(&p.idleCount, -1)
		
		// Validate connection if configured
		if p.config.ValidateOnAcquire && !conn.Validate() {
			conn.Close()
			return p.Acquire(ctx) // Retry
		}
		
		conn.Use()
		atomic.AddInt32(&p.activeConns, 1)
		return conn, nil
	default:
		// No idle connections available
	}
	
	// Try to create a new connection if under limit
	if atomic.LoadInt32(&p.totalConns) < int32(p.config.MaxSize) {
		if err := p.createConnection(); err == nil {
			return p.Acquire(ctx) // Retry with new connection
		}
	}
	
	// Wait for a connection to become available
	select {
	case conn := <-p.idleConns:
		atomic.AddInt32(&p.idleCount, -1)
		
		// Validate connection if configured
		if p.config.ValidateOnAcquire && !conn.Validate() {
			conn.Close()
			return p.Acquire(ctx) // Retry
		}
		
		conn.Use()
		atomic.AddInt32(&p.activeConns, 1)
		return conn, nil
	case <-acquireCtx.Done():
		p.stats.mutex.Lock()
		p.stats.AcquireTimeouts++
		p.stats.mutex.Unlock()
		return nil, pkgerrors.WithOperation("acquire", "connection", acquireCtx.Err()).(*pkgerrors.OperationError).WithCode("ACQUIRE_TIMEOUT")
	}
}

// Return returns a connection to the pool
func (p *ConnectionPool) Return(conn *PooledConnection) error {
	if conn == nil {
		return errors.New("connection is nil")
	}
	
	p.stats.mutex.Lock()
	p.stats.TotalReturned++
	p.stats.mutex.Unlock()
	
	// Validate connection if configured
	if p.config.ValidateOnReturn && !conn.Validate() {
		return p.closeConnection(conn)
	}
	
	// Check if connection is expired
	if conn.IsExpired(p.config.MaxLifetime, p.config.MaxIdleTime) {
		return p.closeConnection(conn)
	}
	
	// Mark as not in use
	conn.Return()
	atomic.AddInt32(&p.activeConns, -1)
	
	// Try to return to idle pool
	select {
	case p.idleConns <- conn:
		atomic.AddInt32(&p.idleCount, 1)
		return nil
	default:
		// Idle pool is full, close connection
		return p.closeConnection(conn)
	}
}

// createConnection creates a new connection
func (p *ConnectionPool) createConnection() error {
	start := time.Now()
	
	conn, err := p.factory.CreateConnection(p.ctx)
	if err != nil {
		p.stats.mutex.Lock()
		p.stats.TotalErrors++
		p.stats.mutex.Unlock()
		return err
	}
	
	// Create pooled connection
	id := fmt.Sprintf("conn-%d", atomic.AddInt32(&p.totalConns, 1))
	pooledConn := NewPooledConnection(id, conn, p)
	
	// Add to pool
	p.mutex.Lock()
	p.connections[id] = pooledConn
	p.mutex.Unlock()
	
	// Add to idle connections
	select {
	case p.idleConns <- pooledConn:
		atomic.AddInt32(&p.idleCount, 1)
	default:
		// Pool is full, close connection
		pooledConn.Close()
		atomic.AddInt32(&p.totalConns, -1)
		return errors.New("idle pool is full")
	}
	
	// Update stats
	p.stats.mutex.Lock()
	p.stats.TotalCreated++
	p.stats.mutex.Unlock()
	
	p.updateCreateStats(time.Since(start))
	
	return nil
}

// closeConnection closes and removes a connection from the pool
func (p *ConnectionPool) closeConnection(conn *PooledConnection) error {
	if conn == nil {
		return nil
	}
	
	// Remove from pool
	p.mutex.Lock()
	delete(p.connections, conn.ID)
	p.mutex.Unlock()
	
	// Close connection
	err := conn.Close()
	
	// Update counters
	atomic.AddInt32(&p.totalConns, -1)
	
	// Update stats
	p.stats.mutex.Lock()
	p.stats.TotalClosed++
	p.stats.mutex.Unlock()
	
	return err
}

// healthCheckWorker periodically checks connection health
func (p *ConnectionPool) healthCheckWorker() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.performHealthCheck()
		}
	}
}

// performHealthCheck checks all connections for health
func (p *ConnectionPool) performHealthCheck() {
	p.mutex.RLock()
	connections := make([]*PooledConnection, 0, len(p.connections))
	for _, conn := range p.connections {
		connections = append(connections, conn)
	}
	p.mutex.RUnlock()
	
	for _, conn := range connections {
		if !conn.Validate() {
			p.closeConnection(conn)
		}
	}
}

// maintenanceWorker performs pool maintenance tasks
func (p *ConnectionPool) maintenanceWorker() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.performMaintenance()
		}
	}
}

// performMaintenance performs pool maintenance
func (p *ConnectionPool) performMaintenance() {
	// Remove expired connections
	p.removeExpiredConnections()
	
	// Ensure minimum idle connections
	p.ensureMinimumIdle()
	
	// Clean up idle connections above maximum
	p.cleanupExcessIdle()
}

// removeExpiredConnections removes expired connections
func (p *ConnectionPool) removeExpiredConnections() {
	p.mutex.RLock()
	connections := make([]*PooledConnection, 0, len(p.connections))
	for _, conn := range p.connections {
		connections = append(connections, conn)
	}
	p.mutex.RUnlock()
	
	for _, conn := range connections {
		if conn.IsExpired(p.config.MaxLifetime, p.config.MaxIdleTime) {
			p.closeConnection(conn)
		}
	}
}

// ensureMinimumIdle ensures minimum number of idle connections
func (p *ConnectionPool) ensureMinimumIdle() {
	currentIdle := atomic.LoadInt32(&p.idleCount)
	needed := int32(p.config.MinIdleSize) - currentIdle
	
	for i := int32(0); i < needed; i++ {
		if atomic.LoadInt32(&p.totalConns) >= int32(p.config.MaxSize) {
			break
		}
		
		if err := p.createConnection(); err != nil {
			// Log error but continue
			break
		}
	}
}

// cleanupExcessIdle removes excess idle connections
func (p *ConnectionPool) cleanupExcessIdle() {
	currentIdle := atomic.LoadInt32(&p.idleCount)
	excess := currentIdle - int32(p.config.MaxIdleSize)
	
	for i := int32(0); i < excess; i++ {
		select {
		case conn := <-p.idleConns:
			atomic.AddInt32(&p.idleCount, -1)
			p.closeConnection(conn)
		default:
			return
		}
	}
}

// GetStats returns pool statistics
func (p *ConnectionPool) GetStats() *PoolStats {
	p.stats.mutex.RLock()
	defer p.stats.mutex.RUnlock()
	
	return &PoolStats{
		TotalAcquired:    p.stats.TotalAcquired,
		TotalReturned:    p.stats.TotalReturned,
		TotalCreated:     p.stats.TotalCreated,
		TotalClosed:      p.stats.TotalClosed,
		TotalErrors:      p.stats.TotalErrors,
		AcquireTimeouts:  p.stats.AcquireTimeouts,
		ValidationErrors: p.stats.ValidationErrors,
		AvgAcquireTime:   p.stats.AvgAcquireTime,
		AvgCreateTime:    p.stats.AvgCreateTime,
	}
}

// GetPoolInfo returns current pool information
func (p *ConnectionPool) GetPoolInfo() map[string]interface{} {
	return map[string]interface{}{
		"total_connections":  atomic.LoadInt32(&p.totalConns),
		"active_connections": atomic.LoadInt32(&p.activeConns),
		"idle_connections":   atomic.LoadInt32(&p.idleCount),
		"max_size":          p.config.MaxSize,
		"min_idle":          p.config.MinIdleSize,
		"max_idle":          p.config.MaxIdleSize,
		"stats":             p.GetStats(),
	}
}

// Close closes the connection pool
func (p *ConnectionPool) Close() error {
	p.mutex.Lock()
	if p.closed {
		p.mutex.Unlock()
		return nil
	}
	p.closed = true
	p.mutex.Unlock()
	
	// Cancel context to stop workers
	p.cancel()
	
	// Wait for workers to finish
	p.wg.Wait()
	
	// Close all connections
	p.mutex.Lock()
	connections := make([]*PooledConnection, 0, len(p.connections))
	for _, conn := range p.connections {
		connections = append(connections, conn)
	}
	p.connections = make(map[string]*PooledConnection)
	p.mutex.Unlock()
	
	// Close connections
	for _, conn := range connections {
		conn.Close()
	}
	
	// Close idle channel
	close(p.idleConns)
	
	return nil
}

// updateAcquireStats updates acquire timing statistics
func (p *ConnectionPool) updateAcquireStats(duration time.Duration) {
	p.stats.mutex.Lock()
	defer p.stats.mutex.Unlock()
	
	if p.stats.TotalAcquired > 0 {
		p.stats.AvgAcquireTime = time.Duration(
			(int64(p.stats.AvgAcquireTime)*p.stats.TotalAcquired + int64(duration)) / 
			(p.stats.TotalAcquired + 1),
		)
	} else {
		p.stats.AvgAcquireTime = duration
	}
}

// updateCreateStats updates create timing statistics
func (p *ConnectionPool) updateCreateStats(duration time.Duration) {
	p.stats.mutex.Lock()
	defer p.stats.mutex.Unlock()
	
	if p.stats.TotalCreated > 0 {
		p.stats.AvgCreateTime = time.Duration(
			(int64(p.stats.AvgCreateTime)*p.stats.TotalCreated + int64(duration)) / 
			(p.stats.TotalCreated + 1),
		)
	} else {
		p.stats.AvgCreateTime = duration
	}
}

// HTTPConnectionFactory creates HTTP connections
type HTTPConnectionFactory struct {
	target string
	client *http.Client
}

// NewHTTPConnectionFactory creates a new HTTP connection factory
func NewHTTPConnectionFactory(target string) *HTTPConnectionFactory {
	return &HTTPConnectionFactory{
		target: target,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateConnection creates a new HTTP connection
func (f *HTTPConnectionFactory) CreateConnection(ctx context.Context) (net.Conn, error) {
	// This is a simplified implementation
	// In practice, you'd establish a proper HTTP connection
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}
	return dialer.DialContext(ctx, "tcp", f.target)
}

// ValidateConnection validates an HTTP connection
func (f *HTTPConnectionFactory) ValidateConnection(conn net.Conn) bool {
	// Simple validation - check if connection is not nil and not closed
	if conn == nil {
		return false
	}
	
	// Try to read from connection with a very short timeout
	conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	buffer := make([]byte, 1)
	_, err := conn.Read(buffer)
	
	// Reset deadline
	conn.SetReadDeadline(time.Time{})
	
	// If we get a timeout, connection is likely healthy
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	
	// Connection is closed or has other issues
	return false
}

// CloseConnection closes an HTTP connection
func (f *HTTPConnectionFactory) CloseConnection(conn net.Conn) error {
	if conn != nil {
		return conn.Close()
	}
	return nil
}

// WebSocketConnectionFactory creates WebSocket connections
type WebSocketConnectionFactory struct {
	url    string
	dialer *websocket.Dialer
}

// NewWebSocketConnectionFactory creates a new WebSocket connection factory
func NewWebSocketConnectionFactory(url string) *WebSocketConnectionFactory {
	return &WebSocketConnectionFactory{
		url: url,
		dialer: &websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		},
	}
}

// CreateConnection creates a new WebSocket connection
func (f *WebSocketConnectionFactory) CreateConnection(ctx context.Context) (net.Conn, error) {
	conn, _, err := f.dialer.DialContext(ctx, f.url, nil)
	if err != nil {
		return nil, err
	}
	
	// Return the underlying network connection
	return conn.UnderlyingConn(), nil
}

// ValidateConnection validates a WebSocket connection
func (f *WebSocketConnectionFactory) ValidateConnection(conn net.Conn) bool {
	// WebSocket-specific validation would go here
	return f.basicValidation(conn)
}

// CloseConnection closes a WebSocket connection
func (f *WebSocketConnectionFactory) CloseConnection(conn net.Conn) error {
	if conn != nil {
		return conn.Close()
	}
	return nil
}

// basicValidation performs basic connection validation
func (f *WebSocketConnectionFactory) basicValidation(conn net.Conn) bool {
	if conn == nil {
		return false
	}
	
	// Try to read from connection with a very short timeout
	conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	buffer := make([]byte, 1)
	_, err := conn.Read(buffer)
	
	// Reset deadline
	conn.SetReadDeadline(time.Time{})
	
	// If we get a timeout, connection is likely healthy
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	
	// Connection is closed or has other issues
	return false
}