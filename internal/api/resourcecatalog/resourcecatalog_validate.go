package resourcecatalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
)

func (s *Service) requireResourceCapability(ctx context.Context, gameID, env, resourceKey string) error {
	if _, err := s.capabilityModel.FindByScopeAndResourceKey(ctx, gameID, env, resourceKey); err != nil {
		return fmt.Errorf("resource capability not found: %w", err)
	}
	return nil
}

// validateFunctionBinding checks if a function belongs to the resource and capability slot.
func (s *Service) validateFunctionBinding(ctx context.Context, gameID, env, resourceKey string, functionID uint, capability string) (*model.FunctionContract, error) {
	var contract model.FunctionContract
	if err := s.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND id = ?", gameID, env, functionID).
		First(&contract).Error; err != nil {
		return nil, err
	}
	if contract.ResourceKey != resourceKey {
		return nil, fmt.Errorf("function %s belongs to resource %s, not %s", contract.FunctionID, contract.ResourceKey, resourceKey)
	}
	if contract.Capability.String() != capability {
		return nil, fmt.Errorf("function %s capability is %s, not %s", contract.FunctionID, contract.Capability.String(), capability)
	}
	if !contract.Enabled {
		return nil, fmt.Errorf("function %s is disabled", contract.FunctionID)
	}
	return &contract, nil
}

// Helper functions

func (s *Service) validateActionSemantics(
	ctx context.Context,
	gameID string,
	env string,
	resourceKey string,
	actions []ActionSemanticInfo,
) ([]ActionSemanticInfo, error) {
	out := make([]ActionSemanticInfo, 0, len(actions))
	seen := map[string]struct{}{}
	for index, action := range actions {
		functionID := strings.TrimSpace(action.FunctionID)
		if functionID == "" {
			return nil, fmt.Errorf("invalid actions[%d]: functionId is required", index)
		}
		contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
		if err != nil {
			return nil, fmt.Errorf("invalid actions[%d].functionId: %w", index, err)
		}
		if strings.TrimSpace(contract.ResourceKey) != resourceKey {
			return nil, fmt.Errorf("invalid actions[%d].functionId: function does not belong to resource %s", index, resourceKey)
		}
		if contract.Capability != dbenum.CapabilityAction {
			return nil, fmt.Errorf("invalid actions[%d].functionId: function capability must be action", index)
		}
		subject := strings.TrimSpace(action.Subject)
		switch subject {
		case "resource_item", "resource_selection":
			if strings.TrimSpace(action.IdentityInput) == "" || !isJSONPointer(action.IdentityInput) {
				return nil, fmt.Errorf("invalid actions[%d].identityInput: must be a JSON Pointer", index)
			}
		case "none":
			action.IdentityInput = ""
		default:
			return nil, fmt.Errorf("invalid actions[%d].subject: must be resource_item, resource_selection, or none", index)
		}
		key := functionID + ":" + subject + ":" + strings.TrimSpace(action.IdentityInput)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ActionSemanticInfo{
			FunctionID:    functionID,
			Subject:       subject,
			IdentityInput: strings.TrimSpace(action.IdentityInput),
		})
	}
	return out, nil
}

