package ops

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/powerops"
	"github.com/cuihairu/croupier/internal/svc"
)

// ---------------------------------------------------------------------------
// 测试替身：ops 层只依赖 powerops 导出接口（ApprovalGate/Provider/Store），
// 构造真实 Orchestrator 走同一编排代码路径。审批侧直接用 approvals.MemStore
//（与 svc 装配的 poweropsApprovalGate 同构），approve/reject 走 store 真实状态。
// ---------------------------------------------------------------------------

type powerTestGate struct{ store approvals.Store }

func (g *powerTestGate) CreateApproval(_ context.Context, action powerops.Action, requestID, requestedBy, reason string, targetCount int) (string, error) {
	a, err := g.store.Create(&approvals.Approval{
		ID:         "apr-" + requestID,
		FunctionID: action.FunctionID(),
		Actor:      requestedBy,
		Mode:       "powerops",
		State:      "pending",
		Metadata: map[string]string{
			"powerRequestId": requestID,
			"action":         string(action),
			"targets":        strconv.Itoa(targetCount),
			"reason":         reason,
		},
	})
	if err != nil {
		return "", err
	}
	return a.ID, nil
}

func (g *powerTestGate) Approved(_ context.Context, approvalID string) (bool, error) {
	a, err := g.store.Get(approvalID)
	if err != nil {
		return false, err
	}
	return a.State == "approved", nil
}

// fakePowerProvider 是可脚本化 Status 的 IPMI 替身（执行动作恒成功）。
type fakePowerProvider struct {
	mu       sync.Mutex
	statuses []string // 依序返回，最后一个重复
	calls    int      // Status 调用计数（拒绝路径须为 0）
}

func (p *fakePowerProvider) Kind() string { return powerops.ProviderIPMI }

func (p *fakePowerProvider) PowerOn(context.Context, powerops.Machine) (string, error) {
	return "on sent", nil
}

func (p *fakePowerProvider) PowerOff(context.Context, powerops.Machine, bool) (string, error) {
	return "off sent", nil
}

func (p *fakePowerProvider) Reboot(context.Context, powerops.Machine) (string, error) {
	return "reboot sent", nil
}

func (p *fakePowerProvider) Status(context.Context, powerops.Machine) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if len(p.statuses) == 0 {
		return "on", nil
	}
	st := p.statuses[0]
	if len(p.statuses) > 1 {
		p.statuses = p.statuses[1:]
	}
	return st, nil
}

func (p *fakePowerProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

type powerHarness struct {
	service   *Service
	handler   *Handler
	store     *powerops.GormStore
	gate      *powerTestGate
	approvals approvals.Store
	provider  *fakePowerProvider
	db        *gorm.DB
}

func newPowerHarness(t *testing.T, enabled bool, statuses []string) *powerHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1) // :memory: 单连接（test 基建坑）
	require.NoError(t, db.AutoMigrate(&model.PowerRequest{}, &model.PowerRequestTarget{}, &model.Node{}))

	approvalsStore := approvals.NewMemStore()
	provider := &fakePowerProvider{statuses: statuses}
	svcCtx := &svc.ServiceContext{
		Config:         config.Config{Powerops: config.PoweropsConfig{Enabled: enabled}},
		DB:             db,
		NodeModel:      model.NewNodeModel(db),
		ApprovalsStore: approvalsStore,
	}
	h := &powerHarness{
		store:     powerops.NewGormStore(db),
		approvals: approvalsStore,
		provider:  provider,
		db:        db,
	}
	if enabled {
		h.gate = &powerTestGate{store: approvalsStore}
		svcCtx.Powerops = &powerops.Orchestrator{
			Store:          h.store,
			Approval:       h.gate,
			Providers:      map[string]powerops.Provider{powerops.ProviderIPMI: provider},
			VerifyTimeout:  300 * time.Millisecond,
			VerifyInterval: 10 * time.Millisecond,
		}
	}
	h.service = NewService(svcCtx)
	h.handler = NewHandler(h.service)

	// 登记两台 IPMI 机器 + 一台 WOL 机器（nodes.meta.power）。
	seedPowerNode(t, db, "node-1", "ipmi", "10.0.0.7", "")
	seedPowerNode(t, db, "node-2", "ipmi", "10.0.0.8", "")
	seedPowerNode(t, db, "wol-1", "wol", "", "aa:bb:cc:dd:ee:01")
	return h
}

