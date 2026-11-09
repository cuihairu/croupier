package generator

import (
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

func findResourceContract(contracts []*model.FunctionContract, id uint, capability spec.CapabilityKind) *model.FunctionContract {
	if id == 0 {
		return nil
	}
	contract := findContractByID(contracts, id)
	if contract != nil && spec.CapabilityKind(contract.Capability.String()) == capability {
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
		Capability:   spec.CapabilityKind(c.Capability.String()),
		Execution:    spec.FunctionExecution(c.Execution),
		Approval:     jsonMapToApprovalPolicy(c.Approval),
		Risk:         spec.RiskLevel(c.Risk.String()),
		Permission:   c.Permission,
	}
}
