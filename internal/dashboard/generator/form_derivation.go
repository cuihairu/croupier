package generator

// SUI-025：发布页（Go 生成器）与调试页（web schemaHints.ts）默认表单派生的
// 同源实现。预期结果共享 testdata/form_presentation_parity.json，两侧测试读取
// 同一文件断言一致（顶层字段顺序、控件、分组、layout hints）。语义对齐
// web/src/utils/schemaHints.ts 的 derivePresentationSpec / deriveLeaf：
// - 顶层字段按 JSON 声明序产出（map 解析会丢序），x-order 稳定排序、声明序
//   做 tiebreak；
// - 嵌套对象子字段/数组项字段以 JSON Pointer 标识（RFC 6901，~0/~1 转义、
//   `-` 数组段），仅在携带 hints 时产出，递归深度上限与前端一致；
// - 控件取 x-widget，缺省时按类型推导缺省控件（类型缺省不算 hint）；
// - 分组取 x-ui-groups 声明 + x-group 归组，未声明组按字段出现顺序自动补组；
// - layout 取 x-ui-layout（受控枚举，缺省 vertical）；
// - 无任何 hints 时不产出 fields/groups（无配置 = 标准纵向表单，渲染端按
//   schema 兜底合成字段并做 label 人性化，见 deriveDefaultPresentationSpec）；
//   派生不再下发 label humanize / placeholder 兜底。
// 改动任一侧须同步另一侧并更新共享 fixture。

import (
	"bytes"
	"encoding/json"
	"math"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// nestedFieldDepth 嵌套呈现字段的递归深度上限（与前端 NESTED_FIELD_DEPTH 一致）。
const nestedFieldDepth = 8

type orderedEntry struct {
	key   string
	value json.RawMessage
}

// orderedJSONObject 解析 JSON 对象并保留声明顺序。非对象输入返回 nil。
func orderedJSONObject(raw json.RawMessage) []orderedEntry {
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	token, err := dec.Token()
	if err != nil {
		return nil
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return nil
	}
	var entries []orderedEntry
	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			return entries
		}
		key, ok := keyToken.(string)
		if !ok {
			return entries
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return entries
		}
		entries = append(entries, orderedEntry{key: key, value: value})
	}
	return entries
}

// escapePointerToken JSON Pointer 段转义（~ → ~0、/ → ~1），与前端同规。
func escapePointerToken(key string) string {
	key = strings.ReplaceAll(key, "~", "~0")
	return strings.ReplaceAll(key, "/", "~1")
}

// formFieldDerived 字段派生中间态：field + 分组归属 + 是否带显式 hint +
// 声明的 x-order（含 0；nil = 未声明）。
type formFieldDerived struct {
	field    spec.FormFieldSpec
	group    string
	included bool
	order    *float64
}

// rawOrderFloat 解析 x-order 为任意有限数（与前端 asOrder 同规；非法/非有限
// 值视为未声明）。
func rawOrderFloat(raw json.RawMessage) *float64 {
	if len(raw) == 0 {
		return nil
	}
	var value float64
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return nil
	}
	return &value
}

