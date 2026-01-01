package connpool

import (
	"context"
	"crypto/tls"
	"errors"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
)

// TestDefaultPoolConfig 测试默认配置
func TestDefaultPoolConfig(t *testing.T) {
	config := DefaultPoolConfig()

	if config.MaxConnections != 10 {
		t.Errorf("Expected MaxConnections 10, got %d", config.MaxConnections)
	}
	if config.MaxIdleTime != 5*time.Minute {
		t.Errorf("Expected MaxIdleTime 5m, got %v", config.MaxIdleTime)
	}
	if config.HealthCheckInterval != 30*time.Second {
		t.Errorf("Expected HealthCheckInterval 30s, got %v", config.HealthCheckInterval)
	}
	if config.DialTimeout != 10*time.Second {
		t.Errorf("Expected DialTimeout 10s, got %v", config.DialTimeout)
	}
	if config.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify false")
	}
}

// TestNewConnectionPool_NilConfig 测试空配置使用默认值
func TestNewConnectionPool_NilConfig(t *testing.T) {
	pool := NewConnectionPool(nil).(*DefaultConnectionPool)
	defer pool.Close()

	if pool.config.MaxConnections != 10 {
		t.Errorf("Expected default MaxConnections 10, got %d", pool.config.MaxConnections)
	}
}

// TestNewConnectionPool_ZeroValues 测试零值使用默认值
func TestNewConnectionPool_ZeroValues(t *testing.T) {
	config := &PoolConfig{
		MaxConnections:      0,
		MaxIdleTime:         0,
		HealthCheckInterval: 0,
		DialTimeout:         0,
	}
	pool := NewConnectionPool(config).(*DefaultConnectionPool)
	defer pool.Close()

	if pool.config.MaxConnections != 10 {
		t.Errorf("Expected default MaxConnections 10, got %d", pool.config.MaxConnections)
	}
	if pool.config.MaxIdleTime != 5*time.Minute {
		t.Errorf("Expected default MaxIdleTime 5m, got %v", pool.config.MaxIdleTime)
	}
}

// TestNewConnectionPool_CustomConfig 测试自定义配置
func TestNewConnectionPool_CustomConfig(t *testing.T) {
	config := &PoolConfig{
		MaxConnections:      5,
		MaxIdleTime:         10 * time.Minute,
		HealthCheckInterval: 1 * time.Minute,
		DialTimeout:         30 * time.Second,
		InsecureSkipVerify:  true,
	}
	pool := NewConnectionPool(config).(*DefaultConnectionPool)
	defer pool.Close()

	if pool.config.MaxConnections != 5 {
		t.Errorf("Expected MaxConnections 5, got %d", pool.config.MaxConnections)
	}
	if pool.config.MaxIdleTime != 10*time.Minute {
		t.Errorf("Expected MaxIdleTime 10m, got %v", pool.config.MaxIdleTime)
	}
}

// TestConnectionPool_Close 测试关闭连接池
func TestConnectionPool_Close(t *testing.T) {
	pool := NewConnectionPool(nil).(*DefaultConnectionPool)

	err := pool.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// 关闭应该是幂等的
	err = pool.Close()
	if err != nil {
		t.Errorf("Close() second call error = %v", err)
	}

	// 关闭后不能获取连接
	pool.mu.RLock()
	closed := pool.closed
	pool.mu.RUnlock()

	if !closed {
		t.Error("Pool should be marked as closed")
	}
}

// TestConnectionPool_Get_ClosedPool 测试从关闭的池获取连接
func TestConnectionPool_Get_ClosedPool(t *testing.T) {
	pool := NewConnectionPool(nil).(*DefaultConnectionPool)
	pool.Close()

	ctx := context.Background()
	_, err := pool.Get(ctx, "target1")

	if !errors.Is(err, ErrPoolClosed) {
		t.Errorf("Expected ErrPoolClosed, got %v", err)
	}
}

