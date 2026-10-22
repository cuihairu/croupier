package server

// server 本机探针事件入站 seam（R3 server 侧内核装配）：与 agent 上行同
// 一条链路——诊断环留证 + 窗口→Alert 生命周期（视角=server，alertID 派生
// probe:server:<target>）。

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// serverWindowEvent server 视角窗口事件（agentID 空走回退）。
func serverWindowEvent(target, kind string) *opsv1.AlertEvent {
	return &opsv1.AlertEvent{
		Kind:    "probe.window",
		AgentId: "",
		Window: &opsv1.ProbeWindow{
			Target:        target,
			Semantics:     "readiness",
			WindowKind:    kind,
			StartTsUnixMs: 1000,
			LastReason:    "dial refused",
		},
	}
}

// 空载荷 agentID 回退 server；开窗落 firing、alertID=probe:server:<target>；
// 诊断环留证（明细查询同口径）。
func TestHandleServerProbeEvent_WindowLifecycle(t *testing.T) {
	svc, alertModel := newProbeLifecycleTestService(t)

	svc.HandleServerProbeEvent(context.Background(), serverWindowEvent("billing-api", "unavailable"))

	row, err := alertModel.FindByAlertID(context.Background(), probeAlertID(ServerProbeAgentID, "billing-api"))
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, "firing", row.Status)

	// 诊断环留证：server 视角可查最近事件。
	recents := svc.ProbeEventSink().recent(ServerProbeAgentID, 0)
	require.Len(t, recents, 1)
	assert.Equal(t, "billing-api", recents[0].GetWindow().GetTarget())

	// 闭窗 → resolved（同一行转换）。
	svc.HandleServerProbeEvent(context.Background(), serverWindowEvent("billing-api", "recovered"))
	row, err = alertModel.FindByAlertID(context.Background(), probeAlertID(ServerProbeAgentID, "billing-api"))
	require.NoError(t, err)
	assert.Equal(t, "resolved", row.Status)
}

// lifecycle 未装配（未 SetProbeLifecycleModel）时仅诊断环留证，不 panic。
func TestHandleServerProbeEvent_LifecycleAbsentKeepsSinkOnly(t *testing.T) {
	svc := NewControlService(nil, nil)
	svc.SetLogger(nil)

	svc.HandleServerProbeEvent(context.Background(), serverWindowEvent("lonely-api", "unavailable"))

	assert.Len(t, svc.ProbeEventSink().recent(ServerProbeAgentID, 0), 1)
}

// nil 事件与 nil 接收者安全。
func TestHandleServerProbeEvent_NilSafe(t *testing.T) {
	svc, _ := newProbeLifecycleTestService(t)
	svc.HandleServerProbeEvent(context.Background(), nil)

	var nilSvc *ControlService
	nilSvc.HandleServerProbeEvent(context.Background(), serverWindowEvent("x", "unavailable"))
}

// 视角并存：server 与 agent 探同一目标不串行（alertID 维度隔离，§4.3）。
func TestHandleServerProbeEvent_ViewpointsCoexist(t *testing.T) {
	svc, alertModel := newProbeLifecycleTestService(t)

	agentEv := probeWindowEvent("billing-api", "unavailable", 1000, 0, 0)
	agentEv.AgentId = "agent-77"
	if _, err := svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, agentEv)); err != nil {
		t.Fatalf("agent event: %v", err)
	}
	svc.HandleServerProbeEvent(context.Background(), serverWindowEvent("billing-api", "unavailable"))

	for _, id := range []string{"probe:agent-77:billing-api", "probe:server:billing-api"} {
		row, err := alertModel.FindByAlertID(context.Background(), id)
		require.NoError(t, err, id)
		require.NotNil(t, row, id)
		assert.Equal(t, "firing", row.Status, id)
	}
}
