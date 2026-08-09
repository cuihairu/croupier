package generator

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/datatypes"
)

type resourceActionSemantic struct {
	FunctionID    string
	Subject       string
	IdentityInput string
}

// GenerateResourcePageProposal generates a ResourcePage proposal from
// persistent CapabilitySemantics and FunctionContracts. This path is not driven
// by SDK registration UI metadata.
func GenerateResourcePageProposal(
	semantics *model.CapabilitySemantics,
	contracts []*model.FunctionContract,
	opts GenerateOptions,
) (spec.GeneratedPageSpec, bool) {
	opts = normalizeOptions(opts)
	if semantics == nil || strings.TrimSpace(semantics.ResourceKey) == "" {
		return spec.GeneratedPageSpec{}, false
	}
	if semantics.CollectionQueryID == 0 || strings.TrimSpace(semantics.IdentityField) == "" {
		return spec.GeneratedPageSpec{}, false
	}

	collectionContract := findResourceContract(contracts, semantics.CollectionQueryID, spec.CapabilityCollectionQuery)
	if collectionContract == nil {
		return spec.GeneratedPageSpec{}, false
	}

	createContract := findResourceContract(contracts, semantics.CreateID, spec.CapabilityCreate)
	updateContract := findResourceContract(contracts, semantics.UpdateID, spec.CapabilityUpdate)
	deleteContract := findResourceContract(contracts, semantics.DeleteID, spec.CapabilityDelete)
	itemContract := findResourceContract(contracts, semantics.ItemQueryID, spec.CapabilityItemQuery)

	resourceKey := strings.TrimSpace(semantics.ResourceKey)
	pageKey := "resource--" + sanitizeSourceKey(resourceKey)
	locale := opts.DefaultLocale
	title := spec.LocalizedText{locale: humanizeKey(resourceKey)}
	listView := buildListViewFromContract(collectionContract, semantics)
	detailView := buildDetailViewFromContracts(collectionContract, itemContract, semantics)

	bindings := []spec.PageFunctionBinding{
		resourceBinding(collectionContract, "list", spec.BindingUsageQuery, semantics),
	}
	if detailBinding, ok := buildDetailBinding(itemContract, semantics); ok {
		bindings = append(bindings, detailBinding)
	}
	if createContract != nil {
		bindings = append(bindings, resourceBinding(createContract, "create", spec.BindingUsageAction, nil))
	}
	rowActions := []spec.ActionSpec{}
	if updateContract != nil {
		bindings = append(bindings, resourceBinding(updateContract, "update", spec.BindingUsageAction, semantics))
		rowActions = append(rowActions, spec.ActionSpec{
			Key:       "edit",
			Title:     spec.LocalizedText{locale: "编辑"},
			Type:      "link",
			BindingID: "update",
		})
	}
	if deleteContract != nil {
		bindings = append(bindings, resourceBinding(deleteContract, "delete", spec.BindingUsageAction, semantics))
	}
	inlineRowActions, inlineBatchActions, inlineToolbarActions, inlineBindings, inlineDiags := buildInlineResourceActions(semantics, contracts, locale)
	if listView != nil {
		listView.RowActions = append(rowActions, inlineRowActions...)
		listView.BatchActions = inlineBatchActions
		listView.ToolbarActions = inlineToolbarActions
	}

	diags := assessResourceSemantics(semantics)
	for _, contract := range contracts {
		if contract == nil {
			continue
		}
		diags = append(diags, schemaSubsetDiagnostics(contract.FunctionID, "inputSchema", spec.JSONSchema(contract.InputSchema))...)
		diags = append(diags, schemaSubsetDiagnostics(contract.FunctionID, "outputSchema", spec.JSONSchema(contract.OutputSchema))...)
	}
	diags = append(diags, inlineDiags...)
	diags = append(diags, validateGeneratedResourceViews(listView, detailView, semantics)...)
	bindings = append(bindings, inlineBindings...)
	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeResource,
			ResourceKey: resourceKey,
			Title:       title,
			Category:    categoryForResource(resourceKey, locale),
			Navigation: &spec.NavigationSpec{
				Title: title,
			},
			Resource: &spec.ResourcePageSpec{
				ListView:     listView,
				DetailView:   detailView,
				CreateForm:   buildFormFromContract(createContract),
				UpdateForm:   buildUpdateFormFromContract(updateContract, semantics),
				DeleteAction: buildDeleteAction(deleteContract, locale),
			},
			Bindings: bindings,
		},
		Quality:     resourceQuality(semantics, diags),
		Diagnostics: diags,
	}, true
}

