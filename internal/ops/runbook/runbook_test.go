// 覆盖目标：OPS-009 runbook 编排——定义校验、参数化步骤序列执行
// （batch_exec/batch_push/deploy_start/deploy_rollback）、单步失败中断、
// continueOnFailure 部分成功、参数替换、审计触发。
package runbook_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/batchop"
	"github.com/cuihairu/croupier/internal/ops/runbook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"time"
)

// fakeStore 内存 store：runbooks + runs，记录 audit。
type fakeStore struct {
	mu       sync.Mutex
	runbooks map[string]*model.Runbook
	runs     map[string]*model.RunbookRun
	nextID   uint
	audited  []string
	// updateRunStatusErrFails 终态写库前 N 次失败（审核发现 865 重试回归）。
	updateRunStatusErrFails int
}

func newFakeStore() *fakeStore {
	return &fakeStore{runbooks: map[string]*model.Runbook{}, runs: map[string]*model.RunbookRun{}}
}

func (s *fakeStore) Create(_ context.Context, rb *model.Runbook) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	rb.ID = s.nextID
	s.runbooks[rb.RunbookID] = rb
	return nil
}

func (s *fakeStore) Find(_ context.Context, gameID, env, runbookID string) (*model.Runbook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rb, ok := s.runbooks[runbookID]
	if !ok || rb.GameID != gameID || rb.Env != env {
		return nil, errors.New("not found")
	}
	return rb, nil
}

func (s *fakeStore) List(_ context.Context, gameID, env string, limit int) ([]model.Runbook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []model.Runbook
	for _, rb := range s.runbooks {
		if rb.GameID == gameID && rb.Env == env {
			out = append(out, *rb)
		}
	}
	return out, nil
}

func (s *fakeStore) Delete(_ context.Context, gameID, env, runbookID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rb, ok := s.runbooks[runbookID]
	if !ok || rb.GameID != gameID || rb.Env != env {
		return errors.New("not found")
	}
	delete(s.runbooks, runbookID)
	return nil
}

func (s *fakeStore) CreateRun(_ context.Context, run *model.RunbookRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	run.ID = s.nextID
	s.runs[run.RunID] = run
	return nil
}

func (s *fakeStore) FindRun(_ context.Context, gameID, env, runID string) (*model.RunbookRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	if !ok || run.GameID != gameID || run.Env != env {
		return nil, errors.New("not found")
	}
	return run, nil
}

func (s *fakeStore) ListRuns(_ context.Context, gameID, env, runbookID string, limit int) ([]model.RunbookRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []model.RunbookRun
	for _, run := range s.runs {
		if run.GameID == gameID && run.Env == env && run.RunbookID == runbookID {
			out = append(out, *run)
		}
	}
	return out, nil
}

func (s *fakeStore) UpdateRunStatus(_ context.Context, id uint, status string, currentStep int, stepsResultJSON, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.updateRunStatusErrFails > 0 {
		s.updateRunStatusErrFails--
		return errors.New("db down")
	}
	for _, run := range s.runs {
		if run.ID == id {
			run.Status = status
			run.CurrentStep = currentStep
			run.StepsResultJSON = stepsResultJSON
			run.Error = errMsg
			now := time.Now().UTC()
			run.FinishedAt = &now
			return nil
		}
	}
	return errors.New("run not found")
}

// fakeBatch 记录收到的 CreateRequest，可注入失败/错误。
type fakeBatch struct {
	mu   sync.Mutex
	got  []*batchop.CreateRequest
	fail bool // 聚合失败（res.Status=failed）
	err  error
}

func (f *fakeBatch) RunBatch(_ context.Context, _ string, _ string, req *batchop.CreateRequest) (*batchop.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, req)
	if f.err != nil {
		return nil, f.err
	}
	if f.fail {
		return &batchop.Result{BatchID: "b-fail", Status: model.BatchOpStatusFailed, Targets: 1, Failed: 1}, nil
	}
	return &batchop.Result{BatchID: "b-ok", Status: model.BatchOpStatusCompleted, Targets: 1, Succeeded: 1}, nil
}

func (f *fakeBatch) requests() []*batchop.CreateRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.got
}

