// Package merge implements three-way merge for PageSpec proposals.
// It compares base Proposal, current Draft, and latest Proposal to produce
// auto-merge items and conflict items.
package merge

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// MergeResult represents the result of a three-way merge.
type MergeResult struct {
	// AutoMerge contains fields that can be safely merged automatically.
	// Only display fields are eligible for auto-merge.
	AutoMerge []MergeItem `json:"autoMerge"`

	// Conflicts contains fields that require explicit user resolution.
	Conflicts []MergeConflict `json:"conflicts"`

	// HasConflicts returns true if there are conflicts requiring resolution.
	HasConflicts bool `json:"hasConflicts"`
}

// MergeItem represents a single field that can be auto-merged.
type MergeItem struct {
	// Field is the field path (e.g., "title", "category.labels", "resource.listView.columns[0].title").
	Field string `json:"field"`

	// BaseValue is the value from the base Proposal.
	BaseValue json.RawMessage `json:"baseValue"`

	// DraftValue is the value from the current Draft.
	DraftValue json.RawMessage `json:"draftValue"`

	// LatestValue is the value from the latest Proposal.
	LatestValue json.RawMessage `json:"latestValue"`

	// MergedValue is the value after auto-merge.
	MergedValue json.RawMessage `json:"mergedValue"`

	// Reason explains why this field was auto-merged.
	Reason string `json:"reason"`
}

// MergeConflict represents a field that requires explicit resolution.
type MergeConflict struct {
	// Field is the field path.
	Field string `json:"field"`

	// BaseValue is the value from the base Proposal.
	BaseValue json.RawMessage `json:"baseValue"`

	// DraftValue is the value from the current Draft (user's changes).
	DraftValue json.RawMessage `json:"draftValue"`

	// LatestValue is the value from the latest Proposal (generator's changes).
	LatestValue json.RawMessage `json:"latestValue"`

	// Reason explains why this field cannot be auto-merged.
	Reason string `json:"reason"`
}

// AutoMergeFields defines fields that can be safely auto-merged.
// These are display-only fields that don't affect execution semantics.
var AutoMergeFields = map[string]bool{
	"title":                                  true,
	"description":                            true,
	"icon":                                   true,
	"order":                                  true,
	"category.labels":                        true,
	"category.order":                         true,
	"navigation.title":                       true,
	"navigation.breadcrumb":                  true,
	"resource.listView.columns[].title":      true,
	"resource.listView.columns[].width":      true,
	"resource.listView.columns[].visible":    true,
	"resource.listView.columns[].sortable":   true,
	"resource.listView.columns[].filterable": true,
	"resource.listView.defaultSort":          true,
	"resource.detailView.fields[].title":     true,
	"resource.detailView.fields[].span":      true,
	"resource.detailView.fields[].visible":   true,
	"operation.form.fields[].label":          true,
	"operation.form.fields[].placeholder":    true,
	"operation.form.fields[].description":    true,
	"operation.form.fields[].order":          true,
	"operation.form.fields[].group":          true,
	"operation.form.fields[].widget":         true,
	"task.form.fields[].label":               true,
	"task.form.fields[].placeholder":         true,
	"task.form.fields[].description":         true,
	"report.queryForm.fields[].label":        true,
	"report.queryForm.fields[].placeholder":  true,
	"report.queryForm.fields[].description":  true,
	"report.charts[].title":                  true,
}

