package page

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validatePublishBindingSelectors：input selector 校验错误（errors 循环）
// 传播为 binding_selector_* 诊断，fieldPath 拼接 item.Field。
func TestValidatePublishBindingSelectors_InputSelectorErrorsV10(t *testing.T) {
	page := spec.PageSpec{Type: spec.PageTypeResource}
	fn := spec.FunctionSpec{
		Enabled:     true,
		InputSchema: spec.JSONSchema(`{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}`),
	}

	// target 非 JSON Pointer → invalid_path error（带 Field）；
	// required /q 未赋值 → missing_required error。
	binding := spec.PageFunctionBinding{
		ID:         "b1",
		FunctionID: "f1",
		Usage:      spec.BindingUsageQuery,
		Selectors: &spec.BindingSelectors{
			Input: spec.SelectorAST{
				Assignments: []spec.InputAssignment{{
					Target: "not-a-pointer",
					Source: spec.ValueSource{Kind: spec.SourceLiteral},
				}},
			},
		},
		Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
	}

	diags := validatePublishBindingSelectors("b", binding, fn, page)
	assertDiagnostic(t, diags, "binding_selector_invalid_path", "b.selectors.input.not-a-pointer")
	assertDiagnostic(t, diags, "binding_selector_missing_required", "b.selectors.input./q")
}

// 空 InputSchema（requiresInputSelectors=false）时 Selectors.Input 完全不校验。
func TestValidatePublishBindingSelectors_InputSkippedWithoutSchemaV10(t *testing.T) {
	page := spec.PageSpec{Type: spec.PageTypeResource}
	fn := spec.FunctionSpec{Enabled: true, InputSchema: spec.JSONSchema(`{}`)}

	binding := spec.PageFunctionBinding{
		ID:         "b1",
		FunctionID: "f1",
		Usage:      spec.BindingUsageQuery,
		Selectors: &spec.BindingSelectors{
			Input: spec.SelectorAST{
				Assignments: []spec.InputAssignment{{
					Target: "not-a-pointer",
					Source: spec.ValueSource{Kind: spec.SourceLiteral},
				}},
			},
		},
		Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
	}

	diags := validatePublishBindingSelectors("b", binding, fn, page)
	// 空 InputSchema：input selector 完全跳过校验（无 binding_selector_* 诊断）；
	// output 缺失诊断与本测试关注点无关。
	for _, d := range diags {
		assert.NotContains(t, d.Code, "binding_selector_", "input selector must not be validated against empty schema")
	}
}

// 合法 input selector（全部 required 赋值、target 合法）无 input selector 诊断。
func TestValidatePublishBindingSelectors_InputValidV10(t *testing.T) {
	page := spec.PageSpec{Type: spec.PageTypeResource}
	fn := spec.FunctionSpec{
		Enabled:     true,
		InputSchema: spec.JSONSchema(`{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}`),
	}

	binding := spec.PageFunctionBinding{
		ID:         "b1",
		FunctionID: "f1",
		Usage:      spec.BindingUsageQuery,
		Selectors: &spec.BindingSelectors{
			Input: spec.SelectorAST{
				Assignments: []spec.InputAssignment{{
					Target: "/q",
					Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/q"},
				}},
			},
		},
		Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
	}

	diags := validatePublishBindingSelectors("b", binding, fn, page)
	for _, d := range diags {
		assert.NotContains(t, d.Code, "binding_selector_", "valid input selector must not report selector diagnostics")
	}
}

// ---
// 以下测试文档化本包中不可达的防御分支（对齐 internal/analytics/mq/deadbranch_doc_test.go 惯例）：
//
//  1. handler.go ListDrafts（:23）错误分支：PageDraftListRequest 仅含
//     ResourceKey/Status 两个 string form 字段，ShouldBindQuery 对纯 string
//     绑定恒成功，错误分支不可触发。
//  2. handler.go Versions（:216）/VersionDetail（:237）的 *PageNotFoundError
//     分支：service.Versions 忽略 FindByScopeAndPageKey 错误（p, _ :=）、
//     service.VersionDetail 仅返回 errorx.NewNotFound，二者均不产生
//     *PageNotFoundError，errors.As 分支不可触发。
//  3. service.go SaveDraft/RegenerateDraft/Publish/Rollback 的
//     CurrentUsername 错误分支（:116/:261/:394/:620）：requirePage* 先经
//     RequireAnyPermission→LoadCurrentAdmin→CurrentUsername，无 username 时
//     在权限检查处即失败，后续 CurrentUsername 调用恒成功。
//  4. service.go SetTitle/SetCategoryLabels 错误分支（:203/:206/:1222/:1225）：
//     均为 json.Marshal(map[string]string)，恒成功。
//  5. service.go Rollback 二次 ExpectedDraftRevision nil 检查（:644）：
//     :626 已拦截 nil，恒非 nil。
//  6. service.go proposalReplacementForDraft proposal==nil（:749）：
//     PageProposalModel.Find* 为 First() 风格，仅返回 (nil, err) 或
//     (非nil, nil)，永不 (nil, nil)。
//  7. service.go RebuildAllProposals（:1509）：requireScope 已保证
//     gameID/env 非空，二次空检查恒假。
//  8. service.go :982-987 input warnings 循环与 :1002-1007 output warnings
//     循环：spec.ValidateSelector / spec.ValidateOutputAssignments 仅调用
//     addError（addWarning 只在 ValidateSelectorSemantics 中使用），二者的
//     result.Warnings 恒为空。
// ---

func TestValidateSelectorAndOutputNeverWarnV10(t *testing.T) {
	schema := spec.JSONSchema(`{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}`)

	selectorResult := spec.ValidateSelector(spec.SelectorAST{
		Assignments: []spec.InputAssignment{{Target: "not-a-pointer", Source: spec.ValueSource{Kind: spec.SourceLiteral}}},
	}, schema, spec.SelectorContext{})
	require.NotEmpty(t, selectorResult.Errors)
	assert.Empty(t, selectorResult.Warnings, "ValidateSelector only reports errors; the input-warnings loop in validatePublishBindingSelectors is dead code")

	outputResult := spec.ValidateOutputAssignments([]spec.OutputAssignment{{
		StateKey: "sel",
		Source:   "not-a-pointer",
		Shape:    spec.OutputShapeScalar,
	}}, spec.JSONSchema(`{}`))
	require.NotEmpty(t, outputResult.Errors)
	assert.Empty(t, outputResult.Warnings, "ValidateOutputAssignments only reports errors; the output-warnings loop in validatePublishBindingSelectors is dead code")
}
