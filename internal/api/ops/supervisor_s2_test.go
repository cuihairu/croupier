// Supervisor S2 server 侧测试：事件环 API（增量/limit）、事件 DTO 映射、
// handler 路径参数兜底、日志下载端点（Content-Disposition / X-Truncated /
// 字节流直传）与错误路径。
package ops

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupervisorEventsFromMetricsStore(t *testing.T) {
	store := registry.NewMetricsStore()
	store.Add("a1", &opsv1.MetricsReport{AgentId: "a1", SupervisorEvents: []*opsv1.SupervisorEvent{
		{Seq: 1, TsUnix: time.Now().Unix() - 10, Process: "app", Event: "detect_down", ExitCode: 1},
		{Seq: 2, TsUnix: time.Now().Unix(), Process: "app", Event: "auto_restart", NewPid: 9, RestartCount: 1},
	}})
	svcCtx := &svc.ServiceContext{MetricsStore: store}

	// 全量。
	resp, err := opsAgentSupervisorEvents(context.Background(), svcCtx, &OpsAgentSupervisorEventsRequest{AgentID: "a1"})
	require.NoError(t, err)
	assert.Equal(t, "a1", resp.AgentID)
	require.Len(t, resp.Events, 2)
	assert.Equal(t, int64(2), resp.LatestSeq)
	assert.Equal(t, "auto_restart", resp.Events[1].Event)
	assert.NotEmpty(t, resp.Events[1].Ts)

	// sinceSeq 增量。
	resp, err = opsAgentSupervisorEvents(context.Background(), svcCtx, &OpsAgentSupervisorEventsRequest{AgentID: "a1", SinceSeq: 1})
	require.NoError(t, err)
	require.Len(t, resp.Events, 1)
	assert.Equal(t, int64(2), resp.Events[0].Seq)

	// limit 截取最新。
	resp, err = opsAgentSupervisorEvents(context.Background(), svcCtx, &OpsAgentSupervisorEventsRequest{AgentID: "a1", Limit: 1})
	require.NoError(t, err)
	require.Len(t, resp.Events, 1)
	assert.Equal(t, int64(2), resp.Events[0].Seq)

	// 无上报 agent：空集非错误。
	resp, err = opsAgentSupervisorEvents(context.Background(), svcCtx, &OpsAgentSupervisorEventsRequest{AgentID: "ghost"})
	require.NoError(t, err)
	assert.Empty(t, resp.Events)
	assert.Equal(t, int64(0), resp.LatestSeq)

	// store 缺失 → 错误。
	_, err = opsAgentSupervisorEvents(context.Background(), &svc.ServiceContext{}, &OpsAgentSupervisorEventsRequest{AgentID: "a1"})
	assert.Error(t, err)
}

func TestSupervisorEventFromProtoMapping(t *testing.T) {
	out := supervisorEventFromProto(&opsv1.SupervisorEvent{
		Seq: 7, TsUnix: 1700000000, Process: "app", Event: "detect_down",
		OldPid: 5, NewPid: 0, ExitCode: -1, Signal: "SIGKILL", RestartCount: 3,
		Message: "boom", LastHeartbeatUnix: 1699999900, LastError: "panic",
		OomSuspect: true, LastRssBytes: 2048,
	})
	assert.Equal(t, int64(7), out.Seq)
	assert.NotEmpty(t, out.Ts)
	assert.Equal(t, "SIGKILL", out.Signal)
	assert.True(t, out.OomSuspect)
	assert.Equal(t, int64(2048), out.LastRssBytes)
	assert.NotEmpty(t, out.LastHeartbeat)
	assert.Equal(t, "boom", out.Message)

	// 零值时间 → 空串。
	zero := supervisorEventFromProto(&opsv1.SupervisorEvent{Event: "manual_start"})
	assert.Empty(t, zero.Ts)
	assert.Empty(t, zero.LastHeartbeat)
}

func TestSupervisedProcessSnapshotDTOS2Fields(t *testing.T) {
	out := supervisedProcessFromSnapshot(&opsv1.SupervisedProcessSnapshot{
		Name: "app", State: opsv1.ProcessState_PROCESS_STATE_BACKOFF,
		LastEventUnix: 42, NextRestartAtUnix: 99,
	})
	assert.Equal(t, int64(42), out.LastEventUnix)
	assert.Equal(t, int64(99), out.NextRestartAtUnix)
}

func TestOpsAgentSupervisorEventsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := registry.NewMetricsStore()
	store.Add("a1", &opsv1.MetricsReport{AgentId: "a1", SupervisorEvents: []*opsv1.SupervisorEvent{
		{Seq: 1, TsUnix: 100, Event: "detect_down"},
	}})
	svcCtx := &svc.ServiceContext{MetricsStore: store}
	handler := NewHandler(NewService(svcCtx))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ops/agents/a1/supervisor/events", nil)
	c.Params = gin.Params{{Key: "agentId", Value: "a1"}}
	handler.OpsAgentSupervisorEvents(c)

	require.Equal(t, http.StatusOK, w.Code)
	var body OpsAgentSupervisorEventsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "a1", body.AgentID)
	require.Len(t, body.Events, 1)
	assert.Equal(t, "detect_down", body.Events[0].Event)
	assert.Equal(t, int64(1), body.LatestSeq)
}

func TestOpsAgentSupervisorLogHandler_Download(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orig := opsAgentSupLogFn
	t.Cleanup(func() { opsAgentSupLogFn = orig })
	opsAgentSupLogFn = func(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentSupervisorLogRequest) (*opsv1.GetSupervisorLogResponse, error) {
		assert.Equal(t, "a1", req.AgentID)
		assert.Equal(t, 4096, req.MaxBytes) // query 绑定生效
		return &opsv1.GetSupervisorLogResponse{
			Content:  []byte(`{"seq":1}` + "\n" + `{"seq":2}` + "\n"),
			FileName: "supervisor.log", Truncated: true,
		}, nil
	}

	handler := NewHandler(NewService(&svc.ServiceContext{}))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ops/agents/a1/supervisor/logs?maxBytes=4096", nil)
	c.Params = gin.Params{{Key: "agentId", Value: "a1"}}
	handler.OpsAgentSupervisorLog(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `attachment; filename="supervisor.log"`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, "true", w.Header().Get("X-Truncated"))
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	assert.Equal(t, "{\"seq\":1}\n{\"seq\":2}\n", w.Body.String())
}

func TestOpsAgentSupervisorLogHandler_ErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orig := opsAgentSupLogFn
	t.Cleanup(func() { opsAgentSupLogFn = orig })
	opsAgentSupLogFn = func(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentSupervisorLogRequest) (*opsv1.GetSupervisorLogResponse, error) {
		return nil, assert.AnError
	}

	handler := NewHandler(NewService(&svc.ServiceContext{}))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ops/agents/a1/supervisor/logs", nil)
	c.Params = gin.Params{{Key: "agentId", Value: "a1"}}
	handler.OpsAgentSupervisorLog(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotEmpty(t, body["error"])
}
