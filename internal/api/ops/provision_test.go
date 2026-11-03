package ops

import (
	"context"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/provision"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

// ---------------------------------------------------------------------------
// 测试替身：ops 层只用 provision 导出接口（Runner/AgentRegistry/ApprovalGate/
// RunStore），构造真实 Orchestrator 走同一编排代码路径，不 mock 编排内部。
// ---------------------------------------------------------------------------

type inertRunner struct{}

func (inertRunner) Init(context.Context, string) error { return nil }
func (inertRunner) Apply(context.Context, string, map[string]string) (map[string]string, error) {
	return map[string]string{"instance_id": `"i-x"`}, nil
}
func (inertRunner) Destroy(context.Context, string) error { return nil }

// emptyRegistry 立即返回空注册（apply 后台判 failed，不影响 202 提交响应）。
type emptyRegistry struct{}

func (emptyRegistry) WaitRegistered(context.Context, string, string, time.Duration) ([]string, error) {
	return nil, nil
}

// fixedApproval 返回固定批准状态。
type fixedApproval struct{ approved bool }

func (f *fixedApproval) CreateApproval(context.Context, *provision.Request, string) (string, error) {
	return "approval-t", nil
}
func (f *fixedApproval) CreateDestroyApproval(context.Context, *provision.Run) (string, error) {
	return "destroy-approval-t", nil
}
func (f *fixedApproval) Approved(context.Context, string) (bool, error) { return f.approved, nil }

// memStore 是 map 背书的 RunStore（测试副本，隔离于 provision 包内同类）。
// Get 缺失返回 gorm.ErrRecordNotFound——镜像生产 DBStore（映射 404 契约）。
type memStore struct {
	mu   sync.Mutex
	runs map[string]*provision.Run
}

func (m *memStore) Create(_ context.Context, r *provision.Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.runs[r.ID] = &cp
	return nil
}
func (m *memStore) Update(_ context.Context, r *provision.Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.runs[r.ID] = &cp
	return nil
}
func (m *memStore) Get(_ context.Context, id string) (*provision.Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runs[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	cp := *r
	return &cp, nil
}
func (m *memStore) List(_ context.Context, _ int) ([]*provision.Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*provision.Run, 0, len(m.runs))
	for _, r := range m.runs {
		cp := *r
		out = append(out, &cp)
	}
	return out, nil
}

func newProvisionService(t *testing.T, enabled bool, approved bool, store *memStore) *Service {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		Provision: config.ProvisionConfig{
			Enabled:            enabled,
			HubAddr:            "hub:19090",
			BootstrapTokenEnv:  "CROUPIER_PROVISION_TEST_TOKEN",
			AgentInstallURL:    "https://dl.example.com/agent.sh",
			RegisterTimeoutSec: 1,
		},
	}
	ctx := &svc.ServiceContext{Config: cfg}
	if enabled {
		if store == nil {
			store = &memStore{runs: map[string]*provision.Run{}}
		}
		ctx.Provision = &provision.Orchestrator{
			Runner:          inertRunner{},
			Registry:        emptyRegistry{},
			Approval:        &fixedApproval{approved: approved},
			Store:           store,
			WorkDir:         t.TempDir(),
			InstallURL:      cfg.Provision.AgentInstallURL,
			RegisterTimeout: time.Second,
		}
	}
	return NewService(ctx)
}

func validSubmitBody() string {
	return `{"provider":"aws","region":"us-east-1","count":1,"spec":{"instanceType":"t3.micro"},"gameId":"demo","env":"prod","policy":"direct"}`
}

// ---------------------------------------------------------------------------
// 未配置装配：所有端点如实报 503 provision_not_configured（不假装已开启）。
// ---------------------------------------------------------------------------

func TestProvisionHandlers_NotConfigured_503(t *testing.T) {
	h := NewHandler(newProvisionService(t, false, false, nil))

	cases := []struct {
		name   string
		fn     func(*gin.Context)
		method string
		target string
		body   string
	}{
		{"submit", h.ProvisionSubmit, http.MethodPost, "/api/v1/ops/provision", validSubmitBody()},
		{"list", h.ProvisionList, http.MethodGet, "/api/v1/ops/provision", ""},
		{"get", h.ProvisionGet, http.MethodGet, "/api/v1/ops/provision/prov-x", ""},
		{"resume", h.ProvisionResume, http.MethodPost, "/api/v1/ops/provision/prov-x/resume", ""},
		{"destroy", h.ProvisionDestroy, http.MethodDelete, "/api/v1/ops/provision/prov-x", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx, rec := newOpsTestContext(c.method, c.target, c.body)
			c.fn(ctx)
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
			assert.Contains(t, rec.Body.String(), "provision_not_configured")
		})
	}
}

