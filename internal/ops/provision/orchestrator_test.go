package provision

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner 记录调用并返回预设输出；Init/Apply/Destroy 全部走 Fake 路径。
type fakeRunner struct {
	mu       sync.Mutex
	applied  int
	inited   int
	destroy  int
	outputs  map[string]string
	applyErr error
	lastVars map[string]string
}

func (f *fakeRunner) Init(_ context.Context, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inited++
	return nil
}

func (f *fakeRunner) Apply(_ context.Context, _ string, vars map[string]string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applied++
	f.lastVars = vars
	if f.applyErr != nil {
		return nil, f.applyErr
	}
	return f.outputs, nil
}

func (f *fakeRunner) Destroy(_ context.Context, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.destroy++
	return nil
}

// fakeRegistry 返回注册结果（空=超时）。记录等待的标签键值。
type fakeRegistry struct {
	ids        []string
	delay      time.Duration
	callCount  int
	labelKey   string
	labelValue string
}

func (f *fakeRegistry) WaitRegistered(_ context.Context, labelKey, labelValue string, _ time.Duration) ([]string, error) {
	f.callCount++
	f.labelKey = labelKey
	f.labelValue = labelValue
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return f.ids, nil
}

// fakeApproval 记录创建并返回批准状态。
type fakeApproval struct {
	mu        sync.Mutex
	created   []*Request
	createdID string
	approved  bool
}

func (f *fakeApproval) CreateApproval(_ context.Context, req *Request, runID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, req)
	f.createdID = runID
	return "approval-1", nil
}

func (f *fakeApproval) Approved(_ context.Context, _ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.approved, nil
}

// fakeStore 是 map 背书的 RunStore。
type fakeStore struct {
	mu   sync.Mutex
	runs map[string]*Run
}

func newFakeStore() *fakeStore { return &fakeStore{runs: map[string]*Run{}} }

func (s *fakeStore) Create(_ context.Context, r *Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *r
	s.runs[r.ID] = &cp
	return nil
}

func (s *fakeStore) Update(_ context.Context, r *Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *r
	s.runs[r.ID] = &cp
	return nil
}

func (s *fakeStore) Get(_ context.Context, id string) (*Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	cp := *r
	return &cp, nil
}

func (s *fakeStore) List(_ context.Context, _ int) ([]*Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Run, 0, len(s.runs))
	for _, r := range s.runs {
		cp := *r
		out = append(out, &cp)
	}
	return out, nil
}

func newOrchestrator(t *testing.T, r *fakeRunner, reg *fakeRegistry, app *fakeApproval, st *fakeStore) *Orchestrator {
	t.Helper()
	o := &Orchestrator{
		Runner:          r,
		Registry:        reg,
		Approval:        app,
		Store:           st,
		WorkDir:         t.TempDir(),
		InstallURL:      "https://dl.example.com/agent.sh",
		RegisterTimeout: time.Second,
	}
	return o
}

func baseRequest() *Request {
	return &Request{
		Provider:       ProviderAWS,
		Region:         "us-east-1",
		Count:          1,
		Spec:           Spec{InstanceType: "t3.micro"},
		GameID:         "demo",
		Env:            "prod",
		HubAddr:        "hub:19090",
		BootstrapToken: "tok-1",
	}
}

func TestSubmit_Direct_RunsFullLoopAndAdopts(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{"instance_id": `"i-abc"`}}
	reg := &fakeRegistry{ids: []string{`"i-abc"`}}
	st := newFakeStore()
	o := newOrchestrator(t, runner, reg, &fakeApproval{}, st)

	run, err := o.Submit(context.Background(), baseRequest())
	require.NoError(t, err)
	assert.Equal(t, StateAdopted, run.State)
	assert.Equal(t, []string{`"i-abc"`}, run.AgentIDs)
	assert.Equal(t, `"i-abc"`, run.Outputs["instance_id"])
	assert.Equal(t, 1, runner.applied)
	assert.Equal(t, 1, runner.inited)
	assert.Equal(t, 1, reg.callCount)
	// apply vars 必须携带 name_prefix/disk_size/user_data（值经 apply 注入模板）。
	assert.Equal(t, "croupier-demo-prod", runner.lastVars["name_prefix"])
	assert.Equal(t, "40", runner.lastVars["disk_size"])
	assert.Contains(t, runner.lastVars["user_data"], "bootstrapToken: tok-1")
	// user_data 内含供给标签（等待注册按标签匹配），等待调用也用同键值。
	assert.Contains(t, runner.lastVars["user_data"], "croupier-provisioned: "+run.ID)
	assert.Equal(t, ProvisionLabelKey, reg.labelKey)
	assert.Equal(t, run.ID, reg.labelValue)
	// 工作目录必须写了 main.tf（不含 token——user_data 不落工作目录）。
	_, err = os.Stat(filepath.Join(o.WorkDir, run.ID, "main.tf"))
	require.NoError(t, err)
	mainTF, err := os.ReadFile(filepath.Join(o.WorkDir, run.ID, "main.tf"))
	require.NoError(t, err)
	assert.NotContains(t, string(mainTF), "tok-1")
}

