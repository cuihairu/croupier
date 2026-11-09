package deploy

// 部署编排补测：NewDeployment 校验臂与 1% 小批切分、Start 的包校验/CAS 注错、
// Advance 的投递失败/BatchWait 两分支/回写失败两臂、健康门禁的默认参数/
// 空原因/取消/重试成功、Rollback 的 pending 跳过/apply 失败/回写失败，以及
// NewPackageID。
//
// 诚实跳过：
//   - NewDeploymentID 的 rand.Read 失败回退臂（id.go 14-15）：测试无法替换
//     crypto/rand 熵源，熵失败不可复现。
//   - saveTargets 的 json.Marshal 错误臂（325-326）：DeploymentTarget 全为
//     字符串/整型字段，marshal 恒成功。

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

// errKnobStore 在 fakeStore 上注入 CAS 错误与第 N 次 Update 失败。
type errKnobStore struct {
	*fakeStore
	casErr       error
	updateFailAt int // 1-indexed；0 = 永不失败
	updateCalls  int
}

func (f *errKnobStore) Update(ctx context.Context, d *model.Deployment) error {
	f.updateCalls++
	if f.updateFailAt > 0 && f.updateCalls == f.updateFailAt {
		return errors.New("update failpoint")
	}
	return f.fakeStore.Update(ctx, d)
}

func (f *errKnobStore) CompareAndSwapState(ctx context.Context, id, from, to string) (bool, *model.Deployment, error) {
	if f.casErr != nil {
		return false, nil, f.casErr
	}
	return f.fakeStore.CompareAndSwapState(ctx, id, from, to)
}

// flakyHealthyAgent 前 N 次 Healthy 返回不健康，其后透传。
type flakyHealthyAgent struct {
	*fakeAgent
	fails       int
	mu          sync.Mutex
	healthCalls int
}

func (f *flakyHealthyAgent) Healthy(ctx context.Context, agentID string) (bool, string) {
	f.mu.Lock()
	f.healthCalls++
	n := f.healthCalls
	f.mu.Unlock()
	if n <= f.fails {
		return false, ""
	}
	return f.fakeAgent.Healthy(ctx, agentID)
}

// NewDeployment：非法百分比（79）、空目标（82）、1% 小批切分下限（89）。
func TestNewDeployment_ValidationArms(t *testing.T) {
	_, err := NewDeployment("pkg-1", "d", "g", "prod", "role", "game", 25, []string{"a"}, "x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid batch percent 25")

	_, err = NewDeployment("pkg-1", "d", "g", "prod", "role", "game", 100, nil, "x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one target")

	// 5 台 * 1% → 每批下限 1 → 5 批。
	d, err := NewDeployment("pkg-1", "d", "g", "prod", "role", "game", 1, []string{"a", "b", "c", "d", "e"}, "x")
	require.NoError(t, err)
	require.Equal(t, 5, d.BatchCount)
}

// NewPackageID 复用部署单 id 生成器（21-22）。
func TestNewPackageID(t *testing.T) {
	id := NewPackageID()
	require.True(t, strings.HasPrefix(id, "dep-"))
	require.NotEqual(t, id, NewPackageID())
}

// Start：包缺 objectKey（133）、CAS 注错（138）。
func TestStart_ValidationAndCASError(t *testing.T) {
	ctx := context.Background()
	d, err := NewDeployment("pkg-1", "d", "g", "prod", "role", "game", 100, []string{"a"}, "x")
	require.NoError(t, err)
	store := newFakeStore()
	require.NoError(t, store.Create(ctx, d))
	o := New(store, newFakeAgent())

	for _, bad := range []*model.DeployPackage{nil, {PackageID: "pkg-1", Version: "1"}} {
		_, err = o.Start(ctx, d, bad)
		require.Error(t, err)
		require.Contains(t, err.Error(), "package must have objectKey")
	}

	casErr := errors.New("cas down")
	_, err = New(&errKnobStore{fakeStore: store, casErr: casErr}, newFakeAgent()).Start(ctx, d, pkg())
	require.ErrorIs(t, err, casErr)
}

// Start：调用方持陈旧 draft 副本、库内行已推进时由 CAS 拒绝（141）。
func TestStart_StaleDraftRejectedByCAS(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	agent := newFakeAgent()
	agent.healthy["a"] = true
	d, err := NewDeployment("pkg-1", "d", "g", "prod", "role", "game", 100, []string{"a"}, "x")
	require.NoError(t, err)
	require.NoError(t, store.Create(ctx, d))
	stale := *d
	o := New(store, agent)
	o.HealthRetry = time.Millisecond
	o.HealthRetries = 1
	_, err = o.Start(ctx, d, pkg())
	require.NoError(t, err)

	_, err = o.Start(ctx, &stale, pkg())
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot start from state")
}