// ---------------------------------------------------------------------------
// 装配完整：DTO 映射、202 提交、422 校验、503 token、404 运行、409 状态。
// ---------------------------------------------------------------------------

func TestProvisionSubmit_Direct_Returns202Applying(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{}}
	h := NewHandler(newProvisionService(t, true, false, store))
	t.Setenv("CROUPIER_PROVISION_TEST_TOKEN", "tok-1")

	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/provision", validSubmitBody())
	ctx.Set("username", "alice")
	h.ProvisionSubmit(ctx)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	require.Contains(t, rec.Body.String(), `"state":"applying"`)
	require.Contains(t, rec.Body.String(), `"provider":"aws"`)
	require.Contains(t, rec.Body.String(), `"requestedBy":"alice"`)
	// 响应不含敏感 token/hub（json:"-" 不落响应）。
	assert.NotContains(t, rec.Body.String(), "tok-1")
	assert.NotContains(t, rec.Body.String(), "hub:19090")
	// 后台 apply 全闭环最终落 failed（emptyRegistry 无注册）——异步终态正确。
	require.Eventually(t, func() bool {
		runs, _ := store.List(context.Background(), 10)
		return len(runs) == 1 && runs[0].State == provision.StateFailed
	}, 3*time.Second, 20*time.Millisecond)
}

func TestProvisionSubmit_Validation_422(t *testing.T) {
	h := NewHandler(newProvisionService(t, true, false, nil))
	t.Setenv("CROUPIER_PROVISION_TEST_TOKEN", "tok-1")

	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/provision",
		`{"provider":"gcp","region":"r","count":1,"spec":{"instanceType":"t"},"gameId":"g","env":"e"}`)
	h.ProvisionSubmit(ctx)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), "validation_failed")
	assert.Contains(t, rec.Body.String(), "unsupported provider")
}

// 审核发现：ProvisionSubmit 直接采用 body gameId/env，未与认证 scope 比对
// ——作用域受限用户可向越权游戏/环境供给云资源。回归：scope 冲突必须 403。
func TestProvisionSubmit_CrossScopeRejected_403(t *testing.T) {
	h := NewHandler(newProvisionService(t, true, false, nil))
	t.Setenv("CROUPIER_PROVISION_TEST_TOKEN", "tok-1")

	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/provision", validSubmitBody())
	// 认证 scope 是 demo/prod，body 也是 demo/prod —— 应放行。
	ctx.Request = ctx.Request.WithContext(svc.WithGameScope(ctx.Request.Context(),
		svc.GameScope{GameID: "demo", Env: "prod"}))
	ctx.Set("username", "alice")
	h.ProvisionSubmit(ctx)
	assert.NotEqual(t, http.StatusForbidden, rec.Code, "同 scope 提交不应被拒")

	// body 的 gameId 与认证 scope 冲突 → 403 fail-closed。
	ctx2, rec2 := newOpsTestContext(http.MethodPost, "/api/v1/ops/provision", validSubmitBody())
	ctx2.Request = ctx2.Request.WithContext(svc.WithGameScope(ctx2.Request.Context(),
		svc.GameScope{GameID: "other", Env: "prod"}))
	ctx2.Set("username", "alice")
	h.ProvisionSubmit(ctx2)
	assert.Equal(t, http.StatusForbidden, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "scope_mismatch")
}

func TestProvisionSubmit_MissingToken_503(t *testing.T) {
	// 不设置 token 环境变量：server 装配不完整，报 503（非客户端输入错误）。
	h := NewHandler(newProvisionService(t, true, false, nil))
	t.Setenv("CROUPIER_PROVISION_TEST_TOKEN", "")

	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/provision", validSubmitBody())
	h.ProvisionSubmit(ctx)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "provision_token_not_configured")
}

func TestProvisionGet_UnknownRun_404(t *testing.T) {
	h := NewHandler(newProvisionService(t, true, false, &memStore{runs: map[string]*provision.Run{}}))

	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/provision/prov-nope", "")
	h.ProvisionGet(ctx)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_found")
}