func TestSubmit_Ticket_PendingThenResumeAdopts(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{"instance_id": `"i-2"`}}
	reg := &fakeRegistry{ids: []string{`"i-2"`}}
	app := &fakeApproval{}
	st := newFakeStore()
	o := newOrchestrator(t, runner, reg, app, st)

	req := baseRequest()
	req.Policy = PolicyTicket
	run, err := o.Submit(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, StatePending, run.State)
	assert.Equal(t, "approval-1", run.ApprovalID)
	assert.Equal(t, 0, runner.applied) // 未批准不执行
	require.Len(t, app.created, 1)
	assert.Equal(t, PolicyTicket, app.created[0].Policy)

	// 未批准时 Resume 拒绝。
	_, err = o.Resume(context.Background(), run.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet approved")

	// 批准后 Resume 全链。
	app.approved = true
	run2, err := o.Resume(context.Background(), run.ID)
	require.NoError(t, err)
	assert.Equal(t, StateAdopted, run2.State)
	assert.Equal(t, 1, runner.applied)
}

func TestCreate_ApplyFailure_StateFailed(t *testing.T) {
	runner := &fakeRunner{applyErr: os.ErrInvalid}
	reg := &fakeRegistry{}
	st := newFakeStore()
	o := newOrchestrator(t, runner, reg, &fakeApproval{}, st)

	run, err := o.Create(context.Background(), baseRequest())
	require.Error(t, err)
	assert.Equal(t, StateFailed, run.State)
	assert.NotEmpty(t, run.Error)
}

func TestCreate_NoRegister_StateFailed(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{"instance_id": `"i-3"`}}
	reg := &fakeRegistry{} // 空注册
	st := newFakeStore()
	o := newOrchestrator(t, runner, reg, &fakeApproval{}, st)

	run, err := o.Create(context.Background(), baseRequest())
	require.Error(t, err)
	assert.Equal(t, StateFailed, run.State)
	assert.Contains(t, run.Error, "wait register")
}

func TestDestroy_Transitions(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{"instance_id": `"i-4"`}}
	reg := &fakeRegistry{ids: []string{`"i-4"`}}
	st := newFakeStore()
	o := newOrchestrator(t, runner, reg, &fakeApproval{}, st)

	run, err := o.Create(context.Background(), baseRequest())
	require.NoError(t, err)
	require.NoError(t, o.Destroy(context.Background(), run.ID))
	got, _ := st.Get(context.Background(), run.ID)
	assert.Equal(t, StateDestroyed, got.State)
	assert.Equal(t, 1, runner.destroy)
}

func TestValidate_Nil(t *testing.T) {
	_, err := Validate(nil)
	require.Error(t, err)
}

// waitState 轮询 store 至 run 达到目标状态（异步 apply 后台执行，无返回通道）。
func waitState(t *testing.T, st *fakeStore, id string, want State, timeout time.Duration) *Run {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		r, err := st.Get(context.Background(), id)
		require.NoError(t, err)
		if r.State == want {
			return r
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s state=%s, want %s (timeout)", id, r.State, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestSubmitAsync_Direct_ReturnsApplyingThenAdopts(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{"instance_id": `"i-a"`}}
	reg := &fakeRegistry{ids: []string{`"i-a"`}}
	st := newFakeStore()
	o := newOrchestrator(t, runner, reg, &fakeApproval{}, st)

	run, err := o.SubmitAsync(context.Background(), baseRequest())
	require.NoError(t, err)
	// 立即返回：已持久化 applying（请求未阻塞在 apply/等注册）。
	assert.Equal(t, StateApplying, run.State)
	assert.NotEmpty(t, run.ID)

	adopted := waitState(t, st, run.ID, StateAdopted, 2*time.Second)
	assert.Equal(t, []string{`"i-a"`}, adopted.AgentIDs)
	assert.Equal(t, 1, runner.applied)
}

func TestSubmitAsync_Ticket_ReturnsPendingNoApply(t *testing.T) {
	runner := &fakeRunner{}
	reg := &fakeRegistry{}
	app := &fakeApproval{}
	st := newFakeStore()
	o := newOrchestrator(t, runner, reg, app, st)

	req := baseRequest()
	req.Policy = PolicyTicket
	run, err := o.SubmitAsync(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, StatePending, run.State)
	assert.Equal(t, 0, runner.applied)
	assert.Equal(t, "approval-1", run.ApprovalID)
}

func TestResumeAsync_NotApproved_Rejected(t *testing.T) {
	runner := &fakeRunner{}
	app := &fakeApproval{} // 未批准
	st := newFakeStore()
	o := newOrchestrator(t, runner, &fakeRegistry{}, app, st)

	req := baseRequest()
	req.Policy = PolicyTicket
	run, err := o.Submit(context.Background(), req)
	require.NoError(t, err)
	_, err = o.ResumeAsync(context.Background(), run.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet approved")
}

func TestResumeAsync_DoubleResume_SecondRejected(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{"instance_id": `"i-b"`}}
	reg := &fakeRegistry{ids: []string{`"i-b"`}}
	app := &fakeApproval{approved: true}
	st := newFakeStore()
	o := newOrchestrator(t, runner, reg, app, st)

	req := baseRequest()
	req.Policy = PolicyTicket
	run, err := o.Submit(context.Background(), req)
	require.NoError(t, err)

	first, err := o.ResumeAsync(context.Background(), run.ID)
	require.NoError(t, err)
	assert.Equal(t, StateApplying, first.State)
	// 状态闸：第二个续行读到已非 pending，必须被拒（防双跑 terraform）。
	_, err = o.ResumeAsync(context.Background(), run.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not pending")

	adopted := waitState(t, st, run.ID, StateAdopted, 2*time.Second)
	assert.Equal(t, 1, runner.applied)
	assert.Equal(t, StateAdopted, adopted.State)
}
