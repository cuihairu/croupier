package ops

import (
	"strings"

	"github.com/cuihairu/croupier/internal/common/requestbind"
	"github.com/gin-gonic/gin"
)

func bindOpsRequest(c *gin.Context, req interface{}) error {
	if c.Request.Method == "GET" {
		return requestbind.BindQueryCompat(c, req)
	}
	return c.ShouldBindJSON(req)
}

// processActionParams reads the path parameters (agentId / name) of the
// process-action surface. The path is the only source of that surface's input
// parameters — the panel sends a bodyless POST, so bindOpsRequest's
// ShouldBindJSON reports EOF (400) on the empty body, and the `uri` tag is
// not honored by JSON binding: neither path yields agentId/name (#59, the same
// binding gap on the POST side).
func processActionParams(c *gin.Context) (agentId, name string) {
	return strings.TrimSpace(c.Param("agentId")), strings.TrimSpace(c.Param("name"))
}

// Handler serves the Server-side ops REST surface (ops domain, Server role:
// DTO mapping and orchestration). Execution happens on the agent host in
// internal/app/agent/ops_server.go (gRPC OpsServer); this handler only
// resolves targets and forwards over the control tunnel. The two share the
// domain name but not a process or a store.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Agent operations handlers
