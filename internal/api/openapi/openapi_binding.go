package openapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	versionutil "github.com/cuihairu/croupier/internal/common/version"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	funcopenapi "github.com/cuihairu/croupier/internal/function/openapi"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/getkin/kin-openapi/openapi3"
	"gorm.io/gorm"
)

func (s *Service) rebuildContractsForSourceBindings(
	ctx context.Context,
	gameID string,
	env string,
	source *model.OpenAPISource,
	operations []OpenAPISourceOperation,
	bindings []model.OpenAPISourceBinding,
) error {
	for _, binding := range bindings {
		if strings.TrimSpace(binding.Kind) != "provider" {
			continue
		}
		operation, ok := findSourceOperation(operations, binding.OperationID)
		if !ok {
			continue
		}
		runtimeMeta, ok := registeredFunctionMetaInScope(s.svcCtx, gameID, env, binding.FunctionID)
		if !ok {
			return errorx.NewBadRequest("bound functionId is not registered in current game/env runtime: " + strings.TrimSpace(binding.FunctionID))
		}
		if err := s.rebuildContractForSourceBinding(ctx, gameID, env, source, operation, &binding, runtimeMeta); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) rebuildContractForSourceBinding(
	ctx context.Context,
	gameID string,
	env string,
	source *model.OpenAPISource,
	operation OpenAPISourceOperation,
	binding *model.OpenAPISourceBinding,
	runtimeMeta reg.FunctionMeta,
) error {
	if s == nil || s.svcCtx == nil || s.svcCtx.DB == nil || binding == nil {
		return errorx.NewBadRequest("OpenAPI contract rebuild is not initialized")
	}
	functionID := strings.TrimSpace(binding.FunctionID)
	if functionID == "" {
		return errorx.NewBadRequest("functionId is required for OpenAPI contract rebuild")
	}
	// 设计债清理：functionMetaInputForBinding 为纯组装函数（唯一出口无
	// 出错路径），error 返回值已随签名收紧删除。
	meta := s.functionMetaInputForBinding(source, operation, functionID, runtimeMeta)

	slog.Info("rebuilding contract for source binding",
		"game_id", gameID,
		"env", env,
		"function_id", functionID,
		"resource", meta.Resource,
		"capability", meta.Capability,
		"operation", meta.Operation)

	contractService := dashboardservice.NewContractService(s.svcCtx.DB)
	if err := contractService.RebuildContractFromFunctionMeta(ctx, gameID, env, "openapi", meta); err != nil {
		return fmt.Errorf("rebuild OpenAPI function contract %s: %w", functionID, err)
	}
	if resourceKey := strings.TrimSpace(meta.Resource); resourceKey != "" {
		if err := contractService.RebuildResourceCapability(ctx, gameID, env, resourceKey); err != nil {
			return fmt.Errorf("rebuild OpenAPI resource capability %s: %w", resourceKey, err)
		}
		if err := contractService.RebuildProposalsForResource(ctx, gameID, env, resourceKey); err != nil {
			return fmt.Errorf("rebuild OpenAPI page proposals %s: %w", resourceKey, err)
		}
		return nil
	}
	if err := contractService.RebuildProposalForFunction(ctx, gameID, env, functionID); err != nil {
		return fmt.Errorf("rebuild OpenAPI standalone page proposal %s: %w", functionID, err)
	}
	return nil
}

// functionMetaInputForBinding 以 OpenAPI 操作为主、运行时元数据兜底组装
// 契约重建入参。设计债清理：纯组装无出错路径，error 返回值已随签名收紧删除。
func (s *Service) functionMetaInputForBinding(
	source *model.OpenAPISource,
	operation OpenAPISourceOperation,
	functionID string,
	runtimeMeta reg.FunctionMeta,
) dashboardservice.FunctionMetaInput {
	openAPIOp := openAPIOperationFromSource(source, operation.OperationID)
	inputSchema := openAPIRequestSchema(openAPIOp)
	if inputSchema == "" {
		inputSchema = strings.TrimSpace(runtimeMeta.InputSchema)
	}
	outputSchema := openAPIResponseSchema(openAPIOp)
	if outputSchema == "" {
		outputSchema = strings.TrimSpace(runtimeMeta.OutputSchema)
	}
	tags := append([]string(nil), operation.Tags...)
	if len(tags) == 0 {
		tags = append([]string(nil), runtimeMeta.Tags...)
	}
	return dashboardservice.FunctionMetaInput{
		ID:                strings.TrimSpace(functionID),
		Version:           versionutil.ValidOrDefault(runtimeMeta.Version, sourceInfoVersion(source)),
		Enabled:           runtimeMeta.Enabled,
		Deprecated:        runtimeMeta.Deprecated,
		Summary:           firstNonEmpty(operation.Summary, runtimeMeta.Summary, operation.OperationID),
		Description:       firstNonEmpty(operation.Description, runtimeMeta.Description),
		InputSchema:       inputSchema,
		OutputSchema:      outputSchema,
		Resource:          firstNonEmpty(operation.Resource, runtimeMeta.Resource),
		Operation:         firstNonEmpty(operation.Operation, runtimeMeta.Operation),
		Capability:        firstNonEmpty(string(operation.Capability), runtimeMeta.Capability),
		Execution:         firstNonEmpty(string(operation.Execution), runtimeMeta.Execution),
		ApprovalRequired:  operation.Approval.Required,
		ApprovalPolicyKey: operation.Approval.PolicyKey,
		Risk:              firstNonEmpty(string(operation.Risk), runtimeMeta.Risk),
		Permission:        firstNonEmpty(operation.Permission, runtimeMeta.Permission),
		Tags:              tags,
		TimeoutMs:         operation.TimeoutMs,
	}
}

// createUnboundContractsForSource 为无运行时函数的 operation 生成 unbound
// 契约（D1/D4、T4）：functionId 用 deriveFunctionID 同一确定性映射
// （operationId 优先，否则 path 段拼接）——运行时后续注册同名函数时才能
// 自动绑定（D3）。已有 provider 绑定的 operation 走既有 bound 重建路径，
// 此处跳过；已存在契约（bound 或 unbound）不覆盖不降级（幂等）。
// 返回本次新建契约的元信息（供 T5 提案联动与摘要计数）。
func (s *Service) createUnboundContractsForSource(
	ctx context.Context,
	gameID string,
	env string,
	source *model.OpenAPISource,
	operations []OpenAPISourceOperation,
	bindings []model.OpenAPISourceBinding,
) ([]dashboardservice.FunctionMetaInput, error) {
	boundOperations := make(map[string]bool, len(bindings))
	for _, binding := range bindings {
		if strings.TrimSpace(binding.Kind) == "provider" {
			boundOperations[strings.TrimSpace(binding.OperationID)] = true
		}
	}
	operationsByID := openAPIOperationsByID(source.GetSpec())
	contractService := dashboardservice.NewContractService(s.svcCtx.DB)
	created := make([]dashboardservice.FunctionMetaInput, 0)
	for _, operation := range operations {
		operationID := strings.TrimSpace(operation.OperationID)
		if operationID == "" {
			continue
		}
		if boundOperations[operationID] {
			// 该操作已显式绑定运行时函数（bound 契约落在绑定函数名下）：
			// 先前上传在 operationId 名下生成的 unbound 物料已被取代，
			// 重放时清理，避免组件面板出现永不可执行的重复物料。
			supersededID := unboundFunctionID(funcopenapi.DeriveFunctionID(operationsByID[operationID], operation.Path))
			if supersededID != "" {
				if err := s.removeSupersededUnboundContract(ctx, contractService, gameID, env, supersededID); err != nil {
					return created, err
				}
			}
			continue
		}
		openAPIOp := operationsByID[operationID]
		functionID := unboundFunctionID(funcopenapi.DeriveFunctionID(openAPIOp, operation.Path))
		if functionID == "" {
			continue
		}
		meta := s.functionMetaInputForOperation(source, operation, openAPIOp, functionID)
		createdNew, err := contractService.CreateUnboundContract(ctx, gameID, env, "openapi", meta)
		if err != nil {
			return created, fmt.Errorf("create unbound contract for operation %s: %w", operationID, err)
		}
		if createdNew {
			created = append(created, meta)
		}
	}
	if len(created) > 0 {
		slog.InfoContext(ctx, "created unbound contracts from OpenAPI source",
			"game_id", gameID,
			"env", env,
			"source_id", source.SourceID,
			"contracts_created", len(created),
			"operation_count", len(operations))
	}
	return created, nil
}

// unboundFunctionID 把确定性映射结果归一到平台 functionId 字符集
// （^[a-z0-9][a-z0-9._-]*$）：小写折叠，非 [a-z0-9._-] 字符替换为 "-"，
// 去除首尾的 "."/"-"/"_"，首字符仍非小写字母/数字则加 "fn-" 前缀。
// OpenAPI 文档常见 camelCase operationId（listPlayers → listplayers）；
// 运行时注册的 functionId 受同一字符集约束（恒小写），归一后 D3 的
// 精确匹配语义不变。同文档两个 operationId 归一后冲突时后者被存在性
// 检查幂等跳过（一料一契约）。
func unboundFunctionID(derived string) string {
	lowered := strings.ToLower(strings.TrimSpace(derived))
	var b strings.Builder
	for _, r := range lowered {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('-')
	}
	out := strings.Trim(b.String(), ".-_")
	if out == "" {
		return ""
	}
	if (out[0] >= 'a' && out[0] <= 'z') || (out[0] >= '0' && out[0] <= '9') {
		return out
	}
	return "fn-" + out
}

// removeSupersededUnboundContract 删除已被 provider 绑定取代的 operationId
// unbound 契约（仅限 openapi 来源的 unbound 行——bound 行与 sdk 来源行不动；
// 绑定函数与 operationId 同名时 bound 契约就在本名下，同样不删）。
func (s *Service) removeSupersededUnboundContract(
	ctx context.Context,
	contractService *dashboardservice.ContractService,
	gameID, env, functionID string,
) error {
	existing, err := contractService.GetContract(ctx, gameID, env, functionID)
	if err != nil || existing == nil {
		return nil // 不存在即无事可做；查询失败不阻断（下次重放再清）
	}
	if existing.ExecutionState != string(spec.ExecutionStateUnbound) || strings.TrimSpace(existing.Source) != "openapi" {
		return nil
	}
	if _, err := contractService.RemoveFunctionContract(ctx, gameID, env, functionID); err != nil {
		return fmt.Errorf("remove superseded unbound contract %s: %w", functionID, err)
	}
	return nil
}

// functionMetaInputForOperation 以 OpenAPI 操作元数据组装 unbound 契约入参
// （functionMetaInputForBinding 的无运行时版本：无 runtimeMeta 兜底，纯物料）。
func (s *Service) functionMetaInputForOperation(
	source *model.OpenAPISource,
	operation OpenAPISourceOperation,
	openAPIOp *openapi3.Operation,
	functionID string,
) dashboardservice.FunctionMetaInput {
	return dashboardservice.FunctionMetaInput{
		ID:                strings.TrimSpace(functionID),
		Version:           versionutil.ValidOrDefault(sourceInfoVersion(source)),
		Enabled:           true,
		Summary:           firstNonEmpty(operation.Summary, operation.OperationID),
		Description:       operation.Description,
		InputSchema:       openAPIRequestSchema(openAPIOp),
		OutputSchema:      openAPIResponseSchema(openAPIOp),
		Resource:          strings.TrimSpace(operation.Resource),
		Operation:         strings.TrimSpace(operation.Operation),
		Capability:        strings.TrimSpace(string(operation.Capability)),
		Execution:         firstNonEmpty(string(operation.Execution), "sync"),
		ApprovalRequired:  operation.Approval.Required,
		ApprovalPolicyKey: operation.Approval.PolicyKey,
		Risk:              strings.TrimSpace(string(operation.Risk)),
		Permission:        strings.TrimSpace(operation.Permission),
		Tags:              append([]string(nil), operation.Tags...),
		TimeoutMs:         operation.TimeoutMs,
	}
}

func (s *Service) reconcileContractAfterBindingDelete(ctx context.Context, gameID, env string, removed *model.OpenAPISourceBinding) error {
	if removed == nil {
		return nil
	}
	functionID := strings.TrimSpace(removed.FunctionID)
	if functionID == "" {
		return nil
	}
	remaining, err := s.svcCtx.OpenAPISourceBindingModel.ListByScopeAndFunctionID(ctx, gameID, env, functionID)
	if err != nil {
		return fmt.Errorf("list remaining OpenAPI bindings for %s: %w", functionID, err)
	}
	if len(remaining) > 0 {
		return s.rebuildContractFromRemainingBinding(ctx, gameID, env, remaining[0])
	}
	contractService := dashboardservice.NewContractService(s.svcCtx.DB)
	if runtimeMeta, ok := registeredFunctionMetaInScope(s.svcCtx, gameID, env, functionID); ok {
		previous, err := contractService.GetContract(ctx, gameID, env, functionID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load SDK fallback contract %s: %w", functionID, err)
		}
		if err := contractService.RebuildContractFromFunctionMeta(ctx, gameID, env, "sdk", functionMetaInputFromRuntime(functionID, runtimeMeta)); err != nil {
			return fmt.Errorf("restore SDK function contract %s: %w", functionID, err)
		}
		return s.rebuildProposalsAfterContractReconcile(ctx, gameID, env, resourceKeyFromContract(previous, runtimeMeta.Resource), functionID, runtimeMeta.Resource)
	}
	resourceKey, err := contractService.RemoveFunctionContract(ctx, gameID, env, functionID)
	if err != nil {
		return fmt.Errorf("remove OpenAPI function contract %s: %w", functionID, err)
	}
	return s.rebuildProposalsAfterContractReconcile(ctx, gameID, env, resourceKey, functionID, "")
}

func (s *Service) rebuildProposalsAfterContractReconcile(ctx context.Context, gameID, env, previousResource, functionID, currentResource string) error {
	contractService := dashboardservice.NewContractService(s.svcCtx.DB)
	resources := map[string]struct{}{}
	for _, resourceKey := range []string{previousResource, currentResource} {
		if resourceKey = strings.TrimSpace(resourceKey); resourceKey != "" {
			resources[resourceKey] = struct{}{}
		}
	}
	for resourceKey := range resources {
		if err := contractService.RebuildResourceCapability(ctx, gameID, env, resourceKey); err != nil {
			return fmt.Errorf("rebuild resource capability after binding delete %s: %w", resourceKey, err)
		}
		if err := contractService.RebuildProposalsForResource(ctx, gameID, env, resourceKey); err != nil {
			return fmt.Errorf("rebuild page proposals after binding delete %s: %w", resourceKey, err)
		}
	}
	if strings.TrimSpace(currentResource) == "" && strings.TrimSpace(functionID) != "" {
		if err := contractService.RebuildProposalForFunction(ctx, gameID, env, functionID); err != nil {
			return fmt.Errorf("rebuild standalone proposal after binding delete %s: %w", functionID, err)
		}
	}
	return nil
}

func functionMetaInputFromRuntime(functionID string, meta reg.FunctionMeta) dashboardservice.FunctionMetaInput {
	return dashboardservice.FunctionMetaInput{
		ID:                strings.TrimSpace(functionID),
		Version:           meta.Version,
		Enabled:           meta.Enabled,
		Deprecated:        meta.Deprecated,
		Summary:           meta.Summary,
		Description:       meta.Description,
		InputSchema:       meta.InputSchema,
		OutputSchema:      meta.OutputSchema,
		Resource:          meta.Resource,
		Operation:         meta.Operation,
		Capability:        meta.Capability,
		Execution:         meta.Execution,
		ApprovalRequired:  meta.ApprovalRequired,
		ApprovalPolicyKey: meta.ApprovalPolicyKey,
		Risk:              meta.Risk,
		Permission:        meta.Permission,
		Tags:              meta.Tags,
	}
}

func resourceKeyFromContract(contract *model.FunctionContract, fallback string) string {
	if contract != nil && strings.TrimSpace(contract.ResourceKey) != "" {
		return strings.TrimSpace(contract.ResourceKey)
	}
	return strings.TrimSpace(fallback)
}

func (s *Service) rebuildContractFromRemainingBinding(ctx context.Context, gameID, env string, binding model.OpenAPISourceBinding) error {
	source, err := s.svcCtx.OpenAPISourceModel.FindByScopeAndSourceID(ctx, gameID, env, binding.SourceID)
	if err != nil {
		return fmt.Errorf("find remaining binding source %s: %w", binding.SourceID, err)
	}
	var operations []OpenAPISourceOperation
	if err := source.GetOperations(&operations); err != nil {
		return fmt.Errorf("read remaining binding operations: %w", err)
	}
	operation, ok := findSourceOperation(operations, binding.OperationID)
	if !ok {
		return fmt.Errorf("remaining binding operation %s is missing from source %s", binding.OperationID, binding.SourceID)
	}
	runtimeMeta, ok := registeredFunctionMetaInScope(s.svcCtx, gameID, env, binding.FunctionID)
	if !ok {
		return fmt.Errorf("remaining binding function %s is not registered in current runtime", binding.FunctionID)
	}
	return s.rebuildContractForSourceBinding(ctx, gameID, env, source, operation, &binding, runtimeMeta)
}
