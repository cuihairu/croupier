package generator

import (
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

type resourceActionSemantic struct {
	FunctionID    string
	Subject       string
	IdentityInput string
}

// GenerateResourcePageProposal generates a ResourcePage proposal from
// persistent CapabilitySemantics and FunctionContracts. This path is not driven
// by SDK registration UI metadata.
func GenerateResourcePageProposal(
	semantics *model.CapabilitySemantics,
	contracts []*model.FunctionContract,
	opts GenerateOptions,
) (spec.GeneratedPageSpec, bool) {
	opts = normalizeOptions(opts)
	if semantics == nil || strings.TrimSpace(semantics.ResourceKey) == "" {
		return spec.GeneratedPageSpec{}, false
	}
	if semantics.CollectionQueryID == 0 || strings.TrimSpace(semantics.IdentityField) == "" {
		return spec.GeneratedPageSpec{}, false
	}

	collectionContract := findResourceContract(contracts, semantics.CollectionQueryID, spec.CapabilityCollectionQuery)
	if collectionContract == nil {
		return spec.GeneratedPageSpec{}, false
	}

	createContract := findResourceContract(contracts, semantics.CreateID, spec.CapabilityCreate)
	updateContract := findResourceContract(contracts, semantics.UpdateID, spec.CapabilityUpdate)
	deleteContract := findResourceContract(contracts, semantics.DeleteID, spec.CapabilityDelete)
	itemContract := findResourceContract(contracts, semantics.ItemQueryID, spec.CapabilityItemQuery)

	resourceKey := strings.TrimSpace(semantics.ResourceKey)
	pageKey := "resource--" + sanitizeSourceKey(resourceKey)
	locale := opts.DefaultLocale
	title := localizedTitleFallback(resourceKey, locale)
	if term, ok := opts.Terms.Lookup("resource", resourceKey); ok && len(term) > 0 {
		title = term
	}
	listView := buildListViewFromContract(collectionContract, semantics)
	detailView := buildDetailViewFromContracts(collectionContract, itemContract, semantics)

	bindings := []spec.PageFunctionBinding{
		resourceBinding(collectionContract, "list", spec.BindingUsageQuery, semantics),
	}
	if detailBinding, ok := buildDetailBinding(itemContract, semantics); ok {
		bindings = append(bindings, detailBinding)
	}
	if createContract != nil {
		bindings = append(bindings, resourceBinding(createContract, "create", spec.BindingUsageAction, nil))
	}
	rowActions := []spec.ActionSpec{}
	if updateContract != nil {
		bindings = append(bindings, resourceBinding(updateContract, "update", spec.BindingUsageAction, semantics))
		rowActions = append(rowActions, spec.ActionSpec{
			Key:       "edit",
			Title:     spec.LocalizedText{locale: "编辑"},
			Type:      "link",
			BindingID: "update",
		})
	}
	if deleteContract != nil {
		bindings = append(bindings, resourceBinding(deleteContract, "delete", spec.BindingUsageAction, semantics))
	}
	inlineRowActions, inlineBatchActions, inlineToolbarActions, inlineBindings, inlineDiags := buildInlineResourceActions(semantics, contracts, locale)
	if listView != nil {
		listView.RowActions = append(rowActions, inlineRowActions...)
		listView.BatchActions = inlineBatchActions
		listView.ToolbarActions = inlineToolbarActions
	}

	diags := assessResourceSemantics(semantics)
	for _, contract := range contracts {
		if contract == nil {
			continue
		}
		diags = append(diags, schemaSubsetDiagnostics(contract.FunctionID, "inputSchema", spec.JSONSchema(contract.InputSchema))...)
		diags = append(diags, schemaSubsetDiagnostics(contract.FunctionID, "outputSchema", spec.JSONSchema(contract.OutputSchema))...)
	}
	diags = append(diags, inlineDiags...)
	diags = append(diags, validateGeneratedResourceViews(listView, detailView, semantics)...)
	bindings = append(bindings, inlineBindings...)
	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeResource,
			ResourceKey: resourceKey,
			Title:       title,
			Category:    categoryForResource(resourceKey),
			Navigation: &spec.NavigationSpec{
				Title: title,
			},
			Resource: &spec.ResourcePageSpec{
				ListView:     listView,
				DetailView:   detailView,
				CreateForm:   buildFormFromContract(createContract),
				UpdateForm:   buildUpdateFormFromContract(updateContract, semantics),
				DeleteAction: buildDeleteAction(deleteContract, locale),
			},
			Bindings: bindings,
		},
		Quality:     resourceQuality(semantics, diags),
		Diagnostics: diags,
	}, true
}