func resourceBinding(contract *model.FunctionContract, suffix string, usage spec.PageBindingUsage, semantics *model.CapabilitySemantics) spec.PageFunctionBinding {
	binding := spec.PageFunctionBinding{
		ID:         suffix,
		FunctionID: strings.TrimSpace(contract.FunctionID),
		Usage:      usage,
		Execution: spec.PageBindingExecution{
			Mode:           spec.PageExecutionModeSync,
			RequireConfirm: resourceOperationRequiresConfirmation(contract),
		},
	}
	selectors := &spec.BindingSelectors{}
	if len(contract.InputSchema) > 0 {
		selectors.Input = resourceInputSelector(spec.JSONSchema(contract.InputSchema), contract, semantics)
	}
	selectors.Output = resourceOutputAssignments(contract, usage, semantics)
	if len(selectors.Input.Assignments) == 0 && len(selectors.Output) == 0 {
		return binding
	}
	binding.Selectors = selectors
	return binding
}

func resourceOutputAssignments(contract *model.FunctionContract, usage spec.PageBindingUsage, semantics *model.CapabilitySemantics) []spec.OutputAssignment {
	if contract == nil || len(contract.OutputSchema) == 0 {
		return nil
	}
	if usage == spec.BindingUsageDetail {
		return []spec.OutputAssignment{{
			StateKey: "detail",
			Source:   "",
			Shape:    spec.OutputShapeObject,
		}}
	}
	if usage != spec.BindingUsageQuery {
		return []spec.OutputAssignment{{
			StateKey: "result",
			Source:   "",
			Shape:    outputShapeForSchema(spec.JSONSchema(contract.OutputSchema)),
		}}
	}
	arrayKeys := collectionArrayKeys(semantics)
	assignments := collectionOutputAssignments(spec.JSONSchema(contract.OutputSchema), arrayKeys, "items")
	if len(assignments) == 0 {
		return nil
	}
	if semantics != nil && strings.TrimSpace(semantics.TotalFieldName) != "" {
		totalSource := scalarPropertySource(spec.JSONSchema(contract.OutputSchema), []string{strings.TrimSpace(semantics.TotalFieldName)})
		if totalSource == "" {
			return assignments
		}
		for i := range assignments {
			if assignments[i].StateKey == "total" {
				assignments[i].Source = totalSource
				return assignments
			}
		}
		assignments = append(assignments, spec.OutputAssignment{
			StateKey: "total",
			Source:   totalSource,
			Shape:    spec.OutputShapeScalar,
		})
	}
	return assignments
}

func resourceInputSelector(inputSchema spec.JSONSchema, contract *model.FunctionContract, semantics *model.CapabilitySemantics) spec.SelectorAST {
	selector := spec.DefaultSelector(inputSchema)
	if contract == nil || semantics == nil {
		return selector
	}
	if spec.CapabilityKind(contract.Capability) == spec.CapabilityItemQuery {
		return applyIdentityRowSelector(selector, semantics)
	}
	if spec.CapabilityKind(contract.Capability) != spec.CapabilityUpdate && spec.CapabilityKind(contract.Capability) != spec.CapabilityDelete {
		if spec.CapabilityKind(contract.Capability) == spec.CapabilityCollectionQuery {
			return applyCollectionQuerySelector(selector, semantics)
		}
		return selector
	}
	return applyIdentityRowSelector(selector, semantics)
}

