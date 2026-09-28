package node

// #24 聚合链路的错误路径补测（覆盖率巡检：handler ListAllCronJobs 60%、
// service 93.2%）。既有 cron_jobs_aggregate_test.go 覆盖聚合/缓存/离线
// 主链，本文件补：NodeModel 未初始化、节点表查询失败、handler 错误分支
// 与 handler 成功分支。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport"
)

// NodeModel 未初始化：service 直接报「节点表未初始化」而非 panic。
func TestListAllCronJobs_NilNodeModelErrors(t *testing.T) {
	svcUnderTest := NewService(&svc.ServiceContext{})
	reports, err := svcUnderTest.ListAllCronJobs(context.Background())
	require.Error(t, err)
	assert.Nil(t, reports)
	assert.Contains(t, err.Error(), "节点表未初始化")
}

// 节点表不存在（迁移未跑/库损坏）：List 错误原样上抛，不吞不 panic。
func TestListAllCronJobs_NodeListError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/node-bare.db"), &gorm.Config{})
	require.NoError(t, err)
	// 故意不 AutoMigrate：nodes 表不存在。
	svcUnderTest := NewService(&svc.ServiceContext{DB: db, NodeModel: model.NewNodeModel(db)})

	reports, err := svcUnderTest.ListAllCronJobs(context.Background())
	require.Error(t, err, "missing nodes table must surface as error")
	assert.Nil(t, reports)
}

// handler 错误分支：service 失败时经 response.Error 返回统一错误对象。
func TestHandler_ListAllCronJobs_ErrorPath(t *testing.T) {
	handler := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes/cron-jobs", "")
	handler.ListAllCronJobs(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.NotEmpty(t, body["error"], "unified error object expected, got %s", rec.Body.String())
}

// handler 成功分支：有节点时 200 + items/total（离线节点 ok=false 亦入列）。
func TestHandler_ListAllCronJobs_Success(t *testing.T) {
	svcUnderTest := newAggregateService(t, perNodeResolver{})
	db := svcUnderTest.svcCtx.DB
	require.NoError(t, db.Create(&model.Node{NodeID: "agent-offline", Name: "host-b", Status: "offline"}).Error)

	handler := NewHandler(svcUnderTest)
	ctx, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes/cron-jobs", "")
	handler.ListAllCronJobs(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Items []NodeCronJobsReport `json:"items"`
		Total int                  `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	assert.Equal(t, 1, body.Total)
	assert.Equal(t, "agent-offline", body.Items[0].NodeID)
	assert.False(t, body.Items[0].OK)
}

// failingCaller：隧道调用恒失败（采集错误上抛到报告）。
type failingCaller struct{}

func (failingCaller) Call(context.Context, uint32, []byte) (uint32, []byte, error) {
	return 0, nil, errors.New("tunnel down")
}

// 聚合报告的两级错误退化：会话表未初始化（deps 错误）与单节点采集失败
// （Call 错误）都进入 rep.Error 且 ok=false，不阻断其他节点聚合。
func TestListAllCronJobs_DepsAndCollectErrorsSurfaceInReport(t *testing.T) {
	db := newNodeTestDB(t)

	// deps 错误：AgentSessions 未注入。
	nilDeps := NewService(&svc.ServiceContext{DB: db, NodeModel: model.NewNodeModel(db)})
	require.NoError(t, db.Create(&model.Node{NodeID: "agent-1", Name: "host-1", Status: "online"}).Error)
	reports, err := nilDeps.ListAllCronJobs(context.Background())
	require.NoError(t, err)
	require.Len(t, reports, 1)
	assert.False(t, reports[0].OK)
	assert.Contains(t, reports[0].Error, "会话表未初始化")

	// 采集错误：resolver 在线但 Call 失败。
	withFail := NewService(&svc.ServiceContext{
		DB:            db,
		NodeModel:     model.NewNodeModel(db),
		AgentSessions: perNodeResolver{callers: map[string]transport.SessionCaller{"agent-1": failingCaller{}}},
	})
	reports, err = withFail.ListAllCronJobs(context.Background())
	require.NoError(t, err)
	require.Len(t, reports, 1)
	assert.False(t, reports[0].OK)
	assert.Contains(t, reports[0].Error, "tunnel down")
}
