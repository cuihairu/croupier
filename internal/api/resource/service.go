package resource

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

// Service provides Resource API operations.
type Service struct {
	svcCtx *svc.ServiceContext
}

// NewService creates a new Resource Service.
func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns normalized ResourceSpec list from registered functions.
func (s *Service) List(ctx context.Context, req *ResourceListRequest) (*ResourceListResponse, error) {
	if err := s.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	resources, err := s.loadPersistentResources(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]spec.ResourceSpec, 0, len(resources))
	for _, resource := range resources {
		if req.Category != "" && resource.Category.Key != req.Category {
			continue
		}
		if req.Query != "" && !matchesResourceQuery(resource, req.Query) {
			continue
		}
		items = append(items, resource)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Category.Order != items[j].Category.Order {
			return items[i].Category.Order < items[j].Category.Order
		}
		return items[i].Key < items[j].Key
	})

	return &ResourceListResponse{Items: items}, nil
}

// Detail returns a single ResourceSpec by key.
func (s *Service) Detail(ctx context.Context, req *ResourceDetailRequest) (*ResourceDetailResponse, error) {
	if err := s.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	resource, err := s.loadPersistentResource(ctx, req.ResourceKey)
	if err != nil {
		return nil, ErrResourceNotFound(req.ResourceKey)
	}

	return &ResourceDetailResponse{Resource: resource}, nil
}

// Operations returns OperationSpec list for a resource.
func (s *Service) Operations(ctx context.Context, req *ResourceOperationsRequest) (*ResourceOperationsResponse, error) {
	if err := s.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	resource, err := s.loadPersistentResource(ctx, req.ResourceKey)
	if err != nil {
		return nil, ErrResourceNotFound(req.ResourceKey)
	}

	return &ResourceOperationsResponse{Items: resource.Operations}, nil
}

func (s *Service) loadPersistentResources(ctx context.Context) ([]spec.ResourceSpec, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	capabilityModel := model.NewResourceCapabilityModel(s.svcCtx.DB)
	caps, err := capabilityModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	resources := make([]spec.ResourceSpec, 0, len(caps))
	for _, cap := range caps {
		if cap == nil {
			continue
		}
		resource, err := s.resourceSpecFromCapability(ctx, gameID, env, cap)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func (s *Service) loadPersistentResource(ctx context.Context, resourceKey string) (spec.ResourceSpec, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return spec.ResourceSpec{}, err
	}
	capabilityModel := model.NewResourceCapabilityModel(s.svcCtx.DB)
	capability, err := capabilityModel.FindByScopeAndResourceKey(ctx, gameID, env, strings.TrimSpace(resourceKey))
	if err != nil {
		return spec.ResourceSpec{}, err
	}
	return s.resourceSpecFromCapability(ctx, gameID, env, capability)
}

func (s *Service) resourceSpecFromCapability(ctx context.Context, gameID, env string, capability *model.ResourceCapability) (spec.ResourceSpec, error) {
	if capability == nil {
		return spec.ResourceSpec{}, errorx.NewNotFound("resource capability not found")
	}
	contracts, err := model.NewFunctionContractModel(s.svcCtx.DB).ListByResourceKey(ctx, gameID, env, capability.ResourceKey)
	if err != nil {
		return spec.ResourceSpec{}, err
	}
	semantics, err := model.NewCapabilitySemanticsModel(s.svcCtx.DB).FindByScopeAndResourceKey(ctx, gameID, env, capability.ResourceKey)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.ResourceSpec{}, err
	}
	resourceKey := strings.TrimSpace(capability.ResourceKey)
	categoryKey := strings.TrimSpace(capability.CategoryKey)
	if categoryKey == "" {
		categoryKey = categoryFromResourceKey(resourceKey)
	}
	resource := spec.ResourceSpec{
		Key:         resourceKey,
		Labels:      localizedFromJSONMap(capability.Labels, resourceKey),
		Description: localizedFromJSONMap(capability.Description, ""),
		Category: spec.ResourceCategorySpec{
			Key:    categoryKey,
			Labels: spec.LocalizedText{"zh-CN": humanizeKey(categoryKey), "en-US": humanizeKey(categoryKey)},
		},
		Operations:  operationSpecsFromContracts(contracts),
		Diagnostics: resourceDiagnostics(contracts, semantics),
	}
	return resource, nil
}