func TestProvisionList_ReturnsRuns(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{}}
	_ = store.Create(context.Background(), &provision.Run{ID: "prov-1", State: provision.StateAdopted, CreatedAt: time.Now()})
	h := NewHandler(newProvisionService(t, true, false, store))

	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/provision?limit=10", "")
	h.ProvisionList(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"id":"prov-1"`)
	assert.Contains(t, rec.Body.String(), `"state":"adopted"`)
}

func TestProvisionResume_NotApproved_409(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{}}
	h := NewHandler(newProvisionService(t, true, false, store)) // approved=false
	t.Setenv("CROUPIER_PROVISION_TEST_TOKEN", "tok-1")

	// 先建 pending 运行（ticket 策略）——SubmitAsync ticket 走 Submit 非阻塞。
	submitCtx, submitRec := newOpsTestContext(http.MethodPost, "/api/v1/ops/provision",
		`{"provider":"aws","region":"us-east-1","count":1,"spec":{"instanceType":"t3.micro"},"gameId":"demo","env":"prod","policy":"ticket"}`)
	h.ProvisionSubmit(submitCtx)
	require.Equal(t, http.StatusAccepted, submitRec.Code)
	require.Contains(t, submitRec.Body.String(), `"state":"pending"`)

	// 取真实 pending 运行 id（Submit 自动生成）。
	require.Eventually(t, func() bool {
		runs, _ := store.List(context.Background(), 10)
		return len(runs) == 1
	}, 2*time.Second, 10*time.Millisecond)
	runs, _ := store.List(context.Background(), 10)
	realID := runs[0].ID

	// 未批准：resume → 409 provision_state_conflict（资源状态冲突）。
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/provision/"+realID+"/resume", "")
	ctx.Params = gin.Params{{Key: "runId", Value: realID}}
	h.ProvisionResume(ctx)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "provision_state_conflict")
}

func TestProvisionDestroy_Sync_Transitions(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{}}
	_ = store.Create(context.Background(), &provision.Run{
		ID: "prov-d", State: provision.StateAdopted, CreatedAt: time.Now(),
	})
	svc := newProvisionService(t, true, false, store)
	// 供给运行有工作目录 + main.tf（真实 Destroy 前置检查），inertRunner
	// 的 Destroy 恒 nil → 状态推进 destroyed。
	workDir := t.TempDir()
	svc.svcCtx.Provision.WorkDir = workDir
	require.NoError(t, os.MkdirAll(workDir+"/prov-d", 0o755))
	require.NoError(t, os.WriteFile(workDir+"/prov-d/main.tf", []byte("# stub"), 0o600))
	h := NewHandler(svc)

	ctx, rec := newOpsTestContext(http.MethodDelete, "/api/v1/ops/provision/prov-d", "")
	ctx.Params = gin.Params{{Key: "runId", Value: "prov-d"}}
	h.ProvisionDestroy(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"state":"destroyed"`)
}

// ---------------------------------------------------------------------------
// OPS-002 缩容与排水守卫（数据盘护栏 / 在途任务闸 / 排空门）与护栏端点。
// ---------------------------------------------------------------------------

// newProvisionServiceGuarded 构造带可注入 DB（task_runs）与 registry（agent
// 会话）的 Service，供销毁守卫三闸测试。seedDB 为 nil 则无 DB（空 AgentIDs
// 的运行不触库）；agents 为 nil 则无 registry（排空门恒过）。
func newProvisionServiceGuarded(t *testing.T, store *memStore, seedDB func(*gorm.DB), agents map[string]*reg.AgentSession) *Service {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		Provision: config.ProvisionConfig{
			Enabled:            true,
			HubAddr:            "hub:19090",
			BootstrapTokenEnv:  "CROUPIER_PROVISION_TEST_TOKEN",
			RegisterTimeoutSec: 1,
		},
	}
	ctx := &svc.ServiceContext{Config: cfg}
	if seedDB != nil {
		db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&model.TaskRun{}))
		seedDB(db)
		ctx.DB = db
	}
	if agents != nil {
		rs := reg.NewStore()
		for _, sess := range agents {
			require.NoError(t, rs.UpsertAgent(sess))
		}
		ctx.RegistryStore = rs
	}
	ctx.Provision = &provision.Orchestrator{
		Runner:          inertRunner{},
		Registry:        emptyRegistry{},
		Approval:        &fixedApproval{approved: false},
		Store:           store,
		WorkDir:         t.TempDir(),
		InstallURL:      cfg.Provision.AgentInstallURL,
		RegisterTimeout: time.Second,
	}
	return NewService(ctx)
}

