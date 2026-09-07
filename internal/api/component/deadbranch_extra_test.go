package component

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 以下测试文档化本包中不可达的防御分支（对齐 internal/analytics/mq/deadbranch_doc_test.go 惯例）：
//
//  1. demo_constants.go buildDemoConstantTemplate（:69/:81）：两次
//     json.Marshal 的输入均为 string/[]string/map[string]interface{} 组成的
//     字面结构，序列化恒成功，错误分支不可触发。
//  2. demo_constants.go buildDemoConstantTemplates（:107）/SeedDemoConstants
//     （:119）：错误传播链依赖上述恒不发生的 marshal 错误，同样不可触发。
//  3. generator.go GenerateSingleFunctionTemplates（:28）：
//     buildSingleFunctionTemplate 唯一出口为构造字面量并返回（:65），
//     永不返回 nil，tpl==nil 分支不可触发。
//  4. generator.go RegenerateFromContracts（:221）：GenerateSingleFunctionTemplates
//     唯一返回语句为 `return nil`，错误传播分支不可触发；因此 handler.go
//     Regenerate（:422）的 service 错误分支亦不可触发。
//  5. handler.go validateTemplateParams（:131）：params 经
//     json.Unmarshal 得到，[]TemplateParam 中的 RawMessage 字段必然是
//     合法 JSON 子文档，再 Marshal 恒成功，错误分支不可触发。

func TestBuildDemoConstantTemplatesNeverFailsV11(t *testing.T) {
	templates, err := buildDemoConstantTemplates()
	require.NoError(t, err)
	require.Len(t, templates, len(demoConstants))
	for _, tpl := range templates {
		require.NotNil(t, tpl)
		assert.True(t, json.Valid(tpl.Name), "template %s name must be valid JSON", tpl.Key)
		assert.True(t, json.Valid(tpl.Tree), "template %s tree must be valid JSON", tpl.Key)
	}
}

func TestBuildSingleFunctionTemplateNeverNilV11(t *testing.T) {
	for _, contract := range []*model.FunctionContract{
		{FunctionID: "res.list", Capability: 1},
		{FunctionID: "res.get", Capability: 2},
		{FunctionID: "res.save", Capability: 3},
		{},
	} {
		tpl := buildSingleFunctionTemplate(contract)
		require.NotNil(t, tpl, "buildSingleFunctionTemplate has a single literal-return exit; nil is impossible")
	}
}

func TestRegenerateFromContractsNeverErrorsV11(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), model.NewFunctionContractModel(db))

	require.NoError(t, h.GenerateSingleFunctionTemplates(context.Background(), []*model.FunctionContract{
		{FunctionID: "demo.one", ResourceKey: "demo", Capability: 1},
		nil,
	}))
	require.NoError(t, h.RegenerateFromContracts(context.Background(), []*model.FunctionContract{
		{FunctionID: "demo.two", ResourceKey: "demo", Capability: 2},
	}))
}

func TestValidateTemplateParamsMarshalNeverFailsV11(t *testing.T) {
	tree := json.RawMessage(`[{"id":"node1","type":"fnForm","props":{"title":"t"}}]`)
	out, err := validateTemplateParams(json.RawMessage(`[{"key":"k","label":"标题","nodeId":"node1","prop":"title","default":"v"}]`), tree)
	require.NoError(t, err)
	require.True(t, json.Valid(out))
}
