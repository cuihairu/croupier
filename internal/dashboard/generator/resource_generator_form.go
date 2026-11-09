package generator

import (
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

func buildFormFromContract(contract *model.FunctionContract) *spec.FormPresentationSpec {
	if contract == nil || len(contract.InputSchema) == 0 {
		return nil
	}
	schema := spec.JSONSchema(contract.InputSchema)
	form := spec.DefaultFormPresentation(schema)
	// 与 operation/task/report 页同一字段派生：hints（x-ui-*）+ 类型缺省控件，
	// 保证同一函数在各页型表单表现一致。
	form.Fields = buildFormFields(schema, "zh-CN")
	return form
}

func buildUpdateFormFromContract(contract *model.FunctionContract, semantics *model.CapabilitySemantics) *spec.FormPresentationSpec {
	form := buildFormFromContract(contract)
	if form == nil || semantics == nil {
		return form
	}
	identityField := strings.TrimSpace(semantics.IdentityField)
	if identityField == "" {
		return form
	}
	form.JSONSchema = removeTopLevelSchemaField(form.JSONSchema, identityField)
	return form
}

// schemaCanBindIdentityPlusFields 校验 action 输入是否为「identity 必填字段 +
// 任意附加字段」形态：identity 可由 row 注入、附加字段留给用户表单。
// 附加字段全部可选时不构成安全风险；含其他必填字段也可（表单会收集）。
func schemaCanBindIdentityPlusFields(schema spec.JSONSchema, identityField string) bool {
	identityField = strings.TrimSpace(identityField)
	if identityField == "" {
		return false
	}
	root := parseJSONObject(json.RawMessage(schema))
	if len(root) == 0 {
		return false
	}
	properties := objectProperty(root, "properties")
	if len(properties) == 0 {
		return false
	}
	if _, ok := properties[identityField]; !ok {
		return false
	}
	// identity 必须是必填（否则无法安全从行注入）
	for _, required := range requiredFieldNames(root) {
		if required == identityField {
			return true
		}
	}
	return false
}

// buildActionFormPresentation 产出表单行操作的 Form：剥离 identity 字段
// （row 注入），剩余字段交给 SchemaFormRenderer。
func buildActionFormPresentation(contract *model.FunctionContract, semantics *model.CapabilitySemantics, action resourceActionSemantic) *spec.FormPresentationSpec {
	form := buildFormFromContract(contract)
	if form == nil {
		return nil
	}
	if semantics != nil && strings.TrimSpace(semantics.IdentityField) != "" {
		// 表单剥离行注入的 identity（与 update form 同一语义）
		form = buildUpdateFormFromContract(contract, semantics)
	}
	// action.IdentityInput 是 selector 实际映射的字段；若与 semantics
	// identity 不同，同样剥离
	target := strings.TrimSpace(action.IdentityInput)
	targetKey := topLevelPointerToken(target)
	if targetKey != "" && (semantics == nil || targetKey != strings.TrimSpace(semantics.IdentityField)) {
		form.JSONSchema = removeTopLevelSchemaField(form.JSONSchema, targetKey)
	}
	if form.Fields != nil {
		remaining := make([]spec.FormFieldSpec, 0, len(form.Fields))
		for _, f := range form.Fields {
			if f.Key == target || f.Key == targetKey {
				continue
			}
			if semantics != nil && f.Key == strings.TrimSpace(semantics.IdentityField) {
				continue
			}
			remaining = append(remaining, f)
		}
		form.Fields = remaining
	}
	return form
}

func removeTopLevelSchemaField(schema spec.JSONSchema, field string) spec.JSONSchema {
	field = strings.TrimSpace(field)
	if len(schema) == 0 || field == "" {
		return schema
	}
	root := parseJSONObject(json.RawMessage(schema))
	if len(root) == 0 {
		return schema
	}
	properties := objectProperty(root, "properties")
	if len(properties) > 0 {
		delete(properties, field)
		root["properties"] = rawObject(properties)
	}
	if rawRequired := root["required"]; len(rawRequired) > 0 {
		var required []string
		if err := json.Unmarshal(rawRequired, &required); err == nil {
			next := make([]string, 0, len(required))
			for _, item := range required {
				if item != field {
					next = append(next, item)
				}
			}
			if len(next) > 0 {
				raw, err := json.Marshal(next)
				if err == nil {
					root["required"] = raw
				}
			} else {
				delete(root, "required")
			}
		}
	}
	return spec.JSONSchema(rawObject(root))
}
