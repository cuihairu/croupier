package ops

// GET /ops/nodes/:nodeId single-device detail and the Node.version output
// field (promoted from the mobile companion design §2.3 minimal backend
// supplement): the detail shares its source with the list, scope-header
// visibility matches the list, a miss returns 404 per the contract, and
// agent_sessions.Version is exposed as node.version.

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

func TestListNodesFiltersAgentRole(t *testing.T) {
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "capture-a", GameID: "g1", Env: "prod", Addr: "h:3",
		Labels: map[string]string{"role": "capture"}, Functions: map[string]registry.FunctionMeta{},
		LastSeen: time.Now(), ExpireAt: time.Now().Add(time.Hour),
	}))
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "capture-b", GameID: "g1", Env: "prod", Addr: "h:2",
		Labels: map[string]string{"role": "capture"}, Functions: map[string]registry.FunctionMeta{},
		LastSeen: time.Now(), ExpireAt: time.Now().Add(time.Hour),
	}))
	svcCtx := &svc.ServiceContext{RegistryStore: store}

	nodes := listNodes(context.Background(), svcCtx, "g1", "prod", "capture")
	require.Len(t, nodes, 2)
	for _, node := range nodes {
		assert.Equal(t, "capture", node.Labels["role"])
	}
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

	// empty nodeId → 400
	_, err := s.OpsNodeDetail(context.Background(), &OpsNodeDetailRequest{})
	require.Error(t, err)
	var bad *errorx.CodeError
	require.ErrorAs(t, err, &bad)
	assert.Equal(t, http.StatusBadRequest, bad.Code)

	// unknown id → 404
	_, err = s.OpsNodeDetail(context.Background(), &OpsNodeDetailRequest{NodeID: "nope"})
	require.Error(t, err)
	var notFound *errorx.CodeError
	require.True(t, errors.As(err, &notFound))
	assert.Equal(t, http.StatusNotFound, notFound.Code)

	// the scope header (the GameScope injected by X-Game-ID) determines visibility: g2 cannot see g1's node
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g2", Env: "prod"})
	_, err = s.OpsNodeDetail(ctx, &OpsNodeDetailRequest{NodeID: "a1"})
	require.Error(t, err)
	require.ErrorAs(t, err, &notFound)

	// hit within the same scope
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
	// registered isomorphic to the production route table: the static segment
	// /nodes/commands and the parameter segment /nodes/:nodeId coexist (following
	// the existing :nodeId/meta registration shape), proving the route tree does
	// not conflict.
	r := gin.New()
	r.GET("/nodes/commands", h.NodeCommands)
	r.GET("/nodes/:nodeId", h.NodeDetail)
	r.GET("/nodes/:nodeId/meta", h.NodeMeta)

	// hit → 200 + node.version output field
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

	// miss → unified 404 error object
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nodes/nope", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not_found")
}
