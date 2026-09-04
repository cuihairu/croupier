package generator

import (
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// x-ui-* 呈现 hints（docs/architecture/presentation-hints.md）→ FormFieldSpec
// 派生逻辑，与前端 web/src/utils/schemaHints.ts 的 derivePresentationSpec 对齐，
// 保证 SDK 作者声明的控件/标签在发布页与调试页表现一致。

// hintWidget 校验 x-widget 是否为受控枚举内的合法值，非法值静默忽略。
func hintWidget(raw string) spec.FormWidget {
	switch spec.FormWidget(raw) {
	case spec.FormWidgetInput,
		spec.FormWidgetTextArea,
		spec.FormWidgetNumber,
		spec.FormWidgetPassword,
		spec.FormWidgetSelect,
		spec.FormWidgetMultiSelect,
		spec.FormWidgetRadio,
		spec.FormWidgetCheckbox,
		spec.FormWidgetSwitch,
		spec.FormWidgetDatePicker,
		spec.FormWidgetTimePicker,
		spec.FormWidgetDateRange,
		spec.FormWidgetUpload,
		spec.FormWidgetImageUpload,
		spec.FormWidgetFileUpload,
		spec.FormWidgetRichText,
		spec.FormWidgetCode,
		spec.FormWidgetCascader,
		spec.FormWidgetTreeSelect,
		spec.FormWidgetColor,
		spec.FormWidgetSlider,
		spec.FormWidgetRate,
		spec.FormWidgetJSON,
		spec.FormWidgetKeyValue,
		spec.FormWidgetArray,
		spec.FormWidgetObject:
		return spec.FormWidget(raw)
	}
	return ""
}

// hintLocalized 归一 x-label/x-placeholder/x-description：裸字符串按平台契约
// 双语展开（zh-CN/en-US），对象形态兼容遗留短 key 后输出 canonical key。
func hintLocalized(raw json.RawMessage) spec.LocalizedText {
	if len(raw) == 0 {
		return nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil
		}
		return spec.LocalizedText{"zh-CN": text, "en-US": text}
	}
	pairs := parseJSONObject(raw)
	if len(pairs) == 0 {
		return nil
	}
	zh := firstLocalizedStr(pairs, "zh-CN", "zh", "zh_cn")
	en := firstLocalizedStr(pairs, "en-US", "en", "en_us")
	if zh == "" && en == "" {
		return nil
	}
	out := spec.LocalizedText{}
	if zh != "" {
		out["zh-CN"] = zh
	}
	if en != "" {
		out["en-US"] = en
	}
	return out
}

func firstLocalizedStr(pairs map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		if value := rawString(pairs[key]); value != "" {
			return value
		}
	}
	return ""
}

// hintEnumOptions 解析 x-enum-options：取值域以 schema enum 为准，hints 只补
// 标签，因此非 string value 跳过（与前端 asEnumOptions 一致）。
func hintEnumOptions(raw json.RawMessage) []spec.EnumOption {
	if len(raw) == 0 {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	options := make([]spec.EnumOption, 0, len(items))
	for _, item := range items {
		obj := parseJSONObject(item)
		if obj == nil {
			continue
		}
		value := rawString(obj["value"])
		if value == "" {
			continue
		}
		label := hintLocalized(obj["label"])
		if label == nil {
			continue
		}
		options = append(options, spec.EnumOption{Value: value, Label: label})
	}
	if len(options) == 0 {
		return nil
	}
	return options
}

func hintWidgetProps(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	props := parseJSONObject(raw)
	if len(props) == 0 {
		return nil
	}
	return props
}

// hintCondition 解析受限可见性表达式（equals/notEquals/exists/all/any），
// 仅允许表单内 JSON Pointer（/ 开头），深度上限与前端一致。
func hintCondition(raw json.RawMessage) *spec.ConditionSpec {
	return parseHintCondition(raw, 0)
}

func parseHintCondition(raw json.RawMessage, depth int) *spec.ConditionSpec {
	if len(raw) == 0 || depth > 4 {
		return nil
	}
	obj := parseJSONObject(raw)
	if obj == nil {
		return nil
	}
	switch kind := rawString(obj["kind"]); kind {
	case "equals", "notEquals":
		path := rawString(obj["path"])
		if !strings.HasPrefix(path, "/") {
			return nil
		}
		if _, ok := obj["value"]; !ok {
			return nil
		}
		return &spec.ConditionSpec{Kind: kind, Path: path, Value: obj["value"]}
	case "exists":
		path := rawString(obj["path"])
		if !strings.HasPrefix(path, "/") {
			return nil
		}
		return &spec.ConditionSpec{Kind: kind, Path: path}
	case "all", "any":
		if len(obj["conditions"]) == 0 {
			return nil
		}
		var items []json.RawMessage
		if err := json.Unmarshal(obj["conditions"], &items); err != nil {
			return nil
		}
		conditions := make([]spec.ConditionSpec, 0, len(items))
		for _, item := range items {
			if child := parseHintCondition(item, depth+1); child != nil {
				conditions = append(conditions, *child)
			}
		}
		if len(conditions) == 0 {
			return nil
		}
		return &spec.ConditionSpec{Kind: kind, Conditions: conditions}
	}
	return nil
}

// hasEnumValues 判断 schema 节点是否声明非空 enum 数组。
// 注意 enum 是数组而非对象，不能用 parseJSONObject 解析。
func hasEnumValues(prop map[string]json.RawMessage) bool {
	raw := prop["enum"]
	if len(raw) == 0 {
		return false
	}
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return false
	}
	return len(values) > 0
}

// defaultWidgetForSchema 按 schema 类型推导缺省控件（x-widget 缺省时），
// 补齐 date/time/array(enum) 分支；无匹配返回空（交给渲染端默认）。
func defaultWidgetForSchema(prop map[string]json.RawMessage) spec.FormWidget {
	switch schemaTypeFromObject(prop) {
	case "integer", "number":
		return spec.FormWidgetNumber
	case "boolean":
		return spec.FormWidgetSwitch
	case "string":
		format := rawString(prop["format"])
		switch format {
		case "date", "date-time":
			return spec.FormWidgetDatePicker
		case "time":
			return spec.FormWidgetTimePicker
		}
		if format == "textarea" || rawInt(prop["maxLength"]) > 120 {
			return spec.FormWidgetTextArea
		}
	case "array":
		items := objectProperty(prop, "items")
		if hasEnumValues(items) {
			return spec.FormWidgetMultiSelect
		}
	}
	if hasEnumValues(prop) {
		return spec.FormWidgetSelect
	}
	return ""
}

func rawBool(raw json.RawMessage) *bool {
	if len(raw) == 0 {
		return nil
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return &value
}
