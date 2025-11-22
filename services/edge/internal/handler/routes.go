package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/zeromicro/go-zero/rest"
)

func RegisterHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/tunnel",
				Handler: TunnelCreateHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/tunnel/:tunnel_id/status",
				Handler: TunnelStatusHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/tunnels",
				Handler: TunnelListHandler(serverCtx),
			},
			{
				Method:  http.MethodDelete,
				Path:    "/api/v1/tunnel/:tunnel_id",
				Handler: TunnelCloseHandler(serverCtx),
			},
		},
	)

	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/edge/health/:request_id",
				Handler: EdgeHealthHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/edge/metrics",
				Handler: EdgeMetricsHandler(serverCtx),
			},
		},
	)
}