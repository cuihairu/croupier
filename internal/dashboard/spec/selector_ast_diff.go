package spec

import (
	"encoding/json"
	"strings"
)

// SchemaFieldSnapshot is a comparable field view extracted from the supported
// JSON Schema subset.
type SchemaFieldSnapshot struct {
	Path     string `json:"path"`
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required,omitempty"`
}

// SchemaFieldChangeType describes a field-level schema change.
type SchemaFieldChangeType string

const (
	SchemaFieldAdded           SchemaFieldChangeType = "added"
	SchemaFieldRemoved         SchemaFieldChangeType = "removed"
	SchemaFieldTypeChanged     SchemaFieldChangeType = "type_changed"
	SchemaFieldRequiredChanged SchemaFieldChangeType = "required_changed"
)

// SchemaFieldChange is a deterministic field-level schema diff item.
type SchemaFieldChange struct {
	Path       string                `json:"path"`
	ChangeType SchemaFieldChangeType `json:"changeType"`
	Old        *SchemaFieldSnapshot  `json:"old,omitempty"`
	New        *SchemaFieldSnapshot  `json:"new,omitempty"`
}

// FieldRenameCandidate is a conservative rename hint for Page Studio.
type FieldRenameCandidate struct {
	OldPath    string `json:"oldPath"`
	NewPath    string `json:"newPath"`
	Confidence string `json:"confidence"`
	Reason     string `json:"reason"`
}

// SchemaDiffResult contains supported-subset schema diff output.
type SchemaDiffResult struct {
	Changes          []SchemaFieldChange    `json:"changes"`
	RenameCandidates []FieldRenameCandidate `json:"renameCandidates,omitempty"`
	Diagnostics      []Diagnostic           `json:"diagnostics,omitempty"`
}

// DiffJSONSchemaFields compares the supported object/property subset of JSON Schema.
func DiffJSONSchemaFields(oldSchema JSONSchema, newSchema JSONSchema) SchemaDiffResult {
	oldFields, oldOK := schemaFieldSnapshots(oldSchema)
	newFields, newOK := schemaFieldSnapshots(newSchema)
	result := SchemaDiffResult{}
	if !oldOK || !newOK {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Code:     "schema_diff_invalid_schema",
			Severity: SeverityError,
			Message:  "schema diff requires valid JSON Schema objects",
		})
		return result
	}

	allPaths := make(map[string]struct{}, len(oldFields)+len(newFields))
	for path := range oldFields {
		allPaths[path] = struct{}{}
	}
	for path := range newFields {
		allPaths[path] = struct{}{}
	}
	paths := sortedMapKeys(allPaths)

	var removed []SchemaFieldSnapshot
	var added []SchemaFieldSnapshot
	for _, path := range paths {
		oldField, oldExists := oldFields[path]
		newField, newExists := newFields[path]
		switch {
		case !oldExists && newExists:
			newCopy := newField
			added = append(added, newField)
			result.Changes = append(result.Changes, SchemaFieldChange{
				Path:       path,
				ChangeType: SchemaFieldAdded,
				New:        &newCopy,
			})
		case oldExists && !newExists:
			oldCopy := oldField
			removed = append(removed, oldField)
			result.Changes = append(result.Changes, SchemaFieldChange{
				Path:       path,
				ChangeType: SchemaFieldRemoved,
				Old:        &oldCopy,
			})
		case oldField.Type != newField.Type:
			oldCopy := oldField
			newCopy := newField
			result.Changes = append(result.Changes, SchemaFieldChange{
				Path:       path,
				ChangeType: SchemaFieldTypeChanged,
				Old:        &oldCopy,
				New:        &newCopy,
			})
		case oldField.Required != newField.Required:
			oldCopy := oldField
			newCopy := newField
			result.Changes = append(result.Changes, SchemaFieldChange{
				Path:       path,
				ChangeType: SchemaFieldRequiredChanged,
				Old:        &oldCopy,
				New:        &newCopy,
			})
		}
	}
	result.RenameCandidates = fieldRenameCandidates(removed, added)
	return result
}

