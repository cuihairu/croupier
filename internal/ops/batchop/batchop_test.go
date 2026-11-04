package batchop

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSelector struct{ agents []string }

func (f *fakeSelector) Select(ctx context.Context, gameID, env string, labels map[string]string, agentIDs []string) ([]string, error) {
	return f.agents, nil
}

type fakeExecutor struct {
	mu   sync.Mutex
	exec []string
	push []string
	fail map[string]bool
}

func (f *fakeExecutor) ExecuteCommand(ctx context.Context, agentID string, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
	f.mu.Lock()
	f.exec = append(f.exec, agentID)
	fail := f.fail[agentID]
	f.mu.Unlock()
	if fail {
		return nil, errors.New("boom")
	}
	return &opsv1.ExecuteCommandResponse{ExitCode: 0, StdOut: "ok"}, nil
}

func (f *fakeExecutor) PushFile(ctx context.Context, agentID string, req *opsv1.PushFileRequest) (*opsv1.PushFileResponse, error) {
	f.mu.Lock()
	f.push = append(f.push, agentID)
	fail := f.fail[agentID]
	f.mu.Unlock()
	if fail {
		return nil, errors.New("push boom")
	}
	return &opsv1.PushFileResponse{Success: true, BytesWritten: int64(len(req.Content))}, nil
}

type fakeStore struct {
	mu   sync.Mutex
	ops  map[string]*model.BatchOp
	byID map[uint]*model.BatchOp
	next uint
}

func newFakeStore() *fakeStore {
	return &fakeStore{ops: map[string]*model.BatchOp{}, byID: map[uint]*model.BatchOp{}}
}

func (f *fakeStore) Create(ctx context.Context, op *model.BatchOp) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	op.ID = f.next
	f.ops[op.BatchID] = op
	f.byID[op.ID] = op
	return nil
}
func (f *fakeStore) Find(ctx context.Context, gameID, env, batchID string) (*model.BatchOp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	op, ok := f.ops[batchID]
	if !ok {
		return nil, errors.New("not found")
	}
	return op, nil
}
func (f *fakeStore) List(ctx context.Context, gameID, env string, limit int) ([]model.BatchOp, error) {
	return nil, nil
}
func (f *fakeStore) UpdateStatus(ctx context.Context, id uint, status string, fields map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	op := f.byID[id]
	if op != nil {
		op.Status = status
		if fields != nil {
			if v, ok := fields["targets"].(int); ok {
				op.Targets = v
			}
			if v, ok := fields["succeeded"].(int); ok {
				op.Succeeded = v
			}
			if v, ok := fields["failed"].(int); ok {
				op.Failed = v
			}
			if v, ok := fields["skipped"].(int); ok {
				op.Skipped = v
			}
		}
	}
	return nil
}

func (f *fakeStore) CompareAndSwapStatus(ctx context.Context, id uint, from, to string) (bool, *model.BatchOp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	op := f.byID[id]
	if op == nil {
		return false, nil, errors.New("not found")
	}
	if op.Status != from {
		return false, op, nil
	}
	op.Status = to
	return true, op, nil
}

func TestCreate_ValidatesPayload(t *testing.T) {
	o := New(&fakeSelector{}, &fakeExecutor{}, newFakeStore(), nil)
	_, err := o.Create(context.Background(), &CreateRequest{GameID: "g", Env: "e", Kind: "config", AgentIDs: []string{"a"}})
	require.Error(t, err) // config payload missing
	_, err = o.Create(context.Background(), &CreateRequest{GameID: "g", Env: "e", Kind: "exec", AgentIDs: []string{"a"}})
	require.Error(t, err) // exec command missing
	_, err = o.Create(context.Background(), &CreateRequest{GameID: "g", Env: "e", Kind: "exec", AgentIDs: []string{"a"}, Exec: &ExecPayload{Command: "ls"}})
	require.NoError(t, err)
}

func TestExecute_ExecBatchAggregates(t *testing.T) {
	store := newFakeStore()
	exec := &fakeExecutor{fail: map[string]bool{"agent-2": true}}
	o := New(&fakeSelector{agents: []string{"agent-1", "agent-2", "agent-3"}}, exec, store, nil)
	op, err := o.Create(context.Background(), &CreateRequest{
		GameID: "g", Env: "e", Kind: "exec", AgentIDs: []string{"agent-1", "agent-2", "agent-3"},
		Exec: &ExecPayload{Command: "echo", Args: []string{"hi"}, TimeoutSeconds: 5},
	})
	require.NoError(t, err)

	res, err := o.Execute(context.Background(), "g", "e", op.BatchID, false)
	require.NoError(t, err)
	assert.Equal(t, 3, res.Targets)
	assert.Equal(t, 2, res.Succeeded)
	assert.Equal(t, 1, res.Failed)
	assert.Equal(t, model.BatchOpStatusFailed, res.Status)
	require.Len(t, res.Items, 3)
	assert.Len(t, exec.exec, 3)
}

func TestExecute_ConfigBatchPush(t *testing.T) {
	store := newFakeStore()
	exec := &fakeExecutor{}
	o := New(&fakeSelector{agents: []string{"agent-1", "agent-2"}}, exec, store, nil)
	content := base64.StdEncoding.EncodeToString([]byte("hello"))
	op, err := o.Create(context.Background(), &CreateRequest{
		GameID: "g", Env: "e", Kind: "config", AgentIDs: []string{"agent-1", "agent-2"},
		Config: &ConfigPayload{Path: "/etc/conf/app.yaml", ContentBase64: content, Mode: 0o644},
	})
	require.NoError(t, err)
	res, err := o.Execute(context.Background(), "g", "e", op.BatchID, false)
	require.NoError(t, err)
	assert.Equal(t, 2, res.Succeeded)
	assert.Len(t, exec.push, 2)
}

func TestExecute_DryRunDoesNotDispatch(t *testing.T) {
	store := newFakeStore()
	exec := &fakeExecutor{}
	o := New(&fakeSelector{agents: []string{"agent-1", "agent-2"}}, exec, store, nil)
	op, err := o.Create(context.Background(), &CreateRequest{
		GameID: "g", Env: "e", Kind: "exec", AgentIDs: []string{"agent-1", "agent-2"}, DryRun: true,
		Exec: &ExecPayload{Command: "echo"},
	})
	require.NoError(t, err)
	res, err := o.Execute(context.Background(), "g", "e", op.BatchID, false)
	require.NoError(t, err)
	assert.Equal(t, model.BatchOpStatusDryRun, res.Status)
	assert.Len(t, exec.exec, 0)
	assert.Equal(t, 2, res.Targets)
	for _, it := range res.Items {
		assert.Equal(t, "dry_run", it.Status)
	}
}

// #664：已执行（completed/failed）批次不得重复派发——draft→running CAS 状态闸。
func TestExecute_CompletedBatchRejected(t *testing.T) {
	store := newFakeStore()
	exec := &fakeExecutor{}
	o := New(&fakeSelector{agents: []string{"agent-1"}}, exec, store, nil)
	op, err := o.Create(context.Background(), &CreateRequest{
		GameID: "g", Env: "e", Kind: "exec", AgentIDs: []string{"agent-1"},
		Exec: &ExecPayload{Command: "echo"},
	})
	require.NoError(t, err)
	_, err = o.Execute(context.Background(), "g", "e", op.BatchID, false)
	require.NoError(t, err)

	// 第二次执行：批次已 completed，状态闸拒绝，不再派发。
	_, err = o.Execute(context.Background(), "g", "e", op.BatchID, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already completed")
	assert.Len(t, exec.exec, 1)
}
