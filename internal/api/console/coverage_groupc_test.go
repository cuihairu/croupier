// Fill the remaining coverable branches of the console package (group C):
//  1. The recipient de-duplication loop of createPageApproval (runs when the
//     admin-role user set is non-empty).
//  2. An invalid-JSON fallback value of the default transform in
//     buildBindingPayloadFromSelectors (previously it deferred the failure to the
//     final Marshal call; now resolveSelectorValue intercepts it as 422).
//  3. Defensive branches of applyRenameTransform: missing source / invalid JSON
//     / out-of-range number / non-string params target name / a selection array
//     holding non-object elements.
//  4. A nested child Marshal failure of setJSONObjectPointer.
package console

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/service/notify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// When approvalRecipients is non-empty (console_tester holds the admin role),
// the actor de-duplication loop body runs: after the recipient set is built the
// actor is already in it and is not appended again.
func TestCreatePageApprovalWithAdminRoleRecipients(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")
	service.svcCtx.ApprovalsStore = approvals.NewMemStore()
	service.svcCtx.NotifyService = notify.New(nil, nil)

	db := service.svcCtx.DB
	adminRole := model.Role{Name: "admin"}
	require.NoError(t, db.Create(&adminRole).Error)
	var tester model.Admin
	require.NoError(t, db.Where("username = ?", "console_tester").First(&tester).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: tester.ID, RoleID: adminRole.ID}).Error)

	approvalID, err := service.createPageApproval(ctx, "demo-game", "development",
		spec.PageFunctionBinding{ID: "b1", FunctionID: "player.query"},
		spec.BindingContractSnapshot{BindingID: "b1", FunctionID: "player.query"},
		json.RawMessage(`{"keyword":"a"}`), "", map[string]string{"traceId": "t-1"})
	require.NoError(t, err)
	assert.NotEmpty(t, approvalID)
}

// Invalid JSON in the default transform fallback value: it used to bypass
// validation and land in the payload, only surfacing as a 500 at the final
// json.Marshal call; resolveSelectorValue now intercepts it as a semantic 422.
func TestBuildBindingPayloadFromSelectorsMarshalFailure(t *testing.T) {
	binding := spec.PageFunctionBinding{
		ID: "b1",
		Selectors: &spec.BindingSelectors{
			Input: spec.SelectorAST{Assignments: []spec.InputAssignment{{
				Target: "/kw",
				Source: spec.ValueSource{
					Kind: spec.SourcePageState,
					Key:  "missing",
					Transform: &spec.TransformSpec{
						Type:   spec.TransformDefault,
						Params: map[string]json.RawMessage{"value": json.RawMessage(`{invalid`)},
					},
				},
			}}},
		},
	}

	_, err := buildBindingPayloadFromSelectors(binding, ConsoleBindingExecutionContext{})
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok, "必须是语义化 ValidationError 而非 Marshal 内部错误")
	assert.Equal(t, 422, codeErr.Code)
	assert.Contains(t, err.Error(), "not valid JSON")
}

// The defensive branches of applyRenameTransform are each tested directly.
func TestApplyRenameTransformEdgeBranches(t *testing.T) {
	renameWith := func(params map[string]json.RawMessage) *spec.TransformSpec {
		return &spec.TransformSpec{Type: spec.TransformRename, Params: params}
	}

	// The source value is missing (found=false) -> the output is absent rather than
	// an error.
	_, found, err := applyRenameTransform(nil, false, renameWith(nil))
	require.NoError(t, err)
	assert.False(t, found)

	// The source context is invalid JSON -> the validRawJSON error propagates.
	_, _, err = applyRenameTransform(json.RawMessage(`{bad`), true, renameWith(nil))
	require.Error(t, err)

	// The source is json.Valid but out-of-range numerically (1e999): Unmarshal into
	// any fails.
	_, _, err = applyRenameTransform(json.RawMessage(`1e999`), true, renameWith(nil))
	require.Error(t, err)

	// The params target name is not a string (123) -> the mapping is skipped and the
	// output is an empty object.
	val, found, err := applyRenameTransform(json.RawMessage(`{"uid":"p1"}`), true,
		renameWith(map[string]json.RawMessage{"uid": json.RawMessage(`123`)}))
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `{}`, string(val))

	// A selection array holding non-object elements -> an explicit 422.
	_, _, err = applyRenameTransform(json.RawMessage(`[1,"x"]`), true,
		renameWith(map[string]json.RawMessage{"uid": json.RawMessage(`"player_id"`)}))
	require.Error(t, err)
}

// Marshaling fails when the child map of a nested target carries an invalid RawMessage.
func TestSetJSONObjectPointerMarshalFailure(t *testing.T) {
	err := setJSONObjectPointer(map[string]json.RawMessage{}, []string{"a", "b"}, json.RawMessage(`{invalid`))
	require.Error(t, err)
}
