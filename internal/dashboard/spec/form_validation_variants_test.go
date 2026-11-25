package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// variantValidationForm 返回基础呈现 + 两个命名变体的最小合法表单：
// standard（继承基础）/ compact（grid 布局 + 字段覆盖）。
func variantValidationForm() *FormPresentationSpec {
	return &FormPresentationSpec{
		JSONSchema: JSONSchema(formValidationSchema),
		Layout:     FormLayoutVertical,
		Fields: []FormFieldSpec{
			{Key: "name", Widget: FormWidgetInput},
			{Key: "count", Widget: FormWidgetNumber},
		},
		Variants: []FormVariantSpec{
			{Key: "standard", Title: LocalizedText{"zh-CN": "标准"}},
			{
				Key:    "compact",
				Title:  LocalizedText{"zh-CN": "紧凑"},
				Layout: FormLayoutGrid,
				Fields: []FormVariantFieldSpec{
					{Key: "name", Widget: FormWidgetInput, Width: 6},
					{Key: "count", Widget: FormWidgetNumber, Width: 6},
				},
			},
		},
	}
}

func variantCodes(t *testing.T, form *FormPresentationSpec) []string {
	t.Helper()
	codes := make([]string, 0, 4)
	for _, diag := range ValidateFormPresentations(formValidationPage(form)) {
		codes = append(codes, diag.Code)
	}
	return codes
}

func TestValidateFormVariantsValidFixture(t *testing.T) {
	assert.Empty(t, variantCodes(t, variantValidationForm()))
}

func TestValidateFormVariantsKeyRules(t *testing.T) {
	form := variantValidationForm()
	form.Variants[1].Key = "standard" // 与 variants[0] 重复
	codes := variantCodes(t, form)
	assert.Contains(t, codes, "form_variant_key_conflict")
	for _, diag := range ValidateFormPresentations(formValidationPage(form)) {
		if diag.Code == "form_variant_key_conflict" {
			assert.Equal(t, "operation.form.variants[1].key", diag.Field)
		}
	}

	empty := variantValidationForm()
	empty.Variants[0].Key = "  "
	codes = variantCodes(t, empty)
	assert.Contains(t, codes, "form_variant_key_missing")
}

func TestValidateFormVariantsDefaultUnknown(t *testing.T) {
	form := variantValidationForm()
	form.DefaultVariant = "dense" // 未声明的变体 key
	codes := variantCodes(t, form)
	assert.Contains(t, codes, "form_variant_default_unknown")
	for _, diag := range ValidateFormPresentations(formValidationPage(form)) {
		if diag.Code == "form_variant_default_unknown" {
			assert.Equal(t, "operation.form.defaultVariant", diag.Field)
		}
	}

	noVariants := &FormPresentationSpec{
		JSONSchema:     JSONSchema(formValidationSchema),
		DefaultVariant: "compact", // 无任何变体
	}
	assert.Contains(t, variantCodes(t, noVariants), "form_variant_default_unknown")

	// 合法 default：不产生诊断
	valid := variantValidationForm()
	valid.DefaultVariant = "compact"
	assert.Empty(t, variantCodes(t, valid))
}

func TestValidateFormVariantsFieldRules(t *testing.T) {
	// 字段键无法在 jsonSchema 解析（契约字段删除/改名后变体未同步）
	stale := variantValidationForm()
	stale.Variants[1].Fields = []FormVariantFieldSpec{{Key: "removed", Widget: FormWidgetInput}}
	codes := variantCodes(t, stale)
	assert.Contains(t, codes, "form_variant_field_unknown")
	for _, diag := range ValidateFormPresentations(formValidationPage(stale)) {
		if diag.Code == "form_variant_field_unknown" {
			assert.Equal(t, "operation.form.variants[1].fields[0].key", diag.Field)
		}
	}

	missing := variantValidationForm()
	missing.Variants[1].Fields = []FormVariantFieldSpec{{Key: " ", Widget: FormWidgetInput}}
	assert.Contains(t, variantCodes(t, missing), "form_variant_field_key_missing")

	dup := variantValidationForm()
	dup.Variants[1].Fields = []FormVariantFieldSpec{
		{Key: "name", Widget: FormWidgetInput},
		{Key: "name", Widget: FormWidgetInput},
	}
	assert.Contains(t, variantCodes(t, dup), "form_variant_field_duplicate")
}