// ConflictFields defines fields that require explicit resolution.
// These affect execution semantics and cannot be auto-merged.
var ConflictFields = map[string]bool{
	"type":                                      true,
	"resourceKey":                               true,
	"category.key":                              true,
	"bindings":                                  true,
	"bindings[].id":                             true,
	"bindings[].functionId":                     true,
	"bindings[].usage":                          true,
	"bindings[].selectors":                      true,
	"bindings[].execution":                      true,
	"resource.listView.identityKey":             true,
	"resource.listView.rowSchema":               true,
	"resource.listView.filters":                 true,
	"resource.listView.pagination":              true,
	"resource.listView.rowActions":              true,
	"resource.listView.batchActions":            true,
	"resource.listView.toolbarActions":          true,
	"resource.detailView.actions":               true,
	"resource.createForm":                       true,
	"resource.updateForm":                       true,
	"resource.deleteAction":                     true,
	"resource.actions":                          true,
	"resource.actions[].bindingId":              true,
	"resource.actions[].confirm":                true,
	"resource.actions[].risk":                   true,
	"resource.actions[].permission":             true,
	"operation.form.jsonSchema":                 true,
	"operation.form.fields[].key":               true,
	"operation.form.fields[].visibleWhen":       true,
	"operation.form.fields[].required":          true,
	"operation.form.fields[].defaultValue":      true,
	"operation.form.fields[].disabled":          true,
	"operation.form.fields[].widgetProps":       true,
	"operation.form.fields[].validationRules":   true,
	"operation.confirm":                         true,
	"operation.resultView":                      true,
	"task.form.jsonSchema":                      true,
	"task.form.fields[].key":                    true,
	"task.form.fields[].visibleWhen":            true,
	"task.form.fields[].required":               true,
	"task.form.fields[].defaultValue":           true,
	"task.form.fields[].disabled":               true,
	"task.form.fields[].widgetProps":            true,
	"task.form.fields[].validationRules":        true,
	"task.taskView":                             true,
	"task.resultView":                           true,
	"report.queryForm.jsonSchema":               true,
	"report.queryForm.fields[].key":             true,
	"report.queryForm.fields[].visibleWhen":     true,
	"report.queryForm.fields[].required":        true,
	"report.queryForm.fields[].defaultValue":    true,
	"report.queryForm.fields[].disabled":        true,
	"report.queryForm.fields[].widgetProps":     true,
	"report.queryForm.fields[].validationRules": true,
	"report.dataset":                            true,
	"report.table":                              true,
	"permission":                                true,
	"risk":                                      true,
}

// ThreeWayMerge performs a three-way merge of PageSpec proposals.
//
// Parameters:
//   - base: The original Proposal that the Draft was based on
//   - draft: The user's current Draft (may have manual edits)
//   - latest: The latest Proposal from the generator
//
// Returns a MergeResult with auto-merge items and conflicts.
func ThreeWayMerge(
	base spec.PageSpec,
	draft spec.PageSpec,
	latest spec.PageSpec,
) MergeResult {
	result := MergeResult{
		AutoMerge: make([]MergeItem, 0),
		Conflicts: make([]MergeConflict, 0),
	}

	// Compare top-level fields
	compareField("", "title", toJSON(base.Title), toJSON(draft.Title), toJSON(latest.Title), &result)
	compareField("", "description", toJSON(base.Description), toJSON(draft.Description), toJSON(latest.Description), &result)
	compareField("", "icon", toJSON(base.Icon), toJSON(draft.Icon), toJSON(latest.Icon), &result)
	compareField("", "order", toJSON(base.Order), toJSON(draft.Order), toJSON(latest.Order), &result)
	compareField("", "type", toJSON(base.Type), toJSON(draft.Type), toJSON(latest.Type), &result)
	compareField("", "resourceKey", toJSON(base.ResourceKey), toJSON(draft.ResourceKey), toJSON(latest.ResourceKey), &result)

	// Compare category
	compareField("", "category.key", toJSON(base.Category.Key), toJSON(draft.Category.Key), toJSON(latest.Category.Key), &result)
	compareField("", "category.labels", toJSON(base.Category.Labels), toJSON(draft.Category.Labels), toJSON(latest.Category.Labels), &result)
	compareField("", "category.order", toJSON(base.Category.Order), toJSON(draft.Category.Order), toJSON(latest.Category.Order), &result)

	// Compare navigation
	if base.Navigation != nil && draft.Navigation != nil && latest.Navigation != nil {
		compareField("", "navigation.title", toJSON(base.Navigation.Title), toJSON(draft.Navigation.Title), toJSON(latest.Navigation.Title), &result)
	}

	// Compare bindings (conflict field)
	compareField("", "bindings", toJSON(base.Bindings), toJSON(draft.Bindings), toJSON(latest.Bindings), &result)

	// Compare resource-specific fields
	if base.Resource != nil && draft.Resource != nil && latest.Resource != nil {
		compareResourceFields(base.Resource, draft.Resource, latest.Resource, &result)
	}

	// Compare operation-specific fields
	if base.Operation != nil && draft.Operation != nil && latest.Operation != nil {
		compareOperationFields(base.Operation, draft.Operation, latest.Operation, &result)
	}

	// Compare task-specific fields
	if base.Task != nil && draft.Task != nil && latest.Task != nil {
		compareTaskFields(base.Task, draft.Task, latest.Task, &result)
	}

	// Compare report-specific fields
	if base.Report != nil && draft.Report != nil && latest.Report != nil {
		compareReportFields(base.Report, draft.Report, latest.Report, &result)
	}

	result.HasConflicts = len(result.Conflicts) > 0
	return result
}

