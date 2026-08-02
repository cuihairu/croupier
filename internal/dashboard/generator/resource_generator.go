package generator

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

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

	resourceKey := strings.TrimSpace(semantics.ResourceKey)
	pageKey := resourceKey + ".manage"
	locale := opts.DefaultLocale
	title := spec.LocalizedText{locale: resourceKey}

	bindings := []spec.PageFunctionBinding{
		resourceBinding(collectionContract, "list", spec.BindingUsageQuery),
	}

	var actions []spec.ActionSpec
	if createContract != nil {
		bindings = append(bindings, resourceBinding(createContract, "create", spec.BindingUsageAction))
	}
	if updateContract != nil {
		bindings = append(bindings, resourceBinding(updateContract, "update", spec.BindingUsageAction))
		actions = append(actions, spec.ActionSpec{
			Key:       "edit",
			Title:     spec.LocalizedText{locale: "编辑"},
			Type:      "link",
			BindingID: "update",
		})
	}
	if deleteContract != nil {
		bindings = append(bindings, resourceBinding(deleteContract, "delete", spec.BindingUsageAction))
		actions = append(actions, spec.ActionSpec{
			Key:          "delete",
			Title:        spec.LocalizedText{locale: "删除"},
			Type:         "danger",
			Confirm:      true,
			ConfirmTitle: spec.LocalizedText{locale: "确认删除"},
			ConfirmDesc:  spec.LocalizedText{locale: "删除后不可恢复，确认继续？"},
			BindingID:    "delete",
			Risk:         "high",
		})
	}

	diags := assessResourceSemantics(semantics)
	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeResource,
			ResourceKey: resourceKey,
			Title:       title,
			Category:    categoryForPage(resourceKey, pageKey, locale),
			Navigation: &spec.NavigationSpec{
				Title: title,
			},
			Resource: &spec.ResourcePageSpec{
				ListView:     buildListViewFromContract(collectionContract, semantics),
				Actions:      actions,
				CreateForm:   buildFormFromContract(createContract),
				UpdateForm:   buildFormFromContract(updateContract),
				DeleteAction: buildDeleteAction(deleteContract, locale),
			},
			Bindings: bindings,
		},
		Quality:     resourceQuality(semantics, diags),
		Diagnostics: diags,
	}, true
}

func resourceBinding(contract *model.FunctionContract, suffix string, usage spec.PageBindingUsage) spec.PageFunctionBinding {
	binding := spec.PageFunctionBinding{
		ID:         suffix,
		FunctionID: strings.TrimSpace(contract.FunctionID),
		Usage:      usage,
		Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
	}
	if len(contract.InputSchema) > 0 {
		binding.Selectors = &spec.BindingSelectors{
			Input: spec.DefaultSelector(spec.JSONSchema(contract.InputSchema)),
		}
	}
	return binding
}

func buildFormFromContract(contract *model.FunctionContract) *spec.FormPresentationSpec {
	if contract == nil || len(contract.InputSchema) == 0 {
		return nil
	}
	return spec.DefaultFormPresentation(spec.JSONSchema(contract.InputSchema))
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
		columns = append(columns, col)
	}

	list := defaultListView()
	list.Columns = columns
	return list
}

func defaultListView() *spec.ListViewSpec {
	return &spec.ListViewSpec{
		Columns: []spec.ColumnSpec{},
		Pagination: &spec.PaginationSpec{
			Enabled:     true,
			DefaultSize: 20,
			PageSizes:   []int{10, 20, 50, 100},
		},
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
		Risk:        "high",
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
	return diags
}

func resourceQuality(semantics *model.CapabilitySemantics, diags []spec.Diagnostic) spec.GeneratedPageQuality {
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			return spec.GeneratedPageQualityBlocked
		}
	}
	if semantics != nil && semantics.CreateID > 0 && semantics.UpdateID > 0 && semantics.DeleteID > 0 &&
		strings.TrimSpace(semantics.IdentityField) != "" {
		return spec.GeneratedPageQualityReady
	}
	return spec.GeneratedPageQualityBasic
}

func findResourceContract(contracts []*model.FunctionContract, id uint, capability spec.CapabilityKind) *model.FunctionContract {
	if id > 0 {
		if contract := findContractByID(contracts, id); contract != nil {
			return contract
		}
	}
	for _, contract := range contracts {
		if contract != nil && spec.CapabilityKind(contract.Capability) == capability {
			return contract
		}
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
		Risk:         spec.RiskLevel(c.Risk),
		Permission:   c.Permission,
	}
}

func jsonMapToLocalizedText(values map[string]interface{}) spec.LocalizedText {
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