// Advance：StagePackage 失败 → 批次失败并落 paused 原因（169-178）。
func TestAdvance_StageFailure(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	agent := newFakeAgent()
	agent.stageErr["a"] = errors.New("no space")
	d, err := NewDeployment("pkg-1", "d", "g", "prod", "role", "game", 100, []string{"a"}, "x")
	require.NoError(t, err)
	require.NoError(t, store.Create(ctx, d))

	started, err := New(store, agent).Start(ctx, d, pkg())
	require.Error(t, err)
	require.Contains(t, err.Error(), "deploy: stage a")
	require.Equal(t, model.DeploymentStateFailed, started.State)
	require.Contains(t, started.PausedReason, "target a stage failed")
	targets := decodeTargets(started.TargetsJSON)
	require.Equal(t, model.DeploymentTargetFailed, targets[0].State)
	require.Contains(t, targets[0].Message, "stage: no space")
}

// 健康门禁：不健康且原因为空时回退默认文案（197）。
func TestHealthGate_EmptyReason(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	agent := newFakeAgent()
	agent.healthy["a"] = false // healthyMsg 为空
	d, err := NewDeployment("pkg-1", "d", "g", "prod", "role", "game", 100, []string{"a"}, "x")
	require.NoError(t, err)
	require.NoError(t, store.Create(ctx, d))

	o := New(store, agent)
	o.HealthRetry = time.Millisecond
	o.HealthRetries = 1
	started, err := o.Start(ctx, d, pkg())
	require.Error(t, err)
	require.Equal(t, model.DeploymentStatePaused, started.State)
	targets := decodeTargets(started.TargetsJSON)
	require.Contains(t, targets[0].Message, "health gate failed: health gate failed")
}

// Advance：BatchWait 正常等待后完成（214/217）；ctx 已取消时直接返回
// ctx.Err（216）。
func TestAdvance_BatchWaitArms(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	agent := newFakeAgent()
	agent.healthy["a"] = true
	d, err := NewDeployment("pkg-1", "d", "g", "prod", "role", "game", 100, []string{"a"}, "x")
	require.NoError(t, err)
	require.NoError(t, store.Create(ctx, d))

	o := New(store, agent)
	o.HealthRetry = time.Millisecond
	o.HealthRetries = 1
	o.BatchWait = 2 * time.Millisecond
	done, err := o.Start(ctx, d, pkg())
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateComplete, done.State)

	// 预取消的 ctx：BatchWait 的 select 只可能命中 Done。
	draft, err := NewDeployment("pkg-1", "d2", "g", "prod", "role", "game", 100, []string{"a"}, "x")
	require.NoError(t, err)
	require.NoError(t, store.Create(ctx, draft))
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = o.Advance(cancelled, draft, pkg())
	require.ErrorIs(t, err, context.Canceled)
}

// Advance：回写失败两臂——批完成后回写失败（223）与全部完成回写失败（154）。
func TestAdvance_UpdateErrorArms(t *testing.T) {
	ctx := context.Background()
	agent := newFakeAgent()
	agent.healthy["a"] = true

	// 第 1 次 Update（批完成回写）失败。
	store := newFakeStore()
	d, err := NewDeployment("pkg-1", "d", "g", "prod", "role", "game", 100, []string{"a"}, "x")
	require.NoError(t, err)
	require.NoError(t, store.Create(ctx, d))
	o := New(&errKnobStore{fakeStore: store, updateFailAt: 1}, agent)
	o.HealthRetry = time.Millisecond
	o.HealthRetries = 1
	_, err = o.Start(ctx, d, pkg())
	require.ErrorContains(t, err, "update failpoint")

	// 第 2 次 Update（全部完成回写）失败。
	store2 := newFakeStore()
	d2, err := NewDeployment("pkg-1", "d2", "g", "prod", "role", "game", 100, []string{"a"}, "x")
	require.NoError(t, err)
	require.NoError(t, store2.Create(ctx, d2))
	o = New(&errKnobStore{fakeStore: store2, updateFailAt: 2}, agent)
	o.HealthRetry = time.Millisecond
	o.HealthRetries = 1
	_, err = o.Start(ctx, d2, pkg())
	require.ErrorContains(t, err, "update failpoint")
}

// Rollback：pending 目标跳过不回滚（236）。
func TestRollback_SkipsPendingTargets(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	agent := newFakeAgent()
	targets := []model.DeploymentTarget{
		{AgentID: "a", Batch: 1, State: model.DeploymentTargetHealthy},
		{AgentID: "b", Batch: 1, State: model.DeploymentTargetPending},
	}
	raw, err := json.Marshal(targets)
	require.NoError(t, err)
	d := &model.Deployment{DeploymentID: "dep-rb-skip", ProcessName: "game",
		State: model.DeploymentStatePaused, TargetsJSON: model.JSON(raw)}
	require.NoError(t, store.Create(ctx, d))

	rbPkg := &model.DeployPackage{PackageID: "pkg-0", Version: "1.0.0", ObjectKey: "deploy/p/0.bin", Checksum: "999999999999"}
	rolled, err := New(store, agent).Rollback(ctx, d, rbPkg)
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateRolledBack, rolled.State)
	require.Equal(t, []string{"a"}, agent.applied)
	after := decodeTargets(rolled.TargetsJSON)
	require.Equal(t, model.DeploymentTargetRolledBack, after[0].State)
	require.Equal(t, model.DeploymentTargetPending, after[1].State, "pending 目标不参与回滚")
}