func deriveFormLeaf(key string, prop map[string]json.RawMessage) formFieldDerived {
	derived := formFieldDerived{field: spec.FormFieldSpec{Key: key}}

	if widget := hintWidget(rawString(prop["x-widget"])); widget != "" {
		derived.field.Widget = widget
		derived.included = true
	}
	if label := hintLocalized(prop["x-label"]); label != nil {
		derived.field.Label = label
		derived.included = true
	}
	if placeholder := hintLocalized(prop["x-placeholder"]); placeholder != nil {
		derived.field.Placeholder = placeholder
		derived.included = true
	}
	if description := hintLocalized(prop["x-description"]); description != nil {
		derived.field.Description = description
		derived.included = true
	}
	if role := hintFieldRole(rawString(prop["x-field-role"])); role != "" {
		derived.field.Role = role
		derived.included = true
	}
	if unit := hintLocalized(prop["x-field-unit"]); unit != nil {
		derived.field.Unit = unit
		derived.included = true
	}
	if hintFieldPerTarget(prop["x-field-per-target"]) {
		derived.field.PerTarget = true
		derived.included = true
	}
	if width := rawInt(prop["x-width"]); width >= 1 && width <= 12 {
		derived.field.Width = width
		derived.included = true
	}
	if disabled := rawBool(prop["x-disabled"]); disabled != nil {
		derived.field.Disabled = disabled
		derived.included = true
	}
	if condition := hintCondition(prop["x-visible-when"]); condition != nil {
		derived.field.VisibleWhen = condition
		derived.included = true
	}
	if enumOptions := hintEnumOptions(prop["x-enum-options"]); len(enumOptions) > 0 {
		derived.field.EnumOptions = enumOptions
		derived.included = true
	}
	// F9：远程选项源派生（发布页与调试页一致消费；widget 名仍由 x-widget 决定）
	if remote := hintRemoteOptions(prop["x-options-source"]); remote != nil {
		derived.field.RemoteOptions = remote
		derived.included = true
	}
	if widgetProps := hintWidgetProps(prop["x-widget-props"]); len(widgetProps) > 0 {
		derived.field.WidgetProps = widgetProps
		derived.included = true
	}
	if group := strings.TrimSpace(rawString(prop["x-group"])); group != "" {
		derived.group = group
		derived.included = true
	}
	if order := rawOrderFloat(prop["x-order"]); order != nil {
		derived.order = order
		derived.included = true
		if truncated := math.Trunc(*order); truncated == *order {
			derived.field.Order = int(*order)
		}
	}

	// x-widget 缺省时按类型推导缺省控件（类型缺省不算 hint，不翻转 included）
	if derived.field.Widget == "" {
		derived.field.Widget = defaultWidgetForSchema(prop)
	}
	return derived
}

// collectFormFields 按声明序收集字段（DFS：先产出当前字段，再递归对象子字段
// 与数组项字段），与前端 collectFields 同构。顶层一律产出（保证 ui:order 完整）；
// 嵌套仅在带 hints 时产出。
func collectFormFields(entries []orderedEntry, out []formFieldDerived, prefix string, depth int) []formFieldDerived {
	if depth > nestedFieldDepth {
		return out
	}
	for _, entry := range entries {
		prop := parseJSONObject(entry.value)
		if prop == nil {
			continue
		}
		identifier := entry.key
		if prefix != "" {
			identifier = prefix + "/" + escapePointerToken(entry.key)
		}
		derived := deriveFormLeaf(identifier, prop)
		if prefix == "" || derived.included {
			out = append(out, derived)
		}
		childPrefix := "/" + escapePointerToken(entry.key)
		if prefix != "" {
			childPrefix = identifier
		}
		if childProps := orderedJSONObject(prop["properties"]); len(childProps) > 0 {
			out = collectFormFields(childProps, out, childPrefix, depth+1)
		}
		// 数组项：`-` 段指向 items 的子属性（首项语义），渲染侧同解析
		if items := objectProperty(prop, "items"); items != nil {
			if itemProps := orderedJSONObject(items["properties"]); len(itemProps) > 0 {
				out = collectFormFields(itemProps, out, childPrefix+"/-", depth+1)
			}
		}
	}
	return out
}

func formFieldOrderKey(derived formFieldDerived) float64 {
	if derived.order != nil {
		return *derived.order
	}
	return math.Inf(1)
}

// formLayoutHint 解析 x-ui-layout（受控枚举，非法值回退 vertical）。
func formLayoutHint(root map[string]json.RawMessage) spec.FormLayout {
	layout := spec.FormLayout(rawString(root["x-ui-layout"]))
	switch layout {
	case spec.FormLayoutHorizontal, spec.FormLayoutInline, spec.FormLayoutGrid:
		return layout
	}
	return spec.FormLayoutVertical
}

