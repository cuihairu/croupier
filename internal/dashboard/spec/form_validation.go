package spec

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateFormPresentations validates every FormPresentationSpec carried by a
// PageSpec, no matter which page variant owns it. The returned diagnostics use
// publishShapeDiagnostic (severity error) and carry a field path that locates
// the exact form entry (for example "resource.createForm.fields[2].width").
//
// The function is pure and idempotent: it only reads the page, so it can be
// called from both save/validate and publish paths without changing behavior.
func ValidateFormPresentations(page PageSpec) []Diagnostic {
	var diags []Diagnostic
	if page.Resource != nil {
		diags = append(diags, validateFormPresentation(page.Resource.CreateForm, "resource.createForm")...)
		diags = append(diags, validateFormPresentation(page.Resource.UpdateForm, "resource.updateForm")...)
		for i := range page.Resource.Actions {
			diags = append(diags, validateFormPresentation(
				page.Resource.Actions[i].Form,
				fmt.Sprintf("resource.actions[%d].form", i),
			)...)
		}
	}
	if page.Operation != nil {
		diags = append(diags, validateFormPresentation(page.Operation.Form, "operation.form")...)
	}
	if page.Task != nil {
		diags = append(diags, validateFormPresentation(page.Task.Form, "task.form")...)
	}
	if page.Report != nil {
		diags = append(diags, validateFormPresentation(page.Report.QueryForm, "report.queryForm")...)
	}
	if page.Composite != nil {
		for i := range page.Composite.Sections {
			diags = append(diags, validateFormPresentation(
				page.Composite.Sections[i].Form,
				fmt.Sprintf("composite.sections[%d].form", i),
			)...)
		}
	}
	return diags
}

// validateFormPresentation checks field declarations, groups, widget values,
// widget/schema-type compatibility and widths for one form. Schema-driven
// checks are skipped when the form has no object schema with top-level
// properties (root arrays/scalars and empty schemas are legitimate).
func validateFormPresentation(form *FormPresentationSpec, field string) []Diagnostic {
	if form == nil {
		return nil
	}
	var diags []Diagnostic
	root, resolvable := formSchemaRoot(form.JSONSchema)

	declared := make(map[string]bool, len(form.Fields))
	for i, item := range form.Fields {
		base := fmt.Sprintf("%s.fields[%d]", field, i)
		key := strings.TrimSpace(item.Key)
		if key == "" {
			diags = append(diags, publishShapeDiagnostic(
				"form_field_key_missing", "form field key is required", base+".key",
			))
			continue
		}
		if declared[key] {
			diags = append(diags, publishShapeDiagnostic(
				"form_field_duplicate",
				fmt.Sprintf("form field key %q is declared more than once", key),
				base+".key",
			))
		}
		declared[key] = true

		var node map[string]json.RawMessage
		if resolvable {
			var reason string
			node, reason = resolveFormField(root, item.Key)
			if reason != "" {
				diags = append(diags, publishShapeDiagnostic(
					"form_field_unknown",
					fmt.Sprintf("form field %q cannot be resolved in jsonSchema: %s", item.Key, reason),
					base+".key",
				))
			}
		}
		if item.Widget != "" && !isKnownFormWidget(item.Widget) {
			diags = append(diags, publishShapeDiagnostic(
				"form_widget_invalid",
				fmt.Sprintf("unknown form widget %q", item.Widget),
				base+".widget",
			))
		} else if item.Widget != "" && node != nil {
			types := formSchemaTypes(node)
			compatible, checkable := widgetTypeCompatible(item.Widget, types)
			if checkable && !compatible {
				diags = append(diags, publishShapeDiagnostic(
					"form_widget_incompatible",
					fmt.Sprintf("widget %q is not compatible with schema type %s", item.Widget, strings.Join(types, "/")),
					base+".widget",
				))
			}
		}

		if item.Width < 0 || item.Width > 12 {
			diags = append(diags, publishShapeDiagnostic(
				"form_width_invalid", "form field width must be 0 (unset) or between 1 and 12", base+".width",
			))
		}
	}

	seenGroups := make(map[string]bool, len(form.Groups))
	fieldGroups := make(map[string]string, len(form.Groups))
	for i, group := range form.Groups {
		base := fmt.Sprintf("%s.groups[%d]", field, i)
		groupKey := strings.TrimSpace(group.Key)
		if groupKey == "" {
			diags = append(diags, publishShapeDiagnostic(
				"form_group_key_missing", "form group key is required", base+".key",
			))
		} else if seenGroups[groupKey] {
			diags = append(diags, publishShapeDiagnostic(
				"form_group_key_duplicate",
				fmt.Sprintf("form group key %q is declared more than once", groupKey),
				base+".key",
			))
		}
		seenGroups[groupKey] = true
		groupID := groupKey
		if groupID == "" {
			groupID = fmt.Sprintf("#%d", i)
		}

		seenInGroup := make(map[string]bool, len(group.Fields))
		for j, fieldKey := range group.Fields {
			itemField := fmt.Sprintf("%s.fields[%d]", base, j)
			key := strings.TrimSpace(fieldKey)
			if key == "" {
				diags = append(diags, publishShapeDiagnostic(
					"form_group_field_unknown", "form group field key is empty", itemField,
				))
				continue
			}
			var reason string
			if resolvable {
				_, resolveReason := resolveFormField(root, fieldKey)
				reason = resolveReason
			}
			if !declared[key] {
				if reason != "" {
					reason += "; "
				}
				reason += "field is not declared in form.fields"
			}
			if reason != "" {
				diags = append(diags, publishShapeDiagnostic(
					"form_group_field_unknown",
					fmt.Sprintf("form group field %q is invalid: %s", fieldKey, reason),
					itemField,
				))
			}
			if seenInGroup[key] {
				diags = append(diags, publishShapeDiagnostic(
					"form_group_field_duplicate",
					fmt.Sprintf("form group field %q is declared more than once in group %q", fieldKey, group.Key),
					itemField,
				))
				continue
			}
			seenInGroup[key] = true
			if owner, exists := fieldGroups[key]; exists && owner != groupID {
				diags = append(diags, publishShapeDiagnostic(
					"form_group_field_conflict",
					fmt.Sprintf("form field %q is assigned to both group %q and group %q", fieldKey, owner, group.Key),
					itemField,
				))
				continue
			}
			fieldGroups[key] = groupID
		}
	}
	return diags
}