func applyCollectionQuerySelector(selector spec.SelectorAST, semantics *model.CapabilitySemantics) spec.SelectorAST {
	if semantics == nil {
		return selector
	}
	pageField := strings.TrimSpace(semantics.PageFieldName)
	pageSizeField := strings.TrimSpace(semantics.PageSizeFieldName)
	for i := range selector.Assignments {
		switch selector.Assignments[i].Target {
		case pointerForField(pageField):
			selector.Assignments[i].Source = spec.ValueSource{Kind: spec.SourceForm, Path: "/current"}
		case pointerForField(pageSizeField):
			selector.Assignments[i].Source = spec.ValueSource{Kind: spec.SourceForm, Path: "/pageSize"}
		}
	}
	return selector
}

func applyIdentityRowSelector(selector spec.SelectorAST, semantics *model.CapabilitySemantics) spec.SelectorAST {
	if semantics == nil {
		return selector
	}
	identityField := strings.TrimSpace(semantics.IdentityField)
	if identityField == "" {
		return selector
	}
	identityTarget := "/" + escapeJSONPointerToken(identityField)
	for i := range selector.Assignments {
		if selector.Assignments[i].Target == identityTarget {
			selector.Assignments[i].Source = spec.ValueSource{
				Kind: spec.SourceRow,
				Path: identityTarget,
			}
		}
	}
	return selector
}

func pointerForField(field string) string {
	field = strings.TrimSpace(field)
	if field == "" {
		return ""
	}
	return "/" + escapeJSONPointerToken(field)
}

func escapeJSONPointerToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(token, "/", "~1")
}

