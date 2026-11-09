package service

import (
	"context"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

func (s *ProposalService) buildBindingContracts(ctx context.Context, gameID, env string, bindings []spec.PageFunctionBinding) ([]spec.BindingContractSnapshot, error) {
	out := make([]spec.BindingContractSnapshot, 0, len(bindings))
	for _, binding := range bindings {
		functionID := strings.TrimSpace(binding.FunctionID)
		if functionID == "" {
			return nil, errorx.NewValidationError("binding.functionId is required")
		}
		contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
		if err != nil {
			return nil, errorx.NewValidationError("bound function contract does not exist: " + functionID)
		}
		if !contract.Enabled {
			return nil, errorx.NewValidationError("bound function is disabled: " + functionID)
		}
		out = append(out, spec.BindingContractSnapshot{
			BindingID:             strings.TrimSpace(binding.ID),
			FunctionID:            functionID,
			FunctionVersion:       strings.TrimSpace(contract.Version),
			InputSchemaDigest:     digestJSON(contract.InputSchema),
			OutputSchemaDigest:    digestJSON(contract.OutputSchema),
			Risk:                  spec.RiskLevel(contract.Risk.String()),
			Permission:            strings.TrimSpace(contract.Permission),
			Approval:              ApprovalPolicyFromJSONMap(contract.Approval),
			ExecutionMode:         binding.Execution.Mode,
			RendererSchemaVersion: rendererSchemaVersion,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BindingID < out[j].BindingID })
	return out, nil
}

func currentFunctionDigest(ctx context.Context, contractModel *model.FunctionContractModel, gameID, env string, page spec.PageSpec) (string, error) {
	contracts, err := contractModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return "", err
	}
	selectedContracts := make([]*model.FunctionContract, 0, len(page.Bindings))
	bindingFunctions := make(map[string]struct{}, len(page.Bindings))
	for _, binding := range page.Bindings {
		if functionID := strings.TrimSpace(binding.FunctionID); functionID != "" {
			bindingFunctions[functionID] = struct{}{}
		}
	}
	for _, contract := range contracts {
		if contract == nil {
			continue
		}
		if _, wanted := bindingFunctions[strings.TrimSpace(contract.FunctionID)]; !wanted {
			continue
		}
		selectedContracts = append(selectedContracts, contract)
	}
	return computeDigest(selectedContracts), nil
}

func digestJSON(raw model.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	return freshness.CanonicalDigest(raw)
}