// TestConnectionPool_Remove_NonExistent 测试移除不存在的连接
func TestConnectionPool_Remove_NonExistent(t *testing.T) {
	pool := NewConnectionPool(nil).(*DefaultConnectionPool)
	defer pool.Close()

	// 移除不存在的连接不应该报错
	err := pool.Remove("nonexistent")
	if err != nil {
		t.Errorf("Remove() nonexistent error = %v", err)
	}
}

// TestConnectionPool_Stats 测试获取统计信息
func TestConnectionPool_Stats(t *testing.T) {
	pool := NewConnectionPool(nil).(*DefaultConnectionPool)
	defer pool.Close()

	// 初始统计
	stats := pool.Stats()
	if stats.TotalConnections != 0 {
		t.Errorf("Expected 0 total connections, got %d", stats.TotalConnections)
	}
	if stats.HealthyConnections != 0 {
		t.Errorf("Expected 0 healthy connections, got %d", stats.HealthyConnections)
	}
	if stats.IdleConnections != 0 {
		t.Errorf("Expected 0 idle connections, got %d", stats.IdleConnections)
	}
	if len(stats.ConnectionsPerTarget) != 0 {
		t.Errorf("Expected 0 targets, got %d", len(stats.ConnectionsPerTarget))
	}
}

// TestConnectionPool_Put 测试 Put 操作
func TestConnectionPool_Put(t *testing.T) {
	pool := NewConnectionPool(nil).(*DefaultConnectionPool)
	defer pool.Close()

	// Put 应该是空操作（no-op），不引起恐慌
	var conn *grpc.ClientConn
	pool.Put("target", conn)
}

// TestPoolErrors 测试错误变量
func TestPoolErrors(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		want  string
	}{
		{"ErrPoolClosed", ErrPoolClosed, "connection pool is closed"},
		{"ErrTooManyConnections", ErrTooManyConnections, "too many connections for target"},
		{"ErrConnectionUnhealthy", ErrConnectionUnhealthy, "connection is unhealthy"},
		{"ErrDialTimeout", ErrDialTimeout, "connection dial timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Errorf("Error message = %q, want %q", tt.err.Error(), tt.want)
			}
		})
	}
}

// TestPoolConfig_TLSConfig 测试 TLS 配置
func TestPoolConfig_TLSConfig(t *testing.T) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}

	config := &PoolConfig{
		TLSConfig: tlsConfig,
	}

	pool := NewConnectionPool(config).(*DefaultConnectionPool)
	defer pool.Close()

	if pool.config.TLSConfig != tlsConfig {
		t.Error("TLSConfig should be set")
	}
}

// BenchmarkConnectionPool_Stats 性能基准测试
func BenchmarkConnectionPool_Stats(b *testing.B) {
	pool := NewConnectionPool(nil).(*DefaultConnectionPool)
	defer pool.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Stats()
	}
}

// BenchmarkNewConnectionPool 性能基准测试
func BenchmarkNewConnectionPool(b *testing.B) {
	config := DefaultPoolConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool := NewConnectionPool(config)
		pool.Close()
	}
}

// TestConnectionPool_ConcurrentOperations 测试并发操作
func TestConnectionPool_ConcurrentOperations(t *testing.T) {
	pool := NewConnectionPool(nil).(*DefaultConnectionPool)
	defer pool.Close()

	var wg sync.WaitGroup

	// 并发读取统计
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pool.Stats()
		}()
	}

	// 并发移除不存在的连接
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pool.Remove("nonexistent")
		}()
	}

	wg.Wait()
	// 测试不崩溃即可
}

// TestConnectionPool_CloseWithBackgroundGoroutines 测试关闭时停止后台协程
func TestConnectionPool_CloseWithBackgroundGoroutines(t *testing.T) {
	pool := NewConnectionPool(nil).(*DefaultConnectionPool)

	// 等待后台协程启动
	time.Sleep(100 * time.Millisecond)

	// 关闭池应该停止所有后台协程
	err := pool.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// 等待一段时间，确保没有 panic
	time.Sleep(200 * time.Millisecond)
}