// seedProvisionWorkdir 准备运行的工作目录 + main.tf（真实 Destroy 前置检查）。
func seedProvisionWorkdir(t *testing.T, svc *Service, runID string) {
	t.Helper()
	workDir := svc.svcCtx.Provision.WorkDir
	require.NoError(t, os.MkdirAll(workDir+"/"+runID, 0o755))
	require.NoError(t, os.WriteFile(workDir+"/"+runID+"/main.tf", []byte("# stub"), 0o600))
}

func TestProvisionDestroy_Protected_409(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{}}
	_ = store.Create(context.Background(), &provision.Run{
		ID: "prov-p", State: provision.StateAdopted, DestroyProtected: true, CreatedAt: time.Now(),
	})
	h := NewHandler(newProvisionServiceGuarded(t, store, nil, nil))

	ctx, rec := newOpsTestContext(http.MethodDelete, "/api/v1/ops/provision/prov-p", "")
	ctx.Params = gin.Params{{Key: "runId", Value: "prov-p"}}
	h.ProvisionDestroy(ctx)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "provision_destroy_protected")
}

func TestProvisionDestroy_InflightTasks_409(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{}}
	_ = store.Create(context.Background(), &provision.Run{
		ID: "prov-i", State: provision.StateAdopted, CreatedAt: time.Now(),
		AgentIDs: []string{"agent-a"},
		Request: provision.Request{
			Provider: provision.ProviderAWS, GameID: "demo", Env: "prod",
		},
	})
	// 目标机 agent-a 有一个 running 在途任务（未排水）。
	seedDB := func(db *gorm.DB) {
		require.NoError(t, model.NewTaskRunModel(db).Create(context.Background(), &model.TaskRun{
			TaskID: "t-inflight", AgentID: "agent-a", GameID: "demo", Env: "prod", Status: "running",
		}))
	}
	h := NewHandler(newProvisionServiceGuarded(t, store, seedDB, nil))

	ctx, rec := newOpsTestContext(http.MethodDelete, "/api/v1/ops/provision/prov-i", "")
	ctx.Params = gin.Params{{Key: "runId", Value: "prov-i"}}
	h.ProvisionDestroy(ctx)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "provision_tasks_inflight")
}

func TestProvisionDestroy_NotDrained_409(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{}}
	_ = store.Create(context.Background(), &provision.Run{
		ID: "prov-n", State: provision.StateAdopted, CreatedAt: time.Now(),
		AgentIDs: []string{"agent-a"},
		Request: provision.Request{
			Provider: provision.ProviderAWS, GameID: "demo", Env: "prod",
		},
	})
	// agent-a 在线但未 drain（无在途任务，但排空门未过）。
	agents := map[string]*reg.AgentSession{
		"agent-a": {AgentID: "agent-a", GameID: "demo", Env: "prod", Draining: false},
	}
	h := NewHandler(newProvisionServiceGuarded(t, store, nil, agents))

	ctx, rec := newOpsTestContext(http.MethodDelete, "/api/v1/ops/provision/prov-n", "")
	ctx.Params = gin.Params{{Key: "runId", Value: "prov-n"}}
	h.ProvisionDestroy(ctx)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "provision_agents_not_drained")
}

