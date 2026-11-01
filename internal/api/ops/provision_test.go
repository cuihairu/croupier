package ops

import (
	"context"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/ops/provision"
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
