// 补齐 console 包剩余可覆盖分支（group C）：
//  1. createPageApproval 的接收人去重循环（admin 角色用户非空时执行）。
//  2. buildBindingPayloadFromSelectors 的最终 Marshal 失败（default transform
//     兜底值不经 JSON 校验，非法值在编码边界报错）。
//  3. applyRenameTransform 的防御分支：源缺失 / 非法 JSON / 超界数值 /
//     params 目标名非字符串 / selection 数组含非对象元素。
//  4. setJSONObjectPointer 的嵌套 child Marshal 失败。
package console

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/service/notify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// approvalRecipients 返回非空（console_tester 持有 admin 角色）时，actor
// 去重循环体执行：接收人集合构建后 actor 已在集合中、不再重复追加。
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

// default transform 的兜底值来自 Params（不经 validRawJSON 校验）：非法 JSON
// 兜底值写入 payload 后在最终 json.Marshal 处失败。
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
}

// applyRenameTransform 的防御分支逐一直测。
func TestApplyRenameTransformEdgeBranches(t *testing.T) {
	renameWith := func(params map[string]json.RawMessage) *spec.TransformSpec {
		return &spec.TransformSpec{Type: spec.TransformRename, Params: params}
	}

	// 源值缺失（found=false）→ 输出缺席而非报错。
	_, found, err := applyRenameTransform(nil, false, renameWith(nil))
	require.NoError(t, err)
	assert.False(t, found)

	// 源上下文为非法 JSON → validRawJSON 错误透传。
	_, _, err = applyRenameTransform(json.RawMessage(`{bad`), true, renameWith(nil))
	require.Error(t, err)

	// 源是 json.Valid 但数值超界的 JSON（1e999）：Unmarshal 到 any 失败。
	_, _, err = applyRenameTransform(json.RawMessage(`1e999`), true, renameWith(nil))
	require.Error(t, err)

	// params 的目标名不是字符串（123）→ 该映射被跳过，输出空对象。
	val, found, err := applyRenameTransform(json.RawMessage(`{"uid":"p1"}`), true,
		renameWith(map[string]json.RawMessage{"uid": json.RawMessage(`123`)}))
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `{}`, string(val))

	// selection 数组含非对象元素 → 显式 422。
	_, _, err = applyRenameTransform(json.RawMessage(`[1,"x"]`), true,
		renameWith(map[string]json.RawMessage{"uid": json.RawMessage(`"player_id"`)}))
	require.Error(t, err)
}

// 嵌套 target 的 child map 携带非法 RawMessage 时 Marshal 失败。
func TestSetJSONObjectPointerMarshalFailure(t *testing.T) {
	err := setJSONObjectPointer(map[string]json.RawMessage{}, []string{"a", "b"}, json.RawMessage(`{invalid`))
	require.Error(t, err)
}
