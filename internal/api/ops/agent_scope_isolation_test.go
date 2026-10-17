package ops

import (
	"context"
	"testing"

	agentops "github.com/cuihairu/croupier/internal/logic/ops"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestAgentMetricsReadsRequireScope(t *testing.T) {
	metrics := registry.NewMetricsStore()
	metrics.Add("agent-a", &opsv1.MetricsReport{
		AgentId: "agent-a",
		SupervisedProcesses: []*opsv1.SupervisedProcessSnapshot{
			{Name: "game", State: opsv1.ProcessState_PROCESS_STATE_RUNNING},
		},
		SupervisorEvents: []*opsv1.SupervisorEvent{
			{Seq: 1, Process: "game", Event: "started"},
		},
	})
	registryStore := registry.NewStore()
	require.NoError(t, registryStore.UpsertAgent(&registry.AgentSession{AgentID: "agent-a", GameID: "demo", Env: "prod"}))
	svcCtx := &svc.ServiceContext{MetricsStore: metrics, RegistryStore: registryStore}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "other", Env: "prod"})

	_, err := agentMetricsHistory(ctx, svcCtx, &AgentMetricsHistoryRequest{AgentID: "agent-a"})
	require.ErrorContains(t, err, "does not belong")

	_, err = opsAgentSupervisor(ctx, svcCtx, &OpsAgentSupervisorRequest{AgentID: "agent-a"})
	require.ErrorContains(t, err, "does not belong")

	_, err = opsAgentSupervisorEvents(ctx, svcCtx, &OpsAgentSupervisorEventsRequest{AgentID: "agent-a"})
	require.ErrorContains(t, err, "does not belong")
}

func TestAgentSupervisorLogRequiresScope(t *testing.T) {
	caller := &fakeCaller{respFor: map[uint32]proto.Message{
		protocol.MsgGetSupervisorLogRequest: &opsv1.GetSupervisorLogResponse{Content: []byte("log")},
	}}
	client := agentops.GetAgentOpsClient()
	client.SetSessionResolver(&fakeResolver{conns: map[string]transport.SessionCaller{"agent-a": caller}})
	t.Cleanup(func() { client.SetSessionResolver(nil) })

	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "agent-a", GameID: "demo", Env: "prod"}))
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "other", Env: "prod"})

	_, err := opsAgentSupervisorLog(ctx, &svc.ServiceContext{RegistryStore: store}, &OpsAgentSupervisorLogRequest{AgentID: "agent-a"})
	require.ErrorContains(t, err, "does not belong")
	assert.Empty(t, caller.calls)
}

func TestAgentSupervisorReadsAllowMatchingScope(t *testing.T) {
	metrics := registry.NewMetricsStore()
	metrics.Add("agent-a", &opsv1.MetricsReport{AgentId: "agent-a"})
	svcCtx := &svc.ServiceContext{MetricsStore: metrics, RegistryStore: registry.NewStore()}
	require.NoError(t, svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{AgentID: "agent-a", GameID: "demo", Env: "prod"}))
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})

	snapshot, err := opsAgentSupervisor(ctx, svcCtx, &OpsAgentSupervisorRequest{AgentID: "agent-a"})
	require.NoError(t, err)
	assert.Equal(t, "agent-a", snapshot.AgentID)

	events, err := opsAgentSupervisorEvents(ctx, svcCtx, &OpsAgentSupervisorEventsRequest{AgentID: "agent-a"})
	require.NoError(t, err)
	assert.Equal(t, "agent-a", events.AgentID)
}
