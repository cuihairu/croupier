package openapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/function/registrationguard"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
)

func operationPresentationField(op *openapi3.Operation) (string, bool) {
	if op == nil {
		return "", false
	}
	for key := range op.Extensions {
		if forbiddenKey, ok := registrationguard.ForbiddenPresentationField(key); ok {
			return forbiddenKey, true
		}
	}
	return "", false
}

func sourceSummaryFromModel(source *model.OpenAPISource) OpenAPISourceSummary {
	var operations []OpenAPISourceOperation
	_ = source.GetOperations(&operations)
	var diags []spec.Diagnostic
	_ = source.GetDiagnostics(&diags)
	return OpenAPISourceSummary{
		SourceID:        source.SourceID,
		GameID:          source.GameID,
		Env:             source.Env,
		Name:            source.Name,
		Revision:        source.Revision,
		Format:          source.Format,
		OpenAPIVersion:  source.OpenAPIVersion,
		InfoTitle:       source.InfoTitle,
		InfoVersion:     source.InfoVersion,
		ContentHash:     source.ContentHash,
		OperationCount:  len(operations),
		DiagnosticCount: len(diags),
		CreatedAt:       formatTime(source.CreatedAt),
		UpdatedAt:       formatTime(source.UpdatedAt),
		Diagnostics:     diags,
	}
}

func sourceDetailFromModel(source *model.OpenAPISource, bindings []model.OpenAPISourceBinding) OpenAPISourceDetail {
	var operations []OpenAPISourceOperation
	_ = source.GetOperations(&operations)
	bindingDTOs := make([]OpenAPISourceBindingDTO, 0, len(bindings))
	bindingByOperation := make(map[string]OpenAPISourceBindingDTO, len(bindings))
	for _, binding := range bindings {
		dto := bindingDTOFromModel(binding)
		bindingDTOs = append(bindingDTOs, dto)
		bindingByOperation[dto.OperationID] = dto
	}
	for i := range operations {
		if binding, ok := bindingByOperation[operations[i].OperationID]; ok {
			operations[i].Bound = true
			operations[i].BindingID = binding.BindingID
			operations[i].FunctionID = binding.FunctionID
		}
	}
	return OpenAPISourceDetail{
		OpenAPISourceSummary: sourceSummaryFromModel(source),
		Spec:                 source.GetSpec(),
		Operations:           operations,
		Bindings:             bindingDTOs,
	}
}

func bindingDTOFromModel(binding model.OpenAPISourceBinding) OpenAPISourceBindingDTO {
	return OpenAPISourceBindingDTO{
		BindingID:   binding.BindingID,
		OperationID: binding.OperationID,
		Kind:        binding.Kind,
		FunctionID:  binding.FunctionID,
		ProviderID:  binding.ProviderID,
		CreatedAt:   formatTime(binding.CreatedAt),
		UpdatedAt:   formatTime(binding.UpdatedAt),
	}
}

func (s *Service) generatedProposalForBinding(ctx context.Context, gameID, env string, functionID string) *OpenAPIBindingProposalDTO {
	if s == nil || s.svcCtx == nil || s.svcCtx.DB == nil {
		return nil
	}
	contract, err := model.NewFunctionContractModel(s.svcCtx.DB).FindByScopeAndFunctionID(ctx, gameID, env, functionID)
	if err != nil || contract == nil {
		return nil
	}
	proposalModel := model.NewPageProposalModel(s.svcCtx.DB)
	keys := proposalKeysForContract(contract)
	for _, key := range keys {
		proposal, err := proposalModel.FindByScopeAndKey(ctx, gameID, env, key)
		if err == nil && proposal != nil {
			return proposalDTOForOpenAPIBinding(proposal)
		}
	}
	return nil
}

func proposalKeysForContract(contract *model.FunctionContract) []string {
	if contract == nil {
		return nil
	}
	functionID := strings.TrimSpace(contract.FunctionID)
	resourceKey := strings.TrimSpace(contract.ResourceKey)
	capability := spec.CapabilityKind(contract.Capability.String())
	if resourceKey != "" && isOpenAPICRUDCapability(capability) {
		return []string{"resource:" + strings.Trim(resourceKey, ".")}
	}
	pageType := spec.PageTypeOperation
	switch capability {
	case spec.CapabilityTask:
		pageType = spec.PageTypeTask
	case spec.CapabilityReport:
		pageType = spec.PageTypeReport
	}
	key := dashboardStandaloneProposalKey(pageType, functionID)
	if key == "" {
		return nil
	}
	return []string{key}
}

func isOpenAPICRUDCapability(capability spec.CapabilityKind) bool {
	switch capability {
	case spec.CapabilityCollectionQuery,
		spec.CapabilityItemQuery,
		spec.CapabilityCreate,
		spec.CapabilityUpdate,
		spec.CapabilityDelete:
		return true
	default:
		return false
	}
}

func dashboardStandaloneProposalKey(pageType spec.PageType, functionID string) string {
	kind := string(pageType)
	switch pageType {
	case spec.PageTypeTask, spec.PageTypeReport:
	default:
		kind = string(spec.PageTypeOperation)
	}
	functionID = strings.Trim(strings.TrimSpace(functionID), ".")
	if functionID == "" {
		return ""
	}
	return kind + ":" + functionID
}

