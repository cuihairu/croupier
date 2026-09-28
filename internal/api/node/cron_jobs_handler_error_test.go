package node

// ListAllCronJobs 错误分支补测（#24 聚合接口，巡检 handler 60%/service 93.2%）：
// handler 聚合失败走统一 response.Error；节点表未初始化显式报错；
// 在线节点 agent 调用失败时报告行以 error 标注而非整体失败。

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingCaller：Call 恒失败，驱动 collectCronJobs 错误分支
type failingCaller struct{}

func (failingCaller) Call(context.Context, uint32, []byte) (uint32, []byte, error) {
	return 0, nil, errors.New("agent connection refused")
}

func TestListAllCronJobs_Handler_ErrorBranch(t *testing.T) {
	db := newNodeTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	h := NewHandler(NewService(&svc.ServiceContext{
		DB:        db,
		NodeModel: model.NewNodeModel(db),
	}))

	c, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes/cron-jobs", "")
	h.ListAllCronJobs(c)
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

// 节点表未初始化 → 显式错误而非 panic/空列表
func TestListAllCronJobs_NilNodeModel(t *testing.T) {
	db := newNodeTestDB(t)
	svcUnderTest := NewService(&svc.ServiceContext{DB: db})

	_, err := svcUnderTest.ListAllCronJobs(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "节点表未初始化")
}

// 在线节点的 agent 调用失败 → 该节点报告行带 error 标注，聚合本身不失败
func TestListAllCronJobs_CollectError_Annotated(t *testing.T) {
	resolver := perNodeResolver{callers: map[string]transport.SessionCaller{
		"agent-bad": failingCaller{},
	}}
	svcUnderTest := newAggregateService(t, resolver)
	require.NoError(t, svcUnderTest.svcCtx.DB.Create(&model.Node{NodeID: "agent-bad", Name: "host-x", Status: "online"}).Error)

	reports, err := svcUnderTest.ListAllCronJobs(context.Background())
	require.NoError(t, err)
	require.Len(t, reports, 1)
	assert.False(t, reports[0].OK)
	assert.Contains(t, reports[0].Error, "agent connection refused")
}
