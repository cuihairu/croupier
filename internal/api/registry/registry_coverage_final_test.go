// 覆盖目标（coverage final）：
//  1. handler.GetRegistry 的 service 错误分支：请求 context 携带 username
//     且 svcCtx.AdminModel 未初始化 → RequireAnyPermission 失败 → 500。
//     （handler 传入 c.Request.Context()，gin 的 c.Set 不生效，须注入 request ctx。）
//  2. service.GetRegistry 跳过 nil / 空白 AgentID 会话的防御分支
//     （UpsertAgent 拒绝空 AgentID，需直接写入内部 map）。
package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_GetRegistry_ServiceError_UnauthenticatedAdminModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{RegistryStore: registry.NewStore()}
	handler := NewHandler(NewService(svcCtx))

	router := gin.New()
	router.POST("/registry", handler.GetRegistry)

	req := httptest.NewRequest(http.MethodPost, "/registry", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	// handler 读取 c.Request.Context()，必须把 username 注入 request context。
	req = req.WithContext(context.WithValue(req.Context(), "username", "ghost"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestService_GetRegistry_SkipsBlankAndNilSessions(t *testing.T) {
	svcCtx := &svc.ServiceContext{RegistryStore: registry.NewStore()}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// 正常会话 + 直接写入内部 map 的 nil / 空白 AgentID 会话。
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-ok",
		GameID:    "game1",
		Env:       "dev",
		Addr:      "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"f.ok": {Enabled: true}},
	}))
	store.Mu().Lock()
	store.AgentsUnsafe()["nil-entry"] = nil
	store.AgentsUnsafe()["blank-entry"] = &registry.AgentSession{
		AgentID:  "   ",
		GameID:   "gameX",
		Env:      "dev",
		ExpireAt: time.Now().Add(5 * time.Minute),
	}
	store.Mu().Unlock()

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Agents, 1)
	assert.Equal(t, "agent-ok", resp.Agents[0].AgentID)
}