// SelectorStaleDiagnostics explains how selector paths are affected by schema changes.
func SelectorStaleDiagnostics(
	selector SelectorAST,
	outputAssignments []OutputAssignment,
	oldInputSchema JSONSchema,
	newInputSchema JSONSchema,
	oldOutputSchema JSONSchema,
	newOutputSchema JSONSchema,
) []Diagnostic {
	inputDiff := DiffJSONSchemaFields(oldInputSchema, newInputSchema)
	outputDiff := DiffJSONSchemaFields(oldOutputSchema, newOutputSchema)
	diags := append([]Diagnostic{}, inputDiff.Diagnostics...)
	diags = append(diags, outputDiff.Diagnostics...)
	if len(inputDiff.Diagnostics) > 0 || len(outputDiff.Diagnostics) > 0 {
		return diags
	}

	assignedTargets := map[string]struct{}{}
	for _, assignment := range selector.Assignments {
		assignedTargets[assignment.Target] = struct{}{}
		if !schemaHasPath(newInputSchema, assignment.Target) {
			diags = append(diags, staleDiagnostic("selector_target_stale", "selector target no longer exists in input schema", "input."+assignment.Target))
			continue
		}
		if hasChange(inputDiff.Changes, assignment.Target, SchemaFieldTypeChanged) {
			diags = append(diags, staleDiagnostic("selector_target_type_stale", "selector target type changed in input schema", "input."+assignment.Target))
		}
	}

	required, err := requiredPointers(newInputSchema)
	if err == nil {
		for _, path := range sortedMapKeys(required) {
			if _, ok := assignedTargets[path]; !ok {
				diags = append(diags, staleDiagnostic("selector_required_stale", "new required input field is not assigned", "input."+path))
			}
		}
	}

	for _, assignment := range outputAssignments {
		if !schemaHasPath(newOutputSchema, assignment.Source) {
			diags = append(diags, staleDiagnostic("selector_output_source_stale", "output source no longer exists in output schema", "output."+assignment.Source))
			continue
		}
		if hasChange(outputDiff.Changes, assignment.Source, SchemaFieldTypeChanged) {
			diags = append(diags, staleDiagnostic("selector_output_type_stale", "output source type changed in output schema", "output."+assignment.Source))
		}
	}

	for _, candidate := range inputDiff.RenameCandidates {
		if _, ok := assignedTargets[candidate.OldPath]; ok {
			diags = append(diags, Diagnostic{
				Code:     "selector_field_rename_candidate",
				Severity: SeverityInfo,
				Message:  "input selector may be updated from " + candidate.OldPath + " to " + candidate.NewPath,
				Field:    "input." + candidate.OldPath,
			})
		}
	}
	for _, candidate := range outputDiff.RenameCandidates {
		for _, assignment := range outputAssignments {
			if assignment.Source == candidate.OldPath {
				diags = append(diags, Diagnostic{
					Code:     "selector_output_rename_candidate",
					Severity: SeverityInfo,
					Message:  "output selector may be updated from " + candidate.OldPath + " to " + candidate.NewPath,
					Field:    "output." + candidate.OldPath,
				})
			}
		}
	}
	return diags
}

func rawObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, false
	}
	return out, true
}

func schemaFieldSnapshots(schema JSONSchema) (map[string]SchemaFieldSnapshot, bool) {
	if len(schema) == 0 {
		return map[string]SchemaFieldSnapshot{}, true
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(schema, &root); err != nil {
		return nil, false
	}
	out := map[string]SchemaFieldSnapshot{}
	collectSchemaFieldSnapshots("", root, requiredSet(root), out)
	return out, true
}

func collectSchemaFieldSnapshots(prefix string, node map[string]json.RawMessage, required map[string]struct{}, out map[string]SchemaFieldSnapshot) {
	properties, ok := rawObject(node["properties"])
	if !ok {
		return
	}
	for _, name := range sortedRawMapKeys(properties) {
		propRaw := properties[name]
		prop, ok := rawObject(propRaw)
		if !ok {
			continue
		}
		path := prefix + "/" + escapeJSONPointerToken(name)
		_, isRequired := required[name]
		out[path] = SchemaFieldSnapshot{
			Path:     path,
			Type:     schemaTypeFromNode(prop),
			Required: isRequired,
		}
		collectSchemaFieldSnapshots(path, prop, requiredSet(prop), out)
	}
}

func requiredSet(node map[string]json.RawMessage) map[string]struct{} {
	out := map[string]struct{}{}
	var required []string
	if raw := node["required"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &required); err == nil {
			for _, field := range required {
				out[field] = struct{}{}
			}
		}
	}
	return out
}

func schemaTypeFromNode(node map[string]json.RawMessage) string {
	rawType, ok := node["type"]
	if !ok {
		return ""
	}
	var schemaType string
	if err := json.Unmarshal(rawType, &schemaType); err == nil {
		return schemaType
	}
	var schemaTypes []string
	if err := json.Unmarshal(rawType, &schemaTypes); err == nil && len(schemaTypes) == 1 {
		return schemaTypes[0]
	}
	return ""
}

func fieldRenameCandidates(removed []SchemaFieldSnapshot, added []SchemaFieldSnapshot) []FieldRenameCandidate {
	candidates := []FieldRenameCandidate{}
	for _, oldField := range removed {
		for _, newField := range added {
			if oldField.Type == "" || oldField.Type != newField.Type {
				continue
			}
			if oldField.Required != newField.Required {
				continue
			}
			if parentPointer(oldField.Path) != parentPointer(newField.Path) {
				continue
			}
			candidates = append(candidates, FieldRenameCandidate{
				OldPath:    oldField.Path,
				NewPath:    newField.Path,
				Confidence: "low",
				Reason:     "same parent, type, and required flag",
			})
		}
	}
	return candidates
}

func hasChange(changes []SchemaFieldChange, path string, changeType SchemaFieldChangeType) bool {
	for _, change := range changes {
		if change.Path == path && change.ChangeType == changeType {
			return true
		}
	}
	return false
}

func staleDiagnostic(code string, message string, field string) Diagnostic {
	return Diagnostic{
		Code:     code,
		Severity: SeverityError,
		Message:  message,
		Field:    field,
	}
}

func parentPointer(path string) string {
	if path == "" {
		return ""
	}
	index := strings.LastIndex(path, "/")
	if index <= 0 {
		return ""
	}
	return path[:index]
}
