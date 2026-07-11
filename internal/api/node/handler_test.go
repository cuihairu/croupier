package node

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var nodeDBSeq uint64

func newNodeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("node_%d", atomic.AddUint64(&nodeDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func newNodeHandler(db *gorm.DB) *Handler {
	svcCtx := &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
	return NewHandler(NewService(svcCtx))
}

func newNodeRequest(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, rec
}

func assertNodeErrorShape(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
	assert.Contains(t, body, "message")
}

func seedNode(t *testing.T, db *gorm.DB, nodeID string) {
	t.Helper()
	node := &model.Node{
		NodeID: nodeID,
		Name:   "node-" + nodeID,
		Type:   "agent",
		Status: "active",
		IP:     "10.0.0.1",
		Port:   19090,
		Meta:   datatypes.JSONMap{"region": "us-east"},
	}
	require.NoError(t, db.WithContext(context.Background()).Create(node).Error)
}

func TestHandler_List_Empty_Success(t *testing.T) {
	handler := newNodeHandler(newNodeTestDB(t))

	ctx, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes", "")
	handler.List(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp NodesListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	// List shape must be a JSON array field, empty is acceptable.
	assert.NotNil(t, resp.Items)
}

func TestHandler_List_WithSeed_Success(t *testing.T) {
	db := newNodeTestDB(t)
	handler := newNodeHandler(db)
	seedNode(t, db, "agent-1")

	ctx, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes?type=agent&status=active", "")
	handler.List(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp NodesListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "agent-1", resp.Items[0].ID)
	assert.Equal(t, "agent", resp.Items[0].Type)
}

func TestHandler_GetMeta_Success(t *testing.T) {
	db := newNodeTestDB(t)
	handler := newNodeHandler(db)
	seedNode(t, db, "agent-2")

	ctx, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes/agent-2/meta", "")
	ctx.Params = gin.Params{{Key: "id", Value: "agent-2"}}
	handler.GetMeta(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp NodeMetaResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Meta)
}

func TestHandler_GetMeta_EmptyID_BadRequest(t *testing.T) {
	handler := newNodeHandler(newNodeTestDB(t))

	ctx, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes//meta", "")
	ctx.Params = gin.Params{{Key: "id", Value: ""}}
	handler.GetMeta(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assertNodeErrorShape(t, rec)
}

func TestHandler_GetMeta_UnknownNode_Error(t *testing.T) {
	handler := newNodeHandler(newNodeTestDB(t))

	ctx, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes/does-not-exist/meta", "")
	ctx.Params = gin.Params{{Key: "id", Value: "does-not-exist"}}
	handler.GetMeta(ctx)

	// Unknown node surfaces a model error (节点不存在), never 200.
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertNodeErrorShape(t, rec)
}

func TestHandler_UpdateMeta_Success(t *testing.T) {
	db := newNodeTestDB(t)
	handler := newNodeHandler(db)
	seedNode(t, db, "agent-3")

	ctx, rec := newNodeRequest(http.MethodPut, "/api/v1/nodes/agent-3/meta",
		`{"meta":{"region":"eu-west","version":"2"}}`)
	ctx.Params = gin.Params{{Key: "id", Value: "agent-3"}}
	handler.UpdateMeta(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp NodeMetaResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Meta)
}

func TestHandler_UpdateMeta_NotObject_BadRequest(t *testing.T) {
	db := newNodeTestDB(t)
	handler := newNodeHandler(db)
	seedNode(t, db, "agent-4")

	ctx, rec := newNodeRequest(http.MethodPut, "/api/v1/nodes/agent-4/meta", `{"meta":"not-an-object"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "agent-4"}}
	handler.UpdateMeta(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertNodeErrorShape(t, rec)
}

func TestHandler_Drain_Success(t *testing.T) {
	db := newNodeTestDB(t)
	handler := newNodeHandler(db)
	seedNode(t, db, "agent-5")

	ctx, rec := newNodeRequest(http.MethodPost, "/api/v1/nodes/agent-5/drain", `{"timeout":30}`)
	ctx.Params = gin.Params{{Key: "id", Value: "agent-5"}}
	handler.Drain(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestHandler_Undrain_Success(t *testing.T) {
	db := newNodeTestDB(t)
	handler := newNodeHandler(db)
	seedNode(t, db, "agent-6")

	ctx, rec := newNodeRequest(http.MethodPost, "/api/v1/nodes/agent-6/undrain", "")
	ctx.Params = gin.Params{{Key: "id", Value: "agent-6"}}
	handler.Undrain(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestHandler_Restart_UnknownNode_Error(t *testing.T) {
	handler := newNodeHandler(newNodeTestDB(t))

	ctx, rec := newNodeRequest(http.MethodPost, "/api/v1/nodes/ghost/restart", "")
	ctx.Params = gin.Params{{Key: "id", Value: "ghost"}}
	handler.Restart(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertNodeErrorShape(t, rec)
}

func TestHandler_ListCommands_Empty_Success(t *testing.T) {
	handler := newNodeHandler(newNodeTestDB(t))

	ctx, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes/commands", "")
	handler.ListCommands(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp NodeCommandsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Items)
}
