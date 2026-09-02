// 覆盖目标：node service/handler 的错误分支与参数校验路径
// （invalid nodeID、节点不存在、meta 非 map、存储故障、命令列表种子数据）。
package node

import (
	"context"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// nodeSvcCtxOf 构造仅含 NodeModel 的 ServiceContext。
func nodeSvcCtxOf(db *gorm.DB) *svc.ServiceContext {
	return &svc.ServiceContext{NodeModel: model.NewNodeModel(db)}
}

func TestService_List_StoreError(t *testing.T) {
	db := newNodeTestDB(t)
	require.NoError(t, db.Migrator().DropTable("nodes"))
	svc := NewService(nodeSvcCtxOf(db))

	_, err := svc.List(context.Background(), &NodesListRequest{})
	require.Error(t, err)
}

func TestService_GetMeta_InvalidNodeID(t *testing.T) {
	svc := NewService(nodeSvcCtxOf(newNodeTestDB(t)))
	_, err := svc.GetMeta(context.Background(), &NodeMetaRequest{ID: "bad id!"})
	require.Error(t, err)
}

func TestService_UpdateMeta_NotMap(t *testing.T) {
	db := newNodeTestDB(t)
	seedNode(t, db, "agent-x1")
	svc := NewService(nodeSvcCtxOf(db))

	_, err := svc.UpdateMeta(context.Background(), &NodeMetaUpdateRequest{ID: "agent-x1", Meta: "not-a-map"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "对象")
}

func TestService_UpdateMeta_UnknownNode(t *testing.T) {
	svc := NewService(nodeSvcCtxOf(newNodeTestDB(t)))
	_, err := svc.UpdateMeta(context.Background(), &NodeMetaUpdateRequest{ID: "ghost-node", Meta: map[string]interface{}{"a": 1}})
	require.Error(t, err)
}

func TestService_Actions_InvalidNodeID(t *testing.T) {
	svc := NewService(nodeSvcCtxOf(newNodeTestDB(t)))
	ctx := context.Background()
	require.Error(t, svc.Drain(ctx, &NodeDrainRequest{ID: "bad id!"}))
	require.Error(t, svc.Undrain(ctx, &NodeActionRequest{ID: "bad id!"}))
	require.Error(t, svc.Restart(ctx, &NodeActionRequest{ID: "bad id!"}))
}

func TestService_Actions_UnknownNode(t *testing.T) {
	svc := NewService(nodeSvcCtxOf(newNodeTestDB(t)))
	ctx := context.Background()
	require.Error(t, svc.Drain(ctx, &NodeDrainRequest{ID: "ghost"}))
	require.Error(t, svc.Undrain(ctx, &NodeActionRequest{ID: "ghost"}))
	require.Error(t, svc.Restart(ctx, &NodeActionRequest{ID: "ghost"}))
}

func TestService_Drain_TimeoutStatus(t *testing.T) {
	db := newNodeTestDB(t)
	seedNode(t, db, "agent-t1")
	svc := NewService(nodeSvcCtxOf(db))
	ctx := context.Background()

	require.NoError(t, svc.Drain(ctx, &NodeDrainRequest{ID: "agent-t1", Timeout: 45}))
	node, err := svc.svcCtx.NodeModel.FindByNodeID(ctx, "agent-t1")
	require.NoError(t, err)
	assert.Equal(t, "draining:45", node.Status)

	require.NoError(t, svc.Undrain(ctx, &NodeActionRequest{ID: "agent-t1"}))
	node, err = svc.svcCtx.NodeModel.FindByNodeID(ctx, "agent-t1")
	require.NoError(t, err)
	assert.Equal(t, "active", node.Status)

	require.NoError(t, svc.Restart(ctx, &NodeActionRequest{ID: "agent-t1"}))
	node, err = svc.svcCtx.NodeModel.FindByNodeID(ctx, "agent-t1")
	require.NoError(t, err)
	assert.Equal(t, "restarting", node.Status)
}

// 验证 Bug5 修复：Drain handler 现在绑定可选 body 的 timeout
func TestHandler_Drain_BindsTimeoutFromBody(t *testing.T) {
	db := newNodeTestDB(t)
	seedNode(t, db, "agent-tb")
	handler := newNodeHandler(db)

	ctx, rec := newNodeRequest(http.MethodPost, "/api/v1/nodes/agent-tb/drain", `{"timeout":33}`)
	ctx.Params = gin.Params{{Key: "id", Value: "agent-tb"}}
	handler.Drain(ctx)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	svc := NewService(nodeSvcCtxOf(db))
	node, err := svc.svcCtx.NodeModel.FindByNodeID(context.Background(), "agent-tb")
	require.NoError(t, err)
	assert.Equal(t, "draining:33", node.Status, "timeout 必须从 body 读取（历史被静默忽略）")
}

// 空 body 兼容：不带 body 的旧调用方仍可 drain（不带 timeout）
func TestHandler_Drain_EmptyBody_Compatible(t *testing.T) {
	db := newNodeTestDB(t)
	seedNode(t, db, "agent-te")
	handler := newNodeHandler(db)

	ctx, rec := newNodeRequest(http.MethodPost, "/api/v1/nodes/agent-te/drain", "")
	ctx.Params = gin.Params{{Key: "id", Value: "agent-te"}}
	handler.Drain(ctx)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	svc := NewService(nodeSvcCtxOf(db))
	node, err := svc.svcCtx.NodeModel.FindByNodeID(context.Background(), "agent-te")
	require.NoError(t, err)
	assert.Equal(t, "draining", node.Status)
}

func TestService_ListCommands_WithSeed_StoreError(t *testing.T) {
	db := newNodeTestDB(t)
	require.NoError(t, db.Create(&model.NodeCommand{Name: "drain", Description: "Drain node"}).Error)
	svc := NewService(nodeSvcCtxOf(db))

	resp, err := svc.ListCommands(context.Background(), &NodeCommandsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "drain", resp.Items[0].Name)

	require.NoError(t, db.Migrator().DropTable("node_commands"))
	_, err = svc.ListCommands(context.Background(), &NodeCommandsRequest{})
	require.Error(t, err)
}

// handler 层：service 错误与 bind 失败分支
func TestHandler_List_ServiceError(t *testing.T) {
	db := newNodeTestDB(t)
	require.NoError(t, db.Migrator().DropTable("nodes"))
	handler := newNodeHandler(db)

	ctx, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes", "")
	handler.List(ctx)
	assert.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
}

func TestHandler_ListCommands_ServiceError(t *testing.T) {
	db := newNodeTestDB(t)
	require.NoError(t, db.Migrator().DropTable("node_commands"))
	handler := newNodeHandler(db)

	ctx, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes/commands", "")
	handler.ListCommands(ctx)
	assert.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
}

func TestHandler_UpdateMeta_MalformedJSON(t *testing.T) {
	db := newNodeTestDB(t)
	seedNode(t, db, "agent-bad")
	handler := newNodeHandler(db)

	ctx, rec := newNodeRequest(http.MethodPut, "/api/v1/nodes/agent-bad/meta", `{not-json`)
	ctx.Params = gin.Params{{Key: "id", Value: "agent-bad"}}
	handler.UpdateMeta(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestHandler_Drain_UnknownNode_ServiceError(t *testing.T) {
	handler := newNodeHandler(newNodeTestDB(t))

	ctx, rec := newNodeRequest(http.MethodPost, "/api/v1/nodes/ghost/drain", "")
	ctx.Params = gin.Params{{Key: "id", Value: "ghost"}}
	handler.Drain(ctx)
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertNodeErrorShape(t, rec)
}

func TestHandler_Restart_ServiceError(t *testing.T) {
	handler := newNodeHandler(newNodeTestDB(t))

	ctx, rec := newNodeRequest(http.MethodPost, "/api/v1/nodes/ghost/restart", "")
	ctx.Params = gin.Params{{Key: "id", Value: "ghost"}}
	handler.Restart(ctx)
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
}
