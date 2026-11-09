package generator

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

func buildListViewFromContract(contract *model.FunctionContract, semantics *model.CapabilitySemantics) *spec.ListViewSpec {
	if contract == nil || len(contract.OutputSchema) == 0 {
		return defaultListView()
	}

	// 分页字段命名变体解析放在最前：filters 的排除列表、selector 重写与
	// 分页 UI 都消费 semantics 的字段名，必须先对齐 schema 实际命名
	// （SDK 常用 camelCase pageSize，而语义默认是 page/page_size）。
	if inProps := objectProperty(parseJSONObject(json.RawMessage(contract.InputSchema)), "properties"); len(inProps) > 0 {
		resolvePaginationFields(inProps, semantics)
	}

	itemsSchema := findCollectionItemsSchema(spec.JSONSchema(contract.OutputSchema), semantics)
	if len(itemsSchema) == 0 {
		return defaultListView()
	}

	properties := objectProperty(parseJSONObject(itemsSchema), "properties")
	if len(properties) == 0 {
		return defaultListView()
	}

	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// nil semantics（组合页无资源语义上下文）：字段名用通用默认（items/total）
	var identityField string
	var filters []spec.FilterSpec
	if semantics != nil {
		identityField = strings.TrimSpace(semantics.IdentityField)
		filters = buildFiltersFromContract(contract, semantics)
	}
	filterKeys := make(map[string]struct{}, len(filters))
	for _, filter := range filters {
		filterKeys[filter.Key] = struct{}{}
	}
	columns := make([]spec.ColumnSpec, 0, len(keys))
	for _, key := range keys {
		prop := properties[key]
		humanTitle := fallbackLabel(key)
		if humanTitle == "" {
			humanTitle = key
		}
		col := spec.ColumnSpec{
			Key:      key,
			Title:    spec.LocalizedText{"zh-CN": humanTitle, "en-US": humanTitle},
			DataType: inferDataType(prop),
			Visible:  true,
		}
		if key == identityField {
			col.Fixed = "left"
		}
		// D5 显式字段语义：输出侧 x-* hints 机械派生列呈现，不靠字段名猜测。
		// identity 坍缩为固定左列 + 可复制；x-enum-options 坍缩为枚举标签；
		// x-field-unit 提供单位后缀。
		if hints := parseJSONObject(prop); len(hints) > 0 {
			if label := hintLocalized(hints["x-label"]); label != nil {
				col.Title = label
			}
			if hintFieldRole(rawString(hints["x-field-role"])) == "identity" {
				col.Fixed = "left"
				col.Render = "copy"
			}
			if enumOptions := hintEnumOptions(hints["x-enum-options"]); len(enumOptions) > 0 {
				col.Enum = enumOptions
				col.Render = "tag"
			}
			if unit := hintLocalized(hints["x-field-unit"]); unit != nil {
				col.Unit = unit
			}
		}
		if _, ok := filterKeys[key]; ok {
			col.Filterable = true
		}
		columns = append(columns, col)
	}

	list := defaultListView()
	list.IdentityKey = identityField
	list.RowSchema = spec.JSONSchema(itemsSchema)
	list.Columns = columns
	list.Filters = filters
	list.Pagination = paginationFromContract(contract, semantics)
	return list
}

func buildDetailBinding(contract *model.FunctionContract, semantics *model.CapabilitySemantics) (spec.PageFunctionBinding, bool) {
	if contract == nil || semantics == nil || strings.TrimSpace(semantics.IdentityField) == "" {
		return spec.PageFunctionBinding{}, false
	}
	if !canUseItemQueryAsDetailSource(contract, semantics) {
		return spec.PageFunctionBinding{}, false
	}
	return resourceBinding(contract, "detail", spec.BindingUsageDetail, semantics), true
}

