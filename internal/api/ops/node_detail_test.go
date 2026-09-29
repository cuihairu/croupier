package ops

// GET /ops/nodes/:nodeId 单设备详情与 Node.version 出参（移动伴侣端设计
// §2.3 后端最小补集转正）：详情与列表同源、scope 头可见性与列表一致、
// 未命中按契约回 404、agent_sessions.Version 透出为 node.version。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedDetailAgent(store *registry.Store, id, gameID, version string) {
	now := time.Now()
	_ = store.UpsertAgent(&registry.AgentSession{
		AgentID:   id,
		GameID:    gameID,
		Env:       "prod",
		Addr:      "h:1",
		Version:   version,
		Labels:    map[string]string{"hostname": id + "-host"},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  now,
		ExpireAt:  now.Add(time.Hour),
	})
}

func TestListNodes_ExposesAgentVersion(t *testing.T) {
	store := registry.NewStore()
	seedDetailAgent(store, "a1", "g1", "1.4.2")
	svcCtx := &svc.ServiceContext{RegistryStore: store}

	nodes := listNodes(context.Background(), svcCtx, "", "", "")
	require.Len(t, nodes, 1)
	assert.Equal(t, "1.4.2", nodes[0].Version)
}

func TestOpsNodeDetail_FoundSameSourceAsList(t *testing.T) {
	store := registry.NewStore()
	seedDetailAgent(store, "a1", "g1", "1.4.2")
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewService(svcCtx)

	resp, err := s.OpsNodeDetail(context.Background(), &OpsNodeDetailRequest{NodeID: "a1"})
	require.NoError(t, err)
	assert.Equal(t, "a1", resp.Node.Id)
	assert.Equal(t, "1.4.2", resp.Node.Version)
	assert.Equal(t, "g1", resp.Node.GameId)
	assert.Equal(t, "a1-host", resp.Node.Hostname)
	assert.Equal(t, "active", resp.Node.Status)
}

func TestOpsNodeDetail_Errors(t *testing.T) {
	store := registry.NewStore()
	seedDetailAgent(store, "a1", "g1", "1.4.2")
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	s := NewService(svcCtx)

	// 空 nodeId → 400
	_, err := s.OpsNodeDetail(context.Background(), &OpsNodeDetailRequest{})
	require.Error(t, err)
	var bad *errorx.CodeError
	require.ErrorAs(t, err, &bad)
	assert.Equal(t, http.StatusBadRequest, bad.Code)

	// 未知 id → 404
	_, err = s.OpsNodeDetail(context.Background(), &OpsNodeDetailRequest{NodeID: "nope"})
	require.Error(t, err)
	var notFound *errorx.CodeError
	require.True(t, errors.As(err, &notFound))
	assert.Equal(t, http.StatusNotFound, notFound.Code)

	// scope 头（X-Game-ID 注入的 GameScope）决定可见性：g2 看不到 g1 的节点
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g2", Env: "prod"})
	_, err = s.OpsNodeDetail(ctx, &OpsNodeDetailRequest{NodeID: "a1"})
	require.Error(t, err)
	require.ErrorAs(t, err, &notFound)

	// 同 scope 命中
	ctx = svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g1", Env: "prod"})
	resp, err := s.OpsNodeDetail(ctx, &OpsNodeDetailRequest{NodeID: "a1"})
	require.NoError(t, err)
	assert.Equal(t, "a1", resp.Node.Id)
}

func TestNodeDetailHandlerRoutes(t *testing.T) {
	store := registry.NewStore()
	seedDetailAgent(store, "a1", "g1", "1.4.2")
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))

	gin.SetMode(gin.TestMode)
	// 与生产路由表同构注册：静态段 /nodes/commands 与参数段 /nodes/:nodeId
	// 共存（沿用既有 :nodeId/meta 注册形态），证明路由树不冲突。
	r := gin.New()
	r.GET("/nodes/commands", h.NodeCommands)
	r.GET("/nodes/:nodeId", h.NodeDetail)
	r.GET("/nodes/:nodeId/meta", h.NodeMeta)

	// 命中 → 200 + node.version 出参
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nodes/a1", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Node struct {
			Id      string `json:"id"`
			Version string `json:"version"`
			Status  string `json:"status"`
		} `json:"node"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "a1", out.Node.Id)
	assert.Equal(t, "1.4.2", out.Node.Version)
	assert.Equal(t, "active", out.Node.Status)

	// 未命中 → 404 统一错误对象
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nodes/nope", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not_found")
}