// fakeDeploy 记录部署动作，可注入错误。
type fakeDeploy struct {
	mu       sync.Mutex
	starts   []string
	rolls    []string
	startErr error
}

func (f *fakeDeploy) StartDeployment(_ context.Context, deploymentID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts = append(f.starts, deploymentID)
	return f.startErr
}

func (f *fakeDeploy) RollbackDeployment(_ context.Context, deploymentID, rollbackPackageID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rolls = append(f.rolls, deploymentID+"/"+rollbackPackageID)
	return nil
}

func auditRecorder(s *fakeStore) func(context.Context, string, string, string, string, string, map[string]any) {
	return func(_ context.Context, _ string, _ string, event, _ string, _ string, _ map[string]any) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.audited = append(s.audited, event)
	}
}

func execStep(name, command string) runbook.StepSpec {
	return runbook.StepSpec{Name: name, Kind: model.RunbookStepBatchExec,
		Exec: &batchop.ExecPayload{Command: command}, Labels: map[string]string{"role": "game"}}
}

// TestDefine_PersistsAndAudits 定义合法 runbook：落库 + 审计 create。
func TestDefine_PersistsAndAudits(t *testing.T) {
	store := newFakeStore()
	o := runbook.New(store, nil, nil, auditRecorder(store))
	rb, err := o.Define(context.Background(), &runbook.DefineRequest{
		Name: "restart fleet", GameID: "game1", Env: "prod",
		Steps: []runbook.StepSpec{execStep("restart", "systemctl restart game")},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, rb.RunbookID)
	assert.Equal(t, "game1", rb.GameID)
	assert.Equal(t, "prod", rb.Env)
	got, err := o.Get(context.Background(), "game1", "prod", rb.RunbookID)
	require.NoError(t, err)
	assert.Equal(t, rb.Name, got.Name)
	store.mu.Lock()
	defer store.mu.Unlock()
	assert.Contains(t, store.audited, "runbook.create")
}

// TestDefine_Validation 非法定义逐项拒绝。
func TestDefine_Validation(t *testing.T) {
	o := runbook.New(newFakeStore(), nil, nil, nil)
	base := runbook.DefineRequest{Name: "x", GameID: "g", Env: "e", Steps: []runbook.StepSpec{execStep("s", "cmd")}}

	cases := []struct {
		name string
		mut  func(*runbook.DefineRequest)
		want string
	}{
		{"no name", func(r *runbook.DefineRequest) { r.Name = "" }, "name required"},
		{"no game", func(r *runbook.DefineRequest) { r.GameID = "" }, "gameId and env required"},
		{"no env", func(r *runbook.DefineRequest) { r.Env = "" }, "gameId and env required"},
		{"no steps", func(r *runbook.DefineRequest) { r.Steps = nil }, "at least one step required"},
		{"empty kind", func(r *runbook.DefineRequest) { r.Steps[0].Kind = "" }, "kind required"},
		{"unknown kind", func(r *runbook.DefineRequest) { r.Steps[0].Kind = "nope" }, "unknown step kind"},
		{"exec no command", func(r *runbook.DefineRequest) {
			r.Steps[0].Kind = model.RunbookStepBatchExec
			r.Steps[0].Exec = &batchop.ExecPayload{}
		}, "exec.command"},
		{"exec no selectors", func(r *runbook.DefineRequest) {
			r.Steps[0].Kind = model.RunbookStepBatchExec
			r.Steps[0].Exec = &batchop.ExecPayload{Command: "cmd"}
			r.Steps[0].Labels = nil
			r.Steps[0].AgentIDs = nil
		}, "labels or agentIds"},
		{"push no content", func(r *runbook.DefineRequest) {
			r.Steps[0].Kind = model.RunbookStepBatchPush
			r.Steps[0].Exec = nil
			r.Steps[0].Config = &batchop.ConfigPayload{Path: "/etc/x"}
			r.Steps[0].Labels = map[string]string{"role": "game"}
		}, "config.path and config.contentBase64"},
		{"deploy_start no id", func(r *runbook.DefineRequest) {
			r.Steps[0].Kind = model.RunbookStepDeployStart
			r.Steps[0].Exec = nil
			r.Steps[0].DeploymentID = ""
		}, "deploymentId"},
		{"deploy_rollback no pkg", func(r *runbook.DefineRequest) {
			r.Steps[0].Kind = model.RunbookStepDeployRollback
			r.Steps[0].Exec = nil
			r.Steps[0].DeploymentID = "d1"
			r.Steps[0].RollbackPackageID = ""
		}, "rollbackPackageId"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := base
			req.Steps = append([]runbook.StepSpec{}, base.Steps...)
			c.mut(&req)
			_, err := o.Define(context.Background(), &req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}

// TestRun_AllSucceed 三步骤全成功 → succeeded + 审计 run。
func TestRun_AllSucceed(t *testing.T) {
	store := newFakeStore()
	batch := &fakeBatch{}
	deploy := &fakeDeploy{}
	o := runbook.New(store, batch, deploy, auditRecorder(store))
	rb, err := o.Define(context.Background(), &runbook.DefineRequest{
		Name: "rollout", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{
			execStep("restart", "systemctl restart game"),
			{Name: "start", Kind: model.RunbookStepDeployStart, DeploymentID: "dep-1"},
			{Name: "push", Kind: model.RunbookStepBatchPush,
				Config: &batchop.ConfigPayload{Path: "/etc/x", ContentBase64: "aGk="}, Labels: map[string]string{"role": "game"}},
		},
	})
	require.NoError(t, err)
	res, err := o.Run(context.Background(), "g", "e", rb.RunbookID, nil, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusSucceeded, res.Status)
	assert.Equal(t, 3, res.CurrentStep)
	require.Len(t, res.Steps, 3)
	for _, s := range res.Steps {
		assert.Equal(t, "success", s.Status)
	}
	require.Len(t, batch.requests(), 2)
	assert.Equal(t, model.BatchOpKindExec, batch.requests()[0].Kind)
	assert.Equal(t, model.BatchOpKindConfig, batch.requests()[1].Kind)
	require.Len(t, deploy.starts, 1)
	assert.Equal(t, "dep-1", deploy.starts[0])
	store.mu.Lock()
	defer store.mu.Unlock()
	assert.Contains(t, store.audited, "runbook.run")
}

// TestRun_BatchFailStops 聚合失败步骤中断后续步骤，终态 failed。
func TestRun_BatchFailStops(t *testing.T) {
	store := newFakeStore()
	batch := &fakeBatch{fail: true}
	deploy := &fakeDeploy{}
	o := runbook.New(store, batch, deploy, nil)
	rb, err := o.Define(context.Background(), &runbook.DefineRequest{
		Name: "r", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{
			execStep("restart", "systemctl restart game"),
			{Name: "start", Kind: model.RunbookStepDeployStart, DeploymentID: "dep-1"},
		},
	})
	require.NoError(t, err)
	res, err := o.Run(context.Background(), "g", "e", rb.RunbookID, nil, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusFailed, res.Status)
	assert.Equal(t, 1, res.CurrentStep, "failed step stops before deploy")
	assert.Equal(t, "failed", res.Steps[0].Status)
	assert.Empty(t, deploy.starts, "later step must not run")
}

// TestRun_BatchErrStops 运行器错误 → failed。
func TestRun_BatchErrStops(t *testing.T) {
	store := newFakeStore()
	o := runbook.New(store, &fakeBatch{err: errors.New("boom")}, nil, nil)
	rb, err := o.Define(context.Background(), &runbook.DefineRequest{
		Name: "r", GameID: "g", Env: "e", Steps: []runbook.StepSpec{execStep("s", "cmd")},
	})
	require.NoError(t, err)
	res, err := o.Run(context.Background(), "g", "e", rb.RunbookID, nil, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusFailed, res.Status)
	assert.Equal(t, "failed", res.Steps[0].Status)
}

// TestRun_ContinueOnFailure 失败步 continueOnFailure → partial，后续仍执行。
func TestRun_ContinueOnFailure(t *testing.T) {
	store := newFakeStore()
	deploy := &fakeDeploy{}
	o := runbook.New(store, &fakeBatch{fail: true}, deploy, nil)
	rb, err := o.Define(context.Background(), &runbook.DefineRequest{
		Name: "r", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{
			{Name: "restart", Kind: model.RunbookStepBatchExec, ContinueOnFailure: true,
				Exec: &batchop.ExecPayload{Command: "x"}, Labels: map[string]string{"role": "game"}},
			{Name: "start", Kind: model.RunbookStepDeployStart, DeploymentID: "dep-1"},
		},
	})
	require.NoError(t, err)
	res, err := o.Run(context.Background(), "g", "e", rb.RunbookID, nil, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusPartial, res.Status)
	assert.Equal(t, "failed", res.Steps[0].Status)
	assert.Equal(t, "success", res.Steps[1].Status)
	require.Len(t, deploy.starts, 1)
}

// TestRun_ContinueOnFailure_PersistsErrorAndFinishedAt（审核发现 864）：
// partial 运行必须把失败信息写入 RunResult.Error 并给失败步设 FinishedAt，
// 面板才能按 error 列聚合（此前仅步 detail 携带消息）。
func TestRun_ContinueOnFailure_PersistsErrorAndFinishedAt(t *testing.T) {
	store := newFakeStore()
	o := runbook.New(store, &fakeBatch{fail: true}, nil, nil)
	rb, err := o.Define(context.Background(), &runbook.DefineRequest{
		Name: "r", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{
			{Name: "restart", Kind: model.RunbookStepBatchExec, ContinueOnFailure: true,
				Exec: &batchop.ExecPayload{Command: "x"}, Labels: map[string]string{"role": "game"}},
		},
	})
	require.NoError(t, err)
	res, err := o.Run(context.Background(), "g", "e", rb.RunbookID, nil, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusPartial, res.Status)
	require.Len(t, res.Steps, 1)
	assert.Equal(t, "failed", res.Steps[0].Status)
	assert.NotEmpty(t, res.Steps[0].FinishedAt, "失败步必须写 FinishedAt")
	assert.NotEmpty(t, res.Error, "partial 必须写 RunResult.Error")
	store.mu.Lock()
	run := store.runs[res.RunID]
	store.mu.Unlock()
	require.NotNil(t, run)
	assert.Equal(t, model.RunbookRunStatusPartial, run.Status)
	assert.NotEmpty(t, run.Error, "落库 error 列非空")
}

// TestRun_TerminalPersistRetries（审核发现 865）：终态落库失败（如客户端
// 断开后 ctx 已取消）必须重试而非把 run 永久留在 running；成功重试后
// status 终态、不返回错误。
func TestRun_TerminalPersistRetries(t *testing.T) {
	store := newFakeStore()
	store.updateRunStatusErrFails = 1
	o := runbook.New(store, &fakeBatch{}, nil, nil)
	rb, err := o.Define(context.Background(), &runbook.DefineRequest{
		Name: "r", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{{Name: "s", Kind: model.RunbookStepBatchExec,
			Exec: &batchop.ExecPayload{Command: "x"}, Labels: map[string]string{"role": "game"}}},
	})
	require.NoError(t, err)
	// 客户端断开：请求 ctx 已取消，终态仍须落库成功。
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := o.Run(ctx, "g", "e", rb.RunbookID, nil, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusSucceeded, res.Status)
}

// TestRun_ParamSubstitution 步骤串字段 {{param}} 在运行前替换。
func TestRun_ParamSubstitution(t *testing.T) {
	store := newFakeStore()
	batch := &fakeBatch{}
	deploy := &fakeDeploy{}
	o := runbook.New(store, batch, deploy, nil)
	rb, err := o.Define(context.Background(), &runbook.DefineRequest{
		Name: "r", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{
			{Name: "restart", Kind: model.RunbookStepBatchExec,
				Exec:   &batchop.ExecPayload{Command: "{{cmd}}", Args: []string{"{{arg}}"}},
				Labels: map[string]string{"role": "{{role}}"}, AgentIDs: []string{"{{agent}}"}, DryRun: true},
			{Name: "start", Kind: model.RunbookStepDeployStart, DeploymentID: "{{deployment_id}}"},
		},
	})
	require.NoError(t, err)
	res, err := o.Run(context.Background(), "g", "e", rb.RunbookID,
		map[string]string{"cmd": "systemctl", "arg": "restart game", "role": "web", "agent": "ag-1", "deployment_id": "dep-9"}, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusSucceeded, res.Status)
	reqs := batch.requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "systemctl", reqs[0].Exec.Command)
	assert.Equal(t, []string{"restart game"}, reqs[0].Exec.Args)
	assert.Equal(t, "web", reqs[0].Labels["role"])
	assert.Equal(t, []string{"ag-1"}, reqs[0].AgentIDs)
	assert.True(t, reqs[0].DryRun)
	require.Len(t, deploy.starts, 1)
	assert.Equal(t, "dep-9", deploy.starts[0])
}

// TestRun_JsonInjectSafe 参数含 JSON 结构字符只作为字符串值替换，不逃逸结构。
func TestRun_JsonInjectSafe(t *testing.T) {
	store := newFakeStore()
	batch := &fakeBatch{}
	o := runbook.New(store, batch, nil, nil)
	rb, err := o.Define(context.Background(), &runbook.DefineRequest{
		Name: "r", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{{Name: "s", Kind: model.RunbookStepBatchExec,
			Exec: &batchop.ExecPayload{Command: "{{cmd}}"}, Labels: map[string]string{"role": "game"}}},
	})
	require.NoError(t, err)
	_, err = o.Run(context.Background(), "g", "e", rb.RunbookID,
		map[string]string{"cmd": "a\"}; DROP TABLE x; --"}, "admin")
	require.NoError(t, err)
	req := batch.requests()[0]
	assert.Equal(t, `a"}; DROP TABLE x; --`, req.Exec.Command)
	// 步骤 payload 仍是合法 JSON（结构未破坏）
	rb2, _ := o.Get(context.Background(), "g", "e", rb.RunbookID)
	var steps []runbook.StepSpec
	require.NoError(t, json.Unmarshal([]byte(rb2.StepsJSON), &steps))
	assert.Len(t, steps, 1)
}

// TestRun_UnknownKind 库内未知 kind（绕过 Define 校验直插）→ 步诚实失败。
func TestRun_UnknownKind(t *testing.T) {
	store := newFakeStore()
	o := runbook.New(store, &fakeBatch{}, nil, nil)
	// Define 会拒绝未知 kind；此处直插库模拟存量/外部写入的坏数据。
	rb := &model.Runbook{RunbookID: "rb-bad", Name: "bad", GameID: "g", Env: "e",
		StepsJSON: `[{"name":"s","kind":"nope"}]`, CreatedBy: "admin"}
	require.NoError(t, store.Create(context.Background(), rb))
	res, err := o.Run(context.Background(), "g", "e", rb.RunbookID, nil, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusFailed, res.Status)
	assert.Contains(t, res.Steps[0].Detail, "unknown step kind")
}

// TestRun_DeployNilRunner 未装配部署运行器 → 步诚实失败。
func TestRun_DeployNilRunner(t *testing.T) {
	store := newFakeStore()
	o := runbook.New(store, nil, nil, nil)
	rb, err := o.Define(context.Background(), &runbook.DefineRequest{
		Name: "r", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{{Name: "start", Kind: model.RunbookStepDeployStart, DeploymentID: "dep-1"}},
	})
	require.NoError(t, err)
	res, err := o.Run(context.Background(), "g", "e", rb.RunbookID, nil, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusFailed, res.Status)
	assert.Contains(t, res.Steps[0].Detail, "deploy runner unavailable")
}

// TestDelete_NotExist 删除不存在的 runbook 报 not found。
func TestDelete_NotExist(t *testing.T) {
	o := runbook.New(newFakeStore(), nil, nil, nil)
	err := o.Delete(context.Background(), "g", "e", "rb-nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestRun_StoreNil 未初始化 store 报错。
func TestRun_StoreNil(t *testing.T) {
	o := runbook.New(nil, nil, nil, nil)
	_, err := o.Run(context.Background(), "g", "e", "rb", nil, "admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}