func buildInlineResourceActions(
	semantics *model.CapabilitySemantics,
	contracts []*model.FunctionContract,
	locale string,
) ([]spec.ActionSpec, []spec.ActionSpec, []spec.ActionSpec, []spec.PageFunctionBinding, []spec.Diagnostic) {
	actions := parseResourceActionSemantics(semantics)
	if len(actions) == 0 {
		return nil, nil, nil, nil, nil
	}
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].Subject != actions[j].Subject {
			return actions[i].Subject < actions[j].Subject
		}
		return actions[i].FunctionID < actions[j].FunctionID
	})
	rowActions := make([]spec.ActionSpec, 0, len(actions))
	batchActions := make([]spec.ActionSpec, 0, len(actions))
	toolbarActions := make([]spec.ActionSpec, 0, len(actions))
	bindings := make([]spec.PageFunctionBinding, 0, len(actions))
	diags := make([]spec.Diagnostic, 0)
	for _, action := range actions {
		contract := findContractByFunctionID(contracts, action.FunctionID)
		if contract == nil {
			diags = append(diags, spec.Diagnostic{
				Code:       "resource_action_contract_missing",
				Severity:   spec.SeverityWarning,
				Message:    "resource action semantic references a missing function contract",
				FunctionID: action.FunctionID,
				Field:      "resource.actions",
			})
			continue
		}
		actionSpec, binding, placement, ok, diag := buildInlineResourceAction(contract, semantics, action, locale)
		if !ok {
			if diag.Code != "" {
				diags = append(diags, diag)
			}
			continue
		}
		switch placement {
		case "row", "row-form":
			rowActions = append(rowActions, actionSpec)
		case "batch":
			batchActions = append(batchActions, actionSpec)
		case "toolbar":
			toolbarActions = append(toolbarActions, actionSpec)
		}
		bindings = append(bindings, binding)
	}
	return rowActions, batchActions, toolbarActions, bindings, diags
}

func buildInlineResourceAction(
	contract *model.FunctionContract,
	semantics *model.CapabilitySemantics,
	action resourceActionSemantic,
	locale string,
) (spec.ActionSpec, spec.PageFunctionBinding, string, bool, spec.Diagnostic) {
	if contract == nil || semantics == nil {
		return spec.ActionSpec{}, spec.PageFunctionBinding{}, "", false, spec.Diagnostic{}
	}
	selector, placement, ok := buildInlineResourceActionSelector(contract, semantics, action)
	if !ok {
		return spec.ActionSpec{}, spec.PageFunctionBinding{}, "", false, spec.Diagnostic{
			Code:       "resource_action_requires_operation_page",
			Severity:   spec.SeverityWarning,
			Message:    "resource action cannot be safely inlined into ResourcePage and must stay as a standalone operation",
			FunctionID: contract.FunctionID,
			Field:      "resource.actions",
		}
	}
	bindingID := inlineActionBindingID(contract)
	binding := spec.PageFunctionBinding{
		ID:         bindingID,
		FunctionID: strings.TrimSpace(contract.FunctionID),
		Usage:      spec.BindingUsageAction,
		Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
	}
	output := resourceOutputAssignments(contract, spec.BindingUsageAction, semantics)
	if len(selector.Assignments) > 0 || len(output) > 0 {
		binding.Selectors = &spec.BindingSelectors{
			Input:  selector,
			Output: output,
		}
	}
	actionForm := (*spec.FormPresentationSpec)(nil)
	if placement == "row-form" || placement == "batch" {
		actionForm = buildActionFormPresentation(contract, semantics, action)
	}
	return spec.ActionSpec{
		Key:          inlineActionKey(contract),
		Title:        inlineActionTitle(contract, locale),
		Type:         inlineActionType(contract),
		Confirm:      inlineActionNeedsConfirm(contract),
		ConfirmTitle: spec.LocalizedText{locale: "确认操作"},
		ConfirmDesc:  spec.LocalizedText{locale: "确定要执行此操作吗？"},
		BindingID:    bindingID,
		Form:         actionForm,
		Permission:   strings.TrimSpace(contract.Permission),
		Risk:         contract.Risk.String(),
	}, binding, placement, true, spec.Diagnostic{}
}

func buildInlineResourceActionSelector(
	contract *model.FunctionContract,
	semantics *model.CapabilitySemantics,
	action resourceActionSemantic,
) (spec.SelectorAST, string, bool) {
	switch strings.TrimSpace(action.Subject) {
	case "resource_item":
		target := topLevelPointerToken(action.IdentityInput)
		if target == "" {
			return spec.SelectorAST{}, "", false
		}
		// 纯 identity 操作：一键执行。
		if schemaCanBindBySingleRequiredField(spec.JSONSchema(contract.InputSchema), target) {
			selector := spec.DefaultSelector(spec.JSONSchema(contract.InputSchema))
			if !replaceSelectorSource(&selector, action.IdentityInput, spec.ValueSource{
				Kind: spec.SourceRow,
				Path: pointerForField(semantics.IdentityField),
			}) {
				return spec.SelectorAST{}, "", false
			}
			return selector, "row", true
		}
		// identity + 附加字段：表单行操作（identity 注入、其余弹表单）。
		if schemaCanBindIdentityPlusFields(spec.JSONSchema(contract.InputSchema), target) {
			selector := spec.DefaultSelector(spec.JSONSchema(contract.InputSchema))
			if !replaceSelectorSource(&selector, action.IdentityInput, spec.ValueSource{
				Kind: spec.SourceRow,
				Path: pointerForField(semantics.IdentityField),
			}) {
				return spec.SelectorAST{}, "", false
			}
			return selector, "row-form", true
		}
		return spec.SelectorAST{}, "", false
	case "resource_selection":
		target := topLevelPointerToken(action.IdentityInput)
		if target == "" || !schemaCanBindBySingleRequiredArrayField(spec.JSONSchema(contract.InputSchema), target) {
			return spec.SelectorAST{}, "", false
		}
		rowIdentity := pointerForField(semantics.IdentityField)
		if rowIdentity == "" {
			return spec.SelectorAST{}, "", false
		}
		selector := spec.DefaultSelector(spec.JSONSchema(contract.InputSchema))
		if !replaceSelectorSource(&selector, action.IdentityInput, spec.ValueSource{
			Kind: spec.SourceSelection,
			Path: rowIdentity,
			Transform: &spec.TransformSpec{
				Type: spec.TransformPick,
			},
		}) {
			return spec.SelectorAST{}, "", false
		}
		return selector, "batch", true
	case "none":
		if !schemaHasNoRequiredFields(spec.JSONSchema(contract.InputSchema)) {
			return spec.SelectorAST{}, "", false
		}
		return spec.DefaultSelector(spec.JSONSchema(contract.InputSchema)), "toolbar", true
	default:
		return spec.SelectorAST{}, "", false
	}
}

