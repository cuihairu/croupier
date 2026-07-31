package generator

import (
	"encoding/json"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

// GenerateResourcePageProposal generates a ResourcePage proposal from
// persistent CapabilitySemantics and FunctionContracts.
func GenerateResourcePageProposal(
	semantics *model.CapabilitySemantics,
	contracts []*model.FunctionContract,
	opts GenerateOptions,
) (spec.GeneratedPageSpec, bool) {
	opts = normalizeOptions(opts)

	// Need at least collection_query and item_query for a CRUD page
	hasCollection := semantics.CollectionQueryID > 0
	hasIdentity := semantics.IdentityField != ""

	if !hasCollection || !hasIdentity {
		return spec.GeneratedPageSpec{}, false
	}

	// Build function lookup
	functions := make(map[string]spec.FunctionSpec)
	for _, c := range contracts {
		functions[c.FunctionID] = contractToFunctionSpec(c)
	}

	// Find the collection query contract
	var collectionContract *model.FunctionContract
	for _, c := range contracts {
		if c.Capability == "collection_query" {
			collectionContract = c
			break
		}
	}
	if collectionContract == nil {
		return spec.GeneratedPageSpec{}, false
	}

	resourceKey := semantics.ResourceKey
	pageKey := resourceKey + ".manage"

	// Build bindings
	bindings := buildResourceBindings(semantics, contracts)

	// Build diagnostics
	diags := assessResourceSemantics(semantics)

	// Build schema
	schema := buildResourcePageSchema(pageKey, resourceKey, semantics, bindings, opts)

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeEntity,
			ResourceKey: resourceKey,
			Title:       spec.LocalizedText{opts.DefaultLocale: resourceKey},
			Category:    categoryForPage(resourceKey, pageKey, opts.DefaultLocale),
			Schema:      schema,
			Bindings:    bindings,
		},
		Quality:     resourceQuality(semantics, diags),
		Diagnostics: diags,
	}, true
}

// buildResourceBindings creates bindings for a resource page.
func buildResourceBindings(semantics *model.CapabilitySemantics, contracts []*model.FunctionContract) []spec.PageFunctionBinding {
	var bindings []spec.PageFunctionBinding

	// Collection query binding
	if semantics.CollectionQueryID > 0 {
		contract := findContractByID(contracts, semantics.CollectionQueryID)
		if contract != nil {
			bindings = append(bindings, spec.PageFunctionBinding{
				ID:            "list",
				FunctionID:    contract.FunctionID,
				Usage:         spec.BindingUsageQuery,
				InputMapping:  json.RawMessage(`{}`),
				OutputMapping: json.RawMessage(`{}`),
				Execution:     spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
			})
		}
	}

	// Create binding
	if semantics.CreateID > 0 {
		contract := findContractByID(contracts, semantics.CreateID)
		if contract != nil {
			bindings = append(bindings, spec.PageFunctionBinding{
				ID:            "create",
				FunctionID:    contract.FunctionID,
				Usage:         spec.BindingUsageAction,
				InputMapping:  json.RawMessage(`{}`),
				OutputMapping: json.RawMessage(`{}`),
				Execution:     spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
			})
		}
	}

	// Update binding
	if semantics.UpdateID > 0 {
		contract := findContractByID(contracts, semantics.UpdateID)
		if contract != nil {
			bindings = append(bindings, spec.PageFunctionBinding{
				ID:            "update",
				FunctionID:    contract.FunctionID,
				Usage:         spec.BindingUsageAction,
				InputMapping:  json.RawMessage(`{}`),
				OutputMapping: json.RawMessage(`{}`),
				Execution:     spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
			})
		}
	}

	// Delete binding
	if semantics.DeleteID > 0 {
		contract := findContractByID(contracts, semantics.DeleteID)
		if contract != nil {
			bindings = append(bindings, spec.PageFunctionBinding{
				ID:            "delete",
				FunctionID:    contract.FunctionID,
				Usage:         spec.BindingUsageAction,
				InputMapping:  json.RawMessage(`{}`),
				OutputMapping: json.RawMessage(`{}`),
				Execution:     spec.PageBindingExecution{Mode: spec.PageExecutionModeSync, RequireConfirm: true},
			})
		}
	}

	return bindings
}