// TestPoolConfig_DialOptions 测试 DialOptions
func TestPoolConfig_DialOptions(t *testing.T) {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024)),
	}

	config := &PoolConfig{
		DialOptions: opts,
	}

	pool := NewConnectionPool(config).(*DefaultConnectionPool)
	defer pool.Close()

	if len(pool.config.DialOptions) != 2 {
		t.Errorf("Expected 2 dial options, got %d", len(pool.config.DialOptions))
	}
}

// TestConnectionPool_MultipleClose 测试多次关闭
func TestConnectionPool_MultipleClose(t *testing.T) {
	pool := NewConnectionPool(nil).(*DefaultConnectionPool)

	// 第一次关闭
	err := pool.Close()
	if err != nil {
		t.Errorf("First Close() error = %v", err)
	}

	// 第二次关闭
	err = pool.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}

	// 第三次关闭
	err = pool.Close()
	if err != nil {
		t.Errorf("Third Close() error = %v", err)
	}
}

// TestConnectionPool_ConcurrentClose 测试并发关闭
func TestConnectionPool_ConcurrentClose(t *testing.T) {
	pool := NewConnectionPool(nil)

	var wg sync.WaitGroup
	// 并发关闭
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pool.Close()
		}()
	}

	wg.Wait()
	// 测试不崩溃即可
}

// TestConnectionInfo 测试连接信息结构
func TestConnectionInfo(t *testing.T) {
	var conn *grpc.ClientConn = nil

	connInfo := &ConnectionInfo{
		conn:      conn,
		target:    "test-target",
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		useCount:  5,
		healthy:   true,
	}

	// 验证字段
	if connInfo.target != "test-target" {
		t.Errorf("Expected target 'test-target', got %s", connInfo.target)
	}
	if connInfo.useCount != 5 {
		t.Errorf("Expected useCount 5, got %d", connInfo.useCount)
	}
	if !connInfo.healthy {
		t.Error("Expected connection to be healthy")
	}

	// 测试锁
	connInfo.mu.Lock()
	connInfo.useCount = 10
	connInfo.mu.Unlock()

	if connInfo.useCount != 10 {
		t.Errorf("Expected useCount 10 after update, got %d", connInfo.useCount)
	}
}

// TestConnectionInfo_Concurrency 测试并发访问连接信息
func TestConnectionInfo_Concurrency(t *testing.T) {
	var conn *grpc.ClientConn = nil

	connInfo := &ConnectionInfo{
		conn:      conn,
		target:    "test-target",
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		useCount:  0,
		healthy:   true,
	}

	// 并发读写
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			connInfo.mu.RLock()
			_ = connInfo.useCount
			connInfo.mu.RUnlock()
		}()
		go func() {
			defer wg.Done()
			connInfo.mu.Lock()
			connInfo.useCount++
			connInfo.mu.Unlock()
		}()
	}
	wg.Wait()

	if connInfo.useCount != 100 {
		t.Errorf("Expected useCount 100, got %d", connInfo.useCount)
	}
}

// TestPoolStats 测试 PoolStats 结构
func TestPoolStats(t *testing.T) {
	stats := &PoolStats{
		TotalConnections:      10,
		IdleConnections:       3,
		HealthyConnections:    8,
		UnhealthyConnections:  2,
		ConnectionsPerTarget:  map[string]int{"target1": 5, "target2": 5},
	}

	if stats.TotalConnections != 10 {
		t.Errorf("Expected TotalConnections 10, got %d", stats.TotalConnections)
	}
	if len(stats.ConnectionsPerTarget) != 2 {
		t.Errorf("Expected 2 targets, got %d", len(stats.ConnectionsPerTarget))
	}
}
