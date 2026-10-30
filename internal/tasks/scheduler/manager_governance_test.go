package scheduler

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubGovernance 记录收到的入参并回放结论（ARCH-024 治理闸门替身）。
type stubGovernance struct {
	calls     []stubGovernanceCall
	skip      bool
	err       error
	automated string
	actor     string
}

type stubGovernanceCall struct {
	functionID string
	gameID     string
	env        string
	payload    string
	metadata   map[string]string
	actor      string
	automated  string
}

func (g *stubGovernance) CheckGovernance(ctx context.Context, functionID, gameID, env string, payload []byte, metadata map[string]string, actor, automatedID string) (bool, error) {
	g.calls = append(g.calls, stubGovernanceCall{
		functionID: functionID, gameID: gameID, env: env,
		payload: string(payload), metadata: metadata,
		actor: actor, automated: automatedID,
	})
	return g.skip, g.err
}

// ARCH-024：治理闸门放行 → 正常派发，RunLog 落 dispatched。
func TestManager_GovernancePass_Dispatches(t *testing.T) {
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(7, "0 9 * * *", "", 0, 5, now.Add(-time.Minute))}
	d := &fakeDispatcher{}
	m := NewManager(store, d)
	g := &stubGovernance{skip: false}
	m.SetGovernance(g)

	m.tick(now)

	require.Len(t, d.calls, 1, "治理放行后必须派发")
	require.Len(t, g.calls, 1)
	key := fmt.Sprintf("7|%s", Slot(now).Format(time.RFC3339))
	assert.Equal(t, "dispatched", store.runLogs[key].Status)
	assert.NotEmpty(t, store.runLogs[key].TaskRunID)
}

// ARCH-024：治理跳过（政策要求审批且无批准快照 / creator 撤权）→ 不派发，
// RunLog 落 skipped、不计数失败、推进下次触发。
func TestManager_GovernanceSkip_NoDispatch(t *testing.T) {
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(8, "0 9 * * *", "", 0, 5, now.Add(-time.Minute))}
	d := &fakeDispatcher{}
	m := NewManager(store, d)
	m.SetGovernance(&stubGovernance{skip: true})

	m.tick(now)

	assert.Empty(t, d.calls, "治理跳过后不得派发")
	key := fmt.Sprintf("8|%s", Slot(now).Format(time.RFC3339))
	assert.Equal(t, "skipped", store.runLogs[key].Status)
	assert.Contains(t, store.runLogs[key].Message, "governance")
	// 推进下次触发时间（不推进会每周期重复判定同一槽位）。
	assert.Len(t, store.updates[8], 1)
	next, ok := store.updates[8][0]["next_triggered_at"].(time.Time)
	require.True(t, ok)
	assert.True(t, next.After(now))
}

// ARCH-024：治理故障（fail-closed）→ 不派发，RunLog 落 skipped 且带错误信息。
func TestManager_GovernanceError_NoDispatch(t *testing.T) {
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(9, "0 9 * * *", "", 0, 5, now.Add(-time.Minute))}
	d := &fakeDispatcher{}
	m := NewManager(store, d)
	m.SetGovernance(&stubGovernance{err: errors.New("policy store down")})

	m.tick(now)

	assert.Empty(t, d.calls, "治理故障 fail-closed，不得派发")
	key := fmt.Sprintf("9|%s", Slot(now).Format(time.RFC3339))
	assert.Equal(t, "skipped", store.runLogs[key].Status)
	assert.Contains(t, store.runLogs[key].Message, "governance error")
	assert.Contains(t, store.runLogs[key].Message, "policy store down")
}

// ARCH-024：未注入治理（旧测试/未接线）→ 维持派发（生产由 cmd/server 注入）。
func TestManager_NoGovernance_Dispatches(t *testing.T) {
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	store := newFakeStore()
	store.schedules = []model.TaskSchedule{dueSchedule(10, "0 9 * * *", "", 0, 5, now.Add(-time.Minute))}
	d := &fakeDispatcher{}
	m := NewManager(store, d)

	m.tick(now)

	require.Len(t, d.calls, 1)
}

// ARCH-024：治理入参携带调度上下文（function/game/env/payload/scheduleId/
// actor/automated=schedule:<id>）——automation 主体与 creator 回载的契约。
func TestManager_GovernanceSeesScheduleContext(t *testing.T) {
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	store := newFakeStore()
	s := dueSchedule(11, "0 9 * * *", "", 0, 5, now.Add(-time.Minute))
	s.Actor = "alice"
	s.Payload = model.JSON(`{"limit":10}`)
	s.Metadata = model.JSON(`{"route":"hash"}`)
	store.schedules = []model.TaskSchedule{s}
	d := &fakeDispatcher{}
	m := NewManager(store, d)
	g := &stubGovernance{}
	m.SetGovernance(g)

	m.tick(now)

	require.Len(t, g.calls, 1)
	c := g.calls[0]
	assert.Equal(t, "player.cleanup", c.functionID)
	assert.Equal(t, "demo", c.gameID)
	assert.Equal(t, "prod", c.env)
	assert.JSONEq(t, `{"limit":10}`, c.payload)
	assert.Equal(t, "alice", c.actor)
	assert.Equal(t, "schedule:11", c.automated)
	assert.Equal(t, "11", c.metadata["scheduleId"])
	assert.Equal(t, "hash", c.metadata["route"], "调度 metadata 合并进治理入参（批准快照等）")
}