func seedPowerNode(t *testing.T, db *gorm.DB, nodeID, provider, bmcAddr, mac string) {
	t.Helper()
	meta := datatypes.JSONMap{"power": map[string]interface{}{"provider": provider}}
	if bmcAddr != "" {
		meta["power"].(map[string]interface{})["bmcAddr"] = bmcAddr
	}
	if mac != "" {
		meta["power"].(map[string]interface{})["mac"] = mac
	}
	require.NoError(t, db.Create(&model.Node{NodeID: nodeID, Name: nodeID, Status: "online", Meta: meta}).Error)
}

func withParam(c *gin.Context, key, value string) *gin.Context {
	c.Params = gin.Params{{Key: key, Value: value}}
	return c
}

func awaitPowerTerminal(t *testing.T, store *powerops.GormStore, requestID string) *model.PowerRequest {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		req, err := store.Get(requestID)
		require.NoError(t, err)
		switch req.State {
		case powerops.StateDone, powerops.StatePartial, powerops.StateFailed, powerops.StateRejected:
			return req
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("power request never reached a terminal state")
	return nil
}

func powerSubmitBody(nodeIDs ...string) string {
	quoted := make([]string, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		quoted = append(quoted, strconv.Quote(id))
	}
	return `{"nodeIds":[` + strings.Join(quoted, ",") + `],"action":"power_on","reason":"maintenance window"}`
}

// ---------------------------------------------------------------------------
// 未配置装配：所有端点如实报 503 powerops_not_configured（不假装已开启）。
// ---------------------------------------------------------------------------

func TestPowerHandlers_NotConfigured_503(t *testing.T) {
	h := newPowerHarness(t, false, []string{"on"})

	cases := []struct {
		name   string
		fn     func(*gin.Context)
		method string
		target string
		body   string
		params bool
	}{
		{"submit", h.handler.PowerSubmit, http.MethodPost, "/api/v1/ops/power", powerSubmitBody("node-1"), false},
		{"list", h.handler.PowerList, http.MethodGet, "/api/v1/ops/power", "", false},
		{"get", h.handler.PowerGet, http.MethodGet, "/api/v1/ops/power/pw-x", "", true},
		{"resume", h.handler.PowerResume, http.MethodPost, "/api/v1/ops/power/pw-x/resume", "", true},
		{"reject", h.handler.PowerReject, http.MethodPost, "/api/v1/ops/power/pw-x/reject", "", true},
		{"config", h.handler.PowerConfigPut, http.MethodPut, "/api/v1/ops/nodes/node-1/power-config", `{"provider":"ipmi"}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx, rec := newOpsTestContext(c.method, c.target, c.body)
			if c.params {
				withParam(ctx, "requestId", "pw-x")
				withParam(ctx, "nodeId", "node-1")
			}
			c.fn(ctx)
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
			assert.Contains(t, rec.Body.String(), "powerops_not_configured")
		})
	}
}

// ---------------------------------------------------------------------------
// 装配完整：提交 → 审批 → 续行执行 → 状态确认闭环；校验/状态冲突契约。
// ---------------------------------------------------------------------------

func TestPowerSubmit_CreatesPendingWithApproval(t *testing.T) {
	h := newPowerHarness(t, true, []string{"on"})
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/power", powerSubmitBody("node-1", "node-2"))
	ctx.Set("username", "alice")
	h.handler.PowerSubmit(ctx)

	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Contains(t, rec.Body.String(), `"state":"pending"`)
	assert.Contains(t, rec.Body.String(), `"requestedBy":"alice"`)

	requests, err := h.store.List(10)
	require.NoError(t, err)
	require.Len(t, requests, 1)
	assert.NotEmpty(t, requests[0].ApprovalID)
	assert.Equal(t, 2, requests[0].Total)

	// 审批记录落在真实 approvals store（mode=powerops，function=ops.power.power_on）。
	a, err := h.approvals.Get(requests[0].ApprovalID)
	require.NoError(t, err)
	assert.Equal(t, "powerops", a.Mode)
	assert.Equal(t, "ops.power.power_on", a.FunctionID)
	assert.Equal(t, "pending", a.State)
}

func TestPowerSubmit_Validation_422(t *testing.T) {
	h := newPowerHarness(t, true, []string{"on"})

	// 未登记机器。
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/power", powerSubmitBody("ghost"))
	h.handler.PowerSubmit(ctx)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), "validation_failed")
	assert.Contains(t, rec.Body.String(), "not registered")

	// WOL 机器不支持 power_off。
	ctx2, rec2 := newOpsTestContext(http.MethodPost, "/api/v1/ops/power",
		`{"nodeIds":["wol-1"],"action":"power_off","reason":"x"}`)
	h.handler.PowerSubmit(ctx2)
	assert.Equal(t, http.StatusUnprocessableEntity, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "does not support")

	// 校验失败不落任何请求行。
	requests, _ := h.store.List(10)
	assert.Empty(t, requests)
}

func TestPowerResume_Unapproved_409(t *testing.T) {
	h := newPowerHarness(t, true, []string{"on"})
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/power", powerSubmitBody("node-1"))
	h.handler.PowerSubmit(ctx)
	require.Equal(t, http.StatusAccepted, rec.Code)
	requestID := h.approvalRequestID(t, h)

	ctx2, rec2 := newOpsTestContext(http.MethodPost, "/api/v1/ops/power/"+requestID+"/resume", "")
	withParam(ctx2, "requestId", requestID)
	h.handler.PowerResume(ctx2)

	assert.Equal(t, http.StatusConflict, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "power_state_conflict")
	assert.Contains(t, rec2.Body.String(), "approval not granted")
}

func (h *powerHarness) approvalRequestID(t *testing.T, _ *powerHarness) string {
	t.Helper()
	requests, err := h.store.List(10)
	require.NoError(t, err)
	require.Len(t, requests, 1)
	return requests[0].RequestID
}

func TestPowerFullChain_ApprovedExecutesAndVerifies(t *testing.T) {
	h := newPowerHarness(t, true, []string{"off", "on"})
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/power", powerSubmitBody("node-1"))
	h.handler.PowerSubmit(ctx)
	require.Equal(t, http.StatusAccepted, rec.Code)
	requestID := h.approvalRequestID(t, h)

	// 第二人批准（真实 approvals store 状态迁移）。
	req, err := h.store.Get(requestID)
	require.NoError(t, err)
	_, err = h.approvals.Approve(req.ApprovalID, "approver2")
	require.NoError(t, err)

	ctx2, rec2 := newOpsTestContext(http.MethodPost, "/api/v1/ops/power/"+requestID+"/resume", "")
	withParam(ctx2, "requestId", requestID)
	h.handler.PowerResume(ctx2)
	require.Equal(t, http.StatusAccepted, rec2.Code)

	final := awaitPowerTerminal(t, h.store, requestID)
	assert.Equal(t, powerops.StateDone, final.State)
	assert.Equal(t, 1, final.Succeeded)

	// GET 返回请求 + 每台机器结果（verified=true：状态已确认 on）。
	ctx3, rec3 := newOpsTestContext(http.MethodGet, "/api/v1/ops/power/"+requestID, "")
	withParam(ctx3, "requestId", requestID)
	h.handler.PowerGet(ctx3)
	require.Equal(t, http.StatusOK, rec3.Code)
	assert.Contains(t, rec3.Body.String(), `"state":"done"`)
	assert.Contains(t, rec3.Body.String(), `"verified":true`)
	assert.GreaterOrEqual(t, h.provider.callCount(), 2, "verify must poll status")
}

func TestPowerReject_ExecutesNothing(t *testing.T) {
	h := newPowerHarness(t, true, []string{"off"})
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/power", powerSubmitBody("node-1"))
	h.handler.PowerSubmit(ctx)
	require.Equal(t, http.StatusAccepted, rec.Code)
	requestID := h.approvalRequestID(t, h)

	ctx2, rec2 := newOpsTestContext(http.MethodPost, "/api/v1/ops/power/"+requestID+"/reject", "")
	withParam(ctx2, "requestId", requestID)
	h.handler.PowerReject(ctx2)
	require.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), `"state":"rejected"`)

	assert.Equal(t, 0, h.provider.callCount(), "rejected request must never touch providers")
	_, err := h.store.Get(requestID)
	require.NoError(t, err)

	// 拒绝后续行拒绝（409）。
	ctx3, rec3 := newOpsTestContext(http.MethodPost, "/api/v1/ops/power/"+requestID+"/resume", "")
	withParam(ctx3, "requestId", requestID)
	h.handler.PowerResume(ctx3)
	assert.Equal(t, http.StatusConflict, rec3.Code)
}

func TestPowerGet_LazyRejectSync(t *testing.T) {
	// 审批服务里被拒的单据：GET 时把 pending 请求收敛为 rejected（读时同步）。
	h := newPowerHarness(t, true, []string{"off"})
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/power", powerSubmitBody("node-1"))
	h.handler.PowerSubmit(ctx)
	require.Equal(t, http.StatusAccepted, rec.Code)
	requestID := h.approvalRequestID(t, h)

	req, err := h.store.Get(requestID)
	require.NoError(t, err)
	_, err = h.approvals.Reject(req.ApprovalID, "not authorized window", "approver2")
	require.NoError(t, err)

	ctx2, rec2 := newOpsTestContext(http.MethodGet, "/api/v1/ops/power/"+requestID, "")
	withParam(ctx2, "requestId", requestID)
	h.handler.PowerGet(ctx2)
	require.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), `"state":"rejected"`)

	// 同步落库（非只改响应）。
	refreshed, err := h.store.Get(requestID)
	require.NoError(t, err)
	assert.Equal(t, powerops.StateRejected, refreshed.State)
}

func TestPowerConfigPut_RegistersMachineForSubmit(t *testing.T) {
	h := newPowerHarness(t, true, []string{"on"})

	// 新节点先登记电源信息。
	require.NoError(t, h.db.Create(&model.Node{NodeID: "node-9", Name: "node-9", Status: "online"}).Error)
	ctx, rec := newOpsTestContext(http.MethodPut, "/api/v1/ops/nodes/node-9/power-config",
		`{"provider":"ipmi","bmcAddr":"10.0.0.9","bmcPort":623}`)
	withParam(ctx, "nodeId", "node-9")
	h.handler.PowerConfigPut(ctx)
	require.Equal(t, http.StatusOK, rec.Code)

	// meta.power 已写入且保留 meta 其它键。
	var row model.Node
	require.NoError(t, h.db.Where("node_id = ?", "node-9").First(&row).Error)
	require.Contains(t, row.Meta, "power")

	// 登记后即可提交（resolver 从 nodes.meta.power 解析）。
	ctx2, rec2 := newOpsTestContext(http.MethodPost, "/api/v1/ops/power", powerSubmitBody("node-9"))
	h.handler.PowerSubmit(ctx2)
	assert.Equal(t, http.StatusAccepted, rec2.Code)

	// 未知节点 404。
	ctx3, rec3 := newOpsTestContext(http.MethodPut, "/api/v1/ops/nodes/ghost/power-config", `{"provider":"ipmi"}`)
	withParam(ctx3, "nodeId", "ghost")
	h.handler.PowerConfigPut(ctx3)
	assert.Equal(t, http.StatusNotFound, rec3.Code)

	// 未知 provider 422。
	ctx4, rec4 := newOpsTestContext(http.MethodPut, "/api/v1/ops/nodes/node-9/power-config", `{"provider":"smoke"}`)
	withParam(ctx4, "nodeId", "node-9")
	h.handler.PowerConfigPut(ctx4)
	assert.Equal(t, http.StatusUnprocessableEntity, rec4.Code)
}

func TestPowerList_ReturnsRequests(t *testing.T) {
	h := newPowerHarness(t, true, []string{"on"})
	for i := 0; i < 3; i++ {
		ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/power", powerSubmitBody("node-1"))
		h.handler.PowerSubmit(ctx)
		require.Equal(t, http.StatusAccepted, rec.Code)
	}
	ctx, rec := newOpsTestContext(http.MethodGet, "/api/v1/ops/power?limit=2", "")
	h.handler.PowerList(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"requests"`)
	requests, err := h.store.List(10)
	require.NoError(t, err)
	require.Len(t, requests, 3)
}
