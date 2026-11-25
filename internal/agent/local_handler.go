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

// instanceHealthyWindow is the heartbeat window for judging instance health
// (default matches production behavior; tests may inject a negative value to
// reach the stale degradation branch).
var instanceHealthyWindow = 30 * time.Second

// defaultDrainGrace is the default wait window for provider drain: after the
// drain flag is set, wait for in-flight requests to reach zero; once the
// window expires, finish and return (contract semantics: in-flight requests
// complete within the grace period; past the grace period we stop waiting).
// When the request carries retry_after_ms, that value takes precedence.
const defaultDrainGrace = 30 * time.Second

// drainPollInterval is the polling interval used while drain waits for
// in-flight requests to reach zero.
const drainPollInterval = 25 * time.Millisecond

// providerDrainingError explicitly reports "target provider is draining" back
// to the caller so upper layers can present it with retry-after semantics
// (wire contract: arriving requests are answered with draining/retry_after_ms).
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
	expectedGameID   string // gameId from agent config, used to validate SDK registration
	expectedEnv      string // env from agent config, used to validate SDK registration
	// providerCallTimeout is the default budget for synchronous Agent → Provider
	// calls; when request metadata["timeoutMs"] declares a smaller value, the
	// smaller one wins (Go deadline min semantics). The previous hardcoded 10s
	// was inverted relative to the Server dispatch layer's 15s.
	providerCallTimeout time.Duration
	// Provider handshake auth (CR-FIX-11): when requireProviderAuth=true,
	// ProviderConnectRequest.metadata["authToken"] must equal providerAuthToken,
	// otherwise the handshake is rejected (shared by both connection paths).
	providerAuthRequired bool
	providerAuthToken    string
	mu                   sync.RWMutex
}
