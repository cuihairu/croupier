// 覆盖目标：ExecuteDraftCompositeBinding（POST /api/v1/versioning/pages/composite/
// preview/execute，草稿预览执行入口）——服务端按生成链重建草稿 spec 并跑发布级
// 校验，再走与已发布页面共用的执行门禁；治理/运行时 API 以 draft 标记，产物
// 不落库。以及 executeBindingFromSnapshot 的门禁（unknown binding、契约权限、
// 审批闸）。
package console

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
)

const draftInputSchema = `{"type":"object","properties":{"keyword":{"type":"string"}}}`
const draftOutputSchema = `{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"number"}}}`

// draftTableRequest 单表格区块草稿请求（bindingId 由生成链按区块 key 分配）。
func draftTableRequest(pageKey, bindingID string) *ConsoleDraftCompositeExecuteRequest {
	return &ConsoleDraftCompositeExecuteRequest{
		PageKey:   pageKey,
		BindingID: bindingID,
		Sections: []contractsvc.CompositeSectionRequest{{
			Key:        "player.list",
			FunctionID: "player.query",
			View:       "table",
			Title:      "玩家列表",
			AutoRun:    true,
		}},
		Context: ConsoleBindingExecutionContext{
			Form: json.RawMessage(`{"keyword":"alice"}`),
		},
	}
}

func TestDraftExecuteBinding_Success(t *testing.T) {
	service, ctx, auditStore := newConsoleTestServiceWithAudit(t, "function:invoke", "player:query")
	require.NoError(t, upsertConsoleFunctionContract(service.svcCtx, ctx, draftInputSchema, draftOutputSchema, "safe", "player:query"))

	caller := &fakeConsoleSessionCaller{payload: []byte(`{"items":[],"total":0}`)}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{caller: caller})

	resp, err := service.ExecuteDraftCompositeBinding(ctx, draftTableRequest("draft--players", "player.list"))
	require.NoError(t, err)
	assert.Equal(t, spec.PageExecutionKindSync, resp.Result.Kind)
	assert.JSONEq(t, `{"items":[],"total":0}`, string(resp.Result.Data))
	require.NotEmpty(t, resp.Result.RequestID)

	// 执行链标记草稿治理与草稿运行时 API，审计可区分发布页与预览执行。
	req := caller.lastRequest
	require.NotNil(t, req)
	assert.JSONEq(t, `{}`, string(req.Payload), "表格区块无 inputAssignments 时 payload 为空对象")
	assert.Equal(t, "draft", req.Metadata["pageSnapshotGovernance"])
	assert.Equal(t, "versioning.composite.preview.execute", req.Metadata["page_runtime_api"])
	assert.Equal(t, "draft--players", req.Metadata["page_key"])
	assert.Equal(t, "player.list", req.Metadata["binding_id"])

	records, total, err := auditStore.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventPageExecute},
	}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, records, 1)
	assert.Equal(t, "success", records[0].Outcome)
	assert.Equal(t, "draft--players", records[0].Resource.ID)
	assert.Equal(t, "draft", records[0].Details["page_snapshot_governance"])
	assert.Equal(t, "versioning.composite.preview.execute", records[0].Details["page_runtime_api"])
	assert.Equal(t, string(spec.PageExecutionKindSync), records[0].Details["result_kind"])

	// 草稿执行是只读链路：不写提案、不写发布快照。
	var proposals, published int64
	require.NoError(t, service.svcCtx.DB.Model(&model.PageProposal{}).Count(&proposals).Error)
	require.NoError(t, service.svcCtx.DB.Model(&model.PublishedPageSpec{}).Count(&published).Error)
	assert.Zero(t, proposals, "草稿执行不得写入提案")
	assert.Zero(t, published, "草稿执行不得写入发布快照")
}

