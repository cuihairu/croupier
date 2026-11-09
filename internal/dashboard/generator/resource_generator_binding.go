package generator

import (
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

func resourceBinding(contract *model.FunctionContract, suffix string, usage spec.PageBindingUsage, semantics *model.CapabilitySemantics) spec.PageFunctionBinding {
	binding := spec.PageFunctionBinding{
		ID:         suffix,
		FunctionID: strings.TrimSpace(contract.FunctionID),
		Usage:      usage,
		Execution: spec.PageBindingExecution{
			Mode:           spec.PageExecutionModeSync,
			RequireConfirm: resourceOperationRequiresConfirmation(contract),
		},
	}
	selectors := &spec.BindingSelectors{}
	if len(contract.InputSchema) > 0 {
		selectors.Input = resourceInputSelector(spec.JSONSchema(contract.InputSchema), contract, semantics)
	}
	selectors.Output = resourceOutputAssignments(contract, usage, semantics)
	if len(selectors.Input.Assignments) == 0 && len(selectors.Output) == 0 {
		return binding
	}
	binding.Selectors = selectors
	return binding
}

func resourceOutputAssignments(contract *model.FunctionContract, usage spec.PageBindingUsage, semantics *model.CapabilitySemantics) []spec.OutputAssignment {
	if contract == nil || len(contract.OutputSchema) == 0 {
		return nil
	}
	if usage == spec.BindingUsageDetail {
		return []spec.OutputAssignment{{
			StateKey: "detail",
			Source:   "",
			Shape:    spec.OutputShapeObject,
		}}
	}
	if usage != spec.BindingUsageQuery {
		return []spec.OutputAssignment{{
			StateKey: "result",
			Source:   "",
			Shape:    outputShapeForSchema(spec.JSONSchema(contract.OutputSchema)),
		}}
	}
	arrayKeys := collectionArrayKeys(semantics)
	assignments := collectionOutputAssignments(spec.JSONSchema(contract.OutputSchema), arrayKeys, "items")
	if len(assignments) == 0 {
		return nil
	}
	if semantics != nil && strings.TrimSpace(semantics.TotalFieldName) != "" {
		totalSource := scalarPropertySource(spec.JSONSchema(contract.OutputSchema), []string{strings.TrimSpace(semantics.TotalFieldName)})
		if totalSource == "" {
			return assignments
		}
		for i := range assignments {
			if assignments[i].StateKey == "total" {
				assignments[i].Source = totalSource
				return assignments
			}
		}
		assignments = append(assignments, spec.OutputAssignment{
			StateKey: "total",
			Source:   totalSource,
			Shape:    spec.OutputShapeScalar,
		})
	}
	return assignments
}

func resourceInputSelector(inputSchema spec.JSONSchema, contract *model.FunctionContract, semantics *model.CapabilitySemantics) spec.SelectorAST {
	selector := spec.DefaultSelector(inputSchema)
	if contract == nil || semantics == nil {
		return selector
	}
	if spec.CapabilityKind(contract.Capability.String()) == spec.CapabilityItemQuery {
		return applyIdentityRowSelector(selector, semantics)
	}
	if spec.CapabilityKind(contract.Capability.String()) != spec.CapabilityUpdate && spec.CapabilityKind(contract.Capability.String()) != spec.CapabilityDelete {
		if spec.CapabilityKind(contract.Capability.String()) == spec.CapabilityCollectionQuery {
			return applyCollectionQuerySelector(selector, semantics)
		}
		return selector
	}
	return applyIdentityRowSelector(selector, semantics)
}

func applyCollectionQuerySelector(selector spec.SelectorAST, semantics *model.CapabilitySemantics) spec.SelectorAST {
	if semantics == nil {
		return selector
	}
	pageField := strings.TrimSpace(semantics.PageFieldName)
	pageSizeField := strings.TrimSpace(semantics.PageSizeFieldName)
	for i := range selector.Assignments {
		switch selector.Assignments[i].Target {
		case pointerForField(pageField):
			selector.Assignments[i].Source = spec.ValueSource{Kind: spec.SourceForm, Path: "/current"}
		case pointerForField(pageSizeField):
			selector.Assignments[i].Source = spec.ValueSource{Kind: spec.SourceForm, Path: "/pageSize"}
		}
	}
	return selector
}

func applyIdentityRowSelector(selector spec.SelectorAST, semantics *model.CapabilitySemantics) spec.SelectorAST {
	if semantics == nil {
		return selector
	}
	identityField := strings.TrimSpace(semantics.IdentityField)
	if identityField == "" {
		return selector
	}
	identityTarget := "/" + escapeJSONPointerToken(identityField)
	for i := range selector.Assignments {
		if selector.Assignments[i].Target == identityTarget {
			selector.Assignments[i].Source = spec.ValueSource{
				Kind: spec.SourceRow,
				Path: identityTarget,
			}
		}
	}
	return selector
}

func pointerForField(field string) string {
	field = strings.TrimSpace(field)
	if field == "" {
		return ""
	}
	return "/" + escapeJSONPointerToken(field)
}

func escapeJSONPointerToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(token, "/", "~1")
}
