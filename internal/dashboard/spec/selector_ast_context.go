package spec

import (
	"encoding/json"
	"strings"
)

// SelectorContextForBinding builds the runtime selector context for a concrete
// page binding. It must stay aligned with PageRenderer execution contexts.
func SelectorContextForBinding(page PageSpec, binding PageFunctionBinding) SelectorContext {
	// composite 页：组合区块无独立 form/row schema——输入来源受限为
	// page_state（联动键）与 literal；输出为 OutputAssignment 写 state。
	if page.Type == PageTypeComposite && page.Composite != nil {
		return SelectorContext{
			PageType:   page.Type,
			PageState:  compositePageStateSchema(page.Composite),
			FormSchema: JSONSchema(""),
		}
	}
	ctx := SelectorContext{
		PageType:      page.Type,
		HasListView:   page.Resource != nil && page.Resource.ListView != nil,
		HasDetailView: page.Resource != nil && page.Resource.DetailView != nil,
		FormSchema:    FormSchemaForBinding(page, binding),
	}
	if page.Task != nil {
		taskIDStateKey := "taskId"
		if page.Task.TaskView != nil && strings.TrimSpace(page.Task.TaskView.TaskIDStateKey) != "" {
			taskIDStateKey = strings.TrimSpace(page.Task.TaskView.TaskIDStateKey)
		}
		ctx.PageState = map[string]JSONSchema{
			taskIDStateKey: JSONSchema(`{"type":"string"}`),
			"taskStatus":   JSONSchema(`{"type":"object","properties":{}}`),
			"taskEvents":   JSONSchema(`{"type":"array","items":{"type":"object","properties":{}}}`),
			"taskResult":   JSONSchema(`{"type":"object","properties":{}}`),
		}
	}
	if page.Resource != nil && page.Resource.ListView != nil {
		ctx.RowSchema = page.Resource.ListView.RowSchema
	}
	if page.Type == PageTypeResource && binding.Usage == BindingUsageAction {
		ctx.IsRowAction = bindingUsesRowSource(binding)
		ctx.IsBatchAction = bindingUsesSelectionSource(binding)
	}
	return ctx
}

// compositePageStateSchema 汇总组合页全部区块的联动 stateKey（宽松
// object schema：键存在即可，具体形状由函数契约校验兜底）。
func compositePageStateSchema(comp *CompositePageSpec) map[string]JSONSchema {
	out := map[string]JSONSchema{}
	if comp == nil {
		return out
	}
	seen := map[string]bool{}
	add := func(key string) {
		key = strings.TrimSpace(key)
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		// composite 的 page_state 按区块整体存储、渲染层做同名字段合并
		// （静态 schema 无法描述动态合并结果的字段类型）——宽松 schema
		// 让类型校验放行；此前 {"type":"object"} 会把「整对象→string 字段」
		// 全部判为不可赋值，composite 提案发布必失败（生产实测）。
		out[key] = JSONSchema(`{}`)
	}
	for i := range comp.Sections {
		sec := &comp.Sections[i]
		for _, dep := range sec.RefreshOn {
			add(dep)
		}
		// page_state 运行时按 section key 键控（渲染层 onPageStateMerge(key)）；
		// 显式 inputAssignment 引用任一区块（含 static 常量表单——V5 表达式
		// {{filterForm.values.x}} 的编译产物）都必须是合法来源。
		add(sec.Key)
		add(sec.BindingID)
	}
	return out
}

// FormSchemaForBinding returns the PageSpec form that supplies SourceForm
// values for a binding.
func FormSchemaForBinding(page PageSpec, binding PageFunctionBinding) JSONSchema {
	switch {
	case page.Operation != nil && binding.Usage == BindingUsageAction && page.Operation.Form != nil:
		return page.Operation.Form.JSONSchema
	case page.Task != nil && binding.Usage == BindingUsageTask && page.Task.Form != nil:
		return page.Task.Form.JSONSchema
	case page.Report != nil && binding.Usage == BindingUsageReport && page.Report.QueryForm != nil:
		return page.Report.QueryForm.JSONSchema
	case page.Resource != nil && binding.ID == "create" && page.Resource.CreateForm != nil:
		return page.Resource.CreateForm.JSONSchema
	case page.Resource != nil && binding.ID == "update" && page.Resource.UpdateForm != nil:
		return page.Resource.UpdateForm.JSONSchema
	case page.Resource != nil && binding.Usage == BindingUsageQuery:
		return listQuerySchema(page.Resource)
	default:
		return nil
	}
}

// DefaultSelector creates a default selector that maps each top-level form
// field to the same function input JSON Pointer.
func DefaultSelector(schema JSONSchema) SelectorAST {
	var schemaObj map[string]json.RawMessage
	if err := json.Unmarshal(schema, &schemaObj); err != nil {
		return SelectorAST{}
	}

	properties, ok := rawObject(schemaObj["properties"])
	if !ok {
		return SelectorAST{}
	}

	keys := make([]string, 0, len(properties))
	for field := range properties {
		keys = append(keys, field)
	}
	sortStrings(keys)

	assignments := make([]InputAssignment, 0, len(keys))
	for _, field := range keys {
		pointer := "/" + escapeJSONPointerToken(field)
		assignments = append(assignments, InputAssignment{
			Target: pointer,
			Source: ValueSource{
				Kind: SourceForm,
				Path: pointer,
			},
		})
	}
	return SelectorAST{Assignments: assignments}
}