func TestDraftExecuteBinding_RequiresFunctionInvokePermission(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	require.NoError(t, upsertConsoleFunctionContract(service.svcCtx, ctx, draftInputSchema, draftOutputSchema, "safe", "player:query"))

	_, err := service.ExecuteDraftCompositeBinding(ctx, draftTableRequest("draft--denied", "player.list"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权执行运行控制台操作")
}

func TestDraftExecuteBinding_RequiresBindingPermission(t *testing.T) {
	// 仅持有运行控制台权限，缺契约声明的 player:query → 与发布页同规拒绝。
	service, ctx := newConsoleTestService(t, "function:invoke")
	require.NoError(t, upsertConsoleFunctionContract(service.svcCtx, ctx, draftInputSchema, draftOutputSchema, "safe", "player:query"))

	_, err := service.ExecuteDraftCompositeBinding(ctx, draftTableRequest("draft--noperm", "player.list"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权执行该页面操作")
}

func TestDraftExecuteBinding_ContractMissing(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")

	_, err := service.ExecuteDraftCompositeBinding(ctx, draftTableRequest("draft--nocontract", "player.list"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contract")
}

func TestDraftExecuteBinding_UnknownBinding(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	require.NoError(t, upsertConsoleFunctionContract(service.svcCtx, ctx, draftInputSchema, draftOutputSchema, "safe", "player:query"))

	_, err := service.ExecuteDraftCompositeBinding(ctx, draftTableRequest("draft--unknown", "nope.none"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page binding not found")
}

func TestDraftExecuteBinding_BlankKeysRejected(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")

	_, err := service.ExecuteDraftCompositeBinding(ctx, draftTableRequest(" ", "player.list"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")

	_, err = service.ExecuteDraftCompositeBinding(ctx, draftTableRequest("draft--x", " "))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bindingId is required")
}

// 发布级校验未通过（selector 指向契约不存在的字段）在执行前 422 拒绝，
// 而不是让 payload 求值在下游失败成不可定位的错误。
func TestDraftExecuteBinding_FailsPublishValidation(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	require.NoError(t, upsertConsoleFunctionContract(service.svcCtx, ctx, draftInputSchema, draftOutputSchema, "safe", "player:query"))

	req := draftTableRequest("draft--badselector", "player.list")
	req.Sections[0].InputAssignments = []contractsvc.CompositeInputAssignmentRequest{{
		Target: "/doesNotExists", Kind: "literal", Value: json.RawMessage(`"x"`),
	}}

	_, err := service.ExecuteDraftCompositeBinding(ctx, req)
	require.Error(t, err)
	var validation *errorx.ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, 422, validation.CodeError.Code)
	assert.Contains(t, err.Error(), "draft page fails publish validation")
	assert.NotEmpty(t, validation.Details, "诊断压平为稳定机器码 details")
}

// 契约声明审批 → 草稿执行同样走审批闸：不直接下发，落审批单且带草稿治理。
func TestDraftExecuteBinding_CreatesApprovalWhenRequired(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	store := approvals.NewMemStore()
	service.svcCtx.ApprovalsStore = store
	require.NoError(t, seedDraftApprovingContract(service.svcCtx, ctx))

	caller := &fakeConsoleSessionCaller{payload: []byte(`{"ok":true}`)}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{caller: caller})

	resp, err := service.ExecuteDraftCompositeBinding(ctx, draftTableRequest("draft--approval", "player.list"))
	require.NoError(t, err)
	assert.Equal(t, spec.PageExecutionKindApproval, resp.Result.Kind)
	require.NotEmpty(t, resp.Result.ApprovalID)
	assert.Empty(t, caller.lastRequest, "审批闸命中时不得下发执行")

	stored, getErr := store.Get(resp.Result.ApprovalID)
	require.NoError(t, getErr)
	assert.Equal(t, "pending", stored.State)
	assert.Equal(t, "player.query", stored.FunctionID)
	assert.Equal(t, "draft", stored.Metadata["pageSnapshotGovernance"])
	assert.Equal(t, "versioning.composite.preview.execute", stored.Metadata["page_runtime_api"])
	assert.Equal(t, "player.list", stored.Metadata["binding_id"])
	assert.Equal(t, "1.0.0", stored.Metadata["snapshot_function_ver"])
}

// seedDraftApprovingContract 写入声明审批的函数契约（草稿路径无发布快照，
// 契约即唯一真相）。
func seedDraftApprovingContract(svcCtx *svc.ServiceContext, ctx context.Context) error {
	return contractsvc.NewContractService(svcCtx.DB).RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", contractsvc.FunctionMetaInput{
		ID:                "player.query",
		Version:           "1.0.0",
		Enabled:           true,
		Resource:          "player",
		Operation:         "query",
		Capability:        string(spec.CapabilityAction),
		Execution:         string(spec.FunctionExecutionSync),
		Risk:              "safe",
		Permission:        "player:query",
		InputSchema:       draftInputSchema,
		OutputSchema:      draftOutputSchema,
		ApprovalRequired:  true,
		ApprovalPolicyKey: "two_person",
	})
}
