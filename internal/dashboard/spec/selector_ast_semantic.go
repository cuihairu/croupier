package spec

// SemanticContext provides semantic context for selector validation.
type SemanticContext struct {
	// IdentityField is the resource identity field name (e.g., "player_id")
	IdentityField string

	// PageFieldName is the pagination page field name (e.g., "page")
	PageFieldName string

	// PageSizeFieldName is the pagination page size field name (e.g., "page_size")
	PageSizeFieldName string

	// ItemsFieldName is the items array field name (e.g., "items")
	ItemsFieldName string

	// TotalFieldName is the total count field name (e.g., "total")
	TotalFieldName string
}

// ValidateSelectorSemantics validates that selector assignments are consistent
// with the resource's CapabilitySemantics. This ensures that:
// - Identity fields are properly mapped
// - Collection query fields are properly mapped
// - CRUD operations use correct identity inputs
func ValidateSelectorSemantics(
	selector SelectorAST,
	semantics *SemanticContext,
	pageType PageType,
	bindingUsage PageBindingUsage,
) SelectorValidationResult {
	result := SelectorValidationResult{Valid: true}

	if semantics == nil {
		return result
	}

	// Check identity field mapping for resource operations
	if pageType == PageTypeResource && semantics.IdentityField != "" {
		validateIdentityMapping(selector, semantics, bindingUsage, &result)
	}

	// Check collection query mapping
	if bindingUsage == BindingUsageQuery {
		validateCollectionMapping(selector, semantics, &result)
	}

	return result
}

func validateIdentityMapping(
	selector SelectorAST,
	semantics *SemanticContext,
	bindingUsage PageBindingUsage,
	result *SelectorValidationResult,
) {
	// For row actions (update, delete), identity must come from row source
	if bindingUsage == BindingUsageAction {
		hasRowIdentity := false
		for _, assignment := range selector.Assignments {
			if assignment.Source.Kind == SourceRow && assignment.Source.Path != "" {
				// Check if this maps to the identity field
				if isIdentityPath(assignment.Source.Path, semantics.IdentityField) {
					hasRowIdentity = true
					break
				}
			}
		}
		if !hasRowIdentity && semantics.IdentityField != "" {
			result.addWarning("", ErrCodeMissingRequired,
				"row action should map identity field from row source")
		}
	}
}

func validateCollectionMapping(
	selector SelectorAST,
	semantics *SemanticContext,
	result *SelectorValidationResult,
) {
	// Collection query should map pagination fields
	if semantics.PageFieldName != "" || semantics.PageSizeFieldName != "" {
		hasPagination := false
		for _, assignment := range selector.Assignments {
			if assignment.Source.Kind == SourceForm {
				if assignment.Target == "/"+semantics.PageFieldName ||
					assignment.Target == "/"+semantics.PageSizeFieldName {
					hasPagination = true
					break
				}
			}
		}
		if !hasPagination && semantics.PageFieldName != "" {
			result.addWarning("", ErrCodeMissingRequired,
				"collection query should map pagination fields")
		}
	}
}

func isIdentityPath(sourcePath string, identityField string) bool {
	if identityField == "" {
		return false
	}
	// Check if source path points to identity field
	// e.g., "/player_id" or "/id"
	return sourcePath == "/"+identityField
}