func buildFormFromContract(contract *model.FunctionContract) *spec.FormPresentationSpec {
	if contract == nil || len(contract.InputSchema) == 0 {
		return nil
	}
	return spec.DefaultFormPresentation(spec.JSONSchema(contract.InputSchema))
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

func buildListViewFromContract(contract *model.FunctionContract, semantics *model.CapabilitySemantics) *spec.ListViewSpec {
	if contract == nil || len(contract.OutputSchema) == 0 {
		return defaultListView()
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

	identityField := strings.TrimSpace(semantics.IdentityField)
	filters := buildFiltersFromContract(contract, semantics)
	filterKeys := make(map[string]struct{}, len(filters))
	for _, filter := range filters {
		filterKeys[filter.Key] = struct{}{}
	}
	columns := make([]spec.ColumnSpec, 0, len(keys))
	for _, key := range keys {
		prop := properties[key]
		col := spec.ColumnSpec{
			Key:      key,
			Title:    spec.LocalizedText{"zh-CN": key},
			DataType: inferDataType(prop),
			Visible:  true,
		}
		if key == identityField {
			col.Fixed = "left"
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
		case "row":
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
	return spec.ActionSpec{
		Key:          inlineActionKey(contract),
		Title:        inlineActionTitle(contract, locale),
		Type:         inlineActionType(contract),
		Confirm:      inlineActionNeedsConfirm(contract),
		ConfirmTitle: spec.LocalizedText{locale: "确认操作"},
		ConfirmDesc:  spec.LocalizedText{locale: "确定要执行此操作吗？"},
		BindingID:    bindingID,
		Permission:   strings.TrimSpace(contract.Permission),
		Risk:         strings.TrimSpace(contract.Risk),
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
		if target == "" || !schemaCanBindBySingleRequiredField(spec.JSONSchema(contract.InputSchema), target) {
			return spec.SelectorAST{}, "", false
		}
		selector := spec.DefaultSelector(spec.JSONSchema(contract.InputSchema))
		if !replaceSelectorSource(&selector, action.IdentityInput, spec.ValueSource{
			Kind: spec.SourceRow,
			Path: pointerForField(semantics.IdentityField),
		}) {
			return spec.SelectorAST{}, "", false
		}
		return selector, "row", true
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
		fields = append(fields, spec.DetailFieldSpec{
			Key:      key,
			Title:    spec.LocalizedText{"zh-CN": key},
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

func requiredFieldNames(root map[string]json.RawMessage) []string {
	if len(root) == 0 || len(root["required"]) == 0 {
		return nil
	}
	var required []string
	if err := json.Unmarshal(root["required"], &required); err != nil {
		return nil
	}
	return required
}

func schemaHasNoRequiredFields(schema spec.JSONSchema) bool {
	root := parseJSONObject(json.RawMessage(schema))
	return len(requiredFieldNames(root)) == 0
}

func schemaCanBindBySingleRequiredField(schema spec.JSONSchema, field string) bool {
	field = strings.TrimSpace(field)
	if field == "" {
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
	if _, ok := properties[field]; !ok {
		return false
	}
	for _, required := range requiredFieldNames(root) {
		if required != field {
			return false
		}
	}
	return true
}

func schemaCanBindBySingleRequiredArrayField(schema spec.JSONSchema, field string) bool {
	field = strings.TrimSpace(field)
	if field == "" {
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
	prop, ok := properties[field]
	if !ok || schemaTypeFromRaw(prop) != "array" {
		return false
	}
	for _, required := range requiredFieldNames(root) {
		if required != field {
			return false
		}
	}
	return true
}

func schemaTypeFromRaw(raw json.RawMessage) string {
	obj := parseJSONObject(raw)
	return rawString(obj["type"])
}

func topLevelPointerToken(pointer string) string {
	if !strings.HasPrefix(pointer, "/") {
		return ""
	}
	tokens := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	if len(tokens) != 1 || strings.TrimSpace(tokens[0]) == "" {
		return ""
	}
	return strings.ReplaceAll(strings.ReplaceAll(tokens[0], "~1", "/"), "~0", "~")
}

func replaceSelectorSource(selector *spec.SelectorAST, target string, source spec.ValueSource) bool {
	if selector == nil {
		return false
	}
	target = strings.TrimSpace(target)
	for i := range selector.Assignments {
		if selector.Assignments[i].Target == target {
			selector.Assignments[i].Source = source
			return true
		}
	}
	return false
}

func inlineActionBindingID(contract *model.FunctionContract) string {
	return sanitizeBindingID("action." + firstNonEmpty(contract.OperationKey, contract.FunctionID))
}

func inlineActionKey(contract *model.FunctionContract) string {
	return sanitizeBindingID(firstNonEmpty(contract.OperationKey, contract.FunctionID))
}

func inlineActionTitle(contract *model.FunctionContract, locale string) spec.LocalizedText {
	summary := strings.TrimSpace(jsonMapToLocalizedText(contract.Summary)[locale])
	if summary != "" {
		return spec.LocalizedText{locale: summary}
	}
	return spec.LocalizedText{locale: humanizeKey(firstNonEmpty(contract.OperationKey, contract.FunctionID))}
}

func inlineActionType(contract *model.FunctionContract) string {
	switch spec.RiskLevel(contract.Risk) {
	case spec.RiskHigh, spec.RiskDanger:
		return "danger"
	default:
		return "default"
	}
}

func inlineActionNeedsConfirm(contract *model.FunctionContract) bool {
	if jsonMapToApprovalPolicy(contract.Approval).Required {
		return true
	}
	switch spec.RiskLevel(contract.Risk) {
	case spec.RiskHigh, spec.RiskDanger:
		return true
	default:
		return false
	}
}

func defaultListView() *spec.ListViewSpec {
	return &spec.ListViewSpec{
		Columns: []spec.ColumnSpec{},
	}
}

func buildFiltersFromContract(contract *model.FunctionContract, semantics *model.CapabilitySemantics) []spec.FilterSpec {
	if contract == nil || len(contract.InputSchema) == 0 {
		return nil
	}
	properties := objectProperty(parseJSONObject(json.RawMessage(contract.InputSchema)), "properties")
	if len(properties) == 0 {
		return nil
	}
	pageField := strings.TrimSpace(semantics.PageFieldName)
	pageSizeField := strings.TrimSpace(semantics.PageSizeFieldName)
	keys := make([]string, 0, len(properties))
	for key := range properties {
		if key == pageField || key == pageSizeField {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	filters := make([]spec.FilterSpec, 0, len(keys))
	for _, key := range keys {
		prop := properties[key]
		filters = append(filters, spec.FilterSpec{
			Key:     key,
			Title:   spec.LocalizedText{"zh-CN": key},
			Type:    inferFilterType(prop),
			Options: enumOptionsFromSchema(prop),
		})
	}
	return filters
}

func paginationFromContract(contract *model.FunctionContract, semantics *model.CapabilitySemantics) *spec.PaginationSpec {
	if contract == nil || len(contract.InputSchema) == 0 || semantics == nil {
		return nil
	}
	properties := objectProperty(parseJSONObject(json.RawMessage(contract.InputSchema)), "properties")
	if len(properties) == 0 {
		return nil
	}
	pageField := strings.TrimSpace(semantics.PageFieldName)
	pageSizeField := strings.TrimSpace(semantics.PageSizeFieldName)
	if pageField == "" || pageSizeField == "" {
		return nil
	}
	if _, ok := properties[pageField]; !ok {
		return nil
	}
	if _, ok := properties[pageSizeField]; !ok {
		return nil
	}
	return &spec.PaginationSpec{
		Enabled:     true,
		DefaultSize: 20,
		PageSizes:   []int{10, 20, 50, 100},
	}
}

func findCollectionItemsSchema(raw spec.JSONSchema, semantics *model.CapabilitySemantics) json.RawMessage {
	root := parseJSONObject(json.RawMessage(raw))
	if len(root) == 0 {
		return nil
	}
	if items := objectProperty(root, "items"); len(items) > 0 {
		return rawObject(items)
	}

	properties := objectProperty(root, "properties")
	if len(properties) == 0 {
		return nil
	}
	for _, key := range collectionArrayKeys(semantics) {
		prop := objectProperty(properties, key)
		if len(prop) == 0 {
			continue
		}
		if items := objectProperty(prop, "items"); len(items) > 0 {
			return rawObject(items)
		}
	}
	return nil
}

func collectionArrayKeys(semantics *model.CapabilitySemantics) []string {
	keys := []string{}
	if semantics != nil {
		if key := strings.TrimSpace(semantics.ItemsFieldName); key != "" {
			keys = append(keys, key)
		}
	}
	return append(keys, "items", "list", "rows", "data")
}

func buildDeleteAction(contract *model.FunctionContract, locale string) *spec.ConfirmActionSpec {
	if contract == nil {
		return nil
	}
	return &spec.ConfirmActionSpec{
		Title:       spec.LocalizedText{locale: "删除"},
		Description: spec.LocalizedText{locale: "确认删除此记录？删除后不可恢复。"},
		ConfirmText: spec.LocalizedText{locale: "确认删除"},
		CancelText:  spec.LocalizedText{locale: "取消"},
		BindingID:   "delete",
		Permission:  strings.TrimSpace(contract.Permission),
		Risk:        strings.TrimSpace(contract.Risk),
	}
}

func resourceOperationRequiresConfirmation(contract *model.FunctionContract) bool {
	if contract == nil {
		return false
	}
	if jsonMapToApprovalPolicy(contract.Approval).Required {
		return true
	}
	switch spec.RiskLevel(contract.Risk) {
	case spec.RiskHigh, spec.RiskDanger:
		return true
	default:
		return false
	}
}

func inferDataType(raw json.RawMessage) string {
	prop := parseJSONObject(raw)
	typeStr := rawString(prop["type"])
	format := rawString(prop["format"])
	switch typeStr {
	case "integer", "number":
		return "number"
	case "boolean":
		return "boolean"
	case "string":
		switch format {
		case "date-time", "date":
			return "datetime"
		default:
			return "string"
		}
	default:
		return "string"
	}
}

func inferFilterType(raw json.RawMessage) string {
	prop := parseJSONObject(raw)
	typeStr := rawString(prop["type"])
	format := rawString(prop["format"])
	if len(prop["enum"]) > 0 {
		return "select"
	}
	switch typeStr {
	case "integer", "number":
		return "number"
	case "string":
		switch format {
		case "date", "date-time":
			return "date"
		default:
			return "text"
		}
	default:
		return "text"
	}
}

func enumOptionsFromSchema(raw json.RawMessage) []spec.EnumOption {
	prop := parseJSONObject(raw)
	if len(prop["enum"]) == 0 {
		return nil
	}
	var values []string
	if err := json.Unmarshal(prop["enum"], &values); err != nil {
		return nil
	}
	options := make([]spec.EnumOption, 0, len(values))
	for _, value := range values {
		options = append(options, spec.EnumOption{
			Value: value,
			Label: spec.LocalizedText{"zh-CN": value},
		})
	}
	return options
}

// assessResourceSemantics checks if semantics are sufficient for CRUD.
func assessResourceSemantics(semantics *model.CapabilitySemantics) []spec.Diagnostic {
	var diags []spec.Diagnostic
	if semantics == nil {
		return []spec.Diagnostic{{
			Code:     "resource_semantics_missing",
			Severity: spec.SeverityError,
			Message:  "resource capability semantics are required",
		}}
	}
	if strings.TrimSpace(semantics.IdentityField) == "" {
		diags = append(diags, spec.Diagnostic{
			Code:     "identity_missing",
			Severity: spec.SeverityWarning,
			Message:  "identity field not specified; detail view cannot be generated",
		})
	}
	if semantics.CollectionQueryID == 0 {
		diags = append(diags, spec.Diagnostic{
			Code:     "collection_query_missing",
			Severity: spec.SeverityError,
			Message:  "collection_query function not found; list view cannot be generated",
		})
	}
	var conflicts []spec.SemanticConflict
	if len(semantics.Conflicts) > 0 {
		if err := json.Unmarshal(semantics.Conflicts, &conflicts); err != nil {
			diags = append(diags, spec.Diagnostic{
				Code:     "semantic_conflicts_invalid",
				Severity: spec.SeverityError,
				Message:  "capability semantics conflicts cannot be decoded",
				Field:    "capabilitySemantics.conflicts",
			})
		}
	}
	for _, conflict := range conflicts {
		if conflict.Resolution == "" {
			diags = append(diags, spec.Diagnostic{
				Code:     "semantic_conflict_unresolved",
				Severity: spec.SeverityError,
				Message:  "capability semantic conflict requires platform review before publishing",
				Field:    strings.TrimSpace(conflict.Field),
			})
		}
	}
	return diags
}

func validateGeneratedResourceViews(
	listView *spec.ListViewSpec,
	detailView *spec.DetailViewSpec,
	semantics *model.CapabilitySemantics,
) []spec.Diagnostic {
	if listView == nil || semantics == nil {
		return nil
	}
	var diags []spec.Diagnostic
	identityField := strings.TrimSpace(semantics.IdentityField)
	if identityField != "" && !listViewHasColumn(listView, identityField) {
		diags = append(diags, spec.Diagnostic{
			Code:     "resource_identity_column_missing",
			Severity: spec.SeverityError,
			Message:  "resource list view could not derive the identity column from collection item schema",
			Field:    "resource.listView.identityKey",
		})
	}
	return diags
}

func listViewHasColumn(listView *spec.ListViewSpec, key string) bool {
	for _, column := range listView.Columns {
		if strings.TrimSpace(column.Key) == key {
			return true
		}
	}
	return false
}

func resourceQuality(semantics *model.CapabilitySemantics, diags []spec.Diagnostic) spec.GeneratedPageQuality {
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			return spec.GeneratedPageQualityNeedsReview
		}
		if d.Code == "json_schema_generation_subset_unsupported" {
			return spec.GeneratedPageQualityNeedsReview
		}
		if d.Severity == spec.SeverityWarning {
			return spec.GeneratedPageQualityBasic
		}
	}
	if semantics != nil && semantics.CollectionQueryID > 0 && strings.TrimSpace(semantics.IdentityField) != "" {
		return spec.GeneratedPageQualityReady
	}
	return spec.GeneratedPageQualityBasic
}

func findResourceContract(contracts []*model.FunctionContract, id uint, capability spec.CapabilityKind) *model.FunctionContract {
	if id == 0 {
		return nil
	}
	contract := findContractByID(contracts, id)
	if contract != nil && spec.CapabilityKind(contract.Capability) == capability {
		return contract
	}
	return nil
}

func findContractByID(contracts []*model.FunctionContract, id uint) *model.FunctionContract {
	for _, c := range contracts {
		if c != nil && c.ID == id {
			return c
		}
	}
	return nil
}

func findContractByFunctionID(contracts []*model.FunctionContract, functionID string) *model.FunctionContract {
	functionID = strings.TrimSpace(functionID)
	for _, contract := range contracts {
		if contract != nil && strings.TrimSpace(contract.FunctionID) == functionID {
			return contract
		}
	}
	return nil
}

func contractToFunctionSpec(c *model.FunctionContract) spec.FunctionSpec {
	if c == nil {
		return spec.FunctionSpec{}
	}
	return spec.FunctionSpec{
		ID:           c.FunctionID,
		Version:      c.Version,
		Enabled:      c.Enabled,
		Deprecated:   c.Deprecated,
		InputSchema:  spec.JSONSchema(c.InputSchema),
		OutputSchema: spec.JSONSchema(c.OutputSchema),
		Summary:      jsonMapToLocalizedText(c.Summary),
		Description:  jsonMapToLocalizedText(c.Description),
		Resource:     c.ResourceKey,
		Operation:    c.OperationKey,
		Capability:   spec.CapabilityKind(c.Capability),
		Execution:    spec.FunctionExecution(c.Execution),
		Approval:     jsonMapToApprovalPolicy(c.Approval),
		Risk:         spec.RiskLevel(c.Risk),
		Permission:   c.Permission,
	}
}

func jsonMapToApprovalPolicy(values datatypes.JSONMap) spec.ApprovalPolicy {
	if len(values) == 0 {
		return spec.ApprovalPolicy{}
	}
	required, _ := values["required"].(bool)
	policyKey, _ := values["policyKey"].(string)
	if policyKey == "" {
		policyKey, _ = values["policy_key"].(string)
	}
	return spec.ApprovalPolicy{
		Required:  required,
		PolicyKey: strings.TrimSpace(policyKey),
	}
}

func jsonMapToLocalizedText(values datatypes.JSONMap) spec.LocalizedText {
	if len(values) == 0 {
		return nil
	}
	out := make(spec.LocalizedText, len(values))
	for key, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			out[key] = text
		}
	}
	return out
}

func extractString(obj map[string]json.RawMessage, key string) string {
	if len(obj) == 0 {
		return ""
	}
	return strings.TrimSpace(rawString(obj[key]))
}

func parseJSONObject(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func objectProperty(obj map[string]json.RawMessage, key string) map[string]json.RawMessage {
	if len(obj) == 0 {
		return nil
	}
	return parseJSONObject(obj[key])
}

func rawObject(obj map[string]json.RawMessage) json.RawMessage {
	if len(obj) == 0 {
		return nil
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	return raw
}

func rawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}