func operationSpecsFromContracts(contracts []*model.FunctionContract) []spec.OperationSpec {
	operations := make([]spec.OperationSpec, 0, len(contracts))
	for _, contract := range contracts {
		if contract == nil {
			continue
		}
		operations = append(operations, spec.OperationSpec{
			FunctionID:  strings.TrimSpace(contract.FunctionID),
			ResourceKey: strings.TrimSpace(contract.ResourceKey),
			Operation:   strings.TrimSpace(contract.OperationKey),
			Capability:  spec.CapabilityKind(contract.Capability),
			Execution:   spec.FunctionExecution(contract.Execution),
			Approval:    approvalPolicyFromJSONMap(contract.Approval),
			Risk:        spec.RiskLevel(contract.Risk),
			Permission:  strings.TrimSpace(contract.Permission),
			Enabled:     contract.Enabled,
			Diagnostics: diagnosticsFromJSON(contract.Diagnostics, contract.FunctionID),
		})
	}
	sort.Slice(operations, func(i, j int) bool {
		if operations[i].ResourceKey != operations[j].ResourceKey {
			return operations[i].ResourceKey < operations[j].ResourceKey
		}
		return operations[i].FunctionID < operations[j].FunctionID
	})
	return operations
}

func approvalPolicyFromJSONMap(values map[string]interface{}) spec.ApprovalPolicy {
	if len(values) == 0 {
		return spec.ApprovalPolicy{}
	}
	required, _ := values["required"].(bool)
	policyKey, _ := values["policyKey"].(string)
	if policyKey == "" {
		policyKey, _ = values["policy_key"].(string)
	}
	return spec.ApprovalPolicy{Required: required, PolicyKey: strings.TrimSpace(policyKey)}
}

func resourceDiagnostics(contracts []*model.FunctionContract, semantics *model.CapabilitySemantics) []spec.Diagnostic {
	diagnostics := make([]spec.Diagnostic, 0)
	if semantics != nil {
		diagnostics = append(diagnostics, diagnosticsFromJSON(semantics.Diagnostics, "")...)
	}
	for _, contract := range contracts {
		if contract == nil {
			continue
		}
		diagnostics = append(diagnostics, diagnosticsFromJSON(contract.Diagnostics, contract.FunctionID)...)
	}
	return diagnostics
}

func diagnosticsFromJSON(raw []byte, fallbackFunctionID string) []spec.Diagnostic {
	if len(raw) == 0 {
		return nil
	}
	var diagnostics []spec.Diagnostic
	if err := json.Unmarshal(raw, &diagnostics); err != nil {
		return []spec.Diagnostic{{
			Code:       "diagnostic_parse_failed",
			Severity:   spec.SeverityWarning,
			Message:    "diagnostics payload is not readable",
			FunctionID: fallbackFunctionID,
		}}
	}
	for i := range diagnostics {
		if diagnostics[i].FunctionID == "" {
			diagnostics[i].FunctionID = fallbackFunctionID
		}
	}
	return diagnostics
}

func localizedFromJSONMap(values map[string]interface{}, fallback string) spec.LocalizedText {
	out := spec.LocalizedText{}
	for key, value := range values {
		text, ok := value.(string)
		if ok && strings.TrimSpace(key) != "" && strings.TrimSpace(text) != "" {
			out[strings.TrimSpace(key)] = strings.TrimSpace(text)
		}
	}
	if len(out) == 0 && strings.TrimSpace(fallback) != "" {
		text := humanizeKey(fallback)
		out["zh-CN"] = text
		out["en-US"] = text
	}
	return out
}

func categoryFromResourceKey(resourceKey string) string {
	resourceKey = strings.TrimSpace(resourceKey)
	if idx := strings.Index(resourceKey, "."); idx > 0 {
		return resourceKey[:idx]
	}
	return resourceKey
}

func humanizeKey(key string) string {
	key = strings.Trim(strings.TrimSpace(key), "._-")
	if key == "" {
		return ""
	}
	parts := strings.FieldsFunc(key, func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	})
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, " ")
}

func matchesResourceQuery(resource spec.ResourceSpec, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	if strings.Contains(strings.ToLower(resource.Key), q) {
		return true
	}
	for _, label := range resource.Labels {
		if strings.Contains(strings.ToLower(label), q) {
			return true
		}
	}
	return false
}

func requireScope(ctx context.Context) (string, string, error) {
	gameID, env := svc.GameScopeFromContext(ctx)
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	if gameID == "" {
		return "", "", errorx.NewBadRequest("X-Game-ID is required")
	}
	if env == "" {
		return "", "", errorx.NewBadRequest("X-Env is required")
	}
	return gameID, env, nil
}

func (s *Service) requireResourceRead(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权查看资源", "admin:all", "resources:read", "resources:diagnose", "functions:read", "functions:manage", "pages:read", "pages:edit")
	return err
}

func (s *Service) requireResourceDiagnose(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权生成页面候选", "admin:all", "resources:diagnose", "functions:manage", "pages:edit")
	return err
}

// ErrResourceNotFound returns a not-found error for a resource key.
func ErrResourceNotFound(key string) error {
	return &ResourceNotFoundError{Key: key}
}

// ResourceNotFoundError indicates a resource was not found.
type ResourceNotFoundError struct {
	Key string
}

func (e *ResourceNotFoundError) Error() string {
	return "resource not found: " + e.Key
}
