package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

func (s *ProposalService) publishValidationDiagnostics(ctx context.Context, gameID, env string, page spec.PageSpec) map[string]string {
	if s.publishValidator == nil {
		return nil
	}
	diagnostics := s.publishValidator.ValidatePageSpecInScope(ctx, gameID, env, page, true)
	details := make(map[string]string, len(diagnostics))
	for _, item := range diagnostics {
		field := item.Field
		if field == "" {
			field = item.Code
		}
		details[field] = item.Message
	}
	return details
}

func validateAcceptedPageSpec(gameID, env string, proposal *model.PageProposal, page spec.PageSpec) error {
	details := map[string]string{}
	if strings.TrimSpace(gameID) == "" {
		details["gameId"] = "game scope is required"
	}
	if strings.TrimSpace(env) == "" {
		details["env"] = "environment scope is required"
	}
	if strings.TrimSpace(page.PageKey) == "" {
		details["pageKey"] = "pageKey is required"
	}
	if strings.TrimSpace(proposal.PageKey) != "" && strings.TrimSpace(proposal.PageKey) != strings.TrimSpace(page.PageKey) {
		details["pageKey"] = "proposal pageKey must match PageSpec pageKey"
	}
	if !isValidProposalPageType(page.Type) {
		details["type"] = "type must be resource, operation, task, or report"
	}
	if !hasDefaultLocale(page.Title) {
		details["title"] = "title must include a non-empty value in at least one locale"
	}
	if strings.TrimSpace(page.Category.Key) == "" {
		details["category.key"] = "category.key is required"
	}
	if len(page.Bindings) == 0 {
		details["bindings"] = "page must bind at least one function"
	}
	if !pageShapeMatchesType(page) {
		details["shape"] = "page type must include the matching page body"
	}
	for i, binding := range page.Bindings {
		prefix := fmt.Sprintf("bindings[%d]", i)
		if strings.TrimSpace(binding.ID) == "" {
			details[prefix+".id"] = "binding.id is required"
		}
		if strings.TrimSpace(binding.FunctionID) == "" {
			details[prefix+".functionId"] = "binding.functionId is required"
		}
		if !isValidBindingUsage(binding.Usage) {
			details[prefix+".usage"] = "binding.usage is invalid"
		}
		if !isValidExecutionMode(binding.Execution.Mode) {
			details[prefix+".execution.mode"] = "binding.execution.mode is invalid"
		}
	}
	if len(details) > 0 {
		return errorx.NewValidationErrorWithDetails("proposal PageSpec validation failed", details)
	}
	return nil
}

// CollectBindingSelectorIssues runs the canonical selector validators
// (spec.ValidateSelector / ValidateOutputAssignments / ValidateRequiredOutputAssignments)
// over every binding of the page. It is the SINGLE rule source consumed at two
// enforcement points: composite proposal creation (issues become error-level
// diagnostics and downgrade quality) and accept-and-publish (issues hard-fail
// with 422 details). Adding a rule here therefore applies to both ends at once.
func CollectBindingSelectorIssues(functions map[string]spec.FunctionSpec, page spec.PageSpec) map[string]string {
	details := map[string]string{}
	for i, binding := range page.Bindings {
		field := fmt.Sprintf("bindings[%d]", i)
		functionID := strings.TrimSpace(binding.FunctionID)
		fn, ok := functions[functionID]
		if !ok {
			details[field+".functionId"] = "bound function contract does not exist"
			continue
		}
		if !fn.Enabled {
			details[field+".functionId"] = "bound function is disabled"
		}
		requiresInputSelectors := schemaHasFields(fn.InputSchema)
		requiresOutputSelectors := bindingRequiresOutputSelectors(binding, page)
		if requiresInputSelectors && binding.Selectors == nil {
			details[field+".selectors.input"] = "binding.selectors.input is required before publish"
			continue
		}
		if requiresOutputSelectors && binding.Selectors == nil {
			details[field+".selectors.output"] = "binding.selectors.output is required before publish"
			continue
		}
		if binding.Selectors == nil {
			continue
		}
		if requiresInputSelectors {
			inputResult := spec.ValidateSelector(binding.Selectors.Input, fn.InputSchema, spec.SelectorContextForBinding(page, binding))
			for _, item := range inputResult.Errors {
				details[field+".selectors.input."+item.Field] = item.Message
			}
		}
		if requiresOutputSelectors && len(binding.Selectors.Output) == 0 {
			details[field+".selectors.output"] = "binding.selectors.output is required before publish"
		}
		outputResult := spec.ValidateOutputAssignments(binding.Selectors.Output, fn.OutputSchema)
		for _, item := range outputResult.Errors {
			details[field+".selectors.output."+item.Field] = item.Message
		}
		for _, diag := range spec.ValidateRequiredOutputAssignments(binding, page) {
			details[field+".selectors.output"] = diag.Message
		}
	}
	return details
}