// formSchemaRoot decodes a JSON Schema generically and reports whether field
// resolution is meaningful (object schema with at least one top-level
// property). Non-object roots, invalid JSON and property-less schemas are
// intentionally treated as "no field resolution" to avoid false positives.
func formSchemaRoot(raw JSONSchema) (map[string]json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil || len(root) == 0 {
		return nil, false
	}
	if len(formSchemaObject(root["properties"])) == 0 {
		return nil, false
	}
	return root, true
}

func formSchemaObject(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil
	}
	return object
}

// formFieldTokens implements the SUI-019 key contract: a bare key is one
// literal `properties` name (dots included); a leading "/" denotes an RFC 6901
// pointer with ~1/~0 escapes.
func formFieldTokens(key string) []string {
	if !strings.HasPrefix(key, "/") {
		return []string{key}
	}
	parts := strings.Split(strings.TrimPrefix(key, "/"), "/")
	unescape := strings.NewReplacer("~1", "/", "~0", "~")
	tokens := make([]string, len(parts))
	for i, part := range parts {
		tokens[i] = unescape.Replace(part)
	}
	return tokens
}

// resolveFormField walks `properties` and array `items` to find a field
// schema. A non-empty reason means the path could not be resolved.
func resolveFormField(root map[string]json.RawMessage, key string) (map[string]json.RawMessage, string) {
	tokens := formFieldTokens(key)
	if len(tokens) == 0 || tokens[0] == "" {
		return nil, "路径为空"
	}
	current := root
	for i := 0; i < len(tokens)-1; i++ {
		if formSchemaIsArray(current) {
			items := formSchemaObject(current["items"])
			if items == nil {
				return nil, fmt.Sprintf("路径段 %q 在 schema 中没有可解析的 items", tokens[i])
			}
			current = items
			continue
		}
		properties := formSchemaObject(current["properties"])
		raw, exists := properties[tokens[i]]
		if !exists {
			return nil, fmt.Sprintf("路径段 %q 在 schema 中不存在", tokens[i])
		}
		current = formSchemaObject(raw)
		if current == nil {
			// Boolean or otherwise non-object subschemas have no further
			// properties to walk; they are still a valid resolution target.
			current = map[string]json.RawMessage{}
		}
	}
	properties := formSchemaObject(current["properties"])
	last := tokens[len(tokens)-1]
	raw, exists := properties[last]
	if !exists {
		return nil, fmt.Sprintf("字段 %q 不在目标 schema 的 properties 中", last)
	}
	node := formSchemaObject(raw)
	if node == nil {
		node = map[string]json.RawMessage{}
	}
	return node, ""
}