func TestProvisionDestroy_DrainedClean_200(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{}}
	_ = store.Create(context.Background(), &provision.Run{
		ID: "prov-ok", State: provision.StateAdopted, CreatedAt: time.Now(),
		AgentIDs: []string{"agent-a"},
		Request: provision.Request{
			Provider: provision.ProviderAWS, GameID: "demo", Env: "prod",
		},
	})
	// agent-a 已 drain、无在途任务（空 task_runs 表）→ 三闸全过，销毁推进
	// destroyed。
	agents := map[string]*reg.AgentSession{
		"agent-a": {AgentID: "agent-a", GameID: "demo", Env: "prod", Draining: true},
	}
	svc := newProvisionServiceGuarded(t, store, func(*gorm.DB) {}, agents)
	seedProvisionWorkdir(t, svc, "prov-ok")
	h := NewHandler(svc)

	ctx, rec := newOpsTestContext(http.MethodDelete, "/api/v1/ops/provision/prov-ok", "")
	ctx.Params = gin.Params{{Key: "runId", Value: "prov-ok"}}
	h.ProvisionDestroy(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"state":"destroyed"`)
}

func TestProvisionProtection_Toggle(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{}}
	_ = store.Create(context.Background(), &provision.Run{
		ID: "prov-t", State: provision.StateAdopted, CreatedAt: time.Now(),
		Request: provision.Request{Provider: provision.ProviderAWS, GameID: "demo", Env: "prod"},
	})
	h := NewHandler(newProvisionServiceGuarded(t, store, nil, nil))

	// 开护栏：true。
	ctx, rec := newOpsTestContext(http.MethodPut, "/api/v1/ops/provision/prov-t/protection", `{"destroyProtected":true}`)
	ctx.Params = gin.Params{{Key: "runId", Value: "prov-t"}}
	h.ProvisionProtection(ctx)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"destroyProtected":true`)

	// 关护栏：false。
	ctx, rec = newOpsTestContext(http.MethodPut, "/api/v1/ops/provision/prov-t/protection", `{"destroyProtected":false}`)
	ctx.Params = gin.Params{{Key: "runId", Value: "prov-t"}}
	h.ProvisionProtection(ctx)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"destroyProtected":false`)

	// 持久化读回：store 中状态与响应一致。
	got, err := store.Get(context.Background(), "prov-t")
	require.NoError(t, err)
	assert.False(t, got.DestroyProtected)
}

func TestProvisionProtection_NonAdopted_409(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{}}
	// destroyed 状态不允许再改护栏。
	_ = store.Create(context.Background(), &provision.Run{
		ID: "prov-x", State: provision.StateDestroyed, CreatedAt: time.Now(),
	})
	h := NewHandler(newProvisionServiceGuarded(t, store, nil, nil))

	ctx, rec := newOpsTestContext(http.MethodPut, "/api/v1/ops/provision/prov-x/protection", `{"destroyProtected":true}`)
	ctx.Params = gin.Params{{Key: "runId", Value: "prov-x"}}
	h.ProvisionProtection(ctx)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "provision_state_conflict")
}

// ---------------------------------------------------------------------------
// OPS-003 审批双模式（workflow）：提交强制 ticket、销毁走申请单、
// 批准后执行（POST /provision/:runId/destroy）。
// ---------------------------------------------------------------------------

// newProvisionServiceWorkflow 构造 workflow 模式 Service（网络级审批双模式：
// 扩容/销毁均走申请单）。approved 控制审批记录批准状态。
func newProvisionServiceWorkflow(t *testing.T, approved bool, store *memStore) *Service {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		Provision: config.ProvisionConfig{
			Enabled:            true,
			Approval:           config.ProvisionApprovalWorkflow,
			HubAddr:            "hub:19090",
			BootstrapTokenEnv:  "CROUPIER_PROVISION_TEST_TOKEN",
			AgentInstallURL:    "https://dl.example.com/agent.sh",
			RegisterTimeoutSec: 1,
		},
	}
	ctx := &svc.ServiceContext{Config: cfg}
	if store == nil {
		store = &memStore{runs: map[string]*provision.Run{}}
	}
	ctx.Provision = &provision.Orchestrator{
		Runner:          inertRunner{},
		Registry:        emptyRegistry{},
		Approval:        &fixedApproval{approved: approved},
		Store:           store,
		WorkDir:         t.TempDir(),
		InstallURL:      cfg.Provision.AgentInstallURL,
		RegisterTimeout: time.Second,
	}
	return NewService(ctx)
}

// TestProvisionSubmit_Workflow_ForcesTicket 校验：workflow 模式客户端即使传
// direct 也被强制为 ticket（不得绕过审批），响应 pending 且带 approval id。
func TestProvisionSubmit_Workflow_ForcesTicket(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{}}
	h := NewHandler(newProvisionServiceWorkflow(t, false, store))
	t.Setenv("CROUPIER_PROVISION_TEST_TOKEN", "tok-1")

	// policy=direct 被 workflow 覆盖为 ticket。
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/provision", validSubmitBody())
	h.ProvisionSubmit(ctx)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	require.Contains(t, rec.Body.String(), `"state":"pending"`)
	require.Contains(t, rec.Body.String(), `"policy":"ticket"`)
	require.Contains(t, rec.Body.String(), `"approvalID":"approval-t"`)
}

// TestProvisionDestroy_Workflow_RequestsPending 校验：workflow 模式 DELETE
// 销毁走申请单：run 推进 destroy_pending，资源未销毁。
func TestProvisionDestroy_Workflow_RequestsPending(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{}}
	_ = store.Create(context.Background(), &provision.Run{
		ID: "prov-w", State: provision.StateAdopted, CreatedAt: time.Now(),
		Request: provision.Request{Provider: provision.ProviderAWS, GameID: "demo", Env: "prod"},
	})
	h := NewHandler(newProvisionServiceWorkflow(t, false, store))

	ctx, rec := newOpsTestContext(http.MethodDelete, "/api/v1/ops/provision/prov-w", "")
	ctx.Params = gin.Params{{Key: "runId", Value: "prov-w"}}
	h.ProvisionDestroy(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"state":"destroy_pending"`)
	require.Contains(t, rec.Body.String(), `"destroyApprovalID":"destroy-approval-t"`)
}

