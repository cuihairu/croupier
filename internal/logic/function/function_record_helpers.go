package function

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/datatypes"
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

	// Backfill metadata from runtime registry if available.
	// This populates input_schema, category, risk, version from SDK registration.
	backfillFromRegistry(svcCtx, functionID, created)

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
			slog.Default().Info("Created default policy for function",
				"function_id", functionID,
				"risk_level", riskLevel)
		}
	}

	return created, nil
}

// backfillFromRegistry populates a new function record with metadata from the
// runtime registry (SDK registration). This avoids creating empty records that
// lose the input_schema, category, and other metadata carried by the SDK.
func backfillFromRegistry(svcCtx *svc.ServiceContext, functionID string, fn *model.Function) {
	if svcCtx == nil || svcCtx.RegistryStore == nil {
		return
	}

	operations := svcCtx.RegistryStore.ListOpenAPIOperations()
	op, ok := operations[functionID]
	if !ok || op == nil {
		return
	}

	// Extract OpenAPI operation as JSON for storage
	opJSON, err := op.MarshalJSON()
	if err == nil {
		var opMap map[string]interface{}
		if json.Unmarshal(opJSON, &opMap) == nil {
			fn.OpenAPISpec = datatypes.JSONMap(opMap)
			fn.SpecFormat = "openapi3.0.3"
		}
	}

	// Extract category from extensions
	if op.Extensions != nil {
		if cat, ok := op.Extensions["x-category"]; ok {
			if catStr, ok := cat.(string); ok && strings.TrimSpace(catStr) != "" {
				fn.Category = strings.TrimSpace(catStr)
			}
		}
	}

	// Extract version from extensions
	if op.Extensions != nil {
		if ver, ok := op.Extensions["x-version"]; ok {
			if verStr, ok := ver.(string); ok && strings.TrimSpace(verStr) != "" {
				fn.Version = strings.TrimSpace(verStr)
			}
		}
	}

	// Use summary as description if available
	if op.Summary != "" && fn.Description == "" {
		fn.Description = op.Summary
	}

	slog.Default().Debug("Backfilled function metadata from registry",
		"function_id", functionID,
		"has_openapi_spec", fn.OpenAPISpec != nil,
		"category", fn.Category)
}
