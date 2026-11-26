// Coverage targets (coverage final):
//  1. The service error branch of handler.GetRegistry: the request context
//     carries a username while svcCtx.AdminModel is uninitialized
//     -> RequireAnyPermission fails -> 500.
//     (The handler passes c.Request.Context(), so gin's c.Set does not take
//     effect; the username must be injected into the request ctx.)
//  2. The defensive branch of service.GetRegistry that skips sessions with a
//     nil / blank AgentID (UpsertAgent rejects a blank AgentID, so the internal
//     map must be written directly).
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
	// AUD-055：服务层不再做权限检查，AdminModel 缺位的极简上下文不再触发
	// 500——匿名与已登录调用者同样拿到公开只读目录。
	req = req.WithContext(context.WithValue(req.Context(), "username", "ghost"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestService_GetRegistry_SkipsBlankAndNilSessions(t *testing.T) {
	svcCtx := &svc.ServiceContext{RegistryStore: registry.NewStore()}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// A normal session plus nil / blank AgentID sessions written directly into
	// the internal map.
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

	resp, err := service.GetRegistry(context.TODO(), &RegistryRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Agents, 1)
	assert.Equal(t, "agent-ok", resp.Agents[0].AgentID)
}
