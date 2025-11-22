package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/zeromicro/go-zero/rest"
)

func RegisterHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/agent/register",
				Handler: AgentRegisterHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/agent/heartbeat",
				Handler: AgentHeartbeatHandler(serverCtx),
			},
		},
	)

	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/agent/:agent_id/health",
				Handler: AgentHealthHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/agent/:agent_id/metrics",
				Handler: AgentMetricsHandler(serverCtx),
			},
		},
	)

	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/function/register",
				Handler: FunctionRegisterHandler(serverCtx),
			},
		},
	)

	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/job/execute",
				Handler: JobExecuteHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/job/:job_id/status",
				Handler: JobStatusHandler(serverCtx),
			},
		},
	)
}