func (s *ProposalService) validateDirectPublishPageSpec(ctx context.Context, gameID, env string, proposal *model.PageProposal, page spec.PageSpec) error {
	if err := validateAcceptedPageSpec(gameID, env, proposal, page); err != nil {
		return err
	}
	details := map[string]string{}
	// ValidatePublishablePageShape / ValidateRequiredOutputAssignments 的全部
	// 诊断均经 publishShapeDiagnostic 构造，Field 恒为非空字面量前缀的拼接
	// 串，原 field=="" 时回退 Code 的分支为死代码，已删。
	for _, diag := range spec.ValidatePublishablePageShape(page) {
		details[strings.TrimSpace(diag.Field)] = diag.Message
	}
	functions, err := s.functionSpecsByID(ctx, gameID, env)
	if err != nil {
		return err
	}
	for field, message := range CollectBindingSelectorIssues(functions, page) {
		details[field] = message
	}
	if len(details) > 0 {
		return errorx.NewValidationErrorWithDetails("proposal PageSpec publish validation failed", details)
	}
	return nil
}

func isValidProposalPageType(pageType spec.PageType) bool {
	switch pageType {
	case spec.PageTypeResource, spec.PageTypeOperation, spec.PageTypeTask, spec.PageTypeReport,
		spec.PageTypeComposite:
		return true
	default:
		return false
	}
}

func pageShapeMatchesType(page spec.PageSpec) bool {
	switch page.Type {
	case spec.PageTypeResource:
		return page.Resource != nil
	case spec.PageTypeOperation:
		return page.Operation != nil
	case spec.PageTypeTask:
		return page.Task != nil
	case spec.PageTypeReport:
		return page.Report != nil
	case spec.PageTypeComposite:
		return page.Composite != nil && len(page.Composite.Sections) > 0
	default:
		return false
	}
}

func isValidBindingUsage(usage spec.PageBindingUsage) bool {
	switch usage {
	case spec.BindingUsageQuery,
		spec.BindingUsageDetail,
		spec.BindingUsageAction,
		spec.BindingUsageTask,
		spec.BindingUsageTaskStatus,
		spec.BindingUsageTaskEvents,
		spec.BindingUsageTaskResult,
		spec.BindingUsageTaskCancel,
		spec.BindingUsageTaskRetry,
		spec.BindingUsageReport:
		return true
	default:
		return false
	}
}

func isValidExecutionMode(mode spec.PageExecutionMode) bool {
	switch mode {
	case spec.PageExecutionModeSync, spec.PageExecutionModeTask:
		return true
	default:
		return false
	}
}

func executionModeForFunctionSpec(fn spec.FunctionSpec) spec.PageExecutionMode {
	if fn.Execution == spec.FunctionExecutionTask {
		return spec.PageExecutionModeTask
	}
	return spec.PageExecutionModeSync
}
