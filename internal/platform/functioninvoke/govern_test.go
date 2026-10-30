package functioninvoke

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
)

// ── ARCH-024：统一执行准入 Govern ─────────────────────────────────────
// 禁用/分配/主体/政策审批/输入契约对 human 与 automation 主体同一条链；
// 自动化主体以创建者身份逐次重载当前授权——撤权后不继续裸派发。

func setupGovernDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func setupGovernSvc(t *testing.T) (*svc.ServiceContext, *gorm.DB) {
	t.Helper()
	db := setupGovernDB(t)
	// 配置路径不存在 → 使用硬编码默认档（medium: operator/无审批；
	// high: single_admin/需审批），与 configs/default-policies.yaml 一致。
	pm, err := policy.NewManager(db, filepath.Join(t.TempDir(), "no-policy-config.yaml"))
	require.NoError(t, err)
	return &svc.ServiceContext{
		DB:                db,
		FunctionModel:     model.NewFunctionModel(db),
		AdminModel:        model.NewAdminModel(db),
		RoleModel:         model.NewRoleModel(db),
		GameModel:         model.NewGameModel(db),
		PermissionService: permission.NewPermissionService(db),
		RegistryStore:     registry.NewStore(),
		PolicyManager:     pm,
		ApprovalsStore:    approvals.NewMemStore(),
	}, db
}

// seedGovernAdmin 建 admin+role 绑定（role Category 任填），并把 role 绑定
// function:invoke 权限——EnforceFunctionPolicy 的 Casbin 分支要求该权限
// （RequireAnyPermission "function:invoke"），缺它 operator 也会 403。
// 同时建 test-game 行 + admin_game_scopes 绑定：自动化主体走
// RequireGameEnvScope（创建者须仍有 game/env 访问），非 admin 角色也通过。
func seedGovernAdmin(t *testing.T, db *gorm.DB, username, roleName string) {
	t.Helper()
	perm := &model.Permission{ID: "function:invoke", Name: "调用函数", Resource: "function", Action: "invoke", Category: "govern-test"}
	require.NoError(t, db.Create(perm).Error)
	role := &model.Role{Name: roleName, Category: "govern-test"}
	require.NoError(t, db.Create(role).Error)
	require.NoError(t, db.Create(&model.RolePermission{RoleID: role.ID, PermissionID: perm.ID}).Error)
	admin := &model.Admin{Username: username, Nickname: username, Status: 1}
	require.NoError(t, db.Create(admin).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
	game := &model.Game{GameID: "test-game", Name: "测试游戏"}
	require.NoError(t, db.Create(game).Error)
	require.NoError(t, db.Create(&model.AdminGameScope{AdminID: admin.ID, GameID: game.ID}).Error)
}

// humanCtx 模拟鉴权中间件注入 username。
func humanCtx(username string) context.Context {
	return context.WithValue(context.Background(), "username", username)
}

func governReq() *Request {
	return &Request{
		ID:     "player.cleanup",
		GameID: "test-game",
		Env:    "test-env",
		Mode:   "async",
	}
}

// 禁用拦截：automation 与 human 同链拒绝（409 function_disabled）。
func TestGovern_DisabledFunctionDenied(t *testing.T) {
	for _, sub := range []Subject{
		{Kind: SubjectHuman},
		{Kind: SubjectAutomation, ActorUsername: "alice"},
	} {
		svcCtx, _ := setupGovernSvc(t)
		require.NoError(t, svcCtx.FunctionModel.Create(context.Background(), &model.Function{
			FunctionID: "player.cleanup", GameID: "test-game", Status: model.StatusDisabled,
		}))

		ctx := humanCtx("alice")
		if sub.Kind == SubjectAutomation {
			ctx = context.Background()
		}
		_, err := Govern(ctx, svcCtx, governReq(), sub)
		require.Error(t, err)
		var codeErr *errorx.CodeError
		require.ErrorAs(t, err, &codeErr)
		assert.Equal(t, 409, codeErr.Code)
		assert.Equal(t, "function_disabled", codeErr.ErrorCode())
	}
}

// 分配闸门：scope 有分配记录时未分配函数 403 function_not_assigned。
func TestGovern_UnassignedFunctionDenied(t *testing.T) {
	svcCtx, _ := setupGovernSvc(t)
	path := filepath.Join(t.TempDir(), "assignments.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"test-game|test-env":["player.ban"]}`), 0o644))
	svcCtx.Config.Registry.AssignmentsPath = path

	_, err := Govern(humanCtx("alice"), svcCtx, governReq(), Subject{Kind: SubjectHuman})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, 403, codeErr.Code)
	assert.Equal(t, "function_not_assigned", codeErr.ErrorCode())
}

// 自动化主体缺 actor 直接拒绝（调度行不带创建者属于定义不完整）。
func TestGovern_AutomationWithoutActorRejected(t *testing.T) {
	svcCtx, _ := setupGovernSvc(t)
	_, err := Govern(context.Background(), svcCtx, governReq(), Subject{Kind: SubjectAutomation})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "actor")
}

