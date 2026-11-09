// Package agent provides business logic for handling local agent requests
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

// instanceHealthyWindow 判定实例健康的心跳窗口（与生产行为一致的默认值，
// 测试可注入负值覆盖以触达 stale 降级分支）。
var instanceHealthyWindow = 30 * time.Second

// defaultDrainGrace 是 provider drain 的默认等待窗：drain 标记后等待在途
// 请求归零，超窗即完成并返回（contract 语义：in-flight 在宽限内完成，
// 宽限过后不再等待）。请求携带 retry_after_ms 时以该值优先。
const defaultDrainGrace = 30 * time.Second

// drainPollInterval 是 drain 等待在途归零的轮询间隔。
const drainPollInterval = 25 * time.Millisecond

// providerDrainingError 把「目标 provider 正在 drain」显式回给调用方，供
// 上层按 retry-after 语义呈现（wire contract：arriving requests 应答
// draining/retry_after_ms）。
func providerDrainingError(serviceID string) error {
	return fmt.Errorf("provider %s is draining (retry after %dms)", serviceID, defaultDrainGrace.Milliseconds())
}

// ProviderManager is the interface for provider function calls
type ProviderManager interface {
	IsPlatformFunction(functionID string) bool
	Call(ctx context.Context, functionID string, request []byte) ([]byte, error)
}

// PluginFunctionManager dispatches plugin-adapted functions (todo.md:35):
// functions declared by external function adapter plugins. Declared like
// ProviderManager so the handler stays transport- and plugin-runtime
// agnostic; the App wires the real host, tests inject fakes.
type PluginFunctionManager interface {
	IsPluginFunction(functionID string) bool
	CallPluginFunction(ctx context.Context, functionID string, paramsJSON []byte) ([]byte, error)
}

// OpsServerWrapper wraps the OpsServer functionality
type OpsServerWrapper interface {
	GetSystemInfo(ctx context.Context, req *emptypb.Empty) (*opsv1.SystemInfo, error)
	ListProcesses(ctx context.Context, req *emptypb.Empty) (*opsv1.ListProcessesResponse, error)
	ReportMetrics(ctx context.Context, req *opsv1.MetricsReport) (*emptypb.Empty, error)
	RestartProcess(ctx context.Context, req *opsv1.RestartProcessRequest) (*opsv1.RestartProcessResponse, error)
	StopProcess(ctx context.Context, req *opsv1.StopProcessRequest) (*opsv1.StopProcessResponse, error)
	StartProcess(ctx context.Context, req *opsv1.StartProcessRequest) (*opsv1.StartProcessResponse, error)
	UpdateProcess(ctx context.Context, req *opsv1.UpdateProcessRequest) (*opsv1.UpdateProcessResponse, error)
	SetSchedulePause(ctx context.Context, req *opsv1.SchedulePauseRequest) (*opsv1.SchedulePauseResponse, error)
	ExecuteCommand(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error)
	PushFile(ctx context.Context, req *opsv1.PushFileRequest) (*opsv1.PushFileResponse, error)

	// System services (platform-specific, read-only)
	// These use JSON request/response to avoid circular dependencies
	ListServicesJSON(ctx context.Context, jsonReq []byte) ([]byte, error)
	GetServiceStatusJSON(ctx context.Context, jsonReq []byte) ([]byte, error)
	ListCronJobsJSON(ctx context.Context) ([]byte, error)

	// Supervisor event log download (agent-local rotating file, size-capped)
	GetSupervisorLog(ctx context.Context, req *opsv1.GetSupervisorLogRequest) (*opsv1.GetSupervisorLogResponse, error)
}

// LocalHandler contains the business logic for handling agent requests
// without any transport-specific dependencies.
type LocalHandler struct {
	store            *agentlocal.LocalStore
	tasks            *TaskRunner
	pm               ProviderManager // Use field name `pm` to avoid conflict with existing providerManager in app.go
	pluginFuncs      PluginFunctionManager
	opsServer        OpsServerWrapper
	reporter         TaskEventReporter
	providerSessions *ProviderSessionStore
	tlsCfg           *tlsutil.ClientTLSConfig
	logger           *slog.Logger
	configDir        string
	agentID          string
	expectedGameID   string // Agent 配置的 gameId，用于校验 SDK 注册
	expectedEnv      string // Agent 配置的 env，用于校验 SDK 注册
	// providerCallTimeout 是 Agent → Provider 同步调用的默认预算；
	// 请求 metadata["timeoutMs"] 声明更小值时取更小者（Go deadline
	// min 语义）。此前硬编码 10s 与 Server 派发层 15s 倒挂。
	providerCallTimeout time.Duration
	mu                  sync.RWMutex
}
