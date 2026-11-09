package generator

import (
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// InferPageType returns the strongest page shape supported by capability
// semantics. It never reads function names.
func InferPageType(ops []spec.OperationSpec) spec.PageType {
	for _, op := range ops {
		if isReportOperation(op) {
			return spec.PageTypeReport
		}
	}
	for _, op := range ops {
		if isTaskOperation(op) {
			return spec.PageTypeTask
		}
	}
	return spec.PageTypeOperation
}

// InferCategoryFromKey extracts category from a key.
func InferCategoryFromKey(key string) string {
	key = strings.TrimSpace(key)
	if idx := strings.Index(key, "."); idx > 0 {
		return key[:idx]
	}
	return key
}

func diagnostic(code string, severity spec.DiagnosticSeverity, message string, functionID string, field string) spec.Diagnostic {
	return spec.Diagnostic{
		Code:       code,
		Severity:   severity,
		Message:    message,
		FunctionID: functionID,
		Field:      field,
	}
}

// ShouldBlockProposal checks if a proposal should be blocked (cannot be materialized).
// Returns true if the proposal has blocking issues that prevent materialization.
func ShouldBlockProposal(diags []spec.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			// Check for specific blocking conditions
			switch d.Code {
			case "function_id_missing", "function_disabled":
				return true
			}
		}
	}
	return false
}

// CreateBlockedProposalIssue creates a BlockedProposalIssue from diagnostics.
// It only contains diagnostics and repair hints, not a spec.
func CreateBlockedProposalIssue(
	gameID string,
	env string,
	resourceKey string,
	functionID string,
	diags []spec.Diagnostic,
) spec.BlockedProposalIssue {
	// Filter to only error diagnostics
	var errorDiags []spec.Diagnostic
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			errorDiags = append(errorDiags, d)
		}
	}

	// Generate repair hint based on diagnostics
	repairHint := generateRepairHint(errorDiags)

	return spec.BlockedProposalIssue{
		GameID:      gameID,
		Env:         env,
		ResourceKey: resourceKey,
		FunctionID:  functionID,
		Diagnostics: errorDiags,
		RepairHint:  repairHint,
		Status:      "open",
	}
}

// generateRepairHint creates a bilingual repair hint from diagnostics.
// Keys follow the LocalizedText contract (zh-CN/en-US literal keys);
// single-language diagnostic messages expand to both locales (same
// normalization as hintLocalized for bare strings).
func generateRepairHint(diags []spec.Diagnostic) spec.LocalizedText {
	if len(diags) == 0 {
		return spec.LocalizedText{"zh-CN": "未发现问题", "en-US": "No issues found"}
	}

	// Use the first error as the primary hint
	primaryDiag := diags[0]
	switch primaryDiag.Code {
	case "function_id_missing":
		return spec.LocalizedText{
			"zh-CN": "缺少函数 ID，请先注册函数。",
			"en-US": "Function ID is required. Please register the function first.",
		}
	case "function_disabled":
		return spec.LocalizedText{
			"zh-CN": "函数已禁用，创建页面前请先启用。",
			"en-US": "Function is disabled. Please enable it before creating a page.",
		}
	default:
		return spec.LocalizedText{"zh-CN": primaryDiag.Message, "en-US": primaryDiag.Message}
	}
}