func compareField(prefix, field string, base, draft, latest json.RawMessage, result *MergeResult) {
	fullField := field
	if prefix != "" {
		fullField = prefix + "." + field
	}

	// Skip if all values are the same
	if bytesEqual(base, draft) && bytesEqual(draft, latest) {
		return
	}
	if bytesEqual(base, latest) {
		return
	}
	if bytesEqual(draft, latest) {
		return
	}

	// Check if this is an auto-merge field
	if isAutoMergeField(fullField) {
		// Auto-merge: prefer draft (user's changes) over latest (generator's changes)
		merged := draft
		if bytesEqual(draft, base) {
			// User didn't change this field, use latest
			merged = latest
		}

		result.AutoMerge = append(result.AutoMerge, MergeItem{
			Field:       fullField,
			BaseValue:   base,
			DraftValue:  draft,
			LatestValue: latest,
			MergedValue: merged,
			Reason:      "display field auto-merge",
		})
		return
	}

	// Check if this is a conflict field
	if isConflictField(fullField) {
		result.Conflicts = append(result.Conflicts, MergeConflict{
			Field:       fullField,
			BaseValue:   base,
			DraftValue:  draft,
			LatestValue: latest,
			Reason:      "execution semantic field requires explicit resolution",
		})
		return
	}

	// Default: treat as conflict if values differ
	if !bytesEqual(base, latest) && !bytesEqual(draft, latest) {
		result.Conflicts = append(result.Conflicts, MergeConflict{
			Field:       fullField,
			BaseValue:   base,
			DraftValue:  draft,
			LatestValue: latest,
			Reason:      "field changed in both draft and latest proposal",
		})
	}
}