// buildResourcePageSchema builds the page schema for a resource page.
func buildResourcePageSchema(pageKey, resourceKey string, semantics *model.CapabilitySemantics, bindings []spec.PageFunctionBinding, opts GenerateOptions) spec.FormilySchema {
	// Build properties
	properties := make(map[string]json.RawMessage)

	// Query form
	queryFormPropsJSON, _ := json.Marshal(queryFormProps{
		BindingID:    "list",
		InputMapping: json.RawMessage(`{}`),
	})
	properties["form"] = rawNode(componentNode("QueryForm", queryFormPropsJSON))

	// Data table (would need column info from schema)
	tablePropsJSON, _ := json.Marshal(map[string]interface{}{
		"bindingId": "list",
	})
	properties["table"] = rawNode(componentNode("DataTable", tablePropsJSON))

	// Result panel
	resultPanelPropsJSON, _ := json.Marshal(resultPanelProps{
		BindingID: "list",
	})
	properties["result"] = rawNode(componentNode("ResultPanel", resultPanelPropsJSON))

	// Build root node
	consolePagePropsJSON, _ := json.Marshal(consolePageProps{
		SchemaVersion: "formily-page:1",
		PageKey:       pageKey,
		ResourceKey:   resourceKey,
	})
	schema := componentNode("ConsolePage", consolePagePropsJSON)
	schema.Properties = properties

	b, _ := json.Marshal(schema)
	return spec.FormilySchema(b)
}

// assessResourceSemantics checks if semantics are sufficient for CRUD.
func assessResourceSemantics(semantics *model.CapabilitySemantics) []spec.Diagnostic {
	var diags []spec.Diagnostic

	if semantics.IdentityField == "" {
		diags = append(diags, spec.Diagnostic{
			Code:     "identity_missing",
			Severity: spec.SeverityWarning,
			Message:  "identity field not specified; detail view cannot be generated",
		})
	}

	if semantics.CollectionQueryID == 0 {
		diags = append(diags, spec.Diagnostic{
			Code:     "collection_query_missing",
			Severity: spec.SeverityError,
			Message:  "collection_query function not found; list view cannot be generated",
		})
	}

	return diags
}

// resourceQuality determines quality based on semantics completeness.
func resourceQuality(semantics *model.CapabilitySemantics, diags []spec.Diagnostic) spec.GeneratedPageQuality {
	hasError := false
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			hasError = true
			break
		}
	}

	if hasError {
		return spec.GeneratedPageQualityBlocked
	}

	// Check if we have full CRUD
	hasCRUD := semantics.CreateID > 0 && semantics.UpdateID > 0 && semantics.DeleteID > 0
	if hasCRUD && semantics.IdentityField != "" {
		return spec.GeneratedPageQualityReady
	}

	return spec.GeneratedPageQualityBasic
}

// Helper functions

func findContractByID(contracts []*model.FunctionContract, id uint) *model.FunctionContract {
	for _, c := range contracts {
		if c.ID == id {
			return c
		}
	}
	return nil
}

func contractToFunctionSpec(c *model.FunctionContract) spec.FunctionSpec {
	summary := make(spec.LocalizedText)
	for k, v := range c.Summary {
		if s, ok := v.(string); ok {
			summary[k] = s
		}
	}
	description := make(spec.LocalizedText)
	for k, v := range c.Description {
		if s, ok := v.(string); ok {
			description[k] = s
		}
	}

	return spec.FunctionSpec{
		ID:           c.FunctionID,
		Version:      c.Version,
		Enabled:      c.Enabled,
		Deprecated:   c.Deprecated,
		InputSchema:  spec.JSONSchema(c.InputSchema),
		OutputSchema: spec.JSONSchema(c.OutputSchema),
		Summary:      summary,
		Description:  description,
		Resource:     c.ResourceKey,
		Operation:    c.OperationKey,
		Capability:   spec.CapabilityKind(c.Capability),
		Execution:    spec.FunctionExecution(c.Execution),
		Risk:         spec.RiskLevel(c.Risk),
		Permission:   c.Permission,
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
