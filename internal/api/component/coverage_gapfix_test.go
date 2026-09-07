// 覆盖目标（缝隙注入类死分支，对齐 internal/policy/manager.go 的
// 函数变量缝隙惯例；默认缝隙与生产行为完全一致）：
//  1. demo_constants.go buildDemoConstantTemplate 两次 marshal 失败分支
//     （输入均为字面 string/[]string/map 结构，真实序列化恒成功）。
//  2. demo_constants.go buildDemoConstantTemplates / SeedDemoConstants 的
//     错误传播链。
//  3. generator.go GenerateSingleFunctionTemplates 的 tpl==nil 防御分支
//     （buildSingleFunctionTemplate 唯一出口为构造字面量，永不返回 nil）。
//  4. generator.go RegenerateFromContracts / handler.go Regenerate 的
//     GenerateSingleFunctionTemplates 错误传播分支（该方法恒返回 nil）。
//  5. handler.go validateTemplateParams 的规范化 marshal 失败分支
//     （params 中的 RawMessage 字段经外层 Unmarshal 校验必然合法）。
package component

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withDemoJSONMarshalError(t *testing.T, injected error) {
	t.Helper()
	orig := demoJSONMarshal
	demoJSONMarshal = func(v interface{}) ([]byte, error) { return nil, injected }
	t.Cleanup(func() { demoJSONMarshal = orig })
}

// buildDemoConstantTemplate：staticSchema marshal 失败。
func TestGapfixBuildDemoConstantTemplate_StaticSchemaMarshalError(t *testing.T) {
	injected := errors.New("injected staticSchema marshal failure")
	withDemoJSONMarshalError(t, injected)

	tpl, err := buildDemoConstantTemplate(demoConstants[0])
	require.Error(t, err)
	assert.Nil(t, tpl)
	assert.Contains(t, err.Error(), "marshal staticSchema")
	assert.True(t, errors.Is(err, injected), "error should wrap injected failure, got %v", err)
}

// buildDemoConstantTemplate：staticSchema 成功、tree marshal 失败。
func TestGapfixBuildDemoConstantTemplate_TreeMarshalError(t *testing.T) {
	injected := errors.New("injected tree marshal failure")
	orig := demoJSONMarshal
	var calls int32
	demoJSONMarshal = func(v interface{}) ([]byte, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			return orig(v)
		}
		return nil, injected
	}
	t.Cleanup(func() { demoJSONMarshal = orig })

	tpl, err := buildDemoConstantTemplate(demoConstants[0])
	require.Error(t, err)
	assert.Nil(t, tpl)
	assert.Contains(t, err.Error(), "marshal tree")
	assert.True(t, errors.Is(err, injected), "error should wrap injected failure, got %v", err)
}

// buildDemoConstantTemplates：单模板构造失败向上传播。
func TestGapfixBuildDemoConstantTemplates_PropagateError(t *testing.T) {
	injected := errors.New("injected templates marshal failure")
	withDemoJSONMarshalError(t, injected)

	templates, err := buildDemoConstantTemplates()
	require.Error(t, err)
	assert.Nil(t, templates)
	assert.True(t, errors.Is(err, injected), "error should wrap injected failure, got %v", err)
}

// SeedDemoConstants：模板构造失败 → HTTP 500 错误响应，不创建任何模板。
func TestGapfixSeedDemoConstants_TemplateBuildError(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r.Group("/api/v1/component-templates"))

	withDemoJSONMarshalError(t, errors.New("injected seed marshal failure"))

	w := doReq(r, http.MethodPost, "/api/v1/component-templates/seed-demo-constants", "")
	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "injected seed marshal failure")

	var total int64
	require.NoError(t, db.Model(&model.ComponentTemplate{}).Count(&total).Error)
	assert.Zero(t, total, "no demo template should be created when build fails")
}

// GenerateSingleFunctionTemplates：缝隙返回 nil 模板 → 跳过且不落库、不报错。
func TestGapfixGenerateSingleFunctionTemplates_NilTemplateSkipped(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)

	orig := buildSingleFunctionTemplateFn
	buildSingleFunctionTemplateFn = func(*model.FunctionContract) *model.ComponentTemplate { return nil }
	t.Cleanup(func() { buildSingleFunctionTemplateFn = orig })

	contracts := []*model.FunctionContract{
		{FunctionID: "demo.one", ResourceKey: "demo", Capability: 1},
	}
	require.NoError(t, h.GenerateSingleFunctionTemplates(context.Background(), contracts))

	var total int64
	require.NoError(t, db.Model(&model.ComponentTemplate{}).Count(&total).Error)
	assert.Zero(t, total, "nil template must be skipped without upsert")
}

// RegenerateFromContracts：单函数模板生成失败 → 包装错误返回。
func TestGapfixRegenerateFromContracts_SingleFunctionError(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)

	injected := errors.New("injected single function failure")
	orig := generateSingleFunctionTemplates
	generateSingleFunctionTemplates = func(*Handler, context.Context, []*model.FunctionContract) error {
		return injected
	}
	t.Cleanup(func() { generateSingleFunctionTemplates = orig })

	err := h.RegenerateFromContracts(context.Background(), []*model.FunctionContract{
		{FunctionID: "demo.one", ResourceKey: "demo", Capability: 1},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "single function templates")
	assert.True(t, errors.Is(err, injected), "error should wrap injected failure, got %v", err)
}

// Regenerate handler：service 返回错误 → HTTP 500 错误响应。
func TestGapfixRegenerateHandler_ServiceError(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r.Group("/api/v1/component-templates"))

	orig := generateSingleFunctionTemplates
	generateSingleFunctionTemplates = func(*Handler, context.Context, []*model.FunctionContract) error {
		return errors.New("injected regenerate failure")
	}
	t.Cleanup(func() { generateSingleFunctionTemplates = orig })

	w := doReq(r, http.MethodPost, "/api/v1/component-templates/regenerate", "")
	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "injected regenerate failure")
}

// validateTemplateParams：规范化 marshal 失败 → 返回错误。
func TestGapfixValidateTemplateParams_MarshalError(t *testing.T) {
	injected := errors.New("injected params marshal failure")
	orig := paramsJSONMarshal
	paramsJSONMarshal = func(v interface{}) ([]byte, error) { return nil, injected }
	t.Cleanup(func() { paramsJSONMarshal = orig })

	tree := json.RawMessage(`[{"id":"node1","type":"fnForm","props":{"title":"t"}}]`)
	raw := json.RawMessage(`[{"key":"k","label":"标题","nodeId":"node1","prop":"title","default":"v"}]`)
	out, err := validateTemplateParams(raw, tree)
	require.Error(t, err)
	assert.Nil(t, out)
	assert.True(t, errors.Is(err, injected), "error should be injected failure, got %v", err)
}
