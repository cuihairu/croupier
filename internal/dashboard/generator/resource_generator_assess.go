package generator

import (
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

// assessResourceSemantics checks if semantics are sufficient for CRUD.
func assessResourceSemantics(semantics *model.CapabilitySemantics) []spec.Diagnostic {
	var diags []spec.Diagnostic
	if semantics == nil {
		return []spec.Diagnostic{{
			Code:     "resource_semantics_missing",
			Severity: spec.SeverityError,
			Message:  "resource capability semantics are required",
		}}
	}
	if strings.TrimSpace(semantics.IdentityField) == "" {
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
	var conflicts []spec.SemanticConflict
	if len(semantics.Conflicts) > 0 {
		if err := json.Unmarshal(semantics.Conflicts, &conflicts); err != nil {
			diags = append(diags, spec.Diagnostic{
				Code:     "semantic_conflicts_invalid",
				Severity: spec.SeverityError,
				Message:  "capability semantics conflicts cannot be decoded",
				Field:    "capabilitySemantics.conflicts",
			})
		}
	}
	for _, conflict := range conflicts {
		if conflict.Resolution == "" {
			diags = append(diags, spec.Diagnostic{
				Code:     "semantic_conflict_unresolved",
				Severity: spec.SeverityError,
				Message:  "capability semantic conflict requires platform review before publishing",
				Field:    strings.TrimSpace(conflict.Field),
			})
		}
	}
	return diags
}

func validateGeneratedResourceViews(
	listView *spec.ListViewSpec,
	detailView *spec.DetailViewSpec,
	semantics *model.CapabilitySemantics,
) []spec.Diagnostic {
	if listView == nil || semantics == nil {
		return nil
	}
	var diags []spec.Diagnostic
	identityField := strings.TrimSpace(semantics.IdentityField)
	if identityField != "" && !listViewHasColumn(listView, identityField) {
		diags = append(diags, spec.Diagnostic{
			Code:     "resource_identity_column_missing",
			Severity: spec.SeverityError,
			Message:  "resource list view could not derive the identity column from collection item schema",
			Field:    "resource.listView.identityKey",
		})
	}
	return diags
}

func listViewHasColumn(listView *spec.ListViewSpec, key string) bool {
	for _, column := range listView.Columns {
		if strings.TrimSpace(column.Key) == key {
			return true
		}
	}
	return false
}

func resourceQuality(semantics *model.CapabilitySemantics, diags []spec.Diagnostic) spec.GeneratedPageQuality {
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			return spec.GeneratedPageQualityNeedsReview
		}
		if d.Code == "json_schema_generation_subset_unsupported" {
			return spec.GeneratedPageQualityNeedsReview
		}
		if d.Severity == spec.SeverityWarning {
			return spec.GeneratedPageQualityBasic
		}
	}
	if semantics != nil && semantics.CollectionQueryID > 0 && strings.TrimSpace(semantics.IdentityField) != "" {
		return spec.GeneratedPageQualityReady
	}
	return spec.GeneratedPageQualityBasic
}
