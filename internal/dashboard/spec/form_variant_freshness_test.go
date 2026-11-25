package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// freshnessFunctions 返回当前契约函数集：player.action 的 input schema
// 只含 name/count（模拟 "keyword" 已被删除/改名的契约现状）。
func freshnessFunctions() map[string]FunctionSpec {
	return map[string]FunctionSpec{
		"player.action": {
			ID:          "player.action",
			InputSchema: JSONSchema(`{"type":"object","properties":{"name":{"type":"string"},"count":{"type":"integer"}}}`),
		},
	}
}

// freshnessOperationPage 返回单 action 绑定的 operation 页：表单变体引用
// name（现行契约字段）与 keyword（已从契约删除的字段）。
func freshnessOperationPage() PageSpec {
	return PageSpec{
		Type: PageTypeOperation,
		Operation: &OperationPageSpec{
			Form: &FormPresentationSpec{
				JSONSchema: JSONSchema(`{"type":"object","properties":{"name":{"type":"string"},"keyword":{"type":"string"}}}`),
				Fields:     []FormFieldSpec{{Key: "name", Widget: FormWidgetInput}},
				Variants: []FormVariantSpec{
					{
						Key:    "compact",
						Fields: []FormVariantFieldSpec{{Key: "keyword", Widget: FormWidgetInput}},
					},
				},
			},
		},
		Bindings: []PageFunctionBinding{
			{ID: "b1", FunctionID: "player.action", Usage: BindingUsageAction},
		},
	}
}

func TestValidateVariantContractFreshnessLocatesAffectedVariant(t *testing.T) {
	diags := ValidateVariantContractFreshness(freshnessOperationPage(), freshnessFunctions())
	a := assert.New(t)
	a.Len(diags, 1)
	a.Equal("form_variant_contract_stale", diags[0].Code)
	a.Equal("operation.form.variants[0].fields[0].key", diags[0].Field)
	a.Contains(diags[0].Message, `variant "compact"`)
	a.Contains(diags[0].Message, "keyword")
	a.Contains(diags[0].Message, "player.action")
	a.Equal("player.action", diags[0].FunctionID)
}

func TestValidateVariantContractFreshnessGroupField(t *testing.T) {
	page := freshnessOperationPage()
	page.Operation.Form.Variants[0].Fields = nil
	page.Operation.Form.Variants[0].Groups = []FormGroupSpec{{Key: "g", Fields: []string{"keyword"}}}
	diags := ValidateVariantContractFreshness(page, freshnessFunctions())
	assert.Len(t, diags, 1)
	assert.Equal(t, "operation.form.variants[0].groups[0].fields[0].key", diags[0].Field)
}

func TestValidateVariantContractFreshnessCleanWhenContractMatches(t *testing.T) {
	page := freshnessOperationPage()
	page.Operation.Form.Variants[0].Fields = []FormVariantFieldSpec{{Key: "name", Widget: FormWidgetInput}}
	assert.Empty(t, ValidateVariantContractFreshness(page, freshnessFunctions()))
}

func TestValidateVariantContractFreshnessAmbiguousUsageSkipped(t *testing.T) {
	page := freshnessOperationPage()
	// 两个 action 绑定 → 结构性映射不唯一，跳过（不产生假阳性）
	page.Bindings = append(page.Bindings, PageFunctionBinding{ID: "b2", FunctionID: "player.action", Usage: BindingUsageAction})
	assert.Empty(t, ValidateVariantContractFreshness(page, freshnessFunctions()))
}

func TestValidateVariantContractFreshnessMissingFunctionOrSchemaSkipped(t *testing.T) {
	page := freshnessOperationPage()
	page.Bindings[0].FunctionID = "ghost.action"
	assert.Empty(t, ValidateVariantContractFreshness(page, freshnessFunctions()))

	noSchema := freshnessFunctions()
	fn := noSchema["player.action"]
	fn.InputSchema = nil
	noSchema["player.action"] = fn
	assert.Empty(t, ValidateVariantContractFreshness(freshnessOperationPage(), noSchema))
}

func TestValidateVariantContractFreshnessCompositeSection(t *testing.T) {
	page := PageSpec{
		Type: PageTypeComposite,
		Composite: &CompositePageSpec{
			Sections: []CompositeSection{
				{Key: "ban", BindingID: "b1", View: "form", Form: freshnessOperationPage().Operation.Form},
				{Key: "static", Static: true, View: "form", Form: freshnessOperationPage().Operation.Form},
			},
		},
		Bindings: []PageFunctionBinding{
			{ID: "b1", FunctionID: "player.action", Usage: BindingUsageAction},
		},
	}
	diags := ValidateVariantContractFreshness(page, freshnessFunctions())
	// static 区块（无 BindingID）跳过；仅绑定区块的变体被定位
	assert.Len(t, diags, 1)
	assert.Equal(t, "composite.sections[0].form.variants[0].fields[0].key", diags[0].Field)
}

func TestValidateVariantContractFreshnessResourceActionForm(t *testing.T) {
	page := PageSpec{
		Type: PageTypeResource,
		Resource: &ResourcePageSpec{
			Actions: []ActionSpec{
				{Key: "ban", BindingID: "b1", Form: freshnessOperationPage().Operation.Form},
			},
		},
		Bindings: []PageFunctionBinding{
			{ID: "b1", FunctionID: "player.action", Usage: BindingUsageAction},
		},
	}
	diags := ValidateVariantContractFreshness(page, freshnessFunctions())
	assert.Len(t, diags, 1)
	assert.Equal(t, "resource.actions[0].form.variants[0].fields[0].key", diags[0].Field)
}
