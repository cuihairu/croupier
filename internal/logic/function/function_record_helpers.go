package function

import (
	"context"
	"errors"
	"log/slog"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

func getOrCreateFunctionRecord(ctx context.Context, svcCtx *svc.ServiceContext, functionID string) (*model.Function, error) {
	return getOrCreateFunctionRecordWithRisk(ctx, svcCtx, functionID, "")
}

func getOrCreateFunctionRecordWithRisk(ctx context.Context, svcCtx *svc.ServiceContext, functionID string, riskLevel string) (*model.Function, error) {
	fn, err := svcCtx.FunctionModel.FindByFunctionID(ctx, functionID)
	if err == nil {
		return fn, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	created := &model.Function{
		FunctionID: functionID,
		Name:       functionID,
		Status:     1,
	}
	if createErr := svcCtx.FunctionModel.Create(ctx, created); createErr != nil {
		if errors.Is(createErr, gorm.ErrDuplicatedKey) {
			return svcCtx.FunctionModel.FindByFunctionID(ctx, functionID)
		}
		return nil, createErr
	}

	// Create default policy for new function
	if svcCtx.PolicyManager != nil {
		// Default to medium risk if not specified
		if riskLevel == "" {
			riskLevel = string(policy.RiskMedium)
		}
		if policyErr := svcCtx.PolicyManager.EnsureDefaultPolicy(ctx, functionID, policy.RiskLevel(riskLevel)); policyErr != nil {
			slog.Default().Warn("Failed to create default policy for function",
				"function_id", functionID,
				"risk_level", riskLevel,
				"error", policyErr)
		} else {
			slog.Default().Info("Created default policy for new function",
				"function_id", functionID,
				"risk_level", riskLevel)
		}
	}

	return created, nil
}
