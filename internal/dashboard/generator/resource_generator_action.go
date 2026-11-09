package generator

import (
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

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
	return spec.LocalizedText{locale: fallbackLabel(firstNonEmpty(contract.OperationKey, contract.FunctionID))}
}

func inlineActionType(contract *model.FunctionContract) string {
	switch spec.RiskLevel(contract.Risk.String()) {
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
	switch spec.RiskLevel(contract.Risk.String()) {
	case spec.RiskHigh, spec.RiskDanger:
		return true
	default:
		return false
	}
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
		Risk:        contract.Risk.String(),
	}
}

func resourceOperationRequiresConfirmation(contract *model.FunctionContract) bool {
	if contract == nil {
		return false
	}
	if jsonMapToApprovalPolicy(contract.Approval).Required {
		return true
	}
	switch spec.RiskLevel(contract.Risk.String()) {
	case spec.RiskHigh, spec.RiskDanger:
		return true
	default:
		return false
	}
}
