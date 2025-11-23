package context

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Manager 统一管理Context的创建和使用
type Manager struct {
	defaultTimeout time.Duration
	tracer         trace.Tracer
}

// NewManager 创建新的Context管理器
func NewManager(defaultTimeout time.Duration) *Manager {
	return &Manager{
		defaultTimeout: defaultTimeout,
		tracer:         otel.Tracer("croupier"),
	}
}

// DefaultManager 默认的Context管理器
var DefaultManager = NewManager(30 * time.Second)

// FromRequest 从HTTP请求创建Context
func (m *Manager) FromRequest(ctx context.Context, operation string) (context.Context, context.CancelFunc) {
	// 添加trace span
	ctx, span := m.tracer.Start(ctx, operation)

	// 添加超时控制
	timeoutCtx, cancel := context.WithTimeout(ctx, m.defaultTimeout)

	// 返回包装了span结束的cancel函数
	wrappedCancel := func() {
		span.End()
		cancel()
	}

	return timeoutCtx, wrappedCancel
}

// ForServiceCall 为服务间调用创建Context
func (m *Manager) ForServiceCall(ctx context.Context, operation string) (context.Context, context.CancelFunc) {
	// 添加trace span
	ctx, span := m.tracer.Start(ctx, operation)

	// 服务间调用较短超时
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

	// 返回包装了span结束的cancel函数
	wrappedCancel := func() {
		span.End()
		cancel()
	}

	return timeoutCtx, wrappedCancel
}

// ForBackground 为后台任务创建Context
func (m *Manager) ForBackground(operation string) (context.Context, context.CancelFunc) {
	ctx := context.Background()

	// 添加trace span
	ctx, span := m.tracer.Start(ctx, operation)

	// 后台任务较长超时
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)

	// 返回包装了span结束的cancel函数
	wrappedCancel := func() {
		span.End()
		cancel()
	}

	return timeoutCtx, wrappedCancel
}

// ForDatabase 为数据库操作创建Context
func (m *Manager) ForDatabase(ctx context.Context) (context.Context, context.CancelFunc) {
	// 数据库操作通常需要较短的超时
	return context.WithTimeout(ctx, 10*time.Second)
}

// ForCache 为缓存操作创建Context
func (m *Manager) ForCache(ctx context.Context) (context.Context, context.CancelFunc) {
	// 缓存操作需要很短的超时
	return context.WithTimeout(ctx, 1*time.Second)
}

// WithTimeout 创建带超时的Context
func (m *Manager) WithTimeout(parent context.Context, timeout time.Duration, operation string) (context.Context, context.CancelFunc) {
	// 添加trace span
	ctx, span := m.tracer.Start(parent, operation)

	// 设置超时
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)

	// 返回包装了span结束的cancel函数
	wrappedCancel := func() {
		span.End()
		cancel()
	}

	return timeoutCtx, wrappedCancel
}

// 便捷函数，使用默认Manager
var (
	FromRequest     = DefaultManager.FromRequest
	ForServiceCall  = DefaultManager.ForServiceCall
	ForBackground   = DefaultManager.ForBackground
	ForDatabase     = DefaultManager.ForDatabase
	ForCache        = DefaultManager.ForCache
	WithTimeout     = DefaultManager.WithTimeout
)