// 自动化主体逐次重载当前授权：ctx 只带调度上下文（无 username），
// Govern 以 ActorUsername 派生治理 ctx 并加载该主体当前 admin/roles。
func TestGovern_AutomationActorLoadsCurrentRoles(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	seedGovernAdmin(t, db, "alice", "operator")

	// ctx.Background() 模拟调度循环：无鉴权中间件，只有调度元数据。
	res, err := Govern(context.Background(), svcCtx, governReq(), Subject{
		Kind: SubjectAutomation, ActorUsername: "alice",
	})
	require.NoError(t, err)
	require.NotNil(t, res.Admin)
	assert.Equal(t, "alice", res.Admin.Username)
	assert.Equal(t, []string{"operator"}, res.RoleNames)
	assert.False(t, res.ApprovalRequired)
}

// 撤权后不继续裸派发：actor 被删除（或从未存在）时每次触发重新加载失败，
// 拒绝派发而不是沿用创建时的 actor 字符串。
func TestGovern_AutomationRevokedActorDenied(t *testing.T) {
	svcCtx, _ := setupGovernSvc(t)
	// ghost 无 admin 行：Govern 必须以错误拒绝（fail-closed），绝不返回
	// ApprovalRequired=false 的通过结果让调用方裸派发。
	_, err := Govern(context.Background(), svcCtx, governReq(), Subject{
		Kind: SubjectAutomation, ActorUsername: "ghost",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "管理员")
}

// 政策审批：高危函数（契约 risk=high → 默认 high 政策 RequireApproval）+
// admin 主体（HasAdminRole 旁路 AllowedRoles）→ 创建审批、不派发。
func TestGovern_PolicyApprovalRequired(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	seedGovernAdmin(t, db, "boss", "admin")
	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "test-game", Env: "test-env", FunctionID: "player.cleanup",
		Version: "1.0.0", Enabled: true, Risk: dbenum.RiskHigh,
	}).Error)

	res, err := Govern(humanCtx("boss"), svcCtx, governReq(), Subject{Kind: SubjectHuman})
	require.NoError(t, err)
	assert.True(t, res.ApprovalRequired)
	assert.NotEmpty(t, res.ApprovalID)
	assert.Equal(t, "single_admin", res.ApprovalWorkflow)
	// 审批记录已落库（创建者即主体）。
	stored, err := svcCtx.ApprovalsStore.Get(res.ApprovalID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, "boss", stored.Actor)
}

// 输入契约：descriptor 要求 playerId，payload 缺失在派发前 400。
func TestGovern_InputContractValidation(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	seedGovernAdmin(t, db, "alice", "operator")
	require.NoError(t, db.Create(&model.FunctionDescriptor{
		FunctionID: "player.cleanup",
		Version:    "1.0.0",
		Input: datatypes.JSONMap{
			"type":     "object",
			"required": []interface{}{"playerId"},
			"properties": map[string]interface{}{
				"playerId": map[string]interface{}{"type": "string"},
			},
		},
	}).Error)

	req := governReq()
	req.Payload = []byte(`{}`)
	_, err := Govern(humanCtx("alice"), svcCtx, req, Subject{Kind: SubjectHuman})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, 400, codeErr.Code)
	assert.Contains(t, err.Error(), "playerId")
}

