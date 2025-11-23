package database

import (
	"sync/atomic"
	"time"
)

// PoolStats 连接池统计信息
type PoolStats struct {
	MaxOpenConnections         int           `json:"max_open_connections"`
	OpenConnections            int           `json:"open_connections"`
	InUse                      int           `json:"in_use"`
	Idle                       int           `json:"idle"`
	WaitCount                  int64         `json:"wait_count"`
	WaitDuration               time.Duration `json:"wait_duration"`
	MaxIdleClosed              int64         `json:"max_idle_closed"`
	MaxIdleTimeClosed          int64         `json:"max_idle_time_closed"`
	MaxLifetimeClosed          int64         `json:"max_lifetime_closed"`

	// 健康检查统计
	LastHealthCheckSuccess     time.Time `json:"last_health_check_success"`
	LastHealthCheckFailure     time.Time `json:"last_health_check_failure"`
	HealthCheckSuccessCount    int64     `json:"health_check_success_count"`
	HealthCheckFailureCount    int64     `json:"health_check_failure_count"`

	// 重试和操作统计
	RetryCount                 int64     `json:"retry_count"`
	SuccessCount               int64     `json:"success_count"`
	FailureCount               int64     `json:"failure_count"`
}

// Metrics 数据库指标
type Metrics struct {
	// 健康检查指标
	healthCheckSuccessCount int64
	healthCheckFailureCount int64
	lastHealthCheckSuccess  int64 // Unix timestamp
	lastHealthCheckFailure  int64 // Unix timestamp

	// 操作指标
	retryCount   int64
	successCount int64
	failureCount int64
}

// NewMetrics 创建新的指标收集器
func NewMetrics() *Metrics {
	return &Metrics{}
}

// IncrementHealthCheckSuccess 增加健康检查成功次数
func (m *Metrics) IncrementHealthCheckSuccess() {
	atomic.AddInt64(&m.healthCheckSuccessCount, 1)
	atomic.StoreInt64(&m.lastHealthCheckSuccess, time.Now().Unix())
}

// IncrementHealthCheckFailure 增加健康检查失败次数
func (m *Metrics) IncrementHealthCheckFailure() {
	atomic.AddInt64(&m.healthCheckFailureCount, 1)
	atomic.StoreInt64(&m.lastHealthCheckFailure, time.Now().Unix())
}

// HealthCheckSuccessCount 返回健康检查成功次数
func (m *Metrics) HealthCheckSuccessCount() int64 {
	return atomic.LoadInt64(&m.healthCheckSuccessCount)
}

// HealthCheckFailureCount 返回健康检查失败次数
func (m *Metrics) HealthCheckFailureCount() int64 {
	return atomic.LoadInt64(&m.healthCheckFailureCount)
}

// LastHealthCheckSuccess 返回最后一次健康检查成功时间
func (m *Metrics) LastHealthCheckSuccess() time.Time {
	timestamp := atomic.LoadInt64(&m.lastHealthCheckSuccess)
	if timestamp == 0 {
		return time.Time{}
	}
	return time.Unix(timestamp, 0)
}

// LastHealthCheckFailure 返回最后一次健康检查失败时间
func (m *Metrics) LastHealthCheckFailure() time.Time {
	timestamp := atomic.LoadInt64(&m.lastHealthCheckFailure)
	if timestamp == 0 {
		return time.Time{}
	}
	return time.Unix(timestamp, 0)
}

// IncrementRetryCount 增加重试次数
func (m *Metrics) IncrementRetryCount() {
	atomic.AddInt64(&m.retryCount, 1)
}

// IncrementSuccessCount 增加成功操作次数
func (m *Metrics) IncrementSuccessCount() {
	atomic.AddInt64(&m.successCount, 1)
}

// IncrementFailureCount 增加失败操作次数
func (m *Metrics) IncrementFailureCount() {
	atomic.AddInt64(&m.failureCount, 1)
}

// RetryCount 返回重试次数
func (m *Metrics) RetryCount() int64 {
	return atomic.LoadInt64(&m.retryCount)
}

// SuccessCount 返回成功操作次数
func (m *Metrics) SuccessCount() int64 {
	return atomic.LoadInt64(&m.successCount)
}

// FailureCount 返回失败操作次数
func (m *Metrics) FailureCount() int64 {
	return atomic.LoadInt64(&m.failureCount)
}

// Reset 重置所有指标
func (m *Metrics) Reset() {
	atomic.StoreInt64(&m.healthCheckSuccessCount, 0)
	atomic.StoreInt64(&m.healthCheckFailureCount, 0)
	atomic.StoreInt64(&m.lastHealthCheckSuccess, 0)
	atomic.StoreInt64(&m.lastHealthCheckFailure, 0)
	atomic.StoreInt64(&m.retryCount, 0)
	atomic.StoreInt64(&m.successCount, 0)
	atomic.StoreInt64(&m.failureCount, 0)
}