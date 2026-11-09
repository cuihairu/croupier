package batchop

// Orchestrator 校验臂/错误臂与 RegistrySelector 过滤路径补测：Create 的
// 空请求/缺 scope/未知 kind/缺目标/store 失败/审计记录，Execute 的 Find/CAS/
// 选址失败/targets 更新失败/dry-run 更新失败/聚合更新失败/审计，dispatch 的
// 载荷损坏/非零退出/bad base64/push 失败/拒绝/未知 kind，以及 registry 会话
// 的 scope/label/显式 ID 三维过滤。
//
// 诚实跳过：
//   - Create 的三个 json.Marshal 错误臂（137/141/145）：入参是
//     map[string]string / []string / 类型化 payload 结构体，marshal 恒成功。
//   - Execute 聚合的 skipped 计数分支（252）：dispatch 只产出 success/failed
//     两种状态，公开路径无法把 skipped 写进 res.Items（防御性统计）。

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- selector：registry 会话三维过滤 ----

func seedRegistryAgent(t *testing.T, store *reg.Store, sess *reg.AgentSession) {
	t.Helper()
	store.Mu().Lock()
	defer store.Mu().Unlock()
	store.AgentsUnsafe()[sess.AgentID] = sess
}

// TestRegistrySelector_SelectFilters 覆盖 scope 过滤、显式 ID 交集、label
// 匹配（want 空 / 不匹配 / 全匹配）与 nil / 空 AgentID 会话跳过。
func TestRegistrySelector_SelectFilters(t *testing.T) {
	ctx := context.Background()
	store := reg.NewStore()
	seedRegistryAgent(t, store, &reg.AgentSession{AgentID: "agent-1", GameID: "g", Env: "e", Labels: map[string]string{"role": "gate", "dc": "us"}})
	seedRegistryAgent(t, store, &reg.AgentSession{AgentID: "agent-2", GameID: "g", Env: "e", Labels: map[string]string{"role": "battle"}})
	seedRegistryAgent(t, store, &reg.AgentSession{AgentID: "agent-3", GameID: "g", Env: "other"}) // env 不匹配
	seedRegistryAgent(t, store, &reg.AgentSession{AgentID: "agent-4", GameID: "other", Env: "e"}) // game 不匹配
	seedRegistryAgent(t, store, &reg.AgentSession{AgentID: "", GameID: "g", Env: "e"})            // 空 AgentID 跳过
	sel := NewRegistrySelector(store)

	// 无过滤：只回收 scope 内会话。
	got, err := sel.Select(ctx, "g", "e", nil, nil)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"agent-1", "agent-2"}, got)

	// label 不匹配 → 跳过；label 全匹配 → 命中。
	got, err = sel.Select(ctx, "g", "e", map[string]string{"role": "gate"}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"agent-1"}, got)

	// 多 key want 部分命中即失败。
	got, err = sel.Select(ctx, "g", "e", map[string]string{"role": "gate", "dc": "eu"}, nil)
	require.NoError(t, err)
	assert.Empty(t, got)

	// 显式 ID 交集：ID 命中但 label 不匹配 → 空。
	got, err = sel.Select(ctx, "g", "e", map[string]string{"role": "battle"}, []string{"agent-1"})
	require.NoError(t, err)
	assert.Empty(t, got)

	// 仅显式 ID：未列入的跳过。
	got, err = sel.Select(ctx, "g", "e", nil, []string{"agent-1"})
	require.NoError(t, err)
	assert.Equal(t, []string{"agent-1"}, got)
}

// ---- Create 校验与失败臂 ----

type auditRecorder struct {
	mu     sync.Mutex
	events []string
}

func (a *auditRecorder) record(_ context.Context, _, _, event, _, _ string, _ map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, event)
}

