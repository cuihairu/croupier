package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
)

func diagnosticsFromJSON(raw []byte) []spec.Diagnostic {
	if len(raw) == 0 {
		return nil
	}
	var diagnostics []spec.Diagnostic
	if err := json.Unmarshal(raw, &diagnostics); err != nil {
		return []spec.Diagnostic{{
			Code:     "proposal_diagnostics_invalid",
			Severity: spec.SeverityWarning,
			Message:  "proposal diagnostics payload is not readable",
		}}
	}
	return diagnostics
}

func hasDiagnosticSeverity(diagnostics []spec.Diagnostic, severity spec.DiagnosticSeverity) bool {
	for _, item := range diagnostics {
		if item.Severity == severity {
			return true
		}
	}
	return false
}

func staleDiagnosticsForDraft(page spec.PageSpec, functions map[string]spec.FunctionSpec) []spec.BindingFreshnessDiagnostic {
	var out []spec.BindingFreshnessDiagnostic
	for _, binding := range page.Bindings {
		functionID := strings.TrimSpace(binding.FunctionID)
		fn, ok := functions[functionID]
		if !ok {
			out = append(out, spec.BindingFreshnessDiagnostic{
				BindingID:  strings.TrimSpace(binding.ID),
				FunctionID: functionID,
				Status:     spec.BindingFreshnessFunctionMissing,
				Diagnostic: spec.Diagnostic{
					Code:       "binding_function_missing",
					Severity:   spec.SeverityError,
					Message:    "bound function no longer exists; update the draft binding before publishing",
					FunctionID: functionID,
					Field:      "bindings." + strings.TrimSpace(binding.ID),
				},
			})
			continue
		}
		if executionModeForFunctionSpec(fn) != binding.Execution.Mode {
			out = append(out, spec.BindingFreshnessDiagnostic{
				BindingID:  strings.TrimSpace(binding.ID),
				FunctionID: functionID,
				Status:     spec.BindingFreshnessExecutionModeStale,
				Diagnostic: spec.Diagnostic{
					Code:       "draft_binding_execution_mode_stale",
					Severity:   spec.SeverityError,
					Message:    "draft binding execution mode no longer matches the latest function contract",
					FunctionID: functionID,
					Field:      "bindings." + strings.TrimSpace(binding.ID) + ".execution.mode",
				},
			})
		}
	}
	return out
}

func stringSliceFromJSON(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	out := values[:0]
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func normalizeJSONMap(values map[string]interface{}) spec.LocalizedText {
	if len(values) == 0 {
		return nil
	}
	out := make(spec.LocalizedText, len(values))
	for key, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			out[key] = strings.TrimSpace(text)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *ProposalService) functionSpecsByID(ctx context.Context, gameID, env string) (map[string]spec.FunctionSpec, error) {
	contracts, err := s.contractModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return nil, fmt.Errorf("list function contracts: %w", err)
	}
	return FunctionSpecsFromContracts(contracts), nil
}

func isMissingTableErr(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table") ||
		strings.Contains(message, "does not exist") ||
		strings.Contains(message, "undefined table")
}

func normalizeLocalizedText(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		switch strings.ToLower(strings.ReplaceAll(k, "_", "-")) {
		case "zh", "zh-cn":
			out["zh-CN"] = v
		case "en", "en-us":
			out["en-US"] = v
		default:
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// hasDefaultLocale 默认名称必填、翻译可选（T12）：任一 locale 非空即过，
// 仅有 en-US 的存量页面不被误拒；全空拒绝。
func hasDefaultLocale(labels map[string]string) bool {
	if labels == nil {
		return false
	}
	for _, value := range labels {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func localizedTextEqual(left map[string]string, right map[string]string) bool {
	left = normalizeLocalizedText(left)
	right = normalizeLocalizedText(right)
	if len(left) != len(right) {
		return false
	}
	for key, leftValue := range left {
		if strings.TrimSpace(right[key]) != strings.TrimSpace(leftValue) {
			return false
		}
	}
	return true
}

func bindingRequiresOutputSelectors(binding spec.PageFunctionBinding, page spec.PageSpec) bool {
	switch binding.Usage {
	case spec.BindingUsageQuery:
		return page.Type == spec.PageTypeResource
	case spec.BindingUsageReport:
		return page.Type == spec.PageTypeReport
	case spec.BindingUsageTaskStatus, spec.BindingUsageTaskEvents, spec.BindingUsageTaskResult:
		return page.Type == spec.PageTypeTask
	default:
		return false
	}
}

func schemaHasFields(raw spec.JSONSchema) bool {
	if len(raw) == 0 {
		return false
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return true
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(parsed["properties"], &properties); err == nil && len(properties) > 0 {
		return true
	}
	var required []string
	if err := json.Unmarshal(parsed["required"], &required); err == nil && len(required) > 0 {
		return true
	}
	return false
}

func actorFromContext(ctx context.Context) string {
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		return "system"
	}
	return actor
}