func TestValidateFormVariantsIllegalOverride(t *testing.T) {
	// UnknownKeys 由 FormVariantFieldSpec.UnmarshalJSON 填充；此处直接
	// 构造以验证诊断规则本身。
	form := variantValidationForm()
	form.Variants[1].Fields[0].UnknownKeys = []string{"required", "visible"}
	codes := variantCodes(t, form)
	assert.Contains(t, codes, "form_variant_illegal_override")
	for _, diag := range ValidateFormPresentations(formValidationPage(form)) {
		if diag.Code == "form_variant_illegal_override" {
			assert.Equal(t, "operation.form.variants[1].fields[0].key", diag.Field)
			assert.Contains(t, diag.Message, "required")
			assert.Contains(t, diag.Message, "visible")
		}
	}
}

func TestValidateFormVariantsWidgetAndWidth(t *testing.T) {
	badWidget := variantValidationForm()
	badWidget.Variants[1].Fields[0].Widget = "nonsense"
	assert.Contains(t, variantCodes(t, badWidget), "form_widget_invalid")

	badType := variantValidationForm()
	badType.Variants[1].Fields[0].Widget = FormWidgetNumber // name 是 string
	assert.Contains(t, variantCodes(t, badType), "form_widget_incompatible")

	badWidth := variantValidationForm()
	badWidth.Variants[1].Fields[0].Width = 13
	assert.Contains(t, variantCodes(t, badWidth), "form_width_invalid")
}

func TestValidateFormVariantsGroupRules(t *testing.T) {
	dupGroup := variantValidationForm()
	dupGroup.Variants[1].Groups = []FormGroupSpec{
		{Key: "base", Fields: []string{"name"}},
		{Key: "base", Fields: []string{"count"}}, // 重复组 key
	}
	assert.Contains(t, variantCodes(t, dupGroup), "form_group_key_duplicate")

	dupField := variantValidationForm()
	dupField.Variants[1].Groups = []FormGroupSpec{
		{Key: "base", Fields: []string{"name", "name"}}, // 组内字段重复声明
	}
	assert.Contains(t, variantCodes(t, dupField), "form_group_field_duplicate")

	conflict := variantValidationForm()
	conflict.Variants[1].Groups = []FormGroupSpec{
		{Key: "g1", Fields: []string{"name"}},
		{Key: "g2", Fields: []string{"name"}}, // 同一字段跨组
	}
	assert.Contains(t, variantCodes(t, conflict), "form_group_field_conflict")

	undeclared := variantValidationForm()
	undeclared.Variants[1].Groups = []FormGroupSpec{
		{Key: "g", Fields: []string{"flag"}}, // 不在基础 fields，也不在本变体 fields
	}
	for _, diag := range ValidateFormPresentations(formValidationPage(undeclared)) {
		if diag.Code == "form_group_field_unknown" {
			assert.Contains(t, diag.Message, "not declared in form.fields or variant fields")
		}
	}
	assert.Contains(t, variantCodes(t, undeclared), "form_group_field_unknown")

	// 变体 fields 声明过的 key 允许进变体分组
	variantDeclared := variantValidationForm()
	variantDeclared.Variants[1].Fields = append(variantDeclared.Variants[1].Fields, FormVariantFieldSpec{Key: "flag", Widget: FormWidgetSwitch})
	variantDeclared.Variants[1].Groups = []FormGroupSpec{{Key: "g", Fields: []string{"name", "flag"}}}
	assert.Empty(t, variantCodes(t, variantDeclared))
}