func buildDetailViewFromContracts(
	collectionContract *model.FunctionContract,
	itemContract *model.FunctionContract,
	semantics *model.CapabilitySemantics,
) *spec.DetailViewSpec {
	schema := detailSchemaFromContracts(collectionContract, itemContract, semantics)
	if len(schema) == 0 {
		return nil
	}
	properties := objectProperty(parseJSONObject(schema), "properties")
	if len(properties) == 0 {
		return nil
	}
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fields := make([]spec.DetailFieldSpec, 0, len(keys))
	for _, key := range keys {
		humanTitle := fallbackLabel(key)
		if humanTitle == "" {
			humanTitle = key
		}
		fields = append(fields, spec.DetailFieldSpec{
			Key:      key,
			Title:    spec.LocalizedText{"zh-CN": humanTitle, "en-US": humanTitle},
			DataType: inferDataType(properties[key]),
			Visible:  true,
		})
	}
	return &spec.DetailViewSpec{
		Fields: fields,
		Layout: "horizontal",
	}
}

func parseResourceActionSemantics(semantics *model.CapabilitySemantics) []resourceActionSemantic {
	if semantics == nil || len(semantics.Actions) == 0 {
		return nil
	}
	var items []resourceActionSemantic
	if err := json.Unmarshal(semantics.Actions, &items); err != nil {
		return nil
	}
	out := make([]resourceActionSemantic, 0, len(items))
	for _, action := range items {
		action.FunctionID = strings.TrimSpace(action.FunctionID)
		action.Subject = strings.TrimSpace(action.Subject)
		action.IdentityInput = strings.TrimSpace(action.IdentityInput)
		if action.FunctionID == "" || action.Subject == "" {
			continue
		}
		if action.Subject == "none" {
			action.IdentityInput = ""
		}
		out = append(out, action)
	}
	return out
}

func detailSchemaFromContracts(
	collectionContract *model.FunctionContract,
	itemContract *model.FunctionContract,
	semantics *model.CapabilitySemantics,
) json.RawMessage {
	if canUseItemQueryAsDetailSource(itemContract, semantics) {
		root := parseJSONObject(json.RawMessage(itemContract.OutputSchema))
		if len(objectProperty(root, "properties")) > 0 {
			return json.RawMessage(itemContract.OutputSchema)
		}
	}
	return findCollectionItemsSchema(spec.JSONSchema(collectionContract.OutputSchema), semantics)
}

func canUseItemQueryAsDetailSource(contract *model.FunctionContract, semantics *model.CapabilitySemantics) bool {
	if contract == nil || semantics == nil || len(contract.OutputSchema) == 0 {
		return false
	}
	identityField := strings.TrimSpace(semantics.IdentityField)
	if identityField == "" {
		return false
	}
	if !schemaCanBindOnlyIdentity(spec.JSONSchema(contract.InputSchema), identityField) {
		return false
	}
	root := parseJSONObject(json.RawMessage(contract.OutputSchema))
	properties := objectProperty(root, "properties")
	if len(properties) == 0 {
		return false
	}
	_, ok := properties[identityField]
	return ok
}

func schemaCanBindOnlyIdentity(schema spec.JSONSchema, identityField string) bool {
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
	required := requiredFieldNames(root)
	for _, field := range required {
		if field != identityField {
			return false
		}
	}
	return true
}