func compareResourceFields(base, draft, latest *spec.ResourcePageSpec, result *MergeResult) {
	// Compare list view columns (auto-merge for display fields)
	if base.ListView != nil && draft.ListView != nil && latest.ListView != nil {
		compareField("resource.listView", "identityKey", toJSON(base.ListView.IdentityKey), toJSON(draft.ListView.IdentityKey), toJSON(latest.ListView.IdentityKey), result)
		compareField("resource.listView", "rowSchema", toJSON(base.ListView.RowSchema), toJSON(draft.ListView.RowSchema), toJSON(latest.ListView.RowSchema), result)
		compareField("resource.listView", "defaultSort", toJSON(base.ListView.DefaultSort), toJSON(draft.ListView.DefaultSort), toJSON(latest.ListView.DefaultSort), result)
		compareField("resource.listView", "filters", toJSON(base.ListView.Filters), toJSON(draft.ListView.Filters), toJSON(latest.ListView.Filters), result)
		compareField("resource.listView", "pagination", toJSON(base.ListView.Pagination), toJSON(draft.ListView.Pagination), toJSON(latest.ListView.Pagination), result)
		compareField("resource.listView", "rowActions", toJSON(base.ListView.RowActions), toJSON(draft.ListView.RowActions), toJSON(latest.ListView.RowActions), result)
		compareField("resource.listView", "batchActions", toJSON(base.ListView.BatchActions), toJSON(draft.ListView.BatchActions), toJSON(latest.ListView.BatchActions), result)
		compareField("resource.listView", "toolbarActions", toJSON(base.ListView.ToolbarActions), toJSON(draft.ListView.ToolbarActions), toJSON(latest.ListView.ToolbarActions), result)
		compareColumns(base.ListView.Columns, draft.ListView.Columns, latest.ListView.Columns, result)
	}

	// Compare detail view fields (auto-merge for display fields)
	if base.DetailView != nil && draft.DetailView != nil && latest.DetailView != nil {
		compareField("resource.detailView", "actions", toJSON(base.DetailView.Actions), toJSON(draft.DetailView.Actions), toJSON(latest.DetailView.Actions), result)
		compareDetailFields(base.DetailView.Fields, draft.DetailView.Fields, latest.DetailView.Fields, result)
	}

	// Compare actions (conflict field)
	compareField("resource", "actions", toJSON(base.Actions), toJSON(draft.Actions), toJSON(latest.Actions), result)
	compareField("resource", "createForm", toJSON(base.CreateForm), toJSON(draft.CreateForm), toJSON(latest.CreateForm), result)
	compareField("resource", "updateForm", toJSON(base.UpdateForm), toJSON(draft.UpdateForm), toJSON(latest.UpdateForm), result)
	compareField("resource", "deleteAction", toJSON(base.DeleteAction), toJSON(draft.DeleteAction), toJSON(latest.DeleteAction), result)
}

func compareOperationFields(base, draft, latest *spec.OperationPageSpec, result *MergeResult) {
	// Compare form fields (auto-merge for display fields)
	if base.Form != nil && draft.Form != nil && latest.Form != nil {
		compareField("operation.form", "jsonSchema", toJSON(base.Form.JSONSchema), toJSON(draft.Form.JSONSchema), toJSON(latest.Form.JSONSchema), result)
		compareFormFields(base.Form.Fields, draft.Form.Fields, latest.Form.Fields, "operation.form", result)
	}

	// Compare confirm (conflict field)
	compareField("operation", "confirm", toJSON(base.Confirm), toJSON(draft.Confirm), toJSON(latest.Confirm), result)
	compareField("operation", "resultView", toJSON(base.ResultView), toJSON(draft.ResultView), toJSON(latest.ResultView), result)
}

func compareTaskFields(base, draft, latest *spec.TaskPageSpec, result *MergeResult) {
	// Compare form fields (auto-merge for display fields)
	if base.Form != nil && draft.Form != nil && latest.Form != nil {
		compareField("task.form", "jsonSchema", toJSON(base.Form.JSONSchema), toJSON(draft.Form.JSONSchema), toJSON(latest.Form.JSONSchema), result)
		compareFormFields(base.Form.Fields, draft.Form.Fields, latest.Form.Fields, "task.form", result)
	}
	compareField("task", "taskView", toJSON(base.TaskView), toJSON(draft.TaskView), toJSON(latest.TaskView), result)
	compareField("task", "resultView", toJSON(base.ResultView), toJSON(draft.ResultView), toJSON(latest.ResultView), result)
}

func compareReportFields(base, draft, latest *spec.ReportPageSpec, result *MergeResult) {
	// Compare query form fields (auto-merge for display fields)
	if base.QueryForm != nil && draft.QueryForm != nil && latest.QueryForm != nil {
		compareField("report.queryForm", "jsonSchema", toJSON(base.QueryForm.JSONSchema), toJSON(draft.QueryForm.JSONSchema), toJSON(latest.QueryForm.JSONSchema), result)
		compareFormFields(base.QueryForm.Fields, draft.QueryForm.Fields, latest.QueryForm.Fields, "report.queryForm", result)
	}

	// Compare charts (auto-merge for titles)
	compareCharts(base.Charts, draft.Charts, latest.Charts, result)
	compareField("report", "dataset", toJSON(base.Dataset), toJSON(draft.Dataset), toJSON(latest.Dataset), result)
	compareField("report", "table", toJSON(base.Table), toJSON(draft.Table), toJSON(latest.Table), result)
}