// TestProvisionDestroyResume_NotApproved_409 校验：workflow 销毁申请未批准
// 时 POST /destroy 返回 409 provision_state_conflict。
func TestProvisionDestroyResume_NotApproved_409(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{}}
	_ = store.Create(context.Background(), &provision.Run{
		ID: "prov-wr", State: provision.StateAdopted, CreatedAt: time.Now(),
		Request: provision.Request{Provider: provision.ProviderAWS, GameID: "demo", Env: "prod"},
	})
	h := NewHandler(newProvisionServiceWorkflow(t, false, store)) // approved=false

	// 先受理销毁申请。
	reqCtx, reqRec := newOpsTestContext(http.MethodDelete, "/api/v1/ops/provision/prov-wr", "")
	reqCtx.Params = gin.Params{{Key: "runId", Value: "prov-wr"}}
	h.ProvisionDestroy(reqCtx)
	require.Equal(t, http.StatusOK, reqRec.Code)

	// 未批准：destroy resume → 409。
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/provision/prov-wr/destroy", "")
	ctx.Params = gin.Params{{Key: "runId", Value: "prov-wr"}}
	h.ProvisionDestroyResume(ctx)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "provision_state_conflict")
}

// TestProvisionDestroyResume_Approved_200 校验：workflow 销毁申请批准后
// POST /destroy 执行 terraform destroy 原语并推进 destroyed。
func TestProvisionDestroyResume_Approved_200(t *testing.T) {
	store := &memStore{runs: map[string]*provision.Run{}}
	_ = store.Create(context.Background(), &provision.Run{
		ID: "prov-da", State: provision.StateAdopted, CreatedAt: time.Now(),
		Request: provision.Request{Provider: provision.ProviderAWS, GameID: "demo", Env: "prod"},
	})
	svc := newProvisionServiceWorkflow(t, true, store) // approved=true
	seedProvisionWorkdir(t, svc, "prov-da")
	h := NewHandler(svc)

	reqCtx, reqRec := newOpsTestContext(http.MethodDelete, "/api/v1/ops/provision/prov-da", "")
	reqCtx.Params = gin.Params{{Key: "runId", Value: "prov-da"}}
	h.ProvisionDestroy(reqCtx)
	require.Equal(t, http.StatusOK, reqRec.Code)

	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/provision/prov-da/destroy", "")
	ctx.Params = gin.Params{{Key: "runId", Value: "prov-da"}}
	h.ProvisionDestroyResume(ctx)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"state":"destroyed"`)
}

// TestProvisionDestroyResume_NotConfigured_503 校验：未装配引擎时 destroy
// resume 如实报 503（不假装已开启）。
func TestProvisionDestroyResume_NotConfigured_503(t *testing.T) {
	h := NewHandler(newProvisionService(t, false, false, nil))
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/provision/prov-x/destroy", "")
	ctx.Params = gin.Params{{Key: "runId", Value: "prov-x"}}
	h.ProvisionDestroyResume(ctx)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "provision_not_configured")
}