// 审核发现：Govern 门序错位——ValidateInvokeInput 原先在主体/政策之前调用，
// 使无权且载荷非法的调用方由 403（权限）变为 400（输入契约）。修复后政策审批
// 先于输入契约（与 Invoke 既有顺序一致），无权者仍 403，不泄露契约细节。
func TestGovern_PolicyPrecedesInputContract(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	// viewer 角色无 function:invoke 权限 → EnforceFunctionPolicy 403。
	seedGovernAdmin(t, db, "bob", "viewer")
	require.NoError(t, db.Exec("DELETE FROM role_permissions WHERE role_id IN (SELECT id FROM roles WHERE name = ?)", "viewer").Error)
	require.NoError(t, db.Create(&model.FunctionDescriptor{
		FunctionID: "player.cleanup",
		Version:    "1.0.0",
		Input: datatypes.JSONMap{
			"type":     "object",
			"required": []interface{}{"playerId"},
			"properties": map[string]interface{}{
				"playerId": map[string]interface{}{"type": "string"},
			},
		},
	}).Error)

	req := governReq()
	req.Payload = []byte(`{}`)
	_, err := Govern(humanCtx("bob"), svcCtx, req, Subject{Kind: SubjectHuman})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	// 无权必须 403（权限），而非 400（输入契约）——政策先于契约。
	assert.Equal(t, 403, codeErr.Code)
}

// Human 主体全链通过：角色 operator 满足默认 medium 政策（allowedRoles=
// [operator]，无审批），返回主体与政策供派发/审计使用。
func TestGovern_HumanPassesAllGates(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	seedGovernAdmin(t, db, "alice", "operator")

	res, err := Govern(humanCtx("alice"), svcCtx, governReq(), Subject{Kind: SubjectHuman})
	require.NoError(t, err)
	assert.False(t, res.ApprovalRequired)
	require.NotNil(t, res.Admin)
	assert.Equal(t, "alice", res.Admin.Username)
	require.NotNil(t, res.FunctionPolicy)
	assert.False(t, res.FunctionPolicy.RequireApproval)
}

// ── ARCH-024：定义期准入 GovernInvocability（调度 Create） ────────────

// 定义期禁用拦截：Create 对已禁用函数拒绝调度（409 function_disabled），
// 与触发期同一禁用门。
func TestGovernInvocability_DisabledFunctionDenied(t *testing.T) {
	svcCtx, _ := setupGovernSvc(t)
	require.NoError(t, svcCtx.FunctionModel.Create(context.Background(), &model.Function{
		FunctionID: "player.cleanup", GameID: "test-game", Status: model.StatusDisabled,
	}))

	_, err := GovernInvocability(context.Background(), svcCtx, governReq())
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, 409, codeErr.Code)
	assert.Equal(t, "function_disabled", codeErr.ErrorCode())
}

// 定义期分配闸门：scope 有分配记录时未分配函数不可调度（403
// function_not_assigned），调度入口不能成为白名单旁路。
func TestGovernInvocability_UnassignedFunctionDenied(t *testing.T) {
	svcCtx, _ := setupGovernSvc(t)
	path := filepath.Join(t.TempDir(), "assignments.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"test-game|test-env":["player.ban"]}`), 0o644))
	svcCtx.Config.Registry.AssignmentsPath = path

	_, err := GovernInvocability(context.Background(), svcCtx, governReq())
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, 403, codeErr.Code)
	assert.Equal(t, "function_not_assigned", codeErr.ErrorCode())
}

// 定义期输入契约：payload 缺 required 参数在 Create 即拒绝（不等到触发期
// 才发现不可执行）。
func TestGovernInvocability_InputContractValidation(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	require.NoError(t, db.Create(&model.FunctionDescriptor{
		FunctionID: "player.cleanup",
		Version:    "1.0.0",
		Input: datatypes.JSONMap{
			"type":     "object",
			"required": []interface{}{"playerId"},
			"properties": map[string]interface{}{
				"playerId": map[string]interface{}{"type": "string"},
			},
		},
	}).Error)

	req := governReq()
	req.Payload = []byte(`{}`)
	_, err := GovernInvocability(context.Background(), svcCtx, req)
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, 400, codeErr.Code)
	assert.Contains(t, err.Error(), "playerId")
}

// 定义期不加载主体/不跑政策/不建审批：无 username 上下文也通过（不需要
// AdminModel），且审批 Store 保持为空——审批是执行期动作，Create 不制造
// 审批噪音。高危函数（risk=high 需审批）在定义期同样放行，审批拦截留给
// 每次触发。
func TestGovernInvocability_NoSubjectNoApproval(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "test-game", Env: "test-env", FunctionID: "player.cleanup",
		Version: "1.0.0", Enabled: true, Risk: dbenum.RiskHigh,
	}).Error)

	// context.Background()：无 username，主体加载本会失败——定义期链不触达。
	res, err := GovernInvocability(context.Background(), svcCtx, governReq())
	require.NoError(t, err)
	assert.False(t, res.ApprovalRequired)
	require.Nil(t, res.Admin)
	_, total, err := svcCtx.ApprovalsStore.List(approvals.Filter{}, approvals.Page{Page: 1, Size: 10})
	require.NoError(t, err)
	assert.Zero(t, total, "定义期不得创建审批记录")
}

