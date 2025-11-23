package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	ctxmanager "github.com/cuihairu/croupier/internal/context"
	"github.com/cuihairu/croupier/internal/db"
)

// Config 数据库连接配置
type Config struct {
	// 连接池配置
	MaxOpenConns    int           `yaml:"max_open_conns" default:"25"`
	MaxIdleConns    int           `yaml:"max_idle_conns" default:"5"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" default:"5m"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" default:"5m"`

	// 健康检查配置
	HealthCheckInterval time.Duration `yaml:"health_check_interval" default:"30s"`
	HealthCheckTimeout  time.Duration `yaml:"health_check_timeout" default:"3s"`

	// 监控配置
	MetricsEnabled bool `yaml:"metrics_enabled" default:"true"`

	// 日志配置
	LogLevel logger.LogLevel `yaml:"log_level" default:"error"`

	// 重试配置
	RetryAttempts int           `yaml:"retry_attempts" default:"3"`
	RetryDelay    time.Duration `yaml:"retry_delay" default:"100ms"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		MaxOpenConns:        25,
		MaxIdleConns:        5,
		ConnMaxLifetime:     5 * time.Minute,
		ConnMaxIdleTime:     5 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
		HealthCheckTimeout:  3 * time.Second,
		MetricsEnabled:      true,
		LogLevel:            logger.Error,
		RetryAttempts:       3,
		RetryDelay:          100 * time.Millisecond,
	}
}

// Manager 数据库管理器
type Manager struct {
	db      *gorm.DB
	sqlDB   *sql.DB
	config  *Config
	metrics *Metrics
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewManager 创建新的数据库管理器
func NewManager(dsn string, config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 打开数据库连接
	db, err := openDatabase(dsn, config)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 获取底层sql.DB用于连接池配置
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// 配置连接池
	if err := configureConnectionPool(sqlDB, config); err != nil {
		return nil, fmt.Errorf("failed to configure connection pool: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	manager := &Manager{
		db:      db,
		sqlDB:   sqlDB,
		config:  config,
		metrics: NewMetrics(),
		ctx:     ctx,
		cancel:  cancel,
	}

	// 启动健康检查
	go manager.healthChecker()

	return manager, nil
}

// openDatabase 打开数据库连接
func openDatabase(dsn string, config *Config) (*gorm.DB, error) {
	// 这里应该根据DSN自动选择合适的driver
	// 为了简化，我们使用现有的db.Open函数
	// 在实际生产环境中，应该根据DSN类型选择合适的driver

	// 使用现有的db包中的Open函数
	return db.Open(dsn)
}

// configureConnectionPool 配置数据库连接池
func configureConnectionPool(sqlDB *sql.DB, config *Config) error {
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	return nil
}

// GetDB 返回GORM实例
func (m *Manager) GetDB() *gorm.DB {
	return m.db
}

// GetSQLDB 返回底层sql.DB实例
func (m *Manager) GetSQLDB() *sql.DB {
	return m.sqlDB
}

// WithContext 返回带Context的GORM实例
func (m *Manager) WithContext(ctx context.Context) *gorm.DB {
	return m.db.WithContext(ctx)
}

// WithRetry 执行带重试的数据库操作
func (m *Manager) WithRetry(ctx context.Context, operation func(*gorm.DB) error) error {
	var lastErr error

	for attempt := 0; attempt <= m.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			// 等待后重试
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(m.config.RetryDelay):
			}

			// 记录重试指标
			if m.metrics != nil {
				m.metrics.IncrementRetryCount()
			}
		}

		// 执行操作
		if err := operation(m.db.WithContext(ctx)); err != nil {
			lastErr = err

			// 检查是否为可重试的错误
			if !isRetryableError(err) {
				break
			}

			continue
		}

		// 操作成功
		if m.metrics != nil {
			m.metrics.IncrementSuccessCount()
		}
		return nil
	}

	// 所有重试都失败
	if m.metrics != nil {
		m.metrics.IncrementFailureCount()
	}
	return lastErr
}

// isRetryableError 检查错误是否可重试
func isRetryableError(err error) bool {
	// 这里应该根据具体的错误类型判断是否可重试
	// 例如：连接超时、连接断开等
	return true // 简化实现
}

// healthChecker 健康检查器
func (m *Manager) healthChecker() {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
func (m *Manager) performHealthCheck() {
	ctx, cancel := context.WithTimeout(m.ctx, m.config.HealthCheckTimeout)
	defer cancel()

	// 执行简单的SQL查询来检查连接健康状态
	var result int
	err := m.sqlDB.QueryRowContext(ctx, "SELECT 1").Scan(&result)

	if err != nil {
		// 健康检查失败
		if m.metrics != nil {
			m.metrics.IncrementHealthCheckFailure()
		}
		return
	}

	// 健康检查成功
	if m.metrics != nil {
		m.metrics.IncrementHealthCheckSuccess()
	}
}

// GetStats 返回连接池统计信息
func (m *Manager) GetStats() *PoolStats {
	stats := m.sqlDB.Stats()

	return &PoolStats{
		MaxOpenConnections:      stats.MaxOpenConnections,
		OpenConnections:         stats.OpenConnections,
		InUse:                   stats.InUse,
		Idle:                    stats.Idle,
		WaitCount:               stats.WaitCount,
		WaitDuration:            stats.WaitDuration,
		MaxIdleClosed:           stats.MaxIdleClosed,
		MaxIdleTimeClosed:       stats.MaxIdleTimeClosed,
		MaxLifetimeClosed:       stats.MaxLifetimeClosed,
		LastHealthCheckSuccess:  m.metrics.LastHealthCheckSuccess(),
		LastHealthCheckFailure:  m.metrics.LastHealthCheckFailure(),
		HealthCheckSuccessCount: m.metrics.HealthCheckSuccessCount(),
		HealthCheckFailureCount: m.metrics.HealthCheckFailureCount(),
		RetryCount:              m.metrics.RetryCount(),
		SuccessCount:            m.metrics.SuccessCount(),
		FailureCount:            m.metrics.FailureCount(),
	}
}

// Close 关闭数据库连接
func (m *Manager) Close() error {
	m.cancel()
	return m.sqlDB.Close()
}

// Ping 检查数据库连接
func (m *Manager) Ping(ctx context.Context) error {
	dbCtx, cancel := ctxmanager.ForDatabase(ctx)
	defer cancel()

	return m.sqlDB.PingContext(dbCtx)
}
