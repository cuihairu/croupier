package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// GameTelemetryService 游戏遥测服务
// 提供高级API集成到Croupier Server/Agent架构中
type GameTelemetryService struct {
	provider *Provider
	logger   *slog.Logger
}

// NewGameTelemetryService 创建游戏遥测服务
func NewGameTelemetryService(config TelemetryConfig, logger *slog.Logger) (*GameTelemetryService, error) {
	ctx := context.Background()

	provider, err := NewProvider(ctx, config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry provider: %w", err)
	}

	return &GameTelemetryService{
		provider: provider,
		logger:   logger,
	}, nil
}

// === Croupier Function调用追踪 ===

// TrackFunctionCall 追踪Croupier Function调用
func (s *GameTelemetryService) TrackFunctionCall(ctx context.Context, req FunctionCallRequest) (context.Context, trace.Span) {
	tracer := s.provider.GameTracer.tracer

	ctx, span := tracer.Start(ctx, "function.call",
		trace.WithAttributes(
			attribute.String("function.id", req.FunctionID),
			attribute.String("function.version", req.Version),
			GameUserIDKey.String(req.UserID),
			GameSessionIDKey.String(req.SessionID),
			GameIDKey.String(req.GameID),
			attribute.String("function.env", req.Environment),
			attribute.String("function.agent_id", req.AgentID),
		),
	)

	// 发送Analytics事件
	if s.provider.Bridge != nil {
		s.provider.Bridge.SendEvent(ctx, "function.call", span, []attribute.KeyValue{
			attribute.String("function.id", req.FunctionID),
			attribute.String("function.version", req.Version),
			GameUserIDKey.String(req.UserID),
			GameSessionIDKey.String(req.SessionID),
			GameIDKey.String(req.GameID),
			attribute.String("function.env", req.Environment),
			attribute.String("function.agent_id", req.AgentID),
		})
	}

	return ctx, span
}

// CompleteFunctionCall 完成Function调用追踪
func (s *GameTelemetryService) CompleteFunctionCall(ctx context.Context, result FunctionCallResult) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.SetAttributes(
			attribute.Int64("function.duration_ms", result.DurationMs),
			attribute.Bool("function.success", result.Success),
			attribute.String("function.result", result.ResultType),
		)

		if result.Success {
			span.SetName("function.call.success")
		} else {
			span.SetName("function.call.error")
			span.SetAttributes(
				attribute.String("error.message", result.ErrorMessage),
				attribute.String("error.code", result.ErrorCode),
			)
		}

		// 发送Analytics事件
		if s.provider.Bridge != nil {
			eventType := "function.call.success"
			if !result.Success {
				eventType = "function.call.error"
			}

			attrs := []attribute.KeyValue{
				attribute.Int64("function.duration_ms", result.DurationMs),
				attribute.Bool("function.success", result.Success),
				attribute.String("function.result", result.ResultType),
			}

			if !result.Success {
				attrs = append(attrs,
					attribute.String("error.message", result.ErrorMessage),
					attribute.String("error.code", result.ErrorCode),
				)
			}

			s.provider.Bridge.SendEvent(ctx, eventType, span, attrs)
		}

		span.End()
	}
}

// === 权限验证追踪 ===

// TrackPermissionCheck 追踪权限检查
func (s *GameTelemetryService) TrackPermissionCheck(ctx context.Context, req PermissionCheckRequest) (context.Context, trace.Span) {
	tracer := s.provider.GameTracer.tracer

	ctx, span := tracer.Start(ctx, "permission.check",
		trace.WithAttributes(
			GameUserIDKey.String(req.UserID),
			attribute.String("permission.resource", req.Resource),
			attribute.String("permission.action", req.Action),
			attribute.String("permission.scope", req.Scope),
		),
	)

	return ctx, span
}

// CompletePermissionCheck 完成权限检查追踪
func (s *GameTelemetryService) CompletePermissionCheck(ctx context.Context, result PermissionCheckResult) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.SetAttributes(
			attribute.Bool("permission.granted", result.Granted),
			attribute.String("permission.reason", result.Reason),
			attribute.Int64("permission.duration_ms", result.DurationMs),
		)

		if result.Granted {
			span.SetName("permission.check.granted")
		} else {
			span.SetName("permission.check.denied")
		}

		// 发送Analytics事件
		if s.provider.Bridge != nil {
			eventType := "permission.check.granted"
			if !result.Granted {
				eventType = "permission.check.denied"
			}

			s.provider.Bridge.SendEvent(ctx, eventType, span, []attribute.KeyValue{
				attribute.Bool("permission.granted", result.Granted),
				attribute.String("permission.reason", result.Reason),
				attribute.Int64("permission.duration_ms", result.DurationMs),
			})
		}

		span.End()
	}
}

// === HTTP中间件集成 ===

// HTTPMiddleware 返回标准HTTP中间件用于HTTP请求追踪
func (s *GameTelemetryService) HTTPMiddleware(handler http.Handler) http.Handler {
	return otelhttp.NewHandler(handler, "croupier-http")
}

// === 游戏事件代理方法 ===

// StartUserSession 代理到GameTracer
func (s *GameTelemetryService) StartUserSession(ctx context.Context, req SessionStartRequest) (context.Context, trace.Span) {
	return s.provider.GameTracer.StartUserSession(ctx, req)
}

// EndUserSession 代理到GameTracer
func (s *GameTelemetryService) EndUserSession(ctx context.Context, req SessionEndRequest) {
	s.provider.GameTracer.EndUserSession(ctx, req)
}

// StartLevelPlaythrough 代理到GameTracer
func (s *GameTelemetryService) StartLevelPlaythrough(ctx context.Context, req LevelStartRequest) (context.Context, trace.Span) {
	return s.provider.GameTracer.StartLevelPlaythrough(ctx, req)
}

// CompleteLevelPlaythrough 代理到GameTracer
func (s *GameTelemetryService) CompleteLevelPlaythrough(ctx context.Context, result LevelCompleteRequest) {
	s.provider.GameTracer.CompleteLevelPlaythrough(ctx, result)
}

// TrackEconomyTransaction 代理到GameTracer
func (s *GameTelemetryService) TrackEconomyTransaction(ctx context.Context, transaction EconomyTransaction) {
	s.provider.GameTracer.TrackEconomyTransaction(ctx, transaction)
}

// === 健康检查 ===

// Health 健康检查
func (s *GameTelemetryService) Health(ctx context.Context) error {
	if s.provider.Bridge != nil {
		return s.provider.Bridge.Health(ctx)
	}
	return nil
}

// === 生命周期管理 ===

// Shutdown 优雅关闭
func (s *GameTelemetryService) Shutdown(ctx context.Context) error {
	return s.provider.Shutdown(ctx)
}

// === 请求/响应结构体 ===

// FunctionCallRequest Function调用请求
type FunctionCallRequest struct {
	FunctionID  string
	Version     string
	UserID      string
	SessionID   string
	GameID      string
	Environment string
	AgentID     string
	Parameters  map[string]interface{}
}

// FunctionCallResult Function调用结果
type FunctionCallResult struct {
	Success      bool
	DurationMs   int64
	ResultType   string
	ErrorMessage string
	ErrorCode    string
}

// PermissionCheckRequest 权限检查请求
type PermissionCheckRequest struct {
	UserID   string
	Resource string
	Action   string
	Scope    string
}

// PermissionCheckResult 权限检查结果
type PermissionCheckResult struct {
	Granted    bool
	Reason     string
	DurationMs int64
}