// ── ARCH-025：策略查询/初始化故障禁止静默放行 ────────────────────────
// 策略存储故障（GetPolicy SQL 错误）必须 fail-closed 拒绝，绝不返回 nil,nil
// 绕过审批/审计；且不产生审批记录。

// 全链验证：function_policies 表被删（模拟存储故障）→ Govern 以错误拒绝，
// 不派发、不建审批。此前 EnforceFunctionPolicy 在此路径返回 nil,nil 放行。
func TestGovern_PolicyLookupErrorDenies(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	seedGovernAdmin(t, db, "alice", "operator")
	require.NoError(t, db.Migrator().DropTable(&model.FunctionPolicy{}))

	_, err := Govern(humanCtx("alice"), svcCtx, governReq(), Subject{Kind: SubjectHuman})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "查询函数策略失败")

	_, total, err := svcCtx.ApprovalsStore.List(approvals.Filter{}, approvals.Page{Page: 1, Size: 10})
	require.NoError(t, err)
	assert.Zero(t, total, "策略故障不得静默创建审批/放行")
}

// 单元级：EnforceFunctionPolicy 在 GetPolicy 故障时返回错误而非 nil,nil。
func TestEnforceFunctionPolicy_GetPolicyErrorDenies(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	require.NoError(t, db.Migrator().DropTable(&model.FunctionPolicy{}))

	fp, err := EnforceFunctionPolicy(context.Background(), svcCtx, "player.cleanup", "test-game", "test-env", []string{"operator"})
	require.Error(t, err)
	require.Nil(t, fp, "故障时不得返回空政策让调用方视为无限制放行")
	assert.Contains(t, err.Error(), "查询函数策略失败")
}

// ── ARCH-027：输入契约读取失败的明确拒绝语义 ─────────────────────────
// 契约查询失败必须与「旧函数确无 schema」区分：查询失败 fail-closed 拒绝
// （不得作为跳过验证依据），确无 descriptor 才跳过。

// 全链：function_descriptors 表被删（存储故障）→ Govern 拒绝、不建审批。
func TestGovern_InputSchemaReadErrorDenies(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	seedGovernAdmin(t, db, "alice", "operator")
	require.NoError(t, db.Migrator().DropTable(&model.FunctionDescriptor{}))

	_, err := Govern(humanCtx("alice"), svcCtx, governReq(), Subject{Kind: SubjectHuman})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "输入契约")

	_, total, err := svcCtx.ApprovalsStore.List(approvals.Filter{}, approvals.Page{Page: 1, Size: 10})
	require.NoError(t, err)
	assert.Zero(t, total, "契约读取故障不得静默放行/建审批")
}

// 单元级：ValidateInvokeInput 在 ListDescriptors 故障时返回错误而非 nil。
func TestValidateInvokeInput_SchemaReadErrorDenies(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	require.NoError(t, db.Migrator().DropTable(&model.FunctionDescriptor{}))

	err := ValidateInvokeInput(context.Background(), svcCtx, governReq(), []byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "读取函数输入契约失败")
}

// 版本漂移：多个 descriptor 版本时校验取最新（ListDescriptors 按 version
// DESC，descs[0]=最新）。v1 无 required、v2 要求 playerId——校验 v2 才会报错，
// 证明没有回退到旧版本契约。
func TestValidateInvokeInput_UsesLatestDescriptorVersion(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	require.NoError(t, db.Create(&model.FunctionDescriptor{
		FunctionID: "player.cleanup", Version: "1.0.0",
		Input: datatypes.JSONMap{"type": "object", "properties": map[string]interface{}{}},
	}).Error)
	require.NoError(t, db.Create(&model.FunctionDescriptor{
		FunctionID: "player.cleanup", Version: "2.0.0",
		Input: datatypes.JSONMap{
			"type":     "object",
			"required": []interface{}{"playerId"},
			"properties": map[string]interface{}{
				"playerId": map[string]interface{}{"type": "string"},
			},
		},
	}).Error)

	// 按最新契约（v2）校验：缺 playerId 应报错；若误用 v1 则无错。
	err := ValidateInvokeInput(context.Background(), svcCtx, governReq(), []byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "playerId")
}