func proposalDTOForOpenAPIBinding(proposal *model.PageProposal) *OpenAPIBindingProposalDTO {
	if proposal == nil {
		return nil
	}
	return &OpenAPIBindingProposalDTO{
		ProposalKey: strings.TrimSpace(proposal.ProposalKey),
		PageKey:     strings.TrimSpace(proposal.PageKey),
		PageType:    strings.TrimSpace(proposal.PageType),
		ResourceKey: strings.TrimSpace(proposal.ResourceKey),
		Quality:     strings.TrimSpace(proposal.Quality),
		Status:      strings.TrimSpace(proposal.Status.String()),
	}
}

func sourceHasOperation(operations []OpenAPISourceOperation, operationID string) bool {
	_, ok := findSourceOperation(operations, operationID)
	return ok
}

func findSourceOperation(operations []OpenAPISourceOperation, operationID string) (OpenAPISourceOperation, bool) {
	operationID = strings.TrimSpace(operationID)
	for _, operation := range operations {
		if strings.TrimSpace(operation.OperationID) == operationID {
			return operation, true
		}
	}
	return OpenAPISourceOperation{}, false
}

func registeredFunctionMetaInScope(svcCtx *svc.ServiceContext, gameID, env, functionID string) (reg.FunctionMeta, bool) {
	functionID = strings.TrimSpace(functionID)
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	if svcCtx == nil || functionID == "" {
		return reg.FunctionMeta{}, false
	}
	if svcCtx.RegistryStore != nil {
		svcCtx.RegistryStore.Mu().RLock()
		defer svcCtx.RegistryStore.Mu().RUnlock()
		for _, sess := range svcCtx.RegistryStore.AgentsUnsafe() {
			if sess == nil {
				continue
			}
			if gameID != "" && strings.TrimSpace(sess.GameID) != gameID {
				continue
			}
			if env != "" && strings.TrimSpace(sess.Env) != env {
				continue
			}
			meta, ok := sess.Functions[functionID]
			if ok {
				return meta, true
			}
		}
	}
	if svcCtx.FunctionModel != nil {
		fn, err := svcCtx.FunctionModel.FindByFunctionID(context.Background(), functionID)
		if err == nil && fn != nil {
			if gameID != "" && strings.TrimSpace(fn.GameID) != "" && strings.TrimSpace(fn.GameID) != gameID {
				return reg.FunctionMeta{}, false
			}
			return reg.FunctionMeta{
				Enabled:     fn.Status != 0,
				Version:     fn.Version,
				Summary:     firstNonEmpty(fn.Name, fn.Description),
				Description: fn.Description,
				Resource:    fn.Resource,
			}, true
		}
	}
	return reg.FunctionMeta{}, false
}

func sourceInfoVersion(source *model.OpenAPISource) string {
	if source == nil {
		return ""
	}
	return source.InfoVersion
}

func sanitizeBindingID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return uuid.NewString()
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('-')
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return uuid.NewString()
	}
	return out
}

func hasRegisteredFunction(svcCtx *svc.ServiceContext, functionID string) bool {
	functionID = strings.TrimSpace(functionID)
	if functionID == "" {
		return false
	}
	if svcCtx == nil {
		return false
	}
	if svcCtx.RegistryStore != nil {
		svcCtx.RegistryStore.Mu().RLock()
		defer svcCtx.RegistryStore.Mu().RUnlock()
		for _, sess := range svcCtx.RegistryStore.AgentsUnsafe() {
			if sess == nil {
				continue
			}
			if _, ok := sess.Functions[functionID]; ok {
				return true
			}
		}
	}
	if svcCtx.FunctionModel != nil {
		if _, err := svcCtx.FunctionModel.FindByFunctionID(context.Background(), functionID); err == nil {
			return true
		}
	}
	return false
}

// normalizeOpenAPIDoc patches common non-critical gaps from external OpenAPI docs
// so import stays resilient (e.g. response description omitted by third-party generators).
func normalizeOpenAPIDoc(doc *openapi3.T) {
	if doc == nil || doc.Paths == nil {
		return
	}
	for _, pathItem := range doc.Paths.Map() {
		if pathItem == nil {
			continue
		}
		operations := []*openapi3.Operation{
			pathItem.Get,
			pathItem.Post,
			pathItem.Put,
			pathItem.Patch,
			pathItem.Delete,
			pathItem.Options,
			pathItem.Head,
			pathItem.Trace,
		}
		for _, op := range operations {
			if op == nil || op.Responses == nil {
				continue
			}
			for statusCode, responseRef := range op.Responses.Map() {
				if responseRef == nil {
					continue
				}
				if responseRef.Value == nil {
					responseRef.Value = &openapi3.Response{}
				}
				if responseRef.Value.Description == nil || strings.TrimSpace(*responseRef.Value.Description) == "" {
					desc := fmt.Sprintf("Auto-generated response description for status %s", statusCode)
					responseRef.Value.Description = &desc
				}
			}
		}
	}
}