func (s *Service) validateTaskSemantics(
	ctx context.Context,
	gameID string,
	env string,
	resourceKey string,
	tasks []spec.TaskSemantic,
) ([]spec.TaskSemantic, error) {
	out := make([]spec.TaskSemantic, 0, len(tasks))
	seen := map[string]struct{}{}
	for index, task := range tasks {
		start, err := s.validateSemanticFunctionRef(ctx, gameID, env, resourceKey, task.Start, spec.CapabilityTask, fmt.Sprintf("tasks[%d].start", index))
		if err != nil {
			return nil, err
		}
		task.Start = start.ref
		if !isJSONPointer(task.TaskID.ResultPath) {
			return nil, fmt.Errorf("invalid tasks[%d].taskId.resultPath: must be a JSON Pointer", index)
		}
		if !spec.IsValidJsonScalarType(task.TaskID.ValueType) {
			return nil, fmt.Errorf("invalid tasks[%d].taskId.valueType: must be string, number, integer, or boolean", index)
		}
		if !schemaHasPointer(start.contract.OutputSchema, task.TaskID.ResultPath) {
			return nil, fmt.Errorf("invalid tasks[%d].taskId.resultPath: path not found in start output schema", index)
		}
		status, err := s.validateSemanticFunctionRef(ctx, gameID, env, resourceKey, task.Status.Function, "", fmt.Sprintf("tasks[%d].status.function", index))
		if err != nil {
			return nil, err
		}
		task.Status.Function = status.ref
		if err := validateTaskInputPointer(status.contract, task.Status.TaskIDInput, fmt.Sprintf("tasks[%d].status.taskIdInput", index)); err != nil {
			return nil, err
		}
		if !isJSONPointer(task.Status.StatePath) {
			return nil, fmt.Errorf("invalid tasks[%d].status.statePath: must be a JSON Pointer", index)
		}
		if !schemaHasPointer(status.contract.OutputSchema, task.Status.StatePath) {
			return nil, fmt.Errorf("invalid tasks[%d].status.statePath: path not found in status output schema", index)
		}
		if task.Events != nil {
			events, err := s.validateSemanticFunctionRef(ctx, gameID, env, resourceKey, task.Events.Function, "", fmt.Sprintf("tasks[%d].events.function", index))
			if err != nil {
				return nil, err
			}
			task.Events.Function = events.ref
			if err := validateTaskInputPointer(events.contract, task.Events.TaskIDInput, fmt.Sprintf("tasks[%d].events.taskIdInput", index)); err != nil {
				return nil, err
			}
			if !isJSONPointer(task.Events.EventsPath) {
				return nil, fmt.Errorf("invalid tasks[%d].events.eventsPath: must be a JSON Pointer", index)
			}
			if !schemaHasPointer(events.contract.OutputSchema, task.Events.EventsPath) {
				return nil, fmt.Errorf("invalid tasks[%d].events.eventsPath: path not found in events output schema", index)
			}
		}
		if task.Result != nil {
			result, err := s.validateSemanticFunctionRef(ctx, gameID, env, resourceKey, task.Result.Function, "", fmt.Sprintf("tasks[%d].result.function", index))
			if err != nil {
				return nil, err
			}
			task.Result.Function = result.ref
			if err := validateTaskInputPointer(result.contract, task.Result.TaskIDInput, fmt.Sprintf("tasks[%d].result.taskIdInput", index)); err != nil {
				return nil, err
			}
			if !isJSONPointer(task.Result.ResultPath) {
				return nil, fmt.Errorf("invalid tasks[%d].result.resultPath: must be a JSON Pointer", index)
			}
			if !schemaHasPointer(result.contract.OutputSchema, task.Result.ResultPath) {
				return nil, fmt.Errorf("invalid tasks[%d].result.resultPath: path not found in result output schema", index)
			}
		}
		if task.Cancel != nil {
			cancel, err := s.validateSemanticFunctionRef(ctx, gameID, env, resourceKey, task.Cancel.Function, "", fmt.Sprintf("tasks[%d].cancel.function", index))
			if err != nil {
				return nil, err
			}
			task.Cancel.Function = cancel.ref
			if err := validateTaskInputPointer(cancel.contract, task.Cancel.TaskIDInput, fmt.Sprintf("tasks[%d].cancel.taskIdInput", index)); err != nil {
				return nil, err
			}
		}
		if task.Retry != nil {
			return nil, fmt.Errorf("invalid tasks[%d].retry: retry runtime is not available", index)
		}
		key := strings.TrimSpace(task.Start.FunctionID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, task)
	}
	return out, nil
}

