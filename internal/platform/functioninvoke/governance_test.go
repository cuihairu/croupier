package functioninvoke

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	policymgr "github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// governanceFixture 构建治理闸门所需的 svcCtx：AdminModel/RoleModel/
// FunctionModel/PolicyManager 齐备 + admin 用户 "tester"。policyManager 为
// nil 时表示不注入（fail-closed 用例）。
type governanceFixture struct {
	db        *gorm.DB
	svcCtx    *svc.ServiceContext
	policyMgr *policymgr.Manager
	adminCtx  context.Context
}

func newGovernanceFixture(t *testing.T) *governanceFixture {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	pm, err := policymgr.NewManager(db, "")
	require.NoError(t, err)

	role := &model.Role{Name: "admin", Category: "test"}
	require.NoError(t, roleModel.Create(context.Background(), role))
	admin := &model.Admin{Username: "tester", Nickname: "Tester", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "pw"))
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))

	svcCtx := &svc.ServiceContext{
		DB:            db,
		AdminModel:    adminModel,
		RoleModel:     roleModel,
		FunctionModel: model.NewFunctionModel(db),
		PolicyManager: pm,
	}
	return &governanceFixture{
		db:        db,
		svcCtx:    svcCtx,
		policyMgr: pm,
		adminCtx:  context.WithValue(context.Background(), "username", "tester"),
	}
}

func TestGovernance_HumanHappyPath(t *testing.T) {
	f := newGovernanceFixture(t)
	g := NewGovernance(f.svcCtx)

	res, err := g.Gate(f.adminCtx, Subject{}, GateInput{FunctionID: "cov.fn", GameID: "demo", Env: "prod", Payload: []byte(`{}`)})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.NotNil(t, res.Admin)
	assert.Equal(t, []string{"admin"}, res.RoleNames)
	assert.False(t, res.RequireApproval)
}

func TestGovernance_EmptyFunctionID(t *testing.T) {
	f := newGovernanceFixture(t)
	g := NewGovernance(f.svcCtx)

	_, err := g.Gate(f.adminCtx, Subject{}, GateInput{FunctionID: "  ", GameID: "demo", Env: "prod"})
	require.Error(t, err)
	var ce *errorx.CodeError
	require.True(t, errors.As(err, &ce), "期望 errorx.CodeError，得到 %T", err)
	assert.Equal(t, 400, ce.Code)
}

