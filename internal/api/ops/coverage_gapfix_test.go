package ops

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 查询类 handler 的 service 错误分支：底层查询链路（内存 store 读取）
// 恒返回 nil error，通过包级函数缝隙注入错误触达。每个子测试自恢复缝隙。
func TestHandlerQueryServiceErrorBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&Service{})

	run := func(method, target, body string, fn func(*gin.Context)) int {
		ctx, rec := newOpsTestContext(method, target, body)
		fn(ctx)
		return rec.Code
	}

	t.Run("OpsAgentsList", func(t *testing.T) {
		orig := opsAgentsListFn
		opsAgentsListFn = func(ctx context.Context, s *svc.ServiceContext, req *OpsAgentsListRequest) (*OpsAgentsListResponse, error) {
			return nil, errors.New("injected")
		}
		t.Cleanup(func() { opsAgentsListFn = orig })
		assert.GreaterOrEqual(t, run(http.MethodGet, "/api/v1/ops/agents", "", h.OpsAgentsList), http.StatusBadRequest)
	})

	t.Run("OpsAgentMetrics", func(t *testing.T) {
		orig := opsAgentMetricsFn
		opsAgentMetricsFn = func(ctx context.Context, s *svc.ServiceContext, req *OpsAgentMetricsRequest) (*OpsAgentMetricsResponse, error) {
			return nil, errors.New("injected")
		}
		t.Cleanup(func() { opsAgentMetricsFn = orig })
		assert.GreaterOrEqual(t, run(http.MethodGet, "/api/v1/ops/agents/a/metrics?agentId=a", "", h.OpsAgentMetrics), http.StatusBadRequest)
	})

	t.Run("OpsAgentProcesses", func(t *testing.T) {
		orig := opsAgentProcessesFn
		opsAgentProcessesFn = func(ctx context.Context, s *svc.ServiceContext, req *OpsAgentProcessesRequest) (*OpsAgentProcessesResponse, error) {
			return nil, errors.New("injected")
		}
		t.Cleanup(func() { opsAgentProcessesFn = orig })
		assert.GreaterOrEqual(t, run(http.MethodGet, "/api/v1/ops/agents/a/processes", "", h.OpsAgentProcesses), http.StatusBadRequest)
	})

	t.Run("OpsNodes", func(t *testing.T) {
		orig := opsNodesFn
		opsNodesFn = func(ctx context.Context, s *svc.ServiceContext, req *OpsNodesRequest) (*OpsNodesResponse, error) {
			return nil, errors.New("injected")
		}
		t.Cleanup(func() { opsNodesFn = orig })
		assert.GreaterOrEqual(t, run(http.MethodGet, "/api/v1/ops/nodes", "", h.OpsNodes), http.StatusBadRequest)
	})

	t.Run("OpsNodeCommands", func(t *testing.T) {
		orig := opsNodeCommandsFn
		opsNodeCommandsFn = func(ctx context.Context, s *svc.ServiceContext, req *OpsNodeCommandsRequest) (*OpsNodeCommandsResponse, error) {
			return nil, errors.New("injected")
		}
		t.Cleanup(func() { opsNodeCommandsFn = orig })
		assert.GreaterOrEqual(t, run(http.MethodPost, "/api/v1/ops/nodes/commands", `{"nodeId":"n1"}`, h.OpsNodeCommands), http.StatusBadRequest)
	})

	t.Run("OpsHealthGet", func(t *testing.T) {
		orig := opsHealthGetFn
		opsHealthGetFn = func(ctx context.Context, s *svc.ServiceContext, req *OpsHealthGetRequest) (*OpsHealthGetResponse, error) {
			return nil, errors.New("injected")
		}
		t.Cleanup(func() { opsHealthGetFn = orig })
		assert.GreaterOrEqual(t, run(http.MethodGet, "/api/v1/ops/health", "", h.OpsHealthGet), http.StatusBadRequest)
	})

	t.Run("OpsMaintenanceGet", func(t *testing.T) {
		orig := opsMaintenanceGetFn
		opsMaintenanceGetFn = func(ctx context.Context, s *svc.ServiceContext, req *OpsMaintenanceGetRequest) (*OpsMaintenanceGetResponse, error) {
			return nil, errors.New("injected")
		}
		t.Cleanup(func() { opsMaintenanceGetFn = orig })
		assert.GreaterOrEqual(t, run(http.MethodGet, "/api/v1/ops/maintenance", "", h.OpsMaintenanceGet), http.StatusBadRequest)
	})

	t.Run("OpsMetrics", func(t *testing.T) {
		orig := opsMetricsFn
		opsMetricsFn = func(ctx context.Context, s *svc.ServiceContext, req *OpsMetricsRequest) (*OpsMetricsResponse, error) {
			return nil, errors.New("injected")
		}
		t.Cleanup(func() { opsMetricsFn = orig })
		assert.GreaterOrEqual(t, run(http.MethodGet, "/api/v1/ops/metrics?gameId=tower", "", h.OpsMetrics), http.StatusBadRequest)
	})

	t.Run("OpsConfig", func(t *testing.T) {
		orig := opsConfigFn
		opsConfigFn = func(ctx context.Context, s *svc.ServiceContext, req *OpsConfigRequest) (*OpsConfigResponse, error) {
			return nil, errors.New("injected")
		}
		t.Cleanup(func() { opsConfigFn = orig })
		assert.GreaterOrEqual(t, run(http.MethodGet, "/api/v1/ops/config", "", h.OpsConfig), http.StatusBadRequest)
	})

	t.Run("OpsServices", func(t *testing.T) {
		orig := opsServicesFn
		opsServicesFn = func(ctx context.Context, s *svc.ServiceContext, req *OpsServicesRequest) (*OpsServicesResponse, error) {
			return nil, errors.New("injected")
		}
		t.Cleanup(func() { opsServicesFn = orig })
		assert.GreaterOrEqual(t, run(http.MethodGet, "/api/v1/ops/services", "", h.OpsServices), http.StatusBadRequest)
	})

	t.Run("OpsFunctions", func(t *testing.T) {
		orig := opsFunctionsFn
		opsFunctionsFn = func(ctx context.Context, s *svc.ServiceContext, req *OpsFunctionsRequest) (*OpsFunctionsResponse, error) {
			return nil, errors.New("injected")
		}
		t.Cleanup(func() { opsFunctionsFn = orig })
		assert.GreaterOrEqual(t, run(http.MethodGet, "/api/v1/ops/functions", "", h.OpsFunctions), http.StatusBadRequest)
	})

	t.Run("OpsMQ", func(t *testing.T) {
		orig := opsMQFn
		opsMQFn = func(ctx context.Context, s *svc.ServiceContext, req *OpsMQRequest) (*OpsMQResponse, error) {
			return nil, errors.New("injected")
		}
		t.Cleanup(func() { opsMQFn = orig })
		assert.GreaterOrEqual(t, run(http.MethodGet, "/api/v1/ops/mq", "", h.OpsMQ), http.StatusBadRequest)
	})

	t.Run("AgentMetricsHistory", func(t *testing.T) {
		orig := agentMetricsHistoryFn
		agentMetricsHistoryFn = func(ctx context.Context, s *svc.ServiceContext, req *AgentMetricsHistoryRequest) (*AgentMetricsHistoryResponse, error) {
			return nil, errors.New("injected")
		}
		t.Cleanup(func() { agentMetricsHistoryFn = orig })
		assert.GreaterOrEqual(t, run(http.MethodGet, "/api/v1/ops/agents/a/metrics/history?agentId=a", "", h.AgentMetricsHistory), http.StatusBadRequest)
	})
}

// opsServicesLegacyCompatible 的错误透传分支（同样经缝隙注入）。
func TestOpsServicesLegacyCompatibleErrorBranch(t *testing.T) {
	orig := opsServicesFn
	opsServicesFn = func(ctx context.Context, s *svc.ServiceContext, req *OpsServicesRequest) (*OpsServicesResponse, error) {
		return nil, errors.New("injected")
	}
	t.Cleanup(func() { opsServicesFn = orig })

	_, err := opsServicesLegacyCompatible(context.Background(), nil, &OpsServicesRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected")
}
