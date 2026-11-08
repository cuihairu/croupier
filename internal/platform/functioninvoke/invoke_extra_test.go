package functioninvoke

// Invoke 链补测：scope 缺失 400、路由语义前置拦截、Govern 拒绝透传、
// 审批待审响应（含空路由默认 lb 落库）、审批创建失败收口、自动化主体
// scope 复核拒绝、metadata 归集与 timeoutMs 注入臂、审计与执行留痕接入
// 面、异步派发失败臂，以及 ApprovalNotifyRecipients 的成功/失败臂与
// CreateFunctionApproval 的通知块（接收人列表、空用户名跳过、错误兜底）。
//
// 诚实跳过：govern 的 PermissionIDsFromRoles 错误臂——真实 DB 查询无法
// 在不破坏前序闸门的情况下注入失败（闭库会先炸 LoadCurrentAdmin）；
// resolvePolicyRisk 的 GetOpenAPI 克隆序列化失败臂——store 内部拷贝路径
// 无外部注入缝。

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/service/notify"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// failingApprovalStore 只拒绝 Create（嵌入接口其余方法不触碰）。
type failingApprovalStore struct {
	approvals.Store
}

func (f failingApprovalStore) Create(*approvals.Approval) (*approvals.Approval, error) {
	return nil, errors.New("store down")
}

func TestInvokeRequiresGameScope(t *testing.T) {
	_, err := Invoke(context.Background(), nil, &Request{ID: "demo.echo"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "scope 缺失")
}

func TestInvokeRejectsInvalidRouteBeforeGovern(t *testing.T) {
	svcCtx, _ := setupDispatchSvc(t, nil)
	req := syncReq("demo.echo")
	req.Route = "targeted" // 缺 target_service_id
	_, err := Invoke(dispatchCtx(), svcCtx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "target_service_id")
}

func TestInvokeGovernDeniesDisabledFunction(t *testing.T) {
	svcCtx, _ := setupDispatchSvc(t, nil)
	require.NoError(t, svcCtx.FunctionModel.Create(context.Background(), &model.Function{
		FunctionID: "player.cleanup", GameID: "test-game", Status: model.StatusDisabled,
	}))
	_, err := Invoke(dispatchCtx(), svcCtx, governReq())
	require.Error(t, err)
	require.Contains(t, err.Error(), "函数已禁用")
}

// seedHighRiskContract 建高危函数契约（默认 high 政策 RequireApproval）。
func seedHighRiskContract(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "test-game", Env: "test-env", FunctionID: "player.cleanup",
		Version: "1.0.0", Enabled: true, Risk: dbenum.RiskHigh,
	}).Error)
}

func TestInvokeReturnsApprovalResponse(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	seedGovernAdmin(t, db, "boss", "admin")
	seedHighRiskContract(t, db)
	// high 政策 RequireAudit=true：审批创建同步落审计（AuditApprovalCreated）
	svcCtx.AuditService = audit.NewAuditService(audit.NewInMemoryAuditStore(), nil)

	resp, err := Invoke(svc.WithGameScope(humanCtx("boss"),
		svc.GameScope{GameID: "test-game", Env: "test-env"}), svcCtx, governReq())
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.ApprovalRequired)
	require.NotEmpty(t, resp.ApprovalID)
	require.Empty(t, resp.TaskId)

	// 空路由默认 lb 落库（审批存储需可读路由）
	stored, err := svcCtx.ApprovalsStore.Get(resp.ApprovalID)
	require.NoError(t, err)
	require.Equal(t, "lb", stored.Route)
	require.Equal(t, "boss", stored.Actor)
}

func TestInvokeGovernApprovalCreateFailure(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	seedGovernAdmin(t, db, "boss", "admin")
	seedHighRiskContract(t, db)
	svcCtx.ApprovalsStore = failingApprovalStore{}

	_, err := Invoke(svc.WithGameScope(humanCtx("boss"),
		svc.GameScope{GameID: "test-game", Env: "test-env"}), svcCtx, governReq())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create approval request")
}

// 自动化主体 scope 复核：actor 权限仍在但 game 绑定被移除后，逐次
// 重授权拒绝裸派发（撤 scope 即时生效）。
func TestInvokeAutomationScopeRecheck(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	seedGovernAdmin(t, db, "alice", "operator")
	// 硬删 scope 绑定：CheckGameScope 走裸 Table 计数，软删行仍会命中
	require.NoError(t, db.Unscoped().Where("1 = 1").Delete(&model.AdminGameScope{}).Error)

	_, err := Govern(context.Background(), svcCtx, governReq(),
		Subject{Kind: SubjectAutomation, ActorUsername: "alice"})
	require.Error(t, err)
}

