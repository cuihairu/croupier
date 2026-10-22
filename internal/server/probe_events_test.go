package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

func windowEvent(target string) *opsv1.AlertEvent {
	return &opsv1.AlertEvent{
		Kind:    "probe.window",
		AgentId: "agent-1",
		GameId:  "demo",
		Env:     "prod",
		Window:  &opsv1.ProbeWindow{Target: target},
	}
}

func detailEvent(target, err string) *opsv1.AlertEvent {
	return &opsv1.AlertEvent{
		Kind:    "probe.detail",
		AgentId: "agent-1",
		GameId:  "demo",
		Env:     "prod",
		Detail:  &opsv1.ProbeDetail{Target: target, Err: err},
	}
}

func TestProbeEventSinkRecentByAgent(t *testing.T) {
	s := newProbeEventSink()
	s.add("agent-1", windowEvent("redis:6379"))
	s.add("agent-1", detailEvent("redis:6379", "refused"))
	s.add("agent-2", detailEvent("mysql:3306", "timeout"))

	got := s.recent("agent-1", 0)
	require.Len(t, got, 2, "events must not leak across agents")
	assert.Equal(t, "probe.window", got[0].Kind)
	assert.Equal(t, "probe.detail", got[1].Kind)

	assert.Len(t, s.recent("agent-2", 0), 1)
	assert.Empty(t, s.recent("agent-unknown", 0))
}

func TestProbeEventSinkRecentLimit(t *testing.T) {
	s := newProbeEventSink()
	for _, e := range []string{"a", "b", "c"} {
		s.add("agent-1", detailEvent("t", e))
	}
	got := s.recent("agent-1", 2)
	require.Len(t, got, 2)
	assert.Equal(t, "b", got[0].Detail.Err, "recent returns the tail")
	assert.Equal(t, "c", got[1].Detail.Err)
}

func TestProbeEventSinkRingBounded(t *testing.T) {
	s := newProbeEventSink()
	for i := 0; i < probeEventRingSize+25; i++ {
		s.add("agent-1", detailEvent("t", string(rune('a'+i%26))))
	}
	got := s.recent("agent-1", 0)
	assert.Len(t, got, probeEventRingSize, "ring must stay bounded")
}

func TestProbeEventSinkSweepExpired(t *testing.T) {
	s := newProbeEventSink()
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }

	s.add("agent-1", detailEvent("t", "old"))

	// 保留期内仍在。
	now = now.Add(probeDetailRetention - time.Minute)
	assert.Len(t, s.recent("agent-1", 0), 1)

	// 过期即整环丢弃（长驻 agent 的零频目标不能无限占内存）。
	now = now.Add(2 * time.Minute)
	assert.Empty(t, s.recent("agent-1", 0))
}

func TestProbeEventSinkNilAndEmptyGuards(t *testing.T) {
	var s *probeEventSink
	s.add("agent-1", windowEvent("t"))
	assert.Nil(t, s.recent("agent-1", 0))

	live := newProbeEventSink()
	live.add("", windowEvent("t"))
	live.add("agent-1", nil)
	assert.Empty(t, live.recent("agent-1", 0))
}

// --- handler 接收面（R2 边界：校验 + 留证 + 日志，不落 Alert 表） ---

func marshalAlertEvent(t *testing.T, ev *opsv1.AlertEvent) []byte {
	t.Helper()
	b, err := proto.Marshal(ev)
	require.NoError(t, err)
	return b
}

func TestHandleProbeEventAccepts(t *testing.T) {
	svc := newTestControlService()

	_, err := svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, windowEvent("redis:6379")))
	require.NoError(t, err)
	_, err = svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, detailEvent("redis:6379", "refused")))
	require.NoError(t, err)

	got := svc.ProbeEventSink().recent("agent-1", 0)
	require.Len(t, got, 2)
	assert.Equal(t, "probe.window", got[0].Kind)
	assert.Equal(t, "probe.detail", got[1].Kind)
}

func TestHandleProbeEventRejects(t *testing.T) {
	svc := newTestControlService()

	cases := map[string]*opsv1.AlertEvent{
		"missing agent_id": {Kind: "probe.window", Window: &opsv1.ProbeWindow{Target: "t"}},
		"unknown kind":     {Kind: "probe.mystery", AgentId: "agent-1"},
		"kind/payload mismatch (window)": {
			Kind: "probe.window", AgentId: "agent-1", Detail: &opsv1.ProbeDetail{Err: "x"},
		},
		"kind/payload mismatch (detail)": {
			Kind: "probe.detail", AgentId: "agent-1", Window: &opsv1.ProbeWindow{Target: "t"},
		},
	}
	for name, ev := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := svc.handleProbeEvent(context.Background(), marshalAlertEvent(t, ev))
			assert.Error(t, err)
		})
	}

	// 垃圾字节反序列化失败同样拒收。
	_, err := svc.handleProbeEvent(context.Background(), []byte{0xff, 0xff, 0xff})
	assert.Error(t, err)

	assert.Empty(t, svc.ProbeEventSink().recent("agent-1", 0), "rejected events must not enter the diagnostic ring")
}

func TestHandleProbeEventNilServiceSafe(t *testing.T) {
	var svc *ControlService
	assert.Nil(t, svc.ProbeEventSink())
}