func TestCreate_ValidationArms(t *testing.T) {
	o := New(&fakeSelector{}, &fakeExecutor{}, newFakeStore(), nil)
	ctx := context.Background()

	_, err := o.Create(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create request required")

	_, err = o.Create(ctx, &CreateRequest{GameID: " ", Env: "e", Kind: model.BatchOpKindExec, AgentIDs: []string{"a"}, Exec: &ExecPayload{Command: "ls"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gameId and env required")

	_, err = o.Create(ctx, &CreateRequest{GameID: "g", Env: "e", Kind: "restart-apps", AgentIDs: []string{"a"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown batch kind "restart-apps"`)

	_, err = o.Create(ctx, &CreateRequest{GameID: "g", Env: "e", Kind: model.BatchOpKindExec, Exec: &ExecPayload{Command: "ls"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "labels or agentIds required")
}

// Create：store 写入失败透传（163）、审计事件记录（166）。
func TestCreate_StoreErrorAndAudit(t *testing.T) {
	ctx := context.Background()
	createErr := errors.New("store down")
	knob := &knobStore{createErr: createErr}
	o := New(&fakeSelector{}, &fakeExecutor{}, knob, nil)
	_, err := o.Create(ctx, &CreateRequest{GameID: "g", Env: "e", Kind: model.BatchOpKindExec, AgentIDs: []string{"a"}, Exec: &ExecPayload{Command: "ls"}})
	require.ErrorIs(t, err, createErr)

	rec := &auditRecorder{}
	o2 := New(&fakeSelector{}, &fakeExecutor{}, newFakeStore(), rec.record)
	op, err := o2.Create(ctx, &CreateRequest{GameID: "g", Env: "e", Kind: model.BatchOpKindExec, AgentIDs: []string{"a"}, Exec: &ExecPayload{Command: "ls"}})
	require.NoError(t, err)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	assert.Equal(t, []string{string(audit.EventBatchOpCreate)}, rec.events)
	assert.Equal(t, model.BatchOpStatusDraft, op.Status)
}

// ---- Execute 失败臂 ----

type failingSelector struct{ err error }

func (f *failingSelector) Select(context.Context, string, string, map[string]string, []string) ([]string, error) {
	return nil, f.err
}

// knobStore 支持 Create/Find/CAS 注错与「前 N 次 UpdateStatus 成功、其余失败」。
type knobStore struct {
	mu sync.Mutex

	op        *model.BatchOp
	next      uint
	createErr error
	findErr   error
	casErr    error
	// failUpdateAfter: 前 N 次 UpdateStatus 成功，其后全部失败（0 = 首次即失败）。
	failUpdateAfter int
	updateErr       error
	updateCalls     int
}

func (f *knobStore) Create(_ context.Context, op *model.BatchOp) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.next++
	op.ID = f.next
	f.op = op
	return nil
}

func (f *knobStore) Find(_ context.Context, _, _, _ string) (*model.BatchOp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.op == nil {
		return nil, errors.New("not found")
	}
	return f.op, nil
}

func (f *knobStore) List(_ context.Context, _, _ string, _ int) ([]model.BatchOp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.op == nil {
		return nil, nil
	}
	return []model.BatchOp{*f.op}, nil
}

func (f *knobStore) UpdateStatus(_ context.Context, id uint, status string, fields map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateCalls++
	if f.updateCalls > f.failUpdateAfter {
		if f.updateErr != nil {
			return f.updateErr
		}
		return errors.New("update failpoint")
	}
	if f.op != nil && f.op.ID == id {
		f.op.Status = status
		if v, ok := fields["targets"].(int); ok {
			f.op.Targets = v
		}
		if v, ok := fields["succeeded"].(int); ok {
			f.op.Succeeded = v
		}
		if v, ok := fields["failed"].(int); ok {
			f.op.Failed = v
		}
		if v, ok := fields["skipped"].(int); ok {
			f.op.Skipped = v
		}
	}
	return nil
}

func (f *knobStore) CompareAndSwapStatus(_ context.Context, id uint, from, to string) (bool, *model.BatchOp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.casErr != nil {
		return false, nil, f.casErr
	}
	if f.op == nil || f.op.ID != id {
		return false, nil, errors.New("not found")
	}
	if f.op.Status != from {
		return false, f.op, nil
	}
	f.op.Status = to
	return true, f.op, nil
}

// Execute：Find 失败（177）、CAS 失败（182）、选址失败落 failed（195）、
// targets 更新失败（202）、聚合回写失败同时返回结果与错误（265）、审计
// （268），Get/List 透传（275/280）。
func TestExecute_StoreErrorArms(t *testing.T) {
	ctx := context.Background()

	// Find 失败：未知批次。
	o := New(&fakeSelector{}, &fakeExecutor{}, newFakeStore(), nil)
	_, err := o.Execute(ctx, "g", "e", "missing", false)
	require.Error(t, err)

	// CAS 失败。
	casErr := errors.New("cas down")
	knob := &knobStore{casErr: casErr}
	require.NoError(t, knob.Create(ctx, &model.BatchOp{BatchID: "b-cas", GameID: "g", Env: "e", Kind: model.BatchOpKindExec, Status: model.BatchOpStatusDraft}))
	_, err = New(&fakeSelector{agents: []string{"agent-1"}}, &fakeExecutor{}, knob, nil).Execute(ctx, "g", "e", "b-cas", false)
	require.ErrorIs(t, err, casErr)

	// 选址失败：批次落 failed 并回传错误（selectors 须合法才能走到 Select）。
	selErr := errors.New("selector down")
	knob = &knobStore{failUpdateAfter: 10}
	require.NoError(t, knob.Create(ctx, seededRawOp("b-sel", model.BatchOpKindExec, `{"command":"ls"}`, `["agent-1"]`)))
	_, err = New(&failingSelector{err: selErr}, &fakeExecutor{}, knob, nil).Execute(ctx, "g", "e", "b-sel", false)
	require.ErrorIs(t, err, selErr)
	knob.mu.Lock()
	assert.Equal(t, model.BatchOpStatusFailed, knob.op.Status)
	knob.mu.Unlock()

	// targets 更新失败：首轮 UpdateStatus 即注错。
	knob = &knobStore{failUpdateAfter: 0}
	require.NoError(t, knob.Create(ctx, seededRawOp("b-tgt", model.BatchOpKindExec, `{"command":"ls"}`, `["agent-1"]`)))
	_, err = New(&fakeSelector{agents: []string{"agent-1"}}, &fakeExecutor{}, knob, nil).Execute(ctx, "g", "e", "b-tgt", false)
	require.Error(t, err)

	// 聚合回写失败：结果与错误同时返回（执行本身已完成）。
	knob = &knobStore{failUpdateAfter: 1}
	require.NoError(t, knob.Create(ctx, seededRawOp("b-final", model.BatchOpKindExec, `{"command":"ls"}`, `["agent-1"]`)))
	res, err := New(&fakeSelector{agents: []string{"agent-1"}}, &fakeExecutor{}, knob, nil).Execute(ctx, "g", "e", "b-final", false)
	require.Error(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 1, res.Succeeded)

	// Get / List 透传。
	got, err := New(&fakeSelector{}, &fakeExecutor{}, knob, nil).Get(ctx, "g", "e", "b-final")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "b-final", got.BatchID)
	rows, err := New(&fakeSelector{}, &fakeExecutor{}, knob, nil).List(ctx, "g", "e", 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

// Execute：dry-run 状态回写失败（213）与 dry-run 审计（216）、执行审计（268）。
func TestExecute_DryRunUpdateErrorAndAudit(t *testing.T) {
	ctx := context.Background()

	// 前 1 次 UpdateStatus（targets）成功，dry-run 状态回写注错。
	knob := &knobStore{failUpdateAfter: 1}
	rec := &auditRecorder{}
	req := &CreateRequest{GameID: "g", Env: "e", Kind: model.BatchOpKindExec, AgentIDs: []string{"agent-1"}, DryRun: true, Exec: &ExecPayload{Command: "ls"}}
	require.NoError(t, knob.Create(ctx, &model.BatchOp{
		BatchID: "b-dry", GameID: "g", Env: "e", Kind: model.BatchOpKindExec,
		LabelsJSON: "{}", AgentsJSON: `["agent-1"]`, PayloadJSON: `{"command":"ls"}`,
		DryRun: true, Status: model.BatchOpStatusDraft,
	}))
	_, err := New(&fakeSelector{agents: []string{"agent-1"}}, &fakeExecutor{}, knob, rec.record).Execute(ctx, "g", "e", "b-dry", false)
	require.Error(t, err)
	knob.mu.Lock()
	assert.Equal(t, model.BatchOpStatusRunning, knob.op.Status, "dry-run 回写失败时状态停在 running")
	knob.mu.Unlock()
	rec.mu.Lock()
	assert.Empty(t, rec.events, "状态回写失败不得记成功审计")
	rec.mu.Unlock()

	// dry-run 成功路径记 batch_op.dry_run 审计。
	rec2 := &auditRecorder{}
	o := New(&fakeSelector{agents: []string{"agent-1"}}, &fakeExecutor{}, newFakeStore(), rec2.record)
	op, err := o.Create(ctx, req)
	require.NoError(t, err)
	_, err = o.Execute(ctx, "g", "e", op.BatchID, false)
	require.NoError(t, err)
	rec2.mu.Lock()
	assert.Equal(t, []string{string(audit.EventBatchOpCreate), string(audit.EventBatchOpDryRun)}, rec2.events)
	rec2.mu.Unlock()

	// 执行成功路径记 batch_op.execute 审计。
	rec3 := &auditRecorder{}
	o = New(&fakeSelector{agents: []string{"agent-1"}}, &fakeExecutor{}, newFakeStore(), rec3.record)
	op, err = o.Create(ctx, &CreateRequest{GameID: "g", Env: "e", Kind: model.BatchOpKindExec, AgentIDs: []string{"agent-1"}, Exec: &ExecPayload{Command: "ls"}})
	require.NoError(t, err)
	_, err = o.Execute(ctx, "g", "e", op.BatchID, false)
	require.NoError(t, err)
	rec3.mu.Lock()
	assert.Equal(t, []string{string(audit.EventBatchOpCreate), string(audit.EventBatchOpExecute)}, rec3.events)
	rec3.mu.Unlock()
}

// ---- dispatch 臂 ----

type exitCodeExecutor struct{ code int32 }

func (f *exitCodeExecutor) ExecuteCommand(context.Context, string, *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
	return &opsv1.ExecuteCommandResponse{ExitCode: f.code, StdOut: "warn out"}, nil
}

func (f *exitCodeExecutor) PushFile(context.Context, string, *opsv1.PushFileRequest) (*opsv1.PushFileResponse, error) {
	return nil, errors.New("push unused")
}

type pushRejectExecutor struct {
	success bool
	message string
	err     error
}

func (f *pushRejectExecutor) ExecuteCommand(context.Context, string, *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
	return nil, errors.New("exec unused")
}

func (f *pushRejectExecutor) PushFile(context.Context, string, *opsv1.PushFileRequest) (*opsv1.PushFileResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &opsv1.PushFileResponse{Success: f.success, Message: f.message}, nil
}

// seededRawOp 绕过 Create 校验直接铺库内行（decode/未知 kind 臂专用）。
func seededRawOp(batchID, kind, payloadJSON, agentsJSON string) *model.BatchOp {
	return &model.BatchOp{
		BatchID: batchID, GameID: "g", Env: "e", Kind: kind,
		LabelsJSON: "{}", AgentsJSON: agentsJSON, PayloadJSON: payloadJSON,
		Status: model.BatchOpStatusDraft,
	}
}

// dispatch：exec 载荷损坏（288）、非零退出（300）、config 载荷损坏（307）、
// bad base64（311）、push 失败（321）、push 拒绝（324）、未知 kind（328）、
// agentsJSON 损坏（365）。
func TestExecute_DispatchArms(t *testing.T) {
	ctx := context.Background()

	// exec 载荷损坏 → 目标 failed。
	knob := &knobStore{op: seededRawOp("b-badexec", model.BatchOpKindExec, "not-json", `["agent-1"]`), failUpdateAfter: 10}
	res, err := New(&fakeSelector{agents: []string{"agent-1"}}, &fakeExecutor{}, knob, nil).Execute(ctx, "g", "e", "b-badexec", false)
	require.NoError(t, err)
	require.Len(t, res.Items, 1)
	assert.Equal(t, "failed", res.Items[0].Status)
	assert.Equal(t, 1, res.Failed)

	// 非零退出码 → failed + ExitCode 透传。
	o := New(&fakeSelector{agents: []string{"agent-1"}}, &exitCodeExecutor{code: 7}, newFakeStore(), nil)
	op, err := o.Create(ctx, &CreateRequest{GameID: "g", Env: "e", Kind: model.BatchOpKindExec, AgentIDs: []string{"agent-1"}, Exec: &ExecPayload{Command: "false"}})
	require.NoError(t, err)
	res, err = o.Execute(ctx, "g", "e", op.BatchID, false)
	require.NoError(t, err)
	require.Len(t, res.Items, 1)
	assert.Equal(t, "failed", res.Items[0].Status)
	assert.Equal(t, int32(7), res.Items[0].ExitCode)
	assert.Equal(t, model.BatchOpStatusFailed, res.Status)

	// config 载荷损坏 → failed。
	knob = &knobStore{op: seededRawOp("b-badcfg", model.BatchOpKindConfig, "not-json", `["agent-1"]`), failUpdateAfter: 10}
	res, err = New(&fakeSelector{agents: []string{"agent-1"}}, &fakeExecutor{}, knob, nil).Execute(ctx, "g", "e", "b-badcfg", false)
	require.NoError(t, err)
	require.Len(t, res.Items, 1)
	assert.Equal(t, "failed", res.Items[0].Status)

	// bad base64：Create 只校验非空，解码失败落目标 failed。
	o = New(&fakeSelector{agents: []string{"agent-1"}}, &fakeExecutor{}, newFakeStore(), nil)
	op, err = o.Create(ctx, &CreateRequest{GameID: "g", Env: "e", Kind: model.BatchOpKindConfig, AgentIDs: []string{"agent-1"}, Config: &ConfigPayload{Path: "/etc/app.yaml", ContentBase64: "%%not-base64%%"}})
	require.NoError(t, err)
	res, err = o.Execute(ctx, "g", "e", op.BatchID, false)
	require.NoError(t, err)
	require.Len(t, res.Items, 1)
	assert.Equal(t, "failed", res.Items[0].Status)
	assert.Equal(t, "bad base64", res.Items[0].Message)

	// PushFile 传输错误 → failed。
	o = New(&fakeSelector{agents: []string{"agent-1"}}, &pushRejectExecutor{err: errors.New("push refused")}, newFakeStore(), nil)
	op, err = o.Create(ctx, &CreateRequest{GameID: "g", Env: "e", Kind: model.BatchOpKindConfig, AgentIDs: []string{"agent-1"}, Config: &ConfigPayload{Path: "/etc/app.yaml", ContentBase64: "aGVsbG8="}})
	require.NoError(t, err)
	res, err = o.Execute(ctx, "g", "e", op.BatchID, false)
	require.NoError(t, err)
	require.Len(t, res.Items, 1)
	assert.Equal(t, "failed", res.Items[0].Status)
	assert.Contains(t, res.Items[0].Message, "push refused")

	// PushFile 返回 success=false → failed + 消息透传。
	o = New(&fakeSelector{agents: []string{"agent-1"}}, &pushRejectExecutor{success: false, message: "sha mismatch"}, newFakeStore(), nil)
	op, err = o.Create(ctx, &CreateRequest{GameID: "g", Env: "e", Kind: model.BatchOpKindConfig, AgentIDs: []string{"agent-1"}, Config: &ConfigPayload{Path: "/etc/app.yaml", ContentBase64: "aGVsbG8="}})
	require.NoError(t, err)
	res, err = o.Execute(ctx, "g", "e", op.BatchID, false)
	require.NoError(t, err)
	require.Len(t, res.Items, 1)
	assert.Equal(t, "failed", res.Items[0].Status)
	assert.Equal(t, "sha mismatch", res.Items[0].Message)

	// 未知 kind（库内行直接铺入）→ dispatch 默认臂 failed。
	knob = &knobStore{op: seededRawOp("b-kind", "restart-apps", "{}", `["agent-1"]`), failUpdateAfter: 10}
	res, err = New(&fakeSelector{agents: []string{"agent-1"}}, &fakeExecutor{}, knob, nil).Execute(ctx, "g", "e", "b-kind", false)
	require.NoError(t, err)
	require.Len(t, res.Items, 1)
	assert.Equal(t, "failed", res.Items[0].Status)
	assert.Equal(t, "unknown kind", res.Items[0].Message)

	// agentsJSON 损坏：labels 合法、agents 解码失败 → Execute 报错落 failed。
	knob = &knobStore{op: seededRawOp("b-agents", model.BatchOpKindExec, "{}", "not-json"), failUpdateAfter: 10}
	_, err = New(&fakeSelector{agents: []string{"agent-1"}}, &fakeExecutor{}, knob, nil).Execute(ctx, "g", "e", "b-agents", false)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid character"), "agentsJSON 解码错误透传, got %v", err)
	knob.mu.Lock()
	assert.Equal(t, model.BatchOpStatusFailed, knob.op.Status)
	knob.mu.Unlock()
}
