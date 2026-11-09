package resourcecatalog

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
)

func determineStatus(contracts []*model.FunctionContract, semantics *model.CapabilitySemantics) string {
	if len(contracts) == 0 {
		return "not_executable"
	}
	if semantics == nil {
		return "pending"
	}
	if countUnresolvedConflicts(semantics.Conflicts) > 0 {
		return "conflict"
	}
	hasCollectionContract := false
	for _, contract := range contracts {
		if contract != nil && contract.Capability == dbenum.CapabilityCollectionQuery {
			hasCollectionContract = true
			break
		}
	}
	if hasCollectionContract && semantics.CollectionQueryID > 0 && strings.TrimSpace(semantics.IdentityField) != "" {
		return "identified"
	}
	return "pending"
}

func buildFunctionInfos(contracts []*model.FunctionContract) []FunctionInfo {
	infos := make([]FunctionInfo, 0, len(contracts))
	for _, c := range contracts {
		infos = append(infos, FunctionInfo{
			ID:         c.ID,
			FunctionID: c.FunctionID,
			Version:    c.Version,
			Capability: c.Capability.String(),
			Execution:  c.Execution,
			Risk:       c.Risk.String(),
			Enabled:    c.Enabled,
			Source:     c.Source,
		})
	}
	return infos
}

func buildSemanticsInfo(semantics *model.CapabilitySemantics) *SemanticsInfo {
	if semantics == nil {
		return nil
	}
	return &SemanticsInfo{
		Version:             semantics.Version,
		HasIdentity:         semantics.IdentityField != "",
		HasCollection:       semantics.CollectionQueryID > 0,
		HasCreate:           semantics.CreateID > 0,
		HasUpdate:           semantics.UpdateID > 0,
		HasDelete:           semantics.DeleteID > 0,
		HasActions:          len(parseActionSemantics(semantics.Actions)) > 0,
		HasTasks:            len(parseTaskSemantics(semantics.Tasks)) > 0,
		HasReports:          len(parseReportSemantics(semantics.Reports)) > 0,
		Source:              semantics.Source,
		SourceDigest:        semantics.SourceDigest,
		IdentityField:       semantics.IdentityField,
		IdentityFieldType:   semantics.IdentityFieldType,
		IdentityPath:        semantics.IdentityPath,
		CollectionQueryID:   semantics.CollectionQueryID,
		CollectionPath:      semantics.CollectionPath,
		PageFieldName:       semantics.PageFieldName,
		PageSizeFieldName:   semantics.PageSizeFieldName,
		ItemsFieldName:      semantics.ItemsFieldName,
		TotalFieldName:      semantics.TotalFieldName,
		ItemQueryID:         semantics.ItemQueryID,
		ItemPath:            semantics.ItemPath,
		CreateID:            semantics.CreateID,
		UpdateID:            semantics.UpdateID,
		DeleteID:            semantics.DeleteID,
		Actions:             parseActionSemantics(semantics.Actions),
		Tasks:               parseTaskSemantics(semantics.Tasks),
		Reports:             parseReportSemantics(semantics.Reports),
		UnresolvedConflicts: countUnresolvedConflicts(semantics.Conflicts),
	}
}

func buildDiagnostics(contracts []*model.FunctionContract, semantics *model.CapabilitySemantics) []DiagnosticInfo {
	diagnostics := make([]DiagnosticInfo, 0)
	if semantics != nil {
		diagnostics = append(diagnostics, decodeDiagnostics(semantics.Diagnostics, "")...)
		if unresolved := countUnresolvedConflicts(semantics.Conflicts); unresolved > 0 {
			diagnostics = append(diagnostics, DiagnosticInfo{
				Code:     "semantic_conflict",
				Severity: "error",
				Message:  fmt.Sprintf("%d semantic conflict(s) require platform review", unresolved),
			})
		}
	}
	for _, contract := range contracts {
		if contract == nil {
			continue
		}
		diagnostics = append(diagnostics, decodeDiagnostics(contract.Diagnostics, contract.FunctionID)...)
	}
	return diagnostics
}

