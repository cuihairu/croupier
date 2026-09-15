package generator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
)

// 分页边界：collection_query 契约只声明 page（无 page_size 及任何变体）时，
// 页面仍必须可发布——此前 paginationFromContract 因缺 page_size 返回 nil，
// listQuerySchema 不注入 current，而 selector 已把 page 映射到 /current，
// 发布校验报「source path 不在 form schema」，页面永远不可发布。
func TestPaginationEdge_PageOnlyCollectionQuery(t *testing.T) {
	contract := &model.FunctionContract{
		Capability:   dbenum.CapabilityCollectionQuery,
		InputSchema:  model.JSON(`{"type":"object","required":["page"],"properties":{"page":{"type":"integer"}}}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"player_id":{"type":"string"}}}},"total":{"type":"integer"}}}`),
	}
	semantics := &model.CapabilitySemantics{
		IdentityField: "player_id",
		PageFieldName: "page",
	}

	// page 命中即应启用分页（page_size 是 UI 侧可选参数）
	pagination := paginationFromContract(contract, semantics)
	require.NotNil(t, pagination, "只有 page 字段也应启用分页")
	assert.True(t, pagination.Enabled)

	// selector：page → /current；page_size 语义缺失时不产生映射
	selector := applyCollectionQuerySelector(
		spec.DefaultSelector(spec.JSONSchema(contract.InputSchema)), semantics)
	pageAssigned := false
	for _, a := range selector.Assignments {
		if a.Target == "/page" {
			pageAssigned = true
			assert.Equal(t, spec.SourceForm, a.Source.Kind)
			assert.Equal(t, "/current", a.Source.Path)
		}
		assert.NotEqual(t, "/pageSize", a.Source.Path, "无 page_size 字段时不应产生 pageSize 映射")
	}
	assert.True(t, pageAssigned)

	// 端到端：listQuerySchema 注入 current 后 selector 校验通过（复现原报错链）
	list := buildListViewFromContract(contract, semantics)
	require.NotNil(t, list.Pagination)
	page := spec.PageSpec{
		Type: spec.PageTypeResource,
		Resource: &spec.ResourcePageSpec{
			ListView: list,
		},
		Bindings: []spec.PageFunctionBinding{{
			ID:         "list.main",
			FunctionID: "player.list",
			Usage:      spec.BindingUsageQuery,
			Selectors:  &spec.BindingSelectors{Input: selector},
		}},
	}
	ctx := spec.SelectorContextForBinding(page, page.Bindings[0])
	result := spec.ValidateSelector(selector, spec.JSONSchema(contract.InputSchema), ctx)
	assert.Empty(t, result.Errors, "page-only 契约的 collection selector 不应有错误级诊断")
}

// 两者都存在时保持既有行为；page/page_size 命中变体（camelCase 等）时对齐回写。
func TestPaginationEdge_VariantAlignmentStillWorks(t *testing.T) {
	contract := &model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object","required":["pageNo","limit"],"properties":{"pageNo":{"type":"integer"},"limit":{"type":"integer"}}}`),
	}
	semantics := &model.CapabilitySemantics{PageFieldName: "page", PageSizeFieldName: "page_size"}

	props := map[string]json.RawMessage{
		"pageNo": json.RawMessage(`{"type":"integer"}`),
		"limit":  json.RawMessage(`{"type":"integer"}`),
	}
	// 变体对齐是 paginationFromContract 的前置步骤（真实链路 resource_generator
	// 先 resolve 回写语义再取分页）
	assert.True(t, resolvePaginationFields(props, semantics))
	require.NotNil(t, paginationFromContract(contract, semantics))
	assert.Equal(t, "pageNo", semantics.PageFieldName)
	assert.Equal(t, "limit", semantics.PageSizeFieldName)
}
