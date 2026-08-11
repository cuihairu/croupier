package node

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Handler: Commands alias ---

func TestHandler_Commands_DelegatesToHandler_ListCommands(t *testing.T) {
	handler := newNodeHandler(newNodeTestDB(t))

	ctx, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes/commands", "")
	handler.Commands(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp NodeCommandsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Items)
}

// --- Handler: Drain error paths ---

func TestHandler_Drain_UnknownNode_Error(t *testing.T) {
	handler := newNodeHandler(newNodeTestDB(t))

	ctx, rec := newNodeRequest(http.MethodPost, "/api/v1/nodes/ghost/drain", "")
	ctx.Params = gin.Params{{Key: "id", Value: "ghost"}}
	handler.Drain(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertNodeErrorShape(t, rec)
}

func TestHandler_Drain_EmptyID_Error(t *testing.T) {
	handler := newNodeHandler(newNodeTestDB(t))

	ctx, rec := newNodeRequest(http.MethodPost, "/api/v1/nodes//drain", "")
	ctx.Params = gin.Params{{Key: "id", Value: ""}}
	handler.Drain(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assertNodeErrorShape(t, rec)
}

func TestHandler_Drain_NoTimeout_Success(t *testing.T) {
	db := newNodeTestDB(t)
	handler := newNodeHandler(db)
	seedNode(t, db, "agent-drain-notimeout")

	ctx, rec := newNodeRequest(http.MethodPost, "/api/v1/nodes/agent-drain-notimeout/drain", "")
	ctx.Params = gin.Params{{Key: "id", Value: "agent-drain-notimeout"}}
	handler.Drain(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// --- Handler: Undrain error paths ---

func TestHandler_Undrain_UnknownNode_Error(t *testing.T) {
	handler := newNodeHandler(newNodeTestDB(t))

	ctx, rec := newNodeRequest(http.MethodPost, "/api/v1/nodes/ghost/undrain", "")
	ctx.Params = gin.Params{{Key: "id", Value: "ghost"}}
	handler.Undrain(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertNodeErrorShape(t, rec)
}

func TestHandler_Undrain_EmptyID_Error(t *testing.T) {
	handler := newNodeHandler(newNodeTestDB(t))

	ctx, rec := newNodeRequest(http.MethodPost, "/api/v1/nodes//undrain", "")
	ctx.Params = gin.Params{{Key: "id", Value: ""}}
	handler.Undrain(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assertNodeErrorShape(t, rec)
}

// --- Handler: Restart success + error paths ---

func TestHandler_Restart_Success(t *testing.T) {
	db := newNodeTestDB(t)
	handler := newNodeHandler(db)
	seedNode(t, db, "agent-restart")

	ctx, rec := newNodeRequest(http.MethodPost, "/api/v1/nodes/agent-restart/restart", "")
	ctx.Params = gin.Params{{Key: "id", Value: "agent-restart"}}
	handler.Restart(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestHandler_Restart_EmptyID_Error(t *testing.T) {
	handler := newNodeHandler(newNodeTestDB(t))

	ctx, rec := newNodeRequest(http.MethodPost, "/api/v1/nodes//restart", "")
	ctx.Params = gin.Params{{Key: "id", Value: ""}}
	handler.Restart(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assertNodeErrorShape(t, rec)
}

// --- Handler: UpdateMeta error paths ---

func TestHandler_UpdateMeta_EmptyID_Error(t *testing.T) {
	handler := newNodeHandler(newNodeTestDB(t))

	ctx, rec := newNodeRequest(http.MethodPut, "/api/v1/nodes//meta",
		`{"meta":{"key":"val"}}`)
	ctx.Params = gin.Params{{Key: "id", Value: ""}}
	handler.UpdateMeta(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assertNodeErrorShape(t, rec)
}

func TestHandler_UpdateMeta_InvalidJSON_Error(t *testing.T) {
	db := newNodeTestDB(t)
	handler := newNodeHandler(db)
	seedNode(t, db, "agent-update-badjson")

	ctx, rec := newNodeRequest(http.MethodPut, "/api/v1/nodes/agent-update-badjson/meta",
		`{invalid json}`)
	ctx.Params = gin.Params{{Key: "id", Value: "agent-update-badjson"}}
	handler.UpdateMeta(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertNodeErrorShape(t, rec)
}

// --- Service: error paths ---

func TestService_List_Empty(t *testing.T) {
	db := newNodeTestDB(t)
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	resp, err := s.List(t.Context(), &NodesListRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

func TestService_List_WithTypeFilter(t *testing.T) {
	db := newNodeTestDB(t)
	seedNode(t, db, "s-filter")
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	resp, err := s.List(t.Context(), &NodesListRequest{Type: "agent"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "agent", resp.Items[0].Type)
}

func TestService_GetMeta_InvalidID(t *testing.T) {
	db := newNodeTestDB(t)
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	_, err := s.GetMeta(t.Context(), &NodeMetaRequest{ID: ""})
	require.Error(t, err)
}

func TestService_UpdateMeta_NotObject(t *testing.T) {
	db := newNodeTestDB(t)
	seedNode(t, db, "s-upd-meta")
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	_, err := s.UpdateMeta(t.Context(), &NodeMetaUpdateRequest{
		ID:   "s-upd-meta",
		Meta: "not-an-object",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "meta 必须是对象")
}

func TestService_UpdateMeta_InvalidID(t *testing.T) {
	db := newNodeTestDB(t)
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	_, err := s.UpdateMeta(t.Context(), &NodeMetaUpdateRequest{
		ID:   "",
		Meta: map[string]interface{}{"key": "val"},
	})
	require.Error(t, err)
}

func TestService_Drain_InvalidID(t *testing.T) {
	db := newNodeTestDB(t)
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	err := s.Drain(t.Context(), &NodeDrainRequest{ID: ""})
	require.Error(t, err)
}

func TestService_Drain_NotFound(t *testing.T) {
	db := newNodeTestDB(t)
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	err := s.Drain(t.Context(), &NodeDrainRequest{ID: "nonexistent"})
	require.Error(t, err)
}

func TestService_Drain_WithTimeout(t *testing.T) {
	db := newNodeTestDB(t)
	seedNode(t, db, "s-drain-timeout")
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	err := s.Drain(t.Context(), &NodeDrainRequest{ID: "s-drain-timeout", Timeout: 30})
	require.NoError(t, err)
}

func TestService_Drain_NoTimeout(t *testing.T) {
	db := newNodeTestDB(t)
	seedNode(t, db, "s-drain-notimeout")
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	err := s.Drain(t.Context(), &NodeDrainRequest{ID: "s-drain-notimeout"})
	require.NoError(t, err)
}

func TestService_Undrain_InvalidID(t *testing.T) {
	db := newNodeTestDB(t)
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	err := s.Undrain(t.Context(), &NodeActionRequest{ID: ""})
	require.Error(t, err)
}

func TestService_Undrain_NotFound(t *testing.T) {
	db := newNodeTestDB(t)
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	err := s.Undrain(t.Context(), &NodeActionRequest{ID: "nonexistent"})
	require.Error(t, err)
}

func TestService_Undrain_Success(t *testing.T) {
	db := newNodeTestDB(t)
	seedNode(t, db, "s-undrain")
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	err := s.Undrain(t.Context(), &NodeActionRequest{ID: "s-undrain"})
	require.NoError(t, err)
}

func TestService_Restart_InvalidID(t *testing.T) {
	db := newNodeTestDB(t)
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	err := s.Restart(t.Context(), &NodeActionRequest{ID: ""})
	require.Error(t, err)
}

func TestService_Restart_NotFound(t *testing.T) {
	db := newNodeTestDB(t)
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	err := s.Restart(t.Context(), &NodeActionRequest{ID: "nonexistent"})
	require.Error(t, err)
}

func TestService_Restart_Success(t *testing.T) {
	db := newNodeTestDB(t)
	seedNode(t, db, "s-restart")
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	err := s.Restart(t.Context(), &NodeActionRequest{ID: "s-restart"})
	require.NoError(t, err)
}

func TestService_ListCommands_Empty(t *testing.T) {
	db := newNodeTestDB(t)
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	s := NewService(svcCtx)

	resp, err := s.ListCommands(t.Context(), &NodeCommandsRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp.Items)
	assert.Empty(t, resp.Items)
}

// --- DTOs ---

func TestDTO_Node(t *testing.T) {
	n := Node{
		ID:     "test-id",
		Name:   "test-node",
		Type:   "agent",
		Status: "active",
		IP:     "10.0.0.1",
		Port:   19090,
	}
	assert.Equal(t, "test-id", n.ID)
	assert.Equal(t, 19090, n.Port)
}

func TestDTO_NodeDrainRequest(t *testing.T) {
	req := NodeDrainRequest{ID: "n1", Timeout: 60}
	assert.Equal(t, "n1", req.ID)
	assert.Equal(t, 60, req.Timeout)
}

func TestDTO_NodeMetaUpdateRequest(t *testing.T) {
	req := NodeMetaUpdateRequest{ID: "n1", Meta: map[string]string{"k": "v"}}
	assert.Equal(t, "n1", req.ID)
}

func TestDTO_NodesListRequest(t *testing.T) {
	req := NodesListRequest{Type: "server", Status: "active"}
	assert.Equal(t, "server", req.Type)
	assert.Equal(t, "active", req.Status)
}

func TestDTO_NodeCommandsResponse(t *testing.T) {
	resp := NodeCommandsResponse{Items: []NodeCommand{
		{Name: "cmd1", Description: "desc1"},
	}}
	assert.Len(t, resp.Items, 1)
}

func TestDTO_ObjectsData(t *testing.T) {
	d := ObjectsData{
		Objects: []ObjectInfo{
			{Key: "k1", Size: 100},
		},
		Prefixes:    []string{"p1"},
		IsTruncated: true,
		NextMarker:  "m1",
	}
	assert.True(t, d.IsTruncated)
}