func compactJSONPointers(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		pointer := strings.TrimSpace(value)
		if pointer == "" || !strings.HasPrefix(pointer, "/") || !isJSONPointer(pointer) {
			continue
		}
		if _, ok := seen[pointer]; ok {
			continue
		}
		seen[pointer] = struct{}{}
		out = append(out, pointer)
	}
	return out
}

func parseActionSemantics(raw []byte) []ActionSemanticInfo {
	if len(raw) == 0 {
		return nil
	}
	var actions []ActionSemanticInfo
	if err := json.Unmarshal(raw, &actions); err != nil {
		return nil
	}
	return compactActionSemantics(actions)
}

func parseTaskSemantics(raw []byte) []spec.TaskSemantic {
	if len(raw) == 0 {
		return nil
	}
	var tasks []spec.TaskSemantic
	if err := json.Unmarshal(raw, &tasks); err != nil {
		return nil
	}
	out := make([]spec.TaskSemantic, 0, len(tasks))
	for _, task := range tasks {
		if strings.TrimSpace(task.Start.FunctionID) == "" {
			continue
		}
		out = append(out, task)
	}
	return out
}

func parseReportSemantics(raw []byte) []spec.ReportSemantic {
	if len(raw) == 0 {
		return nil
	}
	var reports []spec.ReportSemantic
	if err := json.Unmarshal(raw, &reports); err != nil {
		return nil
	}
	out := make([]spec.ReportSemantic, 0, len(reports))
	for _, report := range reports {
		if strings.TrimSpace(report.Query.FunctionID) == "" {
			continue
		}
		report.Dimensions = compactJSONPointers(report.Dimensions)
		report.Metrics = compactJSONPointers(report.Metrics)
		out = append(out, report)
	}
	return out
}

func compactActionSemantics(actions []ActionSemanticInfo) []ActionSemanticInfo {
	out := make([]ActionSemanticInfo, 0, len(actions))
	for _, action := range actions {
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

func isJSONPointer(path string) bool {
	return path == "" || strings.HasPrefix(strings.TrimSpace(path), "/")
}

func schemaHasPointer(raw []byte, pointer string) bool {
	if len(raw) == 0 {
		return true
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return true
	}
	return schemaObjectHasPointer(root, pointer)
}

func schemaObjectHasPointer(root map[string]json.RawMessage, pointer string) bool {
	if !isJSONPointer(pointer) {
		return false
	}
	if pointer == "" {
		return true
	}
	current := root
	for _, token := range jsonPointerTokens(pointer) {
		properties := parseRawObject(current["properties"])
		if len(properties) == 0 {
			return false
		}
		next := parseRawObject(properties[token])
		if len(next) == 0 {
			return false
		}
		current = next
	}
	return true
}

func arrayItemSchemaAtPointer(raw []byte, pointer string) (map[string]json.RawMessage, bool) {
	if len(raw) == 0 || !isJSONPointer(pointer) {
		return nil, false
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, false
	}
	node := root
	if pointer != "" {
		for _, token := range jsonPointerTokens(pointer) {
			properties := parseRawObject(node["properties"])
			if len(properties) == 0 {
				return nil, false
			}
			node = parseRawObject(properties[token])
			if len(node) == 0 {
				return nil, false
			}
		}
	}
	if schemaStringValue(node["type"]) != "array" {
		return nil, false
	}
	items := parseRawObject(node["items"])
	return items, len(items) > 0
}

func parseRawObject(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func schemaStringValue(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return value
}

func jsonPointerTokens(path string) []string {
	if path == "" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts
}

// digestRawJSON 计算资源语义 functionRef 的 schema digest。与发布/评估链
// 共用 freshness.CanonicalDigest，避免 raw 字节哈希因键序/空格形态不同而
// 与其他 binding digest 比对恒不等（同 api/page digestRaw 2026-09-19 修复）。
func digestRawJSON(raw []byte) string {
	return freshness.CanonicalDigest(raw)
}