func compareColumns(base, draft, latest []spec.ColumnSpec, result *MergeResult) {
	maxLen := len(base)
	if len(draft) > maxLen {
		maxLen = len(draft)
	}
	if len(latest) > maxLen {
		maxLen = len(latest)
	}

	for i := 0; i < maxLen; i++ {
		prefix := "resource.listView.columns[" + strconv.Itoa(i) + "]"
		var baseCol, draftCol, latestCol *spec.ColumnSpec
		if i < len(base) {
			baseCol = &base[i]
		}
		if i < len(draft) {
			draftCol = &draft[i]
		}
		if i < len(latest) {
			latestCol = &latest[i]
		}

		if baseCol != nil && draftCol != nil && latestCol != nil {
			compareField(prefix, "title", toJSON(baseCol.Title), toJSON(draftCol.Title), toJSON(latestCol.Title), result)
			compareField(prefix, "width", toJSON(baseCol.Width), toJSON(draftCol.Width), toJSON(latestCol.Width), result)
			compareField(prefix, "visible", toJSON(baseCol.Visible), toJSON(draftCol.Visible), toJSON(latestCol.Visible), result)
			compareField(prefix, "sortable", toJSON(baseCol.Sortable), toJSON(draftCol.Sortable), toJSON(latestCol.Sortable), result)
			compareField(prefix, "filterable", toJSON(baseCol.Filterable), toJSON(draftCol.Filterable), toJSON(latestCol.Filterable), result)
		}
	}
}

func compareDetailFields(base, draft, latest []spec.DetailFieldSpec, result *MergeResult) {
	maxLen := len(base)
	if len(draft) > maxLen {
		maxLen = len(draft)
	}
	if len(latest) > maxLen {
		maxLen = len(latest)
	}

	for i := 0; i < maxLen; i++ {
		prefix := "resource.detailView.fields[" + strconv.Itoa(i) + "]"
		var baseField, draftField, latestField *spec.DetailFieldSpec
		if i < len(base) {
			baseField = &base[i]
		}
		if i < len(draft) {
			draftField = &draft[i]
		}
		if i < len(latest) {
			latestField = &latest[i]
		}

		if baseField != nil && draftField != nil && latestField != nil {
			compareField(prefix, "title", toJSON(baseField.Title), toJSON(draftField.Title), toJSON(latestField.Title), result)
			compareField(prefix, "span", toJSON(baseField.Span), toJSON(draftField.Span), toJSON(latestField.Span), result)
			compareField(prefix, "visible", toJSON(baseField.Visible), toJSON(draftField.Visible), toJSON(latestField.Visible), result)
		}
	}
}