func (s *Service) validateReportSemantics(
	ctx context.Context,
	gameID string,
	env string,
	resourceKey string,
	reports []spec.ReportSemantic,
) ([]spec.ReportSemantic, error) {
	out := make([]spec.ReportSemantic, 0, len(reports))
	seen := map[string]struct{}{}
	for index, report := range reports {
		query, err := s.validateSemanticFunctionRef(ctx, gameID, env, resourceKey, report.Query, spec.CapabilityReport, fmt.Sprintf("reports[%d].query", index))
		if err != nil {
			return nil, err
		}
		report.Query = query.ref
		if !isJSONPointer(report.DatasetPath) {
			return nil, fmt.Errorf("invalid reports[%d].datasetPath: must be a JSON Pointer", index)
		}
		datasetItemSchema, ok := arrayItemSchemaAtPointer(query.contract.OutputSchema, report.DatasetPath)
		if !ok {
			return nil, fmt.Errorf("invalid reports[%d].datasetPath: path must reference an array in query output schema", index)
		}
		if len(report.Dimensions) == 0 {
			return nil, fmt.Errorf("invalid reports[%d].dimensions: at least one dimension is required", index)
		}
		if len(report.Metrics) == 0 {
			return nil, fmt.Errorf("invalid reports[%d].metrics: at least one metric is required", index)
		}
		dimensions, err := validateJSONPointerList(report.Dimensions, fmt.Sprintf("reports[%d].dimensions", index))
		if err != nil {
			return nil, err
		}
		metrics, err := validateJSONPointerList(report.Metrics, fmt.Sprintf("reports[%d].metrics", index))
		if err != nil {
			return nil, err
		}
		report.Dimensions = dimensions
		report.Metrics = metrics
		// validateJSONPointerList 已保证每个指针非空且以 "/" 开头（见其内部校验），
		// 下述 HasPrefix 前缀分支恒假，已删除；仅保留 schema 存在性检查。
		for _, pointer := range report.Dimensions {
			if !schemaObjectHasPointer(datasetItemSchema, pointer) {
				return nil, fmt.Errorf("invalid reports[%d].dimensions: pointer %s not found in dataset item schema", index, pointer)
			}
		}
		for _, pointer := range report.Metrics {
			if !schemaObjectHasPointer(datasetItemSchema, pointer) {
				return nil, fmt.Errorf("invalid reports[%d].metrics: pointer %s not found in dataset item schema", index, pointer)
			}
		}
		key := strings.TrimSpace(report.Query.FunctionID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, report)
	}
	return out, nil
}

type semanticFunctionRef struct {
	ref      spec.FunctionRef
	contract *model.FunctionContract
}

func (s *Service) validateSemanticFunctionRef(
	ctx context.Context,
	gameID string,
	env string,
	resourceKey string,
	ref spec.FunctionRef,
	requiredCapability spec.CapabilityKind,
	field string,
) (semanticFunctionRef, error) {
	functionID := strings.TrimSpace(ref.FunctionID)
	if functionID == "" {
		return semanticFunctionRef{}, fmt.Errorf("invalid %s.functionId: functionId is required", field)
	}
	contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
	if err != nil {
		return semanticFunctionRef{}, fmt.Errorf("invalid %s.functionId: %w", field, err)
	}
	if strings.TrimSpace(contract.ResourceKey) != resourceKey {
		return semanticFunctionRef{}, fmt.Errorf("invalid %s.functionId: function does not belong to resource %s", field, resourceKey)
	}
	if requiredCapability != "" && spec.CapabilityKind(contract.Capability.String()) != requiredCapability {
		return semanticFunctionRef{}, fmt.Errorf("invalid %s.functionId: function capability must be %s", field, requiredCapability)
	}
	if !contract.Enabled {
		return semanticFunctionRef{}, fmt.Errorf("invalid %s.functionId: function %s is disabled", field, contract.FunctionID)
	}
	return semanticFunctionRef{
		ref: spec.FunctionRef{
			FunctionID:         strings.TrimSpace(contract.FunctionID),
			ContractVersion:    strings.TrimSpace(contract.Version),
			InputSchemaDigest:  digestRawJSON(contract.InputSchema),
			OutputSchemaDigest: digestRawJSON(contract.OutputSchema),
		},
		contract: contract,
	}, nil
}

func validateTaskInputPointer(contract *model.FunctionContract, pointer string, field string) error {
	if !isJSONPointer(pointer) {
		return fmt.Errorf("invalid %s: must be a JSON Pointer", field)
	}
	if contract != nil && !schemaHasPointer(contract.InputSchema, pointer) {
		return fmt.Errorf("invalid %s: path not found in function input schema", field)
	}
	return nil
}

func validateJSONPointerList(values []string, field string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for index, value := range values {
		pointer := strings.TrimSpace(value)
		if pointer == "" || !strings.HasPrefix(pointer, "/") || !isJSONPointer(pointer) {
			return nil, fmt.Errorf("invalid %s[%d]: must be a non-empty JSON Pointer", field, index)
		}
		if _, ok := seen[pointer]; ok {
			continue
		}
		seen[pointer] = struct{}{}
		out = append(out, pointer)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("invalid %s: at least one JSON Pointer is required", field)
	}
	return out, nil
}
