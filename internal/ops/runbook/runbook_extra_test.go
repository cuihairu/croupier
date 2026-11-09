package runbook_test

// runbook 编排补测：Define/Get/List/Delete/GetRun/ListRuns 的 store 守卫与
// 透传、Define 的 Create 失败与缺省步名、Run 的 Find/解码/空步骤/CreateRun
// 失败臂、连续 continueOnFailure 的错误聚合、batch/deploy_rollback 步骤的
// 未装配运行器与回滚透传，以及 batch_push/deploy_rollback 的 Define 校验臂。
//
// 诚实跳过：
//   - Define 的 stepsJSON marshal 错误臂（145）：[]StepSpec 全类型化字段。
//   - Run 的 substituteStep 失败臂（253-259）：substituteStep 四个 JSON 往返
//     均作用于类型化结构，恒成功；其内部 marshal 臂（565/569/574）同理。
//   - persistTerminal 失败透传臂（293）：persistTerminal 重试耗尽后转后台
//     恢复循环并恒返回 nil，该分支为防御性死臂。
//   - recoveryLoop 的「全部成功但仍有新 pending」臂（451）：需 enqueue 恰落在
//     快照与 more 检查之间的并发窗口，单线程测试不可确定复现。
//   - recoveryLoop 的退避封顶钳制臂（457）：需连续 6 轮失败退避（>63s 真实
//     睡眠）才能触顶，测试不可接受该时长。

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/batchop"
	"github.com/cuihairu/croupier/internal/ops/runbook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createFailStore / createRunFailStore：注入 Create/CreateRun 失败。
type createFailStore struct{ *fakeStore }

func (s *createFailStore) Create(_ context.Context, _ *model.Runbook) error {
	return errors.New("create boom")
}

type createRunFailStore struct{ *fakeStore }

func (s *createRunFailStore) CreateRun(_ context.Context, _ *model.RunbookRun) error {
	return errors.New("create run boom")
}

// listRunningFailStore 注入启动恢复的遗留行查询失败。
type listRunningFailStore struct{ *fakeStore }

func (s *listRunningFailStore) ListRunning(_ context.Context, _ time.Time) ([]*model.RunbookRun, error) {
	return nil, errors.New("list running boom")
}