func TestInvokeMetadataAndTimeoutArms(t *testing.T) {
	caller := &stubCaller{invokePayload: []byte(`{"ok":1}`)}
	svcCtx, db := setupDispatchSvc(t, map[string]transport.SessionCaller{"a1": caller})
	seedGovernAdmin(t, db, "alice", "operator")
	// targeted 路由按 ProviderID 选代理：注册会话须携带 provider 归属
	require.NoError(t, svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "a1", GameID: "test-game", Env: "test-env", Addr: "127.0.0.1:9100",
		Functions: map[string]reg.FunctionMeta{"demo.echo": {Enabled: true}},
		Providers: []reg.ProviderSession{{
			ProviderID: "svc-a1", GameID: "test-game", Env: "test-env",
			FunctionIDs: []string{"demo.echo"},
		}},
		ExpireAt: time.Now().Add(5 * time.Minute), LastSeen: time.Now(),
	}))

	req := syncReq("demo.echo")
	req.Route = "targeted"
	req.TargetServiceID = "svc-a1"
	req.HashKey = "player-42" // 元数据透传
	req.TimeoutMs = 5000      // 同步预算注入 timeoutMs
	req.Metadata = map[string]string{
		"":      "dropped", // 空 key 丢弃
		"blank": "   ",     // 空值丢弃
		"k1":    "v1",
	}

	resp, err := Invoke(dispatchCtx(), svcCtx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	md := resp.ExecutionMetadata
	require.Equal(t, "v1", md["k1"])
	require.Equal(t, "5000", md["timeoutMs"])
	require.Equal(t, "svc-a1", md["targetServiceId"])
	require.Equal(t, "player-42", md["hashKey"])
	require.Equal(t, "test-game", md["gameId"])
	require.Equal(t, "alice", md["actor"])
	require.NotContains(t, md, "")
	require.NotContains(t, md, "blank")
}

func TestInvokeAuditAndExecutionLogArms(t *testing.T) {
	caller := &stubCaller{invokePayload: []byte(`{"done":true}`)}
	svcCtx, db := setupDispatchSvc(t, map[string]transport.SessionCaller{"a1": caller})
	seedGovernAdmin(t, db, "alice", "operator")
	registerDispatchAgent(t, svcCtx.RegistryStore, "a1", "demo.echo")
	// 默认 medium 政策 RequireAudit=true：接上审计与执行留痕写入器
	svcCtx.AuditService = audit.NewAuditService(audit.NewInMemoryAuditStore(), nil)
	svcCtx.ExecutionLogWriter = executionlog.NewWriter(db, executionlog.Config{})

	resp, err := Invoke(dispatchCtx(), svcCtx, syncReq("demo.echo"))
	require.NoError(t, err)
	require.NotNil(t, resp.Result)
}

func TestInvokeAsyncDispatchError(t *testing.T) {
	// 空 resolver：无活代理 → StartTask 失败 → 错误响应（TaskId 空）
	svcCtx, db := setupDispatchSvc(t, map[string]transport.SessionCaller{})
	seedGovernAdmin(t, db, "alice", "operator")
	svcCtx.ExecutionLogWriter = executionlog.NewWriter(db, executionlog.Config{})

	req := syncReq("demo.echo")
	req.Mode = "async"
	resp, err := Invoke(dispatchCtx(), svcCtx, req)
	require.Error(t, err)
	require.NotNil(t, resp)
	require.Empty(t, resp.TaskId)
}

func TestApprovalNotifyRecipientsSkipsBlankUsernames(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	seedGovernAdmin(t, db, "bee", "admin")
	// 空用户名 admin 复用同一 admin 角色（seedGovernAdmin 每次都重建
	// function:invoke 权限，重复调用会撞唯一约束）
	var role model.Role
	require.NoError(t, db.Where("name = ?", "admin").First(&role).Error)
	blank := &model.Admin{Username: "", Nickname: "blank", Status: 1}
	require.NoError(t, db.Create(blank).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: blank.ID, RoleID: role.ID}).Error)

	recipients, err := ApprovalNotifyRecipients(context.Background(), svcCtx)
	require.NoError(t, err)
	require.Equal(t, []string{"bee"}, recipients)
}

func TestCreateFunctionApprovalNotifiesActor(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	seedGovernAdmin(t, db, "bee", "admin")
	svcCtx.NotifyService = notify.New(nil, nil) // 无渠道：静默跳过外部分发

	req := &Request{ID: "player.cleanup", GameID: "test-game", Env: "test-env"}
	approvalID, err := CreateFunctionApproval(context.Background(), svcCtx, req,
		[]byte(`{}`), &model.Admin{Username: "bee"}, &policy.Policy{})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(approvalID, "func_player.cleanup_"), approvalID)

	stored, err := svcCtx.ApprovalsStore.Get(approvalID)
	require.NoError(t, err)
	require.Equal(t, "lb", stored.Route, "empty route defaults to lb")
}

