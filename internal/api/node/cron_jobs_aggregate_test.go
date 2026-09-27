package node

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// perNodeResolver：按节点 ID 解析调用器（缺失即离线），驱动聚合的混合在线场景
type perNodeResolver struct {
	callers map[string]transport.SessionCaller
}

func (f perNodeResolver) ResolveSessionCaller(nodeID string) (transport.SessionCaller, bool) {
	c, ok := f.callers[nodeID]
	return c, ok
}

// countingCaller：记录被调用次数（缓存生效断言用）
type countingCaller struct {
	respBody []byte
	calls    atomic.Int32
}

func (c *countingCaller) Call(ctx context.Context, msgID uint32, body []byte) (uint32, []byte, error) {
	c.calls.Add(1)
	return msgID + 1, c.respBody, nil
}

func newAggregateService(t *testing.T, resolver svc.AgentSessionResolver) *Service {
	t.Helper()
	db := newNodeTestDB(t)
	return NewService(&svc.ServiceContext{
		DB:            db,
		NodeModel:     model.NewNodeModel(db),
		AgentSessions: resolver,
	})
}

func cronJobsPayload(jobs string) []byte {
	return []byte(`{"jobs":` + jobs + `}`)
}

// #24：全量聚合——在线节点 ok=true 且任务归位；离线节点 ok=false 带原因
func TestListAllCronJobs_AggregatesPerNode(t *testing.T) {
	online := &countingCaller{respBody: cronJobsPayload(
		`[{"schedule":"*/5 * * * *","command":"/usr/bin/backup","user":"root","sourceFile":"/etc/crontab","enabled":true}]`)}
	resolver := perNodeResolver{callers: map[string]transport.SessionCaller{
		"agent-online": online,
	}}

	svcUnderTest := newAggregateService(t, resolver)
	db := svcUnderTest.svcCtx.DB
	require.NoError(t, db.Create(&model.Node{NodeID: "agent-online", Name: "host-a", Status: "online"}).Error)
	require.NoError(t, db.Create(&model.Node{NodeID: "agent-offline", Name: "host-b", Status: "offline"}).Error)

	reports, err := svcUnderTest.ListAllCronJobs(context.Background())
	require.NoError(t, err)
	require.Len(t, reports, 2)

	byID := map[string]NodeCronJobsReport{}
	for _, r := range reports {
		byID[r.NodeID] = r
	}
	onlineRep := byID["agent-online"]
	assert.True(t, onlineRep.OK)
	assert.Equal(t, "host-a", onlineRep.NodeName)
	require.Len(t, onlineRep.Jobs, 1)
	assert.Equal(t, "/usr/bin/backup", onlineRep.Jobs[0].Command)

	offlineRep := byID["agent-offline"]
	assert.False(t, offlineRep.OK)
	assert.Contains(t, offlineRep.Error, "节点不在线")
	assert.Empty(t, offlineRep.Jobs)
}

func TestListAllCronJobs_EmptyNodeTable(t *testing.T) {
	svcUnderTest := newAggregateService(t, perNodeResolver{})
	reports, err := svcUnderTest.ListAllCronJobs(context.Background())
	require.NoError(t, err)
	assert.Empty(t, reports)
}

// #24：30s TTL 缓存——第二次调用不再打 agent（caller 调用计数不变）
func TestListAllCronJobs_CachesAggregatedReports(t *testing.T) {
	caller := &countingCaller{respBody: cronJobsPayload(`[]`)}
	resolver := perNodeResolver{callers: map[string]transport.SessionCaller{"agent-1": caller}}
	svcUnderTest := newAggregateService(t, resolver)
	require.NoError(t, svcUnderTest.svcCtx.DB.Create(&model.Node{NodeID: "agent-1", Name: "host-a", Status: "online"}).Error)

	_, err := svcUnderTest.ListAllCronJobs(context.Background())
	require.NoError(t, err)
	_, err = svcUnderTest.ListAllCronJobs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(1), caller.calls.Load(), "第二次聚合应命中缓存而非再次调用 agent")
}

// #24：handler 形态——GET /api/v1/nodes/cron-jobs 返回 {items,total}，
// 报告按节点归位且不可达节点以 error 标注
func TestHandler_ListAllCronJobs(t *testing.T) {
	online := &countingCaller{respBody: cronJobsPayload(
		`[{"schedule":"daily","command":"C:\\run.exe","user":"corp\\svc","sourceFile":"\\MyApp\\Nightly","enabled":true}]`)}
	resolver := perNodeResolver{callers: map[string]transport.SessionCaller{"agent-1": online}}
	svcUnderTest := newAggregateService(t, resolver)
	require.NoError(t, svcUnderTest.svcCtx.DB.Create(&model.Node{NodeID: "agent-1", Name: "win-host", Status: "online"}).Error)
	require.NoError(t, svcUnderTest.svcCtx.DB.Create(&model.Node{NodeID: "agent-2", Name: "down-host", Status: "offline"}).Error)

	handler := NewHandler(svcUnderTest)
	ctx, rec := newNodeRequest(http.MethodGet, "/api/v1/nodes/cron-jobs", "")
	handler.ListAllCronJobs(ctx)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Items []NodeCronJobsReport `json:"items"`
		Total int                  `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)

	byName := map[string]NodeCronJobsReport{}
	for _, r := range resp.Items {
		byName[r.NodeName] = r
	}
	winRep := byName["win-host"]
	assert.True(t, winRep.OK)
	require.Len(t, winRep.Jobs, 1)
	assert.Equal(t, `C:\run.exe`, winRep.Jobs[0].Command)

	assert.False(t, byName["down-host"].OK)
}