// Define：store 未初始化（122）、空请求（125）、Create 失败透传（159）。
func TestDefine_GuardsAndCreateError(t *testing.T) {
	ctx := context.Background()
	nilStore := runbook.New(nil, nil, nil, nil)
	_, err := nilStore.Define(ctx, &runbook.DefineRequest{Name: "x", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{execStep("s", "cmd")}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store not initialized")

	o := runbook.New(newFakeStore(), nil, nil, nil)
	_, err = o.Define(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "define request required")

	_, err = runbook.New(&createFailStore{fakeStore: newFakeStore()}, nil, nil, nil).
		Define(ctx, &runbook.DefineRequest{Name: "x", GameID: "g", Env: "e",
			Steps: []runbook.StepSpec{execStep("s", "cmd")}})
	require.ErrorContains(t, err, "create boom")
}

// Define：步名缺省继承 kind（522）、push 缺目标选择（537）、rollback 缺
// deploymentId（545）。
func TestDefine_StepDefaultsAndValidationArms(t *testing.T) {
	store := newFakeStore()
	o := runbook.New(store, nil, nil, nil)

	rb, err := o.Define(context.Background(), &runbook.DefineRequest{Name: "x", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{{Kind: model.RunbookStepBatchExec,
			Exec: &batchop.ExecPayload{Command: "cmd"}, Labels: map[string]string{"role": "game"}}}})
	require.NoError(t, err)
	got, err := o.Get(context.Background(), "g", "e", rb.RunbookID)
	require.NoError(t, err)
	var steps []runbook.StepSpec
	require.NoError(t, json.Unmarshal([]byte(got.StepsJSON), &steps))
	require.Len(t, steps, 1)
	assert.Equal(t, model.RunbookStepBatchExec, steps[0].Name, "步名缺省继承 kind")

	_, err = o.Define(context.Background(), &runbook.DefineRequest{Name: "x", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{{Kind: model.RunbookStepBatchPush,
			Config: &batchop.ConfigPayload{Path: "/etc/x", ContentBase64: "aGk="}}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "labels or agentIds")

	_, err = o.Define(context.Background(), &runbook.DefineRequest{Name: "x", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{{Kind: model.RunbookStepDeployRollback, RollbackPackageID: "pkg-0"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deploymentId")
}

// Get/List/GetRun/ListRuns：store 未初始化守卫与成功透传
// （170/177-180/199-202/207-210）。
func TestReadPaths_GuardsAndPassthrough(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	nilStore := runbook.New(nil, nil, nil, nil)

	_, err := nilStore.Get(ctx, "g", "e", "rb")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store not initialized")
	_, err = nilStore.List(ctx, "g", "e", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store not initialized")
	_, err = nilStore.GetRun(ctx, "g", "e", "run")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store not initialized")
	_, err = nilStore.ListRuns(ctx, "g", "e", "rb", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store not initialized")

	o := runbook.New(store, nil, nil, nil)
	rb, err := o.Define(ctx, &runbook.DefineRequest{Name: "x", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{execStep("s", "cmd")}})
	require.NoError(t, err)

	rows, err := o.List(ctx, "g", "e", 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	res, err := o.Run(ctx, "g", "e", rb.RunbookID, nil, "admin")
	require.NoError(t, err)
	run, err := o.GetRun(ctx, "g", "e", res.RunID)
	require.NoError(t, err)
	assert.Equal(t, rb.RunbookID, run.RunbookID)
	runs, err := o.ListRuns(ctx, "g", "e", rb.RunbookID, 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
}

// Delete：store 未初始化（186）与成功审计（191-194）。
func TestDelete_GuardAndAudit(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	nilStore := runbook.New(nil, nil, nil, nil)
	require.ErrorContains(t, nilStore.Delete(ctx, "g", "e", "rb"), "store not initialized")

	o := runbook.New(store, nil, nil, auditRecorder(store))
	rb, err := o.Define(ctx, &runbook.DefineRequest{Name: "x", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{execStep("s", "cmd")}})
	require.NoError(t, err)
	require.NoError(t, o.Delete(ctx, "g", "e", rb.RunbookID))
	store.mu.Lock()
	defer store.mu.Unlock()
	assert.Contains(t, store.audited, "runbook.delete")
}

// Run：Find 失败（222）、steps 解码失败（226）、空步骤（229）、CreateRun
// 失败（244）。
func TestRun_PreExecutionErrorArms(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	o := runbook.New(store, &fakeBatch{}, nil, nil)

	_, err := o.Run(ctx, "g", "e", "rb-nope", nil, "admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	bad := &model.Runbook{RunbookID: "rb-badjson", Name: "bad", GameID: "g", Env: "e",
		StepsJSON: "not-json", CreatedBy: "admin"}
	require.NoError(t, store.Create(ctx, bad))
	_, err = o.Run(ctx, "g", "e", bad.RunbookID, nil, "admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode steps")

	empty := &model.Runbook{RunbookID: "rb-empty", Name: "empty", GameID: "g", Env: "e",
		StepsJSON: "[]", CreatedBy: "admin"}
	require.NoError(t, store.Create(ctx, empty))
	_, err = o.Run(ctx, "g", "e", empty.RunbookID, nil, "admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no steps")

	rb, err := o.Define(ctx, &runbook.DefineRequest{Name: "x", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{execStep("s", "cmd")}})
	require.NoError(t, err)
	_, err = runbook.New(&createRunFailStore{fakeStore: store}, &fakeBatch{}, nil, nil).
		Run(ctx, "g", "e", rb.RunbookID, nil, "admin")
	require.ErrorContains(t, err, "create run boom")
}

// Run：连续两个 continueOnFailure 失败步 → 错误聚合为 "e1; e2"（273）。
func TestRun_ContinueOnFailureAggregatesErrors(t *testing.T) {
	store := newFakeStore()
	o := runbook.New(store, &fakeBatch{fail: true}, nil, nil)
	rb, err := o.Define(context.Background(), &runbook.DefineRequest{
		Name: "r", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{
			{Name: "a", Kind: model.RunbookStepBatchExec, ContinueOnFailure: true,
				Exec: &batchop.ExecPayload{Command: "x"}, Labels: map[string]string{"role": "game"}},
			{Name: "b", Kind: model.RunbookStepBatchExec, ContinueOnFailure: true,
				Exec: &batchop.ExecPayload{Command: "y"}, Labels: map[string]string{"role": "game"}},
		},
	})
	require.NoError(t, err)
	res, err := o.Run(context.Background(), "g", "e", rb.RunbookID, nil, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusPartial, res.Status)
	require.Len(t, res.Steps, 2)
	assert.Contains(t, res.Error, "; ", "两个失败步的错误必须聚合")
	assert.Contains(t, res.Error, "b-fail")
}

// Run：batch 步骤未装配运行器 → 诚实失败（478）。
func TestRun_BatchNilRunner(t *testing.T) {
	store := newFakeStore()
	o := runbook.New(store, nil, nil, nil)
	rb, err := o.Define(context.Background(), &runbook.DefineRequest{
		Name: "r", GameID: "g", Env: "e", Steps: []runbook.StepSpec{execStep("s", "cmd")},
	})
	require.NoError(t, err)
	res, err := o.Run(context.Background(), "g", "e", rb.RunbookID, nil, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusFailed, res.Status)
	assert.Contains(t, res.Steps[0].Detail, "batch runner unavailable")
}

// RecoverStaleRuns：store 未初始化（340）与 ListRunning 失败透传（344）。
func TestRecoverStaleRuns_GuardsAndListError(t *testing.T) {
	ctx := context.Background()
	_, err := runbook.New(nil, nil, nil, nil).RecoverStaleRuns(ctx, time.Now().UTC())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store not initialized")

	_, err = runbook.New(&listRunningFailStore{fakeStore: newFakeStore()}, nil, nil, nil).
		RecoverStaleRuns(ctx, time.Now().UTC())
	require.ErrorContains(t, err, "list stale runs")
}

// Run：deploy_rollback 步骤——运行器未装配（508-510）与成功透传（511）。
func TestRun_DeployRollbackStep(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	rbStep := runbook.StepSpec{Name: "rb", Kind: model.RunbookStepDeployRollback,
		DeploymentID: "dep-1", RollbackPackageID: "pkg-0"}

	nilDeploy := runbook.New(store, &fakeBatch{}, nil, nil)
	rb, err := nilDeploy.Define(ctx, &runbook.DefineRequest{Name: "r", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{rbStep}})
	require.NoError(t, err)
	res, err := nilDeploy.Run(ctx, "g", "e", rb.RunbookID, nil, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusFailed, res.Status)
	assert.Contains(t, res.Steps[0].Detail, "deploy runner unavailable")

	deploy := &fakeDeploy{}
	o := runbook.New(store, &fakeBatch{}, deploy, nil)
	rb2, err := o.Define(ctx, &runbook.DefineRequest{Name: "r2", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{rbStep}})
	require.NoError(t, err)
	res, err = o.Run(ctx, "g", "e", rb2.RunbookID, nil, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusSucceeded, res.Status)
	require.Len(t, deploy.rolls, 1)
	assert.Equal(t, "dep-1/pkg-0", deploy.rolls[0])
}
