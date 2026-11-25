// Coverage target: ExecuteDraftCompositeBinding (POST /api/v1/versioning/pages/composite/
// preview/execute, the draft preview execution entry point) -- the server
// reconstructs the draft spec via the generation chain and runs publish-grade
// validation, then applies the execution gate shared with published pages; the
// governance/runtime API is marked as draft, and the artifacts are not landed in
// the database. Also the gates of executeBindingFromSnapshot (unknown binding,
// contract permission, approval gate).
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

// draftTableRequest builds a draft request for a single table section
// (bindingID is allocated by the generation chain from the section key).
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

	// The execution chain marks draft governance and the draft runtime API, so the
	// audit trail can tell published pages apart from preview executions.
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

	// Draft execution is a read-only path: no proposal writes, no published snapshot writes.
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
	// Only the runtime console permission is held, and the player:query
	// permission declared by the contract is missing -> rejected with the same rule
	// as published pages.
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

// Publish-grade validation failure (a selector pointing at a field that does not
// exist in the contract) is rejected with 422 before execution, rather than
// letting payload evaluation fail downstream into an unlocatable error.
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

// The contract declares approval -> draft execution goes through the approval gate
// too: no direct dispatch, but an approval ticket is landed with draft governance.
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

// seedDraftApprovingContract writes a contract that declares approval (the draft path
// has no published snapshot, so the contract is the single source of truth).
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