func TestGovernance_NilPolicyManager_FailClosed(t *testing.T) {
	f := newGovernanceFixture(t)
	f.svcCtx.PolicyManager = nil // 配置故障：非「无覆盖策略」
	g := NewGovernance(f.svcCtx)

	_, err := g.Gate(f.adminCtx, Subject{}, GateInput{FunctionID: "cov.fn", GameID: "demo", Env: "prod", Payload: []byte(`{}`)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy manager unavailable")
}

func TestGovernance_RequireApproval_NoSnapshot(t *testing.T) {
	f := newGovernanceFixture(t)
	require.NoError(t, f.policyMgr.SetOverride(context.Background(), "cov.secure", &policymgr.Policy{
		FunctionID: "cov.secure", RequireApproval: true, ApprovalWorkflow: "single_admin",
	}))
	g := NewGovernance(f.svcCtx)

	res, err := g.Gate(f.adminCtx, Subject{}, GateInput{FunctionID: "cov.secure", GameID: "demo", Env: "prod", Payload: []byte(`{}`)})
	require.NoError(t, err)
	assert.True(t, res.RequireApproval)
	assert.Equal(t, "single_admin", res.ApprovalWorkflow)
}

func TestGovernance_RequireApproval_SatisfiedBySnapshot(t *testing.T) {
	f := newGovernanceFixture(t)
	require.NoError(t, f.policyMgr.SetOverride(context.Background(), "cov.secure", &policymgr.Policy{
		FunctionID: "cov.secure", RequireApproval: true,
	}))
	g := NewGovernance(f.svcCtx)

	// 批准续行快照（approvalBypass=approved）：满足审批要求，不再二次审批。
	res, err := g.Gate(f.adminCtx, Subject{}, GateInput{
		FunctionID: "cov.secure", GameID: "demo", Env: "prod", Payload: []byte(`{}`),
		Metadata: map[string]string{"approvalBypass": "approved"},
	})
	require.NoError(t, err)
	assert.False(t, res.RequireApproval)
}

func TestGovernance_ApprovedContinuation_SkipsPolicyManager(t *testing.T) {
	f := newGovernanceFixture(t)
	f.svcCtx.PolicyManager = nil // 续行豁免政策闸，不再需要 PolicyManager
	g := NewGovernance(f.svcCtx)

	// 批准续行：函数级政策豁免（上游已把关），但函数权限/输入契约仍 live 执行。
	res, err := g.Gate(f.adminCtx, Subject{}, GateInput{
		FunctionID: "cov.fn", GameID: "demo", Env: "prod", Payload: []byte(`{}`),
		Metadata: map[string]string{"approvalBypass": "approved"},
	})
	require.NoError(t, err)
	assert.Nil(t, res.FunctionPolicy)
	assert.False(t, res.RequireApproval)
}

func TestGovernance_AutomatedSubject_EnforcesPolicy(t *testing.T) {
	f := newGovernanceFixture(t)
	f.svcCtx.AdminModel = nil // 自动化主体不需要管理员加载
	require.NoError(t, f.policyMgr.SetOverride(context.Background(), "cov.cron", &policymgr.Policy{
		FunctionID: "cov.cron", RequireApproval: true,
	}))
	g := NewGovernance(f.svcCtx)

	res, err := g.Gate(context.Background(), Subject{AutomatedID: "schedule:9"}, GateInput{
		FunctionID: "cov.cron", GameID: "demo", Env: "prod", Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	assert.True(t, res.RequireApproval, "自动化主体必须 live 回读政策（撤权后不裸派发）")
	assert.Nil(t, res.Admin)
}

func TestGovernance_DisabledFunction(t *testing.T) {
	f := newGovernanceFixture(t)
	require.NoError(t, f.svcCtx.FunctionModel.Create(context.Background(), &model.Function{
		FunctionID: "cov.disabled", Status: model.StatusDisabled,
	}))
	g := NewGovernance(f.svcCtx)

	_, err := g.Gate(f.adminCtx, Subject{}, GateInput{FunctionID: "cov.disabled", GameID: "demo", Env: "prod"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已禁用")
}

func TestCheckGovernance_SkipOnRequireApproval(t *testing.T) {
	f := newGovernanceFixture(t)
	require.NoError(t, f.policyMgr.SetOverride(context.Background(), "cov.cron", &policymgr.Policy{
		FunctionID: "cov.cron", RequireApproval: true,
	}))
	g := NewGovernance(f.svcCtx)

	skip, err := g.CheckGovernance(context.Background(), "cov.cron", "demo", "prod", []byte(`{}`), nil, "", "schedule:9")
	require.NoError(t, err)
	assert.True(t, skip, "政策要求审批且无批准快照 → cron 必须跳过派发")
}

func TestCheckGovernance_SnapshotNoSkip(t *testing.T) {
	f := newGovernanceFixture(t)
	require.NoError(t, f.policyMgr.SetOverride(context.Background(), "cov.cron", &policymgr.Policy{
		FunctionID: "cov.cron", RequireApproval: true,
	}))
	g := NewGovernance(f.svcCtx)

	skip, err := g.CheckGovernance(context.Background(), "cov.cron", "demo", "prod", []byte(`{}`),
		map[string]string{"approvalBypass": "approved"}, "", "schedule:9")
	require.NoError(t, err)
	assert.False(t, skip, "批准快照满足审批要求 → 放行派发")
}

func TestCheckGovernance_CreatorRevokedSkips(t *testing.T) {
	f := newGovernanceFixture(t)
	// 政策限制 operator 角色；creator "alice" 不存在 → 回载角色为空 → 拒绝。
	require.NoError(t, f.policyMgr.SetOverride(context.Background(), "cov.cron", &policymgr.Policy{
		FunctionID: "cov.cron", AllowedRoles: []string{"operator"},
	}))
	g := NewGovernance(f.svcCtx)

	skip, err := g.CheckGovernance(context.Background(), "cov.cron", "demo", "prod", []byte(`{}`), nil, "alice", "schedule:9")
	require.Error(t, err)
	assert.True(t, skip, "creator 被撤权（角色不匹配）→ fail-closed 跳过派发")
}
