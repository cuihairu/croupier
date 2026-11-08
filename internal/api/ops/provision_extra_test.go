package ops

// provision.go 补测：provisionServerParams hub 缺失、Resume/List 的边界与
// 非冲突错误透传、Destroy 的 store 错误与 runner 失败、workflow 销毁申请
// 的护栏/状态冲突、DestroyResume 非冲突错误、protection 三分支、以及
// handler 的坏载荷 400 面与 Resume/Get 成功面。
//
// 复用 provision_test.go 的 newProvisionService/memStore/inertRunner/
// emptyRegistry/fixedApproval；runner/store 故障用本地替身注入。

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/ops/provision"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// errRunner Destroy 恒失败的 runner（销毁故障分支）。
type errRunner struct{ inertRunner }

func (errRunner) Destroy(context.Context, string) error {
	return errors.New("tofu destroy failed")
}

// failStore 包装 memStore，Update 注入失败（protection 落库错误分支）。
type failStore struct {
	*memStore
	updateErr error
}

func (s *failStore) Update(ctx context.Context, r *provision.Run) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	return s.memStore.Update(ctx, r)
}

// provisionService 按需组装 Service（workflow/auto 审批模式可注入）。
func provisionService(t *testing.T, approval string, orch *provision.Orchestrator) *Service {
	t.Helper()
	return NewService(&svc.ServiceContext{
		Config:    config.Config{Provision: config.ProvisionConfig{Enabled: true, HubAddr: "hub:19090", Approval: approval}},
		Provision: orch,
	})
}

func TestProvisionServerParamsHubMissing(t *testing.T) {
	s := NewService(&svc.ServiceContext{Config: config.Config{Provision: config.ProvisionConfig{}}})
	err := s.provisionServerParams(&provision.Request{})
	require.ErrorContains(t, err, "hub")
}

func TestOpsProvisionResumeBranches(t *testing.T) {
	ctx := context.Background()

	// 非冲突错误透传（store 缺失 → gorm.ErrRecordNotFound）
	s := newProvisionService(t, true, false, &memStore{runs: map[string]*provision.Run{}})
	_, err := s.OpsProvisionResume(ctx, "missing")
	require.Error(t, err)

	// 成功续行（pending + 已批准）
	store := &memStore{runs: map[string]*provision.Run{
		"p1": {ID: "p1", State: provision.StatePending, ApprovalID: "a-t",
			Request: provision.Request{Provider: "aws", Region: "r", Count: 1, GameID: "g", Env: "e"}},
	}}
	s2 := newProvisionService(t, true, true, store)
	run, err := s2.OpsProvisionResume(ctx, "p1")
	require.NoError(t, err)
	require.Equal(t, "p1", run.ID)
	// 后台 apply（emptyRegistry 等待 1s 判 failed）须在测试结束前落终态，
	// 否则 goroutine 仍持 WorkDir（t.TempDir）引发清理竞态。
	require.Eventually(t, func() bool {
		r, e := store.Get(ctx, "p1")
		return e == nil && r.State == provision.StateFailed
	}, 5*time.Second, 20*time.Millisecond)
}

func TestOpsProvisionListLimitBranches(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{
		"p1": {ID: "p1", State: provision.StateAdopted, Request: provision.Request{GameID: "g", Env: "e"}},
	}}
	s := newProvisionService(t, true, false, store)
	runs, err := s.OpsProvisionList(context.Background(), 0)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	runs, err = s.OpsProvisionList(context.Background(), 500)
	require.NoError(t, err)
	require.Len(t, runs, 1)
}