// Rollback：目标 apply 失败 → rollback_failed 并上抛（248-252）。
func TestRollback_ApplyError(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	agent := newFakeAgent()
	agent.healthy["a"] = true
	d, err := NewDeployment("pkg-1", "d", "g", "prod", "role", "game", 100, []string{"a"}, "x")
	require.NoError(t, err)
	require.NoError(t, store.Create(ctx, d))
	o := New(store, agent)
	o.HealthRetry = time.Millisecond
	o.HealthRetries = 1
	started, err := o.Start(ctx, d, pkg())
	require.NoError(t, err)

	agent.applyErr["a"] = errors.New("apply boom")
	rbPkg := &model.DeployPackage{PackageID: "pkg-0", Version: "1.0.0", ObjectKey: "deploy/p/0.bin", Checksum: "999999999999"}
	rolled, err := o.Rollback(ctx, started, rbPkg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "deploy: rollback apply agent a")
	require.Equal(t, model.DeploymentStateRollbackFailed, rolled.State)
	after := decodeTargets(rolled.TargetsJSON)
	require.Contains(t, after[0].Message, "rollback apply: apply boom")
}

// Rollback：回写失败上抛（267）。
func TestRollback_UpdateError(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	agent := newFakeAgent()
	agent.healthy["a"] = true
	d, err := NewDeployment("pkg-1", "d", "g", "prod", "role", "game", 100, []string{"a"}, "x")
	require.NoError(t, err)
	require.NoError(t, store.Create(ctx, d))
	o := New(store, agent)
	o.HealthRetry = time.Millisecond
	o.HealthRetries = 1
	started, err := o.Start(ctx, d, pkg())
	require.NoError(t, err)

	rbPkg := &model.DeployPackage{PackageID: "pkg-0", Version: "1.0.0", ObjectKey: "deploy/p/0.bin", Checksum: "999999999999"}
	_, err = New(&errKnobStore{fakeStore: store, updateFailAt: 1}, agent).Rollback(ctx, started, rbPkg)
	require.ErrorContains(t, err, "update failpoint")
}

// 健康门禁：零值编排器走默认重试/间隔（279/283）；重试等待中 ctx 取消返回
// cancelled（288）；首次不健康、重试后健康（290）。
func TestHealthGate_DefaultsCancelAndRetry(t *testing.T) {
	ctx := context.Background()
	agent := newFakeAgent()
	agent.healthy["a"] = true

	// 零值编排器：默认 3 次 / 30s（首轮即健康，不触发等待）。
	store := newFakeStore()
	d, err := NewDeployment("pkg-1", "d", "g", "prod", "role", "game", 100, []string{"a"}, "x")
	require.NoError(t, err)
	require.NoError(t, store.Create(ctx, d))
	done, err := (&Orchestrator{store: store, agent: agent}).Advance(ctx, d, pkg())
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateComplete, done.State)

	// 等待间隔内 ctx 已取消：长间隔保证 select 只命中 Done。
	store2 := newFakeStore()
	agent2 := newFakeAgent() // 恒不健康
	d2, err := NewDeployment("pkg-1", "d2", "g", "prod", "role", "game", 100, []string{"a"}, "x")
	require.NoError(t, err)
	require.NoError(t, store2.Create(ctx, d2))
	o := New(store2, agent2)
	o.HealthRetry = 5 * time.Second
	o.HealthRetries = 3
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	paused, err := o.Advance(cancelled, d2, pkg())
	require.Error(t, err)
	require.Equal(t, model.DeploymentStatePaused, paused.State)
	targets := decodeTargets(paused.TargetsJSON)
	require.Equal(t, model.DeploymentTargetFailed, targets[0].State)
	require.Contains(t, targets[0].Message, "cancelled", "取消原因写入目标消息")

	// 首次不健康、重试后健康：等待分支真正走到 time.After。
	store3 := newFakeStore()
	flaky := &flakyHealthyAgent{fakeAgent: newFakeAgent(), fails: 1}
	flaky.healthy["a"] = true
	d3, err := NewDeployment("pkg-1", "d3", "g", "prod", "role", "game", 100, []string{"a"}, "x")
	require.NoError(t, err)
	require.NoError(t, store3.Create(ctx, d3))
	o = New(store3, flaky)
	o.HealthRetry = time.Millisecond
	o.HealthRetries = 2
	done3, err := o.Start(ctx, d3, pkg())
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateComplete, done3.State)
	require.GreaterOrEqual(t, flaky.healthCalls, 2)
}
