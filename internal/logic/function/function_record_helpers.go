package function

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
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
	if svcCtx == nil || svcCtx.FunctionModel == nil {
		return nil, errorx.NewInternalError("FunctionModel 未初始化")
	}
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
	// This populates input_schema, resource, risk, version from SDK registration.
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
// lose the input_schema, resource, and other metadata carried by the SDK.
func backfillFromRegistry(svcCtx *svc.ServiceContext, functionID string, fn *model.Function) {
	if svcCtx == nil || svcCtx.RegistryStore == nil {
		return
	}

	// First try to get metadata from the registry store (SDK registration)
	store := svcCtx.RegistryStore
	store.Mu().RLock()
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		if meta, ok := sess.Functions[functionID]; ok {
			// Found the function in the registry store
			if meta.Summary != "" && fn.Description == "" {
				fn.Description = meta.Summary
			}
			if len(meta.Tags) > 0 {
				// Store tags in metadata
				if fn.Metadata == nil {
					fn.Metadata = make(datatypes.JSONMap)
				}
				fn.Metadata["tags"] = meta.Tags
				fn.Metadata["summary"] = meta.Summary
			}
			if meta.Resource != "" && fn.Resource == "" {
				fn.Resource = meta.Resource
			}
			if meta.Version != "" && fn.Version == "" {
				fn.Version = meta.Version
			}
			break
		}
	}
	store.Mu().RUnlock()

	// Also try OpenAPI operations for additional metadata
	operations := store.ListOpenAPIOperations()
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

	// Extract resource from extensions.
	if op.Extensions != nil {
		if resource, ok := op.Extensions["x-resource"]; ok {
			if resourceStr, ok := resource.(string); ok && strings.TrimSpace(resourceStr) != "" {
				fn.Resource = strings.TrimSpace(resourceStr)
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
		"resource", fn.Resource)
}