func TestCreateFunctionApprovalNotifyRecipientFailureFallback(t *testing.T) {
	// 接收人列表失败（库已关闭）不阻断审批创建：recipients 兜底为申请人
	db := setupGovernDB(t)
	adminModel := model.NewAdminModel(db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	svcCtx := &svc.ServiceContext{
		AdminModel:     adminModel,
		ApprovalsStore: approvals.NewMemStore(),
		NotifyService:  notify.New(nil, nil),
	}
	req := &Request{ID: "player.cleanup"}
	approvalID, err := CreateFunctionApproval(context.Background(), svcCtx, req,
		[]byte(`{}`), &model.Admin{Username: "solo"}, &policy.Policy{})
	require.NoError(t, err)
	require.NotEmpty(t, approvalID)
}

func TestRawJSONFromAnyArms(t *testing.T) {
	require.Nil(t, RawJSONFromAny(nil))

	// []byte 分支：合法 JSON 原样、纯文本字符串编码
	require.JSONEq(t, `{"a":1}`, string(RawJSONFromAny([]byte(`{"a":1}`))))
	require.Equal(t, `"plain text"`, string(RawJSONFromAny([]byte("plain text"))))

	// 不可序列化值 → nil 兜底
	require.Nil(t, RawJSONFromAny(make(chan int)))
}

func TestPolicyRiskFromContractVocabulary(t *testing.T) {
	for _, pair := range []struct {
		in   string
		want policy.RiskLevel
	}{
		{"safe", policy.RiskLow},
		{"warning", policy.RiskMedium},
		{"high", policy.RiskHigh},
		{"danger", policy.RiskDanger},
		{"", policy.RiskMedium},     // 未声明回落 medium
		{" HIGH ", policy.RiskHigh}, // 大小写与空白归一
		{"other", policy.RiskMedium},
	} {
		if got := PolicyRiskFromContract(pair.in); got != pair.want {
			t.Fatalf("PolicyRiskFromContract(%q) = %v, want %v", pair.in, got, pair.want)
		}
	}
}

func TestResolvePolicyRiskFromOpenAPIExtension(t *testing.T) {
	svcCtx, _ := setupGovernSvc(t)
	ctx := context.Background()

	// openapiOperations 的 x-risk 是本次启动内的显式声明（含 safe）
	require.NoError(t, svcCtx.RegistryStore.UpsertOpenAPI("demo.echo",
		&openapi3.Operation{Extensions: map[string]interface{}{"x-risk": "safe"}}))
	level, err := ResolvePolicyRisk(ctx, svcCtx, "demo.echo", "test-game", "test-env")
	require.NoError(t, err)
	require.Equal(t, policy.RiskLow, level)

	// 无 x-risk 的 OpenAPI 记录不采信（safe 与未声明不可区分）→ 契约/medium
	require.NoError(t, svcCtx.RegistryStore.UpsertOpenAPI("demo.plain", &openapi3.Operation{}))
	level, err = ResolvePolicyRisk(ctx, svcCtx, "demo.plain", "test-game", "test-env")
	require.NoError(t, err)
	require.Equal(t, policy.RiskMedium, level)
}

// seedContractTimeout 建带同步预算声明的契约。
func seedContractTimeout(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "test-game", Env: "test-env", FunctionID: "demo.echo",
		Version: "1.0.0", Enabled: true, TimeoutMs: 3000,
	}).Error)
}

func TestEffectiveTimeoutMsDeclaredContract(t *testing.T) {
	svcCtx, db := setupDispatchSvc(t, nil)
	seedContractTimeout(t, db)
	req := &Request{ID: "demo.echo", GameID: "test-game", Env: "test-env"}

	declared := DeclaredTimeoutMs(dispatchCtx(), svcCtx, req)
	require.Equal(t, 3000, declared)

	// 显式与声明并存：min 归一（声明即契约，显式不得突破）
	require.Equal(t, 3000, EffectiveTimeoutMs(dispatchCtx(), svcCtx,
		&Request{ID: "demo.echo", GameID: "test-game", Env: "test-env", TimeoutMs: 5000}))
	require.Equal(t, 1000, EffectiveTimeoutMs(dispatchCtx(), svcCtx,
		&Request{ID: "demo.echo", GameID: "test-game", Env: "test-env", TimeoutMs: 1000}))
	// 只有声明 / 负值显式钳零后回落声明
	require.Equal(t, 3000, EffectiveTimeoutMs(dispatchCtx(), svcCtx,
		&Request{ID: "demo.echo", GameID: "test-game", Env: "test-env"}))
	require.Equal(t, 3000, EffectiveTimeoutMs(dispatchCtx(), svcCtx,
		&Request{ID: "demo.echo", GameID: "test-game", Env: "test-env", TimeoutMs: -5}))
}

