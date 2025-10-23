package connpool

import (
	"context"
	"crypto/tls"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// ConnectionPool manages gRPC connections with pooling and health checks
type ConnectionPool interface {
	// Get returns a connection for the given target
	Get(ctx context.Context, target string) (*grpc.ClientConn, error)
	// Put returns a connection to the pool (for reference counting)
	Put(target string, conn *grpc.ClientConn)
	// Remove forcibly removes and closes a connection
	Remove(target string) error
	// Close closes all connections in the pool
	Close() error
	// Stats returns current pool statistics
	Stats() *PoolStats
}

// PoolConfig holds configuration for the connection pool
type PoolConfig struct {
	// MaxConnections is the maximum number of connections per target
	MaxConnections int
	// MaxIdleTime is the maximum time a connection can be idle
	MaxIdleTime time.Duration
	// HealthCheckInterval is the interval for health checking connections
	HealthCheckInterval time.Duration
	// DialTimeout is the timeout for establishing new connections
	DialTimeout time.Duration
	// TLS configuration
	TLSConfig *tls.Config
	// InsecureSkipVerify disables TLS verification (for development)
	InsecureSkipVerify bool
	// DialOptions are additional options for gRPC dial
	DialOptions []grpc.DialOption
}

// PoolStats holds statistics about the connection pool
type PoolStats struct {
	// TotalConnections is the total number of active connections
	TotalConnections int
	// IdleConnections is the number of idle connections
	IdleConnections int
	// ConnectionsPerTarget shows connections per target
	ConnectionsPerTarget map[string]int
	// HealthyConnections is the number of healthy connections
	HealthyConnections int
	// UnhealthyConnections is the number of unhealthy connections
	UnhealthyConnections int
}

// ConnectionInfo holds information about a pooled connection
type ConnectionInfo struct {
	conn        *grpc.ClientConn
	target      string
	createdAt   time.Time
	lastUsed    time.Time
	useCount    int64
	healthy     bool
	mu          sync.RWMutex
}

// DefaultConnectionPool implements ConnectionPool interface
type DefaultConnectionPool struct {
	mu          sync.RWMutex
	connections map[string]*ConnectionInfo // target -> connection info
	config      *PoolConfig
	ctx         context.Context
	cancel      context.CancelFunc
	closed      bool
}

// NewConnectionPool creates a new connection pool with the given configuration
func NewConnectionPool(config *PoolConfig) ConnectionPool {
	if config == nil {
		config = DefaultPoolConfig()
	}

	// Validate and set defaults
	if config.MaxConnections <= 0 {
		config.MaxConnections = 10
	}
	if config.MaxIdleTime <= 0 {
		config.MaxIdleTime = 5 * time.Minute
	}
	if config.HealthCheckInterval <= 0 {
		config.HealthCheckInterval = 30 * time.Second
	}
	if config.DialTimeout <= 0 {
		config.DialTimeout = 10 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &DefaultConnectionPool{
		connections: make(map[string]*ConnectionInfo),
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start background goroutines
	go pool.healthChecker()
	go pool.idleConnectionCleaner()

	return pool
}

// DefaultPoolConfig returns a default configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxConnections:      10,
		MaxIdleTime:         5 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
		DialTimeout:         10 * time.Second,
		InsecureSkipVerify:  false,
		DialOptions:         []grpc.DialOption{},
	}
}

func (p *DefaultConnectionPool) Get(ctx context.Context, target string) (*grpc.ClientConn, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, ErrPoolClosed
	}

	connInfo, exists := p.connections[target]
	p.mu.RUnlock()

	if exists && connInfo.healthy {
		// Update last used time and increment use count
		connInfo.mu.Lock()
		connInfo.lastUsed = time.Now()
		connInfo.useCount++
		connInfo.mu.Unlock()
		return connInfo.conn, nil
	}

	// Create new connection
	return p.createConnection(ctx, target)
}

func (p *DefaultConnectionPool) Put(target string, conn *grpc.ClientConn) {
	// In this implementation, Put is a no-op since we're not using
	// a traditional object pool pattern. The connection is managed
	// by the pool until it's idle or unhealthy.
}