func TestOpsProvisionDestroyStoreAndRunnerErr(t *testing.T) {
	ctx := context.Background()

	// store 缺失 → 213
	s := newProvisionService(t, true, false, &memStore{runs: map[string]*provision.Run{}})
	_, err := s.OpsProvisionDestroy(ctx, "missing")
	require.Error(t, err)

	// runner 销毁失败 → 224（adopted 运行，空 AgentIDs 免排空门；需真实
	// workdir/<runID>/main.tf——runDestroy 先验 workdir 再调 runner.Destroy）
	store := &memStore{runs: map[string]*provision.Run{
		"d1": {ID: "d1", State: provision.StateAdopted, Request: provision.Request{GameID: "g", Env: "e"}},
	}}
	workDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workDir, "d1"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "d1", "main.tf"), []byte("resource \"x\" {}"), 0o644))
	orch := &provision.Orchestrator{
		Runner: errRunner{}, Registry: emptyRegistry{}, Approval: &fixedApproval{approved: true},
		Store: store, WorkDir: workDir, InstallURL: "https://x/agent.sh", RegisterTimeout: time.Second,
	}
	s2 := NewService(&svc.ServiceContext{Config: config.Config{Provision: config.ProvisionConfig{Enabled: true, HubAddr: "hub"}}, Provision: orch})
	_, err = s2.OpsProvisionDestroy(ctx, "d1")
	require.ErrorContains(t, err, "tofu destroy failed")
}

func TestOpsProvisionDestroyWorkflowRequest(t *testing.T) {
	ctx := context.Background()

	// 护栏拒绝（DestroyProtected）→ 239
	store := &memStore{runs: map[string]*provision.Run{
		"d2": {ID: "d2", State: provision.StateAdopted, DestroyProtected: true,
			Request: provision.Request{GameID: "g", Env: "e"}},
	}}
	orch := &provision.Orchestrator{
		Runner: inertRunner{}, Registry: emptyRegistry{}, Approval: &fixedApproval{approved: true},
		Store: store, WorkDir: t.TempDir(), InstallURL: "https://x/agent.sh", RegisterTimeout: time.Second,
	}
	s := provisionService(t, config.ProvisionApprovalWorkflow, orch)
	_, err := s.OpsProvisionDestroy(ctx, "d2")
	require.ErrorContains(t, err, "护栏")

	// 非 adopted 状态 → RequestDestroy 返回 ErrRunNotDestroyPending → 409
	store2 := &memStore{runs: map[string]*provision.Run{
		"d3": {ID: "d3", State: provision.StatePending, Request: provision.Request{GameID: "g", Env: "e"}},
	}}
	orch2 := &provision.Orchestrator{
		Runner: inertRunner{}, Registry: emptyRegistry{}, Approval: &fixedApproval{approved: true},
		Store: store2, WorkDir: t.TempDir(), InstallURL: "https://x/agent.sh", RegisterTimeout: time.Second,
	}
	s2 := provisionService(t, config.ProvisionApprovalWorkflow, orch2)
	_, err = s2.OpsProvisionDestroy(ctx, "d3")
	require.Error(t, err)
	require.Equal(t, http.StatusConflict, httpCode(err))
}

func TestOpsProvisionDestroyResumeNonConflict(t *testing.T) {
	s := newProvisionService(t, true, false, &memStore{runs: map[string]*provision.Run{}})
	_, err := s.OpsProvisionDestroyResume(context.Background(), "missing")
	require.Error(t, err)
}

func TestOpsProvisionProtectionBranches(t *testing.T) {
	ctx := context.Background()

	// orchestrator 未装配 → 328
	bare := NewService(&svc.ServiceContext{})
	_, err := bare.OpsProvisionProtection(ctx, "x", true)
	require.ErrorContains(t, err, "未配置")

	// store 缺失 → 333
	s := newProvisionService(t, true, false, &memStore{runs: map[string]*provision.Run{}})
	_, err = s.OpsProvisionProtection(ctx, "missing", true)
	require.Error(t, err)

	// 落库失败 → 345
	store := &failStore{memStore: &memStore{runs: map[string]*provision.Run{
		"p1": {ID: "p1", State: provision.StateAdopted, Request: provision.Request{GameID: "g", Env: "e"}},
	}}, updateErr: errors.New("disk full")}
	orch := &provision.Orchestrator{
		Runner: inertRunner{}, Registry: emptyRegistry{}, Approval: &fixedApproval{approved: true},
		Store: store, WorkDir: t.TempDir(), InstallURL: "https://x/agent.sh", RegisterTimeout: time.Second,
	}
	s2 := NewService(&svc.ServiceContext{Config: config.Config{Provision: config.ProvisionConfig{Enabled: true, HubAddr: "hub"}}, Provision: orch})
	_, err = s2.OpsProvisionProtection(ctx, "p1", true)
	require.ErrorContains(t, err, "disk full")
}