func compareFormFields(base, draft, latest []spec.FormFieldSpec, prefix string, result *MergeResult) {
	maxLen := len(base)
	if len(draft) > maxLen {
		maxLen = len(draft)
	}
	if len(latest) > maxLen {
		maxLen = len(latest)
	}

	for i := 0; i < maxLen; i++ {
		fieldPrefix := prefix + ".fields[" + strconv.Itoa(i) + "]"
		var baseField, draftField, latestField *spec.FormFieldSpec
		if i < len(base) {
			baseField = &base[i]
		}
		if i < len(draft) {
			draftField = &draft[i]
		}
		if i < len(latest) {
			latestField = &latest[i]
		}

		if baseField != nil && draftField != nil && latestField != nil {
			compareField(fieldPrefix, "label", toJSON(baseField.Label), toJSON(draftField.Label), toJSON(latestField.Label), result)
			compareField(fieldPrefix, "placeholder", toJSON(baseField.Placeholder), toJSON(draftField.Placeholder), toJSON(latestField.Placeholder), result)
			compareField(fieldPrefix, "description", toJSON(baseField.Description), toJSON(draftField.Description), toJSON(latestField.Description), result)
			compareField(fieldPrefix, "order", toJSON(baseField.Order), toJSON(draftField.Order), toJSON(latestField.Order), result)
			compareField(fieldPrefix, "widget", toJSON(baseField.Widget), toJSON(draftField.Widget), toJSON(latestField.Widget), result)
			compareField(fieldPrefix, "key", toJSON(baseField.Key), toJSON(draftField.Key), toJSON(latestField.Key), result)
			compareField(fieldPrefix, "visibleWhen", toJSON(baseField.VisibleWhen), toJSON(draftField.VisibleWhen), toJSON(latestField.VisibleWhen), result)
			compareField(fieldPrefix, "required", toJSON(baseField.Required), toJSON(draftField.Required), toJSON(latestField.Required), result)
			compareField(fieldPrefix, "defaultValue", toJSON(baseField.DefaultValue), toJSON(draftField.DefaultValue), toJSON(latestField.DefaultValue), result)
			compareField(fieldPrefix, "disabled", toJSON(baseField.Disabled), toJSON(draftField.Disabled), toJSON(latestField.Disabled), result)
			compareField(fieldPrefix, "widgetProps", toJSON(baseField.WidgetProps), toJSON(draftField.WidgetProps), toJSON(latestField.WidgetProps), result)
			compareField(fieldPrefix, "validationRules", toJSON(baseField.ValidationRules), toJSON(draftField.ValidationRules), toJSON(latestField.ValidationRules), result)
		}
	}
}

func compareCharts(base, draft, latest []spec.ChartSpec, result *MergeResult) {
	maxLen := len(base)
	if len(draft) > maxLen {
		maxLen = len(draft)
	}
	if len(latest) > maxLen {
		maxLen = len(latest)
	}

	for i := 0; i < maxLen; i++ {
		prefix := "report.charts[" + strconv.Itoa(i) + "]"
		var baseChart, draftChart, latestChart *spec.ChartSpec
		if i < len(base) {
			baseChart = &base[i]
		}
		if i < len(draft) {
			draftChart = &draft[i]
		}
		if i < len(latest) {
			latestChart = &latest[i]
		}

		if baseChart != nil && draftChart != nil && latestChart != nil {
			compareField(prefix, "title", toJSON(baseChart.Title), toJSON(draftChart.Title), toJSON(latestChart.Title), result)
		}
	}
}

func isAutoMergeField(field string) bool {
	// Check exact match
	if AutoMergeFields[field] {
		return true
	}

	// Check pattern match (e.g., "columns[].title")
	for pattern := range AutoMergeFields {
		if matchPattern(pattern, field) {
			return true
		}
	}

	return false
}

func isConflictField(field string) bool {
	// Check exact match
	if ConflictFields[field] {
		return true
	}

	// Check pattern match
	for pattern := range ConflictFields {
		if matchPattern(pattern, field) {
			return true
		}
	}

	return false
}

func matchPattern(pattern, field string) bool {
	if !strings.Contains(pattern, "[]") {
		return pattern == field
	}
	parts := strings.Split(pattern, "[]")
	if len(parts) != 2 {
		return false
	}
	prefix, suffix := parts[0], parts[1]
	if !strings.HasPrefix(field, prefix+"[") || !strings.HasSuffix(field, suffix) {
		return false
	}
	indexPart := strings.TrimSuffix(strings.TrimPrefix(field, prefix+"["), "]"+suffix)
	if indexPart == "" {
		return false
	}
	_, err := strconv.Atoi(indexPart)
	return err == nil
}

func toJSON[T any](v T) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

func bytesEqual(a, b json.RawMessage) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	return string(a) == string(b)
}