func TestInvokePayloadAndBroadcastResponseNil(t *testing.T) {
	require.Equal(t, "null", string(InvokePayload(nil)))

	resp := BuildBroadcastResponse(nil)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Broadcast)
	require.Equal(t, 0, resp.Broadcast.Total)
}

func TestEnforceFunctionPolicyAllowedRolesArms(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	seedGovernAdmin(t, db, "alice", "operator")
	ctx := humanCtx("alice")

	// DB override：AllowedRoles 与 alice 角色不符 → casbin 通过后仍按
	// 角色白名单拒绝（带角色清单的显式错误）
	require.NoError(t, svcCtx.PolicyManager.SetOverride(ctx, "fn.roles", &policy.Policy{
		RequireAudit: true, AllowedRoles: []string{"auditor"},
	}))
	_, err := EnforceFunctionPolicy(ctx, svcCtx, "fn.roles", "test-game", "test-env",
		[]string{"operator"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "需要角色")

	// override 空白名单 → 无角色限制直接放行
	require.NoError(t, svcCtx.PolicyManager.SetOverride(ctx, "fn.open", &policy.Policy{
		RequireAudit: true,
	}))
	p, err := EnforceFunctionPolicy(ctx, svcCtx, "fn.open", "test-game", "test-env",
		[]string{"operator"})
	require.NoError(t, err)
	require.NotNil(t, p)

	// AdminModel 缺位 → 简单角色匹配兜底：命中放行、不命中拒绝
	// （ServiceContext 含锁字段不可整值拷贝，按需重建）
	fallback := &svc.ServiceContext{
		DB:            svcCtx.DB,
		RegistryStore: svcCtx.RegistryStore,
		PolicyManager: svcCtx.PolicyManager,
	}
	require.NoError(t, svcCtx.PolicyManager.SetOverride(ctx, "fn.only-op", &policy.Policy{
		RequireAudit: true, AllowedRoles: []string{"operator"},
	}))
	p, err = EnforceFunctionPolicy(ctx, fallback, "fn.only-op", "test-game", "test-env",
		[]string{"operator"})
	require.NoError(t, err)
	require.NotNil(t, p)

	_, err = EnforceFunctionPolicy(ctx, fallback, "fn.only-op", "test-game", "test-env",
		[]string{"ghost"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "无权调用该函数")
}

// casbin 路径的主体加载失败（ctx 无 username）→ 权限检查前置失败收口。
func TestEnforceFunctionPolicySubjectLoadFailure(t *testing.T) {
	svcCtx, db := setupGovernSvc(t)
	seedGovernAdmin(t, db, "alice", "operator")
	require.NoError(t, svcCtx.PolicyManager.SetOverride(context.Background(), "fn.op",
		&policy.Policy{RequireAudit: true, AllowedRoles: []string{"operator"}}))

	_, err := EnforceFunctionPolicy(context.Background(), svcCtx, "fn.op",
		"test-game", "test-env", []string{"operator"})
	require.Error(t, err)
}

func TestValidateInvokeInputNilGuards(t *testing.T) {
	req := &Request{ID: "demo.echo"}
	require.NoError(t, ValidateInvokeInput(context.Background(), nil, req, []byte(`{}`)))

	svcCtx, _ := setupDispatchSvc(t, nil)
	svcCtx.FunctionModel = nil
	require.NoError(t, ValidateInvokeInput(context.Background(), svcCtx, req, []byte(`{}`)))

	// FunctionModel 在位但 functionID 为空 → 无契约可查，放行
	fullSvc, _ := setupDispatchSvc(t, nil)
	require.NoError(t, ValidateInvokeInput(context.Background(), fullSvc, &Request{}, []byte(`{}`)))
}

func TestInvokeBroadcastPartialFailure(t *testing.T) {
	okCaller := &stubCaller{invokePayload: []byte(`{"from":"a1"}`)}
	badCaller := &stubCaller{err: context.DeadlineExceeded}
	svcCtx, db := setupDispatchSvc(t, map[string]transport.SessionCaller{
		"a1": okCaller,
		"a2": badCaller,
	})
	seedGovernAdmin(t, db, "alice", "operator")
	registerDispatchAgent(t, svcCtx.RegistryStore, "a1", "demo.echo")
	registerDispatchAgent(t, svcCtx.RegistryStore, "a2", "demo.echo")

	req := syncReq("demo.echo")
	req.Route = "broadcast"
	resp, err := Invoke(dispatchCtx(), svcCtx, req)
	require.NoError(t, err)
	require.NotNil(t, resp.Broadcast)
	require.Equal(t, 1, resp.Broadcast.Success)
	require.Equal(t, 1, resp.Broadcast.Failure)
	require.NotEmpty(t, resp.Broadcast.Results[1].Error)
}