func TestOpsProvisionDestroyGuardsLoadUnverifiable(t *testing.T) {
	ctx := context.Background()
	base := func() *provision.Run {
		return &provision.Run{ID: "g1", State: provision.StateAdopted, AgentIDs: []string{"a1"},
			Request: provision.Request{GameID: "g", Env: "e"}}
	}

	// DB 未装配 → errProvisionLoadUnverifiable (304)
	store := &memStore{runs: map[string]*provision.Run{"g1": base()}}
	orch := &provision.Orchestrator{
		Runner: inertRunner{}, Registry: emptyRegistry{}, Approval: &fixedApproval{approved: true},
		Store: store, WorkDir: t.TempDir(), InstallURL: "https://x/agent.sh", RegisterTimeout: time.Second,
	}
	s := NewService(&svc.ServiceContext{Config: config.Config{Provision: config.ProvisionConfig{Enabled: true, HubAddr: "hub"}}, Provision: orch})
	_, err := s.OpsProvisionDestroy(ctx, "g1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "负载")

	// task_runs 表缺失 → CountActiveByAgents 错误 → 309
	db, err2 := gorm.Open(gsqlite.Open(t.TempDir()+"/pv.db"), &gorm.Config{})
	require.NoError(t, err2)
	store2 := &memStore{runs: map[string]*provision.Run{"g1": base()}}
	orch2 := &provision.Orchestrator{
		Runner: inertRunner{}, Registry: emptyRegistry{}, Approval: &fixedApproval{approved: true},
		Store: store2, WorkDir: t.TempDir(), InstallURL: "https://x/agent.sh", RegisterTimeout: time.Second,
	}
	s2 := NewService(&svc.ServiceContext{Config: config.Config{Provision: config.ProvisionConfig{Enabled: true, HubAddr: "hub"}}, Provision: orch2, DB: db})
	_, err = s2.OpsProvisionDestroy(ctx, "g1")
	require.Error(t, err)
}

func TestProvisionHandlerBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 坏载荷 400（submit / protection）
	h := NewHandler(newProvisionService(t, true, false, &memStore{runs: map[string]*provision.Run{}}))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/provision", "{bad")
	h.ProvisionSubmit(ctx)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	ctx, rec = newOpsTestContext(http.MethodPut, "/api/v1/ops/provision/p1/protection", "{bad")
	h.ProvisionProtection(ctx)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// ProvisionGet 成功（存量运行）→ 394
	store := &memStore{runs: map[string]*provision.Run{
		"p1": {ID: "p1", State: provision.StateAdopted, Request: provision.Request{GameID: "g", Env: "e"}},
	}}
	h2 := NewHandler(newProvisionService(t, true, false, store))
	ctx, rec = newOpsTestContext(http.MethodGet, "/api/v1/ops/provision/p1", "")
	ctx.Params = gin.Params{{Key: "runId", Value: "p1"}}
	h2.ProvisionGet(ctx)
	require.Equal(t, http.StatusOK, rec.Code)

	// ProvisionResume 成功（pending + 已批准）→ 384
	store2 := &memStore{runs: map[string]*provision.Run{
		"p2": {ID: "p2", State: provision.StatePending, ApprovalID: "a-t",
			Request: provision.Request{Provider: "aws", Region: "r", Count: 1, GameID: "g", Env: "e"}},
	}}
	h3 := NewHandler(newProvisionService(t, true, true, store2))
	ctx, rec = newOpsTestContext(http.MethodPost, "/api/v1/ops/provision/p2/resume", "")
	ctx.Params = gin.Params{{Key: "runId", Value: "p2"}}
	h3.ProvisionResume(ctx)
	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Eventually(t, func() bool {
		r, e := store2.Get(context.Background(), "p2")
		return e == nil && r.State == provision.StateFailed
	}, 5*time.Second, 20*time.Millisecond)
}