func (p *DefaultConnectionPool) Remove(target string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	connInfo, exists := p.connections[target]
	if !exists {
		return nil
	}

	delete(p.connections, target)
	return connInfo.conn.Close()
}

func (p *DefaultConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	p.cancel() // Stop background goroutines

	// Close all connections
	for target, connInfo := range p.connections {
		connInfo.conn.Close()
		delete(p.connections, target)
	}

	return nil
}

func (p *DefaultConnectionPool) Stats() *PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := &PoolStats{
		ConnectionsPerTarget: make(map[string]int),
	}

	for target, connInfo := range p.connections {
		stats.TotalConnections++
		stats.ConnectionsPerTarget[target]++

		if connInfo.healthy {
			stats.HealthyConnections++
		} else {
			stats.UnhealthyConnections++
		}

		// Check if idle
		connInfo.mu.RLock()
		if time.Since(connInfo.lastUsed) > p.config.MaxIdleTime {
			stats.IdleConnections++
		}
		connInfo.mu.RUnlock()
	}

	return stats
}

func (p *DefaultConnectionPool) createConnection(ctx context.Context, target string) (*grpc.ClientConn, error) {
	// Check if we're at the connection limit for this target
	p.mu.RLock()
	targetCount := 0
	for t := range p.connections {
		if t == target {
			targetCount++
		}
	}
	p.mu.RUnlock()

	if targetCount >= p.config.MaxConnections {
		return nil, ErrTooManyConnections
	}

	// Prepare dial options
	dialOpts := append([]grpc.DialOption{}, p.config.DialOptions...)

	// Add credentials
	if p.config.TLSConfig != nil {
		creds := credentials.NewTLS(p.config.TLSConfig)
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else if p.config.InsecureSkipVerify {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// Use system's root CAs
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(nil)))
	}

	// Set timeout
	dialCtx, cancel := context.WithTimeout(ctx, p.config.DialTimeout)
	defer cancel()

	// Create connection
	conn, err := grpc.DialContext(dialCtx, target, dialOpts...)
	if err != nil {
		return nil, err
	}

	// Store connection info
	connInfo := &ConnectionInfo{
		conn:      conn,
		target:    target,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		useCount:  1,
		healthy:   true,
	}

	p.mu.Lock()
	if !p.closed {
		p.connections[target] = connInfo
	}
	p.mu.Unlock()

	// If pool was closed while we were creating the connection, close it
	if p.closed {
		conn.Close()
		return nil, ErrPoolClosed
	}

	return conn, nil
}

func (p *DefaultConnectionPool) healthChecker() {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.checkHealth()
		}
	}
}

func (p *DefaultConnectionPool) checkHealth() {
	p.mu.RLock()
	connections := make([]*ConnectionInfo, 0, len(p.connections))
	for _, connInfo := range p.connections {
		connections = append(connections, connInfo)
	}
	p.mu.RUnlock()

	for _, connInfo := range connections {
		state := connInfo.conn.GetState()
		healthy := state == connectivity.Ready || state == connectivity.Idle

		connInfo.mu.Lock()
		wasHealthy := connInfo.healthy
		connInfo.healthy = healthy
		connInfo.mu.Unlock()

		// Remove unhealthy connections that were previously healthy
		if wasHealthy && !healthy {
			p.Remove(connInfo.target)
		}
	}
}

func (p *DefaultConnectionPool) idleConnectionCleaner() {
	ticker := time.NewTicker(p.config.MaxIdleTime / 2) // Check every half of max idle time
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.cleanIdleConnections()
		}
	}
}

func (p *DefaultConnectionPool) cleanIdleConnections() {
	p.mu.RLock()
	toRemove := make([]string, 0)

	for target, connInfo := range p.connections {
		connInfo.mu.RLock()
		isIdle := time.Since(connInfo.lastUsed) > p.config.MaxIdleTime
		connInfo.mu.RUnlock()

		if isIdle {
			toRemove = append(toRemove, target)
		}
	}
	p.mu.RUnlock()

	// Remove idle connections
	for _, target := range toRemove {
		p.Remove(target)
	}
}