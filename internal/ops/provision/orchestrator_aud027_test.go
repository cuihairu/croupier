package provision

// AUD-027 回归：CAS 竞争失败（另一调用在 Get 与 CAS 之间已把 state 改走）时，
// 失败分支不得解引用 nil 当前行（真实 DB 适配器曾在此丢弃当前行，返回 nil），
// ResumeAsync/Destroy/DestroyResume 三处直接读 current.State 会 nil pointer
// panic。修复形态：适配层如实回读当前行（见 internal/svc provision_wiring_test
// 的 CAS 契约用例）+ 调用方经 casConflict 安全格式化（current nil 时不带 state）。

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// casLostNilStore 模拟真实 DB 适配器在 CAS 竞争失败时的旧缺陷形态：Get 仍读到
// CAS 前的状态快照（调用方预检通过），但 CompareAndSwapState 失败且当前行 nil
// （旧 provisionDBStore 丢弃当前行）。这复现「另一调用先领取」的竞态窗口——
// 调用方失败分支直接读 current.State 即 panic（AUD-027）。
type casLostNilStore struct {
	*fakeStore
}

func (s *casLostNilStore) CompareAndSwapState(context.Context, string, string, string) (bool, *Run, error) {
	return false, nil, nil
}

// TestResumeAsync_CASConflictNilCurrent_NoPanic 覆盖 beginResume（ResumeAsync）
// 的 CAS 失败分支：current 为 nil 时返回稳定冲突错误（ErrRunNotPending），不带
// state 段，且不 panic。
func TestResumeAsync_CASConflictNilCurrent_NoPanic(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{"instance_id": `"i-nil"`}}
	reg := &fakeRegistry{ids: []string{`"i-nil"`}}
	app := &fakeApproval{approved: true}
	st := &casLostNilStore{fakeStore: newFakeStore()}
	o := newOrchestrator(t, runner, reg, app, st)

	req := baseRequest()
	req.Policy = PolicyTicket
	run, err := o.Submit(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, StatePending, run.State)

	// 另一调用在 Get 与 CAS 之间已把 state 改走：CAS 失败、当前行 nil。
	_, err = o.ResumeAsync(context.Background(), run.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRunNotPending)
	assert.Contains(t, err.Error(), "run not pending")
	assert.NotContains(t, err.Error(), "(state=") // current nil：不带 state 的稳定冲突
	assert.Equal(t, 0, runner.applied)            // 状态闸：未执行 apply
}

// TestDestroy_CASConflictNilCurrent_NoPanic 覆盖 Destroy 的 CAS 失败分支：
// current 为 nil 时返回稳定冲突错误（ErrRunNotDestroyable），不 panic。
func TestDestroy_CASConflictNilCurrent_NoPanic(t *testing.T) {
	runner := &fakeRunner{}
	st := &casLostNilStore{fakeStore: newFakeStore()}
	o := newOrchestrator(t, runner, &fakeRegistry{ids: []string{`"i-x"`}}, &fakeApproval{}, st)

	run, err := o.Create(context.Background(), baseRequest())
	require.NoError(t, err)
	require.Equal(t, StateAdopted, run.State)

	err = o.Destroy(context.Background(), run.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRunNotDestroyable)
	assert.Contains(t, err.Error(), "run not destroyable")
	assert.NotContains(t, err.Error(), "(state=")
	assert.Equal(t, 0, runner.destroy) // 状态闸：未执行 terraform destroy
}

// TestDestroyResume_CASConflictNilCurrent_NoPanic 覆盖 DestroyResume 的 CAS 失败
// 分支：current 为 nil 时返回稳定冲突错误（ErrRunNotDestroyPending），不 panic。
func TestDestroyResume_CASConflictNilCurrent_NoPanic(t *testing.T) {
	runner := &fakeRunner{}
	app := &fakeApproval{approved: true}
	st := &casLostNilStore{fakeStore: newFakeStore()}
	o := newOrchestrator(t, runner, &fakeRegistry{ids: []string{`"i-x"`}}, app, st)

	run, err := o.Create(context.Background(), baseRequest())
	require.NoError(t, err)
	_, err = o.RequestDestroy(context.Background(), run.ID)
	require.NoError(t, err)

	_, err = o.DestroyResume(context.Background(), run.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRunNotDestroyPending)
	assert.Contains(t, err.Error(), "not destroy pending")
	assert.NotContains(t, err.Error(), "(state=")
	assert.Equal(t, 0, runner.destroy)
}
