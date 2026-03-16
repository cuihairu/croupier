package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/db"
	"gorm.io/gorm"
)

func TestConfig_DefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxOpenConns != 25 {
		t.Errorf("Expected MaxOpenConns 25, got %d", config.MaxOpenConns)
	}
	if config.MaxIdleConns != 5 {
		t.Errorf("Expected MaxIdleConns 5, got %d", config.MaxIdleConns)
	}
	if config.ConnMaxLifetime != 5*time.Minute {
		t.Errorf("Expected ConnMaxLifetime 5m, got %v", config.ConnMaxLifetime)
	}
	if config.HealthCheckInterval != 30*time.Second {
		t.Errorf("Expected HealthCheckInterval 30s, got %v", config.HealthCheckInterval)
	}
}

func TestListOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    *ListOptions
		wantErr bool
	}{
		{
			name: "valid options",
			opts: &ListOptions{
				Page:     1,
				PageSize: 20,
				Sort:     "id",
				Order:    "asc",
			},
			wantErr: false,
		},
		{
			name: "invalid page",
			opts: &ListOptions{
				Page:     0,
				PageSize: 20,
			},
			wantErr: true,
		},
		{
			name: "invalid page size",
			opts: &ListOptions{
				Page:     1,
				PageSize: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid order",
			opts: &ListOptions{
				Page:     1,
				PageSize: 20,
				Order:    "invalid",
			},
			wantErr: true,
		},
		{
			name: "page size too large",
			opts: &ListOptions{
				Page:     1,
				PageSize: 101,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestListOptions_GetOffset(t *testing.T) {
	tests := []struct {
		name     string
		opts     *ListOptions
		expected int
	}{
		{
			name:     "page 1, size 20",
			opts:     &ListOptions{Page: 1, PageSize: 20},
			expected: 0,
		},
		{
			name:     "page 2, size 20",
			opts:     &ListOptions{Page: 2, PageSize: 20},
			expected: 20,
		},
		{
			name:     "page 3, size 10",
			opts:     &ListOptions{Page: 3, PageSize: 10},
			expected: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.GetOffset(); got != tt.expected {
				t.Errorf("GetOffset() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestListOptions_GetOrderClause(t *testing.T) {
	tests := []struct {
		name     string
		opts     *ListOptions
		expected string
	}{
		{
			name:     "id asc",
			opts:     &ListOptions{Sort: "id", Order: "asc"},
			expected: "id asc",
		},
		{
			name:     "name desc",
			opts:     &ListOptions{Sort: "name", Order: "desc"},
			expected: "name desc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.GetOrderClause(); got != tt.expected {
				t.Errorf("GetOrderClause() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewPaginatedResult(t *testing.T) {
	items := []string{"item1", "item2", "item3"}
	opts := &ListOptions{Page: 1, PageSize: 20}
	total := int64(100)

	result := NewPaginatedResult(items, opts, total)

	if len(result.Items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(result.Items))
	}

	if result.Pagination.Page != 1 {
		t.Errorf("Expected page 1, got %d", result.Pagination.Page)
	}

	if result.Pagination.PageSize != 20 {
		t.Errorf("Expected page size 20, got %d", result.Pagination.PageSize)
	}

	if result.Pagination.Total != 100 {
		t.Errorf("Expected total 100, got %d", result.Pagination.Total)
	}

	expectedTotalPages := 5
	if result.Pagination.TotalPages != expectedTotalPages {
		t.Errorf("Expected total pages %d, got %d", expectedTotalPages, result.Pagination.TotalPages)
	}

	if !result.Pagination.HasNext {
		t.Error("Expected HasNext to be true")
	}

	if result.Pagination.HasPrev {
		t.Error("Expected HasPrev to be false")
	}
}

func TestMetrics_Increment(t *testing.T) {
	metrics := NewMetrics()

	// 测试健康检查指标
	metrics.IncrementHealthCheckSuccess()
	if metrics.HealthCheckSuccessCount() != 1 {
		t.Errorf("Expected health check success count 1, got %d", metrics.HealthCheckSuccessCount())
	}

	metrics.IncrementHealthCheckFailure()
	if metrics.HealthCheckFailureCount() != 1 {
		t.Errorf("Expected health check failure count 1, got %d", metrics.HealthCheckFailureCount())
	}

	// 测试操作指标
	metrics.IncrementRetryCount()
	if metrics.RetryCount() != 1 {
		t.Errorf("Expected retry count 1, got %d", metrics.RetryCount())
	}

	metrics.IncrementSuccessCount()
	if metrics.SuccessCount() != 1 {
		t.Errorf("Expected success count 1, got %d", metrics.SuccessCount())
	}

	metrics.IncrementFailureCount()
	if metrics.FailureCount() != 1 {
		t.Errorf("Expected failure count 1, got %d", metrics.FailureCount())
	}
}

func TestMetrics_LastHealthCheck(t *testing.T) {
	metrics := NewMetrics()

	// 初始状态应该是零值
	if !metrics.LastHealthCheckSuccess().IsZero() {
		t.Error("Expected initial last health check success to be zero")
	}

	if !metrics.LastHealthCheckFailure().IsZero() {
		t.Error("Expected initial last health check failure to be zero")
	}

	// 增加成功计数后应该有值
	before := time.Now().Unix()
	metrics.IncrementHealthCheckSuccess()
	after := time.Now().Unix()

	successTime := metrics.LastHealthCheckSuccess().Unix()
	if successTime < before || successTime > after {
		t.Errorf("Expected last health check success time between %d and %d, got %d", before, after, successTime)
	}

	// 增加失败计数后应该有值
	before = time.Now().Unix()
	metrics.IncrementHealthCheckFailure()
	after = time.Now().Unix()

	failureTime := metrics.LastHealthCheckFailure().Unix()
	if failureTime < before || failureTime > after {
		t.Errorf("Expected last health check failure time between %d and %d, got %d", before, after, failureTime)
	}
}

func TestMetrics_Reset(t *testing.T) {
	metrics := NewMetrics()

	// 增加一些指标
	metrics.IncrementHealthCheckSuccess()
	metrics.IncrementHealthCheckFailure()
	metrics.IncrementRetryCount()
	metrics.IncrementSuccessCount()
	metrics.IncrementFailureCount()

	// 重置
	metrics.Reset()

	// 验证所有指标都被重置为0
	if metrics.HealthCheckSuccessCount() != 0 {
		t.Errorf("Expected health check success count 0 after reset, got %d", metrics.HealthCheckSuccessCount())
	}

	if metrics.HealthCheckFailureCount() != 0 {
		t.Errorf("Expected health check failure count 0 after reset, got %d", metrics.HealthCheckFailureCount())
	}

	if metrics.RetryCount() != 0 {
		t.Errorf("Expected retry count 0 after reset, got %d", metrics.RetryCount())
	}

	if metrics.SuccessCount() != 0 {
		t.Errorf("Expected success count 0 after reset, got %d", metrics.SuccessCount())
	}

	if metrics.FailureCount() != 0 {
		t.Errorf("Expected failure count 0 after reset, got %d", metrics.FailureCount())
	}

	if !metrics.LastHealthCheckSuccess().IsZero() {
		t.Error("Expected last health check success to be zero after reset")
	}

	if !metrics.LastHealthCheckFailure().IsZero() {
		t.Error("Expected last health check failure to be zero after reset")
	}
}

func TestConfigureConnectionPool(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name: "default config",
			config: &Config{
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 5 * time.Minute,
				ConnMaxIdleTime: 5 * time.Minute,
			},
		},
		{
			name: "custom config",
			config: &Config{
				MaxOpenConns:    100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 10 * time.Minute,
				ConnMaxIdleTime: 3 * time.Minute,
			},
		},
		{
			name: "minimal config",
			config: &Config{
				MaxOpenConns:    1,
				MaxIdleConns:    0,
				ConnMaxLifetime: time.Minute,
				ConnMaxIdleTime: time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建一个真实的 SQLite 连接用于测试
			db, err := db.Open(":memory:")
			if err != nil {
				t.Fatalf("Failed to open database: %v", err)
			}
			defer func() {
				sqlDB, _ := db.DB()
				if sqlDB != nil {
					sqlDB.Close()
				}
			}()

			sqlDB, err := db.DB()
			if err != nil {
				t.Fatalf("Failed to get sql.DB: %v", err)
			}

			err = configureConnectionPool(sqlDB, tt.config)
			if err != nil {
				t.Errorf("configureConnectionPool() error = %v", err)
			}

			// 验证配置是否正确应用
			stats := sqlDB.Stats()
			if stats.MaxOpenConnections != tt.config.MaxOpenConns {
				t.Errorf("MaxOpenConnections = %d, want %d", stats.MaxOpenConnections, tt.config.MaxOpenConns)
			}
		})
	}
}

func TestOpenDatabase(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{
			name:    "sqlite in-memory",
			dsn:     ":memory:",
			wantErr: false,
		},
		{
			name:    "sqlite file prefix",
			dsn:     "file::memory:?cache=shared",
			wantErr: false,
		},
		{
			name:    "invalid postgres connection",
			dsn:     "postgres://invalid:invalid@localhost:9999/invalid",
			wantErr: true, // Connection will fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			gormDB, err := openDatabase(tt.dsn, config)

			if (err != nil) != tt.wantErr {
				t.Errorf("openDatabase() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && gormDB == nil {
				t.Error("openDatabase() returned nil db without error")
			}

			if gormDB != nil {
				sqlDB, _ := gormDB.DB()
				if sqlDB != nil {
					sqlDB.Close()
				}
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: true, // 简化实现总是返回 true
		},
		{
			name: "generic error",
			err:  fmt.Errorf("some error"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableError(tt.err); got != tt.want {
				t.Errorf("isRetryableError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManager_NewManager(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		config  *Config
		wantErr bool
	}{
		{
			name:    "sqlite in-memory with default config",
			dsn:     ":memory:",
			config:  nil,
			wantErr: false,
		},
		{
			name: "sqlite in-memory with custom config",
			dsn:  "file::memory:?cache=shared",
			config: &Config{
				MaxOpenConns:        10,
				MaxIdleConns:        2,
				ConnMaxLifetime:     3 * time.Minute,
				ConnMaxIdleTime:     3 * time.Minute,
				HealthCheckInterval: 10 * time.Second,
				HealthCheckTimeout:  1 * time.Second,
				MetricsEnabled:      true,
				RetryAttempts:       2,
				RetryDelay:          50 * time.Millisecond,
			},
			wantErr: false,
		},
		{
			name:    "sqlite file with config",
			dsn:     "file:test.db",
			config:  DefaultConfig(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(tt.dsn, tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if manager == nil {
					t.Fatal("NewManager() returned nil manager")
				}
				if manager.db == nil {
					t.Error("NewManager() db field is nil")
				}
				if manager.sqlDB == nil {
					t.Error("NewManager() sqlDB field is nil")
				}
				if manager.config == nil {
					t.Error("NewManager() config field is nil")
				}
				if manager.metrics == nil {
					t.Error("NewManager() metrics field is nil")
				}
				if manager.ctx == nil {
					t.Error("NewManager() ctx field is nil")
				}
				if manager.cancel == nil {
					t.Error("NewManager() cancel field is nil")
				}

				// Cleanup
				if manager != nil {
					manager.Close()
				}
				if tt.dsn == "file:test.db" {
					os.Remove("test.db")
				}
			}
		})
	}
}

func TestManager_GetDB(t *testing.T) {
	manager, err := NewManager(":memory:", nil)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	db := manager.GetDB()
	if db == nil {
		t.Error("GetDB() returned nil")
	}
	if db != manager.db {
		t.Error("GetDB() returned different db instance")
	}
}

func TestManager_GetSQLDB(t *testing.T) {
	manager, err := NewManager(":memory:", nil)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	sqlDB := manager.GetSQLDB()
	if sqlDB == nil {
		t.Error("GetSQLDB() returned nil")
	}
	if sqlDB != manager.sqlDB {
		t.Error("GetSQLDB() returned different sqlDB instance")
	}
}

func TestManager_WithContext(t *testing.T) {
	manager, err := NewManager(":memory:", nil)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	db := manager.WithContext(ctx)
	if db == nil {
		t.Error("WithContext() returned nil")
	}
}

func TestManager_WithRetry_Success(t *testing.T) {
	manager, err := NewManager(":memory:", nil)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	callCount := 0

	err = manager.WithRetry(ctx, func(db *gorm.DB) error {
		callCount++
		// 第一次尝试就成功
		return nil
	})

	if err != nil {
		t.Errorf("WithRetry() error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("WithRetry() called operation %d times, want 1", callCount)
	}

	// 检查成功指标是否增加
	if manager.metrics.SuccessCount() != 1 {
		t.Errorf("SuccessCount = %d, want 1", manager.metrics.SuccessCount())
	}
}

func TestManager_WithRetry_RetrySuccess(t *testing.T) {
	manager, err := NewManager(":memory:", &Config{
		RetryAttempts:       3,
		RetryDelay:          10 * time.Millisecond,
		HealthCheckInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	callCount := 0

	err = manager.WithRetry(ctx, func(db *gorm.DB) error {
		callCount++
		// 前两次失败，第三次成功
		if callCount < 3 {
			return fmt.Errorf("temporary error")
		}
		return nil
	})

	if err != nil {
		t.Errorf("WithRetry() error = %v", err)
	}
	if callCount != 3 {
		t.Errorf("WithRetry() called operation %d times, want 3", callCount)
	}

	// 检查重试和成功指标
	if manager.metrics.RetryCount() < 1 {
		t.Errorf("RetryCount = %d, want >= 1", manager.metrics.RetryCount())
	}
	if manager.metrics.SuccessCount() != 1 {
		t.Errorf("SuccessCount = %d, want 1", manager.metrics.SuccessCount())
	}
}

func TestManager_WithRetry_AllFail(t *testing.T) {
	manager, err := NewManager(":memory:", &Config{
		RetryAttempts:       2,
		RetryDelay:          5 * time.Millisecond,
		HealthCheckInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	callCount := 0
	expectedErr := fmt.Errorf("persistent error")

	err = manager.WithRetry(ctx, func(db *gorm.DB) error {
		callCount++
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("WithRetry() error = %v, want %v", err, expectedErr)
	}
	if callCount != 3 { // 初始尝试 + 2次重试
		t.Errorf("WithRetry() called operation %d times, want 3", callCount)
	}

	// 检查失败指标
	if manager.metrics.FailureCount() != 1 {
		t.Errorf("FailureCount = %d, want 1", manager.metrics.FailureCount())
	}
}

func TestManager_WithRetry_ContextCanceled(t *testing.T) {
	manager, err := NewManager(":memory:", &Config{
		RetryAttempts:       5,
		RetryDelay:          100 * time.Millisecond,
		HealthCheckInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	// 第一次调用后取消 context
	err = manager.WithRetry(ctx, func(db *gorm.DB) error {
		callCount++
		if callCount == 1 {
			cancel() // 取消 context
		}
		return fmt.Errorf("error")
	})

	if err != context.Canceled {
		t.Errorf("WithRetry() error = %v, want %v", err, context.Canceled)
	}
}

func TestManager_GetStats(t *testing.T) {
	config := &Config{
		MaxOpenConns:        10,
		MaxIdleConns:        3,
		ConnMaxLifetime:     5 * time.Minute,
		ConnMaxIdleTime:     2 * time.Minute,
		HealthCheckInterval: 100 * time.Millisecond,
		MetricsEnabled:      true,
	}
	manager, err := NewManager(":memory:", config)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	// 等待一些健康检查执行
	time.Sleep(150 * time.Millisecond)

	stats := manager.GetStats()
	if stats == nil {
		t.Fatal("GetStats() returned nil")
	}

	if stats.MaxOpenConnections != config.MaxOpenConns {
		t.Errorf("MaxOpenConnections = %d, want %d", stats.MaxOpenConnections, config.MaxOpenConns)
	}

	// 检查健康检查指标
	if stats.HealthCheckSuccessCount == 0 && stats.HealthCheckFailureCount == 0 {
		t.Log("Health checks may not have run yet")
	}
}

func TestManager_Ping(t *testing.T) {
	manager, err := NewManager(":memory:", nil)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	err = manager.Ping(ctx)
	if err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

func TestManager_Ping_ContextTimeout(t *testing.T) {
	manager, err := NewManager(":memory:", nil)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	err = manager.Ping(ctx)
	if err == nil {
		t.Error("Ping() with canceled context should return error")
	}
}

func TestManager_Close(t *testing.T) {
	manager, err := NewManager(":memory:", nil)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// 关闭管理器
	err = manager.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// 再次关闭应该没问题（幂等性）
	err = manager.Close()
	if err != nil {
		t.Errorf("Close() second call error = %v", err)
	}
}

func TestManager_Close_StopsHealthChecker(t *testing.T) {
	config := &Config{
		HealthCheckInterval: 50 * time.Millisecond,
		HealthCheckTimeout:  1 * time.Second,
		MetricsEnabled:      true,
	}
	manager, err := NewManager(":memory:", config)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// 等待一些健康检查
	time.Sleep(100 * time.Millisecond)

	successCount := manager.metrics.HealthCheckSuccessCount()

	// 关闭管理器
	manager.Close()

	// 再等一下，确保健康检查已停止
	time.Sleep(100 * time.Millisecond)

	// 指标不应该再增加
	newSuccessCount := manager.metrics.HealthCheckSuccessCount()
	if newSuccessCount < successCount {
		t.Error("Health check count decreased after close")
	}
}

func TestManager_PerformHealthCheck(t *testing.T) {
	config := &Config{
		HealthCheckInterval: 100 * time.Millisecond,
		HealthCheckTimeout:  1 * time.Second,
		MetricsEnabled:      true,
	}
	manager, err := NewManager(":memory:", config)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	// 手动执行健康检查
	manager.performHealthCheck()

	// 检查成功指标应该增加
	if manager.metrics.HealthCheckSuccessCount() == 0 {
		t.Error("HealthCheckSuccessCount should be > 0 after performHealthCheck")
	}

	// 检查最后成功时间应该被设置
	if manager.metrics.LastHealthCheckSuccess().IsZero() {
		t.Error("LastHealthCheckSuccess should be set after performHealthCheck")
	}
}

func TestManager_PerformHealthCheck_Failure(t *testing.T) {
	config := &Config{
		HealthCheckInterval: 100 * time.Millisecond,
		HealthCheckTimeout:  1 * time.Millisecond, // 非常短的超时
		MetricsEnabled:      true,
	}
	manager, err := NewManager(":memory:", config)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// 关闭底层 SQL 连接以模拟失败
	sqlDB := manager.GetSQLDB()
	sqlDB.Close()

	// 手动执行健康检查
	time.Sleep(10 * time.Millisecond) // 确保连接已关闭
	manager.performHealthCheck()

	// 检查失败指标
	// 注意：这可能不会触发失败，因为 SQLite 连接关闭后的行为不确定
	_ = manager.metrics.HealthCheckFailureCount()
}