// deriveFormGroups 分组派生（前端 deriveGroups 同构）：声明组保序，字段
// x-group 归组（含嵌套指针字段），未声明的组按字段出现顺序自动补组（title
// 取 key 人性化，zh-CN 单 key——D7 归一）；无成员的声明组不产出。
func deriveFormGroups(root map[string]json.RawMessage, ordered []formFieldDerived) []spec.FormGroupSpec {
	var groups []spec.FormGroupSpec
	if rawGroups := root["x-ui-groups"]; len(rawGroups) > 0 {
		var items []json.RawMessage
		if err := json.Unmarshal(rawGroups, &items); err == nil {
			for _, item := range items {
				obj := parseJSONObject(item)
				if obj == nil {
					continue
				}
				key := strings.TrimSpace(rawString(obj["key"]))
				if key == "" {
					continue
				}
				group := spec.FormGroupSpec{Key: key, Fields: []string{}}
				if title := hintLocalized(obj["title"]); title != nil {
					group.Title = title
				}
				if value := rawBool(obj["collapsible"]); value != nil {
					group.Collapsible = *value
				}
				if value := rawBool(obj["collapsed"]); value != nil {
					group.Collapsed = *value
				}
				groups = append(groups, group)
			}
		}
	}

	for _, derived := range ordered {
		if derived.group == "" {
			continue
		}
		idx := -1
		for i := range groups {
			if groups[i].Key == derived.group {
				idx = i
				break
			}
		}
		if idx < 0 {
			// 未声明的分组 key：按字段出现顺序自动补组，title 取 key 人性化
			groups = append(groups, spec.FormGroupSpec{
				Key:    derived.group,
				Title:  spec.LocalizedText{"zh-CN": HumanizeKey(derived.group)},
				Fields: []string{},
			})
			idx = len(groups) - 1
		}
		groups[idx].Fields = append(groups[idx].Fields, derived.field.Key)
	}
	populated := make([]spec.FormGroupSpec, 0, len(groups))
	for _, group := range groups {
		if len(group.Fields) > 0 {
			populated = append(populated, group)
		}
	}
	return populated
}

// formPresentationDerivation 默认表单派生结果。
type formPresentationDerivation struct {
	fields   []spec.FormFieldSpec
	groups   []spec.FormGroupSpec
	layout   spec.FormLayout
	hasHints bool
}

// deriveFormPresentation 默认表单派生主入口（SUI-025）。语义与前端
// derivePresentationSpec 一致：无任何 hints 时 fields/groups 均不产出，只给
// layout（渲染端兜底合成标准纵向表单）；有任一 hint 时产出全部顶层字段
// （保 ui:order 完整）+ 带 hints 的嵌套指针字段与分组。
func deriveFormPresentation(schema spec.JSONSchema) formPresentationDerivation {
	result := formPresentationDerivation{layout: spec.FormLayoutVertical}
	root := parseJSONObject(jsonRaw(schema))
	if len(root) == 0 {
		return result
	}
	result.layout = formLayoutHint(root)
	collected := collectFormFields(orderedJSONObject(root["properties"]), nil, "", 0)
	// x-order 排序：未声明者按声明序排后（稳定排序，tiebreak 与前端一致）
	sort.SliceStable(collected, func(i, j int) bool {
		return formFieldOrderKey(collected[i]) < formFieldOrderKey(collected[j])
	})
	result.groups = deriveFormGroups(root, collected)
	for i := range collected {
		if collected[i].included {
			result.hasHints = true
			break
		}
	}
	if result.hasHints {
		result.fields = make([]spec.FormFieldSpec, 0, len(collected))
		for _, derived := range collected {
			result.fields = append(result.fields, derived.field)
		}
	}
	return result
}