func formSchemaTypes(node map[string]json.RawMessage) []string {
	raw, exists := node["type"]
	if !exists {
		return nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if single == "" {
			return nil
		}
		return []string{single}
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		return many
	}
	return nil
}

func formSchemaIsArray(node map[string]json.RawMessage) bool {
	for _, kind := range formSchemaTypes(node) {
		if kind == "array" {
			return true
		}
	}
	return false
}

func isKnownFormWidget(widget FormWidget) bool {
	switch widget {
	case FormWidgetInput, FormWidgetTextArea, FormWidgetNumber, FormWidgetPassword,
		FormWidgetSelect, FormWidgetMultiSelect, FormWidgetRadio, FormWidgetCheckbox,
		FormWidgetSwitch, FormWidgetDatePicker, FormWidgetTimePicker, FormWidgetDateRange,
		FormWidgetUpload, FormWidgetImageUpload, FormWidgetFileUpload, FormWidgetRichText,
		FormWidgetCode, FormWidgetCascader, FormWidgetTreeSelect, FormWidgetColor,
		FormWidgetSlider, FormWidgetRate, FormWidgetJSON, FormWidgetKeyValue,
		FormWidgetArray, FormWidgetObject:
		return true
	default:
		return false
	}
}

// formWidgetAcceptedTypes mirrors web/src/utils/widgetCapabilities.ts. A
// missing entry means "no compatibility check" (DateRange is declared but not
// implemented; invalid widgets are reported by isKnownFormWidget).
var formWidgetAcceptedTypes = map[FormWidget]map[string]bool{
	FormWidgetInput:       {"string": true},
	FormWidgetTextArea:    {"string": true},
	FormWidgetNumber:      {"number": true, "integer": true},
	FormWidgetPassword:    {"string": true},
	FormWidgetSelect:      {"string": true},
	FormWidgetMultiSelect: {"array": true},
	FormWidgetRadio:       {"string": true},
	FormWidgetCheckbox:    {"boolean": true, "array": true},
	FormWidgetSwitch:      {"boolean": true},
	FormWidgetDatePicker:  {"string": true},
	FormWidgetTimePicker:  {"string": true},
	// DateRange: declared-only, compatibility intentionally skipped.
	FormWidgetUpload:      {"string": true, "array": true},
	FormWidgetImageUpload: {"string": true, "array": true},
	FormWidgetFileUpload:  {"string": true, "array": true},
	FormWidgetRichText:    {"string": true},
	FormWidgetCode:        {"string": true},
	FormWidgetCascader:    {"string": true},
	FormWidgetTreeSelect:  {"string": true, "array": true},
	FormWidgetColor:       {"string": true},
	FormWidgetSlider:      {"number": true, "integer": true},
	FormWidgetRate:        {"integer": true, "number": true},
	FormWidgetJSON:        {"string": true},
	FormWidgetKeyValue:    {"object": true},
	FormWidgetArray:       {"array": true},
	FormWidgetObject:      {"object": true},
}

// widgetTypeCompatible reports whether the widget accepts any of the schema
// types. checkable is false when there is no declared widget rule or when the
// schema node has no recognized type, in which case compatibility is skipped.
func widgetTypeCompatible(widget FormWidget, types []string) (compatible bool, checkable bool) {
	accepted, declared := formWidgetAcceptedTypes[widget]
	if !declared {
		return true, false
	}
	for _, kind := range types {
		switch kind {
		case "string", "number", "integer", "boolean", "array", "object":
			checkable = true
			if accepted[kind] {
				return true, true
			}
		}
	}
	return false, checkable
}
